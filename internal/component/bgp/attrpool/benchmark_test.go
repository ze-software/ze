package attrpool

import (
	"fmt"
	"sync/atomic"
	"testing"
)

func benchIntern(b *testing.B, p *Pool, data []byte) Handle {
	b.Helper()
	h, err := p.Intern(data)
	if err != nil {
		b.Fatal(err)
	}
	return h
}

// BenchmarkInternExisting measures performance of interning existing data.
// Target: < 100ns per operation.
func BenchmarkInternExisting(b *testing.B) {
	p := New(1024 * 1024)
	benchIntern(b, p, []byte("benchmark-data"))

	for b.Loop() {
		benchIntern(b, p, []byte("benchmark-data"))
	}
}

// BenchmarkInternNew measures performance of interning new unique data.
// Target: < 500ns per operation.
func BenchmarkInternNew(b *testing.B) {
	p := New(1024 * 1024 * 100) // 100MB to avoid reallocation

	for i := 0; b.Loop(); i++ {
		benchIntern(b, p, fmt.Appendf(nil, "data-%d", i))
	}
}

// BenchmarkGet measures performance of retrieving data.
// Target: < 50ns per operation.
func BenchmarkGet(b *testing.B) {
	p := New(1024)
	h := benchIntern(b, p, []byte("benchmark-data"))

	for b.Loop() {
		if _, err := p.Get(h); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRelease measures performance of releasing handles.
// Target: < 100ns per operation.
func BenchmarkRelease(b *testing.B) {
	p := New(1024 * 1024 * 100) // 100MB
	handles := make([]Handle, b.N)

	// Pre-allocate unique handles
	for i := range b.N {
		handles[i] = benchIntern(b, p, fmt.Appendf(nil, "data-%d", i))
	}

	b.ResetTimer()
	for i := range b.N {
		_ = p.Release(handles[i])
	}
}

// BenchmarkLength measures performance of getting data length.
func BenchmarkLength(b *testing.B) {
	p := New(1024)
	h := benchIntern(b, p, []byte("benchmark-data"))

	for b.Loop() {
		if _, err := p.Length(h); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMetrics measures performance of getting metrics.
func BenchmarkMetrics(b *testing.B) {
	p := New(1024)
	// Add some entries
	for i := range 100 {
		benchIntern(b, p, fmt.Appendf(nil, "data-%d", i))
	}

	for b.Loop() {
		_ = p.Metrics()
	}
}

// BenchmarkCompact measures performance of compaction.
func BenchmarkCompact(b *testing.B) {
	pools := make([]*Pool, b.N)
	for i := range b.N {
		p := New(16 * 1024)
		handles := make([]Handle, 1000)
		for j := range 1000 {
			handles[j] = benchIntern(b, p, fmt.Appendf(nil, "data-%d", j))
		}
		for j := range 500 {
			_ = p.Release(handles[j])
		}
		pools[i] = p
	}

	b.ResetTimer()
	for i := range b.N {
		pools[i].Compact()
	}
}

// BenchmarkConcurrentIntern measures performance under concurrent load.
func BenchmarkConcurrentIntern(b *testing.B) {
	p := New(1024 * 1024 * 100)

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			benchIntern(b, p, fmt.Appendf(nil, "data-%d", i))
			i++
		}
	})
}

// BenchmarkConcurrentGet measures Get performance under concurrent load.
func BenchmarkConcurrentGet(b *testing.B) {
	p := New(1024)
	h := benchIntern(b, p, []byte("benchmark-data"))

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := p.Get(h); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkConcurrentInternSharded measures Intern under concurrent load with a
// disjoint keyspace per goroutine, representative of the production path where
// many peers intern diverse attributes. Diverse content hashes spread the work
// across shards, so the per-op time should not rise above the single-threaded
// miss (BenchmarkInternNew) the way the single-lock design did (AC-9).
func BenchmarkConcurrentInternSharded(b *testing.B) {
	p := New(1024 * 1024 * 100)

	var gid atomic.Uint64
	b.RunParallel(func(pb *testing.PB) {
		// Unique per-goroutine prefix → globally distinct keys → all shards.
		prefix := gid.Add(1)
		i := 0
		for pb.Next() {
			benchIntern(b, p, fmt.Appendf(nil, "g%d-data-%d", prefix, i))
			i++
		}
	})
}

// BenchmarkConcurrentInternSingleShard is the worst case: every key is forced to
// hash into one shard, reproducing the pre-sharding single-lock serialization.
// Contrast with BenchmarkConcurrentInternSharded to see the sharding win (AC-9).
func BenchmarkConcurrentInternSingleShard(b *testing.B) {
	// Pre-generate distinct keys that all hash to shard 0 (done outside timing).
	const keys = 1 << 16
	sameShard := make([][]byte, 0, keys)
	for i := 0; len(sameShard) < keys; i++ {
		k := fmt.Appendf(nil, "single-%d", i)
		if defaultShardOf(k) == 0 {
			sameShard = append(sameShard, k)
		}
	}

	p := New(1024 * 1024 * 100)
	var idx atomic.Uint64
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			k := sameShard[int(idx.Add(1))%len(sameShard)]
			benchIntern(b, p, k)
		}
	})
}

// genKeysInShard returns n distinct keys that all hash to the given shard.
func genKeysInShard(targetShard uint32, n int) [][]byte {
	keys := make([][]byte, 0, n)
	for i := 0; len(keys) < n; i++ {
		k := fmt.Appendf(nil, "k-%d-%d", targetShard, i)
		if defaultShardOf(k) == targetShard {
			keys = append(keys, k)
		}
	}
	return keys
}

// BenchmarkConcurrentInternHitSpread isolates write-lock contention on the hot
// dedup-hit path (re-interning already-present shared attributes, which takes
// the write lock). Keys are pre-generated and pre-interned across all shards, so
// the timed loop does no allocation — only the lock + map lookup + refcount.
// Hits spread across 16 shard locks, so this should scale (AC-9).
func BenchmarkConcurrentInternHitSpread(b *testing.B) {
	p := New(1024 * 1024)
	const perShard = 64

	keys := make([][]byte, 0, numShards*perShard)
	for s := range uint32(numShards) {
		for _, k := range genKeysInShard(s, perShard) {
			benchIntern(b, p, k) // pre-intern: refcount 1
			keys = append(keys, k)
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			benchIntern(b, p, keys[i%len(keys)]) // dedup hit, no allocation
			i++
		}
	})
}

// BenchmarkConcurrentInternHitSingleShard is the pre-sharding worst case: every
// dedup hit lands on one shard's write lock, so all goroutines serialize on it.
// Contrast with BenchmarkConcurrentInternHitSpread to see the sharding win (AC-9).
func BenchmarkConcurrentInternHitSingleShard(b *testing.B) {
	p := New(1024 * 1024)
	keys := genKeysInShard(0, 64)
	for _, k := range keys {
		benchIntern(b, p, k) // pre-intern into shard 0
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			benchIntern(b, p, keys[i%len(keys)]) // dedup hit, all on shard 0
			i++
		}
	})
}

// BenchmarkConcurrentGetShardedSpread measures Get under concurrent load where
// reads are spread across handles in every shard. Each shard has its own
// RWMutex reader-count word on its own cache line, so unlike BenchmarkConcurrentGet
// (a single handle in a single shard) the read path is not bound by one shared
// atomic (AC-10).
func BenchmarkConcurrentGetShardedSpread(b *testing.B) {
	p := New(1024 * 1024)

	// One live handle per shard.
	handles := make([]Handle, 0, numShards)
	seen := make(map[uint32]bool)
	for i := 0; len(handles) < numShards; i++ {
		data := fmt.Appendf(nil, "spread-%d", i)
		s := defaultShardOf(data)
		if !seen[s] {
			seen[s] = true
			handles = append(handles, benchIntern(b, p, data))
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if _, err := p.Get(handles[i%len(handles)]); err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

// BenchmarkDeduplication measures deduplication hit rate performance.
func BenchmarkDeduplication(b *testing.B) {
	p := New(1024 * 1024)

	// Pre-populate with some entries
	for i := range 100 {
		benchIntern(b, p, fmt.Appendf(nil, "data-%d", i))
	}

	for i := 0; b.Loop(); i++ {
		// 50% hit rate
		benchIntern(b, p, fmt.Appendf(nil, "data-%d", i%100))
	}
}
