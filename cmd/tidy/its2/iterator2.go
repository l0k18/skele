package its2

import (
	"errors"
	"fmt"
)

type (
	Iter2 struct {
		s    [][]string
		x, y int
		lx   int
		e    error
	}
)

var (
	Keys = []string{
		"package", "import", "type", "const", "var", "func"}
)

func (i *Iter2) Cur() (int, int) {
	return i.x, i.y
}
func (i *Iter2) CurSlot() int {
	return i.x
}
func (i *Iter2) Err() error {
	e := i.e
	i.e = nil
	return e
}
func (i *Iter2) Get() (s string) {
	i.e = nil
	return i.s[i.x][i.y]
}
func (i *Iter2) Len() int {
	i.e = nil
	return len(i.s[i.x][i.y])
}
func (i *Iter2) MatchEnd(s string) bool {
	i.e = nil
	if len(s) <= len(i.s[i.x]) {
		if s == i.s[i.x][i.y][len(i.s[i.x])-len(s):] {
			return true
		}
	}
	return false
}
func (i *Iter2) MatchStart(s string) bool {
	i.e = nil
	if len(s) <= len(i.s[i.x]) {
		if s == i.s[i.x][i.y][:len(s)] {
			return true
		}
	}
	return false
}
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
func (i *Iter2) OK() bool {
	b := i.e == nil
	i.e = nil
	return b
}
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
func (i *Iter2) Sel(s string) {
	i.e = nil
	for n, x := range Keys {
		if s == x {
			i.x = n
			i.y = 0
		}
	}
}
func Create(s [][]string) (S *Iter2) {
	S = new(Iter2)
	S.s = s
	S.lx = len(S.s)
	return
}
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
