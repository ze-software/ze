// VALIDATES: spec-ospf-af-unify Phase 5 -- the v6 AFPrefixStrategy decodes the
// address-free OSPFv3 Router-LSA into the shared SPF graph: a point-to-point link
// keys the neighbor by Router ID so the AF-agnostic Dijkstra (two-way check, metric)
// runs unchanged. PREVENTS: the v6 engine falling back to the OSPFv2 BuildGraph,
// which would misparse a v6 LSA (its 16-bit LS Type sits where v2 has Options+Type).
package ospf

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	ospfspf "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/spf"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
	ospfv3packet "codeberg.org/thomas-mangin/ze/internal/plugins/ospfv3/packet"
	ospfv3types "codeberg.org/thomas-mangin/ze/internal/plugins/ospfv3/types"
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

	routes := v6BuildRoutes(src, res)
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

	routes := v6BuildRoutes(src, res)
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
