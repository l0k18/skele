package T

import (
	"time"
)

var Int int
var IntList []int
var Float float64
var FloatList []float64
var Duration time.Duration
var DurationList []time.Duration
var Time time.Time
var TimeList []time.Time
var Date time.Time
var DateList []time.Time
var Size int
var SizeList []int
var String string
var StringList []string
var Url string
var UrlList []string
var Address string
var AddressList []string
var Base58 []byte
var Base58List [][]byte
var Base32 []byte
var Base32List [][]byte
var Hex string
var HexList []string
var Key struct {
	Label    string
	Template interface{}
}

// Cmd is the interface defining an API interface item and metadata
type Cmd interface {
	Append(p ...Cmd) Cmd
	AUTH(...string) Cmd
	Authors() []string
	Cursor() Cursor
	DATA(interface{}) Cmd
	Data() interface{}
	DESC(string) Cmd
	Description() string
	ERR(string, string) Cmd
	Error() error
	FUNC(func() error) Cmd
	Function() error
	HELP(string, string) Cmd
	Help(string) string
	Item(int) Cmd
	LCNS(string) Cmd
	Len() int
	License() string
	LIST(...Cmd) Cmd
	List() []Cmd
	Name() string
	NAME(string) Cmd
	OK() bool
	Parent() Cmd
	Path() string
	PRNT(Cmd) Cmd
	Scan([]string) error
	Status() string
	String() string
	TYPE(string) Cmd
	Type() string
	VERS(string) Cmd
	Version() string
}

// Cursor is the interface for a cursor on a Command tree
type Cursor interface {
	Cmd() Cmd
	In() bool
	Item() Cmd
	Next() bool
	Out() bool
	Position() int
	Prev() bool
}

// SecureBuffer is an interface for data types that require secure disposal rather than garbage collection
type SecureBuffer interface {
	Wipe()
	Buf() []byte
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

func addType(name string, value interface{}) Key {
	k := Key{name, value}
	Types = append(Types, k)
	return k
}
