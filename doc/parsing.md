# Parsing

just some notes


Parameter list is a slice of strings created by separating by spaces - available from os.Args

Parse command is called with parameter list on root Command.

Parse command scans one string at a time to identify a scope keyword, load them into the data structure

If it is a command, feed the remaining paramaters not consumed by the parser

Parser exhaustively explores every matching node and fills all the structures