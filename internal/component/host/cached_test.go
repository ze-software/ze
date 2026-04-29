package host

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: CachedDetector returns a cached result within TTL and
// re-detects after TTL expires.
// PREVENTS: cache never refreshing, or refreshing on every call.
func TestCachedDetector_TTL(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	cd := NewCachedDetector(d, 200*time.Millisecond)

	// First call populates the cache.
	inv1, err := cd.Detect()
	require.NoError(t, err)
	require.NotNil(t, inv1)

	// Second call within TTL returns the same pointer (cached).
	inv2, err := cd.Detect()
	require.NoError(t, err)
	assert.Same(t, inv1, inv2, "expected same pointer from cache within TTL")

	// Wait for TTL to expire.
	time.Sleep(250 * time.Millisecond)

	// Third call after TTL returns a fresh Inventory (different pointer).
	inv3, err := cd.Detect()
	require.NoError(t, err)
	require.NotNil(t, inv3)
	assert.NotSame(t, inv1, inv3, "expected fresh Inventory after TTL expiry")
}

// VALIDATES: Invalidate forces the next Detect to re-run detection
// even if TTL has not expired.
// PREVENTS: Invalidate being a no-op or leaving stale data.
func TestCachedDetector_Invalidate(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	cd := NewCachedDetector(d, 10*time.Second)

	inv1, err := cd.Detect()
	require.NoError(t, err)
	require.NotNil(t, inv1)

	cd.Invalidate()

	inv2, err := cd.Detect()
	require.NoError(t, err)
	require.NotNil(t, inv2)
	assert.NotSame(t, inv1, inv2, "expected new pointer after Invalidate")
}

// VALIDATES: concurrent Detect calls do not race (run under -race).
// PREVENTS: data race on mu/cached/cachedAt fields.
func TestCachedDetector_Concurrent(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	cd := NewCachedDetector(d, 50*time.Millisecond)

	const n = 32
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			inv, err := cd.Detect()
			if err != nil {
				t.Errorf("concurrent Detect: %v", err)
				return
			}
			if inv == nil {
				t.Error("concurrent Detect returned nil")
			}
		}()
	}
	wg.Wait()
}

// VALIDATES: NewCachedDetector with zero TTL disables caching (every
// call runs detection and returns a distinct pointer).
// PREVENTS: zero TTL causing permanent caching or panic.
func TestCachedDetector_ZeroTTL(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	cd := NewCachedDetector(d, 0)

	inv1, err := cd.Detect()
	require.NoError(t, err)
	require.NotNil(t, inv1)

	inv2, err := cd.Detect()
	require.NoError(t, err)
	require.NotNil(t, inv2)
	assert.NotSame(t, inv1, inv2, "zero TTL should not cache")
}
