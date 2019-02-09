# skele

## Simple golang project framework

### Command line is just another function call syntax

Have you ever thought about the fact that command lines are basically just function calls? Well, stop treating them differently, use skele.

Instead of confusing `-` and `--` prefixes, the parameter names themselves are just sequences that unfold some specific syntax or other. To make the parser as simple as possible, we make a simple set of rules.

An example off a skele syntax invocation of the pod wallet:

    pod l debug d test wallet rpc addr 127.0.0.1:11048 u user p password s addr 127.0.0.1:11046

This can be easily decomposed into this grammar tree by preferentially scanning for keywords, and if a symbol completely matches the first part of a symbol it is interpreted as that symbol. The next symbol is interpreted as the value of that key.

pod (executable filename)
- loglevel debug
- datadir test
- rpcconnect
  - address 127.0.0.1:11048
  - username user
  - password password
- server
  - address 127.0.0.1:11046

The grammar is rigid, in that you must put the keywords in there, but flexible in that you only have to type usually only one character to signify a keyword.

There is two types of keywords, commands and pairs. Pairs are in arrays attached to commands, and can contain commands. Commands are followed by pairs of keyword and value.

Due to the parsing procedure, it is not possible for any keyword to be the same between a parent command and child, however this is not an onerous restriction because if one needs two addresses they probably relate to two different things, and won't appear in this configuration.



### Environment Variables

Environment variables are also searched for matches. Their construction matches the hierarchy of the tree for parsing CLI commands - 

