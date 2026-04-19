package rib

import (
	"fmt"
	"net/netip"
	"runtime"
	"testing"

	ribstore "codeberg.org/thomas-mangin/ze/internal/core/rib/store"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// BenchmarkBestPathRecordHeapFootprint measures the steady-state heap cost of
// storing N distinct packed records in Store[bestPathRecord] plus the shared
// interner. Provides a lower-bounds figure that complements (and is available
// without root / netns) the full `make ze-stress-profile` run in AC-1.
//
// Compare with the Phase-4b baseline (72-byte struct, five GC pointers):
// Phase-4b 1M-prefix stress captured
// `bart.NewFringeNode[bestPathRecord_struct]` at 56.5 MB flat. With the named
// uint64 the same node specialises on an 8-byte scalar; this benchmark prints
// the allocated bytes so reviewers can verify the drop without reproducing
// the root-only stress run.
func BenchmarkBestPathRecordHeapFootprint(b *testing.B) {
	cases := []int{100_000, 1_000_000}
	for _, n := range cases {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			for range b.N {
				fam := family.Family{AFI: 1, SAFI: 1}
				store := ribstore.NewStore[bestPathRecord](fam)
				interner := newBestPrevInterner()
				// Pre-intern a small realistic cardinality (2k peers, 256 NHs,
				// 16 metrics) so the per-record cost is dominated by the
				// packed uint64 plus BART fringe metadata, matching the
				// deployment shape.
				peerIdxs := make([]uint16, 2000)
				for i := range peerIdxs {
					idx, ok := interner.internPeer(fmt.Sprintf("10.0.%d.%d", i/256, i%256))
					if !ok {
						b.Fatalf("peer fill %d: interner unexpectedly saturated (cap=%d)", i, internerCap)
					}
					peerIdxs[i] = idx
				}
				nhIdxs := make([]uint16, 256)
				for i := range nhIdxs {
					idx, ok := interner.internNextHop(netip.AddrFrom4([4]byte{192, 168, 0, byte(i)}))
					if !ok {
						b.Fatalf("nexthop fill %d: interner unexpectedly saturated (cap=%d)", i, internerCap)
					}
					nhIdxs[i] = idx
				}
				metricIdxs := make([]uint16, 16)
				for i := range metricIdxs {
					idx, ok := interner.internMetric(uint32(i * 100))
					if !ok {
						b.Fatalf("metric fill %d: interner unexpectedly saturated (cap=%d)", i, internerCap)
					}
					metricIdxs[i] = idx
				}

				var beforeMs runtime.MemStats
				runtime.GC()
				runtime.ReadMemStats(&beforeMs)

				for i := range n {
					// Distinct /24 per iteration up to 16M prefixes.
					pfx := netip.PrefixFrom(
						netip.AddrFrom4([4]byte{byte(i >> 16), byte(i >> 8), byte(i), 0}),
						24,
					)
					rec := packBestPath(
						metricIdxs[i%len(metricIdxs)],
						peerIdxs[i%len(peerIdxs)],
						nhIdxs[i%len(nhIdxs)],
						flagEBGP,
					)
					store.Insert(pfx, rec)
				}
				runtime.GC()
				var afterMs runtime.MemStats
				runtime.ReadMemStats(&afterMs)
				// HeapAlloc counts bytes of live heap objects (allocated
				// and not yet freed by the GC cycle we just triggered).
				// It is stable across both Store backends (BART default
				// and the map variant under -tags maprib), where the
				// span-level HeapInuse figure can shrink on map bucket
				// release and produce a misleading zero.
				delta := max(int64(afterMs.HeapAlloc)-int64(beforeMs.HeapAlloc), 0)
				b.ReportMetric(float64(delta), "heap-alloc-bytes")
				b.ReportMetric(float64(delta)/float64(n), "heap-bytes/entry")
				_ = store.Len()
				_ = interner
			}
		})
	}
}
