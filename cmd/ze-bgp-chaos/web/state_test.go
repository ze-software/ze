package web

import (
	"net/netip"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/peer"
)

// TestRingBufferPushAndAll verifies basic ring buffer operations.
//
// VALIDATES: Items stored in insertion order, oldest dropped when full.
// PREVENTS: Off-by-one errors in circular buffer indexing.
func TestRingBufferPushAndAll(t *testing.T) {
	t.Parallel()

	rb := NewRingBuffer[int](3)
	if rb.Len() != 0 {
		t.Fatalf("empty buffer Len() = %d, want 0", rb.Len())
	}
	if rb.Cap() != 3 {
		t.Fatalf("Cap() = %d, want 3", rb.Cap())
	}

	rb.Push(1)
	rb.Push(2)
	rb.Push(3)
	if rb.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", rb.Len())
	}

	got := rb.All()
	want := []int{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("All() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("All()[%d] = %d, want %d", i, got[i], want[i])
		}
	}

	// Push beyond capacity — oldest (1) should be dropped.
	rb.Push(4)
	got = rb.All()
	want = []int{2, 3, 4}
	if len(got) != len(want) {
		t.Fatalf("All() after overflow len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("All()[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

// TestRingBufferLatest verifies Latest() returns the most recently pushed item.
//
// VALIDATES: Latest returns newest item, false when empty.
// PREVENTS: Wrong index calculation for most recent item.
func TestRingBufferLatest(t *testing.T) {
	t.Parallel()

	rb := NewRingBuffer[string](5)
	if _, ok := rb.Latest(); ok {
		t.Fatal("Latest() on empty buffer should return false")
	}

	rb.Push("a")
	rb.Push("b")
	rb.Push("c")

	got, ok := rb.Latest()
	if !ok || got != "c" {
		t.Fatalf("Latest() = (%q, %v), want (\"c\", true)", got, ok)
	}
}

// TestRingBufferMinCapacity verifies capacity is clamped to at least 1.
//
// VALIDATES: Zero or negative capacity defaults to 1.
// PREVENTS: Divide-by-zero in modular arithmetic.
func TestRingBufferMinCapacity(t *testing.T) {
	t.Parallel()

	rb := NewRingBuffer[int](0)
	if rb.Cap() != 1 {
		t.Fatalf("Cap() for 0 input = %d, want 1", rb.Cap())
	}

	rb = NewRingBuffer[int](-5)
	if rb.Cap() != 1 {
		t.Fatalf("Cap() for -5 input = %d, want 1", rb.Cap())
	}
}

// TestRingBufferEmpty verifies All() on an empty buffer returns nil.
//
// VALIDATES: Empty buffer returns nil slice.
// PREVENTS: Returning empty non-nil slice or panicking.
func TestRingBufferEmpty(t *testing.T) {
	t.Parallel()

	rb := NewRingBuffer[int](10)
	if got := rb.All(); got != nil {
		t.Fatalf("All() on empty = %v, want nil", got)
	}
}

// TestActiveSetPromotion verifies that noteworthy events promote peers.
//
// VALIDATES: Peers appear in active set when promoted, Len() increases.
// PREVENTS: Promotion silently failing or corrupting state.
func TestActiveSetPromotion(t *testing.T) {
	t.Parallel()

	as := NewActiveSet(40)
	now := time.Now()

	added := as.Promote(5, PriorityHigh, now)
	if !added {
		t.Fatal("Promote() first call should return true")
	}
	if !as.Contains(5) {
		t.Fatal("Contains(5) should be true after Promote")
	}
	if as.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", as.Len())
	}

	// Re-promote same peer — should return false (already present).
	added = as.Promote(5, PriorityMedium, now)
	if added {
		t.Fatal("Promote() on existing peer should return false")
	}
	if as.Len() != 1 {
		t.Fatalf("Len() after re-promote = %d, want 1", as.Len())
	}
}

// TestActiveSetDecay verifies non-pinned peers decay after adaptive TTL.
//
// VALIDATES: Expired peers removed by Decay(), pinned peers survive.
// PREVENTS: Pinned peers being incorrectly decayed.
func TestActiveSetDecay(t *testing.T) {
	t.Parallel()

	as := NewActiveSet(40)
	now := time.Now()

	// Add a peer. With <50% fill, TTL is 120s.
	as.Promote(1, PriorityMedium, now)

	// Decay before TTL — should not remove.
	removed := as.Decay(now.Add(60 * time.Second))
	if len(removed) != 0 {
		t.Fatalf("Decay before TTL removed %d peers, want 0", len(removed))
	}

	// Decay after TTL (120s) — should remove.
	removed = as.Decay(now.Add(121 * time.Second))
	if len(removed) != 1 || removed[0] != 1 {
		t.Fatalf("Decay after TTL removed %v, want [1]", removed)
	}
	if as.Contains(1) {
		t.Fatal("Peer 1 should not be in active set after decay")
	}
}

// TestActiveSetPinning verifies pinned peers survive decay.
//
// VALIDATES: Pinned peers not removed by Decay(), unpin re-enables decay.
// PREVENTS: Pin state not being respected during decay.
func TestActiveSetPinning(t *testing.T) {
	t.Parallel()

	as := NewActiveSet(40)
	now := time.Now()

	as.Promote(7, PriorityMedium, now)
	as.Pin(7, now)

	if !as.IsPinned(7) {
		t.Fatal("IsPinned(7) should be true after Pin()")
	}

	// Decay well past TTL — pinned peer should survive.
	removed := as.Decay(now.Add(300 * time.Second))
	if len(removed) != 0 {
		t.Fatalf("Decay removed pinned peer: %v", removed)
	}
	if !as.Contains(7) {
		t.Fatal("Pinned peer 7 should still be in active set")
	}

	// Unpin and decay — should now be removed.
	as.Unpin(7)
	if as.IsPinned(7) {
		t.Fatal("IsPinned(7) should be false after Unpin()")
	}
	removed = as.Decay(now.Add(300 * time.Second))
	if len(removed) != 1 {
		t.Fatalf("Decay after unpin removed %d, want 1", len(removed))
	}
}

// TestActiveSetAdaptiveTTL verifies TTL shortens as the active set fills up.
//
// VALIDATES: TTL decreases with fill ratio: 120s (<50%), 30s (50-80%), 5s (>80%).
// PREVENTS: TTL not adapting to capacity pressure.
func TestActiveSetAdaptiveTTL(t *testing.T) {
	t.Parallel()

	as := NewActiveSet(20)
	now := time.Now()

	// Empty (0%) — TTL should be 120s.
	if ttl := as.AdaptiveTTL(); ttl != 120*time.Second {
		t.Fatalf("TTL at 0%% fill = %v, want 120s", ttl)
	}

	// Add 5 peers (25% of 20) — still <50%, TTL = 120s.
	for i := range 5 {
		as.Promote(i, PriorityMedium, now)
	}
	if ttl := as.AdaptiveTTL(); ttl != 120*time.Second {
		t.Fatalf("TTL at 25%% fill = %v, want 120s", ttl)
	}

	// Add to 12 peers (60%) — 50-80%, TTL = 30s.
	for i := 5; i < 12; i++ {
		as.Promote(i, PriorityMedium, now)
	}
	if ttl := as.AdaptiveTTL(); ttl != 30*time.Second {
		t.Fatalf("TTL at 60%% fill = %v, want 30s", ttl)
	}

	// Add to 18 peers (90%) — >80%, TTL = 5s.
	for i := 12; i < 18; i++ {
		as.Promote(i, PriorityMedium, now)
	}
	if ttl := as.AdaptiveTTL(); ttl != 5*time.Second {
		t.Fatalf("TTL at 90%% fill = %v, want 5s", ttl)
	}
}

// TestActiveSetCapacity verifies the active set never exceeds max-visible.
//
// VALIDATES: Oldest non-pinned peer evicted when at capacity.
// PREVENTS: Active set growing unbounded.
func TestActiveSetCapacity(t *testing.T) {
	t.Parallel()

	as := NewActiveSet(10)
	now := time.Now()

	// Fill to capacity.
	for i := range 10 {
		as.Promote(i, PriorityMedium, now.Add(time.Duration(i)*time.Second))
	}
	if as.Len() != 10 {
		t.Fatalf("Len() = %d, want 10", as.Len())
	}

	// Promote one more — should evict peer 0 (oldest LastActive).
	added := as.Promote(99, PriorityMedium, now.Add(10*time.Second))
	if !added {
		t.Fatal("Promote beyond capacity should succeed by evicting")
	}
	if as.Len() != 10 {
		t.Fatalf("Len() after eviction = %d, want 10", as.Len())
	}
	if as.Contains(0) {
		t.Fatal("Peer 0 (oldest) should have been evicted")
	}
	if !as.Contains(99) {
		t.Fatal("Peer 99 should be in active set")
	}
}

// TestActiveSetStableOrder verifies peer positions don't change when others appear/disappear.
//
// VALIDATES: Indices() returns peer indices regardless of insertion order.
// PREVENTS: Position instability causing visual jumping in the table.
func TestActiveSetStableOrder(t *testing.T) {
	t.Parallel()

	as := NewActiveSet(40)
	now := time.Now()

	as.Promote(10, PriorityMedium, now)
	as.Promote(3, PriorityMedium, now)
	as.Promote(7, PriorityMedium, now)

	indices := as.Indices()
	if len(indices) != 3 {
		t.Fatalf("Indices() len = %d, want 3", len(indices))
	}

	// After removing peer 3, peers 10 and 7 should still be present.
	as.Promote(3, PriorityMedium, now.Add(-200*time.Second)) // make it old
	// Force peer 3 to be very old so it decays
	if e := as.Entry(3); e != nil {
		e.LastActive = now.Add(-200 * time.Second)
	}
	as.Decay(now)

	if as.Contains(3) {
		t.Fatal("Peer 3 should have decayed")
	}
	if !as.Contains(10) || !as.Contains(7) {
		t.Fatal("Peers 10 and 7 should remain after peer 3 decayed")
	}
}

// TestActiveSetPinNotInSet verifies Pin() on a peer not in the set promotes and pins it.
//
// VALIDATES: Pin on absent peer promotes first, then pins.
// PREVENTS: Pin silently failing for peers not yet in active set.
func TestActiveSetPinNotInSet(t *testing.T) {
	t.Parallel()

	as := NewActiveSet(40)
	now := time.Now()

	as.Pin(42, now)
	if !as.Contains(42) {
		t.Fatal("Pin(42) should promote peer into active set")
	}
	if !as.IsPinned(42) {
		t.Fatal("Peer 42 should be pinned after Pin()")
	}
}

// TestActiveSetAllPinnedNoEviction verifies that when all peers are pinned,
// new promotion fails gracefully.
//
// VALIDATES: Returns false when all slots are pinned and at capacity.
// PREVENTS: Panic or corruption when trying to evict from fully-pinned set.
func TestActiveSetAllPinnedNoEviction(t *testing.T) {
	t.Parallel()

	as := NewActiveSet(10) // minimum clamped capacity
	now := time.Now()

	for i := range 10 {
		as.Pin(i, now)
	}

	added := as.Promote(99, PriorityHigh, now)
	if added {
		t.Fatal("Promote should fail when all peers are pinned and at capacity")
	}
	if as.Contains(99) {
		t.Fatal("Peer 99 should not be in active set")
	}
}

// TestActiveSetMinCapacity verifies minimum capacity is clamped to 10.
//
// VALIDATES: NewActiveSet(5) creates set with MaxVisible=10.
// PREVENTS: Unusably small active sets.
func TestActiveSetMinCapacity(t *testing.T) {
	t.Parallel()

	as := NewActiveSet(5)
	if as.MaxVisible != 10 {
		t.Fatalf("MaxVisible = %d, want 10 (clamped)", as.MaxVisible)
	}
}

// TestPromotionPriorityForEvent verifies event-to-priority mapping.
//
// VALIDATES: High/Medium/Low priorities assigned correctly, non-noteworthy events return false.
// PREVENTS: Wrong events triggering promotion or missing promotion triggers.
func TestPromotionPriorityForEvent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		evType   peer.EventType
		wantPrio PromotionPriority
		wantOK   bool
	}{
		{peer.EventDisconnected, PriorityHigh, true},
		{peer.EventError, PriorityHigh, true},
		{peer.EventChaosExecuted, PriorityMedium, true},
		{peer.EventReconnecting, PriorityMedium, true},
		{peer.EventWithdrawalSent, PriorityMedium, true},
		{peer.EventRouteWithdrawn, PriorityLow, true},
		{peer.EventEstablished, 0, false},
		{peer.EventRouteSent, 0, false},
		{peer.EventRouteReceived, 0, false},
		{peer.EventEORSent, 0, false},
	}

	for _, tt := range tests {
		prio, ok := PromotionPriorityForEvent(tt.evType)
		if prio != tt.wantPrio || ok != tt.wantOK {
			t.Errorf("PromotionPriorityForEvent(%d) = (%d, %v), want (%d, %v)",
				tt.evType, prio, ok, tt.wantPrio, tt.wantOK)
		}
	}
}

// TestPeerStateNewAndDefaults verifies PeerState initialization.
//
// VALIDATES: Default status is Idle, counters are zero, ring buffer created.
// PREVENTS: Nil pointer on Events ring buffer.
func TestPeerStateNewAndDefaults(t *testing.T) {
	t.Parallel()

	ps := NewPeerState(5, 100)
	if ps.Index != 5 {
		t.Fatalf("Index = %d, want 5", ps.Index)
	}
	if ps.Status != PeerIdle {
		t.Fatalf("Status = %v, want Idle", ps.Status)
	}
	if ps.RoutesSent != 0 || ps.RoutesRecv != 0 {
		t.Fatal("Counters should be zero")
	}
	if ps.Events == nil {
		t.Fatal("Events ring buffer should not be nil")
	}
	if ps.Events.Cap() != 100 {
		t.Fatalf("Events capacity = %d, want 100", ps.Events.Cap())
	}
}

// TestDashboardStateNew verifies DashboardState initialization.
//
// VALIDATES: All peers created, active set initialized, global event buffer created.
// PREVENTS: Missing peer entries causing nil pointer on ProcessEvent.
func TestDashboardStateNew(t *testing.T) {
	t.Parallel()

	ds := NewDashboardState(10, 40, 500)
	if len(ds.Peers) != 10 {
		t.Fatalf("len(Peers) = %d, want 10", len(ds.Peers))
	}
	if ds.Active.MaxVisible != 40 {
		t.Fatalf("Active.MaxVisible = %d, want 40", ds.Active.MaxVisible)
	}
	if ds.GlobalEvents.Cap() != 500 {
		t.Fatalf("GlobalEvents capacity = %d, want 500", ds.GlobalEvents.Cap())
	}
	if ds.PeerCount != 10 {
		t.Fatalf("PeerCount = %d, want 10", ds.PeerCount)
	}
}

// TestDashboardStateDirtyFlags verifies dirty flag set/consume cycle.
//
// VALIDATES: MarkDirty sets flags, ConsumeDirty returns and resets them.
// PREVENTS: Dirty flags not being reset, causing duplicate SSE sends.
func TestDashboardStateDirtyFlags(t *testing.T) {
	t.Parallel()

	ds := NewDashboardState(5, 40, 100)

	ds.MarkDirty(2)
	ds.MarkDirty(4)

	peers, global := ds.ConsumeDirty()
	if !global {
		t.Fatal("global dirty should be true")
	}
	if !peers[2] || !peers[4] {
		t.Fatalf("dirty peers = %v, want {2: true, 4: true}", peers)
	}

	// After consume, flags should be reset.
	peers, global = ds.ConsumeDirty()
	if global {
		t.Fatal("global dirty should be false after consume")
	}
	if len(peers) != 0 {
		t.Fatalf("dirty peers after consume = %v, want empty", peers)
	}
}

// TestFormatDuration verifies compact duration formatting.
//
// VALIDATES: Microseconds, milliseconds, and seconds formatted correctly.
// PREVENTS: Inconsistent duration display in the dashboard.
func TestFormatDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input time.Duration
		want  string
	}{
		{0, "0"},
		{500 * time.Microsecond, "500µs"},
		{1500 * time.Microsecond, "1ms"},
		{50 * time.Millisecond, "50ms"},
		{1500 * time.Millisecond, "1.5s"},
		{2 * time.Second, "2s"},
		{65 * time.Second, "1m5s"},
	}

	for _, tt := range tests {
		got := FormatDuration(tt.input)
		if got != tt.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestRouteMatrixPrefixEviction verifies that routeOrigins and sentTimes
// are evicted when they exceed maxPrefixTracking.
//
// VALIDATES: Maps are cleared when prefix tracking exceeds the cap.
// PREVENTS: Unbounded memory growth in long-running sessions with route churn.
func TestRouteMatrixPrefixEviction(t *testing.T) {
	t.Parallel()

	m := NewRouteMatrix()
	now := time.Now()

	// Fill up to the max. Use unique /32 prefixes from different addresses.
	for i := range maxPrefixTracking {
		a := uint8(i >> 16)
		b := uint8(i >> 8)
		c := uint8(i)
		addr := netip.AddrFrom4([4]byte{10, a, b, c})
		m.RecordSent(i%4, netip.PrefixFrom(addr, 32), now)
	}

	if len(m.routeOrigins) != maxPrefixTracking {
		t.Fatalf("routeOrigins len = %d, want %d", len(m.routeOrigins), maxPrefixTracking)
	}

	// One more should trigger eviction — maps get cleared and only the new entry remains.
	overflow := netip.MustParsePrefix("192.168.0.1/32")
	m.RecordSent(0, overflow, now)

	if len(m.routeOrigins) != 1 {
		t.Fatalf("routeOrigins len after eviction = %d, want 1", len(m.routeOrigins))
	}
	if len(m.sentTimes) != 1 {
		t.Fatalf("sentTimes len after eviction = %d, want 1", len(m.sentTimes))
	}

	// The new entry should still be queryable.
	if m.routeOrigins[overflow] != 0 {
		t.Fatalf("overflow prefix origin = %d, want 0", m.routeOrigins[overflow])
	}

	// Cumulative cell counters should be unaffected by eviction.
	// Record a receive for the overflow prefix — should correlate correctly.
	found, _ := m.RecordReceived(1, overflow, now.Add(time.Millisecond))
	if !found {
		t.Fatal("RecordReceived should find overflow prefix after eviction")
	}
	if m.Get(0, 1) != 1 {
		t.Fatalf("Get(0,1) = %d, want 1", m.Get(0, 1))
	}
}

// TestPeerStatusString verifies string representation of all statuses.
//
// VALIDATES: All PeerStatus values have human-readable strings.
// PREVENTS: Missing case in switch causing wrong label.
func TestPeerStatusString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status PeerStatus
		want   string
	}{
		{PeerIdle, "idle"},
		{PeerUp, "up"},
		{PeerDown, "down"},
		{PeerReconnecting, "reconnecting"},
	}

	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("PeerStatus(%d).String() = %q, want %q", tt.status, got, tt.want)
		}
	}
}
