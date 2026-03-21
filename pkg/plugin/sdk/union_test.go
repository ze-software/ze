package sdk

import (
	"sync"
	"testing"
	"time"
)

// VALIDATES: Union correlates two event streams by message ID
// PREVENTS: Events delivered uncorrelated or lost

func TestUnionBothArrive(t *testing.T) {
	// Both primary and secondary arrive -- handler called with both.
	var mu sync.Mutex
	var gotPrimary, gotSecondary string

	u := NewUnion("update", "rpki", 5*time.Second, func(primary, secondary string) {
		mu.Lock()
		gotPrimary = primary
		gotSecondary = secondary
		mu.Unlock()
	})
	defer u.Stop()

	u.OnEvent("update", "10.0.0.1", 42, `{"update":"data"}`)
	u.OnEvent("rpki", "10.0.0.1", 42, `{"rpki":"valid"}`)

	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if gotPrimary != `{"update":"data"}` {
		t.Fatalf("expected primary, got %q", gotPrimary)
	}
	if gotSecondary != `{"rpki":"valid"}` {
		t.Fatalf("expected secondary, got %q", gotSecondary)
	}
}

func TestUnionPrimaryFirst(t *testing.T) {
	// Primary arrives first, secondary follows -- handler called when secondary arrives.
	var mu sync.Mutex
	var called int

	u := NewUnion("update", "rpki", 5*time.Second, func(_, _ string) {
		mu.Lock()
		called++
		mu.Unlock()
	})
	defer u.Stop()

	u.OnEvent("update", "10.0.0.1", 1, `primary`)
	time.Sleep(20 * time.Millisecond)

	mu.Lock()
	c := called
	mu.Unlock()
	if c != 0 {
		t.Fatalf("handler should not be called yet, called=%d", c)
	}

	u.OnEvent("rpki", "10.0.0.1", 1, `secondary`)
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if called != 1 {
		t.Fatalf("expected 1 call, got %d", called)
	}
}

func TestUnionSecondaryFirst(t *testing.T) {
	// Secondary arrives before primary -- handler called when primary arrives.
	var mu sync.Mutex
	var gotPrimary, gotSecondary string

	u := NewUnion("update", "rpki", 5*time.Second, func(primary, secondary string) {
		mu.Lock()
		gotPrimary = primary
		gotSecondary = secondary
		mu.Unlock()
	})
	defer u.Stop()

	u.OnEvent("rpki", "10.0.0.1", 7, `sec`)
	time.Sleep(20 * time.Millisecond)
	u.OnEvent("update", "10.0.0.1", 7, `pri`)
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if gotPrimary != `pri` || gotSecondary != `sec` {
		t.Fatalf("expected (pri, sec), got (%q, %q)", gotPrimary, gotSecondary)
	}
}

func TestUnionTimeout(t *testing.T) {
	// Secondary never arrives -- handler called with empty secondary after timeout.
	var mu sync.Mutex
	var gotPrimary, gotSecondary string

	u := NewUnion("update", "rpki", 100*time.Millisecond, func(primary, secondary string) {
		mu.Lock()
		gotPrimary = primary
		gotSecondary = secondary
		mu.Unlock()
	})
	u.sweepInterval = 100 * time.Millisecond // faster sweep for test
	defer u.Stop()

	u.OnEvent("update", "10.0.0.1", 99, `timeout-test`)
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if gotPrimary != `timeout-test` {
		t.Fatalf("expected primary after timeout, got %q", gotPrimary)
	}
	if gotSecondary != "" {
		t.Fatalf("expected empty secondary on timeout, got %q", gotSecondary)
	}
}

func TestUnionFlushPeer(t *testing.T) {
	// FlushPeer delivers all pending for that peer with empty secondary.
	var mu sync.Mutex
	var calls int

	u := NewUnion("update", "rpki", 5*time.Second, func(_, _ string) {
		mu.Lock()
		calls++
		mu.Unlock()
	})
	defer u.Stop()

	u.OnEvent("update", "10.0.0.1", 1, `a`)
	u.OnEvent("update", "10.0.0.1", 2, `b`)
	u.OnEvent("update", "10.0.0.2", 3, `c`) // different peer

	u.FlushPeer("10.0.0.1")
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if calls != 2 {
		t.Fatalf("expected 2 flushed calls for 10.0.0.1, got %d", calls)
	}
}

func TestUnionMaxPending(t *testing.T) {
	// Oldest entries evicted when max pending reached.
	var mu sync.Mutex
	var evictedPrimary string

	u := NewUnion("update", "rpki", 5*time.Second, func(primary, secondary string) {
		mu.Lock()
		evictedPrimary = primary
		mu.Unlock()
	})
	u.maxPending = 2 // override for test
	defer u.Stop()

	u.OnEvent("update", "10.0.0.1", 1, `first`)
	u.OnEvent("update", "10.0.0.1", 2, `second`)
	// Third should evict first
	u.OnEvent("update", "10.0.0.1", 3, `third`)
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if evictedPrimary != `first` {
		t.Fatalf("expected oldest evicted, got %q", evictedPrimary)
	}
}

func TestUnionOrphanSecondary(t *testing.T) {
	// VALIDATES: Orphan secondary (no primary ever arrives) is discarded after sweep timeout
	// PREVENTS: Handler called with empty primary for orphan secondary events
	var mu sync.Mutex
	var called int

	u := NewUnion("update", "rpki", 100*time.Millisecond, func(_, _ string) {
		mu.Lock()
		called++
		mu.Unlock()
	})
	u.sweepInterval = 100 * time.Millisecond // faster sweep for test
	defer u.Stop()

	// Send only secondary -- no primary ever arrives.
	u.OnEvent("rpki", "10.0.0.1", 50, `orphan-secondary`)
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if called != 0 {
		t.Fatalf("handler should NOT be called for orphan secondary, called=%d", called)
	}
}

func TestUnionFlushPeerSecondaryOnly(t *testing.T) {
	// VALIDATES: FlushPeer with secondary-only entries does not call handler
	// PREVENTS: Handler called with empty primary when only secondary exists
	var mu sync.Mutex
	var called int

	u := NewUnion("update", "rpki", 5*time.Second, func(_, _ string) {
		mu.Lock()
		called++
		mu.Unlock()
	})
	defer u.Stop()

	// Send only secondary for a peer.
	u.OnEvent("rpki", "10.0.0.1", 60, `sec-only`)

	u.FlushPeer("10.0.0.1")
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	c := called
	mu.Unlock()
	if c != 0 {
		t.Fatalf("handler should NOT be called for secondary-only flush, called=%d", c)
	}
}

func TestUnionCorrelationKey(t *testing.T) {
	// Same message ID + different peers are separate entries.
	var mu sync.Mutex
	var calls int

	u := NewUnion("update", "rpki", 5*time.Second, func(_, _ string) {
		mu.Lock()
		calls++
		mu.Unlock()
	})
	defer u.Stop()

	u.OnEvent("update", "10.0.0.1", 42, `p1`)
	u.OnEvent("update", "10.0.0.2", 42, `p2`)
	u.OnEvent("rpki", "10.0.0.1", 42, `r1`)
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	c := calls
	mu.Unlock()
	if c != 1 {
		t.Fatalf("expected 1 call (only peer1 matched), got %d", c)
	}

	u.OnEvent("rpki", "10.0.0.2", 42, `r2`)
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if calls != 2 {
		t.Fatalf("expected 2 calls total, got %d", calls)
	}
}
