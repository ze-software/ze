package storage

import (
	"net/netip"
	"testing"

	pool "github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/store"
)

func benchSetupRIB(b *testing.B, n int) (*FamilyRIB, [][]byte) {
	b.Helper()
	rib := NewFamilyRIB(family.IPv4Unicast, false)
	attrs := concat(wireOriginIGP, wireASPath65001, wireNextHop, wireLocalPref100, wireMED100)
	nlris := make([][]byte, n)
	for i := range n {
		pfx := netip.PrefixFrom(
			netip.AddrFrom4([4]byte{byte(10 + i>>16), byte(i >> 8), byte(i), 0}), 24,
		)
		nlris[i] = store.PrefixToNLRI(pfx)
		rib.Insert(attrs, nlris[i], true)
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
			rib.Insert(attrs, nlri, true)
		}
	}
}

func BenchmarkRIBInsertUnique(b *testing.B) {
	attrs := concat(wireOriginIGP, wireASPath65001, wireNextHop, wireLocalPref100, wireMED100)

	nlris := make([][]byte, 1000)
	for i := range 1000 {
		pfx := netip.PrefixFrom(
			netip.AddrFrom4([4]byte{byte(10 + i>>16), byte(i >> 8), byte(i), 0}), 24,
		)
		nlris[i] = store.PrefixToNLRI(pfx)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		rib := NewFamilyRIB(family.IPv4Unicast, false)
		for _, nlri := range nlris {
			rib.Insert(attrs, nlri, true)
		}
		rib.Release()
	}
}

func BenchmarkRIBInsertReplace(b *testing.B) {
	const n = 10_000
	rib, nlris := benchSetupRIB(b, n)
	defer rib.Release()
	attrsA := concat(wireOriginIGP, wireASPath65001, wireNextHop, wireLocalPref100, wireMED100)
	attrsB := concat(wireOriginIGP, wireASPath65001, wireNextHop, wireLocalPref100,
		[]byte{0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0xC8}) // MED=200
	attrSets := [2][]byte{attrsA, attrsB}

	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		attrs := attrSets[i&1]
		for _, nlri := range nlris {
			rib.Insert(attrs, nlri, true)
		}
	}
}

// BenchmarkRIBInsertUniqueSharedAttrs measures initial table load: 1000 unique
// NLRIs sharing one attribute blob, parsed once via InsertEntry.
func BenchmarkRIBInsertUniqueSharedAttrs(b *testing.B) {
	attrs := concat(wireOriginIGP, wireASPath65001, wireNextHop, wireLocalPref100, wireMED100)

	nlris := make([][]byte, 1000)
	for i := range 1000 {
		pfx := netip.PrefixFrom(
			netip.AddrFrom4([4]byte{byte(10 + i>>16), byte(i >> 8), byte(i), 0}), 24,
		)
		nlris[i] = store.PrefixToNLRI(pfx)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		rib := NewFamilyRIB(family.IPv4Unicast, false)
		entry, fp, attrLen, err := ParseRouteEntry(attrs, true)
		if err != nil {
			b.Fatal(err)
		}
		for _, nlri := range nlris {
			rib.InsertEntry(nlri, entry, fp, attrLen)
		}
		entry.Release()
		rib.Release()
	}
}

// BenchmarkRIBInsertReplaceSharedAttrs measures convergence: alternating
// between two attribute blobs, each parsed once per pass via InsertEntry.
func BenchmarkRIBInsertReplaceSharedAttrs(b *testing.B) {
	const n = 10_000
	rib := NewFamilyRIB(family.IPv4Unicast, false)
	defer rib.Release()
	attrsA := concat(wireOriginIGP, wireASPath65001, wireNextHop, wireLocalPref100, wireMED100)
	attrsB := concat(wireOriginIGP, wireASPath65001, wireNextHop, wireLocalPref100,
		[]byte{0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0xC8}) // MED=200
	attrSets := [2][]byte{attrsA, attrsB}

	nlris := make([][]byte, n)
	for i := range n {
		pfx := netip.PrefixFrom(
			netip.AddrFrom4([4]byte{byte(10 + i>>16), byte(i >> 8), byte(i), 0}), 24,
		)
		nlris[i] = store.PrefixToNLRI(pfx)
	}
	// Seed with attrsA.
	entryA, fpA, alA, _ := ParseRouteEntry(attrsA, true)
	for _, nlri := range nlris {
		rib.InsertEntry(nlri, entryA, fpA, alA)
	}
	entryA.Release()

	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		raw := attrSets[i&1]
		entry, fp, attrLen, err := ParseRouteEntry(raw, true)
		if err != nil {
			b.Fatal(err)
		}
		for _, nlri := range nlris {
			rib.InsertEntry(nlri, entry, fp, attrLen)
		}
		entry.Release()
	}
}
