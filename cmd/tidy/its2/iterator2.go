package its2

import (
	"errors"
	"fmt"
)

type (
	// Iter2 is an iterator for walking around a slice of string slices
	Iter2 struct {
		s    [][]string
		x, y int
		lx   int
		e    error
	}
)

var (
	// Keys are the slots in the string array
	Keys = []string{
		"package", "import", "type", "const", "var", "func"}
)

// IsKey returns true if the string has one of the keys at the start
func IsKey(s string) bool {
	for _, x := range Keys {
		if len(x) <= len(s) {
			if x == s[:len(x)] {
				return true
			}
		}
	}
	return false
}

// Create is
func Create(s [][]string) (S *Iter2) {
	S = new(Iter2)
	S.s = s
	S.lx = len(S.s)
	return
}

// Next returns the current item and increments the cursor, setting an error if the cursor is at the end
func (i *Iter2) Next() string {
	i.e = nil
	if i.y < len(i.s[i.x][i.y]) {
		x := i.s[i.x][i.y]
		i.y++
		fmt.Println(i.y, x)
		return x
	}
	i.e = errors.New("cannot step forward before the start")
	return ""
}

// Prev returns the current item, decrements the cursor and sets an error if it is at the first element
func (i *Iter2) Prev() string {
	i.e = nil
	if i.y > 0 {
		x := i.s[i.x][i.y]
		i.y--
		return x
	}
	i.e = errors.New("cannot step back before the start")
	return ""
}

// Sel is
func (i *Iter2) Sel(s string) {
	i.e = nil
	for n, x := range Keys {
		if s == x {
			i.x = n
			i.y = 0
		}
	}
}

// CurSlot returns the current slot that is
func (i *Iter2) CurSlot() int {
	return i.x
}

// MatchStart returns true if the string is a (sub) string of the current item from the start of the line
func (i *Iter2) MatchStart(s string) bool {
	i.e = nil
	if len(s) <= len(i.s[i.x]) {
		if s == i.s[i.x][i.y][:len(s)] {
			return true
		}
	}
	return false
}

// MatchEnd returns true if the string matches from the end of the string
func (i *Iter2) MatchEnd(s string) bool {
	i.e = nil

	if len(s) <= len(i.s[i.x]) {
		if s == i.s[i.x][i.y][len(i.s[i.x])-len(s):] {
			return true
		}
	}
	return false
}

// Cur returns the current cursor position
func (i *Iter2) Cur() (int, int) {
	return i.x, i.y
}

// Get returns the value at the cursor
func (i *Iter2) Get() (s string) {
	i.e = nil
	return i.s[i.x][i.y]
}

// Len returns the length of the current item
func (i *Iter2) Len() int {
	i.e = nil
	return len(i.s[i.x][i.y])
}

// Err returns the error and resets it
func (i *Iter2) Err() error {
	e := i.e
	i.e = nil
	return e
}

// OK returns true if the iterator has no error state
func (i *Iter2) OK() bool {
	b := i.e == nil
	i.e = nil
	return b
}
