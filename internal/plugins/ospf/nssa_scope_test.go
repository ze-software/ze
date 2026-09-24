// VALIDATES: RFC 3101 Section 2.5 ASBR and forwarding-address scope through
// the production external readers for OSPFv2 and every OSPFv3 address family.
package ospf

import (
	"net/netip"
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

type nssaTestFamily struct {
	name string
	v3   bool
	af   addressFamily
}

var nssaTestFamilies = []nssaTestFamily{
	{"v2", false, afIPv4Unicast},
	{"v3-ipv6-unicast", true, afIPv6Unicast},
	{"v3-ipv6-multicast", true, afIPv6Multicast},
	{"v3-ipv4-unicast", true, afIPv4Unicast},
	{"v3-ipv4-multicast", true, afIPv4Multicast},
}

func nssaScopeInput(t *testing.T, f nssaTestFamily, type7, forwarding bool) (ospfspf.ExternalInput, func(ospfspf.ExternalInput) []ospfspf.RouteEntry) {
	t.Helper()
	area := types.AreaID{0, 0, 0, 7}
	asbr := types.RouterID{2, 2, 2, 2}
	prefix := netip.MustParsePrefix("203.0.113.0/24")
	fa := netip.MustParseAddr("192.0.2.2")
	if !f.af.isIPv4() {
		prefix = netip.MustParsePrefix("2001:db8:100::/64")
		fa = netip.MustParseAddr("2001:db8:7::2")
	}
	originArea := types.BackboneArea
	if type7 {
		originArea = area
	}
	db := ospflsdb.New(nil)
	var lsa packet.LSA
	compute := ospfspf.ComputeExternal
	if f.v3 {
		pfx, ok := netipToV6Prefix(prefix, 0)
		if !ok {
			t.Fatal("cannot encode test prefix")
		}
		kind := ospfv3types.LSTypeASExternal
		if type7 {
			kind = ospfv3types.LSTypeNSSA
		}
		body := ospfv3packet.ExternalLSA{Metric: 5, Prefix: pfx}
		if forwarding {
			body.HasForwardingAddr = true
			if f.af.isIPv4() {
				addr := fa.As4()
				copy(body.ForwardingAddr[:4], addr[:])
			} else {
				body.ForwardingAddr = fa.As16()
			}
		}
		lsa = v6SelfLSA(ospfv3packet.LSA{Header: v6OriginHeader(kind, ospfv3types.LinkStateID{0, 0, 0, 1}, asbr, types.InitialSequenceNumber, false), External: &body})
		compute = v6Strategy{eng: &engine{af: f.af}}.ComputeExternal
	} else {
		kind := types.LSTypeASExternal
		if type7 {
			kind = types.LSTypeNSSA
		}
		body := &packet.ExternalLSA{NetworkMask: maskBytes(prefix.Bits()), Metric: 5}
		if forwarding {
			body.ForwardingAddr = fa.As4()
		}
		lsa = packet.LSA{Header: packet.LSAHeader{Type: kind, LinkStateID: types.LinkStateID(prefix.Addr().As4()), AdvertisingRouter: asbr, Sequence: types.InitialSequenceNumber}, External: body}
	}
	if !db.Install(originArea, lsa) {
		t.Fatal("external LSA was not installed")
	}
	hop := netip.MustParseAddr("192.0.2.9")
	in := ospfspf.ExternalInput{
		Source: db, Root: types.RouterID{1, 1, 1, 1}, NSSAAreas: []types.AreaID{area},
		NSSAPolicies:  map[types.AreaID]ospfspf.AreaSummaryPolicy{area: {Type: types.AreaTypeNSSA}},
		BorderRouters: []ospfspf.BorderRouterEntry{{RouterID: asbr, AreaID: originArea, Kind: ospfspf.BorderRouterASBR, Metric: 10, NextHops: []ospfspf.NextHop{{Addr: hop}}}},
		Routes:        []ospfspf.RouteEntry{{AreaID: originArea, Prefix: netip.PrefixFrom(fa, fa.BitLen()), Type: ospfspf.RouteIntraArea, Metric: 7, NextHops: []ospfspf.NextHop{{Addr: hop}}}},
	}
	return in, compute
}

// RFC requirement: RFC3101-2.5-2 positive -- a Type-7 uses its own NSSA's ASBR entry even when another area's entry is cheaper.
// MUTATION: collapse ASBR reachability by Router ID before checking the LSA's area.
func TestNSSAASBRUsesOriginatingArea(t *testing.T) {
	for _, f := range nssaTestFamilies {
		t.Run(f.name, func(t *testing.T) {
			in, compute := nssaScopeInput(t, f, true, false)
			other := in.BorderRouters[0]
			other.AreaID = types.BackboneArea
			other.Metric = 1
			other.NextHops = []ospfspf.NextHop{{Addr: netip.MustParseAddr("192.0.2.99")}}
			in.BorderRouters = append(in.BorderRouters, other)
			routes := compute(in)
			if len(routes) != 1 || routes[0].Metric != 15 || routes[0].NextHops[0].Addr != netip.MustParseAddr("192.0.2.9") {
				t.Fatalf("NSSA ASBR route = %+v, want own-area cost 15 via 192.0.2.9", routes)
			}
		})
	}
}

// RFC requirement: RFC3101-2.5-2 negative -- ASBR reachability solely outside the originating NSSA cannot install its Type-7, even with a reachable forwarding address.
// MUTATION: skip the ASBR lookup whenever a forwarding address is present.
func TestNSSAASBRRejectsOtherArea(t *testing.T) {
	for _, f := range nssaTestFamilies {
		t.Run(f.name, func(t *testing.T) {
			for _, forwarding := range []bool{false, true} {
				in, compute := nssaScopeInput(t, f, true, forwarding)
				in.BorderRouters[0].AreaID = types.BackboneArea
				if routes := compute(in); len(routes) != 0 {
					t.Fatalf("out-of-area ASBR installed %+v (forwarding address %v)", routes, forwarding)
				}
			}
		})
	}
}

// RFC requirement: RFC3101-2.5-3 positive -- a Type-5 forwarding address resolves through an intra-area or inter-area route in a Type-5 capable area.
// RFC requirement: RFC3101-2.5-4 positive -- a Type-7 forwarding address resolves through its own NSSA's intra-area route.
// MUTATION: reject all non-zero forwarding addresses.
func TestExternalForwardingScopeAcceptsEligiblePaths(t *testing.T) {
	for _, f := range nssaTestFamilies {
		t.Run(f.name, func(t *testing.T) {
			for _, type7 := range []bool{false, true} {
				in, compute := nssaScopeInput(t, f, type7, true)
				if !type7 {
					in.Routes[0].Type = ospfspf.RouteInterArea
				}
				routes := compute(in)
				if len(routes) != 1 || routes[0].Metric != 12 || routes[0].NextHops[0].Addr != netip.MustParseAddr("192.0.2.9") {
					t.Fatalf("eligible forwarding route = %+v, want cost 12 via 192.0.2.9", routes)
				}
			}
		})
	}
}

// RFC requirement: RFC3101-2.5-3 negative -- a Type-5 cannot resolve its forwarding address through an NSSA, a stub area, or an external route.
// RFC requirement: RFC3101-2.5-4 negative -- a Type-7 cannot use another area's route or an inter-area route to its forwarding address.
// MUTATION: resolveForwarding accepts every route covering the forwarding address.
func TestExternalForwardingScopeRejectsIneligiblePaths(t *testing.T) {
	for _, f := range nssaTestFamilies {
		t.Run(f.name, func(t *testing.T) {
			for _, type7 := range []bool{false, true} {
				for _, reason := range []string{"wrong-area", "wrong-path-type", "stub"} {
					in, compute := nssaScopeInput(t, f, type7, true)
					switch reason {
					case "wrong-area":
						in.Routes[0].AreaID = types.AreaID{0, 0, 0, 7}
						if type7 {
							in.Routes[0].AreaID = types.BackboneArea
						}
					case "wrong-path-type":
						in.Routes[0].Type = ospfspf.RouteExternalType1
						if type7 {
							in.Routes[0].Type = ospfspf.RouteInterArea
						}
					case "stub":
						area := types.AreaID{0, 0, 0, 8}
						in.NSSAPolicies[area] = ospfspf.AreaSummaryPolicy{Type: types.AreaTypeStub}
						in.Routes[0].AreaID = area
					}
					if routes := compute(in); len(routes) != 0 {
						t.Fatalf("%s Type-7=%v installed %+v", reason, type7, routes)
					}
				}
			}
		})
	}
}
