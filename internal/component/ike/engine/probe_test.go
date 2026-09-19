// VALIDATES: spec-ike-padded-path-probe, the two rows that need no datagram: the
// engine registers a prober into the ikeprobe leaf at init, and a probe outstanding
// on an SA the peer rekeys is answered rather than left pending.
// PREVENTS: a leaf nobody registers into, so show mtu reads "no engine in this
// build" on a box that has one; an MTU diagnostic left waiting on a retired SA.
//
// The exchange itself needs the datagram to leave the host with the DF bit set,
// which only Linux carries, so those rows are probe_linux_test.go.
package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/ikeprobe"
	"github.com/ze-software/ze/internal/core/probe"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// probeRequestFor is the request every test here sends: one peer, one size the path
// carries, the kernel cache honored.
func probeRequestFor(peer string) ikeprobe.Request {
	return ikeprobe.Request{Peer: peer, WireOctets: 1400, DF: probe.DFHonorCache}
}

// TestIKEProbeRegisteredByEngine proves that linking the engine registers a prober
// into the leaf: a request for a peer this engine does not run is refused sa-down by
// the ENGINE, which is distinct from the leaf's ErrNotRegistered. A session that owns
// no established SA is refused the same way, at once, without touching its channel.
//
// MUTATION: removing ikeprobe.Register(probePeer) from init (register.go) makes the
// first assertion fail with ErrNotRegistered.
func TestIKEProbeRegisteredByEngine(t *testing.T) {
	SetActivePeersForTest(nil)
	t.Cleanup(func() { SetActivePeersForTest(nil) })

	result, err := ikeprobe.Probe(context.Background(), probeRequestFor("no-such-peer"))
	if errors.Is(err, ikeprobe.ErrNotRegistered) {
		t.Fatal("the engine is linked and the leaf still answers ErrNotRegistered: init registered nothing")
	}
	if err != nil {
		t.Fatalf("a request for an unknown peer answered an error: %v", err)
	}
	if result.Outcome != ikeprobe.OutcomeRefused {
		t.Fatalf("a request for an unknown peer answered %v, want refused", result.Outcome)
	}
	if result.Refusal != ikeprobe.RefusalSADown {
		t.Fatalf("a request for an unknown peer was refused %v, want sa-down", result.Refusal)
	}

	// A configured peer whose owner loop is not running: nothing owns an SA, so the
	// refusal is immediate. The channel is nil on purpose: a send on it would block
	// forever, so a bounded context proves the request never reached it.
	ps := &PeerSession{peerName: "site-a"}
	SetActivePeersForTest(map[string]*PeerSession{ps.peerName: ps})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err = ikeprobe.Probe(ctx, probeRequestFor(ps.peerName))
	if err != nil {
		t.Fatalf("a request for a peer with no established SA answered an error: %v", err)
	}
	if result.Refusal != ikeprobe.RefusalSADown {
		t.Fatalf("a peer with no established SA was refused %v, want sa-down", result.Refusal)
	}
}

// TestProbeRetiredByPeerRekeyIsAnsweredRekeyed pins the arm maintainSA takes when
// the peer rekeys the IKE SA while a padded probe is outstanding on it: the probe is
// answered `refused: rekeyed` naming the size it sent, and nothing is left pending,
// so the loop's retransmit path never repeats a retired SA's request and the MTU
// diagnostic is never left waiting (AC-10). A session with no pending probe is
// untouched.
func TestProbeRetiredByPeerRekeyIsAnsweredRekeyed(t *testing.T) {
	ps := &PeerSession{peerName: "ze"}
	log := slogutil.DiscardLogger()
	ps.retirePendingProbe(log)
	reply := make(chan ikeprobe.Result, 1)
	ps.pendingProbe = &probeState{msgID: 7, octets: 1500, reply: reply}
	ps.retirePendingProbe(log)
	if ps.pendingProbe != nil {
		t.Fatalf("the retired probe is still pending: %+v", ps.pendingProbe)
	}
	select {
	case result := <-reply:
		if result.Outcome != ikeprobe.OutcomeRefused || result.Refusal != ikeprobe.RefusalRekeyed {
			t.Errorf("the retired probe was answered %v/%v, want refused/rekeyed", result.Outcome, result.Refusal)
		}
		if result.WireOctets != 1500 {
			t.Errorf("the answer names %d octets, want the 1500 sent", result.WireOctets)
		}
	default:
		t.Fatal("the retired probe was not answered")
	}
}
