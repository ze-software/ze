package dnsresolver

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/dnssource"
)

// TestQueryWaitsForReplay: spec-home-cpe-23b AC-13. A resolver that expects
// the learned-resolver replay holds a query until the first snapshot is
// accepted, so a plugin that starts while leases exist never answers from
// the empty list it starts with. Method: one goroutine runs learned.AwaitReplay, the
// query's first step in admit; it must still be held after a pause, and
// released once relearn accepts a snapshot.
func TestQueryWaitsForReplay(t *testing.T) {
	res := newResolver(time.Now)
	res.learned.ExpectReplay()

	released := make(chan struct{})
	go func() {
		res.learned.AwaitReplay()
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
	res.learned.AwaitReplay()
	if waited := time.Since(start); waited > 50*time.Millisecond {
		t.Fatalf("a query after the replay waited %v", waited)
	}
}

// TestQueryNotHeldWithoutReplay: a resolver that expects no replay (a test's,
// or one whose request could not be sent and called learned.Replayed) never holds a
// query. Method: time learned.AwaitReplay on both.
func TestQueryNotHeldWithoutReplay(t *testing.T) {
	plain := newResolver(time.Now)
	start := time.Now()
	plain.learned.AwaitReplay()
	if waited := time.Since(start); waited > 50*time.Millisecond {
		t.Fatalf("a resolver expecting no replay held a query %v", waited)
	}

	unsent := newResolver(time.Now)
	unsent.learned.ExpectReplay()
	unsent.learned.Replayed()
	start = time.Now()
	unsent.learned.AwaitReplay()
	if waited := time.Since(start); waited > 50*time.Millisecond {
		t.Fatalf("a resolver whose request was not sent held a query %v", waited)
	}
}

// TestReplayWaitBounded: a replay that never arrives holds the first query
// for dnssource.ReplayWaitMax at most, then releases every query. Method: no relearn;
// time learned.AwaitReplay against the bound.
func TestReplayWaitBounded(t *testing.T) {
	res := newResolver(time.Now)
	res.learned.ExpectReplay()
	start := time.Now()
	res.learned.AwaitReplay()
	waited := time.Since(start)
	if waited < dnssource.ReplayWaitMax {
		t.Fatalf("the first query was released after %v, before the bound %v", waited, dnssource.ReplayWaitMax)
	}
	if waited > dnssource.ReplayWaitMax+time.Second {
		t.Fatalf("the first query was held %v, past the bound %v", waited, dnssource.ReplayWaitMax)
	}
	start = time.Now()
	res.learned.AwaitReplay()
	if again := time.Since(start); again > 50*time.Millisecond {
		t.Fatalf("a query after the bound passed waited %v", again)
	}
}
