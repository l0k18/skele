
# tidy

Keeps your go source files in a consistent structure.

## Functions

### Root level block sort and group

Groups all top level sections of source into thte following order:

- package header
- imports block
- types block
- const block
- var block
- functions

### Function block sort

The function section is sorted in case sensitive lexical order, and then the subsections are reversed to put the methods adjacent to the preceding types block:

- exported package scope functions
- pointer receiver methods  
- value receiver methods
- unexported package scope functions


## Usage

    tidy <go source file> [output file]

## Caveats

- The section splitter depends on the root level keywords being at the start of the line. Any root level keyword not appearing as the first part of a line will be missed.
