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
var ss, sss, zz []string
var e error
var i, j, k, start int
var keylist []int
var sections map[string][]string

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
	for i, x = range ss {
		if len(x) > 7 && matchstart("package", x[:7]) {
			keylist = append(keylist, i)
		}
	}
	for i, x = range ss {
		if len(x) > 6 && matchstart("import", x[:6]) {
			keylist = append(keylist, i)
		}
	}
	for i, x = range ss {
		if len(x) > 4 && matchstart("type", x[:4]) {
			keylist = append(keylist, i)
		}
	}
	for i, x = range ss {
		if len(x) > 5 && matchstart("const", x[:5]) {
			keylist = append(keylist, i)
		}
	}
	for i, x = range ss {
		if len(x) > 3 && matchstart("var", x[:3]) {
			keylist = append(keylist, i)
		}
	}
	for i, x = range ss {
		if len(x) > 4 && matchstart("func", x[:4]) {
			keylist = append(keylist, i)
		}
	}
	sections = make(map[string][]string)
	sort.Ints(keylist)

	fnd = true
	sss = []string{}
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
