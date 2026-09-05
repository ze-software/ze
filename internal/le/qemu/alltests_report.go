// Design: docs/architecture/testing/qemu-integration.md -- what the VM proves
// Overview: alltests.go -- the run this report describes
//
// alltests_report.go holds what a whole in-VM run ANSWERS, apart from what
// produced it.
//
// The suites' own output already reached the terminal as each child streamed
// it. The payload contains the facts that a reader cannot recover from that
// stream. It records which phases ran, which were skipped and why, what each
// phase was actually told to do, and which phases failed.

package qemu

import "github.com/ze-software/ze/internal/core/textbuf"

// PhaseResult is one phase of the run. It is a functional suite, the unit pass,
// or the integration pass.
//
// Command is in the payload because a reader often needs this fact and cannot
// get it any other way. It records the concurrency that a suite used and the
// tag set that the integration pass compiled with. These details determine
// what a failure means.
type PhaseResult struct {
	Name    string   `json:"name"`
	Command []string `json:"command,omitempty"`
	Code    int      `json:"code"`
	Skipped bool     `json:"skipped,omitempty"`
	// Reason says why the phase did not run, or why its verdict is what it is.
	Reason string `json:"reason,omitempty"`
	// Namespace names the network namespace a functional suite ran in. It is
	// what tells a reader whether a suite could have reached the guest state
	// the run itself depends on.
	Namespace string `json:"namespace,omitempty"`
	// Tests is how many tests the phase executed, and Counted says the number
	// was read rather than assumed. A phase that reports no count of its own
	// leaves Counted false: the go test phases do, and a suite whose runner
	// printed no summary line does, which is a failure the suite records.
	//
	// The pair exists because an exit code cannot tell a suite that ran two
	// thousand tests from one that ran none and answered 0
	// (plan/journal/gate-excludes-part-of-its-population.md).
	Tests   int  `json:"tests"`
	Counted bool `json:"counted,omitempty"`
}

// AllTestsReport is the whole answer of one run.
type AllTestsReport struct {
	// Selection names the population the functional suites ran over, when it
	// was not all of it. It is carried because a filtered run reads exactly
	// like a whole one: the same suites start, and each answers 0. A reader
	// who cannot see the filter reads "ALL PHASES PASSED" as a verdict over
	// tests that never ran.
	Selection string `json:"selection,omitempty"`
	// Planned names every phase the run intended to reach, in order, as it was
	// known before the first child started. A run that ends early is then a run
	// whose Phases are shorter than its Planned, rather than a run that looks
	// complete because the phases it never reached are in neither list.
	Planned []string      `json:"planned"`
	Phases  []PhaseResult `json:"phases"`
	// Failed names every phase that answered non-zero, in the order they ran.
	// It is carried rather than derived at render time so `| json` and the
	// summary line cannot disagree about what failed.
	Failed []string `json:"failed,omitempty"`
}

// add records one phase and its verdict.
func (r *AllTestsReport) add(phase PhaseResult) {
	r.Phases = append(r.Phases, phase)
	if !phase.Skipped && phase.Code != 0 {
		r.Failed = append(r.Failed, phase.Name)
	}
}

// Unreached names every planned phase that produced no result, in the order it
// was planned.
//
// A skipped phase is REACHED: the run decided about it and said so. An
// unreached phase is one the run never got to, which is what a stopped run
// leaves behind and what must never render as a pass.
func (r AllTestsReport) Unreached() []string {
	reached := make(map[string]bool, len(r.Phases))
	for _, phase := range r.Phases {
		reached[phase.Name] = true
	}
	var missing []string
	for _, name := range r.Planned {
		if !reached[name] {
			missing = append(missing, name)
		}
	}
	return missing
}

// Text renders the summary a person reads at the end of an hour-long run, and
// ends in a newline.
//
// The phase-by-phase progress went to stderr while it happened. What this adds
// is the verdict: which population the run covered, how many phases ran, how
// many were skipped, and the name of every one that failed.
func (r AllTestsReport) Text() string {
	var tb textbuf.Buffer

	ran, skipped := 0, 0
	for _, phase := range r.Phases {
		if phase.Skipped {
			skipped++
			continue
		}
		ran++
	}

	// A report with no phase at all is a run that never STARTED. It must not
	// render as a verdict. Otherwise, the empty Failed list reads as
	// "all phases passed" beside a non-zero exit code.
	if len(r.Phases) == 0 {
		return tb.Str("no phase ran\n").String()
	}

	if r.Selection != "" {
		tb.Str("selection: only the .ci tests marked option=").Str(r.Selection).
			Str(" ran; the unit, installer and integration phases are unfiltered\n")
	}
	tb.Str("phases: ").Int(int64(ran)).Str(" ran, ").Int(int64(skipped)).Str(" skipped, ").
		Int(int64(len(r.Planned))).Str(" planned\n")
	tb.Str("tests: ").Int(int64(r.tests())).Str(" executed across the functional suites\n")

	unreached := r.Unreached()
	if len(unreached) > 0 {
		tb.Str("NOT REACHED: ").Join(unreached, ", ").Byte('\n')
	}
	if len(r.Failed) > 0 {
		tb.Str("FAILED: ").Join(r.Failed, ", ").Byte('\n')
	}
	if len(unreached) == 0 && len(r.Failed) == 0 {
		tb.Str("ALL PHASES PASSED\n")
	}
	return tb.String()
}

// tests is how many tests the counting phases executed between them.
func (r AllTestsReport) tests() int {
	total := 0
	for _, phase := range r.Phases {
		if phase.Counted {
			total += phase.Tests
		}
	}
	return total
}
