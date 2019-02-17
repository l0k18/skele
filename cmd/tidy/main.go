//
package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/l0k1verloren/skele/cmd/tidy/its1"
	"github.com/l0k1verloren/skele/cmd/tidy/its2"
)

var scanner *bufio.Scanner

type sectMap map[string][]int

const pi = 3.1415927

var chute int
var e error
var f *os.File
var infile, outfile string
var lineBuffer []string
var readBuffer []byte
var skm []string
var sorted []string
var sectMarkers []int
var output [][]string
var sectBuffer string

func main() {
	if len(os.Args) > 1 {
		infile = os.Args[1]
		switch infile {
		case "stdin":
			f = os.Stdin
			_, e := os.Stdin.Stat()
			if e != nil {
				panic(e)
			}
			readBuffer, e = ioutil.ReadAll(os.Stdin)
			infile = ""
		default:
			fmt.Println("reading file in")
			if readBuffer, e = ioutil.ReadFile(os.Args[1]); e != nil {
				panic(e)
			}
		}
	} else {
		printHelp()
	}
	// outfile = infile
	if len(os.Args) > 2 {
		outfile = os.Args[2]
		if f, e = os.Create(outfile); e != nil {
			panic(e)
		}
	} else {
		f = os.Stdout
	}
	scanner = bufio.NewScanner(strings.NewReader(string(readBuffer)))
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		lineBuffer = append(lineBuffer, scanner.Text())
	}
	if e = scanner.Err(); e != nil {
		panic(e)
	}
	lineBuffer = removeBlankLines(lineBuffer)
	clean(lineBuffer)
	// sectBuffer = section(lineBuffer)
	// fmt.Fprintln(f, sectBuffer)
}

func clean(s []string) {
	printStrings(s)
}

func printStrings(s []string) {
	for _, x := range s {
		fmt.Fprintln(f, x)
	}
}

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

func hasKey(s string) (int, bool) {
	it := its1.Create(its2.Keys)
	for it.OK() {
		if it.MatchStart(s) {
			return it.Cur(), true
		}
	}
	return 0, false
}

func match(s1, s2 string) bool {
	if len(s1) <= len(s2) {
		if s1 == s2[:len(s1)] {
			return true
		}
	}
	return false
}

func printHelp() {
	fmt.Print(
		"go source tidy\n\n" +
			"usage: tidy <infile> [outfile]\n\n" +
			"	reads go source files, cleans and cuts them into individual declarations, groups and sorts them\n\n" +
			"	use 'stdin' as <infile> to read from stdin, in this\n\n" +
			"	multiple source files concatenated and fed to stdin automatically consolidates the everything\n\n" +
			"	duplicate file scope symbols and are not handled automatically\n\n",
	)
	os.Exit(1)
}

func removeBlankLines(in []string) (out []string) {
	for _, x := range in {
		if len(x) > 0 {
			out = append(out, x)
		}
	}
	return
}

func section(s1 []string) (s2 string) {
	if len(s1) < 1 {
		os.Exit(1)
	}
	keyMap := make(sectMap)
	i1 := its1.Create(s1)
	for i1.Zero(); i1.OK(); {
		if its2.IsKey(i1.Get()) {
			keyMap[i1.Get()] = append(keyMap[i1.Get()], i1.Cur())
		}
		i1.Next()
	}

	// spew.Dump(keyMap)

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
				i1.Cur() > 0 {
				// if len(i1.Get()) < 1 {
				// 	i1.Next()
				// }
				keyMap[i] = append(keyMap[i], i1.Cur())
				break
			}
			// if len(keyMap[i]) < 1 {
			// 	i1.Next()
			// }
			i1.Next()
			keyMap[i] = append(keyMap[i], i1.Cur())
		}
	}
	i1.Zero()
	for x := range keyMap {
		sorted = append(sorted, x)
	}
	sort.Strings(sorted)
	spew.Dump(sorted)
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
		for item.Zero(); item.OK(); item.Next() {
			if match(ord.Get(), item.Get()) {
				// if item.Get() == "func main() {" ||
				// 	item.Get() == "func init() {" {
				// 	continue
				// }
				skm = append(skm, item.Get())
			}
		}
		item.Zero()
	}
	for _, x := range keyMap {
		sectMarkers = append(sectMarkers, x[1])
	}
	sectMarkers = append(sectMarkers, len(s1))
	sort.Ints(sectMarkers)
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
			if len(s1[i]) > 0 {
				output[len(output)-1] =
					append(output[len(output)-1], s1[i])
			}
		}
		// output[len(output)-1] = append(output[len(output)-1], "//>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>")
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
