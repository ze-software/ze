// Design: (none -- new component, predates documentation)
// Related: resolver.go -- DNS resolver uses cache for query results

// Package dns provides a DNS resolver component for Ze.
package dns

import (
	"fmt"
	"sync"
	"time"
)

// cacheEntry holds a cached DNS result with expiry time.
type cacheEntry struct {
	records []string
	expires time.Time
	key     string // For LRU eviction tracking.
}

// cache is an in-memory DNS cache with TTL-based expiry and LRU eviction.
// Safe for concurrent use.
type cache struct {
	mu      sync.Mutex
	maxSize uint32
	maxTTL  uint32 // Seconds. 0 means use response TTL only.
	entries map[string]*cacheEntry
	order   []string // LRU order: oldest first.
}

// newCache creates a DNS cache. maxSize=0 disables caching.
// maxTTL caps entry lifetime in seconds (0 means no cap).
func newCache(maxSize, maxTTL uint32) *cache {
	return &cache{
		maxSize: maxSize,
		maxTTL:  maxTTL,
		entries: make(map[string]*cacheEntry),
	}
}

// cacheKey builds a lookup key from domain name and record type.
func cacheKey(name string, qtype uint16) string {
	return fmt.Sprintf("%s:%d", name, qtype)
}

// get looks up a cached result. Returns records and true on hit, nil and false on miss.
// Expired entries are evicted on access.
func (c *cache) get(name string, qtype uint16) ([]string, bool) {
	if c.maxSize == 0 {
		return nil, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	key := cacheKey(name, qtype)
	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}

	if time.Now().After(entry.expires) {
		c.removeLocked(key)
		return nil, false
	}

	// Move to end of LRU order (most recently used).
	c.touchLocked(key)

	// Return a copy to prevent caller mutation.
	result := make([]string, len(entry.records))
	copy(result, entry.records)
	return result, true
}

// put stores a DNS result in the cache. responseTTL is the TTL from the DNS response
// in seconds. The effective TTL is min(responseTTL, maxTTL) when maxTTL > 0.
func (c *cache) put(name string, qtype uint16, records []string, responseTTL uint32) {
	if c.maxSize == 0 {
		return
	}

	ttl := responseTTL
	if c.maxTTL > 0 && ttl > c.maxTTL {
		ttl = c.maxTTL
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	key := cacheKey(name, qtype)

	// Update existing entry.
	if _, exists := c.entries[key]; exists {
		c.removeLocked(key)
	}

	// Evict LRU if at capacity.
	for uint32(len(c.entries)) >= c.maxSize {
		if len(c.order) == 0 {
			break
		}
		c.removeLocked(c.order[0])
	}

	stored := make([]string, len(records))
	copy(stored, records)

	c.entries[key] = &cacheEntry{
		records: stored,
		expires: time.Now().Add(time.Duration(ttl) * time.Second),
		key:     key,
	}
	c.order = append(c.order, key)
}

// removeLocked removes an entry by key. Caller MUST hold c.mu.
func (c *cache) removeLocked(key string) {
	delete(c.entries, key)
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}

// touchLocked moves a key to the end of the LRU order. Caller MUST hold c.mu.
func (c *cache) touchLocked(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			c.order = append(c.order, key)
			break
		}
	}
}
