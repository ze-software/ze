// Design: docs/architecture/testing/test-health.md -- the test-sensitivity area, as one command
//
// actions.go is the Python area, ported. The dispatch, the listing, the help
// line and the two refusals live in letools/leaction. What stays here is the
// TABLE, because the table is the only part of an area that is about tests that
// cannot go red.
//
// The two REPORT actions differ in one thing, their population, and that
// difference is why they are two actions rather than one with a flag: the tree
// a run reads is a keyword the caller types, not a value it passes.

package testsensitivity

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/letools/leaction"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "test-sensitivity"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{
		Gate:   "ze-test-sensitivity-check",
		Why:    "no more tests than the committed floor assert nothing or sit behind a build tag nothing supplies, over the WORKING TREE so an inert test is caught before it is committed",
		Answer: runCheck,
	},
	leaction.Action{
		Gate:   "ze-test-sensitivity-selftest",
		Why:    "both detectors still tell an asserting test from an inert one and a reachable build tag from an orphan, proved against fixtures rather than against the tree they judge",
		Answer: runSelftest,
	},
	leaction.Action{
		// No Make target names this one: it is the script's default mode, the
		// page a person reads when they want the list rather than the verdict.
		Verb:   "report",
		Why:    "every assert-nothing test and every tag orphan in the working tree, listed rather than ratcheted",
		Answer: runReport,
	},
	leaction.Action{
		// No Make target names this one either: it is the script's
		// --tracked-only, and the generated test-health page reads it.
		Verb:   "tracked",
		Why:    "the same lists over the files GIT holds, so a generated page is reproducible from a clean checkout whatever is uncommitted",
		Answer: runTracked,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le test-sensitivity` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runReport is the `le test-sensitivity report` action.
func runReport() (any, int) { return runScan(WorkingTree) }

// runTracked is the `le test-sensitivity tracked` action.
func runTracked() (any, int) { return runScan(Tracked) }

// runScan answers one population's lists, with no ratchet: the report says what
// is there, and the check says whether it is too much.
func runScan(population Population) (any, int) {
	tree, err := treeRoot()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	result, err := Scan(tree, population)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return result, 0
}

// runCheck is the `le test-sensitivity check` action.
//
// The two codes stay apart: 1 for a ratchet that fired over a tree it read, and
// 2 for a scan that could not read the tree at all.
func runCheck() (any, int) {
	tree, err := treeRoot()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	baseline, err := ReadBaseline(tree)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	result, err := Scan(tree, WorkingTree)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}

	verdict := Judge(result, baseline)
	// The breach is advice rather than an answer: it tells a person what to do
	// next, and a caller who typed `| json` has no use for it. stderr is where
	// the script put it and where it stays.
	if breach := verdict.Breach(); breach != "" {
		fmt.Fprint(os.Stderr, breach) //nolint:errcheck // CLI output
	}
	if !verdict.Result.Valid {
		return verdict, 1
	}
	return verdict, 0
}
