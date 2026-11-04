// Design: docs/contributing/committing.md -- exact target/unit publication cannot be renewed by a new reason.
package commit

import (
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

// TestPublishedApprovalTargetCannotBeRenewedWithAnotherReason publishes a real
// targeted judge row, records a different verdict for the same exact pair, and
// requires the native create entry to refuse spending that pair a second time.
func TestPublishedApprovalTargetCannotBeRenewedWithAnotherReason(t *testing.T) {
	root, first, _ := approvalPublicationFixture(t)
	prepared := createApprovalPublication(t, first, approvalUnit)
	runCommitScript(t, root, prepared.Script)
	if payload, code := rfc.Answer([]string{"approve", "commit", first, "unit", approvalUnit,
		"reason", "judge: another wording for the same reviewed change"}); code != 0 {
		t.Fatalf("record replacement verdict = (%#v, %d)", payload, code)
	}
	pending := readApprovalFile(t, root)
	if _, code := Answer([]string{"create", "subject", "do not renew a published target", "rfc-approval", first, approvalUnit}); code != 1 {
		t.Fatalf("renewed target publication exit = %d, want 1", code)
	}
	if got := readApprovalFile(t, root); got != pending {
		t.Fatal("refused replay changed pending approval rows")
	}
}
