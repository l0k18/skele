package skele

import (
	"errors"
	"time"
)

// Skeller is the interface defining an API interface item and metadata
type Skeller interface {
	GetName() string
	SetName(string) error
	GetAuthors() []string
	SetAuthors([]string) error
	GetVersion() string
	SetVersion(string) error
	GetLicence() string
	SetLicence(string) error
	GetInit() []interface{}
	SetInit(...interface{})
	GetBrief() string
	SetBrief(string) error
	GetHelp(string) string
	SetHelp(string, string) error
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
	ptCmd      Cmd

	// ParamTypes allows recognition the types with the names using type switches
	ParamTypes = []Pair{
		{"cmd", ptCmd},
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
		o, err = ParseInt(in)
		if err == nil {
			p.V = o
		}
	case Float:
		var o Float
		o, err = ParseFloat(in)
		if err == nil {
			p.V = o
		}
	case Duration:
		var o Duration
		o, err = ParseDuration(in)
		if err == nil {
			p.V = o
		}
	case Time:
		var o Time
		o, err = ParseTime(in)
		if err == nil {
			p.V = o
		}
	case Date:
		var o Date
		o, err = ParseDate(in)
		if err == nil {
			p.V = o
		}
	case Size:
		var o Size
		o, err = ParseSize(in)
		if err == nil {
			p.V = o
		}
	case String:
		var o String
		o, err = ParseString(in)
		p.V = o
	case URL:
		var o URL
		o, err = ParseURL(in)
		if err == nil {
			p.V = o
		}
	case Address:
		var o Address
		o, err = ParseAddress(in)
		if err == nil {
			p.V = o
		}
	case Base58:
		var o Base58
		o, err = ParseBase58(in)
		if err == nil {
			p.V = o
		}
	case Base32:
		var o Base32
		o, err = ParseBase32(in)
		if err == nil {
			p.V = o
		}
	case Hex:
		var o Hex
		o, err = ParseHex(in)
		if err == nil {
			p.V = o
		}
	default:
		err = errors.New("unhandled type")
	}
	return
}

// Cmd is a command, which can form a tree potentially launching the first, last, fifo or lifo order as implemented when the Pairs contains more Cmd items
type Cmd struct {
	name    string
	authors []string
	version string
	licence string
	inits   []func(...interface{}) error
	brief   string
	help    map[string]string
	Handler func() error
	Pairs   []Pair
}
