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
		switch infile {
		case "stdin":
			f = os.Stdin
		default:
			if readBuffer, e = ioutil.ReadFile(os.Args[1]); e != nil {
				panic(e)
			}
		}
	} else {
		printHelp()
	}
	outfile = infile
	if len(os.Args) > 2 {
		outfile = os.Args[2]
	}
	if f, e = os.Create(outfile); e != nil {
		panic(e)
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
	fmt.Fprintln(f, sectBuffer)
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
func section(s1 []string) (s2 string) {
	keyMap := make(sectMap)
	i1 := its1.Create(s1)
	// find and gather line numbers of all root level keywords at the start of the line
	for i1.Zero(); i1.OK(); {
		if its2.IsKey(i1.Get()) {
			// This makes a map between key lines and their original position
			keyMap[i1.Get()] = append(keyMap[i1.Get()], i1.Cur())
		}
		i1.Next()
	}

	// find the start of the comments above each section
	for i, x := range keyMap {
		i1.Goto(x[0])
		for {
			i1.Prev()
			if IsKey(i1.Get()) {
				keyMap[i] = append(keyMap[i], keyMap[i][0])
				break
			}
			for ; i1.MatchStart("//") && i1.Cur() > 1; i1.Prev() {
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
				break
			}
			i1.Next()
			keyMap[i] = append(keyMap[i], i1.Cur())
		}
	}
	i1.Zero()

	// sort the keymap
	var sorted []string
	for x := range keyMap {
		sorted = append(sorted, x)
	}
	sort.Strings(sorted)

	hasMain := false
	for _, x := range sorted {
		if x == "func main() {" {
			hasMain = true
		}
	}

	// skm (sorted key map) Assemble section keymap entry array for final composition
	var skm []string
	order := []string{
		"package",
		"import",
		"type",
		"const",
		"var",
		"func",
	}

	ord := its1.Create(order)
	item := its1.Create(sorted)
	for ord.Zero(); ord.OK(); ord.Next() {
		if ord.Get() == "func" {
			if hasMain {
				// main always first function
				skm = append(skm, "func main() {")
			}
		}
		for item.Zero(); item.OK(); item.Next() {
			if match(ord.Get(), item.Get()) {
				if item.Get() == "func main() {" {
					// main already at the top
					continue
				}
				skm = append(skm, item.Get())
			}
		}
		item.Zero()
	}

	var sectMarkers []int
	for _, x := range keyMap {
		sectMarkers = append(sectMarkers, x[1])
	}
	sectMarkers = append(sectMarkers, len(s1))
	sort.Ints(sectMarkers)

	var output [][]string
	for _, x := range skm {
		start := keyMap[x][1]
		end := start
		for j, y := range sectMarkers {
			if start == y {
				end = sectMarkers[j+1]
			}
		}
		output = append(output, []string{})
		for i := start; i < end; i++ {
			output[len(output)-1] = append(output[len(output)-1], s1[i])
		}
	}

	for i, x := range output {
		for {
			oi := its1.Create(output[i])
			oi.Last()
			if oi.Len() < 1 {
				output[i] = x[:oi.Cur()]
				continue
			}
			break
		}
	}
	for _, x := range output {
		for _, y := range x {
			s2 += y + "\n"
		}
		s2 += "\n"
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

		use 'stdin' as <infile> to read from stdin
	
		multiple source files concatenated and fed to stdin automatically consolidates the everything
		
		duplicate file scope symbols and are not handled automatically
	
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
var sectBuffer string
var chute int

// IsKey returns true if the string has one of the keys at the start
func IsKey(s string) bool {
	for _, x := range its2.Keys {
		if len(x) <= len(s) {
			if x == s[:len(x)] {
				return true
			}
		}
	}
	return false
}
