// Design: plan/spec-host-0-inventory.md -- cached inventory with TTL
// Related: inventory.go -- Detector and Inventory types

package host

import (
	"sync"
	"time"
)

// CachedDetector wraps a Detector with a time-based cache. Calls to
// Detect return the cached Inventory if it is within the configured
// TTL; otherwise the underlying Detector runs a fresh detection.
//
// CachedDetector is safe for concurrent use. There are no background
// goroutines; refresh is lazy (triggered by the next Detect call
// after the TTL expires).
type CachedDetector struct {
	detector *Detector
	ttl      time.Duration

	mu       sync.RWMutex
	cached   *Inventory
	cachedAt time.Time
}

// NewCachedDetector creates a CachedDetector that caches the result
// of d.Detect() for the given TTL. A zero or negative TTL disables
// caching (every call runs detection).
func NewCachedDetector(d *Detector, ttl time.Duration) *CachedDetector {
	return &CachedDetector{
		detector: d,
		ttl:      ttl,
	}
}

// Detect returns the cached Inventory if it was obtained less than
// TTL ago. Otherwise it runs a fresh detection, caches the result,
// and returns it. On detection error the cache is not updated and
// any previously cached value is left intact for the next call.
func (c *CachedDetector) Detect() (*Inventory, error) {
	c.mu.RLock()
	if c.cached != nil && time.Since(c.cachedAt) < c.ttl {
		inv := c.cached
		c.mu.RUnlock()
		return inv, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check: another goroutine may have refreshed while we
	// waited for the write lock.
	if c.cached != nil && time.Since(c.cachedAt) < c.ttl {
		return c.cached, nil
	}

	inv, err := c.detector.Detect()
	if err != nil {
		return nil, err
	}
	c.cached = inv
	c.cachedAt = time.Now()
	return inv, nil
}

// Invalidate clears the cached value so the next Detect call will
// run a fresh detection regardless of TTL.
func (c *CachedDetector) Invalidate() {
	c.mu.Lock()
	c.cached = nil
	c.cachedAt = time.Time{}
	c.mu.Unlock()
}
