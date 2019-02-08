# skele

## Simple golang project framework

### Command line is just another function call syntax

Have you ever thought about the fact that command lines are basically just function calls? Well, stop treating them differently, use skele.

Instead of confusing `-` and `--` prefixes, the parameter names themselves are just sequences that unfold some specific syntax or other, such as 

    rpc 127.0.0.1:11048 u loki p pa55word s 127.0.0.1:11046

Here we see the parser finds 'rpc' which in this case is the full node RPC, which can have certain things following it before the block is considered closed:

rpc means next follows specs from an RPC client connection. Only one thing parsing as an address is allowed in the block, and u, user, and username, case insensitive, could replace the u, or any stupid prefixing punctuation (hah! i said it!), and p/pass/password are keywords belonging under rpc, and above, the `s` is not part of the 'rpc' set, but rather the rpcserver set, and so marks the end of the rpc block.

The address above does not need a specific prefix keyword because it contains the pattern of an address, but generic strings must have a prefix keyword and are indistinguishable, and if unlabeled are interpreted into the order from the specification.

Command line parameters are supposed to be easy for humans to type, otherwise you you compile it.

### Logging should be everywhere

A simple and low-overhead channel based logging system with easy per-package drop-in boilerplate, and closures for heavy log jobs infrequently used. Switch out its' stream writer to write logs to files, subscribers, log rotators. Using channels for logging means not interfering so much with the main goroutines.

Logging is very helpful for diagnosing errors that have slipped past many layers of effort to eliminate them and can help improve user experience as well. The same model could be used to create a journalling system to implement undo and concurrent versioning.

