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
// can remain in the cache after being passed over (a later entry is fully acked).
// Detects crashed or stuck plugins without penalizing slow-but-correct processing.
const safetyValveDuration = 5 * time.Minute

// gapScanInterval controls how often the gap-based safety valve scan runs.
// The scan only matters for fault detection — normal eviction is immediate via Ack().
const gapScanInterval = 30 * time.Second

// warnInterval controls how often the soft-limit warning is logged.
const warnInterval = 30 * time.Second

// RecentUpdateCache stores recently received UPDATEs for efficient forwarding.
// Thread-safe for concurrent Add/Get/Ack operations.
//
// Design (Phase 2 — mandatory consumer acknowledgment):
//   - Count-based tracking: entries track pending consumer count (not names)
//   - Mandatory ack: every cache-consumer plugin MUST forward or release each update
//   - FIFO ordering: plugin acks must follow receive order (implicit cumulative ack)
//   - Immediate eviction: when all consumers ack, entry evicted in O(1)
//   - Gap-based safety valve: entries passed over (later entry fully acked) are
//     force-evicted after safetyValveDuration (crashed plugin detection)
//   - Soft max-entries: warns when exceeding limit but never rejects
//   - No TTL: entries never expire based on time alone
//
// Lifecycle:
//  1. Add() inserts entry with pending=true, pendingConsumers=0
//  2. Activate(id, count) sets pendingConsumers from delivery count, clears pending
//  3. Each consumer calls Get() to read, then Ack(id, plugin) when done
//  4. When all consumers ack: entry evicted immediately, buffer returned to pool
//  5. Gap scan (every 30s) force-evicts stalled entries (crashed plugin protection)
type RecentUpdateCache struct {
	mu           sync.RWMutex
	clock        sim.Clock
	entries      map[uint64]*cacheEntry
	maxEntries   int       // Soft limit — warns but never rejects
	lastWarnTime time.Time // Rate-limits soft-limit warnings
	lastGapScan  time.Time // When last gap safety valve scan ran

	// Per-plugin FIFO tracking: last acked message ID per plugin.
	// Acks for IDs <= this value are silently accepted as no-ops
	// (already handled by a previous cumulative ack).
	// Initialized by RegisterConsumer() to highestAddedID at registration time.
	pluginLastAck map[string]uint64

	// Highest message ID that was fully acked and evicted.
	// Used for gap detection: if an older entry still has consumers but
	// highestFullyAcked is higher, the entry has been "passed over."
	highestFullyAcked uint64

	// Highest message ID ever passed to Add().
	// Used by RegisterConsumer() to initialize pluginLastAck so that
	// implicit acks never touch pre-registration entries.
	highestAddedID uint64
}

// cacheEntry wraps an update with consumer tracking.
type cacheEntry struct {
	update           *ReceivedUpdate
	pending          bool      // True between Add() and Activate(); not evictable
	retainedAt       time.Time // When consumers first became > 0 (for safety valve)
	pendingConsumers int       // How many plugin acks are still needed
	earlyAckCount    int       // Acks received before Activate() (fast plugin race)
	retainCount      int32     // API-level retains (separate from plugin consumers)
}

// totalConsumers returns the total number of active consumers (plugin + API retain).
func (e *cacheEntry) totalConsumers() int {
	return e.pendingConsumers + int(e.retainCount)
}

// isGapEvictable reports whether this entry should be force-evicted by the safety valve.
// An entry is gap-evictable when:
//  1. Not pending (activation complete)
//  2. Has consumers (otherwise it would already be evicted)
//  3. A later entry has been fully acked (gap detected)
//  4. The entry has been retained longer than safetyValveDuration
//
// Entries at the processing frontier (no later entry fully acked) are never
// timed out — they represent slow-but-correct processing.
func (e *cacheEntry) isGapEvictable(now time.Time, entryID, highestFullyAcked uint64) bool {
	if e.pending {
		return false
	}
	if e.totalConsumers() <= 0 {
		return false // Zero consumers — would be evicted normally
	}
	if highestFullyAcked <= entryID {
		return false // No later entry fully acked — at the frontier
	}
	if e.retainedAt.IsZero() {
		return false
	}
	return now.Sub(e.retainedAt) > safetyValveDuration
}

// NewRecentUpdateCache creates a cache with the given soft max size.
// maxEntries: soft limit — warns when exceeded but never rejects (0 = unlimited).
func NewRecentUpdateCache(maxEntries int) *RecentUpdateCache {
	capacity := maxEntries
	if capacity == 0 {
		capacity = 1000 // Default capacity for hint
	}
	return &RecentUpdateCache{
		clock:         sim.RealClock{},
		entries:       make(map[uint64]*cacheEntry, capacity),
		maxEntries:    maxEntries,
		pluginLastAck: make(map[string]uint64),
	}
}

// SetClock sets the clock used for safety valve calculations.
func (c *RecentUpdateCache) SetClock(clk sim.Clock) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clock = clk
}

// Add inserts an update into the cache with pending=true and zero consumers.
// The entry is not evictable until Activate() is called.
// Triggers gap-based safety valve scan at most once per gapScanInterval.
// Always succeeds — soft limit logs a rate-limited warning but never rejects.
// Caller must call Activate(id, count) after dispatching to subscribed plugins.
func (c *RecentUpdateCache) Add(update *ReceivedUpdate) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.clock.Now()

	// Gap-based safety valve scan: evict stalled entries (infrequent — every 30s).
	// Only needed for fault detection; normal eviction is immediate via Ack().
	if now.Sub(c.lastGapScan) >= gapScanInterval {
		c.lastGapScan = now
		for id, e := range c.entries {
			if e.isGapEvictable(now, id, c.highestFullyAcked) {
				cacheLogger().Warn("safety valve: force-evicting stalled entry",
					"id", id, "consumers", e.pendingConsumers,
					"retained-for", now.Sub(e.retainedAt))
				ReturnReadBuffer(e.update.poolBuf)
				delete(c.entries, id)
			}
		}
	}

	// Soft limit: warn but never reject (rate-limited to avoid log flood)
	if c.maxEntries > 0 && len(c.entries) >= c.maxEntries {
		if now.Sub(c.lastWarnTime) >= warnInterval {
			c.lastWarnTime = now
			cacheLogger().Warn("cache exceeding soft limit",
				"current", len(c.entries), "limit", c.maxEntries)
		}
	}

	msgID := update.WireUpdate.MessageID()
	c.entries[msgID] = &cacheEntry{
		update:  update,
		pending: true, // Protected until Activate()
	}
	if msgID > c.highestAddedID {
		c.highestAddedID = msgID
	}
}

// Activate sets the consumer count and clears the pending flag.
// count is the number of cache-consumer plugins that successfully received the event.
// Called after dispatching the event to subscribed plugins.
// Early acks (from fast plugins that acked before Activate) are subtracted:
// pendingConsumers = count - earlyAckCount.
// If no consumers remain, entry is evicted immediately.
func (c *RecentUpdateCache) Activate(id uint64, count int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.entries[id]
	if !ok {
		return
	}

	e.pending = false

	// Set consumer count, subtracting early acks (fast plugin race).
	// Clamp to zero: earlyAckCount can exceed count if a plugin acked
	// and was then unregistered before Activate (both increment earlyAckCount).
	e.pendingConsumers = max(0, count-e.earlyAckCount)
	e.earlyAckCount = 0 // Clear early acks after applying

	if e.totalConsumers() > 0 {
		e.retainedAt = c.clock.Now()
	} else {
		// No consumers left — immediate eviction
		c.evictLocked(id, e)
	}
}

// Get retrieves an update by ID without removing it from the cache.
// Returns (nil, false) if not found. All entries in the map are valid
// (no TTL — entries are evicted only by Ack or safety valve).
// Thread-safe for concurrent access — multiple consumers can Get() simultaneously.
func (c *RecentUpdateCache) Get(id uint64) (*ReceivedUpdate, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[id]
	if !ok {
		return nil, false
	}
	return entry.update, true
}

// Ack records that a plugin has processed (forwarded or dropped) an update.
// When id > lastAck: cumulative ack — implicitly acks all entries between
// the plugin's last ack and this id (TCP-like).
// When id <= lastAck: no-op — this plugin already acked the entry via a
// previous cumulative ack. This happens when multi-peer delivery causes
// session goroutines to compete for callMu, making delivery order differ
// from message ID order.
// Returns ErrUpdateExpired if the target id is not in the cache.
func (c *RecentUpdateCache) Ack(id uint64, plugin string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	lastAck := c.pluginLastAck[plugin]

	if id <= lastAck {
		// Already acked via cumulative ack from a previous forward ack.
		// Multi-peer concurrent delivery can cause events to arrive in
		// non-ID-order, so this is expected — silently accept as no-op.
		return nil
	}

	// Check that the target entry exists
	e, ok := c.entries[id]
	if !ok {
		return ErrUpdateExpired
	}

	// Forward ack: implicit cumulative ack for entries between lastAck+1 and id.
	// Acking N means "I've handled everything up to N."
	for intermediateID := lastAck + 1; intermediateID < id; intermediateID++ {
		if ie, exists := c.entries[intermediateID]; exists {
			c.ackEntryLocked(intermediateID, ie)
		}
	}

	// Ack the target entry
	c.ackEntryLocked(id, e)

	// Update last ack for this plugin
	c.pluginLastAck[plugin] = id

	return nil
}

// ackEntryLocked decrements the entry's pending consumer count.
// If the entry is pending (fast plugin race), the early ack count is incremented
// to be applied when Activate() is called.
// If all consumers have acked, the entry is evicted immediately.
// Must be called with c.mu held.
func (c *RecentUpdateCache) ackEntryLocked(id uint64, e *cacheEntry) {
	if e.pending {
		// Fast plugin race: ack arrived before Activate
		e.earlyAckCount++
		return
	}

	if e.pendingConsumers > 0 {
		e.pendingConsumers--
	}

	if e.totalConsumers() <= 0 {
		c.evictLocked(id, e)
	}
}

// evictLocked removes an entry from the cache and returns its buffer to the pool.
// Updates highestFullyAcked for gap detection.
// Must be called with c.mu held.
func (c *RecentUpdateCache) evictLocked(id uint64, e *cacheEntry) {
	ReturnReadBuffer(e.update.poolBuf)
	delete(c.entries, id)
	if id > c.highestFullyAcked {
		c.highestFullyAcked = id
	}
}

// Decrement decreases the retain count by 1.
// Used by Release() for API-level retain/release commands.
// When total consumers (plugin + retain) reaches 0 and entry is not pending,
// the entry is evicted immediately.
// Safe to call before Activate() — retainCount goes negative, corrected by Retain().
// Returns true if entry found, false if not found.
func (c *RecentUpdateCache) Decrement(id uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.entries[id]
	if !ok {
		return false
	}

	e.retainCount--

	if e.totalConsumers() <= 0 && !e.pending {
		c.evictLocked(id, e)
	}

	return true
}

// Contains checks if an entry exists in the cache.
// All entries in the map are valid (no TTL).
// Does NOT remove the entry or transfer ownership.
func (c *RecentUpdateCache) Contains(id uint64) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.entries[id]
	return ok
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

// Retain increments the retain count by 1.
// Used by the `bgp cache N retain` API command.
// Returns true if entry found, false if not found.
func (c *RecentUpdateCache) Retain(id uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.entries[id]; ok {
		e.retainCount++
		if e.retainCount == 1 && e.pendingConsumers == 0 && e.retainedAt.IsZero() {
			e.retainedAt = c.clock.Now()
		}
		return true
	}
	return false
}

// Release decrements the retain count.
// Used by the `bgp cache N release` API command.
// Returns true if entry found, false if not found.
func (c *RecentUpdateCache) Release(id uint64) bool {
	return c.Decrement(id)
}

// Len returns the current number of entries in the cache.
// All entries are valid (no TTL filtering).
func (c *RecentUpdateCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// List returns all cached msg-ids.
// Used by API to show cached updates.
func (c *RecentUpdateCache) List() []uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ids := make([]uint64, 0, len(c.entries))
	for id := range c.entries {
		ids = append(ids, id)
	}
	return ids
}

// RegisterConsumer initializes FIFO tracking for a new cache-consumer plugin.
// Sets pluginLastAck to highestAddedID so that implicit acks from this plugin
// never touch entries that existed before registration.
// Called when a plugin declares cache-consumer: true during Stage 1 registration.
func (c *RecentUpdateCache) RegisterConsumer(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.pluginLastAck[name]; exists {
		cacheLogger().Error("BUG: duplicate RegisterConsumer — plugin registered twice without unregister",
			"plugin", name)
		return
	}
	c.pluginLastAck[name] = c.highestAddedID
}

// UnregisterConsumer removes a cache-consumer plugin and adjusts pending counts.
// Walks all entries with id > pluginLastAck[name] and decrements their
// pendingConsumers (the plugin will never ack these). Entries that reach
// zero total consumers are evicted immediately.
// Called when a cache-consumer plugin disconnects or is removed.
func (c *RecentUpdateCache) UnregisterConsumer(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	lastAck, ok := c.pluginLastAck[name]
	if !ok {
		return
	}
	delete(c.pluginLastAck, name)

	for id, e := range c.entries {
		if id <= lastAck {
			continue // Plugin already acked this entry
		}
		if e.pending {
			// Entry not yet activated — record as early ack so Activate()
			// applies a reduced consumer count (prevents stuck entries
			// when a plugin disconnects between delivery and activation).
			e.earlyAckCount++
			continue
		}
		if e.pendingConsumers > 0 {
			e.pendingConsumers--
		}
		if e.totalConsumers() <= 0 {
			c.evictLocked(id, e)
		}
	}
}
