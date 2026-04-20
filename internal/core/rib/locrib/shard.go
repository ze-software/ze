// Design: plan/learned/639-rib-unified.md -- Phase 4 (sharded Loc-RIB)
// Related: manager.go -- RIB delegates per-prefix mutations to familyShards
// Related: change.go -- subscriberList replicated per shard

package locrib

import (
	"hash/maphash"
	"net/netip"
	"runtime"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/rib/store"
)

// Shard count bounds. Below 1 makes no sense; above 64 is wasted memory on
// any realistic deployment (one writer per core saturates the trie cost).
const (
	minShards = 1
	maxShards = 64
)

var _ = env.MustRegister(env.EnvEntry{
	Key:         "ze.rib.shards",
	Type:        "int",
	Description: "Number of shards per address family in the unified Loc-RIB. Defaults to GOMAXPROCS, clamped to [1, 64].",
})

// prefixHashSeed seeds maphash.Comparable so the same prefix always hashes
// to the same shard within one process. Different processes may use different
// seeds; that is intentional (best-effort load balancing, no protocol meaning).
var prefixHashSeed = maphash.MakeSeed()

// shardCount returns the per-family shard count, honoring ze.rib.shards
// and clamping to [minShards, maxShards].
func shardCount() int {
	n := env.GetInt("ze.rib.shards", runtime.GOMAXPROCS(0))
	if n < minShards {
		return minShards
	}
	if n > maxShards {
		return maxShards
	}
	return n
}

// shardIndex picks the shard owning prefix p in a familyShards of size n.
func shardIndex(p netip.Prefix, n int) int {
	if n <= 1 {
		return 0
	}
	h := maphash.Comparable(prefixHashSeed, p)
	return int(h % uint64(n))
}

// shard wraps one prefix-keyed Store with its own write lock and subscriber
// list. Writers contend only with other writers on the same shard; readers
// (Lookup, Best, Iterate) take the shard read lock.
type shard struct {
	mu    sync.RWMutex
	store *store.Store[PathGroup]
	subs  subscriberList
}

// familyShards holds the N shards for one address family. The shard slice is
// allocated once at construction; element addresses are stable, so callers
// safely retain *shard pointers across the lifetime of the familyShards.
type familyShards struct {
	fam    family.Family
	shards []shard
}

// newFamilyShards constructs n shards for fam, seeding each shard's
// subscriber list from subs (the RIB's current subscriber template). Caller
// MUST hold whatever lock serializes subscriber-template mutations so the
// seed snapshot is consistent.
func newFamilyShards(fam family.Family, n int, subs []subEntry) *familyShards {
	if n < minShards {
		n = minShards
	}
	fs := &familyShards{fam: fam, shards: make([]shard, n)}
	for i := range fs.shards {
		fs.shards[i].store = store.NewStore[PathGroup](fam)
		if len(subs) > 0 {
			fs.shards[i].subs.replace(subs)
		}
	}
	return fs
}

// shardFor returns the shard owning prefix p.
func (fs *familyShards) shardFor(p netip.Prefix) *shard {
	return &fs.shards[shardIndex(p, len(fs.shards))]
}

// Len returns the total number of prefixes across every shard. Takes each
// shard's read lock in turn.
func (fs *familyShards) Len() int {
	total := 0
	for i := range fs.shards {
		fs.shards[i].mu.RLock()
		total += fs.shards[i].store.Len()
		fs.shards[i].mu.RUnlock()
	}
	return total
}
