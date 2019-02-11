package T

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
