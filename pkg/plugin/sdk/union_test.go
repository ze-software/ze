package sdk

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return gotPrimary == `{"update":"data"}` && gotSecondary == `{"rpki":"valid"}`
	}, 2*time.Second, time.Millisecond)

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

	// Handler should NOT be called with only the primary.
	require.Never(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return called != 0
	}, 50*time.Millisecond, time.Millisecond)

	u.OnEvent("rpki", "10.0.0.1", 1, `secondary`)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return called == 1
	}, 2*time.Second, time.Millisecond)
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
	u.OnEvent("update", "10.0.0.1", 7, `pri`)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return gotPrimary == `pri` && gotSecondary == `sec`
	}, 2*time.Second, time.Millisecond)

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
	var handlerCalled bool

	u := NewUnion("update", "rpki", 100*time.Millisecond, func(primary, secondary string) {
		mu.Lock()
		gotPrimary = primary
		gotSecondary = secondary
		handlerCalled = true
		mu.Unlock()
	})
	u.sweepInterval = 100 * time.Millisecond // faster sweep for test
	defer u.Stop()

	u.OnEvent("update", "10.0.0.1", 99, `timeout-test`)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return handlerCalled
	}, 2*time.Second, time.Millisecond)

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

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return calls == 2
	}, 2*time.Second, time.Millisecond)
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

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return evictedPrimary == `first`
	}, 2*time.Second, time.Millisecond)
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

	// Handler should NOT be called for orphan secondary, even after sweep runs.
	require.Never(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return called != 0
	}, 500*time.Millisecond, 10*time.Millisecond)
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

	// Handler should NOT be called for secondary-only flush.
	require.Never(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return called != 0
	}, 50*time.Millisecond, time.Millisecond)
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

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return calls == 1
	}, 2*time.Second, time.Millisecond)

	u.OnEvent("rpki", "10.0.0.2", 42, `r2`)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return calls == 2
	}, 2*time.Second, time.Millisecond)
}
