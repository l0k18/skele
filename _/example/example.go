package main

import (
	"fmt"
	"os"

	sk "github.com/l0k1verloren/skele"
)

func main() {
	c := sk.Cmd()
	// tidy up
	NAME := func(i sk.Commander, n string) sk.Commander {
		return sk.Attach(i).NAME(n)
	}
	ctl := sk.Pair{
		K: "command",
		V: NAME(nil, "ctl").
			DESC("send RPC commands and print responses").
			HELP("pre",
				`sends commands to a bitcoin protocol compliant RPC node/wallet endpoint, or with parallelcoin extensions, and prints the replies from the server`).
			FUNC(func() error {
				return nil
			})}
	c.Scan(os.Args)

	helpFunc := sk.Pair{K: "command",
		V: sk.Attach(c).
			NAME("help").
			DESC("prints help on ctl").
			FUNC(func() error {
				fmt.Println("help text")
				os.Exit(0)
				return nil
			})}
	main :=
		NAME(nil, "example").
			VERS("v0.0.0").
			AUTH("Loki Verloren <l0k18@protonmail.com>").
			LCNS("dedicated to public domain").
			DESC("just an example deployment of skele").
			HELP("pre", `This is some preformatted text`).
			HELP("markdown", `# Markdown`).
			FUNC(func() error {
				fmt.Println("help text")
				os.Exit(0)
				return nil
			})

}
