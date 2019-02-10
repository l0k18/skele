package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strings"
)

var err error

func main() {
	if len(os.Args) > 1 {
		if len(os.Args) > 2 {
			out, err = os.OpenFile(os.Args[2], os.O_CREATE|os.O_RDWR, 0600)
			if err != nil {
				panic(err)
			}
		}
		tidyFile()
	} else {
		printHelp()
	}
}

func printHelp() {
	print(`go source tidy
	
usage:

	%s <source file> [output file]

- cuts source file into pieces, recomposes in the following order:

	package, import, type, const, var, main, funcs sorted alphabetically
	 
- prints to stdout or if a filename is given and opens, to a file

- (to be implemented, in order of priority) -->

  - joins separate base sections so there is one import, type, const, var in a bracket surrounded block

  - break all parameter lists and literal blocks into one per line comma separated no final comma if they expand a line past 72 characters

  - join contiguous // comments into one line, automatically add above exported declarations and sync to variable name and the word 'is' if nothing exists

  - sort fields of struct, map and interface, declarations and named field struct literals

`, os.Args[0])
}

var sections [][][]string

const (
	_p = iota
	_i
	_t
	_c
	_v
	_f
)

var out io.Writer = os.Stdout

func print(i ...interface{}) {
	fmt.Fprint(out, i...)
}

func tidyFile() {
	b, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	s := string(b)
	lines := strings.Split(s, "\n")
	for i, x := range lines {
		if len(x) < 1 && i < len(lines) {
			lines = append(lines[:i], lines[i+1:]...)
		}
	}
	var keyLines []int
	var keyText []string
	for i, x := range lines {
		switch {
		case hasRootKeyword(x):
			keyLines = append(keyLines, i)
			keyText = append(keyText, x)
		}
	}
	li := getLineIter(lines)
	var adjusted []int
	for i, x := range keyLines {
		if i == 0 {
			continue
		}
		li.i = x
		for li.prev(); isComment(li.get()); li.prev() {
		}
		adjusted = append(adjusted, li.i+1)
	}
	var i, prev int
	sections = make([][][]string, 6)
	for i = range adjusted {
		l := lines[prev:adjusted[i]]
		prev = adjusted[i]
		var section []string
		for _, x := range l {
			section = append(section, x)
		}
		section = append(section, "")
		switch lines[keyLines[i]][0] {
		case 'p':
			sections[_p] = append(sections[_p], section)
		case 'i':
			sections[_i] = append(sections[_i], section)
		case 't':
			sections[_t] = append(sections[_t], section)
		case 'c':
			sections[_c] = append(sections[_c], section)
		case 'v':
			sections[_v] = append(sections[_v], section)
		case 'f':
			sections[_f] = append(sections[_f], section)
		}
	}
	var sectMap []map[string]map[int][]string
	for i := range sections {
		sectMap = append(sectMap, make(map[string]map[int][]string))
		for j := range sections[i] {
			for _, x := range sections[i][j] {
				if isComment(x) {
					continue
				}
				sectMap[i][x] = make(map[int][]string)
				sectMap[i][x][j] = sections[i][j]
				break
			}
		}
	}
	var fmap []string
	for i := range sectMap[5] {
		fmap = append(fmap, i)
	}
	sort.Strings(fmap)
	for _, x := range sectMap[_p] {
		for _, y := range x {
			for _, z := range y {
				print(z, "\n")
			}
		}
	}
	print("// section:", keywords[_i], "s\n\n")
	print("import (\n")
	for _, x := range sectMap[_i] {
		for _, y := range x {
			sort.Strings(y)
			var internal, external []string
			for _, z := range y {
				if strings.Contains(z, ".") {
					external = append(external, z)
				} else if strings.Contains(z, "\"") {
					internal = append(internal, z)
				}
			}
			for _, a := range internal {
				print(a, "\n")
			}
			print("\n")
			for _, a := range external {
				print(a, "\n")
			}
		}
	}
	print(")\n\n")
	print("// section:", keywords[_t], "s\n\n")
	for _, x := range sectMap[_t] {
		for _, y := range x {
			// Put prefix comment above uncommented type blocks
			print("FIRST LINE ", y[0], "\n")

			for _, z := range y {
				print(z, "\n")
			}
		}
	}
	print(sectMap[_t], "\n\n")

	print("// section:", keywords[_c], "s\n\n")
	print(sectMap[_c], "\n\n")
	print("// section:", keywords[_v], "s\n\n")
	print(sectMap[_v], "\n\n")
	print("// section:", keywords[_f], "s\n\n")
	for _, x := range fmap {
		for _, y := range sectMap[5][x] {
			for _, z := range y {
				print(z)
			}
		}
	}
}

func isComment(l string) bool {
	if len(l) > 1 {
		x := 0
		// skip spaces
		for ; charIsOneOf(l[x], '\t', ' '); x++ {
		}
		if l[x:x+2] == "//" {
			return true
		}
	}
	return false
}

var keywords = []string{
	"package", "import", "type", "const", "var", "func",
}

func hasRootKeyword(l string) bool {
	for _, x := range keywords {
		if len(x) <= len(l) {
			if x == l[:len(x)] {
				return true
			}
		}
	}
	return false
}

func charIsOneOf(a byte, b ...byte) bool {
	for _, x := range b {
		if x == a {
			return true
		}
	}
	return false
}

type iL struct {
	ss []string
	i  int
	bool
}

func getLineIter(s []string) iL {
	return iL{s, 0, true}
}

func (r *iL) next() string {
	r.bool = true
	if r.i < len(r.ss)-1 {
		r.i++
		return r.ss[r.i]
	}
	r.bool = false
	return ""
}

func (r *iL) prev() string {
	r.bool = true
	if r.i > 0 {
		r.i--
		return r.ss[r.i]
	}
	return ""
}

func (r *iL) get() string {
	return r.ss[r.i]
}
