// Related: hookcheck.go -- the selftest report and the text the sweep prints
//
// VALIDATES: the rendered selftest names every failing result and the cause the
// check recorded for it.
// PREVENTS: a red gate whose whole output is a count, which leaves the operator
// with nothing to act on (plan/journal/failing-gate-prints-no-cause.md).
package hookcheck

import (
	"strings"
	"testing"
)

// TestReportTextNamesEveryFailingResult renders a report holding one pass and
// two failures. The method is Report.Text, which is the only string the action
// sweep prints for a native gate, so a cause the check recorded and the text
// dropped would be invisible in CI.
func TestReportTextNamesEveryFailingResult(t *testing.T) {
	report := Report{
		Results: []Result{
			{Name: "green-check", Passed: true},
			{Name: "digest-check", Code: 2, Message: "content changed: got aa, want bb"},
			{Name: "mute-check", Code: 1},
		},
		Code: 2,
	}
	text := report.Text()
	if !strings.HasPrefix(text, "hook native selftest: 1/3 passed\n") {
		t.Fatalf("summary line = %q", text)
	}
	for _, want := range []string{
		"FAIL digest-check (exit 2): content changed: got aa, want bb",
		"FAIL mute-check (exit 1): no cause recorded by the check",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text does not name %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "green-check") {
		t.Errorf("a passing result was named:\n%s", text)
	}
}
