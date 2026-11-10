// Design: docs/architecture/ike/ipsec-6-ikev2-crypto.md -- RFC 7296 Section 3.3.6, an ESP offer missing a mandatory Transform Type.
// Related: rfc7296_esn_test.go -- the ESN values the responder answers and the initiator accepts.

package engine

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// RFC requirement: RFC7296-3.3.6-4 negative -- "If the responder receives a proposal that
// contains a Transform Type it does not understand, or a proposal that is missing a
// mandatory Transform Type, it MUST consider this proposal unacceptable" (RFC 7296 Section
// 3.3.6). RFC 7296 Section 3.3.3 makes ENCR and ESN the mandatory types of ESP, so an ESP
// proposal that carries ze's suite but no Extended Sequence Numbers transform is refused
// with NO_PROPOSAL_CHOSEN by every responder selection: selectResponderESP (the Child SA of
// IKE_AUTH), selectNewChildESP (a new Child SA by CREATE_CHILD_SA) and respondChildRekey (a
// Child SA rekey by CREATE_CHILD_SA).
// RFC requirement: RFC7296-3.3.6-4 positive -- "however, other proposals in the same SA
// payload are processed as usual": with that proposal first and the same suite plus ESN 0
// second, both new-SA selections accept the second, and the Proposal Num they echo is its.
// The rekey accepts the same suite with ESN 0, so its refusal is the missing type alone.
//
// Method: the product selection entry points over hand-built SA payloads. The two offers
// differ only in the ESN transform, so a refusal of the first names the missing type.
func TestRFC7296ResponderRefusesESPOfferMissingESN(t *testing.T) {
	missing := esnSAPayload(t, 1)
	if ids := esnIDsOf(missing.Proposals[0]); len(ids) != 0 {
		t.Fatalf("the ESN-less offer carries ESN transforms %v, so it tests nothing", ids)
	}

	t.Run("ike-auth-refuses", func(t *testing.T) {
		sa := esnResponderSA(t)
		err := selectResponderESP(sa, missing)
		if !errors.Is(err, crypto.ErrNoProposalChosen) {
			t.Fatalf("an ESP offer with no ESN transform was answered with %v, want NO_PROPOSAL_CHOSEN", err)
		}
		if got := notifyForRefusal(err); got != wire.NotifyNoProposalChosen {
			t.Errorf("the refusal maps to notify %d, want NO_PROPOSAL_CHOSEN (%d)", got, wire.NotifyNoProposalChosen)
		}
	})

	t.Run("create-child-refuses", func(t *testing.T) {
		_, _, err := selectNewChildESP(missing, testESPGroup(), dhGroupNone)
		if !errors.Is(err, crypto.ErrNoProposalChosen) {
			t.Fatalf("a CREATE_CHILD_SA offer with no ESN transform was answered with %v, want NO_PROPOSAL_CHOSEN", err)
		}
	})

	// The ESN-less proposal first, then the same suite with ESN 0 as proposal 2.
	both := esnSAPayload(t, 1)
	both.Proposals = append(both.Proposals, esnSAPayload(t, 2, espESNNotExtended).Proposals[0])

	t.Run("ike-auth-takes-the-next-proposal", func(t *testing.T) {
		sa := esnResponderSA(t)
		if err := selectResponderESP(sa, both); err != nil {
			t.Fatalf("the complete second proposal was refused: %v", err)
		}
		if sa.ChildProposalNum != 2 {
			t.Errorf("selected proposal %d, want the complete second one", sa.ChildProposalNum)
		}
	})

	t.Run("create-child-takes-the-next-proposal", func(t *testing.T) {
		_, number, err := selectNewChildESP(both, testESPGroup(), dhGroupNone)
		if err != nil {
			t.Fatalf("the complete second proposal was refused: %v", err)
		}
		if number != 2 {
			t.Errorf("selected proposal %d, want the complete second one", number)
		}
	})

	t.Run("child-rekey-refuses", func(t *testing.T) {
		sa, old, dp, log := peerRekeyFixture(t)
		_, child, err := respondChildRekey(sa, esnRekeyRequest(t, missing), old, 5, dp, log)
		if !errors.Is(err, crypto.ErrNoProposalChosen) {
			t.Fatalf("a Child SA rekey offer with no ESN transform was answered with %v, want NO_PROPOSAL_CHOSEN", err)
		}
		if child != nil {
			t.Error("a replacement Child SA was installed for a refused rekey offer")
		}
	})

	t.Run("child-rekey-accepts-esn-0", func(t *testing.T) {
		sa, old, dp, log := peerRekeyFixture(t)
		complete := esnSAPayload(t, 1, espESNNotExtended)
		_, child, err := respondChildRekey(sa, esnRekeyRequest(t, complete), old, 5, dp, log)
		if err != nil {
			t.Fatalf("the same rekey offer with ESN 0 was refused, so the refusal above is not the missing type: %v", err)
		}
		if child == nil {
			t.Fatal("the rekey offer with ESN 0 installed no replacement Child SA")
		}
	})
}

// esnRekeyRequest builds the CREATE_CHILD_SA rekey a peer sends with the given SA payload,
// inside the scope peerRekeyFixture has in use. The peer sent Ni, so its own side is TSi.
func esnRekeyRequest(t *testing.T, offer *wire.PayloadSA) []wire.PayloadEntry {
	t.Helper()
	return []wire.PayloadEntry{
		{Payload: offer},
		{Payload: &wire.PayloadNonce{NonceData: testNonce(3)}},
		{Payload: tsPayload(t, wire.PayloadTypeTSi, "10.2.0.0/24")},
		{Payload: tsPayload(t, wire.PayloadTypeTSr, "10.1.0.0/24")},
	}
}
