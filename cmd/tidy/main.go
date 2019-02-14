//
package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/davecgh/go-spew/spew"
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

func section(s1 []string) (s2 [][][]string) {
	var keyList []sectMap
	i1 := its1.Create(s1)
	// find and gather line numbers of all root level keywords at the start of the line
	for i1.Goto(0); i1.OK(); {
		if its2.IsKey(i1.Get()) {
			i := map[string]transMap{
				i1.Get(): transMap{i1.Cur(), i1.Cur()},
			}
			keyList = append(keyList, i)
		}
		i1.Next()
	}
	i1.Goto(0)
	// find the start of the comments above each section
	for _, x := range keyList {
		for _, y := range x {
			i1.Goto(y.line - 1)
			for ; i1.MatchStart("//") && i1.Cur() > 0; i1.Prev() {
				y.line = i1.Cur() + 1
			}
		}
	}
	i1.Goto(0)
	spew.Dump(keyList)
	// // Sort the keyList
	// sectText := make([][]string, len(its2.Keys))
	// for i, x := range keyList {
	// 	for j := range x {
	// 		sectText[i] = append(sectText[i], j)
	// 	}
	// }
	// spew.Dump(sectText)
	// i2 := its2.Create(sectText)
	// var sections [][]string
	// for i2.OK() {
	// 	for i2.OK() {
	// 		c := i2.Next()
	// 		for i, x := range its2.Keys {
	// 			if i2.MatchStart(x) {
	// 				sections[i] = append(sections[i], c)
	// 			}
	// 		}
	// 	}
	// 	if i2.CurSlot() < len(its2.Keys) {
	// 		i2.Sel(its2.Keys[i2.CurSlot()+1])
	// 	}
	// }
	// for i := range sections {
	// 	sort.Strings(sections[i])
	// }
	// spew.Dump(sections)
	// spew.Dump(sectText)
	return
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

type (
	transMap struct {
		line int
		key  int
	}
	sectMap map[string]transMap
)

//
//
var (
	e               error
	infile, outfile string
	f               *os.File
	readBuffer      []byte
	lineBuffer      []string
	sectBuffer      [][][]string
	chute           int
)
