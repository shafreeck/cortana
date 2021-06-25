package cortana

import (
	"strings"

	"github.com/google/btree"
)

// Command is an executive unit
type Command struct {
	Path     string
	Proc     func()
	Brief    string
	Alias    string
	order    int // the order is the sequence of invoking add command
	internal bool
}

type command Command

func (c *command) Less(than btree.Item) bool {
	t := than.(*command)
	return strings.Compare(c.Path, t.Path) < 0
}

type commands struct {
	t *btree.BTree
}

func (c commands) scan(prefix string) []*command {
	var cmds []*command
	begin := &command{Path: prefix}
	end := &command{Path: prefix + "\xFF"}

	c.t.AscendRange(begin, end, func(i btree.Item) bool {
		cmds = append(cmds, i.(*command))
		return true
	})
	return cmds
}
func (c commands) get(path string) *command {
	i := c.t.Get(&command{Path: path})
	if i != nil {
		return i.(*command)
	}
	return nil
}

// orderedCommands keep the order of adding a command
type orderedCommands []*command

func (cmds orderedCommands) Len() int {
	return len(cmds)
}
func (cmds orderedCommands) Less(i, j int) bool {
	return cmds[i].order < cmds[j].order
}
func (cmds orderedCommands) Swap(i, j int) {
	cmds[i], cmds[j] = cmds[j], cmds[i]
}
