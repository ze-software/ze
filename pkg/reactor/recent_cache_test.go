package reactor

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	bgpctx "github.com/exa-networks/zebgp/pkg/bgp/context"
)

// TestRecentUpdateCacheAdd verifies cache insertion.
//
// VALIDATES: Updates are cached and retrievable.
// PREVENTS: Lost updates, broken forwarding.
func TestRecentUpdateCacheAdd(t *testing.T) {
	cache := NewRecentUpdateCache(time.Minute, 100)

	update := &ReceivedUpdate{
		UpdateID:     1,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		SourceCtxID:  bgpctx.ContextID(1),
		ReceivedAt:   time.Now(),
	}

	cache.Add(update)

	got, ok := cache.Get(1)
	if !ok {
		t.Fatal("expected to find update 1")
	}
	if got.UpdateID != 1 {
		t.Errorf("UpdateID = %d, want 1", got.UpdateID)
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
// VALIDATES: Expired entries return not found.
// PREVENTS: Stale data being forwarded.
func TestRecentUpdateCacheExpiry(t *testing.T) {
	// Use very short TTL for test
	cache := NewRecentUpdateCache(10*time.Millisecond, 100)

	update := &ReceivedUpdate{
		UpdateID:     1,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		SourceCtxID:  bgpctx.ContextID(1),
		ReceivedAt:   time.Now(),
	}

	cache.Add(update)

	// Should be found immediately
	if _, ok := cache.Get(1); !ok {
		t.Fatal("expected to find update before TTL")
	}

	// Wait for expiry
	time.Sleep(20 * time.Millisecond)

	// Should be expired
	if _, ok := cache.Get(1); ok {
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
	cache.Add(&ReceivedUpdate{
		UpdateID:     1,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		SourceCtxID:  bgpctx.ContextID(1),
		ReceivedAt:   time.Now(),
	})

	// Wait for expiry
	time.Sleep(20 * time.Millisecond)

	// Add second update - should trigger cleanup
	cache.Add(&ReceivedUpdate{
		UpdateID:     2,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		SourceCtxID:  bgpctx.ContextID(1),
		ReceivedAt:   time.Now(),
	})

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
		cache.Add(&ReceivedUpdate{
			UpdateID:     i,
			SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
			SourceCtxID:  bgpctx.ContextID(1),
			ReceivedAt:   time.Now(),
		})
	}

	if cache.Len() != 3 {
		t.Errorf("Len() = %d, want 3", cache.Len())
	}

	// Try to add one more - should be dropped
	cache.Add(&ReceivedUpdate{
		UpdateID:     4,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		SourceCtxID:  bgpctx.ContextID(1),
		ReceivedAt:   time.Now(),
	})

	// Size should still be 3
	if cache.Len() != 3 {
		t.Errorf("Len() = %d, want 3 after overflow", cache.Len())
	}

	// Update 4 should not exist
	if _, ok := cache.Get(4); ok {
		t.Error("expected update 4 to be dropped")
	}

	// Original 3 should still exist
	for i := uint64(1); i <= 3; i++ {
		if _, ok := cache.Get(i); !ok {
			t.Errorf("expected update %d to exist", i)
		}
	}
}

// TestRecentUpdateCacheConcurrency verifies thread safety.
//
// VALIDATES: Concurrent Add/Get are safe.
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
				cache.Add(&ReceivedUpdate{
					UpdateID:     uint64(base*opsPerGoroutine + i), //nolint:gosec // G115: test values are small
					SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
					SourceCtxID:  bgpctx.ContextID(1),
					ReceivedAt:   time.Now(),
				})
			}
		}(g)
	}

	// Concurrent readers
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				cache.Get(uint64(i)) //nolint:gosec // G115: test values are small
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

	cache.Add(&ReceivedUpdate{
		UpdateID:     1,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		SourceCtxID:  bgpctx.ContextID(1),
		ReceivedAt:   time.Now(),
	})

	// Should be expired immediately
	if _, ok := cache.Get(1); ok {
		t.Error("expected immediate expiry with zero TTL")
	}
}

// TestRecentUpdateCacheDelete verifies explicit deletion.
//
// VALIDATES: Delete removes entry from cache.
// PREVENTS: Memory leaks from unflushed entries.
func TestRecentUpdateCacheDelete(t *testing.T) {
	cache := NewRecentUpdateCache(time.Minute, 100)

	cache.Add(&ReceivedUpdate{
		UpdateID:     1,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		SourceCtxID:  bgpctx.ContextID(1),
		ReceivedAt:   time.Now(),
	})

	// Should exist
	if _, ok := cache.Get(1); !ok {
		t.Fatal("expected to find update before delete")
	}

	// Delete
	if !cache.Delete(1) {
		t.Error("Delete returned false for existing entry")
	}

	// Should not exist
	if _, ok := cache.Get(1); ok {
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

	cache.Add(&ReceivedUpdate{
		UpdateID:     1,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		SourceCtxID:  bgpctx.ContextID(1),
		ReceivedAt:   time.Now(),
	})

	// Wait 15ms (before original expiry)
	time.Sleep(15 * time.Millisecond)

	// Reset TTL
	if !cache.ResetTTL(1) {
		t.Error("ResetTTL returned false for existing entry")
	}

	// Wait another 15ms (would have expired without reset)
	time.Sleep(15 * time.Millisecond)

	// Should still exist (TTL was reset)
	if _, ok := cache.Get(1); !ok {
		t.Error("expected entry to exist after TTL reset")
	}

	// Wait for new TTL to expire
	time.Sleep(10 * time.Millisecond)

	// Now should be expired
	if _, ok := cache.Get(1); ok {
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
