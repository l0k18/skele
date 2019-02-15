package tree

import "github.com/l0k1verloren/skele/pkg/T"

// walker is a way to get around the Command tree
type walker struct {
	cmd      T.Cmd
	position int
}

var _ T.Cursor = new(walker)

// Cmd just returns the Commander inside it
func (c *walker) Cmd() T.Cmd {
	return c.cmd
}

// In goes up to the parent of the current node
func (c *walker) In() (did bool) {
	if p := c.cmd.Parent(); p.OK() {
		c.cmd = p
		c.position = 0
		did = true
	}
	return
}

// Item returns the item under the cursor
func (c *walker) Item() (p T.Cmd) {
	if c.cmd.Len() > c.position {
		return c.cmd.Item(c.position)
	}
	c.cmd.ERR("warn", "somehow cursor fell off the edge, moving back to edge")
	c.position = c.cmd.Len() - 1
	return
}

// Next just returns the next item in the pairs slice and returns false when it is at the end
func (c *walker) Next() (did bool) {
	c.position++
	if c.cmd.Len() > c.position {
		did = true
	} else {
		c.cmd.ERR("warn", "no more pairs in slice")
		did = false
	}
	return
}

// Out walks outwards on a KV containing a Commander, returns true if it walked
func (c *walker) Out() (b bool) {
	c.cmd.Item(c.position).Len()
	return
}

func (c *walker) Position() int {
	return c.position
}

// Prev steps back in the current Pair Slice
func (c *walker) Prev() (b bool) {
	if c.position > 0 {
		c.position--
	} else {
		c.cmd.ERR("warn", "at start of slice cannot go back")
	}
	return
}

// Walker returns a cursor given a Commander. The index is -1 so that a loop can pre-increment and start at zero
func Walker(C T.Cmd) T.Cursor {
	return &walker{C, -1}
}


