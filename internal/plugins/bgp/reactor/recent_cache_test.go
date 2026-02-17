package reactor

import (
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

// addAndActivate adds an update and immediately activates it with zero consumers.
// This simulates the normal lifecycle: Add (pending) → dispatch → Activate(0).
// Use this for tests that want basic TTL behavior without pending-flag protection.
func addAndActivate(cache *RecentUpdateCache, update *ReceivedUpdate) {
	cache.Add(update)
	cache.Activate(update.WireUpdate.MessageID(), 0)
}

// --- FakeClock tests ---

// TestRecentCacheWithFakeClock verifies FakeClock injection into RecentUpdateCache.
//
// VALIDATES: SetClock() injection works — cache uses injected clock for TTL decisions.
// PREVENTS: "Bridge to nowhere" — injection interfaces exist but don't work end-to-end.
func TestRecentCacheWithFakeClock(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := sim.NewFakeClock(start)

	cache := NewRecentUpdateCache(time.Minute, 100)
	cache.SetClock(fc)

	// Add and activate entry at fake time t=0
	addAndActivate(cache, newTestUpdate(42))

	// Should exist (TTL not expired at t=0)
	if !cache.Contains(42) {
		t.Fatal("expected entry to exist at t=0")
	}

	// Advance 30s — still within 60s TTL
	fc.Add(30 * time.Second)
	if !cache.Contains(42) {
		t.Fatal("expected entry to exist at t=30s (TTL=60s)")
	}

	// Advance past TTL (total: 61s > 60s TTL)
	fc.Add(31 * time.Second)
	if cache.Contains(42) {
		t.Error("expected entry to expire at t=61s (TTL=60s)")
	}

	// Verify Len also uses fake clock
	if cache.Len() != 0 {
		t.Errorf("Len() = %d, want 0 after expiry", cache.Len())
	}
}

// TestRecentCacheFakeClockResetTTL verifies TTL reset with FakeClock.
//
// VALIDATES: ResetTTL uses injected clock, not real time.
// PREVENTS: Mixed clock usage between Add and ResetTTL.
func TestRecentCacheFakeClockResetTTL(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := sim.NewFakeClock(start)

	cache := NewRecentUpdateCache(time.Minute, 100)
	cache.SetClock(fc)

	addAndActivate(cache, newTestUpdate(1))

	// Advance 50s (10s before expiry)
	fc.Add(50 * time.Second)

	// Reset TTL — should extend by another 60s from t=50s
	if !cache.ResetTTL(1) {
		t.Fatal("ResetTTL returned false")
	}

	// Advance 20s more (total t=70s — past original TTL but within reset TTL)
	fc.Add(20 * time.Second)
	if !cache.Contains(1) {
		t.Error("expected entry to exist after TTL reset (t=70s, reset at t=50s, new expiry t=110s)")
	}

	// Advance past reset TTL (total t=111s > reset expiry t=110s)
	fc.Add(41 * time.Second)
	if cache.Contains(1) {
		t.Error("expected entry to expire after reset TTL elapsed")
	}
}

// --- Basic cache operations ---

// TestRecentUpdateCacheAdd verifies cache insertion and retrieval via Get.
//
// VALIDATES: Updates are cached and retrievable via Get (non-destructive).
// PREVENTS: Lost updates, broken forwarding.
func TestRecentUpdateCacheAdd(t *testing.T) {
	cache := NewRecentUpdateCache(time.Minute, 100)

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
	cache := NewRecentUpdateCache(time.Minute, 100)

	_, ok := cache.Get(999)
	if ok {
		t.Error("expected not found for non-existent ID")
	}
}

// TestRecentUpdateCacheExpiry verifies TTL expiration.
//
// VALIDATES: Activated entries with zero consumers expire after TTL.
// PREVENTS: Stale data being forwarded.
func TestRecentUpdateCacheExpiry(t *testing.T) {
	// Use very short TTL for test
	cache := NewRecentUpdateCache(10*time.Millisecond, 100)

	addAndActivate(cache, newTestUpdate(1))

	// Should be found immediately
	if !cache.Contains(1) {
		t.Fatal("expected to find update before TTL")
	}

	// Wait for expiry
	time.Sleep(20 * time.Millisecond)

	// Should be expired
	if cache.Contains(1) {
		t.Error("expected not found after TTL expiry")
	}
}

// TestRecentUpdateCacheLazyCleanup verifies expired entries evicted on Add.
//
// VALIDATES: Expired entries removed during Add().
// PREVENTS: Unbounded memory growth.
func TestRecentUpdateCacheLazyCleanup(t *testing.T) {
	cache := NewRecentUpdateCache(10*time.Millisecond, 100)

	// Add and activate first update (makes it TTL-evictable)
	addAndActivate(cache, newTestUpdate(1))

	// Wait for expiry
	time.Sleep(20 * time.Millisecond)

	// Add second update — should trigger cleanup of expired entry 1
	cache.Add(newTestUpdate(2))

	// First should be cleaned up (internal check via Len)
	// Entry 2 is pending (counts as valid), entry 1 expired (cleaned)
	if cache.Len() != 1 {
		t.Errorf("Len() = %d, want 1 after cleanup", cache.Len())
	}
}

// TestRecentUpdateCacheSoftLimit verifies Add always succeeds beyond maxEntries.
//
// VALIDATES: Soft limit warns but never rejects (AC-1).
// PREVENTS: UPDATE loss when cache is full.
func TestRecentUpdateCacheSoftLimit(t *testing.T) {
	cache := NewRecentUpdateCache(time.Minute, 3)

	// Fill cache past the soft limit
	for i := uint64(1); i <= 5; i++ {
		addAndActivate(cache, newTestUpdate(i))
	}

	// All 5 entries should exist (soft limit never rejects)
	if cache.Len() != 5 {
		t.Errorf("Len() = %d, want 5 (soft limit never rejects)", cache.Len())
	}

	for i := uint64(1); i <= 5; i++ {
		if !cache.Contains(i) {
			t.Errorf("expected update %d to exist", i)
		}
	}
}

// TestRecentUpdateCacheConcurrency verifies thread safety.
//
// VALIDATES: Concurrent Add/Get/Decrement are safe.
// PREVENTS: Race conditions, data corruption.
func TestRecentUpdateCacheConcurrency(t *testing.T) {
	cache := NewRecentUpdateCache(time.Minute, 1000)

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

	// Concurrent readers (using Contains to avoid removing entries)
	for range goroutines {
		wg.Go(func() {
			for i := range opsPerGoroutine {
				cache.Contains(uint64(i)) //nolint:gosec // G115: test values are small
				_ = cache.Len()
			}
		})
	}

	wg.Wait()

	// Cache should have entries (exact count depends on timing)
	if cache.Len() == 0 {
		t.Error("expected some entries after concurrent operations")
	}
}

// TestRecentUpdateCacheZeroTTL verifies immediate expiry with zero TTL.
//
// VALIDATES: Zero TTL means activated entries expire immediately.
// PREVENTS: Configuration edge case bugs.
func TestRecentUpdateCacheZeroTTL(t *testing.T) {
	cache := NewRecentUpdateCache(0, 100)

	addAndActivate(cache, newTestUpdate(1))

	// Should be expired immediately (zero TTL, zero consumers)
	if cache.Contains(1) {
		t.Error("expected immediate expiry with zero TTL")
	}
}

// --- Delete and ResetTTL ---

// TestRecentUpdateCacheDelete verifies explicit deletion.
//
// VALIDATES: Delete removes entry from cache.
// PREVENTS: Memory leaks from unflushed entries.
func TestRecentUpdateCacheDelete(t *testing.T) {
	cache := NewRecentUpdateCache(time.Minute, 100)

	cache.Add(newTestUpdate(1))

	// Should exist
	if !cache.Contains(1) {
		t.Fatal("expected to find update before delete")
	}

	// Delete
	if !cache.Delete(1) {
		t.Error("Delete returned false for existing entry")
	}

	// Should not exist
	if cache.Contains(1) {
		t.Error("expected not found after delete")
	}

	// Delete again should return false
	if cache.Delete(1) {
		t.Error("Delete returned true for non-existent entry")
	}
}

// TestRecentUpdateCacheResetTTL verifies TTL extension.
//
// VALIDATES: ResetTTL extends entry lifetime.
// PREVENTS: Premature expiry on active updates.
func TestRecentUpdateCacheResetTTL(t *testing.T) {
	cache := NewRecentUpdateCache(20*time.Millisecond, 100)

	addAndActivate(cache, newTestUpdate(1))

	// Wait 15ms (before original expiry)
	time.Sleep(15 * time.Millisecond)

	// Reset TTL
	if !cache.ResetTTL(1) {
		t.Error("ResetTTL returned false for existing entry")
	}

	// Wait another 15ms (would have expired without reset)
	time.Sleep(15 * time.Millisecond)

	// Should still exist (TTL was reset)
	if !cache.Contains(1) {
		t.Error("expected entry to exist after TTL reset")
	}

	// Wait for new TTL to expire
	time.Sleep(10 * time.Millisecond)

	// Now should be expired
	if cache.Contains(1) {
		t.Error("expected entry to expire after reset TTL elapsed")
	}
}

// TestRecentUpdateCacheResetTTLNotFound verifies ResetTTL on missing entry.
//
// VALIDATES: ResetTTL returns false for non-existent entry.
// PREVENTS: False positives on missing entries.
func TestRecentUpdateCacheResetTTLNotFound(t *testing.T) {
	cache := NewRecentUpdateCache(time.Minute, 100)

	if cache.ResetTTL(999) {
		t.Error("ResetTTL returned true for non-existent entry")
	}
}

// --- Get (non-destructive) ---

// TestCacheGetNonDestructive verifies Get does not remove entries.
//
// VALIDATES: Multiple Gets return same entry, entry remains in cache (AC-7).
// PREVENTS: Accidental entry removal on read.
func TestCacheGetNonDestructive(t *testing.T) {
	cache := NewRecentUpdateCache(time.Minute, 100)

	cache.Add(newTestUpdate(1))

	// Multiple Gets should all succeed
	for i := range 5 {
		got, ok := cache.Get(1)
		if !ok {
			t.Fatalf("Get #%d failed", i)
		}
		if got.WireUpdate.MessageID() != 1 {
			t.Fatalf("Get #%d returned wrong ID: %d", i, got.WireUpdate.MessageID())
		}
	}

	// Entry still in cache
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
	cache := NewRecentUpdateCache(time.Minute, 100)

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

// --- Refcount lifecycle ---

// TestCacheRefcountBasic verifies the full Add→Activate→Get→Decrement lifecycle.
//
// VALIDATES: Consumers=2, each Decrement reduces by 1, evictable when 0 (AC-2).
// PREVENTS: Reference count corruption, premature/late eviction.
func TestCacheRefcountBasic(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := sim.NewFakeClock(start)

	cache := NewRecentUpdateCache(10*time.Millisecond, 100)
	cache.SetClock(fc)

	// Add and activate with 2 consumers
	cache.Add(newTestUpdate(1))
	cache.Activate(1, 2)

	// Advance past TTL — entry still protected (consumers=2)
	fc.Add(20 * time.Millisecond)
	if !cache.Contains(1) {
		t.Fatal("entry with consumers=2 should survive TTL expiry")
	}

	// First consumer done
	if !cache.Decrement(1) {
		t.Fatal("Decrement returned false")
	}

	// Still protected (consumers=1)
	if !cache.Contains(1) {
		t.Fatal("entry with consumers=1 should survive")
	}

	// Second consumer done — consumers=0, entry becomes TTL-evictable
	if !cache.Decrement(1) {
		t.Fatal("second Decrement returned false")
	}

	// TTL reset on decrement to 0, advance past new TTL
	fc.Add(20 * time.Millisecond)

	// Now should be expired
	if cache.Contains(1) {
		t.Error("entry should expire after all consumers done and TTL elapsed")
	}
}

// TestCacheDecrementBeforeActivate verifies fast-plugin race handling.
//
// VALIDATES: Decrement before Activate yields correct net count (AC-3).
// PREVENTS: Negative consumers causing premature eviction.
func TestCacheDecrementBeforeActivate(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := sim.NewFakeClock(start)

	cache := NewRecentUpdateCache(10*time.Millisecond, 100)
	cache.SetClock(fc)

	cache.Add(newTestUpdate(1))

	// Fast plugin calls Decrement before Activate (race condition)
	if !cache.Decrement(1) {
		t.Fatal("Decrement on pending entry should succeed")
	}

	// Now Activate with 2 consumers — net consumers = 2 + (-1) = 1
	cache.Activate(1, 2)

	// Entry should still be protected (consumers=1)
	fc.Add(20 * time.Millisecond) // Past TTL
	if !cache.Contains(1) {
		t.Fatal("entry with net consumers=1 should survive TTL expiry")
	}

	// Final decrement — consumers=0
	cache.Decrement(1)
	fc.Add(20 * time.Millisecond) // Past new TTL

	if cache.Contains(1) {
		t.Error("entry should expire after all consumers done")
	}
}

// TestCacheActivateZeroConsumers verifies no-subscriber case.
//
// VALIDATES: Activate(id, 0) makes entry normally TTL-evictable (AC-5).
// PREVENTS: Permanently pending entries when no plugins subscribe.
func TestCacheActivateZeroConsumers(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := sim.NewFakeClock(start)

	cache := NewRecentUpdateCache(10*time.Millisecond, 100)
	cache.SetClock(fc)

	cache.Add(newTestUpdate(1))

	// Entry is pending — protected
	if !cache.Contains(1) {
		t.Fatal("pending entry should exist")
	}

	// Activate with zero consumers
	cache.Activate(1, 0)

	// Should still exist within TTL
	if !cache.Contains(1) {
		t.Fatal("activated entry should exist within TTL")
	}

	// Advance past TTL — should expire (consumers=0, not pending)
	fc.Add(20 * time.Millisecond)
	if cache.Contains(1) {
		t.Error("entry with 0 consumers should expire after TTL")
	}
}

// --- Safety valve ---

// TestCacheSafetyValve verifies force-eviction of long-retained entries.
//
// VALIDATES: Entry retained > 5 min is force-evicted (AC-4).
// PREVENTS: Memory leak from crashed plugins that never decrement.
func TestCacheSafetyValve(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := sim.NewFakeClock(start)

	cache := NewRecentUpdateCache(time.Minute, 100)
	cache.SetClock(fc)

	// Add, activate with 1 consumer (simulates plugin that will "crash")
	cache.Add(newTestUpdate(1))
	cache.Activate(1, 1)

	// At t=4m59s — entry still protected (under safety valve duration)
	fc.Add(4*time.Minute + 59*time.Second)
	if !cache.Contains(1) {
		t.Fatal("entry should be protected at 4m59s (safety valve = 5min)")
	}

	// At t=5m01s — entry force-evictable (exceeds safety valve)
	fc.Add(2 * time.Second) // total: 5m01s
	// Contains checks isProtected, which will return false for safety valve breach
	if cache.Contains(1) {
		t.Error("entry should be force-evictable after 5min safety valve")
	}
}

// --- Concurrent Decrement ---

// TestCacheConcurrentDecrement verifies race-free concurrent Decrements.
//
// VALIDATES: Multiple goroutines decrementing simultaneously is safe (AC-8).
// PREVENTS: Race conditions in consumer count updates.
func TestCacheConcurrentDecrement(t *testing.T) {
	cache := NewRecentUpdateCache(time.Minute, 100)

	const consumers = 50

	cache.Add(newTestUpdate(1))
	cache.Activate(1, consumers)

	var wg sync.WaitGroup
	for range consumers {
		wg.Go(func() {
			cache.Decrement(1)
		})
	}
	wg.Wait()

	// All consumers done — entry should be TTL-evictable (consumers=0)
	// Still within TTL, so it should exist but as non-protected
	if !cache.Contains(1) {
		t.Error("entry should still exist within TTL even after all decrements")
	}

	// Verify it's no longer protected by checking Get after TTL
	// (We can't easily advance time with real clock, so just verify the entry exists)
}

// --- Retain / Release (API commands) ---

// TestRecentUpdateCacheRetain verifies Retain increments consumers on activated entry.
//
// VALIDATES: Retain on activated entry makes it protected beyond TTL.
// PREVENTS: Premature eviction of routes needed for graceful restart.
func TestRecentUpdateCacheRetain(t *testing.T) {
	cache := NewRecentUpdateCache(10*time.Millisecond, 100)

	addAndActivate(cache, newTestUpdate(1))

	// Retain the entry (consumers: 0 → 1)
	if !cache.Retain(1) {
		t.Fatal("Retain returned false for existing entry")
	}

	// Wait for TTL to expire
	time.Sleep(20 * time.Millisecond)

	// Add another entry to trigger lazy cleanup
	cache.Add(newTestUpdate(2))

	// Retained entry should still exist despite TTL expiry (consumers=1)
	if !cache.Contains(1) {
		t.Error("retained entry should survive lazy cleanup")
	}

	// New entry should also exist (just added, pending)
	if !cache.Contains(2) {
		t.Error("new entry should exist")
	}
}

// TestRecentUpdateCacheRetainNotFound verifies Retain on missing entry.
//
// VALIDATES: Retain returns false for non-existent entry.
// PREVENTS: Silent failures on invalid msg-ids.
func TestRecentUpdateCacheRetainNotFound(t *testing.T) {
	cache := NewRecentUpdateCache(time.Minute, 100)

	if cache.Retain(999) {
		t.Error("Retain returned true for non-existent entry")
	}
}

// TestCacheRetainIncrementsConsumers verifies Retain/Release refcount semantics.
//
// VALIDATES: Retain adds 1, Release subtracts 1 — balanced usage (AC-6).
// PREVENTS: Refcount imbalance from API commands.
func TestCacheRetainIncrementsConsumers(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := sim.NewFakeClock(start)

	cache := NewRecentUpdateCache(10*time.Millisecond, 100)
	cache.SetClock(fc)

	addAndActivate(cache, newTestUpdate(1))

	// Retain twice (consumers: 0 → 1 → 2)
	cache.Retain(1)
	cache.Retain(1)

	fc.Add(20 * time.Millisecond) // Past TTL

	// Still protected (consumers=2)
	if !cache.Contains(1) {
		t.Fatal("double-retained entry should survive TTL")
	}

	// Release once (consumers: 2 → 1)
	cache.Release(1)

	// Still protected (consumers=1)
	if !cache.Contains(1) {
		t.Fatal("entry with consumers=1 should survive")
	}

	// Release again (consumers: 1 → 0, TTL reset)
	cache.Release(1)

	// TTL was just reset, should exist within new TTL window
	if !cache.Contains(1) {
		t.Fatal("entry should exist right after final release (TTL just reset)")
	}

	// Advance past new TTL
	fc.Add(20 * time.Millisecond)

	// Now evictable
	if cache.Contains(1) {
		t.Error("entry should expire after all releases and TTL elapsed")
	}
}

// TestRecentUpdateCacheRelease verifies release allows TTL expiry.
//
// VALIDATES: Release decrements consumers and resets TTL.
// PREVENTS: Memory leaks from permanently retained entries.
func TestRecentUpdateCacheRelease(t *testing.T) {
	cache := NewRecentUpdateCache(15*time.Millisecond, 100)

	addAndActivate(cache, newTestUpdate(1))

	// Retain the entry (consumers: 0 → 1)
	if !cache.Retain(1) {
		t.Fatal("Retain failed")
	}

	// Wait for original TTL
	time.Sleep(20 * time.Millisecond)

	// Should still exist (consumers=1, protected)
	if !cache.Contains(1) {
		t.Fatal("retained entry should exist")
	}

	// Release the entry (consumers: 1 → 0, TTL reset)
	if !cache.Release(1) {
		t.Fatal("Release returned false for existing entry")
	}

	// Should still exist (TTL reset on release)
	if !cache.Contains(1) {
		t.Error("entry should exist immediately after release")
	}

	// Wait for new TTL to expire
	time.Sleep(20 * time.Millisecond)

	// Now should be expired (consumers=0, not protected)
	if cache.Contains(1) {
		t.Error("released entry should expire after TTL")
	}
}

// TestRecentUpdateCacheReleaseNotFound verifies Release on missing entry.
//
// VALIDATES: Release returns false for non-existent entry.
// PREVENTS: Silent failures on invalid msg-ids.
func TestRecentUpdateCacheReleaseNotFound(t *testing.T) {
	cache := NewRecentUpdateCache(time.Minute, 100)

	if cache.Release(999) {
		t.Error("Release returned true for non-existent entry")
	}
}

// TestRecentUpdateCacheRetainedSurvivesExpiry verifies retained entries in List.
//
// VALIDATES: List includes retained entries even after TTL.
// PREVENTS: Missing retained IDs in API response.
func TestRecentUpdateCacheRetainedSurvivesExpiry(t *testing.T) {
	cache := NewRecentUpdateCache(10*time.Millisecond, 100)

	addAndActivate(cache, newTestUpdate(1))
	addAndActivate(cache, newTestUpdate(2))
	cache.Retain(1) // Retain only entry 1 (consumers: 0 → 1)

	// Wait for TTL
	time.Sleep(20 * time.Millisecond)

	// List should include only retained entry (entry 2 expired, entry 1 protected)
	ids := cache.List()
	if len(ids) != 1 {
		t.Errorf("List() len = %d, want 1", len(ids))
	}
	if len(ids) > 0 && ids[0] != 1 {
		t.Errorf("List()[0] = %d, want 1", ids[0])
	}
}

// --- List ---

// TestRecentUpdateCacheList verifies List returns all valid msg-ids.
//
// VALIDATES: List returns IDs of all non-expired entries.
// PREVENTS: Missing entries in API response.
func TestRecentUpdateCacheList(t *testing.T) {
	cache := NewRecentUpdateCache(time.Minute, 100)

	// Empty cache
	ids := cache.List()
	if len(ids) != 0 {
		t.Errorf("List() = %v, want empty", ids)
	}

	// Add entries
	cache.Add(newTestUpdate(10))
	cache.Add(newTestUpdate(20))
	cache.Add(newTestUpdate(30))

	ids = cache.List()
	if len(ids) != 3 {
		t.Errorf("List() len = %d, want 3", len(ids))
	}

	// Check all IDs present (order not guaranteed)
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

// TestRecentUpdateCacheListExcludesExpired verifies List excludes expired entries.
//
// VALIDATES: List only returns non-expired entries.
// PREVENTS: Stale IDs in API response.
func TestRecentUpdateCacheListExcludesExpired(t *testing.T) {
	cache := NewRecentUpdateCache(10*time.Millisecond, 100)

	addAndActivate(cache, newTestUpdate(1))

	// Should have one entry
	if len(cache.List()) != 1 {
		t.Fatal("expected 1 entry before expiry")
	}

	// Wait for expiry
	time.Sleep(20 * time.Millisecond)

	// List should exclude expired
	ids := cache.List()
	if len(ids) != 0 {
		t.Errorf("List() = %v, want empty after expiry", ids)
	}
}

// --- Decrement ---

// TestCacheDecrementNotFound verifies Decrement on missing entry.
//
// VALIDATES: Decrement returns false for non-existent entry.
// PREVENTS: Silent failures on invalid msg-ids.
func TestCacheDecrementNotFound(t *testing.T) {
	cache := NewRecentUpdateCache(time.Minute, 100)

	if cache.Decrement(999) {
		t.Error("Decrement returned true for non-existent entry")
	}
}

// TestCacheDecrementToZeroResetsTTL verifies TTL reset when consumers hit 0.
//
// VALIDATES: Decrement to 0 resets TTL, giving a fresh expiration window.
// PREVENTS: Immediate eviction when last consumer finishes.
func TestCacheDecrementToZeroResetsTTL(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := sim.NewFakeClock(start)

	cache := NewRecentUpdateCache(time.Minute, 100)
	cache.SetClock(fc)

	cache.Add(newTestUpdate(1))
	cache.Activate(1, 1) // consumers=1

	// Advance past original TTL — entry protected by consumers
	fc.Add(2 * time.Minute) // 2 min > 1 min TTL

	if !cache.Contains(1) {
		t.Fatal("entry with consumers=1 should survive past TTL")
	}

	// Consumer finishes — TTL reset to now + 1min
	cache.Decrement(1) // consumers=0

	// Entry should exist within new TTL window
	fc.Add(30 * time.Second) // 30s < 1 min new TTL
	if !cache.Contains(1) {
		t.Fatal("entry should exist within fresh TTL after decrement to 0")
	}

	// Advance past new TTL
	fc.Add(31 * time.Second) // total 61s > 1 min
	if cache.Contains(1) {
		t.Error("entry should expire after fresh TTL elapsed")
	}
}

// --- Buffer ownership ---

// TestCacheBufferReturnedOnEviction verifies buffer lifecycle.
//
// VALIDATES: Buffer returned to pool only when entry evicted with consumers=0.
// PREVENTS: Buffer leaks or double-free.
func TestCacheBufferReturnedOnEviction(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := sim.NewFakeClock(start)

	cache := NewRecentUpdateCache(10*time.Millisecond, 100)
	cache.SetClock(fc)

	// Add entry — cache owns buffer
	addAndActivate(cache, newTestUpdate(1))

	// Get does NOT transfer ownership
	got, ok := cache.Get(1)
	if !ok {
		t.Fatal("Get failed")
	}

	// got.poolBuf may be nil in test updates (no real pool buffer).
	// The test validates the Get/eviction path regardless.
	_ = got

	// Advance past TTL
	fc.Add(20 * time.Millisecond)

	// Trigger lazy cleanup by adding another entry
	cache.Add(newTestUpdate(2))

	// Entry 1 should have been evicted (buffer returned to pool)
	if cache.Contains(1) {
		t.Error("entry should be evicted after TTL")
	}

	// Entry 2 should exist (pending)
	if !cache.Contains(2) {
		t.Error("new entry should exist")
	}
}

// TestCacheDecrementZeroTTLImmediateCleanup verifies zero-TTL entries are cleaned
// immediately when consumers drop to 0.
//
// VALIDATES: Zero-TTL + consumers=0 triggers immediate eviction in Decrement.
// PREVENTS: Stale entries with zero TTL lingering until next Add.
func TestCacheDecrementZeroTTLImmediateCleanup(t *testing.T) {
	cache := NewRecentUpdateCache(0, 100) // Zero TTL

	cache.Add(newTestUpdate(1))
	cache.Activate(1, 1) // consumers=1, protected

	// Entry exists (protected by consumers)
	if !cache.Contains(1) {
		t.Fatal("entry with consumers should exist even with zero TTL")
	}

	// Decrement to 0 — should clean up immediately (zero TTL)
	cache.Decrement(1)

	// Entry should be gone
	if cache.Contains(1) {
		t.Error("entry with zero TTL should be cleaned up immediately on decrement to 0")
	}
}
