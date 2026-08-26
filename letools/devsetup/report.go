// Design: docs/architecture/core-design.md -- the vocabulary a setup report is written in
//
// report.go is a Go port of scripts/le/console.py. It defines the `State` that a
// step reached, the `Outcome` it returned, and the `Report` that determines the
// verdict.
//
// ONE VALUE CARRIES THE LABEL AND THE VERDICT. The previous shell version used
// four parallel lists in one loop. It manually added entries beside each print
// operation. Thus, the displayed label and the list used for the verdict were
// separate records of one fact. The records became inconsistent. An install in
// a directory outside PATH printed `[installed]` but added nothing to the list.
//
// The run ended with "Setup complete" and exit 0, while `--check` on the same
// host exited 1. Each step now returns one Outcome. The code calculates the
// tally at the end.
//
// THE REPORT IS ALSO THE TRANSCRIPT. Python wrote each line to stdout when the
// event occurred. A Go le tool returns a payload and lets the operator select
// the rendering (ai/rules/cli.md). Therefore, Report records every line in
// order. `Text` renders the page, and the JSON encoding contains the outcomes.
//
// Both forms use one record. This is the purpose of the module.

package devsetup

import (
	"encoding/json"
	"slices"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// State is what a setup step found, and what that means for the exit code.
//
// The label is what a reader sees and Blocking is what the run is judged by.
// They travel together because the shell version let them drift apart.
type State string

const (
	// StatePresent means already there and working. Nothing to do.
	StatePresent State = "present"
	// StateInstalled means this run made it so.
	StateInstalled State = "installed"
	// StatePending means the machine was changed and a human must finish it: a
	// PATH to extend, a session to restart. Re-running does not help, so the
	// run must not report success.
	StatePending State = "pending"
	// StateSkipped means nothing to do here and nothing wrong: an optional
	// tool, or a platform with no package for it.
	StateSkipped State = "skipped"
	// StateMissing means required and absent, with no route taken to fix it.
	// The spelling is upper case because it is the one state a reader must not
	// skim past.
	StateMissing State = "MISSING"
)

// stateWidth is the column the state label is padded to in a report line. It is
// len("installed"), the longest of the five.
const stateWidth = 9

// Blocking reports whether a step in this state must fail the run.
func (s State) Blocking() bool { return s == StatePending || s == StateMissing }

// Outcome is one step's result: what it was, how it went, and why.
//
// Detail is written for whoever has to fix it, so it names the command, the
// path, or the complaint rather than restating the state.
type Outcome struct {
	Name   string `json:"name"`
	State  State  `json:"state"`
	Detail string `json:"detail,omitempty"`
}

// Line renders the report line for this outcome.
func (o Outcome) Line() string {
	var tb textbuf.Buffer
	tb.Str("  [").PadRight(string(o.State), stateWidth).Str("] ").Str(o.Name)
	if o.Detail != "" {
		tb.Str(" (").Str(o.Detail).Str(")")
	}
	return tb.String()
}

// Report contains every outcome of one run and every line that the run would
// have printed. It also contains the verdict derived from the outcomes.
//
// The code does not calculate totals during the steps. Outcomes is the complete
// record, and each method below reads it. Therefore, a step cannot be counted in
// one place but omitted from another.
type Report struct {
	// Outcomes is the record the verdict is derived from, in the order the
	// steps happened.
	Outcomes []Outcome
	// lines is the page that a reader sees. It contains every outcome line and
	// the interstitial lines that a step printed before it returned.
	lines []string
}

// Note records one line of the transcript that is not an outcome: an install
// command about to run, a manual fix, a PATH to extend.
func (r *Report) Note(line string) { r.lines = append(r.lines, line) }

// Add records one outcome and its line. It answers the outcome, for a caller
// that wants to branch on what it just recorded.
func (r *Report) Add(o Outcome) Outcome {
	r.Outcomes = append(r.Outcomes, o)
	r.lines = append(r.lines, o.Line())
	return o
}

// having answers every outcome in one of these states, in the order they
// happened.
func (r *Report) having(states ...State) []Outcome {
	var found []Outcome
	for _, o := range r.Outcomes {
		if slices.Contains(states, o.State) {
			found = append(found, o)
		}
	}
	return found
}

// names joins the names of these outcomes for a one-line verdict.
func names(outcomes []Outcome) string {
	list := make([]string, 0, len(outcomes))
	for _, o := range outcomes {
		list = append(list, o.Name)
	}
	var tb textbuf.Buffer
	return tb.Join(list, ", ").String()
}

// Summarize records the final verdict of an install run and returns the exit
// code.
//
// Missing is reported first because it is the more severe failure.
// Pending means that the machine changed and a person must complete the work.
// Missing means that no action occurred.
func (r *Report) Summarize() int {
	r.Note("")

	if missing := r.having(StateMissing); len(missing) > 0 {
		var tb textbuf.Buffer
		r.Note(tb.Str("Missing required tools: ").Str(names(missing)).String())
		return 1
	}

	if pending := r.having(StatePending); len(pending) > 0 {
		// "Steps", not "install commands": a tool that landed in a directory
		// off PATH needs the PATH fixed, not the install re-run.
		var tb textbuf.Buffer
		r.Note(tb.Str("Finish the steps above for: ").Str(names(pending)).String())
		r.Note("Then re-run: ./le setup")
		return 1
	}

	var parts []string
	if installed := r.having(StateInstalled); len(installed) > 0 {
		var tb textbuf.Buffer
		parts = append(parts, tb.Str("installed: ").Str(names(installed)).String())
	}
	if skipped := r.having(StateSkipped); len(skipped) > 0 {
		var tb textbuf.Buffer
		parts = append(parts, tb.Str("skipped (optional): ").Str(names(skipped)).String())
	}

	if len(parts) > 0 {
		var tb textbuf.Buffer
		r.Note(tb.Str("Setup complete. ").Join(parts, "; ").String())
	} else {
		r.Note("Setup complete. All tools already present.")
	}
	return 0
}

// CheckVerdict records the final verdict of a probe-only run and returns the
// exit code.
//
// A probe changes nothing. Therefore, PENDING here never means "this run changed
// the machine and a human must finish". It means that the check found an action
// that only a human CAN do. Examples are a plugin that this program must not
// install (editor.go) and a group that requires a new login. Both conditions
// fail the run, but the report keeps them separate because their corrections
// differ. If the report calls a plugin a missing tool, the reader goes to the
// tool table, which does not install it.
func (r *Report) CheckVerdict() int {
	r.Note("")

	missing := r.having(StateMissing)
	pending := r.having(StatePending)

	if len(missing) > 0 {
		var tb textbuf.Buffer
		r.Note(tb.Str("Missing required tools: ").Str(names(missing)).String())
	}
	if len(pending) > 0 {
		var tb textbuf.Buffer
		r.Note(tb.Str("Needs a step only you can take: ").Str(names(pending)).String())
	}
	if len(missing) > 0 || len(pending) > 0 {
		return 1
	}

	r.Note("All required tools present.")
	return 0
}

// Text replays the whole page, ending in a newline. It is the default
// rendering leroot.Prose asks for.
func (r *Report) Text() string {
	var tb textbuf.Buffer
	for _, line := range r.lines {
		tb.Str(line).Byte('\n')
	}
	return tb.String()
}

// MarshalJSON answers the outcomes rather than the struct, so `| json` gives a
// row set the operators can act on and `| table` renders it.
func (r *Report) MarshalJSON() ([]byte, error) {
	if r.Outcomes == nil {
		return json.Marshal([]Outcome{})
	}
	return json.Marshal(r.Outcomes)
}
