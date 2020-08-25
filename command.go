package cortana

import (
	"strings"

	"github.com/google/btree"
)

type command struct {
	path  string
	brief string
	proc  func()
}

func (c *command) Less(than btree.Item) bool {
	t := than.(*command)
	return strings.Compare(c.path, t.path) < 0
}

type commands struct {
	t *btree.BTree
}

func (c commands) scan(prefix string) []*command {
	var cmds []*command
	begin := &command{path: prefix}
	end := &command{path: prefix + "\xFF"}

	c.t.AscendRange(begin, end, func(i btree.Item) bool {
		cmds = append(cmds, i.(*command))
		return true
	})
	return cmds
}
func (c commands) get(path string) *command {
	i := c.t.Get(&command{path: path})
	if i != nil {
		return i.(*command)
	}
	return nil
}
