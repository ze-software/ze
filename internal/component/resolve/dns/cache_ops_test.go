package dns

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/test/sim"
)

func TestCacheClear(t *testing.T) {
	c := newCache(100, 3600)
	c.put("a.com", 1, []string{"1.1.1.1"}, 300)
	c.put("b.com", 1, []string{"2.2.2.2"}, 300)
	c.get("a.com", 1)
	c.get("missing.com", 1)

	c.Clear()

	stats := c.Stats()
	assert.Equal(t, 0, stats.Entries)
	assert.Equal(t, uint64(0), stats.Hits)
	assert.Equal(t, uint64(0), stats.Misses)
	assert.Equal(t, uint64(0), stats.Evictions)
	assert.Equal(t, uint64(0), stats.Expired)
	assert.Equal(t, uint32(100), stats.Capacity, "capacity preserved after clear")

	_, ok := c.get("a.com", 1)
	assert.False(t, ok, "entries gone after clear")
}

func TestCacheDelete(t *testing.T) {
	c := newCache(100, 3600)
	c.put("a.com", 1, []string{"1.1.1.1"}, 300)
	c.put("a.com", 28, []string{"::1"}, 300)
	c.put("b.com", 1, []string{"2.2.2.2"}, 300)

	found := c.Delete("a.com", 1)
	assert.True(t, found)

	_, ok := c.get("a.com", 1)
	assert.False(t, ok, "deleted entry should be gone")

	_, ok = c.get("a.com", 28)
	assert.True(t, ok, "other type for same name should survive")

	_, ok = c.get("b.com", 1)
	assert.True(t, ok, "other name should survive")

	found = c.Delete("nonexistent.com", 1)
	assert.False(t, found, "deleting absent entry returns false")
}

func TestCacheDeleteByName(t *testing.T) {
	c := newCache(100, 3600)
	c.put("a.com", 1, []string{"1.1.1.1"}, 300)
	c.put("a.com", 28, []string{"::1"}, 300)
	c.put("b.com", 1, []string{"2.2.2.2"}, 300)

	removed := c.DeleteByName("a.com")
	assert.Equal(t, 2, removed, "should remove both A and AAAA entries")

	_, ok := c.get("a.com", 1)
	assert.False(t, ok)
	_, ok = c.get("a.com", 28)
	assert.False(t, ok)

	_, ok = c.get("b.com", 1)
	assert.True(t, ok, "other name should survive")

	removed = c.DeleteByName("nonexistent.com")
	assert.Equal(t, 0, removed)
}

func TestCacheResetStats(t *testing.T) {
	c := newCache(100, 3600)
	c.put("a.com", 1, []string{"1.1.1.1"}, 300)
	c.get("a.com", 1)
	c.get("missing.com", 1)

	c.ResetStats()

	stats := c.Stats()
	assert.Equal(t, 1, stats.Entries, "entries preserved after stats reset")
	assert.Equal(t, uint64(0), stats.Hits)
	assert.Equal(t, uint64(0), stats.Misses)

	_, ok := c.get("a.com", 1)
	assert.True(t, ok, "entries still accessible after stats reset")
}

func TestCacheEntriesSortedByTTL(t *testing.T) {
	c := newCache(100, 3600)
	c.put("short.com", 1, []string{"1.1.1.1"}, 60)
	c.put("long.com", 1, []string{"2.2.2.2"}, 600)
	c.put("medium.com", 28, []string{"::1", "::2"}, 300)

	entries := c.Entries()
	require.Len(t, entries, 3)

	assert.Equal(t, "short.com", entries[0].Name)
	assert.Equal(t, "medium.com", entries[1].Name)
	assert.Equal(t, "long.com", entries[2].Name)

	assert.Equal(t, uint16(1), entries[0].Type)
	assert.Equal(t, []string{"1.1.1.1"}, entries[0].Records)
	assert.Greater(t, entries[0].TTLSeconds, 0)

	assert.Equal(t, uint16(28), entries[1].Type)
	assert.Equal(t, []string{"::1", "::2"}, entries[1].Records)
}

func TestCacheEntriesExcludesExpired(t *testing.T) {
	c := newCache(100, 0)
	clk := sim.NewFakeClock(time.Now())
	c.clk = clk
	c.put("live.com", 1, []string{"1.1.1.1"}, 300)
	c.put("dying.com", 1, []string{"2.2.2.2"}, 1)

	clk.Add(1100 * time.Millisecond)

	entries := c.Entries()
	require.Len(t, entries, 1, "expired entry should be excluded")
	assert.Equal(t, "live.com", entries[0].Name)
}

func TestCacheEntriesEmpty(t *testing.T) {
	c := newCache(100, 3600)
	entries := c.Entries()
	assert.Empty(t, entries)
}

func TestCacheEntriesDisabled(t *testing.T) {
	c := newCache(0, 3600)
	entries := c.Entries()
	assert.Empty(t, entries)
}

func TestCacheClearConcurrent(t *testing.T) {
	c := newCache(100, 3600)
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := "domain" + string(rune('a'+n)) + ".com"
			c.put(key, 1, []string{"1.2.3.4"}, 300)
			c.get(key, 1)
			if n%3 == 0 {
				c.Clear()
			}
			if n%5 == 0 {
				c.Delete(key, 1)
			}
			if n%4 == 0 {
				c.DeleteByName(key)
			}
			c.Entries()
			c.ResetStats()
		}(i)
	}
	wg.Wait()
}
