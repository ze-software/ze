package storage

import (
	"runtime"
	"testing"
	"unsafe"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/store"

	"net/netip"
)

func TestRouteEntrySizes(t *testing.T) {
	var re RouteEntry
	var b Bundle
	t.Logf("RouteEntry struct: %d bytes", unsafe.Sizeof(re))
	t.Logf("Bundle struct:     %d bytes", unsafe.Sizeof(b))
}

func TestHeapBytesPerPluginRIBRoute(t *testing.T) {
	if testing.Short() {
		t.Skip("memory profiling")
	}

	const N = 100_000

	attrs := concat(wireOriginIGP, wireASPath65001, wireNextHop, wireLocalPref100, wireMED100)

	rib := NewFamilyRIB(family.IPv4Unicast, false)

	runtime.GC()
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	for i := range N {
		pfx := netip.PrefixFrom(
			netip.AddrFrom4([4]byte{byte(10 + i>>16), byte(i >> 8), byte(i), 0}), 24,
		)
		nlriBytes := store.PrefixToNLRI(pfx)
		rib.Insert(attrs, nlriBytes, true)
	}

	runtime.GC()
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	totalAlloc := int64(after.TotalAlloc - before.TotalAlloc)
	mallocs := int64(after.Mallocs - before.Mallocs)
	bytesPerRoute := totalAlloc / N

	t.Logf("=== Plugin RIB RouteEntry + BART trie (100K routes, shared attrs) ===")
	t.Logf("Routes:          %d (count=%d)", N, rib.Len())
	t.Logf("TotalAlloc:      %d bytes (%.1f MB)", totalAlloc, float64(totalAlloc)/(1024*1024))
	t.Logf("Bytes per route: %d", bytesPerRoute)
	t.Logf("Mallocs:         %d (%.1f per route)", mallocs, float64(mallocs)/N)

	t.Logf("")
	t.Logf("--- Projection (bytes per route = %d) ---", bytesPerRoute)
	for _, count := range []int{100_000, 1_000_000} {
		totalMB := float64(bytesPerRoute) * float64(count) / (1024 * 1024)
		t.Logf("  %dk routes (shared attrs) = %.0f MB", count/1000, totalMB)
	}

	rib.Release()
}
