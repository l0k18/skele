package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/l0k1verloren/skele/pkg/T"
	"github.com/l0k1verloren/skele/pkg/parse"
	"github.com/l0k1verloren/skele/pkg/tree"
)

var _ T.Cmd = new(cmd)

type cmd struct {
	authors []string
	brief   string
	data    interface{}
	err     error
	handler func() error
	help    map[string]string
	inits   []func(...interface{}) error
	license string
	list    []T.Cmd
	name    string
	parent  T.Cmd
	skType  string
	version string
}

// Cmd returns a new command
func Cmd() T.Cmd {
	c := new(cmd)
	c.help = make(map[string]string)
	return c
}

// Name creates a node and names it in one
func Name(n string) T.Cmd {
	return Cmd().NAME(n)
}

// NameType creates a node with a named type and given name
func NameType(n, t string) T.Cmd {
	return Cmd().NAME(n).TYPE(t)
}

// TYPE sets the type of the node
func (c *cmd) TYPE(t string) T.Cmd {
	var found bool
	for _, x := range T.Types {
		if x.Label == t {
			c.skType = t
			found = true
		}
	}
	if !found {
		c.ERR("warn", "invalid type: "+t)
	}
	return c
}

// Type returns the currently set type string descriptor
func (c *cmd) Type() string {
	return c.skType
}

// String renders a string containing the human readable parts of a command
func (c *cmd) String() (s string) {
	s = "name " + c.name + " authors"
	for _, v := range c.authors {
		s += fmt.Sprint(" '", v, "'")
	}
	s += " version " + c.version +
		" license '" + c.license +
		"' brief '" + c.brief
	if len(c.help) > 0 {
		s += "' help"
	}
	for i, v := range c.help {
		s += i + " '" + v + "' "
	}
	if len(c.list) > 0 {
		s += "pairs "
		for i, v := range c.list {
			s += fmt.Sprint(i) + " '" + spew.Sdump(v) + "' "
		}
	}
	if c.err != nil {
		s += "error '" + c.err.Error() + "'"
	}
	return
}

// Parent returns the parent off the current command if it isn't the root
func (c *cmd) Parent() T.Cmd {
	if c.parent != nil {
		return c.parent
	}
	c.ERR("warn", "this command has no parent")
	return nil
}

// PRNT sets the parent node of a Command
func (c *cmd) PRNT(C T.Cmd) T.Cmd {
	if C != nil {
		c.parent = C
		t := C.(*cmd)
		t.list = append(t.list, c)
	} else {
		c.ERR("warn", "nil parameter received")
	}
	return c
}

// Cursor returns a cursor at the root of the T.Commander
func (c *cmd) Cursor() T.Cursor {
	return tree.Walker(c)
}

// Path returns the path of a T.commander from the root
func (c *cmd) Path() (s string) {
	s = c.Name()
	p := c.Parent()
	for p != nil {
		s = p.Name() + "/" + s
		p = p.Parent()
	}
	return
}

// Name returns the name of the command
func (c *cmd) Name() string {
	return c.name
}

// N sets the name of the command
func (c *cmd) NAME(in string) T.Cmd {
	c.name = in
	return c
}

// DATA loads a value into a T.Commander
func (c *cmd) DATA(i interface{}) T.Cmd {
	switch d := c.data.(type) {
	case T.SecureBuffer:
		d.Wipe()
	}
	c.data = i
	return c
}

// Data returns the data stored in a T.Commander
func (c *cmd) Data() interface{} {
	switch d := c.data.(type) {
	case T.SecureBuffer:
		return d.Buf()
	default:
	}
	return c.data
}

// Authors returns the authors array
func (c *cmd) Authors() []string {
	return c.authors
}

// AUTH sets the authors array
func (c *cmd) AUTH(in ...string) T.Cmd {
	c.authors = in
	return c
}

// Version returns the command version string
func (c *cmd) Version() string {
	return c.version
}

// V sets the command version string
func (c *cmd) VERS(in string) T.Cmd {
	if in[0] != 'v' {
		c.ERR("error", "version string must start with 'v', received '"+in+"'")
	}
	numbers := strings.Split(in[1:], ".")
	for _, num := range numbers {
		_, c.err = parse.Int(num)
		if c.err != nil {
			c.ERR("error", "improperly formatted version string: '"+in+"' : "+c.err.Error())
		}
	}
	c.version = in
	return c
}

// License returns the license field of the command
func (c *cmd) License() string {
	return c.license
}

// L sets the license field of the command
func (c *cmd) LCNS(in string) T.Cmd {
	c.license = in
	return c
}

// Description gets the brief text of a command
func (c *cmd) Description() string {
	return c.brief
}

// DESC sets the brief string of a command
func (c *cmd) DESC(in string) T.Cmd {
	c.brief = in
	return c
}

// Help returns the help string of a given type
func (c *cmd) Help(t string) string {
	if s, ok := c.help[t]; ok {
		return s
	}
	return ""
}

// HELP sets one of the fields of a command's help
func (c *cmd) HELP(t string, v string) T.Cmd {
	c.help[t] = v
	return c
}

// Function runs the handler
func (c *cmd) Function() error {
	return c.handler()
}

// FUNC loads the handler for a command
func (c *cmd) FUNC(in func() error) T.Cmd {
	c.handler = in
	return c
}

// Error returns the error in a command and resets it
func (c *cmd) Error() (e error) {
	e = c.err
	e = nil
	return
}

// Status returns the current error string
func (c *cmd) Status() (s string) {
	if c.err != nil {
		s = c.err.Error()
		c.err = nil
	}
	return
}

// ERR sets the error in a command
func (c *cmd) ERR(loglevel, err string) T.Cmd {
	c.err = errors.New(err)
	return c
}

// OK returns true if there is no error and resets the error
func (c *cmd) OK() (b bool) {
	b = c.err == nil
	c.Error()
	return
}

// Item returns the pair at the specified index, if it exists
func (c *cmd) Item(i int) (p T.Cmd) {
	if len(c.list) > i {
		return c.list[i]
	}
	c.ERR("warn", "index out of bounds")
	return
}

// LIST loads subcommand into a command
func (c *cmd) LIST(cc ...T.Cmd) T.Cmd {
	c.list = cc
	return c
}

// List returns the commands attached to a command
func (c *cmd) List() []T.Cmd {
	return c.list
}

// Append adds an item to the list
func (c *cmd) Append(p ...T.Cmd) T.Cmd {
	for _, x := range p {
		x.PRNT(c)
	}
	c.list = append(c.list, p...)
	return c
}

// Len returns the length of the pairs slice
func (c *cmd) Len() int {
	return len(c.list)
}
