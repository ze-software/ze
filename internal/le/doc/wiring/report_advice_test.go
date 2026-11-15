// Design: docs/contributing/documentation-testing.md -- reviewed claim repair.

package docwiring

import (
	"strings"
	"testing"
)

// A drift finding must offer the ledger route for a claim that remains true,
// while keeping the page repair for a changed claim. Exercise the prose answer
// a caller sees, rather than reading the advice table directly.
func TestDocDriftReportExplainsReviewedClaimRepair(t *testing.T) {
	report := Report{Failed: true, Checks: []CheckResult{{
		Name:       checkDocDriftName,
		Failed:     true,
		Message:    "Documentation drift FAILED:",
		Violations: []string{"docs/example.md:3: Documented changed under this claim"},
	}}}
	text := report.Text()
	for _, want := range []string{
		"Update each page",
		"code that made it wrong",
		"<!-- source: -->",
		"When the change leaves",
		"the claim true",
		"plan/doc-reviewed/<session>.md",
		"Doc, Source, Symbol, Commit and Reason",
		"every unpushed commit",
		"docs/contributing/documentation-testing.md",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("drift repair omits %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "the page still owes the edit") {
		t.Errorf("an unchanged claim is still told to edit its page:\n%s", text)
	}
}
