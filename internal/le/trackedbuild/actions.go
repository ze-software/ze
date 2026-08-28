// Design: docs/architecture/testing/tracked-build-gate.md -- the tracked-build area, as one command
//
// actions.go is the Python area, ported. The dispatch, the listing, the help
// line and the two refusals live in internal/le/leaction. What stays here is the
// TABLE, because the table is the only part of an area that is about compiling
// the tree git holds.

package trackedbuild

import (
	"context"
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "repository-tracked-build"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{Verb: "check", Why: "the tree GIT HOLDS compiles in every shipped flavor, which is the one population every other gate misses because they all build the working tree",
		Answer: runCheck},
	leaction.Action{Verb: "selftest", Why: "the two vacuity guards still fire. `go build ./...` exits 0 over a pattern that matched nothing buildable, so a flavor compiling zero packages would otherwise report success",
		Answer: runSelftest},
	leaction.Action{
		// No Make target names this one: it is the script's --matrix, and it
		// exists so a reader can see WHICH flavors a run compiles without
		// waiting for six builds.
		Verb:   "matrix",
		Why:    "the build flavors this gate compiles, with the tags and the anchor file each one exists to select",
		Answer: runMatrix,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le repository-tracked-build` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runMatrix is the `le repository-tracked-build matrix` action.
func runMatrix() (any, int) { return buildMatrix, 0 }

// runCheck is the `le repository-tracked-build check` action.
//
// The three codes stay apart: 0 for a commit that compiles, 1 for one that does
// not, and 2 for a run that could not judge it. A killed build and a broken
// commit send a reader after completely different things.
func runCheck() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	options, err := defaultOptions()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), options.Deadline)
	defer cancel()

	report, code, err := Run(ctx, root, options)
	if err != nil {
		leaction.ReportError(err)
	}
	if code == 2 && len(report.Results) == 0 {
		return nil, 2
	}

	// The diagnosis is advice rather than an answer: it tells a person what to
	// do next, and a caller who typed `| json` has no use for it. stderr is
	// where the script put it and where it stays.
	if diagnosis := report.Diagnosis(); diagnosis != "" {
		fmt.Fprint(os.Stderr, diagnosis) //nolint:errcheck // CLI output
	}
	return report, code
}
