package skele

import (
	"time"
)

// Skeller is the interface defining an API interface item and metadata
type Skeller interface {
	Name() string
	Authors() []string
	Version() string
	Licence() string
	Init(...interface{})
	Brief() string
	Help(string) string
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

// Cmd is a command, which can form a tree potentially launching the first, last, fifo or lifo order as implemented
type Cmd struct {
	name    string
	authors []string
	version string
	licence string
	inits   []func(...interface{}) error
	brief   string
	help    map[string]string
	Pairs   []Pair
}
