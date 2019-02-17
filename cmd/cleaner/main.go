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
var s, fname, tc, tp, tn, x, y, z string
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
	for j = range ss {
		if matchstart("package", ss[i]) {
			// fmt.Printf("%s\n", ss[i])
			break
		}
	}

	fnd = true
	sss = []string{}
	for i = range ss {
		if matchstart("import", ss[i]) && fnd {
			for j, x = range ss[i+1:] {
				if x == ")" {
					break
				}
				// fmt.Println(x)
				sss = append(sss, x)
			}
		}
	}
	sort.Strings(sss)
	zz = append(append(zz, ss[0]), "import (")
	for i, x = range sss {
		if z != x {
			zz = append(zz, x)
		}
		z = x
	}
	for i, z = range zz {
		fmt.Println(z)
	}
	fmt.Println(")")

	// sss = []string{}

	for i = range ss {
		if matchstart("type", ss[i]) && fnd {
			fmt.Println(ss[i])
			for j = range ss[i] {
				fmt.Println(ss[i])
			}
		}
	}

	sort.Strings(sss)
	// fmt.Println(sss)
	for i, x = range sss {
		if z != x {
			zz = append(zz, x)
		}
		z = x
	}
	for i, z = range zz {
		fmt.Println(z)
	}
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
