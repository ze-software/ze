package bgp

import (
	"runtime"
	"strconv"
	"testing"
	"time"
	"unsafe"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

func TestPluginRouteSizes(t *testing.T) {
	var r Route
	t.Logf("bgp.Route struct: %d bytes", unsafe.Sizeof(r))
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

func TestHeapBytesPerPluginRibOutRoute(t *testing.T) {
	if testing.Short() {
		t.Skip("memory profiling")
	}

	const N = 100_000

	type ribOut = map[string]map[family.Family]map[string]*Route

	var store ribOut

	totalAlloc, mallocs := measureHeap(func() {
		store = make(ribOut)
		peerAddr := "192.168.1.1"
		store[peerAddr] = make(map[family.Family]map[string]*Route)
		store[peerAddr][family.IPv4Unicast] = make(map[string]*Route, N)

		fam := family.IPv4Unicast
		famRoutes := store[peerAddr][fam]

		for i := range N {
			prefix := strconv.Itoa(10+i>>16) + "." + strconv.Itoa((i>>8)&0xFF) + "." + strconv.Itoa(i&0xFF) + ".0/32"
			key := prefix

			origin := attribute.OriginIGP
			med := uint32(100)
			lp := uint32(200)

			famRoutes[key] = &Route{
				MsgID:           uint64(i),
				Family:          fam,
				Prefix:          prefix,
				NextHop:         "192.168.1.1",
				Timestamp:       time.Now(),
				Origin:          &origin,
				ASPath:          []uint32{65000, 65001, 65002},
				MED:             &med,
				LocalPreference: &lp,
				Communities:     []attribute.Community{0x00010001, 0x00020002},
				RawAttrs:        "400101004002060201fde8fde9fdea40030419a80101",
				SourcePeer:      "10.0.0.1",
			}
		}
	})

	bytesPerRoute := totalAlloc / N

	t.Logf("=== Plugin ribOut bgp.Route (typical attrs) ===")
	t.Logf("Routes:          %d", N)
	t.Logf("TotalAlloc:      %d bytes (%.1f MB)", totalAlloc, float64(totalAlloc)/(1024*1024))
	t.Logf("Bytes per route: %d", bytesPerRoute)
	t.Logf("Mallocs:         %d (%.1f per route)", mallocs, float64(mallocs)/N)

	t.Logf("")
	t.Logf("--- Projection (bytes per route = %d) ---", bytesPerRoute)
	for _, count := range []int{100_000, 1_000_000} {
		for _, peers := range []int{1, 5, 10, 50} {
			totalMB := float64(bytesPerRoute) * float64(count) * float64(peers) / (1024 * 1024)
			t.Logf("  %dk routes x %d peers = %.0f MB", count/1000, peers, totalMB)
		}
	}

	_ = store
}

func TestHeapBytesPerPluginRibOutRouteWithMeta(t *testing.T) {
	if testing.Short() {
		t.Skip("memory profiling")
	}

	const N = 100_000

	type ribOut = map[string]map[family.Family]map[string]*Route

	var store ribOut

	totalAlloc, mallocs := measureHeap(func() {
		store = make(ribOut)
		peerAddr := "192.168.1.1"
		store[peerAddr] = make(map[family.Family]map[string]*Route)
		store[peerAddr][family.IPv4Unicast] = make(map[string]*Route, N)

		fam := family.IPv4Unicast
		famRoutes := store[peerAddr][fam]

		for i := range N {
			prefix := strconv.Itoa(10+i>>16) + "." + strconv.Itoa((i>>8)&0xFF) + "." + strconv.Itoa(i&0xFF) + ".0/32"
			key := prefix

			origin := attribute.OriginIGP
			med := uint32(100)
			lp := uint32(200)

			famRoutes[key] = &Route{
				MsgID:           uint64(i),
				Family:          fam,
				Prefix:          prefix,
				NextHop:         "192.168.1.1",
				Timestamp:       time.Now(),
				Origin:          &origin,
				ASPath:          []uint32{65000, 65001, 65002},
				MED:             &med,
				LocalPreference: &lp,
				Communities:     []attribute.Community{0x00010001, 0x00020002},
				LargeCommunities: []attribute.LargeCommunity{
					{GlobalAdmin: 65000, LocalData1: 1, LocalData2: 2},
				},
				ExtendedCommunities: []attribute.ExtendedCommunity{
					{0x00, 0x02, 0xfd, 0xe8, 0x00, 0x00, 0x00, 0x64},
				},
				RawAttrs:   "400101004002060201fde8fde9fdea40030419a80101c0081000010001000200024008040000006440050400000064",
				Meta:       map[string]any{"src-role": "peer", "otc": uint32(65000)},
				SourcePeer: "10.0.0.1",
			}
		}
	})

	bytesPerRoute := totalAlloc / N

	t.Logf("=== Plugin ribOut bgp.Route (full attrs + meta) ===")
	t.Logf("Routes:          %d", N)
	t.Logf("TotalAlloc:      %d bytes (%.1f MB)", totalAlloc, float64(totalAlloc)/(1024*1024))
	t.Logf("Bytes per route: %d", bytesPerRoute)
	t.Logf("Mallocs:         %d (%.1f per route)", mallocs, float64(mallocs)/N)

	_ = store
}
