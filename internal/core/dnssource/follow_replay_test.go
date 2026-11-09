package dnssource

import (
	"testing"
	"time"
)

// TestAwaitReplayHoldsUntilReplayed: a Follower that expects the replay
// holds AwaitReplay until Replayed runs, then passes every later caller at
// once. Method: one goroutine runs AwaitReplay; it must still be held after
// a pause, and released once Replayed runs.
func TestAwaitReplayHoldsUntilReplayed(t *testing.T) {
	var f Follower
	f.ExpectReplay()

	released := make(chan struct{})
	go func() {
		f.AwaitReplay()
		close(released)
	}()

	select {
	case <-released:
		t.Fatal("a caller was released before Replayed ran")
	case <-time.After(100 * time.Millisecond):
	}

	f.Replayed()
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("Replayed did not release the held caller")
	}

	start := time.Now()
	f.AwaitReplay()
	if waited := time.Since(start); waited > 50*time.Millisecond {
		t.Fatalf("a caller after Replayed waited %v", waited)
	}
	// A second Replayed must not close the channel twice.
	f.Replayed()
}

// TestAwaitReplayNotHeldWithoutExpect: the zero Follower expects no replay,
// so AwaitReplay never holds, and Replayed on it is a no-op. Method: time
// AwaitReplay on a zero Follower, before and after Replayed.
func TestAwaitReplayNotHeldWithoutExpect(t *testing.T) {
	var f Follower
	start := time.Now()
	f.AwaitReplay()
	f.Replayed()
	f.AwaitReplay()
	if waited := time.Since(start); waited > 50*time.Millisecond {
		t.Fatalf("a Follower expecting no replay held a caller %v", waited)
	}
}

// TestAwaitReplayBounded: a replay that never arrives holds the first caller
// for ReplayWaitMax at most, then releases every caller. Method: no
// Replayed; time AwaitReplay against the bound, then once more.
func TestAwaitReplayBounded(t *testing.T) {
	var f Follower
	f.ExpectReplay()

	start := time.Now()
	f.AwaitReplay()
	waited := time.Since(start)
	if waited < ReplayWaitMax {
		t.Fatalf("the first caller was released after %v, before the bound %v", waited, ReplayWaitMax)
	}
	if waited > ReplayWaitMax+time.Second {
		t.Fatalf("the first caller was held %v, past the bound %v", waited, ReplayWaitMax)
	}

	start = time.Now()
	f.AwaitReplay()
	if again := time.Since(start); again > 50*time.Millisecond {
		t.Fatalf("a caller after the bound passed waited %v", again)
	}
}
