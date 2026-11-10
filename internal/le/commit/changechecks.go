// Design: docs/contributing/committing.md -- the change-set checks `./le commit create` runs
//
// Related: changecheck/changecheck.go -- the seam each check registers into

package commit

import (
	"errors"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/commit/changecheck"
	"github.com/ze-software/ze/internal/le/commit/hunks"
)

// changeCheckRefusal runs every check registered in changecheck over the
// commit being prepared and refuses it with their refusal texts. This code
// names no check: a check registers from its own package, and the
// composition root links it.
//
// After reads the carried selection (selection.Read), so a `hunks` commit is
// judged on the blob it carries, never on the working-tree file, and Before
// reads the tree the commit starts from (head), as dependencyRefusal does.
func changeCheckRefusal(root string, paths, removed []string, selection hunks.Selection, head string) error {
	return runChangeChecks(root, paths, removed, selection, head, changecheck.Registered())
}

// runChangeChecks is changeCheckRefusal over a given check list, so a test
// judges a fixture check without the composition root.
func runChangeChecks(root string, paths, removed []string, selection hunks.Selection, head string, checks []changecheck.Check) error {
	if len(checks) == 0 {
		return nil
	}
	sides := workingTreeSides(root, head, selection)
	change := changecheck.Change{
		Root:    root,
		Paths:   paths,
		Removed: removed,
		Before:  sideReader(sides.before),
		After:   sideReader(sides.after),
	}
	refusals, err := changecheck.Run(change, checks)
	if err != nil {
		return couldNotRun(err)
	}
	if len(refusals) == 0 {
		return nil
	}
	var tb textbuf.Buffer
	for position, refusal := range refusals {
		if position > 0 {
			tb.Byte('\n')
		}
		tb.Str("./le ").Str(refusal.Check).Str(" refused what this commit adds:\n").Str(refusal.Text)
	}
	return errors.New(tb.String())
}

// sideReader adapts one commit side to the seam's Side, turning a read
// problem into an error a check cannot mistake for an absent file. The side's
// own present flag is passed through, so an absent path stays absent.
func sideReader(side specSide) changecheck.Side {
	return func(path string) (string, bool, error) {
		text, present, problem := side(path)
		if problem != "" {
			return "", false, errors.New(problem)
		}
		return text, present, nil
	}
}
