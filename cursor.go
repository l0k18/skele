package sk

// Cursor is the interface for a cursor on a Command tree
type Cursor interface {
	In() bool
	Out() bool
	Next() bool
	Prev() bool
	Pair() *Pair
	Cmd() Commander
	Position() int
}

// cursor is a way to get around the Command tree
type cursor struct {
	cmd      Commander
	position int
}

// Crsr returns a cursor given a Commander. The index is -1 so that a loop can pre-increment and start at zero
func Crsr(C Commander) Cursor {
	return &cursor{C, -1}
}

// In goes up to the parent of the current node
func (c *cursor) In() (did bool) {
	if p := c.cmd.Parent(); p.OK() {
		c.cmd = p
		c.position = 0
		did = true
	}
	return
}

// Out walks outwards on a KV containing a Commander, returns true if it walked
func (c *cursor) Out() (b bool) {
	if c.Pair().IsType(COMMAND) {
		c.cmd = c.Pair().V.(Commander)
		c.position = -1
	}
	return
}

// Next just returns the next item in the pairs slice and returns false when it is at the end
func (c *cursor) Next() (did bool) {
	c.position++
	if c.cmd.NumPairs() > c.position {
		did = true
	} else {
		c.cmd.ERR("warn", "no more pairs in slice")
		did = false
	}
	return
}

// Prev steps back in the current Pair Slice
func (c *cursor) Prev() (b bool) {
	if c.position > 0 {
		c.position--
	} else {
		c.cmd.ERR("warn", "at start of slice cannot go back")
	}
	return
}

// Pair returns the pair under the cursor
func (c *cursor) Pair() (p *Pair) {
	if c.cmd.NumPairs() > c.position {
		return c.cmd.Pair(c.position)
	}
	c.cmd.ERR("warn", "somehow cursor fell off the edge, moving back to edge")
	c.position = c.cmd.NumPairs() - 1
	return
}

// Cmd just returns the Commander inside it
func (c *cursor) Cmd() Commander {
	return c.cmd
}

func (c *cursor) Position() int {
	return c.position
}
