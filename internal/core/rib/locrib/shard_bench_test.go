package locrib

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
)

// BenchmarkShardedInsertParallel measures sharded throughput when N
// goroutines hammer different prefixes in parallel. Compare against the
// b.RunParallel knob to see how shard contention scales.
func BenchmarkShardedInsertParallel(b *testing.B) {
	r := NewRIB()
	p := pathBGP(1, 50)
	var counter atomic.Uint64
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := counter.Add(1)
			pfx := netip.PrefixFrom(
				netip.AddrFrom4([4]byte{10, byte(i >> 24), byte(i >> 8), byte(i)}),
				32,
			)
			r.Insert(famV4, pfx, p)
		}
	})
}

// BenchmarkShardedInsertSerial measures sharded throughput from a single
// goroutine: the lower bound of cost vs. the parallel benchmark.
func BenchmarkShardedInsertSerial(b *testing.B) {
	r := NewRIB()
	p := pathBGP(1, 50)
	for i := range b.N {
		pfx := netip.PrefixFrom(
			netip.AddrFrom4([4]byte{10, byte(i >> 16), byte(i >> 8), byte(i)}),
			32,
		)
		r.Insert(famV4, pfx, p)
	}
}

// BenchmarkShardedManyWritersDistinctShards forces every goroutine onto a
// different shard via the prefix's high bits, so contention reduces to
// shared interner / family map operations only. Useful for confirming
// per-shard locking does not regress with the shard count.
func BenchmarkShardedManyWritersDistinctShards(b *testing.B) {
	r := NewRIB()
	p := pathBGP(1, 50)
	const goroutines = 8
	per := b.N / goroutines
	if per <= 0 {
		per = 1
	}
	var wg sync.WaitGroup
	b.ResetTimer()
	for g := range goroutines {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := range per {
				pfx := netip.PrefixFrom(
					netip.AddrFrom4([4]byte{10, byte(g), byte(i >> 8), byte(i)}),
					32,
				)
				r.Insert(famV4, pfx, p)
			}
		}(g)
	}
	wg.Wait()
}
