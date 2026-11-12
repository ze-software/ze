// Design: docs/guide/operational-reports.md -- the fix an entry carries
// Related: report.go -- Issue.Fix, RaiseWarningFix, RaiseErrorFix

package report

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestIssueFixCarriedAndOmitted raises a warning and an error event that each
// carry a fix, then a warning that carries none, and marshals each snapshot
// as show warnings and show errors do.
//
// VALIDATES: AC-3. The fix reaches the snapshot and the JSON key "fix"; an
// entry with no fix carries no "fix" key, as an empty detail carries no
// "detail" key; a re-raise refreshes the fix.
// PREVENTS: a producer's fix command lost between the raise and the operator.
func TestIssueFixCarriedAndOmitted(t *testing.T) {
	reset()
	t.Cleanup(reset)

	RaiseWarningFix("bgp", "prefix-threshold", "10.0.0.1", "over limit", "ze show bgp peer 10.0.0.1")
	RaiseErrorFix("plugin", "plugin-exit", "bgp-rib", "exited", "ze show errors source plugin")
	RaiseWarning("iface", "link-down", "eth0", "carrier lost")

	warnings := Warnings()
	fixes := map[string]string{}
	for i := range warnings {
		fixes[warnings[i].Subject] = warnings[i].Fix
	}
	if fixes["10.0.0.1"] != "ze show bgp peer 10.0.0.1" {
		t.Errorf("warning fix: got %q", fixes["10.0.0.1"])
	}
	if fixes["eth0"] != "" {
		t.Errorf("a warning raised with no fix carries %q", fixes["eth0"])
	}
	errs := Errors(0)
	if len(errs) != 1 || errs[0].Fix != "ze show errors source plugin" {
		t.Fatalf("error event fix: got %+v", errs)
	}

	for _, issue := range append(warnings, errs...) {
		raw, err := json.Marshal(issue)
		if err != nil {
			t.Fatalf("marshal %s: %v", issue.Subject, err)
		}
		hasKey := strings.Contains(string(raw), `"fix":`)
		if hasKey != (issue.Fix != "") {
			t.Errorf("%s: JSON %s, fix key present %v for fix %q", issue.Subject, raw, hasKey, issue.Fix)
		}
	}

	RaiseWarningFix("bgp", "prefix-threshold", "10.0.0.1", "over limit", "ze clear bgp peer 10.0.0.1")
	for _, issue := range Warnings() {
		if issue.Subject == "10.0.0.1" && issue.Fix != "ze clear bgp peer 10.0.0.1" {
			t.Errorf("re-raise kept fix %q", issue.Fix)
		}
	}
}

// TestIssueFixBounded raises a warning whose fix is one byte over the bound,
// then one exactly at it.
//
// VALIDATES: a fix longer than FixMax is refused like any other oversized
// field, and one at the bound is kept.
// PREVENTS: a producer filling the bus through an unbounded fix field.
func TestIssueFixBounded(t *testing.T) {
	reset()
	t.Cleanup(reset)

	RaiseWarningFix("bgp", "code", "over", "message", strings.Repeat("f", FixMax+1))
	RaiseWarningFix("bgp", "code", "at", "message", strings.Repeat("f", FixMax))
	warnings := Warnings()
	if len(warnings) != 1 || warnings[0].Subject != "at" {
		t.Fatalf("want only the fix at the bound kept, got %+v", warnings)
	}
}
