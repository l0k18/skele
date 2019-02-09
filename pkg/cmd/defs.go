package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/davecgh/go-spew/spew"
)

// ensurirg command implements skeller
var _ Commander = new(command)

// command is a command, which can form a tree that executes a fifo chain of subcommands
type command struct {
	parent  Commander
	name    string
	authors []string
	version string
	license string
	inits   []func(...interface{}) error
	brief   string
	help    map[string]string
	handler func() error
	list    []Commander
	err     error
}

// Attach copies inheritable properties from another command and links back to the parent
func Attach(i Commander) Commander {
	c := Cmd()
	if i != nil {
		c.VERS(i.Version()).
			AUTH(i.Authors()...).
			LCNS(i.License()).
			PRNT(i)
	}
	return c
}

// Cmd returns a new command
func Cmd() Commander {
	c := new(command)
	c.help = make(map[string]string)
	return c
}

// Parse takes a string and a variable and attempts to decode the value according to the type of the variable
func Parse(in string, T interface{}) (out interface{}, err error) {
	switch T.(type) {
	case Int:
		var o Int
		if o, err = ParseInt(in); err == nil {
			out = o
		}
	case Float:
		var o Float
		if o, err = ParseFloat(in); err == nil {
			out = o
		}
	case Duration:
		var o Duration
		if o, err = ParseDuration(in); err == nil {
			out = o
		}
	case Time:
		var o Time
		if o, err = ParseTime(in); err == nil {
			out = o
		}
	case Date:
		var o Date
		if o, err = ParseDate(in); err == nil {
			out = o
		}
	case Size:
		var o Size
		if o, err = ParseSize(in); err == nil {
			out = o
		}
	case String:
		var o String
		if o, err = ParseString(in); err == nil {
			out = o
		}
	case Url:
		var o Url
		if o, err = ParseURL(in); err == nil {
			out = o
		}
	case Address:
		var o Address
		if o, err = ParseAddress(in); err == nil {
			out = o
		}
	case Base58:
		var o Base58
		if o, err = ParseBase58(in); err == nil {
			out = o
		}
	case Base32:
		var o Base32
		if o, err = ParseBase32(in); err == nil {
			out = o
		}
	case Hex:
		var o Hex
		if o, err = ParseHex(in); err == nil {
			out = o
		}
	default:
		err = errors.New("unhandled type")
	}
	return
}

// String renders a string containing the human readable parts of a command
func (c *command) String() (s string) {
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
func (c *command) Parent() Commander {
	if c.parent != nil {
		return c.parent
	}
	c.ERR("warn", "this command has no parent")
	return nil
}

// PRNT sets the parent node of a Command
func (c *command) PRNT(C Commander) Commander {
	if C != nil {
		c.parent = C
	} else {
		c.ERR("warn", "nil parameter received")
	}
	return c
}

// Cursor returns a cursor at the root of the Commander
func (c *command) Cursor() Cursor {
	return Crsr(c)
}

// Path returns the path of a commander from the root
func (c *command) Path() (s string) {
	s = c.Name()
	p := c.Parent()
	for p != nil {
		s = p.Name() + "/" + s
		p = p.Parent()
	}
	return
}

// Name returns the name of the command
func (c *command) Name() string {
	return c.name
}

// N sets the name of the command
func (c *command) NAME(in string) Commander {
	c.name = in
	return c
}

// Authors returns the authors array
func (c *command) Authors() []string {
	return c.authors
}

// A sets the authors array
func (c *command) AUTH(in ...string) Commander {
	c.authors = in
	return c
}

// Version returns the command version string
func (c *command) Version() string {
	return c.version
}

// V sets the command version string
func (c *command) VERS(in string) Commander {
	if in[0] != 'v' {
		c.ERR("error", "version string must start with 'v', received '"+in+"'")
	}
	numbers := strings.Split(in[1:], ".")
	for _, num := range numbers {
		_, c.err = ParseInt(num)
		if c.err != nil {
			c.ERR("error", "improperly formatted version string: '"+in+"' : "+c.err.Error())
		}
	}
	c.version = in
	return c
}

// License returns the license field of the command
func (c *command) License() string {
	return c.license
}

// L sets the license field of the command
func (c *command) LCNS(in string) Commander {
	c.license = in
	return c
}

// Inits returns the array of init functions stored in the command, that re run for a new instance
func (c *command) Inits() (out []func(...interface{}) error) {
	for _, item := range c.inits {
		out = append(out, item)
	}
	return
}

// I loads the array of init functions
func (c *command) INIT(in ...func(...interface{}) error) Commander {
	c.inits = in
	return c
}

// Brief gets the brief text of a command
func (c *command) Description() string {
	return c.brief
}

// B sets the brief string of a command
func (c *command) DESC(in string) Commander {
	c.brief = in
	return c
}

// Help returns the help string of a given type
func (c *command) Help(t string) string {
	if s, ok := c.help[t]; ok {
		return s
	}
	return ""
}

// H sets one of the fields of a command's help
func (c *command) HELP(t string, v string) Commander {
	c.help[t] = v
	return c
}

// Function runs the handler
func (c *command) Function() error {
	return c.handler()
}

// F loads the handler for a command
func (c *command) FUNC(in func() error) Commander {
	c.handler = in
	return c
}

// Error returns the error in a command and resets it
func (c *command) Error() (e error) {
	e = c.err
	e = nil
	return
}

// Status returns the current error string
func (c *command) Status() (s string) {
	if c.err != nil {
		s = c.err.Error()
		c.err = nil
	}
	return
}

// E sets the error in a command
func (c *command) ERR(loglevel, err string) Commander {
	c.err = errors.New(err)
	return c
}

// OK returns true if there is no error and resets the error
func (c *command) OK() (b bool) {
	b = c.err == nil
	c.Error()
	return
}

// List returns the pair at the specified index, if it exists
func (c *command) List(i int) (p Commander) {
	if len(c.list) > i {
		return c.list[i]
	}
	c.ERR("warn", "index out of bounds")
	return
}

// LIST loads subcommand into a command
func (c *command) LIST(cc ...Commander) Commander {
	c.list = cc
	return c
}

// Lists returns the commands attached to a command
func (c *command) Lists() []Commander {
	return c.list
}

// AddList adds a pair to the pairs array
func (c *command) AddList(p Commander) Commander {
	c.list = append(c.list, p)
	return c
}

// NumLists returns the length of the pairs slice
func (c *command) NumLists() int {
	return len(c.list)
}

// IsType returns true if the constant integer code matches the string code in the List
func (p *List) IsType(t int) bool {
	return p.K == Types[t].K
}
