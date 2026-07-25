// Design: (none -- new component, predates documentation)
// Related: resolver.go -- DNS resolver uses cache for query results

// Package dns provides a DNS resolver component for Ze.
package dns

import (
	"container/list"
	"sort"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/clock"
)

// cacheKey identifies a cached DNS query by name and record type.
type cacheKey struct {
	name  string
	qtype uint16
}

// cacheEntry holds a cached DNS result with expiry time.
type cacheEntry struct {
	key     cacheKey
	records []string
	expires time.Time
	element *list.Element // Position in LRU list for O(1) removal/touch.
}

// CacheStats holds DNS cache hit/miss/eviction counters.
type CacheStats struct {
	Entries   int    `json:"entries"`
	Capacity  uint32 `json:"capacity"`
	Hits      uint64 `json:"hits"`
	Misses    uint64 `json:"misses"`
	Evictions uint64 `json:"evictions"`
	Expired   uint64 `json:"expired"`
}

// CacheEntryInfo describes a single cached DNS entry for display.
type CacheEntryInfo struct {
	Name       string   `json:"name"`
	Type       uint16   `json:"type"`
	TypeName   string   `json:"type-name"`
	Records    []string `json:"records"`
	TTLSeconds int      `json:"ttl-seconds"`
}

// cache is an in-memory DNS cache with TTL-based expiry and LRU eviction.
// Safe for concurrent use.
type cache struct {
	mu        sync.Mutex
	maxSize   uint32
	maxTTL    uint32 // Seconds. 0 means use response TTL only.
	entries   map[cacheKey]*cacheEntry
	lru       *list.List  // Front = oldest (evict first), Back = newest.
	clk       clock.Clock // Time source for expiry. Defaults to RealClock.
	hits      uint64
	misses    uint64
	evictions uint64
	expired   uint64
}

// newCache creates a DNS cache. maxSize=0 disables caching.
// maxTTL caps entry lifetime in seconds (0 means no cap).
func newCache(maxSize, maxTTL uint32) *cache {
	return &cache{
		maxSize: maxSize,
		maxTTL:  maxTTL,
		entries: make(map[cacheKey]*cacheEntry),
		lru:     list.New(),
		clk:     clock.RealClock{},
	}
}

// get looks up a cached result. Returns records and true on hit, nil and false on miss.
// Expired entries are evicted on access.
func (c *cache) get(name string, qtype uint16) ([]string, bool) {
	records, _, ok := c.getWithTTL(name, qtype)
	return records, ok
}

// getWithTTL looks up a cached result and returns the remaining TTL in seconds.
// Returns (nil, 0, false) on miss.
func (c *cache) getWithTTL(name string, qtype uint16) ([]string, uint32, bool) {
	if c.maxSize == 0 {
		return nil, 0, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	key := cacheKey{name: name, qtype: qtype}
	entry, ok := c.entries[key]
	if !ok {
		c.misses++
		return nil, 0, false
	}

	now := c.clk.Now()
	if now.After(entry.expires) {
		c.removeLocked(entry)
		c.expired++
		c.misses++
		return nil, 0, false
	}

	c.hits++

	// Move to back of LRU list (most recently used).
	c.lru.MoveToBack(entry.element)

	remaining := uint32(entry.expires.Sub(now).Seconds())

	// Return a copy to prevent caller mutation.
	result := make([]string, len(entry.records))
	copy(result, entry.records)
	return result, remaining, true
}

// put stores a DNS result in the cache. responseTTL is the TTL from the DNS response
// in seconds. The effective TTL is min(responseTTL, maxTTL) when maxTTL > 0.
// A responseTTL of 0 means "do not cache" per RFC 1035; the entry is not stored.
func (c *cache) put(name string, qtype uint16, records []string, responseTTL uint32) {
	if c.maxSize == 0 {
		return
	}

	ttl := responseTTL
	if c.maxTTL > 0 && ttl > c.maxTTL {
		ttl = c.maxTTL
	}

	// TTL=0 means the DNS server says "do not cache." Respect that.
	if ttl == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	key := cacheKey{name: name, qtype: qtype}

	// Update existing entry.
	if existing, exists := c.entries[key]; exists {
		c.removeLocked(existing)
	}

	// Evict LRU if at capacity.
	for uint32(len(c.entries)) >= c.maxSize {
		front := c.lru.Front()
		if front == nil {
			break
		}
		entry, ok := front.Value.(*cacheEntry)
		if !ok {
			c.lru.Remove(front)
			break
		}
		c.removeLocked(entry)
		c.evictions++
	}

	stored := make([]string, len(records))
	copy(stored, records)

	entry := &cacheEntry{
		key:     key,
		records: stored,
		expires: c.clk.Now().Add(time.Duration(ttl) * time.Second),
	}
	entry.element = c.lru.PushBack(entry)
	c.entries[key] = entry
}

// removeLocked removes an entry from both the map and LRU list. Caller MUST hold c.mu.
func (c *cache) removeLocked(entry *cacheEntry) {
	delete(c.entries, entry.key)
	c.lru.Remove(entry.element)
}

// Stats returns a snapshot of cache counters. Safe for concurrent use.
func (c *cache) Stats() CacheStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return CacheStats{
		Entries:   len(c.entries),
		Capacity:  c.maxSize,
		Hits:      c.hits,
		Misses:    c.misses,
		Evictions: c.evictions,
		Expired:   c.expired,
	}
}

// Clear removes all entries and resets all counters. Safe for concurrent use.
func (c *cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[cacheKey]*cacheEntry)
	c.lru.Init()
	c.hits = 0
	c.misses = 0
	c.evictions = 0
	c.expired = 0
}

// Delete removes a single entry by name and record type.
// Returns true if the entry existed and was removed.
func (c *cache) Delete(name string, qtype uint16) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[cacheKey{name: name, qtype: qtype}]
	if !ok {
		return false
	}
	c.removeLocked(entry)
	return true
}

// DeleteByName removes all entries matching the given name regardless of record type.
// Returns the number of entries removed.
func (c *cache) DeleteByName(name string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	var removed int
	for key, entry := range c.entries {
		if key.name == name {
			c.removeLocked(entry)
			removed++
		}
	}
	return removed
}

// ResetStats zeros all counters without removing cached entries.
func (c *cache) ResetStats() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hits = 0
	c.misses = 0
	c.evictions = 0
	c.expired = 0
}

// Entries returns a snapshot of all non-expired cached entries sorted by
// remaining TTL ascending (entries closest to expiry first).
func (c *cache) Entries() []CacheEntryInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.clk.Now()
	out := make([]CacheEntryInfo, 0, len(c.entries))
	for _, entry := range c.entries {
		ttl := int(entry.expires.Sub(now).Seconds())
		if ttl <= 0 {
			continue
		}
		records := make([]string, len(entry.records))
		copy(records, entry.records)
		out = append(out, CacheEntryInfo{
			Name:       entry.key.name,
			Type:       entry.key.qtype,
			Records:    records,
			TTLSeconds: ttl,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TTLSeconds < out[j].TTLSeconds })
	return out
}
