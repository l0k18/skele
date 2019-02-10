package T

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
	Key          struct {
		Label    string
		Template interface{}
	}
)

func addType(name string, value interface{}) Key {
	k := Key{name, value}
	Types = append(Types, k)
	return k
}

// This combines a visible and recognisable label for skele types with a means to template variables parsed from strings
var (
	// Types aggregates the defined types at the same time as they are assigned to their identifier
	Types        []Key
	COMMAND      = addType("command", *new(Cmd))
	INT          = addType("int", *new(Int))
	INTLIST      = addType("intlist", *new(IntList))
	FLOAT        = addType("float", *new(Float))
	FLOATLIST    = addType("floatlist", *new(FloatList))
	DURATION     = addType("duration", *new(Duration))
	DURATIONLIST = addType("durationlist", *new(DurationList))
	TIME         = addType("time", *new(Time))
	TIMELIST     = addType("timelist", *new(TimeList))
	DATE         = addType("date", *new(Date))
	DATELIST     = addType("datelist", *new(DateList))
	SIZE         = addType("size", *new(Size))
	SIZELIST     = addType("sizelist", *new(SizeList))
	STRING       = addType("string", *new(StringList))
	STRINGLIST   = addType("stringlist", *new(StringList))
	URL          = addType("url", *new(Url))
	URLLIST      = addType("urllist", *new(UrlList))
	ADDRESS      = addType("address", *new(Address))
	ADDRESSLIST  = addType("addresslist", *new(AddressList))
	BASE58       = addType("base58", *new(Base58List))
	BASE58LIST   = addType("base58list", *new(Base58List))
	BASE32       = addType("base32", *new(Base32List))
	BASE32LIST   = addType("base32list", *new(Base32))
	HEX          = addType("hex", *new(Hex))
	HEXLIST      = addType("hexlist", *new(HexList))
	HelpTypes    = []string{"pre", "markdown", "html"}
)
