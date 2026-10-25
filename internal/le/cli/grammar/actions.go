// Design: docs/architecture/cli/command-namespacing.md -- the grammar gate, as one command
//
// actions.go is the command itself: `le cli grammar` judges the checkout it is
// run in, and the rendering is a pipe operator, so its one action, `check`,
// takes no argument at all.

package cligrammar

import (
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// Answer is the `le cli grammar` command. A bare line runs the gate, as the
// `check` action does: hooks and verify stages call it bare, and a bare listing
// would answer them 0 with nothing judged. Every other line is the table's,
// which refuses a word naming no action with 2.
func Answer(args []string) (any, int) {
	if len(args) == 0 {
		return table.Answer([]string{checkVerb})
	}
	return table.Answer(args)
}

// answerCheck runs the gate over the checkout. The table has already refused
// any word after the verb.
//
// The three codes stay apart (le help ai exit-codes): 0 for a tree that obeys the
// grammar, 1 for one that does not, and 3 for a tree the gate could not read.
// A caller that reads them apart keeps reading them apart.
func answerCheck() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 3
	}

	result, err := Check(tree, DefaultFloor, leroot.Owned())
	if err != nil {
		leaction.ReportError(err)
		return nil, 3
	}
	if !result.Valid {
		return result, 1
	}
	return result, 0
}
