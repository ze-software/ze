// VALIDATES: RFC 3948 Section 5.1, "Implementors MUST devise ways of preventing this
// from occurring.", for a tunnel whose Child SA is gone but whose policy is not: the
// stranded record still refuses an overlapping tunnel of another peer (conflictLocked).
// PREVENTS: a second peer's tunnel installed beside a policy still in the dataplane, so
// one packet could match either.

package engine

import (
	"errors"
	"syscall"
	"testing"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestRFC3948StrandedRecordRefusesOverlap: method: peer a's Child SA for inner address
// 10.1.2.3/32 is removed on a dataplane whose policy removal always fails with EIO, so
// its record is stranded and every retry fails; peer b then proposes 10.1.2.3/32 and
// must be answered TS_UNACCEPTABLE with the stranded refusal, with nothing installed on
// b's dataplane.
func TestRFC3948StrandedRecordRefusesOverlap(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- peer a's Child SA for inner address 10.1.2.3/32 (TSr 0.0.0.0/0) is removed by removeChildSA on a dataplane whose policy removal always fails with EIO, so its record is stranded; peer b proposing 10.1.2.3/32 is answered TS_UNACCEPTABLE with an error wrapping errInnerSelectorStranded, and b's dataplane holds no state and no policy.
	log := slogutil.DiscardLogger()
	reg := newInnerSelectorRegistry()
	dp := &failingRemoveDP{err: syscall.EIO}
	child := mustInnerChild(t, innerPeerSA(t, reg, "a", "198.18.0.7"), dp, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
	clearLedgerAfter(t, child)
	removeChildSA(child, dp, log)

	dpB := &mockDP{}
	_, err := innerAuthChild(t, innerPeerSA(t, reg, "b", "198.18.0.9"), dpB, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
	mustInnerRefused(t, err, "peer b beside a's stranded policy")
	if !errors.Is(err, errInnerSelectorStranded) {
		t.Fatalf("refusal %v does not wrap errInnerSelectorStranded", err)
	}
	if len(dpB.sas) != 0 || len(dpB.policies) != 0 {
		t.Fatalf("refused peer b reached its dataplane: %d states, %d policies", len(dpB.sas), len(dpB.policies))
	}
}
