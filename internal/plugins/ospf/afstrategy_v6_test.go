// VALIDATES: spec-ospf-af-unify Phase 5 -- the v6 AFPrefixStrategy decodes the
// address-free OSPFv3 Router-LSA into the shared SPF graph: a point-to-point link
// keys the neighbor by Router ID so the AF-agnostic Dijkstra (two-way check, metric)
// runs unchanged. PREVENTS: the v6 engine falling back to the OSPFv2 BuildGraph,
// which would misparse a v6 LSA (its 16-bit LS Type sits where v2 has Options+Type).
package ospf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

type fakeV6Source struct {
	headers []packet.LSAHeader
	lsas    map[types.LSAKey]packet.LSA
}

func (s fakeV6Source) Summary(types.AreaID) []packet.LSAHeader { return s.headers }

func (s fakeV6Source) LookupLSA(_ types.AreaID, key types.LSAKey) (packet.LSA, bool) {
	l, ok := s.lsas[key]
	return l, ok
}

func TestOSPFv6BuildGraph(t *testing.T) {
	// An OSPFv3 Router-LSA from 1.1.1.1 with a single p2p link to 2.2.2.2.
	rlsa := ospfv3packet.LSA{
		Header: ospfv3packet.LSAHeader{
			Age:               1,
			Type:              ospfv3types.LSTypeRouter,
			LinkStateID:       ospfv3types.LinkStateID{0, 0, 0, 0},
			AdvertisingRouter: ospfv3types.RouterID{1, 1, 1, 1},
			Sequence:          ospfv3types.InitialSequenceNumber,
		},
		Router: &ospfv3packet.RouterLSA{
			Options: ospfv3types.OptV6 | ospfv3types.OptR,
			Links: []ospfv3packet.RouterLink{{
				Type:                ospfv3packet.RouterLinkTypeP2P,
				Metric:              10,
				InterfaceID:         1,
				NeighborInterfaceID: 1,
				NeighborRouterID:    ospfv3types.RouterID{2, 2, 2, 2},
			}},
		},
	}
	raw := make([]byte, (&rlsa).EncodedLen())
	(&rlsa).WriteTo(raw, 0)

	hdr := v6LSAHeaderToNeutral(rlsa.Header)
	src := fakeV6Source{
		headers: []packet.LSAHeader{hdr},
		lsas:    map[types.LSAKey]packet.LSA{hdr.Key(): {Header: hdr, RawBytes: raw}},
	}

	g := v6Strategy{}.BuildGraph(src, types.BackboneArea)
	rv := g.Routers[types.RouterID{1, 1, 1, 1}]
	if rv == nil {
		t.Fatalf("router vertex 1.1.1.1 missing from v6 graph")
	}
	if len(rv.Links) != 1 {
		t.Fatalf("links = %d, want 1 (the p2p link)", len(rv.Links))
	}
	l := rv.Links[0]
	if l.Type != packet.RouterLinkTypeP2P {
		t.Errorf("link type = %d, want p2p", l.Type)
	}
	if l.LinkID != types.LinkStateID([4]byte{2, 2, 2, 2}) {
		t.Errorf("link id = %v, want neighbor router id 2.2.2.2", l.LinkID)
	}
	if uint64(l.Metric) != 10 {
		t.Errorf("metric = %d, want 10", l.Metric)
	}
	// The v6 strategy still satisfies the SPF seam (compile-time guarantee).
	var _ ospfspf.AFPrefixStrategy = v6Strategy{}
}

func TestOSPFv6ComputeInterArea(t *testing.T) {
	// An ABR 2.2.2.2 advertises an Inter-Area-Prefix-LSA for 2001:db8:5::/64 with metric 20.
	pfx, ok := netipToV6Prefix(netip.MustParsePrefix("2001:db8:5::/64"), 0)
	if !ok {
		t.Fatal("netipToV6Prefix failed")
	}
	abr := types.RouterID{2, 2, 2, 2}
	iap := ospfv3packet.LSA{
		Header: ospfv3packet.LSAHeader{
			Age: 1, Type: ospfv3types.LSTypeInterAreaPrefix,
			LinkStateID: ospfv3types.LinkStateID{0, 0, 0, 1}, AdvertisingRouter: ospfv3types.RouterID(abr),
			Sequence: ospfv3types.InitialSequenceNumber,
		},
		InterAreaPfx: &ospfv3packet.InterAreaPrefixLSA{Metric: 20, Prefix: pfx},
	}
	raw := make([]byte, (&iap).EncodedLen())
	(&iap).WriteTo(raw, 0)
	hdr := v6LSAHeaderToNeutral(iap.Header)
	src := fakeV6Source{
		headers: []packet.LSAHeader{hdr},
		lsas:    map[types.LSAKey]packet.LSA{hdr.Key(): {Header: hdr, RawBytes: raw}},
	}

	// The ABR is reached intra-area at cost 10 with next-hop link-local fe80::2.
	nh := netip.MustParseAddr("fe80::2")
	res := &ospfspf.Result{
		Area:  types.BackboneArea,
		Graph: ospfspf.NewGraph(types.BackboneArea),
		Nodes: map[ospfspf.VertexID]*ospfspf.NodeResult{
			{Kind: ospfspf.VertexRouter, Router: abr}: {Metric: 10, NextHops: []ospfspf.NextHop{{Addr: nh}}},
		},
	}
	in := ospfspf.InterAreaInput{
		Source:  src,
		Root:    types.RouterID{1, 1, 1, 1},
		Areas:   []types.AreaID{types.BackboneArea},
		Results: map[types.AreaID]*ospfspf.Result{types.BackboneArea: res},
	}

	routes, _ := v6Strategy{}.ComputeInterArea(in)
	if len(routes) != 1 {
		t.Fatalf("inter-area routes = %d, want 1", len(routes))
	}
	r := routes[0]
	if r.Prefix != netip.MustParsePrefix("2001:db8:5::/64") {
		t.Errorf("prefix = %s, want 2001:db8:5::/64", r.Prefix)
	}
	if r.Metric != 30 {
		t.Errorf("metric = %d, want 30 (10 to ABR + 20 summary)", r.Metric)
	}
	if r.Type != ospfspf.RouteInterArea {
		t.Errorf("type = %v, want inter-area", r.Type)
	}
	if len(r.NextHops) != 1 || r.NextHops[0].Addr != nh {
		t.Errorf("next-hops = %v, want [fe80::2] (inherited from the ABR)", r.NextHops)
	}
}

func TestOSPFv6ComputeExternal(t *testing.T) {
	// An ASBR 2.2.2.2 originates an OSPFv3 AS-External-LSA (0x4005) for 2001:db8:7::/64,
	// metric 40, external-type-2, no forwarding address (forward via the ASBR).
	asbr := types.RouterID{2, 2, 2, 2}
	pfx, ok := netipToV6Prefix(netip.MustParsePrefix("2001:db8:7::/64"), 0)
	if !ok {
		t.Fatal("netipToV6Prefix failed")
	}
	ext := ospfv3packet.LSA{
		Header: ospfv3packet.LSAHeader{
			Age: 1, Type: ospfv3types.LSTypeASExternal,
			LinkStateID: ospfv3types.LinkStateID{0, 0, 0, 1}, AdvertisingRouter: ospfv3types.RouterID(asbr),
			Sequence: ospfv3types.InitialSequenceNumber,
		},
		External: &ospfv3packet.ExternalLSA{ExternalType2: true, Metric: 40, Prefix: pfx},
	}
	raw := make([]byte, (&ext).EncodedLen())
	(&ext).WriteTo(raw, 0)
	hdr := v6LSAHeaderToNeutral(ext.Header)
	src := fakeV6Source{
		headers: []packet.LSAHeader{hdr},
		lsas:    map[types.LSAKey]packet.LSA{hdr.Key(): {Header: hdr, RawBytes: raw}},
	}

	// The ASBR is reachable at cost 10 with next-hop link-local fe80::2.
	nh := netip.MustParseAddr("fe80::2")
	in := ospfspf.ExternalInput{
		Source:        src,
		Root:          types.RouterID{1, 1, 1, 1},
		BorderRouters: []ospfspf.BorderRouterEntry{{RouterID: asbr, Kind: ospfspf.BorderRouterASBR, Metric: 10, NextHops: []ospfspf.NextHop{{Addr: nh}}}},
	}

	routes := v6Strategy{}.ComputeExternal(in)
	if len(routes) != 1 {
		t.Fatalf("external routes = %d, want 1", len(routes))
	}
	r := routes[0]
	if r.Prefix != netip.MustParsePrefix("2001:db8:7::/64") {
		t.Errorf("prefix = %s, want 2001:db8:7::/64", r.Prefix)
	}
	if r.Type != ospfspf.RouteExternalType2 {
		t.Errorf("type = %v, want external-type-2", r.Type)
	}
	if r.Metric != 40 {
		t.Errorf("metric = %d, want 40 (E2 = advertised metric only)", r.Metric)
	}
	if len(r.NextHops) != 1 || r.NextHops[0].Addr != nh {
		t.Errorf("next-hops = %v, want [fe80::2] via the ASBR", r.NextHops)
	}
}

func TestOSPFv6ComputeExternalNSSA(t *testing.T) {
	// An NSSA internal ASBR 2.2.2.2 originates an OSPFv3 NSSA-LSA (0x2007) for
	// 2001:db8:8::/64, metric 50, external-type-2, with the prefix P-bit set (propagate).
	asbr := types.RouterID{2, 2, 2, 2}
	nssaArea := types.AreaID{0, 0, 0, 1}
	p, ok := netipToV6Prefix(netip.MustParsePrefix("2001:db8:8::/64"), 0)
	if !ok {
		t.Fatal("netipToV6Prefix failed")
	}
	p.Options = ospfv3types.OptPrefixP // P-bit: ask the NSSA ABR to translate to Type 5
	nssa := ospfv3packet.LSA{
		Header: ospfv3packet.LSAHeader{
			Age: 1, Type: ospfv3types.LSTypeNSSA,
			LinkStateID: ospfv3types.LinkStateID{0, 0, 0, 1}, AdvertisingRouter: ospfv3types.RouterID(asbr),
			Sequence: ospfv3types.InitialSequenceNumber,
		},
		External: &ospfv3packet.ExternalLSA{ExternalType2: true, Metric: 50, Prefix: p},
	}
	raw := make([]byte, (&nssa).EncodedLen())
	(&nssa).WriteTo(raw, 0)
	hdr := v6LSAHeaderToNeutral(nssa.Header)
	src := fakeV6Source{
		headers: []packet.LSAHeader{hdr},
		lsas:    map[types.LSAKey]packet.LSA{hdr.Key(): {Header: hdr, RawBytes: raw}},
	}

	// The NSSA ASBR is reachable intra-area at cost 10 with next-hop link-local fe80::2.
	nh := netip.MustParseAddr("fe80::2")
	in := ospfspf.ExternalInput{
		Source:        src,
		Root:          types.RouterID{1, 1, 1, 1},
		NSSAAreas:     []types.AreaID{nssaArea},
		BorderRouters: []ospfspf.BorderRouterEntry{{RouterID: asbr, Kind: ospfspf.BorderRouterASBR, Metric: 10, NextHops: []ospfspf.NextHop{{Addr: nh}}}},
	}

	routes := v6Strategy{}.ComputeExternal(in)
	if len(routes) != 1 {
		t.Fatalf("NSSA external routes = %d, want 1", len(routes))
	}
	r := routes[0]
	if r.Prefix != netip.MustParsePrefix("2001:db8:8::/64") {
		t.Errorf("prefix = %s, want 2001:db8:8::/64", r.Prefix)
	}
	if r.Type != ospfspf.RouteExternalType2 {
		t.Errorf("type = %v, want external-type-2", r.Type)
	}
	if r.Metric != 50 {
		t.Errorf("metric = %d, want 50 (E2 = advertised metric)", r.Metric)
	}
	if r.AreaID != nssaArea {
		t.Errorf("origin area = %v, want the NSSA area %v", r.AreaID, nssaArea)
	}
	if len(r.NextHops) != 1 || r.NextHops[0].Addr != nh {
		t.Errorf("next-hops = %v, want [fe80::2] via the NSSA ASBR", r.NextHops)
	}
}

// TestV6PrefixToNetipAFWidth pins RFC 5838 §2.7: v6PrefixToNetip decodes into a 4-byte
// address for an IPv4 AF and a 16-byte address for an IPv6 AF.
// RFC requirement: RFC5838-2.3-1 negative -- a prefix length wider than the instance address family (a 64-bit prefix under an IPv4 AF) is rejected, so a non-conforming prefix is skipped and never enters the route computation.
func TestV6PrefixToNetipAFWidth(t *testing.T) {
	v4 := ospfv3packet.Prefix{Length: 24, Address: []byte{10, 20, 30, 0}}
	got, ok := v6PrefixToNetip(v4, afIPv4Unicast)
	if !ok || got != netip.MustParsePrefix("10.20.30.0/24") {
		t.Fatalf("IPv4 AF: got %v ok=%v, want 10.20.30.0/24", got, ok)
	}
	if !got.Addr().Is4() {
		t.Errorf("IPv4 AF produced a non-4-byte address: %v", got.Addr())
	}
	v6 := ospfv3packet.Prefix{Length: 64, Address: []byte{0x20, 0x01, 0x0d, 0xb8, 0, 2, 0, 0}}
	got6, ok := v6PrefixToNetip(v6, afIPv6Unicast)
	if !ok || got6 != netip.MustParsePrefix("2001:db8:2::/64") {
		t.Fatalf("IPv6 AF: got %v ok=%v, want 2001:db8:2::/64", got6, ok)
	}
	if !got6.Addr().Is6() {
		t.Errorf("IPv6 AF produced a non-16-byte address: %v", got6.Addr())
	}
	// A prefix length wider than IPv4 rejects the LSA under an IPv4 AF.
	if _, ok := v6PrefixToNetip(ospfv3packet.Prefix{Length: 64, Address: make([]byte, 8)}, afIPv4Unicast); ok {
		t.Error("IPv4 AF accepted a 64-bit prefix length")
	}
}

// TestIPv4OverV3BuildRoutes pins AC-8/AC-9: an IPv4 prefix carried in an OSPFv3
// Intra-Area-Prefix-LSA on an IPv4-unicast instance decodes to a 4-byte netip.Prefix and
// inherits the SPF next-hop resolved from the adjacency (RFC 5838 §2.7).
// RFC requirement: RFC5838-2.3-1 positive -- an IPv4 prefix conforming to the IPv4-unicast instance is used in the route computation and produces an installed IPv4 route.
func TestIPv4OverV3BuildRoutes(t *testing.T) {
	wire, ok := netipToV6Prefix(netip.MustParsePrefix("10.4.0.0/24"), 5)
	if !ok {
		t.Fatal("netipToV6Prefix rejected an IPv4 prefix")
	}
	if wire.Length.ByteLen() != 4 {
		t.Fatalf("IPv4 /24 encoded in %d bytes, want 4 (one word)", wire.Length.ByteLen())
	}
	iap := ospfv3packet.LSA{
		Header: ospfv3packet.LSAHeader{
			Age: 1, Type: ospfv3types.LSTypeIntraAreaPrefix,
			LinkStateID: ospfv3types.LinkStateID{0, 0, 0, 1}, AdvertisingRouter: ospfv3types.RouterID{2, 2, 2, 2},
			Sequence: ospfv3types.InitialSequenceNumber,
		},
		IntraAreaPfx: &ospfv3packet.IntraAreaPrefixLSA{
			ReferencedLSType:    ospfv3types.LSTypeRouter,
			ReferencedAdvRouter: ospfv3types.RouterID{2, 2, 2, 2},
			Prefixes:            []ospfv3packet.Prefix{wire},
		},
	}
	raw := make([]byte, (&iap).EncodedLen())
	(&iap).WriteTo(raw, 0)
	hdr := v6LSAHeaderToNeutral(iap.Header)
	src := fakeV6Source{
		headers: []packet.LSAHeader{hdr},
		lsas:    map[types.LSAKey]packet.LSA{hdr.Key(): {Header: hdr, RawBytes: raw}},
	}
	nh := netip.MustParseAddr("fe80::2")
	res := &ospfspf.Result{
		Area: types.BackboneArea,
		Nodes: map[ospfspf.VertexID]*ospfspf.NodeResult{
			{Kind: ospfspf.VertexRouter, Router: types.RouterID{2, 2, 2, 2}}: {Metric: 10, NextHops: []ospfspf.NextHop{{Addr: nh}}},
		},
	}
	routes := v6BuildRoutes(src, res, afIPv4Unicast)
	if len(routes) != 1 {
		t.Fatalf("routes = %d, want 1", len(routes))
	}
	r := routes[0]
	if r.Prefix != netip.MustParsePrefix("10.4.0.0/24") || !r.Prefix.Addr().Is4() {
		t.Errorf("prefix = %s, want 10.4.0.0/24 (4-byte IPv4)", r.Prefix)
	}
	if len(r.NextHops) != 1 || r.NextHops[0].Addr != nh {
		t.Errorf("next-hops = %v, want [fe80::2] (adjacency link-local, RFC 5838 §2.7)", r.NextHops)
	}
}

// TestV6ForwardingAddrAFWidth pins RFC 5838 §2.6: a received AS-external / NSSA forwarding
// address is rendered at the instance address family's width. An IPv4 AF reads the IPv4
// address from the leading 32 bits of the 128-bit field; an IPv6 AF reads the full 128 bits.
// Origination writes the IPv4 address into the leading 4 octets and zeroes the rest
// (origination_v6_nssa.go:29-31), so for an IPv4 AF only the leading 32 bits are significant.
// RFC requirement: RFC5838-2.6-1 positive -- an IPv4-unicast AF renders the forwarding address as the IPv4 address held in the leading 32 bits of the 128-bit field.
// RFC requirement: RFC5838-2.6-1 negative -- an IPv6 AF renders the full 128-bit forwarding address, so the IPv4 width is applied only for IPv4 address families.
// RFC requirement: RFC5838-2.6-2 positive -- for an IPv4 AF only the leading 32 bits are significant; trailing octets, which origination sets to zero, do not alter the rendered IPv4 address.
func TestV6ForwardingAddrAFWidth(t *testing.T) {
	// IPv4 in the leading 4 octets, remaining 12 zero -- the shape origination writes.
	var fa [16]byte
	copy(fa[:4], []byte{192, 0, 2, 7})

	if got := v6ForwardingAddr(fa, afIPv4Unicast); !got.Is4() || got != netip.MustParseAddr("192.0.2.7") {
		t.Errorf("IPv4 AF forwarding address = %v (is4=%v), want 192.0.2.7", got, got.Is4())
	}

	// An IPv6 AF must consume all 16 octets, never truncate to the leading 4.
	got6 := v6ForwardingAddr(fa, afIPv6Unicast)
	if !got6.Is6() || got6.Is4() {
		t.Errorf("IPv6 AF forwarding address = %v, want a 16-byte IPv6 address", got6)
	}
	if got6 == netip.MustParseAddr("192.0.2.7") {
		t.Errorf("IPv6 AF wrongly rendered the forwarding address at IPv4 width: %v", got6)
	}

	// Trailing octets are not significant for an IPv4 AF: only the leading 32 bits carry the
	// IPv4 address, matching the remaining-bits-zero origination convention.
	noisy := fa
	noisy[4], noisy[15] = 0xff, 0xff
	if got := v6ForwardingAddr(noisy, afIPv4Unicast); got != netip.MustParseAddr("192.0.2.7") {
		t.Errorf("IPv4 AF forwarding address with non-zero trailing octets = %v, want 192.0.2.7 (leading 32 bits only)", got)
	}
}

// TestIPv4OverV3NextHop pins AC-8/A-7: the IPv4-over-OSPFv3 next-hop is resolved from the
// adjacency table, exactly like IPv6 (OSPFv3 forms adjacencies over IPv6 link-local). The
// v6NextHop bound to an IPv4-unicast engine resolves a neighbor's address by Router ID.
func TestIPv4OverV3NextHop(t *testing.T) {
	engV4u := newEngineWithCodecAF(nil, v6Codec{}, afIPv4Unicast)
	if engV4u.af != afIPv4Unicast {
		t.Fatalf("engine af = %s, want ipv4-unicast", engV4u.af)
	}
	if engV4u.installFamily().AFI != afIPv4Unicast.family().AFI {
		t.Fatalf("installFamily AFI = %v, want IPv4", engV4u.installFamily().AFI)
	}
	src := v6Strategy{eng: engV4u}.NextHopSource()
	nh, ok := src.(v6NextHop)
	if !ok {
		t.Fatalf("NextHopSource = %T, want v6NextHop (adjacency-resolved)", src)
	}
	if nh.neighbors != engV4u.neighbors {
		t.Error("v6NextHop is not bound to the engine's neighbor table")
	}
	// An unknown neighbor resolves to no next-hop (no adjacency yet).
	if _, ok := nh.P2PNextHop(nil, types.RouterID{9, 9, 9, 9}, types.RouterID{}); ok {
		t.Error("resolved a next-hop for an unknown neighbor")
	}
}

func TestOSPFv6BuildRoutes(t *testing.T) {
	// An Intra-Area-Prefix-LSA from 2.2.2.2 referencing its own Router-LSA, advertising
	// 2001:db8:2::/64 with prefix-metric 5.
	iap := ospfv3packet.LSA{
		Header: ospfv3packet.LSAHeader{
			Age:               1,
			Type:              ospfv3types.LSTypeIntraAreaPrefix,
			LinkStateID:       ospfv3types.LinkStateID{0, 0, 0, 1},
			AdvertisingRouter: ospfv3types.RouterID{2, 2, 2, 2},
			Sequence:          ospfv3types.InitialSequenceNumber,
		},
		IntraAreaPfx: &ospfv3packet.IntraAreaPrefixLSA{
			ReferencedLSType:    ospfv3types.LSTypeRouter,
			ReferencedAdvRouter: ospfv3types.RouterID{2, 2, 2, 2},
			Prefixes: []ospfv3packet.Prefix{{
				Length:  64,
				Field16: 5,
				Address: []byte{0x20, 0x01, 0x0d, 0xb8, 0x00, 0x02, 0x00, 0x00},
			}},
		},
	}
	raw := make([]byte, (&iap).EncodedLen())
	(&iap).WriteTo(raw, 0)
	hdr := v6LSAHeaderToNeutral(iap.Header)
	src := fakeV6Source{
		headers: []packet.LSAHeader{hdr},
		lsas:    map[types.LSAKey]packet.LSA{hdr.Key(): {Header: hdr, RawBytes: raw}},
	}

	// SPF result: 2.2.2.2 reached at cost 10 with next-hop link-local fe80::2.
	nh := netip.MustParseAddr("fe80::2")
	res := &ospfspf.Result{
		Area: types.BackboneArea,
		Nodes: map[ospfspf.VertexID]*ospfspf.NodeResult{
			{Kind: ospfspf.VertexRouter, Router: types.RouterID{2, 2, 2, 2}}: {Metric: 10, NextHops: []ospfspf.NextHop{{Addr: nh}}},
		},
	}

	routes := v6BuildRoutes(src, res, afIPv6Unicast)
	if len(routes) != 1 {
		t.Fatalf("routes = %d, want 1", len(routes))
	}
	r := routes[0]
	if r.Prefix != netip.MustParsePrefix("2001:db8:2::/64") {
		t.Errorf("prefix = %s, want 2001:db8:2::/64", r.Prefix)
	}
	if r.Metric != 15 {
		t.Errorf("metric = %d, want 15 (10 vertex + 5 prefix)", r.Metric)
	}
	if len(r.NextHops) != 1 || r.NextHops[0].Addr != nh {
		t.Errorf("next-hops = %v, want [fe80::2]", r.NextHops)
	}
	if r.Type != ospfspf.RouteIntraArea {
		t.Errorf("type = %v, want intra-area", r.Type)
	}
}

func v6srcFromLSAs(t *testing.T, lsas ...ospfv3packet.LSA) fakeV6Source {
	t.Helper()
	src := fakeV6Source{lsas: map[types.LSAKey]packet.LSA{}}
	for i := range lsas {
		raw := make([]byte, (&lsas[i]).EncodedLen())
		(&lsas[i]).WriteTo(raw, 0)
		hdr := v6LSAHeaderToNeutral(lsas[i].Header)
		src.headers = append(src.headers, hdr)
		src.lsas[hdr.Key()] = packet.LSA{Header: hdr, RawBytes: raw}
	}
	return src
}

func TestOSPFv6BuildGraphBroadcast(t *testing.T) {
	// A broadcast segment: DR 2.2.2.2 (Interface ID 5) originates a Network-LSA attaching
	// both routers; 1.1.1.1 and the DR each carry a transit link naming the DR. BuildGraph
	// must key the Network vertex by a synthetic handle, store the real (DR-RID, DR-iface-ID),
	// and join both routers' transit links to it so the shared Dijkstra reaches across.
	dr := types.RouterID{2, 2, 2, 2}
	other := types.RouterID{1, 1, 1, 1}
	const drIfaceID = 5
	opts := ospfv3types.OptV6 | ospfv3types.OptR
	netLSA := ospfv3packet.LSA{
		Header: ospfv3packet.LSAHeader{
			Age: 1, Type: ospfv3types.LSTypeNetwork,
			LinkStateID: ospfv3types.LinkStateID{0, 0, 0, drIfaceID}, AdvertisingRouter: ospfv3types.RouterID(dr),
			Sequence: ospfv3types.InitialSequenceNumber,
		},
		Network: &ospfv3packet.NetworkLSA{Options: opts, AttachedRouters: []ospfv3types.RouterID{ospfv3types.RouterID(dr), ospfv3types.RouterID(other)}},
	}
	drRouter := ospfv3packet.LSA{
		Header: ospfv3packet.LSAHeader{Age: 1, Type: ospfv3types.LSTypeRouter, AdvertisingRouter: ospfv3types.RouterID(dr), Sequence: ospfv3types.InitialSequenceNumber},
		Router: &ospfv3packet.RouterLSA{Options: opts, Links: []ospfv3packet.RouterLink{{Type: ospfv3packet.RouterLinkTypeTransit, Metric: 10, InterfaceID: drIfaceID, NeighborInterfaceID: drIfaceID, NeighborRouterID: ospfv3types.RouterID(dr)}}},
	}
	otherRouter := ospfv3packet.LSA{
		Header: ospfv3packet.LSAHeader{Age: 1, Type: ospfv3types.LSTypeRouter, AdvertisingRouter: ospfv3types.RouterID(other), Sequence: ospfv3types.InitialSequenceNumber},
		Router: &ospfv3packet.RouterLSA{Options: opts, Links: []ospfv3packet.RouterLink{{Type: ospfv3packet.RouterLinkTypeTransit, Metric: 10, InterfaceID: 9, NeighborInterfaceID: drIfaceID, NeighborRouterID: ospfv3types.RouterID(dr)}}},
	}
	src := v6srcFromLSAs(t, netLSA, drRouter, otherRouter)

	g := v6Strategy{}.BuildGraph(src, types.BackboneArea)
	if len(g.Networks) != 1 {
		t.Fatalf("network vertices = %d, want 1", len(g.Networks))
	}
	var nv *ospfspf.NetworkVertex
	var synID types.LinkStateID
	for id, v := range g.Networks {
		nv, synID = v, id
	}
	if nv.AdvertisingDR != dr {
		t.Errorf("network DR = %v, want %v", nv.AdvertisingDR, dr)
	}
	if nv.DRInterfaceID != (types.LinkStateID{0, 0, 0, drIfaceID}) {
		t.Errorf("network DR iface id = %v, want {0,0,0,5}", nv.DRInterfaceID)
	}
	if len(nv.AttachedRouters) != 2 {
		t.Errorf("attached routers = %d, want 2", len(nv.AttachedRouters))
	}
	for _, rid := range []types.RouterID{dr, other} {
		rv := g.Routers[rid]
		if rv == nil || len(rv.Links) != 1 {
			t.Fatalf("router %v: links missing", rid)
		}
		if rv.Links[0].Type != packet.RouterLinkTypeTransit || rv.Links[0].LinkID != synID {
			t.Errorf("router %v transit link = %+v, want transit LinkID=%v (the network)", rid, rv.Links[0], synID)
		}
	}
	// The shared Dijkstra must reach the DR from `other` across the transit network.
	res := ospfspf.Compute(g, other, 8)
	if res.Nodes[ospfspf.VertexID{Kind: ospfspf.VertexRouter, Router: dr}] == nil {
		t.Errorf("DR %v not reached over the broadcast segment", dr)
	}
}

func TestOSPFv6InstallNetworkReferencedPrefix(t *testing.T) {
	dr := types.RouterID{2, 2, 2, 2}
	networkLSID := types.LinkStateID{0, 0, 0, 7}
	prefix, ok := netipToV6Prefix(netip.MustParsePrefix("2001:db8:77::/64"), 3)
	if !ok {
		t.Fatal("netipToV6Prefix failed")
	}
	iap := ospfv3packet.LSA{
		Header: ospfv3packet.LSAHeader{
			Age:               1,
			Type:              ospfv3types.LSTypeIntraAreaPrefix,
			LinkStateID:       ospfv3types.LinkStateID{0, 0, 0, 7},
			AdvertisingRouter: ospfv3types.RouterID(dr),
			Sequence:          ospfv3types.InitialSequenceNumber,
		},
		IntraAreaPfx: &ospfv3packet.IntraAreaPrefixLSA{
			ReferencedLSType:      ospfv3types.LSTypeNetwork,
			ReferencedLinkStateID: ospfv3types.LinkStateID(networkLSID),
			ReferencedAdvRouter:   ospfv3types.RouterID(dr),
			Prefixes:              []ospfv3packet.Prefix{prefix},
		},
	}
	raw := make([]byte, (&iap).EncodedLen())
	(&iap).WriteTo(raw, 0)
	hdr := v6LSAHeaderToNeutral(iap.Header)
	src := fakeV6Source{
		headers: []packet.LSAHeader{hdr},
		lsas:    map[types.LSAKey]packet.LSA{hdr.Key(): {Header: hdr, RawBytes: raw}},
	}

	graph := ospfspf.NewGraph(types.BackboneArea)
	graph.Networks[types.LinkStateID{0, 0, 0, 1}] = &ospfspf.NetworkVertex{
		ID:            types.LinkStateID{0, 0, 0, 1},
		AdvertisingDR: dr,
		DRInterfaceID: networkLSID,
	}
	nh := netip.MustParseAddr("fe80::2")
	vid := ospfspf.VertexID{Kind: ospfspf.VertexNetwork, Network: types.LinkStateID{0, 0, 0, 1}}
	res := &ospfspf.Result{
		Area:  types.BackboneArea,
		Graph: graph,
		Nodes: map[ospfspf.VertexID]*ospfspf.NodeResult{
			vid: {ID: vid, Metric: 10, NextHops: []ospfspf.NextHop{{Addr: nh}}},
		},
	}

	routes := v6BuildRoutes(src, res, afIPv6Unicast)
	if len(routes) != 1 {
		t.Fatalf("routes = %d, want 1", len(routes))
	}
	route := routes[0]
	if route.Prefix != netip.MustParsePrefix("2001:db8:77::/64") {
		t.Fatalf("prefix = %s, want 2001:db8:77::/64", route.Prefix)
	}
	if route.Metric != 13 {
		t.Fatalf("metric = %d, want network cost 10 + prefix metric 3", route.Metric)
	}
	if len(route.NextHops) != 1 || route.NextHops[0].Addr != nh {
		t.Fatalf("next-hops = %+v, want %s", route.NextHops, nh)
	}
}
