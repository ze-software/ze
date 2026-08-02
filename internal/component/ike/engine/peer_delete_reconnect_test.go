// VALIDATES: a peer's Delete of an ESTABLISHED IKE SA ends the owner loop with a
// NON-NIL error, so PeerSession.run reconnects. The operator `clear` path keeps
// returning nil, because TerminateAllSAs deletes the peer and calls reEstablish
// to rebuild it; nothing rebuilds after a peer's Delete.
// PREVENTS: the tunnel staying down until the next config apply. run reads a nil
// return as a clean shutdown and returns, so the session goroutine exited for
// good the moment a peer said goodbye.

package engine

import (
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// runDeadCase drives maintainSA against an SA a peer has already marked dead and
// returns what the owner loop gave back.
//
// StateDead is read on the loop's one-second tick, so this waits for one.
func runDeadCase(t *testing.T) error {
	t.Helper()
	log := slogutil.DiscardLogger()
	_, resp, ps := establishPSK(t)

	// The state handleDeletePayload leaves behind for an IKE Delete (delete.go).
	resp.State = StateDead

	ps.stopCh = make(chan struct{})
	ps.supersede = make(chan struct{}, 1)

	type result struct{ err error }
	done := make(chan result, 1)
	go func() {
		err := ps.maintainSA(resp, nil, newLifetimeState(3600), newLifetimeState(3600),
			testIKEGroup(), NewSATable(), dataplane.Get(), nil, nil, log)
		done <- result{err: err}
	}()

	select {
	case r := <-done:
		return r.err
	case <-time.After(10 * time.Second):
		close(ps.stopCh)
		<-done
		t.Fatal("the owner loop never noticed StateDead")
		return nil
	}
}

// TestPeerDeleteEndsTheOwnerLoopWithAnError is the positive half.
//
// The assertion is on the ERROR being non-nil, because that is the whole property:
// PeerSession.run (reconcile.go) branches on `err == nil` and returns, so a nil here
// is the tunnel staying down. The sentinel is checked too, so a future edit cannot
// satisfy this by returning some unrelated failure.
func TestPeerDeleteEndsTheOwnerLoopWithAnError(t *testing.T) {
	err := runDeadCase(t)
	if err == nil {
		t.Fatal("the owner loop returned nil after a peer Delete; " +
			"PeerSession.run reads that as a clean shutdown and never reconnects")
	}
	if !errors.Is(err, errSADeletedByPeer) {
		t.Errorf("owner loop returned %v, want errSADeletedByPeer", err)
	}
}

// TestOperatorClearStillEndsTheOwnerLoopCleanly is the negative half, and it is the
// one that stops the fix from being "return an error everywhere".
//
// WHAT IT PINS IS THE DISCRIMINATOR. A maintainSA that returned errSADeletedByPeer from
// every exit would satisfy the positive test above, and the sentinel would stop meaning
// "the peer deleted the SA". The operator `clear` path is the exit that must stay nil, so
// it is the one that measures whether the error is chosen by its CAUSE.
//
// It pins no race, and an earlier version of this comment claimed one. PeerSession.run
// (reconcile.go) returns on `err == nil || ps.stopped()`, and ps.stopped() is true on this
// path, so a non-nil return here could never have reached the reconnect branch.
func TestOperatorClearStillEndsTheOwnerLoopCleanly(t *testing.T) {
	log := slogutil.DiscardLogger()
	_, resp, ps := establishPSK(t)

	ps.stopCh = make(chan struct{})
	ps.supersede = make(chan struct{}, 1)
	close(ps.stopCh) // the first select takes the stop path

	done := make(chan error, 1)
	go func() {
		done <- ps.maintainSA(resp, nil, newLifetimeState(3600), newLifetimeState(3600),
			testIKEGroup(), NewSATable(), dataplane.Get(), nil, nil, log)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("an operator clear returned %v, want nil: the errSADeletedByPeer of "+
				"the test above must be chosen by its cause, not returned from every exit", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the owner loop never took its stop path")
	}
}
