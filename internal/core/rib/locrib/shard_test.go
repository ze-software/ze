package locrib

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

// TestShardCountDefaultClamps ensures the env-driven shard count stays in
// the documented [minShards, maxShards] window even when the operator sets
// extreme values.
func TestShardCountDefaultClamps(t *testing.T) {
	defer env.ResetCache()

	require.NoError(t, env.Set("ze.rib.shards", "0"))
	env.ResetCache()
	assert.Equal(t, minShards, shardCount(), "0 must clamp up to minShards")

	require.NoError(t, env.Set("ze.rib.shards", "999"))
	env.ResetCache()
	assert.Equal(t, maxShards, shardCount(), "999 must clamp down to maxShards")

	require.NoError(t, env.Set("ze.rib.shards", "8"))
	env.ResetCache()
	assert.Equal(t, 8, shardCount(), "in-range value passes through")

	require.NoError(t, env.Set("ze.rib.shards", ""))
	env.ResetCache()
}

// TestShardIndexDeterministic verifies that hashing the same prefix always
// yields the same shard index within a process.
func TestShardIndexDeterministic(t *testing.T) {
	pfx := netip.MustParsePrefix("10.20.30.0/24")
	first := shardIndex(pfx, 16)
	for range 1000 {
		assert.Equal(t, first, shardIndex(pfx, 16))
	}
}

// TestShardIndexDistribution gives a sanity check that 1024 random IPv4
// /24s spread across 16 shards in a non-pathological way: every shard
// receives at least one prefix.
func TestShardIndexDistribution(t *testing.T) {
	const n = 16
	hits := make([]int, n)
	for i := range 1024 {
		pfx := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(i >> 8), byte(i), 0}), 24)
		hits[shardIndex(pfx, n)]++
	}
	for i, h := range hits {
		assert.NotZero(t, h, "shard %d received no prefixes -- hash skewed", i)
	}
}

// TestParallelInsertNoLostWrites stresses the sharded RIB with N goroutines
// each inserting M distinct prefixes, then asserts every (prefix, source)
// landed exactly once.
//
// VALIDATES: per-shard locks remain correct under concurrent writers.
// PREVENTS: regressions that drop a write because two shards collide on
// shared state (interner, family map).
func TestParallelInsertNoLostWrites(t *testing.T) {
	const (
		goroutines = 8
		perRoutine = 256
	)
	r := NewRIB()
	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := range perRoutine {
				pfx := netip.PrefixFrom(
					netip.AddrFrom4([4]byte{10, byte(g), byte(i >> 8), byte(i)}),
					32,
				)
				r.Insert(famV4, pfx, pathBGP(uint32(g*perRoutine+i), 50))
			}
		}(g)
	}
	wg.Wait()

	got := r.Len(famV4)
	assert.Equal(t, goroutines*perRoutine, got, "every distinct prefix must land in exactly one shard")
}

// TestParallelOnChangeReceivesEveryShardWrite verifies that a single
// OnChange handler registered before the writes runs sees one Change per
// inserted prefix, regardless of which shard owned each one.
//
// VALIDATES: per-shard subscriber lists deliver the registration to every
// shard, including shards that contain prefixes mapping to them.
// PREVENTS: a regression where a subscriber registered before any insert
// only receives Changes from the first-touched shard.
func TestParallelOnChangeReceivesEveryShardWrite(t *testing.T) {
	const (
		goroutines = 4
		perRoutine = 64
	)
	r := NewRIB()

	var (
		mu      sync.Mutex
		changes int
	)
	r.OnChange(func(c Change) {
		mu.Lock()
		changes++
		mu.Unlock()
	})

	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := range perRoutine {
				pfx := netip.PrefixFrom(
					netip.AddrFrom4([4]byte{172, 16, byte(g), byte(i)}),
					32,
				)
				r.Insert(famV4, pfx, pathBGP(uint32(g*perRoutine+i), 10))
			}
		}(g)
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, goroutines*perRoutine, changes, "every shard must dispatch to the global subscriber")
}

// TestSubscriberInheritedByLazyFamily verifies that a subscriber registered
// before any Insert (i.e. before any family was created) is wired into the
// new family's shards on first Insert.
//
// VALIDATES: newFamilyShards seeds shard subscriber lists from the RIB
// subsList template.
// PREVENTS: a regression that creates familyShards with empty subscriber
// lists, silently dropping Changes for the first family ever seen.
func TestSubscriberInheritedByLazyFamily(t *testing.T) {
	r := NewRIB()
	var seen int
	r.OnChange(func(Change) { seen++ })

	r.Insert(famV4, pfx, pathBGP(1, 10))
	assert.Equal(t, 1, seen, "subscriber must fire on the first Insert that creates the family")
}

// TestUnsubscribeRemovesFromEveryShard verifies unsubscribe walks every
// shard, not just the most-recently-touched one.
func TestUnsubscribeRemovesFromEveryShard(t *testing.T) {
	r := NewRIB()
	var seen int
	unsub := r.OnChange(func(Change) { seen++ })

	// Insert into multiple shards to grow the per-shard subscriber lists.
	for i := range 16 {
		pfx := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 0, byte(i), 0}), 24)
		r.Insert(famV4, pfx, pathBGP(uint32(i), 10))
	}
	require.Equal(t, 16, seen)

	unsub()
	for i := range 16 {
		pfx := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 1, byte(i), 0}), 24)
		r.Insert(famV4, pfx, pathBGP(uint32(i+100), 10))
	}
	assert.Equal(t, 16, seen, "unsubscribe must take effect on every shard")
}
