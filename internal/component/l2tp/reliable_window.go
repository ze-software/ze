// Design: docs/architecture/wire/l2tp.md — L2TP reliable delivery engine
// RFC: rfc/short/rfc2661.md — RFC 2661 Appendix A (slow start & congestion avoidance)
// Related: reliable.go — engine core that holds the window struct per tunnel

package l2tp

// window carries the per-tunnel congestion and flow-control state. RFC
// 2661 Section 5.8 combines two windows: the peer's advertised Receive
// Window Size (a flow-control limit MUST be honored) and the congestion
// window CWND (a SHOULD from Appendix A). The effective send window is
// min(CWND, peerRWS).
//
// NOT safe for concurrent use. The engine owns this struct and runs on
// phase 3's reactor goroutine.
type window struct {
	// cwnd is the congestion window in messages. Starts at 1, grows per
	// the slow-start / congestion-avoidance algorithm, resets to 1 on
	// retransmit.
	cwnd uint16

	// ssthresh is the slow-start threshold. Initialized from peer RWS,
	// halved on retransmit (floored at 1).
	ssthresh uint16

	// counter is the fractional-ACK accumulator used during congestion
	// avoidance: once it reaches cwnd, CWND grows by 1 and counter
	// resets. This is the integer encoding of "1/CWND per ACK" from
	// RFC 2661 Appendix A, matching guide Section 12.3.
	counter uint16

	// peerRWS is the peer's advertised receive window size. Coerced from
	// 0 to 1 on construction (RFC 2661 Section 5.8 line 2616-2617: "A
	// value of 0 for the Receive Window Size AVP is invalid"; guide
	// Section 24.6 recommends treat-as-1).
	peerRWS uint16
}

// newWindow constructs the congestion/flow state for a peer whose
// advertised receive window is peerRWS. If peerRWS is 0, it is coerced
// to 1 per RFC 2661 Section 5.8. If peerRWS exceeds RecvWindowMax
// (32768, half the 16-bit Ns space), it is clamped because windows
// larger than that break sequence-number classification: outstanding
// Ns values spanning >32767 are ambiguous under seqBefore.
func newWindow(peerRWS uint16) *window {
	peerRWS = clampPeerRWS(peerRWS)
	return &window{
		cwnd:     1,
		ssthresh: peerRWS,
		counter:  0,
		peerRWS:  peerRWS,
	}
}

// onAck is called once per newly acknowledged message (i.e., each
// message that the peer's Nr advanced past). Drives CWND growth per
// RFC 2661 Appendix A: linear during slow start, fractional during
// congestion avoidance. Never exceeds peerRWS.
func (w *window) onAck() {
	if w.cwnd < w.ssthresh {
		// Slow start: +1 per ACK (exponential per RTT).
		w.cwnd++
	} else {
		// Congestion avoidance: +1/CWND per ACK, integer-encoded via
		// the fractional counter.
		w.counter++
		if w.counter >= w.cwnd {
			w.cwnd++
			w.counter = 0
		}
	}
	if w.cwnd > w.peerRWS {
		w.cwnd = w.peerRWS
	}
}

// onRetransmit is called when the retransmission timer fires (signaling
// congestion). RFC 2661 Appendix A: "one half of the CWND is saved in
// SSTHRESH, and CWND is set to one". SSTHRESH is floored at 1 so that
// slow start eventually transitions into avoidance even after repeated
// retransmits.
func (w *window) onRetransmit() {
	w.ssthresh = max(w.cwnd/2, 1)
	w.cwnd = 1
	w.counter = 0
}

// updatePeerRWS handles a mid-tunnel change to the peer's advertised
// receive window (e.g., SCCRQ uses default, SCCRP carries the real
// value). CWND is preserved but clamped to the new cap. SSTHRESH is
// NOT touched -- it reflects past congestion history, not the current
// flow-control limit. peerRWS is also clamped defensively via
// clampPeerRWS; see newWindow for the rationale.
func (w *window) updatePeerRWS(peerRWS uint16) {
	peerRWS = clampPeerRWS(peerRWS)
	w.peerRWS = peerRWS
	if w.cwnd > peerRWS {
		w.cwnd = peerRWS
	}
}

// clampPeerRWS maps a raw peer-advertised or config-supplied Receive
// Window Size to a legal value: 0 becomes 1 (RFC 2661 S5.8 line
// 2616-2617 "value of 0 ... is invalid"), values above RecvWindowMax
// become RecvWindowMax (a peer advertising >32768 is either buggy or
// hostile; larger windows break the 32767-half seqBefore
// classification).
func clampPeerRWS(peerRWS uint16) uint16 {
	if peerRWS == 0 {
		return 1
	}
	if peerRWS > RecvWindowMax {
		return RecvWindowMax
	}
	return peerRWS
}

// available reports how many additional messages may be placed in
// flight given the current outstanding count. Returns min(cwnd, peerRWS)
// - outstanding, clamped at 0.
func (w *window) available(outstanding uint16) uint16 {
	limit := min(w.cwnd, w.peerRWS)
	if outstanding >= limit {
		return 0
	}
	return limit - outstanding
}
