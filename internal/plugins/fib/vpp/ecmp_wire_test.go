// Design: docs/architecture/fib/fib-vpp-route-ownership.md -- VPP FIB ECMP wire-level assertions
// Related: backend.go -- addRichRouteInTable and toFibPath render multipath FibPath slices
// Related: routesource.go -- sendRoute encodes IPRouteAddDelV2 under fib-vpp's FIB source

package fibvpp

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.fd.io/govpp/binapi/fib_types"
	"go.fd.io/govpp/binapi/ip"
	"go.fd.io/govpp/binapi/ip_types"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
)

func ipRouteRequests(ch *testChannel, from int) []*ip.IPRouteAddDelV2 {
	var out []*ip.IPRouteAddDelV2
	for _, msg := range ch.requests[from:] {
		if req, ok := msg.(*ip.IPRouteAddDelV2); ok {
			out = append(out, req)
		}
	}
	return out
}

func newGovppECMPWriter() (*fibVPP, *testChannel) {
	ch := &testChannel{}
	source := testRouteSource(ch)
	f := newFibVPPWithMPLS(newGovppBackend(ch, 0, source), newGovppMPLSBackend(ch, 0, source))
	f.routes = source
	return f, ch
}

// TestGovppBackendECMPMultipathWire proves fib-vpp emits exact VPP
// ip_route_add_del_v2 multipath wire payloads over the real govppBackend for
// IPv4 ECMP, IPv6 ECMP, AC-14 member-failure shrinkage without a preceding
// delete, and per-path uniform-TTL labeled ECMP.
//
// VALIDATES: spec-fib-depth AC-5 (VPP multi-path FibPath wire encoding),
// AC-14 (ECMP member failure updates path list without full withdrawal),
// AC-21 (uniform TTL on labeled ECMP pushes).
// PREVENTS: mockBackend hiding broken wire encoding, sw_if_index 0 (local0)
// on gateway paths, IsMultipath=true appending stale members instead of
// replacing, or processMPLSChange dropping ECMPPaths on labeled multipath.
func TestGovppBackendECMPMultipathWire(t *testing.T) {
	t.Run("ipv4_3path_ecmp", func(t *testing.T) {
		f, ch := newGovppECMPWriter()
		prefix := netip.MustParsePrefix("10.20.0.0/24")
		nh1 := netip.MustParseAddr("10.0.0.11")
		nh2 := netip.MustParseAddr("10.0.0.12")
		nh3 := netip.MustParseAddr("10.0.0.13")

		f.processEvent(&incomingBatch{
			Family: family.IPv4Unicast,
			Changes: []incomingChange{{
				Action:   routeaction.Add,
				Prefix:   prefix,
				NextHop:  nh1,
				Weight:   2,
				Protocol: "bgp",
				ECMPPaths: []sysribevents.ECMPPath{
					{NextHop: nh2, Weight: 3},
					{NextHop: nh3, Weight: 5},
				},
			}},
		})

		reqs := ipRouteRequests(ch, 0)
		require.Len(t, reqs, 1)
		req := reqs[0]
		assert.True(t, req.IsAdd)
		// VPP fib_api_route_add_del treats IsMultipath=false + IsAdd=true as
		// fib_table_entry_update, atomically replacing the source's full path
		// list with Route.Paths. IsMultipath=true appends instead of replacing.
		assert.False(t, req.IsMultipath)
		assert.Equal(t, uint8(testRouteSourceID), req.Route.Src)
		assert.Equal(t, ip_types.ADDRESS_IP4, req.Route.Prefix.Address.Af)
		assert.Equal(t, uint8(24), req.Route.Prefix.Len)
		assert.Equal(t, ip_types.IP4Address(prefix.Addr().As4()), req.Route.Prefix.Address.Un.GetIP4())
		assert.Equal(t, uint8(3), req.Route.NPaths)
		require.Len(t, req.Route.Paths, 3)

		wantAddrs := []netip.Addr{nh1, nh2, nh3}
		wantWeights := []uint8{2, 3, 5}
		for i, p := range req.Route.Paths {
			assert.Equal(t, fib_types.FIB_API_PATH_NH_PROTO_IP4, p.Proto, "path %d proto", i)
			assert.Equal(t, ip_types.IP4Address(wantAddrs[i].As4()), p.Nh.Address.GetIP4(), "path %d next-hop", i)
			assert.Equal(t, wantWeights[i], p.Weight, "path %d weight", i)
			assert.Equal(t, noInterface, p.SwIfIndex, "path %d sw_if_index", i)
			assert.Equal(t, fib_types.FIB_API_PATH_TYPE_NORMAL, p.Type, "path %d type", i)
		}
	})

	t.Run("ipv6_2path_ecmp", func(t *testing.T) {
		f, ch := newGovppECMPWriter()
		prefix := netip.MustParsePrefix("2001:db8:20::/48")
		nh1 := netip.MustParseAddr("2001:db8::11")
		nh2 := netip.MustParseAddr("2001:db8::12")

		f.processEvent(&incomingBatch{
			Family: family.IPv6Unicast,
			Changes: []incomingChange{{
				Action:   routeaction.Add,
				Prefix:   prefix,
				NextHop:  nh1,
				Weight:   4,
				Protocol: "bgp",
				ECMPPaths: []sysribevents.ECMPPath{
					{NextHop: nh2, Weight: 7},
				},
			}},
		})

		reqs := ipRouteRequests(ch, 0)
		require.Len(t, reqs, 1)
		req := reqs[0]
		assert.True(t, req.IsAdd)
		assert.False(t, req.IsMultipath)
		assert.Equal(t, ip_types.ADDRESS_IP6, req.Route.Prefix.Address.Af)
		assert.Equal(t, uint8(48), req.Route.Prefix.Len)
		assert.Equal(t, ip_types.IP6Address(prefix.Addr().As16()), req.Route.Prefix.Address.Un.GetIP6())
		assert.Equal(t, uint8(2), req.Route.NPaths)
		require.Len(t, req.Route.Paths, 2)

		wantAddrs := []netip.Addr{nh1, nh2}
		wantWeights := []uint8{4, 7}
		for i, p := range req.Route.Paths {
			assert.Equal(t, fib_types.FIB_API_PATH_NH_PROTO_IP6, p.Proto, "path %d proto", i)
			assert.Equal(t, ip_types.IP6Address(wantAddrs[i].As16()), p.Nh.Address.GetIP6(), "path %d next-hop", i)
			assert.Equal(t, wantWeights[i], p.Weight, "path %d weight", i)
			assert.Equal(t, noInterface, p.SwIfIndex, "path %d sw_if_index", i)
		}
	})

	t.Run("ecmp_member_failure_shrink", func(t *testing.T) {
		f, ch := newGovppECMPWriter()
		prefix := netip.MustParsePrefix("10.20.0.0/24")
		nh1 := netip.MustParseAddr("10.0.0.11")
		nh2 := netip.MustParseAddr("10.0.0.12")
		nh3 := netip.MustParseAddr("10.0.0.13")

		// Initial 3-path install.
		f.processEvent(&incomingBatch{
			Family: family.IPv4Unicast,
			Changes: []incomingChange{{
				Action:   routeaction.Add,
				Prefix:   prefix,
				NextHop:  nh1,
				Protocol: "bgp",
				ECMPPaths: []sysribevents.ECMPPath{
					{NextHop: nh2, Weight: 1},
					{NextHop: nh3, Weight: 1},
				},
			}},
		})
		require.Len(t, ipRouteRequests(ch, 0), 1)

		// First member fails: update 3 -> 2 paths with no preceding delete.
		beforeTwo := len(ch.requests)
		f.processEvent(&incomingBatch{
			Family: family.IPv4Unicast,
			Changes: []incomingChange{{
				Action:   routeaction.Update,
				Prefix:   prefix,
				NextHop:  nh2,
				Protocol: "bgp",
				ECMPPaths: []sysribevents.ECMPPath{
					{NextHop: nh3, Weight: 1},
				},
			}},
		})
		reqsTwo := ipRouteRequests(ch, beforeTwo)
		require.Len(t, reqsTwo, 1, "3->2 update must be one atomic replace with no preceding delete")
		assert.True(t, reqsTwo[0].IsAdd)
		assert.False(t, reqsTwo[0].IsMultipath)
		assert.Equal(t, uint8(2), reqsTwo[0].Route.NPaths)
		require.Len(t, reqsTwo[0].Route.Paths, 2)
		assert.Equal(t, ip_types.IP4Address(nh2.As4()), reqsTwo[0].Route.Paths[0].Nh.Address.GetIP4())
		assert.Equal(t, ip_types.IP4Address(nh3.As4()), reqsTwo[0].Route.Paths[1].Nh.Address.GetIP4())

		// Second member fails: update 2 -> 1 path with no preceding delete.
		beforeOne := len(ch.requests)
		f.processEvent(&incomingBatch{
			Family: family.IPv4Unicast,
			Changes: []incomingChange{{
				Action:   routeaction.Update,
				Prefix:   prefix,
				NextHop:  nh3,
				Protocol: "bgp",
			}},
		})
		reqsOne := ipRouteRequests(ch, beforeOne)
		require.Len(t, reqsOne, 1, "2->1 update must be one atomic replace with no preceding delete")
		assert.True(t, reqsOne[0].IsAdd)
		assert.False(t, reqsOne[0].IsMultipath)
		assert.Equal(t, uint8(1), reqsOne[0].Route.NPaths)
		require.Len(t, reqsOne[0].Route.Paths, 1)
		assert.Equal(t, ip_types.IP4Address(nh3.As4()), reqsOne[0].Route.Paths[0].Nh.Address.GetIP4())
	})

	t.Run("labeled_ecmp_paths", func(t *testing.T) {
		f, ch := newGovppECMPWriter()
		prefix := netip.MustParsePrefix("10.30.0.0/24")
		nh1 := netip.MustParseAddr("10.0.0.11")
		nh2 := netip.MustParseAddr("10.0.0.12")

		f.processEvent(&incomingBatch{
			Family: family.IPv4Unicast,
			Changes: []incomingChange{{
				Action:   routeaction.Add,
				Prefix:   prefix,
				NextHop:  nh1,
				Weight:   1,
				Labels:   []uint32{100, 101},
				Protocol: "bgp",
				ECMPPaths: []sysribevents.ECMPPath{
					{NextHop: nh2, Weight: 2, Labels: []uint32{200}},
				},
			}},
		})

		reqs := ipRouteRequests(ch, 0)
		require.Len(t, reqs, 1)
		req := reqs[0]
		assert.True(t, req.IsAdd)
		assert.False(t, req.IsMultipath)
		assert.Equal(t, uint8(2), req.Route.NPaths)
		require.Len(t, req.Route.Paths, 2)

		p0 := req.Route.Paths[0]
		assert.Equal(t, ip_types.IP4Address(nh1.As4()), p0.Nh.Address.GetIP4())
		assert.Equal(t, uint8(1), p0.Weight)
		assert.Equal(t, noInterface, p0.SwIfIndex)
		assert.Equal(t, uint8(2), p0.NLabels)
		assert.Equal(t, uint32(100), p0.LabelStack[0].Label)
		assert.Equal(t, uint8(1), p0.LabelStack[0].IsUniform)
		assert.Equal(t, uint8(0), p0.LabelStack[0].TTL)
		assert.Equal(t, uint32(101), p0.LabelStack[1].Label)
		assert.Equal(t, uint8(1), p0.LabelStack[1].IsUniform)
		assert.Equal(t, uint8(0), p0.LabelStack[1].TTL)

		p1 := req.Route.Paths[1]
		assert.Equal(t, ip_types.IP4Address(nh2.As4()), p1.Nh.Address.GetIP4())
		assert.Equal(t, uint8(2), p1.Weight)
		assert.Equal(t, noInterface, p1.SwIfIndex)
		assert.Equal(t, uint8(1), p1.NLabels)
		assert.Equal(t, uint32(200), p1.LabelStack[0].Label)
		assert.Equal(t, uint8(1), p1.LabelStack[0].IsUniform)
		assert.Equal(t, uint8(0), p1.LabelStack[0].TTL)
	})
}
