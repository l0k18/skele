package def

import (
	"time"
)

type (
	Int          int
	IntList      []int
	Float        float64
	FloatList    []float64
	Duration     time.Duration
	DurationList []time.Duration
	Time         time.Time
	TimeList     []time.Time
	Date         time.Time
	DateList     []time.Time
	Size         int
	SizeList     []int
	String       string
	StringList   []string
	Url          string
	UrlList      []string
	Address      string
	AddressList  []string
	Base58       []byte
	Base58List   [][]byte
	Base32       []byte
	Base32List   [][]byte
	Hex          string
	HexList      []string

	Key struct {
		Label    string
		Template interface{}
	}
)

// The programmer-friendly enum for all the types handled by skele
var (
	COMMAND      = Key{"command", *new(Commander)}
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

	HelpTypes = []string{
		"pre",
		"markdown",
		"html",
	}
)
