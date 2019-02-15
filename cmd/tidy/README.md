
# tidy

Keeps your go source files in a consistent structure.

Are you finding as your project gets larger, you more and more often have problems finding where things are?

Having problems with mysterious disappearing values or other symptoms of the most common type of bug in Go: variable shadowing, and probably equally vexatious, variable names differing only by case, causing data to evaporate or come out of nowhere.

With tidy, your code stays in a logical and consistent root level structure that makes large source files much more managable.

## Functions

### Root level block sort and group

Groups all top level sections of source into thte following order:

- package header
- imports block
- types block
- const block
- var block
- functions
  - main function always is moved to the top of the function block

This program does only section reordering, it does not temper with anything inside any of the types of block in a go source file.

If you want to take advantage of this reordering on const, var and type, make a separate declaration for each item, and they will then be part of the sort process.

This tool is deliberately extremely simple and only interacts with the syntax on the root level of Go source files. It should be perfectly safe to use this as part of a code beautification routine, especially as a final step as it is completely retarded in terms of its awareness of Go syntax.

A possible use of tidy can be to concatenate all of the source files in a directory together, in order to untangle types that have ended up with declarations in several files when they need to be worked on in context with other parts that they are separated from, for example. Tidy does not collapse the package header and preceding comment lines, they

## Usage

    tidy <go source file> [output file]

    By default, tidy rewrites the given source file if only one filename is given.

## Caveats

- The section splitter depends on the root level keywords being at the start of the line. Any root level keyword not appearing as the first part of a line will be missed.
