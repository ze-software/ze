package reactor

import (
	"errors"
	"sync"
	"time"
)

// ErrUpdateExpired is returned when an update-id is expired or not found.
var ErrUpdateExpired = errors.New("update-id expired or not found")

// RecentUpdateCache stores recently received UPDATEs for efficient forwarding.
// Thread-safe for concurrent Add/Get operations.
//
// Design:
//   - Time-based TTL: entries expire after configured duration
//   - Lazy cleanup: expired entries evicted on Add(), no background goroutine
//   - Fixed max-entries: rejects new entries when full after eviction
//
// Performance: O(n) scan on each Add(). Keep maxEntries reasonable (1000-10000).
type RecentUpdateCache struct {
	mu         sync.RWMutex
	entries    map[uint64]*cacheEntry
	ttl        time.Duration
	maxEntries int
}

// cacheEntry wraps an update with expiration time.
type cacheEntry struct {
	update    *ReceivedUpdate
	expiresAt time.Time
}

// NewRecentUpdateCache creates a cache with the given TTL and max size.
// ttl: how long entries remain valid (0 = immediate expiry)
// maxEntries: maximum number of entries (0 = unlimited, not recommended).
func NewRecentUpdateCache(ttl time.Duration, maxEntries int) *RecentUpdateCache {
	capacity := maxEntries
	if capacity == 0 {
		capacity = 1000 // Default capacity for hint
	}
	return &RecentUpdateCache{
		entries:    make(map[uint64]*cacheEntry, capacity),
		ttl:        ttl,
		maxEntries: maxEntries,
	}
}

// Add inserts an update into the cache.
// Triggers lazy cleanup of expired entries.
// Drops the new entry if cache is full after eviction.
func (c *RecentUpdateCache) Add(update *ReceivedUpdate) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	// Lazy cleanup: evict expired entries on each Add
	// BGP keepalives ensure regular Add activity
	for id, e := range c.entries {
		if now.After(e.expiresAt) {
			delete(c.entries, id)
		}
	}

	// Fixed size: drop new entry if still at capacity after eviction
	if c.maxEntries > 0 && len(c.entries) >= c.maxEntries {
		return
	}

	c.entries[update.WireUpdate.MessageID()] = &cacheEntry{
		update:    update,
		expiresAt: now.Add(c.ttl),
	}
}

// Get retrieves an update by ID.
// Returns (nil, false) if not found or expired.
func (c *RecentUpdateCache) Get(id uint64) (*ReceivedUpdate, bool) {
	c.mu.RLock()
	entry, ok := c.entries[id]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	// Check expiry
	if time.Now().After(entry.expiresAt) {
		return nil, false
	}

	return entry.update, true
}

// Delete removes an update from the cache.
// Called after forward completes or when controller acks without forwarding.
// Returns true if the entry was found and deleted.
func (c *RecentUpdateCache) Delete(id uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.entries[id]; ok {
		delete(c.entries, id)
		return true
	}
	return false
}

// ResetTTL extends the expiration time for an entry.
// Called on API commands that reference the update-id.
// Returns true if the entry was found and TTL reset.
func (c *RecentUpdateCache) ResetTTL(id uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.entries[id]; ok {
		e.expiresAt = time.Now().Add(c.ttl)
		return true
	}
	return false
}

// Len returns the current number of entries (including expired but not yet cleaned).
// Used primarily for testing.
func (c *RecentUpdateCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Count only non-expired entries for accuracy
	now := time.Now()
	count := 0
	for _, e := range c.entries {
		if !now.After(e.expiresAt) {
			count++
		}
	}
	return count
}
