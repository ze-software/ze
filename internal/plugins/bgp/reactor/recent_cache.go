package reactor

import (
	"errors"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/sim"
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
	clock      sim.Clock
	entries    map[uint64]*cacheEntry
	ttl        time.Duration
	maxEntries int
}

// cacheEntry wraps an update with expiration time and retention state.
type cacheEntry struct {
	update    *ReceivedUpdate
	expiresAt time.Time
	retained  bool // If true, entry survives TTL expiry until explicitly released
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
		clock:      sim.RealClock{},
		entries:    make(map[uint64]*cacheEntry, capacity),
		ttl:        ttl,
		maxEntries: maxEntries,
	}
}

// SetClock sets the clock used for TTL calculations.
func (c *RecentUpdateCache) SetClock(clk sim.Clock) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clock = clk
}

// Add inserts an update into the cache.
// Triggers lazy cleanup of expired entries (returning their buffers to pool).
// Returns true if accepted, false if rejected (cache full).
// IMPORTANT: Does NOT return buffer when rejected - caller still needs it for handleUpdate().
// Caller must check return value and handle buffer accordingly.
func (c *RecentUpdateCache) Add(update *ReceivedUpdate) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.clock.Now()

	// Lazy cleanup: evict expired entries on each Add
	// BGP keepalives ensure regular Add activity
	// Skip retained entries - they survive until explicitly released
	for id, e := range c.entries {
		if !e.retained && now.After(e.expiresAt) {
			// Return buffer to pool before deleting
			ReturnReadBuffer(e.update.poolBuf)
			delete(c.entries, id)
		}
	}

	// Fixed size: reject new entry if still at capacity after eviction
	// DON'T return buffer here - caller still needs it for handleUpdate()
	if c.maxEntries > 0 && len(c.entries) >= c.maxEntries {
		return false // rejected, caller handles buffer
	}

	c.entries[update.WireUpdate.MessageID()] = &cacheEntry{
		update:    update,
		expiresAt: now.Add(c.ttl),
	}
	return true // accepted, cache owns buffer
}

// Take retrieves and removes an update by ID, transferring ownership to caller.
// Caller MUST call ReceivedUpdate.Release() when done to return buffer to pool.
// Returns (nil, false) if not found or expired (unless retained).
func (c *RecentUpdateCache) Take(id uint64) (*ReceivedUpdate, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[id]
	if !ok {
		return nil, false
	}

	// Check expiry (retained entries never expire)
	if !entry.retained && c.clock.Now().After(entry.expiresAt) {
		// Expired - return buffer and remove
		ReturnReadBuffer(entry.update.poolBuf)
		delete(c.entries, id)
		return nil, false
	}

	// Remove from cache - caller now owns the buffer
	delete(c.entries, id)
	return entry.update, true
}

// Contains checks if an entry exists and is valid (not expired or retained).
// Does NOT remove the entry or transfer ownership.
// Used primarily for testing cache state.
func (c *RecentUpdateCache) Contains(id uint64) bool {
	c.mu.RLock()
	entry, ok := c.entries[id]
	c.mu.RUnlock()

	if !ok {
		return false
	}
	// Retained entries are always valid; others must not be expired
	return entry.retained || !c.clock.Now().After(entry.expiresAt)
}

// Delete removes an update from the cache and returns its buffer to pool.
// Called after forward completes or when controller acks without forwarding.
// Returns true if the entry was found and deleted.
func (c *RecentUpdateCache) Delete(id uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.entries[id]; ok {
		// Return buffer to pool before deleting
		ReturnReadBuffer(e.update.poolBuf)
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
		e.expiresAt = c.clock.Now().Add(c.ttl)
		return true
	}
	return false
}

// Len returns the current number of valid entries.
// Counts retained entries and non-expired entries.
// Used primarily for testing.
func (c *RecentUpdateCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Count retained entries and non-expired entries
	now := c.clock.Now()
	count := 0
	for _, e := range c.entries {
		if e.retained || !now.After(e.expiresAt) {
			count++
		}
	}
	return count
}

// Retain marks an entry to survive TTL expiry until explicitly released.
// Used by API for graceful restart - retain routes for replay.
// Returns true if entry found (valid or expired), false if not found.
func (c *RecentUpdateCache) Retain(id uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.entries[id]; ok {
		e.retained = true
		return true
	}
	return false
}

// Release clears the retained flag and resets TTL.
// Entry will expire normally after TTL elapses.
// Returns true if entry found, false if not found.
func (c *RecentUpdateCache) Release(id uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.entries[id]; ok {
		e.retained = false
		e.expiresAt = c.clock.Now().Add(c.ttl)
		return true
	}
	return false
}

// List returns all valid msg-ids (retained or non-expired).
// Used by API to show cached updates.
func (c *RecentUpdateCache) List() []uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := c.clock.Now()
	var ids []uint64
	for id, e := range c.entries {
		if e.retained || !now.After(e.expiresAt) {
			ids = append(ids, id)
		}
	}
	return ids
}
