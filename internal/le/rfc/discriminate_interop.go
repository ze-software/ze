// Design: docs/contributing/rfc-conformance-gates.md -- what an interop observation must show
// Overview: discriminate_observe.go -- the observation this judgement serves
// Related: discriminate.go -- interopCitationState, the number a citation must be

package rfc

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The verdicts interoplab.Suite prints on a scenario's progress line,
// `interop: [i/n] <scenario>: pass (<elapsed>)` or
// `interop: [i/n] <scenario>: FAIL (<elapsed>): <error>`
// (internal/le/interoplab/lab.go, Suite.Run and scenarioLine).
const (
	interopVerdictPass = ": pass ("
	interopVerdictFail = ": FAIL ("
)

// interopScenarioVerdict answers the rest of the progress line that gives the
// scenario its verdict, starting at the verdict marker, or false when no such
// line names the scenario.
//
// The scenario is matched on the whole name between `] ` and the marker, so a
// scenario whose name extends another's is never read as it.
func interopScenarioVerdict(output, scenario, marker string) (string, bool) {
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

// interopCleanUnobserved answers why a clean interop run did not observe the
// scenario passing, or the empty string when it did.
//
// An exit status is not that evidence. A selector that matched no scenario, a
// lab whose images never built and a cross-compile of ze that failed each end
// the run with `0 passed, 0 failed`, and a run that executed nothing cannot be
// the green a break is judged against
// (plan/journal/green-that-could-not-have-been-red.md, 2026-10-23).
func (o *observationRunner) interopCleanUnobserved(output string) string {
	if _, ran := interopScenarioVerdict(output, o.scenario, interopVerdictPass); ran {
		return ""
	}
	var tb textbuf.Buffer
	return tb.Str(o.record.Unit).Str(" did not run in the clean run: no ").
		Quoted("interop: [i/n] " + o.scenario + interopVerdictPass).
		Str(" line. A run that executed no scenario is not a green, so nothing a break ").
		Str("does to it can be observed:\n").Str(excerpt(output)).String()
}

// interopBrokenUnobserved answers why a run under the break did not observe
// the scenario failing, or the empty string when it did.
//
// The failure the record rests on is the scenario's own FAIL line. A build of
// the broken ze that did not compile fails the run before any scenario starts,
// and that red says nothing about the claim
// (plan/journal/green-that-could-not-have-been-red.md, 2026-10-23).
func (o *observationRunner) interopBrokenUnobserved(output string) string {
	if _, ran := interopScenarioVerdict(output, o.scenario, interopVerdictFail); ran {
		return ""
	}
	if _, ran := interopScenarioVerdict(output, o.scenario, interopVerdictPass); ran {
		return ""
	}
	var tb textbuf.Buffer
	return describeUnobservedBreak(o.record.Unit,
		tb.Str("no progress line gives scenario ").Str(o.scenario).Str(" a verdict").String(),
		output)
}

// interopAttribution answers why a failing interop run is NOT the cited
// assertion's red, or the empty string when it is.
//
// The checker renders `assertion N` for `fail(N, cause)`, and the citation is
// that N (interopCitationState). The number is matched whole, so assertion 1
// is not named by `assertion 12`.
func (o *observationRunner) interopAttribution(output string) string {
	verdict, failed := interopScenarioVerdict(output, o.scenario, interopVerdictFail)
	var tb textbuf.Buffer
	if !failed {
		return tb.Str("scenario ").Str(o.scenario).Str(" printed no FAIL line").String()
	}
	if o.record.Citation == "" {
		return tb.Str("the record cites no assertion, so no failure can be tied to one").String()
	}
	if namesAssertion(verdict, o.record.Citation) {
		return ""
	}
	return tb.Str("the FAIL line of scenario ").Str(o.scenario).Str(" does not name assertion ").
		Str(o.record.Citation).Str(", the one the record cites: ").Str(verdict).String()
}

// namesAssertion answers whether text carries `assertion <number>` with the
// number complete, not the prefix of a longer one.
func namesAssertion(text, number string) bool {
	needle := "assertion " + number
	for {
		at := strings.Index(text, needle)
		if at < 0 {
			return false
		}
		text = text[at+len(needle):]
		if text == "" {
			return true
		}
		if text[0] < '0' || text[0] > '9' {
			return true
		}
	}
}
