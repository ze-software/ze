// VALIDATES: the two halves of the RFC 7296 Section 5 PRF rule. The allowlist holds no
// PRF below the floor, and the floor refuses a size below it. Neither half writes to a
// package-level map, so the result does not depend on test order.
// PREVENTS: a PRF below 128 bits reaching negotiation. It also prevents an unreachable
// guard passing as evidence. The floor is proven with a number rather than with an entry
// injected into the specified set.
package crypto

import (
	"errors"
	"testing"
)

// RFC requirement: RFC7296-5-3 negative -- "A PRF with an output of less than 128 bits MUST NOT be
// used with this protocol" (RFC 7296 Section 5). An output of 64 bits is refused, and so
// is the size one below the floor. An identifier the specified set does not hold is
// refused before any size is read.
// RFC requirement: RFC7296-5-3 positive -- the floor itself is accepted, and every PRF this
// implementation specifies is at or above it. The specified set is what a running daemon
// consults, so that set carries the rule. The floor guards the set against a later entry.
func TestPrfFloorRefusesOutputBelow128Bits(t *testing.T) {
	if err := prfOutputAcceptable(64, true); !errors.Is(err, ErrPRFTooWeak) {
		t.Errorf("prfOutputAcceptable(64) = %v, want ErrPRFTooWeak", err)
	}
	if err := prfOutputAcceptable(minPRFOutputBits-1, true); !errors.Is(err, ErrPRFTooWeak) {
		t.Errorf("prfOutputAcceptable(%d) = %v, want ErrPRFTooWeak", minPRFOutputBits-1, err)
	}
	if err := prfOutputAcceptable(minPRFOutputBits, true); err != nil {
		t.Errorf("prfOutputAcceptable(%d) = %v, want acceptance at the floor", minPRFOutputBits, err)
	}
	if err := prfOutputAcceptable(0, false); !errors.Is(err, ErrTransformUnspecified) {
		t.Errorf("prfOutputAcceptable(unspecified) = %v, want ErrTransformUnspecified", err)
	}
	if !errors.Is(ErrPRFTooWeak, ErrNoProposalChosen) {
		t.Error("ErrPRFTooWeak must report as ErrNoProposalChosen to the peer")
	}

	// The allowlist is the half that runs for a real offer. It holds no entry the floor
	// would refuse, and it is not empty.
	if len(prfOutputBits) == 0 {
		t.Fatal("the specified PRF set is empty, so the check above proves nothing")
	}
	for id, bits := range prfOutputBits {
		if err := prfOutputAcceptable(bits, true); err != nil {
			t.Errorf("specified PRF %d has a %d-bit output: %v", id, bits, err)
		}
	}
}
