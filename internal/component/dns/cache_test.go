package dns

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCacheHit verifies cached entries are returned without network query.
//
// VALIDATES: AC-8 -- same query repeated within cache TTL returns cached result.
// PREVENTS: Cache miss on repeated identical queries.
func TestCacheHit(t *testing.T) {
	c := newCache(100, 3600)

	c.put("example.com", 1, []string{"1.2.3.4"}, 300)
	records, ok := c.get("example.com", 1)

	require.True(t, ok, "cache should return a hit for stored entry")
	assert.Equal(t, []string{"1.2.3.4"}, records)
}

// TestCacheMiss verifies cache miss returns false.
//
// VALIDATES: AC-8 -- cache miss triggers network query (no cached result).
// PREVENTS: False cache hits on unseen queries.
func TestCacheMiss(t *testing.T) {
	c := newCache(100, 3600)

	_, ok := c.get("nonexistent.com", 1)

	assert.False(t, ok, "cache should return miss for absent entry")
}

// TestCacheEviction verifies LRU eviction when cache is at capacity.
//
// VALIDATES: AC-9 -- cache at capacity evicts oldest entries.
// PREVENTS: Unbounded cache growth.
func TestCacheEviction(t *testing.T) {
	c := newCache(2, 3600)

	c.put("first.com", 1, []string{"1.1.1.1"}, 300)
	c.put("second.com", 1, []string{"2.2.2.2"}, 300)
	// Access first to make second the LRU.
	c.get("first.com", 1)
	// Third entry should evict second (LRU).
	c.put("third.com", 1, []string{"3.3.3.3"}, 300)

	_, okFirst := c.get("first.com", 1)
	_, okSecond := c.get("second.com", 1)
	_, okThird := c.get("third.com", 1)

	assert.True(t, okFirst, "first entry should survive (recently accessed)")
	assert.False(t, okSecond, "second entry should be evicted (LRU)")
	assert.True(t, okThird, "third entry should be present")
}

// TestCacheTTL verifies entries expire based on configured TTL.
//
// VALIDATES: AC-10 -- cache entries expire at configured TTL.
// PREVENTS: Stale DNS entries served indefinitely.
func TestCacheTTL(t *testing.T) {
	// Use a very short max TTL so the entry expires quickly.
	c := newCache(100, 1)

	c.put("example.com", 1, []string{"1.2.3.4"}, 1)

	// Entry should be present immediately.
	records, ok := c.get("example.com", 1)
	require.True(t, ok)
	assert.Equal(t, []string{"1.2.3.4"}, records)

	// Wait for expiry.
	time.Sleep(1100 * time.Millisecond)

	_, ok = c.get("example.com", 1)
	assert.False(t, ok, "entry should have expired after TTL")
}

// TestCacheTTLCappedByConfig verifies response TTL is capped by configured max TTL.
//
// VALIDATES: AC-10 -- cache entries expire at configured TTL (capped by response TTL).
// PREVENTS: Response with very high TTL living longer than configured maximum.
func TestCacheTTLCappedByConfig(t *testing.T) {
	// Max TTL = 1 second, but response TTL is 3600.
	c := newCache(100, 1)

	c.put("example.com", 1, []string{"1.2.3.4"}, 3600)

	// Wait for config max TTL to expire.
	time.Sleep(1100 * time.Millisecond)

	_, ok := c.get("example.com", 1)
	assert.False(t, ok, "entry should have expired at config max TTL, not response TTL")
}

// TestCacheDisabled verifies cache-size 0 disables caching.
//
// VALIDATES: AC-11 -- cache-size 0 disables caching.
// PREVENTS: Caching when user explicitly disables it.
func TestCacheDisabled(t *testing.T) {
	c := newCache(0, 3600)

	c.put("example.com", 1, []string{"1.2.3.4"}, 300)
	_, ok := c.get("example.com", 1)

	assert.False(t, ok, "cache should always miss when size is 0")
}

// TestCacheDifferentTypes verifies entries are keyed by name+type.
//
// VALIDATES: AC-3, AC-4 -- different record types are cached separately.
// PREVENTS: TXT query returning A record data.
func TestCacheDifferentTypes(t *testing.T) {
	c := newCache(100, 3600)

	c.put("example.com", 1, []string{"1.2.3.4"}, 300)     // A
	c.put("example.com", 16, []string{"v=spf1 ..."}, 300) // TXT

	aRecords, okA := c.get("example.com", 1)
	txtRecords, okTXT := c.get("example.com", 16)

	require.True(t, okA)
	require.True(t, okTXT)
	assert.Equal(t, []string{"1.2.3.4"}, aRecords)
	assert.Equal(t, []string{"v=spf1 ..."}, txtRecords)
}

// TestCacheTTLZeroNotStored verifies TTL=0 entries are not cached per RFC 1035.
//
// VALIDATES: AC-10 -- TTL=0 from DNS server means "do not cache."
// PREVENTS: Caching records the server says should not be cached.
func TestCacheTTLZeroNotStored(t *testing.T) {
	c := newCache(100, 0)

	c.put("example.com", 1, []string{"1.2.3.4"}, 0)
	_, ok := c.get("example.com", 1)

	assert.False(t, ok, "TTL=0 entry should not be stored in cache")
}

// TestCacheMaxTTLZeroUsesResponseTTL verifies maxTTL=0 does not cap response TTL.
//
// VALIDATES: AC-10 -- maxTTL=0 means use response TTL as-is.
// PREVENTS: maxTTL=0 accidentally zeroing out effective TTL.
func TestCacheMaxTTLZeroUsesResponseTTL(t *testing.T) {
	c := newCache(100, 0)

	c.put("example.com", 1, []string{"1.2.3.4"}, 300)
	records, ok := c.get("example.com", 1)

	require.True(t, ok, "entry with response TTL=300 should be cached when maxTTL=0")
	assert.Equal(t, []string{"1.2.3.4"}, records)
}

// TestCacheOverwrite verifies putting the same key twice replaces the old value.
//
// VALIDATES: AC-8 -- cache stores latest result for repeated queries.
// PREVENTS: Stale data persisting after a newer result arrives.
func TestCacheOverwrite(t *testing.T) {
	c := newCache(100, 3600)

	c.put("example.com", 1, []string{"1.1.1.1"}, 300)
	c.put("example.com", 1, []string{"2.2.2.2"}, 300)

	records, ok := c.get("example.com", 1)
	require.True(t, ok)
	assert.Equal(t, []string{"2.2.2.2"}, records, "should return updated value")

	// Verify no duplicate entries: cache should have exactly 1 entry.
	c.mu.Lock()
	count := len(c.entries)
	c.mu.Unlock()
	assert.Equal(t, 1, count, "overwrite should not create duplicate entries")
}

// TestCacheConcurrent verifies cache is safe for concurrent access.
//
// VALIDATES: cache "safe for concurrent use" contract.
// PREVENTS: Data races on concurrent get/put.
func TestCacheConcurrent(t *testing.T) {
	c := newCache(100, 3600)

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := "domain" + string(rune('0'+n)) + ".com"
			c.put(key, 1, []string{"1.2.3.4"}, 300)
			c.get(key, 1)
			c.put(key, 1, []string{"5.6.7.8"}, 300)
			c.get(key, 1)
		}(i)
	}
	wg.Wait()
}
