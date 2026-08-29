// Design: docs/architecture/rib/unified-locrib.md -- sharded BGP bestPrev tracking
// Related: rib_bestchange.go -- bestPrevStore type wrapped per shard
// Related: rib.go -- RIBManager owns one bestPrevShards instance

package rib

import (
	"hash/maphash"
	"net/netip"
	"runtime"
	"sync"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/family"
)

// Shard count bounds match locrib's. Below 1 makes no sense; above 64 is
// wasted memory on any realistic deployment.
const (
	bestPrevMinShards = 1
	bestPrevMaxShards = 64
)

var _ = env.MustRegister(env.EnvEntry{
	Key:         "ze.bgp.rib.bestprev.shards",
	Type:        "int",
	Description: "Number of shards per address family in the BGP plugin best-prev store. Defaults to GOMAXPROCS, clamped to [1, 64].",
})

// bestPrevHashSeed seeds maphash.Comparable so the same prefix always hashes
// to the same shard within one process. Independent of locrib's seed -- the
// two stores are unrelated.
var bestPrevHashSeed = maphash.MakeSeed()

// bestPrevShardCount returns the per-family bestPrev shard count.
func bestPrevShardCount() int {
	n := env.GetInt("ze.bgp.rib.bestprev.shards", runtime.GOMAXPROCS(0))
	if n < bestPrevMinShards {
		return bestPrevMinShards
	}
	if n > bestPrevMaxShards {
		return bestPrevMaxShards
	}
	return n
}

// bestPrevShardIndex picks the shard owning prefix p in a family with n shards.
func bestPrevShardIndex(p netip.Prefix, n int) int {
	if n <= 1 {
		return 0
	}
	h := maphash.Comparable(bestPrevHashSeed, p)
	return int(h % uint64(n))
}

// bestPrevShard wraps one bestPrevStore with its own lock.
type bestPrevShard struct {
	mu    sync.RWMutex
	store *bestPrevStore
}

// bestPrevFamilyShards holds the N shards for one family.
type bestPrevFamilyShards struct {
	fam    family.Family
	shards []bestPrevShard
}

// newBestPrevFamilyShards constructs n shards for fam, each with its own
// bestPrevStore. Element addresses in shards are stable for the lifetime of
// the bestPrevFamilyShards.
func newBestPrevFamilyShards(fam family.Family, n int) *bestPrevFamilyShards {
	if n < bestPrevMinShards {
		n = bestPrevMinShards
	}
	fs := &bestPrevFamilyShards{fam: fam, shards: make([]bestPrevShard, n)}
	for i := range fs.shards {
		fs.shards[i].store = newBestPrevStore(fam)
	}
	return fs
}

// shardFor returns the shard owning prefix p. CIDR families only; a non-CIDR
// family has no prefix and uses shardForNLRI.
func (fs *bestPrevFamilyShards) shardFor(p netip.Prefix) *bestPrevShard {
	return &fs.shards[bestPrevShardIndex(p, len(fs.shards))]
}

// shardForNLRI returns the shard owning the route named by nlriBytes. It is
// the non-CIDR counterpart of shardFor: those families key their store on the
// wire bytes, so the bytes are what the shard index has to hash. The seed is
// the same one shardFor uses, which costs nothing -- the two key spaces never
// share a family, so they never collide.
func (fs *bestPrevFamilyShards) shardForNLRI(nlriBytes []byte) *bestPrevShard {
	if len(fs.shards) <= 1 {
		return &fs.shards[0]
	}
	h := maphash.Bytes(bestPrevHashSeed, nlriBytes)
	return &fs.shards[h%uint64(len(fs.shards))]
}

// bestPrevShards holds one bestPrevFamilyShards per address family. The
// outer famMu protects only the family map (rare creation). Per-prefix
// mutations contend only on the owning shard's mu.
type bestPrevShards struct {
	nShards  int
	famMu    sync.RWMutex
	families map[family.Family]*bestPrevFamilyShards
}

// newBestPrevShards constructs an empty bestPrevShards. Families are created
// lazily on first access.
func newBestPrevShards() *bestPrevShards {
	return &bestPrevShards{
		nShards:  bestPrevShardCount(),
		families: make(map[family.Family]*bestPrevFamilyShards),
	}
}

// familyShards returns the shards for fam, optionally creating them. When
// create is false and fam is absent, returns nil.
func (b *bestPrevShards) familyShards(fam family.Family, create bool) *bestPrevFamilyShards {
	b.famMu.RLock()
	fs, ok := b.families[fam]
	b.famMu.RUnlock()
	if ok {
		return fs
	}
	if !create {
		return nil
	}
	b.famMu.Lock()
	defer b.famMu.Unlock()
	fs, ok = b.families[fam]
	if !ok {
		fs = newBestPrevFamilyShards(fam, b.nShards)
		b.families[fam] = fs
	}
	return fs
}

// families returns a snapshot of the family map. Order is unspecified.
func (b *bestPrevShards) familyList() []family.Family {
	b.famMu.RLock()
	defer b.famMu.RUnlock()
	out := make([]family.Family, 0, len(b.families))
	for fam := range b.families {
		out = append(out, fam)
	}
	return out
}

// bestprevLabelKey pairs a family string and a shard label so the metrics
// loop can track emitted (family, shard) combinations across updateMetrics
// cycles and delete stale series for vanished families without relying on
// string concatenation and re-splitting.
type bestprevLabelKey struct {
	family string
	shard  string
}

// shardDepth returns the per-shard route counts for fam, in shard-index
// order. Returns nil when fam has no shards. For a CIDR family each shard's
// count includes both direct entries and the sum of AP path-id entries; for a
// non-CIDR family it is the opaque map size.
func (b *bestPrevShards) shardDepth(fam family.Family) []int {
	fs := b.familyShards(fam, false)
	if fs == nil {
		return nil
	}
	out := make([]int, len(fs.shards))
	for i := range fs.shards {
		sh := &fs.shards[i]
		sh.mu.RLock()
		count := 0
		if sh.store.cidr {
			count = sh.store.direct.Len()
			sh.store.multi.Iterate(func(_ netip.Prefix, ps bestPrevSet) bool {
				count += len(ps.entries)
				return true
			})
		} else {
			count = len(sh.store.opaque)
		}
		sh.mu.RUnlock()
		out[i] = count
	}
	return out
}
