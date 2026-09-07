package rib

import (
	"net/netip"
	"runtime"
	"testing"
	"unsafe"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

func TestStructSizes(t *testing.T) {
	var r Route
	var asp attribute.ASPath
	var seg attribute.ASPathSegment

	t.Logf("Route struct:         %d bytes", unsafe.Sizeof(r))
	t.Logf("  nlri (NLRI iface):  %d bytes", unsafe.Sizeof(r.nlri))
	t.Logf("  nextHop:            %d bytes", unsafe.Sizeof(r.nextHop))
	t.Logf("  attributes slice:   %d bytes", unsafe.Sizeof(r.attributes))
	t.Logf("  asPath pointer:     %d bytes", unsafe.Sizeof(r.asPath))
	t.Logf("  indexCache slice:   %d bytes", unsafe.Sizeof(r.indexCache))
	t.Logf("")
	t.Logf("ASPath struct:        %d bytes", unsafe.Sizeof(asp))
	t.Logf("ASPathSegment struct: %d bytes", unsafe.Sizeof(seg))
	t.Logf("family.Family:        %d bytes", unsafe.Sizeof(family.Family{}))
}

func makePrefix(i int) netip.Prefix {
	return netip.PrefixFrom(netip.AddrFrom4([4]byte{
		byte(10 + i>>16),
		byte(i >> 8),
		byte(i),
		0,
	}), 32)
}

func makeTypicalRoute(i int) *Route {
	prefix := makePrefix(i)
	n := nlri.NewINET(family.IPv4Unicast, prefix, 0)
	nh := netip.AddrFrom4([4]byte{192, 168, 1, 1})

	origin := attribute.OriginIGP
	asPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65000, 65001, 65002}},
		},
	}
	med := attribute.MED(100)
	lp := attribute.LocalPref(200)
	attrs := []attribute.Attribute{origin, med, lp}

	return NewRouteWithASPath(n, nh, attrs, asPath)
}

func measureHeap(fn func()) (allocBytes, allocObjects int64) {
	runtime.GC()
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	fn()

	runtime.GC()
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	return int64(after.TotalAlloc - before.TotalAlloc),
		int64(after.Mallocs - before.Mallocs)
}

func TestHeapBytesPerRoute(t *testing.T) {
	if testing.Short() {
		t.Skip("memory profiling")
	}

	const N = 100_000

	var routes []*Route

	totalAlloc, mallocs := measureHeap(func() {
		routes = make([]*Route, N)
		for i := range N {
			routes[i] = makeTypicalRoute(i)
		}
	})

	bytesPerRoute := totalAlloc / N

	t.Logf("=== Engine rib.Route (typical attrs) ===")
	t.Logf("Routes:          %d", N)
	t.Logf("TotalAlloc:      %d bytes (%.1f MB)", totalAlloc, float64(totalAlloc)/(1024*1024))
	t.Logf("Bytes per route: %d", bytesPerRoute)
	t.Logf("Mallocs:         %d (%.1f per route)", mallocs, float64(mallocs)/N)

	_ = routes
}

func TestHeapBytesPerRouteMinimal(t *testing.T) {
	if testing.Short() {
		t.Skip("memory profiling")
	}

	const N = 100_000

	var routes []*Route

	totalAlloc, mallocs := measureHeap(func() {
		routes = make([]*Route, N)
		for i := range N {
			prefix := makePrefix(i)
			n := nlri.NewINET(family.IPv4Unicast, prefix, 0)
			nh := netip.AddrFrom4([4]byte{192, 168, 1, 1})
			routes[i] = NewRouteWithASPath(n, nh, nil, nil)
		}
	})

	bytesPerRoute := totalAlloc / N

	t.Logf("=== Engine rib.Route (minimal, no attrs) ===")
	t.Logf("Routes:          %d", N)
	t.Logf("TotalAlloc:      %d bytes (%.1f MB)", totalAlloc, float64(totalAlloc)/(1024*1024))
	t.Logf("Bytes per route: %d", bytesPerRoute)
	t.Logf("Mallocs:         %d (%.1f per route)", mallocs, float64(mallocs)/N)

	_ = routes
}
