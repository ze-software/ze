package reactor

import (
	"errors"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/sim"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// ErrUpdateExpired is returned when an update-id is expired or not found.
var ErrUpdateExpired = errors.New("update-id expired or not found")

// cacheLogger returns the lazy logger for cache warnings.
var cacheLogger = slogutil.LazyLogger("bgp.reactor.cache")

// safetyValveDuration is the maximum time an entry with consumers > 0
// can remain in the cache. After this, it is force-evicted (safety valve
// for crashed plugins that never decrement).
const safetyValveDuration = 5 * time.Minute

// RecentUpdateCache stores recently received UPDATEs for efficient forwarding.
// Thread-safe for concurrent Add/Get/Decrement operations.
//
// Design:
//   - Reference-counted: entries track active consumers (plugins processing the update)
//   - Time-based TTL: entries with zero consumers expire after configured duration
//   - Lazy cleanup: expired entries evicted on Add(), no background goroutine
//   - Soft max-entries: warns when exceeding limit but never rejects (prevents update loss)
//   - Safety valve: entries retained > 5 min are force-evicted (crashed plugin protection)
//
// Lifecycle:
//  1. Add() inserts entry with pending=true, consumers=0
//  2. Activate(id, N) sets consumers=N, clears pending
//  3. Each consumer calls Get() to read, then Decrement() when done
//  4. When consumers reaches 0: entry becomes TTL-evictable
//  5. Lazy cleanup in Add() returns buffer to pool and removes entry
//
// Performance: O(n) scan on each Add(). Keep maxEntries reasonable (1000-10000).
type RecentUpdateCache struct {
	mu         sync.RWMutex
	clock      sim.Clock
	entries    map[uint64]*cacheEntry
	ttl        time.Duration
	maxEntries int // Soft limit — warns but never rejects
}

// cacheEntry wraps an update with expiration time and consumer tracking.
type cacheEntry struct {
	update     *ReceivedUpdate
	expiresAt  time.Time
	consumers  int32     // Active consumers; entry not evictable when > 0
	pending    bool      // True between Add() and Activate(); not evictable
	retainedAt time.Time // When consumers first became > 0 (for safety valve)
}

// isProtected reports whether this entry is protected from eviction.
// Protected entries have active consumers, are pending activation,
// or have not exceeded the safety valve duration.
func (e *cacheEntry) isProtected(now time.Time) bool {
	if e.pending {
		return true
	}
	if e.consumers <= 0 {
		return false
	}
	// Safety valve: force-evict entries retained too long (crashed plugin)
	if !e.retainedAt.IsZero() && now.Sub(e.retainedAt) > safetyValveDuration {
		return false
	}
	return true
}

// NewRecentUpdateCache creates a cache with the given TTL and soft max size.
// ttl: how long entries with zero consumers remain valid (0 = immediate expiry)
// maxEntries: soft limit — warns when exceeded but never rejects (0 = unlimited).
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

// Add inserts an update into the cache with pending=true and consumers=0.
// The entry is not evictable until Activate() is called.
// Triggers lazy cleanup of expired entries (returning their buffers to pool).
// Always succeeds — soft limit logs a warning but never rejects.
// Caller must call Activate(id, N) after dispatching to N consumers.
func (c *RecentUpdateCache) Add(update *ReceivedUpdate) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.clock.Now()

	// Lazy cleanup: evict expired entries on each Add
	// Skip protected entries (consumers > 0 or pending)
	for id, e := range c.entries {
		if !e.isProtected(now) && now.After(e.expiresAt) {
			// Log safety valve evictions (consumers > 0 means a plugin never decremented)
			if e.consumers > 0 {
				cacheLogger().Warn("safety valve: force-evicting retained entry",
					"id", id, "consumers", e.consumers,
					"retained-for", now.Sub(e.retainedAt))
			}
			ReturnReadBuffer(e.update.poolBuf)
			delete(c.entries, id)
		}
	}

	// Soft limit: warn but never reject
	if c.maxEntries > 0 && len(c.entries) >= c.maxEntries {
		cacheLogger().Warn("cache exceeding soft limit",
			"current", len(c.entries), "limit", c.maxEntries)
	}

	c.entries[update.WireUpdate.MessageID()] = &cacheEntry{
		update:    update,
		expiresAt: now.Add(c.ttl),
		pending:   true, // Protected until Activate()
	}
}

// Activate sets the consumer count and clears the pending flag.
// Called after dispatching the event to N subscribed plugins.
// If Decrement() was called before Activate() (fast plugin race),
// consumers may be negative; Activate adds N to the current count,
// yielding the correct net value.
// If N == 0 (no subscribers), the entry becomes normally TTL-evictable.
func (c *RecentUpdateCache) Activate(id uint64, consumers int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.entries[id]
	if !ok {
		return
	}

	e.consumers += int32(consumers) //nolint:gosec // G115: consumers is a small count
	e.pending = false
	if e.consumers > 0 {
		e.retainedAt = c.clock.Now()
	}
}

// Get retrieves an update by ID without removing it from the cache.
// Returns (nil, false) if not found or expired (with zero consumers and not pending).
// Thread-safe for concurrent access — multiple consumers can Get() simultaneously.
//
// Caller must ensure the entry remains in cache while using the returned pointer.
// For consumer-tracked entries (consumers > 0), the entry is protected from eviction
// until Decrement() is called. For zero-consumer entries accessed via API, the caller
// should Retain() before Get() or accept a narrow race with lazy cleanup in Add().
func (c *RecentUpdateCache) Get(id uint64) (*ReceivedUpdate, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[id]
	if !ok {
		return nil, false
	}

	// Protected entries (pending or consumers > 0) are always accessible
	if entry.isProtected(c.clock.Now()) {
		return entry.update, true
	}

	// Non-protected: check TTL expiry
	if c.clock.Now().After(entry.expiresAt) {
		return nil, false
	}

	return entry.update, true
}

// Decrement decreases the consumer count by 1.
// When consumers reaches 0: entry becomes TTL-evictable (normal lazy cleanup).
// If the entry is already expired when consumers hits 0, it is cleaned up immediately.
// Safe to call before Activate() — consumers goes negative, corrected by Activate(N).
// Returns true if entry found, false if not found.
func (c *RecentUpdateCache) Decrement(id uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.entries[id]
	if !ok {
		return false
	}

	e.consumers--

	// If consumers dropped to 0 (or below, during race) and not pending:
	// reset TTL so the entry gets a fresh expiration window
	if e.consumers <= 0 && !e.pending {
		e.retainedAt = time.Time{} // Clear safety valve timer
		e.expiresAt = c.clock.Now().Add(c.ttl)

		// If TTL is zero, clean up immediately
		if c.ttl == 0 {
			ReturnReadBuffer(e.update.poolBuf)
			delete(c.entries, id)
		}
	}

	return true
}

// Contains checks if an entry exists and is valid.
// Protected entries (pending, consumers > 0) are always valid.
// Non-protected entries must not be expired.
// Does NOT remove the entry or transfer ownership.
func (c *RecentUpdateCache) Contains(id uint64) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[id]
	if !ok {
		return false
	}

	now := c.clock.Now()
	if entry.isProtected(now) {
		return true
	}
	return !now.After(entry.expiresAt)
}

// Delete removes an update from the cache and returns its buffer to pool.
// Returns true if the entry was found and deleted.
func (c *RecentUpdateCache) Delete(id uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.entries[id]; ok {
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
// Counts protected entries and non-expired entries.
func (c *RecentUpdateCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := c.clock.Now()
	count := 0
	for _, e := range c.entries {
		if e.isProtected(now) || !now.After(e.expiresAt) {
			count++
		}
	}
	return count
}

// Retain increments the consumer count by 1.
// Used by the `bgp cache N retain` API command.
// Returns true if entry found, false if not found.
func (c *RecentUpdateCache) Retain(id uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.entries[id]; ok {
		e.consumers++
		if e.consumers == 1 {
			e.retainedAt = c.clock.Now()
		}
		return true
	}
	return false
}

// Release decrements the consumer count and resets TTL.
// Used by the `bgp cache N release` API command.
// Returns true if entry found, false if not found.
func (c *RecentUpdateCache) Release(id uint64) bool {
	return c.Decrement(id)
}

// List returns all valid msg-ids (protected or non-expired).
// Used by API to show cached updates.
func (c *RecentUpdateCache) List() []uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := c.clock.Now()
	var ids []uint64
	for id, e := range c.entries {
		if e.isProtected(now) || !now.After(e.expiresAt) {
			ids = append(ids, id)
		}
	}
	return ids
}
