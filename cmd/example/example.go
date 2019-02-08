package main

import (
	"fmt"

	"github.com/l0k1verloren/skele"
)

func main() {
	c := skele.Cmd().
		N("example").
		V("v0.0.0").
		A("Loki Verloren <l0k18@protonmail.com>").
		L("dedicated to public domain").
		B("just an example deployment of skele").
		H("pre", `This is some preformatted text
with newlines
and so on

...
`).
		H("markdown", `# Markdown

This is markdown *text* and ~~this~~ is a hyperlink [https://google.com](https://google.com)
`)
	fmt.Println(c.String())
}
