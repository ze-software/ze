package l2tp

import "testing"

// VALIDATES: AC-15 slow-start growth. CWND increments by 1 per ACK while
// CWND < SSTHRESH. PREVENTS: the stagnation bug where slow start never
// gets out of the starting gate because CWND is stuck at 1.
func TestWindowSlowStart(t *testing.T) {
	w := newWindow(4) // peer RWS = 4, so SSTHRESH = 4, initial CWND = 1
	if w.cwnd != 1 {
		t.Fatalf("initial cwnd = %d, want 1", w.cwnd)
	}
	if w.ssthresh != 4 {
		t.Fatalf("initial ssthresh = %d, want 4 (= peer RWS)", w.ssthresh)
	}

	w.onAck() // cwnd 1 -> 2 (slow start: 1 < 4)
	if w.cwnd != 2 {
		t.Errorf("after ack 1: cwnd = %d, want 2", w.cwnd)
	}
	w.onAck() // 2 -> 3
	w.onAck() // 3 -> 4 (enters congestion avoidance)
	if w.cwnd != 4 {
		t.Errorf("after 3 acks: cwnd = %d, want 4", w.cwnd)
	}
}

// VALIDATES: AC-16 congestion-avoidance integer fractional counter. Once
// CWND >= SSTHRESH, CWND grows by 1 only after CWND ACKs. PREVENTS: the
// "1/CWND in integer arithmetic == 0 forever" trap (guide S12.3).
//
// Construct the window directly at cwnd=4/ssthresh=4 with peerRWS=8 so
// growth past 4 is not clamped by the flow-control cap. A natural climb
// from cwnd=1 via newWindow(8) works too but requires 7 slow-start acks
// plus 8 avoidance acks just to see cwnd go 8->9; the direct
// construction exercises the fractional counter more tightly.
func TestWindowCongestionAvoidance(t *testing.T) {
	w := &window{cwnd: 4, ssthresh: 4, counter: 0, peerRWS: 8}

	// In avoidance, need CWND acks to increment CWND.
	for i := range 3 {
		w.onAck()
		if w.cwnd != 4 {
			t.Errorf("avoidance ack %d: cwnd = %d, want 4 (counter not yet filled)", i+1, w.cwnd)
		}
	}
	w.onAck() // 4th avoidance ack: counter 3->4, cwnd 4 -> 5, counter resets
	if w.cwnd != 5 {
		t.Errorf("after 4 avoidance acks: cwnd = %d, want 5", w.cwnd)
	}
	if w.counter != 0 {
		t.Errorf("counter after increment: %d, want 0", w.counter)
	}
}

// VALIDATES: AC-17 CWND capped at peer RWS. PREVENTS: CWND growing
// unbounded into violation of the peer's advertised window (RFC 2661
// Appendix A: "CWND is never allowed to exceed the size of the
// advertised window").
func TestWindowCappedByPeerRWS(t *testing.T) {
	w := newWindow(2) // tiny peer RWS
	// Push through slow start.
	w.onAck() // 1 -> 2 (enters avoidance at cwnd=ssthresh=2)
	w.onAck()
	w.onAck()
	if w.cwnd > 2 {
		t.Errorf("cwnd = %d exceeds peer RWS of 2", w.cwnd)
	}
}

// VALIDATES: AC-14 retransmit resets the window to slow start with
// halved SSTHRESH. PREVENTS: a congestion event leaving CWND elevated
// (which would worsen the congestion the retransmit just signaled).
func TestWindowRetransmitReset(t *testing.T) {
	w := newWindow(8)
	// Grow cwnd to 4 via slow start.
	w.onAck()
	w.onAck()
	w.onAck()
	if w.cwnd != 4 {
		t.Fatalf("setup: cwnd = %d, want 4", w.cwnd)
	}

	w.onRetransmit()
	if w.cwnd != 1 {
		t.Errorf("after retransmit: cwnd = %d, want 1", w.cwnd)
	}
	if w.ssthresh != 2 {
		t.Errorf("after retransmit: ssthresh = %d, want 2 (= max(4/2, 1))", w.ssthresh)
	}
	if w.counter != 0 {
		t.Errorf("after retransmit: counter = %d, want 0", w.counter)
	}
}

// VALIDATES: AC-14 edge case: retransmit when CWND=1 must not produce
// SSTHRESH=0. RFC 2661 Appendix A: "one half of the CWND is saved in
// SSTHRESH" with the floor at 1 (otherwise slow-start phase could
// degenerate, never entering avoidance).
func TestWindowRetransmitFloor(t *testing.T) {
	w := newWindow(4)
	// cwnd is already 1.
	w.onRetransmit()
	if w.ssthresh != 1 {
		t.Errorf("ssthresh floor: %d, want 1", w.ssthresh)
	}
	if w.cwnd != 1 {
		t.Errorf("cwnd = %d, want 1", w.cwnd)
	}
}

// VALIDATES: AC-18 peer RWS = 0 is coerced to 1. RFC 2661 Section 5.8
// line 2616-2617: "A value of 0 for the Receive Window Size AVP is
// invalid". Guide Section 24.6 recommends treating as 1.
func TestWindowPeerRWSZero(t *testing.T) {
	w := newWindow(0)
	if w.peerRWS != 1 {
		t.Errorf("peer RWS 0 coerced to %d, want 1", w.peerRWS)
	}
	if w.ssthresh != 1 {
		t.Errorf("ssthresh = %d, want 1", w.ssthresh)
	}
}

// VALIDATES: peer RWS above RecvWindowMax is clamped to RecvWindowMax.
// A hostile or buggy peer advertising an oversized window would break
// sequence-number classification (outstanding Ns values spanning >32767
// are ambiguous under seqBefore) and could force unbounded rtms_queue
// memory. clampPeerRWS is the single point of defense.
func TestWindowPeerRWSAboveMax(t *testing.T) {
	cases := []struct {
		in, want uint16
	}{
		{RecvWindowMax, RecvWindowMax},
		{RecvWindowMax + 1, RecvWindowMax},
		{40000, RecvWindowMax},
		{65535, RecvWindowMax},
	}
	for _, tc := range cases {
		t.Run("new", func(t *testing.T) {
			w := newWindow(tc.in)
			if w.peerRWS != tc.want {
				t.Errorf("newWindow(%d).peerRWS = %d, want %d", tc.in, w.peerRWS, tc.want)
			}
		})
		t.Run("update", func(t *testing.T) {
			w := newWindow(4)
			w.updatePeerRWS(tc.in)
			if w.peerRWS != tc.want {
				t.Errorf("updatePeerRWS(%d) -> peerRWS = %d, want %d", tc.in, w.peerRWS, tc.want)
			}
		})
	}
}

// VALIDATES: updatePeerRWS honors mid-tunnel changes (e.g., peer
// re-advertises in SCCRP after SCCRQ initial). CWND caps and SSTHRESH
// initial are derived fresh from the new value.
func TestWindowUpdatePeerRWS(t *testing.T) {
	w := newWindow(4)
	w.onAck() // cwnd 1 -> 2
	w.onAck() // cwnd 2 -> 3

	w.updatePeerRWS(8)
	if w.peerRWS != 8 {
		t.Errorf("peerRWS after update = %d, want 8", w.peerRWS)
	}
	// CWND should be preserved (not reset to 1 -- that's retransmit's
	// job). But it must not exceed the new cap.
	if w.cwnd != 3 {
		t.Errorf("cwnd preserved, got %d, want 3", w.cwnd)
	}

	w.updatePeerRWS(2) // shrink below current cwnd
	if w.cwnd != 2 {
		t.Errorf("cwnd clamped to new peerRWS: got %d, want 2", w.cwnd)
	}
}

// VALIDATES: available() reports how many new messages may be sent. Key
// for the engine's Enqueue gating logic.
func TestWindowAvailable(t *testing.T) {
	w := newWindow(4)
	// outstanding = 0, cwnd = 1, peerRWS = 4. available = min(cwnd, peerRWS) - outstanding = 1.
	if got := w.available(0); got != 1 {
		t.Errorf("available(0) with cwnd=1 = %d, want 1", got)
	}
	if got := w.available(1); got != 0 {
		t.Errorf("available(1) with cwnd=1 = %d, want 0", got)
	}
	// Push cwnd to 4.
	w.onAck()
	w.onAck()
	w.onAck()
	if got := w.available(2); got != 2 {
		t.Errorf("available(2) with cwnd=4 = %d, want 2", got)
	}
	if got := w.available(4); got != 0 {
		t.Errorf("available(4) with cwnd=4 = %d, want 0", got)
	}
}
