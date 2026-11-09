package dnsresolver

import (
	"testing"
	"time"
)

// TestQueryWaitsForReplay: spec-home-cpe-23b AC-13. A resolver that expects
// the learned-resolver replay holds a query until the first snapshot is
// accepted, so a plugin that starts while leases exist never answers from
// the empty list it starts with. Method: one goroutine runs awaitReplay, the
// query's first step in admit; it must still be held after a pause, and
// released once relearn accepts a snapshot.
func TestQueryWaitsForReplay(t *testing.T) {
	res := newResolver(time.Now)
	res.expectReplay()

	released := make(chan struct{})
	go func() {
		res.awaitReplay()
		close(released)
	}()

	select {
	case <-released:
		t.Fatal("a query was released before any snapshot was accepted")
	case <-time.After(100 * time.Millisecond):
	}

	if err := res.relearn(learnedSnapshot(t, 1, []string{"192.0.2.53"})); err != nil {
		t.Fatalf("relearn: %v", err)
	}
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("the accepted snapshot did not release the held query")
	}

	// Every later query passes at once.
	start := time.Now()
	res.awaitReplay()
	if waited := time.Since(start); waited > 50*time.Millisecond {
		t.Fatalf("a query after the replay waited %v", waited)
	}
}

// TestQueryNotHeldWithoutReplay: a resolver that expects no replay (a test's,
// or one whose request could not be sent and called replayed) never holds a
// query. Method: time awaitReplay on both.
func TestQueryNotHeldWithoutReplay(t *testing.T) {
	plain := newResolver(time.Now)
	start := time.Now()
	plain.awaitReplay()
	if waited := time.Since(start); waited > 50*time.Millisecond {
		t.Fatalf("a resolver expecting no replay held a query %v", waited)
	}

	unsent := newResolver(time.Now)
	unsent.expectReplay()
	unsent.replayed()
	start = time.Now()
	unsent.awaitReplay()
	if waited := time.Since(start); waited > 50*time.Millisecond {
		t.Fatalf("a resolver whose request was not sent held a query %v", waited)
	}
}

// TestReplayWaitBounded: a replay that never arrives holds the first query
// for replayWaitMax at most, then releases every query. Method: no relearn;
// time awaitReplay against the bound.
func TestReplayWaitBounded(t *testing.T) {
	res := newResolver(time.Now)
	res.expectReplay()
	start := time.Now()
	res.awaitReplay()
	waited := time.Since(start)
	if waited < replayWaitMax {
		t.Fatalf("the first query was released after %v, before the bound %v", waited, replayWaitMax)
	}
	if waited > replayWaitMax+time.Second {
		t.Fatalf("the first query was held %v, past the bound %v", waited, replayWaitMax)
	}
	start = time.Now()
	res.awaitReplay()
	if again := time.Since(start); again > 50*time.Millisecond {
		t.Fatalf("a query after the bound passed waited %v", again)
	}
}
