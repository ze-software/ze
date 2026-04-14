package l2tp

import (
	"testing"
	"time"
)

// This file holds the phase 2 wiring tests: scenarios that exercise the
// engine end-to-end across its public API. Per the spec's Wiring Test
// table, these stand in for functional .ci tests (which cannot exist
// until phase 3 wires the UDP listener).

// pair wraps two engines simulating the two ends of a tunnel, with a
// controllable clock and per-direction drop flags for the simulated wire.
type pair struct {
	a, b  *ReliableEngine
	t     time.Time
	dropA bool // drop next packet flowing A -> B
	dropB bool // drop next packet flowing B -> A
}

func newPair() *pair {
	// Matched configs: each sees the other's tunnel ID.
	a := NewReliableEngine(ReliableConfig{
		LocalTunnelID:  100,
		PeerTunnelID:   200,
		RTimeout:       time.Second,
		RTimeoutCap:    16 * time.Second,
		MaxRetransmit:  3,
		RecvWindow:     4,
		InitialPeerRWS: 4,
	})
	b := NewReliableEngine(ReliableConfig{
		LocalTunnelID:  200,
		PeerTunnelID:   100,
		RTimeout:       time.Second,
		RTimeoutCap:    16 * time.Second,
		MaxRetransmit:  3,
		RecvWindow:     4,
		InitialPeerRWS: 4,
	})
	return &pair{a: a, b: b, t: time.Unix(0, 0)}
}

// deliverTo hands bytes to the target engine, returning the classify
// result. Honors drop flags.
func (p *pair) deliverTo(target *ReliableEngine, bytes []byte, fromA bool) ReceiveResult {
	if fromA && p.dropA {
		p.dropA = false
		return ReceiveResult{}
	}
	if !fromA && p.dropB {
		p.dropB = false
		return ReceiveResult{}
	}
	hdr, err := ParseMessageHeader(bytes)
	if err != nil {
		return ReceiveResult{}
	}
	return target.OnReceive(hdr, bytes[hdr.PayloadOff:], p.t)
}

// VALIDATES: full four-message SCCRQ/SCCRP/SCCCN/ZLB handshake as in
// RFC 2661 Appendix B.1 (Lock-step tunnel establishment). Both engines
// track sequence numbers consistently; the tunnel handshake completes.
func TestTunnelHandshakeWiring(t *testing.T) {
	p := newPair()

	// A sends SCCRQ (msg type 1).
	sccrq := mustEnqueue(t, p.a, 0, messageTypeAVP(1), p.t)
	ra := p.deliverTo(p.b, sccrq, true)
	if ra.Class != ClassDelivered || ra.Delivered[0].MessageType != 1 {
		t.Fatalf("SCCRQ delivery: %+v", ra)
	}

	// B sends SCCRP (msg type 2) -- piggybacks ACK of A's Ns=0.
	sccrp := mustEnqueue(t, p.b, 0, messageTypeAVP(2), p.t)
	rb := p.deliverTo(p.a, sccrp, false)
	if rb.Class != ClassDelivered || rb.Delivered[0].MessageType != 2 {
		t.Fatalf("SCCRP delivery: %+v", rb)
	}
	if p.a.Outstanding() != 0 {
		t.Errorf("A Outstanding after SCCRP = %d, want 0 (SCCRP acks SCCRQ)", p.a.Outstanding())
	}

	// A sends SCCCN (msg type 3).
	scccn := mustEnqueue(t, p.a, 0, messageTypeAVP(3), p.t)
	ra2 := p.deliverTo(p.b, scccn, true)
	if ra2.Class != ClassDelivered || ra2.Delivered[0].MessageType != 3 {
		t.Fatalf("SCCCN delivery: %+v", ra2)
	}
	if p.b.Outstanding() != 0 {
		t.Errorf("B Outstanding after SCCCN = %d, want 0 (SCCCN acks SCCRP)", p.b.Outstanding())
	}

	// B sends ZLB ACK (no more messages pending, but needsZLB=true).
	if !p.b.NeedsZLB() {
		t.Fatalf("B should need ZLB after receiving SCCCN with no outbound pending")
	}
	buf := make([]byte, 64)
	n := p.b.BuildZLB(buf, 0)
	rb2 := p.deliverTo(p.a, buf[:n], false)
	if rb2.Class != ClassZLB {
		t.Fatalf("ZLB delivery: %+v", rb2)
	}
	if p.a.Outstanding() != 0 {
		t.Errorf("A Outstanding after ZLB = %d, want 0", p.a.Outstanding())
	}
}

// VALIDATES: message lost on the wire triggers retransmit. Matches RFC
// 2661 Appendix B.2 "Lost packet with retransmission".
func TestRetransmitOnDrop(t *testing.T) {
	p := newPair()

	// A sends SCCRQ; drop it on the way to B.
	p.dropA = true
	sccrq := mustEnqueue(t, p.a, 0, messageTypeAVP(1), p.t)
	_ = p.deliverTo(p.b, sccrq, true)
	if p.b.nextRecvSeq != 0 {
		t.Fatalf("B saw dropped message: nextRecvSeq = %d, want 0", p.b.nextRecvSeq)
	}

	// Advance A's clock past the retransmit deadline.
	p.t = p.t.Add(time.Second)
	result := p.a.Tick(p.t)
	if len(result.Retransmits) != 1 {
		t.Fatalf("Tick Retransmits = %d, want 1", len(result.Retransmits))
	}
	if result.TeardownRequired {
		t.Fatalf("TeardownRequired after 1st retransmit")
	}

	// Deliver the retransmitted SCCRQ; B sees it.
	rb := p.deliverTo(p.b, result.Retransmits[0], true)
	if rb.Class != ClassDelivered || rb.Delivered[0].MessageType != 1 {
		t.Errorf("retransmitted SCCRQ not delivered: %+v", rb)
	}
	if p.b.nextRecvSeq != 1 {
		t.Errorf("B nextRecvSeq after retransmit = %d, want 1", p.b.nextRecvSeq)
	}
}

// VALIDATES: out-of-order delivery results in in-order upper-layer
// presentation after the gap fills. RFC 2661 S5.8 "Messages arriving
// out of order may be queued for in-order delivery when the missing
// messages are received".
func TestReorderDelivery(t *testing.T) {
	p := newPair()

	// B wants to send two messages in order but arrive reversed. To
	// simulate we'll Enqueue twice at B and deliver A's packets in
	// reverse order. First boost B's window so both can flight.
	_ = p.b.UpdatePeerRWS(4, p.t) // ensure window>=2
	// B needs to send Ns=0 and Ns=1. Both end up in rtms via Enqueue
	// (window=1 initially from cwnd=1, so second one queues -- but we
	// can trigger "both in flight" by first letting an ACK open cwnd).
	// Simpler path: directly call send via public API in sequence,
	// receiving each before sending the next so both are in flight.
	//
	// Alternative approach used here: bypass Enqueue's window gating by
	// pre-encoding two packets from B destined for A, then deliver
	// them out of order. We do this by constructing the packets via
	// Enqueue with an artificially opened cwnd.
	p.b.win.cwnd = 2
	msg0 := mustEnqueue(t, p.b, 0, messageTypeAVP(1), p.t)
	msg1 := mustEnqueue(t, p.b, 0, messageTypeAVP(2), p.t)

	// Deliver Ns=1 first (out of order for A).
	r1 := p.deliverTo(p.a, msg1, false)
	if r1.Class != ClassReorderQueued {
		t.Fatalf("first delivery: Class = %v, want ClassReorderQueued", r1.Class)
	}
	if len(r1.Delivered) != 0 {
		t.Errorf("premature delivery of out-of-order message: %+v", r1.Delivered)
	}

	// Deliver Ns=0; A should deliver both in order.
	r0 := p.deliverTo(p.a, msg0, false)
	if r0.Class != ClassDelivered {
		t.Fatalf("second delivery: Class = %v, want ClassDelivered", r0.Class)
	}
	if len(r0.Delivered) != 2 {
		t.Fatalf("gap fill delivered %d messages, want 2", len(r0.Delivered))
	}
	if r0.Delivered[0].Ns != 0 || r0.Delivered[1].Ns != 1 {
		t.Errorf("delivery order wrong: %+v", r0.Delivered)
	}
}

// VALIDATES: post-teardown state retention. After A receives StopCCN
// and closes, a duplicate StopCCN retransmit from B is still
// acknowledged via BuildZLB. After retentionDuration elapses, A
// expires. RFC 2661 S5.8 line 2602-2605.
func TestPostTeardownAckRetention(t *testing.T) {
	p := newPair()
	stopCCNBody := messageTypeAVP(4) // StopCCN message type

	// B sends StopCCN.
	stopccn := mustEnqueue(t, p.b, 0, stopCCNBody, p.t)
	ra := p.deliverTo(p.a, stopccn, false)
	if ra.Class != ClassDelivered || ra.Delivered[0].MessageType != 4 {
		t.Fatalf("StopCCN delivery: %+v", ra)
	}
	if !p.a.NeedsZLB() {
		t.Fatalf("A should need ZLB after StopCCN")
	}

	// A builds a ZLB ACK (and simulates loss by NOT delivering to B).
	buf := make([]byte, 64)
	_ = p.a.BuildZLB(buf, 0)

	// A closes its end (phase 3 tunnel state machine decides to tear
	// down after processing StopCCN).
	p.a.Close(p.t)

	// B's retransmit timer fires (B didn't see the lost ZLB). B
	// retransmits its StopCCN.
	p.t = p.t.Add(time.Second)
	bTick := p.b.Tick(p.t)
	if len(bTick.Retransmits) != 1 {
		t.Fatalf("B retransmit: got %d, want 1", len(bTick.Retransmits))
	}

	// A receives the retransmit. Classify as duplicate; needsZLB set.
	rb := p.deliverTo(p.a, bTick.Retransmits[0], false)
	if rb.Class != ClassDuplicate {
		t.Errorf("duplicate StopCCN after close: Class = %v, want ClassDuplicate", rb.Class)
	}
	if !p.a.NeedsZLB() {
		t.Errorf("A should still need ZLB during retention")
	}

	// A is not expired yet (retention = 1+2+4 = 7s for this config).
	if p.a.Expired(p.t.Add(5 * time.Second)) {
		t.Errorf("A expired at +6s; retention is 7s")
	}

	// A is expired after retention.
	expireTime := time.Unix(0, 0).Add(7 * time.Second)
	if !p.a.Expired(expireTime) {
		t.Errorf("A not expired at +7s")
	}
}
