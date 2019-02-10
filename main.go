package main

import (
	"github.com/l0k1verloren/skele/pkg/cmd"
)

var (
	// CMD is a shortcut for creating and naming a node
	CMD, VAR = cmd.Name, cmd.NameType
	mainApp  = CMD("pod").
			Append(
			CMD("datadir"),
			CMD("ctl").Append(
				CMD("list")),

			CMD("conf").Append(
				CMD("init"),
				CMD("show")),
			CMD("gui"),
			CMD("node").Append(
				CMD("dropaddrindex"),
				CMD("droptxindex"),
				CMD("reindex"),
			),
			CMD("setup"),
			CMD("shell"),
			CMD("wallet"),
		)
)

func main() {}
