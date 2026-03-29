// Design: (none -- new component, predates documentation)
// Related: resolver.go -- DNS resolver uses cache for query results

// Package dns provides a DNS resolver component for Ze.
package dns

import (
	"container/list"
	"sync"
	"time"
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

// cache is an in-memory DNS cache with TTL-based expiry and LRU eviction.
// Safe for concurrent use.
type cache struct {
	mu      sync.Mutex
	maxSize uint32
	maxTTL  uint32 // Seconds. 0 means use response TTL only.
	entries map[cacheKey]*cacheEntry
	lru     *list.List // Front = oldest (evict first), Back = newest.
}

// newCache creates a DNS cache. maxSize=0 disables caching.
// maxTTL caps entry lifetime in seconds (0 means no cap).
func newCache(maxSize, maxTTL uint32) *cache {
	return &cache{
		maxSize: maxSize,
		maxTTL:  maxTTL,
		entries: make(map[cacheKey]*cacheEntry),
		lru:     list.New(),
	}
}

// get looks up a cached result. Returns records and true on hit, nil and false on miss.
// Expired entries are evicted on access.
func (c *cache) get(name string, qtype uint16) ([]string, bool) {
	if c.maxSize == 0 {
		return nil, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	key := cacheKey{name: name, qtype: qtype}
	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}

	if time.Now().After(entry.expires) {
		c.removeLocked(entry)
		return nil, false
	}

	// Move to back of LRU list (most recently used).
	c.lru.MoveToBack(entry.element)

	// Return a copy to prevent caller mutation.
	result := make([]string, len(entry.records))
	copy(result, entry.records)
	return result, true
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
	}

	stored := make([]string, len(records))
	copy(stored, records)

	entry := &cacheEntry{
		key:     key,
		records: stored,
		expires: time.Now().Add(time.Duration(ttl) * time.Second),
	}
	entry.element = c.lru.PushBack(entry)
	c.entries[key] = entry
}

// removeLocked removes an entry from both the map and LRU list. Caller MUST hold c.mu.
func (c *cache) removeLocked(entry *cacheEntry) {
	delete(c.entries, entry.key)
	c.lru.Remove(entry.element)
}
