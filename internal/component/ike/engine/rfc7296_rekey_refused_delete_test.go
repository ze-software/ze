// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- a refused rekey answer closes the new pair
// RFC: rfc/short/rfc7296.md -- RFC7296-1.4.1-1

package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestRFC7296RefusedRekeyAnswerDeletesTheNewPair drives a Child SA rekey through the
// session's own exchange path, with a peer that answers with a new SA this node refuses.
//
// VALIDATES: an answer that carries no error notification, and that this node refuses for
// any reason outside the named arms (here, no Nr), sends an ESP Delete listing this node's
// inbound SPI of the new pair, installs nothing and keeps the Child SA in use.
// PREVENTS: the peer keeping a new SA this node never installed. That refusal used to fall
// into the default arm of the rekey response handler (inbound.go), which only logged.
func TestRFC7296RefusedRekeyAnswerDeletesTheNewPair(t *testing.T) {
	// RFC requirement: RFC7296-1.4.1-1 positive -- a Child SA rekey answer this node refuses closes the new pair: an ESP Delete listing this node's inbound SPI of that pair reaches the peer, while the Child SA in use stays installed.
	r := newRefusedRig(t)
	old, err := createFirstChildSA(r.sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, r.dp, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	r.ps.setChildSA(old)
	r.old = old
	installed := len(r.dp.installed)

	r.ps.startChildRekey(r.sa, r.myTr, slogutil.DiscardLogger())
	if r.ps.pendingRekey == nil || r.ps.pendingRekey.kind != rekeyChild {
		t.Fatal("the rekey left no outstanding Child rekey")
	}
	inSPI := r.ps.pendingRekey.newInboundSPI
	// A success answer (no error notify) with no Nr: rekeyRefusal finds nothing, so the
	// refusal is this node's own.
	answer := append([]wire.PayloadEntry{{Payload: espSAPayload(0x99AABBCC)}}, childRekeyAnswerTS(t, old)...)
	r.relayCreate(t, r.peerScripted(t, answer))

	if len(r.dp.installed) != installed {
		t.Errorf("the refused rekey installed %d states", len(r.dp.installed)-installed)
	}
	if r.ps.getChildSA() != old {
		t.Error("the refused rekey replaced the Child SA in use")
	}
	if !deleteListsSPI(t, r.peer, r.peerTr, inSPI) {
		t.Errorf("no ESP Delete listing ze's new inbound SPI %#x reached the peer", inSPI)
	}
	if deleteListsSPI(t, r.peer, r.peerTr, old.InboundSPI) {
		t.Errorf("the Delete closed the Child SA in use (SPI %#x)", old.InboundSPI)
	}
}
