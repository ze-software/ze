package storage

import (
	"net/netip"
	"testing"

	pool "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/pool"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/rib/store"
)

func benchSetupRIB(b *testing.B, n int) (*FamilyRIB, [][]byte) {
	b.Helper()
	rib := NewFamilyRIB(family.IPv4Unicast, false)
	attrs := concat(wireOriginIGP, wireASPath65001, wireNextHop, wireLocalPref100, wireMED100)
	nlris := make([][]byte, n)
	for i := range n {
		pfx := netip.MustParsePrefix(
			netip.AddrFrom4([4]byte{byte(10 + i>>16), byte(i >> 8), byte(i), 0}).String() + "/24",
		)
		nlris[i] = store.PrefixToNLRI(pfx)
		rib.Insert(attrs, nlris[i])
	}
	return rib, nlris
}

func BenchmarkRIBScan(b *testing.B) {
	const n = 100_000
	rib, _ := benchSetupRIB(b, n)
	defer rib.Release()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		count := 0
		rib.IterateEntry(func(_ []byte, _ RouteEntry) bool {
			count++
			return true
		})
		if count != n {
			b.Fatalf("expected %d entries, got %d", n, count)
		}
	}
}

func BenchmarkEntriesEqual(b *testing.B) {
	bun := NewBundle()
	bun.Origin, _ = pool.Origin.Intern([]byte{0x00})
	bun.NextHop, _ = pool.NextHop.Intern([]byte{0x0A, 0x00, 0x00, 0x01})
	bun.LocalPref, _ = pool.LocalPref.Intern([]byte{0x00, 0x00, 0x00, 0x64})
	bun.MED, _ = pool.MED.Intern([]byte{0x00, 0x00, 0x00, 0x64})
	bun.Communities, _ = pool.Communities.Intern([]byte{0xFD, 0xE8, 0x00, 0x64})
	bun.LargeCommunities, _ = pool.LargeCommunities.Intern([]byte{0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02})
	bun.ExtCommunities, _ = pool.ExtCommunities.Intern([]byte{0x00, 0x02, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64})
	bun.ClusterList, _ = pool.ClusterList.Intern([]byte{0x01, 0x01, 0x01, 0x01})
	bun.OriginatorID, _ = pool.OriginatorID.Intern([]byte{0x02, 0x02, 0x02, 0x02})
	bun.AtomicAggregate, _ = pool.AtomicAggregate.Intern([]byte{})
	bun.Aggregator, _ = pool.Aggregator.Intern([]byte{0x00, 0x00, 0xFD, 0xE9, 0x0A, 0x00, 0x00, 0x01})

	aspathH, _ := pool.ASPath.Intern([]byte{0x02, 0x01, 0x00, 0x00, 0xFD, 0xE9})

	a := RouteEntry{
		Bundle: Bundles.Intern(bun),
		ASPath: aspathH,
	}
	defer a.Release()

	bEntry := a
	_ = bEntry.AddRef()
	defer bEntry.Release()

	var sink bool
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		sink = entriesEqual(a, bEntry)
	}
	_ = sink
}

func BenchmarkRIBInsertNoOp(b *testing.B) {
	const n = 100_000
	rib, nlris := benchSetupRIB(b, n)
	defer rib.Release()
	attrs := concat(wireOriginIGP, wireASPath65001, wireNextHop, wireLocalPref100, wireMED100)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		for _, nlri := range nlris {
			rib.Insert(attrs, nlri)
		}
	}
}
