// Design: docs/architecture/cli/command-namespacing.md -- the grammar gate, as one command
//
// actions.go is the command itself: `le cli-grammar` judges the checkout it is
// run in, and the rendering is a pipe operator, so it takes no argument at all.

package cligrammar

import (
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
)

// Answer is the `le cli-grammar` command.
//
// The three codes are the script's and stay apart: 0 for a tree that obeys the
// grammar, 1 for one that does not, and 2 for a tree the gate could not read.
// A caller that reads them apart keeps reading them apart.
func Answer(args []string) (any, int) {
	if len(args) > 0 {
		return nil, leroot.RefuseArgument(name, args[0])
	}

	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}

	result, err := Check(tree, DefaultFloor, leroot.Owned())
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	if !result.Valid {
		return result, 1
	}
	return result, 0
}
