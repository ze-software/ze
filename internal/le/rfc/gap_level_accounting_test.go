// Design: docs/contributing/rfc-conformance-gates.md -- obligation-level accounting.
package rfc

import (
	"strings"
	"testing"
)

// TestGapLevelAccounting runs the real check/report stage over one mandatory gap
// and two optional gaps. Descriptive gap totals include all levels, while the
// mandatory denominator and mandatory polarity findings must not absorb them.
func TestGapLevelAccounting(t *testing.T) {
	const source = "RFC 9999\n\n2.  Widgets\n\n   A speaker MUST send the widget.\n   A speaker SHOULD preserve the widget.\n   A speaker MAY retain the widget.\n"
	const rows = "\n## Compliance Checklist\n\n" +
		"- [ ] [RFC9999-2-1] [MUST] A speaker MUST send the widget. (§2) {gap: the fixture does not send}\n" +
		"- [ ] [RFC9999-2-2] [SHOULD] A speaker SHOULD preserve the widget. (§2) {gap: the fixture does not preserve}\n" +
		"- [ ] [RFC9999-2-3] [MAY] A speaker MAY retain the widget. (§2) {gap: the fixture does not retain}\n"
	root := t.TempDir()
	writeFixtureFiles(t, root, map[string]string{
		FixtureAggregateRel: FixtureAggregateProfile,
		selftestWorkflowRel: selftestWorkflow,
		selftestSummaryRel:  "# RFC 9999\n\n" + selftestMeta + rows,
		selftestSourceRel:   source,
	})
	// Check returns a nonzero status for violations; this fixture deliberately
	// lacks mandatory proof. Judge its accounting and findings below instead.
	report, _ := Check(root, nil)
	if report.CannotRun != "" {
		t.Fatal(report.CannotRun)
	}
	if report.Gated != 1 {
		t.Errorf("mandatory denominator=%d, want 1", report.Gated)
	}
	if report.GapsDescribed != 3 {
		t.Errorf("descriptive gaps=%d demonstrated=%d, want 3 and 0", report.GapsDescribed, report.GapsDemonstrated)
	}
	if report.GapsDemonstrated != 0 {
		t.Errorf("descriptive gaps=%d demonstrated=%d, want 3 and 0", report.GapsDescribed, report.GapsDemonstrated)
	}
	for _, finding := range report.Findings {
		optional := strings.Contains(finding.Message, "RFC9999-2-2")
		if !optional {
			optional = strings.Contains(finding.Message, "RFC9999-2-3")
		}
		if !optional {
			continue
		}
		for _, missing := range []string{"has no test", "has no positive", "has no negative"} {
			if strings.Contains(finding.Message, missing) {
				t.Errorf("optional gap acquired mandatory proof debt: %s", finding.Message)
				break
			}
		}
	}
}
