// Design: docs/contributing/rfc-implementation-guide.md -- recorded authorization remains agent-usable.
package rfc

import (
	"testing"

	"github.com/ze-software/ze/internal/le/le/human/humantest"
)

// TestRecordedOwnerRulingConsumptionRemainsNoninteractive issues through Approve
// with a terminal, then reads the real ledger and consumes its ruling with pipes.
func TestRecordedOwnerRulingConsumptionRemainsNoninteractive(t *testing.T) {
	root := t.TempDir()
	const reason = "owner ruling 2026-11-03 accepts the absent feature"
	t.Run("issue", func(t *testing.T) {
		humantest.Streams(t, "terminal")
		if _, err := Approve(root, approvalSession, "pkg.TestX", reason); err != nil {
			t.Fatal(err)
		}
	})
	humantest.Streams(t, "piped-both")
	approvals, err := approvalRows(root, approvalSession)
	if err != nil {
		t.Fatal(err)
	}
	requirement := Requirement{RID: "fixture", Annotation: &Annotation{Kind: AnnotationGap, Reason: reason}}
	held := map[Cover][]Tag{{RID: "fixture", Polarity: PolarityNegative, Unit: "pkg/widget_test.go::TestX"}: nil}
	if !ownerRuledMove(requirement, PolarityNegative, held, approvals) {
		t.Fatal("a recorded owner ruling was not consumed under pipes")
	}
}
