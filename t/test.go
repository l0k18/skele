package node
import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"git.parallelcoin.io/pod/cmd/node/mempool"
	blockchain "git.parallelcoin.io/pod/pkg/chain"
	"git.parallelcoin.io/pod/pkg/chaincfg"
	"git.parallelcoin.io/pod/pkg/chaincfg/chainhash"
	cl "git.parallelcoin.io/pod/pkg/clog"
	"git.parallelcoin.io/pod/pkg/connmgr"
	database "git.parallelcoin.io/pod/pkg/db"
	_ "git.parallelcoin.io/pod/pkg/db/ffldb"
	"git.parallelcoin.io/pod/pkg/fork"
	"git.parallelcoin.io/pod/pkg/peer"
	"git.parallelcoin.io/pod/pkg/util"
	"github.com/btcsuite/go-socks/socks"
	flags "github.com/jessevdk/go-flags"
)
const (
	DefaultConfigFilename        = "conf.json"
	DefaultDataDirname           = "node"
	DefaultLogLevel              = "info"
	DefaultLogDirname            = "node"
	DefaultLogFilename           = "log"
	DefaultAddress               = "127.0.0.1"
	DefaultPort                  = "11047"
	DefaultRPCPort               = "11048"
	DefalutRPCAddr               = "127.0.0.1"
	DefaultRPCServer             = "127.0.0.1:11048"
	DefaultListener              = "127.0.0.1:11047"
	DefaultRPCListener           = "127.0.0.1:11048"
	DefaultMaxPeers              = 125
	DefaultBanDuration           = time.Hour * 24
	DefaultBanThreshold          = 100
	DefaultConnectTimeout        = time.Second * 30
	DefaultMaxRPCClients         = 10
	DefaultMaxRPCWebsockets      = 25
	DefaultMaxRPCConcurrentReqs  = 20
	DefaultDbType                = "ffldb"
	DefaultFreeTxRelayLimit      = 15.0
	DefaultTrickleInterval       = peer.DefaultTrickleInterval
	DefaultBlockMinSize          = 80
	DefaultBlockMaxSize          = 200000
	DefaultBlockMinWeight        = 10
	DefaultBlockMaxWeight        = 3000000
	BlockMaxSizeMin              = 1000
	BlockMaxSizeMax              = blockchain.MaxBlockBaseSize - 1000
	BlockMaxWeightMin            = 4000
	BlockMaxWeightMax            = blockchain.MaxBlockWeight - 4000
	DefaultGenerate              = false
	DefaultGenThreads            = 1
	DefaultMinerListener         = "127.0.0.1:11011"
	DefaultMaxOrphanTransactions = 100
	DefaultMaxOrphanTxSize       = 100000
	DefaultSigCacheMaxSize       = 100000
	// These are set to default on because more often one wants them than not
	DefaultTxIndex   = true
	DefaultAddrIndex = true
	DefaultAlgo      = "random"
)
var (
	DefaultHomeDir    = util.AppDataDir("pod", false)
	DefaultConfigFile = filepath.Join(
		DefaultHomeDir, DefaultConfigFilename)
	DefaultDataDir     = filepath.Join(DefaultHomeDir, DefaultDataDirname)
	KnownDbTypes       = database.SupportedDrivers()
	DefaultRPCKeyFile  = filepath.Join(DefaultHomeDir, "rpc.key")
	DefaultRPCCertFile = filepath.Join(DefaultHomeDir, "rpc.cert")
	DefaultLogDir      = filepath.Join(DefaultHomeDir, DefaultLogDirname)
)
// runServiceCommand is only set to a real function on Windows.  It is used to parse and execute service commands specified via the -s flag.
var runServiceCommand func(string) error
// minUint32 is a helper function to return the minimum of two uint32s. This avoids a math import and the need to cast to floats.
func minUint32(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}
// Config defines the configuration options for pod. See loadConfig for details on the configuration load process.
type Config struct {
	ShowVersion          bool          `short:"V" long:"version" description:"Display version information and exit"`
	ConfigFile           string        `short:"C" long:"configfile" description:"Path to configuration file"`
	DataDir              string        `short:"b" long:"datadir" description:"Directory to store data"`
	LogDir               string        `long:"logdir" description:"Directory to log output."`
	DebugLevel           string        `long:"debuglevel" description:"baseline debug level for all subsystems unless specified"`
	AddPeers             []string      `short:"a" long:"addpeer" description:"Add a peer to connect with at startup"`
	ConnectPeers         []string      `long:"connect" description:"Connect only to the specified peers at startup"`
	DisableListen        bool          `long:"nolisten" description:"Disable listening for incoming connections -- NOTE: Listening is automatically disabled if the --connect or --proxy options are used without also specifying listen interfaces via --listen"`
	Listeners            []string      `long:"listen" description:"Add an interface/port to listen for connections (default all interfaces port: 11047, testnet: 21047)"`
	MaxPeers             int           `long:"maxpeers" description:"Max number of inbound and outbound peers"`
	DisableBanning       bool          `long:"nobanning" description:"Disable banning of misbehaving peers"`
	BanDuration          time.Duration `long:"banduration" description:"How long to ban misbehaving peers.  Valid time units are {s, m, h, d}.  Minimum 1 second"`
	BanThreshold         uint32        `long:"banthreshold" description:"Maximum allowed ban score before disconnecting and banning misbehaving peers."`
	Whitelists           []string      `long:"whitelist" description:"Add an IP network or IP that will not be banned. (eg. 192.168.1.0/24 or ::1)"`
	RPCUser              string        `short:"u" long:"rpcuser" description:"Username for RPC connections"`
	RPCPass              string        `short:"P" long:"rpcpass" default-mask:"-" description:"Password for RPC connections"`
	RPCLimitUser         string        `long:"rpclimituser" description:"Username for limited RPC connections"`
	RPCLimitPass         string        `long:"rpclimitpass" default-mask:"-" description:"Password for limited RPC connections"`
	RPCListeners         []string      `long:"rpclisten" description:"Add an interface/port to listen for RPC connections (default port: 11048, testnet: 21048) gives sha256d block templates"`
	RPCCert              string        `long:"rpccert" description:"File containing the certificate file"`
	RPCKey               string        `long:"rpckey" description:"File containing the certificate key"`
	RPCMaxClients        int           `long:"rpcmaxclients" description:"Max number of RPC clients for standard connections"`
	RPCMaxWebsockets     int           `long:"rpcmaxwebsockets" description:"Max number of RPC websocket connections"`
	RPCMaxConcurrentReqs int           `long:"rpcmaxconcurrentreqs" description:"Max number of concurrent RPC requests that may be processed concurrently"`
	RPCQuirks            bool          `long:"rpcquirks" description:"Mirror some JSON-RPC quirks of Bitcoin Core -- NOTE: Discouraged unless interoperability issues need to be worked around"`
	DisableRPC           bool          `long:"norpc" description:"Disable built-in RPC server -- NOTE: The RPC server is disabled by default if no rpcuser/rpcpass or rpclimituser/rpclimitpass is specified"`
	TLS                  bool          `long:"tls" description:"Enable TLS for the RPC server"`
	DisableDNSSeed       bool          `long:"nodnsseed" description:"Disable DNS seeding for peers"`
	ExternalIPs          []string      `long:"externalip" description:"Add an ip to the list of local addresses we claim to listen on to peers"`
	Proxy                string        `long:"proxy" description:"Connect via SOCKS5 proxy (eg. 127.0.0.1:9050)"`
	ProxyUser            string        `long:"proxyuser" description:"Username for proxy server"`
	ProxyPass            string        `long:"proxypass" default-mask:"-" description:"Password for proxy server"`
	OnionProxy           string        `long:"onion" description:"Connect to tor hidden services via SOCKS5 proxy (eg. 127.0.0.1:9050)"`
	OnionProxyUser       string        `long:"onionuser" description:"Username for onion proxy server"`
	OnionProxyPass       string        `long:"onionpass" default-mask:"-" description:"Password for onion proxy server"`
	NoOnion              bool          `long:"noonion" description:"Disable connecting to tor hidden services"`
	TorIsolation         bool          `long:"torisolation" description:"Enable Tor stream isolation by randomizing user credentials for each connection."`
	TestNet3             bool          `long:"testnet" description:"Use the test network"`
	RegressionTest       bool          `long:"regtest" description:"Use the regression test network"`
	SimNet               bool          `long:"simnet" description:"Use the simulation test network"`
	AddCheckpoints       []string      `long:"addcheckpoint" description:"Add a custom checkpoint.  Format: '<height>:<hash>'"`
	DisableCheckpoints   bool          `long:"nocheckpoints" description:"Disable built-in checkpoints.  Don't do this unless you know what you're doing."`
	DbType               string        `long:"dbtype" description:"Database backend to use for the Block Chain"`
	Profile              string        `long:"profile" description:"Enable HTTP profiling on given port -- NOTE port must be between 1024 and 65536"`
	CPUProfile           string        `long:"cpuprofile" description:"Write CPU profile to the specified file"`
	Upnp                 bool          `long:"upnp" description:"Use UPnP to map our listening port outside of NAT"`
	MinRelayTxFee        float64       `long:"minrelaytxfee" description:"The minimum transaction fee in DUO/kB to be considered a non-zero fee."`
	FreeTxRelayLimit     float64       `long:"limitfreerelay" description:"Limit relay of transactions with no transaction fee to the given amount in thousands of bytes per minute"`
	NoRelayPriority      bool          `long:"norelaypriority" description:"Do not require free or low-fee transactions to have high priority for relaying"`
	TrickleInterval      time.Duration `long:"trickleinterval" description:"Minimum time between attempts to send new inventory to a connected peer"`
	MaxOrphanTxs         int           `long:"maxorphantx" description:"Max number of orphan transactions to keep in memory"`
	Algo                 string        `long:"algo" description:"Sets the algorithm for the CPU miner ( blake14lr, cryptonight7v2, keccak, lyra2rev2, scrypt, sha256d, stribog, skein, x11 default is 'random')"`
	Generate             bool          `long:"generate" description:"Generate (mine) bitcoins using the CPU"`
	GenThreads           int32         `long:"genthreads" description:"Number of CPU threads to use with CPU miner -1 = all cores"`
	MiningAddrs          []string      `long:"miningaddr" description:"Add the specified payment address to the list of addresses to use for generated blocks, at least one is required if generate or minerport are set"`
	MinerListener        string        `long:"minerlistener" description:"listen address for miner controller"`
	MinerPass            string        `long:"minerpass" description:"Encryption password required for miner clients to subscribe to work updates, for use over insecure connections"`
	BlockMinSize         uint32        `long:"blockminsize" description:"Mininum block size in bytes to be used when creating a block"`
	BlockMaxSize         uint32        `long:"blockmaxsize" description:"Maximum block size in bytes to be used when creating a block"`
	BlockMinWeight       uint32        `long:"blockminweight" description:"Mininum block weight to be used when creating a block"`
	BlockMaxWeight       uint32        `long:"blockmaxweight" description:"Maximum block weight to be used when creating a block"`
	BlockPrioritySize    uint32        `long:"blockprioritysize" description:"Size in bytes for high-priority/low-fee transactions when creating a block"`
	UserAgentComments    []string      `long:"uacomment" description:"Comment to add to the user agent -- See BIP 14 for more information."`
	NoPeerBloomFilters   bool          `long:"nopeerbloomfilters" description:"Disable bloom filtering support"`
	NoCFilters           bool          `long:"nocfilters" description:"Disable committed filtering (CF) support"`
	DropCfIndex          bool          `long:"dropcfindex" description:"Deletes the index used for committed filtering (CF) support from the database on start up and then exits."`
	SigCacheMaxSize      uint          `long:"sigcachemaxsize" description:"The maximum number of entries in the signature verification cache"`
	BlocksOnly           bool          `long:"blocksonly" description:"Do not accept transactions from remote peers."`
	TxIndex              bool          `long:"txindex" description:"Maintain a full hash-based transaction index which makes all transactions available via the getrawtransaction RPC"`
	DropTxIndex          bool          `long:"droptxindex" description:"Deletes the hash-based transaction index from the database on start up and then exits."`
	AddrIndex            bool          `long:"addrindex" description:"Maintain a full address-based transaction index which makes the searchrawtransactions RPC available"`
	DropAddrIndex        bool          `long:"dropaddrindex" description:"Deletes the address-based transaction index from the database on start up and then exits."`
	RelayNonStd          bool          `long:"relaynonstd" description:"Relay non-standard transactions regardless of the default settings for the active network."`
	RejectNonStd         bool          `long:"rejectnonstd" description:"Reject non-standard transactions regardless of the default settings for the active network."`
}
// StateConfig stores current state of the node
type StateConfig struct {
	Lookup              func(string) ([]net.IP, error)
	Oniondial           func(string, string, time.Duration) (net.Conn, error)
	Dial                func(string, string, time.Duration) (net.Conn, error)
	AddedCheckpoints    []chaincfg.Checkpoint
	ActiveMiningAddrs   []util.Address
	ActiveMinerKey      []byte
	ActiveMinRelayTxFee util.Amount
	ActiveWhitelists    []*net.IPNet
}
// serviceOptions defines the configuration options for the daemon as a service on Windows.
type serviceOptions struct {
	ServiceCommand string `short:"s" long:"service" description:"Service command {install, remove, start, stop}"`
}
// CleanAndExpandPath expands environment variables and leading ~ in the passed path, cleans the result, and returns it.
func CleanAndExpandPath(path string) string {
	// Expand initial ~ to OS specific home directory.
	if strings.HasPrefix(path, "~") {
		homeDir := filepath.Dir(DefaultHomeDir)
		path = strings.Replace(path, "~", homeDir, 1)
	}
	// NOTE: The os.ExpandEnv doesn't work with Windows-style %VARIABLE%, but they variables can still be expanded via POSIX-style $VARIABLE.
	return filepath.Clean(os.ExpandEnv(path))
}
// ValidLogLevel returns whether or not logLevel is a valid debug log level.
func ValidLogLevel(logLevel string) bool {
	switch logLevel {
	case "trace":
		fallthrough
	case "debug":
		fallthrough
	case "info":
		fallthrough
	case "warn":
		fallthrough
	case "error":
		fallthrough
	case "critical":
		return true
	}
	return false
}
// ValidDbType returns whether or not dbType is a supported database type.
func ValidDbType(dbType string) bool {
	for _, knownType := range KnownDbTypes {
		if dbType == knownType {
			return true
		}
	}
	return false
}
// RemoveDuplicateAddresses returns a new slice with all duplicate entries in addrs removed.
func RemoveDuplicateAddresses(addrs []string) []string {
	result := make([]string, 0, len(addrs))
	seen := map[string]struct{}{}
	for _, val := range addrs {
		if _, ok := seen[val]; !ok {
			result = append(result, val)
			seen[val] = struct{}{}
		}
	}
	return result
}
// NormalizeAddress returns addr with the passed default port appended if there is not already a port specified.
func NormalizeAddress(addr, defaultPort string) string {
	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		return net.JoinHostPort(addr, defaultPort)
	}
	return addr
}
// NormalizeAddresses returns a new slice with all the passed peer addresses normalized with the given default port, and all duplicates removed.
func NormalizeAddresses(addrs []string, defaultPort string) []string {
	for i, addr := range addrs {
		addrs[i] = NormalizeAddress(addr, defaultPort)
	}
	return RemoveDuplicateAddresses(addrs)
}
// NewCheckpointFromStr parses checkpoints in the '<height>:<hash>' format.
func NewCheckpointFromStr(checkpoint string) (chaincfg.Checkpoint, error) {
	parts := strings.Split(checkpoint, ":")
	if len(parts) != 2 {
		return chaincfg.Checkpoint{}, fmt.Errorf("unable to parse "+
			"checkpoint %q -- use the syntax <height>:<hash>",
			checkpoint)
	}
	height, err := strconv.ParseInt(parts[0], 10, 32)
	if err != nil {
		return chaincfg.Checkpoint{}, fmt.Errorf("unable to parse "+
			"checkpoint %q due to malformed height", checkpoint)
	}
	if len(parts[1]) == 0 {
		return chaincfg.Checkpoint{}, fmt.Errorf("unable to parse "+
			"checkpoint %q due to missing hash", checkpoint)
	}
	hash, err := chainhash.NewHashFromStr(parts[1])
	if err != nil {
		return chaincfg.Checkpoint{}, fmt.Errorf("unable to parse "+
			"checkpoint %q due to malformed hash", checkpoint)
	}
	return chaincfg.Checkpoint{
		Height: int32(height),
		Hash:   hash,
	}, nil
}
// ParseCheckpoints checks the checkpoint strings for valid syntax ('<height>:<hash>') and parses them to chaincfg.Checkpoint instances.
func ParseCheckpoints(checkpointStrings []string) ([]chaincfg.Checkpoint, error) {
	if len(checkpointStrings) == 0 {
		return nil, nil
	}
	checkpoints := make([]chaincfg.Checkpoint, len(checkpointStrings))
	for i, cpString := range checkpointStrings {
		checkpoint, err := NewCheckpointFromStr(cpString)
		if err != nil {
			return nil, err
		}
		checkpoints[i] = checkpoint
	}
	return checkpoints, nil
}
// FileExists reports whether the named file or directory exists.
func FileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}
// NewConfigParser returns a new command line flags parser.
func NewConfigParser(cfg *Config, so *serviceOptions, options flags.Options) *flags.Parser {
	parser := flags.NewParser(cfg, options)
	if runtime.GOOS == "windows" {
		parser.AddGroup("Service Options", "Service Options", so)
	}
	return parser
}
// loadConfig initializes and parses the config using a config file and command line options.
// The configuration proceeds as follows:
// 	1) Start with a default config with sane settings
// 	2) Pre-parse the command line to check for an alternative config file
// 	3) Load configuration file overwriting defaults with any specified options
// 	4) Parse CLI options and overwrite/add any specified options
// The above results in pod functioning properly without any config settings while still allowing the user to override settings with config files and command line options.  Command line options always take precedence.
func loadConfig() (*Config, []string, error) {
	// Default config.
	cfg := Config{
		ConfigFile:           DefaultConfigFile,
		MaxPeers:             DefaultMaxPeers,
		BanDuration:          DefaultBanDuration,
		BanThreshold:         DefaultBanThreshold,
		RPCMaxClients:        DefaultMaxRPCClients,
		RPCMaxWebsockets:     DefaultMaxRPCWebsockets,
		RPCMaxConcurrentReqs: DefaultMaxRPCConcurrentReqs,
		DataDir:              DefaultDataDir,
		LogDir:               DefaultLogDir,
		DbType:               DefaultDbType,
		RPCKey:               DefaultRPCKeyFile,
		RPCCert:              DefaultRPCCertFile,
		MinRelayTxFee:        mempool.DefaultMinRelayTxFee.ToDUO(),
		FreeTxRelayLimit:     DefaultFreeTxRelayLimit,
		TrickleInterval:      DefaultTrickleInterval,
		BlockMinSize:         DefaultBlockMinSize,
		BlockMaxSize:         DefaultBlockMaxSize,
		BlockMinWeight:       DefaultBlockMinWeight,
		BlockMaxWeight:       DefaultBlockMaxWeight,
		BlockPrioritySize:    mempool.DefaultBlockPrioritySize,
		MaxOrphanTxs:         DefaultMaxOrphanTransactions,
		SigCacheMaxSize:      DefaultSigCacheMaxSize,
		Generate:             DefaultGenerate,
		GenThreads:           1,
		TxIndex:              DefaultTxIndex,
		AddrIndex:            DefaultAddrIndex,
		Algo:                 DefaultAlgo,
	}
	// Service options which are only added on Windows.
	serviceOpts := serviceOptions{}
	// Pre-parse the command line options to see if an alternative config file or the version flag was specified.  Any errors aside from the help message error can be ignored here since they will be caught by the final parse below.
	preCfg := cfg
	preParser := NewConfigParser(&preCfg, &serviceOpts, flags.HelpFlag)
	_, err := preParser.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			fmt.Fprintln(os.Stderr, err)
			return nil, nil, err
		}
	}
	// Show the version and exit if the version flag was specified.
	appName := filepath.Base(os.Args[0])
	appName = strings.TrimSuffix(appName, filepath.Ext(appName))
	usageMessage := fmt.Sprintf("Use %s -h to show usage", appName)
	if preCfg.ShowVersion {
		fmt.Println(appName, "version", Version())
		os.Exit(0)
	}
	// Perform service command and exit if specified.  Invalid service commands show an appropriate error.  Only runs on Windows since the runServiceCommand function will be nil when not on Windows.
	if serviceOpts.ServiceCommand != "" && runServiceCommand != nil {
		err := runServiceCommand(serviceOpts.ServiceCommand)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(0)
	}
	// Load additional config from file.
	var configFileError error
	parser := NewConfigParser(&cfg, &serviceOpts, flags.Default)
	if !(preCfg.RegressionTest || preCfg.SimNet) || preCfg.ConfigFile !=
		DefaultConfigFile {
		if _, err := os.Stat(preCfg.ConfigFile); os.IsNotExist(err) {
			err := createDefaultConfigFile(preCfg.ConfigFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating a "+
					"default config file: %v\n", err)
			}
		}
		err := flags.NewIniParser(parser).ParseFile(preCfg.ConfigFile)
		if err != nil {
			if _, ok := err.(*os.PathError); !ok {
				fmt.Fprintf(os.Stderr, "Error parsing config "+
					"file: %v\n", err)
				fmt.Fprintln(os.Stderr, usageMessage)
				return nil, nil, err
			}
			configFileError = err
		}
	}
	// Don't add peers from the config file when in regression test mode.
	if preCfg.RegressionTest && len(cfg.AddPeers) > 0 {
		cfg.AddPeers = nil
	}
	// Parse command line options again to ensure they take precedence.
	remainingArgs, err := parser.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); !ok || e.Type != flags.ErrHelp {
			fmt.Fprintln(os.Stderr, usageMessage)
		}
		return nil, nil, err
	}
	// Create the home directory if it doesn't already exist.
	funcName := "loadConfig"
	err = os.MkdirAll(DefaultHomeDir, 0700)
	if err != nil {
		// Show a nicer error message if it's because a symlink is linked to a directory that does not exist (probably because it's not mounted).
		if e, ok := err.(*os.PathError); ok && os.IsExist(err) {
			if link, lerr := os.Readlink(e.Path); lerr == nil {
				str := "is symlink %s -> %s mounted?"
				err = fmt.Errorf(str, e.Path, link)
			}
		}
		str := "%s: Failed to create home directory: %v"
		err := fmt.Errorf(str, funcName, err)
		fmt.Fprintln(os.Stderr, err)
		return nil, nil, err
	}
	// Multiple networks can't be selected simultaneously.
	numNets := 0
	// Count number of network flags passed; assign active network params while we're at it
	if cfg.TestNet3 {
		numNets++
		ActiveNetParams = &TestNet3Params
		fork.IsTestnet = true
	}
	if cfg.RegressionTest {
		numNets++
		ActiveNetParams = &RegressionNetParams
		fork.IsTestnet = true
	}
	if cfg.SimNet {
		numNets++
		// Also disable dns seeding on the simulation test network.
		ActiveNetParams = &SimNetParams
		cfg.DisableDNSSeed = true
		fork.IsTestnet = true
	}
	if numNets > 1 {
		str := "%s: The testnet, regtest, segnet, and simnet params " +
			"can't be used together -- choose one of the four"
		err := fmt.Errorf(str, funcName)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}
	// Set the mining algorithm correctly, default to sha256d if unrecognised
	switch cfg.Algo {
	case "blake14lr", "cryptonight7v2", "keccak", "lyra2rev2", "scrypt", "skein", "x11", "stribog", "random", "easy":
	default:
		cfg.Algo = "sha256d"
	}
	// Set the default policy for relaying non-standard transactions according to the default of the active network. The set configuration value takes precedence over the default value for the selected network.
	relayNonStd := ActiveNetParams.RelayNonStdTxs
	switch {
	case cfg.RelayNonStd && cfg.RejectNonStd:
		str := "%s: rejectnonstd and relaynonstd cannot be used " +
			"together -- choose only one"
		err := fmt.Errorf(str, funcName)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	case cfg.RejectNonStd:
		relayNonStd = false
	case cfg.RelayNonStd:
		relayNonStd = true
	}
	cfg.RelayNonStd = relayNonStd
	// Append the network type to the data directory so it is "namespaced" per network.  In addition to the block database, there are other pieces of data that are saved to disk such as address manager state. All data is specific to a network, so namespacing the data directory means each individual piece of serialized data does not have to worry about changing names per network and such.
	cfg.DataDir = CleanAndExpandPath(cfg.DataDir)
	cfg.DataDir = filepath.Join(cfg.DataDir, NetName(ActiveNetParams))
	// Append the network type to the log directory so it is "namespaced" per network in the same fashion as the data directory.
	cfg.LogDir = CleanAndExpandPath(cfg.LogDir)
	cfg.LogDir = filepath.Join(cfg.LogDir, NetName(ActiveNetParams))
	// Initialize log rotation.  After log rotation has been initialized, the logger variables may be used.
	// initLogRotator(filepath.Join(cfg.LogDir, DefaultLogFilename))
	// Validate database type.
	if !ValidDbType(cfg.DbType) {
		str := "%s: The specified database type [%v] is invalid -- supported types %v"
		err := fmt.Errorf(str, funcName, cfg.DbType, KnownDbTypes)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}
	// Validate profile port number
	if cfg.Profile != "" {
		log <- cl.Trace{"profiling to", cfg.Profile}
		profilePort, err := strconv.Atoi(cfg.Profile)
		if err != nil || profilePort < 1024 || profilePort > 65535 {
			str := "%s: The profile port must be between 1024 and 65535"
			err := fmt.Errorf(str, funcName)
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, usageMessage)
			return nil, nil, err
		}
	}
	// Don't allow ban durations that are too short.
	if cfg.BanDuration < time.Second {
		str := "%s: The banduration option may not be less than 1s -- parsed [%v]"
		err := fmt.Errorf(str, funcName, cfg.BanDuration)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}
	// Validate any given whitelisted IP addresses and networks.
	if len(StateCfg.ActiveWhitelists) > 0 {
		var ip net.IP
		StateCfg.ActiveWhitelists = make([]*net.IPNet, 0, len(StateCfg.ActiveWhitelists))
		for _, addr := range cfg.Whitelists {
			_, ipnet, err := net.ParseCIDR(addr)
			if err != nil {
				ip = net.ParseIP(addr)
				if ip == nil {
					str := "%s: The whitelist value of '%s' is invalid"
					err = fmt.Errorf(str, funcName, addr)
					fmt.Fprintln(os.Stderr, err)
					fmt.Fprintln(os.Stderr, usageMessage)
					return nil, nil, err
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
	if len(cfg.AddPeers) > 0 && len(cfg.ConnectPeers) > 0 {
		str := "%s: the --addpeer and --connect options can not be " +
			"mixed"
		err := fmt.Errorf(str, funcName)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}
	// --proxy or --connect without --listen disables listening.
	if (cfg.Proxy != "" || len(cfg.ConnectPeers) > 0) &&
		len(cfg.Listeners) == 0 {
		cfg.DisableListen = true
	}
	// Connect means no DNS seeding.
	if len(cfg.ConnectPeers) > 0 {
		cfg.DisableDNSSeed = true
	}
	// Add the default listener if none were specified. The default listener is all addresses on the listen port for the network we are to connect to.
	if len(cfg.Listeners) == 0 {
		cfg.Listeners = []string{
			net.JoinHostPort("", ActiveNetParams.DefaultPort),
		}
	}
	// Check to make sure limited and admin users don't have the same username
	if cfg.RPCUser == cfg.RPCLimitUser && cfg.RPCUser != "" {
		str := "%s: --rpcuser and --rpclimituser must not specify the " +
			"same username"
		err := fmt.Errorf(str, funcName)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}
	// Check to make sure limited and admin users don't have the same password
	if cfg.RPCPass == cfg.RPCLimitPass && cfg.RPCPass != "" {
		str := "%s: --rpcpass and --rpclimitpass must not specify the " +
			"same password"
		err := fmt.Errorf(str, funcName)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}
	// The RPC server is disabled if no username or password is provided.
	if (cfg.RPCUser == "" || cfg.RPCPass == "") &&
		(cfg.RPCLimitUser == "" || cfg.RPCLimitPass == "") {
		cfg.DisableRPC = true
	}
	if cfg.DisableRPC {
		log <- cl.Inf("RPC service is disabled")
		// Default RPC to listen on localhost only.
		if !cfg.DisableRPC && len(cfg.RPCListeners) == 0 {
			addrs, err := net.LookupHost("127.0.0.1:11048")
			if err != nil {
				return nil, nil, err
			}
			cfg.RPCListeners = make([]string, 0, len(addrs))
			for _, addr := range addrs {
				addr = net.JoinHostPort(addr, ActiveNetParams.RPCPort)
				cfg.RPCListeners = append(cfg.RPCListeners, addr)
			}
		}
		if cfg.RPCMaxConcurrentReqs < 0 {
			str := "%s: The rpcmaxwebsocketconcurrentrequests option may not be less than 0 -- parsed [%d]"
			err := fmt.Errorf(str, funcName, cfg.RPCMaxConcurrentReqs)
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, usageMessage)
			return nil, nil, err
		}
	}
	// Validate the the minrelaytxfee.
	StateCfg.ActiveMinRelayTxFee, err = util.NewAmount(cfg.MinRelayTxFee)
	if err != nil {
		str := "%s: invalid minrelaytxfee: %v"
		err := fmt.Errorf(str, funcName, err)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}
	// Limit the max block size to a sane value.
	if cfg.BlockMaxSize < BlockMaxSizeMin || cfg.BlockMaxSize >
		BlockMaxSizeMax {
		str := "%s: The blockmaxsize option must be in between %d " +
			"and %d -- parsed [%d]"
		err := fmt.Errorf(str, funcName, BlockMaxSizeMin,
			BlockMaxSizeMax, cfg.BlockMaxSize)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}
	// Limit the max block weight to a sane value.
	if cfg.BlockMaxWeight < BlockMaxWeightMin ||
		cfg.BlockMaxWeight > BlockMaxWeightMax {
		str := "%s: The blockmaxweight option must be in between %d " +
			"and %d -- parsed [%d]"
		err := fmt.Errorf(str, funcName, BlockMaxWeightMin,
			BlockMaxWeightMax, cfg.BlockMaxWeight)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}
	// Limit the max orphan count to a sane vlue.
	if cfg.MaxOrphanTxs < 0 {
		str := "%s: The maxorphantx option may not be less than 0 " +
			"-- parsed [%d]"
		err := fmt.Errorf(str, funcName, cfg.MaxOrphanTxs)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}
	// Limit the block priority and minimum block sizes to max block size.
	cfg.BlockPrioritySize = minUint32(cfg.BlockPrioritySize, cfg.BlockMaxSize)
	cfg.BlockMinSize = minUint32(cfg.BlockMinSize, cfg.BlockMaxSize)
	cfg.BlockMinWeight = minUint32(cfg.BlockMinWeight, cfg.BlockMaxWeight)
	switch {
	// If the max block size isn't set, but the max weight is, then we'll set the limit for the max block size to a safe limit so weight takes precedence.
	case cfg.BlockMaxSize == DefaultBlockMaxSize &&
		cfg.BlockMaxWeight != DefaultBlockMaxWeight:
		cfg.BlockMaxSize = blockchain.MaxBlockBaseSize - 1000
	// If the max block weight isn't set, but the block size is, then we'll scale the set weight accordingly based on the max block size value.
	case cfg.BlockMaxSize != DefaultBlockMaxSize &&
		cfg.BlockMaxWeight == DefaultBlockMaxWeight:
		cfg.BlockMaxWeight = cfg.BlockMaxSize * blockchain.WitnessScaleFactor
	}
	// Look for illegal characters in the user agent comments.
	for _, uaComment := range cfg.UserAgentComments {
		if strings.ContainsAny(uaComment, "/:()") {
			err := fmt.Errorf("%s: The following characters must not "+
				"appear in user agent comments: '/', ':', '(', ')'",
				funcName)
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, usageMessage)
			return nil, nil, err
		}
	}
	// --txindex and --droptxindex do not mix.
	if cfg.TxIndex && cfg.DropTxIndex {
		err := fmt.Errorf("%s: the --txindex and --droptxindex "+
			"options may  not be activated at the same time",
			funcName)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}
	// --addrindex and --dropaddrindex do not mix.
	if cfg.AddrIndex && cfg.DropAddrIndex {
		err := fmt.Errorf("%s: the --addrindex and --dropaddrindex "+
			"options may not be activated at the same time",
			funcName)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}
	// --addrindex and --droptxindex do not mix.
	if cfg.AddrIndex && cfg.DropTxIndex {
		err := fmt.Errorf("%s: the --addrindex and --droptxindex options may not be activated at the same time because the address index relies on the transaction index", funcName)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}
	// Check mining addresses are valid and saved parsed versions.
	StateCfg.ActiveMiningAddrs = make([]util.Address, 0, len(cfg.MiningAddrs))
	for _, strAddr := range cfg.MiningAddrs {
		addr, err := util.DecodeAddress(strAddr, ActiveNetParams.Params)
		if err != nil {
			str := "%s: mining address '%s' failed to decode: %v"
			err := fmt.Errorf(str, funcName, strAddr, err)
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, usageMessage)
			return nil, nil, err
		}
		if !addr.IsForNet(ActiveNetParams.Params) {
			str := "%s: mining address '%s' is on the wrong network"
			err := fmt.Errorf(str, funcName, strAddr)
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, usageMessage)
			return nil, nil, err
		}
		StateCfg.ActiveMiningAddrs = append(StateCfg.ActiveMiningAddrs, addr)
	}
	// Ensure there is at least one mining address when the generate flag is set.
	if (cfg.Generate || cfg.MinerListener != "") && len(cfg.MiningAddrs) == 0 {
		str := "%s: the generate flag is set, but there are no mining addresses specified "
		err := fmt.Errorf(str, funcName)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}
	if cfg.MinerPass != "" {
		StateCfg.ActiveMinerKey = fork.Argon2i([]byte(cfg.MinerPass))
	}
	// Add default port to all listener addresses if needed and remove duplicate addresses.
	cfg.Listeners = NormalizeAddresses(cfg.Listeners,
		ActiveNetParams.DefaultPort)
	// Add default port to all rpc listener addresses if needed and remove duplicate addresses.
	cfg.RPCListeners = NormalizeAddresses(cfg.RPCListeners,
		ActiveNetParams.RPCPort)
	if !cfg.DisableRPC && !cfg.TLS {
		for _, addr := range cfg.RPCListeners {
			if err != nil {
				str := "%s: RPC listen interface '%s' is invalid: %v"
				err := fmt.Errorf(str, funcName, addr, err)
				fmt.Fprintln(os.Stderr, err)
				fmt.Fprintln(os.Stderr, usageMessage)
				return nil, nil, err
			}
		}
	}
	// Add default port to all added peer addresses if needed and remove duplicate addresses.
	cfg.AddPeers = NormalizeAddresses(cfg.AddPeers,
		ActiveNetParams.DefaultPort)
	cfg.ConnectPeers = NormalizeAddresses(cfg.ConnectPeers,
		ActiveNetParams.DefaultPort)
	// --noonion and --onion do not mix.
	if cfg.NoOnion && cfg.OnionProxy != "" {
		err := fmt.Errorf("%s: the --noonion and --onion options may not be activated at the same time", funcName)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}
	// Check the checkpoints for syntax errors.
	StateCfg.AddedCheckpoints, err = ParseCheckpoints(cfg.AddCheckpoints)
	if err != nil {
		str := "%s: Error parsing checkpoints: %v"
		err := fmt.Errorf(str, funcName, err)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}
	// Tor stream isolation requires either proxy or onion proxy to be set.
	if cfg.TorIsolation && cfg.Proxy == "" && cfg.OnionProxy == "" {
		str := "%s: Tor stream isolation requires either proxy or onionproxy to be set"
		err := fmt.Errorf(str, funcName)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}
	// Setup dial and DNS resolution (lookup) functions depending on the specified options.  The default is to use the standard net.DialTimeout function as well as the system DNS resolver.  When a proxy is specified, the dial function is set to the proxy specific dial function and the lookup is set to use tor (unless --noonion is specified in which case the system DNS resolver is used).
	StateCfg.Dial = net.DialTimeout
	StateCfg.Lookup = net.LookupIP
	if cfg.Proxy != "" {
		_, _, err := net.SplitHostPort(cfg.Proxy)
		if err != nil {
			str := "%s: Proxy address '%s' is invalid: %v"
			err := fmt.Errorf(str, funcName, cfg.Proxy, err)
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, usageMessage)
			return nil, nil, err
		}
		// Tor isolation flag means proxy credentials will be overridden unless there is also an onion proxy configured in which case that one will be overridden.
		torIsolation := false
		if cfg.TorIsolation && cfg.OnionProxy == "" &&
			(cfg.ProxyUser != "" || cfg.ProxyPass != "") {
			torIsolation = true
			fmt.Fprintln(os.Stderr, "Tor isolation set -- overriding specified proxy user credentials")
		}
		proxy := &socks.Proxy{
			Addr:         cfg.Proxy,
			Username:     cfg.ProxyUser,
			Password:     cfg.ProxyPass,
			TorIsolation: torIsolation,
		}
		StateCfg.Dial = proxy.DialTimeout
		// Treat the proxy as tor and perform DNS resolution through it unless the --noonion flag is set or there is an onion-specific proxy configured.
		if !cfg.NoOnion && cfg.OnionProxy == "" {
			StateCfg.Lookup = func(host string) ([]net.IP, error) {
				return connmgr.TorLookupIP(host, cfg.Proxy)
			}
		}
	}
	// Setup onion address dial function depending on the specified options. The default is to use the same dial function selected above.  However, when an onion-specific proxy is specified, the onion address dial function is set to use the onion-specific proxy while leaving the normal dial function as selected above.  This allows .onion address traffic to be routed through a different proxy than normal traffic.
	if cfg.OnionProxy != "" {
		_, _, err := net.SplitHostPort(cfg.OnionProxy)
		if err != nil {
			str := "%s: Onion proxy address '%s' is invalid: %v"
			err := fmt.Errorf(str, funcName, cfg.OnionProxy, err)
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, usageMessage)
			return nil, nil, err
		}
		// Tor isolation flag means onion proxy credentials will be overridden.
		if cfg.TorIsolation &&
			(cfg.OnionProxyUser != "" || cfg.OnionProxyPass != "") {
			fmt.Fprintln(os.Stderr, "Tor isolation set -- "+
				"overriding specified onionproxy user "+
				"credentials ")
		}
		StateCfg.Oniondial = func(network, addr string, timeout time.Duration) (net.Conn, error) {
			proxy := &socks.Proxy{
				Addr:         cfg.OnionProxy,
				Username:     cfg.OnionProxyUser,
				Password:     cfg.OnionProxyPass,
				TorIsolation: cfg.TorIsolation,
			}
			return proxy.DialTimeout(network, addr, timeout)
		}
		// When configured in bridge mode (both --onion and --proxy are configured), it means that the proxy configured by --proxy is not a tor proxy, so override the DNS resolution to use the onion-specific proxy.
		if cfg.Proxy != "" {
			StateCfg.Lookup = func(host string) ([]net.IP, error) {
				return connmgr.TorLookupIP(host, cfg.OnionProxy)
			}
		}
	} else {
		StateCfg.Oniondial = StateCfg.Dial
	}
	// Specifying --noonion means the onion address dial function results in an error.
	if cfg.NoOnion {
		StateCfg.Oniondial = func(a, b string, t time.Duration) (net.Conn, error) {
			return nil, errors.New("tor has been disabled")
		}
	}
	// Warn about missing config file only after all other configuration is done.  This prevents the warning on help messages and invalid options.  Note this should go directly before the return.
	if configFileError != nil {
		log <- cl.Warn{configFileError}
	}
	return &cfg, remainingArgs, nil
}
// createDefaultConfig copies the file sample-pod.conf to the given destination path, and populates it with some randomly generated RPC username and password.
func createDefaultConfigFile(destinationPath string) error {
	// Create the destination directory if it does not exists
	err := os.MkdirAll(filepath.Dir(destinationPath), 0700)
	if err != nil {
		return err
	}
	// We generate a random user and password
	randomBytes := make([]byte, 20)
	_, err = rand.Read(randomBytes)
	if err != nil {
		return err
	}
	generatedRPCUser := base64.StdEncoding.EncodeToString(randomBytes)
	_, err = rand.Read(randomBytes)
	if err != nil {
		return err
	}
	generatedRPCPass := base64.StdEncoding.EncodeToString(randomBytes)
	var bb bytes.Buffer
	bb.Write(samplePodConf)
	dest, err := os.OpenFile(destinationPath,
		os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer dest.Close()
	reader := bufio.NewReader(&bb)
	for err != io.EOF {
		var line string
		line, err = reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return err
		}
		if strings.Contains(line, "rpcuser=") {
			line = "rpcuser=" + generatedRPCUser + "\n"
		} else if strings.Contains(line, "rpcpass=") {
			line = "rpcpass=" + generatedRPCPass + "\n"
		}
		if _, err := dest.WriteString(line); err != nil {
			return err
		}
	}
	return nil
}
// podDial connects to the address on the named network using the appropriate dial function depending on the address and configuration options.  For example, .onion addresses will be dialed using the onion specific proxy if one was specified, but will otherwise use the normal dial function (which could itself use a proxy or not).
func podDial(addr net.Addr) (net.Conn, error) {
	if strings.Contains(addr.String(), ".onion:") {
		return StateCfg.Oniondial(addr.Network(), addr.String(),
			DefaultConnectTimeout)
	}
	return StateCfg.Dial(addr.Network(), addr.String(), DefaultConnectTimeout)
}
// podLookup resolves the IP of the given host using the correct DNS lookup function depending on the configuration options.  For example, addresses will be resolved using tor when the --proxy flag was specified unless --noonion was also specified in which case the normal system DNS resolver will be used. Any attempt to resolve a tor address (.onion) will return an error since they are not intended to be resolved outside of the tor proxy.
func podLookup(host string) ([]net.IP, error) {
	if strings.HasSuffix(host, ".onion") {
		return nil, fmt.Errorf("attempt to resolve tor address %s", host)
	}
	return StateCfg.Lookup(host)
}
/*
pod is a full-node Parallelcoin implementation written in Go.
The default options are sane for most users.  This means pod will work 'out of the box' for most users.  However, there are also a wide variety of flags that can be used to control it.
The following section provides a usage overview which enumerates the flags.  An interesting point to note is that the long form of all of these options (except -C) can be specified in a configuration file that is automatically parsed when pod starts up.  By default, the configuration file is located at ~/.pod/pod.conf on POSIX-style operating systems and %LOCALAPPDATA%\pod\pod.conf on Windows.  The -C (--configfile) flag, as shown below, can be used to override this location.
Usage:
  pod [OPTIONS]
Application Options:
  -V, --version               Display version information and exit
  -C, --configfile=           Path to configuration file (default: /home/loki/.pod/pod.conf)
  -b, --datadir=              Directory to store data (default: /home/loki/.pod/data)
      --logdir=               Directory to log output. (default: /home/loki/.pod/logs)
  -a, --addpeer=              Add a peer to connect with at startup
      --connect=              Connect only to the specified peers at startup
      --nolisten              Disable listening for incoming connections -- NOTE: Listening is automatically disabled if the --connect or --proxy options are used without also specifying listen interfaces via --listen
      --listen=               Add an interface/port to listen for connections (default all interfaces port: 11047, testnet: 21047)
      --maxpeers=             Max number of inbound and outbound peers (default: 125)
      --nobanning             Disable banning of misbehaving peers
      --banduration=          How long to ban misbehaving peers.  Valid time units are {s, m, h}.  Minimum 1 second (default: 24h0m0s)
      --banthreshold=         Maximum allowed ban score before disconnecting and banning misbehaving peers. (default: 100)
      --whitelist=            Add an IP network or IP that will not be banned. (eg. 192.168.1.0/24 or ::1)
  -u, --rpcuser=              Username for RPC connections
  -P, --rpcpass=              Password for RPC connections
      --rpclimituser=         Username for limited RPC connections
      --rpclimitpass=         Password for limited RPC connections
      --rpclisten=            Add an interface/port to listen for RPC connections (default port: 11048, testnet: 21048) gives sha256d block templates
      --blake14lrlisten=      Additional RPC port that delivers blake14lr versioned block templates
      --cryptonight7v2=      Additional RPC port that delivers cryptonight7v2 versioned block templates
      --keccaklisten=         Additional RPC port that delivers keccak versioned block templates
      --lyra2rev2listen=      Additional RPC port that delivers lyra2rev2 versioned block templates
      --scryptlisten=         Additional RPC port that delivers scrypt versioned block templates
      --striboglisten=           Additional RPC port that delivers stribog versioned block templates
      --skeinlisten=          Additional RPC port that delivers skein versioned block templates
      --x11listen=            Additional RPC port that delivers x11 versioned block templates
      --rpccert=              File containing the certificate file (default: /home/loki/.pod/rpc.cert)
      --rpckey=               File containing the certificate key (default: /home/loki/.pod/rpc.key)
      --rpcmaxclients=        Max number of RPC clients for standard connections (default: 10)
      --rpcmaxwebsockets=     Max number of RPC websocket connections (default: 25)
      --rpcmaxconcurrentreqs= Max number of concurrent RPC requests that may be processed concurrently (default: 20)
      --rpcquirks             Mirror some JSON-RPC quirks of Bitcoin Core -- NOTE: Discouraged unless interoperability issues need to be worked around
      --norpc                 Disable built-in RPC server -- NOTE: The RPC server is disabled by default if no rpcuser/rpcpass or rpclimituser/rpclimitpass is specified
      --tls                   Enable TLS for the RPC server
      --nodnsseed             Disable DNS seeding for peers
      --externalip=           Add an ip to the list of local addresses we claim to listen on to peers
      --proxy=                Connect via SOCKS5 proxy (eg. 127.0.0.1:9050)
      --proxyuser=            Username for proxy server
      --proxypass=            Password for proxy server
      --onion=                Connect to tor hidden services via SOCKS5 proxy (eg. 127.0.0.1:9050)
      --onionuser=            Username for onion proxy server
      --onionpass=            Password for onion proxy server
      --noonion               Disable connecting to tor hidden services
      --torisolation          Enable Tor stream isolation by randomizing user credentials for each connection.
      --testnet               Use the test network
      --regtest               Use the regression test network
      --simnet                Use the simulation test network
      --addcheckpoint=        Add a custom checkpoint.  Format: '<height>:<hash>'
      --nocheckpoints         Disable built-in checkpoints.  Don't do this unless you know what you're doing.
      --dbtype=               Database backend to use for the Block Chain (default: ffldb)
      --profile=              Enable HTTP profiling on given port -- NOTE port must be between 1024 and 65536
      --cpuprofile=           Write CPU profile to the specified file
  -d, --debuglevel=           Logging level for all subsystems {trace, debug, info, warn, error, critical} -- You may also specify <subsystem>=<level>,<subsystem2>=<level>,... to set the log level for individual subsystems -- Use show to list available
                              subsystems (default: info)
      --upnp                  Use UPnP to map our listening port outside of NAT
      --minrelaytxfee=        The minimum transaction fee in DUO/kB to be considered a non-zero fee. (default: 1e-05)
      --limitfreerelay=       Limit relay of transactions with no transaction fee to the given amount in thousands of bytes per minute (default: 15)
      --norelaypriority       Do not require free or low-fee transactions to have high priority for relaying
      --trickleinterval=      Minimum time between attempts to send new inventory to a connected peer (default: 10s)
      --maxorphantx=          Max number of orphan transactions to keep in memory (default: 100)
      --algo=                 Sets the algorithm for the CPU miner ( blake14lr, cryptonight7v2, keccak, lyra2rev2, sha256d, scrypt, stribog, skein, x11,default sha256d) (default: sha256d)
      --generate              Generate (mine) bitcoins using the CPU
      --genthreads=           Number of CPU threads to use with CPU miner -1 = all cores (default: 1)
      --miningaddr=           Add the specified payment address to the list of addresses to use for generated blocks -- At least one address is required if the generate option is set
      --blockminsize=         Mininum block size in bytes to be used when creating a block (default: 80)
      --blockmaxsize=         Maximum block size in bytes to be used when creating a block (default: 200000)
      --blockminweight=       Mininum block weight to be used when creating a block (default: 10)
      --blockmaxweight=       Maximum block weight to be used when creating a block (default: 3000000)
      --blockprioritysize=    Size in bytes for high-priority/low-fee transactions when creating a block (default: 50000)
      --uacomment=            Comment to add to the user agent -- See BIP 14 for more information.
      --nopeerbloomfilters    Disable bloom filtering support
      --nocfilters            Disable committed filtering (CF) support
      --dropcfindex           Deletes the index used for committed filtering (CF) support from the database on start up and then exits.
      --sigcachemaxsize=      The maximum number of entries in the signature verification cache (default: 100000)
      --blocksonly            Do not accept transactions from remote peers.
      --txindex               Maintain a full hash-based transaction index which makes all transactions available via the getrawtransaction RPC
      --droptxindex           Deletes the hash-based transaction index from the database on start up and then exits.
      --addrindex             Maintain a full address-based transaction index which makes the searchrawtransactions RPC available
      --dropaddrindex         Deletes the address-based transaction index from the database on start up and then exits.
      --relaynonstd           Relay non-standard transactions regardless of the default settings for the active network.
      --rejectnonstd          Reject non-standard transactions regardless of the default settings for the active network.
Help Options:
  -h, --help                  Show this help message
*/
package node
package node
import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"time"
	cl "git.parallelcoin.io/pod/pkg/clog"
	blockchain "git.parallelcoin.io/pod/pkg/chain"
	"git.parallelcoin.io/pod/pkg/chaincfg/chainhash"
	"git.parallelcoin.io/pod/pkg/fork"
	"git.parallelcoin.io/pod/pkg/json"
	"git.parallelcoin.io/pod/pkg/util"
	"git.parallelcoin.io/pod/pkg/wire"
	"github.com/conformal/fastsha256"
)
var (
	// getworkDataLen is the length of the data field of the getwork RPC. It consists of the serialized block header plus the internal sha256 padding.  The internal sha256 padding consists of a single 1 bit followed by enough zeros to pad the message out to 56 bytes followed by length of the message in bits encoded as a big-endian uint64 (8 bytes).  Thus, the resulting length is a multiple of the sha256 block size (64 bytes).
	getworkDataLen = (1 + ((wire.MaxBlockHeaderPayload + 8) /
		fastsha256.BlockSize)) * fastsha256.BlockSize
	// hash1Len is the length of the hash1 field of the getwork RPC.  It consists of a zero hash plus the internal sha256 padding.  See the getworkDataLen comment for details about the internal sha256 padding format.
	hash1Len = (1 + ((chainhash.HashSize + 8) / fastsha256.BlockSize)) * fastsha256.BlockSize
)
func handleGetWork(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*json.GetWorkCmd)
	if len(StateCfg.ActiveMiningAddrs) == 0 {
		return nil, &json.RPCError{
			Code: json.ErrRPCInternal.Code,
			Message: "No payment addresses specified " +
				"via --miningaddr",
		}
	}
	if !(cfg.RegressionTest || cfg.SimNet) &&
		s.cfg.ConnMgr.ConnectedCount() == 0 {
		return nil, &json.RPCError{
			Code:    json.ErrRPCClientNotConnected,
			Message: "Pod is not connected to network",
		}
	}
	// No point in generating or accepting work before the chain is synced.
	latestHeight := s.cfg.Chain.BestSnapshot().Height
	if latestHeight != 0 && !s.cfg.SyncMgr.IsCurrent() {
		return nil, &json.RPCError{
			Code:    json.ErrRPCClientInInitialDownload,
			Message: "Pod is not yet synchronised...",
		}
	}
	state := s.gbtWorkState
	state.Lock()
	defer state.Unlock()
	if c.Data != nil {
		return handleGetWorkSubmission(s, *c.Data)
	}
	// Choose a payment address at random.
	rand.Seed(time.Now().UnixNano())
	payToAddr := StateCfg.ActiveMiningAddrs[rand.Intn(len(StateCfg.ActiveMiningAddrs))]
	lastTxUpdate := s.gbtWorkState.lastTxUpdate
	latestHash := &s.cfg.Chain.BestSnapshot().Hash
	generator := s.cfg.Generator
	if state.template == nil {
		var err error
		state.template, err = generator.NewBlockTemplate(payToAddr, s.cfg.Algo)
		if err != nil {
			return nil, err
		}
	}
	msgBlock := state.template.Block
	if msgBlock == nil || state.prevHash == nil ||
		!state.prevHash.IsEqual(latestHash) ||
		(state.lastTxUpdate != lastTxUpdate &&
			time.Now().After(state.lastGenerated.Add(time.Minute))) {
		/*	Reset the extra nonce and clear all cached template
			variations if the best block changed. */
		if state.prevHash != nil && !state.prevHash.IsEqual(latestHash) {
			state.updateBlockTemplate(s, false)
		}
		/*	Reset the previous best hash the block template was generated
			against so any errors below cause the next invocation to try
			again. */
		state.prevHash = nil
		var err error
		state.template, err = generator.NewBlockTemplate(payToAddr, s.cfg.Algo)
		if err != nil {
			errStr := fmt.Sprintf("Failed to create new block template: %v", err)
			log <- cl.Err(errStr)
			return nil, &json.RPCError{
				Code:    json.ErrRPCInternal.Code,
				Message: errStr,
			}
		}
		msgBlock = state.template.Block
		// Update work state to ensure another block template isn't generated until needed.
		state.template.Block = msgBlock
		state.lastGenerated = time.Now()
		state.lastTxUpdate = lastTxUpdate
		state.prevHash = latestHash
		Log.Dbgc(func() string {
			return fmt.Sprintf(
				"generated block template (timestamp %v, target %064x, merkle root %s, signature script %x)",
				msgBlock.Header.Timestamp,
				blockchain.CompactToBig(msgBlock.Header.Bits),
				msgBlock.Header.MerkleRoot,
				msgBlock.Transactions[0].TxIn[0].SignatureScript,
			)
		})
	} else {
		//	At this point, there is a saved block template and a new request for work was made, but either the available transactions haven't change or it hasn't been long enough to trigger a new block template to be generated.  So, update the existing block template and track the variations so each variation can be regenerated if a caller finds an answer and makes a submission against it. Update the time of the block template to the current time while accounting for the median time of the past several blocks per the chain consensus rules.
		generator.UpdateBlockTime(msgBlock)
		// Increment the extra nonce and update the block template with the new value by regenerating the coinbase script and setting the merkle root to the new value.
		log <- cl.Debugf{
			"updated block template (timestamp %v, target %064x, merkle root %s, signature script %x)",
			msgBlock.Header.Timestamp,
			blockchain.CompactToBig(msgBlock.Header.Bits),
			msgBlock.Header.MerkleRoot,
			msgBlock.Transactions[0].TxIn[0].SignatureScript,
		}
	}
	//	In order to efficiently store the variations of block templates that have been provided to callers, save a pointer to the block as well as the modified signature script keyed by the merkle root.  This information, along with the data that is included in a work submission, is used to rebuild the block before checking the submitted solution.
	/*
		coinbaseTx := msgBlock.Transactions[0]
		state.blockInfo[msgBlock.Header.MerkleRoot] = &workStateBlockInfo{
			msgBlock:        msgBlock,
			signatureScript: coinbaseTx.TxIn[0].SignatureScript,
		}
	*/
	// Serialize the block header into a buffer large enough to hold the the block header and the internal sha256 padding that is added and returned as part of the data below.
	data := make([]byte, 0, getworkDataLen)
	buf := bytes.NewBuffer(data)
	err := msgBlock.Header.Serialize(buf)
	if err != nil {
		errStr := fmt.Sprintf("Failed to serialize data: %v", err)
		log <- cl.Wrn(errStr)
		return nil, &json.RPCError{
			Code:    json.ErrRPCInternal.Code,
			Message: errStr,
		}
	}
	// Calculate the midstate for the block header.  The midstate here is the internal state of the sha256 algorithm for the first chunk of the block header (sha256 operates on 64-byte chunks) which is before the nonce.  This allows sophisticated callers to avoid hashing the first chunk over and over while iterating the nonce range.
	data = data[:buf.Len()]
	midstate := fastsha256.MidState256(data)
	// Expand the data slice to include the full data buffer and apply the internal sha256 padding which consists of a single 1 bit followed by enough zeros to pad the message out to 56 bytes followed by the length of the message in bits encoded as a big-endian uint64 (8 bytes).  Thus, the resulting length is a multiple of the sha256 block size (64 bytes).  This makes the data ready for sophisticated caller to make use of only the second chunk along with the midstate for the first chunk.
	data = data[:getworkDataLen]
	data[wire.MaxBlockHeaderPayload] = 0x80
	binary.BigEndian.PutUint64(data[len(data)-8:],
		wire.MaxBlockHeaderPayload*8)
	//	Create the hash1 field which is a zero hash along with the internal sha256 padding as described above.  This field is really quite useless, but it is required for compatibility with the reference implementation.
	var hash1 = make([]byte, hash1Len)
	hash1[chainhash.HashSize] = 0x80
	binary.BigEndian.PutUint64(hash1[len(hash1)-8:], chainhash.HashSize*8)
	// The final result reverses the each of the fields to little endian. In particular, the data, hash1, and midstate fields are treated as arrays of uint32s (per the internal sha256 hashing state) which are in big endian, and thus each 4 bytes is byte swapped.  The target is also in big endian, but it is treated as a uint256 and byte swapped to little endian accordingly. The fact the fields are reversed in this way is rather odd and likely an artifact of some legacy internal state in the reference implementation, but it is required for compatibility.
	reverseUint32Array(data)
	reverseUint32Array(hash1[:])
	reverseUint32Array(midstate[:])
	target := bigToLEUint256(blockchain.CompactToBig(msgBlock.Header.Bits))
	reply := &json.GetWorkResult{
		Data:     hex.EncodeToString(data),
		Hash1:    hex.EncodeToString(hash1[:]),
		Midstate: hex.EncodeToString(midstate[:]),
		Target:   hex.EncodeToString(target[:]),
	}
	return reply, nil
}
//	handleGetWorkSubmission is a helper for handleGetWork which deals with the calling submitting work to be verified and processed. This function MUST be called with the RPC workstate locked.
func handleGetWorkSubmission(s *rpcServer, hexData string) (interface{}, error) {
	// Ensure the provided data is sane.
	if len(hexData)%2 != 0 {
		hexData = "0" + hexData
	}
	data, err := hex.DecodeString(hexData)
	if err != nil {
		return nil, &json.RPCError{
			Code: json.ErrRPCInvalidParameter,
			Message: fmt.Sprintf("argument must be "+
				"hexadecimal string (not %q)", hexData),
		}
	}
	if len(data) != getworkDataLen {
		return false, &json.RPCError{
			Code: json.ErrRPCInvalidParameter,
			Message: fmt.Sprintf("argument must be "+
				"%d bytes (not %d)", getworkDataLen,
				len(data)),
		}
	}
	// Reverse the data as if it were an array of 32-bit unsigned integers. The fact the getwork request and submission data is reversed in this way is rather odd and likey an artifact of some legacy internal state in the reference implementation, but it is required for compatibility.
	reverseUint32Array(data)
	// Deserialize the block header from the data.
	var submittedHeader wire.BlockHeader
	bhBuf := bytes.NewBuffer(data[0:wire.MaxBlockHeaderPayload])
	err = submittedHeader.Deserialize(bhBuf)
	if err != nil {
		return false, &json.RPCError{
			Code: json.ErrRPCInvalidParameter,
			Message: fmt.Sprintf("argument does not "+
				"contain a valid block header: %v", err),
		}
	}
	// Look up the full block for the provided data based on the merkle root.  Return false to indicate the solve failed if it's not available.
	state := s.gbtWorkState
	if state.template.Block.Header.MerkleRoot.String() == "" {
		log <- cl.Debug{
			"Block submitted via getwork has no matching template for merkle root",
			submittedHeader.MerkleRoot,
		}
		return false, nil
	}
	// Reconstruct the block using the submitted header stored block info.
	msgBlock := state.template.Block
	block := util.NewBlock(msgBlock)
	msgBlock.Header.Timestamp = submittedHeader.Timestamp
	msgBlock.Header.Nonce = submittedHeader.Nonce
	msgBlock.Transactions[0].TxIn[0].SignatureScript = state.template.Block.Transactions[0].TxIn[0].SignatureScript
	merkles := blockchain.BuildMerkleTreeStore(block.Transactions(), false)
	msgBlock.Header.MerkleRoot = *merkles[len(merkles)-1]
	// Ensure the submitted block hash is less than the target difficulty.
	pl := fork.GetMinDiff(s.cfg.Algo, s.cfg.Chain.BestSnapshot().Height)
	log <- cl.Trace{"powlimit", pl}
	err = blockchain.CheckProofOfWork(block, pl, s.cfg.Chain.BestSnapshot().Height)
	if err != nil {
		// Anything other than a rule violation is an unexpected error, so return that error as an internal error.
		if _, ok := err.(blockchain.RuleError); !ok {
			return nil, &json.RPCError{
				Code:    json.ErrRPCInternal.Code,
				Message: fmt.Sprintf("Unexpected error while checking proof of work: %v", err),
			}
		}
		log <- cl.Debug{
			"block submitted via getwork does not meet the required proof of work:", err,
		}
		return false, nil
	}
	latestHash := &s.cfg.Chain.BestSnapshot().Hash
	if !msgBlock.Header.PrevBlock.IsEqual(latestHash) {
		log <- cl.Debugf{
			"block submitted via getwork with previous block %s is stale",
			msgBlock.Header.PrevBlock,
		}
		return false, nil
	}
	// Process this block using the same rules as blocks coming from other nodes.  This will in turn relay it to the network like normal.
	_, isOrphan, err := s.cfg.Chain.ProcessBlock(block, 0, s.cfg.Chain.BestSnapshot().Height)
	if err != nil || isOrphan {
		// Anything other than a rule violation is an unexpected error, so return that error as an internal error.
		if _, ok := err.(blockchain.RuleError); !ok {
			return nil, &json.RPCError{
				Code:    json.ErrRPCInternal.Code,
				Message: fmt.Sprintf("Unexpected error while processing block: %v", err),
			}
		}
		log <- cl.Info{"block submitted via getwork rejected:", err}
		return false, nil
	}
	// The block was accepted.
	blockSha := block.Hash()
	log <- cl.Info{"block submitted via getwork accepted:", blockSha}
	return true, nil
}
// reverseUint32Array treats the passed bytes as a series of uint32s and reverses the byte order of each uint32.  The passed byte slice must be a multiple of 4 for a correct result.  The passed bytes slice is modified.
func reverseUint32Array(b []byte) {
	blen := len(b)
	for i := 0; i < blen; i += 4 {
		b[i], b[i+3] = b[i+3], b[i]
		b[i+1], b[i+2] = b[i+2], b[i+1]
	}
}
// bigToLEUint256 returns the passed big integer as an unsigned 256-bit integer encoded as little-endian bytes.  Numbers which are larger than the max unsigned 256-bit integer are truncated.
func bigToLEUint256(n *big.Int) [uint256Size]byte {
	// Pad or truncate the big-endian big int to correct number of bytes.
	nBytes := n.Bytes()
	nlen := len(nBytes)
	pad := 0
	start := 0
	if nlen <= uint256Size {
		pad = uint256Size - nlen
	} else {
		start = nlen - uint256Size
	}
	var buf [uint256Size]byte
	copy(buf[pad:], nBytes[start:])
	// Reverse the bytes to little endian and return them.
	for i := 0; i < uint256Size/2; i++ {
		buf[i], buf[uint256Size-1-i] = buf[uint256Size-1-i], buf[i]
	}
	return buf
}
package node
import (
	cl "git.parallelcoin.io/pod/pkg/clog"
)
// Log is the logger for node
var Log = cl.NewSubSystem("cmd/node       ", "info")
var log = Log.Ch
// UseLogger uses a specified Logger to output package logging info. This should be used in preference to SetLogWriter if the caller is also using log.
func UseLogger(logger *cl.SubSystem) {
	Log = logger
	log = Log.Ch
}
// directionString is a helper function that returns a string that represents the direction of a connection (inbound or outbound).
func directionString(inbound bool) string {
	if inbound {
		return "inbound"
	}
	return "outbound"
}
// pickNoun returns the singular or plural form of a noun depending on the count n.
func pickNoun(n uint64, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
package node
import (
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime/pprof"
	cl "git.parallelcoin.io/pod/pkg/clog"
	"git.parallelcoin.io/pod/pkg/fork"
	"git.parallelcoin.io/pod/pkg/interrupt"
	indexers "git.parallelcoin.io/pod/pkg/chain/index"
	database "git.parallelcoin.io/pod/pkg/db"
)
const (
	// blockDbNamePrefix is the prefix for the block database name.  The database type is appended to this value to form the full block database name.
	blockDbNamePrefix = "blocks"
)
var (
	cfg      *Config
	StateCfg = new(StateConfig)
)
// winServiceMain is only invoked on Windows.  It detects when pod is running as a service and reacts accordingly.
var winServiceMain func() (bool, error)
// Main is the real main function for pod.  It is necessary to work around the fact that deferred functions do not run when os.Exit() is called.  The optional serverChan parameter is mainly used by the service code to be notified with the server once it is setup so it can gracefully stop it when requested from the service control manager.
func Main(c *Config, activeNet *Params, serverChan chan<- *server) (err error) {
	cfg = c
	switch activeNet.Name {
	case "testnet":
		fork.IsTestnet = true
		ActiveNetParams = &TestNet3Params
	case "simnet":
		ActiveNetParams = &SimNetParams
	default:
		ActiveNetParams = &MainNetParams
	}
	shutdownChan := make(chan struct{})
	interrupt.AddHandler(
		func() {
			log <- cl.Inf("shutdown complete")
			close(shutdownChan)
		},
	)
	// Show version at startup.
	log <- cl.Info{"version", Version()}
	// Enable http profiling server if requested.
	if cfg.Profile != "" {
		log <- cl.Dbg("profiling requested")
		go func() {
			listenAddr := net.JoinHostPort("", cfg.Profile)
			log <- cl.Info{"profile server listening on", listenAddr}
			profileRedirect := http.RedirectHandler("/debug/pprof",
				http.StatusSeeOther)
			http.Handle("/", profileRedirect)
			log <- cl.Error{"profile server", http.ListenAndServe(listenAddr, nil)}
		}()
	}
	// Write cpu profile if requested.
	if cfg.CPUProfile != "" {
		var f *os.File
		f, err = os.Create(cfg.CPUProfile)
		if err != nil {
			log <- cl.Error{"unable to create cpu profile:", err}
			return
		}
		pprof.StartCPUProfile(f)
		defer f.Close()
		defer pprof.StopCPUProfile()
	}
	// Perform upgrades to pod as new versions require it.
	if err = doUpgrades(); err != nil {
		log <- cl.Error{err}
		return
	}
	// Return now if an interrupt signal was triggered.
	if interrupt.Requested() {
		return nil
	}
	// Load the block database.
	var db database.DB
	log <- cl.Debug{"loading db with", activeNet.Params.Name, cfg.TestNet3}
	db, err = loadBlockDB()
	if err != nil {
		log <- cl.Error{err}
		return
	}
	defer func() {
		// Ensure the database is sync'd and closed on shutdown.
		log <- cl.Inf("gracefully shutting down the database...")
		db.Close()
	}()
	// Return now if an interrupt signal was triggered.
	if interrupt.Requested() {
		return nil
	}
	// Drop indexes and exit if requested. NOTE: The order is important here because dropping the tx index also drops the address index since it relies on it.
	if cfg.DropAddrIndex {
		if err = indexers.DropAddrIndex(db, interrupt.ShutdownRequestChan); err != nil {
			log <- cl.Error{err}
			return
		}
		return nil
	}
	if cfg.DropTxIndex {
		if err = indexers.DropTxIndex(db, interrupt.ShutdownRequestChan); err != nil {
			log <- cl.Error{err}
			return
		}
		return nil
	}
	if cfg.DropCfIndex {
		if err := indexers.DropCfIndex(db, interrupt.ShutdownRequestChan); err != nil {
			log <- cl.Error{err}
			return err
		}
		return nil
	}
	// Create server and start it.
	server, err := newServer(cfg.Listeners, db, ActiveNetParams.Params, interrupt.ShutdownRequestChan, cfg.Algo)
	if err != nil {
		// TODO: this logging could do with some beautifying.
		log <- cl.Errorf{"unable to start server on %v: %v", cfg.Listeners, err}
		return err
	}
	interrupt.AddHandler(func() {
		log <- cl.Inf("gracefully shutting down the server...")
		server.Stop()
		server.WaitForShutdown()
		log <- cl.Inf("server shutdown complete")
	})
	server.Start()
	if serverChan != nil {
		serverChan <- server
	}
	// Wait until the interrupt signal is received from an OS signal or shutdown is requested through one of the subsystems such as the RPC server.
	<-interrupt.HandlersDone
	return nil
}
// removeRegressionDB removes the existing regression test database if running in regression test mode and it already exists.
func removeRegressionDB(dbPath string) error {
	// Don't do anything if not in regression test mode.
	if !cfg.RegressionTest {
		return nil
	}
	// Remove the old regression test database if it already exists.
	fi, err := os.Stat(dbPath)
	if err == nil {
		log <- cl.Infof{"removing regression test database from '%s'", dbPath}
		if fi.IsDir() {
			err := os.RemoveAll(dbPath)
			if err != nil {
				return err
			}
		} else {
			err := os.Remove(dbPath)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
// dbPath returns the path to the block database given a database type.
func blockDbPath(dbType string) string {
	// The database name is based on the database type.
	dbName := blockDbNamePrefix + "_" + dbType
	if dbType == "sqlite" {
		dbName = dbName + ".db"
	}
	dbPath := filepath.Join(cfg.DataDir, dbName)
	return dbPath
}
// warnMultipleDBs shows a warning if multiple block database types are detected. This is not a situation most users want.  It is handy for development however to support multiple side-by-side databases.
func warnMultipleDBs() {
	// This is intentionally not using the known db types which depend on the database types compiled into the binary since we want to detect legacy db types as well.
	dbTypes := []string{"ffldb", "leveldb", "sqlite"}
	duplicateDbPaths := make([]string, 0, len(dbTypes)-1)
	for _, dbType := range dbTypes {
		if dbType == cfg.DbType {
			continue
		}
		// Store db path as a duplicate db if it exists.
		dbPath := blockDbPath(dbType)
		if FileExists(dbPath) {
			duplicateDbPaths = append(duplicateDbPaths, dbPath)
		}
	}
	// Warn if there are extra databases.
	if len(duplicateDbPaths) > 0 {
		selectedDbPath := blockDbPath(cfg.DbType)
		log <- cl.Warnf{
			"\nThere are multiple block chain databases using different database types.\n" +
				"You probably don't want to waste disk space by having more than one.\n" +
				"Your current database is located at [%v].\n" +
				"The additional database is located at %v",
			selectedDbPath,
			duplicateDbPaths,
		}
	}
}
// loadBlockDB loads (or creates when needed) the block database taking into account the selected database backend and returns a handle to it.  It also additional logic such warning the user if there are multiple databases which consume space on the file system and ensuring the regression test database is clean when in regression test mode.
func loadBlockDB() (database.DB, error) {
	// The memdb backend does not have a file path associated with it, so handle it uniquely.  We also don't want to worry about the multiple database type warnings when running with the memory database.
	if cfg.DbType == "memdb" {
		log <- cl.Inf("creating block database in memory")
		db, err := database.Create(cfg.DbType)
		if err != nil {
			return nil, err
		}
		return db, nil
	}
	warnMultipleDBs()
	// The database name is based on the database type.
	dbPath := blockDbPath(cfg.DbType)
	// The regression test is special in that it needs a clean database for each run, so remove it now if it already exists.
	removeRegressionDB(dbPath)
	log <- cl.Infof{"loading block database from '%s'", dbPath}
	db, err := database.Open(cfg.DbType, dbPath, ActiveNetParams.Net)
	if err != nil {
		// Return the error if it's not because the database doesn't exist.
		if dbErr, ok := err.(database.Error); !ok || dbErr.ErrorCode !=
			database.ErrDbDoesNotExist {
			return nil, err
		}
		// Create the db if it does not exist.
		err = os.MkdirAll(cfg.DataDir, 0700)
		if err != nil {
			return nil, err
		}
		db, err = database.Create(cfg.DbType, dbPath, ActiveNetParams.Net)
		if err != nil {
			return nil, err
		}
	}
	log <- cl.Inf("block database loaded")
	return db, nil
}
// func PreMain() {
// 	// Use all processor cores.
// 	runtime.GOMAXPROCS(runtime.NumCPU())
// 	// Block and transaction processing can cause bursty allocations.  This limits the garbage collector from excessively overallocating during bursts.  This value was arrived at with the help of profiling live usage.
// 	debug.SetGCPercent(10)
// 	// Up some limits.
// 	if err := limits.SetLimits(); err != nil {
// 		fmt.Fprintf(os.Stderr, "failed to set limits: %v\n", err)
// 		os.Exit(1)
// 	}
// 	// Call serviceMain on Windows to handle running as a service.  When the return isService flag is true, exit now since we ran as a service.  Otherwise, just fall through to normal operation.
// 	if runtime.GOOS == "windows" {
// 		isService, err := winServiceMain()
// 		if err != nil {
// 			fmt.Println(err)
// 			os.Exit(1)
// 		}
// 		if isService {
// 			os.Exit(0)
// 		}
// 	}
// 	// Work around defer not working after os.Exit()
// 	if err := Main(nil); err != nil {
// 		os.Exit(1)
// 	}
// }
package node
import (
	"git.parallelcoin.io/pod/pkg/chaincfg"
	"git.parallelcoin.io/pod/pkg/wire"
)
// ActiveNetParams is a pointer to the parameters specific to the currently active bitcoin network.
var ActiveNetParams = &MainNetParams
// Params is used to group parameters for various networks such as the main network and test networks.
type Params struct {
	*chaincfg.Params
	RPCPort string
}
// MainNetParams contains parameters specific to the main network (wire.MainNet).  NOTE: The RPC port is intentionally different than the reference implementation because pod does not handle wallet requests.  The separate wallet process listens on the well-known port and forwards requests it does not handle on to pod.  This approach allows the wallet process to emulate the full reference implementation RPC API.
var MainNetParams = Params{
	Params:  &chaincfg.MainNetParams,
	RPCPort: "11048",
}
// RegressionNetParams contains parameters specific to the regression test network (wire.TestNet).  NOTE: The RPC port is intentionally different than the reference implementation - see the MainNetParams comment for details.
var RegressionNetParams = Params{
	Params:  &chaincfg.RegressionNetParams,
	RPCPort: "31048",
}
// TestNet3Params contains parameters specific to the test network (version 3) (wire.TestNet3).  NOTE: The RPC port is intentionally different than the reference implementation - see the MainNetParams comment for details.
var TestNet3Params = Params{
	Params:  &chaincfg.TestNet3Params,
	RPCPort: "21048",
}
// SimNetParams contains parameters specific to the simulation test network (wire.SimNet).
var SimNetParams = Params{
	Params:  &chaincfg.SimNetParams,
	RPCPort: "41048",
}
// NetName returns the name used when referring to a bitcoin network.  At the time of writing, pod currently places blocks for testnet version 3 in the data and log directory "testnet", which does not match the Name field of the chaincfg parameters.  This function can be used to override this directory name as "testnet" when the passed active network matches wire.TestNet3. A proper upgrade to move the data and log directories for this network to "testnet3" is planned for the future, at which point this function can be removed and the network parameter's name used instead.
func NetName(chainParams *Params) string {
	switch chainParams.Net {
	case wire.TestNet3:
		return "testnet"
	default:
		return chainParams.Name
	}
}
package node
import (
	"sync/atomic"
	"git.parallelcoin.io/pod/pkg/chain"
	"git.parallelcoin.io/pod/pkg/chaincfg/chainhash"
	"git.parallelcoin.io/pod/pkg/netsync"
	"git.parallelcoin.io/pod/pkg/peer"
	"git.parallelcoin.io/pod/pkg/util"
	"git.parallelcoin.io/pod/pkg/wire"
	"git.parallelcoin.io/pod/cmd/node/mempool"
)
// rpcPeer provides a peer for use with the RPC server and implements the rpcserverPeer interface.
type rpcPeer serverPeer
// Ensure rpcPeer implements the rpcserverPeer interface.
var _ rpcserverPeer = (*rpcPeer)(nil)
// ToPeer returns the underlying peer instance. This function is safe for concurrent access and is part of the rpcserverPeer interface implementation.
func (p *rpcPeer) ToPeer() *peer.Peer {
	if p == nil {
		return nil
	}
	return (*serverPeer)(p).Peer
}
// IsTxRelayDisabled returns whether or not the peer has disabled transaction relay. This function is safe for concurrent access and is part of the rpcserverPeer interface implementation.
func (p *rpcPeer) IsTxRelayDisabled() bool {
	return (*serverPeer)(p).disableRelayTx
}
// BanScore returns the current integer value that represents how close the peer is to being banned. This function is safe for concurrent access and is part of the rpcserverPeer interface implementation.
func (p *rpcPeer) BanScore() uint32 {
	return (*serverPeer)(p).banScore.Int()
}
// FeeFilter returns the requested current minimum fee rate for which transactions should be announced. This function is safe for concurrent access and is part of the rpcserverPeer interface implementation.
func (p *rpcPeer) FeeFilter() int64 {
	return atomic.LoadInt64(&(*serverPeer)(p).feeFilter)
}
// rpcConnManager provides a connection manager for use with the RPC server and implements the rpcserver ConnManager interface.
type rpcConnManager struct {
	server *server
}
// Ensure rpcConnManager implements the rpcserverConnManager interface.
var _ rpcserverConnManager = &rpcConnManager{}
// Connect adds the provided address as a new outbound peer.  The permanent flag indicates whether or not to make the peer persistent and reconnect if the connection is lost.  Attempting to connect to an already existing peer will return an error. This function is safe for concurrent access and is part of the rpcserverConnManager interface implementation.
func (cm *rpcConnManager) Connect(addr string, permanent bool) error {
	replyChan := make(chan error)
	cm.server.query <- connectNodeMsg{
		addr:      addr,
		permanent: permanent,
		reply:     replyChan,
	}
	return <-replyChan
}
// RemoveByID removes the peer associated with the provided id from the list of persistent peers.  Attempting to remove an id that does not exist will return an error. This function is safe for concurrent access and is part of the rpcserverConnManager interface implementation.
func (cm *rpcConnManager) RemoveByID(id int32) error {
	replyChan := make(chan error)
	cm.server.query <- removeNodeMsg{
		cmp:   func(sp *serverPeer) bool { return sp.ID() == id },
		reply: replyChan,
	}
	return <-replyChan
}
// RemoveByAddr removes the peer associated with the provided address from the list of persistent peers.  Attempting to remove an address that does not exist will return an error. This function is safe for concurrent access and is part of the rpcserverConnManager interface implementation.
func (cm *rpcConnManager) RemoveByAddr(addr string) error {
	replyChan := make(chan error)
	cm.server.query <- removeNodeMsg{
		cmp:   func(sp *serverPeer) bool { return sp.Addr() == addr },
		reply: replyChan,
	}
	return <-replyChan
}
// DisconnectByID disconnects the peer associated with the provided id.  This applies to both inbound and outbound peers.  Attempting to remove an id that does not exist will return an error. This function is safe for concurrent access and is part of the rpcserverConnManager interface implementation.
func (cm *rpcConnManager) DisconnectByID(id int32) error {
	replyChan := make(chan error)
	cm.server.query <- disconnectNodeMsg{
		cmp:   func(sp *serverPeer) bool { return sp.ID() == id },
		reply: replyChan,
	}
	return <-replyChan
}
// DisconnectByAddr disconnects the peer associated with the provided address. This applies to both inbound and outbound peers.  Attempting to remove an address that does not exist will return an error. This function is safe for concurrent access and is part of the rpcserverConnManager interface implementation.
func (cm *rpcConnManager) DisconnectByAddr(addr string) error {
	replyChan := make(chan error)
	cm.server.query <- disconnectNodeMsg{
		cmp:   func(sp *serverPeer) bool { return sp.Addr() == addr },
		reply: replyChan,
	}
	return <-replyChan
}
// ConnectedCount returns the number of currently connected peers. This function is safe for concurrent access and is part of the rpcserverConnManager interface implementation.
func (cm *rpcConnManager) ConnectedCount() int32 {
	return cm.server.ConnectedCount()
}
// NetTotals returns the sum of all bytes received and sent across the network for all peers. This function is safe for concurrent access and is part of the rpcserverConnManager interface implementation.
func (cm *rpcConnManager) NetTotals() (uint64, uint64) {
	return cm.server.NetTotals()
}
// ConnectedPeers returns an array consisting of all connected peers. This function is safe for concurrent access and is part of the rpcserverConnManager interface implementation.
func (cm *rpcConnManager) ConnectedPeers() []rpcserverPeer {
	replyChan := make(chan []*serverPeer)
	cm.server.query <- getPeersMsg{reply: replyChan}
	serverPeers := <-replyChan
	// Convert to RPC server peers.
	peers := make([]rpcserverPeer, 0, len(serverPeers))
	for _, sp := range serverPeers {
		peers = append(peers, (*rpcPeer)(sp))
	}
	return peers
}
// PersistentPeers returns an array consisting of all the added persistent peers. This function is safe for concurrent access and is part of the rpcserverConnManager interface implementation.
func (cm *rpcConnManager) PersistentPeers() []rpcserverPeer {
	replyChan := make(chan []*serverPeer)
	cm.server.query <- getAddedNodesMsg{reply: replyChan}
	serverPeers := <-replyChan
	// Convert to generic peers.
	peers := make([]rpcserverPeer, 0, len(serverPeers))
	for _, sp := range serverPeers {
		peers = append(peers, (*rpcPeer)(sp))
	}
	return peers
}
// BroadcastMessage sends the provided message to all currently connected peers. This function is safe for concurrent access and is part of the rpcserverConnManager interface implementation.
func (cm *rpcConnManager) BroadcastMessage(msg wire.Message) {
	cm.server.BroadcastMessage(msg)
}
// AddRebroadcastInventory adds the provided inventory to the list of inventories to be rebroadcast at random intervals until they show up in a block. This function is safe for concurrent access and is part of the rpcserverConnManager interface implementation.
func (cm *rpcConnManager) AddRebroadcastInventory(iv *wire.InvVect, data interface{}) {
	cm.server.AddRebroadcastInventory(iv, data)
}
// RelayTransactions generates and relays inventory vectors for all of the passed transactions to all connected peers.
func (cm *rpcConnManager) RelayTransactions(txns []*mempool.TxDesc) {
	cm.server.relayTransactions(txns)
}
// rpcSyncMgr provides a block manager for use with the RPC server and implements the rpcserverSyncManager interface.
type rpcSyncMgr struct {
	server  *server
	syncMgr *netsync.SyncManager
}
// Ensure rpcSyncMgr implements the rpcserverSyncManager interface.
var _ rpcserverSyncManager = (*rpcSyncMgr)(nil)
// IsCurrent returns whether or not the sync manager believes the chain is current as compared to the rest of the network. This function is safe for concurrent access and is part of the rpcserverSyncManager interface implementation.
func (b *rpcSyncMgr) IsCurrent() bool {
	return b.syncMgr.IsCurrent()
}
// SubmitBlock submits the provided block to the network after processing it locally. This function is safe for concurrent access and is part of the rpcserverSyncManager interface implementation.
func (b *rpcSyncMgr) SubmitBlock(block *util.Block, flags blockchain.BehaviorFlags) (bool, error) {
	return b.syncMgr.ProcessBlock(block, flags)
}
// Pause pauses the sync manager until the returned channel is closed. This function is safe for concurrent access and is part of the rpcserverSyncManager interface implementation.
func (b *rpcSyncMgr) Pause() chan<- struct{} {
	return b.syncMgr.Pause()
}
// SyncPeerID returns the peer that is currently the peer being used to sync from. This function is safe for concurrent access and is part of the rpcserverSyncManager interface implementation.
func (b *rpcSyncMgr) SyncPeerID() int32 {
	return b.syncMgr.SyncPeerID()
}
// LocateBlocks returns the hashes of the blocks after the first known block in the provided locators until the provided stop hash or the current tip is reached, up to a max of wire.MaxBlockHeadersPerMsg hashes. This function is safe for concurrent access and is part of the rpcserverSyncManager interface implementation.
func (b *rpcSyncMgr) LocateHeaders(locators []*chainhash.Hash, hashStop *chainhash.Hash) []wire.BlockHeader {
	return b.server.chain.LocateHeaders(locators, hashStop)
}
package node
import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	js "encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"git.parallelcoin.io/pod/cmd/node/mempool"
	blockchain "git.parallelcoin.io/pod/pkg/chain"
	indexers "git.parallelcoin.io/pod/pkg/chain/index"
	"git.parallelcoin.io/pod/pkg/chaincfg"
	"git.parallelcoin.io/pod/pkg/chaincfg/chainhash"
	cl "git.parallelcoin.io/pod/pkg/clog"
	database "git.parallelcoin.io/pod/pkg/db"
	ec "git.parallelcoin.io/pod/pkg/ec"
	"git.parallelcoin.io/pod/pkg/fork"
	"git.parallelcoin.io/pod/pkg/json"
	"git.parallelcoin.io/pod/pkg/mining"
	"git.parallelcoin.io/pod/pkg/mining/cpuminer"
	p "git.parallelcoin.io/pod/pkg/peer"
	"git.parallelcoin.io/pod/pkg/txscript"
	"git.parallelcoin.io/pod/pkg/util"
	"git.parallelcoin.io/pod/pkg/wire"
	"github.com/btcsuite/websocket"
)
// API version constants
const (
	jsonrpcSemverString = "1.3.0"
	jsonrpcSemverMajor  = 1
	jsonrpcSemverMinor  = 3
	jsonrpcSemverPatch  = 0
)
const (
	// rpcAuthTimeoutSeconds is the number of seconds a connection to the RPC server is allowed to stay open without authenticating before it is closed.
	rpcAuthTimeoutSeconds = 10
	// uint256Size is the number of bytes needed to represent an unsigned 256-bit integer.
	uint256Size = 32
	// gbtNonceRange is two 32-bit big-endian hexadecimal integers which represent the valid ranges of nonces returned by the getblocktemplate RPC.
	gbtNonceRange = "00000000ffffffff"
	// gbtRegenerateSeconds is the number of seconds that must pass before a new template is generated when the previous block hash has not changed and there have been changes to the available transactions in the memory pool.
	gbtRegenerateSeconds = 60
	// maxProtocolVersion is the max protocol version the server supports.
	maxProtocolVersion = 70002
)
var (
	// gbtMutableFields are the manipulations the server allows to be made to block templates generated by the getblocktemplate RPC.  It is declared here to avoid the overhead of creating the slice on every invocation for constant data.
	gbtMutableFields = []string{
		"time", "transactions/add", "prevblock", "coinbase/append",
	}
	// gbtCoinbaseAux describes additional data that miners should include in the coinbase signature script.  It is declared here to avoid the overhead of creating a new object on every invocation for constant data.
	gbtCoinbaseAux = &json.GetBlockTemplateResultAux{
		Flags: hex.EncodeToString(builderScript(txscript.
			NewScriptBuilder().
			AddData([]byte(mining.CoinbaseFlags)))),
	}
	// gbtCapabilities describes additional capabilities returned with a block template generated by the getblocktemplate RPC. It is declared here to avoid the overhead of creating the slice on every invocation for constant data.
	gbtCapabilities = []string{"proposal"}
)
// Errors
var (
	// ErrRPCUnimplemented is an error returned to RPC clients when the provided command is recognized, but not implemented.
	ErrRPCUnimplemented = &json.RPCError{
		Code:    json.ErrRPCUnimplemented,
		Message: "Command unimplemented",
	}
	// ErrRPCNoWallet is an error returned to RPC clients when the provided command is recognized as a wallet command.
	ErrRPCNoWallet = &json.RPCError{
		Code:    json.ErrRPCNoWallet,
		Message: "This implementation does not implement wallet commands",
	}
)
type commandHandler func(*rpcServer, interface{}, <-chan struct{}) (interface{}, error)
// rpcHandlers maps RPC command strings to appropriate handler functions. This is set by init because help references rpcHandlers and thus causes a dependency loop.
var rpcHandlers map[string]commandHandler
var rpcHandlersBeforeInit = map[string]commandHandler{
	"addnode":              handleAddNode,
	"createrawtransaction": handleCreateRawTransaction,
	// "debuglevel":            handleDebugLevel,
	"decoderawtransaction":  handleDecodeRawTransaction,
	"decodescript":          handleDecodeScript,
	"estimatefee":           handleEstimateFee,
	"generate":              handleGenerate,
	"getaddednodeinfo":      handleGetAddedNodeInfo,
	"getbestblock":          handleGetBestBlock,
	"getbestblockhash":      handleGetBestBlockHash,
	"getblock":              handleGetBlock,
	"getblockchaininfo":     handleGetBlockChainInfo,
	"getblockcount":         handleGetBlockCount,
	"getblockhash":          handleGetBlockHash,
	"getblockheader":        handleGetBlockHeader,
	"getblocktemplate":      handleGetBlockTemplate,
	"getcfilter":            handleGetCFilter,
	"getcfilterheader":      handleGetCFilterHeader,
	"getconnectioncount":    handleGetConnectionCount,
	"getcurrentnet":         handleGetCurrentNet,
	"getdifficulty":         handleGetDifficulty,
	"getgenerate":           handleGetGenerate,
	"gethashespersec":       handleGetHashesPerSec,
	"getheaders":            handleGetHeaders,
	"getinfo":               handleGetInfo,
	"getmempoolinfo":        handleGetMempoolInfo,
	"getmininginfo":         handleGetMiningInfo,
	"getnettotals":          handleGetNetTotals,
	"getnetworkhashps":      handleGetNetworkHashPS,
	"getpeerinfo":           handleGetPeerInfo,
	"getrawmempool":         handleGetRawMempool,
	"getrawtransaction":     handleGetRawTransaction,
	"gettxout":              handleGetTxOut,
	"getwork":               handleGetWork,
	"help":                  handleHelp,
	"node":                  handleNode,
	"ping":                  handlePing,
	"searchrawtransactions": handleSearchRawTransactions,
	"sendrawtransaction":    handleSendRawTransaction,
	"setgenerate":           handleSetGenerate,
	"stop":                  handleStop,
	"submitblock":           handleSubmitBlock,
	"uptime":                handleUptime,
	"validateaddress":       handleValidateAddress,
	"verifychain":           handleVerifyChain,
	"verifymessage":         handleVerifyMessage,
	"version":               handleVersion,
}
// list of commands that we recognize, but for which pod has no support because it lacks support for wallet functionality. For these commands the user should ask a connected instance of btcwallet.
var rpcAskWallet = map[string]struct{}{
	"addmultisigaddress":     {},
	"backupwallet":           {},
	"createencryptedwallet":  {},
	"createmultisig":         {},
	"dumpprivkey":            {},
	"dumpwallet":             {},
	"encryptwallet":          {},
	"getaccount":             {},
	"getaccountaddress":      {},
	"getaddressesbyaccount":  {},
	"getbalance":             {},
	"getnewaddress":          {},
	"getrawchangeaddress":    {},
	"getreceivedbyaccount":   {},
	"getreceivedbyaddress":   {},
	"gettransaction":         {},
	"gettxoutsetinfo":        {},
	"getunconfirmedbalance":  {},
	"getwalletinfo":          {},
	"importprivkey":          {},
	"importwallet":           {},
	"keypoolrefill":          {},
	"listaccounts":           {},
	"listaddressgroupings":   {},
	"listlockunspent":        {},
	"listreceivedbyaccount":  {},
	"listreceivedbyaddress":  {},
	"listsinceblock":         {},
	"listtransactions":       {},
	"listunspent":            {},
	"lockunspent":            {},
	"move":                   {},
	"sendfrom":               {},
	"sendmany":               {},
	"sendtoaddress":          {},
	"setaccount":             {},
	"settxfee":               {},
	"signmessage":            {},
	"signrawtransaction":     {},
	"walletlock":             {},
	"walletpassphrase":       {},
	"walletpassphrasechange": {},
}
// Commands that are currently unimplemented, but should ultimately be.
var rpcUnimplemented = map[string]struct{}{
	"estimatepriority": {},
	"getchaintips":     {},
	"getmempoolentry":  {},
	"getnetworkinfo":   {},
	"getwork":          {},
	"invalidateblock":  {},
	"preciousblock":    {},
	"reconsiderblock":  {},
}
// Commands that are available to a limited user
var rpcLimited = map[string]struct{}{
	// Websockets commands
	"loadtxfilter":          {},
	"notifyblocks":          {},
	"notifynewtransactions": {},
	"notifyreceived":        {},
	"notifyspent":           {},
	"rescan":                {},
	"rescanblocks":          {},
	"session":               {},
	// Websockets AND HTTP/S commands
	"help": {},
	// HTTP/S-only commands
	"createrawtransaction":  {},
	"decoderawtransaction":  {},
	"decodescript":          {},
	"estimatefee":           {},
	"getbestblock":          {},
	"getbestblockhash":      {},
	"getblock":              {},
	"getblockcount":         {},
	"getblockhash":          {},
	"getblockheader":        {},
	"getcfilter":            {},
	"getcfilterheader":      {},
	"getcurrentnet":         {},
	"getdifficulty":         {},
	"getheaders":            {},
	"getinfo":               {},
	"getnettotals":          {},
	"getnetworkhashps":      {},
	"getrawmempool":         {},
	"getrawtransaction":     {},
	"gettxout":              {},
	"searchrawtransactions": {},
	"sendrawtransaction":    {},
	"submitblock":           {},
	"uptime":                {},
	"validateaddress":       {},
	"verifymessage":         {},
	"version":               {},
}
// builderScript is a convenience function which is used for hard-coded scripts built with the script builder. Any errors are converted to a panic since it is only, and must only, be used with hard-coded, and therefore, known good, scripts.
func builderScript(builder *txscript.ScriptBuilder) []byte {
	script, err := builder.Script()
	if err != nil {
		panic(err)
	}
	return script
}
// internalRPCError is a convenience function to convert an internal error to an RPC error with the appropriate code set.  It also logs the error to the RPC server subsystem since internal errors really should not occur.  The context parameter is only used in the log message and may be empty if it's not needed.
func internalRPCError(errStr, context string) *json.RPCError {
	logStr := errStr
	if context != "" {
		logStr = context + ": " + errStr
	}
	log <- cl.Err(logStr)
	return json.NewRPCError(json.ErrRPCInternal.Code, errStr)
}
// rpcDecodeHexError is a convenience function for returning a nicely formatted RPC error which indicates the provided hex string failed to decode.
func rpcDecodeHexError(gotHex string) *json.RPCError {
	return json.NewRPCError(json.ErrRPCDecodeHexString,
		fmt.Sprintf("Argument must be hexadecimal string (not %q)",
			gotHex))
}
// rpcNoTxInfoError is a convenience function for returning a nicely formatted RPC error which indicates there is no information available for the provided transaction hash.
func rpcNoTxInfoError(txHash *chainhash.Hash) *json.RPCError {
	return json.NewRPCError(json.ErrRPCNoTxInfo,
		fmt.Sprintf("No information available about transaction %v",
			txHash))
}
// gbtWorkState houses state that is used in between multiple RPC invocations to getblocktemplate.
type gbtWorkState struct {
	sync.Mutex
	lastTxUpdate  time.Time
	lastGenerated time.Time
	prevHash      *chainhash.Hash
	minTimestamp  time.Time
	template      *mining.BlockTemplate
	notifyMap     map[chainhash.Hash]map[int64]chan struct{}
	timeSource    blockchain.MedianTimeSource
	algo          string
}
// newGbtWorkState returns a new instance of a gbtWorkState with all internal fields initialized and ready to use.
func newGbtWorkState(timeSource blockchain.MedianTimeSource, algoname string) *gbtWorkState {
	return &gbtWorkState{
		notifyMap:  make(map[chainhash.Hash]map[int64]chan struct{}),
		timeSource: timeSource,
		algo:       algoname,
	}
}
// handleUnimplemented is the handler for commands that should ultimately be supported but are not yet implemented.
func handleUnimplemented(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	return nil, ErrRPCUnimplemented
}
// handleAskWallet is the handler for commands that are recognized as valid, but are unable to answer correctly since it involves wallet state. These commands will be implemented in btcwallet.
func handleAskWallet(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	return nil, ErrRPCNoWallet
}
// handleAddNode handles addnode commands.
func handleAddNode(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*json.AddNodeCmd)
	addr := NormalizeAddress(c.Addr, s.cfg.ChainParams.DefaultPort)
	var err error
	switch c.SubCmd {
	case "add":
		err = s.cfg.ConnMgr.Connect(addr, true)
	case "remove":
		err = s.cfg.ConnMgr.RemoveByAddr(addr)
	case "onetry":
		err = s.cfg.ConnMgr.Connect(addr, false)
	default:
		return nil, &json.RPCError{
			Code:    json.ErrRPCInvalidParameter,
			Message: "invalid subcommand for addnode",
		}
	}
	if err != nil {
		return nil, &json.RPCError{
			Code:    json.ErrRPCInvalidParameter,
			Message: err.Error(),
		}
	}
	// no data returned unless an error.
	return nil, nil
}
// handleNode handles node commands.
func handleNode(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*json.NodeCmd)
	var addr string
	var nodeID uint64
	var errN, err error
	params := s.cfg.ChainParams
	switch c.SubCmd {
	case "disconnect":
		// If we have a valid uint disconnect by node id. Otherwise, attempt to disconnect by address, returning an error if a valid IP address is not supplied.
		if nodeID, errN = strconv.ParseUint(c.Target, 10, 32); errN == nil {
			err = s.cfg.ConnMgr.DisconnectByID(int32(nodeID))
		} else {
			if _, _, errP := net.SplitHostPort(c.Target); errP == nil || net.ParseIP(c.Target) != nil {
				addr = NormalizeAddress(c.Target, params.DefaultPort)
				err = s.cfg.ConnMgr.DisconnectByAddr(addr)
			} else {
				return nil, &json.RPCError{
					Code:    json.ErrRPCInvalidParameter,
					Message: "invalid address or node ID",
				}
			}
		}
		if err != nil && peerExists(s.cfg.ConnMgr, addr, int32(nodeID)) {
			return nil, &json.RPCError{
				Code:    json.ErrRPCMisc,
				Message: "can't disconnect a permanent peer, use remove",
			}
		}
	case "remove":
		// If we have a valid uint disconnect by node id. Otherwise, attempt to disconnect by address, returning an error if a valid IP address is not supplied.
		if nodeID, errN = strconv.ParseUint(c.Target, 10, 32); errN == nil {
			err = s.cfg.ConnMgr.RemoveByID(int32(nodeID))
		} else {
			if _, _, errP := net.SplitHostPort(c.Target); errP == nil || net.ParseIP(c.Target) != nil {
				addr = NormalizeAddress(c.Target, params.DefaultPort)
				err = s.cfg.ConnMgr.RemoveByAddr(addr)
			} else {
				return nil, &json.RPCError{
					Code:    json.ErrRPCInvalidParameter,
					Message: "invalid address or node ID",
				}
			}
		}
		if err != nil && peerExists(s.cfg.ConnMgr, addr, int32(nodeID)) {
			return nil, &json.RPCError{
				Code:    json.ErrRPCMisc,
				Message: "can't remove a temporary peer, use disconnect",
			}
		}
	case "connect":
		addr = NormalizeAddress(c.Target, params.DefaultPort)
		// Default to temporary connections.
		subCmd := "temp"
		if c.ConnectSubCmd != nil {
			subCmd = *c.ConnectSubCmd
		}
		switch subCmd {
		case "perm", "temp":
			err = s.cfg.ConnMgr.Connect(addr, subCmd == "perm")
		default:
			return nil, &json.RPCError{
				Code:    json.ErrRPCInvalidParameter,
				Message: "invalid subcommand for node connect",
			}
		}
	default:
		return nil, &json.RPCError{
			Code:    json.ErrRPCInvalidParameter,
			Message: "invalid subcommand for node",
		}
	}
	if err != nil {
		return nil, &json.RPCError{
			Code:    json.ErrRPCInvalidParameter,
			Message: err.Error(),
		}
	}
	// no data returned unless an error.
	return nil, nil
}
// peerExists determines if a certain peer is currently connected given information about all currently connected peers. Peer existence is determined using either a target address or node id.
func peerExists(connMgr rpcserverConnManager, addr string, nodeID int32) bool {
	for _, p := range connMgr.ConnectedPeers() {
		if p.ToPeer().ID() == nodeID || p.ToPeer().Addr() == addr {
			return true
		}
	}
	return false
}
// messageToHex serializes a message to the wire protocol encoding using the latest protocol version and returns a hex-encoded string of the result.
func messageToHex(msg wire.Message) (string, error) {
	var buf bytes.Buffer
	if err := msg.BtcEncode(&buf, maxProtocolVersion, wire.WitnessEncoding); err != nil {
		context := fmt.Sprintf("Failed to encode msg of type %T", msg)
		return "", internalRPCError(err.Error(), context)
	}
	return hex.EncodeToString(buf.Bytes()), nil
}
// handleCreateRawTransaction handles createrawtransaction commands.
func handleCreateRawTransaction(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*json.CreateRawTransactionCmd)
	// Validate the locktime, if given.
	if c.LockTime != nil &&
		(*c.LockTime < 0 || *c.LockTime > int64(wire.MaxTxInSequenceNum)) {
		return nil, &json.RPCError{
			Code:    json.ErrRPCInvalidParameter,
			Message: "Locktime out of range",
		}
	}
	// Add all transaction inputs to a new transaction after performing some validity checks.
	mtx := wire.NewMsgTx(wire.TxVersion)
	for _, input := range c.Inputs {
		txHash, err := chainhash.NewHashFromStr(input.Txid)
		if err != nil {
			return nil, rpcDecodeHexError(input.Txid)
		}
		prevOut := wire.NewOutPoint(txHash, input.Vout)
		txIn := wire.NewTxIn(prevOut, []byte{}, nil)
		if c.LockTime != nil && *c.LockTime != 0 {
			txIn.Sequence = wire.MaxTxInSequenceNum - 1
		}
		mtx.AddTxIn(txIn)
	}
	// Add all transaction outputs to the transaction after performing some validity checks.
	params := s.cfg.ChainParams
	for encodedAddr, amount := range c.Amounts {
		// Ensure amount is in the valid range for monetary amounts.
		if amount <= 0 || amount > util.MaxSatoshi {
			return nil, &json.RPCError{
				Code:    json.ErrRPCType,
				Message: "Invalid amount",
			}
		}
		// Decode the provided address.
		addr, err := util.DecodeAddress(encodedAddr, params)
		if err != nil {
			return nil, &json.RPCError{
				Code:    json.ErrRPCInvalidAddressOrKey,
				Message: "Invalid address or key: " + err.Error(),
			}
		}
		// Ensure the address is one of the supported types and that the network encoded with the address matches the network the server is currently on.
		switch addr.(type) {
		case *util.AddressPubKeyHash:
		case *util.AddressScriptHash:
		default:
			return nil, &json.RPCError{
				Code:    json.ErrRPCInvalidAddressOrKey,
				Message: "Invalid address or key",
			}
		}
		if !addr.IsForNet(params) {
			return nil, &json.RPCError{
				Code: json.ErrRPCInvalidAddressOrKey,
				Message: "Invalid address: " + encodedAddr +
					" is for the wrong network",
			}
		}
		// Create a new script which pays to the provided address.
		pkScript, err := txscript.PayToAddrScript(addr)
		if err != nil {
			context := "Failed to generate pay-to-address script"
			return nil, internalRPCError(err.Error(), context)
		}
		// Convert the amount to satoshi.
		satoshi, err := util.NewAmount(amount)
		if err != nil {
			context := "Failed to convert amount"
			return nil, internalRPCError(err.Error(), context)
		}
		txOut := wire.NewTxOut(int64(satoshi), pkScript)
		mtx.AddTxOut(txOut)
	}
	// Set the Locktime, if given.
	if c.LockTime != nil {
		mtx.LockTime = uint32(*c.LockTime)
	}
	// Return the serialized and hex-encoded transaction.  Note that this is intentionally not directly returning because the first return value is a string and it would result in returning an empty string to the client instead of nothing (nil) in the case of an error.
	mtxHex, err := messageToHex(mtx)
	if err != nil {
		return nil, err
	}
	return mtxHex, nil
}
// // handleDebugLevel handles debuglevel commands.
// func handleDebugLevel(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
// 	c := cmd.(*json.DebugLevelCmd)
// 	// Special show command to list supported subsystems.
// 	if c.LevelSpec == "show" {
// 		return fmt.Sprintf("Supported subsystems %v",
// 			supportedSubsystems()), nil
// 	}
// 	err := parseAndSetDebugLevels(c.LevelSpec)
// 	if err != nil {
// 		return nil, &json.RPCError{
// 			Code:    json.ErrRPCInvalidParams.Code,
// 			Message: err.Error(),
// 		}
// 	}
// 	return "Done.", nil
// }
// witnessToHex formats the passed witness stack as a slice of hex-encoded strings to be used in a JSON response.
func witnessToHex(witness wire.TxWitness) []string {
	// Ensure nil is returned when there are no entries versus an empty slice so it can properly be omitted as necessary.
	if len(witness) == 0 {
		return nil
	}
	result := make([]string, 0, len(witness))
	for _, wit := range witness {
		result = append(result, hex.EncodeToString(wit))
	}
	return result
}
// createVinList returns a slice of JSON objects for the inputs of the passed transaction.
func createVinList(mtx *wire.MsgTx) []json.Vin {
	// Coinbase transactions only have a single txin by definition.
	vinList := make([]json.Vin, len(mtx.TxIn))
	if blockchain.IsCoinBaseTx(mtx) {
		txIn := mtx.TxIn[0]
		vinList[0].Coinbase = hex.EncodeToString(txIn.SignatureScript)
		vinList[0].Sequence = txIn.Sequence
		vinList[0].Witness = witnessToHex(txIn.Witness)
		return vinList
	}
	for i, txIn := range mtx.TxIn {
		// The disassembled string will contain [error] inline if the script doesn't fully parse, so ignore the error here.
		disbuf, _ := txscript.DisasmString(txIn.SignatureScript)
		vinEntry := &vinList[i]
		vinEntry.Txid = txIn.PreviousOutPoint.Hash.String()
		vinEntry.Vout = txIn.PreviousOutPoint.Index
		vinEntry.Sequence = txIn.Sequence
		vinEntry.ScriptSig = &json.ScriptSig{
			Asm: disbuf,
			Hex: hex.EncodeToString(txIn.SignatureScript),
		}
		if mtx.HasWitness() {
			vinEntry.Witness = witnessToHex(txIn.Witness)
		}
	}
	return vinList
}
// createVoutList returns a slice of JSON objects for the outputs of the passed transaction.
func createVoutList(mtx *wire.MsgTx, chainParams *chaincfg.Params, filterAddrMap map[string]struct{}) []json.Vout {
	voutList := make([]json.Vout, 0, len(mtx.TxOut))
	for i, v := range mtx.TxOut {
		// The disassembled string will contain [error] inline if the script doesn't fully parse, so ignore the error here.
		disbuf, _ := txscript.DisasmString(v.PkScript)
		// Ignore the error here since an error means the script couldn't parse and there is no additional information about it anyways.
		scriptClass, addrs, reqSigs, _ := txscript.ExtractPkScriptAddrs(v.PkScript, chainParams)
		// Encode the addresses while checking if the address passes the filter when needed.
		passesFilter := len(filterAddrMap) == 0
		encodedAddrs := make([]string, len(addrs))
		for j, addr := range addrs {
			encodedAddr := addr.EncodeAddress()
			encodedAddrs[j] = encodedAddr
			// No need to check the map again if the filter already passes.
			if passesFilter {
				continue
			}
			if _, exists := filterAddrMap[encodedAddr]; exists {
				passesFilter = true
			}
		}
		if !passesFilter {
			continue
		}
		var vout json.Vout
		vout.N = uint32(i)
		vout.Value = util.Amount(v.Value).ToDUO()
		vout.ScriptPubKey.Addresses = encodedAddrs
		vout.ScriptPubKey.Asm = disbuf
		vout.ScriptPubKey.Hex = hex.EncodeToString(v.PkScript)
		vout.ScriptPubKey.Type = scriptClass.String()
		vout.ScriptPubKey.ReqSigs = int32(reqSigs)
		voutList = append(voutList, vout)
	}
	return voutList
}
// createTxRawResult converts the passed transaction and associated parameters to a raw transaction JSON object.
func createTxRawResult(chainParams *chaincfg.Params, mtx *wire.MsgTx,
	txHash string, blkHeader *wire.BlockHeader, blkHash string,
	blkHeight int32, chainHeight int32) (*json.TxRawResult, error) {
	mtxHex, err := messageToHex(mtx)
	if err != nil {
		return nil, err
	}
	txReply := &json.TxRawResult{
		Hex:      mtxHex,
		Txid:     txHash,
		Hash:     mtx.WitnessHash().String(),
		Size:     int32(mtx.SerializeSize()),
		Vsize:    int32(mempool.GetTxVirtualSize(util.NewTx(mtx))),
		Vin:      createVinList(mtx),
		Vout:     createVoutList(mtx, chainParams, nil),
		Version:  mtx.Version,
		LockTime: mtx.LockTime,
	}
	if blkHeader != nil {
		// This is not a typo, they are identical in bitcoind as well.
		txReply.Time = blkHeader.Timestamp.Unix()
		txReply.Blocktime = blkHeader.Timestamp.Unix()
		txReply.BlockHash = blkHash
		txReply.Confirmations = uint64(1 + chainHeight - blkHeight)
	}
	return txReply, nil
}
// handleDecodeRawTransaction handles decoderawtransaction commands.
func handleDecodeRawTransaction(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*json.DecodeRawTransactionCmd)
	// Deserialize the transaction.
	hexStr := c.HexTx
	if len(hexStr)%2 != 0 {
		hexStr = "0" + hexStr
	}
	serializedTx, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, rpcDecodeHexError(hexStr)
	}
	var mtx wire.MsgTx
	err = mtx.Deserialize(bytes.NewReader(serializedTx))
	if err != nil {
		return nil, &json.RPCError{
			Code:    json.ErrRPCDeserialization,
			Message: "TX decode failed: " + err.Error(),
		}
	}
	// Create and return the result.
	txReply := json.TxRawDecodeResult{
		Txid:     mtx.TxHash().String(),
		Version:  mtx.Version,
		Locktime: mtx.LockTime,
		Vin:      createVinList(&mtx),
		Vout:     createVoutList(&mtx, s.cfg.ChainParams, nil),
	}
	return txReply, nil
}
// handleDecodeScript handles decodescript commands.
func handleDecodeScript(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*json.DecodeScriptCmd)
	// Convert the hex script to bytes.
	hexStr := c.HexScript
	if len(hexStr)%2 != 0 {
		hexStr = "0" + hexStr
	}
	script, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, rpcDecodeHexError(hexStr)
	}
	// The disassembled string will contain [error] inline if the script doesn't fully parse, so ignore the error here.
	disbuf, _ := txscript.DisasmString(script)
	// Get information about the script. Ignore the error here since an error means the script couldn't parse and there is no additinal information about it anyways.
	scriptClass, addrs, reqSigs, _ := txscript.ExtractPkScriptAddrs(script, s.cfg.ChainParams)
	addresses := make([]string, len(addrs))
	for i, addr := range addrs {
		addresses[i] = addr.EncodeAddress()
	}
	// Convert the script itself to a pay-to-script-hash address.
	p2sh, err := util.NewAddressScriptHash(script, s.cfg.ChainParams)
	if err != nil {
		context := "Failed to convert script to pay-to-script-hash"
		return nil, internalRPCError(err.Error(), context)
	}
	// Generate and return the reply.
	reply := json.DecodeScriptResult{
		Asm:       disbuf,
		ReqSigs:   int32(reqSigs),
		Type:      scriptClass.String(),
		Addresses: addresses,
	}
	if scriptClass != txscript.ScriptHashTy {
		reply.P2sh = p2sh.EncodeAddress()
	}
	return reply, nil
}
// handleEstimateFee handles estimatefee commands.
func handleEstimateFee(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*json.EstimateFeeCmd)
	if s.cfg.FeeEstimator == nil {
		return nil, errors.New("Fee estimation disabled")
	}
	if c.NumBlocks <= 0 {
		return -1.0, errors.New("Parameter NumBlocks must be positive")
	}
	feeRate, err := s.cfg.FeeEstimator.EstimateFee(uint32(c.NumBlocks))
	if err != nil {
		return -1.0, err
	}
	// Convert to satoshis per kb.
	return float64(feeRate), nil
}
// handleGenerate handles generate commands.
func handleGenerate(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// Respond with an error if there are no addresses to pay the created blocks to.
	if len(StateCfg.ActiveMiningAddrs) == 0 {
		return nil, &json.RPCError{
			Code:    json.ErrRPCInternal.Code,
			Message: "No payment addresses specified via --miningaddr",
		}
	}
	// Respond with an error if there's virtually 0 chance of mining a block with the CPU.
	if !s.cfg.ChainParams.GenerateSupported {
		return nil, &json.RPCError{
			Code:    json.ErrRPCDifficulty,
			Message: fmt.Sprintf("No support for `generate` on the current network, %s, as it's unlikely to be possible to mine a block with the CPU.", s.cfg.ChainParams.Net),
		}
	}
	// Set the algorithm according to the port we were called on
	s.cfg.CPUMiner.SetAlgo(s.cfg.Algo)
	c := cmd.(*json.GenerateCmd)
	// Respond with an error if the client is requesting 0 blocks to be generated.
	if c.NumBlocks == 0 {
		return nil, &json.RPCError{
			Code:    json.ErrRPCInternal.Code,
			Message: "Please request a nonzero number of blocks to generate.",
		}
	}
	// Create a reply
	reply := make([]string, c.NumBlocks)
	fmt.Println(s.cfg.Algo)
	blockHashes, err := s.cfg.CPUMiner.GenerateNBlocks(c.NumBlocks, s.cfg.Algo)
	if err != nil {
		return nil, &json.RPCError{
			Code:    json.ErrRPCInternal.Code,
			Message: err.Error(),
		}
	}
	// Mine the correct number of blocks, assigning the hex representation of the hash of each one to its place in the reply.
	for i, hash := range blockHashes {
		reply[i] = hash.String()
	}
	return reply, nil
}
// handleGetAddedNodeInfo handles getaddednodeinfo commands.
func handleGetAddedNodeInfo(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*json.GetAddedNodeInfoCmd)
	// Retrieve a list of persistent (added) peers from the server and filter the list of peers per the specified address (if any).
	peers := s.cfg.ConnMgr.PersistentPeers()
	if c.Node != nil {
		node := *c.Node
		found := false
		for i, peer := range peers {
			if peer.ToPeer().Addr() == node {
				peers = peers[i : i+1]
				found = true
			}
		}
		if !found {
			return nil, &json.RPCError{
				Code:    json.ErrRPCClientNodeNotAdded,
				Message: "Node has not been added",
			}
		}
	}
	// Without the dns flag, the result is just a slice of the addresses as strings.
	if !c.DNS {
		results := make([]string, 0, len(peers))
		for _, peer := range peers {
			results = append(results, peer.ToPeer().Addr())
		}
		return results, nil
	}
	// With the dns flag, the result is an array of JSON objects which include the result of DNS lookups for each peer.
	results := make([]*json.GetAddedNodeInfoResult, 0, len(peers))
	for _, rpcPeer := range peers {
		// Set the "address" of the peer which could be an ip address or a domain name.
		peer := rpcPeer.ToPeer()
		var result json.GetAddedNodeInfoResult
		result.AddedNode = peer.Addr()
		result.Connected = json.Bool(peer.Connected())
		// Split the address into host and port portions so we can do a DNS lookup against the host.  When no port is specified in the address, just use the address as the host.
		host, _, err := net.SplitHostPort(peer.Addr())
		if err != nil {
			host = peer.Addr()
		}
		var ipList []string
		switch {
		case net.ParseIP(host) != nil, strings.HasSuffix(host, ".onion"):
			ipList = make([]string, 1)
			ipList[0] = host
		default:
			// Do a DNS lookup for the address.  If the lookup fails, just use the host.
			ips, err := podLookup(host)
			if err != nil {
				ipList = make([]string, 1)
				ipList[0] = host
				break
			}
			ipList = make([]string, 0, len(ips))
			for _, ip := range ips {
				ipList = append(ipList, ip.String())
			}
		}
		// Add the addresses and connection info to the result.
		addrs := make([]json.GetAddedNodeInfoResultAddr, 0, len(ipList))
		for _, ip := range ipList {
			var addr json.GetAddedNodeInfoResultAddr
			addr.Address = ip
			addr.Connected = "false"
			if ip == host && peer.Connected() {
				addr.Connected = directionString(peer.Inbound())
			}
			addrs = append(addrs, addr)
		}
		result.Addresses = &addrs
		results = append(results, &result)
	}
	return results, nil
}
// handleGetBestBlock implements the getbestblock command.
func handleGetBestBlock(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// All other "get block" commands give either the height, the hash, or both but require the block SHA.  This gets both for the best block.
	best := s.cfg.Chain.BestSnapshot()
	result := &json.GetBestBlockResult{
		Hash:   best.Hash.String(),
		Height: best.Height,
	}
	return result, nil
}
// handleGetBestBlockHash implements the getbestblockhash command.
func handleGetBestBlockHash(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	best := s.cfg.Chain.BestSnapshot()
	return best.Hash.String(), nil
}
// getDifficultyRatio returns the proof-of-work difficulty as a multiple of the minimum difficulty using the passed bits field from the header of a block.
func getDifficultyRatio(bits uint32, params *chaincfg.Params, algo int32) float64 {
	// The minimum difficulty is the max possible proof-of-work limit bits converted back to a number.  Note this is not the same as the proof of work limit directly because the block difficulty is encoded in a block with the compact form which loses precision.
	max := blockchain.CompactToBig(0x1d00ffff)
	target := blockchain.CompactToBig(bits)
	difficulty := new(big.Rat).SetFrac(max, target)
	outString := difficulty.FloatString(8)
	diff, err := strconv.ParseFloat(outString, 64)
	if err != nil {
		log <- cl.Error{"cannot get difficulty:", err}
		return 0
	}
	return diff
}
// handleGetBlock implements the getblock command.
func handleGetBlock(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*json.GetBlockCmd)
	// Load the raw block bytes from the database.
	hash, err := chainhash.NewHashFromStr(c.Hash)
	if err != nil {
		return nil, rpcDecodeHexError(c.Hash)
	}
	var blkBytes []byte
	err = s.cfg.DB.View(func(dbTx database.Tx) error {
		var err error
		blkBytes, err = dbTx.FetchBlock(hash)
		return err
	})
	if err != nil {
		return nil, &json.RPCError{
			Code:    json.ErrRPCBlockNotFound,
			Message: "Block not found",
		}
	}
	// When the verbose flag isn't set, simply return the serialized block as a hex-encoded string.
	if c.Verbose != nil && !*c.Verbose {
		return hex.EncodeToString(blkBytes), nil
	}
	// The verbose flag is set, so generate the JSON object and return it. Deserialize the block.
	blk, err := util.NewBlockFromBytes(blkBytes)
	if err != nil {
		context := "Failed to deserialize block"
		return nil, internalRPCError(err.Error(), context)
	}
	// Get the block height from chain.
	blockHeight, err := s.cfg.Chain.BlockHeightByHash(hash)
	if err != nil {
		context := "Failed to obtain block height"
		return nil, internalRPCError(err.Error(), context)
	}
	blk.SetHeight(blockHeight)
	best := s.cfg.Chain.BestSnapshot()
	// Get next block hash unless there are none.
	var nextHashString string
	if blockHeight < best.Height {
		nextHash, err := s.cfg.Chain.BlockHashByHeight(blockHeight + 1)
		if err != nil {
			context := "No next block"
			return nil, internalRPCError(err.Error(), context)
		}
		nextHashString = nextHash.String()
	}
	params := s.cfg.ChainParams
	blockHeader := &blk.MsgBlock().Header
	algoname := fork.GetAlgoName(blockHeader.Version, blockHeight)
	a := fork.GetAlgoVer(algoname, blockHeight)
	algoid := fork.GetAlgoID(algoname, blockHeight)
	blockReply := json.GetBlockVerboseResult{
		Hash:          c.Hash,
		Version:       blockHeader.Version,
		VersionHex:    fmt.Sprintf("%08x", blockHeader.Version),
		PowAlgoID:     algoid,
		PowAlgo:       algoname,
		PowHash:       blk.MsgBlock().BlockHashWithAlgos(blockHeight).String(),
		MerkleRoot:    blockHeader.MerkleRoot.String(),
		PreviousHash:  blockHeader.PrevBlock.String(),
		Nonce:         blockHeader.Nonce,
		Time:          blockHeader.Timestamp.Unix(),
		Confirmations: int64(1 + best.Height - blockHeight),
		Height:        int64(blockHeight),
		Size:          int32(len(blkBytes)),
		StrippedSize:  int32(blk.MsgBlock().SerializeSizeStripped()),
		Weight:        int32(blockchain.GetBlockWeight(blk)),
		Bits:          strconv.FormatInt(int64(blockHeader.Bits), 16),
		Difficulty:    getDifficultyRatio(blockHeader.Bits, params, a),
		NextHash:      nextHashString,
	}
	if c.VerboseTx == nil || !*c.VerboseTx {
		transactions := blk.Transactions()
		txNames := make([]string, len(transactions))
		for i, tx := range transactions {
			txNames[i] = tx.Hash().String()
		}
		blockReply.Tx = txNames
	} else {
		txns := blk.Transactions()
		rawTxns := make([]json.TxRawResult, len(txns))
		for i, tx := range txns {
			rawTxn, err := createTxRawResult(params, tx.MsgTx(),
				tx.Hash().String(), blockHeader, hash.String(),
				blockHeight, best.Height)
			if err != nil {
				return nil, err
			}
			rawTxns[i] = *rawTxn
		}
		blockReply.RawTx = rawTxns
	}
	return blockReply, nil
}
// softForkStatus converts a ThresholdState state into a human readable string corresponding to the particular state.
func softForkStatus(state blockchain.ThresholdState) (string, error) {
	switch state {
	case blockchain.ThresholdDefined:
		return "defined", nil
	case blockchain.ThresholdStarted:
		return "started", nil
	case blockchain.ThresholdLockedIn:
		return "lockedin", nil
	case blockchain.ThresholdActive:
		return "active", nil
	case blockchain.ThresholdFailed:
		return "failed", nil
	default:
		return "", fmt.Errorf("unknown deployment state: %v", state)
	}
}
// handleGetBlockChainInfo implements the getblockchaininfo command.
func handleGetBlockChainInfo(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// Obtain a snapshot of the current best known blockchain state. We'll populate the response to this call primarily from this snapshot.
	params := s.cfg.ChainParams
	chain := s.cfg.Chain
	chainSnapshot := chain.BestSnapshot()
	chainInfo := &json.GetBlockChainInfoResult{
		Chain:         params.Name,
		Blocks:        chainSnapshot.Height,
		Headers:       chainSnapshot.Height,
		BestBlockHash: chainSnapshot.Hash.String(),
		Difficulty:    getDifficultyRatio(chainSnapshot.Bits, params, 2),
		MedianTime:    chainSnapshot.MedianTime.Unix(),
		Pruned:        false,
		Bip9SoftForks: make(map[string]*json.Bip9SoftForkDescription),
	}
	// Next, populate the response with information describing the current status of soft-forks deployed via the super-majority block signalling mechanism.
	height := chainSnapshot.Height
	chainInfo.SoftForks = []*json.SoftForkDescription{
		{
			ID:      "bip34",
			Version: 2,
			Reject: struct {
				Status bool `json:"status"`
			}{
				Status: height >= params.BIP0034Height,
			},
		},
		{
			ID:      "bip66",
			Version: 3,
			Reject: struct {
				Status bool `json:"status"`
			}{
				Status: height >= params.BIP0066Height,
			},
		},
		{
			ID:      "bip65",
			Version: 4,
			Reject: struct {
				Status bool `json:"status"`
			}{
				Status: height >= params.BIP0065Height,
			},
		},
	}
	// Finally, query the BIP0009 version bits state for all currently defined BIP0009 soft-fork deployments.
	for deployment, deploymentDetails := range params.Deployments {
		// Map the integer deployment ID into a human readable fork-name.
		var forkName string
		switch deployment {
		case chaincfg.DeploymentTestDummy:
			forkName = "dummy"
		case chaincfg.DeploymentCSV:
			forkName = "csv"
		case chaincfg.DeploymentSegwit:
			forkName = "segwit"
		default:
			return nil, &json.RPCError{
				Code: json.ErrRPCInternal.Code,
				Message: fmt.Sprintf("Unknown deployment %v "+
					"detected", deployment),
			}
		}
		// Query the chain for the current status of the deployment as identified by its deployment ID.
		deploymentStatus, err := chain.ThresholdState(uint32(deployment))
		if err != nil {
			context := "Failed to obtain deployment status"
			return nil, internalRPCError(err.Error(), context)
		}
		// Attempt to convert the current deployment status into a human readable string. If the status is unrecognized, then a non-nil error is returned.
		statusString, err := softForkStatus(deploymentStatus)
		if err != nil {
			return nil, &json.RPCError{
				Code: json.ErrRPCInternal.Code,
				Message: fmt.Sprintf("unknown deployment status: %v",
					deploymentStatus),
			}
		}
		// Finally, populate the soft-fork description with all the information gathered above.
		chainInfo.Bip9SoftForks[forkName] = &json.Bip9SoftForkDescription{
			Status:    strings.ToLower(statusString),
			Bit:       deploymentDetails.BitNumber,
			StartTime: int64(deploymentDetails.StartTime),
			Timeout:   int64(deploymentDetails.ExpireTime),
		}
	}
	return chainInfo, nil
}
// handleGetBlockCount implements the getblockcount command.
func handleGetBlockCount(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	best := s.cfg.Chain.BestSnapshot()
	return int64(best.Height), nil
}
// handleGetBlockHash implements the getblockhash command.
func handleGetBlockHash(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*json.GetBlockHashCmd)
	hash, err := s.cfg.Chain.BlockHashByHeight(int32(c.Index))
	if err != nil {
		return nil, &json.RPCError{
			Code:    json.ErrRPCOutOfRange,
			Message: "Block number out of range",
		}
	}
	return hash.String(), nil
}
// handleGetBlockHeader implements the getblockheader command.
func handleGetBlockHeader(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*json.GetBlockHeaderCmd)
	// Fetch the header from chain.
	hash, err := chainhash.NewHashFromStr(c.Hash)
	if err != nil {
		return nil, rpcDecodeHexError(c.Hash)
	}
	blockHeader, err := s.cfg.Chain.HeaderByHash(hash)
	if err != nil {
		return nil, &json.RPCError{
			Code:    json.ErrRPCBlockNotFound,
			Message: "Block not found",
		}
	}
	// When the verbose flag isn't set, simply return the serialized block header as a hex-encoded string.
	if c.Verbose != nil && !*c.Verbose {
		var headerBuf bytes.Buffer
		err := blockHeader.Serialize(&headerBuf)
		if err != nil {
			context := "Failed to serialize block header"
			return nil, internalRPCError(err.Error(), context)
		}
		return hex.EncodeToString(headerBuf.Bytes()), nil
	}
	// The verbose flag is set, so generate the JSON object and return it. Get the block height from chain.
	blockHeight, err := s.cfg.Chain.BlockHeightByHash(hash)
	if err != nil {
		context := "Failed to obtain block height"
		return nil, internalRPCError(err.Error(), context)
	}
	best := s.cfg.Chain.BestSnapshot()
	// Get next block hash unless there are none.
	var nextHashString string
	if blockHeight < best.Height {
		nextHash, err := s.cfg.Chain.BlockHashByHeight(blockHeight + 1)
		if err != nil {
			context := "No next block"
			return nil, internalRPCError(err.Error(), context)
		}
		nextHashString = nextHash.String()
	}
	var a int32 = 2
	if blockHeader.Version == 514 {
		a = 514
	}
	params := s.cfg.ChainParams
	blockHeaderReply := json.GetBlockHeaderVerboseResult{
		Hash:          c.Hash,
		Confirmations: int64(1 + best.Height - blockHeight),
		Height:        blockHeight,
		Version:       blockHeader.Version,
		VersionHex:    fmt.Sprintf("%08x", blockHeader.Version),
		MerkleRoot:    blockHeader.MerkleRoot.String(),
		NextHash:      nextHashString,
		PreviousHash:  blockHeader.PrevBlock.String(),
		Nonce:         uint64(blockHeader.Nonce),
		Time:          blockHeader.Timestamp.Unix(),
		Bits:          strconv.FormatInt(int64(blockHeader.Bits), 16),
		Difficulty:    getDifficultyRatio(blockHeader.Bits, params, a),
	}
	return blockHeaderReply, nil
}
// encodeTemplateID encodes the passed details into an ID that can be used to uniquely identify a block template.
func encodeTemplateID(prevHash *chainhash.Hash, lastGenerated time.Time) string {
	return fmt.Sprintf("%s-%d", prevHash.String(), lastGenerated.Unix())
}
// decodeTemplateID decodes an ID that is used to uniquely identify a block template.  This is mainly used as a mechanism to track when to update clients that are using long polling for block templates.  The ID consists of the previous block hash for the associated template and the time the associated template was generated.
func decodeTemplateID(templateID string) (*chainhash.Hash, int64, error) {
	fields := strings.Split(templateID, "-")
	if len(fields) != 2 {
		return nil, 0, errors.New("invalid longpollid format")
	}
	prevHash, err := chainhash.NewHashFromStr(fields[0])
	if err != nil {
		return nil, 0, errors.New("invalid longpollid format")
	}
	lastGenerated, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return nil, 0, errors.New("invalid longpollid format")
	}
	return prevHash, lastGenerated, nil
}
// notifyLongPollers notifies any channels that have been registered to be notified when block templates are stale. This function MUST be called with the state locked.
func (state *gbtWorkState) notifyLongPollers(latestHash *chainhash.Hash, lastGenerated time.Time) {
	// Notify anything that is waiting for a block template update from a hash which is not the hash of the tip of the best chain since their work is now invalid.
	for hash, channels := range state.notifyMap {
		if !hash.IsEqual(latestHash) {
			for _, c := range channels {
				close(c)
			}
			delete(state.notifyMap, hash)
		}
	}
	// Return now if the provided last generated timestamp has not been initialized.
	if lastGenerated.IsZero() {
		return
	}
	// Return now if there is nothing registered for updates to the current best block hash.
	channels, ok := state.notifyMap[*latestHash]
	if !ok {
		return
	}
	// Notify anything that is waiting for a block template update from a block template generated before the most recently generated block template.
	lastGeneratedUnix := lastGenerated.Unix()
	for lastGen, c := range channels {
		if lastGen < lastGeneratedUnix {
			close(c)
			delete(channels, lastGen)
		}
	}
	// Remove the entry altogether if there are no more registered channels.
	if len(channels) == 0 {
		delete(state.notifyMap, *latestHash)
	}
}
// NotifyBlockConnected uses the newly-connected block to notify any long poll clients with a new block template when their existing block template is stale due to the newly connected block.
func (state *gbtWorkState) NotifyBlockConnected(blockHash *chainhash.Hash) {
	go func() {
		state.Lock()
		defer state.Unlock()
		state.notifyLongPollers(blockHash, state.lastTxUpdate)
	}()
}
// NotifyMempoolTx uses the new last updated time for the transaction memory pool to notify any long poll clients with a new block template when their existing block template is stale due to enough time passing and the contents of the memory pool changing.
func (state *gbtWorkState) NotifyMempoolTx(lastUpdated time.Time) {
	go func() {
		state.Lock()
		defer state.Unlock()
		// No need to notify anything if no block templates have been generated yet.
		if state.prevHash == nil || state.lastGenerated.IsZero() {
			return
		}
		if time.Now().After(state.lastGenerated.Add(time.Second * gbtRegenerateSeconds)) {
			state.notifyLongPollers(state.prevHash, lastUpdated)
		}
	}()
}
// templateUpdateChan returns a channel that will be closed once the block template associated with the passed previous hash and last generated time is stale.  The function will return existing channels for duplicate parameters which allows  to wait for the same block template without requiring a different channel for each client. This function MUST be called with the state locked.
func (state *gbtWorkState) templateUpdateChan(prevHash *chainhash.Hash, lastGenerated int64) chan struct{} {
	// Either get the current list of channels waiting for updates about changes to block template for the previous hash or create a new one.
	channels, ok := state.notifyMap[*prevHash]
	if !ok {
		m := make(map[int64]chan struct{})
		state.notifyMap[*prevHash] = m
		channels = m
	}
	// Get the current channel associated with the time the block template was last generated or create a new one.
	c, ok := channels[lastGenerated]
	if !ok {
		c = make(chan struct{})
		channels[lastGenerated] = c
	}
	return c
}
// updateBlockTemplate creates or updates a block template for the work state. A new block template will be generated when the current best block has changed or the transactions in the memory pool have been updated and it has been long enough since the last template was generated.  Otherwise, the timestamp for the existing block template is updated (and possibly the difficulty on testnet per the consesus rules).  Finally, if the useCoinbaseValue flag is false and the existing block template does not already contain a valid payment address, the block template will be updated with a randomly selected payment address from the list of configured addresses. This function MUST be called with the state locked.
func (state *gbtWorkState) updateBlockTemplate(s *rpcServer, useCoinbaseValue bool) error {
	generator := s.cfg.Generator
	lastTxUpdate := generator.TxSource().LastUpdated()
	if lastTxUpdate.IsZero() {
		lastTxUpdate = time.Now()
	}
	// Generate a new block template when the current best block has changed or the transactions in the memory pool have been updated and it has been at least gbtRegenerateSecond since the last template was generated.
	var msgBlock *wire.MsgBlock
	var targetDifficulty string
	latestHash := &s.cfg.Chain.BestSnapshot().Hash
	template := state.template
	if template == nil || state.prevHash == nil ||
		!state.prevHash.IsEqual(latestHash) ||
		(state.lastTxUpdate != lastTxUpdate &&
			time.Now().After(state.lastGenerated.Add(time.Second*
				gbtRegenerateSeconds))) {
		// Reset the previous best hash the block template was generated against so any errors below cause the next invocation to try again.
		state.prevHash = nil
		// Choose a payment address at random if the caller requests a full coinbase as opposed to only the pertinent details needed to create their own coinbase.
		var payAddr util.Address
		if !useCoinbaseValue {
			payAddr = StateCfg.ActiveMiningAddrs[rand.Intn(len(StateCfg.ActiveMiningAddrs))]
		}
		// Create a new block template that has a coinbase which anyone can redeem.  This is only acceptable because the returned block template doesn't include the coinbase, so the caller will ultimately create their own coinbase which pays to the appropriate address(es).
		blkTemplate, err := generator.NewBlockTemplate(payAddr, state.algo)
		if err != nil {
			return internalRPCError("(rpcserver.go) Failed to create new block "+
				"template: "+err.Error(), "")
		}
		template = blkTemplate
		msgBlock = template.Block
		targetDifficulty = fmt.Sprintf("%064x",
			blockchain.CompactToBig(msgBlock.Header.Bits))
		// Get the minimum allowed timestamp for the block based on the median timestamp of the last several blocks per the chain consensus rules.
		best := s.cfg.Chain.BestSnapshot()
		minTimestamp := mining.MinimumMedianTime(best)
		// Update work state to ensure another block template isn't generated until needed.
		state.template = template
		state.lastGenerated = time.Now()
		state.lastTxUpdate = lastTxUpdate
		state.prevHash = latestHash
		state.minTimestamp = minTimestamp
		log <- cl.Debugf{
			"generated block template (timestamp %v, target %s, merkle root %s)",
			msgBlock.Header.Timestamp,
			targetDifficulty,
			msgBlock.Header.MerkleRoot,
		}
		// Notify any clients that are long polling about the new template.
		state.notifyLongPollers(latestHash, lastTxUpdate)
	} else {
		// At this point, there is a saved block template and another request for a template was made, but either the available transactions haven't change or it hasn't been long enough to trigger a new block template to be generated.  So, update the existing block template. When the caller requires a full coinbase as opposed to only the pertinent details needed to create their own coinbase, add a payment address to the output of the coinbase of the template if it doesn't already have one.  Since this requires mining addresses to be specified via the config, an error is returned if none have been specified.
		if !useCoinbaseValue && !template.ValidPayAddress {
			// Choose a payment address at random.
			payToAddr := StateCfg.ActiveMiningAddrs[rand.Intn(len(StateCfg.ActiveMiningAddrs))]
			// Update the block coinbase output of the template to pay to the randomly selected payment address.
			pkScript, err := txscript.PayToAddrScript(payToAddr)
			if err != nil {
				context := "Failed to create pay-to-addr script"
				return internalRPCError(err.Error(), context)
			}
			template.Block.Transactions[0].TxOut[0].PkScript = pkScript
			template.ValidPayAddress = true
			// Update the merkle root.
			block := util.NewBlock(template.Block)
			merkles := blockchain.BuildMerkleTreeStore(block.Transactions(), false)
			template.Block.Header.MerkleRoot = *merkles[len(merkles)-1]
		}
		// Set locals for convenience.
		msgBlock = template.Block
		targetDifficulty = fmt.Sprintf("%064x",
			blockchain.CompactToBig(msgBlock.Header.Bits))
		// Update the time of the block template to the current time while accounting for the median time of the past several blocks per the chain consensus rules.
		generator.UpdateBlockTime(msgBlock)
		msgBlock.Header.Nonce = 0
		log <- cl.Debugf{
			"updated block template (timestamp %v, target %s)",
			msgBlock.Header.Timestamp,
			targetDifficulty,
		}
	}
	return nil
}
// blockTemplateResult returns the current block template associated with the state as a json.GetBlockTemplateResult that is ready to be encoded to JSON and returned to the caller. This function MUST be called with the state locked.
func (state *gbtWorkState) blockTemplateResult(useCoinbaseValue bool, submitOld *bool) (*json.GetBlockTemplateResult, error) {
	// Ensure the timestamps are still in valid range for the template. This should really only ever happen if the local clock is changed after the template is generated, but it's important to avoid serving invalid block templates.
	template := state.template
	msgBlock := template.Block
	header := &msgBlock.Header
	adjustedTime := state.timeSource.AdjustedTime()
	maxTime := adjustedTime.Add(time.Second * blockchain.MaxTimeOffsetSeconds)
	if header.Timestamp.After(maxTime) {
		return nil, &json.RPCError{
			Code: json.ErrRPCOutOfRange,
			Message: fmt.Sprintf("The template time is after the "+
				"maximum allowed time for a block - template "+
				"time %v, maximum time %v", adjustedTime,
				maxTime),
		}
	}
	// Convert each transaction in the block template to a template result transaction.  The result does not include the coinbase, so notice the adjustments to the various lengths and indices.
	numTx := len(msgBlock.Transactions)
	transactions := make([]json.GetBlockTemplateResultTx, 0, numTx-1)
	txIndex := make(map[chainhash.Hash]int64, numTx)
	for i, tx := range msgBlock.Transactions {
		txHash := tx.TxHash()
		txIndex[txHash] = int64(i)
		// Skip the coinbase transaction.
		if i == 0 {
			continue
		}
		// Create an array of 1-based indices to transactions that come before this one in the transactions list which this one depends on.  This is necessary since the created block must ensure proper ordering of the dependencies.  A map is used before creating the final array to prevent duplicate entries when multiple inputs reference the same transaction.
		dependsMap := make(map[int64]struct{})
		for _, txIn := range tx.TxIn {
			if idx, ok := txIndex[txIn.PreviousOutPoint.Hash]; ok {
				dependsMap[idx] = struct{}{}
			}
		}
		depends := make([]int64, 0, len(dependsMap))
		for idx := range dependsMap {
			depends = append(depends, idx)
		}
		// Serialize the transaction for later conversion to hex.
		txBuf := bytes.NewBuffer(make([]byte, 0, tx.SerializeSize()))
		if err := tx.Serialize(txBuf); err != nil {
			context := "Failed to serialize transaction"
			return nil, internalRPCError(err.Error(), context)
		}
		bTx := util.NewTx(tx)
		resultTx := json.GetBlockTemplateResultTx{
			Data:    hex.EncodeToString(txBuf.Bytes()),
			Hash:    txHash.String(),
			Depends: depends,
			Fee:     template.Fees[i],
			SigOps:  template.SigOpCosts[i],
			Weight:  blockchain.GetTransactionWeight(bTx),
		}
		transactions = append(transactions, resultTx)
	}
	// Generate the block template reply.  Note that following mutations are implied by the included or omission of fields:  Including MinTime -> time/decrement  Omitting CoinbaseTxn -> coinbase, generation
	targetDifficulty := fmt.Sprintf("%064x", blockchain.CompactToBig(header.Bits))
	templateID := encodeTemplateID(state.prevHash, state.lastGenerated)
	reply := json.GetBlockTemplateResult{
		Bits:         strconv.FormatInt(int64(header.Bits), 16),
		CurTime:      header.Timestamp.Unix(),
		Height:       int64(template.Height),
		PreviousHash: header.PrevBlock.String(),
		WeightLimit:  blockchain.MaxBlockWeight,
		SigOpLimit:   blockchain.MaxBlockSigOpsCost,
		SizeLimit:    wire.MaxBlockPayload,
		Transactions: transactions,
		Version:      header.Version,
		LongPollID:   templateID,
		SubmitOld:    submitOld,
		Target:       targetDifficulty,
		MinTime:      state.minTimestamp.Unix(),
		MaxTime:      maxTime.Unix(),
		Mutable:      gbtMutableFields,
		NonceRange:   gbtNonceRange,
		Capabilities: gbtCapabilities,
	}
	// If the generated block template includes transactions with witness data, then include the witness commitment in the GBT result.
	if template.WitnessCommitment != nil {
		reply.DefaultWitnessCommitment = hex.EncodeToString(template.WitnessCommitment)
	}
	if useCoinbaseValue {
		reply.CoinbaseAux = gbtCoinbaseAux
		reply.CoinbaseValue = &msgBlock.Transactions[0].TxOut[0].Value
	} else {
		// Ensure the template has a valid payment address associated with it when a full coinbase is requested.
		if !template.ValidPayAddress {
			return nil, &json.RPCError{
				Code: json.ErrRPCInternal.Code,
				Message: "A coinbase transaction has been " +
					"requested, but the server has not " +
					"been configured with any payment " +
					"addresses via --miningaddr",
			}
		}
		// Serialize the transaction for conversion to hex.
		tx := msgBlock.Transactions[0]
		txBuf := bytes.NewBuffer(make([]byte, 0, tx.SerializeSize()))
		if err := tx.Serialize(txBuf); err != nil {
			context := "Failed to serialize transaction"
			return nil, internalRPCError(err.Error(), context)
		}
		resultTx := json.GetBlockTemplateResultTx{
			Data:    hex.EncodeToString(txBuf.Bytes()),
			Hash:    tx.TxHash().String(),
			Depends: []int64{},
			Fee:     template.Fees[0],
			SigOps:  template.SigOpCosts[0],
		}
		reply.CoinbaseTxn = &resultTx
	}
	return &reply, nil
}
// handleGetBlockTemplateLongPoll is a helper for handleGetBlockTemplateRequest which deals with handling long polling for block templates.  When a caller sends a request with a long poll ID that was previously returned, a response is not sent until the caller should stop working on the previous block template in favor of the new one.  In particular, this is the case when the old block template is no longer valid due to a solution already being found and added to the block chain, or new transactions have shown up and some time has passed without finding a solution. See https://en.bitcoin.it/wiki/BIP_0022 for more details.
func handleGetBlockTemplateLongPoll(s *rpcServer, longPollID string, useCoinbaseValue bool, closeChan <-chan struct{}) (interface{}, error) {
	state := s.gbtWorkState
	state.Lock()
	// The state unlock is intentionally not deferred here since it needs to be manually unlocked before waiting for a notification about block template changes.
	if err := state.updateBlockTemplate(s, useCoinbaseValue); err != nil {
		state.Unlock()
		return nil, err
	}
	// Just return the current block template if the long poll ID provided by the caller is invalid.
	prevHash, lastGenerated, err := decodeTemplateID(longPollID)
	if err != nil {
		result, err := state.blockTemplateResult(useCoinbaseValue, nil)
		if err != nil {
			state.Unlock()
			return nil, err
		}
		state.Unlock()
		return result, nil
	}
	// Return the block template now if the specific block template/ identified by the long poll ID no longer matches the current block template as this means the provided template is stale.
	prevTemplateHash := &state.template.Block.Header.PrevBlock
	if !prevHash.IsEqual(prevTemplateHash) ||
		lastGenerated != state.lastGenerated.Unix() {
		// Include whether or not it is valid to submit work against the old block template depending on whether or not a solution has already been found and added to the block chain.
		submitOld := prevHash.IsEqual(prevTemplateHash)
		result, err := state.blockTemplateResult(useCoinbaseValue,
			&submitOld)
		if err != nil {
			state.Unlock()
			return nil, err
		}
		state.Unlock()
		return result, nil
	}
	// Register the previous hash and last generated time for notifications Get a channel that will be notified when the template associated with the provided ID is stale and a new block template should be returned to the caller.
	longPollChan := state.templateUpdateChan(prevHash, lastGenerated)
	state.Unlock()
	select {
	// When the client closes before it's time to send a reply, just return now so the goroutine doesn't hang around.
	case <-closeChan:
		// fmt.Println("chan:<-closeChan")
		return nil, ErrClientQuit
	// Wait until signal received to send the reply.
	case <-longPollChan:
		// fmt.Println("chan:<-longPollChan")
		// Fallthrough
	}
	// Get the lastest block template
	state.Lock()
	defer state.Unlock()
	if err := state.updateBlockTemplate(s, useCoinbaseValue); err != nil {
		return nil, err
	}
	// Include whether or not it is valid to submit work against the old block template depending on whether or not a solution has already been found and added to the block chain.
	submitOld := prevHash.IsEqual(&state.template.Block.Header.PrevBlock)
	result, err := state.blockTemplateResult(useCoinbaseValue, &submitOld)
	if err != nil {
		return nil, err
	}
	return result, nil
}
// handleGetBlockTemplateRequest is a helper for handleGetBlockTemplate which deals with generating and returning block templates to the caller.  It handles both long poll requests as specified by BIP 0022 as well as regular requests.  In addition, it detects the capabilities reported by the caller in regards to whether or not it supports creating its own coinbase (the coinbasetxn and coinbasevalue capabilities) and modifies the returned block template accordingly.
func handleGetBlockTemplateRequest(s *rpcServer, request *json.TemplateRequest, closeChan <-chan struct{}) (interface{}, error) {
	// Extract the relevant passed capabilities and restrict the result to either a coinbase value or a coinbase transaction object depending on the request.  Default to only providing a coinbase value.
	useCoinbaseValue := true
	if request != nil {
		var hasCoinbaseValue, hasCoinbaseTxn bool
		for _, capability := range request.Capabilities {
			switch capability {
			case "coinbasetxn":
				hasCoinbaseTxn = true
			case "coinbasevalue":
				hasCoinbaseValue = true
			}
		}
		if hasCoinbaseTxn && !hasCoinbaseValue {
			useCoinbaseValue = false
		}
	}
	// When a coinbase transaction has been requested, respond with an error if there are no addresses to pay the created block template to.
	if !useCoinbaseValue && len(StateCfg.ActiveMiningAddrs) == 0 {
		return nil, &json.RPCError{
			Code: json.ErrRPCInternal.Code,
			Message: "A coinbase transaction has been requested, " +
				"but the server has not been configured with " +
				"any payment addresses via --miningaddr",
		}
	}
	// Return an error if there are no peers connected since there is no way to relay a found block or receive transactions to work on. However, allow this state when running in the regression test or simulation test mode.
	if !(cfg.RegressionTest || cfg.SimNet) &&
		s.cfg.ConnMgr.ConnectedCount() == 0 {
		return nil, &json.RPCError{
			Code:    json.ErrRPCClientNotConnected,
			Message: "Pod is not connected to network",
		}
	}
	// No point in generating or accepting work before the chain is synced.
	currentHeight := s.cfg.Chain.BestSnapshot().Height
	if currentHeight != 0 && !s.cfg.SyncMgr.IsCurrent() {
		return nil, &json.RPCError{
			Code:    json.ErrRPCClientInInitialDownload,
			Message: "Pod is not yet synchronised...",
		}
	}
	// When a long poll ID was provided, this is a long poll request by the client to be notified when block template referenced by the ID should be replaced with a new one.
	if request != nil && request.LongPollID != "" {
		return handleGetBlockTemplateLongPoll(s, request.LongPollID,
			useCoinbaseValue, closeChan)
	}
	// Protect concurrent access when updating block templates.
	state := s.gbtWorkState
	state.Lock()
	defer state.Unlock()
	// Get and return a block template.  A new block template will be generated when the current best block has changed or the transactions in the memory pool have been updated and it has been at least five seconds since the last template was generated.  Otherwise, the timestamp for the existing block template is updated (and possibly the difficulty on testnet per the consesus rules).
	if err := state.updateBlockTemplate(s, useCoinbaseValue); err != nil {
		return nil, err
	}
	return state.blockTemplateResult(useCoinbaseValue, nil)
}
// chainErrToGBTErrString converts an error returned from btcchain to a string which matches the reasons and format described in BIP0022 for rejection reasons.
func chainErrToGBTErrString(err error) string {
	// When the passed error is not a RuleError, just return a generic rejected string with the error text.
	ruleErr, ok := err.(blockchain.RuleError)
	if !ok {
		return "rejected: " + err.Error()
	}
	switch ruleErr.ErrorCode {
	case blockchain.ErrDuplicateBlock:
		return "duplicate"
	case blockchain.ErrBlockTooBig:
		return "bad-blk-length"
	case blockchain.ErrBlockWeightTooHigh:
		return "bad-blk-weight"
	case blockchain.ErrBlockVersionTooOld:
		return "bad-version"
	case blockchain.ErrInvalidTime:
		return "bad-time"
	case blockchain.ErrTimeTooOld:
		return "time-too-old"
	case blockchain.ErrTimeTooNew:
		return "time-too-new"
	case blockchain.ErrDifficultyTooLow:
		return "bad-diffbits"
	case blockchain.ErrUnexpectedDifficulty:
		return "bad-diffbits"
	case blockchain.ErrHighHash:
		return "high-hash"
	case blockchain.ErrBadMerkleRoot:
		return "bad-txnmrklroot"
	case blockchain.ErrBadCheckpoint:
		return "bad-checkpoint"
	case blockchain.ErrForkTooOld:
		return "fork-too-old"
	case blockchain.ErrCheckpointTimeTooOld:
		return "checkpoint-time-too-old"
	case blockchain.ErrNoTransactions:
		return "bad-txns-none"
	case blockchain.ErrNoTxInputs:
		return "bad-txns-noinputs"
	case blockchain.ErrNoTxOutputs:
		return "bad-txns-nooutputs"
	case blockchain.ErrTxTooBig:
		return "bad-txns-size"
	case blockchain.ErrBadTxOutValue:
		return "bad-txns-outputvalue"
	case blockchain.ErrDuplicateTxInputs:
		return "bad-txns-dupinputs"
	case blockchain.ErrBadTxInput:
		return "bad-txns-badinput"
	case blockchain.ErrMissingTxOut:
		return "bad-txns-missinginput"
	case blockchain.ErrUnfinalizedTx:
		return "bad-txns-unfinalizedtx"
	case blockchain.ErrDuplicateTx:
		return "bad-txns-duplicate"
	case blockchain.ErrOverwriteTx:
		return "bad-txns-overwrite"
	case blockchain.ErrImmatureSpend:
		return "bad-txns-maturity"
	case blockchain.ErrSpendTooHigh:
		return "bad-txns-highspend"
	case blockchain.ErrBadFees:
		return "bad-txns-fees"
	case blockchain.ErrTooManySigOps:
		return "high-sigops"
	case blockchain.ErrFirstTxNotCoinbase:
		return "bad-txns-nocoinbase"
	case blockchain.ErrMultipleCoinbases:
		return "bad-txns-multicoinbase"
	case blockchain.ErrBadCoinbaseScriptLen:
		return "bad-cb-length"
	case blockchain.ErrBadCoinbaseValue:
		return "bad-cb-value"
	case blockchain.ErrMissingCoinbaseHeight:
		return "bad-cb-height"
	case blockchain.ErrBadCoinbaseHeight:
		return "bad-cb-height"
	case blockchain.ErrScriptMalformed:
		return "bad-script-malformed"
	case blockchain.ErrScriptValidation:
		return "bad-script-validate"
	case blockchain.ErrUnexpectedWitness:
		return "unexpected-witness"
	case blockchain.ErrInvalidWitnessCommitment:
		return "bad-witness-nonce-size"
	case blockchain.ErrWitnessCommitmentMismatch:
		return "bad-witness-merkle-match"
	case blockchain.ErrPreviousBlockUnknown:
		return "prev-blk-not-found"
	case blockchain.ErrInvalidAncestorBlock:
		return "bad-prevblk"
	case blockchain.ErrPrevBlockNotBest:
		return "inconclusive-not-best-prvblk"
	}
	return "rejected: " + err.Error()
}
// handleGetBlockTemplateProposal is a helper for handleGetBlockTemplate which deals with block proposals. See https://en.bitcoin.it/wiki/BIP_0023 for more details.
func handleGetBlockTemplateProposal(s *rpcServer, request *json.TemplateRequest) (interface{}, error) {
	hexData := request.Data
	if hexData == "" {
		return false, &json.RPCError{
			Code: json.ErrRPCType,
			Message: fmt.Sprintf("Data must contain the " +
				"hex-encoded serialized block that is being " +
				"proposed"),
		}
	}
	// Ensure the provided data is sane and deserialize the proposed block.
	if len(hexData)%2 != 0 {
		hexData = "0" + hexData
	}
	dataBytes, err := hex.DecodeString(hexData)
	if err != nil {
		return false, &json.RPCError{
			Code:    json.ErrRPCDeserialization,
			Message: fmt.Sprintf("data must be hexadecimal string (not %q)", hexData),
		}
	}
	var msgBlock wire.MsgBlock
	if err := msgBlock.Deserialize(bytes.NewReader(dataBytes)); err != nil {
		return nil, &json.RPCError{
			Code:    json.ErrRPCDeserialization,
			Message: "block decode failed: " + err.Error(),
		}
	}
	block := util.NewBlock(&msgBlock)
	// Ensure the block is building from the expected previous block.
	expectedPrevHash := s.cfg.Chain.BestSnapshot().Hash
	prevHash := &block.MsgBlock().Header.PrevBlock
	if !expectedPrevHash.IsEqual(prevHash) {
		return "bad-prevblk", nil
	}
	if err := s.cfg.Chain.CheckConnectBlockTemplate(block); err != nil {
		if _, ok := err.(blockchain.RuleError); !ok {
			errStr := fmt.Sprintf("failed to process block proposal: %v", err)
			log <- cl.Err(errStr)
			return nil, &json.RPCError{
				Code:    json.ErrRPCVerify,
				Message: errStr,
			}
		}
		log <- cl.Info{"rejected block proposal:", err}
		return chainErrToGBTErrString(err), nil
	}
	return nil, nil
}
// handleGetBlockTemplate implements the getblocktemplate command. See https://en.bitcoin.it/wiki/BIP_0022 and https://en.bitcoin.it/wiki/BIP_0023 for more details.
func handleGetBlockTemplate(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*json.GetBlockTemplateCmd)
	request := c.Request
	// Set the default mode and override it if supplied.
	mode := "template"
	if request != nil && request.Mode != "" {
		mode = request.Mode
	}
	switch mode {
	case "template":
		return handleGetBlockTemplateRequest(s, request, closeChan)
	case "proposal":
		return handleGetBlockTemplateProposal(s, request)
	}
	return nil, &json.RPCError{
		Code:    json.ErrRPCInvalidParameter,
		Message: "Invalid mode",
	}
}
// handleGetCFilter implements the getcfilter command.
func handleGetCFilter(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	if s.cfg.CfIndex == nil {
		return nil, &json.RPCError{
			Code:    json.ErrRPCNoCFIndex,
			Message: "The CF index must be enabled for this command",
		}
	}
	c := cmd.(*json.GetCFilterCmd)
	hash, err := chainhash.NewHashFromStr(c.Hash)
	if err != nil {
		return nil, rpcDecodeHexError(c.Hash)
	}
	filterBytes, err := s.cfg.CfIndex.FilterByBlockHash(hash, c.FilterType)
	if err != nil {
		log <- cl.Debugf{
			"could not find committed filter for %v: %v",
			hash,
			err,
		}
		return nil, &json.RPCError{
			Code:    json.ErrRPCBlockNotFound,
			Message: "block not found",
		}
	}
	log <- cl.Debug{"found committed filter for", hash}
	return hex.EncodeToString(filterBytes), nil
}
// handleGetCFilterHeader implements the getcfilterheader command.
func handleGetCFilterHeader(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	if s.cfg.CfIndex == nil {
		return nil, &json.RPCError{
			Code:    json.ErrRPCNoCFIndex,
			Message: "The CF index must be enabled for this command",
		}
	}
	c := cmd.(*json.GetCFilterHeaderCmd)
	hash, err := chainhash.NewHashFromStr(c.Hash)
	if err != nil {
		return nil, rpcDecodeHexError(c.Hash)
	}
	headerBytes, err := s.cfg.CfIndex.FilterHeaderByBlockHash(hash, c.FilterType)
	if len(headerBytes) > 0 {
		log <- cl.Debug{"found header of committed filter for", hash}
	} else {
		log <- cl.Debugf{
			"could not find header of committed filter for %v: %v",
			hash,
			err,
		}
		return nil, &json.RPCError{
			Code:    json.ErrRPCBlockNotFound,
			Message: "Block not found",
		}
	}
	hash.SetBytes(headerBytes)
	return hash.String(), nil
}
// handleGetConnectionCount implements the getconnectioncount command.
func handleGetConnectionCount(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	return s.cfg.ConnMgr.ConnectedCount(), nil
}
// handleGetCurrentNet implements the getcurrentnet command.
func handleGetCurrentNet(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	return s.cfg.ChainParams.Net, nil
}
// handleGetDifficulty implements the getdifficulty command. TODO: This command should default to the configured algo for cpu mining and take an optional parameter to query by algo
func handleGetDifficulty(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*json.GetDifficultyCmd)
	best := s.cfg.Chain.BestSnapshot()
	prev, err := s.cfg.Chain.BlockByHash(&best.Hash)
	if err != nil {
		fmt.Println("ERROR", err)
	}
	var algo = prev.MsgBlock().Header.Version
	if algo != 514 {
		algo = 2
	}
	bestbits := best.Bits
	if c.Algo == "scrypt" && algo != 514 {
		algo = 514
		for {
			if prev.MsgBlock().Header.Version != 514 {
				ph := prev.MsgBlock().Header.PrevBlock
				prev, err = s.cfg.Chain.BlockByHash(&ph)
				if err != nil {
					fmt.Println("ERROR", err)
				}
				continue
			}
			bestbits = uint32(prev.MsgBlock().Header.Bits)
			break
		}
	}
	if c.Algo == "sha256d" && algo != 2 {
		algo = 2
		for {
			if prev.MsgBlock().Header.Version == 514 {
				ph := prev.MsgBlock().Header.PrevBlock
				prev, err = s.cfg.Chain.BlockByHash(&ph)
				if err != nil {
					fmt.Println("ERROR", err)
				}
				continue
			}
			bestbits = uint32(prev.MsgBlock().Header.Bits)
			break
		}
	}
	return getDifficultyRatio(bestbits, s.cfg.ChainParams, algo), nil
}
// handleGetGenerate implements the getgenerate command.
func handleGetGenerate(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	return s.cfg.CPUMiner.IsMining(), nil
}
// handleGetHashesPerSec implements the gethashespersec command.
func handleGetHashesPerSec(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	return int64(s.cfg.CPUMiner.HashesPerSecond()), nil
}
// handleGetHeaders implements the getheaders command. NOTE: This is a btcsuite extension originally ported from github.com/decred/dcrd.
func handleGetHeaders(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*json.GetHeadersCmd)
	// Fetch the requested headers from chain while respecting the provided block locators and stop hash.
	blockLocators := make([]*chainhash.Hash, len(c.BlockLocators))
	for i := range c.BlockLocators {
		blockLocator, err := chainhash.NewHashFromStr(c.BlockLocators[i])
		if err != nil {
			return nil, rpcDecodeHexError(c.BlockLocators[i])
		}
		blockLocators[i] = blockLocator
	}
	var hashStop chainhash.Hash
	if c.HashStop != "" {
		err := chainhash.Decode(&hashStop, c.HashStop)
		if err != nil {
			return nil, rpcDecodeHexError(c.HashStop)
		}
	}
	headers := s.cfg.SyncMgr.LocateHeaders(blockLocators, &hashStop)
	// Return the serialized block headers as hex-encoded strings.
	hexBlockHeaders := make([]string, len(headers))
	var buf bytes.Buffer
	for i, h := range headers {
		err := h.Serialize(&buf)
		if err != nil {
			return nil, internalRPCError(err.Error(), "Failed to serialize block header")
		}
		hexBlockHeaders[i] = hex.EncodeToString(buf.Bytes())
		buf.Reset()
	}
	return hexBlockHeaders, nil
}
// handleGetInfo implements the getinfo command. We only return the fields that are not related to wallet functionality.
func handleGetInfo(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (ret interface{}, err error) {
	var Difficulty, dSHA256D, dScrypt, dBlake14lr, dCryptonight7v2, dLyra2rev2, dSkein, dX11, dStribog, dKeccak float64
	var lastbitsSHA256D, lastbitsScrypt, lastbitsBlake14lr, lastbitsCryptonight7v2, lastbitsLyra2rev2, lastbitsSkein, lastbitsX11, lastbitsStribog, lastbitsKeccak uint32
	best := s.cfg.Chain.BestSnapshot()
	v := s.cfg.Chain.Index.LookupNode(&best.Hash)
	foundcount, height := 0, best.Height
	switch fork.GetCurrent(height) {
	case 0:
		for foundcount < 9 && height > 0 {
			switch fork.GetAlgoName(v.Header().Version, height) {
			case "sha256d":
				if lastbitsSHA256D == 0 {
					foundcount++
					lastbitsSHA256D = v.Header().Bits
					dSHA256D = getDifficultyRatio(lastbitsSHA256D, s.cfg.ChainParams, v.Header().Version)
				}
			case "scrypt":
				if lastbitsScrypt == 0 {
					foundcount++
					lastbitsScrypt = v.Header().Bits
					dScrypt = getDifficultyRatio(lastbitsScrypt, s.cfg.ChainParams, v.Header().Version)
				}
			default:
			}
			v = v.RelativeAncestor(1)
			height--
		}
		switch s.cfg.Algo {
		case "sha256d":
			Difficulty = dSHA256D
		case "scrypt":
			Difficulty = dScrypt
		default:
		}
		ret = &json.InfoChainResult0{
			Version:           int32(1000000*appMajor + 10000*appMinor + 100*appPatch),
			ProtocolVersion:   int32(maxProtocolVersion),
			Blocks:            best.Height,
			TimeOffset:        int64(s.cfg.TimeSource.Offset().Seconds()),
			Connections:       s.cfg.ConnMgr.ConnectedCount(),
			Proxy:             cfg.Proxy,
			PowAlgoID:         fork.GetAlgoID(s.cfg.Algo, height),
			PowAlgo:           s.cfg.Algo,
			Difficulty:        Difficulty,
			DifficultySHA256D: dSHA256D,
			DifficultyScrypt:  dScrypt,
			TestNet:           cfg.TestNet3,
			RelayFee:          StateCfg.ActiveMinRelayTxFee.ToDUO(),
		}
	case 1:
		foundcount, height := 0, best.Height
		for foundcount < 9 &&
			height > fork.List[fork.GetCurrent(height)].ActivationHeight-512 {
			switch fork.GetAlgoName(v.Header().Version, height) {
			case "sha256d":
				if lastbitsSHA256D == 0 {
					foundcount++
					lastbitsSHA256D = v.Header().Bits
					dSHA256D = getDifficultyRatio(lastbitsSHA256D, s.cfg.ChainParams, v.Header().Version)
				}
			case "blake14lr":
				if lastbitsBlake14lr == 0 {
					foundcount++
					lastbitsBlake14lr = v.Header().Bits
					dBlake14lr = getDifficultyRatio(lastbitsBlake14lr, s.cfg.ChainParams, v.Header().Version)
				}
			case "whirlpool":
				if lastbitsCryptonight7v2 == 0 {
					foundcount++
					lastbitsCryptonight7v2 = v.Header().Bits
					dCryptonight7v2 = getDifficultyRatio(lastbitsCryptonight7v2, s.cfg.ChainParams, v.Header().Version)
				}
			case "lyra2rev2":
				if lastbitsLyra2rev2 == 0 {
					foundcount++
					lastbitsLyra2rev2 = v.Header().Bits
					dLyra2rev2 = getDifficultyRatio(lastbitsLyra2rev2, s.cfg.ChainParams, v.Header().Version)
				}
			case "skein":
				if lastbitsSkein == 0 {
					foundcount++
					lastbitsSkein = v.Header().Bits
					dSkein = getDifficultyRatio(lastbitsSkein, s.cfg.ChainParams, v.Header().Version)
				}
			case "x11":
				if lastbitsX11 == 0 {
					foundcount++
					lastbitsX11 = v.Header().Bits
					dX11 = getDifficultyRatio(lastbitsX11, s.cfg.ChainParams, v.Header().Version)
				}
			case "stribog":
				if lastbitsStribog == 0 {
					foundcount++
					lastbitsStribog = v.Header().Bits
					dStribog = getDifficultyRatio(lastbitsStribog, s.cfg.ChainParams, v.Header().Version)
				}
			case "keccak":
				if lastbitsKeccak == 0 {
					foundcount++
					lastbitsKeccak = v.Header().Bits
					dKeccak = getDifficultyRatio(lastbitsKeccak, s.cfg.ChainParams, v.Header().Version)
				}
			case "scrypt":
				if lastbitsScrypt == 0 {
					foundcount++
					lastbitsScrypt = v.Header().Bits
					dScrypt = getDifficultyRatio(lastbitsScrypt, s.cfg.ChainParams, v.Header().Version)
				}
			default:
			}
			v = v.RelativeAncestor(1)
			height--
		}
		switch s.cfg.Algo {
		case "sha256d":
			Difficulty = dSHA256D
		case "blake14lr":
			Difficulty = dBlake14lr
		case "cryptonight7v2":
			Difficulty = dCryptonight7v2
		case "lyra2rev2":
			Difficulty = dLyra2rev2
		case "skein":
			Difficulty = dSkein
		case "x11":
			Difficulty = dX11
		case "stribog":
			Difficulty = dStribog
		case "keccak":
			Difficulty = dKeccak
		case "scrypt":
			Difficulty = dScrypt
		default:
		}
		ret = &json.InfoChainResult{
			Version:                  int32(1000000*appMajor + 10000*appMinor + 100*appPatch),
			ProtocolVersion:          int32(maxProtocolVersion),
			Blocks:                   best.Height,
			TimeOffset:               int64(s.cfg.TimeSource.Offset().Seconds()),
			Connections:              s.cfg.ConnMgr.ConnectedCount(),
			Proxy:                    cfg.Proxy,
			PowAlgoID:                fork.GetAlgoID(s.cfg.Algo, height),
			PowAlgo:                  s.cfg.Algo,
			Difficulty:               Difficulty,
			DifficultyScrypt:         dScrypt,
			DifficultyBlake14lr:      dBlake14lr,
			DifficultyCryptonight7v2: dCryptonight7v2,
			DifficultyLyra2rev2:      dLyra2rev2,
			DifficultySkein:          dSkein,
			DifficultySHA256D:        dSHA256D,
			DifficultyX11:            dX11,
			DifficultyStribog:        dStribog,
			DifficultyKeccak:         dKeccak,
			TestNet:                  cfg.TestNet3,
			RelayFee:                 StateCfg.ActiveMinRelayTxFee.ToDUO(),
		}
	}
	return ret, nil
}
// handleGetMempoolInfo implements the getmempoolinfo command.
func handleGetMempoolInfo(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	mempoolTxns := s.cfg.TxMemPool.TxDescs()
	var numBytes int64
	for _, txD := range mempoolTxns {
		numBytes += int64(txD.Tx.MsgTx().SerializeSize())
	}
	ret := &json.GetMempoolInfoResult{
		Size:  int64(len(mempoolTxns)),
		Bytes: numBytes,
	}
	return ret, nil
}
// handleGetMiningInfo implements the getmininginfo command. We only return the fields that are not related to wallet functionality. This function returns more information than parallelcoind.
func handleGetMiningInfo(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (ret interface{}, err error) {
	// Create a default getnetworkhashps command to use defaults and make use of the existing getnetworkhashps handler.
	gnhpsCmd := json.NewGetNetworkHashPSCmd(nil, nil)
	networkHashesPerSecIface, err := handleGetNetworkHashPS(s, gnhpsCmd, closeChan)
	if err != nil {
		return nil, err
	}
	networkHashesPerSec, ok := networkHashesPerSecIface.(int64)
	if !ok {
		return nil, &json.RPCError{
			Code:    json.ErrRPCInternal.Code,
			Message: "networkHashesPerSec is not an int64",
		}
	}
	var Difficulty, dSHA256D, dScrypt, dBlake14lr, dCryptonight7v2, dLyra2rev2, dSkein, dX11, dStribog, dKeccak float64
	var lastbitsSHA256D, lastbitsScrypt, lastbitsBlake14lr, lastbitsCryptonight7v2, lastbitsLyra2rev2, lastbitsSkein, lastbitsX11, lastbitsStribog, lastbitsKeccak uint32
	best := s.cfg.Chain.BestSnapshot()
	v := s.cfg.Chain.Index.LookupNode(&best.Hash)
	foundcount, height := 0, best.Height
	switch fork.GetCurrent(height) {
	case 0:
		for foundcount < 2 && height > 0 {
			switch fork.GetAlgoName(v.Header().Version, height) {
			case "sha256d":
				if lastbitsSHA256D == 0 {
					foundcount++
					lastbitsSHA256D = v.Header().Bits
					dSHA256D = getDifficultyRatio(lastbitsSHA256D, s.cfg.ChainParams, v.Header().Version)
				}
			case "scrypt":
				if lastbitsScrypt == 0 {
					foundcount++
					lastbitsScrypt = v.Header().Bits
					dScrypt = getDifficultyRatio(lastbitsScrypt, s.cfg.ChainParams, v.Header().Version)
				}
			default:
			}
			v = v.RelativeAncestor(1)
			height--
		}
		switch s.cfg.Algo {
		case "sha256d":
			Difficulty = dSHA256D
		case "scrypt":
			Difficulty = dScrypt
		default:
		}
		ret = &json.GetMiningInfoResult0{
			Blocks:             int64(best.Height),
			CurrentBlockSize:   best.BlockSize,
			CurrentBlockWeight: best.BlockWeight,
			CurrentBlockTx:     best.NumTxns,
			PowAlgoID:          fork.GetAlgoID(s.cfg.Algo, height),
			PowAlgo:            s.cfg.Algo,
			Difficulty:         Difficulty,
			DifficultySHA256D:  dSHA256D,
			DifficultyScrypt:   dScrypt,
			Generate:           s.cfg.CPUMiner.IsMining(),
			GenProcLimit:       s.cfg.CPUMiner.NumWorkers(),
			HashesPerSec:       int64(s.cfg.CPUMiner.HashesPerSecond()),
			NetworkHashPS:      networkHashesPerSec,
			PooledTx:           uint64(s.cfg.TxMemPool.Count()),
			TestNet:            cfg.TestNet3,
		}
	case 1:
		foundcount, height := 0, best.Height
		for foundcount < 9 && height > fork.List[fork.GetCurrent(height)].ActivationHeight-512 {
			switch fork.GetAlgoName(v.Header().Version, height) {
			case "sha256d":
				if lastbitsSHA256D == 0 {
					foundcount++
					lastbitsSHA256D = v.Header().Bits
					dSHA256D = getDifficultyRatio(lastbitsSHA256D, s.cfg.ChainParams, v.Header().Version)
				}
			case "blake14lr":
				if lastbitsBlake14lr == 0 {
					foundcount++
					lastbitsBlake14lr = v.Header().Bits
					dBlake14lr = getDifficultyRatio(lastbitsBlake14lr, s.cfg.ChainParams, v.Header().Version)
				}
			case "whirlpool":
				if lastbitsCryptonight7v2 == 0 {
					foundcount++
					lastbitsCryptonight7v2 = v.Header().Bits
					dCryptonight7v2 = getDifficultyRatio(lastbitsCryptonight7v2, s.cfg.ChainParams, v.Header().Version)
				}
			case "lyra2rev2":
				if lastbitsLyra2rev2 == 0 {
					foundcount++
					lastbitsLyra2rev2 = v.Header().Bits
					dLyra2rev2 = getDifficultyRatio(lastbitsLyra2rev2, s.cfg.ChainParams, v.Header().Version)
				}
			case "skein":
				if lastbitsSkein == 0 {
					foundcount++
					lastbitsSkein = v.Header().Bits
					dSkein = getDifficultyRatio(lastbitsSkein, s.cfg.ChainParams, v.Header().Version)
				}
			case "x11":
				if lastbitsX11 == 0 {
					foundcount++
					lastbitsX11 = v.Header().Bits
					dX11 = getDifficultyRatio(lastbitsX11, s.cfg.ChainParams, v.Header().Version)
				}
			case "stribog":
				if lastbitsStribog == 0 {
					foundcount++
					lastbitsStribog = v.Header().Bits
					dStribog = getDifficultyRatio(lastbitsStribog, s.cfg.ChainParams, v.Header().Version)
				}
			case "keccak":
				if lastbitsKeccak == 0 {
					foundcount++
					lastbitsKeccak = v.Header().Bits
					dKeccak = getDifficultyRatio(lastbitsKeccak, s.cfg.ChainParams, v.Header().Version)
				}
			case "scrypt":
				if lastbitsScrypt == 0 {
					foundcount++
					lastbitsScrypt = v.Header().Bits
					dScrypt = getDifficultyRatio(lastbitsScrypt, s.cfg.ChainParams, v.Header().Version)
				}
			default:
			}
			v = v.RelativeAncestor(1)
			height--
		}
		switch s.cfg.Algo {
		case "sha256d":
			Difficulty = dSHA256D
		case "blake14lr":
			Difficulty = dBlake14lr
		case "whirlpool":
			Difficulty = dCryptonight7v2
		case "lyra2rev2":
			Difficulty = dLyra2rev2
		case "skein":
			Difficulty = dSkein
		case "x11":
			Difficulty = dX11
		case "stribog":
			Difficulty = dStribog
		case "keccak":
			Difficulty = dKeccak
		case "scrypt":
			Difficulty = dScrypt
		default:
		}
		ret = &json.GetMiningInfoResult{
			Blocks:                   int64(best.Height),
			CurrentBlockSize:         best.BlockSize,
			CurrentBlockWeight:       best.BlockWeight,
			CurrentBlockTx:           best.NumTxns,
			PowAlgoID:                fork.GetAlgoID(s.cfg.Algo, height),
			PowAlgo:                  s.cfg.Algo,
			Difficulty:               Difficulty,
			DifficultySHA256D:        dSHA256D,
			DifficultyScrypt:         dScrypt,
			DifficultyBlake14lr:      dBlake14lr,
			DifficultyCryptonight7v2: dCryptonight7v2,
			DifficultyLyra2rev2:      dLyra2rev2,
			DifficultySkein:          dSkein,
			DifficultyX11:            dX11,
			DifficultyStribog:        dStribog,
			DifficultyKeccak:         dKeccak,
			Generate:                 s.cfg.CPUMiner.IsMining(),
			GenAlgo:                  s.cfg.CPUMiner.GetAlgo(),
			GenProcLimit:             s.cfg.CPUMiner.NumWorkers(),
			HashesPerSec:             int64(s.cfg.CPUMiner.HashesPerSecond()),
			NetworkHashPS:            networkHashesPerSec,
			PooledTx:                 uint64(s.cfg.TxMemPool.Count()),
			TestNet:                  cfg.TestNet3,
		}
	}
	return ret, nil
}
// handleGetNetTotals implements the getnettotals command.
func handleGetNetTotals(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	totalBytesRecv, totalBytesSent := s.cfg.ConnMgr.NetTotals()
	reply := &json.GetNetTotalsResult{
		TotalBytesRecv: totalBytesRecv,
		TotalBytesSent: totalBytesSent,
		TimeMillis:     time.Now().UTC().UnixNano() / int64(time.Millisecond),
	}
	return reply, nil
}
// handleGetNetworkHashPS implements the getnetworkhashps command. This command does not default to the same end block as the parallelcoind. TODO: Really this needs to be expanded to show per-algorithm hashrates
func handleGetNetworkHashPS(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// Note: All valid error return paths should return an int64. Literal zeros are inferred as int, and won't coerce to int64 because the return value is an interface{}.
	c := cmd.(*json.GetNetworkHashPSCmd)
	// When the passed height is too high or zero, just return 0 now since we can't reasonably calculate the number of network hashes per second from invalid values.  When it's negative, use the current best block height.
	best := s.cfg.Chain.BestSnapshot()
	endHeight := int32(-1)
	if c.Height != nil {
		endHeight = int32(*c.Height)
	}
	if endHeight > best.Height || endHeight == 0 {
		return int64(0), nil
	}
	if endHeight < 0 {
		endHeight = best.Height
	}
	// Calculate the number of blocks per retarget interval based on the chain parameters.
	blocksPerRetarget := int32(s.cfg.ChainParams.TargetTimespan / s.cfg.ChainParams.TargetTimePerBlock)
	// Calculate the starting block height based on the passed number of blocks.  When the passed value is negative, use the last block the difficulty changed as the starting height.  Also make sure the starting height is not before the beginning of the chain.
	numBlocks := int32(120)
	if c.Blocks != nil {
		numBlocks = int32(*c.Blocks)
	}
	var startHeight int32
	if numBlocks <= 0 {
		startHeight = endHeight - ((endHeight % blocksPerRetarget) + 1)
	} else {
		startHeight = endHeight - numBlocks
	}
	if startHeight < 0 {
		startHeight = 0
	}
	log <- cl.Debugf{
		"calculating network hashes per second from %d to %d",
		startHeight,
		endHeight,
	}
	// Find the min and max block timestamps as well as calculate the total amount of work that happened between the start and end blocks.
	var minTimestamp, maxTimestamp time.Time
	totalWork := big.NewInt(0)
	for curHeight := startHeight; curHeight <= endHeight; curHeight++ {
		hash, err := s.cfg.Chain.BlockHashByHeight(curHeight)
		if err != nil {
			context := "Failed to fetch block hash"
			return nil, internalRPCError(err.Error(), context)
		}
		// Fetch the header from chain.
		header, err := s.cfg.Chain.HeaderByHash(hash)
		if err != nil {
			context := "Failed to fetch block header"
			return nil, internalRPCError(err.Error(), context)
		}
		if curHeight == startHeight {
			minTimestamp = header.Timestamp
			maxTimestamp = minTimestamp
		} else {
			totalWork.Add(totalWork, blockchain.CalcWork(header.Bits, best.Height+1, header.Version))
			if minTimestamp.After(header.Timestamp) {
				minTimestamp = header.Timestamp
			}
			if maxTimestamp.Before(header.Timestamp) {
				maxTimestamp = header.Timestamp
			}
		}
	}
	// Calculate the difference in seconds between the min and max block timestamps and avoid division by zero in the case where there is no time difference.
	timeDiff := int64(maxTimestamp.Sub(minTimestamp) / time.Second)
	if timeDiff == 0 {
		return int64(0), nil
	}
	hashesPerSec := new(big.Int).Div(totalWork, big.NewInt(timeDiff))
	return hashesPerSec.Int64(), nil
}
// handleGetPeerInfo implements the getpeerinfo command.
func handleGetPeerInfo(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	peers := s.cfg.ConnMgr.ConnectedPeers()
	syncPeerID := s.cfg.SyncMgr.SyncPeerID()
	infos := make([]*json.GetPeerInfoResult, 0, len(peers))
	for _, p := range peers {
		statsSnap := p.ToPeer().StatsSnapshot()
		info := &json.GetPeerInfoResult{
			ID:             statsSnap.ID,
			Addr:           statsSnap.Addr,
			AddrLocal:      p.ToPeer().LocalAddr().String(),
			Services:       fmt.Sprintf("%08d", uint64(statsSnap.Services)),
			RelayTxes:      !p.IsTxRelayDisabled(),
			LastSend:       statsSnap.LastSend.Unix(),
			LastRecv:       statsSnap.LastRecv.Unix(),
			BytesSent:      statsSnap.BytesSent,
			BytesRecv:      statsSnap.BytesRecv,
			ConnTime:       statsSnap.ConnTime.Unix(),
			PingTime:       float64(statsSnap.LastPingMicros),
			TimeOffset:     statsSnap.TimeOffset,
			Version:        statsSnap.Version,
			SubVer:         statsSnap.UserAgent,
			Inbound:        statsSnap.Inbound,
			StartingHeight: statsSnap.StartingHeight,
			CurrentHeight:  statsSnap.LastBlock,
			BanScore:       int32(p.BanScore()),
			FeeFilter:      p.FeeFilter(),
			SyncNode:       statsSnap.ID == syncPeerID,
		}
		if p.ToPeer().LastPingNonce() != 0 {
			wait := float64(time.Since(statsSnap.LastPingTime).Nanoseconds())
			// We actually want microseconds.
			info.PingWait = wait / 1000
		}
		infos = append(infos, info)
	}
	return infos, nil
}
// handleGetRawMempool implements the getrawmempool command.
func handleGetRawMempool(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*json.GetRawMempoolCmd)
	mp := s.cfg.TxMemPool
	if c.Verbose != nil && *c.Verbose {
		return mp.RawMempoolVerbose(), nil
	}
	// The response is simply an array of the transaction hashes if the verbose flag is not set.
	descs := mp.TxDescs()
	hashStrings := make([]string, len(descs))
	for i := range hashStrings {
		hashStrings[i] = descs[i].Tx.Hash().String()
	}
	return hashStrings, nil
}
// handleGetRawTransaction implements the getrawtransaction command.
func handleGetRawTransaction(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*json.GetRawTransactionCmd)
	// Convert the provided transaction hash hex to a Hash.
	txHash, err := chainhash.NewHashFromStr(c.Txid)
	if err != nil {
		return nil, rpcDecodeHexError(c.Txid)
	}
	verbose := false
	if c.Verbose != nil {
		verbose = *c.Verbose != 0
	}
	// Try to fetch the transaction from the memory pool and if that fails, try the block database.
	var mtx *wire.MsgTx
	var blkHash *chainhash.Hash
	var blkHeight int32
	tx, err := s.cfg.TxMemPool.FetchTransaction(txHash)
	if err != nil {
		if s.cfg.TxIndex == nil {
			return nil, &json.RPCError{
				Code: json.ErrRPCNoTxInfo,
				Message: "The transaction index must be " +
					"enabled to query the blockchain " +
					"(specify --txindex)",
			}
		}
		// Look up the location of the transaction.
		blockRegion, err := s.cfg.TxIndex.TxBlockRegion(txHash)
		if err != nil {
			context := "Failed to retrieve transaction location"
			return nil, internalRPCError(err.Error(), context)
		}
		if blockRegion == nil {
			return nil, rpcNoTxInfoError(txHash)
		}
		// Load the raw transaction bytes from the database.
		var txBytes []byte
		err = s.cfg.DB.View(func(dbTx database.Tx) error {
			var err error
			txBytes, err = dbTx.FetchBlockRegion(blockRegion)
			return err
		})
		if err != nil {
			return nil, rpcNoTxInfoError(txHash)
		}
		// When the verbose flag isn't set, simply return the serialized transaction as a hex-encoded string.  This is done here to avoid deserializing it only to reserialize it again later.
		if !verbose {
			return hex.EncodeToString(txBytes), nil
		}
		// Grab the block height.
		blkHash = blockRegion.Hash
		blkHeight, err = s.cfg.Chain.BlockHeightByHash(blkHash)
		if err != nil {
			context := "Failed to retrieve block height"
			return nil, internalRPCError(err.Error(), context)
		}
		// Deserialize the transaction
		var msgTx wire.MsgTx
		err = msgTx.Deserialize(bytes.NewReader(txBytes))
		if err != nil {
			context := "Failed to deserialize transaction"
			return nil, internalRPCError(err.Error(), context)
		}
		mtx = &msgTx
	} else {
		// When the verbose flag isn't set, simply return the network-serialized transaction as a hex-encoded string.
		if !verbose {
			// Note that this is intentionally not directly returning because the first return value is a string and it would result in returning an empty string to the client instead of nothing (nil) in the case of an error.
			mtxHex, err := messageToHex(tx.MsgTx())
			if err != nil {
				return nil, err
			}
			return mtxHex, nil
		}
		mtx = tx.MsgTx()
	}
	// The verbose flag is set, so generate the JSON object and return it.
	var blkHeader *wire.BlockHeader
	var blkHashStr string
	var chainHeight int32
	if blkHash != nil {
		// Fetch the header from chain.
		header, err := s.cfg.Chain.HeaderByHash(blkHash)
		if err != nil {
			context := "Failed to fetch block header"
			return nil, internalRPCError(err.Error(), context)
		}
		blkHeader = &header
		blkHashStr = blkHash.String()
		chainHeight = s.cfg.Chain.BestSnapshot().Height
	}
	rawTxn, err := createTxRawResult(s.cfg.ChainParams, mtx, txHash.String(),
		blkHeader, blkHashStr, blkHeight, chainHeight)
	if err != nil {
		return nil, err
	}
	return *rawTxn, nil
}
// handleGetTxOut handles gettxout commands.
func handleGetTxOut(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*json.GetTxOutCmd)
	// Convert the provided transaction hash hex to a Hash.
	txHash, err := chainhash.NewHashFromStr(c.Txid)
	if err != nil {
		return nil, rpcDecodeHexError(c.Txid)
	}
	// If requested and the tx is available in the mempool try to fetch it from there, otherwise attempt to fetch from the block database.
	var bestBlockHash string
	var confirmations int32
	var value int64
	var pkScript []byte
	var isCoinbase bool
	includeMempool := true
	if c.IncludeMempool != nil {
		includeMempool = *c.IncludeMempool
	}
	// TODO: This is racy.  It should attempt to fetch it directly and check the error.
	if includeMempool && s.cfg.TxMemPool.HaveTransaction(txHash) {
		tx, err := s.cfg.TxMemPool.FetchTransaction(txHash)
		if err != nil {
			return nil, rpcNoTxInfoError(txHash)
		}
		mtx := tx.MsgTx()
		if c.Vout > uint32(len(mtx.TxOut)-1) {
			return nil, &json.RPCError{
				Code: json.ErrRPCInvalidTxVout,
				Message: "Output index number (vout) does not " +
					"exist for transaction.",
			}
		}
		txOut := mtx.TxOut[c.Vout]
		if txOut == nil {
			errStr := fmt.Sprintf("Output index: %d for txid: %s "+
				"does not exist", c.Vout, txHash)
			return nil, internalRPCError(errStr, "")
		}
		best := s.cfg.Chain.BestSnapshot()
		bestBlockHash = best.Hash.String()
		confirmations = 0
		value = txOut.Value
		pkScript = txOut.PkScript
		isCoinbase = blockchain.IsCoinBaseTx(mtx)
	} else {
		out := wire.OutPoint{Hash: *txHash, Index: c.Vout}
		entry, err := s.cfg.Chain.FetchUtxoEntry(out)
		if err != nil {
			return nil, rpcNoTxInfoError(txHash)
		}
		// To match the behavior of the reference client, return nil (JSON null) if the transaction output is spent by another transaction already in the main chain.  Mined transactions that are spent by a mempool transaction are not affected by this.
		if entry == nil || entry.IsSpent() {
			return nil, nil
		}
		best := s.cfg.Chain.BestSnapshot()
		bestBlockHash = best.Hash.String()
		confirmations = 1 + best.Height - entry.BlockHeight()
		value = entry.Amount()
		pkScript = entry.PkScript()
		isCoinbase = entry.IsCoinBase()
	}
	// Disassemble script into single line printable format. The disassembled string will contain [error] inline if the script doesn't fully parse, so ignore the error here.
	disbuf, _ := txscript.DisasmString(pkScript)
	// Get further info about the script. Ignore the error here since an error means the script couldn't parse and there is no additional information about it anyways.
	scriptClass, addrs, reqSigs, _ := txscript.ExtractPkScriptAddrs(pkScript, s.cfg.ChainParams)
	addresses := make([]string, len(addrs))
	for i, addr := range addrs {
		addresses[i] = addr.EncodeAddress()
	}
	txOutReply := &json.GetTxOutResult{
		BestBlock:     bestBlockHash,
		Confirmations: int64(confirmations),
		Value:         util.Amount(value).ToDUO(),
		ScriptPubKey: json.ScriptPubKeyResult{
			Asm:       disbuf,
			Hex:       hex.EncodeToString(pkScript),
			ReqSigs:   int32(reqSigs),
			Type:      scriptClass.String(),
			Addresses: addresses,
		},
		Coinbase: isCoinbase,
	}
	return txOutReply, nil
}
// handleHelp implements the help command.
func handleHelp(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*json.HelpCmd)
	// Provide a usage overview of all commands when no specific command was specified.
	var command string
	if c.Command != nil {
		command = *c.Command
	}
	if command == "" {
		usage, err := s.helpCacher.rpcUsage(false)
		if err != nil {
			context := "Failed to generate RPC usage"
			return nil, internalRPCError(err.Error(), context)
		}
		return usage, nil
	}
	// Check that the command asked for is supported and implemented.  Only search the main list of handlers since help should not be provided for commands that are unimplemented or related to wallet functionality.
	if _, ok := rpcHandlers[command]; !ok {
		return nil, &json.RPCError{
			Code:    json.ErrRPCInvalidParameter,
			Message: "Unknown command: " + command,
		}
	}
	// Get the help for the command.
	help, err := s.helpCacher.rpcMethodHelp(command)
	if err != nil {
		context := "Failed to generate help"
		return nil, internalRPCError(err.Error(), context)
	}
	return help, nil
}
// handlePing implements the ping command.
func handlePing(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// Ask server to ping \o_
	nonce, err := wire.RandomUint64()
	if err != nil {
		return nil, internalRPCError("Not sending ping - failed to generate nonce: "+err.Error(), "")
	}
	s.cfg.ConnMgr.BroadcastMessage(wire.NewMsgPing(nonce))
	return nil, nil
}
// retrievedTx represents a transaction that was either loaded from the transaction memory pool or from the database.  When a transaction is loaded from the database, it is loaded with the raw serialized bytes while the mempool has the fully deserialized structure.  This structure therefore will have one of the two fields set depending on where is was retrieved from. This is mainly done for efficiency to avoid extra serialization steps when possible.
type retrievedTx struct {
	txBytes []byte
	blkHash *chainhash.Hash // Only set when transaction is in a block.
	tx      *util.Tx
}
// fetchInputTxos fetches the outpoints from all transactions referenced by the inputs to the passed transaction by checking the transaction mempool first then the transaction index for those already mined into blocks.
func fetchInputTxos(s *rpcServer, tx *wire.MsgTx) (map[wire.OutPoint]wire.TxOut, error) {
	mp := s.cfg.TxMemPool
	originOutputs := make(map[wire.OutPoint]wire.TxOut)
	for txInIndex, txIn := range tx.TxIn {
		// Attempt to fetch and use the referenced transaction from the memory pool.
		origin := &txIn.PreviousOutPoint
		originTx, err := mp.FetchTransaction(&origin.Hash)
		if err == nil {
			txOuts := originTx.MsgTx().TxOut
			if origin.Index >= uint32(len(txOuts)) {
				errStr := fmt.Sprintf("unable to find output %v referenced from transaction %s:%d", origin, tx.TxHash(), txInIndex)
				return nil, internalRPCError(errStr, "")
			}
			originOutputs[*origin] = *txOuts[origin.Index]
			continue
		}
		// Look up the location of the transaction.
		blockRegion, err := s.cfg.TxIndex.TxBlockRegion(&origin.Hash)
		if err != nil {
			context := "Failed to retrieve transaction location"
			return nil, internalRPCError(err.Error(), context)
		}
		if blockRegion == nil {
			return nil, rpcNoTxInfoError(&origin.Hash)
		}
		// Load the raw transaction bytes from the database.
		var txBytes []byte
		err = s.cfg.DB.View(func(dbTx database.Tx) error {
			var err error
			txBytes, err = dbTx.FetchBlockRegion(blockRegion)
			return err
		})
		if err != nil {
			return nil, rpcNoTxInfoError(&origin.Hash)
		}
		// Deserialize the transaction
		var msgTx wire.MsgTx
		err = msgTx.Deserialize(bytes.NewReader(txBytes))
		if err != nil {
			context := "Failed to deserialize transaction"
			return nil, internalRPCError(err.Error(), context)
		}
		// Add the referenced output to the map.
		if origin.Index >= uint32(len(msgTx.TxOut)) {
			errStr := fmt.Sprintf("unable to find output %v "+
				"referenced from transaction %s:%d", origin,
				tx.TxHash(), txInIndex)
			return nil, internalRPCError(errStr, "")
		}
		originOutputs[*origin] = *msgTx.TxOut[origin.Index]
	}
	return originOutputs, nil
}
// createVinListPrevOut returns a slice of JSON objects for the inputs of the passed transaction.
func createVinListPrevOut(s *rpcServer, mtx *wire.MsgTx, chainParams *chaincfg.Params, vinExtra bool, filterAddrMap map[string]struct{}) ([]json.VinPrevOut, error) {
	// Coinbase transactions only have a single txin by definition.
	if blockchain.IsCoinBaseTx(mtx) {
		// Only include the transaction if the filter map is empty because a coinbase input has no addresses and so would never match a non-empty filter.
		if len(filterAddrMap) != 0 {
			return nil, nil
		}
		txIn := mtx.TxIn[0]
		vinList := make([]json.VinPrevOut, 1)
		vinList[0].Coinbase = hex.EncodeToString(txIn.SignatureScript)
		vinList[0].Sequence = txIn.Sequence
		return vinList, nil
	}
	// Use a dynamically sized list to accommodate the address filter.
	vinList := make([]json.VinPrevOut, 0, len(mtx.TxIn))
	// Lookup all of the referenced transaction outputs needed to populate the previous output information if requested.
	var originOutputs map[wire.OutPoint]wire.TxOut
	if vinExtra || len(filterAddrMap) > 0 {
		var err error
		originOutputs, err = fetchInputTxos(s, mtx)
		if err != nil {
			return nil, err
		}
	}
	for _, txIn := range mtx.TxIn {
		// The disassembled string will contain [error] inline if the script doesn't fully parse, so ignore the error here.
		disbuf, _ := txscript.DisasmString(txIn.SignatureScript)
		// Create the basic input entry without the additional optional previous output details which will be added later if requested and available.
		prevOut := &txIn.PreviousOutPoint
		vinEntry := json.VinPrevOut{
			Txid:     prevOut.Hash.String(),
			Vout:     prevOut.Index,
			Sequence: txIn.Sequence,
			ScriptSig: &json.ScriptSig{
				Asm: disbuf,
				Hex: hex.EncodeToString(txIn.SignatureScript),
			},
		}
		if len(txIn.Witness) != 0 {
			vinEntry.Witness = witnessToHex(txIn.Witness)
		}
		// Add the entry to the list now if it already passed the filter since the previous output might not be available.
		passesFilter := len(filterAddrMap) == 0
		if passesFilter {
			vinList = append(vinList, vinEntry)
		}
		// Only populate previous output information if requested and available.
		if len(originOutputs) == 0 {
			continue
		}
		originTxOut, ok := originOutputs[*prevOut]
		if !ok {
			continue
		}
		// Ignore the error here since an error means the script couldn't parse and there is no additional information about it anyways.
		_, addrs, _, _ := txscript.ExtractPkScriptAddrs(originTxOut.PkScript, chainParams)
		// Encode the addresses while checking if the address passes the filter when needed.
		encodedAddrs := make([]string, len(addrs))
		for j, addr := range addrs {
			encodedAddr := addr.EncodeAddress()
			encodedAddrs[j] = encodedAddr
			// No need to check the map again if the filter already passes.
			if passesFilter {
				continue
			}
			if _, exists := filterAddrMap[encodedAddr]; exists {
				passesFilter = true
			}
		}
		// Ignore the entry if it doesn't pass the filter.
		if !passesFilter {
			continue
		}
		// Add entry to the list if it wasn't already done above.
		if len(filterAddrMap) != 0 {
			vinList = append(vinList, vinEntry)
		}
		// Update the entry with previous output information if requested.
		if vinExtra {
			vinListEntry := &vinList[len(vinList)-1]
			vinListEntry.PrevOut = &json.PrevOut{
				Addresses: encodedAddrs,
				Value:     util.Amount(originTxOut.Value).ToDUO(),
			}
		}
	}
	return vinList, nil
}
// fetchMempoolTxnsForAddress queries the address index for all unconfirmed transactions that involve the provided address.  The results will be limited by the number to skip and the number requested.
func fetchMempoolTxnsForAddress(s *rpcServer, addr util.Address, numToSkip, numRequested uint32) ([]*util.Tx, uint32) {
	// There are no entries to return when there are less available than the number being skipped.
	mpTxns := s.cfg.AddrIndex.UnconfirmedTxnsForAddress(addr)
	numAvailable := uint32(len(mpTxns))
	if numToSkip > numAvailable {
		return nil, numAvailable
	}
	// Filter the available entries based on the number to skip and number requested.
	rangeEnd := numToSkip + numRequested
	if rangeEnd > numAvailable {
		rangeEnd = numAvailable
	}
	return mpTxns[numToSkip:rangeEnd], numToSkip
}
// handleSearchRawTransactions implements the searchrawtransactions command.
func handleSearchRawTransactions(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// Respond with an error if the address index is not enabled.
	addrIndex := s.cfg.AddrIndex
	if addrIndex == nil {
		return nil, &json.RPCError{
			Code:    json.ErrRPCMisc,
			Message: "Address index must be enabled (--addrindex)",
		}
	}
	// Override the flag for including extra previous output information in each input if needed.
	c := cmd.(*json.SearchRawTransactionsCmd)
	vinExtra := false
	if c.VinExtra != nil {
		vinExtra = *c.VinExtra != 0
	}
	// Including the extra previous output information requires the transaction index.  Currently the address index relies on the transaction index, so this check is redundant, but it's better to be safe in case the address index is ever changed to not rely on it.
	if vinExtra && s.cfg.TxIndex == nil {
		return nil, &json.RPCError{
			Code:    json.ErrRPCMisc,
			Message: "Transaction index must be enabled (--txindex)",
		}
	}
	// Attempt to decode the supplied address.
	params := s.cfg.ChainParams
	addr, err := util.DecodeAddress(c.Address, params)
	if err != nil {
		return nil, &json.RPCError{
			Code:    json.ErrRPCInvalidAddressOrKey,
			Message: "Invalid address or key: " + err.Error(),
		}
	}
	// Override the default number of requested entries if needed.  Also, just return now if the number of requested entries is zero to avoid extra work.
	numRequested := 100
	if c.Count != nil {
		numRequested = *c.Count
		if numRequested < 0 {
			numRequested = 1
		}
	}
	if numRequested == 0 {
		return nil, nil
	}
	// Override the default number of entries to skip if needed.
	var numToSkip int
	if c.Skip != nil {
		numToSkip = *c.Skip
		if numToSkip < 0 {
			numToSkip = 0
		}
	}
	// Override the reverse flag if needed.
	var reverse bool
	if c.Reverse != nil {
		reverse = *c.Reverse
	}
	// Add transactions from mempool first if client asked for reverse order.  Otherwise, they will be added last (as needed depending on the requested counts). NOTE: This code doesn't sort by dependency.  This might be something to do in the future for the client's convenience, or leave it to the client.
	numSkipped := uint32(0)
	addressTxns := make([]retrievedTx, 0, numRequested)
	if reverse {
		// Transactions in the mempool are not in a block header yet, so the block header field in the retieved transaction struct is left nil.
		mpTxns, mpSkipped := fetchMempoolTxnsForAddress(s, addr, uint32(numToSkip), uint32(numRequested))
		numSkipped += mpSkipped
		for _, tx := range mpTxns {
			addressTxns = append(addressTxns, retrievedTx{tx: tx})
		}
	}
	// Fetch transactions from the database in the desired order if more are needed.
	if len(addressTxns) < numRequested {
		err = s.cfg.DB.View(func(dbTx database.Tx) error {
			regions, dbSkipped, err := addrIndex.TxRegionsForAddress(dbTx, addr, uint32(numToSkip)-numSkipped, uint32(numRequested-len(addressTxns)), reverse)
			if err != nil {
				return err
			}
			// Load the raw transaction bytes from the database.
			serializedTxns, err := dbTx.FetchBlockRegions(regions)
			if err != nil {
				return err
			}
			// Add the transaction and the hash of the block it is contained in to the list.  Note that the transaction is left serialized here since the caller might have requested non-verbose output and hence there would be/ no point in deserializing it just to reserialize it later.
			for i, serializedTx := range serializedTxns {
				addressTxns = append(addressTxns, retrievedTx{
					txBytes: serializedTx,
					blkHash: regions[i].Hash,
				})
			}
			numSkipped += dbSkipped
			return nil
		})
		if err != nil {
			context := "Failed to load address index entries"
			return nil, internalRPCError(err.Error(), context)
		}
	}
	// Add transactions from mempool last if client did not request reverse order and the number of results is still under the number requested.
	if !reverse && len(addressTxns) < numRequested {
		// Transactions in the mempool are not in a block header yet, so the block header field in the retieved transaction struct is left nil.
		mpTxns, mpSkipped := fetchMempoolTxnsForAddress(s, addr, uint32(numToSkip)-numSkipped, uint32(numRequested-len(addressTxns)))
		numSkipped += mpSkipped
		for _, tx := range mpTxns {
			addressTxns = append(addressTxns, retrievedTx{tx: tx})
		}
	}
	// Address has never been used if neither source yielded any results.
	if len(addressTxns) == 0 {
		return nil, &json.RPCError{
			Code:    json.ErrRPCNoTxInfo,
			Message: "No information available about address",
		}
	}
	// Serialize all of the transactions to hex.
	hexTxns := make([]string, len(addressTxns))
	for i := range addressTxns {
		// Simply encode the raw bytes to hex when the retrieved transaction is already in serialized form.
		rtx := &addressTxns[i]
		if rtx.txBytes != nil {
			hexTxns[i] = hex.EncodeToString(rtx.txBytes)
			continue
		}
		// Serialize the transaction first and convert to hex when the retrieved transaction is the deserialized structure.
		hexTxns[i], err = messageToHex(rtx.tx.MsgTx())
		if err != nil {
			return nil, err
		}
	}
	// When not in verbose mode, simply return a list of serialized txns.
	if c.Verbose != nil && *c.Verbose == 0 {
		return hexTxns, nil
	}
	// Normalize the provided filter addresses (if any) to ensure there are no duplicates.
	filterAddrMap := make(map[string]struct{})
	if c.FilterAddrs != nil && len(*c.FilterAddrs) > 0 {
		for _, addr := range *c.FilterAddrs {
			filterAddrMap[addr] = struct{}{}
		}
	}
	// The verbose flag is set, so generate the JSON object and return it.
	best := s.cfg.Chain.BestSnapshot()
	srtList := make([]json.SearchRawTransactionsResult, len(addressTxns))
	for i := range addressTxns {
		// The deserialized transaction is needed, so deserialize the retrieved transaction if it's in serialized form (which will be the case when it was lookup up from the database). Otherwise, use the existing deserialized transaction.
		rtx := &addressTxns[i]
		var mtx *wire.MsgTx
		if rtx.tx == nil {
			// Deserialize the transaction.
			mtx = new(wire.MsgTx)
			err := mtx.Deserialize(bytes.NewReader(rtx.txBytes))
			if err != nil {
				context := "Failed to deserialize transaction"
				return nil, internalRPCError(err.Error(), context)
			}
		} else {
			mtx = rtx.tx.MsgTx()
		}
		result := &srtList[i]
		result.Hex = hexTxns[i]
		result.Txid = mtx.TxHash().String()
		result.Vin, err = createVinListPrevOut(s, mtx, params, vinExtra, filterAddrMap)
		if err != nil {
			return nil, err
		}
		result.Vout = createVoutList(mtx, params, filterAddrMap)
		result.Version = mtx.Version
		result.LockTime = mtx.LockTime
		// Transactions grabbed from the mempool aren't yet in a block, so conditionally fetch block details here.  This will be reflected in the final JSON output (mempool won't have confirmations or block information).
		var blkHeader *wire.BlockHeader
		var blkHashStr string
		var blkHeight int32
		if blkHash := rtx.blkHash; blkHash != nil {
			// Fetch the header from chain.
			header, err := s.cfg.Chain.HeaderByHash(blkHash)
			if err != nil {
				return nil, &json.RPCError{
					Code:    json.ErrRPCBlockNotFound,
					Message: "Block not found",
				}
			}
			// Get the block height from chain.
			height, err := s.cfg.Chain.BlockHeightByHash(blkHash)
			if err != nil {
				context := "Failed to obtain block height"
				return nil, internalRPCError(err.Error(), context)
			}
			blkHeader = &header
			blkHashStr = blkHash.String()
			blkHeight = height
		}
		// Add the block information to the result if there is any.
		if blkHeader != nil {
			// This is not a typo, they are identical in Bitcoin Core as well.
			result.Time = blkHeader.Timestamp.Unix()
			result.Blocktime = blkHeader.Timestamp.Unix()
			result.BlockHash = blkHashStr
			result.Confirmations = uint64(1 + best.Height - blkHeight)
		}
	}
	return srtList, nil
}
// handleSendRawTransaction implements the sendrawtransaction command.
func handleSendRawTransaction(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*json.SendRawTransactionCmd)
	// Deserialize and send off to tx relay
	hexStr := c.HexTx
	if len(hexStr)%2 != 0 {
		hexStr = "0" + hexStr
	}
	serializedTx, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, rpcDecodeHexError(hexStr)
	}
	var msgTx wire.MsgTx
	err = msgTx.Deserialize(bytes.NewReader(serializedTx))
	if err != nil {
		return nil, &json.RPCError{
			Code:    json.ErrRPCDeserialization,
			Message: "TX decode failed: " + err.Error(),
		}
	}
	// Use 0 for the tag to represent local node.
	tx := util.NewTx(&msgTx)
	acceptedTxs, err := s.cfg.TxMemPool.ProcessTransaction(tx, false, false, 0)
	if err != nil {
		// When the error is a rule error, it means the transaction was simply rejected as opposed to something actually going wrong, so log it as such.  Otherwise, something really did go wrong, so log it as an actual error.  In both cases, a JSON-RPC error is returned to the client with the deserialization error code (to match bitcoind behavior).
		if _, ok := err.(mempool.RuleError); ok {
			log <- cl.Debugf{
				"rejected transaction %v: %v", tx.Hash(), err,
			}
		} else {
			log <- cl.Errorf{
				"failed to process transaction %v: %v", tx.Hash(), err,
			}
		}
		return nil, &json.RPCError{
			Code:    json.ErrRPCDeserialization,
			Message: "TX rejected: " + err.Error(),
		}
	}
	// When the transaction was accepted it should be the first item in the returned array of accepted transactions.  The only way this will not be true is if the API for ProcessTransaction changes and this code is not properly updated, but ensure the condition holds as a safeguard. Also, since an error is being returned to the caller, ensure the transaction is removed from the memory pool.
	if len(acceptedTxs) == 0 || !acceptedTxs[0].Tx.Hash().IsEqual(tx.Hash()) {
		s.cfg.TxMemPool.RemoveTransaction(tx, true)
		errStr := fmt.Sprintf("transaction %v is not in accepted list", tx.Hash())
		return nil, internalRPCError(errStr, "")
	}
	// Generate and relay inventory vectors for all newly accepted transactions into the memory pool due to the original being accepted.
	s.cfg.ConnMgr.RelayTransactions(acceptedTxs)
	// Notify both websocket and getblocktemplate long poll clients of all newly accepted transactions.
	s.NotifyNewTransactions(acceptedTxs)
	// Keep track of all the sendrawtransaction request txns so that they can be rebroadcast if they don't make their way into a block.
	txD := acceptedTxs[0]
	iv := wire.NewInvVect(wire.InvTypeTx, txD.Tx.Hash())
	s.cfg.ConnMgr.AddRebroadcastInventory(iv, txD)
	return tx.Hash().String(), nil
}
// handleSetGenerate implements the setgenerate command.
func handleSetGenerate(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*json.SetGenerateCmd)
	// Disable generation regardless of the provided generate flag if the maximum number of threads (goroutines for our purposes) is 0. Otherwise enable or disable it depending on the provided flag.
	fmt.Println(*c.GenProcLimit, c.Generate)
	generate := c.Generate
	genProcLimit := -1
	if c.GenProcLimit != nil {
		genProcLimit = *c.GenProcLimit
	}
	if genProcLimit == 0 {
		generate = false
	}
	if s.cfg.CPUMiner.IsMining() {
		// if s.cfg.CPUMiner.GetAlgo() != s.cfg.Algo {
		s.cfg.CPUMiner.Stop()
		generate = true
		// }
	}
	if !generate {
		s.cfg.CPUMiner.Stop()
	} else {
		// Respond with an error if there are no addresses to pay the created blocks to.
		if len(StateCfg.ActiveMiningAddrs) == 0 {
			return nil, &json.RPCError{
				Code:    json.ErrRPCInternal.Code,
				Message: "no payment addresses specified via --miningaddr",
			}
		}
		// fmt.Println("generating with algo", s.cfg.Algo)
		// s.cfg.CPUMiner.SetAlgo(s.cfg.Algo)
		// It's safe to call start even if it's already started.
		s.cfg.CPUMiner.SetNumWorkers(int32(genProcLimit))
		s.cfg.CPUMiner.Start()
	}
	return nil, nil
}
// handleStop implements the stop command.
func handleStop(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	select {
	case s.requestProcessShutdown <- struct{}{}:
		// fmt.Println("chan:s.requestProcessShutdown <- struct{}{}")
	default:
	}
	return "node stopping", nil
}
// handleSubmitBlock implements the submitblock command.
func handleSubmitBlock(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*json.SubmitBlockCmd)
	// Deserialize the submitted block.
	hexStr := c.HexBlock
	if len(hexStr)%2 != 0 {
		hexStr = "0" + c.HexBlock
	}
	serializedBlock, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, rpcDecodeHexError(hexStr)
	}
	block, err := util.NewBlockFromBytes(serializedBlock)
	if err != nil {
		return nil, &json.RPCError{
			Code:    json.ErrRPCDeserialization,
			Message: "Block decode failed: " + err.Error(),
		}
	}
	// Process this block using the same rules as blocks coming from other nodes.  This will in turn relay it to the network like normal.
	_, err = s.cfg.SyncMgr.SubmitBlock(block, blockchain.BFNone)
	if err != nil {
		return fmt.Sprintf("rejected: %s", err.Error()), nil
	}
	log <- cl.Infof{
		"accepted block %s via submitblock", block.Hash(),
	}
	return nil, nil
}
// handleUptime implements the uptime command.
func handleUptime(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	return time.Now().Unix() - s.cfg.StartupTime, nil
}
// handleValidateAddress implements the validateaddress command.
func handleValidateAddress(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*json.ValidateAddressCmd)
	result := json.ValidateAddressChainResult{}
	addr, err := util.DecodeAddress(c.Address, s.cfg.ChainParams)
	if err != nil {
		// Return the default value (false) for IsValid.
		return result, nil
	}
	result.Address = addr.EncodeAddress()
	result.IsValid = true
	return result, nil
}
func verifyChain(s *rpcServer, level, depth int32) error {
	best := s.cfg.Chain.BestSnapshot()
	finishHeight := best.Height - depth
	if finishHeight < 0 {
		finishHeight = 0
	}
	log <- cl.Infof{
		"verifying chain for %d blocks at level %d",
		best.Height - finishHeight,
		level,
	}
	for height := best.Height; height > finishHeight; height-- {
		// Level 0 just looks up the block.
		block, err := s.cfg.Chain.BlockByHeight(height)
		if err != nil {
			log <- cl.Errorf{
				"verify is unable to fetch block at height %d: %v",
				height,
				err,
			}
			return err
		}
		powLimit := fork.GetMinDiff(fork.GetAlgoName(block.MsgBlock().Header.Version, height), height)
		// Level 1 does basic chain sanity checks.
		if level > 0 {
			err := blockchain.CheckBlockSanity(block, powLimit, s.cfg.TimeSource, true, block.Height(), s.cfg.ChainParams.Name == "testnet")
			if err != nil {
				log <- cl.Errorf{
					"verify is unable to validate block at hash %v height %d: %v",
					block.Hash(), height, err}
				return err
			}
		}
	}
	log <- cl.Inf("chain verify completed successfully")
	return nil
}
// handleVerifyChain implements the verifychain command.
func handleVerifyChain(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*json.VerifyChainCmd)
	var checkLevel, checkDepth int32
	if c.CheckLevel != nil {
		checkLevel = *c.CheckLevel
	}
	if c.CheckDepth != nil {
		checkDepth = *c.CheckDepth
	}
	err := verifyChain(s, checkLevel, checkDepth)
	return err == nil, nil
}
// handleVerifyMessage implements the verifymessage command.
func handleVerifyMessage(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*json.VerifyMessageCmd)
	// Decode the provided address.
	params := s.cfg.ChainParams
	addr, err := util.DecodeAddress(c.Address, params)
	if err != nil {
		return nil, &json.RPCError{
			Code:    json.ErrRPCInvalidAddressOrKey,
			Message: "Invalid address or key: " + err.Error(),
		}
	}
	// Only P2PKH addresses are valid for signing.
	if _, ok := addr.(*util.AddressPubKeyHash); !ok {
		return nil, &json.RPCError{
			Code:    json.ErrRPCType,
			Message: "Address is not a pay-to-pubkey-hash address",
		}
	}
	// Decode base64 signature.
	sig, err := base64.StdEncoding.DecodeString(c.Signature)
	if err != nil {
		return nil, &json.RPCError{
			Code:    json.ErrRPCParse.Code,
			Message: "Malformed base64 encoding: " + err.Error(),
		}
	}
	// Validate the signature - this just shows that it was valid at all. we will compare it with the key next.
	var buf bytes.Buffer
	wire.WriteVarString(&buf, 0, "Bitcoin Signed Message:\n")
	wire.WriteVarString(&buf, 0, c.Message)
	expectedMessageHash := chainhash.DoubleHashB(buf.Bytes())
	pk, wasCompressed, err := ec.RecoverCompact(ec.S256(), sig,
		expectedMessageHash)
	if err != nil {
		// Mirror Bitcoin Core behavior, which treats error in RecoverCompact as invalid signature.
		return false, nil
	}
	// Reconstruct the pubkey hash.
	var serializedPK []byte
	if wasCompressed {
		serializedPK = pk.SerializeCompressed()
	} else {
		serializedPK = pk.SerializeUncompressed()
	}
	address, err := util.NewAddressPubKey(serializedPK, params)
	if err != nil {
		// Again mirror Bitcoin Core behavior, which treats error in public key reconstruction as invalid signature.
		return false, nil
	}
	// Return boolean if addresses match.
	return address.EncodeAddress() == c.Address, nil
}
// handleVersion implements the version command. NOTE: This is a btcsuite extension ported from github.com/decred/dcrd.
func handleVersion(s *rpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	result := map[string]json.VersionResult{
		"podjsonrpcapi": {
			VersionString: jsonrpcSemverString,
			Major:         jsonrpcSemverMajor,
			Minor:         jsonrpcSemverMinor,
			Patch:         jsonrpcSemverPatch,
		},
	}
	return result, nil
}
// rpcServer provides a concurrent safe RPC server to a chain server.
type rpcServer struct {
	started                int32
	shutdown               int32
	cfg                    rpcserverConfig
	authsha                [sha256.Size]byte
	limitauthsha           [sha256.Size]byte
	ntfnMgr                *wsNotificationManager
	numClients             int32
	statusLines            map[int]string
	statusLock             sync.RWMutex
	wg                     sync.WaitGroup
	gbtWorkState           *gbtWorkState
	helpCacher             *helpCacher
	requestProcessShutdown chan struct{}
	quit                   chan int
}
// httpStatusLine returns a response Status-Line (RFC 2616 Section 6.1) for the given request and response status code.  This function was lifted and adapted from the standard library HTTP server code since it's not exported.
func (s *rpcServer) httpStatusLine(req *http.Request, code int) string {
	// Fast path:
	key := code
	proto11 := req.ProtoAtLeast(1, 1)
	if !proto11 {
		key = -key
	}
	s.statusLock.RLock()
	line, ok := s.statusLines[key]
	s.statusLock.RUnlock()
	if ok {
		return line
	}
	// Slow path:
	proto := "HTTP/1.0"
	if proto11 {
		proto = "HTTP/1.1"
	}
	codeStr := strconv.Itoa(code)
	text := http.StatusText(code)
	if text != "" {
		line = proto + " " + codeStr + " " + text + "\r\n"
		s.statusLock.Lock()
		s.statusLines[key] = line
		s.statusLock.Unlock()
	} else {
		text = "status code " + codeStr
		line = proto + " " + codeStr + " " + text + "\r\n"
	}
	return line
}
// writeHTTPResponseHeaders writes the necessary response headers prior to writing an HTTP body given a request to use for protocol negotiation, headers to write, a status code, and a writer.
func (s *rpcServer) writeHTTPResponseHeaders(req *http.Request, headers http.Header, code int, w io.Writer) error {
	_, err := io.WriteString(w, s.httpStatusLine(req, code))
	if err != nil {
		return err
	}
	err = headers.Write(w)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, "\r\n")
	return err
}
// Stop is used by server.go to stop the rpc listener.
func (s *rpcServer) Stop() error {
	if atomic.AddInt32(&s.shutdown, 1) != 1 {
		log <- cl.Inf("RPC server is already in the process of shutting down")
		return nil
	}
	log <- cl.Wrn("RPC server shutting down")
	for _, listener := range s.cfg.Listeners {
		err := listener.Close()
		if err != nil {
			log <- cl.Error{"problem shutting down RPC:", err}
			return err
		}
	}
	s.ntfnMgr.Shutdown()
	s.ntfnMgr.WaitForShutdown()
	close(s.quit)
	s.wg.Wait()
	log <- cl.Inf("RPC server shutdown complete")
	return nil
}
// RequestedProcessShutdown returns a channel that is sent to when an authorized RPC client requests the process to shutdown.  If the request can not be read immediately, it is dropped.
func (s *rpcServer) RequestedProcessShutdown() <-chan struct{} {
	return s.requestProcessShutdown
}
// NotifyNewTransactions notifies both websocket and getblocktemplate long poll clients of the passed transactions.  This function should be called whenever new transactions are added to the mempool.
func (s *rpcServer) NotifyNewTransactions(txns []*mempool.TxDesc) {
	for _, txD := range txns {
		// Notify websocket clients about mempool transactions.
		s.ntfnMgr.NotifyMempoolTx(txD.Tx, true)
		// Potentially notify any getblocktemplate long poll clients about stale block templates due to the new transaction.
		s.gbtWorkState.NotifyMempoolTx(s.cfg.TxMemPool.LastUpdated())
	}
}
// limitConnections responds with a 503 service unavailable and returns true if adding another client would exceed the maximum allow RPC clients. This function is safe for concurrent access.
func (s *rpcServer) limitConnections(w http.ResponseWriter, remoteAddr string) bool {
	if int(atomic.LoadInt32(&s.numClients)+1) > cfg.RPCMaxClients {
		log <- cl.Infof{
			"max RPC clients exceeded [%d] - disconnecting client %s",
			cfg.RPCMaxClients, remoteAddr}
		http.Error(w, "503 Too busy.  Try again later.",
			http.StatusServiceUnavailable)
		return true
	}
	return false
}
// incrementClients adds one to the number of connected RPC clients.  Note this only applies to standard clients.  Websocket clients have their own limits and are tracked separately. This function is safe for concurrent access.
func (s *rpcServer) incrementClients() {
	atomic.AddInt32(&s.numClients, 1)
}
// decrementClients subtracts one from the number of connected RPC clients. Note this only applies to standard clients.  Websocket clients have their own limits and are tracked separately. This function is safe for concurrent access.
func (s *rpcServer) decrementClients() {
	atomic.AddInt32(&s.numClients, -1)
}
// checkAuth checks the HTTP Basic authentication supplied by a wallet or RPC client in the HTTP request r.  If the supplied authentication does not match the username and password expected, a non-nil error is returned. This check is time-constant. The first bool return value signifies auth success (true if successful) and the second bool return value specifies whether the user can change the state of the server (true) or whether the user is limited (false). The second is always false if the first is.
func (s *rpcServer) checkAuth(r *http.Request, require bool) (bool, bool, error) {
	authhdr := r.Header["Authorization"]
	if len(authhdr) <= 0 {
		if require {
			log <- cl.Warn{"RPC authentication failure from", r.RemoteAddr}
			return false, false, errors.New("auth failure")
		}
		return false, false, nil
	}
	authsha := sha256.Sum256([]byte(authhdr[0]))
	// Check for limited auth first as in environments with limited users, those are probably expected to have a higher volume of calls
	limitcmp := subtle.ConstantTimeCompare(authsha[:], s.limitauthsha[:])
	if limitcmp == 1 {
		return true, false, nil
	}
	// Check for admin-level auth
	cmp := subtle.ConstantTimeCompare(authsha[:], s.authsha[:])
	if cmp == 1 {
		return true, true, nil
	}
	// Request's auth doesn't match either user
	log <- cl.Warn{"RPC authentication failure from", r.RemoteAddr}
	return false, false, errors.New("auth failure")
}
// parsedRPCCmd represents a JSON-RPC request object that has been parsed into a known concrete command along with any error that might have happened while parsing it.
type parsedRPCCmd struct {
	id     interface{}
	method string
	cmd    interface{}
	err    *json.RPCError
}
// standardCmdResult checks that a parsed command is a standard Bitcoin JSON-RPC command and runs the appropriate handler to reply to the command.  Any commands which are not recognized or not implemented will return an error suitable for use in replies.
func (s *rpcServer) standardCmdResult(cmd *parsedRPCCmd, closeChan <-chan struct{}) (interface{}, error) {
	handler, ok := rpcHandlers[cmd.method]
	if ok {
		goto handled
	}
	_, ok = rpcAskWallet[cmd.method]
	if ok {
		handler = handleAskWallet
		goto handled
	}
	_, ok = rpcUnimplemented[cmd.method]
	if ok {
		handler = handleUnimplemented
		goto handled
	}
	return nil, json.ErrRPCMethodNotFound
handled:
	return handler(s, cmd.cmd, closeChan)
}
// parseCmd parses a JSON-RPC request object into known concrete command.  The err field of the returned parsedRPCCmd struct will contain an RPC error that is suitable for use in replies if the command is invalid in some way such as an unregistered command or invalid parameters.
func parseCmd(request *json.Request) *parsedRPCCmd {
	var parsedCmd parsedRPCCmd
	parsedCmd.id = request.ID
	parsedCmd.method = request.Method
	cmd, err := json.UnmarshalCmd(request)
	if err != nil {
		// When the error is because the method is not registered, produce a method not found RPC error.
		if jerr, ok := err.(json.Error); ok &&
			jerr.ErrorCode == json.ErrUnregisteredMethod {
			parsedCmd.err = json.ErrRPCMethodNotFound
			return &parsedCmd
		}
		// Otherwise, some type of invalid parameters is the cause, so produce the equivalent RPC error.
		parsedCmd.err = json.NewRPCError(
			json.ErrRPCInvalidParams.Code, err.Error())
		return &parsedCmd
	}
	parsedCmd.cmd = cmd
	return &parsedCmd
}
// createMarshalledReply returns a new marshalled JSON-RPC response given the passed parameters.  It will automatically convert errors that are not of the type *json.RPCError to the appropriate type as needed.
func createMarshalledReply(id, result interface{}, replyErr error) ([]byte, error) {
	var jsonErr *json.RPCError
	if replyErr != nil {
		if jErr, ok := replyErr.(*json.RPCError); ok {
			jsonErr = jErr
		} else {
			jsonErr = internalRPCError(replyErr.Error(), "")
		}
	}
	return json.MarshalResponse(id, result, jsonErr)
}
// jsonRPCRead handles reading and responding to RPC messages.
func (s *rpcServer) jsonRPCRead(w http.ResponseWriter, r *http.Request, isAdmin bool) {
	if atomic.LoadInt32(&s.shutdown) != 0 {
		return
	}
	// Read and close the JSON-RPC request body from the caller.
	body, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		errCode := http.StatusBadRequest
		http.Error(w, fmt.Sprintf("%d error reading JSON message: %v",
			errCode, err), errCode)
		return
	}
	// Unfortunately, the http server doesn't provide the ability to change the read deadline for the new connection and having one breaks long polling.  However, not having a read deadline on the initial connection would mean clients can connect and idle forever.  Thus, hijack the connecton from the HTTP server, clear the read deadline, and handle writing the response manually.
	hj, ok := w.(http.Hijacker)
	if !ok {
		errMsg := "webserver doesn't support hijacking"
		log <- cl.Warnf{errMsg}
		errCode := http.StatusInternalServerError
		http.Error(w, strconv.Itoa(errCode)+" "+errMsg, errCode)
		return
	}
	conn, buf, err := hj.Hijack()
	if err != nil {
		log <- cl.Warn{"failed to hijack HTTP connection:", err}
		errCode := http.StatusInternalServerError
		http.Error(w, strconv.Itoa(errCode)+" "+err.Error(), errCode)
		return
	}
	defer conn.Close()
	defer buf.Flush()
	conn.SetReadDeadline(timeZeroVal)
	// Attempt to parse the raw body into a JSON-RPC request.
	var responseID interface{}
	var jsonErr error
	var result interface{}
	var request json.Request
	if err := js.Unmarshal(body, &request); err != nil {
		jsonErr = &json.RPCError{
			Code:    json.ErrRPCParse.Code,
			Message: "Failed to parse request: " + err.Error(),
		}
	}
	if jsonErr == nil {
		// The JSON-RPC 1.0 spec defines that notifications must have their "id" set to null and states that notifications do not have a response. A JSON-RPC 2.0 notification is a request with "json-rpc":"2.0", and without an "id" member. The specification states that notifications must not be responded to. JSON-RPC 2.0 permits the null value as a valid request id, therefore such requests are not notifications. Bitcoin Core serves requests with "id":null or even an absent "id", and responds to such requests with "id":null in the response. Pod does not respond to any request without and "id" or "id":null, regardless the indicated JSON-RPC protocol version unless RPC quirks are enabled. With RPC quirks enabled, such requests will be responded to if the reqeust does not indicate JSON-RPC version. RPC quirks can be enabled by the user to avoid compatibility issues with software relying on Core's behavior.
		if request.ID == nil && !(cfg.RPCQuirks && request.Jsonrpc == "") {
			return
		}
		// The parse was at least successful enough to have an ID so set it for the response.
		responseID = request.ID
		// Setup a close notifier.  Since the connection is hijacked, the CloseNotifer on the ResponseWriter is not available.
		closeChan := make(chan struct{}, 1)
		go func() {
			_, err := conn.Read(make([]byte, 1))
			if err != nil {
				close(closeChan)
			}
		}()
		// Check if the user is limited and set error if method unauthorized
		if !isAdmin {
			if _, ok := rpcLimited[request.Method]; !ok {
				jsonErr = &json.RPCError{
					Code:    json.ErrRPCInvalidParams.Code,
					Message: "limited user not authorized for this method",
				}
			}
		}
		if jsonErr == nil {
			// Attempt to parse the JSON-RPC request into a known concrete command.
			parsedCmd := parseCmd(&request)
			if parsedCmd.err != nil {
				jsonErr = parsedCmd.err
			} else {
				result, jsonErr = s.standardCmdResult(parsedCmd, closeChan)
			}
		}
	}
	// Marshal the response.
	msg, err := createMarshalledReply(responseID, result, jsonErr)
	if err != nil {
		log <- cl.Error{"failed to marshal reply:", err}
		return
	}
	// Write the response.
	err = s.writeHTTPResponseHeaders(r, w.Header(), http.StatusOK, buf)
	if err != nil {
		log <- cl.Error{err.Error()}
		return
	}
	if _, err := buf.Write(msg); err != nil {
		log <- cl.Error{"failed to write marshalled reply:", err}
	}
	// Terminate with newline to maintain compatibility with Bitcoin Core.
	if err := buf.WriteByte('\n'); err != nil {
		log <- cl.Error{"failed to append terminating newline to reply:", err}
	}
}
// jsonAuthFail sends a message back to the client if the http auth is rejected.
func jsonAuthFail(w http.ResponseWriter) {
	w.Header().Add("WWW-Authenticate", `Basic realm="pod RPC"`)
	http.Error(w, "401 Unauthorized.", http.StatusUnauthorized)
}
// Start is used by server.go to start the rpc listener.
func (s *rpcServer) Start() {
	if atomic.AddInt32(&s.started, 1) != 1 {
		return
	}
	rpcServeMux := http.NewServeMux()
	httpServer := &http.Server{
		Handler: rpcServeMux,
		// Timeout connections which don't complete the initial handshake within the allowed timeframe.
		ReadTimeout: time.Second * rpcAuthTimeoutSeconds,
	}
	rpcServeMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Connection", "close")
		w.Header().Set("Content-Type", "application/json")
		r.Close = true
		// Limit the number of connections to max allowed.
		if s.limitConnections(w, r.RemoteAddr) {
			return
		}
		// Keep track of the number of connected clients.
		s.incrementClients()
		defer s.decrementClients()
		_, isAdmin, err := s.checkAuth(r, true)
		if err != nil {
			jsonAuthFail(w)
			return
		}
		// Read and respond to the request.
		s.jsonRPCRead(w, r, isAdmin)
	})
	// Websocket endpoint.
	rpcServeMux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		authenticated, isAdmin, err := s.checkAuth(r, false)
		if err != nil {
			jsonAuthFail(w)
			return
		}
		// Attempt to upgrade the connection to a websocket connection using the default size for read/write buffers.
		ws, err := websocket.Upgrade(w, r, nil, 0, 0)
		if err != nil {
			if _, ok := err.(websocket.HandshakeError); !ok {
				log <- cl.Error{"unexpected websocket error:", err}
			}
			http.Error(w, "400 Bad Request.", http.StatusBadRequest)
			return
		}
		s.WebsocketHandler(ws, r.RemoteAddr, authenticated, isAdmin)
	})
	for _, listener := range s.cfg.Listeners {
		s.wg.Add(1)
		go func(listener net.Listener) {
			log <- cl.Info{"RPC server listening on", listener.Addr()}
			httpServer.Serve(listener)
			log <- cl.Trace{"RPC listener done for", listener.Addr()}
			s.wg.Done()
		}(listener)
	}
	s.ntfnMgr.Start()
}
// genCertPair generates a key/cert pair to the paths provided.
func genCertPair(certFile, keyFile string) error {
	log <- cl.Inf("generating TLS certificates...")
	org := "pod autogenerated cert"
	validUntil := time.Now().Add(10 * 365 * 24 * time.Hour)
	cert, key, err := util.NewTLSCertPair(org, validUntil, nil)
	if err != nil {
		return err
	}
	// Write cert and key files.
	if err = ioutil.WriteFile(certFile, cert, 0666); err != nil {
		return err
	}
	if err = ioutil.WriteFile(keyFile, key, 0600); err != nil {
		os.Remove(certFile)
		return err
	}
	log <- cl.Inf("Done generating TLS certificates")
	return nil
}
// rpcserverPeer represents a peer for use with the RPC server. The interface contract requires that all of these methods are safe for concurrent access.
type rpcserverPeer interface {
	// ToPeer returns the underlying peer instance.
	ToPeer() *p.Peer
	// IsTxRelayDisabled returns whether or not the peer has disabled transaction relay.
	IsTxRelayDisabled() bool
	// BanScore returns the current integer value that represents how close the peer is to being banned.
	BanScore() uint32
	// FeeFilter returns the requested current minimum fee rate for which transactions should be announced.
	FeeFilter() int64
}
// rpcserverConnManager represents a connection manager for use with the RPC server. The interface contract requires that all of these methods are safe for concurrent access.
type rpcserverConnManager interface {
	// Connect adds the provided address as a new outbound peer.  The permanent flag indicates whether or not to make the peer persistent and reconnect if the connection is lost.  Attempting to connect to an already existing peer will return an error.
	Connect(addr string, permanent bool) error
	// RemoveByID removes the peer associated with the provided id from the list of persistent peers.  Attempting to remove an id that does not exist will return an error.
	RemoveByID(id int32) error
	// RemoveByAddr removes the peer associated with the provided address from the list of persistent peers.  Attempting to remove an address that does not exist will return an error.
	RemoveByAddr(addr string) error
	// DisconnectByID disconnects the peer associated with the provided id. This applies to both inbound and outbound peers.  Attempting to remove an id that does not exist will return an error.
	DisconnectByID(id int32) error
	// DisconnectByAddr disconnects the peer associated with the provided address.  This applies to both inbound and outbound peers. Attempting to remove an address that does not exist will return an error.
	DisconnectByAddr(addr string) error
	// ConnectedCount returns the number of currently connected peers.
	ConnectedCount() int32
	// NetTotals returns the sum of all bytes received and sent across the network for all peers.
	NetTotals() (uint64, uint64)
	// ConnectedPeers returns an array consisting of all connected peers.
	ConnectedPeers() []rpcserverPeer
	// PersistentPeers returns an array consisting of all the persistent peers.
	PersistentPeers() []rpcserverPeer
	// BroadcastMessage sends the provided message to all currently connected peers.
	BroadcastMessage(msg wire.Message)
	// AddRebroadcastInventory adds the provided inventory to the list of inventories to be rebroadcast at random intervals until they show up in a block.
	AddRebroadcastInventory(iv *wire.InvVect, data interface{})
	// RelayTransactions generates and relays inventory vectors for all of the passed transactions to all connected peers.
	RelayTransactions(txns []*mempool.TxDesc)
}
// rpcserverSyncManager represents a sync manager for use with the RPC server. The interface contract requires that all of these methods are safe for concurrent access.
type rpcserverSyncManager interface {
	// IsCurrent returns whether or not the sync manager believes the chain is current as compared to the rest of the network.
	IsCurrent() bool
	// SubmitBlock submits the provided block to the network after processing it locally.
	SubmitBlock(block *util.Block, flags blockchain.BehaviorFlags) (bool, error)
	// Pause pauses the sync manager until the returned channel is closed.
	Pause() chan<- struct{}
	// SyncPeerID returns the ID of the peer that is currently the peer being used to sync from or 0 if there is none.
	SyncPeerID() int32
	// LocateHeaders returns the headers of the blocks after the first known block in the provided locators until the provided stop hash or the current tip is reached, up to a max of wire.MaxBlockHeadersPerMsg hashes.
	LocateHeaders(locators []*chainhash.Hash, hashStop *chainhash.Hash) []wire.BlockHeader
}
// rpcserverConfig is a descriptor containing the RPC server configuration.
type rpcserverConfig struct {
	// Listeners defines a slice of listeners for which the RPC server will take ownership of and accept connections.  Since the RPC server takes ownership of these listeners, they will be closed when the RPC server is stopped.
	Listeners []net.Listener
	// StartupTime is the unix timestamp for when the server that is hosting the RPC server started.
	StartupTime int64
	// ConnMgr defines the connection manager for the RPC server to use.  It provides the RPC server with a means to do things such as add, remove, connect, disconnect, and query peers as well as other connection-related data and tasks.
	ConnMgr rpcserverConnManager
	// SyncMgr defines the sync manager for the RPC server to use.
	SyncMgr rpcserverSyncManager
	// These fields allow the RPC server to interface with the local block chain data and state.
	TimeSource  blockchain.MedianTimeSource
	Chain       *blockchain.BlockChain
	ChainParams *chaincfg.Params
	DB          database.DB
	// TxMemPool defines the transaction memory pool to interact with.
	TxMemPool *mempool.TxPool
	// These fields allow the RPC server to interface with mining. Generator produces block templates and the CPUMiner solves them using the CPU.  CPU mining is typically only useful for test purposes when doing regression or simulation testing.
	Generator *mining.BlkTmplGenerator
	CPUMiner  *cpuminer.CPUMiner
	// These fields define any optional indexes the RPC server can make use of to provide additional data when queried.
	TxIndex   *indexers.TxIndex
	AddrIndex *indexers.AddrIndex
	CfIndex   *indexers.CfIndex
	// The fee estimator keeps track of how long transactions are left in the mempool before they are mined into blocks.
	FeeEstimator *mempool.FeeEstimator
	// Algo sets the algorithm expected from the RPC endpoint. This allows multiple ports to serve multiple types of miners with one main node per algorithm. Currently 514 for scrypt and anything else passes for sha256d. After hard fork 1 there is 9, and may be expanded in the future (equihash, cuckoo and cryptonight all require substantial block header/tx formatting changes)
	Algo string
}
// newRPCServer returns a new instance of the rpcServer struct.
func newRPCServer(config *rpcserverConfig) (*rpcServer, error) {
	rpc := rpcServer{
		cfg:                    *config,
		statusLines:            make(map[int]string),
		gbtWorkState:           newGbtWorkState(config.TimeSource, config.Algo),
		helpCacher:             newHelpCacher(),
		requestProcessShutdown: make(chan struct{}),
		quit:                   make(chan int),
	}
	if cfg.RPCUser != "" && cfg.RPCPass != "" {
		login := cfg.RPCUser + ":" + cfg.RPCPass
		auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(login))
		rpc.authsha = sha256.Sum256([]byte(auth))
	}
	if cfg.RPCLimitUser != "" && cfg.RPCLimitPass != "" {
		login := cfg.RPCLimitUser + ":" + cfg.RPCLimitPass
		auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(login))
		rpc.limitauthsha = sha256.Sum256([]byte(auth))
	}
	rpc.ntfnMgr = newWsNotificationManager(&rpc)
	rpc.cfg.Chain.Subscribe(rpc.handleBlockchainNotification)
	return &rpc, nil
}
// Callback for notifications from blockchain.  It notifies clients that are long polling for changes or subscribed to websockets notifications.
func (s *rpcServer) handleBlockchainNotification(notification *blockchain.Notification) {
	switch notification.Type {
	case blockchain.NTBlockAccepted:
		block, ok := notification.Data.(*util.Block)
		if !ok {
			log <- cl.Wrn("chain accepted notification is not a block")
			break
		}
		// Allow any clients performing long polling via the getblocktemplate RPC to be notified when the new block causes their old block template to become stale.
		s.gbtWorkState.NotifyBlockConnected(block.Hash())
	case blockchain.NTBlockConnected:
		block, ok := notification.Data.(*util.Block)
		if !ok {
			log <- cl.Wrn("chain connected notification is not a block")
			break
		}
		// Notify registered websocket clients of incoming block.
		s.ntfnMgr.NotifyBlockConnected(block)
	case blockchain.NTBlockDisconnected:
		block, ok := notification.Data.(*util.Block)
		if !ok {
			log <- cl.Wrn("chain disconnected notification is not a block.")
			break
		}
		// Notify registered websocket clients.
		s.ntfnMgr.NotifyBlockDisconnected(block)
	}
}
func init() {
	rpcHandlers = rpcHandlersBeforeInit
	rand.Seed(time.Now().UnixNano())
}
package node
import (
	"errors"
	"sort"
	"strings"
	"sync"
	json "git.parallelcoin.io/pod/pkg/json"
)
// helpDescsEnUS defines the English descriptions used for the help strings.
var helpDescsEnUS = map[string]string{
	// DebugLevelCmd help.
	"debuglevel--synopsis": "Dynamically changes the debug logging level.\n" +
		"The levelspec can either a debug level or of the form:\n" +
		"<subsystem>=<level>,<subsystem2>=<level2>,...\n" +
		"The valid debug levels are trace, debug, info, warn, error, and critical.\n" +
		"The valid subsystems are AMGR, ADXR, BCDB, BMGR, NODE, CHAN, DISC, PEER, RPCS, SCRP, SRVR, and TXMP.\n" +
		"Finally the keyword 'show' will return a list of the available subsystems.",
	"debuglevel-levelspec":   "The debug level(s) to use or the keyword 'show'",
	"debuglevel--condition0": "levelspec!=show",
	"debuglevel--condition1": "levelspec=show",
	"debuglevel--result0":    "The string 'Done.'",
	"debuglevel--result1":    "The list of subsystems",
	// AddNodeCmd help.
	"addnode--synopsis": "Attempts to add or remove a persistent peer.",
	"addnode-addr":      "IP address and port of the peer to operate on",
	"addnode-subcmd":    "'add' to add a persistent peer, 'remove' to remove a persistent peer, or 'onetry' to try a single connection to a peer",
	// NodeCmd help.
	"node--synopsis":     "Attempts to add or remove a peer.",
	"node-subcmd":        "'disconnect' to remove all matching non-persistent peers, 'remove' to remove a persistent peer, or 'connect' to connect to a peer",
	"node-target":        "Either the IP address and port of the peer to operate on, or a valid peer ID.",
	"node-connectsubcmd": "'perm' to make the connected peer a permanent one, 'temp' to try a single connect to a peer",
	// TransactionInput help.
	"transactioninput-txid": "The hash of the input transaction",
	"transactioninput-vout": "The specific output of the input transaction to redeem",
	// CreateRawTransactionCmd help.
	"createrawtransaction--synopsis": "Returns a new transaction spending the provided inputs and sending to the provided addresses.\n" +
		"The transaction inputs are not signed in the created transaction.\n" +
		"The signrawtransaction RPC command provided by wallet must be used to sign the resulting transaction.",
	"createrawtransaction-inputs":         "The inputs to the transaction",
	"createrawtransaction-amounts":        "JSON object with the destination addresses as keys and amounts as values",
	"createrawtransaction-amounts--key":   "address",
	"createrawtransaction-amounts--value": "n.nnn",
	"createrawtransaction-amounts--desc":  "The destination address as the key and the amount in DUO as the value",
	"createrawtransaction-locktime":       "Locktime value; a non-zero value will also locktime-activate the inputs",
	"createrawtransaction--result0":       "Hex-encoded bytes of the serialized transaction",
	// ScriptSig help.
	"scriptsig-asm": "Disassembly of the script",
	"scriptsig-hex": "Hex-encoded bytes of the script",
	// PrevOut help.
	"prevout-addresses": "previous output addresses",
	"prevout-value":     "previous output value",
	// VinPrevOut help.
	"vinprevout-coinbase":    "The hex-encoded bytes of the signature script (coinbase txns only)",
	"vinprevout-txid":        "The hash of the origin transaction (non-coinbase txns only)",
	"vinprevout-vout":        "The index of the output being redeemed from the origin transaction (non-coinbase txns only)",
	"vinprevout-scriptSig":   "The signature script used to redeem the origin transaction as a JSON object (non-coinbase txns only)",
	"vinprevout-txinwitness": "The witness stack of the passed input, encoded as a JSON string array",
	"vinprevout-prevOut":     "Data from the origin transaction output with index vout.",
	"vinprevout-sequence":    "The script sequence number",
	// Vin help.
	"vin-coinbase":    "The hex-encoded bytes of the signature script (coinbase txns only)",
	"vin-txid":        "The hash of the origin transaction (non-coinbase txns only)",
	"vin-vout":        "The index of the output being redeemed from the origin transaction (non-coinbase txns only)",
	"vin-scriptSig":   "The signature script used to redeem the origin transaction as a JSON object (non-coinbase txns only)",
	"vin-txinwitness": "The witness used to redeem the input encoded as a string array of its items",
	"vin-sequence":    "The script sequence number",
	// ScriptPubKeyResult help.
	"scriptpubkeyresult-asm":       "Disassembly of the script",
	"scriptpubkeyresult-hex":       "Hex-encoded bytes of the script",
	"scriptpubkeyresult-reqSigs":   "The number of required signatures",
	"scriptpubkeyresult-type":      "The type of the script (e.g. 'pubkeyhash')",
	"scriptpubkeyresult-addresses": "The bitcoin addresses associated with this script",
	// Vout help.
	"vout-value":        "The amount in DUO",
	"vout-n":            "The index of this transaction output",
	"vout-scriptPubKey": "The public key script used to pay coins as a JSON object",
	// TxRawDecodeResult help.
	"txrawdecoderesult-txid":     "The hash of the transaction",
	"txrawdecoderesult-version":  "The transaction version",
	"txrawdecoderesult-locktime": "The transaction lock time",
	"txrawdecoderesult-vin":      "The transaction inputs as JSON objects",
	"txrawdecoderesult-vout":     "The transaction outputs as JSON objects",
	// DecodeRawTransactionCmd help.
	"decoderawtransaction--synopsis": "Returns a JSON object representing the provided serialized, hex-encoded transaction.",
	"decoderawtransaction-hextx":     "Serialized, hex-encoded transaction",
	// DecodeScriptResult help.
	"decodescriptresult-asm":       "Disassembly of the script",
	"decodescriptresult-reqSigs":   "The number of required signatures",
	"decodescriptresult-type":      "The type of the script (e.g. 'pubkeyhash')",
	"decodescriptresult-addresses": "The bitcoin addresses associated with this script",
	"decodescriptresult-p2sh":      "The script hash for use in pay-to-script-hash transactions (only present if the provided redeem script is not already a pay-to-script-hash script)",
	// DecodeScriptCmd help.
	"decodescript--synopsis": "Returns a JSON object with information about the provided hex-encoded script.",
	"decodescript-hexscript": "Hex-encoded script",
	// EstimateFeeCmd help.
	"estimatefee--synopsis": "Estimate the fee per kilobyte in satoshis " +
		"required for a transaction to be mined before a certain number of " +
		"blocks have been generated.",
	"estimatefee-numblocks": "The maximum number of blocks which can be " +
		"generated before the transaction is mined.",
	"estimatefee--result0": "Estimated fee per kilobyte in satoshis for a block to " +
		"be mined in the next NumBlocks blocks.",
	// GenerateCmd help
	"generate--synopsis": "Generates a set number of blocks (simnet or regtest only) and returns a JSON\n" +
		" array of their hashes.",
	"generate-numblocks": "Number of blocks to generate",
	"generate--result0":  "The hashes, in order, of blocks generated by the call",
	// GetAddedNodeInfoResultAddr help.
	"getaddednodeinforesultaddr-address":   "The ip address for this DNS entry",
	"getaddednodeinforesultaddr-connected": "The connection 'direction' (inbound/outbound/false)",
	// GetAddedNodeInfoResult help.
	"getaddednodeinforesult-addednode": "The ip address or domain of the added peer",
	"getaddednodeinforesult-connected": "Whether or not the peer is currently connected",
	"getaddednodeinforesult-addresses": "DNS lookup and connection information about the peer",
	// GetAddedNodeInfo help.
	"getaddednodeinfo--synopsis":   "Returns information about manually added (persistent) peers.",
	"getaddednodeinfo-dns":         "Specifies whether the returned data is a JSON object including DNS and connection information, or just a list of added peers",
	"getaddednodeinfo-node":        "Only return information about this specific peer instead of all added peers",
	"getaddednodeinfo--condition0": "dns=false",
	"getaddednodeinfo--condition1": "dns=true",
	"getaddednodeinfo--result0":    "List of added peers",
	// GetBestBlockResult help.
	"getbestblockresult-hash":   "Hex-encoded bytes of the best block hash",
	"getbestblockresult-height": "Height of the best block",
	// GetBestBlockCmd help.
	"getbestblock--synopsis": "Get block height and hash of best block in the main chain.",
	"getbestblock--result0":  "Get block height and hash of best block in the main chain.",
	// GetBestBlockHashCmd help.
	"getbestblockhash--synopsis": "Returns the hash of the of the best (most recent) block in the longest block chain.",
	"getbestblockhash--result0":  "The hex-encoded block hash",
	// GetBlockCmd help.
	"getblock--synopsis":   "Returns information about a block given its hash.",
	"getblock-hash":        "The hash of the block",
	"getblock-verbose":     "Specifies the block is returned as a JSON object instead of hex-encoded string",
	"getblock-verbosetx":   "Specifies that each transaction is returned as a JSON object and only applies if the verbose flag is true (pod extension)",
	"getblock--condition0": "verbose=false",
	"getblock--condition1": "verbose=true",
	"getblock--result0":    "Hex-encoded bytes of the serialized block",
	// GetBlockChainInfoCmd help.
	"getblockchaininfo--synopsis": "Returns information about the current blockchain state and the status of any active soft-fork deployments.",
	// GetBlockChainInfoResult help.
	"getblockchaininforesult-chain":                 "The name of the chain the daemon is on (testnet, mainnet, etc)",
	"getblockchaininforesult-blocks":                "The number of blocks in the best known chain",
	"getblockchaininforesult-headers":               "The number of headers that we've gathered for in the best known chain",
	"getblockchaininforesult-bestblockhash":         "The block hash for the latest block in the main chain",
	"getblockchaininforesult-difficulty":            "The current chain difficulty",
	"getblockchaininforesult-mediantime":            "The median time from the PoV of the best block in the chain",
	"getblockchaininforesult-verificationprogress":  "An estimate for how much of the best chain we've verified",
	"getblockchaininforesult-pruned":                "A bool that indicates if the node is pruned or not",
	"getblockchaininforesult-pruneheight":           "The lowest block retained in the current pruned chain",
	"getblockchaininforesult-chainwork":             "The total cumulative work in the best chain",
	"getblockchaininforesult-softforks":             "The status of the super-majority soft-forks",
	"getblockchaininforesult-bip9_softforks":        "JSON object describing active BIP0009 deployments",
	"getblockchaininforesult-bip9_softforks--key":   "bip9_softforks",
	"getblockchaininforesult-bip9_softforks--value": "An object describing a particular BIP009 deployment",
	"getblockchaininforesult-bip9_softforks--desc":  "The status of any defined BIP0009 soft-fork deployments",
	// SoftForkDescription help.
	"softforkdescription-reject":  "The current activation status of the softfork",
	"softforkdescription-version": "The block version that signals enforcement of this softfork",
	"softforkdescription-id":      "The string identifier for the soft fork",
	"-status":                     "A bool which indicates if the soft fork is active",
	// TxRawResult help.
	"txrawresult-hex":           "Hex-encoded transaction",
	"txrawresult-txid":          "The hash of the transaction",
	"txrawresult-version":       "The transaction version",
	"txrawresult-locktime":      "The transaction lock time",
	"txrawresult-vin":           "The transaction inputs as JSON objects",
	"txrawresult-vout":          "The transaction outputs as JSON objects",
	"txrawresult-blockhash":     "Hash of the block the transaction is part of",
	"txrawresult-confirmations": "Number of confirmations of the block",
	"txrawresult-time":          "Transaction time in seconds since 1 Jan 1970 GMT",
	"txrawresult-blocktime":     "Block time in seconds since the 1 Jan 1970 GMT",
	"txrawresult-size":          "The size of the transaction in bytes",
	"txrawresult-vsize":         "The virtual size of the transaction in bytes",
	"txrawresult-hash":          "The wtxid of the transaction",
	// SearchRawTransactionsResult help.
	"searchrawtransactionsresult-hex":           "Hex-encoded transaction",
	"searchrawtransactionsresult-txid":          "The hash of the transaction",
	"searchrawtransactionsresult-hash":          "The wxtid of the transaction",
	"searchrawtransactionsresult-version":       "The transaction version",
	"searchrawtransactionsresult-locktime":      "The transaction lock time",
	"searchrawtransactionsresult-vin":           "The transaction inputs as JSON objects",
	"searchrawtransactionsresult-vout":          "The transaction outputs as JSON objects",
	"searchrawtransactionsresult-blockhash":     "Hash of the block the transaction is part of",
	"searchrawtransactionsresult-confirmations": "Number of confirmations of the block",
	"searchrawtransactionsresult-time":          "Transaction time in seconds since 1 Jan 1970 GMT",
	"searchrawtransactionsresult-blocktime":     "Block time in seconds since the 1 Jan 1970 GMT",
	"searchrawtransactionsresult-size":          "The size of the transaction in bytes",
	"searchrawtransactionsresult-vsize":         "The virtual size of the transaction in bytes",
	// GetBlockVerboseResult help.
	"getblockverboseresult-hash":              "The hash of the block (same as provided)",
	"getblockverboseresult-confirmations":     "The number of confirmations",
	"getblockverboseresult-size":              "The size of the block",
	"getblockverboseresult-height":            "The height of the block in the block chain",
	"getblockverboseresult-version":           "The block version",
	"getblockverboseresult-versionHex":        "The block version in hexadecimal",
	"getblockverboseresult-merkleroot":        "Root hash of the merkle tree",
	"getblockverboseresult-tx":                "The transaction hashes (only when verbosetx=false)",
	"getblockverboseresult-rawtx":             "The transactions as JSON objects (only when verbosetx=true)",
	"getblockverboseresult-time":              "The block time in seconds since 1 Jan 1970 GMT",
	"getblockverboseresult-nonce":             "The block nonce",
	"getblockverboseresult-bits":              "The bits which represent the block difficulty",
	"getblockverboseresult-difficulty":        "The proof-of-work difficulty as a multiple of the minimum difficulty",
	"getblockverboseresult-previousblockhash": "The hash of the previous block",
	"getblockverboseresult-nextblockhash":     "The hash of the next block (only if there is one)",
	"getblockverboseresult-strippedsize":      "The size of the block without witness data",
	"getblockverboseresult-weight":            "The weight of the block",
	// GetBlockCountCmd help.
	"getblockcount--synopsis": "Returns the number of blocks in the longest block chain.",
	"getblockcount--result0":  "The current block count",
	// GetBlockHashCmd help.
	"getblockhash--synopsis": "Returns hash of the block in best block chain at the given height.",
	"getblockhash-index":     "The block height",
	"getblockhash--result0":  "The block hash",
	// GetBlockHeaderCmd help.
	"getblockheader--synopsis":   "Returns information about a block header given its hash.",
	"getblockheader-hash":        "The hash of the block",
	"getblockheader-verbose":     "Specifies the block header is returned as a JSON object instead of hex-encoded string",
	"getblockheader--condition0": "verbose=false",
	"getblockheader--condition1": "verbose=true",
	"getblockheader--result0":    "The block header hash",
	// GetBlockHeaderVerboseResult help.
	"getblockheaderverboseresult-hash":              "The hash of the block (same as provided)",
	"getblockheaderverboseresult-confirmations":     "The number of confirmations",
	"getblockheaderverboseresult-height":            "The height of the block in the block chain",
	"getblockheaderverboseresult-version":           "The block version",
	"getblockheaderverboseresult-versionHex":        "The block version in hexadecimal",
	"getblockheaderverboseresult-merkleroot":        "Root hash of the merkle tree",
	"getblockheaderverboseresult-time":              "The block time in seconds since 1 Jan 1970 GMT",
	"getblockheaderverboseresult-nonce":             "The block nonce",
	"getblockheaderverboseresult-bits":              "The bits which represent the block difficulty",
	"getblockheaderverboseresult-difficulty":        "The proof-of-work difficulty as a multiple of the minimum difficulty",
	"getblockheaderverboseresult-previousblockhash": "The hash of the previous block",
	"getblockheaderverboseresult-nextblockhash":     "The hash of the next block (only if there is one)",
	// TemplateRequest help.
	"templaterequest-mode":         "This is 'template', 'proposal', or omitted",
	"templaterequest-capabilities": "List of capabilities",
	"templaterequest-longpollid":   "The long poll ID of a job to monitor for expiration; required and valid only for long poll requests ",
	"templaterequest-sigoplimit":   "Number of signature operations allowed in blocks (this parameter is ignored)",
	"templaterequest-sizelimit":    "Number of bytes allowed in blocks (this parameter is ignored)",
	"templaterequest-maxversion":   "Highest supported block version number (this parameter is ignored)",
	"templaterequest-target":       "The desired target for the block template (this parameter is ignored)",
	"templaterequest-data":         "Hex-encoded block data (only for mode=proposal)",
	"templaterequest-workid":       "The server provided workid if provided in block template (not applicable)",
	// GetBlockTemplateResultTx help.
	"getblocktemplateresulttx-data":    "Hex-encoded transaction data (byte-for-byte)",
	"getblocktemplateresulttx-hash":    "Hex-encoded transaction hash (little endian if treated as a 256-bit number)",
	"getblocktemplateresulttx-depends": "Other transactions before this one (by 1-based index in the 'transactions'  list) that must be present in the final block if this one is",
	"getblocktemplateresulttx-fee":     "Difference in value between transaction inputs and outputs (in Satoshi)",
	"getblocktemplateresulttx-sigops":  "Total number of signature operations as counted for purposes of block limits",
	"getblocktemplateresulttx-weight":  "The weight of the transaction",
	// GetBlockTemplateResultAux help.
	"getblocktemplateresultaux-flags": "Hex-encoded byte-for-byte data to include in the coinbase signature script",
	// GetBlockTemplateResult help.
	"getblocktemplateresult-bits":                       "Hex-encoded compressed difficulty",
	"getblocktemplateresult-curtime":                    "Current time as seen by the server (recommended for block time); must fall within mintime/maxtime rules",
	"getblocktemplateresult-height":                     "Height of the block to be solved",
	"getblocktemplateresult-previousblockhash":          "Hex-encoded big-endian hash of the previous block",
	"getblocktemplateresult-sigoplimit":                 "Number of sigops allowed in blocks ",
	"getblocktemplateresult-sizelimit":                  "Number of bytes allowed in blocks",
	"getblocktemplateresult-transactions":               "Array of transactions as JSON objects",
	"getblocktemplateresult-version":                    "The block version",
	"getblocktemplateresult-coinbaseaux":                "Data that should be included in the coinbase signature script",
	"getblocktemplateresult-coinbasetxn":                "Information about the coinbase transaction",
	"getblocktemplateresult-coinbasevalue":              "Total amount available for the coinbase in Satoshi",
	"getblocktemplateresult-workid":                     "This value must be returned with result if provided (not provided)",
	"getblocktemplateresult-longpollid":                 "Identifier for long poll request which allows monitoring for expiration",
	"getblocktemplateresult-longpolluri":                "An alternate URI to use for long poll requests if provided (not provided)",
	"getblocktemplateresult-submitold":                  "Not applicable",
	"getblocktemplateresult-target":                     "Hex-encoded big-endian number which valid results must be less than",
	"getblocktemplateresult-expires":                    "Maximum number of seconds (starting from when the server sent the response) this work is valid for",
	"getblocktemplateresult-maxtime":                    "Maximum allowed time",
	"getblocktemplateresult-mintime":                    "Minimum allowed time",
	"getblocktemplateresult-mutable":                    "List of mutations the server explicitly allows",
	"getblocktemplateresult-noncerange":                 "Two concatenated hex-encoded big-endian 32-bit integers which represent the valid ranges of nonces the miner may scan",
	"getblocktemplateresult-capabilities":               "List of server capabilities including 'proposal' to indicate support for block proposals",
	"getblocktemplateresult-reject-reason":              "Reason the proposal was invalid as-is (only applies to proposal responses)",
	"getblocktemplateresult-default_witness_commitment": "The witness commitment itself. Will be populated if the block has witness data",
	"getblocktemplateresult-weightlimit":                "The current limit on the max allowed weight of a block",
	// GetBlockTemplateCmd help.
	"getblocktemplate--synopsis": "Returns a JSON object with information necessary to construct a block to mine or accepts a proposal to validate.\n" +
		"See BIP0022 and BIP0023 for the full specification.",
	"getblocktemplate-request":     "Request object which controls the mode and several parameters",
	"getblocktemplate--condition0": "mode=template",
	"getblocktemplate--condition1": "mode=proposal, rejected",
	"getblocktemplate--condition2": "mode=proposal, accepted",
	"getblocktemplate--result1":    "An error string which represents why the proposal was rejected or nothing if accepted",
	// GetCFilterCmd help.
	"getcfilter--synopsis":  "Returns a block's committed filter given its hash.",
	"getcfilter-filtertype": "The type of filter to return (0=regular)",
	"getcfilter-hash":       "The hash of the block",
	"getcfilter--result0":   "The block's committed filter",
	// GetCFilterHeaderCmd help.
	"getcfilterheader--synopsis":  "Returns a block's compact filter header given its hash.",
	"getcfilterheader-filtertype": "The type of filter header to return (0=regular)",
	"getcfilterheader-hash":       "The hash of the block",
	"getcfilterheader--result0":   "The block's gcs filter header",
	// GetConnectionCountCmd help.
	"getconnectioncount--synopsis": "Returns the number of active connections to other peers.",
	"getconnectioncount--result0":  "The number of connections",
	// GetCurrentNetCmd help.
	"getcurrentnet--synopsis": "Get bitcoin network the server is running on.",
	"getcurrentnet--result0":  "The network identifer",
	// GetDifficultyCmd help.
	"getdifficulty--synopsis":   "Returns the proof-of-work difficulty as a multiple of the minimum difficulty, according to the currently configured cpu mining algorithm.",
	"getdifficulty-algo":        "Defaults to the configured --algo for the CPU miner, can be set to sha256 or scrypt",
	"getdifficulty--condition0": "algo=sha256d or scrypt",
	"getdifficulty--result0":    "The difficulty of the requested algorithm",
	// GetGenerateCmd help.
	"getgenerate--synopsis": "Returns if the server is set to generate coins (mine) or not.",
	"getgenerate--result0":  "True if mining, false if not",
	// GetHashesPerSecCmd help.
	"gethashespersec--synopsis": "Returns a recent hashes per second performance measurement while generating coins (mining).",
	"gethashespersec--result0":  "The number of hashes per second",
	// InfoChainResult help.
	"infochainresult-version":         "The version of the server",
	"infochainresult-protocolversion": "The latest supported protocol version",
	"infochainresult-blocks":          "The number of blocks processed",
	"infochainresult-timeoffset":      "The time offset",
	"infochainresult-connections":     "The number of connected peers",
	"infochainresult-proxy":           "The proxy used by the server",
	"infochainresult-difficulty":      "The current target difficulty",
	"infochainresult-testnet":         "Whether or not server is using testnet",
	"infochainresult-relayfee":        "The minimum relay fee for non-free transactions in DUO/KB",
	"infochainresult-errors":          "Any current errors",
	// InfoWalletResult help.
	"infowalletresult-version":         "The version of the server",
	"infowalletresult-protocolversion": "The latest supported protocol version",
	"infowalletresult-walletversion":   "The version of the wallet server",
	"infowalletresult-balance":         "The total bitcoin balance of the wallet",
	"infowalletresult-blocks":          "The number of blocks processed",
	"infowalletresult-timeoffset":      "The time offset",
	"infowalletresult-connections":     "The number of connected peers",
	"infowalletresult-proxy":           "The proxy used by the server",
	"infowalletresult-difficulty":      "The current target difficulty",
	"infowalletresult-testnet":         "Whether or not server is using testnet",
	"infowalletresult-keypoololdest":   "Seconds since 1 Jan 1970 GMT of the oldest pre-generated key in the key pool",
	"infowalletresult-keypoolsize":     "The number of new keys that are pre-generated",
	"infowalletresult-unlocked_until":  "The timestamp in seconds since 1 Jan 1970 GMT that the wallet is unlocked for transfers, or 0 if the wallet is locked",
	"infowalletresult-paytxfee":        "The transaction fee set in DUO/KB",
	"infowalletresult-relayfee":        "The minimum relay fee for non-free transactions in DUO/KB",
	"infowalletresult-errors":          "Any current errors",
	// GetHeadersCmd help.
	"getheaders--synopsis":     "Returns block headers starting with the first known block hash from the request",
	"getheaders-blocklocators": "JSON array of hex-encoded hashes of blocks.  Headers are returned starting from the first known hash in this list",
	"getheaders-hashstop":      "Block hash to stop including block headers for; if not found, all headers to the latest known block are returned.",
	"getheaders--result0":      "Serialized block headers of all located blocks, limited to some arbitrary maximum number of hashes (currently 2000, which matches the wire protocol headers message, but this is not guaranteed)",
	// GetInfoCmd help.
	"getinfo--synopsis": "Returns a JSON object containing various state info.",
	// GetMempoolInfoCmd help.
	"getmempoolinfo--synopsis": "Returns memory pool information",
	// GetMempoolInfoResult help.
	"getmempoolinforesult-bytes": "Size in bytes of the mempool",
	"getmempoolinforesult-size":  "Number of transactions in the mempool",
	// GetMiningInfoResult help.
	"getmininginforesult-blocks":             "Height of the latest best block",
	"getmininginforesult-currentblocksize":   "Size of the latest best block",
	"getmininginforesult-currentblockweight": "Weight of the latest best block",
	"getmininginforesult-currentblocktx":     "Number of transactions in the latest best block",
	"getmininginforesult-difficulty":         "Current target difficulty",
	"getmininginforesult-errors":             "Any current errors",
	"getmininginforesult-generate":           "Whether or not server is set to generate coins",
	"getmininginforesult-genproclimit":       "Number of processors to use for coin generation (-1 when disabled)",
	"getmininginforesult-hashespersec":       "Recent hashes per second performance measurement while generating coins",
	"getmininginforesult-networkhashps":      "Estimated network hashes per second for the most recent blocks",
	"getmininginforesult-pooledtx":           "Number of transactions in the memory pool",
	"getmininginforesult-testnet":            "Whether or not server is using testnet",
	// GetMiningInfoCmd help.
	"getmininginfo--synopsis": "Returns a JSON object containing mining-related information.",
	// GetNetworkHashPSCmd help.
	"getnetworkhashps--synopsis": "Returns the estimated network hashes per second for the block heights provided by the parameters.",
	"getnetworkhashps-blocks":    "The number of blocks, or -1 for blocks since last difficulty change",
	"getnetworkhashps-height":    "Perform estimate ending with this height or -1 for current best chain block height",
	"getnetworkhashps--result0":  "Estimated hashes per second",
	// GetNetTotalsCmd help.
	"getnettotals--synopsis": "Returns a JSON object containing network traffic statistics.",
	// GetNetTotalsResult help.
	"getnettotalsresult-totalbytesrecv": "Total bytes received",
	"getnettotalsresult-totalbytessent": "Total bytes sent",
	"getnettotalsresult-timemillis":     "Number of milliseconds since 1 Jan 1970 GMT",
	// GetPeerInfoResult help.
	"getpeerinforesult-id":             "A unique node ID",
	"getpeerinforesult-addr":           "The ip address and port of the peer",
	"getpeerinforesult-addrlocal":      "Local address",
	"getpeerinforesult-services":       "Services bitmask which represents the services supported by the peer",
	"getpeerinforesult-relaytxes":      "Peer has requested transactions be relayed to it",
	"getpeerinforesult-lastsend":       "Time the last message was received in seconds since 1 Jan 1970 GMT",
	"getpeerinforesult-lastrecv":       "Time the last message was sent in seconds since 1 Jan 1970 GMT",
	"getpeerinforesult-bytessent":      "Total bytes sent",
	"getpeerinforesult-bytesrecv":      "Total bytes received",
	"getpeerinforesult-conntime":       "Time the connection was made in seconds since 1 Jan 1970 GMT",
	"getpeerinforesult-timeoffset":     "The time offset of the peer",
	"getpeerinforesult-pingtime":       "Number of microseconds the last ping took",
	"getpeerinforesult-pingwait":       "Number of microseconds a queued ping has been waiting for a response",
	"getpeerinforesult-version":        "The protocol version of the peer",
	"getpeerinforesult-subver":         "The user agent of the peer",
	"getpeerinforesult-inbound":        "Whether or not the peer is an inbound connection",
	"getpeerinforesult-startingheight": "The latest block height the peer knew about when the connection was established",
	"getpeerinforesult-currentheight":  "The current height of the peer",
	"getpeerinforesult-banscore":       "The ban score",
	"getpeerinforesult-feefilter":      "The requested minimum fee a transaction must have to be announced to the peer",
	"getpeerinforesult-syncnode":       "Whether or not the peer is the sync peer",
	// GetPeerInfoCmd help.
	"getpeerinfo--synopsis": "Returns data about each connected network peer as an array of json objects.",
	// GetRawMempoolVerboseResult help.
	"getrawmempoolverboseresult-size":             "Transaction size in bytes",
	"getrawmempoolverboseresult-fee":              "Transaction fee in bitcoins",
	"getrawmempoolverboseresult-time":             "Local time transaction entered pool in seconds since 1 Jan 1970 GMT",
	"getrawmempoolverboseresult-height":           "Block height when transaction entered the pool",
	"getrawmempoolverboseresult-startingpriority": "Priority when transaction entered the pool",
	"getrawmempoolverboseresult-currentpriority":  "Current priority",
	"getrawmempoolverboseresult-depends":          "Unconfirmed transactions used as inputs for this transaction",
	"getrawmempoolverboseresult-vsize":            "The virtual size of a transaction",
	// GetRawMempoolCmd help.
	"getrawmempool--synopsis":   "Returns information about all of the transactions currently in the memory pool.",
	"getrawmempool-verbose":     "Returns JSON object when true or an array of transaction hashes when false",
	"getrawmempool--condition0": "verbose=false",
	"getrawmempool--condition1": "verbose=true",
	"getrawmempool--result0":    "Array of transaction hashes",
	// GetRawTransactionCmd help.
	"getrawtransaction--synopsis":   "Returns information about a transaction given its hash.",
	"getrawtransaction-txid":        "The hash of the transaction",
	"getrawtransaction-verbose":     "Specifies the transaction is returned as a JSON object instead of a hex-encoded string",
	"getrawtransaction--condition0": "verbose=false",
	"getrawtransaction--condition1": "verbose=true",
	"getrawtransaction--result0":    "Hex-encoded bytes of the serialized transaction",
	// GetTxOutResult help.
	"gettxoutresult-bestblock":     "The block hash that contains the transaction output",
	"gettxoutresult-confirmations": "The number of confirmations",
	"gettxoutresult-value":         "The transaction amount in DUO",
	"gettxoutresult-scriptPubKey":  "The public key script used to pay coins as a JSON object",
	"gettxoutresult-version":       "The transaction version",
	"gettxoutresult-coinbase":      "Whether or not the transaction is a coinbase",
	// GetTxOutCmd help.
	"gettxout--synopsis":      "Returns information about an unspent transaction output..",
	"gettxout-txid":           "The hash of the transaction",
	"gettxout-vout":           "The index of the output",
	"gettxout-includemempool": "Include the mempool when true",
	// HelpCmd help.
	"help--synopsis":   "Returns a list of all commands or help for a specified command.",
	"help-command":     "The command to retrieve help for",
	"help--condition0": "no command provided",
	"help--condition1": "command specified",
	"help--result0":    "List of commands",
	"help--result1":    "Help for specified command",
	// PingCmd help.
	"ping--synopsis": "Queues a ping to be sent to each connected peer.\n" +
		"Ping times are provided by getpeerinfo via the pingtime and pingwait fields.",
	// SearchRawTransactionsCmd help.
	"searchrawtransactions--synopsis": "Returns raw data for transactions involving the passed address.\n" +
		"Returned transactions are pulled from both the database, and transactions currently in the mempool.\n" +
		"Transactions pulled from the mempool will have the 'confirmations' field set to 0.\n" +
		"Usage of this RPC requires the optional --addrindex flag to be activated, otherwise all responses will simply return with an error stating the address index has not yet been built.\n" +
		"Similarly, until the address index has caught up with the current best height, all requests will return an error response in order to avoid serving stale data.",
	"searchrawtransactions-address":     "The Bitcoin address to search for",
	"searchrawtransactions-verbose":     "Specifies the transaction is returned as a JSON object instead of hex-encoded string",
	"searchrawtransactions--condition0": "verbose=0",
	"searchrawtransactions--condition1": "verbose=1",
	"searchrawtransactions-skip":        "The number of leading transactions to leave out of the final response",
	"searchrawtransactions-count":       "The maximum number of transactions to return",
	"searchrawtransactions-vinextra":    "Specify that extra data from previous output will be returned in vin",
	"searchrawtransactions-reverse":     "Specifies that the transactions should be returned in reverse chronological order",
	"searchrawtransactions-filteraddrs": "Address list.  Only inputs or outputs with matching address will be returned",
	"searchrawtransactions--result0":    "Hex-encoded serialized transaction",
	// SendRawTransactionCmd help.
	"sendrawtransaction--synopsis":     "Submits the serialized, hex-encoded transaction to the local peer and relays it to the network.",
	"sendrawtransaction-hextx":         "Serialized, hex-encoded signed transaction",
	"sendrawtransaction-allowhighfees": "Whether or not to allow insanely high fees (pod does not yet implement this parameter, so it has no effect)",
	"sendrawtransaction--result0":      "The hash of the transaction",
	// SetGenerateCmd help.
	"setgenerate--synopsis":    "Set the server to generate coins (mine) or not.",
	"setgenerate-generate":     "Use true to enable generation, false to disable it",
	"setgenerate-genproclimit": "The number of processors (cores) to limit generation to or -1 for default",
	// StopCmd help.
	"stop--synopsis": "Shutdown pod.",
	"stop--result0":  "The string 'pod stopping.'",
	// SubmitBlockOptions help.
	"submitblockoptions-workid": "This parameter is currently ignored",
	// SubmitBlockCmd help.
	"submitblock--synopsis":   "Attempts to submit a new serialized, hex-encoded block to the network.",
	"submitblock-hexblock":    "Serialized, hex-encoded block",
	"submitblock-options":     "This parameter is currently ignored",
	"submitblock--condition0": "Block successfully submitted",
	"submitblock--condition1": "Block rejected",
	"submitblock--result1":    "The reason the block was rejected",
	// ValidateAddressResult help.
	"validateaddresschainresult-isvalid": "Whether or not the address is valid",
	"validateaddresschainresult-address": "The bitcoin address (only when isvalid is true)",
	// ValidateAddressCmd help.
	"validateaddress--synopsis": "Verify an address is valid.",
	"validateaddress-address":   "Bitcoin address to validate",
	// VerifyChainCmd help.
	"verifychain--synopsis": "Verifies the block chain database.\n" +
		"The actual checks performed by the checklevel parameter are implementation specific.\n" +
		"For pod this is:\n" +
		"checklevel=0 - Look up each block and ensure it can be loaded from the database.\n" +
		"checklevel=1 - Perform basic context-free sanity checks on each block.",
	"verifychain-checklevel": "How thorough the block verification is",
	"verifychain-checkdepth": "The number of blocks to check",
	"verifychain--result0":   "Whether or not the chain verified",
	// VerifyMessageCmd help.
	"verifymessage--synopsis": "Verify a signed message.",
	"verifymessage-address":   "The bitcoin address to use for the signature",
	"verifymessage-signature": "The base-64 encoded signature provided by the signer",
	"verifymessage-message":   "The signed message",
	"verifymessage--result0":  "Whether or not the signature verified",
	// -------- Websocket-specific help --------
	// Session help.
	"session--synopsis":       "Return details regarding a websocket client's current connection session.",
	"sessionresult-sessionid": "The unique session ID for a client's websocket connection.",
	// NotifyBlocksCmd help.
	"notifyblocks--synopsis": "Request notifications for whenever a block is connected or disconnected from the main (best) chain.",
	// StopNotifyBlocksCmd help.
	"stopnotifyblocks--synopsis": "Cancel registered notifications for whenever a block is connected or disconnected from the main (best) chain.",
	// NotifyNewTransactionsCmd help.
	"notifynewtransactions--synopsis": "Send either a txaccepted or a txacceptedverbose notification when a new transaction is accepted into the mempool.",
	"notifynewtransactions-verbose":   "Specifies which type of notification to receive. If verbose is true, then the caller receives txacceptedverbose, otherwise the caller receives txaccepted",
	// StopNotifyNewTransactionsCmd help.
	"stopnotifynewtransactions--synopsis": "Stop sending either a txaccepted or a txacceptedverbose notification when a new transaction is accepted into the mempool.",
	// NotifyReceivedCmd help.
	"notifyreceived--synopsis": "Send a recvtx notification when a transaction added to mempool or appears in a newly-attached block contains a txout pkScript sending to any of the passed addresses.\n" +
		"Matching outpoints are automatically registered for redeemingtx notifications.",
	"notifyreceived-addresses": "List of address to receive notifications about",
	// StopNotifyReceivedCmd help.
	"stopnotifyreceived--synopsis": "Cancel registered receive notifications for each passed address.",
	"stopnotifyreceived-addresses": "List of address to cancel receive notifications for",
	// OutPoint help.
	"outpoint-hash":  "The hex-encoded bytes of the outpoint hash",
	"outpoint-index": "The index of the outpoint",
	// NotifySpentCmd help.
	"notifyspent--synopsis": "Send a redeemingtx notification when a transaction spending an outpoint appears in mempool (if relayed to this pod instance) and when such a transaction first appears in a newly-attached block.",
	"notifyspent-outpoints": "List of transaction outpoints to monitor.",
	// StopNotifySpentCmd help.
	"stopnotifyspent--synopsis": "Cancel registered spending notifications for each passed outpoint.",
	"stopnotifyspent-outpoints": "List of transaction outpoints to stop monitoring.",
	// LoadTxFilterCmd help.
	"loadtxfilter--synopsis": "Load, add to, or reload a websocket client's transaction filter for mempool transactions, new blocks and rescanblocks.",
	"loadtxfilter-reload":    "Load a new filter instead of adding data to an existing one",
	"loadtxfilter-addresses": "Array of addresses to add to the transaction filter",
	"loadtxfilter-outpoints": "Array of outpoints to add to the transaction filter",
	// Rescan help.
	"rescan--synopsis": "Rescan block chain for transactions to addresses.\n" +
		"When the endblock parameter is omitted, the rescan continues through the best block in the main chain.\n" +
		"Rescan results are sent as recvtx and redeemingtx notifications.\n" +
		"This call returns once the rescan completes.",
	"rescan-beginblock": "Hash of the first block to begin rescanning",
	"rescan-addresses":  "List of addresses to include in the rescan",
	"rescan-outpoints":  "List of transaction outpoints to include in the rescan",
	"rescan-endblock":   "Hash of final block to rescan",
	// RescanBlocks help.
	"rescanblocks--synopsis":   "Rescan blocks for transactions matching the loaded transaction filter.",
	"rescanblocks-blockhashes": "List of hashes to rescan.  Each next block must be a child of the previous.",
	"rescanblocks--result0":    "List of matching blocks.",
	// RescannedBlock help.
	"rescannedblock-hash":         "Hash of the matching block.",
	"rescannedblock-transactions": "List of matching transactions, serialized and hex-encoded.",
	// Uptime help.
	"uptime--synopsis": "Returns the total uptime of the server.",
	"uptime--result0":  "The number of seconds that the server has been running",
	// Version help.
	"version--synopsis":       "Returns the JSON-RPC API version (semver)",
	"version--result0--desc":  "Version objects keyed by the program or API name",
	"version--result0--key":   "Program or API name",
	"version--result0--value": "Object containing the semantic version",
	// VersionResult help.
	"versionresult-versionstring": "The JSON-RPC API version (semver)",
	"versionresult-major":         "The major component of the JSON-RPC API version",
	"versionresult-minor":         "The minor component of the JSON-RPC API version",
	"versionresult-patch":         "The patch component of the JSON-RPC API version",
	"versionresult-prerelease":    "Prerelease info about the current build",
	"versionresult-buildmetadata": "Metadata about the current build",
}
// rpcResultTypes specifies the result types that each RPC command can return. This information is used to generate the help.  Each result type must be a pointer to the type (or nil to indicate no return value).
var rpcResultTypes = map[string][]interface{}{
	"addnode":               nil,
	"createrawtransaction":  {(*string)(nil)},
	"debuglevel":            {(*string)(nil), (*string)(nil)},
	"decoderawtransaction":  {(*json.TxRawDecodeResult)(nil)},
	"decodescript":          {(*json.DecodeScriptResult)(nil)},
	"estimatefee":           {(*float64)(nil)},
	"generate":              {(*[]string)(nil)},
	"getaddednodeinfo":      {(*[]string)(nil), (*[]json.GetAddedNodeInfoResult)(nil)},
	"getbestblock":          {(*json.GetBestBlockResult)(nil)},
	"getbestblockhash":      {(*string)(nil)},
	"getblock":              {(*string)(nil), (*json.GetBlockVerboseResult)(nil)},
	"getblockcount":         {(*int64)(nil)},
	"getblockhash":          {(*string)(nil)},
	"getblockheader":        {(*string)(nil), (*json.GetBlockHeaderVerboseResult)(nil)},
	"getblocktemplate":      {(*json.GetBlockTemplateResult)(nil), (*string)(nil), nil},
	"getblockchaininfo":     {(*json.GetBlockChainInfoResult)(nil)},
	"getcfilter":            {(*string)(nil)},
	"getcfilterheader":      {(*string)(nil)},
	"getconnectioncount":    {(*int32)(nil)},
	"getcurrentnet":         {(*uint32)(nil)},
	"getdifficulty":         {(*float64)(nil)},
	"getgenerate":           {(*bool)(nil)},
	"gethashespersec":       {(*float64)(nil)},
	"getheaders":            {(*[]string)(nil)},
	"getinfo":               {(*json.InfoChainResult)(nil)},
	"getmempoolinfo":        {(*json.GetMempoolInfoResult)(nil)},
	"getmininginfo":         {(*json.GetMiningInfoResult)(nil)},
	"getnettotals":          {(*json.GetNetTotalsResult)(nil)},
	"getnetworkhashps":      {(*int64)(nil)},
	"getpeerinfo":           {(*[]json.GetPeerInfoResult)(nil)},
	"getrawmempool":         {(*[]string)(nil), (*json.GetRawMempoolVerboseResult)(nil)},
	"getrawtransaction":     {(*string)(nil), (*json.TxRawResult)(nil)},
	"gettxout":              {(*json.GetTxOutResult)(nil)},
	"node":                  nil,
	"help":                  {(*string)(nil), (*string)(nil)},
	"ping":                  nil,
	"searchrawtransactions": {(*string)(nil), (*[]json.SearchRawTransactionsResult)(nil)},
	"sendrawtransaction":    {(*string)(nil)},
	"setgenerate":           nil,
	"stop":                  {(*string)(nil)},
	"submitblock":           {nil, (*string)(nil)},
	"uptime":                {(*int64)(nil)},
	"validateaddress":       {(*json.ValidateAddressChainResult)(nil)},
	"verifychain":           {(*bool)(nil)},
	"verifymessage":         {(*bool)(nil)},
	"version":               {(*map[string]json.VersionResult)(nil)},
	// Websocket commands.
	"loadtxfilter":              nil,
	"session":                   {(*json.SessionResult)(nil)},
	"notifyblocks":              nil,
	"stopnotifyblocks":          nil,
	"notifynewtransactions":     nil,
	"stopnotifynewtransactions": nil,
	"notifyreceived":            nil,
	"stopnotifyreceived":        nil,
	"notifyspent":               nil,
	"stopnotifyspent":           nil,
	"rescan":                    nil,
	"rescanblocks":              {(*[]json.RescannedBlock)(nil)},
}
// helpCacher provides a concurrent safe type that provides help and usage for the RPC server commands and caches the results for future calls.
type helpCacher struct {
	sync.Mutex
	usage      string
	methodHelp map[string]string
}
// rpcMethodHelp returns an RPC help string for the provided method. This function is safe for concurrent access.
func (c *helpCacher) rpcMethodHelp(method string) (string, error) {
	c.Lock()
	defer c.Unlock()
	// Return the cached method help if it exists.
	if help, exists := c.methodHelp[method]; exists {
		return help, nil
	}
	// Look up the result types for the method.
	resultTypes, ok := rpcResultTypes[method]
	if !ok {
		return "", errors.New("no result types specified for method " +
			method)
	}
	// Generate, cache, and return the help.
	help, err := json.GenerateHelp(method, helpDescsEnUS, resultTypes...)
	if err != nil {
		return "", err
	}
	c.methodHelp[method] = help
	return help, nil
}
// rpcUsage returns one-line usage for all support RPC commands. This function is safe for concurrent access.
func (c *helpCacher) rpcUsage(includeWebsockets bool) (string, error) {
	c.Lock()
	defer c.Unlock()
	// Return the cached usage if it is available.
	if c.usage != "" {
		return c.usage, nil
	}
	// Generate a list of one-line usage for every command.
	usageTexts := make([]string, 0, len(rpcHandlers))
	for k := range rpcHandlers {
		usage, err := json.MethodUsageText(k)
		if err != nil {
			return "", err
		}
		usageTexts = append(usageTexts, usage)
	}
	// Include websockets commands if requested.
	if includeWebsockets {
		for k := range wsHandlers {
			usage, err := json.MethodUsageText(k)
			if err != nil {
				return "", err
			}
			usageTexts = append(usageTexts, usage)
		}
	}
	sort.Sort(sort.StringSlice(usageTexts))
	c.usage = strings.Join(usageTexts, "\n")
	return c.usage, nil
}
// newHelpCacher returns a new instance of a help cacher which provides help and usage for the RPC server commands and caches the results for future calls.
func newHelpCacher() *helpCacher {
	return &helpCacher{
		methodHelp: make(map[string]string),
	}
}
package node
import (
	"bytes"
	"container/list"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	js "encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sync"
	"time"
	blockchain "git.parallelcoin.io/pod/pkg/chain"
	"git.parallelcoin.io/pod/pkg/chaincfg"
	"git.parallelcoin.io/pod/pkg/chaincfg/chainhash"
	cl "git.parallelcoin.io/pod/pkg/clog"
	database "git.parallelcoin.io/pod/pkg/db"
	"git.parallelcoin.io/pod/pkg/json"
	"git.parallelcoin.io/pod/pkg/txscript"
	"git.parallelcoin.io/pod/pkg/util"
	"git.parallelcoin.io/pod/pkg/wire"
	"github.com/btcsuite/websocket"
	"golang.org/x/crypto/ripemd160"
)
const (
	// websocketSendBufferSize is the number of elements the send channel can queue before blocking.  Note that this only applies to requests handled directly in the websocket client input handler or the async handler since notifications have their own queuing mechanism independent of the send channel buffer.
	websocketSendBufferSize = 50
)
type semaphore chan struct{}
func makeSemaphore(n int) semaphore {
	return make(chan struct{}, n)
}
func (s semaphore) acquire() { s <- struct{}{} }
func (s semaphore) release() { <-s }
// timeZeroVal is simply the zero value for a time.Time and is used to avoid creating multiple instances.
var timeZeroVal time.Time
// wsCommandHandler describes a callback function used to handle a specific command.
type wsCommandHandler func(*wsClient, interface{}) (interface{}, error)
// wsHandlers maps RPC command strings to appropriate websocket handler functions.  This is set by init because help references wsHandlers and thus causes a dependency loop.
var wsHandlers map[string]wsCommandHandler
var wsHandlersBeforeInit = map[string]wsCommandHandler{
	"loadtxfilter":              handleLoadTxFilter,
	"help":                      handleWebsocketHelp,
	"notifyblocks":              handleNotifyBlocks,
	"notifynewtransactions":     handleNotifyNewTransactions,
	"notifyreceived":            handleNotifyReceived,
	"notifyspent":               handleNotifySpent,
	"session":                   handleSession,
	"stopnotifyblocks":          handleStopNotifyBlocks,
	"stopnotifynewtransactions": handleStopNotifyNewTransactions,
	"stopnotifyspent":           handleStopNotifySpent,
	"stopnotifyreceived":        handleStopNotifyReceived,
	"rescan":                    handleRescan,
	"rescanblocks":              handleRescanBlocks,
}
// WebsocketHandler handles a new websocket client by creating a new wsClient, starting it, and blocking until the connection closes.  Since it blocks, it must be run in a separate goroutine.  It should be invoked from the websocket server handler which runs each new connection in a new goroutine thereby satisfying the requirement.
func (s *rpcServer) WebsocketHandler(conn *websocket.Conn, remoteAddr string,
	authenticated bool, isAdmin bool) {
	// Clear the read deadline that was set before the websocket hijacked the connection.
	conn.SetReadDeadline(timeZeroVal)
	// Limit max number of websocket clients.
	log <- cl.Info{"new websocket client", remoteAddr}
	if s.ntfnMgr.NumClients()+1 > cfg.RPCMaxWebsockets {
		log <- cl.Infof{
			"max websocket clients exceeded [%d] - disconnecting client %s",
			cfg.RPCMaxWebsockets,
			remoteAddr,
		}
		conn.Close()
		return
	}
	// Create a new websocket client to handle the new websocket connection and wait for it to shutdown.  Once it has shutdown (and hence disconnected), remove it and any notifications it registered for.
	client, err := newWebsocketClient(s, conn, remoteAddr, authenticated, isAdmin)
	if err != nil {
		log <- cl.Errorf{
			"failed to serve client %s: %v", remoteAddr, err,
		}
		conn.Close()
		return
	}
	s.ntfnMgr.AddClient(client)
	client.Start()
	client.WaitForShutdown()
	s.ntfnMgr.RemoveClient(client)
	log <- cl.Infof{
		"disconnected websocket client %s", remoteAddr,
	}
}
// wsNotificationManager is a connection and notification manager used for websockets.  It allows websocket clients to register for notifications they are interested in.  When an event happens elsewhere in the code such as transactions being added to the memory pool or block connects/disconnects, the notification manager is provided with the relevant details needed to figure out which websocket clients need to be notified based on what they have registered for and notifies them accordingly.  It is also used to keep track of all connected websocket clients.
type wsNotificationManager struct {
	// server is the RPC server the notification manager is associated with.
	server *rpcServer
	// queueNotification queues a notification for handling.
	queueNotification chan interface{}
	// notificationMsgs feeds notificationHandler with notifications and client (un)registeration requests from a queue as well as registeration and unregisteration requests from clients.
	notificationMsgs chan interface{}
	// Access channel for current number of connected clients.
	numClients chan int
	// Shutdown handling
	wg   sync.WaitGroup
	quit chan struct{}
}
// queueHandler manages a queue of empty interfaces, reading from in and sending the oldest unsent to out.  This handler stops when either of the in or quit channels are closed, and closes out before returning, without waiting to send any variables still remaining in the queue.
func queueHandler(in <-chan interface{}, out chan<- interface{}, quit <-chan struct{}) {
	var q []interface{}
	var dequeue chan<- interface{}
	skipQueue := out
	var next interface{}
out:
	for {
		select {
		case n, ok := <-in:
			// fmt.Println("chan:n, ok := <-in")
			if !ok {
				// Sender closed input channel.
				break out
			}
			// Either send to out immediately if skipQueue is non-nil (queue is empty) and reader is ready, or append to the queue and send later.
			select {
			case skipQueue <- n:
				// fmt.Println("chan:skipQueue <- n")
			default:
				q = append(q, n)
				dequeue = out
				skipQueue = nil
				next = q[0]
			}
		case dequeue <- next:
			// fmt.Println("chan:dequeue <- next")
			copy(q, q[1:])
			q[len(q)-1] = nil // avoid leak
			q = q[:len(q)-1]
			if len(q) == 0 {
				dequeue = nil
				skipQueue = out
			} else {
				next = q[0]
			}
		case <-quit:
			// fmt.Println("chan:<-quit")
			break out
		}
	}
	close(out)
}
// queueHandler maintains a queue of notifications and notification handler control messages.
func (m *wsNotificationManager) queueHandler() {
	queueHandler(m.queueNotification, m.notificationMsgs, m.quit)
	m.wg.Done()
}
// NotifyBlockConnected passes a block newly-connected to the best chain to the notification manager for block and transaction notification processing.
func (m *wsNotificationManager) NotifyBlockConnected(block *util.Block) {
	// As NotifyBlockConnected will be called by the block manager and the RPC server may no longer be running, use a select statement to unblock enqueuing the notification once the RPC server has begun shutting down.
	select {
	case m.queueNotification <- (*notificationBlockConnected)(block):
		// fmt.Println("chan:m.queueNotification <- (*notificationBlockConnected)(block)")
	case <-m.quit:
	}
}
// NotifyBlockDisconnected passes a block disconnected from the best chain to the notification manager for block notification processing.
func (m *wsNotificationManager) NotifyBlockDisconnected(block *util.Block) {
	// As NotifyBlockDisconnected will be called by the block manager and the RPC server may no longer be running, use a select statement to unblock enqueuing the notification once the RPC server has begun shutting down.
	select {
	case m.queueNotification <- (*notificationBlockDisconnected)(block):
		// fmt.Println("chan:m.queueNotification <- (*notificationBlockDisconnected)(block)")
	case <-m.quit:
	}
}
// NotifyMempoolTx passes a transaction accepted by mempool to the notification manager for transaction notification processing.  If isNew is true, the tx is is a new transaction, rather than one added to the mempool during a reorg.
func (m *wsNotificationManager) NotifyMempoolTx(tx *util.Tx, isNew bool) {
	n := &notificationTxAcceptedByMempool{
		isNew: isNew,
		tx:    tx,
	}
	// As NotifyMempoolTx will be called by mempool and the RPC server may no longer be running, use a select statement to unblock enqueuing the notification once the RPC server has begun shutting down.
	select {
	case m.queueNotification <- n:
		// fmt.Println("chan:m.queueNotification <- n")
	case <-m.quit:
	}
}
// wsClientFilter tracks relevant addresses for each websocket client for the `rescanblocks` extension. It is modified by the `loadtxfilter` command. NOTE: This extension was ported from github.com/decred/dcrd
type wsClientFilter struct {
	mu sync.Mutex
	// Implemented fast paths for address lookup.
	pubKeyHashes        map[[ripemd160.Size]byte]struct{}
	scriptHashes        map[[ripemd160.Size]byte]struct{}
	compressedPubKeys   map[[33]byte]struct{}
	uncompressedPubKeys map[[65]byte]struct{}
	// A fallback address lookup map in case a fast path doesn't exist. Only exists for completeness.  If using this shows up in a profile, there's a good chance a fast path should be added.
	otherAddresses map[string]struct{}
	// Outpoints of unspent outputs.
	unspent map[wire.OutPoint]struct{}
}
// newWSClientFilter creates a new, empty wsClientFilter struct to be used for a websocket client. NOTE: This extension was ported from github.com/decred/dcrd
func newWSClientFilter(addresses []string, unspentOutPoints []wire.OutPoint, params *chaincfg.Params) *wsClientFilter {
	filter := &wsClientFilter{
		pubKeyHashes:        map[[ripemd160.Size]byte]struct{}{},
		scriptHashes:        map[[ripemd160.Size]byte]struct{}{},
		compressedPubKeys:   map[[33]byte]struct{}{},
		uncompressedPubKeys: map[[65]byte]struct{}{},
		otherAddresses:      map[string]struct{}{},
		unspent:             make(map[wire.OutPoint]struct{}, len(unspentOutPoints)),
	}
	for _, s := range addresses {
		filter.addAddressStr(s, params)
	}
	for i := range unspentOutPoints {
		filter.addUnspentOutPoint(&unspentOutPoints[i])
	}
	return filter
}
// addAddress adds an address to a wsClientFilter, treating it correctly based on the type of address passed as an argument. NOTE: This extension was ported from github.com/decred/dcrd
func (f *wsClientFilter) addAddress(a util.Address) {
	switch a := a.(type) {
	case *util.AddressPubKeyHash:
		f.pubKeyHashes[*a.Hash160()] = struct{}{}
		return
	case *util.AddressScriptHash:
		f.scriptHashes[*a.Hash160()] = struct{}{}
		return
	case *util.AddressPubKey:
		serializedPubKey := a.ScriptAddress()
		switch len(serializedPubKey) {
		case 33: // compressed
			var compressedPubKey [33]byte
			copy(compressedPubKey[:], serializedPubKey)
			f.compressedPubKeys[compressedPubKey] = struct{}{}
			return
		case 65: // uncompressed
			var uncompressedPubKey [65]byte
			copy(uncompressedPubKey[:], serializedPubKey)
			f.uncompressedPubKeys[uncompressedPubKey] = struct{}{}
			return
		}
	}
	f.otherAddresses[a.EncodeAddress()] = struct{}{}
}
// addAddressStr parses an address from a string and then adds it to the wsClientFilter using addAddress. NOTE: This extension was ported from github.com/decred/dcrd
func (f *wsClientFilter) addAddressStr(s string, params *chaincfg.Params) {
	// If address can't be decoded, no point in saving it since it should also impossible to create the address from an inspected transaction output script.
	a, err := util.DecodeAddress(s, params)
	if err != nil {
		return
	}
	f.addAddress(a)
}
// existsAddress returns true if the passed address has been added to the wsClientFilter. NOTE: This extension was ported from github.com/decred/dcrd
func (f *wsClientFilter) existsAddress(a util.Address) bool {
	switch a := a.(type) {
	case *util.AddressPubKeyHash:
		_, ok := f.pubKeyHashes[*a.Hash160()]
		return ok
	case *util.AddressScriptHash:
		_, ok := f.scriptHashes[*a.Hash160()]
		return ok
	case *util.AddressPubKey:
		serializedPubKey := a.ScriptAddress()
		switch len(serializedPubKey) {
		case 33: // compressed
			var compressedPubKey [33]byte
			copy(compressedPubKey[:], serializedPubKey)
			_, ok := f.compressedPubKeys[compressedPubKey]
			if !ok {
				_, ok = f.pubKeyHashes[*a.AddressPubKeyHash().Hash160()]
			}
			return ok
		case 65: // uncompressed
			var uncompressedPubKey [65]byte
			copy(uncompressedPubKey[:], serializedPubKey)
			_, ok := f.uncompressedPubKeys[uncompressedPubKey]
			if !ok {
				_, ok = f.pubKeyHashes[*a.AddressPubKeyHash().Hash160()]
			}
			return ok
		}
	}
	_, ok := f.otherAddresses[a.EncodeAddress()]
	return ok
}
// removeAddress removes the passed address, if it exists, from the wsClientFilter. NOTE: This extension was ported from github.com/decred/dcrd
func (f *wsClientFilter) removeAddress(a util.Address) {
	switch a := a.(type) {
	case *util.AddressPubKeyHash:
		delete(f.pubKeyHashes, *a.Hash160())
		return
	case *util.AddressScriptHash:
		delete(f.scriptHashes, *a.Hash160())
		return
	case *util.AddressPubKey:
		serializedPubKey := a.ScriptAddress()
		switch len(serializedPubKey) {
		case 33: // compressed
			var compressedPubKey [33]byte
			copy(compressedPubKey[:], serializedPubKey)
			delete(f.compressedPubKeys, compressedPubKey)
			return
		case 65: // uncompressed
			var uncompressedPubKey [65]byte
			copy(uncompressedPubKey[:], serializedPubKey)
			delete(f.uncompressedPubKeys, uncompressedPubKey)
			return
		}
	}
	delete(f.otherAddresses, a.EncodeAddress())
}
// removeAddressStr parses an address from a string and then removes it from the wsClientFilter using removeAddress. NOTE: This extension was ported from github.com/decred/dcrd
func (f *wsClientFilter) removeAddressStr(s string, params *chaincfg.Params) {
	a, err := util.DecodeAddress(s, params)
	if err == nil {
		f.removeAddress(a)
	} else {
		delete(f.otherAddresses, s)
	}
}
// addUnspentOutPoint adds an outpoint to the wsClientFilter. NOTE: This extension was ported from github.com/decred/dcrd
func (f *wsClientFilter) addUnspentOutPoint(op *wire.OutPoint) {
	f.unspent[*op] = struct{}{}
}
// existsUnspentOutPoint returns true if the passed outpoint has been added to the wsClientFilter. NOTE: This extension was ported from github.com/decred/dcrd
func (f *wsClientFilter) existsUnspentOutPoint(op *wire.OutPoint) bool {
	_, ok := f.unspent[*op]
	return ok
}
// removeUnspentOutPoint removes the passed outpoint, if it exists, from the wsClientFilter. NOTE: This extension was ported from github.com/decred/dcrd
func (f *wsClientFilter) removeUnspentOutPoint(op *wire.OutPoint) {
	delete(f.unspent, *op)
}
// Notification types
type notificationBlockConnected util.Block
type notificationBlockDisconnected util.Block
type notificationTxAcceptedByMempool struct {
	isNew bool
	tx    *util.Tx
}
// Notification control requests
type notificationRegisterClient wsClient
type notificationUnregisterClient wsClient
type notificationRegisterBlocks wsClient
type notificationUnregisterBlocks wsClient
type notificationRegisterNewMempoolTxs wsClient
type notificationUnregisterNewMempoolTxs wsClient
type notificationRegisterSpent struct {
	wsc *wsClient
	ops []*wire.OutPoint
}
type notificationUnregisterSpent struct {
	wsc *wsClient
	op  *wire.OutPoint
}
type notificationRegisterAddr struct {
	wsc   *wsClient
	addrs []string
}
type notificationUnregisterAddr struct {
	wsc  *wsClient
	addr string
}
// notificationHandler reads notifications and control messages from the queue handler and processes one at a time.
func (m *wsNotificationManager) notificationHandler() {
	// clients is a map of all currently connected websocket clients.
	clients := make(map[chan struct{}]*wsClient)
	// Maps used to hold lists of websocket clients to be notified on certain events.  Each websocket client also keeps maps for the events which have multiple triggers to make removal from these lists on connection close less horrendously. Where possible, the quit channel is used as the unique id for a client since it is quite a bit more efficient than using the entire struct.
	blockNotifications := make(map[chan struct{}]*wsClient)
	txNotifications := make(map[chan struct{}]*wsClient)
	watchedOutPoints := make(map[wire.OutPoint]map[chan struct{}]*wsClient)
	watchedAddrs := make(map[string]map[chan struct{}]*wsClient)
out:
	for {
		select {
		case n, ok := <-m.notificationMsgs:
			// fmt.Println("chan:n, ok := <-m.notificationMsgs")
			if !ok {
				// queueHandler quit.
				break out
			}
			switch n := n.(type) {
			case *notificationBlockConnected:
				block := (*util.Block)(n)
				// Skip iterating through all txs if no tx notification requests exist.
				if len(watchedOutPoints) != 0 || len(watchedAddrs) != 0 {
					for _, tx := range block.Transactions() {
						m.notifyForTx(watchedOutPoints,
							watchedAddrs, tx, block)
					}
				}
				if len(blockNotifications) != 0 {
					m.notifyBlockConnected(blockNotifications,
						block)
					m.notifyFilteredBlockConnected(blockNotifications,
						block)
				}
			case *notificationBlockDisconnected:
				block := (*util.Block)(n)
				if len(blockNotifications) != 0 {
					m.notifyBlockDisconnected(blockNotifications,
						block)
					m.notifyFilteredBlockDisconnected(blockNotifications,
						block)
				}
			case *notificationTxAcceptedByMempool:
				if n.isNew && len(txNotifications) != 0 {
					m.notifyForNewTx(txNotifications, n.tx)
				}
				m.notifyForTx(watchedOutPoints, watchedAddrs, n.tx, nil)
				m.notifyRelevantTxAccepted(n.tx, clients)
			case *notificationRegisterBlocks:
				wsc := (*wsClient)(n)
				blockNotifications[wsc.quit] = wsc
			case *notificationUnregisterBlocks:
				wsc := (*wsClient)(n)
				delete(blockNotifications, wsc.quit)
			case *notificationRegisterClient:
				wsc := (*wsClient)(n)
				clients[wsc.quit] = wsc
			case *notificationUnregisterClient:
				wsc := (*wsClient)(n)
				// Remove any requests made by the client as well as the client itself.
				delete(blockNotifications, wsc.quit)
				delete(txNotifications, wsc.quit)
				for k := range wsc.spentRequests {
					op := k
					m.removeSpentRequest(watchedOutPoints, wsc, &op)
				}
				for addr := range wsc.addrRequests {
					m.removeAddrRequest(watchedAddrs, wsc, addr)
				}
				delete(clients, wsc.quit)
			case *notificationRegisterSpent:
				m.addSpentRequests(watchedOutPoints, n.wsc, n.ops)
			case *notificationUnregisterSpent:
				m.removeSpentRequest(watchedOutPoints, n.wsc, n.op)
			case *notificationRegisterAddr:
				m.addAddrRequests(watchedAddrs, n.wsc, n.addrs)
			case *notificationUnregisterAddr:
				m.removeAddrRequest(watchedAddrs, n.wsc, n.addr)
			case *notificationRegisterNewMempoolTxs:
				wsc := (*wsClient)(n)
				txNotifications[wsc.quit] = wsc
			case *notificationUnregisterNewMempoolTxs:
				wsc := (*wsClient)(n)
				delete(txNotifications, wsc.quit)
			default:
				log <- cl.Wrn("unhandled notification type")
			}
		case m.numClients <- len(clients):
			// fmt.Println("chan:m.numClients <- len(clients)")
		case <-m.quit:
			// fmt.Println("chan:<-m.quit")
			// RPC server shutting down.
			break out
		}
	}
	for _, c := range clients {
		c.Disconnect()
	}
	m.wg.Done()
}
// NumClients returns the number of clients actively being served.
func (m *wsNotificationManager) NumClients() (n int) {
	select {
	case n = <-m.numClients:
		// fmt.Println("chan:n = <-m.numClients")
	case <-m.quit: // Use default n (0) if server has shut down.
		// fmt.Println("chan:<-m.quit:")
	}
	return
}
// RegisterBlockUpdates requests block update notifications to the passed websocket client.
func (m *wsNotificationManager) RegisterBlockUpdates(wsc *wsClient) {
	m.queueNotification <- (*notificationRegisterBlocks)(wsc)
}
// UnregisterBlockUpdates removes block update notifications for the passed websocket client.
func (m *wsNotificationManager) UnregisterBlockUpdates(wsc *wsClient) {
	m.queueNotification <- (*notificationUnregisterBlocks)(wsc)
}
// subscribedClients returns the set of all websocket client quit channels that are registered to receive notifications regarding tx, either due to tx spending a watched output or outputting to a watched address.  Matching client's filters are updated based on this transaction's outputs and output addresses that may be relevant for a client.
func (m *wsNotificationManager) subscribedClients(tx *util.Tx,
	clients map[chan struct{}]*wsClient) map[chan struct{}]struct{} {
	// Use a map of client quit channels as keys to prevent duplicates when multiple inputs and/or outputs are relevant to the client.
	subscribed := make(map[chan struct{}]struct{})
	msgTx := tx.MsgTx()
	for _, input := range msgTx.TxIn {
		for quitChan, wsc := range clients {
			wsc.Lock()
			filter := wsc.filterData
			wsc.Unlock()
			if filter == nil {
				continue
			}
			filter.mu.Lock()
			if filter.existsUnspentOutPoint(&input.PreviousOutPoint) {
				subscribed[quitChan] = struct{}{}
			}
			filter.mu.Unlock()
		}
	}
	for i, output := range msgTx.TxOut {
		_, addrs, _, err := txscript.ExtractPkScriptAddrs(
			output.PkScript, m.server.cfg.ChainParams)
		if err != nil {
			// Clients are not able to subscribe to nonstandard or non-address outputs.
			continue
		}
		for quitChan, wsc := range clients {
			wsc.Lock()
			filter := wsc.filterData
			wsc.Unlock()
			if filter == nil {
				continue
			}
			filter.mu.Lock()
			for _, a := range addrs {
				if filter.existsAddress(a) {
					subscribed[quitChan] = struct{}{}
					op := wire.OutPoint{
						Hash:  *tx.Hash(),
						Index: uint32(i),
					}
					filter.addUnspentOutPoint(&op)
				}
			}
			filter.mu.Unlock()
		}
	}
	return subscribed
}
// notifyBlockConnected notifies websocket clients that have registered for block updates when a block is connected to the main chain.
func (*wsNotificationManager) notifyBlockConnected(clients map[chan struct{}]*wsClient,
	block *util.Block) {
	// Notify interested websocket clients about the connected block.
	ntfn := json.NewBlockConnectedNtfn(block.Hash().String(), block.Height(),
		block.MsgBlock().Header.Timestamp.Unix())
	marshalledJSON, err := json.MarshalCmd(nil, ntfn)
	if err != nil {
		log <- cl.Error{"failed to marshal block connected notification:", err}
		return
	}
	for _, wsc := range clients {
		wsc.QueueNotification(marshalledJSON)
	}
}
// notifyBlockDisconnected notifies websocket clients that have registered for block updates when a block is disconnected from the main chain (due to a reorganize).
func (*wsNotificationManager) notifyBlockDisconnected(clients map[chan struct{}]*wsClient, block *util.Block) {
	// Skip notification creation if no clients have requested block connected/disconnected notifications.
	if len(clients) == 0 {
		return
	}
	// Notify interested websocket clients about the disconnected block.
	ntfn := json.NewBlockDisconnectedNtfn(block.Hash().String(),
		block.Height(), block.MsgBlock().Header.Timestamp.Unix())
	marshalledJSON, err := json.MarshalCmd(nil, ntfn)
	if err != nil {
		log <- cl.Error{"failed to marshal block disconnected notification:", err}
		return
	}
	for _, wsc := range clients {
		wsc.QueueNotification(marshalledJSON)
	}
}
// notifyFilteredBlockConnected notifies websocket clients that have registered for block updates when a block is connected to the main chain.
func (m *wsNotificationManager) notifyFilteredBlockConnected(clients map[chan struct{}]*wsClient,
	block *util.Block) {
	// Create the common portion of the notification that is the same for every client.
	var w bytes.Buffer
	err := block.MsgBlock().Header.Serialize(&w)
	if err != nil {
		log <- cl.Error{
			"failed to serialize header for filtered block connected notification:", err,
		}
		return
	}
	ntfn := json.NewFilteredBlockConnectedNtfn(block.Height(),
		hex.EncodeToString(w.Bytes()), nil)
	// Search for relevant transactions for each client and save them serialized in hex encoding for the notification.
	subscribedTxs := make(map[chan struct{}][]string)
	for _, tx := range block.Transactions() {
		var txHex string
		for quitChan := range m.subscribedClients(tx, clients) {
			if txHex == "" {
				txHex = txHexString(tx.MsgTx())
			}
			subscribedTxs[quitChan] = append(subscribedTxs[quitChan], txHex)
		}
	}
	for quitChan, wsc := range clients {
		// Add all discovered transactions for this client. For clients that have no new-style filter, add the empty string slice.
		ntfn.SubscribedTxs = subscribedTxs[quitChan]
		// Marshal and queue notification.
		marshalledJSON, err := json.MarshalCmd(nil, ntfn)
		if err != nil {
			log <- cl.Errorf{
				"failed to marshal filtered block connected notification:", err,
			}
			return
		}
		wsc.QueueNotification(marshalledJSON)
	}
}
// notifyFilteredBlockDisconnected notifies websocket clients that have registered for block updates when a block is disconnected from the main chain (due to a reorganize).
func (*wsNotificationManager) notifyFilteredBlockDisconnected(clients map[chan struct{}]*wsClient,
	block *util.Block) {
	// Skip notification creation if no clients have requested block connected/disconnected notifications.
	if len(clients) == 0 {
		return
	}
	// Notify interested websocket clients about the disconnected block.
	var w bytes.Buffer
	err := block.MsgBlock().Header.Serialize(&w)
	if err != nil {
		log <- cl.Error{
			"failed to serialize header for filtered block disconnected notification:", err,
		}
		return
	}
	ntfn := json.NewFilteredBlockDisconnectedNtfn(block.Height(),
		hex.EncodeToString(w.Bytes()))
	marshalledJSON, err := json.MarshalCmd(nil, ntfn)
	if err != nil {
		log <- cl.Error{
			"failed to marshal filtered block disconnected notification:", err,
		}
		return
	}
	for _, wsc := range clients {
		wsc.QueueNotification(marshalledJSON)
	}
}
// RegisterNewMempoolTxsUpdates requests notifications to the passed websocket client when new transactions are added to the memory pool.
func (m *wsNotificationManager) RegisterNewMempoolTxsUpdates(wsc *wsClient) {
	m.queueNotification <- (*notificationRegisterNewMempoolTxs)(wsc)
}
// UnregisterNewMempoolTxsUpdates removes notifications to the passed websocket client when new transaction are added to the memory pool.
func (m *wsNotificationManager) UnregisterNewMempoolTxsUpdates(wsc *wsClient) {
	m.queueNotification <- (*notificationUnregisterNewMempoolTxs)(wsc)
}
// notifyForNewTx notifies websocket clients that have registered for updates when a new transaction is added to the memory pool.
func (m *wsNotificationManager) notifyForNewTx(clients map[chan struct{}]*wsClient, tx *util.Tx) {
	txHashStr := tx.Hash().String()
	mtx := tx.MsgTx()
	var amount int64
	for _, txOut := range mtx.TxOut {
		amount += txOut.Value
	}
	ntfn := json.NewTxAcceptedNtfn(txHashStr, util.Amount(amount).ToDUO())
	marshalledJSON, err := json.MarshalCmd(nil, ntfn)
	if err != nil {
		log <- cl.Error{"failed to marshal tx notification:", err}
		return
	}
	var verboseNtfn *json.TxAcceptedVerboseNtfn
	var marshalledJSONVerbose []byte
	for _, wsc := range clients {
		if wsc.verboseTxUpdates {
			if marshalledJSONVerbose != nil {
				wsc.QueueNotification(marshalledJSONVerbose)
				continue
			}
			net := m.server.cfg.ChainParams
			rawTx, err := createTxRawResult(net, mtx, txHashStr, nil,
				"", 0, 0)
			if err != nil {
				return
			}
			verboseNtfn = json.NewTxAcceptedVerboseNtfn(*rawTx)
			marshalledJSONVerbose, err = json.MarshalCmd(nil,
				verboseNtfn)
			if err != nil {
				log <- cl.Error{"failed to marshal verbose tx notification:", err}
				return
			}
			wsc.QueueNotification(marshalledJSONVerbose)
		} else {
			wsc.QueueNotification(marshalledJSON)
		}
	}
}
// RegisterSpentRequests requests a notification when each of the passed outpoints is confirmed spent (contained in a block connected to the main chain) for the passed websocket client.  The request is automatically removed once the notification has been sent.
func (m *wsNotificationManager) RegisterSpentRequests(wsc *wsClient, ops []*wire.OutPoint) {
	m.queueNotification <- &notificationRegisterSpent{
		wsc: wsc,
		ops: ops,
	}
}
// addSpentRequests modifies a map of watched outpoints to sets of websocket clients to add a new request watch all of the outpoints in ops and create and send a notification when spent to the websocket client wsc.
func (m *wsNotificationManager) addSpentRequests(opMap map[wire.OutPoint]map[chan struct{}]*wsClient,
	wsc *wsClient, ops []*wire.OutPoint) {
	for _, op := range ops {
		// Track the request in the client as well so it can be quickly be removed on disconnect.
		wsc.spentRequests[*op] = struct{}{}
		// Add the client to the list to notify when the outpoint is seen. Create the list as needed.
		cmap, ok := opMap[*op]
		if !ok {
			cmap = make(map[chan struct{}]*wsClient)
			opMap[*op] = cmap
		}
		cmap[wsc.quit] = wsc
	}
	// Check if any transactions spending these outputs already exists in the mempool, if so send the notification immediately.
	spends := make(map[chainhash.Hash]*util.Tx)
	for _, op := range ops {
		spend := m.server.cfg.TxMemPool.CheckSpend(*op)
		if spend != nil {
			log <- cl.Debugf{
				"found existing mempool spend for outpoint<%v>: %v",
				op, spend.Hash(),
			}
			spends[*spend.Hash()] = spend
		}
	}
	for _, spend := range spends {
		m.notifyForTx(opMap, nil, spend, nil)
	}
}
// UnregisterSpentRequest removes a request from the passed websocket client to be notified when the passed outpoint is confirmed spent (contained in a block connected to the main chain).
func (m *wsNotificationManager) UnregisterSpentRequest(wsc *wsClient, op *wire.OutPoint) {
	m.queueNotification <- &notificationUnregisterSpent{
		wsc: wsc,
		op:  op,
	}
}
// removeSpentRequest modifies a map of watched outpoints to remove the websocket client wsc from the set of clients to be notified when a watched outpoint is spent.  If wsc is the last client, the outpoint key is removed from the map.
func (*wsNotificationManager) removeSpentRequest(ops map[wire.OutPoint]map[chan struct{}]*wsClient,
	wsc *wsClient, op *wire.OutPoint) {
	// Remove the request tracking from the client.
	delete(wsc.spentRequests, *op)
	// Remove the client from the list to notify.
	notifyMap, ok := ops[*op]
	if !ok {
		log <- cl.Warn{
			"attempt to remove nonexistent spent request for websocket client", wsc.addr,
		}
		return
	}
	delete(notifyMap, wsc.quit)
	// Remove the map entry altogether if there are no more clients interested in it.
	if len(notifyMap) == 0 {
		delete(ops, *op)
	}
}
// txHexString returns the serialized transaction encoded in hexadecimal.
func txHexString(tx *wire.MsgTx) string {
	buf := bytes.NewBuffer(make([]byte, 0, tx.SerializeSize()))
	// Ignore Serialize's error, as writing to a bytes.buffer cannot fail.
	tx.Serialize(buf)
	return hex.EncodeToString(buf.Bytes())
}
// blockDetails creates a BlockDetails struct to include in btcws notifications from a block and a transaction's block index.
func blockDetails(block *util.Block, txIndex int) *json.BlockDetails {
	if block == nil {
		return nil
	}
	return &json.BlockDetails{
		Height: block.Height(),
		Hash:   block.Hash().String(),
		Index:  txIndex,
		Time:   block.MsgBlock().Header.Timestamp.Unix(),
	}
}
// newRedeemingTxNotification returns a new marshalled redeemingtx notification with the passed parameters.
func newRedeemingTxNotification(txHex string, index int, block *util.Block) ([]byte, error) {
	// Create and marshal the notification.
	ntfn := json.NewRedeemingTxNtfn(txHex, blockDetails(block, index))
	return json.MarshalCmd(nil, ntfn)
}
// notifyForTxOuts examines each transaction output, notifying interested websocket clients of the transaction if an output spends to a watched address.  A spent notification request is automatically registered for the client for each matching output.
func (m *wsNotificationManager) notifyForTxOuts(ops map[wire.OutPoint]map[chan struct{}]*wsClient,
	addrs map[string]map[chan struct{}]*wsClient, tx *util.Tx, block *util.Block) {
	// Nothing to do if nobody is listening for address notifications.
	if len(addrs) == 0 {
		return
	}
	txHex := ""
	wscNotified := make(map[chan struct{}]struct{})
	for i, txOut := range tx.MsgTx().TxOut {
		_, txAddrs, _, err := txscript.ExtractPkScriptAddrs(
			txOut.PkScript, m.server.cfg.ChainParams)
		if err != nil {
			continue
		}
		for _, txAddr := range txAddrs {
			cmap, ok := addrs[txAddr.EncodeAddress()]
			if !ok {
				continue
			}
			if txHex == "" {
				txHex = txHexString(tx.MsgTx())
			}
			ntfn := json.NewRecvTxNtfn(txHex, blockDetails(block,
				tx.Index()))
			marshalledJSON, err := json.MarshalCmd(nil, ntfn)
			if err != nil {
				log <- cl.Error{
					"Failed to marshal processedtx notification:", err,
				}
				continue
			}
			op := []*wire.OutPoint{wire.NewOutPoint(tx.Hash(), uint32(i))}
			for wscQuit, wsc := range cmap {
				m.addSpentRequests(ops, wsc, op)
				if _, ok := wscNotified[wscQuit]; !ok {
					wscNotified[wscQuit] = struct{}{}
					wsc.QueueNotification(marshalledJSON)
				}
			}
		}
	}
}
// notifyRelevantTxAccepted examines the inputs and outputs of the passed transaction, notifying websocket clients of outputs spending to a watched address and inputs spending a watched outpoint.  Any outputs paying to a watched address result in the output being watched as well for future notifications.
func (m *wsNotificationManager) notifyRelevantTxAccepted(tx *util.Tx,
	clients map[chan struct{}]*wsClient) {
	clientsToNotify := m.subscribedClients(tx, clients)
	if len(clientsToNotify) != 0 {
		n := json.NewRelevantTxAcceptedNtfn(txHexString(tx.MsgTx()))
		marshalled, err := json.MarshalCmd(nil, n)
		if err != nil {
			log <- cl.Error{
				"failed to marshal notification:", err,
			}
			return
		}
		for quitChan := range clientsToNotify {
			clients[quitChan].QueueNotification(marshalled)
		}
	}
}
// notifyForTx examines the inputs and outputs of the passed transaction, notifying websocket clients of outputs spending to a watched address and inputs spending a watched outpoint.
func (m *wsNotificationManager) notifyForTx(ops map[wire.OutPoint]map[chan struct{}]*wsClient,
	addrs map[string]map[chan struct{}]*wsClient, tx *util.Tx, block *util.Block) {
	if len(ops) != 0 {
		m.notifyForTxIns(ops, tx, block)
	}
	if len(addrs) != 0 {
		m.notifyForTxOuts(ops, addrs, tx, block)
	}
}
// notifyForTxIns examines the inputs of the passed transaction and sends interested websocket clients a redeemingtx notification if any inputs spend a watched output.  If block is non-nil, any matching spent requests are removed.
func (m *wsNotificationManager) notifyForTxIns(ops map[wire.OutPoint]map[chan struct{}]*wsClient,
	tx *util.Tx, block *util.Block) {
	// Nothing to do if nobody is watching outpoints.
	if len(ops) == 0 {
		return
	}
	txHex := ""
	wscNotified := make(map[chan struct{}]struct{})
	for _, txIn := range tx.MsgTx().TxIn {
		prevOut := &txIn.PreviousOutPoint
		if cmap, ok := ops[*prevOut]; ok {
			if txHex == "" {
				txHex = txHexString(tx.MsgTx())
			}
			marshalledJSON, err := newRedeemingTxNotification(txHex, tx.Index(), block)
			if err != nil {
				log <- cl.Warn{
					"failed to marshal redeemingtx notification:", err,
				}
				continue
			}
			for wscQuit, wsc := range cmap {
				if block != nil {
					m.removeSpentRequest(ops, wsc, prevOut)
				}
				if _, ok := wscNotified[wscQuit]; !ok {
					wscNotified[wscQuit] = struct{}{}
					wsc.QueueNotification(marshalledJSON)
				}
			}
		}
	}
}
// RegisterTxOutAddressRequests requests notifications to the passed websocket client when a transaction output spends to the passed address.
func (m *wsNotificationManager) RegisterTxOutAddressRequests(wsc *wsClient, addrs []string) {
	m.queueNotification <- &notificationRegisterAddr{
		wsc:   wsc,
		addrs: addrs,
	}
}
// addAddrRequests adds the websocket client wsc to the address to client set addrMap so wsc will be notified for any mempool or block transaction outputs spending to any of the addresses in addrs.
func (*wsNotificationManager) addAddrRequests(addrMap map[string]map[chan struct{}]*wsClient,
	wsc *wsClient, addrs []string) {
	for _, addr := range addrs {
		// Track the request in the client as well so it can be quickly be removed on disconnect.
		wsc.addrRequests[addr] = struct{}{}
		// Add the client to the set of clients to notify when the outpoint is seen.  Create map as needed.
		cmap, ok := addrMap[addr]
		if !ok {
			cmap = make(map[chan struct{}]*wsClient)
			addrMap[addr] = cmap
		}
		cmap[wsc.quit] = wsc
	}
}
// UnregisterTxOutAddressRequest removes a request from the passed websocket client to be notified when a transaction spends to the passed address.
func (m *wsNotificationManager) UnregisterTxOutAddressRequest(wsc *wsClient, addr string) {
	m.queueNotification <- &notificationUnregisterAddr{
		wsc:  wsc,
		addr: addr,
	}
}
// removeAddrRequest removes the websocket client wsc from the address to client set addrs so it will no longer receive notification updates for any transaction outputs send to addr.
func (*wsNotificationManager) removeAddrRequest(addrs map[string]map[chan struct{}]*wsClient,
	wsc *wsClient, addr string) {
	// Remove the request tracking from the client.
	delete(wsc.addrRequests, addr)
	// Remove the client from the list to notify.
	cmap, ok := addrs[addr]
	if !ok {
		log <- cl.Warnf{
			"attempt to remove nonexistent addr request <%s> for websocket client %s",
			addr, wsc.addr,
		}
		return
	}
	delete(cmap, wsc.quit)
	// Remove the map entry altogether if there are no more clients interested in it.
	if len(cmap) == 0 {
		delete(addrs, addr)
	}
}
// AddClient adds the passed websocket client to the notification manager.
func (m *wsNotificationManager) AddClient(wsc *wsClient) {
	m.queueNotification <- (*notificationRegisterClient)(wsc)
}
// RemoveClient removes the passed websocket client and all notifications registered for it.
func (m *wsNotificationManager) RemoveClient(wsc *wsClient) {
	select {
	case m.queueNotification <- (*notificationUnregisterClient)(wsc):
		// fmt.Println("chan:m.queueNotification <- (*notificationUnregisterClient)(wsc)")
	case <-m.quit:
		// fmt.Println("chan:<-m.quit")
	}
}
// Start starts the goroutines required for the manager to queue and process websocket client notifications.
func (m *wsNotificationManager) Start() {
	m.wg.Add(2)
	go m.queueHandler()
	go m.notificationHandler()
}
// WaitForShutdown blocks until all notification manager goroutines have finished.
func (m *wsNotificationManager) WaitForShutdown() {
	m.wg.Wait()
}
// Shutdown shuts down the manager, stopping the notification queue and notification handler goroutines.
func (m *wsNotificationManager) Shutdown() {
	close(m.quit)
}
// newWsNotificationManager returns a new notification manager ready for use. See wsNotificationManager for more details.
func newWsNotificationManager(server *rpcServer) *wsNotificationManager {
	return &wsNotificationManager{
		server:            server,
		queueNotification: make(chan interface{}),
		notificationMsgs:  make(chan interface{}),
		numClients:        make(chan int),
		quit:              make(chan struct{}),
	}
}
// wsResponse houses a message to send to a connected websocket client as well as a channel to reply on when the message is sent.
type wsResponse struct {
	msg      []byte
	doneChan chan bool
}
// wsClient provides an abstraction for handling a websocket client.  The overall data flow is split into 3 main goroutines, a possible 4th goroutine for long-running operations (only started if request is made), and a websocket manager which is used to allow things such as broadcasting requested notifications to all connected websocket clients.   Inbound messages are read via the inHandler goroutine and generally dispatched to their own handler.  However, certain potentially long-running operations such as rescans, are sent to the asyncHander goroutine and are limited to one at a time.  There are two outbound message types - one for responding to client requests and another for async notifications.  Responses to client requests use SendMessage which employs a buffered channel thereby limiting the number of outstanding requests that can be made.  Notifications are sent via QueueNotification which implements a queue via notificationQueueHandler to ensure sending notifications from other subsystems can't block.  Ultimately, all messages are sent via the outHandler.
type wsClient struct {
	sync.Mutex
	// server is the RPC server that is servicing the client.
	server *rpcServer
	// conn is the underlying websocket connection.
	conn *websocket.Conn
	// disconnected indicated whether or not the websocket client is disconnected.
	disconnected bool
	// addr is the remote address of the client.
	addr string
	// authenticated specifies whether a client has been authenticated and therefore is allowed to communicated over the websocket.
	authenticated bool
	// isAdmin specifies whether a client may change the state of the server; false means its access is only to the limited set of RPC calls.
	isAdmin bool
	// sessionID is a random ID generated for each client when connected. These IDs may be queried by a client using the session RPC.  A change to the session ID indicates that the client reconnected.
	sessionID uint64
	// verboseTxUpdates specifies whether a client has requested verbose information about all new transactions.
	verboseTxUpdates bool
	// addrRequests is a set of addresses the caller has requested to be notified about.  It is maintained here so all requests can be removed when a wallet disconnects.  Owned by the notification manager.
	addrRequests map[string]struct{}
	// spentRequests is a set of unspent Outpoints a wallet has requested notifications for when they are spent by a processed transaction. Owned by the notification manager.
	spentRequests map[wire.OutPoint]struct{}
	// filterData is the new generation transaction filter backported from github.com/decred/dcrd for the new backported `loadtxfilter` and `rescanblocks` methods.
	filterData *wsClientFilter
	// Networking infrastructure.
	serviceRequestSem semaphore
	ntfnChan          chan []byte
	sendChan          chan wsResponse
	quit              chan struct{}
	wg                sync.WaitGroup
}
// inHandler handles all incoming messages for the websocket connection.  It must be run as a goroutine.
func (c *wsClient) inHandler() {
out:
	for {
		// Break out of the loop once the quit channel has been closed. Use a non-blocking select here so we fall through otherwise.
		select {
		case <-c.quit:
			// fmt.Println("chan:<-c.quit")
			break out
		default:
		}
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			// Log the error if it's not due to disconnecting.
			if err != io.EOF {
				log <- cl.Errorf{
					"websocket receive error from %s: %v",
					c.addr, err,
				}
			}
			break out
		}
		var request json.Request
		err = js.Unmarshal(msg, &request)
		if err != nil {
			if !c.authenticated {
				break out
			}
			jsonErr := &json.RPCError{
				Code:    json.ErrRPCParse.Code,
				Message: "Failed to parse request: " + err.Error(),
			}
			reply, err := createMarshalledReply(nil, nil, jsonErr)
			if err != nil {
				log <- cl.Error{
					"failed to marshal parse failure reply:", err,
				}
				continue
			}
			c.SendMessage(reply, nil)
			continue
		}
		// The JSON-RPC 1.0 spec defines that notifications must have their "id" set to null and states that notifications do not have a response.
		// A JSON-RPC 2.0 notification is a request with "json-rpc":"2.0", and without an "id" member. The specification states that notifications must not be responded to. JSON-RPC 2.0 permits the null value as a valid request id, therefore such requests are not notifications.
		// Bitcoin Core serves requests with "id":null or even an absent "id", and responds to such requests with "id":null in the response.
		// Pod does not respond to any request without and "id" or "id":null, regardless the indicated JSON-RPC protocol version unless RPC quirks are enabled. With RPC quirks enabled, such requests will be responded to if the reqeust does not indicate JSON-RPC version.
		// RPC quirks can be enabled by the user to avoid compatibility issues with software relying on Core's behavior.
		if request.ID == nil && !(cfg.RPCQuirks && request.Jsonrpc == "") {
			if !c.authenticated {
				break out
			}
			continue
		}
		cmd := parseCmd(&request)
		if cmd.err != nil {
			if !c.authenticated {
				break out
			}
			reply, err := createMarshalledReply(cmd.id, nil, cmd.err)
			if err != nil {
				log <- cl.Errorf{
					"failed to marshal parse failure reply:", err,
				}
				continue
			}
			c.SendMessage(reply, nil)
			continue
		}
		log <- cl.Tracef{
			"received command <%s> from %s", cmd.method, c.addr,
		}
		// Check auth.  The client is immediately disconnected if the first request of an unauthentiated websocket client is not the authenticate request, an authenticate request is received when the client is already authenticated, or incorrect authentication credentials are provided in the request.
		switch authCmd, ok := cmd.cmd.(*json.AuthenticateCmd); {
		case c.authenticated && ok:
			log <- cl.Warnf{
				"websocket client %s is already authenticated", c.addr,
			}
			break out
		case !c.authenticated && !ok:
			log <- cl.Warnf{
				"unauthenticated websocket message received"}
			break out
		case !c.authenticated:
			// Check credentials.
			login := authCmd.Username + ":" + authCmd.Passphrase
			auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(login))
			authSha := sha256.Sum256([]byte(auth))
			cmp := subtle.ConstantTimeCompare(authSha[:], c.server.authsha[:])
			limitcmp := subtle.ConstantTimeCompare(authSha[:], c.server.limitauthsha[:])
			if cmp != 1 && limitcmp != 1 {
				log <- cl.Warn{"authentication failure from", c.addr}
				break out
			}
			c.authenticated = true
			c.isAdmin = cmp == 1
			// Marshal and send response.
			reply, err := createMarshalledReply(cmd.id, nil, nil)
			if err != nil {
				log <- cl.Error{"failed to marshal authenticate reply:", err}
				continue
			}
			c.SendMessage(reply, nil)
			continue
		}
		// Check if the client is using limited RPC credentials and error when not authorized to call this RPC.
		if !c.isAdmin {
			if _, ok := rpcLimited[request.Method]; !ok {
				jsonErr := &json.RPCError{
					Code:    json.ErrRPCInvalidParams.Code,
					Message: "limited user not authorized for this method",
				}
				// Marshal and send response.
				reply, err := createMarshalledReply(request.ID, nil, jsonErr)
				if err != nil {
					log <- cl.Error{"failed to marshal parse failure reply:", err}
					continue
				}
				c.SendMessage(reply, nil)
				continue
			}
		}
		// Asynchronously handle the request.  A semaphore is used to limit the number of concurrent requests currently being serviced.  If the semaphore can not be acquired, simply wait until a request finished before reading the next RPC request from the websocket client.
		// This could be a little fancier by timing out and erroring when it takes too long to service the request, but if that is done, the read of the next request should not be blocked by this semaphore, otherwise the next request will be read and will probably sit here for another few seconds before timing out as well.  This will cause the total timeout duration for later requests to be much longer than the check here would imply.
		// If a timeout is added, the semaphore acquiring should be moved inside of the new goroutine with a select statement that also reads a time.After channel.  This will unblock the read of the next request from the websocket client and allow many requests to be waited on concurrently.
		c.serviceRequestSem.acquire()
		go func() {
			c.serviceRequest(cmd)
			c.serviceRequestSem.release()
		}()
	}
	// Ensure the connection is closed.
	c.Disconnect()
	c.wg.Done()
	log <- cl.Trace{"websocket client input handler done for", c.addr}
}
// serviceRequest services a parsed RPC request by looking up and executing the appropriate RPC handler.  The response is marshalled and sent to the websocket client.
func (c *wsClient) serviceRequest(r *parsedRPCCmd) {
	var (
		result interface{}
		err    error
	)
	// Lookup the websocket extension for the command and if it doesn't exist fallback to handling the command as a standard command.
	wsHandler, ok := wsHandlers[r.method]
	if ok {
		result, err = wsHandler(c, r.cmd)
	} else {
		result, err = c.server.standardCmdResult(r, nil)
	}
	reply, err := createMarshalledReply(r.id, result, err)
	if err != nil {
		log <- cl.Errorf{
			"failed to marshal reply for <%s> command: %v", r.method, err,
		}
		return
	}
	c.SendMessage(reply, nil)
}
// notificationQueueHandler handles the queuing of outgoing notifications for the websocket client.  This runs as a muxer for various sources of input to ensure that queuing up notifications to be sent will not block.  Otherwise, slow clients could bog down the other systems (such as the mempool or block manager) which are queuing the data.  The data is passed on to outHandler to actually be written.  It must be run as a goroutine.
func (c *wsClient) notificationQueueHandler() {
	ntfnSentChan := make(chan bool, 1) // nonblocking sync
	// pendingNtfns is used as a queue for notifications that are ready to be sent once there are no outstanding notifications currently being sent.  The waiting flag is used over simply checking for items in the pending list to ensure cleanup knows what has and hasn't been sent to the outHandler.  Currently no special cleanup is needed, however if something like a done channel is added to notifications in the future, not knowing what has and hasn't been sent to the outHandler (and thus who should respond to the done channel) would be problematic without using this approach.
	pendingNtfns := list.New()
	waiting := false
out:
	for {
		select {
		// This channel is notified when a message is being queued to be sent across the network socket.  It will either send the message immediately if a send is not already in progress, or queue the message to be sent once the other pending messages are sent.
		case msg := <-c.ntfnChan:
			// fmt.Println("chan:msg := <-c.ntfnChan")
			if !waiting {
				c.SendMessage(msg, ntfnSentChan)
			} else {
				pendingNtfns.PushBack(msg)
			}
			waiting = true
		// This channel is notified when a notification has been sent across the network socket.
		case <-ntfnSentChan:
			// fmt.Println("chan:<-ntfnSentChan")
			// No longer waiting if there are no more messages in the pending messages queue.
			next := pendingNtfns.Front()
			if next == nil {
				waiting = false
				continue
			}
			// Notify the outHandler about the next item to asynchronously send.
			msg := pendingNtfns.Remove(next).([]byte)
			c.SendMessage(msg, ntfnSentChan)
		case <-c.quit:
			break out
		}
	}
	// Drain any wait channels before exiting so nothing is left waiting around to send.
cleanup:
	for {
		select {
		case <-c.ntfnChan:
			// fmt.Println("chan:<-c.ntfnChan")
		case <-ntfnSentChan:
			// fmt.Println("chan:<-ntfnSentChan")
		default:
			break cleanup
		}
	}
	c.wg.Done()
	log <- cl.Trace{
		"websocket client notification queue handler done for", c.addr,
	}
}
// outHandler handles all outgoing messages for the websocket connection.  It must be run as a goroutine.  It uses a buffered channel to serialize output messages while allowing the sender to continue running asynchronously.  It must be run as a goroutine.
func (c *wsClient) outHandler() {
out:
	for {
		// Send any messages ready for send until the quit channel is closed.
		select {
		case r := <-c.sendChan:
			// fmt.Println("chan:r := <-c.sendChan")
			err := c.conn.WriteMessage(websocket.TextMessage, r.msg)
			if err != nil {
				c.Disconnect()
				break out
			}
			if r.doneChan != nil {
				r.doneChan <- true
			}
		case <-c.quit:
			// fmt.Println("chan:<-c.quit")
			break out
		}
	}
	// Drain any wait channels before exiting so nothing is left waiting around to send.
cleanup:
	for {
		select {
		case r := <-c.sendChan:
			// fmt.Println("chan:r := <-c.sendChan")
			if r.doneChan != nil {
				r.doneChan <- false
			}
		default:
			break cleanup
		}
	}
	c.wg.Done()
	log <- cl.Trace{
		"websocket client output handler done for", c.addr,
	}
}
// SendMessage sends the passed json to the websocket client.  It is backed by a buffered channel, so it will not block until the send channel is full. Note however that QueueNotification must be used for sending async notifications instead of the this function.  This approach allows a limit to the number of outstanding requests a client can make without preventing or blocking on async notifications.
func (c *wsClient) SendMessage(marshalledJSON []byte, doneChan chan bool) {
	// Don't send the message if disconnected.
	if c.Disconnected() {
		if doneChan != nil {
			doneChan <- false
		}
		return
	}
	c.sendChan <- wsResponse{msg: marshalledJSON, doneChan: doneChan}
}
// ErrClientQuit describes the error where a client send is not processed due to the client having already been disconnected or dropped.
var ErrClientQuit = errors.New("client quit")
// QueueNotification queues the passed notification to be sent to the websocket client.  This function, as the name implies, is only intended for notifications since it has additional logic to prevent other subsystems, such as the memory pool and block manager, from blocking even when the send channel is full.
// If the client is in the process of shutting down, this function returns ErrClientQuit.  This is intended to be checked by long-running notification handlers to stop processing if there is no more work needed to be done.
func (c *wsClient) QueueNotification(marshalledJSON []byte) error {
	// Don't queue the message if disconnected.
	if c.Disconnected() {
		return ErrClientQuit
	}
	c.ntfnChan <- marshalledJSON
	return nil
}
// Disconnected returns whether or not the websocket client is disconnected.
func (c *wsClient) Disconnected() bool {
	c.Lock()
	isDisconnected := c.disconnected
	c.Unlock()
	return isDisconnected
}
// Disconnect disconnects the websocket client.
func (c *wsClient) Disconnect() {
	c.Lock()
	defer c.Unlock()
	// Nothing to do if already disconnected.
	if c.disconnected {
		return
	}
	log <- cl.Trace{"disconnecting websocket client", c.addr}
	close(c.quit)
	c.conn.Close()
	c.disconnected = true
}
// Start begins processing input and output messages.
func (c *wsClient) Start() {
	log <- cl.Trace{"starting websocket client", c.addr}
	// Start processing input and output.
	c.wg.Add(3)
	go c.inHandler()
	go c.notificationQueueHandler()
	go c.outHandler()
}
// WaitForShutdown blocks until the websocket client goroutines are stopped and the connection is closed.
func (c *wsClient) WaitForShutdown() {
	c.wg.Wait()
}
// newWebsocketClient returns a new websocket client given the notification manager, websocket connection, remote address, and whether or not the client has already been authenticated (via HTTP Basic access authentication).  The returned client is ready to start.  Once started, the client will process incoming and outgoing messages in separate goroutines complete with queuing and asynchrous handling for long-running operations.
func newWebsocketClient(server *rpcServer, conn *websocket.Conn,
	remoteAddr string, authenticated bool, isAdmin bool) (*wsClient, error) {
	sessionID, err := wire.RandomUint64()
	if err != nil {
		return nil, err
	}
	client := &wsClient{
		conn:              conn,
		addr:              remoteAddr,
		authenticated:     authenticated,
		isAdmin:           isAdmin,
		sessionID:         sessionID,
		server:            server,
		addrRequests:      make(map[string]struct{}),
		spentRequests:     make(map[wire.OutPoint]struct{}),
		serviceRequestSem: makeSemaphore(cfg.RPCMaxConcurrentReqs),
		ntfnChan:          make(chan []byte, 1), // nonblocking sync
		sendChan:          make(chan wsResponse, websocketSendBufferSize),
		quit:              make(chan struct{}),
	}
	return client, nil
}
// handleWebsocketHelp implements the help command for websocket connections.
func handleWebsocketHelp(wsc *wsClient, icmd interface{}) (interface{}, error) {
	cmd, ok := icmd.(*json.HelpCmd)
	if !ok {
		return nil, json.ErrRPCInternal
	}
	// Provide a usage overview of all commands when no specific command was specified.
	var command string
	if cmd.Command != nil {
		command = *cmd.Command
	}
	if command == "" {
		usage, err := wsc.server.helpCacher.rpcUsage(true)
		if err != nil {
			context := "Failed to generate RPC usage"
			return nil, internalRPCError(err.Error(), context)
		}
		return usage, nil
	}
	// Check that the command asked for is supported and implemented. Search the list of websocket handlers as well as the main list of handlers since help should only be provided for those cases.
	valid := true
	if _, ok := rpcHandlers[command]; !ok {
		if _, ok := wsHandlers[command]; !ok {
			valid = false
		}
	}
	if !valid {
		return nil, &json.RPCError{
			Code:    json.ErrRPCInvalidParameter,
			Message: "Unknown command: " + command,
		}
	}
	// Get the help for the command.
	help, err := wsc.server.helpCacher.rpcMethodHelp(command)
	if err != nil {
		context := "Failed to generate help"
		return nil, internalRPCError(err.Error(), context)
	}
	return help, nil
}
// handleLoadTxFilter implements the loadtxfilter command extension for websocket connections. NOTE: This extension is ported from github.com/decred/dcrd
func handleLoadTxFilter(wsc *wsClient, icmd interface{}) (interface{}, error) {
	cmd := icmd.(*json.LoadTxFilterCmd)
	outPoints := make([]wire.OutPoint, len(cmd.OutPoints))
	for i := range cmd.OutPoints {
		hash, err := chainhash.NewHashFromStr(cmd.OutPoints[i].Hash)
		if err != nil {
			return nil, &json.RPCError{
				Code:    json.ErrRPCInvalidParameter,
				Message: err.Error(),
			}
		}
		outPoints[i] = wire.OutPoint{
			Hash:  *hash,
			Index: cmd.OutPoints[i].Index,
		}
	}
	params := wsc.server.cfg.ChainParams
	wsc.Lock()
	if cmd.Reload || wsc.filterData == nil {
		wsc.filterData = newWSClientFilter(cmd.Addresses, outPoints,
			params)
		wsc.Unlock()
	} else {
		wsc.Unlock()
		wsc.filterData.mu.Lock()
		for _, a := range cmd.Addresses {
			wsc.filterData.addAddressStr(a, params)
		}
		for i := range outPoints {
			wsc.filterData.addUnspentOutPoint(&outPoints[i])
		}
		wsc.filterData.mu.Unlock()
	}
	return nil, nil
}
// handleNotifyBlocks implements the notifyblocks command extension for websocket connections.
func handleNotifyBlocks(wsc *wsClient, icmd interface{}) (interface{}, error) {
	wsc.server.ntfnMgr.RegisterBlockUpdates(wsc)
	return nil, nil
}
// handleSession implements the session command extension for websocket connections.
func handleSession(wsc *wsClient, icmd interface{}) (interface{}, error) {
	return &json.SessionResult{SessionID: wsc.sessionID}, nil
}
// handleStopNotifyBlocks implements the stopnotifyblocks command extension for websocket connections.
func handleStopNotifyBlocks(wsc *wsClient, icmd interface{}) (interface{}, error) {
	wsc.server.ntfnMgr.UnregisterBlockUpdates(wsc)
	return nil, nil
}
// handleNotifySpent implements the notifyspent command extension for websocket connections.
func handleNotifySpent(wsc *wsClient, icmd interface{}) (interface{}, error) {
	cmd, ok := icmd.(*json.NotifySpentCmd)
	if !ok {
		return nil, json.ErrRPCInternal
	}
	outpoints, err := deserializeOutpoints(cmd.OutPoints)
	if err != nil {
		return nil, err
	}
	wsc.server.ntfnMgr.RegisterSpentRequests(wsc, outpoints)
	return nil, nil
}
// handleNotifyNewTransations implements the notifynewtransactions command extension for websocket connections.
func handleNotifyNewTransactions(wsc *wsClient, icmd interface{}) (interface{}, error) {
	cmd, ok := icmd.(*json.NotifyNewTransactionsCmd)
	if !ok {
		return nil, json.ErrRPCInternal
	}
	wsc.verboseTxUpdates = cmd.Verbose != nil && *cmd.Verbose
	wsc.server.ntfnMgr.RegisterNewMempoolTxsUpdates(wsc)
	return nil, nil
}
// handleStopNotifyNewTransations implements the stopnotifynewtransactions command extension for websocket connections.
func handleStopNotifyNewTransactions(wsc *wsClient, icmd interface{}) (interface{}, error) {
	wsc.server.ntfnMgr.UnregisterNewMempoolTxsUpdates(wsc)
	return nil, nil
}
// handleNotifyReceived implements the notifyreceived command extension for websocket connections.
func handleNotifyReceived(wsc *wsClient, icmd interface{}) (interface{}, error) {
	cmd, ok := icmd.(*json.NotifyReceivedCmd)
	if !ok {
		return nil, json.ErrRPCInternal
	}
	// Decode addresses to validate input, but the strings slice is used directly if these are all ok.
	err := checkAddressValidity(cmd.Addresses, wsc.server.cfg.ChainParams)
	if err != nil {
		return nil, err
	}
	wsc.server.ntfnMgr.RegisterTxOutAddressRequests(wsc, cmd.Addresses)
	return nil, nil
}
// handleStopNotifySpent implements the stopnotifyspent command extension for websocket connections.
func handleStopNotifySpent(wsc *wsClient, icmd interface{}) (interface{}, error) {
	cmd, ok := icmd.(*json.StopNotifySpentCmd)
	if !ok {
		return nil, json.ErrRPCInternal
	}
	outpoints, err := deserializeOutpoints(cmd.OutPoints)
	if err != nil {
		return nil, err
	}
	for _, outpoint := range outpoints {
		wsc.server.ntfnMgr.UnregisterSpentRequest(wsc, outpoint)
	}
	return nil, nil
}
// handleStopNotifyReceived implements the stopnotifyreceived command extension for websocket connections.
func handleStopNotifyReceived(wsc *wsClient, icmd interface{}) (interface{}, error) {
	cmd, ok := icmd.(*json.StopNotifyReceivedCmd)
	if !ok {
		return nil, json.ErrRPCInternal
	}
	// Decode addresses to validate input, but the strings slice is used directly if these are all ok.
	err := checkAddressValidity(cmd.Addresses, wsc.server.cfg.ChainParams)
	if err != nil {
		return nil, err
	}
	for _, addr := range cmd.Addresses {
		wsc.server.ntfnMgr.UnregisterTxOutAddressRequest(wsc, addr)
	}
	return nil, nil
}
// checkAddressValidity checks the validity of each address in the passed string slice. It does this by attempting to decode each address using the current active network parameters. If any single address fails to decode properly, the function returns an error. Otherwise, nil is returned.
func checkAddressValidity(addrs []string, params *chaincfg.Params) error {
	for _, addr := range addrs {
		_, err := util.DecodeAddress(addr, params)
		if err != nil {
			return &json.RPCError{
				Code: json.ErrRPCInvalidAddressOrKey,
				Message: fmt.Sprintf("Invalid address or key: %v",
					addr),
			}
		}
	}
	return nil
}
// deserializeOutpoints deserializes each serialized outpoint.
func deserializeOutpoints(serializedOuts []json.OutPoint) ([]*wire.OutPoint, error) {
	outpoints := make([]*wire.OutPoint, 0, len(serializedOuts))
	for i := range serializedOuts {
		blockHash, err := chainhash.NewHashFromStr(serializedOuts[i].Hash)
		if err != nil {
			return nil, rpcDecodeHexError(serializedOuts[i].Hash)
		}
		index := serializedOuts[i].Index
		outpoints = append(outpoints, wire.NewOutPoint(blockHash, index))
	}
	return outpoints, nil
}
type rescanKeys struct {
	fallbacks           map[string]struct{}
	pubKeyHashes        map[[ripemd160.Size]byte]struct{}
	scriptHashes        map[[ripemd160.Size]byte]struct{}
	compressedPubKeys   map[[33]byte]struct{}
	uncompressedPubKeys map[[65]byte]struct{}
	unspent             map[wire.OutPoint]struct{}
}
// unspentSlice returns a slice of currently-unspent outpoints for the rescan lookup keys.  This is primarily intended to be used to register outpoints for continuous notifications after a rescan has completed.
func (r *rescanKeys) unspentSlice() []*wire.OutPoint {
	ops := make([]*wire.OutPoint, 0, len(r.unspent))
	for op := range r.unspent {
		opCopy := op
		ops = append(ops, &opCopy)
	}
	return ops
}
// ErrRescanReorg defines the error that is returned when an unrecoverable reorganize is detected during a rescan.
var ErrRescanReorg = json.RPCError{
	Code:    json.ErrRPCDatabase,
	Message: "Reorganize",
}
// rescanBlock rescans all transactions in a single block.  This is a helper function for handleRescan.
func rescanBlock(wsc *wsClient, lookups *rescanKeys, blk *util.Block) {
	for _, tx := range blk.Transactions() {
		// Hexadecimal representation of this tx.  Only created if needed, and reused for later notifications if already made.
		var txHex string
		// All inputs and outputs must be iterated through to correctly modify the unspent map, however, just a single notification for any matching transaction inputs or outputs should be created and sent.
		spentNotified := false
		recvNotified := false
		for _, txin := range tx.MsgTx().TxIn {
			if _, ok := lookups.unspent[txin.PreviousOutPoint]; ok {
				delete(lookups.unspent, txin.PreviousOutPoint)
				if spentNotified {
					continue
				}
				if txHex == "" {
					txHex = txHexString(tx.MsgTx())
				}
				marshalledJSON, err := newRedeemingTxNotification(txHex, tx.Index(), blk)
				if err != nil {
					log <- cl.Error{"failed to marshal redeemingtx notification:", err}
					continue
				}
				err = wsc.QueueNotification(marshalledJSON)
				// Stop the rescan early if the websocket client disconnected.
				if err == ErrClientQuit {
					return
				}
				spentNotified = true
			}
		}
		for txOutIdx, txout := range tx.MsgTx().TxOut {
			_, addrs, _, _ := txscript.ExtractPkScriptAddrs(
				txout.PkScript, wsc.server.cfg.ChainParams)
			for _, addr := range addrs {
				switch a := addr.(type) {
				case *util.AddressPubKeyHash:
					if _, ok := lookups.pubKeyHashes[*a.Hash160()]; !ok {
						continue
					}
				case *util.AddressScriptHash:
					if _, ok := lookups.scriptHashes[*a.Hash160()]; !ok {
						continue
					}
				case *util.AddressPubKey:
					found := false
					switch sa := a.ScriptAddress(); len(sa) {
					case 33: // Compressed
						var key [33]byte
						copy(key[:], sa)
						if _, ok := lookups.compressedPubKeys[key]; ok {
							found = true
						}
					case 65: // Uncompressed
						var key [65]byte
						copy(key[:], sa)
						if _, ok := lookups.uncompressedPubKeys[key]; ok {
							found = true
						}
					default:
						log <- cl.Warnf{
							"skipping rescanned pubkey of unknown serialized length", len(sa),
						}
						continue
					}
					// If the transaction output pays to the pubkey of a rescanned P2PKH address, include it as well.
					if !found {
						pkh := a.AddressPubKeyHash()
						if _, ok := lookups.pubKeyHashes[*pkh.Hash160()]; !ok {
							continue
						}
					}
				default:
					// A new address type must have been added.  Encode as a payment address string and check the fallback map.
					addrStr := addr.EncodeAddress()
					_, ok := lookups.fallbacks[addrStr]
					if !ok {
						continue
					}
				}
				outpoint := wire.OutPoint{
					Hash:  *tx.Hash(),
					Index: uint32(txOutIdx),
				}
				lookups.unspent[outpoint] = struct{}{}
				if recvNotified {
					continue
				}
				if txHex == "" {
					txHex = txHexString(tx.MsgTx())
				}
				ntfn := json.NewRecvTxNtfn(txHex,
					blockDetails(blk, tx.Index()))
				marshalledJSON, err := json.MarshalCmd(nil, ntfn)
				if err != nil {
					log <- cl.Error{"failed to marshal recvtx notification:", err}
					return
				}
				err = wsc.QueueNotification(marshalledJSON)
				// Stop the rescan early if the websocket client disconnected.
				if err == ErrClientQuit {
					return
				}
				recvNotified = true
			}
		}
	}
}
// rescanBlockFilter rescans a block for any relevant transactions for the passed lookup keys. Any discovered transactions are returned hex encoded as a string slice. NOTE: This extension is ported from github.com/decred/dcrd
func rescanBlockFilter(filter *wsClientFilter, block *util.Block, params *chaincfg.Params) []string {
	var transactions []string
	filter.mu.Lock()
	for _, tx := range block.Transactions() {
		msgTx := tx.MsgTx()
		// Keep track of whether the transaction has already been added to the result.  It shouldn't be added twice.
		added := false
		// Scan inputs if not a coinbase transaction.
		if !blockchain.IsCoinBaseTx(msgTx) {
			for _, input := range msgTx.TxIn {
				if !filter.existsUnspentOutPoint(&input.PreviousOutPoint) {
					continue
				}
				if !added {
					transactions = append(
						transactions,
						txHexString(msgTx))
					added = true
				}
			}
		}
		// Scan outputs.
		for i, output := range msgTx.TxOut {
			_, addrs, _, err := txscript.ExtractPkScriptAddrs(
				output.PkScript, params)
			if err != nil {
				continue
			}
			for _, a := range addrs {
				if !filter.existsAddress(a) {
					continue
				}
				op := wire.OutPoint{
					Hash:  *tx.Hash(),
					Index: uint32(i),
				}
				filter.addUnspentOutPoint(&op)
				if !added {
					transactions = append(
						transactions,
						txHexString(msgTx))
					added = true
				}
			}
		}
	}
	filter.mu.Unlock()
	return transactions
}
// handleRescanBlocks implements the rescanblocks command extension for websocket connections. NOTE: This extension is ported from github.com/decred/dcrd
func handleRescanBlocks(wsc *wsClient, icmd interface{}) (interface{}, error) {
	cmd, ok := icmd.(*json.RescanBlocksCmd)
	if !ok {
		return nil, json.ErrRPCInternal
	}
	// Load client's transaction filter.  Must exist in order to continue.
	wsc.Lock()
	filter := wsc.filterData
	wsc.Unlock()
	if filter == nil {
		return nil, &json.RPCError{
			Code:    json.ErrRPCMisc,
			Message: "Transaction filter must be loaded before rescanning",
		}
	}
	blockHashes := make([]*chainhash.Hash, len(cmd.BlockHashes))
	for i := range cmd.BlockHashes {
		hash, err := chainhash.NewHashFromStr(cmd.BlockHashes[i])
		if err != nil {
			return nil, err
		}
		blockHashes[i] = hash
	}
	discoveredData := make([]json.RescannedBlock, 0, len(blockHashes))
	// Iterate over each block in the request and rescan.  When a block contains relevant transactions, add it to the response.
	bc := wsc.server.cfg.Chain
	params := wsc.server.cfg.ChainParams
	var lastBlockHash *chainhash.Hash
	for i := range blockHashes {
		block, err := bc.BlockByHash(blockHashes[i])
		if err != nil {
			return nil, &json.RPCError{
				Code:    json.ErrRPCBlockNotFound,
				Message: "Failed to fetch block: " + err.Error(),
			}
		}
		if lastBlockHash != nil && block.MsgBlock().Header.PrevBlock != *lastBlockHash {
			return nil, &json.RPCError{
				Code: json.ErrRPCInvalidParameter,
				Message: fmt.Sprintf("Block %v is not a child of %v",
					blockHashes[i], lastBlockHash),
			}
		}
		lastBlockHash = blockHashes[i]
		transactions := rescanBlockFilter(filter, block, params)
		if len(transactions) != 0 {
			discoveredData = append(discoveredData, json.RescannedBlock{
				Hash:         cmd.BlockHashes[i],
				Transactions: transactions,
			})
		}
	}
	return &discoveredData, nil
}
// recoverFromReorg attempts to recover from a detected reorganize during a rescan.  It fetches a new range of block shas from the database and verifies that the new range of blocks is on the same fork as a previous range of blocks.  If this condition does not hold true, the JSON-RPC error for an unrecoverable reorganize is returned.
func recoverFromReorg(chain *blockchain.BlockChain, minBlock, maxBlock int32,
	lastBlock *chainhash.Hash) ([]chainhash.Hash, error) {
	hashList, err := chain.HeightRange(minBlock, maxBlock)
	if err != nil {
		log <- cl.Error{"error looking up block range:", err}
		return nil, &json.RPCError{
			Code:    json.ErrRPCDatabase,
			Message: "Database error: " + err.Error(),
		}
	}
	if lastBlock == nil || len(hashList) == 0 {
		return hashList, nil
	}
	blk, err := chain.BlockByHash(&hashList[0])
	if err != nil {
		log <- cl.Error{"error looking up possibly reorged block:", err}
		return nil, &json.RPCError{
			Code:    json.ErrRPCDatabase,
			Message: "Database error: " + err.Error(),
		}
	}
	jsonErr := descendantBlock(lastBlock, blk)
	if jsonErr != nil {
		return nil, jsonErr
	}
	return hashList, nil
}
// descendantBlock returns the appropriate JSON-RPC error if a current block fetched during a reorganize is not a direct child of the parent block hash.
func descendantBlock(prevHash *chainhash.Hash, curBlock *util.Block) error {
	curHash := &curBlock.MsgBlock().Header.PrevBlock
	if !prevHash.IsEqual(curHash) {
		log <- cl.Errorf{
			"stopping rescan for reorged block %v (replaced by block %v)",
			prevHash, curHash,
		}
		return &ErrRescanReorg
	}
	return nil
}
// handleRescan implements the rescan command extension for websocket connections.
// NOTE: This does not smartly handle reorgs, and fixing requires database changes (for safe, concurrent access to full block ranges, and support for other chains than the best chain).  It will, however, detect whether a reorg removed a block that was previously processed, and result in the handler erroring.  Clients must handle this by finding a block still in the chain (perhaps from a rescanprogress notification) to resume their rescan.
func handleRescan(wsc *wsClient, icmd interface{}) (interface{}, error) {
	cmd, ok := icmd.(*json.RescanCmd)
	if !ok {
		return nil, json.ErrRPCInternal
	}
	outpoints := make([]*wire.OutPoint, 0, len(cmd.OutPoints))
	for i := range cmd.OutPoints {
		cmdOutpoint := &cmd.OutPoints[i]
		blockHash, err := chainhash.NewHashFromStr(cmdOutpoint.Hash)
		if err != nil {
			return nil, rpcDecodeHexError(cmdOutpoint.Hash)
		}
		outpoint := wire.NewOutPoint(blockHash, cmdOutpoint.Index)
		outpoints = append(outpoints, outpoint)
	}
	numAddrs := len(cmd.Addresses)
	if numAddrs == 1 {
		log <- cl.Inf("beginning rescan for 1 address")
	} else {
		log <- cl.Infof{"beginning rescan for %d addresses", numAddrs}
	}
	// Build lookup maps.
	lookups := rescanKeys{
		fallbacks:           map[string]struct{}{},
		pubKeyHashes:        map[[ripemd160.Size]byte]struct{}{},
		scriptHashes:        map[[ripemd160.Size]byte]struct{}{},
		compressedPubKeys:   map[[33]byte]struct{}{},
		uncompressedPubKeys: map[[65]byte]struct{}{},
		unspent:             map[wire.OutPoint]struct{}{},
	}
	var compressedPubkey [33]byte
	var uncompressedPubkey [65]byte
	params := wsc.server.cfg.ChainParams
	for _, addrStr := range cmd.Addresses {
		addr, err := util.DecodeAddress(addrStr, params)
		if err != nil {
			jsonErr := json.RPCError{
				Code: json.ErrRPCInvalidAddressOrKey,
				Message: "Rescan address " + addrStr + ": " +
					err.Error(),
			}
			return nil, &jsonErr
		}
		switch a := addr.(type) {
		case *util.AddressPubKeyHash:
			lookups.pubKeyHashes[*a.Hash160()] = struct{}{}
		case *util.AddressScriptHash:
			lookups.scriptHashes[*a.Hash160()] = struct{}{}
		case *util.AddressPubKey:
			pubkeyBytes := a.ScriptAddress()
			switch len(pubkeyBytes) {
			case 33: // Compressed
				copy(compressedPubkey[:], pubkeyBytes)
				lookups.compressedPubKeys[compressedPubkey] = struct{}{}
			case 65: // Uncompressed
				copy(uncompressedPubkey[:], pubkeyBytes)
				lookups.uncompressedPubKeys[uncompressedPubkey] = struct{}{}
			default:
				jsonErr := json.RPCError{
					Code:    json.ErrRPCInvalidAddressOrKey,
					Message: "Pubkey " + addrStr + " is of unknown length",
				}
				return nil, &jsonErr
			}
		default:
			// A new address type must have been added.  Use encoded payment address string as a fallback until a fast path is added.
			lookups.fallbacks[addrStr] = struct{}{}
		}
	}
	for _, outpoint := range outpoints {
		lookups.unspent[*outpoint] = struct{}{}
	}
	chain := wsc.server.cfg.Chain
	minBlockHash, err := chainhash.NewHashFromStr(cmd.BeginBlock)
	if err != nil {
		return nil, rpcDecodeHexError(cmd.BeginBlock)
	}
	minBlock, err := chain.BlockHeightByHash(minBlockHash)
	if err != nil {
		return nil, &json.RPCError{
			Code:    json.ErrRPCBlockNotFound,
			Message: "Error getting block: " + err.Error(),
		}
	}
	maxBlock := int32(math.MaxInt32)
	if cmd.EndBlock != nil {
		maxBlockHash, err := chainhash.NewHashFromStr(*cmd.EndBlock)
		if err != nil {
			return nil, rpcDecodeHexError(*cmd.EndBlock)
		}
		maxBlock, err = chain.BlockHeightByHash(maxBlockHash)
		if err != nil {
			return nil, &json.RPCError{
				Code:    json.ErrRPCBlockNotFound,
				Message: "Error getting block: " + err.Error(),
			}
		}
	}
	// lastBlock and lastBlockHash track the previously-rescanned block. They equal nil when no previous blocks have been rescanned.
	var lastBlock *util.Block
	var lastBlockHash *chainhash.Hash
	// A ticker is created to wait at least 10 seconds before notifying the websocket client of the current progress completed by the rescan.
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	// Instead of fetching all block shas at once, fetch in smaller chunks to ensure large rescans consume a limited amount of memory.
fetchRange:
	for minBlock < maxBlock {
		// Limit the max number of hashes to fetch at once to the maximum number of items allowed in a single inventory. This value could be higher since it's not creating inventory messages, but this mirrors the limiting logic used in the peer-to-peer protocol.
		maxLoopBlock := maxBlock
		if maxLoopBlock-minBlock > wire.MaxInvPerMsg {
			maxLoopBlock = minBlock + wire.MaxInvPerMsg
		}
		hashList, err := chain.HeightRange(minBlock, maxLoopBlock)
		if err != nil {
			log <- cl.Error{"error looking up block range:", err}
			return nil, &json.RPCError{
				Code:    json.ErrRPCDatabase,
				Message: "Database error: " + err.Error(),
			}
		}
		if len(hashList) == 0 {
			// The rescan is finished if no blocks hashes for this range were successfully fetched and a stop block was provided.
			if maxBlock != math.MaxInt32 {
				break
			}
			// If the rescan is through the current block, set up the client to continue to receive notifications regarding all rescanned addresses and the current set of unspent outputs.
			// This is done safely by temporarily grabbing exclusive access of the block manager.  If no more blocks have been attached between this pause and the fetch above, then it is safe to register the websocket client for continuous notifications if necessary.  Otherwise, continue the fetch loop again to rescan the new blocks (or error due to an irrecoverable reorganize).
			pauseGuard := wsc.server.cfg.SyncMgr.Pause()
			best := wsc.server.cfg.Chain.BestSnapshot()
			curHash := &best.Hash
			again := true
			if lastBlockHash == nil || *lastBlockHash == *curHash {
				again = false
				n := wsc.server.ntfnMgr
				n.RegisterSpentRequests(wsc, lookups.unspentSlice())
				n.RegisterTxOutAddressRequests(wsc, cmd.Addresses)
			}
			close(pauseGuard)
			if err != nil {
				log <- cl.Errorf{
					"Error fetching best block hash:", err,
				}
				return nil, &json.RPCError{
					Code:    json.ErrRPCDatabase,
					Message: "Database error: " + err.Error(),
				}
			}
			if again {
				continue
			}
			break
		}
	loopHashList:
		for i := range hashList {
			blk, err := chain.BlockByHash(&hashList[i])
			if err != nil {
				// Only handle reorgs if a block could not be found for the hash.
				if dbErr, ok := err.(database.Error); !ok ||
					dbErr.ErrorCode != database.ErrBlockNotFound {
					log <- cl.Error{"error looking up block:", err}
					return nil, &json.RPCError{
						Code: json.ErrRPCDatabase,
						Message: "Database error: " +
							err.Error(),
					}
				}
				// If an absolute max block was specified, don't attempt to handle the reorg.
				if maxBlock != math.MaxInt32 {
					log <- cl.Error{
						"stopping rescan for reorged block", cmd.EndBlock,
					}
					return nil, &ErrRescanReorg
				}
				// If the lookup for the previously valid block hash failed, there may have been a reorg. Fetch a new range of block hashes and verify that the previously processed block (if there was any) still exists in the database.  If it doesn't, we error.
				// A goto is used to branch executation back to before the range was evaluated, as it must be reevaluated for the new hashList.
				minBlock += int32(i)
				hashList, err = recoverFromReorg(chain,
					minBlock, maxBlock, lastBlockHash)
				if err != nil {
					return nil, err
				}
				if len(hashList) == 0 {
					break fetchRange
				}
				goto loopHashList
			}
			if i == 0 && lastBlockHash != nil {
				// Ensure the new hashList is on the same fork as the last block from the old hashList.
				jsonErr := descendantBlock(lastBlockHash, blk)
				if jsonErr != nil {
					return nil, jsonErr
				}
			}
			// A select statement is used to stop rescans if the client requesting the rescan has disconnected.
			select {
			case <-wsc.quit:
				// fmt.Println("chan:<-wsc.quit")
				log <- cl.Debugf{
					"stopped rescan at height %v for disconnected client",
					blk.Height(),
				}
				return nil, nil
			default:
				rescanBlock(wsc, &lookups, blk)
				lastBlock = blk
				lastBlockHash = blk.Hash()
			}
			// Periodically notify the client of the progress completed.  Continue with next block if no progress notification is needed yet.
			select {
			case <-ticker.C: // fallthrough
				// fmt.Println("chan:<-ticker.C")
			default:
				continue
			}
			n := json.NewRescanProgressNtfn(hashList[i].String(),
				blk.Height(), blk.MsgBlock().Header.Timestamp.Unix())
			mn, err := json.MarshalCmd(nil, n)
			if err != nil {
				log <- cl.Errorf{
					"failed to marshal rescan progress notification: %v", err,
				}
				continue
			}
			if err = wsc.QueueNotification(mn); err == ErrClientQuit {
				// Finished if the client disconnected.
				log <- cl.Debugf{
					"stopped rescan at height %v for disconnected client",
					blk.Height(),
				}
				return nil, nil
			}
		}
		minBlock += int32(len(hashList))
	}
	// Notify websocket client of the finished rescan.  Due to how pod asynchronously queues notifications to not block calling code, there is no guarantee that any of the notifications created during rescan (such as rescanprogress, recvtx and redeemingtx) will be received before the rescan RPC returns.  Therefore, another method is needed to safely inform clients that all rescan notifications have been sent.
	n := json.NewRescanFinishedNtfn(lastBlockHash.String(),
		lastBlock.Height(),
		lastBlock.MsgBlock().Header.Timestamp.Unix())
	if mn, err := json.MarshalCmd(nil, n); err != nil {
		log <- cl.Errorf{
			"failed to marshal rescan finished notification: %v", err,
		}
	} else {
		// The rescan is finished, so we don't care whether the client has disconnected at this point, so discard error.
		_ = wsc.QueueNotification(mn)
	}
	log <- cl.Info{"finished rescan"}
	return nil, nil
}
func init() {
	wsHandlers = wsHandlersBeforeInit
}
package node
var samplePodConf = []byte(`[Application Options]
;datadir=              ;;; Directory to store data (default: /home/loki/.pod/data)
;logdir=               ;;; Directory to log output. (default: /home/loki/.pod/logs)
;addpeer=              ;;; Add a peer to connect with at startup
;connect=              ;;; Connect only to the specified peers at startup
;nolisten              ;;; Disable listening for incoming connections -- NOTE: Listening is automatically disabled if the --connect or --proxy options are used without also specifying listen interfaces via --listen
;listen=               ;;; Add an interface/port to listen for connections (default all interfaces port: 11047, testnet: 21047)
;maxpeers=             ;;; Max number of inbound and outbound peers (default: 125)
;nobanning             ;;; Disable banning of misbehaving peers
;banduration=          ;;; How long to ban misbehaving peers.  Valid time units are {s, m, h}.  Minimum 1 second (default: 24h0m0s)
;banthreshold=         ;;; Maximum allowed ban score before disconnecting and banning misbehaving peers. (default: 100)
;whitelist=            ;;; Add an IP network or IP that will not be banned. (eg. 192.168.1.0/24 or ::1)
;rpcuser=              ;;; Username for RPC connections
;rpcpass=              ;;; Password for RPC connections
;rpclimituser=         ;;; Username for limited RPC connections
;rpclimitpass=         ;;; Password for limited RPC connections
;rpclisten=            ;;; Add an interface/port to listen for RPC connections (default port: 11048, testnet: 21048) gives sha256d block templates
;blake14lrlisten=      ;;; Additional RPC port that delivers blake14lr versioned block templates
;cryptonight7v2=       ;;; Additional RPC port that delivers cryptonight7v2 versioned block templates
;keccaklisten=         ;;; Additional RPC port that delivers keccak versioned block templates
;lyra2rev2listen=      ;;; Additional RPC port that delivers lyra2rev2 versioned block templates
;scryptlisten=         ;;; Additional RPC port that delivers scrypt versioned block templates
;striboglisten=        ;;; Additional RPC port that delivers stribog versioned block templates
;skeinlisten=          ;;; Additional RPC port that delivers skein versioned block templates
;x11listen=            ;;; Additional RPC port that delivers x11 versioned block templates
;rpccert=              ;;; File containing the certificate file (default: /home/loki/.pod/rpc.cert)
;rpckey=               ;;; File containing the certificate key (default: /home/loki/.pod/rpc.key)
;rpcmaxclients=        ;;; Max number of RPC clients for standard connections (default: 10)
;rpcmaxwebsockets=     ;;; Max number of RPC websocket connections (default: 25)
;rpcmaxconcurrentreqs= ;;; Max number of concurrent RPC requests that may be processed concurrently (default: 20)
;rpcquirks             ;;; Mirror some JSON-RPC quirks of Bitcoin Core -- NOTE: Discouraged unless interoperability issues need to be worked around
;norpc                 ;;; Disable built-in RPC server -- NOTE: The RPC server is disabled by default if no rpcuser/rpcpass or rpclimituser/rpclimitpass is specified
;tls                   ;;; Enable TLS for the RPC server
;nodnsseed             ;;; Disable DNS seeding for peers
;externalip=           ;;; Add an ip to the list of local addresses we claim to listen on to peers
;proxy=                ;;; Connect via SOCKS5 proxy (eg. 127.0.0.1:9050)
;proxyuser=            ;;; Username for proxy server
;proxypass=            ;;; Password for proxy server
;onion=                ;;; Connect to tor hidden services via SOCKS5 proxy (eg. 127.0.0.1:9050)
;onionuser=            ;;; Username for onion proxy server
;onionpass=            ;;; Password for onion proxy server
;noonion               ;;; Disable connecting to tor hidden services
;torisolation          ;;; Enable Tor stream isolation by randomizing user credentials for each connection.
;testnet               ;;; Use the test network
;regtest               ;;; Use the regression test network
;simnet                ;;; Use the simulation test network
;addcheckpoint=        ;;; Add a custom checkpoint.  Format: '<height>:<hash>'
;nocheckpoints         ;;; Disable built-in checkpoints.  Don't do this unless you know what you're doing.
;dbtype=               ;;; Database backend to use for the Block Chain (default: ffldb)
;profile=              ;;; Enable HTTP profiling on given port -- NOTE port must be between 1024 and 65536
;cpuprofile=           ;;; Write CPU profile to the specified file
;debuglevel=           ;;; Logging level for all subsystems {trace, debug, info, warn, error, critical} -- You may also specify <subsystem>=<level>,<subsystem2>=<level>,... to set the log level for individual subsystems -- Use show to list available subsystems (default: info)
;upnp                  ;;; Use UPnP to map our listening port outside of NAT
;minrelaytxfee=        ;;; The minimum transaction fee in DUO/kB to be considered a non-zero fee. (default: 1e-05)
;limitfreerelay=       ;;; Limit relay of transactions with no transaction fee to the given amount in thousands of bytes per minute (default: 15)
;norelaypriority       ;;; Do not require free or low-fee transactions to have high priority for relaying
;trickleinterval=      ;;; Minimum time between attempts to send new inventory to a connected peer (default: 10s)
;maxorphantx=          ;;; Max number of orphan transactions to keep in memory (default: 100)
;algo=                 ;;; Sets the algorithm for the CPU miner ( blake14lr, blake2b, keccak, lyra2rev2, scrypt, skein, x11, x13, sha256d, scrypt default sha256d) (default: sha256d)
;generate              ;;; Generate (mine) bitcoins using the CPU
;genthreads=           ;;; Number of CPU threads to use with CPU miner -1 = all cores (default: 1)
;miningaddr=           ;;; Add the specified payment address to the list of addresses to use for generated blocks -- At least one address is required if the generate option is set
;blockminsize=         ;;; Mininum block size in bytes to be used when creating a block (default: 80)
;blockmaxsize=         ;;; Maximum block size in bytes to be used when creating a block (default: 200000)
;blockminweight=       ;;; Mininum block weight to be used when creating a block (default: 10)
;blockmaxweight=       ;;; Maximum block weight to be used when creating a block (default: 3000000)
;blockprioritysize=    ;;; Size in bytes for high-priority/low-fee transactions when creating a block (default: 50000)
;uacomment=            ;;; Comment to add to the user agent -- See BIP 14 for more information.
;nopeerbloomfilters    ;;; Disable bloom filtering support
;nocfilters            ;;; Disable committed filtering (CF) support
;dropcfindex           ;;; Deletes the index used for committed filtering (CF) support from the database on start up and then exits.
;sigcachemaxsize=      ;;; The maximum number of entries in the signature verification cache (default: 100000)
;blocksonly            ;;; Do not accept transactions from remote peers.
;txindex               ;;; Maintain a full hash-based transaction index which makes all transactions available via the getrawtransaction RPC
;droptxindex           ;;; Deletes the hash-based transaction index from the database on start up and then exits.
;addrindex             ;;; Maintain a full address-based transaction index which makes the searchrawtransactions RPC available
;dropaddrindex         ;;; Deletes the address-based transaction index from the database on start up and then exits.
;relaynonstd           ;;; Relay non-standard transactions regardless of the default settings for the active network.
;rejectnonstd          ;;; Reject non-standard transactions regardless of the default settings for the active network.
`)
package node
import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"net"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"git.parallelcoin.io/pod/cmd/node/mempool"
	"git.parallelcoin.io/pod/pkg/addrmgr"
	blockchain "git.parallelcoin.io/pod/pkg/chain"
	indexers "git.parallelcoin.io/pod/pkg/chain/index"
	"git.parallelcoin.io/pod/pkg/chaincfg"
	"git.parallelcoin.io/pod/pkg/chaincfg/chainhash"
	cl "git.parallelcoin.io/pod/pkg/clog"
	"git.parallelcoin.io/pod/pkg/connmgr"
	database "git.parallelcoin.io/pod/pkg/db"
	"git.parallelcoin.io/pod/pkg/interrupt"
	"git.parallelcoin.io/pod/pkg/mining"
	"git.parallelcoin.io/pod/pkg/mining/cpuminer"
	controller "git.parallelcoin.io/pod/pkg/mining/dispatch"
	"git.parallelcoin.io/pod/pkg/netsync"
	"git.parallelcoin.io/pod/pkg/peer"
	"git.parallelcoin.io/pod/pkg/txscript"
	"git.parallelcoin.io/pod/pkg/util"
	"git.parallelcoin.io/pod/pkg/util/bloom"
	"git.parallelcoin.io/pod/pkg/wire"
)
const (
	// defaultServices describes the default services that are supported by the server.
	defaultServices = wire.SFNodeNetwork | wire.SFNodeBloom |
		wire.SFNodeWitness | wire.SFNodeCF
	// defaultRequiredServices describes the default services that are required to be supported by outbound peers.
	defaultRequiredServices = wire.SFNodeNetwork
	// defaultTargetOutbound is the default number of outbound peers to target.
	defaultTargetOutbound = 9
	// connectionRetryInterval is the base amount of time to wait in between retries when connecting to persistent peers.  It is adjusted by the number of retries such that there is a retry backoff.
	connectionRetryInterval = time.Second * 9
)
var (
	// userAgentName is the user agent name and is used to help identify ourselves to other bitcoin peers.
	userAgentName = "pod"
	// userAgentVersion is the user agent version and is used to help identify ourselves to other bitcoin peers.
	userAgentVersion = fmt.Sprintf("%d.%d.%d", appMajor, appMinor, appPatch)
)
// zeroHash is the zero value hash (all zeros).  It is defined as a convenience.
var zeroHash chainhash.Hash
// onionAddr implements the net.Addr interface and represents a tor address.
type onionAddr struct {
	addr string
}
// String returns the onion address. This is part of the net.Addr interface.
func (oa *onionAddr) String() string {
	return oa.addr
}
// Network returns "onion". This is part of the net.Addr interface.
func (oa *onionAddr) Network() string {
	return "onion"
}
// Ensure onionAddr implements the net.Addr interface.
var _ net.Addr = (*onionAddr)(nil)
// simpleAddr implements the net.Addr interface with two struct fields
type simpleAddr struct {
	net, addr string
}
// String returns the address. This is part of the net.Addr interface.
func (a simpleAddr) String() string {
	return a.addr
}
// Network returns the network. This is part of the net.Addr interface.
func (a simpleAddr) Network() string {
	return a.net
}
// Ensure simpleAddr implements the net.Addr interface.
var _ net.Addr = simpleAddr{}
// broadcastMsg provides the ability to house a bitcoin message to be broadcast to all connected peers except specified excluded peers.
type broadcastMsg struct {
	message      wire.Message
	excludePeers []*serverPeer
}
// broadcastInventoryAdd is a type used to declare that the InvVect it contains needs to be added to the rebroadcast map
type broadcastInventoryAdd relayMsg
// broadcastInventoryDel is a type used to declare that the InvVect it contains needs to be removed from the rebroadcast map
type broadcastInventoryDel *wire.InvVect
// relayMsg packages an inventory vector along with the newly discovered inventory so the relay has access to that information.
type relayMsg struct {
	invVect *wire.InvVect
	data    interface{}
}
// updatePeerHeightsMsg is a message sent from the blockmanager to the server after a new block has been accepted. The purpose of the message is to update the heights of peers that were known to announce the block before we connected it to the main chain or recognized it as an orphan. With these updates, peer heights will be kept up to date, allowing for fresh data when selecting sync peer candidacy.
type updatePeerHeightsMsg struct {
	newHash    *chainhash.Hash
	newHeight  int32
	originPeer *peer.Peer
}
// peerState maintains state of inbound, persistent, outbound peers as well as banned peers and outbound groups.
type peerState struct {
	inboundPeers    map[int32]*serverPeer
	outboundPeers   map[int32]*serverPeer
	persistentPeers map[int32]*serverPeer
	banned          map[string]time.Time
	outboundGroups  map[string]int
}
// Count returns the count of all known peers.
func (ps *peerState) Count() int {
	return len(ps.inboundPeers) + len(ps.outboundPeers) +
		len(ps.persistentPeers)
}
// forAllOutboundPeers is a helper function that runs closure on all outbound peers known to peerState.
func (ps *peerState) forAllOutboundPeers(closure func(sp *serverPeer)) {
	for _, e := range ps.outboundPeers {
		closure(e)
	}
	for _, e := range ps.persistentPeers {
		closure(e)
	}
}
// forAllPeers is a helper function that runs closure on all peers known to peerState.
func (ps *peerState) forAllPeers(closure func(sp *serverPeer)) {
	for _, e := range ps.inboundPeers {
		closure(e)
	}
	ps.forAllOutboundPeers(closure)
}
// cfHeaderKV is a tuple of a filter header and its associated block hash. The struct is used to cache cfcheckpt responses.
type cfHeaderKV struct {
	blockHash    chainhash.Hash
	filterHeader chainhash.Hash
}
// server provides a bitcoin server for handling communications to and from bitcoin peers.
type server struct {
	// The following variables must only be used atomically. Putting the uint64s first makes them 64-bit aligned for 32-bit systems.
	bytesReceived        uint64 // Total bytes received from all peers since start.
	bytesSent            uint64 // Total bytes sent by all peers since start.
	started              int32
	shutdown             int32
	shutdownSched        int32
	startupTime          int64
	chainParams          *chaincfg.Params
	addrManager          *addrmgr.AddrManager
	connManager          *connmgr.ConnManager
	sigCache             *txscript.SigCache
	hashCache            *txscript.HashCache
	rpcServers           []*rpcServer
	syncManager          *netsync.SyncManager
	chain                *blockchain.BlockChain
	txMemPool            *mempool.TxPool
	cpuMiner             *cpuminer.CPUMiner
	minerController      *controller.Controller
	modifyRebroadcastInv chan interface{}
	newPeers             chan *serverPeer
	donePeers            chan *serverPeer
	banPeers             chan *serverPeer
	query                chan interface{}
	relayInv             chan relayMsg
	broadcast            chan broadcastMsg
	peerHeightsUpdate    chan updatePeerHeightsMsg
	wg                   sync.WaitGroup
	quit                 chan struct{}
	nat                  NAT
	db                   database.DB
	timeSource           blockchain.MedianTimeSource
	services             wire.ServiceFlag
	// The following fields are used for optional indexes.  They will be nil if the associated index is not enabled.  These fields are set during initial creation of the server and never changed afterwards, so they do not need to be protected for concurrent access.
	txIndex   *indexers.TxIndex
	addrIndex *indexers.AddrIndex
	cfIndex   *indexers.CfIndex
	// The fee estimator keeps track of how long transactions are left in the mempool before they are mined into blocks.
	feeEstimator *mempool.FeeEstimator
	// cfCheckptCaches stores a cached slice of filter headers for cfcheckpt messages for each filter type.
	cfCheckptCaches    map[wire.FilterType][]cfHeaderKV
	cfCheckptCachesMtx sync.RWMutex
	algo               string
	numthreads         uint32
}
// serverPeer extends the peer to maintain state shared by the server and the blockmanager.
type serverPeer struct {
	// The following variables must only be used atomically
	feeFilter int64
	*peer.Peer
	connReq        *connmgr.ConnReq
	server         *server
	persistent     bool
	continueHash   *chainhash.Hash
	relayMtx       sync.Mutex
	disableRelayTx bool
	sentAddrs      bool
	isWhitelisted  bool
	filter         *bloom.Filter
	knownAddresses map[string]struct{}
	banScore       connmgr.DynamicBanScore
	quit           chan struct{}
	// The following chans are used to sync blockmanager and server.
	txProcessed    chan struct{}
	blockProcessed chan struct{}
}
// newServerPeer returns a new serverPeer instance. The peer needs to be set by the caller.
func newServerPeer(s *server, isPersistent bool) *serverPeer {
	return &serverPeer{
		server:         s,
		persistent:     isPersistent,
		filter:         bloom.LoadFilter(nil),
		knownAddresses: make(map[string]struct{}),
		quit:           make(chan struct{}),
		txProcessed:    make(chan struct{}, 1),
		blockProcessed: make(chan struct{}, 1),
	}
}
// newestBlock returns the current best block hash and height using the format required by the configuration for the peer package.
func (sp *serverPeer) newestBlock() (*chainhash.Hash, int32, error) {
	best := sp.server.chain.BestSnapshot()
	return &best.Hash, best.Height, nil
}
// addKnownAddresses adds the given addresses to the set of known addresses to the peer to prevent sending duplicate addresses.
func (sp *serverPeer) addKnownAddresses(addresses []*wire.NetAddress) {
	for _, na := range addresses {
		sp.knownAddresses[addrmgr.NetAddressKey(na)] = struct{}{}
	}
}
// addressKnown true if the given address is already known to the peer.
func (sp *serverPeer) addressKnown(na *wire.NetAddress) bool {
	_, exists := sp.knownAddresses[addrmgr.NetAddressKey(na)]
	return exists
}
// setDisableRelayTx toggles relaying of transactions for the given peer. It is safe for concurrent access.
func (sp *serverPeer) setDisableRelayTx(disable bool) {
	sp.relayMtx.Lock()
	sp.disableRelayTx = disable
	sp.relayMtx.Unlock()
}
// relayTxDisabled returns whether or not relaying of transactions for the given peer is disabled. It is safe for concurrent access.
func (sp *serverPeer) relayTxDisabled() bool {
	sp.relayMtx.Lock()
	isDisabled := sp.disableRelayTx
	sp.relayMtx.Unlock()
	return isDisabled
}
// pushAddrMsg sends an addr message to the connected peer using the provided addresses.
func (sp *serverPeer) pushAddrMsg(addresses []*wire.NetAddress) {
	// Filter addresses already known to the peer.
	addrs := make([]*wire.NetAddress, 0, len(addresses))
	for _, addr := range addresses {
		if !sp.addressKnown(addr) {
			addrs = append(addrs, addr)
		}
	}
	known, err := sp.PushAddrMsg(addrs)
	if err != nil {
		log <- cl.Errorf{
			"can't push address message to %s: %v", sp.Peer, err,
		}
		sp.Disconnect()
		return
	}
	sp.addKnownAddresses(known)
}
// addBanScore increases the persistent and decaying ban score fields by the values passed as parameters. If the resulting score exceeds half of the ban threshold, a warning is logged including the reason provided. Further, if the score is above the ban threshold, the peer will be banned and disconnected.
func (sp *serverPeer) addBanScore(persistent, transient uint32, reason string) {
	// No warning is logged and no score is calculated if banning is disabled.
	if cfg.DisableBanning {
		return
	}
	if sp.isWhitelisted {
		log <- cl.Debugf{
			"misbehaving whitelisted peer %s: %s", sp, reason,
		}
		return
	}
	warnThreshold := cfg.BanThreshold >> 1
	if transient == 0 && persistent == 0 {
		// The score is not being increased, but a warning message is still logged if the score is above the warn threshold.
		score := sp.banScore.Int()
		if score > warnThreshold {
			log <- cl.Warnf{
				"misbehaving peer %s: %s -- ban score is %d, it was not increased this time",
				sp, reason, score,
			}
		}
		return
	}
	score := sp.banScore.Increase(persistent, transient)
	if score > warnThreshold {
		log <- cl.Warnf{
			"misbehaving peer %s: %s -- ban score increased to %d",
			sp, reason, score,
		}
		if score > cfg.BanThreshold {
			log <- cl.Warnf{
				"misbehaving peer %s -- banning and disconnecting", sp,
			}
			sp.server.BanPeer(sp)
			sp.Disconnect()
		}
	}
}
// hasServices returns whether or not the provided advertised service flags have all of the provided desired service flags set.
func hasServices(advertised, desired wire.ServiceFlag) bool {
	return advertised&desired == desired
}
// OnVersion is invoked when a peer receives a version bitcoin message and is used to negotiate the protocol version details as well as kick start the communications.
func (sp *serverPeer) OnVersion(_ *peer.Peer, msg *wire.MsgVersion) *wire.MsgReject {
	// Update the address manager with the advertised services for outbound connections in case they have changed.  This is not done for inbound connections to help prevent malicious behavior and is skipped when running on the simulation test network since it is only intended to connect to specified peers and actively avoids advertising and connecting to discovered peers. NOTE: This is done before rejecting peers that are too old to ensure it is updated regardless in the case a new minimum protocol version is enforced and the remote node has not upgraded yet.
	isInbound := sp.Inbound()
	remoteAddr := sp.NA()
	addrManager := sp.server.addrManager
	if !cfg.SimNet && !isInbound {
		addrManager.SetServices(remoteAddr, msg.Services)
	}
	// Ignore peers that have a protcol version that is too old.  The peer negotiation logic will disconnect it after this callback returns.
	if msg.ProtocolVersion < int32(peer.MinAcceptableProtocolVersion) {
		return nil
	}
	// Reject outbound peers that are not full nodes.
	wantServices := wire.SFNodeNetwork
	if !isInbound && !hasServices(msg.Services, wantServices) {
		missingServices := wantServices & ^msg.Services
		log <- cl.Debugf{
			"rejecting peer %s with services %v due to not providing desired services %v",
			sp.Peer, msg.Services, missingServices,
		}
		reason := fmt.Sprintf("required services %#x not offered",
			uint64(missingServices))
		return wire.NewMsgReject(msg.Command(), wire.RejectNonstandard, reason)
	}
	// Update the address manager and request known addresses from the remote peer for outbound connections.  This is skipped when running on the simulation test network since it is only intended to connect to specified peers and actively avoids advertising and connecting to discovered peers.
	if !cfg.SimNet && !isInbound {
		// After soft-fork activation, only make outbound connection to peers if they flag that they're segwit enabled.
		chain := sp.server.chain
		segwitActive, err := chain.IsDeploymentActive(chaincfg.DeploymentSegwit)
		if err != nil {
			log <- cl.Error{
				"unable to query for segwit soft-fork state:", err,
			}
			return nil
		}
		if segwitActive && !sp.IsWitnessEnabled() {
			log <- cl.Info{
				"disconnecting non-segwit peer", sp,
				"as it isn't segwit enabled and we need more segwit enabled peers",
			}
			sp.Disconnect()
			return nil
		}
		// Advertise the local address when the server accepts incoming connections and it believes itself to be close to the best known tip.
		if !cfg.DisableListen && sp.server.syncManager.IsCurrent() {
			// Get address that best matches.
			lna := addrManager.GetBestLocalAddress(remoteAddr)
			if addrmgr.IsRoutable(lna) {
				// Filter addresses the peer already knows about.
				addresses := []*wire.NetAddress{lna}
				sp.pushAddrMsg(addresses)
			}
		}
		// Request known addresses if the server address manager needs more and the peer has a protocol version new enough to include a timestamp with addresses.
		hasTimestamp := sp.ProtocolVersion() >= wire.NetAddressTimeVersion
		if addrManager.NeedMoreAddresses() && hasTimestamp {
			sp.QueueMessage(wire.NewMsgGetAddr(), nil)
		}
		// Mark the address as a known good address.
		addrManager.Good(remoteAddr)
	}
	// Add the remote peer time as a sample for creating an offset against the local clock to keep the network time in sync.
	sp.server.timeSource.AddTimeSample(sp.Addr(), msg.Timestamp)
	// Signal the sync manager this peer is a new sync candidate.
	sp.server.syncManager.NewPeer(sp.Peer)
	// Choose whether or not to relay transactions before a filter command is received.
	sp.setDisableRelayTx(msg.DisableRelayTx)
	// Add valid peer to the server.
	sp.server.AddPeer(sp)
	return nil
}
// OnMemPool is invoked when a peer receives a mempool bitcoin message. It creates and sends an inventory message with the contents of the memory pool up to the maximum inventory allowed per message.  When the peer has a bloom filter loaded, the contents are filtered accordingly.
func (sp *serverPeer) OnMemPool(_ *peer.Peer, msg *wire.MsgMemPool) {
	// Only allow mempool requests if the server has bloom filtering enabled.
	if sp.server.services&wire.SFNodeBloom != wire.SFNodeBloom {
		log <- cl.Debugf{
			"peer", sp, "sent mempool request with bloom filtering disabled -- disconnecting",
		}
		sp.Disconnect()
		return
	}
	// A decaying ban score increase is applied to prevent flooding. The ban score accumulates and passes the ban threshold if a burst of mempool messages comes from a peer. The score decays each minute to half of its value.
	sp.addBanScore(0, 33, "mempool")
	// Generate inventory message with the available transactions in the transaction memory pool.  Limit it to the max allowed inventory per message.  The NewMsgInvSizeHint function automatically limits the passed hint to the maximum allowed, so it's safe to pass it without double checking it here.
	txMemPool := sp.server.txMemPool
	txDescs := txMemPool.TxDescs()
	invMsg := wire.NewMsgInvSizeHint(uint(len(txDescs)))
	for _, txDesc := range txDescs {
		// Either add all transactions when there is no bloom filter, or only the transactions that match the filter when there is one.
		if !sp.filter.IsLoaded() || sp.filter.MatchTxAndUpdate(txDesc.Tx) {
			iv := wire.NewInvVect(wire.InvTypeTx, txDesc.Tx.Hash())
			invMsg.AddInvVect(iv)
			if len(invMsg.InvList)+1 > wire.MaxInvPerMsg {
				break
			}
		}
	}
	// Send the inventory message if there is anything to send.
	if len(invMsg.InvList) > 0 {
		sp.QueueMessage(invMsg, nil)
	}
}
// OnTx is invoked when a peer receives a tx bitcoin message.  It blocks until the bitcoin transaction has been fully processed.  Unlock the block handler this does not serialize all transactions through a single thread transactions don't rely on the previous one in a linear fashion like blocks.
func (sp *serverPeer) OnTx(_ *peer.Peer, msg *wire.MsgTx) {
	if cfg.BlocksOnly {
		log <- cl.Tracef{
			"ignoring tx %v from %v - blocksonly enabled",
			msg.TxHash(), sp,
		}
		return
	}
	// Add the transaction to the known inventory for the peer. Convert the raw MsgTx to a util.Tx which provides some convenience methods and things such as hash caching.
	tx := util.NewTx(msg)
	iv := wire.NewInvVect(wire.InvTypeTx, tx.Hash())
	sp.AddKnownInventory(iv)
	// Queue the transaction up to be handled by the sync manager and intentionally block further receives until the transaction is fully processed and known good or bad.  This helps prevent a malicious peer from queuing up a bunch of bad transactions before disconnecting (or being disconnected) and wasting memory.
	sp.server.syncManager.QueueTx(tx, sp.Peer, sp.txProcessed)
	<-sp.txProcessed
}
// OnBlock is invoked when a peer receives a block bitcoin message.  It blocks until the bitcoin block has been fully processed.
func (sp *serverPeer) OnBlock(_ *peer.Peer, msg *wire.MsgBlock, buf []byte) {
	// Convert the raw MsgBlock to a util.Block which provides some convenience methods and things such as hash caching.
	block := util.NewBlockFromBlockAndBytes(msg, buf)
	// Add the block to the known inventory for the peer.
	iv := wire.NewInvVect(wire.InvTypeBlock, block.Hash())
	sp.AddKnownInventory(iv)
	// Queue the block up to be handled by the block manager and intentionally block further receives until the bitcoin block is fully processed and known good or bad.  This helps prevent a malicious peer from queuing up a bunch of bad blocks before disconnecting (or being disconnected) and wasting memory.  Additionally, this behavior is depended on by at least the block acceptance test tool as the reference implementation processes blocks in the same thread and therefore blocks further messages until the bitcoin block has been fully processed.
	sp.server.syncManager.QueueBlock(block, sp.Peer, sp.blockProcessed)
	<-sp.blockProcessed
}
// OnInv is invoked when a peer receives an inv bitcoin message and is used to examine the inventory being advertised by the remote peer and react accordingly.  We pass the message down to blockmanager which will call QueueMessage with any appropriate responses.
func (sp *serverPeer) OnInv(_ *peer.Peer, msg *wire.MsgInv) {
	if !cfg.BlocksOnly {
		if len(msg.InvList) > 0 {
			sp.server.syncManager.QueueInv(msg, sp.Peer)
		}
		return
	}
	newInv := wire.NewMsgInvSizeHint(uint(len(msg.InvList)))
	for _, invVect := range msg.InvList {
		if invVect.Type == wire.InvTypeTx {
			log <- cl.Tracef{
				"ignoring tx %v in inv from %v -- blocksonly enabled",
				invVect.Hash, sp,
			}
			if sp.ProtocolVersion() >= wire.BIP0037Version {
				log <- cl.Infof{
					"peer %v is announcing transactions -- disconnecting", sp,
				}
				sp.Disconnect()
				return
			}
			continue
		}
		err := newInv.AddInvVect(invVect)
		if err != nil {
			log <- cl.Error{"failed to add inventory vector:", err}
			break
		}
	}
	if len(newInv.InvList) > 0 {
		sp.server.syncManager.QueueInv(newInv, sp.Peer)
	}
}
// OnHeaders is invoked when a peer receives a headers bitcoin message.  The message is passed down to the sync manager.
func (sp *serverPeer) OnHeaders(_ *peer.Peer, msg *wire.MsgHeaders) {
	sp.server.syncManager.QueueHeaders(msg, sp.Peer)
}
// handleGetData is invoked when a peer receives a getdata bitcoin message and is used to deliver block and transaction information.
func (sp *serverPeer) OnGetData(_ *peer.Peer, msg *wire.MsgGetData) {
	numAdded := 0
	notFound := wire.NewMsgNotFound()
	length := len(msg.InvList)
	// A decaying ban score increase is applied to prevent exhausting resources with unusually large inventory queries. Requesting more than the maximum inventory vector length within a short period of time yields a score above the default ban threshold. Sustained bursts of small requests are not penalized as that would potentially ban peers performing IBD. This incremental score decays each minute to half of its value.
	sp.addBanScore(0, uint32(length)*99/wire.MaxInvPerMsg, "getdata")
	// We wait on this wait channel periodically to prevent queuing far more data than we can send in a reasonable time, wasting memory. The waiting occurs after the database fetch for the next one to provide a little pipelining.
	var waitChan chan struct{}
	doneChan := make(chan struct{}, 1)
	for i, iv := range msg.InvList {
		var c chan struct{}
		// If this will be the last message we send.
		if i == length-1 && len(notFound.InvList) == 0 {
			c = doneChan
		} else if (i+1)%3 == 0 {
			// Buffered so as to not make the send goroutine block.
			c = make(chan struct{}, 1)
		}
		var err error
		switch iv.Type {
		case wire.InvTypeWitnessTx:
			err = sp.server.pushTxMsg(sp, &iv.Hash, c, waitChan, wire.WitnessEncoding)
		case wire.InvTypeTx:
			err = sp.server.pushTxMsg(sp, &iv.Hash, c, waitChan, wire.BaseEncoding)
		case wire.InvTypeWitnessBlock:
			err = sp.server.pushBlockMsg(sp, &iv.Hash, c, waitChan, wire.WitnessEncoding)
		case wire.InvTypeBlock:
			err = sp.server.pushBlockMsg(sp, &iv.Hash, c, waitChan, wire.BaseEncoding)
		case wire.InvTypeFilteredWitnessBlock:
			err = sp.server.pushMerkleBlockMsg(sp, &iv.Hash, c, waitChan, wire.WitnessEncoding)
		case wire.InvTypeFilteredBlock:
			err = sp.server.pushMerkleBlockMsg(sp, &iv.Hash, c, waitChan, wire.BaseEncoding)
		default:
			log <- cl.Warn{"unknown type in inventory request", iv.Type}
			continue
		}
		if err != nil {
			notFound.AddInvVect(iv)
			// When there is a failure fetching the final entry and the done channel was sent in due to there being no outstanding not found inventory, consume it here because there is now not found inventory that will use the channel momentarily.
			if i == len(msg.InvList)-1 && c != nil {
				<-c
			}
		}
		numAdded++
		waitChan = c
	}
	if len(notFound.InvList) != 0 {
		sp.QueueMessage(notFound, doneChan)
	}
	// Wait for messages to be sent. We can send quite a lot of data at this point and this will keep the peer busy for a decent amount of time. We don't process anything else by them in this time so that we have an idea of when we should hear back from them - else the idle timeout could fire when we were only half done sending the blocks.
	if numAdded > 0 {
		<-doneChan
	}
}
// OnGetBlocks is invoked when a peer receives a getblocks bitcoin message.
func (sp *serverPeer) OnGetBlocks(_ *peer.Peer, msg *wire.MsgGetBlocks) {
	// Find the most recent known block in the best chain based on the block locator and fetch all of the block hashes after it until either wire.MaxBlocksPerMsg have been fetched or the provided stop hash is encountered. Use the block after the genesis block if no other blocks in the provided locator are known.  This does mean the client will start over with the genesis block if unknown block locators are provided. This mirrors the behavior in the reference implementation.
	chain := sp.server.chain
	hashList := chain.LocateBlocks(msg.BlockLocatorHashes, &msg.HashStop,
		wire.MaxBlocksPerMsg)
	// Generate inventory message.
	invMsg := wire.NewMsgInv()
	for i := range hashList {
		iv := wire.NewInvVect(wire.InvTypeBlock, &hashList[i])
		invMsg.AddInvVect(iv)
	}
	// Send the inventory message if there is anything to send.
	if len(invMsg.InvList) > 0 {
		invListLen := len(invMsg.InvList)
		if invListLen == wire.MaxBlocksPerMsg {
			// Intentionally use a copy of the final hash so there is not a reference into the inventory slice which would prevent the entire slice from being eligible for GC as soon as it's sent.
			continueHash := invMsg.InvList[invListLen-1].Hash
			sp.continueHash = &continueHash
		}
		sp.QueueMessage(invMsg, nil)
	}
}
// OnGetHeaders is invoked when a peer receives a getheaders bitcoin message.
func (sp *serverPeer) OnGetHeaders(_ *peer.Peer, msg *wire.MsgGetHeaders) {
	// Ignore getheaders requests if not in sync.
	if !sp.server.syncManager.IsCurrent() {
		return
	}
	// Find the most recent known block in the best chain based on the block locator and fetch all of the headers after it until either wire.MaxBlockHeadersPerMsg have been fetched or the provided stop hash is encountered. Use the block after the genesis block if no other blocks in the provided locator are known.  This does mean the client will start over with the genesis block if unknown block locators are provided. This mirrors the behavior in the reference implementation.
	chain := sp.server.chain
	headers := chain.LocateHeaders(msg.BlockLocatorHashes, &msg.HashStop)
	// Send found headers to the requesting peer.
	blockHeaders := make([]*wire.BlockHeader, len(headers))
	for i := range headers {
		blockHeaders[i] = &headers[i]
	}
	sp.QueueMessage(&wire.MsgHeaders{Headers: blockHeaders}, nil)
}
// OnGetCFilters is invoked when a peer receives a getcfilters bitcoin message.
func (sp *serverPeer) OnGetCFilters(_ *peer.Peer, msg *wire.MsgGetCFilters) {
	// Ignore getcfilters requests if not in sync.
	if !sp.server.syncManager.IsCurrent() {
		return
	}
	// We'll also ensure that the remote party is requesting a set of filters that we actually currently maintain.
	switch msg.FilterType {
	case wire.GCSFilterRegular:
		break
	default:
		log <- cl.Debug{"filter request for unknown filter:", msg.FilterType}
		return
	}
	hashes, err := sp.server.chain.HeightToHashRange(
		int32(msg.StartHeight), &msg.StopHash, wire.MaxGetCFiltersReqRange,
	)
	if err != nil {
		log <- cl.Debug{"invalid getcfilters request:", err}
		return
	}
	// Create []*chainhash.Hash from []chainhash.Hash to pass to FiltersByBlockHashes.
	hashPtrs := make([]*chainhash.Hash, len(hashes))
	for i := range hashes {
		hashPtrs[i] = &hashes[i]
	}
	filters, err := sp.server.cfIndex.FiltersByBlockHashes(
		hashPtrs, msg.FilterType,
	)
	if err != nil {
		log <- cl.Error{"error retrieving cfilters:", err}
		return
	}
	for i, filterBytes := range filters {
		if len(filterBytes) == 0 {
			log <- cl.Warn{"could not obtain cfilter for", hashes[i]}
			return
		}
		filterMsg := wire.NewMsgCFilter(
			msg.FilterType, &hashes[i], filterBytes,
		)
		sp.QueueMessage(filterMsg, nil)
	}
}
// OnGetCFHeaders is invoked when a peer receives a getcfheader bitcoin message.
func (sp *serverPeer) OnGetCFHeaders(_ *peer.Peer, msg *wire.MsgGetCFHeaders) {
	// Ignore getcfilterheader requests if not in sync.
	if !sp.server.syncManager.IsCurrent() {
		return
	}
	// We'll also ensure that the remote party is requesting a set of headers for filters that we actually currently maintain.
	switch msg.FilterType {
	case wire.GCSFilterRegular:
		break
	default:
		log <- cl.Debug{
			"filter request for unknown headers for filter:", msg.FilterType,
		}
		return
	}
	startHeight := int32(msg.StartHeight)
	maxResults := wire.MaxCFHeadersPerMsg
	// If StartHeight is positive, fetch the predecessor block hash so we can populate the PrevFilterHeader field.
	if msg.StartHeight > 0 {
		startHeight--
		maxResults++
	}
	// Fetch the hashes from the block index.
	hashList, err := sp.server.chain.HeightToHashRange(
		startHeight, &msg.StopHash, maxResults,
	)
	if err != nil {
		log <- cl.Debug{
			"invalid getcfheaders request:", err,
		}
	}
	// This is possible if StartHeight is one greater that the height of StopHash, and we pull a valid range of hashes including the previous filter header.
	if len(hashList) == 0 || (msg.StartHeight > 0 && len(hashList) == 1) {
		log <- cl.Dbg("no results for getcfheaders request")
		return
	}
	// Create []*chainhash.Hash from []chainhash.Hash to pass to FilterHeadersByBlockHashes.
	hashPtrs := make([]*chainhash.Hash, len(hashList))
	for i := range hashList {
		hashPtrs[i] = &hashList[i]
	}
	// Fetch the raw filter hash bytes from the database for all blocks.
	filterHashes, err := sp.server.cfIndex.FilterHashesByBlockHashes(
		hashPtrs, msg.FilterType,
	)
	if err != nil {
		log <- cl.Error{"error retrieving cfilter hashes:", err}
		return
	}
	// Generate cfheaders message and send it.
	headersMsg := wire.NewMsgCFHeaders()
	// Populate the PrevFilterHeader field.
	if msg.StartHeight > 0 {
		prevBlockHash := &hashList[0]
		// Fetch the raw committed filter header bytes from the database.
		headerBytes, err := sp.server.cfIndex.FilterHeaderByBlockHash(
			prevBlockHash, msg.FilterType)
		if err != nil {
			log <- cl.Error{"error retrieving CF header:", err}
			return
		}
		if len(headerBytes) == 0 {
			log <- cl.Warn{"could not obtain CF header for", prevBlockHash}
			return
		}
		// Deserialize the hash into PrevFilterHeader.
		err = headersMsg.PrevFilterHeader.SetBytes(headerBytes)
		if err != nil {
			log <- cl.Warn{
				"committed filter header deserialize failed:", err,
			}
			return
		}
		hashList = hashList[1:]
		filterHashes = filterHashes[1:]
	}
	// Populate HeaderHashes.
	for i, hashBytes := range filterHashes {
		if len(hashBytes) == 0 {
			log <- cl.Warn{
				"could not obtain CF hash for", hashList[i],
			}
			return
		}
		// Deserialize the hash.
		filterHash, err := chainhash.NewHash(hashBytes)
		if err != nil {
			log <- cl.Warn{
				"committed filter hash deserialize failed:", err,
			}
			return
		}
		headersMsg.AddCFHash(filterHash)
	}
	headersMsg.FilterType = msg.FilterType
	headersMsg.StopHash = msg.StopHash
	sp.QueueMessage(headersMsg, nil)
}
// OnGetCFCheckpt is invoked when a peer receives a getcfcheckpt bitcoin message.
func (sp *serverPeer) OnGetCFCheckpt(_ *peer.Peer, msg *wire.MsgGetCFCheckpt) {
	// Ignore getcfcheckpt requests if not in sync.
	if !sp.server.syncManager.IsCurrent() {
		return
	}
	// We'll also ensure that the remote party is requesting a set of checkpoints for filters that we actually currently maintain.
	switch msg.FilterType {
	case wire.GCSFilterRegular:
		break
	default:
		log <- cl.Debugf{
			"filter request for unknown checkpoints for filter:", msg.FilterType,
		}
		return
	}
	// Now that we know the client is fetching a filter that we know of, we'll fetch the block hashes et each check point interval so we can compare against our cache, and create new check points if necessary.
	blockHashes, err := sp.server.chain.IntervalBlockHashes(
		&msg.StopHash, wire.CFCheckptInterval,
	)
	if err != nil {
		log <- cl.Debug{"invalid getcfilters request:", err}
		return
	}
	checkptMsg := wire.NewMsgCFCheckpt(
		msg.FilterType, &msg.StopHash, len(blockHashes),
	)
	// Fetch the current existing cache so we can decide if we need to extend it or if its adequate as is.
	sp.server.cfCheckptCachesMtx.RLock()
	checkptCache := sp.server.cfCheckptCaches[msg.FilterType]
	// If the set of block hashes is beyond the current size of the cache, then we'll expand the size of the cache and also retain the write lock.
	var updateCache bool
	if len(blockHashes) > len(checkptCache) {
		// Now that we know we'll need to modify the size of the cache, we'll release the read lock and grab the write lock to possibly expand the cache size.
		sp.server.cfCheckptCachesMtx.RUnlock()
		sp.server.cfCheckptCachesMtx.Lock()
		defer sp.server.cfCheckptCachesMtx.Unlock()
		// Now that we have the write lock, we'll check again as it's possible that the cache has already been expanded.
		checkptCache = sp.server.cfCheckptCaches[msg.FilterType]
		// If we still need to expand the cache, then We'll mark that we need to update the cache for below and also expand the size of the cache in place.
		if len(blockHashes) > len(checkptCache) {
			updateCache = true
			additionalLength := len(blockHashes) - len(checkptCache)
			newEntries := make([]cfHeaderKV, additionalLength)
			log <- cl.Infof{
				"growing size of checkpoint cache from %v to %v block hashes",
				len(checkptCache), len(blockHashes),
			}
			checkptCache = append(
				sp.server.cfCheckptCaches[msg.FilterType],
				newEntries...,
			)
		}
	} else {
		// Otherwise, we'll hold onto the read lock for the remainder of this method.
		defer sp.server.cfCheckptCachesMtx.RUnlock()
		log <- cl.Tracef{
			"serving stale cache of size %v", len(checkptCache),
		}
	}
	// Now that we know the cache is of an appropriate size, we'll iterate backwards until the find the block hash. We do this as it's possible a re-org has occurred so items in the db are now in the main china while the cache has been partially invalidated.
	var forkIdx int
	for forkIdx = len(blockHashes); forkIdx > 0; forkIdx-- {
		if checkptCache[forkIdx-1].blockHash == blockHashes[forkIdx-1] {
			break
		}
	}
	// Now that we know the how much of the cache is relevant for this query, we'll populate our check point message with the cache as is. Shortly below, we'll populate the new elements of the cache.
	for i := 0; i < forkIdx; i++ {
		checkptMsg.AddCFHeader(&checkptCache[i].filterHeader)
	}
	// We'll now collect the set of hashes that are beyond our cache so we can look up the filter headers to populate the final cache.
	blockHashPtrs := make([]*chainhash.Hash, 0, len(blockHashes)-forkIdx)
	for i := forkIdx; i < len(blockHashes); i++ {
		blockHashPtrs = append(blockHashPtrs, &blockHashes[i])
	}
	filterHeaders, err := sp.server.cfIndex.FilterHeadersByBlockHashes(
		blockHashPtrs, msg.FilterType,
	)
	if err != nil {
		log <- cl.Error{"error retrieving cfilter headers:", err}
		return
	}
	// Now that we have the full set of filter headers, we'll add them to the checkpoint message, and also update our cache in line.
	for i, filterHeaderBytes := range filterHeaders {
		if len(filterHeaderBytes) == 0 {
			log <- cl.Warn{
				"could not obtain CF header for", blockHashPtrs[i],
			}
			return
		}
		filterHeader, err := chainhash.NewHash(filterHeaderBytes)
		if err != nil {
			log <- cl.Warn{
				"committed filter header deserialize failed:", err,
			}
			return
		}
		checkptMsg.AddCFHeader(filterHeader)
		// If the new main chain is longer than what's in the cache, then we'll override it beyond the fork point.
		if updateCache {
			checkptCache[forkIdx+i] = cfHeaderKV{
				blockHash:    blockHashes[forkIdx+i],
				filterHeader: *filterHeader,
			}
		}
	}
	// Finally, we'll update the cache if we need to, and send the final message back to the requesting peer.
	if updateCache {
		sp.server.cfCheckptCaches[msg.FilterType] = checkptCache
	}
	sp.QueueMessage(checkptMsg, nil)
}
// enforceNodeBloomFlag disconnects the peer if the server is not configured to allow bloom filters.  Additionally, if the peer has negotiated to a protocol version  that is high enough to observe the bloom filter service support bit, it will be banned since it is intentionally violating the protocol.
func (sp *serverPeer) enforceNodeBloomFlag(cmd string) bool {
	if sp.server.services&wire.SFNodeBloom != wire.SFNodeBloom {
		// Ban the peer if the protocol version is high enough that the peer is knowingly violating the protocol and banning is enabled. NOTE: Even though the addBanScore function already examines whether or not banning is enabled, it is checked here as well to ensure the violation is logged and the peer is disconnected regardless.
		if sp.ProtocolVersion() >= wire.BIP0111Version &&
			!cfg.DisableBanning {
			// Disconnect the peer regardless of whether it was banned.
			sp.addBanScore(100, 0, cmd)
			sp.Disconnect()
			return false
		}
		// Disconnect the peer regardless of protocol version or banning state.
		log <- cl.Debugf{
			"%s sent an unsupported %s request -- disconnecting", sp, cmd,
		}
		sp.Disconnect()
		return false
	}
	return true
}
// OnFeeFilter is invoked when a peer receives a feefilter bitcoin message and is used by remote peers to request that no transactions which have a fee rate lower than provided value are inventoried to them.  The peer will be disconnected if an invalid fee filter value is provided.
func (sp *serverPeer) OnFeeFilter(_ *peer.Peer, msg *wire.MsgFeeFilter) {
	// Check that the passed minimum fee is a valid amount.
	if msg.MinFee < 0 || msg.MinFee > util.MaxSatoshi {
		log <- cl.Debugf{
			"peer %v sent an invalid feefilter '%v' -- disconnecting",
			sp, util.Amount(msg.MinFee)}
		sp.Disconnect()
		return
	}
	atomic.StoreInt64(&sp.feeFilter, msg.MinFee)
}
// OnFilterAdd is invoked when a peer receives a filteradd bitcoin message and is used by remote peers to add data to an already loaded bloom filter.  The peer will be disconnected if a filter is not loaded when this message is received or the server is not configured to allow bloom filters.
func (sp *serverPeer) OnFilterAdd(_ *peer.Peer, msg *wire.MsgFilterAdd) {
	// Disconnect and/or ban depending on the node bloom services flag and negotiated protocol version.
	if !sp.enforceNodeBloomFlag(msg.Command()) {
		return
	}
	if !sp.filter.IsLoaded() {
		log <- cl.Debugf{
			"%s sent a filteradd request with no filter loaded -- disconnecting", sp,
		}
		sp.Disconnect()
		return
	}
	sp.filter.Add(msg.Data)
}
// OnFilterClear is invoked when a peer receives a filterclear bitcoin message and is used by remote peers to clear an already loaded bloom filter. The peer will be disconnected if a filter is not loaded when this message is received  or the server is not configured to allow bloom filters.
func (sp *serverPeer) OnFilterClear(_ *peer.Peer, msg *wire.MsgFilterClear) {
	// Disconnect and/or ban depending on the node bloom services flag and negotiated protocol version.
	if !sp.enforceNodeBloomFlag(msg.Command()) {
		return
	}
	if !sp.filter.IsLoaded() {
		log <- cl.Debugf{
			"%s sent a filterclear request with no filter loaded -- disconnecting", sp,
		}
		sp.Disconnect()
		return
	}
	sp.filter.Unload()
}
// OnFilterLoad is invoked when a peer receives a filterload bitcoin message and it used to load a bloom filter that should be used for delivering merkle blocks and associated transactions that match the filter. The peer will be disconnected if the server is not configured to allow bloom filters.
func (sp *serverPeer) OnFilterLoad(_ *peer.Peer, msg *wire.MsgFilterLoad) {
	// Disconnect and/or ban depending on the node bloom services flag and negotiated protocol version.
	if !sp.enforceNodeBloomFlag(msg.Command()) {
		return
	}
	sp.setDisableRelayTx(false)
	sp.filter.Reload(msg)
}
// OnGetAddr is invoked when a peer receives a getaddr bitcoin message and is used to provide the peer with known addresses from the address manager.
func (sp *serverPeer) OnGetAddr(_ *peer.Peer, msg *wire.MsgGetAddr) {
	// Don't return any addresses when running on the simulation test network.  This helps prevent the network from becoming another public test network since it will not be able to learn about other peers that have not specifically been provided.
	if cfg.SimNet {
		return
	}
	// Do not accept getaddr requests from outbound peers.  This reduces fingerprinting attacks.
	if !sp.Inbound() {
		log <- cl.Debug{"ignoring getaddr request from outbound peer", sp}
		return
	}
	// Only allow one getaddr request per connection to discourage address stamping of inv announcements.
	if sp.sentAddrs {
		log <- cl.Debugf{"ignoring repeated getaddr request from peer", sp}
		return
	}
	sp.sentAddrs = true
	// Get the current known addresses from the address manager.
	addrCache := sp.server.addrManager.AddressCache()
	// Push the addresses.
	sp.pushAddrMsg(addrCache)
}
// OnAddr is invoked when a peer receives an addr bitcoin message and is used to notify the server about advertised addresses.
func (sp *serverPeer) OnAddr(_ *peer.Peer, msg *wire.MsgAddr) {
	// Ignore addresses when running on the simulation test network.  This helps prevent the network from becoming another public test network since it will not be able to learn about other peers that have not specifically been provided.
	if cfg.SimNet {
		return
	}
	// Ignore old style addresses which don't include a timestamp.
	if sp.ProtocolVersion() < wire.NetAddressTimeVersion {
		return
	}
	// A message that has no addresses is invalid.
	if len(msg.AddrList) == 0 {
		log <- cl.Errorf{
			"command [%s] from %s does not contain any addresses",
			msg.Command(), sp.Peer,
		}
		sp.Disconnect()
		return
	}
	for _, na := range msg.AddrList {
		// Don't add more address if we're disconnecting.
		if !sp.Connected() {
			return
		}
		// Set the timestamp to 5 days ago if it's more than 24 hours in the future so this address is one of the first to be removed when space is needed.
		now := time.Now()
		if na.Timestamp.After(now.Add(time.Minute * 10)) {
			na.Timestamp = now.Add(-1 * time.Hour * 24 * 5)
		}
		// Add address to known addresses for this peer.
		sp.addKnownAddresses([]*wire.NetAddress{na})
	}
	// Add addresses to server address manager.  The address manager handles the details of things such as preventing duplicate addresses, max addresses, and last seen updates. XXX bitcoind gives a 2 hour time penalty here, do we want to do the same?
	sp.server.addrManager.AddAddresses(msg.AddrList, sp.NA())
}
// OnRead is invoked when a peer receives a message and it is used to update the bytes received by the server.
func (sp *serverPeer) OnRead(_ *peer.Peer, bytesRead int, msg wire.Message, err error) {
	sp.server.AddBytesReceived(uint64(bytesRead))
}
// OnWrite is invoked when a peer sends a message and it is used to update the bytes sent by the server.
func (sp *serverPeer) OnWrite(_ *peer.Peer, bytesWritten int, msg wire.Message, err error) {
	sp.server.AddBytesSent(uint64(bytesWritten))
}
// randomUint16Number returns a random uint16 in a specified input range.  Note that the range is in zeroth ordering; if you pass it 1800, you will get values from 0 to 1800.
func randomUint16Number(max uint16) uint16 {
	// In order to avoid modulo bias and ensure every possible outcome in [0, max) has equal probability, the random number must be sampled from a random source that has a range limited to a multiple of the modulus.
	var randomNumber uint16
	var limitRange = (math.MaxUint16 / max) * max
	for {
		binary.Read(rand.Reader, binary.LittleEndian, &randomNumber)
		if randomNumber < limitRange {
			return (randomNumber % max)
		}
	}
}
// AddRebroadcastInventory adds 'iv' to the list of inventories to be rebroadcasted at random intervals until they show up in a block.
func (s *server) AddRebroadcastInventory(iv *wire.InvVect, data interface{}) {
	// Ignore if shutting down.
	if atomic.LoadInt32(&s.shutdown) != 0 {
		return
	}
	s.modifyRebroadcastInv <- broadcastInventoryAdd{invVect: iv, data: data}
}
// RemoveRebroadcastInventory removes 'iv' from the list of items to be rebroadcasted if present.
func (s *server) RemoveRebroadcastInventory(iv *wire.InvVect) {
	// Log<-cl.Debug{emoveBroadcastInventory"
	// Ignore if shutting down.
	if atomic.LoadInt32(&s.shutdown) != 0 {
		// Log<-cl.Debug{gnoring due to shutdown"
		return
	}
	s.modifyRebroadcastInv <- broadcastInventoryDel(iv)
}
// relayTransactions generates and relays inventory vectors for all of the passed transactions to all connected peers.
func (s *server) relayTransactions(txns []*mempool.TxDesc) {
	for _, txD := range txns {
		iv := wire.NewInvVect(wire.InvTypeTx, txD.Tx.Hash())
		s.RelayInventory(iv, txD)
	}
}
// AnnounceNewTransactions generates and relays inventory vectors and notifies both websocket and getblocktemplate long poll clients of the passed transactions.  This function should be called whenever new transactions are added to the mempool.
func (s *server) AnnounceNewTransactions(txns []*mempool.TxDesc) {
	// Generate and relay inventory vectors for all newly accepted transactions.
	s.relayTransactions(txns)
	// Notify both websocket and getblocktemplate long poll clients of all newly accepted transactions.
	for i := range s.rpcServers {
		if s.rpcServers[i] != nil {
			s.rpcServers[i].NotifyNewTransactions(txns)
		}
	}
}
// Transaction has one confirmation on the main chain. Now we can mark it as no longer needing rebroadcasting.
func (s *server) TransactionConfirmed(tx *util.Tx) {
	// Log<-cl.Debug{ransactionConfirmed"
	// Rebroadcasting is only necessary when the RPC server is active.
	for i := range s.rpcServers {
		// Log<-cl.Debug{ending to RPC servers"
		if s.rpcServers[i] == nil {
			return
		}
	}
	// Log<-cl.Debug{etting new inventory vector"
	iv := wire.NewInvVect(wire.InvTypeTx, tx.Hash())
	// Log<-cl.Debug{emoving broadcast inventory"
	s.RemoveRebroadcastInventory(iv)
	// Log<-cl.Debug{one TransactionConfirmed"
}
// pushTxMsg sends a tx message for the provided transaction hash to the connected peer.  An error is returned if the transaction hash is not known.
func (s *server) pushTxMsg(sp *serverPeer, hash *chainhash.Hash, doneChan chan<- struct{},
	waitChan <-chan struct{}, encoding wire.MessageEncoding) error {
	// Attempt to fetch the requested transaction from the pool.  A call could be made to check for existence first, but simply trying to fetch a missing transaction results in the same behavior.
	tx, err := s.txMemPool.FetchTransaction(hash)
	if err != nil {
		log <- cl.Tracef{
			"unable to fetch tx %v from transaction pool: %v", hash, err,
		}
		if doneChan != nil {
			doneChan <- struct{}{}
		}
		return err
	}
	// Once we have fetched data wait for any previous operation to finish.
	if waitChan != nil {
		<-waitChan
	}
	sp.QueueMessageWithEncoding(tx.MsgTx(), doneChan, encoding)
	return nil
}
// pushBlockMsg sends a block message for the provided block hash to the connected peer.  An error is returned if the block hash is not known.
func (s *server) pushBlockMsg(sp *serverPeer, hash *chainhash.Hash, doneChan chan<- struct{},
	waitChan <-chan struct{}, encoding wire.MessageEncoding) error {
	// Fetch the raw block bytes from the database.
	var blockBytes []byte
	err := sp.server.db.View(func(dbTx database.Tx) error {
		var err error
		blockBytes, err = dbTx.FetchBlock(hash)
		return err
	})
	if err != nil {
		log <- cl.Tracef{
			"unable to fetch requested block hash %v: %v",
			hash, err,
		}
		if doneChan != nil {
			doneChan <- struct{}{}
		}
		return err
	}
	// Deserialize the block.
	var msgBlock wire.MsgBlock
	err = msgBlock.Deserialize(bytes.NewReader(blockBytes))
	if err != nil {
		log <- cl.Tracef{
			"unable to deserialize requested block hash %v: %v",
			hash, err,
		}
		if doneChan != nil {
			doneChan <- struct{}{}
		}
		return err
	}
	// Once we have fetched data wait for any previous operation to finish.
	if waitChan != nil {
		<-waitChan
	}
	// We only send the channel for this message if we aren't sending an inv straight after.
	var dc chan<- struct{}
	continueHash := sp.continueHash
	sendInv := continueHash != nil && continueHash.IsEqual(hash)
	if !sendInv {
		dc = doneChan
	}
	sp.QueueMessageWithEncoding(&msgBlock, dc, encoding)
	// When the peer requests the final block that was advertised in response to a getblocks message which requested more blocks than would fit into a single message, send it a new inventory message to trigger it to issue another getblocks message for the next batch of inventory.
	if sendInv {
		best := sp.server.chain.BestSnapshot()
		invMsg := wire.NewMsgInvSizeHint(1)
		iv := wire.NewInvVect(wire.InvTypeBlock, &best.Hash)
		invMsg.AddInvVect(iv)
		sp.QueueMessage(invMsg, doneChan)
		sp.continueHash = nil
	}
	return nil
}
// pushMerkleBlockMsg sends a merkleblock message for the provided block hash to the connected peer.  Since a merkle block requires the peer to have a filter loaded, this call will simply be ignored if there is no filter loaded.  An error is returned if the block hash is not known.
func (s *server) pushMerkleBlockMsg(sp *serverPeer, hash *chainhash.Hash,
	doneChan chan<- struct{}, waitChan <-chan struct{}, encoding wire.MessageEncoding) error {
	// Do not send a response if the peer doesn't have a filter loaded.
	if !sp.filter.IsLoaded() {
		if doneChan != nil {
			doneChan <- struct{}{}
		}
		return nil
	}
	// Fetch the raw block bytes from the database.
	blk, err := sp.server.chain.BlockByHash(hash)
	if err != nil {
		log <- cl.Tracef{
			"unable to fetch requested block hash %v: %v",
			hash, err,
		}
		if doneChan != nil {
			doneChan <- struct{}{}
		}
		return err
	}
	// Generate a merkle block by filtering the requested block according to the filter for the peer.
	merkle, matchedTxIndices := bloom.NewMerkleBlock(blk, sp.filter)
	// Once we have fetched data wait for any previous operation to finish.
	if waitChan != nil {
		<-waitChan
	}
	// Send the merkleblock.  Only send the done channel with this message if no transactions will be sent afterwards.
	var dc chan<- struct{}
	if len(matchedTxIndices) == 0 {
		dc = doneChan
	}
	sp.QueueMessage(merkle, dc)
	// Finally, send any matched transactions.
	blkTransactions := blk.MsgBlock().Transactions
	for i, txIndex := range matchedTxIndices {
		// Only send the done channel on the final transaction.
		var dc chan<- struct{}
		if i == len(matchedTxIndices)-1 {
			dc = doneChan
		}
		if txIndex < uint32(len(blkTransactions)) {
			sp.QueueMessageWithEncoding(blkTransactions[txIndex], dc,
				encoding)
		}
	}
	return nil
}
// handleUpdatePeerHeight updates the heights of all peers who were known to announce a block we recently accepted.
func (s *server) handleUpdatePeerHeights(state *peerState, umsg updatePeerHeightsMsg) {
	state.forAllPeers(func(sp *serverPeer) {
		// The origin peer should already have the updated height.
		if sp.Peer == umsg.originPeer {
			return
		}
		// This is a pointer to the underlying memory which doesn't change.
		latestBlkHash := sp.LastAnnouncedBlock()
		// Skip this peer if it hasn't recently announced any new blocks.
		if latestBlkHash == nil {
			return
		}
		// If the peer has recently announced a block, and this block matches our newly accepted block, then update their block height.
		if *latestBlkHash == *umsg.newHash {
			sp.UpdateLastBlockHeight(umsg.newHeight)
			sp.UpdateLastAnnouncedBlock(nil)
		}
	})
}
// handleAddPeerMsg deals with adding new peers.  It is invoked from the peerHandler goroutine.
func (s *server) handleAddPeerMsg(state *peerState, sp *serverPeer) bool {
	if sp == nil {
		return false
	}
	// Ignore new peers if we're shutting down.
	if atomic.LoadInt32(&s.shutdown) != 0 {
		log <- cl.Infof{
			"new peer %s ignored - server is shutting down", sp,
		}
		sp.Disconnect()
		return false
	}
	// Disconnect banned peers.
	host, _, err := net.SplitHostPort(sp.Addr())
	if err != nil {
		log <- cl.Debug{"can't split hostport", err}
		sp.Disconnect()
		return false
	}
	if banEnd, ok := state.banned[host]; ok {
		if time.Now().Before(banEnd) {
			log <- cl.Debugf{
				"peer %s is banned for another %v - disconnecting",
				host, time.Until(banEnd),
			}
			sp.Disconnect()
			return false
		}
		log <- cl.Infof{"peer %s is no longer banned", host}
		delete(state.banned, host)
	}
	// TODO: Check for max peers from a single IP. Limit max number of total peers.
	if state.Count() >= cfg.MaxPeers {
		log <- cl.Infof{
			"max peers reached [%d] - disconnecting peer %s",
			cfg.MaxPeers, sp,
		}
		sp.Disconnect()
		// TODO: how to handle permanent peers here? they should be rescheduled.
		return false
	}
	// Add the new peer and start it.
	log <- cl.Debug{"new peer", sp}
	if sp.Inbound() {
		state.inboundPeers[sp.ID()] = sp
	} else {
		state.outboundGroups[addrmgr.GroupKey(sp.NA())]++
		if sp.persistent {
			state.persistentPeers[sp.ID()] = sp
		} else {
			state.outboundPeers[sp.ID()] = sp
		}
	}
	return true
}
// handleDonePeerMsg deals with peers that have signalled they are done.  It is invoked from the peerHandler goroutine.
func (s *server) handleDonePeerMsg(state *peerState, sp *serverPeer) {
	var list map[int32]*serverPeer
	if sp.persistent {
		list = state.persistentPeers
	} else if sp.Inbound() {
		list = state.inboundPeers
	} else {
		list = state.outboundPeers
	}
	if _, ok := list[sp.ID()]; ok {
		if !sp.Inbound() && sp.VersionKnown() {
			state.outboundGroups[addrmgr.GroupKey(sp.NA())]--
		}
		if !sp.Inbound() && sp.connReq != nil {
			s.connManager.Disconnect(sp.connReq.ID())
		}
		delete(list, sp.ID())
		log <- cl.Debug{"Removed peer", sp}
		return
	}
	if sp.connReq != nil {
		s.connManager.Disconnect(sp.connReq.ID())
	}
	// Update the address' last seen time if the peer has acknowledged our version and has sent us its version as well.
	if sp.VerAckReceived() && sp.VersionKnown() && sp.NA() != nil {
		s.addrManager.Connected(sp.NA())
	}
	// If we get here it means that either we didn't know about the peer or we purposefully deleted it.
}
// handleBanPeerMsg deals with banning peers.  It is invoked from the peerHandler goroutine.
func (s *server) handleBanPeerMsg(state *peerState, sp *serverPeer) {
	host, _, err := net.SplitHostPort(sp.Addr())
	if err != nil {
		log <- cl.Debugf{"can't split ban peer %s %v", sp.Addr(), err}
		return
	}
	direction := directionString(sp.Inbound())
	log <- cl.Infof{
		"banned peer %s (%s) for %v", host, direction, cfg.BanDuration,
	}
	state.banned[host] = time.Now().Add(cfg.BanDuration)
}
// handleRelayInvMsg deals with relaying inventory to peers that are not already known to have it.  It is invoked from the peerHandler goroutine.
func (s *server) handleRelayInvMsg(state *peerState, msg relayMsg) {
	state.forAllPeers(func(sp *serverPeer) {
		if !sp.Connected() {
			return
		}
		// If the inventory is a block and the peer prefers headers, generate and send a headers message instead of an inventory message.
		if msg.invVect.Type == wire.InvTypeBlock && sp.WantsHeaders() {
			blockHeader, ok := msg.data.(wire.BlockHeader)
			if !ok {
				log <- cl.Wrn("underlying data for headers is not a block header")
				return
			}
			msgHeaders := wire.NewMsgHeaders()
			if err := msgHeaders.AddBlockHeader(&blockHeader); err != nil {
				log <- cl.Error{"failed to add block header:", err}
				return
			}
			sp.QueueMessage(msgHeaders, nil)
			return
		}
		if msg.invVect.Type == wire.InvTypeTx {
			// Don't relay the transaction to the peer when it has transaction relaying disabled.
			if sp.relayTxDisabled() {
				return
			}
			txD, ok := msg.data.(*mempool.TxDesc)
			if !ok {
				log <- cl.Warnf{
					"underlying data for tx inv relay is not a *mempool.TxDesc: %T",
					msg.data,
				}
				return
			}
			// Don't relay the transaction if the transaction fee-per-kb is less than the peer's feefilter.
			feeFilter := atomic.LoadInt64(&sp.feeFilter)
			if feeFilter > 0 && txD.FeePerKB < feeFilter {
				return
			}
			// Don't relay the transaction if there is a bloom filter loaded and the transaction doesn't match it.
			if sp.filter.IsLoaded() {
				if !sp.filter.MatchTxAndUpdate(txD.Tx) {
					return
				}
			}
		}
		// Queue the inventory to be relayed with the next batch. It will be ignored if the peer is already known to have the inventory.
		sp.QueueInventory(msg.invVect)
	})
}
// handleBroadcastMsg deals with broadcasting messages to peers.  It is invoked from the peerHandler goroutine.
func (s *server) handleBroadcastMsg(state *peerState, bmsg *broadcastMsg) {
	state.forAllPeers(func(sp *serverPeer) {
		if !sp.Connected() {
			return
		}
		for _, ep := range bmsg.excludePeers {
			if sp == ep {
				return
			}
		}
		sp.QueueMessage(bmsg.message, nil)
	})
}
type getConnCountMsg struct {
	reply chan int32
}
type getPeersMsg struct {
	reply chan []*serverPeer
}
type getOutboundGroup struct {
	key   string
	reply chan int
}
type getAddedNodesMsg struct {
	reply chan []*serverPeer
}
type disconnectNodeMsg struct {
	cmp   func(*serverPeer) bool
	reply chan error
}
type connectNodeMsg struct {
	addr      string
	permanent bool
	reply     chan error
}
type removeNodeMsg struct {
	cmp   func(*serverPeer) bool
	reply chan error
}
// handleQuery is the central handler for all queries and commands from other goroutines related to peer state.
func (s *server) handleQuery(state *peerState, querymsg interface{}) {
	switch msg := querymsg.(type) {
	case getConnCountMsg:
		nconnected := int32(0)
		state.forAllPeers(func(sp *serverPeer) {
			if sp.Connected() {
				nconnected++
			}
		})
		msg.reply <- nconnected
	case getPeersMsg:
		peers := make([]*serverPeer, 0, state.Count())
		state.forAllPeers(func(sp *serverPeer) {
			if !sp.Connected() {
				return
			}
			peers = append(peers, sp)
		})
		msg.reply <- peers
	case connectNodeMsg:
		// TODO: duplicate oneshots? Limit max number of total peers.
		if state.Count() >= cfg.MaxPeers {
			msg.reply <- errors.New("max peers reached")
			return
		}
		for _, peer := range state.persistentPeers {
			if peer.Addr() == msg.addr {
				if msg.permanent {
					msg.reply <- errors.New("peer already connected")
				} else {
					msg.reply <- errors.New("peer exists as a permanent peer")
				}
				return
			}
		}
		netAddr, err := addrStringToNetAddr(msg.addr)
		if err != nil {
			msg.reply <- err
			return
		}
		// TODO: if too many, nuke a non-perm peer.
		go s.connManager.Connect(&connmgr.ConnReq{
			Addr:      netAddr,
			Permanent: msg.permanent,
		})
		msg.reply <- nil
	case removeNodeMsg:
		found := disconnectPeer(state.persistentPeers, msg.cmp, func(sp *serverPeer) {
			// Keep group counts ok since we remove from the list now.
			state.outboundGroups[addrmgr.GroupKey(sp.NA())]--
		})
		if found {
			msg.reply <- nil
		} else {
			msg.reply <- errors.New("peer not found")
		}
	case getOutboundGroup:
		count, ok := state.outboundGroups[msg.key]
		if ok {
			msg.reply <- count
		} else {
			msg.reply <- 0
		}
	// Request a list of the persistent (added) peers.
	case getAddedNodesMsg:
		// Respond with a slice of the relevant peers.
		peers := make([]*serverPeer, 0, len(state.persistentPeers))
		for _, sp := range state.persistentPeers {
			peers = append(peers, sp)
		}
		msg.reply <- peers
	case disconnectNodeMsg:
		// Check inbound peers. We pass a nil callback since we don't require any additional actions on disconnect for inbound peers.
		found := disconnectPeer(state.inboundPeers, msg.cmp, nil)
		if found {
			msg.reply <- nil
			return
		}
		// Check outbound peers.
		found = disconnectPeer(state.outboundPeers, msg.cmp, func(sp *serverPeer) {
			// Keep group counts ok since we remove from the list now.
			state.outboundGroups[addrmgr.GroupKey(sp.NA())]--
		})
		if found {
			// If there are multiple outbound connections to the same ip:port, continue disconnecting them all until no such peers are found.
			for found {
				found = disconnectPeer(state.outboundPeers, msg.cmp, func(sp *serverPeer) {
					state.outboundGroups[addrmgr.GroupKey(sp.NA())]--
				})
			}
			msg.reply <- nil
			return
		}
		msg.reply <- errors.New("peer not found")
	}
}
// disconnectPeer attempts to drop the connection of a targeted peer in the passed peer list. Targets are identified via usage of the passed `compareFunc`, which should return `true` if the passed peer is the target peer. This function returns true on success and false if the peer is unable to be located. If the peer is found, and the passed callback: `whenFound' isn't nil, we call it with the peer as the argument before it is removed from the peerList, and is disconnected from the server.
func disconnectPeer(peerList map[int32]*serverPeer, compareFunc func(*serverPeer) bool, whenFound func(*serverPeer)) bool {
	for addr, peer := range peerList {
		if compareFunc(peer) {
			if whenFound != nil {
				whenFound(peer)
			}
			// This is ok because we are not continuing to iterate so won't corrupt the loop.
			delete(peerList, addr)
			peer.Disconnect()
			return true
		}
	}
	return false
}
// newPeerConfig returns the configuration for the given serverPeer.
func newPeerConfig(sp *serverPeer) *peer.Config {
	return &peer.Config{
		Listeners: peer.MessageListeners{
			OnVersion:      sp.OnVersion,
			OnMemPool:      sp.OnMemPool,
			OnTx:           sp.OnTx,
			OnBlock:        sp.OnBlock,
			OnInv:          sp.OnInv,
			OnHeaders:      sp.OnHeaders,
			OnGetData:      sp.OnGetData,
			OnGetBlocks:    sp.OnGetBlocks,
			OnGetHeaders:   sp.OnGetHeaders,
			OnGetCFilters:  sp.OnGetCFilters,
			OnGetCFHeaders: sp.OnGetCFHeaders,
			OnGetCFCheckpt: sp.OnGetCFCheckpt,
			OnFeeFilter:    sp.OnFeeFilter,
			OnFilterAdd:    sp.OnFilterAdd,
			OnFilterClear:  sp.OnFilterClear,
			OnFilterLoad:   sp.OnFilterLoad,
			OnGetAddr:      sp.OnGetAddr,
			OnAddr:         sp.OnAddr,
			OnRead:         sp.OnRead,
			OnWrite:        sp.OnWrite,
			// Note: The reference client currently bans peers that send alerts not signed with its key.  We could verify against their key, but since the reference client is currently unwilling to support other implementations' alert messages, we will not relay theirs.
			OnAlert: nil,
		},
		NewestBlock:       sp.newestBlock,
		HostToNetAddress:  sp.server.addrManager.HostToNetAddress,
		Proxy:             cfg.Proxy,
		UserAgentName:     userAgentName,
		UserAgentVersion:  userAgentVersion,
		UserAgentComments: cfg.UserAgentComments,
		ChainParams:       sp.server.chainParams,
		Services:          sp.server.services,
		DisableRelayTx:    cfg.BlocksOnly,
		ProtocolVersion:   peer.MaxProtocolVersion,
		TrickleInterval:   cfg.TrickleInterval,
	}
}
// inboundPeerConnected is invoked by the connection manager when a new inbound connection is established.  It initializes a new inbound server peer instance, associates it with the connection, and starts a goroutine to wait for disconnection.
func (s *server) inboundPeerConnected(conn net.Conn) {
	sp := newServerPeer(s, false)
	sp.isWhitelisted = isWhitelisted(conn.RemoteAddr())
	sp.Peer = peer.NewInboundPeer(newPeerConfig(sp))
	sp.AssociateConnection(conn)
	go s.peerDoneHandler(sp)
}
// outboundPeerConnected is invoked by the connection manager when a new outbound connection is established.  It initializes a new outbound server peer instance, associates it with the relevant state such as the connection request instance and the connection itself, and finally notifies the address manager of the attempt.
func (s *server) outboundPeerConnected(c *connmgr.ConnReq, conn net.Conn) {
	sp := newServerPeer(s, c.Permanent)
	p, err := peer.NewOutboundPeer(newPeerConfig(sp), c.Addr.String())
	if err != nil {
		log <- cl.Debugf{"Cannot create outbound peer %s: %v", c.Addr, err}
		s.connManager.Disconnect(c.ID())
	}
	sp.Peer = p
	sp.connReq = c
	sp.isWhitelisted = isWhitelisted(conn.RemoteAddr())
	sp.AssociateConnection(conn)
	go s.peerDoneHandler(sp)
	s.addrManager.Attempt(sp.NA())
}
// peerDoneHandler handles peer disconnects by notifiying the server that it's done along with other performing other desirable cleanup.
func (s *server) peerDoneHandler(sp *serverPeer) {
	sp.WaitForDisconnect()
	s.donePeers <- sp
	// Only tell sync manager we are gone if we ever told it we existed.
	if sp.VersionKnown() {
		s.syncManager.DonePeer(sp.Peer)
		// Evict any remaining orphans that were sent by the peer.
		numEvicted := s.txMemPool.RemoveOrphansByTag(mempool.Tag(sp.ID()))
		if numEvicted > 0 {
			log <- cl.Debugf{
				"Evicted %d %s from peer %v (id %d)",
				numEvicted, pickNoun(numEvicted, "orphan", "orphans"),
				sp, sp.ID(),
			}
		}
	}
	close(sp.quit)
}
// peerHandler is used to handle peer operations such as adding and removing peers to and from the server, banning peers, and broadcasting messages to peers.  It must be run in a goroutine.
func (s *server) peerHandler() {
	// Start the address manager and sync manager, both of which are needed by peers.  This is done here since their lifecycle is closely tied to this handler and rather than adding more channels to sychronize things, it's easier and slightly faster to simply start and stop them in this handler.
	s.addrManager.Start()
	s.syncManager.Start()
	log <- cl.Trc("starting peer handler")
	state := &peerState{
		inboundPeers:    make(map[int32]*serverPeer),
		persistentPeers: make(map[int32]*serverPeer),
		outboundPeers:   make(map[int32]*serverPeer),
		banned:          make(map[string]time.Time),
		outboundGroups:  make(map[string]int),
	}
	if !cfg.DisableDNSSeed {
		log <- cl.Trc("seeding from DNS")
		// Add peers discovered through DNS to the address manager.
		connmgr.SeedFromDNS(ActiveNetParams.Params, defaultRequiredServices,
			podLookup, func(addrs []*wire.NetAddress) {
				// Bitcoind uses a lookup of the dns seeder here. This is rather strange since the values looked up by the DNS seed lookups will vary quite a lot. to replicate this behaviour we put all addresses as having come from the first one.
				s.addrManager.AddAddresses(addrs, addrs[0])
			})
	}
	log <- cl.Trc("starting connmgr")
	go s.connManager.Start()
out:
	for {
		select {
		// New peers connected to the server.
		case p := <-s.newPeers:
			// fmt.Println("chan:p := <-s.newPeers")
			s.handleAddPeerMsg(state, p)
		// Disconnected peers.
		case p := <-s.donePeers:
			// fmt.Println("chan:p := <-s.donePeers")
			s.handleDonePeerMsg(state, p)
		// Block accepted in mainchain or orphan, update peer height.
		case umsg := <-s.peerHeightsUpdate:
			// fmt.Println("chan:umsg := <-s.peerHeightsUpdate")
			s.handleUpdatePeerHeights(state, umsg)
		// Peer to ban.
		case p := <-s.banPeers:
			// fmt.Println("chan:p := <-s.banPeers")
			s.handleBanPeerMsg(state, p)
		// New inventory to potentially be relayed to other peers.
		case invMsg := <-s.relayInv:
			// fmt.Println("chan:invMsg := <-s.relayInv")
			s.handleRelayInvMsg(state, invMsg)
		// Message to broadcast to all connected peers except those which are excluded by the message.
		case bmsg := <-s.broadcast:
			// fmt.Println("chan:bmsg := <-s.broadcast")
			s.handleBroadcastMsg(state, &bmsg)
		case qmsg := <-s.query:
			// fmt.Println("chan:qmsg := <-s.query")
			s.handleQuery(state, qmsg)
		case <-s.quit:
			// fmt.Println("chan:<-s.quit")
			// Disconnect all peers on server shutdown.
			state.forAllPeers(func(sp *serverPeer) {
				log <- cl.Tracef{"Shutdown peer %s", sp}
				sp.Disconnect()
			})
			break out
		}
	}
	s.connManager.Stop()
	s.syncManager.Stop()
	s.addrManager.Stop()
	// Drain channels before exiting so nothing is left waiting around to send.
cleanup:
	for {
		select {
		case <-s.newPeers:
		case <-s.donePeers:
		case <-s.peerHeightsUpdate:
		case <-s.relayInv:
		case <-s.broadcast:
		case <-s.query:
		default:
			break cleanup
		}
	}
	s.wg.Done()
	log <- cl.Tracef{"Peer handler done"}
}
// AddPeer adds a new peer that has already been connected to the server.
func (s *server) AddPeer(sp *serverPeer) {
	s.newPeers <- sp
}
// BanPeer bans a peer that has already been connected to the server by ip.
func (s *server) BanPeer(sp *serverPeer) {
	s.banPeers <- sp
}
// RelayInventory relays the passed inventory vector to all connected peers that are not already known to have it.
func (s *server) RelayInventory(invVect *wire.InvVect, data interface{}) {
	s.relayInv <- relayMsg{invVect: invVect, data: data}
}
// BroadcastMessage sends msg to all peers currently connected to the server except those in the passed peers to exclude.
func (s *server) BroadcastMessage(msg wire.Message, exclPeers ...*serverPeer) {
	// XXX: Need to determine if this is an alert that has already been broadcast and refrain from broadcasting again.
	bmsg := broadcastMsg{message: msg, excludePeers: exclPeers}
	s.broadcast <- bmsg
}
// ConnectedCount returns the number of currently connected peers.
func (s *server) ConnectedCount() int32 {
	replyChan := make(chan int32)
	s.query <- getConnCountMsg{reply: replyChan}
	return <-replyChan
}
// OutboundGroupCount returns the number of peers connected to the given outbound group key.
func (s *server) OutboundGroupCount(key string) int {
	replyChan := make(chan int)
	s.query <- getOutboundGroup{key: key, reply: replyChan}
	return <-replyChan
}
// AddBytesSent adds the passed number of bytes to the total bytes sent counter for the server.  It is safe for concurrent access.
func (s *server) AddBytesSent(bytesSent uint64) {
	atomic.AddUint64(&s.bytesSent, bytesSent)
}
// AddBytesReceived adds the passed number of bytes to the total bytes received counter for the server.  It is safe for concurrent access.
func (s *server) AddBytesReceived(bytesReceived uint64) {
	atomic.AddUint64(&s.bytesReceived, bytesReceived)
}
// NetTotals returns the sum of all bytes received and sent across the network for all peers.  It is safe for concurrent access.
func (s *server) NetTotals() (uint64, uint64) {
	return atomic.LoadUint64(&s.bytesReceived),
		atomic.LoadUint64(&s.bytesSent)
}
// UpdatePeerHeights updates the heights of all peers who have have announced the latest connected main chain block, or a recognized orphan. These height updates allow us to dynamically refresh peer heights, ensuring sync peer selection has access to the latest block heights for each peer.
func (s *server) UpdatePeerHeights(latestBlkHash *chainhash.Hash, latestHeight int32, updateSource *peer.Peer) {
	s.peerHeightsUpdate <- updatePeerHeightsMsg{
		newHash:    latestBlkHash,
		newHeight:  latestHeight,
		originPeer: updateSource,
	}
}
// rebroadcastHandler keeps track of user submitted inventories that we have sent out but have not yet made it into a block. We periodically rebroadcast them in case our peers restarted or otherwise lost track of them.
func (s *server) rebroadcastHandler() {
	// Log<-cl.Debug{tarting rebroadcastHandler"
	// Wait 5 min before first tx rebroadcast.
	timer := time.NewTimer(5 * time.Minute)
	pendingInvs := make(map[wire.InvVect]interface{})
out:
	for {
		select {
		case riv := <-s.modifyRebroadcastInv:
			// fmt.Println("chan:riv := <-s.modifyRebroadcastInv")
			// Log<-cl.Debug{eceived modify rebroadcast inventory"
			switch msg := riv.(type) {
			// Incoming InvVects are added to our map of RPC txs.
			case broadcastInventoryAdd:
				// Log<-cl.Debug{roadcast inventory add"
				pendingInvs[*msg.invVect] = msg.data
			// When an InvVect has been added to a block, we can now remove it, if it was present.
			case broadcastInventoryDel:
				// Log<-cl.Debug{roadcast inventory delete"
				if _, ok := pendingInvs[*msg]; ok {
					delete(pendingInvs, *msg)
				}
			}
		case <-timer.C:
			// fmt.Println("chan:<-timer.C")
			// Any inventory we have has not made it into a block yet. We periodically resubmit them until they have.
			for iv, data := range pendingInvs {
				ivCopy := iv
				s.RelayInventory(&ivCopy, data)
			}
			// Process at a random time up to 30mins (in seconds) in the future.
			timer.Reset(time.Second *
				time.Duration(randomUint16Number(1800)))
		case <-s.quit:
			// fmt.Println("chan:<-s.quit")
			break out
			// default:
		}
	}
	timer.Stop()
	// Drain channels before exiting so nothing is left waiting around to send.
cleanup:
	for {
		select {
		case <-s.modifyRebroadcastInv:
		default:
			break cleanup
		}
	}
	s.wg.Done()
}
// Start begins accepting connections from peers.
func (s *server) Start() {
	// Log<-cl.Debug{tarting server"
	// Already started?
	if atomic.AddInt32(&s.started, 1) != 1 {
		// Log<-cl.Debug{lready started"
		return
	}
	log <- cl.Trace{"Starting server"}
	// Server startup time. Used for the uptime command for uptime calculation.
	s.startupTime = time.Now().Unix()
	// Start the peer handler which in turn starts the address and block managers.
	s.wg.Add(1)
	go s.peerHandler()
	if s.nat != nil {
		s.wg.Add(1)
		go s.upnpUpdateThread()
	}
	if !cfg.DisableRPC {
		s.wg.Add(1)
		// Log<-cl.Debug{tarting rebroadcast handler"
		// Start the rebroadcastHandler, which ensures user tx received by the RPC server are rebroadcast until being included in a block.
		go s.rebroadcastHandler()
		for i := range s.rpcServers {
			s.rpcServers[i].Start()
		}
	} else {
		panic("cannot run without RPC")
	}
	// Start the CPU miner if generation is enabled.
	if cfg.Generate {
		s.cpuMiner.Start()
	}
	if cfg.MinerListener != "" {
		s.minerController.Start()
	}
}
// Stop gracefully shuts down the server by stopping and disconnecting all peers and the main listener.
func (s *server) Stop() error {
	// Make sure this only happens once.
	if atomic.AddInt32(&s.shutdown, 1) != 1 {
		log <- cl.Infof{"Server is already in the process of shutting down"}
		return nil
	}
	log <- cl.Wrn("server shutting down")
	// Stop the CPU miner if needed
	s.cpuMiner.Stop()
	// Stop miner controller if needed
	s.minerController.Stop()
	// Shutdown the RPC server if it's not disabled.
	if !cfg.DisableRPC {
		for i := range s.rpcServers {
			s.rpcServers[i].Stop()
		}
	}
	// Save fee estimator state in the database.
	s.db.Update(func(tx database.Tx) error {
		metadata := tx.Metadata()
		metadata.Put(mempool.EstimateFeeDatabaseKey, s.feeEstimator.Save())
		return nil
	})
	// Signal the remaining goroutines to quit.
	close(s.quit)
	return nil
}
// WaitForShutdown blocks until the main listener and peer handlers are stopped.
func (s *server) WaitForShutdown() {
	s.wg.Wait()
}
// ScheduleShutdown schedules a server shutdown after the specified duration. It also dynamically adjusts how often to warn the server is going down based on remaining duration.
func (s *server) ScheduleShutdown(duration time.Duration) {
	// Don't schedule shutdown more than once.
	if atomic.AddInt32(&s.shutdownSched, 1) != 1 {
		return
	}
	log <- cl.Warnf{"Server shutdown in %v", duration}
	go func() {
		remaining := duration
		tickDuration := dynamicTickDuration(remaining)
		done := time.After(remaining)
		ticker := time.NewTicker(tickDuration)
	out:
		for {
			select {
			case <-done:
				// fmt.Println("chan:<-done")
				ticker.Stop()
				s.Stop()
				break out
			case <-ticker.C:
				// fmt.Println("chan:<-ticker.C")
				remaining = remaining - tickDuration
				if remaining < time.Second {
					continue
				}
				// Change tick duration dynamically based on remaining time.
				newDuration := dynamicTickDuration(remaining)
				if tickDuration != newDuration {
					tickDuration = newDuration
					ticker.Stop()
					ticker = time.NewTicker(tickDuration)
				}
				log <- cl.Warnf{"Server shutdown in %v", remaining}
			}
		}
	}()
}
// parseListeners determines whether each listen address is IPv4 and IPv6 and returns a slice of appropriate net.Addrs to listen on with TCP. It also properly detects addresses which apply to "all interfaces" and adds the address as both IPv4 and IPv6.
func parseListeners(addrs []string) ([]net.Addr, error) {
	netAddrs := make([]net.Addr, 0, len(addrs)*2)
	for _, addr := range addrs {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			// Shouldn't happen due to already being normalized.
			return nil, err
		}
		// Empty host or host of * on plan9 is both IPv4 and IPv6.
		if host == "" || (host == "*" && runtime.GOOS == "plan9") {
			netAddrs = append(netAddrs, simpleAddr{net: "tcp4", addr: addr})
			netAddrs = append(netAddrs, simpleAddr{net: "tcp6", addr: addr})
			continue
		}
		// Strip IPv6 zone id if present since net.ParseIP does not handle it.
		zoneIndex := strings.LastIndex(host, "%")
		if zoneIndex > 0 {
			host = host[:zoneIndex]
		}
		// Parse the IP.
		ip := net.ParseIP(host)
		if ip == nil {
			return nil, fmt.Errorf("'%s' is not a valid IP address", host)
		}
		// To4 returns nil when the IP is not an IPv4 address, so use this determine the address type.
		if ip.To4() == nil {
			netAddrs = append(netAddrs, simpleAddr{net: "tcp6", addr: addr})
		} else {
			netAddrs = append(netAddrs, simpleAddr{net: "tcp4", addr: addr})
		}
	}
	return netAddrs, nil
}
func (s *server) upnpUpdateThread() {
	// Go off immediately to prevent code duplication, thereafter we renew lease every 15 minutes.
	timer := time.NewTimer(0 * time.Second)
	lport, _ := strconv.ParseInt(ActiveNetParams.DefaultPort, 10, 16)
	first := true
out:
	for {
		select {
		case <-timer.C:
			// TODO: pick external port  more cleverly
			// TODO: know which ports we are listening to on an external net.
			// TODO: if specific listen port doesn't work then ask for wildcard
			// listen port?
			// XXX this assumes timeout is in seconds.
			listenPort, err := s.nat.AddPortMapping("tcp", int(lport), int(lport),
				"pod listen port", 20*60)
			if err != nil {
				log <- cl.Warnf{"can't add UPnP port mapping: %v", err}
			}
			if first && err == nil {
				// TODO: look this up periodically to see if upnp domain changed and so did ip.
				externalip, err := s.nat.GetExternalAddress()
				if err != nil {
					log <- cl.Warnf{"UPnP can't get external address: %v", err}
					continue out
				}
				na := wire.NewNetAddressIPPort(externalip, uint16(listenPort),
					s.services)
				err = s.addrManager.AddLocalAddress(na, addrmgr.UpnpPrio)
				if err != nil {
					// XXX DeletePortMapping?
				}
				log <- cl.Warnf{"Successfully bound via UPnP to %s", addrmgr.NetAddressKey(na)}
				first = false
			}
			timer.Reset(time.Minute * 15)
		case <-s.quit:
			fmt.Println("<-s.quit")
			break out
		}
	}
	timer.Stop()
	if err := s.nat.DeletePortMapping("tcp", int(lport), int(lport)); err != nil {
		log <- cl.Warnf{"unable to remove UPnP port mapping: %v", err}
	} else {
		log <- cl.Debugf{"successfully disestablished UPnP port mapping"}
	}
	s.wg.Done()
}
// setupRPCListeners returns a slice of listeners that are configured for use with the RPC server depending on the configuration settings for listen addresses and TLS.
func setupRPCListeners(urls []string) ([]net.Listener, error) {
	// Setup TLS if not disabled.
	listenFunc := net.Listen
	if cfg.TLS {
		// Generate the TLS cert and key file if both don't already exist.
		if !FileExists(cfg.RPCKey) && !FileExists(cfg.RPCCert) {
			err := genCertPair(cfg.RPCCert, cfg.RPCKey)
			if err != nil {
				return nil, err
			}
		}
		keypair, err := tls.LoadX509KeyPair(cfg.RPCCert, cfg.RPCKey)
		if err != nil {
			return nil, err
		}
		tlsConfig := tls.Config{
			Certificates: []tls.Certificate{keypair},
			MinVersion:   tls.VersionTLS12,
		}
		// Change the standard net.Listen function to the tls one.
		listenFunc = func(net string, laddr string) (net.Listener, error) {
			return tls.Listen(net, laddr, &tlsConfig)
		}
	}
	netAddrs, err := parseListeners(urls)
	if err != nil {
		return nil, err
	}
	listeners := make([]net.Listener, 0, len(netAddrs))
	for _, addr := range netAddrs {
		listener, err := listenFunc(addr.Network(), addr.String())
		if err != nil {
			log <- cl.Warnf{"Can't listen on %s: %v", addr, err}
			continue
		}
		listeners = append(listeners, listener)
	}
	return listeners, nil
}
// newServer returns a new pod server configured to listen on addr for the bitcoin network type specified by chainParams.  Use start to begin accepting connections from peers.
func newServer(listenAddrs []string, db database.DB, chainParams *chaincfg.Params, interruptChan <-chan struct{}, algo string) (*server, error) {
	services := defaultServices
	if cfg.NoPeerBloomFilters {
		services &^= wire.SFNodeBloom
	}
	if cfg.NoCFilters {
		services &^= wire.SFNodeCF
	}
	amgr := addrmgr.New(cfg.DataDir, podLookup)
	var listeners []net.Listener
	var nat NAT
	if !cfg.DisableListen {
		var err error
		listeners, nat, err = initListeners(amgr, listenAddrs, services)
		if err != nil {
			return nil, err
		}
		if len(listeners) == 0 {
			return nil, errors.New("no valid listen address")
		}
	}
	nthr := uint32(runtime.NumCPU())
	var thr uint32
	if cfg.GenThreads == -1 || thr > nthr {
		thr = uint32(nthr)
	} else {
		thr = uint32(cfg.GenThreads)
	}
	s := server{
		chainParams:          chainParams,
		addrManager:          amgr,
		newPeers:             make(chan *serverPeer, cfg.MaxPeers),
		donePeers:            make(chan *serverPeer, cfg.MaxPeers),
		banPeers:             make(chan *serverPeer, cfg.MaxPeers),
		query:                make(chan interface{}),
		relayInv:             make(chan relayMsg, cfg.MaxPeers),
		broadcast:            make(chan broadcastMsg, cfg.MaxPeers),
		quit:                 make(chan struct{}),
		modifyRebroadcastInv: make(chan interface{}),
		peerHeightsUpdate:    make(chan updatePeerHeightsMsg),
		nat:                  nat,
		db:                   db,
		timeSource:           blockchain.NewMedianTime(),
		services:             services,
		sigCache:             txscript.NewSigCache(cfg.SigCacheMaxSize),
		hashCache:            txscript.NewHashCache(cfg.SigCacheMaxSize),
		cfCheckptCaches:      make(map[wire.FilterType][]cfHeaderKV),
		numthreads:           thr,
		algo:                 algo,
	}
	// Create the transaction and address indexes if needed.
	// CAUTION: the txindex needs to be first in the indexes array because the addrindex uses data from the txindex during catchup.  If the addrindex is run first, it may not have the transactions from the current block indexed.
	var indexes []indexers.Indexer
	if cfg.TxIndex || cfg.AddrIndex {
		// Enable transaction index if address index is enabled since it requires it.
		if !cfg.TxIndex {
			log <- cl.Infof{"transaction index enabled because it " +
				"is required by the address index"}
			cfg.TxIndex = true
		} else {
			log <- cl.Info{"transaction index is enabled"}
		}
		s.txIndex = indexers.NewTxIndex(db)
		indexes = append(indexes, s.txIndex)
	}
	if cfg.AddrIndex {
		log <- cl.Info{"address index is enabled"}
		s.addrIndex = indexers.NewAddrIndex(db, chainParams)
		indexes = append(indexes, s.addrIndex)
	}
	if !cfg.NoCFilters {
		log <- cl.Info{"committed filter index is enabled"}
		s.cfIndex = indexers.NewCfIndex(db, chainParams)
		indexes = append(indexes, s.cfIndex)
	}
	// Create an index manager if any of the optional indexes are enabled.
	var indexManager blockchain.IndexManager
	if len(indexes) > 0 {
		indexManager = indexers.NewManager(db, indexes)
	}
	// Merge given checkpoints with the default ones unless they are disabled.
	var checkpoints []chaincfg.Checkpoint
	if !cfg.DisableCheckpoints {
		checkpoints = mergeCheckpoints(s.chainParams.Checkpoints, StateCfg.AddedCheckpoints)
	}
	// Create a new block chain instance with the appropriate configuration.
	var err error
	s.chain, err = blockchain.New(&blockchain.Config{
		DB:           s.db,
		Interrupt:    interruptChan,
		ChainParams:  s.chainParams,
		Checkpoints:  checkpoints,
		TimeSource:   s.timeSource,
		SigCache:     s.sigCache,
		IndexManager: indexManager,
		HashCache:    s.hashCache,
	})
	if err != nil {
		return nil, err
	}
	s.chain.DifficultyAdjustments = make(map[string]float64)
	// Search for a FeeEstimator state in the database. If none can be found or if it cannot be loaded, create a new one.
	db.Update(func(tx database.Tx) error {
		metadata := tx.Metadata()
		feeEstimationData := metadata.Get(mempool.EstimateFeeDatabaseKey)
		if feeEstimationData != nil {
			// delete it from the database so that we don't try to restore the same thing again somehow.
			metadata.Delete(mempool.EstimateFeeDatabaseKey)
			// If there is an error, log it and make a new fee estimator.
			var err error
			s.feeEstimator, err = mempool.RestoreFeeEstimator(feeEstimationData)
			if err != nil {
				log <- cl.Errorf{"Failed to restore fee estimator %v", err}
			}
		}
		return nil
	})
	// If no feeEstimator has been found, or if the one that has been found is behind somehow, create a new one and start over.
	if s.feeEstimator == nil || s.feeEstimator.LastKnownHeight() != s.chain.BestSnapshot().Height {
		s.feeEstimator = mempool.NewFeeEstimator(
			mempool.DefaultEstimateFeeMaxRollback,
			mempool.DefaultEstimateFeeMinRegisteredBlocks)
	}
	txC := mempool.Config{
		Policy: mempool.Policy{
			DisableRelayPriority: cfg.NoRelayPriority,
			AcceptNonStd:         cfg.RelayNonStd,
			FreeTxRelayLimit:     cfg.FreeTxRelayLimit,
			MaxOrphanTxs:         cfg.MaxOrphanTxs,
			MaxOrphanTxSize:      DefaultMaxOrphanTxSize,
			MaxSigOpCostPerTx:    blockchain.MaxBlockSigOpsCost / 4,
			MinRelayTxFee:        StateCfg.ActiveMinRelayTxFee,
			MaxTxVersion:         2,
		},
		ChainParams:    chainParams,
		FetchUtxoView:  s.chain.FetchUtxoView,
		BestHeight:     func() int32 { return s.chain.BestSnapshot().Height },
		MedianTimePast: func() time.Time { return s.chain.BestSnapshot().MedianTime },
		CalcSequenceLock: func(tx *util.Tx, view *blockchain.UtxoViewpoint) (*blockchain.SequenceLock, error) {
			return s.chain.CalcSequenceLock(tx, view, true)
		},
		IsDeploymentActive: s.chain.IsDeploymentActive,
		SigCache:           s.sigCache,
		HashCache:          s.hashCache,
		AddrIndex:          s.addrIndex,
		FeeEstimator:       s.feeEstimator,
	}
	s.txMemPool = mempool.New(&txC)
	s.syncManager, err = netsync.New(&netsync.Config{
		PeerNotifier:       &s,
		Chain:              s.chain,
		TxMemPool:          s.txMemPool,
		ChainParams:        s.chainParams,
		DisableCheckpoints: cfg.DisableCheckpoints,
		MaxPeers:           cfg.MaxPeers,
		FeeEstimator:       s.feeEstimator,
	})
	if err != nil {
		return nil, err
	}
	// Create the mining policy and block template generator based on the configuration options.
	// NOTE: The CPU miner relies on the mempool, so the mempool has to be created before calling the function to create the CPU miner.
	policy := mining.Policy{
		BlockMinWeight:    cfg.BlockMinWeight,
		BlockMaxWeight:    cfg.BlockMaxWeight,
		BlockMinSize:      cfg.BlockMinSize,
		BlockMaxSize:      cfg.BlockMaxSize,
		BlockPrioritySize: cfg.BlockPrioritySize,
		TxMinFreeFee:      StateCfg.ActiveMinRelayTxFee,
	}
	blockTemplateGenerator := mining.NewBlkTmplGenerator(&policy,
		s.chainParams, s.txMemPool, s.chain, s.timeSource,
		s.sigCache, s.hashCache, s.algo)
	s.cpuMiner = cpuminer.New(&cpuminer.Config{
		Blockchain:             s.chain,
		ChainParams:            chainParams,
		BlockTemplateGenerator: blockTemplateGenerator,
		MiningAddrs:            StateCfg.ActiveMiningAddrs,
		ProcessBlock:           s.syncManager.ProcessBlock,
		ConnectedCount:         s.ConnectedCount,
		IsCurrent:              s.syncManager.IsCurrent,
		NumThreads:             s.numthreads,
		Algo:                   s.algo,
	})
	s.minerController = controller.New(&controller.Config{
		Blockchain:             s.chain,
		ChainParams:            chainParams,
		BlockTemplateGenerator: blockTemplateGenerator,
		MiningAddrs:            StateCfg.ActiveMiningAddrs,
		ProcessBlock:           s.syncManager.ProcessBlock,
		MinerListener:          cfg.MinerListener,
		MinerKey:               StateCfg.ActiveMinerKey,
		ConnectedCount:         s.ConnectedCount,
		IsCurrent:              s.syncManager.IsCurrent,
	})
	/*	Only setup a function to return new addresses to connect to when
		not running in connect-only mode.  The simulation network is always
		in connect-only mode since it is only intended to connect to
		specified peers and actively avoid advertising and connecting to
		discovered peers in order to prevent it from becoming a public test
		network. */
	var newAddressFunc func() (net.Addr, error)
	if !cfg.SimNet && len(cfg.ConnectPeers) == 0 {
		newAddressFunc = func() (net.Addr, error) {
			for tries := 0; tries < 100; tries++ {
				addr := s.addrManager.GetAddress()
				if addr == nil {
					break
				}
				/*	Address will not be invalid, local or unroutable
					because addrmanager rejects those on addition.
					Just check that we don't already have an address
					in the same group so that we are not connecting
					to the same network segment at the expense of
					others. */
				key := addrmgr.GroupKey(addr.NetAddress())
				if s.OutboundGroupCount(key) != 0 {
					continue
				}
				// only allow recent nodes (10mins) after we failed 30 times
				if tries < 30 && time.Since(addr.LastAttempt()) < 10*time.Minute {
					continue
				}
				// allow nondefault ports after 50 failed tries.
				if tries < 50 && fmt.Sprintf("%d", addr.NetAddress().Port) !=
					ActiveNetParams.DefaultPort {
					continue
				}
				addrString := addrmgr.NetAddressKey(addr.NetAddress())
				return addrStringToNetAddr(addrString)
			}
			return nil, errors.New("no valid connect address")
		}
	}
	// Create a connection manager.
	targetOutbound := defaultTargetOutbound
	if cfg.MaxPeers < targetOutbound {
		targetOutbound = cfg.MaxPeers
	}
	cmgr, err := connmgr.New(&connmgr.Config{
		Listeners:      listeners,
		OnAccept:       s.inboundPeerConnected,
		RetryDuration:  connectionRetryInterval,
		TargetOutbound: uint32(targetOutbound),
		Dial:           podDial,
		OnConnection:   s.outboundPeerConnected,
		GetNewAddress:  newAddressFunc,
	})
	if err != nil {
		return nil, err
	}
	s.connManager = cmgr
	// Start up persistent peers.
	permanentPeers := cfg.ConnectPeers
	if len(permanentPeers) == 0 {
		permanentPeers = cfg.AddPeers
	}
	for _, addr := range permanentPeers {
		netAddr, err := addrStringToNetAddr(addr)
		if err != nil {
			return nil, err
		}
		go s.connManager.Connect(&connmgr.ConnReq{
			Addr:      netAddr,
			Permanent: true,
		})
	}
	if !cfg.DisableRPC {
		/*	Setup listeners for the configured RPC listen addresses and
			TLS settings. */
		listeners := map[string][]string{
			"sha256d": cfg.RPCListeners,
		}
		for l := range listeners {
			rpcListeners, err := setupRPCListeners(listeners[l])
			if err != nil {
				return nil, err
			}
			if len(rpcListeners) == 0 {
				return nil, errors.New("RPCS: No valid listen address")
			}
			rp, err := newRPCServer(&rpcserverConfig{
				Listeners:    rpcListeners,
				StartupTime:  s.startupTime,
				ConnMgr:      &rpcConnManager{&s},
				SyncMgr:      &rpcSyncMgr{&s, s.syncManager},
				TimeSource:   s.timeSource,
				Chain:        s.chain,
				ChainParams:  chainParams,
				DB:           db,
				TxMemPool:    s.txMemPool,
				Generator:    blockTemplateGenerator,
				CPUMiner:     s.cpuMiner,
				TxIndex:      s.txIndex,
				AddrIndex:    s.addrIndex,
				CfIndex:      s.cfIndex,
				FeeEstimator: s.feeEstimator,
				Algo:         l,
			})
			if err != nil {
				return nil, err
			}
			s.rpcServers = append(s.rpcServers, rp)
		}
		// Signal process shutdown when the RPC server requests it.
		go func() {
			for i := range s.rpcServers {
				<-s.rpcServers[i].RequestedProcessShutdown()
			}
			interrupt.Request()
		}()
	}
	return &s, nil
}
/*	initListeners initializes the configured net listeners and adds any bound
	addresses to the address manager. Returns the listeners and a NAT interface,
	which is non-nil if UPnP is in use. */
func initListeners(amgr *addrmgr.AddrManager, listenAddrs []string, services wire.ServiceFlag) ([]net.Listener, NAT, error) {
	// Listen for TCP connections at the configured addresses
	netAddrs, err := parseListeners(listenAddrs)
	if err != nil {
		return nil, nil, err
	}
	listeners := make([]net.Listener, 0, len(netAddrs))
	for _, addr := range netAddrs {
		listener, err := net.Listen(addr.Network(), addr.String())
		if err != nil {
			log <- cl.Warnf{"Can't listen on %s: %v", addr, err}
			continue
		}
		listeners = append(listeners, listener)
	}
	var nat NAT
	if len(cfg.ExternalIPs) != 0 {
		defaultPort, err := strconv.ParseUint(ActiveNetParams.DefaultPort, 10, 16)
		if err != nil {
			log <- cl.Errorf{"Can not parse default port %s for active chain: %v",
				ActiveNetParams.DefaultPort, err}
			return nil, nil, err
		}
		for _, sip := range cfg.ExternalIPs {
			eport := uint16(defaultPort)
			host, portstr, err := net.SplitHostPort(sip)
			if err != nil {
				// no port, use default.
				host = sip
			} else {
				port, err := strconv.ParseUint(portstr, 10, 16)
				if err != nil {
					log <- cl.Warnf{"Can not parse port from %s for " +
						"externalip: %v", sip, err}
					continue
				}
				eport = uint16(port)
			}
			na, err := amgr.HostToNetAddress(host, eport, services)
			if err != nil {
				log <- cl.Warnf{"Not adding %s as externalip: %v", sip, err}
				continue
			}
			err = amgr.AddLocalAddress(na, addrmgr.ManualPrio)
			if err != nil {
				log <- cl.Warnf{"Skipping specified external IP: %v", err}
			}
		}
	} else {
		if cfg.Upnp {
			var err error
			nat, err = Discover()
			if err != nil {
				log <- cl.Warnf{"Can't discover upnp: %v", err}
			}
			// nil nat here is fine, just means no upnp on network.
		}
		// Add bound addresses to address manager to be advertised to peers.
		for _, listener := range listeners {
			addr := listener.Addr().String()
			err := addLocalAddress(amgr, addr, services)
			if err != nil {
				log <- cl.Warnf{"Skipping bound address %s: %v", addr, err}
			}
		}
	}
	return listeners, nat, nil
}
/*	addrStringToNetAddr takes an address in the form of 'host:port' and returns
	a net.Addr which maps to the original address with any host names resolved
	to IP addresses.  It also handles tor addresses properly by returning a
	net.Addr that encapsulates the address. */
func addrStringToNetAddr(addr string) (net.Addr, error) {
	host, strPort, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(strPort)
	if err != nil {
		return nil, err
	}
	// Skip if host is already an IP address.
	if ip := net.ParseIP(host); ip != nil {
		return &net.TCPAddr{
			IP:   ip,
			Port: port,
		}, nil
	}
	// Tor addresses cannot be resolved to an IP, so just return an onion address instead.
	if strings.HasSuffix(host, ".onion") {
		if cfg.NoOnion {
			return nil, errors.New("tor has been disabled")
		}
		return &onionAddr{addr: addr}, nil
	}
	// Attempt to look up an IP address associated with the parsed host.
	ips, err := podLookup(host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no addresses found for %s", host)
	}
	return &net.TCPAddr{
		IP:   ips[0],
		Port: port,
	}, nil
}
/*	addLocalAddress adds an address that this node is listening on to the
	address manager so that it may be relayed to peers. */
func addLocalAddress(addrMgr *addrmgr.AddrManager, addr string, services wire.ServiceFlag) error {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return err
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		// If bound to unspecified address, advertise all local interfaces
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			return err
		}
		for _, addr := range addrs {
			ifaceIP, _, err := net.ParseCIDR(addr.String())
			if err != nil {
				continue
			}
			/*	If bound to 0.0.0.0, do not add IPv6 interfaces and if bound to
				::, do not add IPv4 interfaces. */
			if (ip.To4() == nil) != (ifaceIP.To4() == nil) {
				continue
			}
			netAddr := wire.NewNetAddressIPPort(ifaceIP, uint16(port), services)
			addrMgr.AddLocalAddress(netAddr, addrmgr.BoundPrio)
		}
	} else {
		netAddr, err := addrMgr.HostToNetAddress(host, uint16(port), services)
		if err != nil {
			return err
		}
		addrMgr.AddLocalAddress(netAddr, addrmgr.BoundPrio)
	}
	return nil
}
/*	dynamicTickDuration is a convenience function used to dynamically choose a
	tick duration based on remaining time.  It is primarily used during
	server shutdown to make shutdown warnings more frequent as the shutdown time
	approaches. */
func dynamicTickDuration(remaining time.Duration) time.Duration {
	switch {
	case remaining <= time.Second*5:
		return time.Second
	case remaining <= time.Second*15:
		return time.Second * 5
	case remaining <= time.Minute:
		return time.Second * 15
	case remaining <= time.Minute*5:
		return time.Minute
	case remaining <= time.Minute*15:
		return time.Minute * 5
	case remaining <= time.Hour:
		return time.Minute * 15
	}
	return time.Hour
}
/*	isWhitelisted returns whether the IP address is included in the whitelisted
	networks and IPs. */
func isWhitelisted(addr net.Addr) bool {
	if len(StateCfg.ActiveWhitelists) == 0 {
		return false
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		log <- cl.Warnf{"Unable to SplitHostPort on '%s': %v", addr, err}
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		log <- cl.Warnf{"Unable to parse IP '%s'", addr}
		return false
	}
	for _, ipnet := range StateCfg.ActiveWhitelists {
		if ipnet.Contains(ip) {
			return true
		}
	}
	return false
}
// checkpointSorter implements sort.Interface to allow a slice of checkpoints to be sorted.
type checkpointSorter []chaincfg.Checkpoint
// Len returns the number of checkpoints in the slice.  It is part of the sort.Interface implementation.
func (s checkpointSorter) Len() int {
	return len(s)
}
// Swap swaps the checkpoints at the passed indices.  It is part of the sort.Interface implementation.
func (s checkpointSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
/*	Less returns whether the checkpoint with index i should sort before the
	checkpoint with index j.  It is part of the sort.Interface implementation. */
func (s checkpointSorter) Less(i, j int) bool {
	return s[i].Height < s[j].Height
}
/*	mergeCheckpoints returns two slices of checkpoints merged into one slice
	such that the checkpoints are sorted by height.  In the case the additional
	checkpoints contain a checkpoint with the same height as a checkpoint in the
	default checkpoints, the additional checkpoint will take precedence and
	overwrite the default one. */
func mergeCheckpoints(defaultCheckpoints, additional []chaincfg.Checkpoint) []chaincfg.Checkpoint {
	/*	Create a map of the additional checkpoints to remove duplicates while
		leaving the most recently-specified checkpoint. */
	extra := make(map[int32]chaincfg.Checkpoint)
	for _, checkpoint := range additional {
		extra[checkpoint.Height] = checkpoint
	}
	// Add all default checkpoints that do not have an override in the additional checkpoints.
	numDefault := len(defaultCheckpoints)
	checkpoints := make([]chaincfg.Checkpoint, 0, numDefault+len(extra))
	for _, checkpoint := range defaultCheckpoints {
		if _, exists := extra[checkpoint.Height]; !exists {
			checkpoints = append(checkpoints, checkpoint)
		}
	}
	// Append the additional checkpoints and return the sorted results.
	for _, checkpoint := range extra {
		checkpoints = append(checkpoints, checkpoint)
	}
	sort.Sort(checkpointSorter(checkpoints))
	return checkpoints
}
package node
import (
	"fmt"
	"os"
	"path/filepath"
	"time"
	"github.com/btcsuite/winsvc/eventlog"
	"github.com/btcsuite/winsvc/mgr"
	"github.com/btcsuite/winsvc/svc"
)
const (
	// svcName is the name of pod service.
	svcName = "podsvc"
	// svcDisplayName is the service name that will be shown in the windows services list.  Not the svcName is the "real" name which is used to control the service.  This is only for display purposes.
	svcDisplayName = "Pod Service"
	// svcDesc is the description of the service.
	svcDesc = "Downloads and stays synchronized with the bitcoin block " +
		"chain and provides chain services to applications."
)
// elog is used to send messages to the Windows event log.
var elog *eventlog.Log
// logServiceStartOfDay logs information about pod when the main server has been started to the Windows event log.
func logServiceStartOfDay(srvr *server) {
	var message string
	message += fmt.Sprintf("Version %s\n", version())
	message += fmt.Sprintf("Configuration directory: %s\n", defaultHomeDir)
	message += fmt.Sprintf("Configuration file: %s\n", cfg.ConfigFile)
	message += fmt.Sprintf("Data directory: %s\n", cfg.DataDir)
	elog.Info(1, message)
}
// podService houses the main service handler which handles all service updates and launching podMain.
type podService struct{}
// Execute is the main entry point the winsvc package calls when receiving information from the Windows service control manager.  It launches the long-running podMain (which is the real meat of pod), handles service change requests, and notifies the service control manager of changes.
func (s *podService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	// Service start is pending.
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}
	// Start podMain in a separate goroutine so the service can start quickly.  Shutdown (along with a potential error) is reported via doneChan.  serverChan is notified with the main server instance once it is started so it can be gracefully stopped.
	doneChan := make(chan error)
	serverChan := make(chan *server)
	go func() {
		err := podMain(serverChan)
		doneChan <- err
	}()
	// Service is now started.
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	var mainServer *server
loop:
	for {
		select {
		case c := <-r:
			// fmt.Println("chan:c := <-r")
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				// Service stop is pending.  Don't accept any more commands while pending.
				changes <- svc.Status{State: svc.StopPending}
				// Signal the main function to exit.
				shutdownRequestChannel <- struct{}{}
			default:
				elog.Error(1, fmt.Sprintf(
					"Unexpected control request #%d.", c,
				))
			}
		case srvr := <-serverChan:
			// fmt.Println("chan:srvr := <-serverChan")
			mainServer = srvr
			logServiceStartOfDay(mainServer)
		case err := <-doneChan:
			// fmt.Println("chan:err := <-doneChan")
			if err != nil {
				elog.Error(1, err.Error())
			}
			break loop
		}
	}
	// Service is now stopped.
	changes <- svc.Status{State: svc.Stopped}
	return false, 0
}
// installService attempts to install the pod service.  Typically this should be done by the msi installer, but it is provided here since it can be useful for development.
func installService() error {
	// Get the path of the current executable.  This is needed because os.Args[0] can vary depending on how the application was launched. For example, under cmd.exe it will only be the name of the app without the path or extension, but under mingw it will be the full path including the extension.
	exePath, err := filepath.Abs(os.Args[0])
	if err != nil {
		return err
	}
	if filepath.Ext(exePath) == "" {
		exePath += ".exe"
	}
	// Connect to the windows service manager.
	serviceManager, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer serviceManager.Disconnect()
	// Ensure the service doesn't already exist.
	service, err := serviceManager.OpenService(svcName)
	if err == nil {
		service.Close()
		return fmt.Errorf("service %s already exists", svcName)
	}
	// Install the service.
	service, err = serviceManager.CreateService(svcName, exePath, mgr.Config{
		DisplayName: svcDisplayName,
		Description: svcDesc,
	})
	if err != nil {
		return err
	}
	defer service.Close()
	// Support events to the event log using the standard "standard" Windows EventCreate.exe message file.  This allows easy logging of custom messges instead of needing to create our own message catalog.
	eventlog.Remove(svcName)
	eventsSupported := uint32(eventlog.Error | eventlog.Warning | eventlog.Info)
	return eventlog.InstallAsEventCreate(svcName, eventsSupported)
}
// removeService attempts to uninstall the pod service.  Typically this should be done by the msi uninstaller, but it is provided here since it can be useful for development.  Not the eventlog entry is intentionally not removed since it would invalidate any existing event log messages.
func removeService() error {
	// Connect to the windows service manager.
	serviceManager, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer serviceManager.Disconnect()
	// Ensure the service exists.
	service, err := serviceManager.OpenService(svcName)
	if err != nil {
		return fmt.Errorf("service %s is not installed", svcName)
	}
	defer service.Close()
	// Remove the service.
	return service.Delete()
}
// startService attempts to start the pod service.
func startService() error {
	// Connect to the windows service manager.
	serviceManager, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer serviceManager.Disconnect()
	service, err := serviceManager.OpenService(svcName)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer service.Close()
	err = service.Start(os.Args)
	if err != nil {
		return fmt.Errorf("could not start service: %v", err)
	}
	return nil
}
// controlService allows commands which change the status of the service.  It also waits for up to 10 seconds for the service to change to the passed state.
func controlService(c svc.Cmd, to svc.State) error {
	// Connect to the windows service manager.
	serviceManager, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer serviceManager.Disconnect()
	service, err := serviceManager.OpenService(svcName)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer service.Close()
	status, err := service.Control(c)
	if err != nil {
		return fmt.Errorf("could not send control=%d: %v", c, err)
	}
	// Send the control message.
	timeout := time.Now().Add(10 * time.Second)
	for status.State != to {
		if timeout.Before(time.Now()) {
			return fmt.Errorf("timeout waiting for service to go "+
				"to state=%d", to)
		}
		time.Sleep(300 * time.Millisecond)
		status, err = service.Query()
		if err != nil {
			return fmt.Errorf("could not retrieve service "+
				"status: %v", err)
		}
	}
	return nil
}
// performServiceCommand attempts to run one of the supported service commands provided on the command line via the service command flag.  An appropriate error is returned if an invalid command is specified.
func performServiceCommand(command string) error {
	var err error
	switch command {
	case "install":
		err = installService()
	case "remove":
		err = removeService()
	case "start":
		err = startService()
	case "stop":
		err = controlService(svc.Stop, svc.Stopped)
	default:
		err = fmt.Errorf("invalid service command [%s]", command)
	}
	return err
}
// serviceMain checks whether we're being invoked as a service, and if so uses the service control manager to start the long-running server.  A flag is returned to the caller so the application can determine whether to exit (when running as a service) or launch in normal interactive mode.
func serviceMain() (bool, error) {
	// Don't run as a service if we're running interactively (or that can't be determined due to an error).
	isInteractive, err := svc.IsAnInteractiveSession()
	if err != nil {
		return false, err
	}
	if isInteractive {
		return false, nil
	}
	elog, err = eventlog.Open(svcName)
	if err != nil {
		return false, err
	}
	defer elog.Close()
	err = svc.Run(svcName, &podService{})
	if err != nil {
		elog.Error(1, fmt.Sprintf("Service start failed: %v", err))
		return true, err
	}
	return true, nil
}
// Set windows specific functions to real functions.
func init() {
	runServiceCommand = performServiceCommand
	winServiceMain = serviceMain
}
package node
import (
	"io"
	"os"
	"path/filepath"
	cl "git.parallelcoin.io/pod/pkg/clog"
)
// dirEmpty returns whether or not the specified directory path is empty.
func dirEmpty(dirPath string) (bool, error) {
	f, err := os.Open(dirPath)
	if err != nil {
		return false, err
	}
	defer f.Close()
	// Read the names of a max of one entry from the directory.  When the directory is empty, an io.EOF error will be returned, so allow it.
	names, err := f.Readdirnames(1)
	if err != nil && err != io.EOF {
		return false, err
	}
	return len(names) == 0, nil
}
// oldPodHomeDir returns the OS specific home directory pod used prior to version 0.3.3.  This has since been replaced with util.AppDataDir, but this function is still provided for the automatic upgrade path.
func oldPodHomeDir() string {
	// Search for Windows APPDATA first.  This won't exist on POSIX OSes.
	appData := os.Getenv("APPDATA")
	if appData != "" {
		return filepath.Join(appData, "pod")
	}
	// Fall back to standard HOME directory that works for most POSIX OSes.
	home := os.Getenv("HOME")
	if home != "" {
		return filepath.Join(home, ".pod")
	}
	// In the worst case, use the current directory.
	return "."
}
// upgradeDBPathNet moves the database for a specific network from its location prior to pod version 0.2.0 and uses heuristics to ascertain the old database type to rename to the new format.
func upgradeDBPathNet(oldDbPath, netName string) error {
	// Prior to version 0.2.0, the database was named the same thing for both sqlite and leveldb.  Use heuristics to figure out the type of the database and move it to the new path and name introduced with version 0.2.0 accordingly.
	fi, err := os.Stat(oldDbPath)
	if err == nil {
		oldDbType := "sqlite"
		if fi.IsDir() {
			oldDbType = "leveldb"
		}
		// The new database name is based on the database type and resides in a directory named after the network type.
		newDbRoot := filepath.Join(filepath.Dir(cfg.DataDir), netName)
		newDbName := blockDbNamePrefix + "_" + oldDbType
		if oldDbType == "sqlite" {
			newDbName = newDbName + ".db"
		}
		newDbPath := filepath.Join(newDbRoot, newDbName)
		// Create the new path if needed.
		err = os.MkdirAll(newDbRoot, 0700)
		if err != nil {
			return err
		}
		// Move and rename the old database.
		err := os.Rename(oldDbPath, newDbPath)
		if err != nil {
			return err
		}
	}
	return nil
}
// upgradeDBPaths moves the databases from their locations prior to pod version 0.2.0 to their new locations.
func upgradeDBPaths() error {
	// Prior to version 0.2.0, the databases were in the "db" directory and their names were suffixed by "testnet" and "regtest" for their respective networks.  Check for the old database and update it to the new path introduced with version 0.2.0 accordingly.
	oldDbRoot := filepath.Join(oldPodHomeDir(), "db")
	upgradeDBPathNet(filepath.Join(oldDbRoot, "pod.db"), "mainnet")
	upgradeDBPathNet(filepath.Join(oldDbRoot, "pod_testnet.db"), "testnet")
	upgradeDBPathNet(filepath.Join(oldDbRoot, "pod_regtest.db"), "regtest")
	// Remove the old db directory.
	return os.RemoveAll(oldDbRoot)
}
// upgradeDataPaths moves the application data from its location prior to pod version 0.3.3 to its new location.
func upgradeDataPaths() error {
	// No need to migrate if the old and new home paths are the same.
	oldHomePath := oldPodHomeDir()
	newHomePath := DefaultHomeDir
	if oldHomePath == newHomePath {
		return nil
	}
	// Only migrate if the old path exists and the new one doesn't.
	if FileExists(oldHomePath) && !FileExists(newHomePath) {
		// Create the new path.
		log <- cl.Infof{
			"migrating application home path from '%s' to '%s'",
			oldHomePath, newHomePath,
		}
		err := os.MkdirAll(newHomePath, 0700)
		if err != nil {
			return err
		}
		// Move old pod.conf into new location if needed.
		oldConfPath := filepath.Join(oldHomePath, DefaultConfigFilename)
		newConfPath := filepath.Join(newHomePath, DefaultConfigFilename)
		if FileExists(oldConfPath) && !FileExists(newConfPath) {
			err := os.Rename(oldConfPath, newConfPath)
			if err != nil {
				return err
			}
		}
		// Move old data directory into new location if needed.
		oldDataPath := filepath.Join(oldHomePath, DefaultDataDirname)
		newDataPath := filepath.Join(newHomePath, DefaultDataDirname)
		if FileExists(oldDataPath) && !FileExists(newDataPath) {
			err := os.Rename(oldDataPath, newDataPath)
			if err != nil {
				return err
			}
		}
		// Remove the old home if it is empty or show a warning if not.
		ohpEmpty, err := dirEmpty(oldHomePath)
		if err != nil {
			return err
		}
		if ohpEmpty {
			err := os.Remove(oldHomePath)
			if err != nil {
				return err
			}
		} else {
			log <- cl.Warnf{
				"not removing '%s' since it contains files not created by this application," +
					"you may want to manually move them or delete them.", oldHomePath}
		}
	}
	return nil
}
// doUpgrades performs upgrades to pod as new versions require it.
func doUpgrades() error {
	err := upgradeDBPaths()
	if err != nil {
		return err
	}
	return upgradeDataPaths()
}
package node
// Upnp code taken from Taipei Torrent license is below:
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//    * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//    * Redistributions in binary form must reproduce the above
// in the documentation and/or other materials provided with the
// distribution.
//    * Neither the name of Google Inc. nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
// Just enough UPnP to be able to forward ports
import (
	"bytes"
	"encoding/xml"
	"errors"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)
// NAT is an interface representing a NAT traversal options for example UPNP or NAT-PMP. It provides methods to query and manipulate this traversal to allow access to services.
type NAT interface {
	// Get the external address from outside the NAT.
	GetExternalAddress() (addr net.IP, err error)
	// Add a port mapping for protocol ("udp" or "tcp") from external port to internal port with description lasting for timeout.
	AddPortMapping(protocol string, externalPort, internalPort int, description string, timeout int) (mappedExternalPort int, err error)
	// Remove a previously added port mapping from external port to internal port.
	DeletePortMapping(protocol string, externalPort, internalPort int) (err error)
}
type upnpNAT struct {
	serviceURL string
	ourIP      string
}
// Discover searches the local network for a UPnP router returning a NAT for the network if so, nil if not.
func Discover() (nat NAT, err error) {
	ssdp, err := net.ResolveUDPAddr("udp4", "239.255.255.250:1900")
	if err != nil {
		return
	}
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return
	}
	socket := conn.(*net.UDPConn)
	defer socket.Close()
	err = socket.SetDeadline(time.Now().Add(3 * time.Second))
	if err != nil {
		return
	}
	st := "ST: urn:schemas-upnp-org:device:InternetGatewayDevice:1\r\n"
	buf := bytes.NewBufferString(
		"M-SEARCH * HTTP/1.1\r\n" +
			"HOST: 239.255.255.250:1900\r\n" +
			st +
			"MAN: \"ssdp:discover\"\r\n" +
			"MX: 2\r\n\r\n")
	message := buf.Bytes()
	answerBytes := make([]byte, 1024)
	for i := 0; i < 3; i++ {
		_, err = socket.WriteToUDP(message, ssdp)
		if err != nil {
			return
		}
		var n int
		n, _, err = socket.ReadFromUDP(answerBytes)
		if err != nil {
			continue
			// socket.Close()
			// return
		}
		answer := string(answerBytes[0:n])
		if !strings.Contains(answer, "\r\n"+st) {
			continue
		}
		// HTTP header field names are case-insensitive. http://www.w3.org/Protocols/rfc2616/rfc2616-sec4.html#sec4.2
		locString := "\r\nlocation: "
		locIndex := strings.Index(strings.ToLower(answer), locString)
		if locIndex < 0 {
			continue
		}
		loc := answer[locIndex+len(locString):]
		endIndex := strings.Index(loc, "\r\n")
		if endIndex < 0 {
			continue
		}
		locURL := loc[0:endIndex]
		var serviceURL string
		serviceURL, err = getServiceURL(locURL)
		if err != nil {
			return
		}
		var ourIP string
		ourIP, err = getOurIP()
		if err != nil {
			return
		}
		nat = &upnpNAT{serviceURL: serviceURL, ourIP: ourIP}
		return
	}
	err = errors.New("UPnP port discovery failed")
	return
}
// service represents the Service type in an UPnP xml description. Only the parts we care about are present and thus the xml may have more fields than present in the structure.
type service struct {
	ServiceType string `xml:"serviceType"`
	ControlURL  string `xml:"controlURL"`
}
// deviceList represents the deviceList type in an UPnP xml description. Only the parts we care about are present and thus the xml may have more fields than present in the structure.
type deviceList struct {
	XMLName xml.Name `xml:"deviceList"`
	Device  []device `xml:"device"`
}
// serviceList represents the serviceList type in an UPnP xml description. Only the parts we care about are present and thus the xml may have more fields than present in the structure.
type serviceList struct {
	XMLName xml.Name  `xml:"serviceList"`
	Service []service `xml:"service"`
}
// device represents the device type in an UPnP xml description. Only the parts we care about are present and thus the xml may have more fields than present in the structure.
type device struct {
	XMLName     xml.Name    `xml:"device"`
	DeviceType  string      `xml:"deviceType"`
	DeviceList  deviceList  `xml:"deviceList"`
	ServiceList serviceList `xml:"serviceList"`
}
// specVersion represents the specVersion in a UPnP xml description. Only the parts we care about are present and thus the xml may have more fields than present in the structure.
type specVersion struct {
	XMLName xml.Name `xml:"specVersion"`
	Major   int      `xml:"major"`
	Minor   int      `xml:"minor"`
}
// root represents the Root document for a UPnP xml description. Only the parts we care about are present and thus the xml may have more fields than present in the structure.
type root struct {
	XMLName     xml.Name `xml:"root"`
	SpecVersion specVersion
	Device      device
}
// getChildDevice searches the children of device for a device with the given type.
func getChildDevice(d *device, deviceType string) *device {
	for i := range d.DeviceList.Device {
		if d.DeviceList.Device[i].DeviceType == deviceType {
			return &d.DeviceList.Device[i]
		}
	}
	return nil
}
// getChildDevice searches the service list of device for a service with the given type.
func getChildService(d *device, serviceType string) *service {
	for i := range d.ServiceList.Service {
		if d.ServiceList.Service[i].ServiceType == serviceType {
			return &d.ServiceList.Service[i]
		}
	}
	return nil
}
// getOurIP returns a best guess at what the local IP is.
func getOurIP() (ip string, err error) {
	hostname, err := os.Hostname()
	if err != nil {
		return
	}
	return net.LookupCNAME(hostname)
}
// getServiceURL parses the xml description at the given root url to find the url for the WANIPConnection service to be used for port forwarding.
func getServiceURL(rootURL string) (url string, err error) {
	r, err := http.Get(rootURL)
	if err != nil {
		return
	}
	defer r.Body.Close()
	if r.StatusCode >= 400 {
		err = errors.New(string(r.StatusCode))
		return
	}
	var root root
	err = xml.NewDecoder(r.Body).Decode(&root)
	if err != nil {
		return
	}
	a := &root.Device
	if a.DeviceType != "urn:schemas-upnp-org:device:InternetGatewayDevice:1" {
		err = errors.New("no InternetGatewayDevice")
		return
	}
	b := getChildDevice(a, "urn:schemas-upnp-org:device:WANDevice:1")
	if b == nil {
		err = errors.New("no WANDevice")
		return
	}
	c := getChildDevice(b, "urn:schemas-upnp-org:device:WANConnectionDevice:1")
	if c == nil {
		err = errors.New("no WANConnectionDevice")
		return
	}
	d := getChildService(c, "urn:schemas-upnp-org:service:WANIPConnection:1")
	if d == nil {
		err = errors.New("no WANIPConnection")
		return
	}
	url = combineURL(rootURL, d.ControlURL)
	return
}
// combineURL appends subURL onto rootURL.
func combineURL(rootURL, subURL string) string {
	protocolEnd := "://"
	protoEndIndex := strings.Index(rootURL, protocolEnd)
	a := rootURL[protoEndIndex+len(protocolEnd):]
	rootIndex := strings.Index(a, "/")
	return rootURL[0:protoEndIndex+len(protocolEnd)+rootIndex] + subURL
}
// soapBody represents the <s:Body> element in a SOAP reply. fields we don't care about are elided.
type soapBody struct {
	XMLName xml.Name `xml:"Body"`
	Data    []byte   `xml:",innerxml"`
}
// soapEnvelope represents the <s:Envelope> element in a SOAP reply. fields we don't care about are elided.
type soapEnvelope struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    soapBody `xml:"Body"`
}
// soapRequests performs a soap request with the given parameters and returns the xml replied stripped of the soap headers. in the case that the request is unsuccessful the an error is returned.
func soapRequest(url, function, message string) (replyXML []byte, err error) {
	fullMessage := "<?xml version=\"1.0\" ?>" +
		"<s:Envelope xmlns:s=\"http://schemas.xmlsoap.org/soap/envelope/\" s:encodingStyle=\"http://schemas.xmlsoap.org/soap/encoding/\">\r\n" +
		"<s:Body>" + message + "</s:Body></s:Envelope>"
	req, err := http.NewRequest("POST", url, strings.NewReader(fullMessage))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "text/xml ; charset=\"utf-8\"")
	req.Header.Set("User-Agent", "Darwin/10.0.0, UPnP/1.0, MiniUPnPc/1.3")
	//req.Header.Set("Transfer-Encoding", "chunked")
	req.Header.Set("SOAPAction", "\"urn:schemas-upnp-org:service:WANIPConnection:1#"+function+"\"")
	req.Header.Set("Connection", "Close")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if r.Body != nil {
		defer r.Body.Close()
	}
	if r.StatusCode >= 400 {
		// log.Stderr(function, r.StatusCode)
		err = errors.New("Error " + strconv.Itoa(r.StatusCode) + " for " + function)
		r = nil
		return
	}
	var reply soapEnvelope
	err = xml.NewDecoder(r.Body).Decode(&reply)
	if err != nil {
		return nil, err
	}
	return reply.Body.Data, nil
}
// getExternalIPAddressResponse represents the XML response to a GetExternalIPAddress SOAP request.
type getExternalIPAddressResponse struct {
	XMLName           xml.Name `xml:"GetExternalIPAddressResponse"`
	ExternalIPAddress string   `xml:"NewExternalIPAddress"`
}
// GetExternalAddress implements the NAT interface by fetching the external IP from the UPnP router.
func (n *upnpNAT) GetExternalAddress() (addr net.IP, err error) {
	message := "<u:GetExternalIPAddress xmlns:u=\"urn:schemas-upnp-org:service:WANIPConnection:1\"/>\r\n"
	response, err := soapRequest(n.serviceURL, "GetExternalIPAddress", message)
	if err != nil {
		return nil, err
	}
	var reply getExternalIPAddressResponse
	err = xml.Unmarshal(response, &reply)
	if err != nil {
		return nil, err
	}
	addr = net.ParseIP(reply.ExternalIPAddress)
	if addr == nil {
		return nil, errors.New("unable to parse ip address")
	}
	return addr, nil
}
// AddPortMapping implements the NAT interface by setting up a port forwarding from the UPnP router to the local machine with the given ports and protocol.
func (n *upnpNAT) AddPortMapping(protocol string, externalPort, internalPort int, description string, timeout int) (mappedExternalPort int, err error) {
	// A single concatenation would break ARM compilation.
	message := "<u:AddPortMapping xmlns:u=\"urn:schemas-upnp-org:service:WANIPConnection:1\">\r\n" +
		"<NewRemoteHost></NewRemoteHost><NewExternalPort>" + strconv.Itoa(externalPort)
	message += "</NewExternalPort><NewProtocol>" + strings.ToUpper(protocol) + "</NewProtocol>"
	message += "<NewInternalPort>" + strconv.Itoa(internalPort) + "</NewInternalPort>" +
		"<NewInternalClient>" + n.ourIP + "</NewInternalClient>" +
		"<NewEnabled>1</NewEnabled><NewPortMappingDescription>"
	message += description +
		"</NewPortMappingDescription><NewLeaseDuration>" + strconv.Itoa(timeout) +
		"</NewLeaseDuration></u:AddPortMapping>"
	response, err := soapRequest(n.serviceURL, "AddPortMapping", message)
	if err != nil {
		return
	}
	// TODO: check response to see if the port was forwarded
	// If the port was not wildcard we don't get an reply with the port in it. Not sure about wildcard yet. miniupnpc just checks for error codes here.
	mappedExternalPort = externalPort
	_ = response
	return
}
// DeletePortMapping implements the NAT interface by removing up a port forwarding from the UPnP router to the local machine with the given ports and.
func (n *upnpNAT) DeletePortMapping(protocol string, externalPort, internalPort int) (err error) {
	message := "<u:DeletePortMapping xmlns:u=\"urn:schemas-upnp-org:service:WANIPConnection:1\">\r\n" +
		"<NewRemoteHost></NewRemoteHost><NewExternalPort>" + strconv.Itoa(externalPort) +
		"</NewExternalPort><NewProtocol>" + strings.ToUpper(protocol) + "</NewProtocol>" +
		"</u:DeletePortMapping>"
	response, err := soapRequest(n.serviceURL, "DeletePortMapping", message)
	if err != nil {
		return
	}
	// TODO: check response to see if the port was deleted log.Println(message, response)
	_ = response
	return
}
package node
import (
	"bytes"
	"fmt"
	"strings"
)
// semanticAlphabet
const semanticAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"
// These constants define the application version and follow the semantic versioning 2.0.0 spec (http://semver.org/).
const (
	appMajor uint = 0
	appMinor uint = 1
	appPatch uint = 4
	// appPreRelease MUST only contain characters from semanticAlphabet per the semantic versioning spec.
	appPreRelease = "beta"
)
// appBuild is defined as a variable so it can be overridden during the build process with '-ldflags "-X main.appBuild foo' if needed.  It MUST only contain characters from semanticAlphabet per the semantic versioning spec.
var appBuild string
// Version returns the application version as a properly formed string per the semantic versioning 2.0.0 spec (http://semver.org/).
func Version() string {
	// Start with the major, minor, and patch versions.
	version := fmt.Sprintf("%d.%d.%d", appMajor, appMinor, appPatch)
	// Append pre-release version if there is one.  The hyphen called for by the semantic versioning spec is automatically appended and should not be contained in the pre-release string.  The pre-release version is not appended if it contains invalid characters.
	preRelease := normalizeVerString(appPreRelease)
	if preRelease != "" {
		version = fmt.Sprintf("%s-%s", version, preRelease)
	}
	// Append build metadata if there is any.  The plus called for by the semantic versioning spec is automatically appended and should not be contained in the build metadata string.  The build metadata string is not appended if it contains invalid characters.
	build := normalizeVerString(appBuild)
	if build != "" {
		version = fmt.Sprintf("%s+%s", version, build)
	}
	return version
}
// normalizeVerString returns the passed string stripped of all characters which are not valid according to the semantic versioning guidelines for pre-release version and build metadata strings.  In particular they MUST only contain characters in semanticAlphabet.
func normalizeVerString(str string) string {
	var result bytes.Buffer
	for _, r := range str {
		if strings.ContainsRune(semanticAlphabet, r) {
			result.WriteRune(r)
		}
	}
	return result.String()
}
