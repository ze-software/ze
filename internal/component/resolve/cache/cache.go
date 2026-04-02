// Design: docs/architecture/resolve.md -- Shared TTL cache for resolution components
//
// Package cache provides a generic TTL cache used by Cymru, PeeringDB, and IRR
// resolvers. DNS keeps its own TTL-from-response cache and does not use this package.
//
// Safe for concurrent use.
package cache

import (
	"sync"
	"time"
)

// entry holds a cached value with its expiry time.
type entry[V any] struct {
	value   V
	expires time.Time
}

// Cache is a generic TTL cache. All entries share the same TTL configured
// at construction time. Safe for concurrent use.
type Cache[V any] struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]entry[V]
}

// New creates a TTL cache where every entry lives for the given duration.
func New[V any](ttl time.Duration) *Cache[V] {
	return &Cache[V]{
		ttl:     ttl,
		entries: make(map[string]entry[V]),
	}
}

// Get looks up a cached value. Returns the value and true on hit,
// or the zero value and false on miss or expiry.
// Expired entries are evicted on access.
func (c *Cache[V]) Get(key string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.entries[key]
	if !ok {
		var zero V
		return zero, false
	}

	if time.Now().After(e.expires) {
		delete(c.entries, key)
		var zero V
		return zero, false
	}

	return e.value, true
}

// Set stores a value in the cache with the configured TTL.
func (c *Cache[V]) Set(key string, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = entry[V]{
		value:   value,
		expires: time.Now().Add(c.ttl),
	}
}

// Len returns the number of entries in the cache (including expired but not yet evicted).
func (c *Cache[V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.entries)
}
