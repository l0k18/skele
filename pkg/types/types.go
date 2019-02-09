package types

import (
	"time"

	iface "github.com/l0k1verloren/skele/interfaces"
)

// Int is a skele integer type
type Int int

// IntList is a skele integer list type
type IntList []int

// Float is a skele float type
type Float float64

// FloatList is a skele float list type
type FloatList []float64

// Duration is a skele duration type
type Duration time.Duration

// DurationList is a skele duration list type
type DurationList []time.Duration

// Time is a skele time type
type Time time.Time

// TimeList is a skele time list type
type TimeList []time.Time

// Date is a skele date type
type Date time.Time

// DateList is a skele date list type
type DateList []time.Time

// Size is a size in bytes, can be specified with K/k M/m G/g T/t P/p
type Size int

// SizeList is a list of sizes in bytes, can be specified with K/k M/m G/g T/t P/p
type SizeList []int

// String is a string
type String string

// StringList is a list of strings
type StringList []string

// Url is a string describing a file in URL format
type Url string

// UrlList is a list of strings describing a file in URL format
type UrlList []string

// Address is a string describing a network address
type Address string

// AddressList is a list of network addresses
type AddressList []string

// Base58 is a Base58check encoded binary like a cryptocurrency address
type Base58 []byte

// Base58List is a list off Base58check encoded binary like a cryptocurrency address
type Base58List [][]byte

// Base32 is a base32 encoding as is used with some cryptocurrencies and cryptographic tools, not case sensitive but follows standard
type Base32 []byte

// Base32List is a list of binary data with base32 encoding as is used with some cryptocurrencies and cryptographic tools, not case sensitive but follows standard
type Base32List [][]byte

// Hex is a hexadecimal string, not case sensitive
type Hex string

// HexList is a list of hexadecimal strings, not case sensitive
type HexList []string

// Key is the mapping between an instance of the type and its string label
type Key struct {
	Label    string
	Template interface{}
}

// The programmer-friendly enum for all the types handled by skele
var (
	COMMAND      = Key{"command", *new(iface.Commander)}
	INT          = Key{"int", *new(Int)}
	INTLIST      = Key{"intlist", *new(IntList)}
	FLOAT        = Key{"float", *new(Float)}
	FLOATLIST    = Key{"floatlist", *new(FloatList)}
	DURATION     = Key{"duration", *new(Duration)}
	DURATIONLIST = Key{"durationlist", *new(DurationList)}
	TIME         = Key{"time", *new(Time)}
	TIMELIST     = Key{"timelist", *new(TimeList)}
	DATE         = Key{"date", *new(Date)}
	DATELIST     = Key{"datelist", *new(DateList)}
	SIZE         = Key{"size", *new(Size)}
	SIZELIST     = Key{"sizelist", *new(SizeList)}
	STRING       = Key{"string", *new(StringList)}
	STRINGLIST   = Key{"stringlist", *new(StringList)}
	URL          = Key{"url", *new(Url)}
	URLLIST      = Key{"urllist", *new(UrlList)}
	ADDRESS      = Key{"address", *new(Address)}
	ADDRESSLIST  = Key{"addresslist", *new(AddressList)}
	BASE58       = Key{"base58", *new(Base58List)}
	BASE58LIST   = Key{"base58list", *new(Base58List)}
	BASE32       = Key{"base32", *new(Base32List)}
	BASE32LIST   = Key{"base32list", *new(Base32)}
	HEX          = Key{"hex", *new(Hex)}
	HEXLIST      = Key{"hexlist", *new(HexList)}
)

var (
	// T are the types of values in a skele commandline parser. This map is used with a type switch to determine how to interpret a token
	T = map[int]List{}
)

var (
	// HelpTypes are the different formats a help text can be encoded in
	HelpTypes = []string{
		"pre",
		"markdown",
		"html",
	}
)
