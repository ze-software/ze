package reactor

import (
	"testing"
)

// freeCount reads how many buffers the pool still has to lend.
func freeCount(pp *peerPool) int {
	pp.mu.Lock()
	defer pp.mu.Unlock()
	return pp.top
}

// TestDispatchOverflowReleasesItemWhenStopped pins the buffer return on the
// shutdown path.
//
// VALIDATES: an item handed to DispatchOverflow after the pool has stopped gives
// its Outgoing Peer Pool buffer back.
// PREVENTS: the leak this replaces. Both stopped branches called item.done() and
// returned false without releaseItem. done() releases the recent-update cache
// reference, which the doc comment says and which is NOT the same as returning
// the buffers, so the MOD buffer the caller had already acquired was never
// handed back. Callers construct the item holding it (reactor_api_forward.go and
// forward_rs.go build fwdItem with peerBufIdx and peerPoolRef set), and neither
// TryDispatch nor the caller releases on a false return, so nothing else could
// have.
func TestDispatchOverflowReleasesItemWhenStopped(t *testing.T) {
	pp := newPeerPool(64)
	before := freeCount(pp)

	buf, idx := pp.Get()
	if buf == nil || idx == 0 {
		t.Fatal("setup: the pool must lend a buffer")
	}
	if freeCount(pp) != before-1 {
		t.Fatalf("setup: expected the pool down one buffer, got %d of %d", freeCount(pp), before)
	}

	fp := &fwdPool{}
	fp.stopped = true

	var doneCalled bool
	item := fwdItem{
		peerBufIdx:  idx,
		peerPoolRef: pp,
		done:        func() { doneCalled = true },
	}

	if fp.DispatchOverflow(fwdKey{}, item) {
		t.Fatal("a stopped pool must refuse the dispatch")
	}

	if !doneCalled {
		t.Error("done() must still run: it releases the recent-update cache reference")
	}
	if got := freeCount(pp); got != before {
		t.Errorf("the peer-pool buffer leaked: %d free, want %d", got, before)
	}
}

// TestDispatchOverflowStoppedIsSafeWithoutABuffer proves the release is
// conditional, so the shutdown path does not fault on items carrying no pool
// resource.
//
// VALIDATES: a sentinel item (no peer, no buffer) still takes the stopped path
// cleanly. forward_pool_barrier.go dispatches exactly such an item.
// PREVENTS: an unconditional Return on a zero index, which peerPool.Return
// rejects with an out-of-range error rather than ignoring.
func TestDispatchOverflowStoppedIsSafeWithoutABuffer(t *testing.T) {
	fp := &fwdPool{}
	fp.stopped = true

	var doneCalled bool
	if fp.DispatchOverflow(fwdKey{}, fwdItem{done: func() { doneCalled = true }}) {
		t.Fatal("a stopped pool must refuse the dispatch")
	}
	if !doneCalled {
		t.Error("done() must run for a sentinel item too")
	}
}

// COVERAGE NOTE, recorded because the gap is real and a reader should not assume
// otherwise.
//
// DispatchOverflow has TWO stopped branches and these tests reach only the first.
// Setting fp.stopped before the call makes the fast path (RLock) return
// immediately, so the slow path is never entered. The second branch is reachable
// only when the pool is running at the RLock check, the worker for the key does
// not exist, and Stop lands in the window between that RUnlock and the
// subsequent Lock.
//
// Mutation-verified 2026-08-05: reverting the release in the FIRST branch fails
// TestDispatchOverflowReleasesItemWhenStopped. Reverting it in the SECOND branch
// fails nothing, so that release is asserted by no test.
//
// It is kept because it is the same obligation on the same item and omitting it
// would leave a leak on a path that does occur, not because a test proves it.
// Closing the gap needs a synchronization seam in DispatchOverflow that does not
// exist today, and adding one to the forwarding hot path to test a shutdown race
// is a worse trade than recording this.
