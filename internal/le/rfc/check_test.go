package rfc

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
)

// VALIDATES: The public check entry point preserves the checkout's five known RFC 9552 failures.
// PREVENTS: A partial driver silently omits one check or adds unrelated diagnostics.
func TestRFCCheckRealTreeReportsFiveRFC9552Violations(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve checkout: %v", err)
	}
	report, code := Check(root)
	if code != 2 {
		t.Fatalf("the real tree answered %d, want 2:\n%s", code, report.Text())
	}
	if report.CannotRun != "" {
		t.Fatalf("the real tree could not run: %s", report.CannotRun)
	}
	if len(report.Violations) != 5 {
		t.Fatalf("the real tree reported %d violations, want 5:\n%s", len(report.Violations), report.Text())
	}
	expected := []string{
		"RFC9552-5.2.1.1-1 [MUST NOT] has no test and no annotation",
		"RFC9552-5.2.1.1-2 [MUST NOT] has no test and no annotation",
		"RFC9552-8.2.2-9 [MUST] has no test and no annotation",
		"RFC9552-8.2.3-5 [MUST] has no test and no annotation",
		"RFC9552-8.2.6-2 [MUST] has no test and no annotation",
	}
	for index, violation := range report.Violations {
		if !strings.Contains(violation, "rfc/short/rfc9552.md") ||
			!strings.Contains(violation, expected[index]) {
			t.Errorf("violation %d is not the expected RFC 9552 baseline failure:\n%s",
				index, violation)
		}
	}
}

// VALIDATES: `ze-rfc-check` is a claimed read-only action reached as `le rfc check`.
// PREVENTS: A gate claim with no action, or a check incorrectly marked as writing.
func TestRFCCheckActionIsClaimedReadOnly(t *testing.T) {
	for _, action := range Actions().Actions {
		if action.Gate != "ze-rfc-check" {
			continue
		}
		if action.Verb != "check" {
			t.Errorf("the gate is typed as %q, want check", action.Verb)
		}
		if action.Writes {
			t.Error("the check action is marked as writing")
		}
		if len(action.Forks) != 0 {
			t.Errorf("the check action still forks %v", action.Forks)
		}
		return
	}
	t.Error("ze-rfc-check is not claimed by the rfc action table")
}

// VALIDATES: HEAD's functional Gating slice is resolved from Go constants.
// PREVENTS: A baseline reader that silently falls back because it cannot parse the real source shape.
func TestFunctionalSuitesFromGoResolvesTheGatingConstants(t *testing.T) {
	source := `package functional
const (
	suiteParse = "parse"
	suiteUi = "ui"
)
var Gating = []string{suiteParse, suiteUi}
`
	suites, err := functionalSuitesFromGo(source, "HEAD:internal/le/functional/suites.go")
	if err != nil {
		t.Fatalf("parsing the suite source: %v", err)
	}
	if strings.Join(suites, ",") != "parse,ui" {
		t.Fatalf("the parsed Gating list is %v", suites)
	}
}
