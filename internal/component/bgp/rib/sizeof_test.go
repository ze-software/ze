package rib

import (
	"net/netip"
	"runtime"
	"testing"
	"unsafe"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
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
	t.Logf("  refCount:           %d bytes", unsafe.Sizeof(r.refCount))
	t.Logf("  indexCache slice:   %d bytes", unsafe.Sizeof(r.indexCache))
	t.Logf("  wireBytes slice:    %d bytes", unsafe.Sizeof(r.wireBytes))
	t.Logf("  nlriWireBytes:      %d bytes", unsafe.Sizeof(r.nlriWireBytes))
	t.Logf("  sourceCtxID:        %d bytes", unsafe.Sizeof(r.sourceCtxID))
	t.Logf("")
	t.Logf("ASPath struct:        %d bytes", unsafe.Sizeof(asp))
	t.Logf("ASPathSegment struct: %d bytes", unsafe.Sizeof(seg))
	t.Logf("ContextID:            %d bytes", unsafe.Sizeof(bgpctx.ContextID(0)))
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

	wireBytes := make([]byte, 40)
	nlriBytes := make([]byte, 4)

	return NewRouteWithWireCacheFull(n, nh, attrs, asPath, wireBytes, nlriBytes, bgpctx.ContextID(1))
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

	t.Logf("=== Engine rib.Route (with wire cache, typical attrs) ===")
	t.Logf("Routes:          %d", N)
	t.Logf("TotalAlloc:      %d bytes (%.1f MB)", totalAlloc, float64(totalAlloc)/(1024*1024))
	t.Logf("Bytes per route: %d", bytesPerRoute)
	t.Logf("Mallocs:         %d (%.1f per route)", mallocs, float64(mallocs)/N)

	_ = routes
}

func TestHeapBytesPerRouteInOutgoingRIB(t *testing.T) {
	if testing.Short() {
		t.Skip("memory profiling")
	}

	const N = 100_000

	rib := NewOutgoingRIB()

	totalAlloc, mallocs := measureHeap(func() {
		for i := range N {
			r := makeTypicalRoute(i)
			rib.MarkSent(r)
		}
	})

	stats := rib.Stats()
	bytesPerRoute := totalAlloc / int64(stats.SentRoutes)

	t.Logf("=== Engine OutgoingRIB (MarkSent with wire cache) ===")
	t.Logf("Routes stored:   %d / %d attempted", stats.SentRoutes, N)
	t.Logf("TotalAlloc:      %d bytes (%.1f MB)", totalAlloc, float64(totalAlloc)/(1024*1024))
	t.Logf("Bytes per route: %d", bytesPerRoute)
	t.Logf("Mallocs:         %d (%.1f per route)", mallocs, float64(mallocs)/float64(stats.SentRoutes))

	t.Logf("")
	t.Logf("--- Projection (bytes per route = %d) ---", bytesPerRoute)
	for _, count := range []int{100_000, 1_000_000} {
		for _, peers := range []int{1, 5, 10, 50} {
			totalMB := float64(bytesPerRoute) * float64(count) * float64(peers) / (1024 * 1024)
			t.Logf("  %dk routes x %d peers = %.0f MB", count/1000, peers, totalMB)
		}
	}
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
			routes[i] = NewRoute(n, nh, nil)
		}
	})

	bytesPerRoute := totalAlloc / N

	t.Logf("=== Engine rib.Route (minimal, no attrs, no wire cache) ===")
	t.Logf("Routes:          %d", N)
	t.Logf("TotalAlloc:      %d bytes (%.1f MB)", totalAlloc, float64(totalAlloc)/(1024*1024))
	t.Logf("Bytes per route: %d", bytesPerRoute)
	t.Logf("Mallocs:         %d (%.1f per route)", mallocs, float64(mallocs)/N)

	_ = routes
}
