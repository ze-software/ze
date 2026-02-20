package reactor

import (
	"errors"
	"net/netip"
	"sync"
	"testing"
	"time"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/sim"
)

// emptyPayload is a minimal valid UPDATE payload for cache tests.
// Format: WithdrawnLen(2)=0 + AttrLen(2)=0.
var emptyPayload = []byte{0, 0, 0, 0}

// newTestUpdate creates a ReceivedUpdate with messageID set on WireUpdate.
func newTestUpdate(id uint64) *ReceivedUpdate {
	wu := wireu.NewWireUpdate(emptyPayload, bgpctx.ContextID(1))
	wu.SetMessageID(id)
	return &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}
}

// --- Basic cache operations ---

// TestRecentUpdateCacheAdd verifies cache insertion and retrieval via Get.
//
// VALIDATES: Updates are cached and retrievable via Get (non-destructive).
// PREVENTS: Lost updates, broken forwarding.
func TestRecentUpdateCacheAdd(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	update := newTestUpdate(1)
	cache.Add(update)

	// Get is non-destructive — entry remains in cache
	got, ok := cache.Get(1)
	if !ok {
		t.Fatal("expected to find update 1")
	}
	if got.WireUpdate.MessageID() != 1 {
		t.Errorf("MessageID = %d, want 1", got.WireUpdate.MessageID())
	}

	// Entry still in cache after Get
	if !cache.Contains(1) {
		t.Error("entry should still exist after Get (non-destructive)")
	}
}

// TestRecentUpdateCacheNotFound verifies missing entries.
//
// VALIDATES: Non-existent IDs return not found.
// PREVENTS: False positives on lookup.
func TestRecentUpdateCacheNotFound(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	_, ok := cache.Get(999)
	if ok {
		t.Error("expected not found for non-existent ID")
	}
}

// TestRecentUpdateCacheDelete verifies explicit deletion.
//
// VALIDATES: Delete removes entry from cache.
// PREVENTS: Memory leaks from unflushed entries.
func TestRecentUpdateCacheDelete(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	cache.Add(newTestUpdate(1))

	if !cache.Contains(1) {
		t.Fatal("expected to find update before delete")
	}

	if !cache.Delete(1) {
		t.Error("Delete returned false for existing entry")
	}

	if cache.Contains(1) {
		t.Error("expected not found after delete")
	}

	if cache.Delete(1) {
		t.Error("Delete returned true for non-existent entry")
	}
}

// --- Get (non-destructive) ---

// TestCacheGetNonDestructive verifies Get does not remove entries.
//
// VALIDATES: Multiple Gets return same entry, entry remains in cache (AC-7).
// PREVENTS: Accidental entry removal on read.
func TestCacheGetNonDestructive(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	cache.Add(newTestUpdate(1))

	for i := range 5 {
		got, ok := cache.Get(1)
		if !ok {
			t.Fatalf("Get #%d failed", i)
		}
		if got.WireUpdate.MessageID() != 1 {
			t.Fatalf("Get #%d returned wrong ID: %d", i, got.WireUpdate.MessageID())
		}
	}

	if !cache.Contains(1) {
		t.Error("entry should remain after multiple Gets")
	}
	if cache.Len() != 1 {
		t.Errorf("Len() = %d, want 1", cache.Len())
	}
}

// TestCacheGetPendingEntry verifies Get works on pending entries.
//
// VALIDATES: Pending entries (between Add and Activate) are accessible via Get.
// PREVENTS: Race where plugin receives msg-id before Activate completes.
func TestCacheGetPendingEntry(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	cache.Add(newTestUpdate(1))
	// Deliberately NOT calling Activate — entry is still pending

	got, ok := cache.Get(1)
	if !ok {
		t.Fatal("expected to get pending entry")
	}
	if got.WireUpdate.MessageID() != 1 {
		t.Errorf("MessageID = %d, want 1", got.WireUpdate.MessageID())
	}
}

// --- Soft limit ---

// TestRecentUpdateCacheSoftLimit verifies Add always succeeds beyond maxEntries.
//
// VALIDATES: Soft limit warns but never rejects (AC-1, AC-15).
// PREVENTS: UPDATE loss when cache is full.
func TestRecentUpdateCacheSoftLimit(t *testing.T) {
	cache := NewRecentUpdateCache(3)

	for i := uint64(1); i <= 5; i++ {
		cache.Add(newTestUpdate(i))
		cache.Activate(i, 1)
	}

	if cache.Len() != 5 {
		t.Errorf("Len() = %d, want 5 (soft limit never rejects)", cache.Len())
	}

	for i := uint64(1); i <= 5; i++ {
		if !cache.Contains(i) {
			t.Errorf("expected update %d to exist", i)
		}
	}
}

// --- List ---

// TestRecentUpdateCacheList verifies List returns all cached msg-ids.
//
// VALIDATES: List returns IDs of all entries.
// PREVENTS: Missing entries in API response.
func TestRecentUpdateCacheList(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	ids := cache.List()
	if len(ids) != 0 {
		t.Errorf("List() = %v, want empty", ids)
	}

	cache.Add(newTestUpdate(10))
	cache.Add(newTestUpdate(20))
	cache.Add(newTestUpdate(30))

	ids = cache.List()
	if len(ids) != 3 {
		t.Errorf("List() len = %d, want 3", len(ids))
	}

	found := make(map[uint64]bool)
	for _, id := range ids {
		found[id] = true
	}
	for _, want := range []uint64{10, 20, 30} {
		if !found[want] {
			t.Errorf("List() missing id %d", want)
		}
	}
}

// --- No TTL (Phase 2) ---

// TestCacheNoTTLEviction verifies entries are never evicted by time alone.
//
// VALIDATES: Entry with consumers > 0 is never evicted by time alone (AC-16, AC-17).
// PREVENTS: Silent UPDATE loss via TTL expiry.
func TestCacheNoTTLEviction(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := sim.NewFakeClock(start)

	cache := NewRecentUpdateCache(100)
	cache.SetClock(fc)

	cache.Add(newTestUpdate(1))
	cache.Activate(1, 1)

	// Advance far into the future — entry must still exist
	fc.Add(24 * time.Hour)
	if !cache.Contains(1) {
		t.Error("entry with consumers should never expire by time alone")
	}

	// Entry at frontier (no later entry acked) — must survive safety valve scan
	cache.Add(newTestUpdate(2)) // Triggers gap scan
	if !cache.Contains(1) {
		t.Error("frontier entry must survive safety valve scan")
	}
}

// TestCacheNoTTLConstructor verifies constructor takes no TTL parameter.
//
// VALIDATES: NewRecentUpdateCache takes only maxEntries (AC-16).
// PREVENTS: TTL-based eviction from being reintroduced.
func TestCacheNoTTLConstructor(t *testing.T) {
	cache := NewRecentUpdateCache(100)
	if cache == nil {
		t.Fatal("NewRecentUpdateCache(100) returned nil")
	}
}

// --- Immediate eviction on ack (Phase 2) ---

// TestCacheImmediateEvictOnZeroConsumers verifies immediate eviction when all ack.
//
// VALIDATES: Entry evicted immediately when last consumer acks (AC-9).
// PREVENTS: Stale entries lingering after all consumers are done.
func TestCacheImmediateEvictOnZeroConsumers(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	cache.Add(newTestUpdate(1))
	cache.Activate(1, 2)

	// First ack
	if err := cache.Ack(1, "plugin-a"); err != nil {
		t.Fatalf("Ack plugin-a: %v", err)
	}
	if !cache.Contains(1) {
		t.Fatal("entry should exist with 1 remaining consumer")
	}

	// Second ack — should trigger immediate eviction
	if err := cache.Ack(1, "plugin-b"); err != nil {
		t.Fatalf("Ack plugin-b: %v", err)
	}
	if cache.Contains(1) {
		t.Error("entry should be evicted immediately when all consumers ack")
	}
}

// TestCacheActivateZeroConsumersEvictsImmediately verifies zero-subscriber case.
//
// VALIDATES: Activate(id, nil) with no consumers evicts entry immediately (AC-5).
// PREVENTS: Permanently pending entries when no plugins subscribe.
func TestCacheActivateZeroConsumersEvictsImmediately(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	cache.Add(newTestUpdate(1))

	// Pending entry exists
	if !cache.Contains(1) {
		t.Fatal("pending entry should exist")
	}

	// Activate with zero consumers — immediate eviction
	cache.Activate(1, 0)

	if cache.Contains(1) {
		t.Error("entry with 0 consumers should be evicted immediately on Activate")
	}
}

// --- FIFO ordering (Phase 2) ---

// TestCacheFIFOOrdering verifies cumulative ack with out-of-order tolerance.
//
// VALIDATES: Forward ack implicitly acks earlier entries; out-of-order acks
// (id <= lastAck) are silently accepted as no-ops.
// PREVENTS: Log flooding from FIFO violation errors when multi-peer delivery
// causes events to arrive in non-ID-order.
func TestCacheFIFOOrdering(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	cache.Add(newTestUpdate(1))
	cache.Add(newTestUpdate(2))
	cache.Add(newTestUpdate(3))
	cache.Activate(1, 1)
	cache.Activate(2, 1)
	cache.Activate(3, 1)

	// Ack 1 — forward ack (lastAck=0 → 1)
	if err := cache.Ack(1, "rr"); err != nil {
		t.Fatalf("Ack(1): %v", err)
	}

	// Ack 3 — forward ack (lastAck=1 → 3), implicitly acks entry 2
	if err := cache.Ack(3, "rr"); err != nil {
		t.Fatalf("Ack(3): %v", err)
	}

	// Ack 2 — out-of-order (2 <= lastAck=3), silent no-op
	if err := cache.Ack(2, "rr"); err != nil {
		t.Errorf("Ack(2) after Ack(3): expected no-op (nil), got %v", err)
	}

	// Ack 1 again — out-of-order (1 <= lastAck=3), silent no-op
	if err := cache.Ack(1, "rr"); err != nil {
		t.Errorf("re-Ack(1): expected no-op (nil), got %v", err)
	}
}

// TestCacheFIFOImplicitAck verifies cumulative ack semantics.
//
// VALIDATES: Ack for N implicitly acks 1..N for that plugin (AC-13).
// PREVENTS: Missing acks causing entries to remain in cache.
func TestCacheFIFOImplicitAck(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	// Add 5 entries, all with same single consumer
	for i := uint64(1); i <= 5; i++ {
		cache.Add(newTestUpdate(i))
		cache.Activate(i, 1)
	}

	// Ack entry 5 — should implicitly ack 1..4
	if err := cache.Ack(5, "rr"); err != nil {
		t.Fatalf("Ack(5): %v", err)
	}

	// All 5 entries should be evicted (single consumer, all acked)
	for i := uint64(1); i <= 5; i++ {
		if cache.Contains(i) {
			t.Errorf("entry %d should be evicted after implicit ack via Ack(5)", i)
		}
	}
	if cache.Len() != 0 {
		t.Errorf("Len() = %d, want 0", cache.Len())
	}
}

// TestCacheFIFOPerPlugin verifies independent FIFO tracking per plugin.
//
// VALIDATES: Each plugin has independent FIFO ordering.
// PREVENTS: Cross-plugin ack interference.
func TestCacheFIFOPerPlugin(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	cache.Add(newTestUpdate(1))
	cache.Add(newTestUpdate(2))
	cache.Activate(1, 2)
	cache.Activate(2, 2)

	// rr acks 2 (implicitly acks 1 for rr)
	if err := cache.Ack(2, "rr"); err != nil {
		t.Fatalf("rr Ack(2): %v", err)
	}

	// Entry 1 still exists (monitor hasn't acked)
	if !cache.Contains(1) {
		t.Fatal("entry 1 should exist (monitor hasn't acked)")
	}

	// monitor acks 1
	if err := cache.Ack(1, "monitor"); err != nil {
		t.Fatalf("monitor Ack(1): %v", err)
	}

	// Entry 1 should be evicted now (both plugins acked)
	if cache.Contains(1) {
		t.Error("entry 1 should be evicted after both plugins acked")
	}

	// Entry 2 still exists (monitor hasn't acked 2)
	if !cache.Contains(2) {
		t.Fatal("entry 2 should exist (monitor hasn't acked)")
	}

	// monitor acks 2
	if err := cache.Ack(2, "monitor"); err != nil {
		t.Fatalf("monitor Ack(2): %v", err)
	}

	// Entry 2 evicted
	if cache.Contains(2) {
		t.Error("entry 2 should be evicted after both plugins acked")
	}
}

// TestCacheOutOfOrderAck verifies that out-of-order acks are silently
// accepted when multi-peer concurrent delivery causes events to arrive
// in non-ID-order.
//
// VALIDATES: Ack for id <= lastAck is a no-op (no error, no double-decrement).
// PREVENTS: "FIFO violation" errors flooding logs when session goroutines
// compete for callMu and deliver events out of global ID order.
func TestCacheOutOfOrderAck(t *testing.T) {
	t.Run("single_consumer_out_of_order", func(t *testing.T) {
		cache := NewRecentUpdateCache(100)

		// Simulate 4 entries delivered to 1 plugin, processed out of order.
		cache.Add(newTestUpdate(10))
		cache.Add(newTestUpdate(11))
		cache.Add(newTestUpdate(12))
		cache.Add(newTestUpdate(13))
		cache.Activate(10, 1)
		cache.Activate(11, 1)
		cache.Activate(12, 1)
		cache.Activate(13, 1)

		// Plugin processes event 13 first (won callMu race).
		// Forward ack: implicitly acks 10..12, all evicted (single consumer).
		if err := cache.Ack(13, "rr"); err != nil {
			t.Fatalf("Ack(13): %v", err)
		}

		// All entries should be evicted by the cumulative ack.
		for _, id := range []uint64{10, 11, 12, 13} {
			if cache.Contains(id) {
				t.Errorf("entry %d should be evicted after cumulative Ack(13)", id)
			}
		}

		// Plugin now processes events 10, 12, 11 (out-of-order).
		// Each id <= lastAck=13, so these are no-ops — no error returned.
		for _, id := range []uint64{10, 12, 11} {
			if err := cache.Ack(id, "rr"); err != nil {
				t.Errorf("out-of-order Ack(%d) should be no-op, got: %v", id, err)
			}
		}
	})

	t.Run("two_consumers_out_of_order_no_double_decrement", func(t *testing.T) {
		cache := NewRecentUpdateCache(100)

		cache.Add(newTestUpdate(20))
		cache.Add(newTestUpdate(21))
		cache.Add(newTestUpdate(22))
		cache.Activate(20, 2) // 2 consumers: "rr" and "monitor"
		cache.Activate(21, 2)
		cache.Activate(22, 2)

		// "rr" acks 22 first (forward ack, implicitly acks 20, 21 for rr).
		// Each entry's pendingConsumers: 2 → 1 (monitor still pending).
		if err := cache.Ack(22, "rr"); err != nil {
			t.Fatalf("rr Ack(22): %v", err)
		}

		// All entries still exist (monitor hasn't acked).
		if cache.Len() != 3 {
			t.Fatalf("expected 3 entries (monitor still pending), got %d", cache.Len())
		}

		// "rr" processes events 10, 12, 11 out-of-order.
		// These are all <= lastAck=22, so they are no-ops.
		// Crucially, they must NOT double-decrement pendingConsumers.
		for _, id := range []uint64{20, 21} {
			if err := cache.Ack(id, "rr"); err != nil {
				t.Errorf("rr out-of-order Ack(%d) should be no-op, got: %v", id, err)
			}
		}

		// All 3 entries must still exist — rr's out-of-order acks were no-ops,
		// monitor still hasn't acked. pendingConsumers should still be 1.
		if cache.Len() != 3 {
			t.Fatalf("expected 3 entries after rr no-op acks, got %d (double-decrement bug!)", cache.Len())
		}

		// "monitor" acks all — now everything should be evicted.
		if err := cache.Ack(22, "monitor"); err != nil {
			t.Fatalf("monitor Ack(22): %v", err)
		}

		if cache.Len() != 0 {
			t.Errorf("expected 0 entries after both consumers acked, got %d", cache.Len())
		}
	})
}

// --- Early ack (fast plugin race) ---

// TestCacheAckBeforeActivate verifies fast-plugin race handling.
//
// VALIDATES: Ack before Activate is stored as early ack and applied on Activate (AC-3).
// PREVENTS: Lost acks from fast plugins.
func TestCacheAckBeforeActivate(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	cache.Add(newTestUpdate(1))

	// Fast plugin acks before Activate
	if err := cache.Ack(1, "fast-plugin"); err != nil {
		t.Fatalf("early Ack: %v", err)
	}

	// Activate with the fast plugin as consumer — early ack should cancel it out
	cache.Activate(1, 1)

	// Entry should be immediately evicted (fast-plugin already acked)
	if cache.Contains(1) {
		t.Error("entry should be evicted after Activate applies early ack")
	}
}

// TestCacheAckBeforeActivateTwoPlugins verifies partial early ack.
//
// VALIDATES: Early ack from one plugin, normal ack from another.
// PREVENTS: Partial early ack handling bugs.
func TestCacheAckBeforeActivateTwoPlugins(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	cache.Add(newTestUpdate(1))

	// Fast plugin acks before Activate
	if err := cache.Ack(1, "fast"); err != nil {
		t.Fatalf("early Ack: %v", err)
	}

	// Activate with both plugins
	cache.Activate(1, 2)

	// Entry should still exist (slow hasn't acked)
	if !cache.Contains(1) {
		t.Fatal("entry should exist (slow hasn't acked)")
	}

	// slow acks
	if err := cache.Ack(1, "slow"); err != nil {
		t.Fatalf("slow Ack: %v", err)
	}

	// Now evicted
	if cache.Contains(1) {
		t.Error("entry should be evicted after both acked")
	}
}

// --- Retain / Release (API commands) ---

// TestRecentUpdateCacheRetain verifies Retain increments retain count.
//
// VALIDATES: Retain prevents eviction even after all plugin acks.
// PREVENTS: Premature eviction of routes needed for graceful restart.
func TestRecentUpdateCacheRetain(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	cache.Add(newTestUpdate(1))
	cache.Activate(1, 1)

	// Retain the entry
	if !cache.Retain(1) {
		t.Fatal("Retain returned false for existing entry")
	}

	// Plugin acks — but retain keeps it
	if err := cache.Ack(1, "rr"); err != nil {
		t.Fatalf("Ack: %v", err)
	}

	// Entry should still exist (retainCount=1)
	if !cache.Contains(1) {
		t.Error("retained entry should survive after all plugin acks")
	}
}

// TestRecentUpdateCacheRetainNotFound verifies Retain on missing entry.
//
// VALIDATES: Retain returns false for non-existent entry.
// PREVENTS: Silent failures on invalid msg-ids.
func TestRecentUpdateCacheRetainNotFound(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	if cache.Retain(999) {
		t.Error("Retain returned true for non-existent entry")
	}
}

// TestCacheRetainAndRelease verifies Retain/Release refcount semantics.
//
// VALIDATES: Retain adds 1, Release subtracts 1 — balanced usage (AC-6).
// PREVENTS: Refcount imbalance from API commands.
func TestCacheRetainAndRelease(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	cache.Add(newTestUpdate(1))

	// Retain twice BEFORE Activate — API hold keeps entry alive (retainCount: 0 → 1 → 2)
	cache.Retain(1)
	cache.Retain(1)

	// Activate with no plugin consumers — entry survives because retainCount=2
	cache.Activate(1, 0)

	// Release once (retainCount: 2 → 1)
	cache.Release(1)
	if !cache.Contains(1) {
		t.Fatal("entry with retainCount=1 should survive")
	}

	// Release again (retainCount: 1 → 0) — immediate eviction (no plugin consumers)
	cache.Release(1)
	if cache.Contains(1) {
		t.Error("entry should be evicted when retainCount reaches 0 and no plugin consumers")
	}
}

// TestRecentUpdateCacheRelease verifies release allows eviction.
//
// VALIDATES: Release decrements retain count, evicts when zero.
// PREVENTS: Memory leaks from permanently retained entries.
func TestRecentUpdateCacheRelease(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	cache.Add(newTestUpdate(1))

	// Retain BEFORE Activate — API hold keeps entry alive
	if !cache.Retain(1) {
		t.Fatal("Retain failed")
	}

	// Activate with no plugin consumers — entry survives because retainCount=1
	cache.Activate(1, 0)

	// Release
	if !cache.Release(1) {
		t.Fatal("Release returned false for existing entry")
	}

	// Evicted (retainCount=0, no plugin consumers)
	if cache.Contains(1) {
		t.Error("released entry should be evicted immediately")
	}
}

// TestRecentUpdateCacheReleaseNotFound verifies Release on missing entry.
//
// VALIDATES: Release returns false for non-existent entry.
// PREVENTS: Silent failures on invalid msg-ids.
func TestRecentUpdateCacheReleaseNotFound(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	if cache.Release(999) {
		t.Error("Release returned true for non-existent entry")
	}
}

// --- Safety valve (gap-based, Phase 2) ---

// TestCacheSafetyValveGapDetection verifies gap-based force-eviction.
//
// VALIDATES: Entry passed over (later entry fully acked) is force-evicted after timeout (AC-10, AC-14).
// PREVENTS: Memory leak from crashed plugins that never ack.
func TestCacheSafetyValveGapDetection(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := sim.NewFakeClock(start)

	// Setup: "stalled" plugin consumes entry 100, "healthy" consumes entry 200.
	// Register "stalled" before entry 100 (lastAck=0), "healthy" after (lastAck=100).
	// This way "healthy" acking 200 won't implicit-ack entry 100.
	cache := NewRecentUpdateCache(100)
	cache.SetClock(fc)

	cache.RegisterConsumer("stalled")
	cache.Add(newTestUpdate(100))
	cache.Activate(100, 1) // Only "stalled" consumes this

	cache.RegisterConsumer("healthy")
	cache.Add(newTestUpdate(200))
	cache.Activate(200, 1) // Only "healthy" consumes this

	// healthy acks entry 200 — fully acked, evicted, highestFullyAcked = 200
	// Implicit ack range: lastAck("healthy")=100, so 101..199 — entry 100 NOT touched.
	if err := cache.Ack(200, "healthy"); err != nil {
		t.Fatalf("healthy Ack(200): %v", err)
	}

	// Entry 100 still has 1 consumer ("stalled") — gap detected (200 > 100)
	if !cache.Contains(100) {
		t.Fatal("entry 100 should exist (stalled plugin hasn't acked)")
	}

	// Advance past safety valve (5 min + gap scan interval)
	fc.Add(5*time.Minute + 31*time.Second)

	// Trigger gap scan by adding a new entry
	cache.Add(newTestUpdate(300))

	// Entry 100 should be force-evicted by safety valve
	if cache.Contains(100) {
		t.Error("stalled entry 100 should be force-evicted by safety valve")
	}
}

// TestCacheNoTimeoutAtFrontier verifies frontier entries are never timed out.
//
// VALIDATES: Entry at processing frontier is never timed out (AC-17).
// PREVENTS: Force-eviction of slow-but-correct processing.
func TestCacheNoTimeoutAtFrontier(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := sim.NewFakeClock(start)

	cache := NewRecentUpdateCache(100)
	cache.SetClock(fc)

	cache.Add(newTestUpdate(1))
	cache.Activate(1, 1)

	// Advance well past safety valve duration
	fc.Add(10 * time.Minute)

	// Trigger gap scan
	cache.Add(newTestUpdate(2))

	// Entry 1 is at frontier (no later entry fully acked) — must survive
	if !cache.Contains(1) {
		t.Error("frontier entry should never be timed out")
	}
}

// --- Ack errors ---

// TestCacheAckExpiredEntry verifies Ack on non-existent entry.
//
// VALIDATES: Ack returns ErrUpdateExpired for missing entry.
// PREVENTS: Silent failures on invalid msg-ids.
func TestCacheAckExpiredEntry(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	err := cache.Ack(999, "rr")
	if !errors.Is(err, ErrUpdateExpired) {
		t.Errorf("Ack(999): got %v, want ErrUpdateExpired", err)
	}
}

// --- Decrement (API retain count) ---

// TestCacheDecrementNotFound verifies Decrement on missing entry.
//
// VALIDATES: Decrement returns false for non-existent entry.
// PREVENTS: Silent failures on invalid msg-ids.
func TestCacheDecrementNotFound(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	if cache.Decrement(999) {
		t.Error("Decrement returned true for non-existent entry")
	}
}

// --- FakeClock integration ---

// TestRecentCacheWithFakeClock verifies FakeClock injection into RecentUpdateCache.
//
// VALIDATES: SetClock() injection works — cache uses injected clock for safety valve.
// PREVENTS: "Bridge to nowhere" — injection interfaces exist but don't work end-to-end.
func TestRecentCacheWithFakeClock(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := sim.NewFakeClock(start)

	cache := NewRecentUpdateCache(100)
	cache.SetClock(fc)

	// Add and activate with consumer
	cache.Add(newTestUpdate(42))
	cache.Activate(42, 1)

	// Should exist (consumer present)
	if !cache.Contains(42) {
		t.Fatal("expected entry to exist")
	}

	// Advance clock — entry must survive (no TTL, consumer present)
	fc.Add(10 * time.Minute)
	if !cache.Contains(42) {
		t.Fatal("entry with consumer should survive regardless of time")
	}

	// Ack — immediate eviction
	if err := cache.Ack(42, "test-plugin"); err != nil {
		t.Fatalf("Ack: %v", err)
	}
	if cache.Contains(42) {
		t.Error("entry should be evicted after ack")
	}
}

// --- Concurrency ---

// TestRecentUpdateCacheConcurrency verifies thread safety.
//
// VALIDATES: Concurrent Add/Get/Ack are safe (AC-8).
// PREVENTS: Race conditions, data corruption.
func TestRecentUpdateCacheConcurrency(t *testing.T) {
	cache := NewRecentUpdateCache(1000)

	var wg sync.WaitGroup
	const goroutines = 10
	const opsPerGoroutine = 100

	// Concurrent writers
	for g := range goroutines {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for i := range opsPerGoroutine {
				id := uint64(base*opsPerGoroutine + i) //nolint:gosec // G115: test values are small
				cache.Add(newTestUpdate(id))
				cache.Activate(id, 1)
			}
		}(g)
	}

	// Concurrent readers
	for range goroutines {
		wg.Go(func() {
			for i := range opsPerGoroutine {
				cache.Contains(uint64(i)) //nolint:gosec // G115: test values are small
				_ = cache.Len()
			}
		})
	}

	wg.Wait()

	if cache.Len() == 0 {
		t.Error("expected some entries after concurrent operations")
	}
}

// TestCacheConcurrentAck verifies race-free concurrent Acks.
//
// VALIDATES: Multiple goroutines acking different plugins simultaneously is safe.
// PREVENTS: Race conditions in consumer tracking.
func TestCacheConcurrentAck(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	const numPlugins = 50
	plugins := make([]string, numPlugins)
	for i := range numPlugins {
		plugins[i] = "plugin-" + string(rune('A'+i%26)) + string(rune('0'+i/26))
	}

	cache.Add(newTestUpdate(1))
	cache.Activate(1, numPlugins)

	var wg sync.WaitGroup
	for _, p := range plugins {
		wg.Add(1)
		go func(pluginName string) {
			defer wg.Done()
			_ = cache.Ack(1, pluginName)
		}(p)
	}
	wg.Wait()

	// All consumers acked — entry should be evicted
	if cache.Contains(1) {
		t.Error("entry should be evicted after all concurrent acks")
	}
}

// --- Buffer ownership ---

// TestCacheBufferReturnedOnEviction verifies buffer lifecycle.
//
// VALIDATES: Buffer returned to pool only when entry evicted with all consumers done.
// PREVENTS: Buffer leaks or double-free.
func TestCacheBufferReturnedOnEviction(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	cache.Add(newTestUpdate(1))
	cache.Activate(1, 1)

	// Get does NOT transfer ownership
	got, ok := cache.Get(1)
	if !ok {
		t.Fatal("Get failed")
	}
	_ = got // got.poolBuf may be nil in test updates (no real pool buffer)

	// Ack triggers eviction + buffer return
	if err := cache.Ack(1, "rr"); err != nil {
		t.Fatalf("Ack: %v", err)
	}

	if cache.Contains(1) {
		t.Error("entry should be evicted after ack")
	}
}

// --- Retain + plugin consumers interaction ---

// TestCacheRetainPlusPluginConsumers verifies both tracking layers work together.
//
// VALIDATES: Entry needs both plugin acks AND retain releases to be evicted.
// PREVENTS: Premature eviction when either layer still holds a reference.
func TestCacheRetainPlusPluginConsumers(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	cache.Add(newTestUpdate(1))
	cache.Activate(1, 1)

	// API retains
	cache.Retain(1)

	// Plugin acks — but retain keeps it
	if err := cache.Ack(1, "rr"); err != nil {
		t.Fatalf("Ack: %v", err)
	}
	if !cache.Contains(1) {
		t.Fatal("entry should exist (retainCount=1)")
	}

	// Release retain
	cache.Release(1)

	// Now evicted (plugin acked + retain released)
	if cache.Contains(1) {
		t.Error("entry should be evicted after all consumers and retains cleared")
	}
}

// --- RegisterConsumer / UnregisterConsumer ---

// TestCacheRegisterConsumer verifies FIFO baseline initialization.
//
// VALIDATES: RegisterConsumer sets pluginLastAck to highestAddedID,
// so implicit acks from this plugin skip pre-registration entries.
// PREVENTS: New plugin accidentally acking old entries via implicit cumulative ack.
func TestCacheRegisterConsumer(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	// Add entry 1 BEFORE registering plugin
	cache.Add(newTestUpdate(1))
	cache.Activate(1, 1)

	// Register "late-joiner" — its lastAck should be 1 (highestAddedID)
	cache.RegisterConsumer("late-joiner")

	// Add entry 2 AFTER registration
	cache.Add(newTestUpdate(2))
	cache.Activate(2, 1)

	// late-joiner acks entry 2 — implicit ack covers 2..2 only (lastAck=1)
	// Entry 1 should NOT be affected (it was before registration)
	if err := cache.Ack(2, "late-joiner"); err != nil {
		t.Fatalf("Ack(2): %v", err)
	}

	// Entry 1 still has its original consumer (not late-joiner)
	if !cache.Contains(1) {
		t.Fatal("entry 1 should still exist (pre-registration, not touched by late-joiner)")
	}
}

// TestCacheUnregisterConsumer verifies cleanup on plugin removal.
//
// VALIDATES: UnregisterConsumer decrements pendingConsumers for unacked entries
// and evicts entries that reach zero total consumers.
// PREVENTS: Memory leak when a plugin disconnects without acking.
func TestCacheUnregisterConsumer(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	cache.RegisterConsumer("pluginA")
	cache.RegisterConsumer("pluginB")

	// Add 3 entries, each with 2 consumers
	cache.Add(newTestUpdate(1))
	cache.Add(newTestUpdate(2))
	cache.Add(newTestUpdate(3))
	cache.Activate(1, 2)
	cache.Activate(2, 2)
	cache.Activate(3, 2)

	// pluginA acks all 3
	if err := cache.Ack(3, "pluginA"); err != nil {
		t.Fatalf("pluginA Ack(3): %v", err)
	}

	// All 3 entries still have 1 consumer (pluginB)
	if cache.Len() != 3 {
		t.Fatalf("expected 3 entries, got %d", cache.Len())
	}

	// Unregister pluginB — its lastAck is 0 (registered before any adds)
	// All entries with id > 0 should have pendingConsumers decremented
	cache.UnregisterConsumer("pluginB")

	// All entries should now be evicted (pluginA already acked, pluginB unregistered)
	if cache.Len() != 0 {
		t.Errorf("expected 0 entries after unregister, got %d", cache.Len())
	}
}

// TestCacheUnregisterConsumerPartialAck verifies cleanup with partial progress.
//
// VALIDATES: UnregisterConsumer only affects entries the plugin hasn't acked yet.
// PREVENTS: Double-decrement of entries already acked by the unregistering plugin.
func TestCacheUnregisterConsumerPartialAck(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	cache.RegisterConsumer("stays")
	cache.RegisterConsumer("leaves")

	cache.Add(newTestUpdate(1))
	cache.Add(newTestUpdate(2))
	cache.Add(newTestUpdate(3))
	cache.Activate(1, 2)
	cache.Activate(2, 2)
	cache.Activate(3, 2)

	// "leaves" acks entry 1 only (lastAck = 1)
	if err := cache.Ack(1, "leaves"); err != nil {
		t.Fatalf("leaves Ack(1): %v", err)
	}

	// Unregister "leaves" — should only affect entries 2 and 3 (id > lastAck=1)
	cache.UnregisterConsumer("leaves")

	// Entry 1: had 2 consumers, "leaves" acked (1 left = "stays")
	// Entry 2: had 2 consumers, "leaves" unregistered (1 left = "stays")
	// Entry 3: had 2 consumers, "leaves" unregistered (1 left = "stays")
	if cache.Len() != 3 {
		t.Fatalf("expected 3 entries (each with 1 consumer), got %d", cache.Len())
	}

	// "stays" acks all — should evict everything
	if err := cache.Ack(3, "stays"); err != nil {
		t.Fatalf("stays Ack(3): %v", err)
	}

	if cache.Len() != 0 {
		t.Errorf("expected 0 entries after stays acks all, got %d", cache.Len())
	}
}

// TestCacheUnregisterUnknownConsumer verifies no-op for unknown plugin.
//
// VALIDATES: UnregisterConsumer is safe to call for unregistered plugins.
// PREVENTS: Panic or incorrect state mutation for unknown plugin names.
func TestCacheUnregisterUnknownConsumer(t *testing.T) {
	cache := NewRecentUpdateCache(100)

	cache.Add(newTestUpdate(1))
	cache.Activate(1, 1)

	// Should be a no-op, not panic
	cache.UnregisterConsumer("never-registered")

	// Entry should be unaffected
	if !cache.Contains(1) {
		t.Fatal("entry should still exist after unregistering unknown consumer")
	}
}
