// Design: docs/architecture/core-design.md -- the perf-bench area, as one command
// Overview: perfbench.go -- the nudge these two actions run
//
// actions.go is the table. The dispatch, the listing, the help line and the two
// refusals are internal/le/leaction, which every ported area shares.
//
// The area is perf-bench and its native verbs are suggestion-report and record.
// The action table is the sole command surface.

package perfbench

import (
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

var actions = leaction.New(area,
	leaction.Action{Verb: "suggestion-report", Why: "suggest a perf run when BGP data-plane code changed since the last one." +
		" A NUDGE, never a gate -- always exits 0. The heavy suite needs Docker and" +
		" minutes, so it is not run every edit; this notices when a Docker perf run" +
		" is overdue on THIS machine, beside the nightly Docker-free regression check",
		Answer: suggestHere},
	leaction.Action{
		Verb:   recordVerb,
		Why:    "record the current HEAD as the commit perf last ran at, which clears the suggestion",
		Writes: true,
		Answer: recordHere,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le perf-bench` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// suggestHere runs the nudge over the checkout this command was run in.
func suggestHere() (any, int) {
	run, err := here()
	if err != nil {
		leaction.ReportError(err)
		// This advisory always exits 0 because it must not block a build.
		// It reports a missing checkout instead of converting that condition into a failure.
		return Report{Error: err.Error()}, 0
	}
	return run.Suggest()
}

// recordHere writes the marker for the checkout this command was run in.
func recordHere() (any, int) {
	run, err := here()
	if err != nil {
		leaction.ReportError(err)
		// 1, unlike the nudge: a caller asked for the marker to be written and
		// it was not, which is a failure rather than an advisory.
		return Report{Error: err.Error()}, 1
	}
	return run.Record()
}

// here answers a runner over the checkout this command was run in.
func here() (*Runner, error) {
	root, err := lepath.Root()
	if err != nil {
		return nil, err
	}
	return New(root), nil
}
