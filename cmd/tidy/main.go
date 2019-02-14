//
package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/parallelcointeam/skele/cmd/tidy/its1"
	"github.com/parallelcointeam/skele/cmd/tidy/its2"
)

// main entrypoint to tidy
func main() {
	if len(os.Args) > 1 {
		infile = os.Args[1]
		if infile == "stdin" {
			f = os.Stdin
		} else {
			if readBuffer, e = ioutil.ReadFile(os.Args[1]); e != nil {
				panic(e)
			}
		}
	} else {
		printHelp()
	}
	if len(os.Args) > 2 {
		outfile = os.Args[2]
		if f, e = os.Create(outfile); e != nil {
			panic(e)
		}
	} else {
		f = os.Stdout
	}
	scanner := bufio.NewScanner(strings.NewReader(string(readBuffer)))
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		lineBuffer = append(lineBuffer, scanner.Text())
	}
	if e := scanner.Err(); e != nil {
		panic(e)
	}
	// lineBuffer = removeBlankLines(lineBuffer)
	sectBuffer = section(lineBuffer)
}

func removeBlankLines(in []string) (out []string) {
	for _, x := range in {
		if len(x) > 0 {
			out = append(out, x)
		}
	}
	return
}

// token long comment
//
//
//
func section(s1 []string) (s2 [][][]string) {
	keyMap := make(sectMap)
	var keyList []int
	i1 := its1.Create(s1)
	// find and gather line numbers of all root level keywords at the start of the line
	for i1.Goto(0); i1.OK(); {
		if its2.IsKey(i1.Get()) {
			// This makes a map between key lines and their original position
			keyMap[i1.Get()] = append(keyMap[i1.Get()], i1.Cur())
			// This allows finding the ends of each position from the original
			keyList = append(keyList, i1.Cur())
		}
		i1.Next()
	}
	// find the start of the comments above each section
	for i, x := range keyMap {
		i1.Goto(x[0])
		for {
			// fmt.Println()
			i1.Prev()
			for ; i1.MatchStart("//") && i1.Cur() > 1; i1.Prev() {
				fmt.Println(i1.Cur(), i1.Get())
			}
			if strings.Contains(i1.Get(), "*/") {
				for ; !i1.MatchStart("/*"); i1.Prev() {
				}
			}
			if !i1.MatchStart("//") ||
				!i1.MatchStart("/*") ||
				i1.Cur() == 0 {
				if len(i1.Get()) < 1 {
					i1.Next()
				}
				keyMap[i] = append(keyMap[i], i1.Cur())
				fmt.Println(x, i)
				fmt.Println("FOUND @", i1.Cur(), "::", i1.Next(), "::", i1.Next(), "::", i1.Next())
				break
			}
			i1.Next()
			keyMap[i] = append(keyMap[i], i1.Cur())
		}
	}
	i1.Goto(0)

	// spew.Dump(keyMap)

	var sorted []string
	for x := range keyMap {
		sorted = append(sorted, x)
	}
	sort.Strings(sorted)

	// spew.Dump(sorted)

	for i, x := range keyMap {
		for j, y := range keyList {
			if x[0] == y && j < len(keyList)-1 {
				keyMap[i] = append(x, keyList[j+1])
			}
		}
	}
	// spew.Dump(keyMap)

	collated := make(map[string][]string)
	for _, x := range its2.Keys {
		collated[x] = []string{}
	}
	for _, x := range sorted {
		for j, y := range keyMap {
			if x == j {
				kk := strings.Split(x, " ")
				keyword := kk[0]
				start := y[1]
				end := len(s1) - 1
				if len(y) > 2 {
					end = y[2]
				}
				for i := start; i < end-1; i++ {
					collated[keyword] = append(collated[keyword], s1[i])
				}
			}
		}
	}

	for _, x := range its2.Keys {
		fmt.Println("//", x)
		for _, y := range collated[x] {
			fmt.Println("\t", y)
		}
	}

	return
}

// match returns true if the second string is at least as long and the second string's first part matches the first
func match(s1, s2 string) bool {
	if len(s1) <= len(s2) {
		if s1 == s2[:len(s1)] {
			return true
		}
	}
	return false
}

// hasKey returns true if a key was found in the line
func hasKey(s string) (int, bool) {
	it := its1.Create(its2.Keys)
	for it.OK() {
		if it.MatchStart(s) {
			return it.Cur(), true
		}
	}
	return 0, false
}

// printHelp prints the help
func printHelp() {
	fmt.Print(`go source tidy

usage: tidy <infile> [outfile]

reads go source files, cleans and cuts them into individual declarations, groups and sorts them

use 'stdin' as filename to read from stdin

multiple source files concatenated and fed to stdin automatically consolidates the imports, but will error if there is more than one package specified - and duplicate file scope symbols are not handled automatically

`)
	os.Exit(1)
}

// sectMap stores the key lines mapped to their original line position and allows
type sectMap map[string][]int

// token constant
const pi = 3.1415927

// error
//
var e error
var infile, outfile string
var f *os.File
var readBuffer []byte
var lineBuffer []string

/* token multiline
comment
*/
var sectBuffer [][][]string
var chute int
