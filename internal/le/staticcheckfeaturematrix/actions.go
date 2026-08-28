// Design: docs/architecture/testing/tracked-build-gate.md -- the matrix area, as one command
//
// actions.go is the Python area, ported. The dispatch, the listing, the help
// line and the two refusals live in internal/le/leaction. What stays here is the
// TABLE, because the table is the only part of an area that is about
// type-checking the tree once per feature-tag combination.

package staticcheckfeaturematrix

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "staticcheck-feature-matrix"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{Verb: "check", Why: "the tree type-checks in every feature-tag combination Ze can be built in, so a package compiled out of the default build is still judged",
		Answer: runCheck},
	leaction.Action{
		// No Make target names this one: it is the script's --print-matrix, and
		// it exists so a reader can see WHICH combinations a run judges without
		// waiting for Staticcheck.
		Verb:   "rows",
		Why:    "the feature-tag combinations this run judges, derived from the feature manifest and narrowed to what the change set can move",
		Answer: runRows,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le staticcheck-feature-matrix` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runRows is the `le staticcheck-feature-matrix rows` action.
func runRows() (any, int) {
	matrix, notice, err := derive()
	if err != nil {
		reportFailure(err)
		return nil, 2
	}
	reportNotice(notice)
	return matrix, 0
}

// runCheck is the `le staticcheck-feature-matrix check` action.
//
// The three codes stay apart: 0 for a tree that type-checks in every judged
// combination, 1 for one that does not, and 2 for a matrix that could not be
// judged at all. A caller that reads them apart keeps reading them apart.
func runCheck() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		reportFailure(err)
		return nil, 2
	}
	matrix, notice, err := derive()
	if err != nil {
		reportFailure(err)
		return nil, 2
	}
	reportNotice(notice)

	deadline, err := Deadline()
	if err != nil {
		reportFailure(err)
		return nil, 2
	}

	verdict, judged, err := Judge(tree, matrix, deadline)
	if err != nil {
		reportFailure(err)
		if verdict.Tool != "" {
			fmt.Fprint(os.Stderr, verdict.Tool) //nolint:errcheck // CLI output
		}
		return nil, 2
	}
	if verdict.Tool != "" {
		fmt.Fprint(os.Stderr, verdict.Tool) //nolint:errcheck // CLI output
	}
	if !judged || !verdict.Passed {
		return verdict, 1
	}
	return verdict, 0
}

// derive resolves the checkout and answers the rows this run judges.
func derive() (Matrix, Notice, error) {
	tree, err := lepath.Root()
	if err != nil {
		return nil, Notice{}, err
	}
	return Derive(tree)
}

// reportNotice writes what the run says about its own scope, which is a fact
// about the RUN rather than part of any answer.
func reportNotice(notice Notice) {
	if text := notice.Text(); text != "" {
		fmt.Fprint(os.Stderr, text) //nolint:errcheck // CLI output
	}
}

// reportFailure states a matrix that could not be judged, in the one form every
// caller of this check reads.
func reportFailure(err error) { leaction.ReportError(err) }
