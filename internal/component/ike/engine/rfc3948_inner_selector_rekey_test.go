// Design: docs/architecture/ike/ipsec-14-responder.md -- the cross-peer inner-selector guard
// Related: rekey.go -- newRekeyedChild, which carries the guard to the replacement
// Related: inner_selectors.go -- the registry the replacement is recorded in
// VALIDATES: a Child SA rekey keeps the peer's inner address recorded, so retiring the
// old pair does not hand the address to another peer, on both rekey directions.
// PREVENTS: a replacement Child SA built with no registry, whose claim records nothing,
// so the old pair's removal empties the registry while the peer still holds the address.

package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// rekeyRetiredInner rekeys old, a Child SA of a, in the direction named by how, and
// returns the replacement. The old pair is still installed: the caller retires it.
func rekeyRetiredInner(t *testing.T, how string, a *SA, old *ChildSA, dp *mockDP) *ChildSA {
	t.Helper()
	log := slogutil.DiscardLogger()
	if how == "responder" {
		inner := []wire.PayloadEntry{
			{Payload: espSAPayload(0x01020304)},
			{Payload: &wire.PayloadNonce{NonceData: testNonce(3)}},
			{Payload: tsPayload(t, wire.PayloadTypeTSi, "10.1.2.3/32")},
			{Payload: tsPayload(t, wire.PayloadTypeTSr, "0.0.0.0/0")},
		}
		_, next, err := respondChildRekey(a, inner, old, 5, dp, log)
		if err != nil {
			t.Fatalf("respondChildRekey: %v", err)
		}
		return next
	}
	pending := &pendingRekey{
		kind:          rekeyChild,
		localNonce:    testNonce(1),
		newInboundSPI: 0x11223344,
		oldChild:      old,
	}
	inner := append([]wire.PayloadEntry{
		{Payload: espSAPayload(0xAABBCCDD)},
		{Payload: &wire.PayloadNonce{NonceData: testNonce(2)}},
	}, childRekeyAnswerTS(t, old)...)
	next, err := applyChildRekeyResponse(a, pending, inner, dp, log)
	if err != nil {
		t.Fatalf("applyChildRekeyResponse: %v", err)
	}
	return next
}

// TestRFC3948RekeyedChildKeepsInnerAddress: peer a holds 10.1.2.3/32, its Child SA is
// rekeyed and the old pair retired, and peer b proposing a's inner address is refused.
// Method: each rekey direction in turn, the peer's CREATE_CHILD_SA answered by
// respondChildRekey and Ze's own answered through applyChildRekeyResponse, then
// removeChildSAExcept retires the old pair as the rekey's make-before-break does. The
// registry must still hold one Child SA, a's replacement, and b's IKE_AUTH proposal of
// 10.1.2.3/32 must be answered TS_UNACCEPTABLE with nothing installed.
func TestRFC3948RekeyedChildKeepsInnerAddress(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- after peer a's Child SA holding inner address 10.1.2.3/32 is rekeyed, by the peer (respondChildRekey) or by Ze (applyChildRekeyResponse), and the old pair is retired, the registry still holds a's replacement and a second unbound peer b proposing 10.1.2.3/32 is answered TS_UNACCEPTABLE with no state or policy installed.
	for _, how := range []string{"responder", "initiator"} {
		t.Run(how, func(t *testing.T) {
			log := slogutil.DiscardLogger()
			reg := newInnerSelectorRegistry()
			dpA := &mockDP{}
			a := innerPeerSA(t, reg, "a", "198.18.0.7")
			old := mustInnerChild(t, a, dpA, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})

			next := rekeyRetiredInner(t, how, a, old, dpA)
			removeChildSAExcept(old, next, dpA, log)
			if got := reg.holdings(); got != 1 {
				t.Fatalf("registry holds %d Child SAs after the %s rekey retired the old pair, want 1 (a's replacement)", got, how)
			}

			dpB := &mockDP{}
			b := innerPeerSA(t, reg, "b", "198.18.0.9")
			_, err := innerAuthChild(t, b, dpB, []string{"10.1.2.3/32"}, []string{"192.0.2.10/32"})
			mustInnerRefused(t, err, "peer b after a's "+how+" rekey")
			if len(dpB.sas) != 0 || len(dpB.policies) != 0 {
				t.Fatalf("peer b's dataplane got %d states and %d policies, want none", len(dpB.sas), len(dpB.policies))
			}
		})
	}
}
