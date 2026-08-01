package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// delFixture is an established session with one installed Child SA pair, the loopback
// transports that capture what ze sends, and the fake dataplane the pair was installed
// into.
type delFixture struct {
	local, peer  *SA
	ps           *PeerSession
	peerTr, myTr *transport.UDPTransport
	dp           *rkyDP
	child        *ChildSA
}

// delSession builds a delFixture.
func delSession(t *testing.T) *delFixture {
	t.Helper()
	log := slogutil.DiscardLogger()
	local, peer, ps, peerTr, myTr := lcyLoopback(t)
	dp := &rkyDP{}
	child, err := createFirstChildSA(local, testESPGroup(), "10.0.0.2", "10.0.0.1", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	// The two SPIs must differ, or "closed the pair the peer named" cannot be told
	// apart from "closed some pair".
	if child.InboundSPI == child.OutboundSPI {
		t.Fatalf("the fixture pair shares one SPI (%#x), so no assertion below discriminates", child.InboundSPI)
	}
	ps.setChildSA(child)
	return &delFixture{local: local, peer: peer, ps: ps, peerTr: peerTr, myTr: myTr, dp: dp, child: child}
}

// inbound drives one peer INFORMATIONAL Delete request through the owner-loop handler
// and returns the payload chain of the response ze sent, plus the outcome.
func (f *delFixture) inbound(t *testing.T, del *wire.PayloadDelete) ([]wire.PayloadEntry, ownedOutcome) {
	t.Helper()
	log := slogutil.DiscardLogger()
	msg := &wire.Message{Header: wire.Header{MessageID: f.local.ExpectedMsgID}}
	inner := []wire.PayloadEntry{{Payload: del}}
	out := f.ps.handleInformationalOwned(f.local, msg, inner, false, f.myTr, f.dp, log)
	raw := rtxRecv(t, f.peerTr)
	if raw == nil {
		t.Fatal("the Delete request drew no INFORMATIONAL response at all")
	}
	return lcyDecrypt(t, f.peer, raw), out
}

// VALIDATES: a peer Delete naming a live Child SA by SPI closes that pair, and a Delete
// naming an SPI the session does not hold closes nothing.
// PREVENTS: the regression where handleDeletePayload ignored del.SPIs entirely and
// removed only the make-before-break leftover, so a peer that tore a live Child SA down
// left ze encrypting to an SA that no longer existed.
// RFC requirement: RFC7296-1.4.1-6 positive -- RFC 7296 Section 1.4.1: the recipient
// "MUST close the designated SAs". The pair the peer names by SPI is removed from the
// dataplane, both halves.
func TestDelPeerDeleteClosesTheDesignatedChildSA(t *testing.T) {
	f := delSession(t)

	if f.dp.installedSA(f.child.InboundSPI) == nil || f.dp.installedSA(f.child.OutboundSPI) == nil {
		t.Fatal("the fixture pair is not installed, so its removal proves nothing")
	}

	_, out := f.inbound(t, espDeletePayload([]uint32{f.child.OutboundSPI}))

	if !f.dp.wasRemoved(f.child.OutboundSPI) {
		t.Error("the designated outbound SA is still installed after the peer's Delete")
	}
	if !out.reestablish {
		t.Error("closing the pair that carries the tunnel did not take the tunnel-down exit")
	}
}

// RFC requirement: RFC7296-1.4.1-6 negative -- an SPI this session does not hold
// designates nothing, so the live pair stays installed. Without this the positive test
// above would pass against a handler that closed the Child SA on any Delete at all,
// which is exactly the defect it exists to prevent.
func TestDelUnknownSPIClosesNothing(t *testing.T) {
	f := delSession(t)

	const strangerSPI uint32 = 0x5A5A5A5A
	if strangerSPI == f.child.InboundSPI || strangerSPI == f.child.OutboundSPI {
		t.Fatal("the stranger SPI collides with the fixture pair")
	}

	_, out := f.inbound(t, espDeletePayload([]uint32{strangerSPI}))

	if f.dp.wasRemoved(f.child.InboundSPI) || f.dp.wasRemoved(f.child.OutboundSPI) {
		t.Error("a Delete naming an unknown SPI removed the live Child SA")
	}
	if out.reestablish {
		t.Error("a Delete naming an unknown SPI took the tunnel down")
	}
	if f.ps.getChildSA() != f.child {
		t.Error("the session dropped its Child SA over a Delete that designated nothing")
	}
}

// VALIDATES: the INFORMATIONAL response to a Child SA Delete carries a Delete payload
// naming the paired SA going the other way.
// PREVENTS: the recorded gap on RFC7296-1.4-1, where an inbound Delete was acknowledged
// with an empty INFORMATIONAL response and the peer was never told which of its own SAs
// to close.
// RFC requirement: RFC7296-1.4-1 positive -- RFC 7296 Section 1.4.1: "Normally, the
// response in the INFORMATIONAL exchange will contain Delete payloads for the paired SAs
// going in the other direction." The response names ze's INBOUND SPI, which is the half
// the peer still holds.
func TestDelResponseCarriesThePairedDelete(t *testing.T) {
	f := delSession(t)

	inner, _ := f.inbound(t, espDeletePayload([]uint32{f.child.OutboundSPI}))

	got := lcyOneESPDelete(t, inner)
	if got != f.child.InboundSPI {
		t.Errorf("the paired Delete names SPI %#x, want the inbound half %#x", got, f.child.InboundSPI)
	}
	// The response must not name the half the peer just deleted: that is the direction
	// the peer already closed, and naming it back is the duplicate deletion Section
	// 1.4.1 warns about.
	if got == f.child.OutboundSPI {
		t.Error("the response echoes the SPI the peer deleted instead of the paired one")
	}
}

// RFC requirement: RFC7296-1.4-1 negative -- an IKE SA Delete draws an EMPTY response.
// RFC 7296 Section 1.4.1: "The response to a request that deletes the IKE SA is an empty
// INFORMATIONAL response." So the paired Delete is attached to the pairs actually closed,
// and not to every Delete that arrives.
func TestDelIKEDeleteDrawsAnEmptyResponse(t *testing.T) {
	f := delSession(t)

	inner, _ := f.inbound(t, &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE})

	if dels := lcyDeletes(inner); len(dels) != 0 {
		t.Errorf("the response to an IKE SA Delete carries %d Delete payloads, want none", len(dels))
	}
	if f.local.State != StateDead {
		t.Error("the IKE SA survived the peer's Delete of it")
	}
}

// VALIDATES: when the peer's Delete crosses one ze already sent for the same pair, the
// response carries no Delete payload for that pair.
// PREVENTS: the duplicate deletion RFC 7296 Section 1.4.1 warns about, where both ends
// answer each other's Delete with a Delete and one of them lands on a reused SPI.
// RFC requirement: RFC7296-1.4.1-7 positive -- RFC 7296 Section 1.4.1: "If a node
// receives a delete request for SAs for which it has already issued a delete request, it
// MUST delete the outgoing SAs while processing the request and the incoming SAs while
// processing the response." The same paragraph: "In that case, the responses MUST NOT
// include Delete payloads for the deleted SAs". Both halves are checked here: the
// outgoing SA is gone once the request is processed, and the response is bare.
func TestDelCrossingDeleteAnswersWithoutAPairedDelete(t *testing.T) {
	log := slogutil.DiscardLogger()
	f := delSession(t)

	// Ze issues its own Delete for the pair first, and holds it installed. This is the
	// state the crossing case is defined over.
	f.ps.recordOwnDelete(f.child)
	f.ps.sendDeleteESP(f.local, f.myTr, f.child.InboundSPI, log)
	if rtxRecv(t, f.peerTr) == nil {
		t.Fatal("ze's own Delete never reached the peer, so no crossing can be observed")
	}
	f.local.releaseRequestWindow()
	if f.dp.wasRemoved(f.child.OutboundSPI) {
		t.Fatal("the pair was already removed, so the ordering below cannot be observed")
	}

	inner, _ := f.inbound(t, espDeletePayload([]uint32{f.child.OutboundSPI}))

	if dels := lcyDeletes(inner); len(dels) != 0 {
		t.Errorf("the crossing response carries %d Delete payloads, want none", len(dels))
	}
	if !f.dp.wasRemoved(f.child.OutboundSPI) {
		t.Error("the outgoing SA is still installed after the crossing request was processed")
	}
	if f.dp.wasRemoved(f.child.InboundSPI) {
		t.Error("the incoming SA went while processing the REQUEST; Section 1.4.1 puts it on the response")
	}

	// The second half of the ordering: the incoming SA goes when ze processes the
	// response to its own Delete.
	respMsg := &wire.Message{Header: wire.Header{MessageID: f.local.ExpectedMsgID}}
	f.ps.handleInformationalOwned(f.local, respMsg, nil, true, f.myTr, f.dp, log)
	if !f.dp.wasRemoved(f.child.InboundSPI) {
		t.Error("the incoming SA is still installed after the response was processed")
	}
}

// RFC requirement: RFC7296-1.4.1-7 negative -- with no Delete of ze's own outstanding,
// the very same inbound Delete DOES draw a paired Delete payload. The suppression is
// therefore the crossing record talking, and not a handler that never answers.
func TestDelWithoutOwnDeleteTheSameRequestIsPaired(t *testing.T) {
	f := delSession(t)

	if len(f.ps.deleteRequested) != 0 {
		t.Fatal("the fixture already holds an outstanding Delete, so this is not the negative case")
	}

	inner, _ := f.inbound(t, espDeletePayload([]uint32{f.child.OutboundSPI}))

	if got := lcyOneESPDelete(t, inner); got != f.child.InboundSPI {
		t.Errorf("the paired Delete names SPI %#x, want %#x", got, f.child.InboundSPI)
	}
}
