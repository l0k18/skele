package skele

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
)

// Skeller is the interface defining an API interface item and metadata
type Skeller interface {
	Name() string
	N(string) Skeller
	Authors() []string
	A(...string) Skeller
	Version() string
	V(string) Skeller
	License() string
	L(string) Skeller
	Inits() []interface{}
	I([]func(...interface{}) error) Skeller
	Brief() string
	B(string) Skeller
	Help(string) string
	H(string, string) Skeller
	Function() error
	F(func() error) Skeller
	Error() string
	E(string) Skeller
	Pairs() []Pair
	P([]Pair) Skeller
	String() string
}

// ensurirg Command implements skeller
var _ Skeller = new(Command)

// Command is a command, which can form a tree that executes a fifo chain of subcommands
type Command struct {
	name    string
	authors []string
	version string
	license string
	inits   []func(...interface{}) error
	brief   string
	help    map[string]string
	handler func() error
	pairs   []Pair
	err     error
}

// Cmd returns a new Command
func Cmd() *Command {
	c := new(Command)
	c.help = make(map[string]string)
	return c
}

// Int is a skele integer type
type Int int

// Float is a skele float type
type Float float64

// Duration is a skele duration type
type Duration time.Duration

// Time is a skele time type
type Time time.Time

// Date is a skele date type
type Date time.Time

// Size is a size in bytes, can be specified with K/k M/m G/g T/t P/p
type Size int

// String is a string
type String string

// URL is a string describing a file in URL format
type URL string

// Address is a string describing a network address
type Address string

// Base58 is a Base58check string like a cryptocurrency address
type Base58 []byte

// Base32 is a base32 encoding as is used with some cryptocurrencies and cryptographic tools, not case sensitive but follows standard
type Base32 []byte

// Hex is a hexadecimal string, not case sensitive
type Hex string

var (
	// HelpTypes are the different formats a help text can be encoded in
	HelpTypes = []string{
		"pre",
		"markdown",
		"html",
	}

	ptInt      Int
	ptFloat    Float
	ptDuration Duration
	ptTime     Time
	ptSize     Size
	ptString   String
	ptURL      URL
	ptAddress  Address
	ptBase58   Base58
	ptBase32   Base32
	ptHex      Hex
	ptCommand  Command

	// PairTypes allows recognition the types with the names using type switches
	PairTypes = []Pair{
		{"command", ptCommand},
		{"int", ptInt},
		{"float", ptFloat},
		{"duration", ptDuration},
		{"time", ptTime},
		{"size", ptSize},
		{"string", ptString},
		{"url", ptURL},
		{"address", ptAddress},
		{"base58", ptBase58},
		{"base32", ptBase32},
		{"hex", ptHex},
	}
)

// A Pair is a label that allows reflection-free runtime typing without type switches (strings instead)
type Pair struct {
	T string
	V interface{}
}

// Type returns the type of a pair as a string
func (p *Pair) Type() string {
	return p.T
}

// Value returns the value of a Pair
func (p *Pair) Value() interface{} {
	return p.V
}

// Parse abstracts the type specific parsing using the type of the value in the pair
func (p *Pair) Parse(in string) (err error) {
	switch p.V.(type) {
	case Int:
		var o Int
		if o, err = ParseInt(in); err == nil {
			p.V = o
		}
	case Float:
		var o Float
		if o, err = ParseFloat(in); err == nil {
			p.V = o
		}
	case Duration:
		var o Duration
		if o, err = ParseDuration(in); err == nil {
			p.V = o
		}
	case Time:
		var o Time
		if o, err = ParseTime(in); err == nil {
			p.V = o
		}
	case Date:
		var o Date
		if o, err = ParseDate(in); err == nil {
			p.V = o
		}
	case Size:
		var o Size
		if o, err = ParseSize(in); err == nil {
			p.V = o
		}
	case String:
		var o String
		if o, err = ParseString(in); err == nil {
			p.V = o
		}
	case URL:
		var o URL
		if o, err = ParseURL(in); err == nil {
			p.V = o
		}
	case Address:
		var o Address
		if o, err = ParseAddress(in); err == nil {
			p.V = o
		}
	case Base58:
		var o Base58
		if o, err = ParseBase58(in); err == nil {
			p.V = o
		}
	case Base32:
		var o Base32
		if o, err = ParseBase32(in); err == nil {
			p.V = o
		}
	case Hex:
		var o Hex
		if o, err = ParseHex(in); err == nil {
			p.V = o
		}
	case Command:
		// Here's the magic - we now
	default:
		err = errors.New("unhandled type")
	}
	return
}

// String renders a string containing the human readable parts of a Command
func (c *Command) String() (s string) {
	s = "name " + c.name + " authors"
	for _, v := range c.authors {
		s += fmt.Sprint(" '", v, "'")
	}
	s += " version " + c.version +
		" license '" + c.license +
		"' brief '" + c.brief
	if len(c.help) > 0 {
		s += "' help "
	}
	for i, v := range c.help {
		s += i + " '" + v + "' "
	}
	if len(c.pairs) > 0 {
		s += "pairs "
		for i, v := range c.pairs {
			s += fmt.Sprint(i) + " '" + spew.Sdump(v) + "' "
		}
	}
	if c.err != nil {
		s += "error '" + c.err.Error() + "'"
	}
	return
}

// Name returns the name of the Command
func (c *Command) Name() string {
	return c.name
}

// N sets the name of the Command
func (c *Command) N(in string) Skeller {
	c.name = in
	return c
}

// Authors returns the authors array
func (c *Command) Authors() []string {
	return c.authors
}

// A sets the authors array
func (c *Command) A(in ...string) Skeller {
	c.authors = in
	return c
}

// Version returns the command version string
func (c *Command) Version() string {
	return c.version
}

// V sets the command version string
func (c *Command) V(in string) Skeller {
	if in[0] != 'v' {
		c.E("version string must start with 'v', received '" + in + "'")
	}
	numbers := strings.Split(in[1:], ".")
	for _, num := range numbers {
		_, c.err = ParseInt(num)
		if c.err != nil {
			c.err = errors.New("improperly formatted version string: '" + in + "' : " + c.err.Error())
		}
	}
	c.version = in
	return c
}

// License returns the license field of the Command
func (c *Command) License() string {
	return c.license
}

// L sets the license field of the Command
func (c *Command) L(in string) Skeller {
	c.license = in
	return c
}

// Inits returns the array of init functions stored in the command, that re run for a new instance
func (c *Command) Inits() (out []interface{}) {
	for _, item := range c.inits {
		out = append(out, item)
	}
	return
}

// I loads the array of init functions
func (c *Command) I(in []func(...interface{}) error) Skeller {
	c.inits = in
	return c
}

// Brief gets the brief text of a Command
func (c *Command) Brief() string {
	return c.brief
}

// B sets the brief string of a Command
func (c *Command) B(in string) Skeller {
	c.brief = in
	return c
}

// Help returns the help string of a given type
func (c *Command) Help(t string) string {
	if s, ok := c.help[t]; ok {
		return s
	}
	return ""
}

// H sets one of the fields of a command's help
func (c *Command) H(t string, v string) Skeller {
	c.help[t] = v
	return c
}

// Function runs the handler
func (c *Command) Function() error {
	return c.handler()
}

// F loads the handler for a Command
func (c *Command) F(in func() error) Skeller {
	c.handler = in
	return c
}

// Error returns the error in a Command
func (c *Command) Error() string {
	if c.err == nil {
		return ""
	}
	return c.err.Error()
}

// E sets the error in a Command
func (c *Command) E(err string) Skeller {
	c.err = errors.New(err)
	return c
}

// P loads a set of pairs into a Command
func (c *Command) P(p []Pair) Skeller {
	c.pairs = p
	return c
}

// Pairs returns the pairs attached to a command
func (c *Command) Pairs() []Pair {
	return c.pairs
}
