// Design: docs/architecture/core-design.md — recent UPDATE cache
// Overview: reactor.go — BGP reactor event loop and peer management
// Related: received_update.go — ReceivedUpdate stored in cache

package reactor

import (
	"errors"
	"runtime"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/core/seqmap"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// ErrUpdateExpired is returned when an update-id is expired or not found.
var ErrUpdateExpired = errors.New("update-id expired or not found")

// cacheLogger returns the lazy logger for cache warnings.
var cacheLogger = slogutil.LazyLogger("bgp.reactor.cache")

// defaultSafetyValveDuration is the maximum time an entry with consumers > 0
// can remain in the cache after being passed over (a later entry is fully acked).
// Detects crashed or stuck plugins without penalizing slow-but-correct processing.
// Configurable per-cache via SetSafetyValveDuration(); overridable at startup
// via the ZE_CACHE_SAFETY_VALVE environment variable.
const defaultSafetyValveDuration = 5 * time.Minute

// gapScanInterval controls how often the background gap-based safety valve scan runs.
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
//   - Gap-based safety valve: background goroutine force-evicts stalled entries
//     after safetyValveDuration (crashed plugin detection)
//   - Soft max-entries: warns when exceeding limit but never rejects
//   - No TTL: entries never expire based on time alone
//
// Lifecycle:
//  1. Start() launches background gap scan goroutine
//  2. Add() inserts entry with pending=true, pendingConsumers=0
//  3. Activate(id, count) sets pendingConsumers from delivery count, clears pending
//  4. Each consumer calls Get() to read, then Ack(id, plugin) when done
//  5. When all consumers ack: entry evicted immediately, buffer returned to pool
//  6. Background goroutine force-evicts stalled entries (crashed plugin protection)
//  7. Stop() shuts down background goroutine
type RecentUpdateCache struct {
	mu           sync.RWMutex
	clock        clock.Clock
	entries      *seqmap.Map[uint64, *cacheEntry]
	maxEntries   int           // Soft limit — warns but never rejects
	safetyValve  time.Duration // Per-cache safety valve duration
	lastWarnTime time.Time     // Rate-limits soft-limit warnings

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

	// nonFIFOConsumers tracks consumers that process entries out of global
	// message ID order (e.g., per-source-peer workers in bgp-rs).
	// For these consumers, Ack() uses per-entry semantics: no cumulative
	// ack loop, and id <= lastAck acks are not skipped.
	nonFIFOConsumers map[string]bool

	// Background gap scan lifecycle.
	stopCh       chan struct{}
	closeOnce    sync.Once
	scanInterval time.Duration // Defaults to gapScanInterval; overridable for tests
}

// SetConsumerUnordered marks a registered consumer as non-FIFO (unordered).
// Unordered consumers use per-entry acking: no cumulative ack loop,
// and id <= lastAck acks are not skipped. This is required for consumers
// like bgp-rs that process entries out of global message ID order
// (e.g., per-source-peer worker pools).
func (c *RecentUpdateCache) SetConsumerUnordered(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.nonFIFOConsumers == nil {
		c.nonFIFOConsumers = make(map[string]bool)
	}
	c.nonFIFOConsumers[name] = true
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
func (e *cacheEntry) isGapEvictable(now time.Time, entryID, highestFullyAcked uint64, valve time.Duration) bool {
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
	return now.Sub(e.retainedAt) > valve
}

// NewRecentUpdateCache creates a cache with the given soft max size.
// maxEntries: soft limit — warns when exceeded but never rejects (0 = unlimited).
// Call Start() to launch the background gap scan goroutine.
func NewRecentUpdateCache(maxEntries int) *RecentUpdateCache {
	return &RecentUpdateCache{
		clock:         clock.RealClock{},
		entries:       seqmap.New[uint64, *cacheEntry](),
		maxEntries:    maxEntries,
		safetyValve:   defaultSafetyValveDuration,
		pluginLastAck: make(map[string]uint64),
	}
}

// Start launches the background gap scan goroutine.
// Must be called before the cache receives traffic. Safe to omit in tests
// that don't need background scanning (Stop is still safe to call).
func (c *RecentUpdateCache) Start() {
	c.stopCh = make(chan struct{})
	c.closeOnce = sync.Once{}
	interval := c.scanInterval
	if interval == 0 {
		interval = gapScanInterval
	}
	ticker := c.clock.NewTicker(interval)
	go c.gapScanLoop(ticker)
}

// Stop shuts down the background gap scan goroutine.
// Idempotent — safe to call multiple times or without calling Start().
func (c *RecentUpdateCache) Stop() {
	if c.stopCh == nil {
		return
	}
	c.closeOnce.Do(func() {
		close(c.stopCh)
	})
}

// SetGapScanInterval overrides the gap scan ticker interval.
// Must be called before Start(). Intended for tests that need fast ticking.
func (c *RecentUpdateCache) SetGapScanInterval(d time.Duration) {
	c.scanInterval = d
}

// gapScanLoop runs the background gap scan on a ticker.
// Exits when stopCh is closed.
func (c *RecentUpdateCache) gapScanLoop(ticker clock.Ticker) {
	defer ticker.Stop()
	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C():
			c.safeRunGapScan()
		}
	}
}

// safeRunGapScan wraps runGapScan with panic recovery so that a panic
// in the scan doesn't kill the background maintenance loop.
func (c *RecentUpdateCache) safeRunGapScan() {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			reactorLogger().Error("gap scan panic recovered",
				"panic", r,
				"stack", string(buf[:n]),
			)
		}
	}()
	c.runGapScan()
}

// runGapScan executes the gap-based safety valve scan under write lock.
// Evicts entries that have been passed over (a later entry is fully acked)
// and have exceeded the safety valve duration.
func (c *RecentUpdateCache) runGapScan() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.clock.Now()

	// Collect entries to evict (cannot modify seqmap during Range iteration).
	var toEvict []uint64
	c.entries.Range(func(id uint64, _ uint64, e *cacheEntry) bool {
		if e.isGapEvictable(now, id, c.highestFullyAcked, c.safetyValve) {
			toEvict = append(toEvict, id)
		}
		return true
	})

	for _, id := range toEvict {
		e, ok := c.entries.Get(id)
		if !ok {
			continue
		}
		cacheLogger().Warn("safety valve: force-evicting stalled entry",
			"id", id, "consumers", e.pendingConsumers,
			"retained-for", now.Sub(e.retainedAt))
		c.evictLocked(id, e)
	}
}

// SetClock sets the clock used for safety valve calculations.
func (c *RecentUpdateCache) SetClock(clk clock.Clock) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clock = clk
}

// SetSafetyValveDuration overrides the safety valve duration for gap-based eviction.
// Zero or negative values are ignored (default 5 minutes is kept).
func (c *RecentUpdateCache) SetSafetyValveDuration(d time.Duration) {
	if d <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.safetyValve = d
}

// Add inserts an update into the cache with pending=true and zero consumers.
// The entry is not evictable until Activate() is called.
// Always succeeds — soft limit logs a rate-limited warning but never rejects.
// Caller must call Activate(id, count) after dispatching to subscribed plugins.
// Gap scan runs in a background goroutine (Start), not inline here.
func (c *RecentUpdateCache) Add(update *ReceivedUpdate) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.clock.Now()

	// Soft limit: warn but never reject (rate-limited to avoid log flood)
	if c.maxEntries > 0 && c.entries.Len() >= c.maxEntries {
		if now.Sub(c.lastWarnTime) >= warnInterval {
			c.lastWarnTime = now
			cacheLogger().Warn("cache exceeding soft limit",
				"current", c.entries.Len(), "limit", c.maxEntries)
		}
	}

	msgID := update.WireUpdate.MessageID()
	c.entries.Put(msgID, msgID, &cacheEntry{
		update:  update,
		pending: true, // Protected until Activate()
	})
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

	e, ok := c.entries.Get(id)
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

	entry, ok := c.entries.Get(id)
	if !ok {
		return nil, false
	}
	return entry.update, true
}

// Ack records that a plugin has processed (forwarded or dropped) an update.
//
// FIFO consumers (default):
//
//	When id > lastAck: cumulative ack — implicitly acks all entries between
//	the plugin's last ack and this id via seqmap.Since() (O(log n + k) where
//	k = cached entries in range, not the ID gap).
//	When id <= lastAck: no-op — this plugin already acked the entry via a
//	previous cumulative ack. This happens when multi-peer delivery causes
//	session goroutines to compete for callMu, making delivery order differ
//	from message ID order.
//
// Unordered consumers (SetConsumerUnordered):
//
//	Per-entry ack only. No cumulative loop, no id <= lastAck skip.
//	Required for consumers like bgp-rs that process entries out of global
//	message ID order (per-source-peer workers).
//
// Returns ErrUpdateExpired if the target id is not in the cache.
func (c *RecentUpdateCache) Ack(id uint64, plugin string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Unordered consumer: per-entry ack only.
	// No cumulative loop — acking a high ID must NOT evict lower entries
	// that other workers haven't processed yet.
	// No id <= lastAck skip — workers may ack entries below lastAck.
	if c.nonFIFOConsumers[plugin] {
		e, ok := c.entries.Get(id)
		if !ok {
			return ErrUpdateExpired
		}
		c.ackEntryLocked(id, e)
		if id > c.pluginLastAck[plugin] {
			c.pluginLastAck[plugin] = id
		}
		return nil
	}

	// FIFO consumer: cumulative ack (existing behavior).
	lastAck := c.pluginLastAck[plugin]

	if id <= lastAck {
		// Already acked via cumulative ack from a previous forward ack.
		// Multi-peer concurrent delivery can cause events to arrive in
		// non-ID-order, so this is expected — silently accept as no-op.
		return nil
	}

	// Check that the target entry exists
	e, ok := c.entries.Get(id)
	if !ok {
		return ErrUpdateExpired
	}

	// Forward ack: implicit cumulative ack for entries between lastAck+1 and id.
	// Acking N means "I've handled everything up to N."
	// Uses seqmap.Since() to iterate only cached entries in range — O(log n + k)
	// where k is the number of cached entries, not the ID gap size.
	// Collect-then-ack: cannot modify seqmap during Since iteration (compaction).
	type ackRef struct {
		id    uint64
		entry *cacheEntry
	}
	var intermediates []ackRef
	c.entries.Since(lastAck+1, func(key uint64, _ uint64, ie *cacheEntry) bool {
		if key >= id {
			return false // Stop before target (handled separately below)
		}
		intermediates = append(intermediates, ackRef{key, ie})
		return true
	})
	for _, ref := range intermediates {
		c.ackEntryLocked(ref.id, ref.entry)
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
// Returns all pool buffers: original read buffer and any EBGP patched versions.
// Updates highestFullyAcked for gap detection.
// Must be called with c.mu held.
func (c *RecentUpdateCache) evictLocked(id uint64, e *cacheEntry) {
	ReturnReadBuffer(e.update.poolBuf)
	ReturnReadBuffer(e.update.ebgpPoolBuf4)
	ReturnReadBuffer(e.update.ebgpPoolBuf2)
	c.entries.Delete(id)
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

	e, ok := c.entries.Get(id)
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
	_, ok := c.entries.Get(id)
	return ok
}

// Delete removes an update from the cache and returns its buffer to pool.
// Returns true if the entry was found and deleted.
func (c *RecentUpdateCache) Delete(id uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.entries.Get(id); ok {
		ReturnReadBuffer(e.update.poolBuf)
		ReturnReadBuffer(e.update.ebgpPoolBuf4)
		ReturnReadBuffer(e.update.ebgpPoolBuf2)
		c.entries.Delete(id)
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

	if e, ok := c.entries.Get(id); ok {
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
	return c.entries.Len()
}

// List returns all cached msg-ids.
// Used by API to show cached updates.
func (c *RecentUpdateCache) List() []uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ids := make([]uint64, 0, c.entries.Len())
	c.entries.Range(func(id uint64, _ uint64, _ *cacheEntry) bool {
		ids = append(ids, id)
		return true
	})
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
// For FIFO consumers: uses seqmap.Since(lastAck+1) to walk only entries above
// lastAck (entries below were already acked via cumulative ack).
// For unordered consumers: uses seqmap.Range to walk ALL entries (cannot use
// lastAck as a skip marker because entries below lastAck may not have been
// individually acked).
// Entries that reach zero total consumers are evicted immediately.
// Called when a cache-consumer plugin disconnects or is removed.
func (c *RecentUpdateCache) UnregisterConsumer(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	lastAck, ok := c.pluginLastAck[name]
	if !ok {
		return
	}
	delete(c.pluginLastAck, name)
	isUnordered := c.nonFIFOConsumers[name]
	delete(c.nonFIFOConsumers, name)

	// Collect entries to process (cannot modify seqmap during iteration).
	type entryRef struct {
		id    uint64
		entry *cacheEntry
	}
	var refs []entryRef
	if isUnordered {
		c.entries.Range(func(id uint64, _ uint64, e *cacheEntry) bool {
			refs = append(refs, entryRef{id, e})
			return true
		})
	} else {
		c.entries.Since(lastAck+1, func(id uint64, _ uint64, e *cacheEntry) bool {
			refs = append(refs, entryRef{id, e})
			return true
		})
	}

	// Process collected entries
	for _, r := range refs {
		if r.entry.pending {
			// Entry not yet activated — record as early ack so Activate()
			// applies a reduced consumer count (prevents stuck entries
			// when a plugin disconnects between delivery and activation).
			r.entry.earlyAckCount++
			continue
		}
		if r.entry.pendingConsumers > 0 {
			r.entry.pendingConsumers--
		}
		if r.entry.totalConsumers() <= 0 {
			c.evictLocked(r.id, r.entry)
		}
	}
}
