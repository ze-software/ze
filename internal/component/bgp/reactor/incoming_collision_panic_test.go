// Design: docs/architecture/behavior/peer-lifecycle.md -- panic-safe incoming arbitration.
// Related: incoming_ingress_corrections_test.go -- real TCP admission and useful incumbent.
package reactor

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/test/sim"
)

const incomingCollisionPanic = "incoming collision clock failure"

// TestIncomingCollisionPanicReleasesPeerOwner admits a real OPEN, then injects
// a clock failure at the commit-side HoldBudget check under Peer.mu. Recovery
// must leave that owner free, retirement bounded and the incumbent useful.
func TestIncomingCollisionPanicReleasesPeerOwner(t *testing.T) {
	peer, original, outgoing := incomingCorrectionPeer(t, 0)
	session := peer.newIncomingSession()
	fc := &incomingCollisionPanicClock{FakeClock: sim.NewFakeClock(time.Now()), peer: peer}
	session.SetClock(fc)
	incoming, local := incomingCorrectionStart(t, session, false)
	bridgeDone := make(chan struct{})
	close(bridgeDone)
	candidate := &incomingCandidate{
		peer: peer, conn: local, session: session,
		readDone: make(chan struct{}), handoffDone: make(chan struct{}),
		bridgeStop: make(chan struct{}), bridgeDone: bridgeDone,
		progress: make(chan struct{}, 1),
	}
	peer.mu.Lock()
	peer.pending = candidate
	peer.mu.Unlock()
	type admissionResult struct {
		open *candidateOpen
		err  error
	}
	admitted := make(chan admissionResult, 1)
	go func() {
		defer close(candidate.readDone)
		open, err := candidate.readOpen(t.Context(), session.bufReader)
		admitted <- admissionResult{open: open, err: err}
	}()
	t.Cleanup(func() {
		closeConnQuietly(incoming)
		incomingCorrectionAwait(t, candidate.readDone, "candidate admission cleanup")
		candidate.retire()
	})
	delayOpenWrite(t, incoming, delayOpenRemote(0x03030303))
	var admission admissionResult
	select {
	case admission = <-admitted:
	case <-time.After(5 * time.Second):
		t.Fatal("candidate OPEN admission did not finish")
	}
	if admission.err != nil {
		t.Fatal(admission.err)
	}
	incomingCorrectionAwait(t, candidate.readDone, "candidate admission completion")
	fc.armed.Store(true)
	var contained any
	func() {
		defer func() { contained = recover() }()
		session.openMu.Lock()
		defer session.unlockOpen()
		peer.resolveIncoming(candidate, admission.open, admission.open.open.BGPIdentifier)
	}()
	if contained != incomingCollisionPanic {
		t.Fatalf("commit-side panic was not exercised under Peer.mu: %v", contained)
	}
	if peer.mu.TryLock() {
		peer.mu.Unlock()
	} else {
		t.Error("collision panic stranded Peer.mu before candidate retirement")
		// Only this test calls resolveIncoming: no Peer loop, handoff or real
		// timer worker exists. The injected panic proved this exact owner was
		// held, before commit could launch a handoff. Release the leaked lock
		// only after recording RED so old-source cleanup cannot hang the run.
		peer.mu.Unlock()
	}
	retired := make(chan struct{})
	go func() {
		defer close(retired)
		peer.beginIncomingRetirement(candidate)
		candidate.retire()
		peer.clearIncoming(local)
	}()
	incomingCorrectionAwait(t, retired, "candidate retirement after collision panic")
	incomingIngressEOF(t, incoming)
	select {
	case <-session.done:
	default:
		t.Fatal("candidate retirement left its Session unfinished")
	}
	if session.State() != fsm.StateActive {
		t.Fatalf("retired candidate state=%s, want Active after OpenSent TCP failure", session.State())
	}
	if session.timers.IsHoldTimerRunning() {
		t.Fatal("retired candidate left its HoldTimer armed")
	}
	if peer.hasPendingConnection() {
		t.Fatal("candidate retirement retained incoming admission")
	}
	peer.mu.RLock()
	inbound, handoff := peer.inbound, peer.collisionHandoff
	peer.mu.RUnlock()
	if inbound != nil {
		t.Fatal("collision panic published an inbound receipt")
	}
	if handoff {
		t.Fatal("collision panic reserved a handoff")
	}
	if peer.currentSession() != original {
		t.Fatal("collision panic replaced the incumbent")
	}
	incomingCorrectionConfirm(t, original, outgoing)
}

// incomingCollisionPanicClock injects one failure through the existing clock
// seam after valid OPEN admission. Fake timers never dispatch in this fixture,
// so only the synchronous collision decision can own Peer.mu at this point.
type incomingCollisionPanicClock struct {
	*sim.FakeClock
	peer  *Peer
	armed atomic.Bool
}

func (c *incomingCollisionPanicClock) Now() time.Time {
	if c.armed.Swap(false) {
		if c.peer.mu.TryRLock() {
			c.peer.mu.RUnlock()
			panic("clock failure outside incoming decision")
		}
		panic(incomingCollisionPanic)
	}
	return c.FakeClock.Now()
}
