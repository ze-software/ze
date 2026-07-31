package engine

import (
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// dprLiveProbe returns DPD state that raises a probe on the first tick. The liveness
// budget is long, so the owner loop retransmits inside that budget. It does not reach
// its dead-peer branch.
func dprLiveProbe() *dpdState {
	return &dpdState{
		interval: time.Millisecond,
		timeout:  30 * time.Second,
		lastSent: time.Now().Add(-time.Hour),
	}
}

// dprRunLoop starts the owner loop for an established SA on its own goroutine. The
// channel carries the error the loop ends with, so the caller can wait on the end
// rather than on elapsed time.
func dprRunLoop(t *testing.T, ini *SA, ps *PeerSession, dpd *dpdState, myTr *transport.UDPTransport) chan error {
	t.Helper()
	ps.stopCh = make(chan struct{})
	ps.supersede = make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		done <- ps.maintainSA(ini, dpd, nil, nil,
			testIKEGroup(), NewSATable(), nil, myTr, nil, slogutil.DiscardLogger())
	}()
	return done
}

// VALIDATES: an unanswered liveness probe is retransmitted under its own Message ID
// while the liveness budget lasts, and the peer reads every attempt.
// PREVENTS: one dropped datagram that tears down a healthy tunnel. Nothing
// retransmitted a probe. One lost INFORMATIONAL response left awaitReply standing
// until the dead-peer branch fired. The owner inbound queue drops a packet when it
// is full, and that drop alone ended the tunnel.
//
// RFC 7296 Section 2.4 governs the verdict. An endpoint MUST conclude that the other
// endpoint has failed only when repeated attempts to contact it have gone unanswered
// for a timeout period. One attempt is not repeated attempts. RFC 7296 Section 2.1
// makes a retransmission carry the Message ID of the request it repeats.
func TestDprUnansweredProbeIsRetransmitted(t *testing.T) {
	ini, peer, ps, peerTr, myTr := dpdProbeLink(t)
	done := dprRunLoop(t, ini, ps, dprLiveProbe(), myTr)

	first := rtxRecv(t, peerTr)
	if first == nil {
		t.Fatal("the owner loop wrote no liveness probe")
	}
	// The peer answers nothing, which is what a dropped response looks like here.
	second := rtxRecv(t, peerTr)
	if second == nil {
		t.Fatal("the unanswered probe was never retransmitted")
	}
	close(ps.stopCh)
	<-done

	firstHdr := parseMsg(t, first).Header
	secondHdr := parseMsg(t, second).Header
	if secondHdr.ExchangeType != wire.ExchangeInformational {
		t.Errorf("the retransmission exchange = %d, want INFORMATIONAL", secondHdr.ExchangeType)
	}
	if secondHdr.Flags&wire.FlagResponse != 0 {
		t.Error("the retransmission carries the Response flag, so it is not a request")
	}
	if secondHdr.MessageID != firstHdr.MessageID {
		t.Errorf("the retransmission rides Message ID %d, want %d, the id of the request it repeats",
			secondHdr.MessageID, firstHdr.MessageID)
	}
	if _, err := decryptAndParse(peer, parseMsg(t, second), second); err != nil {
		t.Errorf("the peer could not authenticate the retransmission: %v", err)
	}
}

// VALIDATES: the retransmission does not make a dead peer immortal. Once the liveness
// budget the operator configured runs out, the owner loop still ends the SA.
// PREVENTS: a repair that trades a false teardown for a tunnel that never dies. The
// budget is the operator's timeout, and the retransmissions live inside it.
func TestDprSilentPeerStillTimesOut(t *testing.T) {
	ini, _, ps, peerTr, myTr := dpdProbeLink(t)

	// A budget shorter than one tick, so the tick after the probe finds it spent.
	dpd := &dpdState{
		interval: time.Millisecond,
		timeout:  time.Millisecond,
		lastSent: time.Now().Add(-time.Hour),
	}
	done := dprRunLoop(t, ini, ps, dpd, myTr)

	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the owner loop wrote no liveness probe")
	}
	select {
	case err := <-done:
		if !errors.Is(err, errTimeout) {
			t.Fatalf("the silent peer ended the loop with %v, want a liveness timeout", err)
		}
	case <-time.After(rtxArrive):
		close(ps.stopCh)
		<-done
		t.Fatal("the silent peer never reached the dead-peer branch")
	}
}
