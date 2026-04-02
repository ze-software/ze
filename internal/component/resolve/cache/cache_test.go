package cache

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: shared TTL cache stores and retrieves values.
// PREVENTS: cache miss on stored entry.
func TestTTLCache_StoreAndHit(t *testing.T) {
	c := New[string](time.Hour)

	c.Set("key1", "value1")
	val, ok := c.Get("key1")

	require.True(t, ok, "cache should return hit for stored entry")
	assert.Equal(t, "value1", val)
}

// VALIDATES: cache returns miss for absent key.
// PREVENTS: false hits on unseen keys.
func TestTTLCache_Miss(t *testing.T) {
	c := New[string](time.Hour)

	_, ok := c.Get("absent")

	assert.False(t, ok, "cache should return miss for absent entry")
}

// VALIDATES: entries expire after TTL.
// PREVENTS: stale entries served indefinitely.
func TestTTLCache_Expiry(t *testing.T) {
	c := New[string](50 * time.Millisecond)

	c.Set("key1", "value1")

	// Should be present immediately.
	val, ok := c.Get("key1")
	require.True(t, ok)
	assert.Equal(t, "value1", val)

	// Wait for expiry.
	time.Sleep(100 * time.Millisecond)

	_, ok = c.Get("key1")
	assert.False(t, ok, "entry should have expired after TTL")
}

// VALIDATES: cache is safe for concurrent access.
// PREVENTS: data races on concurrent get/set.
func TestTTLCache_Concurrent(t *testing.T) {
	c := New[int](time.Hour)

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			c.Set("key", n)
			c.Get("key")
			c.Set("key", n+1)
			c.Get("key")
		}(i)
	}
	wg.Wait()
}

// VALIDATES: overwriting a key updates the value and resets TTL.
// PREVENTS: stale data after update.
func TestTTLCache_Overwrite(t *testing.T) {
	c := New[string](time.Hour)

	c.Set("key1", "old")
	c.Set("key1", "new")

	val, ok := c.Get("key1")
	require.True(t, ok)
	assert.Equal(t, "new", val, "should return updated value")
}

// VALIDATES: different keys are independent.
// PREVENTS: key collision.
func TestTTLCache_DifferentKeys(t *testing.T) {
	c := New[string](time.Hour)

	c.Set("a", "alpha")
	c.Set("b", "beta")

	va, okA := c.Get("a")
	vb, okB := c.Get("b")

	require.True(t, okA)
	require.True(t, okB)
	assert.Equal(t, "alpha", va)
	assert.Equal(t, "beta", vb)
}

// VALIDATES: Len returns the number of unexpired entries.
// PREVENTS: wrong count from expired or overwritten entries.
func TestTTLCache_Len(t *testing.T) {
	c := New[string](time.Hour)

	assert.Equal(t, 0, c.Len())

	c.Set("a", "1")
	c.Set("b", "2")
	assert.Equal(t, 2, c.Len())

	// Overwrite should not increase count.
	c.Set("a", "1-updated")
	assert.Equal(t, 2, c.Len())
}
