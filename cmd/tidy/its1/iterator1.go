package its1

import (
	"errors"
	"fmt"
)

type (
	// Iter is for walking back and forth on a slife of strings
	Iter struct {
		s []string
		x int
		e error
	}
)

// Create creates a new walker based on a string slice
func Create(s []string) (S *Iter) {
	S = new(Iter)
	S.s = s
	return
}

// Next returns the current item and increments the cursor, setting an error if the cursor is at the end
func (i *Iter) Next() (s string) {
	i.e = nil
	if i.x < len(i.s)-1 {
		s = i.s[i.x]
		i.x++
		return
	}
	i.e = fmt.Errorf("cannot step forward at the end %d:%d", i.x, len(i.s))
	return
}

// Prev returns the current item, decrements the cursor and sets an error if it is at the first element
func (i *Iter) Prev() (s string) {
	i.e = nil
	if i.x > 0 {
		s = i.s[i.x]
		i.x--
		return
	}
	i.e = errors.New("cannot step back from the start")
	return
}

// MatchStart returns true if the string is a (sub) string of the current item from the start of the line
func (i *Iter) MatchStart(s string) bool {
	i.e = nil
	if i.x < len(i.s) &&
		len(s) <= len(i.s[i.x]) {
		if s == i.s[i.x][:len(s)] {
			return true
		}
	}
	return false
}

// MatchEnd returns true if the string matches from the end of the string
func (i *Iter) MatchEnd(s string) bool {
	i.e = nil
	if i.OK() &&
		i.x < len(i.s) &&
		len(s) <=
			len(i.s[i.x]) {
		if s == i.s[i.x][len(i.s[i.x])-len(s):] {
			return true
		}
	}
	return false
}

// Cur returns the current iterator position
func (i *Iter) Cur() int {
	return i.x
}

// Goto sets the current iterator position, sets an error if it is out of bounds
func (i *Iter) Goto(I int) {
	if I < 0 || I >= len(i.s) {
		i.e = fmt.Errorf("index %d is out of slice bounds", i)
	} else {
		i.x = I
		i.e = nil
	}
}

// Zero goes back to the first index
func (i *Iter) Zero() {
	i.e = nil
	i.x = 0
}

// Last goes to the last element
func (i *Iter) Last() {
	i.x = len(i.s) - 1
}

// Get returns the value at the cursor
func (i *Iter) Get() (s string) {
	return i.s[i.x]
}

// Len returns the length of the current item
func (i *Iter) Len() int {
	return len(i.s[i.x])
}

// Err returns the error and resets it
func (i *Iter) Err() error {
	e := i.e
	i.e = nil
	return e
}

// OK returns true if the iterator has no error state
func (i *Iter) OK() bool {
	b := i.e == nil
	return b
}
