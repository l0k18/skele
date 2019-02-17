package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

var f = os.Stdin
var b []byte
var s, fname, tc, tp, tn string
var ss []string
var e error
var i int

func main() {
	if len(os.Args) > 1 {
		fname = os.Args[1]
		if b, e = ioutil.ReadFile(os.Args[1]); e != nil {
			panic(e)
		}
		// if f, e = os.Create(fname); e != nil {
		// 	panic(e)
		// }
		s = string(b)
		ss = strings.Split(s, "\n")
		for i, s = range ss {
			if i > 1 &&
				len(ss[i-1]) > 1 &&
				matchEnd("{", strings.Trim(ss[i-1], "\t\n\r ")) &&
				!matchEnd("{", strings.Trim(s, "\t\n\r ")) {
				fmt.Fprintln(os.Stdout, "")
			}
			fmt.Fprintln(os.Stdout, s)
		}
	} else {
		fmt.Fprintln(os.Stderr, "go uncommenter - removes comments from a file and rewrites it")
		fmt.Fprintln(os.Stderr, "\n\n\tusage: uncomment <filename.go>")
	}
}
func match(s1, s2 string) bool {
	if len(s1) <= len(s2) {
		if s1 == s2[:len(s1)] {
			return true
		}
	}
	return false
}
func matchEnd(s1, s2 string) bool {
	if len(s1) <= len(s2) {
		if s1 == s2[len(s1):] {
			return true
		}
	}
	return false
}
