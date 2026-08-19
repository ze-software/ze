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

// the two dropped assertions ("the outgoing SA is gone once the request is
// processed", "the incoming SA is still installed until the response") described a state no
// production path reaches, and the code arms that produced it were unreachable and are now
// removed. They are replaced, not weakened: the sequence below is the real one, and the
// record's lifetime is asserted where the ordering used to be.

// VALIDATES: when the peer's Delete crosses one ze already sent for the same pair, the
// response carries no Delete payload for that pair.
// PREVENTS: the duplicate deletion RFC 7296 Section 1.4.1 warns about, where both ends
// answer each other's Delete with a Delete and one of them lands on a reused SPI.
// RFC requirement: RFC7296-1.4.1-7 positive -- RFC 7296 Section 1.4.1: "If a node receives
// a delete request for SAs for which it has already issued a delete request, it MUST delete
// the outgoing SAs while processing the request and the incoming SAs while processing the
// response", and "In that case, the responses MUST NOT include Delete payloads for the
// deleted SAs". Ze removes BOTH halves before it sends its own Delete, so both are already
// gone when each of the section's two events arrives, which satisfies the ordering strictly
// earlier. This drives that production sequence and asserts the bare response.
func TestDelCrossingDeleteAnswersWithoutAPairedDelete(t *testing.T) {
	log := slogutil.DiscardLogger()
	f := delSession(t)

	// The production sequence, in the order inbound.go runs it: record, send, remove.
	f.ps.recordOwnDelete(f.child)
	f.ps.sendDeleteESP(f.local, f.myTr, f.child.InboundSPI, log)
	if rtxRecv(t, f.peerTr) == nil {
		t.Fatal("ze's own Delete never reached the peer, so no crossing can be observed")
	}
	f.local.releaseRequestWindow()
	removeChildSA(f.child, f.dp, log)
	if !f.dp.wasRemoved(f.child.InboundSPI) || !f.dp.wasRemoved(f.child.OutboundSPI) {
		t.Fatal("the production sequence left a half installed, so this fixture is not that state")
	}

	// The peer's Delete for the same pair crosses ours.
	inner, _ := f.inbound(t, espDeletePayload([]uint32{f.child.OutboundSPI}))

	if dels := lcyDeletes(inner); len(dels) != 0 {
		t.Errorf("the crossing response carries %d Delete payloads, want none", len(dels))
	}

	// The record is spent by the response to ze's own Delete, so a LATER Delete naming the
	// same SPI is answered normally rather than swallowed as another crossing.
	respMsg := &wire.Message{Header: wire.Header{MessageID: f.local.ExpectedMsgID}}
	f.ps.handleInformationalOwned(f.local, respMsg, nil, true, f.myTr, f.dp, log)
	if len(f.ps.deleteRequested) != 0 {
		t.Error("the crossing record outlived the response to ze's own Delete")
	}
}

// delNotifies returns every Notify payload in a decrypted chain.
func delNotifies(inner []wire.PayloadEntry) []*wire.PayloadNotify {
	var out []*wire.PayloadNotify
	for i := range inner {
		if n, ok := inner[i].Payload.(*wire.PayloadNotify); ok {
			out = append(out, n)
		}
	}
	return out
}

// VALIDATES: a Delete payload whose ESP SPI Size is not the four octets RFC 7296 Section
// 3.11 fixes draws an INVALID_SYNTAX notification, and closes nothing.
//
// PREVENTS: the defect this test exists for. deleteSPIs returned no SPI for such a payload,
// so the loop closed nothing, `paired` stayed empty, and the peer received an EMPTY
// INFORMATIONAL response. An empty response is what a SUCCESSFUL Delete of nothing looks
// like, so a peer with a broken encoder was told its request was fine.
//
// RFC requirement: RFC7296-3.11-2 negative -- RFC 7296 Section 3.11: the SPI Size "MUST be
// zero for IKE (SPI is in message header) or four for AH and ESP". A payload declaring any
// other size for ESP is refused rather than parsed.
// RFC requirement: RFC7296-2.21.3-1 positive -- RFC 7296 Section 2.21.3: "After the IKE SA
// is authenticated, all requests having errors MUST result in a response notifying the
// other end of the error." The malformed Delete is such a request, and the response names
// the error.
func TestDelMalformedSPISizeDrawsInvalidSyntax(t *testing.T) {
	f := delSession(t)

	// Three octets per SPI is not a size RFC 7296 Section 3.11 permits for ESP.
	bad := &wire.PayloadDelete{
		ProtocolID: wire.ProtocolESP,
		SPISize:    3,
		NumSPIs:    1,
		SPIs:       []byte{0xde, 0xad, 0xbe},
	}
	inner, out := f.inbound(t, bad)

	notifies := delNotifies(inner)
	if len(notifies) != 1 {
		t.Fatalf("the malformed Delete drew %d Notify payloads, want exactly 1; an empty "+
			"response tells the peer its request succeeded", len(notifies))
	}
	if notifies[0].NotifyMsgType != wire.NotifyInvalidSyntax {
		t.Errorf("the malformed Delete drew notify %d, want INVALID_SYNTAX (%d)",
			notifies[0].NotifyMsgType, wire.NotifyInvalidSyntax)
	}
	if dels := lcyDeletes(inner); len(dels) != 0 {
		t.Errorf("the refusal carried %d Delete payloads, want none", len(dels))
	}
	// Nothing was closed, so the tunnel stays up.
	if f.dp.wasRemoved(f.child.InboundSPI) || f.dp.wasRemoved(f.child.OutboundSPI) {
		t.Error("a malformed Delete removed the live Child SA")
	}
	if out.reestablish {
		t.Error("a malformed Delete took the tunnel down")
	}
}

// RFC requirement: RFC7296-3.11-2 positive -- the WELL-FORMED size is accepted through the
// same handler, so the refusal above is the size being checked and not a handler that
// refuses every Delete. RFC 7296 Section 3.11 fixes four octets for ESP, and a four-octet
// payload closes the pair it designates.
func TestDelWellFormedSPISizeIsAccepted(t *testing.T) {
	f := delSession(t)

	inner, out := f.inbound(t, espDeletePayload([]uint32{f.child.OutboundSPI}))

	if notifies := delNotifies(inner); len(notifies) != 0 {
		t.Errorf("a well-formed Delete drew %d Notify payloads, want none", len(notifies))
	}
	if !f.dp.wasRemoved(f.child.OutboundSPI) {
		t.Error("a well-formed Delete closed nothing")
	}
	if !out.reestablish {
		t.Error("closing the pair that carries the tunnel did not take the tunnel-down exit")
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
