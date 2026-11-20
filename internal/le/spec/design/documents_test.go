// VALIDATES: `le spec design documents spec <path>` derives every design page
// a spec's files link and every decision row those pages carry
// (plan/release/validation/spec-design-needs-owner-approval.md, AC-13).
// PREVENTS: a spec that names a file and never gives the decision on that
// file's design page a verdict.
package specdesign_test

import (
	"testing"

	specdesign "github.com/ze-software/ze/internal/le/spec/design"
)

// TestDesignDocumentsDerivation is the wiring row "`./le spec design documents
// spec <path>`". Method: run the registered command over a spec naming one
// code file; the answer lists the file's // Design: page, its decision row and
// the covering door, each with what derived it.
func TestDesignDocumentsDerivation(t *testing.T) {
	widgetRoot(t)
	answer, code := specdesign.Answer([]string{"documents", "spec", widgetSpec})
	if code != 0 {
		t.Fatalf("documents answered (%#v, %d), want exit 0", answer, code)
	}
	requireContains(t, "the derivation", answerText(t, answer), widgetPage, widgetDecision, widgetDoor, widgetCode)
}
