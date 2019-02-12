package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

type (
	imported struct {
		pkg     []string
		imports []string
		types   [][]string
		consts  [][]string
		vars    [][]string
		funcs   [][]string
	}
)

const (
	PACKAGE = iota
	IMPORTS
	TYPES
	CONSTS
	VARS
	FUNCS
)

var (
	sections [][][]string
	out      io.Writer = os.Stdout
	err      error
)

func main() {

	if len(
		os.
			Args,
	) > 1 {

		if len(
			os.
				Args,
		) > 2 {

			if _, err :=
				os.
					Stat(
						os.
							Args[2],
					); !os.
				IsNotExist(
					err,
				) {

				err = os.
					Remove(
						os.
							Args[2],
					)
				if err !=
					nil {

					fmt.
						Println(
							err,
						)
					panic(
						err,
					)
				}
			}

			out, err = os.
				OpenFile(
					os.
						Args[2],
					os.
						O_CREATE|
						os.
							O_RDWR,
					0600,
				)
			if err != nil {

				fmt.Println(err)
				panic(err)
			}
		}

		b, err := ioutil.ReadFile(os.Args[1])
		if err != nil {

			panic(err)
		}
		var splitted []string
		splitted = strings.Split(string(b), "\n")
		// splitted = clean(splitted)
		splitted = rejoinSplitLines(splitted)
		printStrings(splitted)
		_ = splitted
	} else {

		printHelp()
	}
}

func rejoinSplitLines(
	s []string) []string {

	ignoreList := []string{
		"import (",
		"var (",
		"const (",
		"type (",
	}

	continuers := []byte{
		'{', '(', ',', '+', '-', '&', '|', '=', '*', '/', '.',
	}

	iter := getLineIter(s)
	current := iter.get()
	for {

		lastChar := getNthLastChar(
			current,
			1,
		)
		if isComment(current) {
			goto next
		}

		for _, x := range ignoreList {

			if x == current {

				goto next
			}
		}

		for _, x := range continuers {

			secondLast := getNthLastChar(current, 2)
			if lastChar == x {

				if secondLast == '+' ||
					secondLast == '-' {

					break
				}

				// fmt.Println("lastChar", string(lastChar))

				switch lastChar {
				case '{':
					if secondLast != ' ' {

						joinWithNext(s, iter.i)
						// joinWithNext(s, iter.i)
						iter.prev()
					}
				case '(', '=', '&', '|':
					// fmt.Println("terminal '" + current + "'")
					if lastChar != '(' {
						s[iter.i] += " "
					}

					joinWithNext(s, iter.i)
					iter.prev()

				case ',':
					c := s[iter.i+1][0]
					if c == ')' ||
						c == '}' {

						s[iter.i] = removeNLastChars(current, 1)

						joinWithNext(s, iter.i)
						iter.prev()
					} else if c == '"' {

						joinWithNext(s, iter.i)
						iter.prev()
					} else {

						s[iter.i] += " "
						joinWithNext(s, iter.i)
						iter.prev()
					}

				case '.':
					// fmt.Println("terminal .")
					joinWithNext(s, iter.i)
					iter.prev()
				}
			}
		}
	next:
		current = iter.next()
		if !iter.moved {

			break
		}
	}
	return s
}

func joinWithNext(lines []string, pos int) {

	if pos != len(lines)-1 {

		current := lines[pos] + strings.TrimSpace(lines[pos+1])
		before := lines[:pos]
		before = append(before, current)
		after := lines[pos+2:]
		lines = append(before, after...)
	}
}

func getNthLastChar(s string, n int) byte {

	if n >= len(s) {
		return 0
	}

	return s[len(s)-n]
}

func removeNLastChars(s string, n int) (o string) {
	o = s[:len(s)-n]
	return
}

func clean(l []string) (lines []string) {

	var bt, ap, qu bool
	// lines = strings.Split(s, "\n")
	lines = l
	i := -1
	for {

		i++
		if i >= len(lines) {
			fmt.Println("i >= lines")
			break
		}

		x := lines[i]
		// if hasRootKeyword(x) {
		// 	lines[i] = x
		// 	continue
		// }
		// fmt.Println(i, x)

		// if isComment(x) {
		// 	lines[i] = x
		// 	continue
		// }

		var first, last string
		for _, y := range x {

			once := false
			for first != last || !once {

				once = true
				switch y {

				case '`':
					if first == "" {
						first = "`"
					}

					last = "`"
					bt = !bt

				case '\'':
					if first == "" {
						first = "'"
					}

					last = "'"
					ap = !ap

				case '"':
					if first == "" {
						first = "\""
					}

					last = `"`
					qu = !qu

				}
				joinWithNext(lines, i)
			}
		}

		if len(x) < 1 &&
			i < len(lines) {

			lines = append(lines[:i], lines[i+1:]...)
		}

		x = strings.TrimSpace(x)
		if i > len(lines)-1 {
			break
		}

		lines[i] = removeDoubleWhitespace(x)
	}

	return
}

func print(i ...interface{}) {

	fmt.Fprint(out, i...)
}

func printf(f string, v ...interface{}) {

	fmt.Fprintf(out, f, v...)
}

func println(v ...interface{}) {

	fmt.Fprintln(out, v...)
}

func printStrings(s []string) {

	for _, x := range s {

		fmt.Fprintln(out, x)
	}
}

func insertBlankLine(lines []string, position int) []string {

	if position < len(lines) {
		temp := lines[position]
		l1 := append(lines[:position], temp)
		l1 = append(l1, "")
		l2 := lines[position:]
		lines = append(l1, l2...)
	}

	return lines
}

func insertLine(lines []string, line string, pos int) {

	if pos < len(lines)-1 {
		before := append(lines[:pos], line)
		after := lines[pos:]
		lines = append(before, after...)
	}
}

func removeDoubleWhitespace(s string) string {

	if len(s) < 1 {
		return ""
	}

	var prev rune
	var temp string

	for j, x := range s {

		if j > 0 {

			if x == ' ' || x == '\t' || x == '\n' {

				if prev == ' ' ||
					prev == '\t' ||
					prev == '\n' {

					continue
				}
			}
		}

		temp += string(x)
		prev = x
	}

	return temp
}

func printHelp() {
	fmt.Printf(`go source tidy

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

// func tidyFile() {
// 	var i int
// 	var l []string
// 	var keyLines []int
// 	var keyText []string
// 	var sectMap []map[string]map[int][]string
// 	var fmap []string

// 	for i, x := range lines {
// 		switch {
// 		case hasRootKeyword(x):
// 			keyLines = append(keyLines, i)
// 			keyText = append(keyText, x)
// 		}
// 	}

// 	li := getLineIter(lines)
// 	adjusted := []int{0}
// 	for i, x := range keyLines {
// 		if i == 0 {
// 			continue
// 		}

// 		li.i = x
// 		for li.prev(); isComment(li.get()) &&
// 			li.i > 0; li.prev() {
// 		}

// 		adjusted = append(adjusted, li.i+1)
// 	}

// 	prev := adjusted[0]
// 	sections = make([][][]string, 6)
// 	for i = range adjusted {
// 		if i >= len(adjusted)-1 {
// 			l = lines[adjusted[i]:]
// 		} else {
// 			l = lines[prev:adjusted[i+1]]
// 			prev = adjusted[i+1]
// 		}

// 		section := []string{}
// 		for _, x := range l {
// 			section = append(section, x)
// 		}

// 		// spew.Dump(section)
// 		switch lines[keyLines[i]][0] {
// 		case 'p':
// 			sections[PACKAGE] = append(sections[PACKAGE], section)
// 		case 'i':
// 			sections[IMPORTS] = append(sections[IMPORTS], section)
// 		case 't':
// 			sections[TYPES] = append(sections[TYPES], section)
// 		case 'c':
// 			sections[CONSTS] = append(sections[CONSTS], section)
// 		case 'v':
// 			sections[VARS] = append(sections[VARS], section)
// 		case 'f':
// 			sections[FUNCS] = append(sections[FUNCS], section)
// 		}
// 	}

// 	// for i, x := range sections {
// 	// 	fmt.Println(i)
// 	// 	spew.Dump(x)
// 	// }

// 	for i, x := range sections {
// 		sectMap = append(sectMap, make(map[string]map[int][]string))
// 		for j, y := range x {
// 			for _, z := range y {
// 				if isComment(z) {
// 					continue
// 				}

// 				sectMap[i][z] = make(map[int][]string)
// 				sectMap[i][z][j] = sections[i][j]
// 				break
// 			}
// 		}
// 	}

// 	// for i, x := range sectMap {
// 	// 	fmt.Println(i)
// 	// 	spew.Dump(x)
// 	// }

// 	for i := range sectMap[FUNCS] {
// 		fmap = append(fmap, i)
// 	}

// 	sort.Strings(fmap)
// 	for _, x := range sectMap[PACKAGE] {
// 		for _, y := range x {
// 			for _, z := range y {
// 				print(z, "\n")
// 			}
// 		}

// 		print("\n")
// 	}

// 	if len(sectMap[IMPORTS]) > 0 {
// 		print("import (\n")
// 		for _, x := range sectMap[IMPORTS] {
// 			for _, y := range x {
// 				sort.Strings(y)
// 				var internal, external []string
// 				for _, z := range y {
// 					if len(z) < 1 {
// 						continue
// 					}

// 					if strings.Contains(z, ".") {
// 						external = append(external, z)
// 					} else if strings.Contains(z, "\"") {
// 						internal = append(internal, z)
// 					}
// 				}

// 				for _, a := range internal {
// 					print(a, "\n")
// 				}

// 				if len(internal) > 0 {
// 					print("\n")
// 				}

// 				bounds := false
// 				for _, a := range external {
// 					if a[:2] == "\t\"" {
// 						print(a, "\n")
// 					} else {
// 						if !bounds {
// 							bounds = true
// 							print("\n")
// 						}

// 						print(a, "\n")
// 					}
// 				}
// 			}
// 		}

// 		print(")\n\n")
// 	}

// 	print("// section:", keywords[TYPES], "s\n\n")
// 	if len(sectMap[TYPES]) > 0 {
// 		print("type (\n")
// 		for _, x := range sectMap[TYPES] {
// 			for i, y := range x {
// 				if len(y) == 0 {
// 					continue
// 				}

// 				if i != 0 {
// 					print("\n")
// 				}

// 				// Put prefix comment above uncommented type blocks
// 				if y[0][0] != '/' {
// 					print("\t// ")
// 					if strings.Contains(y[0], "(") {
// 						print("type block\n\n")
// 					} else {
// 						s := strings.Split(y[0], " ")
// 						print(s[1], " is not documented\n")
// 					}
// 				}

// 				for _, z := range y {
// 					if len(z) > 4 && z[:5] == "type " {
// 						z = z[5:]
// 					}

// 					if len(z) != 0 {
// 						print("\t", z, "\n")
// 					}
// 				}
// 			}
// 		}

// 		print(")\n\n")
// 	}

// 	print("// section:", keywords[CONSTS], "s\n\n")
// 	items := make(map[string][]string)
// 	if len(sectMap[CONSTS]) > 0 {
// 		print("const (\n")
// 		// fmt.Println(sectMap[CONSTS])
// 		var t = make([][]string, len(sectMap[CONSTS]))
// 		for _, x := range sectMap[CONSTS] {
// 			for j, y := range x {
// 				for iter := getLineIter(y); ; iter.next() {
// 					if iter.i > len(iter.ss)-2 {
// 						break
// 					}

// 					v := iter.get()
// 					if len(v) >= 7 && v[:7] == "const (" ||
// 						len(v) > 0 && v[:1] == ")" {
// 						y = append(y[:iter.i], y[iter.i+1:]...)
// 					}

// 					if len(v) < 1 ||
// 						isComment(v) {
// 						continue
// 					}

// 					b := strings.TrimSpace(v)
// 					if b[0] >= 'A' && b[0] <= 'Z' {
// 						var ii int
// 						var xx rune
// 						for ii, xx = range b {
// 							if xx == ' ' || xx == '\t' {
// 								break
// 							}
// 						}

// 						expFunc := b[:ii]
// 						for isComment(iter.prev()) && iter.moved {
// 						}

// 						for iter.next(); isComment(iter.get()); iter.next() {
// 							t[j] = append(t[j], iter.get())
// 						}

// 						comment := "\t// " + expFunc + " is SHAZAM"
// 						if len(t) < 2 {
// 							t[j] = append(t[j], comment)
// 							items[t[j][0]] = t[j]
// 							t[j] = append(t[j], iter.get())
// 						} else {
// 							topline := t[j][0][3:]
// 							switch {
// 							case topline[:2] == "A ":
// 								topline = topline[2:]
// 							case topline[:4] == "The ":
// 								topline = topline[4:]
// 							}

// 							if topline[:len(expFunc)] == expFunc {
// 								items[t[j][0]] = t[j]
// 							} else {
// 								t[j] = append(t[j], iter.get())
// 								t[j] = append(t[j], comment)
// 								items[t[j][0]] = t[j]
// 							}
// 						}
// 					}
// 				}

// 				for _, g := range t[j] {
// 					fmt.Println(g)
// 				}

// 				for _, g := range items {
// 					spew.Dump(g)
// 				}

// 				var tmp []string
// 				for i := range items {
// 					tmp = append(tmp, i)
// 				}

// 				y = nil
// 				sort.Strings(tmp)
// 				for i, z := range tmp {
// 					fmt.Println(i, z)
// 					y = append(y, z)
// 				}

// 				spew.Dump(y)
// 				for i, z := range y {
// 					// if len(z) < 1 {
// 					// 	continue
// 					// }
// 					if z != "const (" &&
// 						z != ")" &&
// 						i > 0 {
// 						prev := y[i-1]
// 						if !isComment(prev) {
// 							b := strings.TrimSpace(z)
// 							if b[0] >= 'A' && b[0] <= 'Z' {
// 								var ii int
// 								var xx rune
// 								for ii, xx = range b {
// 									if xx == ' ' || xx == '\t' {
// 										break
// 									}
// 								}

// 								print("\t// ", b[:ii], " is")
// 							}
// 						}

// 						if i > 0 {
// 							print("\n")
// 						}
// 					}

// 					print(z, "\n")
// 					if len(z) > 5 && z[:6] == "const " {
// 						z = z[6:]
// 					}
// 				}
// 			}
// 		}

// 		print(")\n\n")
// 	}

// 	if len(sectMap[VARS]) > 0 {
// 		print("var (\n")
// 		for _, x := range sectMap[VARS] {
// 			for _, y := range x {
// 				for _, z := range y {
// 					if len(z) < 1 {
// 						continue
// 					}

// 					if z != "var (" && z != ")" {
// 						if z[0] == '/' {
// 							z = "\t" + z
// 						}

// 						if len(z) > 3 && z[:4] == "var " {
// 							z = "\t" + z[4:]
// 						}

// 						print(z, "\n")
// 					}
// 				}
// 			}
// 		}

// 		print(")\n\n")
// 	}

// 	print("// section:", keywords[FUNCS], "s\n\n")
// 	if len(sectMap[FUNCS]) > 0 {
// 		for _, x := range sectMap[FUNCS] {
// 			for _, y := range x {
// 				for _, z := range y {
// 					print(z, "\n")
// 				}
// 			}
// 		}
// 	}
// }

func isComment(l string) bool {
	t := strings.TrimSpace(l)
	if len(t) > 1 &&
		t[:2] == "//" {
		return true
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
	ss    []string
	i     int
	moved bool
}

func getLineIter(s []string) iL {
	return iL{s, 0, true}
}

func (r *iL) next() string {
	r.moved = true
	if r.i < len(r.ss)-1 {
		r.i++
		return r.ss[r.i]
	}
	r.moved = false
	return ""
}

func (r *iL) prev() string {
	r.moved = true
	if r.i > 0 {
		r.i--
		return r.ss[r.i]
	}
	r.moved = false
	return ""
}

func (r *iL) get() string {
	r.moved = true
	if r.i > len(r.ss)-1 {
		r.i = len(r.ss) - 1
		r.moved = false
		return ""
	}

	return r.ss[r.i]
}
