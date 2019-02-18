package app

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"git.parallelcoin.io/pod/cmd/ctl"
	"git.parallelcoin.io/pod/cmd/node"
	"git.parallelcoin.io/pod/cmd/shell"
	walletmain "git.parallelcoin.io/pod/cmd/wallet"
	cl "git.parallelcoin.io/pod/pkg/clog"
	"git.parallelcoin.io/pod/pkg/fork"
	"github.com/tucnak/climax"
)

var confFile = DefaultDataDir + "/conf"

const lH = "127.0.0.1:"

type tokenalias int

// ConfCfg is the settings that can be set to synchronise across all pod modules
type ConfCfg struct {
	DataDir          string
	ConfigFile       string
	NodeListeners    []string
	NodeRPCListeners []string
	WalletListeners  []string
	NodeUser         string
	NodePass         string
	WalletPass       string
	RPCKey           string
	RPCCert          string
	CAFile           string
	TLS              bool
	SkipVerify       bool
	Proxy            string
	ProxyUser        string
	ProxyPass        string
	Network          string
}

// ConfConfig is the configuration for this tool
var ConfConfig ConfCfg

// Confs is the central repository of all the other app configurations
var Confs ConfConfigs

// ConfConfigs are the configurations for each app that are applied
type ConfConfigs struct {
	Ctl    ctl.Config
	Node   node.Config
	Wallet walletmain.Config
	Shell  shell.Config
}

// ConfCommand is a command to send RPC queries to bitcoin RPC protocol server for node and wallet queries
var ConfCommand = climax.Command{
	Name:  "conf",
	Brief: "sets configurations common across modules",
	Help:  "automates synchronising common settings between servers and clients",
	Flags: []climax.Flag{
		t("version", "V", "show version number and quit"),
		t("init", "i", "resets configuration to defaults"),
		t("show", "s", "prints currently configuration"),

		f("createtest", "1", "create test configuration (set number to create max 10)"),
		f("testname", "test", "base name for test configurations"),
		f("testportbase", "21047", "base port number for test configurations"),

		s("datadir", "D", "~/.pod", "where to create the new profile"),

		f("nodelistener", node.DefaultListener,
			"main peer to peer address for apps that connect to the parallelcoin peer to peer network"),
		f("noderpclistener", node.DefaultRPCListener,
			"address where node listens for RPC"),
		f("walletlistener", walletmain.DefaultListener, "address where wallet listens for RPC"),
		s("user", "u", "user", "username for all the things"),
		s("pass", "P", "pa55word", "password for all the things"),
		s("walletpass", "", "w", "public password for wallet"),
		f("rpckey", walletmain.DefaultRPCKeyFile,
			"RPC server certificate key"),
		f("rpccert", walletmain.DefaultRPCCertFile,
			"RPC server certificate"),
		f("cafile", walletmain.DefaultCAFile,
			"RPC server certificate chain for validation"),
		f("tls", "false", "enable TLS"),
		f("skipverify", "false", "do not verify TLS certificates (not recommended!)"),
		f("proxy", "127.0.0.1:9050", "connect via SOCKS5 proxy"),
		f("proxyuser", "user", "username for proxy"),
		f("proxypass", "pa55word", "password for proxy"),

		f("network", "mainnet", "connect to [mainnet|testnet|regtestnet|simnet]"),
		s("debuglevel", "d", "info", "sets log level for those unspecified below"),
	},
	Examples: []climax.Example{
		{
			Usecase:     "--D test --init",
			Description: "creates a new data directory at test",
		},
	},
	Handle: func(ctx climax.Context) int {
		var dl, ct, tpb string
		var ok bool
		if dl, ok = ctx.Get("debuglevel"); ok {
			log <- cl.Tracef{
				"setting debug level %s",
				dl,
			}
			Log.SetLevel(dl)
			ll := GetAllSubSystems()
			for i := range ll {
				ll[i].SetLevel(dl)
			}
		}

		if ct, ok = ctx.Get("createtest"); ok {
			testname := "test"
			testnum := 1
			testportbase := 21047
			if err := ParseInteger(
				ct, "createtest", &testnum,
			); err != nil {
				log <- cl.Wrn(err.Error())
			}
			if tn, ok := ctx.Get("testname"); ok {
				testname = tn
			}
			if tpb, ok = ctx.Get("testportbase"); ok {
				if err := ParseInteger(
					tpb, "testportbase", &testportbase,
				); err != nil {
					log <- cl.Wrn(err.Error())
				}
			}
			// Generate a full set of default configs first
			var testConfigSet []ConfigSet
			for i := 0; i < testnum; i++ {
				tn := fmt.Sprintf("%s%d", testname, i)
				cs := GetDefaultConfs(tn)
				SyncToConfs(cs)
				testConfigSet = append(testConfigSet, *cs)
			}
			var ps []PortSet
			for i := 0; i < testnum; i++ {
				p := GenPortSet(testportbase + 100*i)
				ps = append(ps, *p)
			}
			// Set the correct listeners and add the correct addpeers entries
			for i, ts := range testConfigSet {
				// conf
				tc := ts.Conf
				tc.NodeListeners = []string{
					lH + ps[i].P2P,
				}
				tc.NodeRPCListeners = []string{
					lH + ps[i].NodeRPC,
				}
				tc.WalletListeners = []string{
					lH + ps[i].WalletRPC,
				}
				tc.TLS = false
				tc.Network = "testnet"
				// ctl
				tcc := ts.Ctl
				tcc.SimNet = false
				tcc.RPCServer = ts.Conf.NodeRPCListeners[0]
				tcc.TestNet3 = true
				tcc.TLS = false
				tcc.Wallet = ts.Conf.WalletListeners[0]
				// node
				tnn := ts.Node.Node
				for j := range ps {
					// add all other peers in the portset list
					if j != i {
						tnn.AddPeers = append(
							tnn.AddPeers,
							lH+ps[j].P2P,
						)
					}
				}
				tnn.Listeners = tc.NodeListeners
				tnn.RPCListeners = tc.NodeRPCListeners
				tnn.SimNet = false
				tnn.TestNet3 = true
				tnn.RegressionTest = false
				tnn.TLS = false
				// wallet
				tw := ts.Wallet.Wallet
				tw.EnableClientTLS = false
				tw.EnableServerTLS = false
				tw.LegacyRPCListeners = ts.Conf.WalletListeners
				tw.RPCConnect = tc.NodeRPCListeners[0]
				tw.SimNet = false
				tw.TestNet3 = true
				// shell
				tss := ts.Shell
				// shell/node
				tsn := tss.Node
				tsn.Listeners = tnn.Listeners
				tsn.RPCListeners = tnn.RPCListeners
				tsn.TestNet3 = true
				tsn.SimNet = true
				for j := range ps {
					// add all other peers in the portset list
					if j != i {
						tsn.AddPeers = append(
							tsn.AddPeers,
							lH+ps[j].P2P,
						)
					}
				}
				tsn.SimNet = false
				tsn.TestNet3 = true
				tsn.RegressionTest = false
				tsn.TLS = false
				// shell/wallet
				tsw := tss.Wallet
				tsw.EnableClientTLS = false
				tsw.EnableServerTLS = false
				tsw.LegacyRPCListeners = ts.Conf.WalletListeners
				tsw.RPCConnect = tcc.RPCServer
				tsw.SimNet = false
				tsw.TestNet3 = true
				// write to disk
				WriteConfigSet(&ts)
			}
			os.Exit(0)
		}

		confFile = DefaultDataDir + "/conf.json"
		if r, ok := ctx.Get("datadir"); ok {
			DefaultDataDir = r
			confFile = DefaultDataDir + "/conf.json"
		}
		confs = []string{
			DefaultDataDir + "/ctl/conf.json",
			DefaultDataDir + "/node/conf.json",
			DefaultDataDir + "/wallet/conf.json",
			DefaultDataDir + "/shell/conf.json",
		}
		for i := range confs {
			EnsureDir(confs[i])
		}
		EnsureDir(confFile)
		if ctx.Is("init") {
			WriteDefaultConfConfig(DefaultDataDir)
			WriteDefaultCtlConfig(DefaultDataDir)
			WriteDefaultNodeConfig(DefaultDataDir)
			WriteDefaultWalletConfig(DefaultDataDir)
			WriteDefaultShellConfig(DefaultDataDir)
		} else {
			if _, err := os.Stat(confFile); os.IsNotExist(err) {
				WriteDefaultConfConfig(DefaultDataDir)
			} else {
				cfgData, err := ioutil.ReadFile(confFile)
				if err != nil {
					WriteDefaultConfConfig(DefaultDataDir)
				}
				err = json.Unmarshal(cfgData, &ConfConfig)
				if err != nil {
					WriteDefaultConfConfig(DefaultDataDir)
				}
			}
		}
		configConf(&ctx, DefaultDataDir, node.DefaultPort)
		runConf()
		return 0
	},
	// Examples: []climax.Example{
	// 	{
	// 		Usecase:     "--nodeuser=user --nodepass=pa55word",
	// 		Description: "set the username and password for the node RPC",
	// 	},
	// },
	// Handle:
}

var confs []string

func init() {
}

// cf is the list of flags and the default values stored in the Usage field
var cf = GetFlags(ConfCommand)

func configConf(ctx *climax.Context, datadir, portbase string) {
	cs := GetDefaultConfs(datadir)
	SyncToConfs(cs)
	var r string
	var ok bool
	var listeners []string
	if r, ok = getIfIs(ctx, "nodelistener"); ok {
		NormalizeAddresses(r, portbase, &listeners)
		fmt.Println("nodelistener set to", listeners)
		ConfConfig.NodeListeners = listeners
		cs.Node.Node.Listeners = listeners
		cs.Shell.Node.Listeners = listeners
	}
	if r, ok = getIfIs(ctx, "noderpclistener"); ok {
		NormalizeAddresses(r, node.DefaultRPCPort, &listeners)
		fmt.Println("noderpclistener set to", listeners)
		ConfConfig.NodeRPCListeners = listeners
		cs.Node.Node.RPCListeners = listeners
		cs.Wallet.Wallet.RPCConnect = r
		cs.Shell.Node.RPCListeners = listeners
		cs.Shell.Wallet.RPCConnect = r
		cs.Ctl.RPCServer = r
	}
	if r, ok = getIfIs(ctx, "walletlistener"); ok {
		NormalizeAddresses(r, node.DefaultRPCPort, &listeners)
		fmt.Println("walletlistener set to", listeners)
		ConfConfig.WalletListeners = listeners
		cs.Wallet.Wallet.LegacyRPCListeners = listeners
		cs.Ctl.Wallet = r
		cs.Shell.Wallet.LegacyRPCListeners = listeners
	}
	if r, ok = getIfIs(ctx, "user"); ok {
		ConfConfig.NodeUser = r
		cs.Node.Node.RPCUser = r
		cs.Wallet.Wallet.PodUsername = r
		cs.Wallet.Wallet.Username = r
		cs.Shell.Node.RPCUser = r
		cs.Shell.Wallet.PodUsername = r
		cs.Shell.Wallet.Username = r
		cs.Ctl.RPCUser = r
	}
	if r, ok = getIfIs(ctx, "pass"); ok {
		ConfConfig.NodePass = r
		cs.Node.Node.RPCPass = r
		cs.Wallet.Wallet.PodPassword = r
		cs.Wallet.Wallet.Password = r
		cs.Shell.Node.RPCPass = r
		cs.Shell.Wallet.PodPassword = r
		cs.Shell.Wallet.Password = r
		cs.Ctl.RPCPass = r
	}
	if r, ok = getIfIs(ctx, "walletpass"); ok {
		ConfConfig.WalletPass = r
		cs.Wallet.Wallet.WalletPass = ConfConfig.WalletPass
		cs.Shell.Wallet.WalletPass = ConfConfig.WalletPass
	}

	if r, ok = getIfIs(ctx, "rpckey"); ok {
		r = node.CleanAndExpandPath(r)
		ConfConfig.RPCKey = r
		cs.Node.Node.RPCKey = r
		cs.Wallet.Wallet.RPCKey = r
		cs.Shell.Node.RPCKey = r
		cs.Shell.Wallet.RPCKey = r
	}
	if r, ok = getIfIs(ctx, "rpccert"); ok {
		r = node.CleanAndExpandPath(r)
		ConfConfig.RPCCert = r
		cs.Node.Node.RPCCert = r
		cs.Wallet.Wallet.RPCCert = r
		cs.Shell.Node.RPCCert = r
		cs.Shell.Wallet.RPCCert = r
	}
	if r, ok = getIfIs(ctx, "cafile"); ok {
		r = node.CleanAndExpandPath(r)
		ConfConfig.CAFile = r
		cs.Wallet.Wallet.CAFile = r
		cs.Shell.Wallet.CAFile = r
	}
	if r, ok = getIfIs(ctx, "tls"); ok {
		ConfConfig.TLS = r == "true"
		cs.Node.Node.TLS = ConfConfig.TLS
		cs.Wallet.Wallet.EnableClientTLS = ConfConfig.TLS
		cs.Shell.Node.TLS = ConfConfig.TLS
		cs.Shell.Wallet.EnableClientTLS = ConfConfig.TLS
		cs.Wallet.Wallet.EnableServerTLS = ConfConfig.TLS
		cs.Shell.Wallet.EnableServerTLS = ConfConfig.TLS
	}
	if r, ok = getIfIs(ctx, "skipverify"); ok {
		ConfConfig.SkipVerify = r == "true"
		cs.Ctl.TLSSkipVerify = r == "true"
	}
	if r, ok = getIfIs(ctx, "proxy"); ok {
		NormalizeAddresses(r, node.DefaultRPCPort, &listeners)
		ConfConfig.Proxy = r
		cs.Ctl.Proxy = ConfConfig.Proxy
		cs.Node.Node.Proxy = ConfConfig.Proxy
		cs.Wallet.Wallet.Proxy = ConfConfig.Proxy
		cs.Shell.Node.Proxy = ConfConfig.Proxy
		cs.Shell.Wallet.Proxy = ConfConfig.Proxy
	}
	if r, ok = getIfIs(ctx, "proxyuser"); ok {
		ConfConfig.ProxyUser = r
		cs.Ctl.ProxyUser = ConfConfig.ProxyUser
		cs.Node.Node.ProxyUser = ConfConfig.ProxyUser
		cs.Wallet.Wallet.ProxyUser = ConfConfig.ProxyUser
		cs.Shell.Node.ProxyUser = ConfConfig.ProxyUser
		cs.Shell.Wallet.ProxyUser = ConfConfig.ProxyUser
	}
	if r, ok = getIfIs(ctx, "proxypass"); ok {
		ConfConfig.ProxyPass = r
		cs.Ctl.ProxyPass = ConfConfig.ProxyPass
		cs.Node.Node.ProxyPass = ConfConfig.ProxyPass
		cs.Wallet.Wallet.ProxyPass = ConfConfig.ProxyPass
		cs.Shell.Node.ProxyPass = ConfConfig.ProxyPass
		cs.Shell.Wallet.ProxyPass = ConfConfig.ProxyPass
	}
	if r, ok = getIfIs(ctx, "network"); ok {
		r = strings.ToLower(r)
		switch r {
		case "mainnet", "testnet", "regtestnet", "simnet":
		default:
			r = "mainnet"
		}
		ConfConfig.Network = r
		fmt.Println("configured for", r, "network")
		switch r {
		case "mainnet":
			cs.Ctl.TestNet3 = false
			cs.Ctl.SimNet = false
			cs.Node.Node.TestNet3 = false
			cs.Node.Node.SimNet = false
			cs.Node.Node.RegressionTest = false
			cs.Wallet.Wallet.SimNet = false
			cs.Wallet.Wallet.TestNet3 = false
			cs.Shell.Node.TestNet3 = false
			cs.Shell.Node.RegressionTest = false
			cs.Shell.Node.SimNet = false
			cs.Shell.Wallet.TestNet3 = false
			cs.Shell.Wallet.SimNet = false
		case "testnet":
			fork.IsTestnet = true
			cs.Ctl.TestNet3 = true
			cs.Ctl.SimNet = false
			cs.Node.Node.TestNet3 = true
			cs.Node.Node.SimNet = false
			cs.Node.Node.RegressionTest = false
			cs.Wallet.Wallet.SimNet = false
			cs.Wallet.Wallet.TestNet3 = true
			cs.Shell.Node.TestNet3 = true
			cs.Shell.Node.RegressionTest = false
			cs.Shell.Node.SimNet = false
			cs.Shell.Wallet.TestNet3 = true
			cs.Shell.Wallet.SimNet = false
		case "regtestnet":
			cs.Ctl.TestNet3 = false
			cs.Ctl.SimNet = false
			cs.Node.Node.TestNet3 = false
			cs.Node.Node.SimNet = false
			cs.Node.Node.RegressionTest = true
			cs.Wallet.Wallet.SimNet = false
			cs.Wallet.Wallet.TestNet3 = false
			cs.Shell.Node.TestNet3 = false
			cs.Shell.Node.RegressionTest = true
			cs.Shell.Node.SimNet = false
			cs.Shell.Wallet.TestNet3 = false
			cs.Shell.Wallet.SimNet = false
		case "simnet":
			cs.Ctl.TestNet3 = false
			cs.Ctl.SimNet = true
			cs.Node.Node.TestNet3 = false
			cs.Node.Node.SimNet = true
			cs.Node.Node.RegressionTest = false
			cs.Wallet.Wallet.SimNet = true
			cs.Wallet.Wallet.TestNet3 = false
			cs.Shell.Node.TestNet3 = false
			cs.Shell.Node.RegressionTest = false
			cs.Shell.Node.SimNet = true
			cs.Shell.Wallet.TestNet3 = false
			cs.Shell.Wallet.SimNet = true
		}
	}

	WriteConfConfig(cs.Conf)
	// Now write the configs for all the others reading them and overwriting the changed values
	WriteCtlConfig(cs.Ctl)
	WriteNodeConfig(cs.Node)
	WriteWalletConfig(cs.Wallet)
	WriteShellConfig(cs.Shell)
	if ctx.Is("show") {
		j, err := json.MarshalIndent(cs.Conf, "", "  ")
		if err != nil {
			panic(err.Error())
		}
		fmt.Println(string(j))
	}
}

// WriteConfConfig creates and writes the config file in the requested location
func WriteConfConfig(cfg *ConfCfg) {
	j, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		panic(err.Error())
	}
	j = append(j, '\n')
	EnsureDir(cfg.ConfigFile)
	err = ioutil.WriteFile(cfg.ConfigFile, j, 0600)
	if err != nil {
		panic(err.Error())
	}
}

// WriteDefaultConfConfig creates and writes a default config file in the requested location
func WriteDefaultConfConfig(datadir string) {
	defCfg := DefaultConfConfig(datadir)
	j, err := json.MarshalIndent(defCfg, "", "  ")
	if err != nil {
		panic(err.Error())
	}
	j = append(j, '\n')
	EnsureDir(defCfg.ConfigFile)
	err = ioutil.WriteFile(defCfg.ConfigFile, j, 0600)
	if err != nil {
		panic(err.Error())
	}
	// if we are writing default config we also want to use it
	ConfConfig = *defCfg
}

// DefaultConfConfig returns a crispy fresh default conf configuration
func DefaultConfConfig(datadir string) *ConfCfg {
	u := GenKey()
	p := GenKey()
	return &ConfCfg{
		DataDir:          datadir,
		ConfigFile:       filepath.Join(datadir, "conf.json"),
		NodeListeners:    []string{node.DefaultListener},
		NodeRPCListeners: []string{node.DefaultRPCListener},
		WalletListeners:  []string{walletmain.DefaultListener},
		NodeUser:         u,
		NodePass:         p,
		WalletPass:       "",
		RPCCert:          filepath.Join(datadir, "rpc.cert"),
		RPCKey:           filepath.Join(datadir, "rpc.key"),
		CAFile: filepath.Join(
			datadir, walletmain.DefaultCAFilename),
		TLS:        false,
		SkipVerify: false,
		Proxy:      "",
		ProxyUser:  "",
		ProxyPass:  "",
		Network:    "mainnet",
	}
}
package app

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	w "git.parallelcoin.io/pod/cmd/wallet"
	walletmain "git.parallelcoin.io/pod/cmd/wallet"
	cl "git.parallelcoin.io/pod/pkg/clog"
	"git.parallelcoin.io/pod/pkg/netparams"
	"git.parallelcoin.io/pod/pkg/util/hdkeychain"
	"git.parallelcoin.io/pod/pkg/wallet"
	"github.com/tucnak/climax"
)

// CreateCfg is the type for the default config data
type CreateCfg struct {
	DataDir    string
	Password   string
	PublicPass string
	Seed       []byte
	Network    string
	Config     *walletmain.Config
}

// CreateConfig is
var CreateConfig = CreateCfg{
	DataDir: walletmain.DefaultAppDataDir,
	Network: "mainnet",
}

// CreateCommand is a command to send RPC queries to bitcoin RPC protocol server for node and wallet queries
var CreateCommand = climax.Command{
	Name:  "create",
	Brief: "creates a new wallet",
	Help:  "creates a new wallet in specified data directory for a specified network",
	Flags: []climax.Flag{
		t("help", "h", "show help text"),
		s("datadir", "D", walletmain.DefaultAppDataDir, "specify where the wallet will be created"),
		s("seed", "s", "", "input pre-existing seed"),
		s("password", "p", "", "specify password for private data"),
		s("publicpass", "P", "", "specify password for public data"),
		t("cli", "c", "use commandline interface interactive input"),
		f("network", "mainnet", "connect to (mainnet|testnet|simnet)"),
	},
	Handle: func(ctx climax.Context) int {
		if ctx.Is("help") {
			fmt.Print(`Usage: create [-h] [-d] [-s] [-p] [-P] [-c] [--network]

creates a new wallet given CLI flags, or interactively

Available options:

	-h, --help
		show help text
	-D, --datadir="~/.pod/wallet"
		specify where the wallet will be created
	-s, --seed=""
		input pre-existing seed
	-p, --password=""
		specify password for private data
	-P, --publicpass=""
		specify password for public data
	-c, --cli
		use commandline interface interactive input
	--network="mainnet"
		connect to (mainnet|testnet|simnet)
`)
			return 0
		}
		argsGiven := false
		CreateConfig.DataDir = w.DefaultDataDir
		if r, ok := getIfIs(&ctx, "datadir"); ok {
			CreateConfig.DataDir = r
		}
		var cfgFile string
		var ok bool
		if cfgFile, ok = ctx.Get("configfile"); !ok {
			cfgFile = filepath.Join(
				filepath.Join(CreateConfig.DataDir, "wallet"),
				w.DefaultConfigFilename)
			argsGiven = true
		}
		if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
			fmt.Println("configuration file does not exist, creating new one")
			WriteDefaultWalletConfig(CreateConfig.DataDir)
		} else {
			fmt.Println("reading app configuration from", cfgFile)
			cfgData, err := ioutil.ReadFile(cfgFile)
			if err != nil {
				fmt.Println("reading app config file", err.Error())
				WriteDefaultWalletConfig(CreateConfig.DataDir)
			}
			log <- cl.Tracef{"parsing app configuration\n%s", cfgData}
			err = json.Unmarshal(cfgData, &WalletConfig)
			if err != nil {
				fmt.Println("parsing app config file", err.Error())
				WriteDefaultWalletConfig(CreateConfig.DataDir)
			}
		}
		CreateConfig.Config = WalletConfig.Wallet
		activeNet := walletmain.ActiveNet
		CreateConfig.Config.TestNet3 = false
		CreateConfig.Config.SimNet = false
		if r, ok := getIfIs(&ctx, "network"); ok {
			switch r {
			case "testnet":
				activeNet = &netparams.TestNet3Params
				CreateConfig.Config.TestNet3 = true
				CreateConfig.Config.SimNet = false
			case "simnet":
				activeNet = &netparams.SimNetParams
				CreateConfig.Config.TestNet3 = false
				CreateConfig.Config.SimNet = true
			default:
				activeNet = &netparams.MainNetParams
			}
			CreateConfig.Network = r
			argsGiven = true
		}

		if CreateConfig.Config.TestNet3 {
			activeNet = &netparams.TestNet3Params
			CreateConfig.Config.TestNet3 = true
			CreateConfig.Config.SimNet = false
		}
		if CreateConfig.Config.SimNet {
			activeNet = &netparams.SimNetParams
			CreateConfig.Config.TestNet3 = false
			CreateConfig.Config.SimNet = true
		}
		CreateConfig.Config.AppDataDir = filepath.Join(
			CreateConfig.DataDir, "wallet")
		// spew.Dump(CreateConfig)
		dbDir := walletmain.NetworkDir(
			filepath.Join(CreateConfig.DataDir, "wallet"), activeNet.Params)
		loader := wallet.NewLoader(
			walletmain.ActiveNet.Params, dbDir, 250)
		exists, err := loader.WalletExists()
		if err != nil {
			fmt.Println("ERROR", err)
			return 1
		}
		if !exists {

		} else {
			fmt.Println("\n!!! A wallet already exists at '" + dbDir + "' !!! \n")
			fmt.Println("if you are sure it isn't valuable you can delete it before running this again")
			return 1
		}
		if ctx.Is("cli") {
			walletmain.CreateWallet(CreateConfig.Config, activeNet)
			fmt.Print("\nYou can now open the wallet\n")
			return 0
		}
		if r, ok := getIfIs(&ctx, "seed"); ok {
			CreateConfig.Seed = []byte(r)
			argsGiven = true
		}
		if r, ok := getIfIs(&ctx, "password"); ok {
			CreateConfig.Password = r
			argsGiven = true
		}
		if r, ok := getIfIs(&ctx, "publicpass"); ok {
			CreateConfig.PublicPass = r
			argsGiven = true
		}
		if argsGiven {
			if CreateConfig.Password == "" {
				fmt.Println("no password given")
				return 1
			}
			if CreateConfig.Seed == nil {
				seed, err := hdkeychain.GenerateSeed(hdkeychain.RecommendedSeedLen)
				if err != nil {
					fmt.Println("failed to generate new seed")
					return 1
				}
				fmt.Println("Your wallet generation seed is:")
				fmt.Printf("\n%x\n\n", seed)
				fmt.Print("IMPORTANT: Keep the seed in a safe place as you will NOT be able to restore your wallet without it.\n\n")
				fmt.Print("Please keep in mind that anyone who has access to the seed can also restore your wallet thereby giving them access to all your funds, so it is imperative that you keep it in a secure location.\n\n")
				CreateConfig.Seed = []byte(seed)
			}
			w, err := loader.CreateNewWallet(
				[]byte(CreateConfig.PublicPass),
				[]byte(CreateConfig.Password),
				CreateConfig.Seed,
				time.Now())
			if err != nil {
				fmt.Println(err)
				return 1
			}
			fmt.Println("Wallet creation completed")
			fmt.Println("Seed:", string(CreateConfig.Seed))
			fmt.Println("Password: '" + string(CreateConfig.Password) + "'")
			fmt.Println("Public Password: '" + string(CreateConfig.PublicPass) + "'")
			w.Manager.Close()
			return 0

		}
		return 0
	},
}
package app

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"git.parallelcoin.io/pod/cmd/ctl"
	w "git.parallelcoin.io/pod/cmd/wallet"
	cl "git.parallelcoin.io/pod/pkg/clog"
	"github.com/tucnak/climax"
)

// CtlCfg is the default configuration native to ctl
var CtlCfg = new(ctl.Config)

// CtlCommand is a command to send RPC queries to bitcoin RPC protocol server for node and wallet queries
var CtlCommand = climax.Command{
	Name:  "ctl",
	Brief: "sends RPC commands and prints the reply",
	Help:  "Send queries to bitcoin JSON-RPC servers using command line shell and prints the reply to stdout",
	Flags: []climax.Flag{
		t("version", "V", "show version number and quit"),
		s("configfile", "C", ctl.DefaultConfigFile, "Path to configuration file"),
		s("datadir", "D", w.DefaultDataDir,
			"set the pod base directory"),

		t("init", "", "resets configuration to defaults"),
		t("save", "", "saves current configuration"),
		t("wallet", "w", "uses configured walletrpc instead of full node rpc"),

		f("walletrpc", ctl.DefaultRPCServer,
			"wallet RPC address to try when given wallet RPC queries"),
		s("rpcuser", "u", "user", "RPC username"),
		s("rpcpass", "P", "pa55word", "RPC password"),
		s("rpcserver", "s", "127.0.0.1:11048", "RPC server to connect to"),
		f("tls", "false", "enable/disable (true|false)"),
		s("rpccert", "c", "rpc.cert", "RPC server certificate chain for validation"),
		f("skipverify", "false", "do not verify tls certificates"),

		s("debuglevel", "d", "off", "sets logging level (off|fatal|error|info|debug|trace)"),

		f("proxy", "", "connect via SOCKS5 proxy"),
		f("proxyuser", "user", "username for proxy server"),
		f("proxypass", "pa55word", "password for proxy server"),

		f("network", "mainnet", "connect to (mainnet|testnet|simnet)"),
	},
	Examples: []climax.Example{
		{
			Usecase:     "-l",
			Description: "lists available commands",
		},
	},
	Handle: func(ctx climax.Context) int {
		Log.SetLevel("off")
		if dl, ok := ctx.Get("debuglevel"); ok {
			log <- cl.Trace{
				"setting debug level", dl,
			}
			Log.SetLevel(dl)
		}
		log <- cl.Debug{
			"pod/ctl version", ctl.Version(),
		}
		if ctx.Is("version") {
			fmt.Println("pod/ctl version", ctl.Version())
			return 0
		}
		if ctx.Is("listcommands") {
			ctl.ListCommands()
		} else {
			var cfgFile, datadir string
			var ok bool
			if cfgFile, ok = ctx.Get("configfile"); !ok {
				cfgFile = ctl.DefaultConfigFile
			}
			datadir = w.DefaultDataDir
			if datadir, ok = ctx.Get("datadir"); ok {
				cfgFile = filepath.Join(filepath.Join(datadir, "ctl"), "conf.json")
				CtlCfg.ConfigFile = cfgFile
			}
			if ctx.Is("init") {
				log <- cl.Debug{
					"writing default configuration to", cfgFile,
				}
				WriteDefaultCtlConfig(datadir)
			} else {
				log <- cl.Info{
					"loading configuration from", cfgFile,
				}
				if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
					log <- cl.Wrn("configuration file does not exist, creating new one")
					WriteDefaultCtlConfig(datadir)
					// then run from this config
					configCtl(&ctx, cfgFile)
				} else {
					log <- cl.Debug{"reading from", cfgFile}
					cfgData, err := ioutil.ReadFile(cfgFile)
					if err != nil {
						WriteDefaultCtlConfig(datadir)
						log <- cl.Error{err}
					}
					log <- cl.Trace{"read in config file\n", string(cfgData)}
					err = json.Unmarshal(cfgData, CtlCfg)
					if err != nil {
						log <- cl.Err(err.Error())
						return 1
					}
				}
				// then run from this config
				configCtl(&ctx, cfgFile)
			}
		}
		log <- cl.Trace{ctx.Args}
		runCtl(ctx.Args, CtlCfg)
		return 0
	},
}

// CtlFlags is the list of flags and the default values stored in the Usage field
var CtlFlags = GetFlags(CtlCommand)

func configCtl(ctx *climax.Context, cfgFile string) {
	var r string
	var ok bool
	// Apply all configurations specified on commandline
	if r, ok = getIfIs(ctx, "debuglevel"); ok {
		CtlCfg.DebugLevel = r
		log <- cl.Trace{
			"set", "debuglevel", "to", r,
		}
	}
	if r, ok = getIfIs(ctx, "rpcuser"); ok {
		CtlCfg.RPCUser = r
		log <- cl.Tracef{
			"set %s to %s", "rpcuser", r,
		}
	}
	if r, ok = getIfIs(ctx, "rpcpass"); ok {
		CtlCfg.RPCPass = r
		log <- cl.Tracef{
			"set %s to %s", "rpcpass", r,
		}
	}
	if r, ok = getIfIs(ctx, "rpcserver"); ok {
		CtlCfg.RPCServer = r
		log <- cl.Tracef{
			"set %s to %s", "rpcserver", r,
		}
	}
	if r, ok = getIfIs(ctx, "rpccert"); ok {
		CtlCfg.RPCCert = r
		log <- cl.Tracef{"set %s to %s", "rpccert", r}
	}
	if r, ok = getIfIs(ctx, "tls"); ok {
		CtlCfg.TLS = r == "true"
		log <- cl.Tracef{"set %s to %s", "tls", r}
	}
	if r, ok = getIfIs(ctx, "proxy"); ok {
		CtlCfg.Proxy = r
		log <- cl.Tracef{"set %s to %s", "proxy", r}
	}
	if r, ok = getIfIs(ctx, "proxyuser"); ok {
		CtlCfg.ProxyUser = r
		log <- cl.Tracef{"set %s to %s", "proxyuser", r}
	}
	if r, ok = getIfIs(ctx, "proxypass"); ok {
		CtlCfg.ProxyPass = r
		log <- cl.Tracef{"set %s to %s", "proxypass", r}
	}
	otn, osn := "false", "false"
	if CtlCfg.TestNet3 {
		otn = "true"
	}
	if CtlCfg.SimNet {
		osn = "true"
	}
	tn, ts := ctx.Get("testnet")
	sn, ss := ctx.Get("simnet")
	if ts {
		CtlCfg.TestNet3 = tn == "true"
	}
	if ss {
		CtlCfg.SimNet = sn == "true"
	}
	if CtlCfg.TestNet3 && CtlCfg.SimNet {
		log <- cl.Error{
			"cannot enable simnet and testnet at the same time. current settings testnet =", otn,
			"simnet =", osn,
		}
	}
	if ctx.Is("skipverify") {
		CtlCfg.TLSSkipVerify = true
		log <- cl.Tracef{
			"set %s to true", "skipverify",
		}
	}
	if ctx.Is("wallet") {
		CtlCfg.RPCServer = CtlCfg.Wallet
		log <- cl.Trc("using configured wallet rpc server")
	}
	if r, ok = getIfIs(ctx, "walletrpc"); ok {
		CtlCfg.Wallet = r
		log <- cl.Tracef{
			"set %s to true", "walletrpc",
		}
	}
	if ctx.Is("save") {
		log <- cl.Info{
			"saving config file to",
			cfgFile,
		}
		j, err := json.MarshalIndent(CtlCfg, "", "  ")
		if err != nil {
			log <- cl.Err(err.Error())
		}
		j = append(j, '\n')
		log <- cl.Trace{
			"JSON formatted config file\n", string(j),
		}
		ioutil.WriteFile(cfgFile, j, 0600)
	}
}

// WriteCtlConfig writes the current config in the requested location
func WriteCtlConfig(cc *ctl.Config) {
	j, err := json.MarshalIndent(cc, "", "  ")
	if err != nil {
		log <- cl.Err(err.Error())
	}
	j = append(j, '\n')
	log <- cl.Tracef{"JSON formatted config file\n%s", string(j)}
	EnsureDir(cc.ConfigFile)
	err = ioutil.WriteFile(cc.ConfigFile, j, 0600)
	if err != nil {
		log <- cl.Fatal{
			"unable to write config file %s",
			err.Error(),
		}
		cl.Shutdown()
	}
}

// WriteDefaultCtlConfig writes a default config in the requested location
func WriteDefaultCtlConfig(datadir string) {
	defCfg := DefaultCtlConfig(datadir)
	j, err := json.MarshalIndent(defCfg, "", "  ")
	if err != nil {
		log <- cl.Err(err.Error())
	}
	j = append(j, '\n')
	log <- cl.Tracef{"JSON formatted config file\n%s", string(j)}
	EnsureDir(defCfg.ConfigFile)
	err = ioutil.WriteFile(defCfg.ConfigFile, j, 0600)
	if err != nil {
		log <- cl.Fatal{
			"unable to write config file %s",
			err.Error(),
		}
		cl.Shutdown()
	}
	// if we are writing default config we also want to use it
	CtlCfg = defCfg
}

// DefaultCtlConfig returns an allocated, default CtlCfg
func DefaultCtlConfig(datadir string) *ctl.Config {
	return &ctl.Config{
		ConfigFile:    filepath.Join(datadir, "ctl/conf.json"),
		DebugLevel:    "off",
		RPCUser:       "user",
		RPCPass:       "pa55word",
		RPCServer:     ctl.DefaultRPCServer,
		RPCCert:       filepath.Join(datadir, "rpc.cert"),
		TLS:           false,
		Proxy:         "",
		ProxyUser:     "",
		ProxyPass:     "",
		TestNet3:      false,
		SimNet:        false,
		TLSSkipVerify: false,
		Wallet:        ctl.DefaultWallet,
	}
}
package app

import (
	"fmt"

	"git.parallelcoin.io/pod/cmd/gui"
	"github.com/tucnak/climax"
)

// GUICfg is the type for the default config data
type GUICfg struct {
	AppDataDir string
	Password   string
	PublicPass string
	Seed       []byte
	Network    string
}

// GUICommand is a command to send RPC queries to bitcoin RPC protocol server for node and wallet queries
var GUICommand = climax.Command{
	Name:  "gui",
	Brief: "runs the GUI",
	Help:  "launches the GUI",
	Flags: []climax.Flag{
		// t("help", "h", "show help text"),
		// s("datadir", "D", walletmain.DefaultAppDataDir, "specify where the wallet will be created"),
		// f("network", "mainnet", "connect to (mainnet|testnet|simnet)"),
	},
	Handle: func(ctx climax.Context) int {
		fmt.Println("launching GUI")
		gui.GUI(ShellConfig)
		return 0
	},
}
package app

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"git.parallelcoin.io/pod/cmd/node"
	n "git.parallelcoin.io/pod/cmd/node"
	"git.parallelcoin.io/pod/cmd/node/mempool"
	cl "git.parallelcoin.io/pod/pkg/clog"
	"git.parallelcoin.io/pod/pkg/util"
	"github.com/davecgh/go-spew/spew"
	"github.com/tucnak/climax"
)

// serviceOptions defines the configuration options for the daemon as a service on Windows.
type serviceOptions struct {
	ServiceCommand string `short:"s" long:"service" description:"Service command {install, remove, start, stop}"`
}

// StateCfg is a reference to the main node state configuration struct
var StateCfg = n.StateCfg

// runServiceCommand is only set to a real function on Windows.  It is used to parse and execute service commands specified via the -s flag.
var runServiceCommand func(string) error

var aN = filepath.Base(os.Args[0])
var appName = strings.TrimSuffix(aN, filepath.Ext(aN))

var usageMessage = fmt.Sprintf("use `%s help node` to show usage", appName)

// NodeCfg is the combined app and logging configuration data
type NodeCfg struct {
	Node      *n.Config
	LogLevels map[string]string
	params    *node.Params
}

// NodeConfig is the combined app and log levels configuration
var NodeConfig = DefaultNodeConfig(n.DefaultDataDir)

// NodeCommand is a command to send RPC queries to bitcoin RPC protocol server for node and wallet queries
var NodeCommand = climax.Command{
	Name:  "node",
	Brief: "parallelcoin full node",
	Help:  "distrubutes, verifies and mines blocks for the parallelcoin duo cryptocurrency, as well as optionally providing search indexes for transactions in the database",
	Flags: []climax.Flag{

		t("version", "V", "show version number and quit"),

		s("configfile", "C", n.DefaultConfigFile, "path to configuration file"),
		s("datadir", "D", n.DefaultDataDir, "path to configuration directory"),

		t("init", "", "resets configuration to defaults"),
		t("save", "", "saves current configuration"),

		f("network", "mainnet", "connect to (mainnet|testnet|simnet)"),

		f("txindex", "true", "enable transaction index"),
		f("addrindex", "true", "enable address index"),
		t("dropcfindex", "", "delete committed filtering (CF) index then exit"),
		t("droptxindex", "", "deletes transaction index then exit"),
		t("dropaddrindex", "", "deletes the address index then exits"),

		s("listeners", "S", n.DefaultListener, "sets an address to listen for P2P connections"),
		f("externalips", "", "additional P2P listeners"),
		f("disablelisten", "false", "disables the P2P listener"),

		f("addpeers", "", "adds a peer to the peers database to try to connect to"),
		f("connectpeers", "", "adds a peer to a connect-only whitelist"),
		f(`maxpeers`, fmt.Sprint(node.DefaultMaxPeers),
			"sets max number of peers to connect to to at once"),
		f(`disablebanning`, "false",
			"disable banning of misbehaving peers"),
		f("banduration", "1d",
			"time to ban misbehaving peers (d/h/m/s)"),
		f("banthreshold", fmt.Sprint(node.DefaultBanThreshold),
			"banscore that triggers a ban"),
		f("whitelists", "", "addresses and networks immune to banning"),

		s("rpcuser", "u", "user", "RPC username"),
		s("rpcpass", "P", "pa55word", "RPC password"),

		f("rpclimituser", "user", "limited user RPC username"),
		f("rpclimitpass", "pa55word", "limited user RPC password"),

		s("rpclisteners", "s", node.DefaultRPCListener, "RPC server to connect to"),

		f("rpccert", node.DefaultRPCCertFile,
			"RPC server tls certificate chain for validation"),
		f("rpckey", node.DefaultRPCKeyFile,
			"RPC server tls key for authentication"),
		f("tls", "false", "enable TLS"),
		f("skipverify", "false", "do not verify tls certificates"),

		f("proxy", "", "connect via SOCKS5 proxy server"),
		f("proxyuser", "", "username for proxy server"),
		f("proxypass", "", "password for proxy server"),

		f("onion", "", "connect via tor proxy relay"),
		f("onionuser", "", "username for onion proxy server"),
		f("onionpass", "", "password for onion proxy server"),
		f("noonion", "false", "disable onion proxy"),
		f("torisolation", "false", "use a different user/pass for each peer"),

		f("trickleinterval", fmt.Sprint(node.DefaultTrickleInterval),
			"time between sending inventory batches to peers"),
		f("minrelaytxfee", "0", "min fee in DUO/kb to relay transaction"),
		f("freetxrelaylimit", fmt.Sprint(node.DefaultFreeTxRelayLimit),
			"limit below min fee transactions in kb/bin"),
		f("norelaypriority", "false",
			"do not discriminate transactions for relaying"),

		f("nopeerbloomfilters", "false",
			"disable bloom filtering support"),
		f("nocfilters", "false",
			"disable committed filtering (CF) support"),
		f("blocksonly", "false", "do not accept transactions from peers"),
		f("relaynonstd", "false", "relay nonstandard transactions"),
		f("rejectnonstd", "true", "reject nonstandard transactions"),

		f("maxorphantxs", fmt.Sprint(node.DefaultMaxOrphanTransactions),
			"max number of orphan transactions to store"),
		f("sigcachemaxsize", fmt.Sprint(node.DefaultSigCacheMaxSize),
			"maximum number of signatures to store in memory"),

		f("generate", "false", "set CPU miner to generate blocks"),
		f("genthreads", "-1", "set number of threads to generate using CPU, -1 = all"),
		f("algo", "random", "set algorithm to be used by cpu miner"),
		f("miningaddrs", "", "add address to pay block rewards to"),
		f("minerlistener", node.DefaultMinerListener,
			"address to listen for mining work subscriptions"),
		f("minerpass", "", "Preshared Key to prevent snooping/spoofing of miner traffic"),

		f("addcheckpoints", "", `add custom checkpoints "height:hash"`),
		f("disablecheckpoints", "", "disable all checkpoints"),

		f("blockminsize", fmt.Sprint(node.DefaultBlockMinSize),
			"min block size for miners"),
		f("blockmaxsize", fmt.Sprint(node.DefaultBlockMaxSize),
			"max block size for miners"),
		f("blockminweight", fmt.Sprint(node.DefaultBlockMinWeight),
			"min block weight for miners"),
		f("blockmaxweight", fmt.Sprint(node.DefaultBlockMaxWeight),
			"max block weight for miners"),
		f("blockprioritysize", "0", "size in bytes of high priority blocks"),

		f("uacomment", "", "comment to add to the P2P network user agent"),
		f("upnp", "false", "use UPNP to automatically port forward to node"),
		f("dbtype", "ffldb", "set database backend type"),
		f("disablednsseed", "false", "disable dns seeding"),

		f("profile", "false", "start HTTP profiling server on given address"),
		f("cpuprofile", "false", "start cpu profiling server on given address"),

		s("debuglevel", "d", "info", "sets log level for those unspecified below"),

		l("lib-addrmgr"), l("lib-blockchain"), l("lib-connmgr"), l("lib-database-ffldb"), l("lib-database"), l("lib-mining-cpuminer"), l("lib-mining"), l("lib-netsync"), l("lib-peer"), l("lib-rpcclient"), l("lib-txscript"), l("node"), l("node-mempool"), l("spv"), l("wallet"), l("wallet-chain"), l("wallet-legacyrpc"), l("wallet-rpcserver"), l("wallet-tx"), l("wallet-votingpool"), l("wallet-waddrmgr"), l("wallet-wallet"), l("wallet-wtxmgr"),
	},
	Examples: []climax.Example{
		{
			Usecase:     "--init --rpcuser=user --rpcpass=pa55word --save",
			Description: "resets the configuration file to default, sets rpc username and password and saves the changes to config after parsing",
		},
		{
			Usecase:     " -D test -d trace",
			Description: "run using the configuration in the 'test' directory with trace logging",
		},
	},
	Handle: func(ctx climax.Context) int {
		var dl string
		var ok bool
		if dl, ok = ctx.Get("debuglevel"); ok {
			log <- cl.Tracef{"setting debug level %s", dl}
			NodeConfig.Node.DebugLevel = dl
			Log.SetLevel(dl)
			ll := GetAllSubSystems()
			for i := range ll {
				ll[i].SetLevel(dl)
			}
		}
		if ctx.Is("version") {
			fmt.Println("pod/node version", n.Version())
			return 0
		}
		var datadir, cfgFile string
		if datadir, ok = ctx.Get("datadir"); !ok {
			datadir = util.AppDataDir("pod", false)
		}
		cfgFile = filepath.Join(filepath.Join(datadir, "node"), "conf.json")
		log <- cl.Debug{"DataDir", datadir, "cfgFile", cfgFile}
		if r, ok := getIfIs(&ctx, "configfile"); ok {
			cfgFile = r
		}
		if ctx.Is("init") {
			log <- cl.Debugf{"writing default configuration to %s", cfgFile}
			WriteDefaultNodeConfig(datadir)
		} else {
			log <- cl.Infof{"loading configuration from %s", cfgFile}
			if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
				log <- cl.Wrn("configuration file does not exist, creating new one")
				WriteDefaultNodeConfig(datadir)
			} else {
				log <- cl.Debug{"reading app configuration from", cfgFile}
				cfgData, err := ioutil.ReadFile(cfgFile)
				if err != nil {
					log <- cl.Error{"reading app config file:", err.Error()}
					WriteDefaultNodeConfig(datadir)
				} else {
					log <- cl.Trace{"parsing app configuration", string(cfgData)}
					err = json.Unmarshal(cfgData, &NodeConfig)
					if err != nil {
						log <- cl.Error{"parsing app config file:", err.Error()}
						WriteDefaultNodeConfig(datadir)
					}
				}
			}
			switch {
			case NodeConfig.Node.TestNet3:
				log <- cl.Info{"running on testnet"}
				NodeConfig.params = &node.TestNet3Params
			case NodeConfig.Node.SimNet:
				log <- cl.Info{"running on simnet"}
				NodeConfig.params = &node.SimNetParams
			default:
				log <- cl.Info{"running on mainnet"}
				NodeConfig.params = &node.MainNetParams
			}
		}
		configNode(NodeConfig.Node, &ctx, cfgFile)
		runNode(NodeConfig.Node, NodeConfig.params)
		return 0
	},
}

// WriteNodeConfig writes the current config to the requested location
func WriteNodeConfig(c *NodeCfg) {
	log <- cl.Dbg("writing config")
	j, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		log <- cl.Error{`marshalling default app config file: "`, err, `"`}
		log <- cl.Err(spew.Sdump(c))
		return
	}
	j = append(j, '\n')
	log <- cl.Tracef{
		"JSON formatted config file\n%s",
		j,
	}
	EnsureDir(c.Node.ConfigFile)
	err = ioutil.WriteFile(c.Node.ConfigFile, j, 0600)
	if err != nil {
		log <- cl.Error{"writing default app config file:", err.Error()}
		return
	}
}

// WriteDefaultNodeConfig creates a default config and writes it to the requested location
func WriteDefaultNodeConfig(datadir string) {
	log <- cl.Dbg("writing default config")
	defCfg := DefaultNodeConfig(datadir)
	j, err := json.MarshalIndent(defCfg, "", "  ")
	if err != nil {
		log <- cl.Error{`marshalling default app config file: "`, err, `"`}
		log <- cl.Err(spew.Sdump(defCfg))
		return
	}
	j = append(j, '\n')
	log <- cl.Tracef{
		"JSON formatted config file\n%s",
		j,
	}
	EnsureDir(defCfg.Node.ConfigFile)
	err = ioutil.WriteFile(defCfg.Node.ConfigFile, j, 0600)
	if err != nil {
		log <- cl.Error{"writing default app config file:", err.Error()}
		return
	}
	// if we are writing default config we also want to use it
	NodeConfig = defCfg
}

// DefaultNodeConfig is the default configuration for node
func DefaultNodeConfig(datadir string) *NodeCfg {
	user := GenKey()
	pass := GenKey()
	params := node.MainNetParams
	switch n.ActiveNetParams.Name {
	case "testnet3":
		params = node.TestNet3Params
	case "simnet":
		params = node.SimNetParams
	}
	appdir := filepath.Join(datadir, "node")
	return &NodeCfg{
		Node: &n.Config{
			RPCUser:              user,
			RPCPass:              pass,
			Listeners:            []string{n.DefaultListener},
			RPCListeners:         []string{n.DefaultRPCListener},
			DebugLevel:           "info",
			ConfigFile:           filepath.Join(appdir, n.DefaultConfigFilename),
			MaxPeers:             n.DefaultMaxPeers,
			BanDuration:          n.DefaultBanDuration,
			BanThreshold:         n.DefaultBanThreshold,
			RPCMaxClients:        n.DefaultMaxRPCClients,
			RPCMaxWebsockets:     n.DefaultMaxRPCWebsockets,
			RPCMaxConcurrentReqs: n.DefaultMaxRPCConcurrentReqs,
			DataDir:              appdir,
			LogDir:               appdir,
			DbType:               n.DefaultDbType,
			RPCKey:               filepath.Join(datadir, "rpc.key"),
			RPCCert:              filepath.Join(datadir, "rpc.cert"),
			MinRelayTxFee:        mempool.DefaultMinRelayTxFee.ToDUO(),
			FreeTxRelayLimit:     n.DefaultFreeTxRelayLimit,
			TrickleInterval:      n.DefaultTrickleInterval,
			BlockMinSize:         n.DefaultBlockMinSize,
			BlockMaxSize:         n.DefaultBlockMaxSize,
			BlockMinWeight:       n.DefaultBlockMinWeight,
			BlockMaxWeight:       n.DefaultBlockMaxWeight,
			BlockPrioritySize:    mempool.DefaultBlockPrioritySize,
			MaxOrphanTxs:         n.DefaultMaxOrphanTransactions,
			SigCacheMaxSize:      n.DefaultSigCacheMaxSize,
			Generate:             n.DefaultGenerate,
			GenThreads:           1,
			TxIndex:              n.DefaultTxIndex,
			AddrIndex:            n.DefaultAddrIndex,
			Algo:                 n.DefaultAlgo,
		},
		LogLevels: GetDefaultLogLevelsConfig(),
		params:    &params,
	}
}
package app

import (
	"fmt"
	"path/filepath"

	w "git.parallelcoin.io/pod/cmd/wallet"
	walletmain "git.parallelcoin.io/pod/cmd/wallet"
	"git.parallelcoin.io/pod/pkg/netparams"
	"git.parallelcoin.io/pod/pkg/wallet"
	"github.com/tucnak/climax"
)

// SetupCfg is the type for the default config data
type SetupCfg struct {
	DataDir string
	Network string
	Config  *walletmain.Config
}

// SetupConfig is
var SetupConfig = SetupCfg{
	DataDir: walletmain.DefaultAppDataDir,
	Network: "mainnet",
}

// SetupCommand is a command to send RPC queries to bitcoin RPC protocol server for node and wallet queries
var SetupCommand = climax.Command{
	Name:  "setup",
	Brief: "initialises configuration and creates a new wallet",
	Help:  "initialises configuration and creates a new wallet in specified data directory for a specified network",
	Flags: []climax.Flag{
		t("help", "h", "show help text"),
		s("datadir", "D", walletmain.DefaultAppDataDir, "specify where the wallet will be created"),
		f("network", "mainnet", "connect to (mainnet|testnet|simnet)"),
	},
	Handle: func(ctx climax.Context) int {
		fmt.Println("pod wallet setup")
		if ctx.Is("help") {
			fmt.Print(`Usage: create [-h] [-D] [--network]

creates a new wallet given CLI flags, or interactively

Available options:

	-h, --help
		show help text
	-D, --datadir="~/.pod/wallet"
		specify where the wallet will be created
	--network="mainnet"
		connect to (mainnet|testnet|simnet)

`)
			return 0
		}
		SetupConfig.DataDir = w.DefaultDataDir
		if r, ok := getIfIs(&ctx, "datadir"); ok {
			SetupConfig.DataDir = r
		}
		activeNet := walletmain.ActiveNet
		wc := DefaultWalletConfig(SetupConfig.DataDir)
		SetupConfig.Config = wc.Wallet
		SetupConfig.Config.TestNet3 = false
		SetupConfig.Config.SimNet = false
		if r, ok := getIfIs(&ctx, "network"); ok {
			switch r {
			case "testnet":
				activeNet = &netparams.TestNet3Params
				SetupConfig.Config.TestNet3 = true
				SetupConfig.Config.SimNet = false
			case "simnet":
				activeNet = &netparams.SimNetParams
				SetupConfig.Config.TestNet3 = false
				SetupConfig.Config.SimNet = true
			default:
				activeNet = &netparams.MainNetParams
			}
			SetupConfig.Network = r
		}
		dbDir := walletmain.NetworkDir(
			filepath.Join(SetupConfig.DataDir, "wallet"), activeNet.Params)
		loader := wallet.NewLoader(
			walletmain.ActiveNet.Params, dbDir, 250)
		exists, err := loader.WalletExists()
		if err != nil {
			fmt.Println("ERROR", err)
			return 1
		}
		if exists {
			fmt.Print("\n!!! A wallet already exists at '" + dbDir + "/wallet.db' !!! \n")
			fmt.Println(`if you are sure it isn't valuable you can delete it before running this again:

	rm ` + dbDir + `/wallet.db
`)
			return 1
		}
		SetupConfig.Config.AppDataDir = filepath.Join(
			SetupConfig.DataDir, "wallet")
		WriteDefaultConfConfig(SetupConfig.DataDir)
		WriteDefaultCtlConfig(SetupConfig.DataDir)
		WriteDefaultNodeConfig(SetupConfig.DataDir)
		WriteDefaultWalletConfig(SetupConfig.DataDir)
		WriteDefaultShellConfig(SetupConfig.DataDir)
		walletmain.CreateWallet(SetupConfig.Config, activeNet)
		fmt.Print("\nYou can now open the wallet\n")
		return 0
	},
}
package app

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	n "git.parallelcoin.io/pod/cmd/node"
	"git.parallelcoin.io/pod/cmd/node/mempool"
	"git.parallelcoin.io/pod/cmd/shell"
	w "git.parallelcoin.io/pod/cmd/wallet"
	walletmain "git.parallelcoin.io/pod/cmd/wallet"
	cl "git.parallelcoin.io/pod/pkg/clog"
	"git.parallelcoin.io/pod/pkg/util"
	"github.com/tucnak/climax"
)

// DefaultShellAppDataDir is the default app data dir
var DefaultShellAppDataDir = filepath.Join(w.DefaultDataDir, "shell")

// DefaultShellConfigFile is the default configfile for shell
var DefaultShellConfigFile = filepath.Join(DefaultShellAppDataDir, "conf.json")

// ShellConfig is the combined app and log levels configuration
var ShellConfig = DefaultShellConfig(w.DefaultDataDir)

// ShellCommand is a command to send RPC queries to bitcoin RPC protocol server for node and wallet queries
var ShellCommand = climax.Command{
	Name:  "shell",
	Brief: "parallelcoin shell",
	Help:  "check balances, make payments, manage contacts, search the chain, it slices, it dices",
	Flags: []climax.Flag{
		t("version", "V", "show version number and quit"),

		s("configfile", "C", DefaultShellConfFileName, "path to configuration file"),

		s("datadir", "D", n.DefaultDataDir, "set the pod base directory"),
		f("appdatadir", "shell", "set app data directory for wallet, configuration and logs"),

		t("init", "i", "resets configuration to defaults"),
		t("save", "S", "saves current flags into configuration"),

		f("network", "mainnet",
			"connect to (mainnet|testnet|regtestnet|simnet)"),

		f("createtemp", "false",
			"create temporary wallet (pass=walletpass) requires --datadir"),

		f("walletpass", "",
			"the public wallet password - only required if the wallet was created with one"),

		s("listeners", "S", n.DefaultListener,
			"sets an address to listen for P2P connections"),
		f("externalips", "", "additional P2P listeners"),
		f("disablelisten", "false", "disables the P2P listener"),

		f("rpclisteners", "127.0.0.1:11046",
			"add a listener for the wallet RPC"),
		f("rpcmaxclients", fmt.Sprint(n.DefaultMaxRPCClients),
			"max connections for wallet RPC"),
		f("rpcmaxwebsockets", fmt.Sprint(n.DefaultMaxRPCWebsockets),
			"max websockets for wallet RPC"),

		f("username", "user", "username for wallet RPC"),
		f("password", "pa55word", "password for wallet RPC"),

		f("rpccert", n.DefaultRPCCertFile,
			"file containing the RPC tls certificate"),
		f("rpckey", n.DefaultRPCKeyFile,
			"file containing RPC TLS key"),
		f("onetimetlskey", "false",
			"generate a new TLS certpair don't save key"),
		f("cafile", w.DefaultCAFile,
			"certificate authority for custom TLS CA"),
		f("tls", "false", "enable TLS on wallet RPC server"),

		f("txindex", "true", "enable transaction index"),
		f("addrindex", "true", "enable address index"),
		t("dropcfindex", "", "delete committed filtering (CF) index then exit"),
		t("droptxindex", "", "deletes transaction index then exit"),
		t("dropaddrindex", "", "deletes the address index then exits"),

		f("proxy", "", "proxy address for outbound connections"),
		f("proxyuser", "", "username for proxy server"),
		f("proxypass", "", "password for proxy server"),

		f("onion", "", "connect via tor proxy relay"),
		f("onionuser", "", "username for onion proxy server"),
		f("onionpass", "", "password for onion proxy server"),
		f("noonion", "false", "disable onion proxy"),
		f("torisolation", "false", "use a different user/pass for each peer"),

		f("addpeers", "",
			"adds a peer to the peers database to try to connect to"),
		f("connectpeers", "",
			"adds a peer to a connect-only whitelist"),
		f(`maxpeers`, fmt.Sprint(n.DefaultMaxPeers),
			"sets max number of peers to connect to to at once"),
		f(`disablebanning`, "false",
			"disable banning of misbehaving peers"),
		f("banduration", "1d",
			"time to ban misbehaving peers (d/h/m/s)"),
		f("banthreshold", fmt.Sprint(n.DefaultBanThreshold),
			"banscore that triggers a ban"),
		f("whitelists", "", "addresses and networks immune to banning"),

		f("trickleinterval", fmt.Sprint(n.DefaultTrickleInterval),
			"time between sending inventory batches to peers"),
		f("minrelaytxfee", "0",
			"min fee in DUO/kb to relay transaction"),
		f("freetxrelaylimit", fmt.Sprint(n.DefaultFreeTxRelayLimit),
			"limit below min fee transactions in kb/bin"),
		f("norelaypriority", "false",
			"do not discriminate transactions for relaying"),

		f("nopeerbloomfilters", "false",
			"disable bloom filtering support"),
		f("nocfilters", "false",
			"disable committed filtering (CF) support"),
		f("blocksonly", "false", "do not accept transactions from peers"),
		f("relaynonstd", "false", "relay nonstandard transactions"),
		f("rejectnonstd", "false", "reject nonstandard transactions"),

		f("maxorphantxs", fmt.Sprint(n.DefaultMaxOrphanTransactions),
			"max number of orphan transactions to store"),
		f("sigcachemaxsize", fmt.Sprint(n.DefaultSigCacheMaxSize),
			"maximum number of signatures to store in memory"),

		f("generate", fmt.Sprint(n.DefaultGenerate),
			"set CPU miner to generate blocks"),
		f("genthreads", fmt.Sprint(n.DefaultGenThreads),
			"set number of threads to generate using CPU, -1 = all"),
		f("algo", n.DefaultAlgo, "set algorithm to be used by cpu miner"),
		f("miningaddrs", "", "add address to pay block rewards to"),
		f("minerlistener", n.DefaultMinerListener,
			"address to listen for mining work subscriptions"),
		f("minerpass", "",
			"PSK to prevent snooping/spoofing of miner traffic"),

		f("addcheckpoints", "false", `add custom checkpoints "height:hash"`),
		f("disablecheckpoints", "false", "disable all checkpoints"),

		f("blockminsize", fmt.Sprint(n.DefaultBlockMinSize),
			"min block size for miners"),
		f("blockmaxsize", fmt.Sprint(n.DefaultBlockMaxSize),
			"max block size for miners"),
		f("blockminweight", fmt.Sprint(n.DefaultBlockMinWeight),
			"min block weight for miners"),
		f("blockmaxweight", fmt.Sprint(n.DefaultBlockMaxWeight),
			"max block weight for miners"),
		f("blockprioritysize", fmt.Sprint(),
			"size in bytes of high priority blocks"),

		f("uacomment", "", "comment to add to the P2P network user agent"),
		f("upnp", "false", "use UPNP to automatically port forward to node"),
		f("dbtype", "ffldb", "set database backend type"),
		f("disablednsseed", "false", "disable dns seeding"),

		f("profile", "false", "start HTTP profiling server on given address"),
		f("cpuprofile", "false", "start cpu profiling server on given address"),

		s("debuglevel", "d", "info", "sets debuglevel, specify per-library below"),

		l("lib-addrmgr"), l("lib-blockchain"), l("lib-connmgr"), l("lib-database-ffldb"), l("lib-database"), l("lib-mining-cpuminer"), l("lib-mining"), l("lib-netsync"), l("lib-peer"), l("lib-rpcclient"), l("lib-txscript"), l("node"), l("node-mempool"), l("spv"), l("wallet"), l("wallet-chain"), l("wallet-legacyrpc"), l("wallet-rpcserver"), l("wallet-tx"), l("wallet-votingpool"), l("wallet-waddrmgr"), l("wallet-wallet"), l("wallet-wtxmgr"),
	},
	Examples: []climax.Example{
		{
			Usecase:     "--init --rpcuser=user --rpcpass=pa55word --save",
			Description: "resets the configuration file to default, sets rpc username and password and saves the changes to config after parsing",
		},
	},
	Handle: shellHandle,
}

func shellHandle(ctx climax.Context) int {
	var dl string
	var ok bool
	if dl, ok = ctx.Get("debuglevel"); ok {
		log <- cl.Tracef{"setting debug level %s", dl}
		ShellConfig.Node.DebugLevel = dl
		Log.SetLevel(dl)
		ll := GetAllSubSystems()
		for i := range ll {
			ll[i].SetLevel(dl)
		}
	}
	if ctx.Is("version") {
		fmt.Println("pod/shell version", Version(),
			"pod/node version", n.Version(),
			"pod/wallet version", w.Version())
		return 0
	}
	var datadir, dd, cfgFile string
	datadir = util.AppDataDir("pod", false)
	if dd, ok = ctx.Get("datadir"); ok {
		ShellConfig.Node.DataDir = dd
		ShellConfig.Wallet.DataDir = dd
		datadir = dd
	}
	cfgFile = filepath.Join(datadir, "shell/conf.json")
	log <- cl.Debug{"DataDir", datadir, "cfgFile", cfgFile}
	if r, ok := ctx.Get("configfile"); ok {
		ShellConfig.ConfigFile = r
		cfgFile = r
	}
	if ctx.Is("init") {
		log <- cl.Debug{"writing default configuration to", cfgFile}
		WriteDefaultShellConfig(datadir)
	} else {
		log <- cl.Info{"loading configuration from", cfgFile}
		if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
			log <- cl.Wrn("configuration file does not exist, creating new one")
			WriteDefaultShellConfig(datadir)
		} else {
			log <- cl.Debug{"reading app configuration from", cfgFile}
			cfgData, err := ioutil.ReadFile(cfgFile)
			if err != nil {
				log <- cl.Error{"reading app config file", err.Error()}
				WriteDefaultShellConfig(datadir)
			} else {
				log <- cl.Tracef{"parsing app configuration\n%s", cfgData}
				err = json.Unmarshal(cfgData, &ShellConfig)
				if err != nil {
					log <- cl.Error{"parsing app config file", err.Error()}
					WriteDefaultShellConfig(datadir)
				}
			}
		}
	}
	j, _ := json.MarshalIndent(ShellConfig, "", "  ")
	log <- cl.Tracef{"parsed configuration:\n%s", string(j)}
	configShell(&ctx, cfgFile)
	j, _ = json.MarshalIndent(ShellConfig, "", "  ")
	log <- cl.Tracef{"after configuration:\n%s", string(j)}
	return runShell()
}

// WriteShellConfig creates and writes the config file in the requested location
func WriteShellConfig(c *shell.Config) {
	log <- cl.Dbg("writing config")
	j, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		panic(err.Error())
	}
	j = append(j, '\n')
	EnsureDir(c.ConfigFile)
	err = ioutil.WriteFile(c.ConfigFile, j, 0600)
	if err != nil {
		panic(err.Error())
	}
}

// WriteDefaultShellConfig creates and writes a default config to the requested location
func WriteDefaultShellConfig(datadir string) {
	defCfg := DefaultShellConfig(datadir)
	j, err := json.MarshalIndent(defCfg, "", "  ")
	if err != nil {
		log <- cl.Error{"marshalling configuration", err}
		panic(err)
	}
	j = append(j, '\n')
	log <- cl.Trace{"JSON formatted config file\n", string(j)}
	EnsureDir(defCfg.ConfigFile)
	err = ioutil.WriteFile(defCfg.ConfigFile, j, 0600)
	if err != nil {
		log <- cl.Error{"writing app config file", defCfg.ConfigFile, err}
		panic(err)
	}
	// if we are writing default config we also want to use it
	ShellConfig = defCfg
}

// DefaultShellConfig returns a default configuration
func DefaultShellConfig(datadir string) *shell.Config {
	log <- cl.Dbg("getting default config")
	u := GenKey()
	p := GenKey()
	appdatadir := filepath.Join(datadir, "shell")
	walletdatadir := filepath.Join(datadir, "wallet")
	nodedatadir := filepath.Join(datadir, "node")
	return &shell.Config{
		ConfigFile: filepath.Join(appdatadir, "conf.json"),
		DataDir:    datadir,
		AppDataDir: appdatadir,
		Node: &n.Config{
			RPCUser:      u,
			RPCPass:      p,
			Listeners:    []string{n.DefaultListener},
			RPCListeners: []string{n.DefaultRPCListener},
			DebugLevel:   "info",
			ConfigFile: filepath.Join(
				appdatadir, "nodeconf.json"),
			MaxPeers:             n.DefaultMaxPeers,
			BanDuration:          n.DefaultBanDuration,
			BanThreshold:         n.DefaultBanThreshold,
			RPCMaxClients:        n.DefaultMaxRPCClients,
			RPCMaxWebsockets:     n.DefaultMaxRPCWebsockets,
			RPCMaxConcurrentReqs: n.DefaultMaxRPCConcurrentReqs,
			DataDir:              nodedatadir,
			LogDir:               appdatadir,
			DbType:               n.DefaultDbType,
			RPCCert:              filepath.Join(datadir, "rpc.cert"),
			RPCKey:               filepath.Join(datadir, "rpc.key"),
			MinRelayTxFee:        mempool.DefaultMinRelayTxFee.ToDUO(),
			FreeTxRelayLimit:     n.DefaultFreeTxRelayLimit,
			TrickleInterval:      n.DefaultTrickleInterval,
			BlockMinSize:         n.DefaultBlockMinSize,
			BlockMaxSize:         n.DefaultBlockMaxSize,
			BlockMinWeight:       n.DefaultBlockMinWeight,
			BlockMaxWeight:       n.DefaultBlockMaxWeight,
			BlockPrioritySize:    mempool.DefaultBlockPrioritySize,
			MaxOrphanTxs:         n.DefaultMaxOrphanTransactions,
			SigCacheMaxSize:      n.DefaultSigCacheMaxSize,
			Generate:             n.DefaultGenerate,
			GenThreads:           -1,
			TxIndex:              n.DefaultTxIndex,
			AddrIndex:            n.DefaultAddrIndex,
			Algo:                 n.DefaultAlgo,
		},
		Wallet: &w.Config{
			PodUsername:        u,
			PodPassword:        p,
			Username:           u,
			Password:           p,
			RPCConnect:         n.DefaultRPCListener,
			LegacyRPCListeners: []string{w.DefaultListener},
			NoInitialLoad:      false,
			ConfigFile: filepath.Join(
				appdatadir, "walletconf.json"),
			DataDir:    walletdatadir,
			AppDataDir: walletdatadir,
			LogDir:     appdatadir,
			RPCCert:    filepath.Join(datadir, "rpc.cert"),
			RPCKey:     filepath.Join(datadir, "rpc.key"),
			WalletPass: "",
			CAFile: filepath.Join(
				datadir, walletmain.DefaultCAFile),
			LegacyRPCMaxClients:    w.DefaultRPCMaxClients,
			LegacyRPCMaxWebsockets: w.DefaultRPCMaxWebsockets,
		},
		Levels: GetDefaultLogLevelsConfig(),
	}
}
package app

import (
	"fmt"

	"github.com/tucnak/climax"
)

// VersionCommand is a command to send RPC queries to bitcoin RPC protocol server for node and wallet queries
var VersionCommand = climax.Command{
	Name:  "version",
	Brief: "prints the version of pod",
	Help:  "",
	Handle: func(ctx climax.Context) int {
		fmt.Println(Version())
		return 0
	},
}
package app

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	n "git.parallelcoin.io/pod/cmd/node"
	w "git.parallelcoin.io/pod/cmd/wallet"
	walletmain "git.parallelcoin.io/pod/cmd/wallet"
	cl "git.parallelcoin.io/pod/pkg/clog"
	"git.parallelcoin.io/pod/pkg/fork"
	"git.parallelcoin.io/pod/pkg/netparams"
	"git.parallelcoin.io/pod/pkg/util"
	"github.com/tucnak/climax"
)

// WalletCfg is the combined app and logging configuration data
type WalletCfg struct {
	Wallet    *w.Config
	Levels    map[string]string
	activeNet *netparams.Params
}

// WalletCommand is a command to send RPC queries to bitcoin RPC protocol server for node and wallet queries
var WalletCommand = climax.Command{
	Name:  "wallet",
	Brief: "parallelcoin wallet",
	Help:  "check balances, make payments, manage contacts",
	Flags: []climax.Flag{
		t("version", "V", "show version number and quit"),

		s("configfile", "C", w.DefaultConfigFilename,
			"path to configuration file"),
		s("datadir", "D", w.DefaultDataDir,
			"set the pod base directory"),
		f("appdatadir", w.DefaultAppDataDir, "set app data directory for wallet, configuration and logs"),

		t("init", "i", "resets configuration to defaults"),
		t("save", "S", "saves current flags into configuration"),

		t("createtemp", "", "create temporary wallet (pass=walletpass) requires --datadir"),

		t("gui", "G", "launch GUI"),
		f("rpcconnect", n.DefaultRPCListener, "connect to the RPC of a parallelcoin node for chain queries"),

		f("podusername", "user", "username for node RPC authentication"),
		f("podpassword", "pa55word", "password for node RPC authentication"),

		f("walletpass", "", "the public wallet password - only required if the wallet was created with one"),

		f("noinitialload", "false", "defer wallet load to be triggered by RPC"),
		f("network", "mainnet", "connect to (mainnet|testnet|regtestnet|simnet)"),

		f("profile", "false", "enable HTTP profiling on given port (1024-65536)"),

		f("rpccert", w.DefaultRPCCertFile,
			"file containing the RPC tls certificate"),
		f("rpckey", w.DefaultRPCKeyFile,
			"file containing RPC TLS key"),
		f("onetimetlskey", "false", "generate a new TLS certpair don't save key"),
		f("cafile", w.DefaultCAFile, "certificate authority for custom TLS CA"),
		f("enableclienttls", "false", "enable TLS for the RPC client"),
		f("enableservertls", "false", "enable TLS on wallet RPC server"),

		f("proxy", "", "proxy address for outbound connections"),
		f("proxyuser", "", "username for proxy server"),
		f("proxypass", "", "password for proxy server"),

		f("legacyrpclisteners", w.DefaultListener, "add a listener for the legacy RPC"),
		f("legacyrpcmaxclients", fmt.Sprint(w.DefaultRPCMaxClients),
			"max connections for legacy RPC"),
		f("legacyrpcmaxwebsockets", fmt.Sprint(w.DefaultRPCMaxWebsockets),
			"max websockets for legacy RPC"),

		f("username", "user",
			"username for wallet RPC when podusername is empty"),
		f("password", "pa55word",
			"password for wallet RPC when podpassword is omitted"),
		f("experimentalrpclisteners", "",
			"listener for experimental rpc"),

		s("debuglevel", "d", "info", "sets debuglevel, specify per-library below"),

		l("lib-addrmgr"), l("lib-blockchain"), l("lib-connmgr"), l("lib-database-ffldb"), l("lib-database"), l("lib-mining-cpuminer"), l("lib-mining"), l("lib-netsync"), l("lib-peer"), l("lib-rpcclient"), l("lib-txscript"), l("node"), l("node-mempool"), l("spv"), l("wallet"), l("wallet-chain"), l("wallet-legacyrpc"), l("wallet-rpcserver"), l("wallet-tx"), l("wallet-votingpool"), l("wallet-waddrmgr"), l("wallet-wallet"), l("wallet-wtxmgr"),
	},
	// Examples: []climax.Example{
	// 	{
	// 		Usecase:     "--init --rpcuser=user --rpcpass=pa55word --save",
	// 		Description: "resets the configuration file to default, sets rpc username and password and saves the changes to config after parsing",
	// 	},
	// },
}

// WalletConfig is the combined app and log levels configuration
var WalletConfig = DefaultWalletConfig(w.DefaultConfigFile)

// wf is the list of flags and the default values stored in the Usage field
var wf = GetFlags(WalletCommand)

func init() {
	// Loads after the var clauses run
	WalletCommand.Handle = func(ctx climax.Context) int {

		Log.SetLevel("off")
		var dl string
		var ok bool
		if dl, ok = ctx.Get("debuglevel"); ok {
			Log.SetLevel(dl)
			ll := GetAllSubSystems()
			for i := range ll {
				ll[i].SetLevel(dl)
			}
		}
		log <- cl.Tracef{"setting debug level %s", dl}
		log <- cl.Trc("starting wallet app")
		log <- cl.Debugf{"pod/wallet version %s", w.Version()}
		if ctx.Is("version") {
			fmt.Println("pod/wallet version", w.Version())
			return 0
		}
		var datadir, cfgFile string
		datadir = util.AppDataDir("pod", false)
		if datadir, ok = ctx.Get("datadir"); !ok {
			datadir = w.DefaultDataDir
		}
		cfgFile = filepath.Join(filepath.Join(datadir, "node"), "conf.json")
		log <- cl.Debug{"DataDir", datadir, "cfgFile", cfgFile}
		if cfgFile, ok = ctx.Get("configfile"); !ok {
			cfgFile = filepath.Join(
				filepath.Join(datadir, "wallet"), w.DefaultConfigFilename)
		}

		if ctx.Is("init") {
			log <- cl.Debug{"writing default configuration to", cfgFile}
			WriteDefaultWalletConfig(cfgFile)
		}
		log <- cl.Info{"loading configuration from", cfgFile}
		if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
			log <- cl.Wrn("configuration file does not exist, creating new one")
			WriteDefaultWalletConfig(cfgFile)
		} else {
			log <- cl.Debug{"reading app configuration from", cfgFile}
			cfgData, err := ioutil.ReadFile(cfgFile)
			if err != nil {
				log <- cl.Error{"reading app config file", err.Error()}
				WriteDefaultWalletConfig(cfgFile)
			}
			log <- cl.Tracef{"parsing app configuration\n%s", cfgData}
			err = json.Unmarshal(cfgData, &WalletConfig)
			if err != nil {
				log <- cl.Error{"parsing app config file", err.Error()}
				WriteDefaultWalletConfig(cfgFile)
			}
			WalletConfig.activeNet = &netparams.MainNetParams
			if WalletConfig.Wallet.TestNet3 {
				WalletConfig.activeNet = &netparams.TestNet3Params
			}
			if WalletConfig.Wallet.SimNet {
				WalletConfig.activeNet = &netparams.SimNetParams
			}
		}

		configWallet(WalletConfig.Wallet, &ctx, cfgFile)
		if dl, ok = ctx.Get("debuglevel"); ok {
			for i := range WalletConfig.Levels {
				WalletConfig.Levels[i] = dl
			}
		}
		fmt.Println("running wallet on", WalletConfig.activeNet.Name)
		runWallet(WalletConfig.Wallet, WalletConfig.activeNet)
		return 0
	}
}

func configWallet(wc *w.Config, ctx *climax.Context, cfgFile string) {
	log <- cl.Trace{"configuring from command line flags ", os.Args}
	if ctx.Is("createtemp") {
		log <- cl.Dbg("request to make temp wallet")
		wc.CreateTemp = true
	}
	if r, ok := getIfIs(ctx, "appdatadir"); ok {
		log <- cl.Debug{"appdatadir set to", r}
		wc.AppDataDir = n.CleanAndExpandPath(r)
	}
	if r, ok := getIfIs(ctx, "logdir"); ok {
		wc.LogDir = n.CleanAndExpandPath(r)
	}
	if r, ok := getIfIs(ctx, "profile"); ok {
		NormalizeAddress(r, "3131", &wc.Profile)
	}
	if r, ok := getIfIs(ctx, "walletpass"); ok {
		wc.WalletPass = r
	}
	if r, ok := getIfIs(ctx, "rpcconnect"); ok {
		NormalizeAddress(r, "11048", &wc.RPCConnect)
	}
	if r, ok := getIfIs(ctx, "cafile"); ok {
		wc.CAFile = n.CleanAndExpandPath(r)
	}
	if r, ok := getIfIs(ctx, "enableclienttls"); ok {
		wc.EnableClientTLS = r == "true"
	}
	if r, ok := getIfIs(ctx, "podusername"); ok {
		wc.PodUsername = r
	}
	if r, ok := getIfIs(ctx, "podpassword"); ok {
		wc.PodPassword = r
	}
	if r, ok := getIfIs(ctx, "proxy"); ok {
		NormalizeAddress(r, "11048", &wc.Proxy)
	}
	if r, ok := getIfIs(ctx, "proxyuser"); ok {
		wc.ProxyUser = r
	}
	if r, ok := getIfIs(ctx, "proxypass"); ok {
		wc.ProxyPass = r
	}
	if r, ok := getIfIs(ctx, "rpccert"); ok {
		wc.RPCCert = n.CleanAndExpandPath(r)
	}
	if r, ok := getIfIs(ctx, "rpckey"); ok {
		wc.RPCKey = n.CleanAndExpandPath(r)
	}
	if r, ok := getIfIs(ctx, "onetimetlskey"); ok {
		wc.OneTimeTLSKey = r == "true"
	}
	if r, ok := getIfIs(ctx, "enableservertls"); ok {
		wc.EnableServerTLS = r == "true"
	}
	if r, ok := getIfIs(ctx, "legacyrpclisteners"); ok {
		NormalizeAddresses(r, "11046", &wc.LegacyRPCListeners)
	}
	if r, ok := getIfIs(ctx, "legacyrpcmaxclients"); ok {
		var bt int
		if err := ParseInteger(r, "legacyrpcmaxclients", &bt); err != nil {
			log <- cl.Wrn(err.Error())
		} else {
			wc.LegacyRPCMaxClients = int64(bt)
		}
	}
	if r, ok := getIfIs(ctx, "legacyrpcmaxwebsockets"); ok {
		_, err := fmt.Sscanf(r, "%d", wc.LegacyRPCMaxWebsockets)
		if err != nil {
			log <- cl.Errorf{
				"malformed legacyrpcmaxwebsockets: `%s` leaving set at `%d`",
				r, wc.LegacyRPCMaxWebsockets,
			}
		}
	}
	if r, ok := getIfIs(ctx, "username"); ok {
		wc.Username = r
	}
	if r, ok := getIfIs(ctx, "password"); ok {
		wc.Password = r
	}
	if r, ok := getIfIs(ctx, "experimentalrpclisteners"); ok {
		NormalizeAddresses(r, "11045", &wc.ExperimentalRPCListeners)
	}
	if r, ok := getIfIs(ctx, "datadir"); ok {
		wc.DataDir = r
	}
	if r, ok := getIfIs(ctx, "network"); ok {
		switch r {
		case "testnet":
			fork.IsTestnet = true
			wc.TestNet3, wc.SimNet = true, false
			WalletConfig.activeNet = &netparams.TestNet3Params
		case "simnet":
			wc.TestNet3, wc.SimNet = false, true
			WalletConfig.activeNet = &netparams.SimNetParams
		default:
			wc.TestNet3, wc.SimNet = false, false
			WalletConfig.activeNet = &netparams.MainNetParams
		}
	}

	// finished configuration
	SetLogging(ctx)

	if ctx.Is("save") {
		log <- cl.Info{"saving config file to", cfgFile}
		j, err := json.MarshalIndent(WalletConfig, "", "  ")
		if err != nil {
			log <- cl.Error{"writing app config file", err}
		}
		j = append(j, '\n')
		log <- cl.Trace{"JSON formatted config file\n", string(j)}
		ioutil.WriteFile(cfgFile, j, 0600)
	}
}

// WriteWalletConfig creates and writes the config file in the requested location
func WriteWalletConfig(c *WalletCfg) {
	log <- cl.Dbg("writing config")
	j, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		panic(err.Error())
	}
	j = append(j, '\n')
	EnsureDir(c.Wallet.ConfigFile)
	err = ioutil.WriteFile(c.Wallet.ConfigFile, j, 0600)
	if err != nil {
		panic(err.Error())
	}
}

// WriteDefaultWalletConfig creates and writes a default config to the requested location
func WriteDefaultWalletConfig(datadir string) {
	defCfg := DefaultWalletConfig(datadir)
	j, err := json.MarshalIndent(defCfg, "", "  ")
	if err != nil {
		log <- cl.Error{"marshalling configuration", err}
		panic(err)
	}
	j = append(j, '\n')
	EnsureDir(defCfg.Wallet.ConfigFile)
	log <- cl.Trace{"JSON formatted config file\n", string(j)}
	EnsureDir(defCfg.Wallet.ConfigFile)
	err = ioutil.WriteFile(defCfg.Wallet.ConfigFile, j, 0600)
	if err != nil {
		log <- cl.Error{"writing app config file", err}
		panic(err)
	}
	// if we are writing default config we also want to use it
	WalletConfig = defCfg
}

// DefaultWalletConfig returns a default configuration
func DefaultWalletConfig(datadir string) *WalletCfg {
	log <- cl.Dbg("getting default config")
	appdatadir := filepath.Join(datadir, w.DefaultAppDataDirname)
	return &WalletCfg{
		Wallet: &w.Config{
			ConfigFile: filepath.Join(
				appdatadir, w.DefaultConfigFilename),
			DataDir:         datadir,
			AppDataDir:      appdatadir,
			RPCConnect:      n.DefaultRPCListener,
			PodUsername:     "user",
			PodPassword:     "pa55word",
			WalletPass:      "",
			NoInitialLoad:   false,
			RPCCert:         filepath.Join(datadir, "rpc.cert"),
			RPCKey:          filepath.Join(datadir, "rpc.key"),
			CAFile:          walletmain.DefaultCAFile,
			EnableClientTLS: false,
			EnableServerTLS: false,
			Proxy:           "",
			ProxyUser:       "",
			ProxyPass:       "",
			LegacyRPCListeners: []string{
				w.DefaultListener,
			},
			LegacyRPCMaxClients:      w.DefaultRPCMaxClients,
			LegacyRPCMaxWebsockets:   w.DefaultRPCMaxWebsockets,
			Username:                 "user",
			Password:                 "pa55word",
			ExperimentalRPCListeners: []string{},
		},
		Levels:    GetDefaultLogLevelsConfig(),
		activeNet: &netparams.MainNetParams,
	}
}
package app

import (
	"fmt"

	"git.parallelcoin.io/pod/cmd/ctl"
	"git.parallelcoin.io/pod/cmd/shell"
	"github.com/davecgh/go-spew/spew"
)

func getConfs(datadir string) {
	confs = []string{
		datadir + "/ctl/conf.json",
		datadir + "/node/conf.json",
		datadir + "/wallet/conf.json",
		datadir + "/shell/conf.json",
	}
}

// ConfigSet is a full set of configuration structs
type ConfigSet struct {
	Conf   *ConfCfg
	Ctl    *ctl.Config
	Node   *NodeCfg
	Wallet *WalletCfg
	Shell  *shell.Config
}

// WriteConfigSet writes a set of configurations to disk
func WriteConfigSet(in *ConfigSet) {
	WriteConfConfig(in.Conf)
	WriteCtlConfig(in.Ctl)
	WriteNodeConfig(in.Node)
	WriteWalletConfig(in.Wallet)
	WriteShellConfig(in.Shell)
	return
}

// GetDefaultConfs returns all of the configurations in their default state
func GetDefaultConfs(datadir string) (out *ConfigSet) {
	out = new(ConfigSet)
	out.Conf = DefaultConfConfig(datadir)
	out.Ctl = DefaultCtlConfig(datadir)
	out.Node = DefaultNodeConfig(datadir)
	out.Wallet = DefaultWalletConfig(datadir)
	out.Shell = DefaultShellConfig(datadir)
	return
}

// SyncToConfs takes a ConfigSet and synchronises the values according to the ConfCfg settings
func SyncToConfs(in *ConfigSet) {
	if in == nil {
		panic("received nil configset")
	}
	if in.Conf == nil ||
		in.Ctl == nil ||
		in.Node == nil ||
		in.Wallet == nil ||
		in.Shell == nil {
		panic("configset had a nil element\n" + spew.Sdump(in))
	}

	// push all current settings as from the conf configuration to the module configs
	in.Node.Node.Listeners = in.Conf.NodeListeners
	in.Shell.Node.Listeners = in.Conf.NodeListeners
	in.Node.Node.RPCListeners = in.Conf.NodeRPCListeners
	in.Wallet.Wallet.RPCConnect = in.Conf.NodeRPCListeners[0]
	in.Shell.Node.RPCListeners = in.Conf.NodeRPCListeners
	in.Shell.Wallet.RPCConnect = in.Conf.NodeRPCListeners[0]
	in.Ctl.RPCServer = in.Conf.NodeRPCListeners[0]

	in.Wallet.Wallet.LegacyRPCListeners = in.Conf.WalletListeners
	in.Ctl.Wallet = in.Conf.NodeRPCListeners[0]
	in.Shell.Wallet.LegacyRPCListeners = in.Conf.NodeRPCListeners
	in.Wallet.Wallet.LegacyRPCListeners = in.Conf.WalletListeners
	in.Ctl.Wallet = in.Conf.WalletListeners[0]
	in.Shell.Wallet.LegacyRPCListeners = in.Conf.WalletListeners

	in.Node.Node.RPCUser = in.Conf.NodeUser
	in.Wallet.Wallet.PodUsername = in.Conf.NodeUser
	in.Wallet.Wallet.Username = in.Conf.NodeUser
	in.Shell.Node.RPCUser = in.Conf.NodeUser
	in.Shell.Wallet.PodUsername = in.Conf.NodeUser
	in.Shell.Wallet.Username = in.Conf.NodeUser
	in.Ctl.RPCUser = in.Conf.NodeUser

	in.Node.Node.RPCPass = in.Conf.NodePass
	in.Wallet.Wallet.PodPassword = in.Conf.NodePass
	in.Wallet.Wallet.Password = in.Conf.NodePass
	in.Shell.Node.RPCPass = in.Conf.NodePass
	in.Shell.Wallet.PodPassword = in.Conf.NodePass
	in.Shell.Wallet.Password = in.Conf.NodePass
	in.Ctl.RPCPass = in.Conf.NodePass

	in.Node.Node.RPCKey = in.Conf.RPCKey
	in.Wallet.Wallet.RPCKey = in.Conf.RPCKey
	in.Shell.Node.RPCKey = in.Conf.RPCKey
	in.Shell.Wallet.RPCKey = in.Conf.RPCKey

	in.Node.Node.RPCCert = in.Conf.RPCCert
	in.Wallet.Wallet.RPCCert = in.Conf.RPCCert
	in.Shell.Node.RPCCert = in.Conf.RPCCert
	in.Shell.Wallet.RPCCert = in.Conf.RPCCert

	in.Wallet.Wallet.CAFile = in.Conf.CAFile
	in.Shell.Wallet.CAFile = in.Conf.CAFile

	in.Node.Node.TLS = in.Conf.TLS
	in.Wallet.Wallet.EnableClientTLS = in.Conf.TLS
	in.Shell.Node.TLS = in.Conf.TLS
	in.Shell.Wallet.EnableClientTLS = in.Conf.TLS
	in.Wallet.Wallet.EnableServerTLS = in.Conf.TLS
	in.Shell.Wallet.EnableServerTLS = in.Conf.TLS
	in.Ctl.TLSSkipVerify = in.Conf.SkipVerify

	in.Ctl.Proxy = in.Conf.Proxy
	in.Node.Node.Proxy = in.Conf.Proxy
	in.Wallet.Wallet.Proxy = in.Conf.Proxy
	in.Shell.Node.Proxy = in.Conf.Proxy
	in.Shell.Wallet.Proxy = in.Conf.Proxy

	in.Ctl.ProxyUser = in.Conf.ProxyUser
	in.Node.Node.ProxyUser = in.Conf.ProxyUser
	in.Wallet.Wallet.ProxyUser = in.Conf.ProxyUser
	in.Shell.Node.ProxyUser = in.Conf.ProxyUser
	in.Shell.Wallet.ProxyUser = in.Conf.ProxyUser

	in.Ctl.ProxyPass = in.Conf.ProxyPass
	in.Node.Node.ProxyPass = in.Conf.ProxyPass
	in.Wallet.Wallet.ProxyPass = in.Conf.ProxyPass
	in.Shell.Node.ProxyPass = in.Conf.ProxyPass
	in.Shell.Wallet.ProxyPass = in.Conf.ProxyPass

	in.Wallet.Wallet.WalletPass = in.Conf.WalletPass
	in.Shell.Wallet.WalletPass = in.Conf.WalletPass
}

// PortSet is a single set of ports for a configuration
type PortSet struct {
	P2P       string
	NodeRPC   string
	WalletRPC string
}

// GenPortSet creates a set of ports for testnet configuration
func GenPortSet(portbase int) (ps *PortSet) {
	// From the base, each element is as follows:
	// - P2P = portbase
	// - NodeRPC = portbase + 1
	// - WalletRPC =  portbase -1
	// For each set, the base is incremented by 100
	// so from 21047, you get 21047, 21048, 21046
	// and next would be 21147, 21148, 21146
	t := portbase
	ps = &PortSet{
		P2P:       fmt.Sprint(t),
		NodeRPC:   fmt.Sprint(t + 1),
		WalletRPC: fmt.Sprint(t - 1),
	}
	return
}
package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	n "git.parallelcoin.io/pod/cmd/node"
	blockchain "git.parallelcoin.io/pod/pkg/chain"
	cl "git.parallelcoin.io/pod/pkg/clog"
	"git.parallelcoin.io/pod/pkg/connmgr"
	"git.parallelcoin.io/pod/pkg/fork"
	"git.parallelcoin.io/pod/pkg/util"
	"github.com/btcsuite/go-socks/socks"
	"github.com/tucnak/climax"
)

func configNode(nc *n.Config, ctx *climax.Context, cfgFile string) int {
	var err error
	if r, ok := getIfIs(ctx, "datadir"); ok {
		nc.DataDir = filepath.Join(n.CleanAndExpandPath(r), "node")
		nc.ConfigFile = filepath.Join(n.CleanAndExpandPath(r), "conf.json")
	}
	if r, ok := getIfIs(ctx, "addpeers"); ok {
		NormalizeAddresses(r, n.DefaultPort, &nc.AddPeers)
	}
	if r, ok := getIfIs(ctx, "connectpeers"); ok {
		NormalizeAddresses(r, n.DefaultPort, &nc.ConnectPeers)
	}
	if r, ok := getIfIs(ctx, "disablelisten"); ok {
		nc.DisableListen = r == "true"
	}
	if r, ok := getIfIs(ctx, "listeners"); ok {
		NormalizeAddresses(r, n.DefaultPort, &nc.Listeners)
	}
	if r, ok := getIfIs(ctx, "maxpeers"); ok {
		if err := ParseInteger(r, "maxpeers", &nc.MaxPeers); err != nil {
			log <- cl.Wrn(err.Error())
		}
	}
	if r, ok := getIfIs(ctx, "disablebanning"); ok {
		nc.DisableBanning = r == "true"
	}
	if r, ok := getIfIs(ctx, "banduration"); ok {
		if err := ParseDuration(r, "banduration", &nc.BanDuration); err != nil {
			log <- cl.Wrn(err.Error())
		}
	}
	if r, ok := getIfIs(ctx, "banthreshold"); ok {
		var bt int
		if err := ParseInteger(r, "banthtreshold", &bt); err != nil {
			log <- cl.Wrn(err.Error())
		} else {
			nc.BanThreshold = uint32(bt)
		}
	}
	if r, ok := getIfIs(ctx, "whitelists"); ok {
		NormalizeAddresses(r, n.DefaultPort, &nc.Whitelists)
	}
	if r, ok := getIfIs(ctx, "rpcuser"); ok {
		nc.RPCUser = r
	}
	if r, ok := getIfIs(ctx, "rpcpass"); ok {
		nc.RPCPass = r
	}
	if r, ok := getIfIs(ctx, "rpclimituser"); ok {
		nc.RPCLimitUser = r
	}
	if r, ok := getIfIs(ctx, "rpclimitpass"); ok {
		nc.RPCLimitPass = r
	}
	if r, ok := getIfIs(ctx, "rpclisteners"); ok {
		NormalizeAddresses(r, n.DefaultRPCPort, &nc.RPCListeners)
	}
	if r, ok := getIfIs(ctx, "rpccert"); ok {
		nc.RPCCert = n.CleanAndExpandPath(r)
	}
	if r, ok := getIfIs(ctx, "rpckey"); ok {
		nc.RPCKey = n.CleanAndExpandPath(r)
	}
	if r, ok := getIfIs(ctx, "tls"); ok {
		nc.TLS = r == "true"
	}
	if r, ok := getIfIs(ctx, "disablednsseed"); ok {
		nc.DisableDNSSeed = r == "true"
	}
	if r, ok := getIfIs(ctx, "externalips"); ok {
		NormalizeAddresses(r, n.DefaultPort, &nc.ExternalIPs)
	}
	if r, ok := getIfIs(ctx, "proxy"); ok {
		NormalizeAddress(r, "9050", &nc.Proxy)
	}
	if r, ok := getIfIs(ctx, "proxyuser"); ok {
		nc.ProxyUser = r
	}
	if r, ok := getIfIs(ctx, "proxypass"); ok {
		nc.ProxyPass = r
	}
	if r, ok := getIfIs(ctx, "onion"); ok {
		NormalizeAddress(r, "9050", &nc.OnionProxy)
	}
	if r, ok := getIfIs(ctx, "onionuser"); ok {
		nc.OnionProxyUser = r
	}
	if r, ok := getIfIs(ctx, "onionpass"); ok {
		nc.OnionProxyPass = r
	}
	if r, ok := getIfIs(ctx, "noonion"); ok {
		nc.NoOnion = r == "true"
	}
	if r, ok := getIfIs(ctx, "torisolation"); ok {
		nc.TorIsolation = r == "true"
	}
	if r, ok := getIfIs(ctx, "network"); ok {
		switch r {
		case "testnet":
			fork.IsTestnet = true
			nc.TestNet3, nc.RegressionTest, nc.SimNet = true, false, false
			NodeConfig.params = &n.TestNet3Params
		case "regtest":
			nc.TestNet3, nc.RegressionTest, nc.SimNet = false, true, false
			NodeConfig.params = &n.RegressionNetParams
		case "simnet":
			nc.TestNet3, nc.RegressionTest, nc.SimNet = false, false, true
			NodeConfig.params = &n.SimNetParams
		default:
			nc.TestNet3, nc.RegressionTest, nc.SimNet = false, false, false
			NodeConfig.params = &n.MainNetParams
		}
		log <- cl.Debug{NodeConfig.params.Name, r}
	}
	if r, ok := getIfIs(ctx, "addcheckpoints"); ok {
		nc.AddCheckpoints = strings.Split(r, " ")
	}
	if r, ok := getIfIs(ctx, "disablecheckpoints"); ok {
		nc.DisableCheckpoints = r == "true"
	}
	if r, ok := getIfIs(ctx, "dbtype"); ok {
		nc.DbType = r
	}
	if r, ok := getIfIs(ctx, "profile"); ok {
		var p int
		if err = ParseInteger(r, "profile", &p); err == nil {
			nc.Profile = fmt.Sprint(p)
		}
	}
	if r, ok := getIfIs(ctx, "cpuprofile"); ok {
		nc.CPUProfile = r
	}
	if r, ok := getIfIs(ctx, "upnp"); ok {
		nc.Upnp = r == "true"
	}
	if r, ok := getIfIs(ctx, "minrelaytxfee"); ok {
		if err := ParseFloat(r, "minrelaytxfee", &nc.MinRelayTxFee); err != nil {
			log <- cl.Wrn(err.Error())
		}
	}
	if r, ok := getIfIs(ctx, "freetxrelaylimit"); ok {
		if err := ParseFloat(r, "freetxrelaylimit", &nc.FreeTxRelayLimit); err != nil {
			log <- cl.Wrn(err.Error())
		}
	}
	if r, ok := getIfIs(ctx, "norelaypriority"); ok {
		nc.NoRelayPriority = r == "true"
	}
	if r, ok := getIfIs(ctx, "trickleinterval"); ok {
		if err := ParseDuration(r, "trickleinterval", &nc.TrickleInterval); err != nil {
			log <- cl.Wrn(err.Error())
		}
	}
	if r, ok := getIfIs(ctx, "maxorphantxs"); ok {
		if err := ParseInteger(r, "maxorphantxs", &nc.MaxOrphanTxs); err != nil {
			log <- cl.Wrn(err.Error())
		}
	}
	if r, ok := getIfIs(ctx, "algo"); ok {
		nc.Algo = r
	}
	if r, ok := getIfIs(ctx, "generate"); ok {
		nc.Generate = r == "true"
	}
	if r, ok := getIfIs(ctx, "genthreads"); ok {
		var gt int
		if err := ParseInteger(r, "genthreads", &gt); err != nil {
			log <- cl.Wrn(err.Error())
		} else {
			nc.GenThreads = int32(gt)
		}
	}
	if r, ok := getIfIs(ctx, "miningaddrs"); ok {
		nc.MiningAddrs = strings.Split(r, " ")
	}
	if r, ok := getIfIs(ctx, "minerlistener"); ok {
		NormalizeAddress(r, n.DefaultRPCPort, &nc.MinerListener)
	}
	if r, ok := getIfIs(ctx, "minerpass"); ok {
		nc.MinerPass = r
	}
	if r, ok := getIfIs(ctx, "blockminsize"); ok {
		if err := ParseUint32(r, "blockminsize", &nc.BlockMinSize); err != nil {
			log <- cl.Wrn(err.Error())
		}
	}
	if r, ok := getIfIs(ctx, "blockmaxsize"); ok {
		if err := ParseUint32(r, "blockmaxsize", &nc.BlockMaxSize); err != nil {
			log <- cl.Wrn(err.Error())
		}
	}
	if r, ok := getIfIs(ctx, "blockminweight"); ok {
		if err := ParseUint32(r, "blockminweight", &nc.BlockMinWeight); err != nil {
			log <- cl.Wrn(err.Error())
		}
	}
	if r, ok := getIfIs(ctx, "blockmaxweight"); ok {
		if err := ParseUint32(r, "blockmaxweight", &nc.BlockMaxWeight); err != nil {
			log <- cl.Wrn(err.Error())
		}
	}
	if r, ok := getIfIs(ctx, "blockprioritysize"); ok {
		if err := ParseUint32(r, "blockmaxweight", &nc.BlockPrioritySize); err != nil {
			log <- cl.Wrn(err.Error())
		}
	}
	if r, ok := getIfIs(ctx, "uacomment"); ok {
		nc.UserAgentComments = strings.Split(r, " ")
	}
	if r, ok := getIfIs(ctx, "nopeerbloomfilters"); ok {
		nc.NoPeerBloomFilters = r == "true"
	}
	if r, ok := getIfIs(ctx, "nocfilters"); ok {
		nc.NoCFilters = r == "true"
	}
	if ctx.Is("dropcfindex") {
		nc.DropCfIndex = true
	}
	if r, ok := getIfIs(ctx, "sigcachemaxsize"); ok {
		var scms int
		if err := ParseInteger(r, "sigcachemaxsize", &scms); err != nil {
			log <- cl.Wrn(err.Error())
		} else {
			nc.SigCacheMaxSize = uint(scms)
		}
	}
	if r, ok := getIfIs(ctx, "blocksonly"); ok {
		nc.BlocksOnly = r == "true"
	}
	if r, ok := getIfIs(ctx, "txindex"); ok {
		nc.TxIndex = r == "true"
	}
	if ctx.Is("droptxindex") {
		nc.DropTxIndex = true
	}
	if ctx.Is("addrindex") {
		r, _ := ctx.Get("addrindex")
		nc.AddrIndex = r == "true"
	}
	if ctx.Is("dropaddrindex") {
		nc.DropAddrIndex = true
	}
	if r, ok := getIfIs(ctx, "relaynonstd"); ok {
		nc.RelayNonStd = r == "true"
	}
	if r, ok := getIfIs(ctx, "rejectnonstd"); ok {
		nc.RejectNonStd = r == "true"
	}

	// finished configuration

	SetLogging(ctx)

	if ctx.Is("save") {
		log <- cl.Infof{
			"saving config file to %s",
			cfgFile,
		}
		j, err := json.MarshalIndent(NodeConfig, "", "  ")
		if err != nil {
			log <- cl.Error{
				"saving config file:",
				err.Error(),
			}
		}
		j = append(j, '\n')
		log <- cl.Tracef{
			"JSON formatted config file\n%s",
			j,
		}
		err = ioutil.WriteFile(cfgFile, j, 0600)
		if err != nil {
			log <- cl.Error{"writing app config file:", err.Error()}
		}
	}

	// Service options which are only added on Windows.
	serviceOpts := serviceOptions{}
	// Perform service command and exit if specified.  Invalid service commands show an appropriate error.  Only runs on Windows since the runServiceCommand function will be nil when not on Windows.
	if serviceOpts.ServiceCommand != "" && runServiceCommand != nil {
		err := runServiceCommand(serviceOpts.ServiceCommand)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	// Don't add peers from the config file when in regression test mode.
	if nc.RegressionTest && len(nc.AddPeers) > 0 {
		nc.AddPeers = nil
	}
	// Set the mining algorithm correctly, default to random if unrecognised
	switch nc.Algo {
	case "blake14lr", "cryptonight7v2", "keccak", "lyra2rev2", "scrypt", "skein", "x11", "stribog", "random", "easy":
	default:
		nc.Algo = "random"
	}
	relayNonStd := NodeConfig.params.RelayNonStdTxs
	funcName := "loadConfig"
	switch {
	case nc.RelayNonStd && nc.RejectNonStd:
		str := "%s: rejectnonstd and relaynonstd cannot be used together -- choose only one"
		err := fmt.Errorf(str, funcName)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	case nc.RejectNonStd:
		relayNonStd = false
	case nc.RelayNonStd:
		relayNonStd = true
	}
	nc.RelayNonStd = relayNonStd
	// Append the network type to the data directory so it is "namespaced" per network.  In addition to the block database, there are other pieces of data that are saved to disk such as address manager state. All data is specific to a network, so namespacing the data directory means each individual piece of serialized data does not have to worry about changing names per network and such.
	nc.DataDir = n.CleanAndExpandPath(nc.DataDir)
	log <- cl.Debug{"netname", NodeConfig.params.Name, n.NetName(NodeConfig.params)}
	nc.DataDir = filepath.Join(nc.DataDir, n.NetName(NodeConfig.params))
	// Append the network type to the log directory so it is "namespaced" per network in the same fashion as the data directory.
	nc.LogDir = n.CleanAndExpandPath(nc.LogDir)
	nc.LogDir = filepath.Join(nc.LogDir, n.NetName(NodeConfig.params))

	// Initialize log rotation.  After log rotation has been initialized, the logger variables may be used.
	// initLogRotator(filepath.Join(nc.LogDir, DefaultLogFilename))
	// Validate database type.
	if !n.ValidDbType(nc.DbType) {
		str := "%s: The specified database type [%v] is invalid -- " +
			"supported types %v"
		err := fmt.Errorf(str, funcName, nc.DbType, n.KnownDbTypes)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// Validate profile port number
	if nc.Profile != "" {
		profilePort, err := strconv.Atoi(nc.Profile)
		if err != nil || profilePort < 1024 || profilePort > 65535 {
			str := "%s: The profile port must be between 1024 and 65535"
			err := fmt.Errorf(str, funcName)
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, usageMessage)
			return 1
		}
	}
	// Don't allow ban durations that are too short.
	if nc.BanDuration < time.Second {
		str := "%s: The banduration option may not be less than 1s -- parsed [%v]"
		err := fmt.Errorf(str, funcName, nc.BanDuration)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// Validate any given whitelisted IP addresses and networks.
	if len(nc.Whitelists) > 0 {
		var ip net.IP
		StateCfg.ActiveWhitelists = make([]*net.IPNet, 0, len(nc.Whitelists))
		for _, addr := range nc.Whitelists {
			_, ipnet, err := net.ParseCIDR(addr)
			if err != nil {
				ip = net.ParseIP(addr)
				if ip == nil {
					str := "%s: The whitelist value of '%s' is invalid"
					err = fmt.Errorf(str, funcName, addr)
					log <- cl.Err(err.Error())
					fmt.Fprintln(os.Stderr, usageMessage)
					return 1
				}
				var bits int
				if ip.To4() == nil {
					// IPv6
					bits = 128
				} else {
					bits = 32
				}
				ipnet = &net.IPNet{
					IP:   ip,
					Mask: net.CIDRMask(bits, bits),
				}
			}
			StateCfg.ActiveWhitelists = append(StateCfg.ActiveWhitelists, ipnet)
		}
	}
	// --addPeer and --connect do not mix.
	if len(nc.AddPeers) > 0 && len(nc.ConnectPeers) > 0 {
		str := "%s: the --addpeer and --connect options can not be " +
			"mixed"
		err := fmt.Errorf(str, funcName)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
	}
	// --proxy or --connect without --listen disables listening.
	if (nc.Proxy != "" || len(nc.ConnectPeers) > 0) &&
		len(nc.Listeners) == 0 {
		nc.DisableListen = true
	}
	// Connect means no DNS seeding.
	if len(nc.ConnectPeers) > 0 {
		nc.DisableDNSSeed = true
	}
	// Add the default listener if none were specified. The default listener is all addresses on the listen port for the network we are to connect to.
	if len(nc.Listeners) == 0 {
		nc.Listeners = []string{
			net.JoinHostPort("", NodeConfig.params.DefaultPort),
		}
	}
	// Check to make sure limited and admin users don't have the same username
	if nc.RPCUser == nc.RPCLimitUser && nc.RPCUser != "" {
		str := "%s: --rpcuser and --rpclimituser must not specify the same username"
		err := fmt.Errorf(str, funcName)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// Check to make sure limited and admin users don't have the same password
	if nc.RPCPass == nc.RPCLimitPass && nc.RPCPass != "" {
		str := "%s: --rpcpass and --rpclimitpass must not specify the " +
			"same password"
		err := fmt.Errorf(str, funcName)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// The RPC server is disabled if no username or password is provided.
	if (nc.RPCUser == "" || nc.RPCPass == "") &&
		(nc.RPCLimitUser == "" || nc.RPCLimitPass == "") {
		nc.DisableRPC = true
	}
	if nc.DisableRPC {
		log <- cl.Inf("RPC service is disabled")
	}
	// Default RPC to listen on localhost only.
	if !nc.DisableRPC && len(nc.RPCListeners) == 0 {
		addrs, err := net.LookupHost(n.DefaultRPCListener)
		if err != nil {
			log <- cl.Err(err.Error())
			return 1
		}
		nc.RPCListeners = make([]string, 0, len(addrs))
		for _, addr := range addrs {
			addr = net.JoinHostPort(addr, NodeConfig.params.RPCPort)
			nc.RPCListeners = append(nc.RPCListeners, addr)
		}
	}
	if nc.RPCMaxConcurrentReqs < 0 {
		str := "%s: The rpcmaxwebsocketconcurrentrequests option may not be less than 0 -- parsed [%d]"
		err := fmt.Errorf(str, funcName, nc.RPCMaxConcurrentReqs)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// Validate the the minrelaytxfee.
	StateCfg.ActiveMinRelayTxFee, err = util.NewAmount(nc.MinRelayTxFee)
	if err != nil {
		str := "%s: invalid minrelaytxfee: %v"
		err := fmt.Errorf(str, funcName, err)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// Limit the max block size to a sane value.
	if nc.BlockMaxSize < n.BlockMaxSizeMin || nc.BlockMaxSize >
		n.BlockMaxSizeMax {
		str := "%s: The blockmaxsize option must be in between %d and %d -- parsed [%d]"
		err := fmt.Errorf(str, funcName, n.BlockMaxSizeMin,
			n.BlockMaxSizeMax, nc.BlockMaxSize)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// Limit the max block weight to a sane value.
	if nc.BlockMaxWeight < n.BlockMaxWeightMin ||
		nc.BlockMaxWeight > n.BlockMaxWeightMax {
		str := "%s: The blockmaxweight option must be in between %d and %d -- parsed [%d]"
		err := fmt.Errorf(str, funcName, n.BlockMaxWeightMin,
			n.BlockMaxWeightMax, nc.BlockMaxWeight)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// Limit the max orphan count to a sane vlue.
	if nc.MaxOrphanTxs < 0 {
		str := "%s: The maxorphantx option may not be less than 0 -- parsed [%d]"
		err := fmt.Errorf(str, funcName, nc.MaxOrphanTxs)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// Limit the block priority and minimum block sizes to max block size.
	nc.BlockPrioritySize = minUint32(nc.BlockPrioritySize, nc.BlockMaxSize)
	nc.BlockMinSize = minUint32(nc.BlockMinSize, nc.BlockMaxSize)
	nc.BlockMinWeight = minUint32(nc.BlockMinWeight, nc.BlockMaxWeight)
	switch {
	// If the max block size isn't set, but the max weight is, then we'll set the limit for the max block size to a safe limit so weight takes precedence.
	case nc.BlockMaxSize == n.DefaultBlockMaxSize &&
		nc.BlockMaxWeight != n.DefaultBlockMaxWeight:
		nc.BlockMaxSize = blockchain.MaxBlockBaseSize - 1000
	// If the max block weight isn't set, but the block size is, then we'll scale the set weight accordingly based on the max block size value.
	case nc.BlockMaxSize != n.DefaultBlockMaxSize &&
		nc.BlockMaxWeight == n.DefaultBlockMaxWeight:
		nc.BlockMaxWeight = nc.BlockMaxSize * blockchain.WitnessScaleFactor
	}
	// Look for illegal characters in the user agent comments.
	for _, uaComment := range nc.UserAgentComments {
		if strings.ContainsAny(uaComment, "/:()") {
			err := fmt.Errorf("%s: The following characters must not "+
				"appear in user agent comments: '/', ':', '(', ')'",
				funcName)
			log <- cl.Err(err.Error())
			fmt.Fprintln(os.Stderr, usageMessage)
			return 1

		}
	}
	// --txindex and --droptxindex do not mix.
	if nc.TxIndex && nc.DropTxIndex {
		err := fmt.Errorf("%s: the --txindex and --droptxindex options may  not be activated at the same time",
			funcName)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1

	}
	// --addrindex and --dropaddrindex do not mix.
	if nc.AddrIndex && nc.DropAddrIndex {
		err := fmt.Errorf("%s: the --addrindex and --dropaddrindex "+
			"options may not be activated at the same time",
			funcName)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// --addrindex and --droptxindex do not mix.
	if nc.AddrIndex && nc.DropTxIndex {
		err := fmt.Errorf("%s: the --addrindex and --droptxindex options may not be activated at the same time "+
			"because the address index relies on the transaction index",
			funcName)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// Check mining addresses are valid and saved parsed versions.
	StateCfg.ActiveMiningAddrs = make([]util.Address, 0, len(nc.MiningAddrs))
	for _, strAddr := range nc.MiningAddrs {
		addr, err := util.DecodeAddress(strAddr, NodeConfig.params.Params)
		if err != nil {
			str := "%s: mining address '%s' failed to decode: %v"
			err := fmt.Errorf(str, funcName, strAddr, err)
			log <- cl.Err(err.Error())
			fmt.Fprintln(os.Stderr, usageMessage)
			return 1
		}
		if !addr.IsForNet(NodeConfig.params.Params) {
			str := "%s: mining address '%s' is on the wrong network"
			err := fmt.Errorf(str, funcName, strAddr)
			log <- cl.Err(err.Error())
			fmt.Fprintln(os.Stderr, usageMessage)
			return 1
		}
		StateCfg.ActiveMiningAddrs = append(StateCfg.ActiveMiningAddrs, addr)
	}
	// Ensure there is at least one mining address when the generate flag is set.
	if (nc.Generate || nc.MinerListener != "") && len(nc.MiningAddrs) == 0 {
		str := "%s: the generate flag is set, but there are no mining addresses specified "
		err := fmt.Errorf(str, funcName)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		os.Exit(1)
	}
	if nc.MinerPass != "" {
		StateCfg.ActiveMinerKey = fork.Argon2i([]byte(nc.MinerPass))
	}
	// Add default port to all listener addresses if needed and remove duplicate addresses.
	nc.Listeners = n.NormalizeAddresses(nc.Listeners,
		NodeConfig.params.DefaultPort)
	// Add default port to all rpc listener addresses if needed and remove duplicate addresses.
	nc.RPCListeners = n.NormalizeAddresses(nc.RPCListeners,
		NodeConfig.params.RPCPort)
	if !nc.DisableRPC && !nc.TLS {
		for _, addr := range nc.RPCListeners {
			if err != nil {
				str := "%s: RPC listen interface '%s' is invalid: %v"
				err := fmt.Errorf(str, funcName, addr, err)
				log <- cl.Err(err.Error())
				fmt.Fprintln(os.Stderr, usageMessage)
				return 1
			}
		}
	}
	// Add default port to all added peer addresses if needed and remove duplicate addresses.
	nc.AddPeers = n.NormalizeAddresses(nc.AddPeers,
		NodeConfig.params.DefaultPort)
	nc.ConnectPeers = n.NormalizeAddresses(nc.ConnectPeers,
		NodeConfig.params.DefaultPort)
	// --noonion and --onion do not mix.
	if nc.NoOnion && nc.OnionProxy != "" {
		err := fmt.Errorf("%s: the --noonion and --onion options may not be activated at the same time", funcName)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// Check the checkpoints for syntax errors.
	StateCfg.AddedCheckpoints, err = n.ParseCheckpoints(nc.AddCheckpoints)
	if err != nil {
		str := "%s: Error parsing checkpoints: %v"
		err := fmt.Errorf(str, funcName, err)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// Tor stream isolation requires either proxy or onion proxy to be set.
	if nc.TorIsolation && nc.Proxy == "" && nc.OnionProxy == "" {
		str := "%s: Tor stream isolation requires either proxy or onionproxy to be set"
		err := fmt.Errorf(str, funcName)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// Setup dial and DNS resolution (lookup) functions depending on the specified options.  The default is to use the standard net.DialTimeout function as well as the system DNS resolver.  When a proxy is specified, the dial function is set to the proxy specific dial function and the lookup is set to use tor (unless --noonion is specified in which case the system DNS resolver is used).
	StateCfg.Dial = net.DialTimeout
	StateCfg.Lookup = net.LookupIP
	if nc.Proxy != "" {
		_, _, err := net.SplitHostPort(nc.Proxy)
		if err != nil {
			str := "%s: Proxy address '%s' is invalid: %v"
			err := fmt.Errorf(str, funcName, nc.Proxy, err)
			log <- cl.Err(err.Error())
			fmt.Fprintln(os.Stderr, usageMessage)
			return 1
		}
		// Tor isolation flag means proxy credentials will be overridden unless there is also an onion proxy configured in which case that one will be overridden.
		torIsolation := false
		if nc.TorIsolation && nc.OnionProxy == "" &&
			(nc.ProxyUser != "" || nc.ProxyPass != "") {
			torIsolation = true
			fmt.Fprintln(os.Stderr, "Tor isolation set -- "+
				"overriding specified proxy user credentials")
		}
		proxy := &socks.Proxy{
			Addr:         nc.Proxy,
			Username:     nc.ProxyUser,
			Password:     nc.ProxyPass,
			TorIsolation: torIsolation,
		}
		StateCfg.Dial = proxy.DialTimeout
		// Treat the proxy as tor and perform DNS resolution through it unless the --noonion flag is set or there is an onion-specific proxy configured.
		if !nc.NoOnion && nc.OnionProxy == "" {
			StateCfg.Lookup = func(host string) ([]net.IP, error) {
				return connmgr.TorLookupIP(host, nc.Proxy)
			}
		}
	}
	// Setup onion address dial function depending on the specified options. The default is to use the same dial function selected above.  However, when an onion-specific proxy is specified, the onion address dial function is set to use the onion-specific proxy while leaving the normal dial function as selected above.  This allows .onion address traffic to be routed through a different proxy than normal traffic.
	if nc.OnionProxy != "" {
		_, _, err := net.SplitHostPort(nc.OnionProxy)
		if err != nil {
			str := "%s: Onion proxy address '%s' is invalid: %v"
			err := fmt.Errorf(str, funcName, nc.OnionProxy, err)
			log <- cl.Err(err.Error())
			fmt.Fprintln(os.Stderr, usageMessage)
			return 1
		}
		// Tor isolation flag means onion proxy credentials will be overridden.
		if nc.TorIsolation &&
			(nc.OnionProxyUser != "" || nc.OnionProxyPass != "") {
			fmt.Fprintln(os.Stderr, "Tor isolation set -- "+
				"overriding specified onionproxy user "+
				"credentials ")
		}
		StateCfg.Oniondial = func(network, addr string, timeout time.Duration) (net.Conn, error) {
			proxy := &socks.Proxy{
				Addr:         nc.OnionProxy,
				Username:     nc.OnionProxyUser,
				Password:     nc.OnionProxyPass,
				TorIsolation: nc.TorIsolation,
			}
			return proxy.DialTimeout(network, addr, timeout)
		}
		// When configured in bridge mode (both --onion and --proxy are configured), it means that the proxy configured by --proxy is not a tor proxy, so override the DNS resolution to use the onion-specific proxy.
		if nc.Proxy != "" {
			StateCfg.Lookup = func(host string) ([]net.IP, error) {
				return connmgr.TorLookupIP(host, nc.OnionProxy)
			}
		}
	} else {
		StateCfg.Oniondial = StateCfg.Dial
	}
	// Specifying --noonion means the onion address dial function results in an error.
	if nc.NoOnion {
		StateCfg.Oniondial = func(a, b string, t time.Duration) (net.Conn, error) {
			return nil, errors.New("tor has been disabled")
		}
	}
	return 0
}
package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"git.parallelcoin.io/pod/cmd/node"
	n "git.parallelcoin.io/pod/cmd/node"
	blockchain "git.parallelcoin.io/pod/pkg/chain"
	cl "git.parallelcoin.io/pod/pkg/clog"
	"git.parallelcoin.io/pod/pkg/connmgr"
	"git.parallelcoin.io/pod/pkg/fork"
	"git.parallelcoin.io/pod/pkg/netparams"
	"git.parallelcoin.io/pod/pkg/util"
	"github.com/btcsuite/go-socks/socks"
	"github.com/tucnak/climax"
)

func configShell(ctx *climax.Context, cfgFile string) int {
	ShellConfig.Wallet.AppDataDir = ShellConfig.Wallet.DataDir
	if r, ok := getIfIs(ctx, "appdatadir"); ok {
		ShellConfig.Wallet.AppDataDir = r
	}
	ShellConfig.SetNodeActiveNet(&node.MainNetParams)
	ShellConfig.SetWalletActiveNet(&netparams.MainNetParams)
	var ok bool
	var r string
	if ShellConfig.Node.TestNet3 {
		r = "testnet"
		ShellConfig.SetNodeActiveNet(&node.TestNet3Params)
		ShellConfig.SetWalletActiveNet(&netparams.TestNet3Params)
	}
	if ShellConfig.Node.SimNet {
		r = "simnet"
		ShellConfig.SetNodeActiveNet(&node.SimNetParams)
		ShellConfig.SetWalletActiveNet(&netparams.SimNetParams)
	}
	fmt.Println("nodeActiveNet.Name", r)
	if r, ok = getIfIs(ctx, "network"); ok {
		switch r {
		case "testnet":
			fork.IsTestnet = true
			ShellConfig.Wallet.TestNet3, ShellConfig.Wallet.SimNet = true, false
			ShellConfig.Node.TestNet3, ShellConfig.Node.SimNet, ShellConfig.Node.RegressionTest = true, false, false
			ShellConfig.SetNodeActiveNet(&node.TestNet3Params)
			ShellConfig.SetWalletActiveNet(&netparams.TestNet3Params)
		case "simnet":
			ShellConfig.Wallet.TestNet3, ShellConfig.Wallet.SimNet = false, true
			ShellConfig.Node.TestNet3, ShellConfig.Node.SimNet, ShellConfig.Node.RegressionTest = false, true, false
			ShellConfig.SetNodeActiveNet(&node.SimNetParams)
			ShellConfig.SetWalletActiveNet(&netparams.SimNetParams)
		default:
			ShellConfig.Wallet.TestNet3, ShellConfig.Wallet.SimNet = false, false
			ShellConfig.Node.TestNet3, ShellConfig.Node.SimNet, ShellConfig.Node.RegressionTest = false, false, false
			ShellConfig.SetNodeActiveNet(&node.MainNetParams)
			ShellConfig.SetWalletActiveNet(&netparams.MainNetParams)
		}
	}

	if ctx.Is("createtemp") {
		ShellConfig.Wallet.CreateTemp = true
	}
	if r, ok := getIfIs(ctx, "walletpass"); ok {
		ShellConfig.Wallet.WalletPass = r
	}
	if r, ok := getIfIs(ctx, "listeners"); ok {
		NormalizeAddresses(
			r, ShellConfig.GetNodeActiveNet().DefaultPort,
			&ShellConfig.Node.Listeners)
		log <- cl.Debug{"node listeners", ShellConfig.Node.Listeners}
	}
	if r, ok := getIfIs(ctx, "externalips"); ok {
		NormalizeAddresses(
			r, ShellConfig.GetNodeActiveNet().DefaultPort,
			&ShellConfig.Node.ExternalIPs)
		log <- cl.Debug{ShellConfig.Node.Listeners}
	}
	if r, ok := getIfIs(ctx, "disablelisten"); ok {
		ShellConfig.Node.DisableListen = strings.ToLower(r) == "true"
	}
	if r, ok := getIfIs(ctx, "rpclisteners"); ok {
		NormalizeAddresses(
			r, ShellConfig.GetWalletActiveNet().RPCServerPort,
			&ShellConfig.Wallet.LegacyRPCListeners)
	}
	if r, ok := getIfIs(ctx, "rpcmaxclients"); ok {
		var bt int
		if err := ParseInteger(r, "legacyrpcmaxclients", &bt); err != nil {
			log <- cl.Wrn(err.Error())
		} else {
			ShellConfig.Wallet.LegacyRPCMaxClients = int64(bt)
		}
	}
	if r, ok := getIfIs(ctx, "rpcmaxwebsockets"); ok {
		_, err := fmt.Sscanf(r, "%d", ShellConfig.Wallet.LegacyRPCMaxWebsockets)
		if err != nil {
			log <- cl.Errorf{
				"malformed legacyrpcmaxwebsockets: `%s` leaving set at `%d`",
				r, ShellConfig.Wallet.LegacyRPCMaxWebsockets,
			}
		}
	}
	if r, ok := getIfIs(ctx, "username"); ok {
		ShellConfig.Wallet.Username = r
		ShellConfig.Wallet.PodPassword = r
		ShellConfig.Node.RPCUser = r
	}
	if r, ok := getIfIs(ctx, "password"); ok {
		ShellConfig.Wallet.Password = r
		ShellConfig.Wallet.PodPassword = r
		ShellConfig.Node.RPCPass = r
	}
	if r, ok := getIfIs(ctx, "rpccert"); ok {
		ShellConfig.Wallet.RPCCert = n.CleanAndExpandPath(r)
		ShellConfig.Node.RPCCert = ShellConfig.Wallet.RPCCert
	}
	if r, ok := getIfIs(ctx, "rpckey"); ok {
		ShellConfig.Wallet.RPCKey = n.CleanAndExpandPath(r)
		ShellConfig.Node.RPCKey = ShellConfig.Wallet.RPCKey
	}
	if r, ok := getIfIs(ctx, "onetimetlskey"); ok {
		ShellConfig.Wallet.OneTimeTLSKey = strings.ToLower(r) == "true"
	}
	if r, ok := getIfIs(ctx, "cafile"); ok {
		ShellConfig.Wallet.CAFile = n.CleanAndExpandPath(r)
	}
	if r, ok := getIfIs(ctx, "tls"); ok {
		ShellConfig.Wallet.EnableServerTLS = strings.ToLower(r) == "true"
	}
	if r, ok := getIfIs(ctx, "txindex"); ok {
		ShellConfig.Node.TxIndex = strings.ToLower(r) == "true"
	}
	if r, ok := getIfIs(ctx, "addrindex"); ok {
		ShellConfig.Node.AddrIndex = strings.ToLower(r) == "true"
	}
	if ctx.Is("dropcfindex") {
		ShellConfig.Node.DropCfIndex = true
	}
	if ctx.Is("droptxindex") {
		ShellConfig.Node.DropTxIndex = true
	}
	if ctx.Is("dropaddrindex") {
		ShellConfig.Node.DropAddrIndex = true
	}
	if r, ok := getIfIs(ctx, "proxy"); ok {
		NormalizeAddress(r, "9050", &ShellConfig.Node.Proxy)
		ShellConfig.Wallet.Proxy = ShellConfig.Node.Proxy
	}
	if r, ok := getIfIs(ctx, "proxyuser"); ok {
		ShellConfig.Node.ProxyUser = r
		ShellConfig.Wallet.ProxyUser = r
	}
	if r, ok := getIfIs(ctx, "proxypass"); ok {
		ShellConfig.Node.ProxyPass = r
		ShellConfig.Node.ProxyPass = r
	}
	if r, ok := getIfIs(ctx, "onion"); ok {
		NormalizeAddress(r, "9050", &ShellConfig.Node.OnionProxy)
	}
	if r, ok := getIfIs(ctx, "onionuser"); ok {
		ShellConfig.Node.OnionProxyUser = r
	}
	if r, ok := getIfIs(ctx, "onionpass"); ok {
		ShellConfig.Node.OnionProxyPass = r
	}
	if r, ok := getIfIs(ctx, "noonion"); ok {
		ShellConfig.Node.NoOnion = r == "true"
	}
	if r, ok := getIfIs(ctx, "torisolation"); ok {
		ShellConfig.Node.TorIsolation = r == "true"
	}
	if r, ok := getIfIs(ctx, "addpeers"); ok {
		NormalizeAddresses(r, n.DefaultPort, &ShellConfig.Node.AddPeers)
	}
	if r, ok := getIfIs(ctx, "connectpeers"); ok {
		NormalizeAddresses(r, n.DefaultPort, &ShellConfig.Node.ConnectPeers)
	}
	if r, ok := getIfIs(ctx, "maxpeers"); ok {
		if err := ParseInteger(
			r, "maxpeers", &ShellConfig.Node.MaxPeers); err != nil {
			log <- cl.Wrn(err.Error())
		}
	}
	if r, ok := getIfIs(ctx, "disablebanning"); ok {
		ShellConfig.Node.DisableBanning = r == "true"
	}
	if r, ok := getIfIs(ctx, "banduration"); ok {
		if err := ParseDuration(r, "banduration", &ShellConfig.Node.BanDuration); err != nil {
			log <- cl.Wrn(err.Error())
		}
	}
	if r, ok := getIfIs(ctx, "banthreshold"); ok {
		var bt int
		if err := ParseInteger(r, "banthtreshold", &bt); err != nil {
			log <- cl.Wrn(err.Error())
		} else {
			ShellConfig.Node.BanThreshold = uint32(bt)
		}
	}
	if r, ok := getIfIs(ctx, "whitelists"); ok {
		NormalizeAddresses(r, n.DefaultPort, &ShellConfig.Node.Whitelists)
	}
	if r, ok := getIfIs(ctx, "trickleinterval"); ok {
		if err := ParseDuration(
			r, "trickleinterval", &ShellConfig.Node.TrickleInterval); err != nil {
			log <- cl.Wrn(err.Error())
		}
	}
	if r, ok := getIfIs(ctx, "minrelaytxfee"); ok {
		if err := ParseFloat(
			r, "minrelaytxfee", &ShellConfig.Node.MinRelayTxFee); err != nil {
			log <- cl.Wrn(err.Error())
		}

	}
	if r, ok := getIfIs(ctx, "freetxrelaylimit"); ok {
		if err := ParseFloat(
			r, "freetxrelaylimit", &ShellConfig.Node.FreeTxRelayLimit); err != nil {
			log <- cl.Wrn(err.Error())
		}
	}
	if r, ok := getIfIs(ctx, "norelaypriority"); ok {
		ShellConfig.Node.NoRelayPriority = r == "true"
	}
	if r, ok := getIfIs(ctx, "nopeerbloomfilters"); ok {
		ShellConfig.Node.NoPeerBloomFilters = r == "true"
	}
	if r, ok := getIfIs(ctx, "nocfilters"); ok {
		ShellConfig.Node.NoCFilters = r == "true"
	}
	if r, ok := getIfIs(ctx, "blocksonly"); ok {
		ShellConfig.Node.BlocksOnly = r == "true"
	}
	if r, ok := getIfIs(ctx, "relaynonstd"); ok {
		ShellConfig.Node.RelayNonStd = r == "true"
	}
	if r, ok := getIfIs(ctx, "rejectnonstd"); ok {
		ShellConfig.Node.RejectNonStd = r == "true"
	}
	if r, ok := getIfIs(ctx, "maxorphantxs"); ok {
		if err := ParseInteger(r, "maxorphantxs", &ShellConfig.Node.MaxOrphanTxs); err != nil {
			log <- cl.Wrn(err.Error())
		}
	}
	if r, ok := getIfIs(ctx, "sigcachemaxsize"); ok {
		var scms int
		if err := ParseInteger(r, "sigcachemaxsize", &scms); err != nil {
			log <- cl.Wrn(err.Error())
		} else {
			ShellConfig.Node.SigCacheMaxSize = uint(scms)
		}
	}
	if r, ok := getIfIs(ctx, "generate"); ok {
		ShellConfig.Node.Generate = r == "true"
	}
	if r, ok := getIfIs(ctx, "genthreads"); ok {
		var gt int
		if err := ParseInteger(r, "genthreads", &gt); err != nil {
			log <- cl.Wrn(err.Error())
		} else {
			ShellConfig.Node.GenThreads = int32(gt)
		}
	}
	if r, ok := getIfIs(ctx, "algo"); ok {
		ShellConfig.Node.Algo = r
	}
	if r, ok := getIfIs(ctx, "miningaddrs"); ok {
		ShellConfig.Node.MiningAddrs = strings.Split(r, " ")
	}
	if r, ok := getIfIs(ctx, "minerlistener"); ok {
		NormalizeAddress(r, n.DefaultRPCPort, &ShellConfig.Node.MinerListener)
	}
	if r, ok := getIfIs(ctx, "minerpass"); ok {
		ShellConfig.Node.MinerPass = r
	}
	if r, ok := getIfIs(ctx, "addcheckpoints"); ok {
		ShellConfig.Node.AddCheckpoints = strings.Split(r, " ")
	}
	if r, ok := getIfIs(ctx, "disablecheckpoints"); ok {
		ShellConfig.Node.DisableCheckpoints = r == "true"
	}
	if r, ok := getIfIs(ctx, "blockminsize"); ok {
		if err := ParseUint32(r, "blockminsize", &ShellConfig.Node.BlockMinSize); err != nil {
			log <- cl.Wrn(err.Error())
		}
	}
	if r, ok := getIfIs(ctx, "blockmaxsize"); ok {
		if err := ParseUint32(r, "blockmaxsize", &ShellConfig.Node.BlockMaxSize); err != nil {
			log <- cl.Wrn(err.Error())
		}
	}
	if r, ok := getIfIs(ctx, "blockminweight"); ok {
		if err := ParseUint32(r, "blockminweight", &ShellConfig.Node.BlockMinWeight); err != nil {
			log <- cl.Wrn(err.Error())
		}
	}
	if r, ok := getIfIs(ctx, "blockmaxweight"); ok {
		if err := ParseUint32(
			r, "blockmaxweight", &ShellConfig.Node.BlockMaxWeight); err != nil {
			log <- cl.Wrn(err.Error())
		}
	}
	if r, ok := getIfIs(ctx, "blockprioritysize"); ok {
		if err := ParseUint32(
			r, "blockmaxweight", &ShellConfig.Node.BlockPrioritySize); err != nil {
			log <- cl.Wrn(err.Error())
		}
	}
	if r, ok := getIfIs(ctx, "uacomment"); ok {
		ShellConfig.Node.UserAgentComments = strings.Split(r, " ")
	}
	if r, ok := getIfIs(ctx, "upnp"); ok {
		ShellConfig.Node.Upnp = r == "true"
	}
	if r, ok := getIfIs(ctx, "dbtype"); ok {
		ShellConfig.Node.DbType = r
	}
	if r, ok := getIfIs(ctx, "disablednsseed"); ok {
		ShellConfig.Node.DisableDNSSeed = r == "true"
	}
	if r, ok := getIfIs(ctx, "profile"); ok {
		var p int
		if err := ParseInteger(r, "profile", &p); err == nil {
			ShellConfig.Node.Profile = fmt.Sprint(p)
		}
	}
	if r, ok := getIfIs(ctx, "cpuprofile"); ok {
		ShellConfig.Node.CPUProfile = r
	}

	// finished configuration

	SetLogging(ctx)

	// Service options which are only added on Windows.
	serviceOpts := serviceOptions{}
	// Perform service command and exit if specified.  Invalid service commands show an appropriate error.  Only runs on Windows since the runServiceCommand function will be nil when not on Windows.
	if serviceOpts.ServiceCommand != "" && runServiceCommand != nil {
		err := runServiceCommand(serviceOpts.ServiceCommand)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	// Don't add peers from the config file when in regression test mode.
	if ShellConfig.Node.RegressionTest && len(ShellConfig.Node.AddPeers) > 0 {
		ShellConfig.Node.AddPeers = nil
	}
	// Set the mining algorithm correctly, default to random if unrecognised
	switch ShellConfig.Node.Algo {
	case "blake14lr", "cryptonight7v2", "keccak", "lyra2rev2", "scrypt", "skein", "x11", "stribog", "random", "easy":
	default:
		ShellConfig.Node.Algo = "random"
	}
	relayNonStd := n.ActiveNetParams.RelayNonStdTxs
	funcName := "loadConfig"
	switch {
	case ShellConfig.Node.RelayNonStd && ShellConfig.Node.RejectNonStd:
		str := "%s: rejectnonstd and relaynonstd cannot be used together -- choose only one"
		err := fmt.Errorf(str, funcName)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	case ShellConfig.Node.RejectNonStd:
		relayNonStd = false
	case ShellConfig.Node.RelayNonStd:
		relayNonStd = true
	}
	ShellConfig.Node.RelayNonStd = relayNonStd
	// Append the network type to the data directory so it is "namespaced" per network.  In addition to the block database, there are other pieces of data that are saved to disk such as address manager state. All data is specific to a network, so namespacing the data directory means each individual piece of serialized data does not have to worry about changing names per network and such.
	ShellConfig.Node.DataDir = n.CleanAndExpandPath(ShellConfig.Node.DataDir)
	ShellConfig.Node.DataDir = filepath.Join(ShellConfig.Node.DataDir, n.NetName(ShellConfig.GetNodeActiveNet()))
	// Append the network type to the log directory so it is "namespaced" per network in the same fashion as the data directory.
	ShellConfig.Node.LogDir = n.CleanAndExpandPath(ShellConfig.Node.LogDir)
	ShellConfig.Node.LogDir = filepath.Join(ShellConfig.Node.LogDir, n.NetName(ShellConfig.GetNodeActiveNet()))

	// Initialize log rotation.  After log rotation has been initialized, the logger variables may be used.
	// initLogRotator(filepath.Join(ShellConfig.Node.LogDir, DefaultLogFilename))
	// Validate database type.
	if !n.ValidDbType(ShellConfig.Node.DbType) {
		str := "%s: The specified database type [%v] is invalid -- supported types %v"
		err := fmt.Errorf(str, funcName, ShellConfig.Node.DbType, n.KnownDbTypes)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// Validate profile port number
	if ShellConfig.Node.Profile != "" {
		profilePort, err := strconv.Atoi(ShellConfig.Node.Profile)
		if err != nil || profilePort < 1024 || profilePort > 65535 {
			str := "%s: The profile port must be between 1024 and 65535"
			err := fmt.Errorf(str, funcName)
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, usageMessage)
			return 1
		}
	}
	// Don't allow ban durations that are too short.
	if ShellConfig.Node.BanDuration < time.Second {
		str := "%s: The banduration option may not be less than 1s -- parsed [%v]"
		err := fmt.Errorf(str, funcName, ShellConfig.Node.BanDuration)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// Validate any given whitelisted IP addresses and networks.
	if len(ShellConfig.Node.Whitelists) > 0 {
		var ip net.IP
		StateCfg.ActiveWhitelists = make([]*net.IPNet, 0, len(ShellConfig.Node.Whitelists))
		for _, addr := range ShellConfig.Node.Whitelists {
			_, ipnet, err := net.ParseCIDR(addr)
			if err != nil {
				ip = net.ParseIP(addr)
				if ip == nil {
					str := "%s: The whitelist value of '%s' is invalid"
					err = fmt.Errorf(str, funcName, addr)
					log <- cl.Err(err.Error())
					fmt.Fprintln(os.Stderr, usageMessage)
					return 1
				}
				var bits int
				if ip.To4() == nil {
					// IPv6
					bits = 128
				} else {
					bits = 32
				}
				ipnet = &net.IPNet{
					IP:   ip,
					Mask: net.CIDRMask(bits, bits),
				}
			}
			StateCfg.ActiveWhitelists = append(StateCfg.ActiveWhitelists, ipnet)
		}
	}
	// --addPeer and --connect do not mix.
	if len(ShellConfig.Node.AddPeers) > 0 && len(ShellConfig.Node.ConnectPeers) > 0 {
		str := "%s: the --addpeer and --connect options can not be " +
			"mixed"
		err := fmt.Errorf(str, funcName)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
	}
	// --proxy or --connect without --listen disables listening.
	if (ShellConfig.Node.Proxy != "" || len(ShellConfig.Node.ConnectPeers) > 0) &&
		len(ShellConfig.Node.Listeners) == 0 {
		ShellConfig.Node.DisableListen = true
	}
	// Connect means no DNS seeding.
	if len(ShellConfig.Node.ConnectPeers) > 0 {
		ShellConfig.Node.DisableDNSSeed = true
	}
	// Add the default listener if none were specified. The default listener is all addresses on the listen port for the network we are to connect to.
	if len(ShellConfig.Node.Listeners) == 0 {
		ShellConfig.Node.Listeners = []string{
			net.JoinHostPort("localhost", ShellConfig.GetNodeActiveNet().DefaultPort),
		}
	}
	// Check to make sure limited and admin users don't have the same username
	if ShellConfig.Node.RPCUser == ShellConfig.Node.RPCLimitUser && ShellConfig.Node.RPCUser != "" {
		str := "%s: --rpcuser and --rpclimituser must not specify the same username"
		err := fmt.Errorf(str, funcName)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// Check to make sure limited and admin users don't have the same password
	if ShellConfig.Node.RPCPass == ShellConfig.Node.RPCLimitPass && ShellConfig.Node.RPCPass != "" {
		str := "%s: --rpcpass and --rpclimitpass must not specify the same password"
		err := fmt.Errorf(str, funcName)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// The RPC server is disabled if no username or password is provided.
	if (ShellConfig.Node.RPCUser == "" || ShellConfig.Node.RPCPass == "") &&
		(ShellConfig.Node.RPCLimitUser == "" || ShellConfig.Node.RPCLimitPass == "") {
		ShellConfig.Node.DisableRPC = true
	}
	if ShellConfig.Node.DisableRPC {
		log <- cl.Inf("RPC service is disabled")
	}
	// Default RPC to listen on localhost only.
	if !ShellConfig.Node.DisableRPC && len(ShellConfig.Node.RPCListeners) == 0 {
		addrs, err := net.LookupHost(n.DefaultRPCListener)
		if err != nil {
			log <- cl.Err(err.Error())
			return 1
		}
		ShellConfig.Node.RPCListeners = make([]string, 0, len(addrs))
		for _, addr := range addrs {
			addr = net.JoinHostPort(addr, n.ActiveNetParams.RPCPort)
			ShellConfig.Node.RPCListeners = append(ShellConfig.Node.RPCListeners, addr)
		}
	}
	if ShellConfig.Node.RPCMaxConcurrentReqs < 0 {
		str := "%s: The rpcmaxwebsocketconcurrentrequests option may not be less than 0 -- parsed [%d]"
		err := fmt.Errorf(str, funcName, ShellConfig.Node.RPCMaxConcurrentReqs)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	var err error
	// Validate the the minrelaytxfee.
	StateCfg.ActiveMinRelayTxFee, err = util.NewAmount(ShellConfig.Node.MinRelayTxFee)
	if err != nil {
		str := "%s: invalid minrelaytxfee: %v"
		err := fmt.Errorf(str, funcName, err)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// Limit the max block size to a sane value.
	if ShellConfig.Node.BlockMaxSize < n.BlockMaxSizeMin || ShellConfig.Node.BlockMaxSize >
		n.BlockMaxSizeMax {
		str := "%s: The blockmaxsize option must be in between %d and %d -- parsed [%d]"
		err := fmt.Errorf(str, funcName, n.BlockMaxSizeMin,
			n.BlockMaxSizeMax, ShellConfig.Node.BlockMaxSize)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// Limit the max block weight to a sane value.
	if ShellConfig.Node.BlockMaxWeight < n.BlockMaxWeightMin ||
		ShellConfig.Node.BlockMaxWeight > n.BlockMaxWeightMax {
		str := "%s: The blockmaxweight option must be in between %d and %d -- parsed [%d]"
		err := fmt.Errorf(str, funcName, n.BlockMaxWeightMin,
			n.BlockMaxWeightMax, ShellConfig.Node.BlockMaxWeight)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// Limit the max orphan count to a sane vlue.
	if ShellConfig.Node.MaxOrphanTxs < 0 {
		str := "%s: The maxorphantx option may not be less than 0 -- parsed [%d]"
		err := fmt.Errorf(str, funcName, ShellConfig.Node.MaxOrphanTxs)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// Limit the block priority and minimum block sizes to max block size.
	ShellConfig.Node.BlockPrioritySize = minUint32(ShellConfig.Node.BlockPrioritySize, ShellConfig.Node.BlockMaxSize)
	ShellConfig.Node.BlockMinSize = minUint32(ShellConfig.Node.BlockMinSize, ShellConfig.Node.BlockMaxSize)
	ShellConfig.Node.BlockMinWeight = minUint32(ShellConfig.Node.BlockMinWeight, ShellConfig.Node.BlockMaxWeight)
	switch {
	// If the max block size isn't set, but the max weight is, then we'll set the limit for the max block size to a safe limit so weight takes precedence.
	case ShellConfig.Node.BlockMaxSize == n.DefaultBlockMaxSize &&
		ShellConfig.Node.BlockMaxWeight != n.DefaultBlockMaxWeight:
		ShellConfig.Node.BlockMaxSize = blockchain.MaxBlockBaseSize - 1000
	// If the max block weight isn't set, but the block size is, then we'll scale the set weight accordingly based on the max block size value.
	case ShellConfig.Node.BlockMaxSize != n.DefaultBlockMaxSize &&
		ShellConfig.Node.BlockMaxWeight == n.DefaultBlockMaxWeight:
		ShellConfig.Node.BlockMaxWeight = ShellConfig.Node.BlockMaxSize * blockchain.WitnessScaleFactor
	}
	// Look for illegal characters in the user agent comments.
	for _, uaComment := range ShellConfig.Node.UserAgentComments {
		if strings.ContainsAny(uaComment, "/:()") {
			err := fmt.Errorf("%s: The following characters must not "+
				"appear in user agent comments: '/', ':', '(', ')'",
				funcName)
			log <- cl.Err(err.Error())
			fmt.Fprintln(os.Stderr, usageMessage)
			return 1

		}
	}
	// --txindex and --droptxindex do not mix.
	if ShellConfig.Node.TxIndex && ShellConfig.Node.DropTxIndex {
		err := fmt.Errorf("%s: the --txindex and --droptxindex options may  not be activated at the same time",
			funcName)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1

	}
	// --addrindex and --dropaddrindex do not mix.
	if ShellConfig.Node.AddrIndex && ShellConfig.Node.DropAddrIndex {
		err := fmt.Errorf("%s: the --addrindex and --dropaddrindex "+
			"options may not be activated at the same time",
			funcName)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// --addrindex and --droptxindex do not mix.
	if ShellConfig.Node.AddrIndex && ShellConfig.Node.DropTxIndex {
		err := fmt.Errorf("%s: the --addrindex and --droptxindex options may not be activated at the same time "+
			"because the address index relies on the transaction index",
			funcName)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// Check mining addresses are valid and saved parsed versions.
	StateCfg.ActiveMiningAddrs = make([]util.Address, 0, len(ShellConfig.Node.MiningAddrs))
	for _, strAddr := range ShellConfig.Node.MiningAddrs {
		addr, err := util.DecodeAddress(strAddr, n.ActiveNetParams.Params)
		if err != nil {
			str := "%s: mining address '%s' failed to decode: %v"
			err := fmt.Errorf(str, funcName, strAddr, err)
			log <- cl.Err(err.Error())
			fmt.Fprintln(os.Stderr, usageMessage)
			return 1
		}
		if !addr.IsForNet(n.ActiveNetParams.Params) {
			str := "%s: mining address '%s' is on the wrong network"
			err := fmt.Errorf(str, funcName, strAddr)
			log <- cl.Err(err.Error())
			fmt.Fprintln(os.Stderr, usageMessage)
			return 1
		}
		StateCfg.ActiveMiningAddrs = append(StateCfg.ActiveMiningAddrs, addr)
	}
	// Ensure there is at least one mining address when the generate flag is set.
	if (ShellConfig.Node.Generate || ShellConfig.Node.MinerListener != "") && len(ShellConfig.Node.MiningAddrs) == 0 {
		str := "%s: the generate flag is set, but there are no mining addresses specified "
		err := fmt.Errorf(str, funcName)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		os.Exit(1)
	}
	if ShellConfig.Node.MinerPass != "" {
		StateCfg.ActiveMinerKey = fork.Argon2i([]byte(ShellConfig.Node.MinerPass))
	}
	// Add default port to all listener addresses if needed and remove duplicate addresses.
	ShellConfig.Node.Listeners = n.NormalizeAddresses(ShellConfig.Node.Listeners,
		ShellConfig.GetNodeActiveNet().DefaultPort)
	// Add default port to all rpc listener addresses if needed and remove duplicate addresses.
	ShellConfig.Node.RPCListeners = n.NormalizeAddresses(ShellConfig.Node.RPCListeners,
		ShellConfig.GetNodeActiveNet().RPCPort)
	if !ShellConfig.Node.DisableRPC && !ShellConfig.Node.TLS {
		for _, addr := range ShellConfig.Node.RPCListeners {
			if err != nil {
				str := "%s: RPC listen interface '%s' is invalid: %v"
				err := fmt.Errorf(str, funcName, addr, err)
				log <- cl.Err(err.Error())
				fmt.Fprintln(os.Stderr, usageMessage)
				return 1
			}
		}
	}
	// Add default port to all added peer addresses if needed and remove duplicate addresses.
	ShellConfig.Node.AddPeers = n.NormalizeAddresses(ShellConfig.Node.AddPeers,
		ShellConfig.GetNodeActiveNet().DefaultPort)
	ShellConfig.Node.ConnectPeers = n.NormalizeAddresses(ShellConfig.Node.ConnectPeers,
		ShellConfig.GetNodeActiveNet().DefaultPort)
	// --noonion and --onion do not mix.
	if ShellConfig.Node.NoOnion && ShellConfig.Node.OnionProxy != "" {
		err := fmt.Errorf("%s: the --noonion and --onion options may not be activated at the same time", funcName)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// Check the checkpoints for syntax errors.
	StateCfg.AddedCheckpoints, err = n.ParseCheckpoints(ShellConfig.Node.AddCheckpoints)
	if err != nil {
		str := "%s: Error parsing checkpoints: %v"
		err := fmt.Errorf(str, funcName, err)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// Tor stream isolation requires either proxy or onion proxy to be set.
	if ShellConfig.Node.TorIsolation && ShellConfig.Node.Proxy == "" && ShellConfig.Node.OnionProxy == "" {
		str := "%s: Tor stream isolation requires either proxy or onionproxy to be set"
		err := fmt.Errorf(str, funcName)
		log <- cl.Err(err.Error())
		fmt.Fprintln(os.Stderr, usageMessage)
		return 1
	}
	// Setup dial and DNS resolution (lookup) functions depending on the specified options.  The default is to use the standard net.DialTimeout function as well as the system DNS resolver.  When a proxy is specified, the dial function is set to the proxy specific dial function and the lookup is set to use tor (unless --noonion is specified in which case the system DNS resolver is used).
	StateCfg.Dial = net.DialTimeout
	StateCfg.Lookup = net.LookupIP
	if ShellConfig.Node.Proxy != "" {
		_, _, err := net.SplitHostPort(ShellConfig.Node.Proxy)
		if err != nil {
			str := "%s: Proxy address '%s' is invalid: %v"
			err := fmt.Errorf(str, funcName, ShellConfig.Node.Proxy, err)
			log <- cl.Err(err.Error())
			fmt.Fprintln(os.Stderr, usageMessage)
			return 1
		}
		// Tor isolation flag means proxy credentials will be overridden unless there is also an onion proxy configured in which case that one will be overridden.
		torIsolation := false
		if ShellConfig.Node.TorIsolation && ShellConfig.Node.OnionProxy == "" &&
			(ShellConfig.Node.ProxyUser != "" || ShellConfig.Node.ProxyPass != "") {
			torIsolation = true
			fmt.Fprintln(os.Stderr, "Tor isolation set -- "+
				"overriding specified proxy user credentials")
		}
		proxy := &socks.Proxy{
			Addr:         ShellConfig.Node.Proxy,
			Username:     ShellConfig.Node.ProxyUser,
			Password:     ShellConfig.Node.ProxyPass,
			TorIsolation: torIsolation,
		}
		StateCfg.Dial = proxy.DialTimeout
		// Treat the proxy as tor and perform DNS resolution through it unless the --noonion flag is set or there is an onion-specific proxy configured.
		if !ShellConfig.Node.NoOnion && ShellConfig.Node.OnionProxy == "" {
			StateCfg.Lookup = func(host string) ([]net.IP, error) {
				return connmgr.TorLookupIP(host, ShellConfig.Node.Proxy)
			}
		}
	}
	// Setup onion address dial function depending on the specified options. The default is to use the same dial function selected above.  However, when an onion-specific proxy is specified, the onion address dial function is set to use the onion-specific proxy while leaving the normal dial function as selected above.  This allows .onion address traffic to be routed through a different proxy than normal traffic.
	if ShellConfig.Node.OnionProxy != "" {
		_, _, err := net.SplitHostPort(ShellConfig.Node.OnionProxy)
		if err != nil {
			str := "%s: Onion proxy address '%s' is invalid: %v"
			err := fmt.Errorf(str, funcName, ShellConfig.Node.OnionProxy, err)
			log <- cl.Err(err.Error())
			fmt.Fprintln(os.Stderr, usageMessage)
			return 1
		}
		// Tor isolation flag means onion proxy credentials will be overridden.
		if ShellConfig.Node.TorIsolation &&
			(ShellConfig.Node.OnionProxyUser != "" || ShellConfig.Node.OnionProxyPass != "") {
			fmt.Fprintln(os.Stderr, "Tor isolation set -- "+
				"overriding specified onionproxy user "+
				"credentials ")
		}
		StateCfg.Oniondial = func(network, addr string, timeout time.Duration) (net.Conn, error) {
			proxy := &socks.Proxy{
				Addr:         ShellConfig.Node.OnionProxy,
				Username:     ShellConfig.Node.OnionProxyUser,
				Password:     ShellConfig.Node.OnionProxyPass,
				TorIsolation: ShellConfig.Node.TorIsolation,
			}
			return proxy.DialTimeout(network, addr, timeout)
		}
		// When configured in bridge mode (both --onion and --proxy are configured), it means that the proxy configured by --proxy is not a tor proxy, so override the DNS resolution to use the onion-specific proxy.
		if ShellConfig.Node.Proxy != "" {
			StateCfg.Lookup = func(host string) ([]net.IP, error) {
				return connmgr.TorLookupIP(host, ShellConfig.Node.OnionProxy)
			}
		}
	} else {
		StateCfg.Oniondial = StateCfg.Dial
	}
	// Specifying --noonion means the onion address dial function results in an error.
	if ShellConfig.Node.NoOnion {
		StateCfg.Oniondial = func(a, b string, t time.Duration) (net.Conn, error) {
			return nil, errors.New("tor has been disabled")
		}
	}

	ShellConfig.Wallet.PodUsername = ShellConfig.Node.RPCUser
	ShellConfig.Wallet.PodPassword = ShellConfig.Node.RPCPass

	if ctx.Is("save") {
		log <- cl.Info{"saving config file to", cfgFile}
		j, err := json.MarshalIndent(ShellConfig, "", "  ")
		if err != nil {
			log <- cl.Error{"writing app config file", err}
		}
		j = append(j, '\n')
		log <- cl.Trace{"JSON formatted config file\n", string(j)}
		ioutil.WriteFile(cfgFile, j, 0600)
	}
	return 0
}
package app

import (
	"path/filepath"

	"git.parallelcoin.io/pod/cmd/node"
	"git.parallelcoin.io/pod/pkg/util"
)

var (
	// AppName is the name of this application
	AppName = "pod"
	// DefaultDataDir is the default location for the data
	DefaultDataDir = util.AppDataDir(AppName, false)
	// DefaultShellDataDir is the default data directory for the shell
	DefaultShellDataDir = filepath.Join(
		node.DefaultHomeDir, "shell")
	// DefaultShellConfFileName is
	DefaultShellConfFileName = filepath.Join(
		filepath.Join(node.DefaultHomeDir, "shell"), "conf")
	f = GenFlag
	t = GenTrig
	s = GenShort
	l = GenLog
)
package app

import (
	"strings"

	"github.com/tucnak/climax"
)

// GetFlags reads out the flags in a climax.Command and reads the default value stored there into a searchable map
func GetFlags(cmd climax.Command) (out map[string]string) {
	out = make(map[string]string)
	for i := range cmd.Flags {
		usage := strings.Split(cmd.Flags[i].Usage, " ")
		if cmd.Flags[i].Usage == `""` ||
			len(cmd.Flags[i].Usage) < 2 ||
			len(usage) < 2 {
			out[cmd.Flags[i].Name] = ""
		}
		if len(usage) > 1 {
			u := usage[1][1 : len(usage)-2]
			out[cmd.Flags[i].Name] = u
		}
	}
	return
}
package app

import (
	cl "git.parallelcoin.io/pod/pkg/clog"
)

// Log is the logger for node
var Log = cl.NewSubSystem("pod/app        ", "info")
var log = Log.Ch
package app

import (
	"git.parallelcoin.io/pod/cmd/node"
	"git.parallelcoin.io/pod/cmd/node/mempool"
	"git.parallelcoin.io/pod/cmd/spv"
	walletmain "git.parallelcoin.io/pod/cmd/wallet"
	"git.parallelcoin.io/pod/pkg/addrmgr"
	blockchain "git.parallelcoin.io/pod/pkg/chain"
	cl "git.parallelcoin.io/pod/pkg/clog"
	"git.parallelcoin.io/pod/pkg/connmgr"
	database "git.parallelcoin.io/pod/pkg/db"
	"git.parallelcoin.io/pod/pkg/db/ffldb"
	"git.parallelcoin.io/pod/pkg/mining"
	"git.parallelcoin.io/pod/pkg/mining/cpuminer"
	"git.parallelcoin.io/pod/pkg/netsync"
	"git.parallelcoin.io/pod/pkg/peer"
	"git.parallelcoin.io/pod/pkg/rpc/legacyrpc"
	"git.parallelcoin.io/pod/pkg/rpc/rpcserver"
	"git.parallelcoin.io/pod/pkg/rpcclient"
	"git.parallelcoin.io/pod/pkg/txscript"
	"git.parallelcoin.io/pod/pkg/votingpool"
	"git.parallelcoin.io/pod/pkg/waddrmgr"
	"git.parallelcoin.io/pod/pkg/wallet"
	"git.parallelcoin.io/pod/pkg/wallettx"
	chain "git.parallelcoin.io/pod/pkg/wchain"
	"git.parallelcoin.io/pod/pkg/wtxmgr"
	"github.com/tucnak/climax"
)

// LogLevels are the configured log level settings
var LogLevels = GetDefaultLogLevelsConfig()

// GetDefaultLogLevelsConfig returns a fresh shiny new default levels map
func GetDefaultLogLevelsConfig() map[string]string {
	return map[string]string{
		"lib-addrmgr":         "info",
		"lib-blockchain":      "info",
		"lib-connmgr":         "info",
		"lib-database-ffldb":  "info",
		"lib-database":        "info",
		"lib-mining-cpuminer": "info",
		"lib-mining":          "info",
		"lib-netsync":         "info",
		"lib-peer":            "info",
		"lib-rpcclient":       "info",
		"lib-txscript":        "info",
		"node":                "info",
		"node-mempool":        "info",
		"spv":                 "info",
		"wallet":              "info",
		"wallet-chain":        "info",
		"wallet-legacyrpc":    "info",
		"wallet-rpcserver":    "info",
		"wallet-tx":           "info",
		"wallet-votingpool":   "info",
		"wallet-waddrmgr":     "info",
		"wallet-wallet":       "info",
		"wallet-wtxmgr":       "info",
	}
}

// GetAllSubSystems returns a map with all the SubSystems in Parallelcoin Pod
func GetAllSubSystems() map[string]*cl.SubSystem {
	return map[string]*cl.SubSystem{
		"lib-addrmgr":         addrmgr.Log,
		"lib-blockchain":      blockchain.Log,
		"lib-connmgr":         connmgr.Log,
		"lib-database-ffldb":  ffldb.Log,
		"lib-database":        database.Log,
		"lib-mining-cpuminer": cpuminer.Log,
		"lib-mining":          mining.Log,
		"lib-netsync":         netsync.Log,
		"lib-peer":            peer.Log,
		"lib-rpcclient":       rpcclient.Log,
		"lib-txscript":        txscript.Log,
		"node":                node.Log,
		"node-mempool":        mempool.Log,
		"spv":                 spv.Log,
		"wallet":              walletmain.Log,
		"wallet-chain":        chain.Log,
		"wallet-legacyrpc":    legacyrpc.Log,
		"wallet-rpcserver":    rpcserver.Log,
		"wallet-tx":           wallettx.Log,
		"wallet-votingpool":   votingpool.Log,
		"wallet-waddrmgr":     waddrmgr.Log,
		"wallet-wallet":       wallet.Log,
		"wallet-wtxmgr":       wtxmgr.Log,
	}
}

// SetLogging sets the logging settings according to the provided context
func SetLogging(ctx *climax.Context) {
	ss := GetAllSubSystems()
	var baselevel = "info"
	if r, ok := getIfIs(ctx, "debuglevel"); ok {
		baselevel = r
	}
	for i := range ss {
		if lvl, ok := ctx.Get(i); ok {
			ss[i].SetLevel(lvl)
		} else {
			ss[i].SetLevel(baselevel)
		}
	}
}

// SetAllLogging sets all the logging to a particular level
func SetAllLogging(level string) {
	ss := GetAllSubSystems()
	for i := range ss {
		ss[i].SetLevel(level)
	}
}
package app

import (
	"github.com/tucnak/climax"
)

// PodApp is the climax main app controller for pod
var PodApp = climax.Application{
	Name:     "pod",
	Brief:    "multi-application launcher for Parallelcoin Pod",
	Version:  Version(),
	Commands: []climax.Command{},
	Topics:   []climax.Topic{},
	Groups:   []climax.Group{},
	// Default:  GUICommand.Handle,
}

// Main is the real pod main
func Main() int {
	PodApp.AddCommand(VersionCommand)
	PodApp.AddCommand(ConfCommand)
	PodApp.AddCommand(GUICommand)
	PodApp.AddCommand(CtlCommand)
	PodApp.AddCommand(NodeCommand)
	PodApp.AddCommand(WalletCommand)
	PodApp.AddCommand(ShellCommand)
	PodApp.AddCommand(CreateCommand)
	PodApp.AddCommand(SetupCommand)
	return PodApp.Run()
}
package app

func runConf() {
}
package app

import (
	"encoding/json"

	"git.parallelcoin.io/pod/cmd/ctl"
	cl "git.parallelcoin.io/pod/pkg/clog"
)

func runCtl(args []string, cc *ctl.Config) {
	j, _ := json.MarshalIndent(cc, "", "  ")
	log <- cl.Tracef{"running with configuration:\n%s", string(j)}
	ctl.Main(args, cc)
}
package app

import (
	"encoding/json"
	"fmt"

	"git.parallelcoin.io/pod/cmd/node"
	cl "git.parallelcoin.io/pod/pkg/clog"
)

func runNode(nc *node.Config, activeNet *node.Params) int {
	j, _ := json.MarshalIndent(nc, "", "  ")
	log <- cl.Tracef{"running with configuration:\n%s", string(j)}
	err := node.Main(nc, activeNet, nil)
	if err != nil {
		fmt.Print(err)
		return 1
	}
	return 0
}
package app

import (
	"encoding/json"
	"sync"
	"time"

	cl "git.parallelcoin.io/pod/pkg/clog"
	"git.parallelcoin.io/pod/pkg/interrupt"
)

func runShell() (out int) {
	j, _ := json.MarshalIndent(ShellConfig, "", "  ")
	log <- cl.Tracef{"running with configuration:\n%s", string(j)}
	var wg sync.WaitGroup
	go func() {
		wg.Add(1)
		out = runNode(ShellConfig.Node, ShellConfig.GetNodeActiveNet())
		wg.Done()
	}()
	time.Sleep(time.Second * 2)
	go func() {
		wg.Add(1)
		out = runWallet(ShellConfig.Wallet, ShellConfig.GetWalletActiveNet())
		wg.Done()
	}()
	wg.Wait()
	<-interrupt.HandlersDone
	return 0
}
package app

import (
	"encoding/json"
	"fmt"

	walletmain "git.parallelcoin.io/pod/cmd/wallet"

	cl "git.parallelcoin.io/pod/pkg/clog"
	"git.parallelcoin.io/pod/pkg/netparams"
)

func runWallet(wc *walletmain.Config, activeNet *netparams.Params) int {
	j, _ := json.MarshalIndent(wc, "", "  ")
	log <- cl.Tracef{"running with configuration:\n%s", string(j)}
	err := walletmain.Main(wc, activeNet)
	if err != nil {
		fmt.Print(err)
		return 1
	}
	return 0
}
package app

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"git.parallelcoin.io/pod/cmd/node"
	"github.com/tucnak/climax"
)

var defaultUser, defaultPass = "user", "pa55word"

// GenKey gets a crypto-random number and encodes it in hex for generated shared credentials
func GenKey() string {
	k, _ := rand.Int(rand.Reader, big.NewInt(int64(^uint32(0))))
	key := k.Uint64()
	return fmt.Sprintf("%0x", key)
}

// EnsureDir checks a file could be written to a path, creates the directories as needed
func EnsureDir(fileName string) {
	dirName := filepath.Dir(fileName)
	if _, serr := os.Stat(dirName); serr != nil {
		merr := os.MkdirAll(dirName, os.ModePerm)
		if merr != nil {
			panic(merr)
		}
	}
}

// NormalizeAddress reads and corrects an address if it is missing pieces
func NormalizeAddress(addr, defaultPort string, out *string) {
	o := node.NormalizeAddress(addr, defaultPort)
	_, _, err := net.ParseCIDR(o)
	if err != nil {
		ip := net.ParseIP(addr)
		if ip != nil {
			out = &o
		}
	} else {
		out = &o
	}
}

// NormalizeAddresses reads and collects a space separated list of addresses contained in a string
func NormalizeAddresses(addrs string, defaultPort string, out *[]string) {
	O := new([]string)
	addrS := strings.Split(addrs, " ")
	for i := range addrS {
		a := addrS[i]
		// o := ""
		NormalizeAddress(a, defaultPort, &a)
		if a != "" {
			*O = append(*O, a)
		}
	}
	// atomically switch out if there was valid addresses
	if len(*O) > 0 {
		*out = *O
	}
}

// ParseInteger reads a string that should contain a integer and returns the number and any parsing error
func ParseInteger(integer, name string, original *int) (err error) {
	var out int
	out, err = strconv.Atoi(integer)
	if err != nil {
		err = fmt.Errorf("malformed %s `%s` leaving set at `%d` err: %s", name, integer, *original, err.Error())
	} else {
		*original = out
	}
	return
}

// ParseUint32 reads a string that should contain a integer and returns the number and any parsing error
func ParseUint32(integer, name string, original *uint32) (err error) {
	var out int
	out, err = strconv.Atoi(integer)
	if err != nil {
		err = fmt.Errorf("malformed %s `%s` leaving set at `%d` err: %s", name, integer, *original, err.Error())
	} else {
		*original = uint32(out)
	}
	return
}

// ParseFloat reads a string that should contain a floating point number and returns it and any parsing error
func ParseFloat(f, name string, original *float64) (err error) {
	var out float64
	_, err = fmt.Sscanf(f, "%0.f", out)
	if err != nil {
		err = fmt.Errorf("malformed %s `%s` leaving set at `%0.f` err: %s", name, f, *original, err.Error())
	} else {
		*original = out
	}
	return
}

// ParseDuration takes a string of the format `Xd/h/m/s` and returns a time.Duration corresponding with that specification
func ParseDuration(d, name string, out *time.Duration) (err error) {
	var t int
	var ti time.Duration
	switch d[len(d)-1] {
	case 's':
		t, err = strconv.Atoi(d[:len(d)-1])
		ti = time.Duration(t) * time.Second
	case 'm':
		t, err = strconv.Atoi(d[:len(d)-1])
		ti = time.Duration(t) * time.Minute
	case 'h':
		t, err = strconv.Atoi(d[:len(d)-1])
		ti = time.Duration(t) * time.Hour
	case 'd':
		t, err = strconv.Atoi(d[:len(d)-1])
		ti = time.Duration(t) * 24 * time.Hour
	}
	if err != nil {
		err = fmt.Errorf("malformed %s `%s` leaving set at `%s` err: %s", name, d, *out, err.Error())
	} else {
		*out = ti
	}
	return
}

// GenFlag allows a flag to be more concisely declared
func GenFlag(name, usage, help string) climax.Flag {
	return climax.Flag{
		Name:     name,
		Usage:    "--" + name + `="` + usage + `"`,
		Help:     help,
		Variable: true,
	}
}

// GenTrig is a short declaration for a trigger type
func GenTrig(name, short, help string) climax.Flag {
	return climax.Flag{
		Name:     name,
		Short:    short,
		Help:     help,
		Variable: false,
	}
}

// GenShort is a short declaration for a variable with a short version
func GenShort(name, short, usage, help string) climax.Flag {
	return climax.Flag{
		Name:     name,
		Short:    short,
		Usage:    "--" + name + `="` + usage + `"`,
		Help:     help,
		Variable: true,
	}
}

// GenLog is a short declaration for a variable with a short version
func GenLog(name string) climax.Flag {
	return climax.Flag{
		Name:     name,
		Usage:    "--" + name + `="info"`,
		Variable: true,
	}
}

// CheckCreateDir checks that the path exists and is a directory.
// If path does not exist, it is created.
func CheckCreateDir(path string) error {
	if fi, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			// Attempt data directory creation
			if err = os.MkdirAll(path, 0700); err != nil {
				return fmt.Errorf("cannot create directory: %s", err)
			}
		} else {
			return fmt.Errorf("error checking directory: %s", err)
		}
	} else {
		if !fi.IsDir() {
			return fmt.Errorf("path '%s' is not a directory", path)
		}
	}
	return nil
}

// minUint32 is a helper function to return the minimum of two uint32s. This avoids a math import and the need to cast to floats.
func minUint32(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

// FileExists reports whether the named file or directory exists.
func FileExists(filePath string) (bool, error) {
	_, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func getIfIs(ctx *climax.Context, name string) (out string, ok bool) {
	if ctx.Is(name) {
		return ctx.Get(name)
	}
	return
}
package app

// Stamp is the version number placed by
var Stamp string

// Name is the name of the application
const Name = "pod"

// Version returns the application version as a properly formed string per the semantic versioning 2.0.0 spec (http://semver.org/).
func Version() string {
	return Name + "-" + Stamp
}