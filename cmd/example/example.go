package main

import (
	"fmt"
	"os"
	"time"

	sk "github.com/l0k1verloren/skele"
)

func main() {
	c := sk.Cmd()
	name := func(name string) sk.Commander {
		return sk.Attach(c).NAME(name)
	}
	c.
		NAME("example").
		VERS("v0.0.0").
		AUTH("Loki Verloren <l0k18@protonmail.com>").
		LCNS("dedicated to public domain").
		DESC("just an example deployment of skele").
		HELP("pre", `This is some preformatted text
with newlines
and so on

...
`).
		HELP("markdown", `# Markdown

This is markdown *text* and ~~this~~ is a hyperlink [https://google.com](https://google.com)
`).
		PAIR(sk.Pair{
			K: "duration",
			V: name("repeat").
				DESC("do it again").
				PAIR(sk.Pair{
					V: sk.Duration(time.Minute * 2),
				})},
			sk.Pair{
				K: "command",
				V: name("ctl").
					DESC("send RPC commands and print responses").
					HELP("pre",
						`sends commands to a bitcoin protocol compliant RPC node/wallet endpoint, or with parallelcoin extensions, and prints the replies from the server`).
					FUNC(func() error {
						return nil
					}).
					PAIR(sk.Pair{
						K: "command",
						V: sk.Attach(c).
							NAME("help").
							DESC("prints help on ctl").
							FUNC(func() error {
								fmt.Println("help text")
								os.Exit(0)
								return nil
							}),
					}),
			}, sk.Pair{
				K: "int",
				V: name("number").
					DESC("just a thing to put more in the syntax tree"),
			}).
		FUNC(func() error {
			fmt.Println("help text")
			os.Exit(0)
			return nil
		})

	// fmt.Println(c.String())
	// c.Function()
	// fmt.Println(os.Args)
	c.Scan(os.Args)
}
