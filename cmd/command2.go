package main
import (
"fmt"
"io/ioutil"
"os"
"strings"
"time"
)
type imported struct {
pkg []string
imports []string
types [][]string
consts [][]string
vars [][]string
funcs [][]string
}
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
out = os.Stdout
err error
)
func main() {
if len(os.Args) > 1 {
b, err := ioutil.ReadFile(os.Args[1], )
if err != nil {
panic(err, )
}
var splitted []string
splitted = strings.Split(string(b), "\n", )
splitted = rejoinSplitLines(splitted, )
splitted = clean(splitted, )
_ = splitted
} else {
printHelp()
}
}
func clean(l []string) (lines []string) {
q := 0
bo, ao, qo := false, false, false
escaped := false
found := false
i := 0
_ = i
x := ""
for i, x = range l {
time.Sleep(time.Second / 50)
for _, y := range x {
switch y {
case '`', '\'', '"':
q++
}
}
if q%2 == 1 {
for _, y := range x {
switch y {
case '\\':
toggle(&escaped)
case '`':
if !escaped {
if !ao && !qo {
toggle(&bo)
}
}
escaped = false
case '\'':
if !escaped {
if !bo && !qo {
toggle(&ao)
}
}
escaped = false
case '"':
if !escaped {
if !bo && !ao {
toggle(&qo)
}
}
escaped = false
default:
escaped = false
}
}
q = 0
}
if found {
found = false
} else {
if i >= len(l) {
continue
}
if len(l[i]) > 0 {
l[i] = strings.TrimSpace(l[i])
l[i] = removeDoubleWhitespace(x)
} else {
if i < len(l) {
l = append(l[:i], l[i+1:]...)
}
}
}
if ao || bo || qo {
found = true
} else if i < len(l) {
l[i] = strings.TrimSpace(l[i])
}
if len(os.Args) > 2 {
out, _ = os.Create(os.Args[2])
printStrings(l)
out.Close()
}
}
return
}
func toggle(b ...*bool) {
for i := range b {
*b[i] = !*b[i]
}
}
func rejoinSplitLines(s []string) []string {
ignoreList := []string{"import (", "var (", "const (", "type (", }
continuers := []byte{'{', '(', ',', '+', '-', '&', '|', '=', '*', '/', '.', }
iter := getLineIter(s)
current := iter.get()
for {
lastChar := getNthLastChar(current, 1, )
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
if secondLast == '+' || secondLast == '-' {
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
if c == ')' || c == '}' {
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
time.Sleep(time.Second / 50)
if len(os.Args) > 2 {
out, _ = os.Create(os.Args[2])
printStrings(s)
out.Close()
}
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
if prev == ' ' || prev == '\t' || prev == '\n' {
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
func isComment(l string) bool {
t := strings.TrimSpace(l)
if len(t) > 1 && t[:2] == "//" {
return true
}
return false
}
var keywords = []string{"package", "import", "type", "const", "var", "func"}
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
i int
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



















