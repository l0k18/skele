package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
)

var fnd bool
var f = os.Stdin
var b []byte
var s, fname, tc, tp, tn, x, y, z, prevs string
var ss, sss, zz, zzz []string
var e error
var i, j, k, start int
var keylist []int
var sections map[string][]string
var types, consts, vars, funcs [][]string

func main() {
	_, e := os.Stdin.Stat()
	if e != nil {
		panic(e)
	}
	b, e = ioutil.ReadAll(os.Stdin)
	if e != nil {
		panic(e)
	}
	s = string(b)
	ss = strings.Split(s, "\n")
	fnd = false

	for _, x = range ss {
		if len(x) > 0 &&
			!matchstart("//", x) {
			zz = append(zz, x)
		}
	}
	ss = zz
	zz = nil

	sections = make(map[string][]string)
	sort.Ints(keylist)

	for i, x = range ss {
		fmt.Println(x)
		if matchstart("package", x) {
			fmt.Println()
			break
		}
	}

	fnd = true
	sss = nil
	for i = range ss {
		if matchstart("import", ss[i]) && fnd {
			for j, x = range ss[i:] {
				if x == ")" {
					break
				}
				if matchstart(x, "import (") {
					continue
				}
				sss = append(sss, strings.TrimSpace(x))
			}
		}
	}
	zz = []string{"import ("}
	sort.Strings(sss)
	for i = range sss {
		if i > 0 {
			if sss[i] == zz[len(zz)-1] {
				continue
			}
		}
		zz = append(zz, sss[i])
	}
	zz = append(zz, ")")
	for i := range zz {
		if i != 0 && i != len(zz)-1 {
			fmt.Print("\t")
		}
		fmt.Println(zz[i])
	}
	fmt.Println()
	for i = range keylist {
		if matchstart("type", ss[i]) {
			fmt.Println(i, ss[i])
		}
	}

	i = -1
	fnd = false
	for _, x = range ss {
		if matchstart("type", x) {
			i++
			types = append(types, []string{})
			fnd = true
		}
		if fnd {
			types[i] = append(types[i], x)
			if matchend("{", x) ||
				matchend("(", x) {
				continue
			}
			if matchstart("}", x) ||
				matchstart("//", x) ||
				len(x) < 1 {
				fnd = false
				continue
			}
		}
	}
	for _, zz = range types {
		zzz = append(zzz, zz[0])
	}
	sort.Strings(zzz)
	for _, x = range zzz {
		for _, zz = range types {
			if x == zz[0] {
				for _, y = range zz {
					fmt.Println(y)
				}
			}
		}
		fmt.Println()
	}
	zzz = nil
	i = -1
	fnd = false
	for _, x = range ss {
		if matchstart("const ", x) {
			i++
			consts = append(consts, []string{})
			fnd = true
		}
		if fnd {
			consts[i] = append(consts[i], x)
			if matchend("{", x) ||
				matchend("(", x) {
				continue
			} else {
				fnd = false
			}
		}
	}
	for _, zz = range consts {
		zzz = append(zzz, zz[0])
	}
	sort.Strings(zzz)
	for _, x = range zzz {
		for _, zz = range consts {
			if x == zz[0] {
				for _, y = range zz {
					fmt.Println(y)
				}
			}
		}
		fmt.Println()
	}

}

func iskeyline(line string) (bb bool) {
	if matchstart("type", line) ||
		matchstart("const", line) ||
		matchstart("var", line) ||
		matchstart("func", line) {
		bb = true
	}
	return
}

func matchstart(key, line string) (bb bool) {
	line = strings.Trim(line, "\t\r\n ")
	if len(line) >= len(key) {
		return line[:len(key)] == key
	}
	return
}

func matchend(key, line string) (bb bool) {
	line = strings.Trim(line, "\t\r\n ")
	if len(line) >= len(key) {
		return line[len(line)-len(key):] == key
	}
	return
}

func unline(s string) string {
	if len(s) < 1 {
		return ""
	}
	return s
}
