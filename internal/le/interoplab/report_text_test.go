package interoplab

import (
	"strings"
	"testing"
)

// TestSuiteReportTextNamesTheFailedScenario proves the terminal rendering of a suite
// report carries the failed scenario and its assertion error, and leaves a passing
// scenario out.
//
// VALIDATES: SuiteReport.Text prints each failed scenario with its error and cleanup
// errors, then the passed/failed summary.
// PREVENTS: `./le integration interop` printing "Failed: interop" and nothing else.
func TestSuiteReportTextNamesTheFailedScenario(t *testing.T) {
	report := SuiteReport{
		Scenarios: []ScenarioResult{
			{Name: "bgp-basic-frr", Passed: true},
			{Name: "ospf-ipsec-frr", Error: "assertion 2: wait for frr output timed out", CleanupErrors: []string{"rm lab: busy"}},
		},
		Passed:      1,
		Failed:      1,
		FailedNames: []string{"ospf-ipsec-frr"},
	}

	text := report.Text()
	for _, want := range []string{
		"interop: ospf-ipsec-frr: FAIL: assertion 2: wait for frr output timed out\n",
		"interop: ospf-ipsec-frr: cleanup: rm lab: busy\n",
		"interop: 1 passed, 1 failed: ospf-ipsec-frr\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("rendering lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "bgp-basic-frr") {
		t.Errorf("a passing scenario is printed:\n%s", text)
	}
}
