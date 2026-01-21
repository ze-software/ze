package reactor

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/plugin"
)

// emptyPayload is a minimal valid UPDATE payload for cache tests.
// Format: WithdrawnLen(2)=0 + AttrLen(2)=0.
var emptyPayload = []byte{0, 0, 0, 0}

// newTestUpdate creates a ReceivedUpdate with messageID set on WireUpdate.
func newTestUpdate(id uint64) *ReceivedUpdate {
	wu := plugin.NewWireUpdate(emptyPayload, bgpctx.ContextID(1))
	wu.SetMessageID(id)
	return &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}
}

// TestRecentUpdateCacheAdd verifies cache insertion.
//
// VALIDATES: Updates are cached and retrievable.
// PREVENTS: Lost updates, broken forwarding.
func TestRecentUpdateCacheAdd(t *testing.T) {
	cache := NewRecentUpdateCache(time.Minute, 100)

	update := newTestUpdate(1)

	cache.Add(update)

	got, ok := cache.Take(1)
	if !ok {
		t.Fatal("expected to find update 1")
	}
	defer got.Release()
	if got.WireUpdate.MessageID() != 1 {
		t.Errorf("MessageID = %d, want 1", got.WireUpdate.MessageID())
	}
}

// TestRecentUpdateCacheNotFound verifies missing entries.
//
// VALIDATES: Non-existent IDs return not found.
// PREVENTS: False positives on lookup.
func TestRecentUpdateCacheNotFound(t *testing.T) {
	cache := NewRecentUpdateCache(time.Minute, 100)

	_, ok := cache.Take(999)
	if ok {
		t.Error("expected not found for non-existent ID")
	}
}

// TestRecentUpdateCacheExpiry verifies TTL expiration.
//
// VALIDATES: Expired entries return not found.
// PREVENTS: Stale data being forwarded.
func TestRecentUpdateCacheExpiry(t *testing.T) {
	// Use very short TTL for test
	cache := NewRecentUpdateCache(10*time.Millisecond, 100)

	cache.Add(newTestUpdate(1))

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

	// Add first update
	cache.Add(newTestUpdate(1))

	// Wait for expiry
	time.Sleep(20 * time.Millisecond)

	// Add second update - should trigger cleanup
	cache.Add(newTestUpdate(2))

	// First should be cleaned up (internal check via Len)
	if cache.Len() != 1 {
		t.Errorf("Len() = %d, want 1 after cleanup", cache.Len())
	}
}

// TestRecentUpdateCacheMaxEntries verifies fixed size limit.
//
// VALIDATES: Cache rejects new entries when full after eviction.
// PREVENTS: Memory exhaustion under high load.
func TestRecentUpdateCacheMaxEntries(t *testing.T) {
	cache := NewRecentUpdateCache(time.Minute, 3)

	// Fill cache
	for i := uint64(1); i <= 3; i++ {
		cache.Add(newTestUpdate(i))
	}

	if cache.Len() != 3 {
		t.Errorf("Len() = %d, want 3", cache.Len())
	}

	// Try to add one more - should be dropped
	cache.Add(newTestUpdate(4))

	// Size should still be 3
	if cache.Len() != 3 {
		t.Errorf("Len() = %d, want 3 after overflow", cache.Len())
	}

	// Update 4 should not exist
	if cache.Contains(4) {
		t.Error("expected update 4 to be dropped")
	}

	// Original 3 should still exist
	for i := uint64(1); i <= 3; i++ {
		if !cache.Contains(i) {
			t.Errorf("expected update %d to exist", i)
		}
	}
}

// TestRecentUpdateCacheConcurrency verifies thread safety.
//
// VALIDATES: Concurrent Add/Take are safe.
// PREVENTS: Race conditions, data corruption.
func TestRecentUpdateCacheConcurrency(t *testing.T) {
	cache := NewRecentUpdateCache(time.Minute, 1000)

	var wg sync.WaitGroup
	const goroutines = 10
	const opsPerGoroutine = 100

	// Concurrent writers
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				cache.Add(newTestUpdate(uint64(base*opsPerGoroutine + i))) //nolint:gosec // G115: test values are small
			}
		}(g)
	}

	// Concurrent readers (using Contains to avoid removing entries)
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				cache.Contains(uint64(i)) //nolint:gosec // G115: test values are small
				_ = cache.Len()
			}
		}()
	}

	wg.Wait()

	// Cache should have entries (exact count depends on timing)
	if cache.Len() == 0 {
		t.Error("expected some entries after concurrent operations")
	}
}

// TestRecentUpdateCacheZeroTTL verifies immediate expiry with zero TTL.
//
// VALIDATES: Zero TTL means entries expire immediately.
// PREVENTS: Configuration edge case bugs.
func TestRecentUpdateCacheZeroTTL(t *testing.T) {
	cache := NewRecentUpdateCache(0, 100)

	cache.Add(newTestUpdate(1))

	// Should be expired immediately
	if cache.Contains(1) {
		t.Error("expected immediate expiry with zero TTL")
	}
}

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

	cache.Add(newTestUpdate(1))

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

// TestRecentUpdateCacheTakeTransfersOwnership verifies Take() removes entry.
//
// VALIDATES: Take() removes entry from cache, caller owns buffer.
// PREVENTS: Use-after-free, double-free.
func TestRecentUpdateCacheTakeTransfersOwnership(t *testing.T) {
	cache := NewRecentUpdateCache(time.Minute, 100)

	cache.Add(newTestUpdate(1))

	// Take should remove from cache
	got, ok := cache.Take(1)
	if !ok {
		t.Fatal("expected to find update")
	}

	// Entry should no longer be in cache
	if cache.Contains(1) {
		t.Error("entry should be removed after Take")
	}

	// Second Take should return not found
	_, ok2 := cache.Take(1)
	if ok2 {
		t.Error("second Take should return not found")
	}

	// Release the buffer
	got.Release()

	// Release is idempotent
	got.Release() // should not panic
}

// TestRecentUpdateCacheRetain verifies retained entries skip lazy eviction.
//
// VALIDATES: Retained entries persist beyond TTL during lazy cleanup.
// PREVENTS: Premature eviction of routes needed for graceful restart.
func TestRecentUpdateCacheRetain(t *testing.T) {
	cache := NewRecentUpdateCache(10*time.Millisecond, 100)

	cache.Add(newTestUpdate(1))

	// Retain the entry
	if !cache.Retain(1) {
		t.Fatal("Retain returned false for existing entry")
	}

	// Wait for TTL to expire
	time.Sleep(20 * time.Millisecond)

	// Add another entry to trigger lazy cleanup
	cache.Add(newTestUpdate(2))

	// Retained entry should still exist despite TTL expiry
	if !cache.Contains(1) {
		t.Error("retained entry should survive lazy cleanup")
	}

	// Non-retained entry should also exist (just added)
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

// TestRecentUpdateCacheRelease verifies release clears retained flag and resets TTL.
//
// VALIDATES: Release allows eviction after TTL expires.
// PREVENTS: Memory leaks from permanently retained entries.
func TestRecentUpdateCacheRelease(t *testing.T) {
	cache := NewRecentUpdateCache(15*time.Millisecond, 100)

	cache.Add(newTestUpdate(1))

	// Retain the entry
	if !cache.Retain(1) {
		t.Fatal("Retain failed")
	}

	// Wait for original TTL
	time.Sleep(20 * time.Millisecond)

	// Should still exist (retained)
	if !cache.Contains(1) {
		t.Fatal("retained entry should exist")
	}

	// Release the entry
	if !cache.Release(1) {
		t.Fatal("Release returned false for existing entry")
	}

	// Should still exist (TTL reset on release)
	if !cache.Contains(1) {
		t.Error("entry should exist immediately after release")
	}

	// Wait for new TTL to expire
	time.Sleep(20 * time.Millisecond)

	// Now should be expired (no longer retained)
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

	cache.Add(newTestUpdate(1))

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

// TestRecentUpdateCacheRetainedSurvivesExpiry verifies retained entries in List.
//
// VALIDATES: List includes retained entries even after TTL.
// PREVENTS: Missing retained IDs in API response.
func TestRecentUpdateCacheRetainedSurvivesExpiry(t *testing.T) {
	cache := NewRecentUpdateCache(10*time.Millisecond, 100)

	cache.Add(newTestUpdate(1))
	cache.Add(newTestUpdate(2))
	cache.Retain(1) // Retain only entry 1

	// Wait for TTL
	time.Sleep(20 * time.Millisecond)

	// List should include only retained entry
	ids := cache.List()
	if len(ids) != 1 {
		t.Errorf("List() len = %d, want 1", len(ids))
	}
	if len(ids) > 0 && ids[0] != 1 {
		t.Errorf("List()[0] = %d, want 1", ids[0])
	}
}

// TestRecentUpdateCacheRetainIdempotent verifies double retain is safe.
//
// VALIDATES: Calling Retain twice on same entry is idempotent.
// PREVENTS: State corruption on duplicate retain calls.
func TestRecentUpdateCacheRetainIdempotent(t *testing.T) {
	cache := NewRecentUpdateCache(time.Minute, 100)

	cache.Add(newTestUpdate(1))

	// Retain twice
	if !cache.Retain(1) {
		t.Fatal("first Retain failed")
	}
	if !cache.Retain(1) {
		t.Fatal("second Retain failed")
	}

	// Entry should still be valid
	if !cache.Contains(1) {
		t.Error("entry should exist after double retain")
	}
}

// TestRecentUpdateCacheReleaseNonRetained verifies release on non-retained entry.
//
// VALIDATES: Release on non-retained entry resets TTL (extends lifetime).
// PREVENTS: Unexpected behavior when release called without prior retain.
func TestRecentUpdateCacheReleaseNonRetained(t *testing.T) {
	cache := NewRecentUpdateCache(15*time.Millisecond, 100)

	cache.Add(newTestUpdate(1))

	// Wait partial TTL
	time.Sleep(10 * time.Millisecond)

	// Release without prior retain - should reset TTL
	if !cache.Release(1) {
		t.Fatal("Release failed on non-retained entry")
	}

	// Wait another partial TTL (would have expired without release)
	time.Sleep(10 * time.Millisecond)

	// Should still exist (TTL was reset)
	if !cache.Contains(1) {
		t.Error("entry should exist after release reset TTL")
	}
}

// TestRecentUpdateCacheTakeRetained verifies Take works on retained entries.
//
// VALIDATES: Retained entries can be taken (forwarded).
// PREVENTS: Retained entries becoming stuck/unfetchable.
func TestRecentUpdateCacheTakeRetained(t *testing.T) {
	cache := NewRecentUpdateCache(10*time.Millisecond, 100)

	cache.Add(newTestUpdate(1))
	cache.Retain(1)

	// Wait for TTL to expire
	time.Sleep(20 * time.Millisecond)

	// Take should succeed (retained)
	got, ok := cache.Take(1)
	if !ok {
		t.Fatal("Take failed on retained entry")
	}
	defer got.Release()

	// Entry should be removed after Take
	if cache.Contains(1) {
		t.Error("entry should be removed after Take")
	}
}

// TestRecentUpdateCacheReleaseAfterTake verifies release fails after take.
//
// VALIDATES: Release returns false for taken (removed) entry.
// PREVENTS: False success on release of non-existent entry.
func TestRecentUpdateCacheReleaseAfterTake(t *testing.T) {
	cache := NewRecentUpdateCache(time.Minute, 100)

	cache.Add(newTestUpdate(1))
	cache.Retain(1)

	// Take the entry
	got, ok := cache.Take(1)
	if !ok {
		t.Fatal("Take failed")
	}
	got.Release()

	// Release should fail (entry no longer exists)
	if cache.Release(1) {
		t.Error("Release should return false for taken entry")
	}
}
