package command

import "time"

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

// The programmer-friendly enum for all the types handled by skele
const (
	COMMAND = iota
	INT
	INTLIST
	FLOAT
	FLOATLIST
	DURATION
	DURATIONLIST
	TIME
	TIMELIST
	DATE
	DATELIST
	SIZE
	SIZELIST
	STRING
	STRINGLIST
	URL
	URLLIST
	ADDRESS
	ADDRESSLIST
	BASE58
	BASE58LIST
	BASE32
	BASE32LIST
	HEX
	HEXLIST
)

var (
	// Types are the types of values in a skele commandline parser. This map is used with a type switch to determine how to interpret a token
	Types = map[int]List{
		COMMAND: List{
			"command", *new(Commander)},
		INT: List{
			"int", *new(Int)},
		INTLIST: List{
			"intlist", *new(IntList)},
		FLOAT: List{
			"float", *new(Float)},
		FLOATLIST: List{
			"floatlist", *new(FloatList)},
		DURATION: List{
			"duration", *new(Duration)},
		DURATIONLIST: List{
			"durationlist", *new(DurationList)},
		TIME: List{
			"time", *new(Time)},
		TIMELIST: List{
			"timelist", *new(TimeList)},
		DATE: List{
			"date", *new(Date)},
		DATELIST: List{
			"datelist", *new(DateList)},
		SIZE: List{
			"size", *new(Size)},
		SIZELIST: List{
			"sizelist", *new(SizeList)},
		STRING: List{
			"string", *new(StringList)},
		STRINGLIST: List{
			"stringlist", *new(StringList)},
		URL: List{
			"url", *new(Url)},
		URLLIST: List{
			"urllist", *new(UrlList)},
		ADDRESS: List{
			"address", *new(Address)},
		ADDRESSLIST: List{
			"addresslist", *new(AddressList)},
		BASE58: List{
			"base58", *new(Base58List)},
		BASE58LIST: List{
			"base58list", *new(Base58List)},
		BASE32: List{
			"base32", *new(Base32List)},
		BASE32LIST: List{
			"base32list", *new(Base32List)},
		HEX: List{
			"hex", *new(Hex)},
		HEXLIST: List{
			"hexlist", *new(HexList)},
	}
)

var (
	// HelpTypes are the different formats a help text can be encoded in
	HelpTypes = []string{
		"pre",
		"markdown",
		"html",
	}
)
