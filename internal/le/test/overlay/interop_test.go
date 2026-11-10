// Design: docs/contributing/rfc-conformance-gates.md -- what an interop observation must show
package overlay

import (
	"strings"
	"testing"
)

// VALIDATES: AC-2 -- the interop verdict readers give a phase-labeled FAIL
// line, `FAIL (<elapsed>) in <phase>: <error>`, the same verdicts as the
// unlabeled line for a checker red, and refuse a red labeled `in setup` or
// `in readiness` as not the cited assertion, naming the phase.
// METHOD: BrokenUnobserved and Attribution over the same red with no label and
// labeled `in checker`, for the cited assertion and another one; then over a
// setup and a readiness red whose error even carries the cited anchor; then a
// checker red whose cause quotes ` in setup:`.
// PREVENTS: a red from a helper that never started being recorded as the
// checker assertion's proof, and the label breaking the anchor read.
func TestInteropVerdictReadsAPhaseLabelledLine(t *testing.T) {
	const (
		scenario = "relay-shape"
		unit     = "internal/le/interoplab/bgp/check_rfc.go::checkRelayWithdrawalShape"
		cited    = "scenario relay-shape assertion 6: withdrawal carried an AS_PATH\n"
		other    = "scenario relay-shape assertion 2: no route\n"
	)
	line := func(label, cause string) string {
		return "interop: [1/1] relay-shape: FAIL (30s)" + label + ": " + cause +
			"interop: 0 passed, 1 failed: relay-shape\n"
	}
	for _, cause := range []string{cited, other} {
		plain := line("", cause)
		labeled := line(" in checker", cause)
		if got, want := BrokenUnobserved(labeled, scenario, unit), BrokenUnobserved(plain, scenario, unit); got != want {
			t.Errorf("BrokenUnobserved differs for the labeled line: %q, unlabeled %q", got, want)
		}
		got, want := Attribution(labeled, scenario, "6"), Attribution(plain, scenario, "6")
		if (got == "") != (want == "") {
			t.Errorf("Attribution differs for the labeled line %q: %q, unlabeled %q", cause, got, want)
		}
	}
	if why := Attribution(line(" in checker", cited), scenario, "6"); why != "" {
		t.Fatalf("a checker red at the cited assertion was refused: %s", why)
	}
	if Attribution(line(" in checker", other), scenario, "6") == "" {
		t.Fatal("a checker red at assertion 2 was read as assertion 6")
	}

	for _, phase := range []string{"setup", "readiness"} {
		red := line(" in "+phase, cited)
		if why := BrokenUnobserved(red, scenario, unit); why != "" {
			t.Errorf("a %s red was refused as unobserved, but the scenario's FAIL line is there: %s", phase, why)
		}
		why := Attribution(red, scenario, "6")
		if why == "" {
			t.Fatalf("a red %s, before the checker ran, was recorded as assertion 6's", phase)
		}
		if !strings.Contains(why, "in "+phase) {
			t.Errorf("the refusal of a %s red does not name the phase: %s", phase, why)
		}
		if got := FailPhase("FAIL (30s) in " + phase + ": " + cited); got != phase {
			t.Errorf("FailPhase = %q, want %q", got, phase)
		}
	}

	quoted := line(" in checker", "scenario relay-shape assertion 6: peer said in setup: no route\n")
	if why := Attribution(quoted, scenario, "6"); why != "" {
		t.Fatalf("a checker red whose cause quotes ` in setup:` was refused: %s", why)
	}
	if got := FailPhase(": FAIL (30s): " + cited); got != "" {
		t.Errorf("FailPhase of an unlabeled verdict = %q, want none", got)
	}
}
