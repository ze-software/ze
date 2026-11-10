// Design: docs/contributing/rfc-conformance-gates.md -- what an interop observation must show
// Overview: overlay.go -- the overlay whose run these readers judge
// Related: result.go -- Excerpt, DescribeUnobservedBreak

package overlay

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// The verdicts interoplab.Suite prints on a scenario's progress line,
// `interop: [i/n] <scenario>: pass (<elapsed>)` or
// `interop: [i/n] <scenario>: FAIL (<elapsed>) in <phase>: <error>`
// (internal/le/interoplab/lab.go, Suite.Run and scenarioLine). A line written
// before the phase label existed has no ` in <phase>`, and reads the same.
const (
	VerdictPass = ": pass ("
	VerdictFail = ": FAIL ("
)

// ScenarioVerdict answers the rest of the progress line that gives the
// scenario its verdict, starting at the verdict marker, or false when no such
// line names the scenario.
//
// The scenario is matched on the whole name between `] ` and the marker, so a
// scenario whose name extends another's is never read as it.
func ScenarioVerdict(output, scenario, marker string) (string, bool) {
	needle := "] " + scenario + marker
	for line := range strings.Lines(output) {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "interop: [") {
			continue
		}
		_, rest, found := strings.Cut(trimmed, needle)
		if found {
			return marker + rest, true
		}
	}
	return "", false
}

// FailPhase answers the phase a FAIL verdict names, the word between
// `(<elapsed>) in ` and the next colon, or the empty string for a verdict that
// carries no phase label.
//
// The label follows the elapsed time's closing parenthesis directly, so a
// cause text that says ` in setup:` is never read as the phase.
func FailPhase(verdict string) string {
	_, rest, found := strings.Cut(verdict, ")")
	if !found {
		return ""
	}
	labeled, held := strings.CutPrefix(rest, " in ")
	if !held {
		return ""
	}
	phase, _, closed := strings.Cut(labeled, ":")
	if !closed {
		return ""
	}
	return phase
}

// CleanUnobserved answers why a clean interop run did not observe the
// scenario passing, or the empty string when it did. unit names the observed
// unit in the answer.
//
// An exit status is not that evidence. A selector that matched no scenario, a
// lab whose images never built and a cross-compile of ze that failed each end
// the run with `0 passed, 0 failed`, and a run that executed nothing cannot be
// the green a break is judged against
// (plan/journal/green-that-could-not-have-been-red.md, 2026-10-23).
func CleanUnobserved(output, scenario, unit string) string {
	if _, ran := ScenarioVerdict(output, scenario, VerdictPass); ran {
		return ""
	}
	var tb textbuf.Buffer
	return tb.Str(unit).Str(" did not run in the clean run: no ").
		Quoted("interop: [i/n] " + scenario + VerdictPass).
		Str(" line. A run that executed no scenario is not a green, so nothing a break ").
		Str("does to it can be observed:\n").Str(Excerpt(output)).String()
}

// BrokenUnobserved answers why a run under the break did not observe the
// scenario failing, or the empty string when it did.
//
// The failure a verdict rests on is the scenario's own FAIL line. A build of
// the broken ze that did not compile fails the run before any scenario starts,
// and that red says nothing about the claim
// (plan/journal/green-that-could-not-have-been-red.md, 2026-10-23).
func BrokenUnobserved(output, scenario, unit string) string {
	if _, ran := ScenarioVerdict(output, scenario, VerdictFail); ran {
		return ""
	}
	if _, ran := ScenarioVerdict(output, scenario, VerdictPass); ran {
		return ""
	}
	var tb textbuf.Buffer
	return DescribeUnobservedBreak(unit,
		tb.Str("no progress line gives scenario ").Str(scenario).Str(" a verdict").String(),
		output)
}

// Attribution answers why a failing interop run is NOT the red of the
// checker's assertion citation, or the empty string when it is.
//
// The checker renders `scenario <name> assertion N: <cause>` for
// `fail(N, cause)` (checkerFailure, internal/le/interoplab/bgp/check_engine.go),
// and the citation is that N. The number is read there and nowhere else on the
// line, so a cause that quotes another assertion is never read as the cited
// one, and assertion 1 is not named by `assertion 12`. A FAIL labeled with a
// phase before the checker (`in setup`, `in readiness`) is refused whatever its
// text says: the checker never ran, so no assertion of it fired.
func Attribution(output, scenario, citation string) string {
	verdict, failed := ScenarioVerdict(output, scenario, VerdictFail)
	var tb textbuf.Buffer
	if !failed {
		return tb.Str("scenario ").Str(scenario).Str(" printed no FAIL line").String()
	}
	if citation == "" {
		return tb.Str("the record cites no assertion, so no failure can be tied to one").String()
	}
	if phase := FailPhase(verdict); phase != "" && phase != string(interoplab.PhaseChecker) {
		return tb.Str("scenario ").Str(scenario).Str(" failed in ").Str(phase).
			Str(", before its checker ran, so the red is not assertion ").Str(citation).
			Str("'s: ").Str(verdict).String()
	}
	if NamesAssertion(verdict, scenario, citation) {
		return ""
	}
	return tb.Str("the FAIL line of scenario ").Str(scenario).Str(" does not name assertion ").
		Str(citation).Str(" where the checker writes it, `scenario ").Str(scenario).
		Str(" assertion ").Str(citation).Str(":`, the one the record cites: ").
		Str(verdict).String()
}

// NamesAssertion answers whether a FAIL verdict's checker anchor,
// `scenario <name> assertion N:`, carries number as its N.
//
// A checker failure can be wrapped by another one: the relay checker's error
// reaches the suite as `scenario S assertion 0: scenario S assertion 6: cause`.
// The anchor read is the innermost of that chain, each link starting exactly
// where the previous one's colon ends, so a cause text that merely quotes an
// assertion is never one of them.
func NamesAssertion(verdict, scenario, number string) bool {
	anchor := "scenario " + scenario + " assertion "
	_, rest, found := strings.Cut(verdict, anchor)
	if !found {
		return false
	}
	for {
		cited, cause, closed := strings.Cut(rest, ":")
		if !closed {
			return false
		}
		inner, wrapped := strings.CutPrefix(strings.TrimPrefix(cause, " "), anchor)
		if !wrapped {
			return cited == number
		}
		rest = inner
	}
}
