// Design: docs/contributing/committing.md -- recorded owner acts remain noninteractive.
package commit

import (
	"testing"

	"github.com/ze-software/ze/internal/le/le/human/humantest"
)

// TestRecordedAttestationConsumptionRemainsNoninteractive builds isolated git
// history carrying an owner's recorded answer and derives its debt under pipes.
// It issues no live approval and executes no push or prepared attestation script.
func TestRecordedAttestationConsumptionRemainsNoninteractive(t *testing.T) {
	root := newOwedRepository(t)
	owed := commitOwed(t, root, "the change", map[string]string{"a.txt": "a\n"}, nil, "Review-owed: demo: no reviewer")
	commitOwed(t, root, "the recorded answer", nil, nil, trailerOwnerAttested+stablePatchID(t, root, owed)+": the owner reviewed this fixture")
	humantest.Streams(t, "piped-both")
	derived := deriveOwed(t, root, "")
	if derived.owed() {
		t.Fatalf("recorded attestation left debt under pipes: %+v", derived)
	}
	if err := derived.pushRefusal(); err != nil {
		t.Fatalf("recorded attestation consumption required a terminal: %v", err)
	}
}
