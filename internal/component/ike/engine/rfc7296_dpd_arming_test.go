// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- Dead Peer Detection

package engine

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// dpdArmedTick runs the probe step of the owner loop (maintainSA, established.go)
// at the instant now: a probe goes out when shouldSend answers true. It returns
// whether a probe was written.
func dpdArmedTick(sa *SA, tr *transport.UDPTransport, dpd *dpdState, now time.Time) bool {
	before := sa.NextMsgID
	if dpd.shouldSend(now) {
		sendDPD(sa, tr, dpd, slogutil.DiscardLogger())
	}
	return sa.NextMsgID != before
}

// dpdArmedState builds the DPD state the owner loop builds for a peer whose
// dead-peer-detection interval is 30 seconds, and fails when nothing is armed.
func dpdArmedState(t *testing.T) *dpdState {
	t.Helper()
	dpd := newDPDState(ipsec.DPDConfig{Interval: 30, Timeout: 120, Action: ipsec.DPDActionRestart})
	if dpd == nil {
		t.Fatal("newDPDState armed no liveness check for a 30-second interval")
	}
	return dpd
}

// VALIDATES: an IKE SA on which nothing protected arrives is checked for liveness
// once the configured interval has passed since the state was armed. The method is
// the owner loop's own probe step driven at explicit instants, so no wall clock is
// slept on, and the probe is read off the peer's socket.
// PREVENTS: a peer that dies while only outgoing traffic flows being sent to for
// ever, the black hole Section 2.4 names.
//
// RFC 7296 Section 2.4: "If no cryptographically protected messages have been
// received on an IKE SA or any of its Child SAs recently, the system needs to
// perform a liveness check in order to prevent sending messages to a dead peer."
//
// RFC requirement: RFC7296-2.4-2 positive -- newDPDState (dpd.go) arms the check
// for a non-zero interval: with no protected message received, no probe is due
// before the interval, and once 30 seconds have passed the probe step writes one
// liveness probe that reaches the peer and is then awaited.
func TestRFC7296LivenessCheckFollowsTheInterval(t *testing.T) {
	ini, _, _, peerTr, myTr := dpdProbeLink(t)
	dpd := dpdArmedState(t)
	armed := dpd.lastSent

	if dpdArmedTick(ini, myTr, dpd, armed.Add(29*time.Second)) {
		t.Fatal("a probe went out before the 30-second interval passed")
	}
	if !dpdArmedTick(ini, myTr, dpd, armed.Add(31*time.Second)) {
		t.Fatal("no probe went out once the interval passed with nothing received")
	}
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the liveness probe never reached the peer")
	}
	if !dpd.awaitingReply() {
		t.Error("the probe that went out is not awaited, so no answer can credit liveness")
	}
}

// VALIDATES: while a liveness probe is outstanding, the probe step raises no second
// one, however long the reply takes. Ticks well past the interval are driven at
// explicit instants and the request counter is read after each.
// PREVENTS: a probe raised on every tick of an unanswered check, which would spend
// a new Message ID each time instead of repeating the one request (Section 2.1).
//
// RFC requirement: RFC7296-2.4-2 negative -- shouldSend (dpd.go) answers false
// at 61 and 300 seconds after arming while the first probe is awaited, so the
// probe step at those instants writes nothing new, and the check stays one
// outstanding request.
func TestRFC7296LivenessCheckWaitsForTheOutstandingProbe(t *testing.T) {
	ini, _, _, _, myTr := dpdProbeLink(t)
	dpd := dpdArmedState(t)
	armed := dpd.lastSent

	if !dpdArmedTick(ini, myTr, dpd, armed.Add(31*time.Second)) {
		t.Fatal("no first probe went out once the interval passed")
	}
	firstID := ini.NextMsgID
	for _, later := range []time.Duration{61 * time.Second, 300 * time.Second} {
		if dpd.shouldSend(armed.Add(later)) {
			t.Errorf("shouldSend answered true at %v while the first probe was awaited", later)
		}
		if dpdArmedTick(ini, myTr, dpd, armed.Add(later)) {
			t.Errorf("a second probe went out at %v while the first was outstanding", later)
		}
	}
	if ini.NextMsgID != firstID {
		t.Errorf("Message ID = %d after the outstanding ticks, want %d", ini.NextMsgID, firstID)
	}
	if !dpd.awaitingReply() {
		t.Error("the first probe is no longer awaited")
	}
}
