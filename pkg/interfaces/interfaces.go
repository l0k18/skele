package iface

// Commander is the interface defining an API interface item and metadata
type Commander interface {
	Parent() Commander
	PRNT(Commander) Commander
	Cursor() Cursor
	Path() string
	Scan([]string) error
	Name() string
	NAME(string) Commander
	Authors() []string
	AUTH(...string) Commander
	Version() string
	VERS(string) Commander
	License() string
	LCNS(string) Commander
	Inits() []func(...interface{}) error
	INIT(...func(...interface{}) error) Commander
	Description() string
	DESC(string) Commander
	Help(string) string
	HELP(string, string) Commander
	Function() error
	FUNC(func() error) Commander
	Error() error
	Status() string
	ERR(string, string) Commander
	OK() bool
	List(int) Commander
	Lists() []Commander
	LIST(...Commander) Commander
	AddList(p Commander) Commander
	NumLists() int
	String() string
}

// Cursor is the interface for a cursor on a Command tree
type Cursor interface {
	In() bool
	Out() bool
	Next() bool
	Prev() bool
	Item() Commander
	Cmd() Commander
	Position() int
}
