// Design: docs/architecture/ospf/ospf-ext-15-multi-af.md -- RFC 5838 address families over OSPFv3.
// Related: afstrategy_v6.go -- v6BuildRoutes, the route computation these tests drive.
// Related: virtuallink_v6.go -- v6ResolveVirtualEndpointLocked, the virtual-link endpoint.
//
// VALIDATES: RFC 5838 section 2.3 through the route computation (a prefix wider than the
// instance's IPv4 AF yields no route while its conforming sibling does), and section 2.8 on
// THIS router's side of an OSPFv3 virtual link (no global address, no endpoint).
// PREVENTS: a route builder that ignores the AF-width check, and a virtual link sourced from a
// link-local address.
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

// RFC requirement: RFC5838-2.3-1 negative -- an Intra-Area-Prefix-LSA on an IPv4-unicast
// instance carries a conforming 10.4.0.0/24 and a non-conforming 64-bit prefix; the route
// computation installs exactly one route, 10.4.0.0/24, and none for the 64-bit prefix.
func TestRFC5838NonConformingPrefixNotInRouteComputation(t *testing.T) {
	// Goal: the AF-width check is applied by the route computation itself. Method: one LSA
	// holding both prefixes, so the conforming one proves the LSA was read.
	conforming, ok := netipToV6Prefix(netip.MustParsePrefix("10.4.0.0/24"), 5)
	if !ok {
		t.Fatal("netipToV6Prefix rejected an IPv4 prefix")
	}
	wide := ospfv3packet.Prefix{Length: 64, Address: []byte{0x20, 0x01, 0x0d, 0xb8, 0, 4, 0, 0}}
	iap := ospfv3packet.LSA{
		Header: ospfv3packet.LSAHeader{
			Age: 1, Type: ospfv3types.LSTypeIntraAreaPrefix,
			LinkStateID: ospfv3types.LinkStateID{0, 0, 0, 1}, AdvertisingRouter: ospfv3types.RouterID{2, 2, 2, 2},
			Sequence: ospfv3types.InitialSequenceNumber,
		},
		IntraAreaPfx: &ospfv3packet.IntraAreaPrefixLSA{
			ReferencedLSType:    ospfv3types.LSTypeRouter,
			ReferencedAdvRouter: ospfv3types.RouterID{2, 2, 2, 2},
			Prefixes:            []ospfv3packet.Prefix{wide, conforming},
		},
	}
	raw := make([]byte, (&iap).EncodedLen())
	(&iap).WriteTo(raw, 0)
	hdr := v6LSAHeaderToNeutral(iap.Header)
	src := fakeV6Source{
		headers: []packet.LSAHeader{hdr},
		lsas:    map[types.LSAKey]packet.LSA{hdr.Key(): {Header: hdr, RawBytes: raw}},
	}
	res := &ospfspf.Result{
		Area: types.BackboneArea,
		Nodes: map[ospfspf.VertexID]*ospfspf.NodeResult{
			{Kind: ospfspf.VertexRouter, Router: types.RouterID{2, 2, 2, 2}}: {Metric: 10, NextHops: []ospfspf.NextHop{{Addr: netip.MustParseAddr("fe80::2")}}},
		},
	}

	routes := v6BuildRoutes(src, res, afIPv4Unicast)
	if len(routes) != 1 {
		t.Fatalf("routes = %v, want only 10.4.0.0/24: the 64-bit prefix MUST NOT be used", routes)
	}
	if routes[0].Prefix != netip.MustParsePrefix("10.4.0.0/24") {
		t.Fatalf("route prefix = %s, want 10.4.0.0/24", routes[0].Prefix)
	}
}

// rfc5838V6Source encodes each LSA and returns a route-computation source that holds them all.
func rfc5838V6Source(lsas ...ospfv3packet.LSA) fakeV6Source {
	src := fakeV6Source{lsas: make(map[types.LSAKey]packet.LSA, len(lsas))}
	for i := range lsas {
		raw := make([]byte, (&lsas[i]).EncodedLen())
		(&lsas[i]).WriteTo(raw, 0)
		hdr := v6LSAHeaderToNeutral(lsas[i].Header)
		src.headers = append(src.headers, hdr)
		src.lsas[hdr.Key()] = packet.LSA{Header: hdr, RawBytes: raw}
	}
	return src
}

// rfc5838V6Header is an LSA header of type lsType and Link State ID id from router 2.2.2.2.
func rfc5838V6Header(lsType ospfv3types.LSType, id byte) ospfv3packet.LSAHeader {
	return ospfv3packet.LSAHeader{
		Age: 1, Type: lsType, LinkStateID: ospfv3types.LinkStateID{0, 0, 0, id},
		AdvertisingRouter: ospfv3types.RouterID{2, 2, 2, 2}, Sequence: ospfv3types.InitialSequenceNumber,
	}
}

// rfc5838WidePrefix is a 64-bit prefix, which no IPv4 address family can hold.
var rfc5838WidePrefix = ospfv3packet.Prefix{Length: 64, Address: []byte{0x20, 0x01, 0x0d, 0xb8, 0, 4, 0, 0}}

// RFC requirement: RFC5838-2.3-1 negative -- on an IPv4-unicast instance, the inter-area input
// of the route computation reads two Inter-Area-Prefix-LSAs, one for the conforming 10.5.0.0/24
// and one for a non-conforming 64-bit prefix, and yields exactly one summary, 10.5.0.0/24.
func TestRFC5838NonConformingInterAreaPrefixNotInRouteComputation(t *testing.T) {
	// Goal: the inter-area path applies the AF-width check. Method: the conforming LSA beside
	// the non-conforming one proves the reader ran over both.
	conforming, ok := netipToV6Prefix(netip.MustParsePrefix("10.5.0.0/24"), 0)
	if !ok {
		t.Fatal("netipToV6Prefix rejected an IPv4 prefix")
	}
	src := rfc5838V6Source(
		ospfv3packet.LSA{Header: rfc5838V6Header(ospfv3types.LSTypeInterAreaPrefix, 1), InterAreaPfx: &ospfv3packet.InterAreaPrefixLSA{Metric: 7, Prefix: rfc5838WidePrefix}},
		ospfv3packet.LSA{Header: rfc5838V6Header(ospfv3types.LSTypeInterAreaPrefix, 2), InterAreaPfx: &ospfv3packet.InterAreaPrefixLSA{Metric: 7, Prefix: conforming}},
	)

	summaries := v6SummaryReader(src, afIPv4Unicast)(types.BackboneArea)
	if len(summaries) != 1 {
		t.Fatalf("inter-area summaries = %v, want only 10.5.0.0/24: the 64-bit prefix MUST NOT be used", summaries)
	}
	if summaries[0].Prefix != netip.MustParsePrefix("10.5.0.0/24") {
		t.Fatalf("inter-area summary prefix = %s, want 10.5.0.0/24", summaries[0].Prefix)
	}
}

// RFC requirement: RFC5838-2.3-1 negative -- on an IPv4-unicast instance, the external input of
// the route computation refuses an AS-External-LSA and an NSSA-LSA that carry a non-conforming
// 64-bit prefix, and accepts the same LSA types carrying the conforming 10.6.0.0/24.
func TestRFC5838NonConformingExternalPrefixNotInRouteComputation(t *testing.T) {
	// Goal: the AS-external and NSSA path applies the AF-width check. Method: each LS type is
	// read once with the conforming prefix and once with the non-conforming one.
	conforming, ok := netipToV6Prefix(netip.MustParsePrefix("10.6.0.0/24"), 0)
	if !ok {
		t.Fatal("netipToV6Prefix rejected an IPv4 prefix")
	}
	for _, lsType := range []ospfv3types.LSType{ospfv3types.LSTypeASExternal, ospfv3types.LSTypeNSSA} {
		good := ospfv3packet.LSA{Header: rfc5838V6Header(lsType, 1), External: &ospfv3packet.ExternalLSA{Metric: 9, Prefix: conforming}}
		wide := ospfv3packet.LSA{Header: rfc5838V6Header(lsType, 2), External: &ospfv3packet.ExternalLSA{Metric: 9, Prefix: rfc5838WidePrefix}}
		src := rfc5838V6Source(good, wide)
		read := v6ExternalReader(src, afIPv4Unicast)

		rec, ok := read(types.BackboneArea, src.headers[0])
		if !ok || rec.Prefix != netip.MustParsePrefix("10.6.0.0/24") {
			t.Fatalf("%v: conforming record = %v (ok %v), want 10.6.0.0/24", lsType, rec.Prefix, ok)
		}
		if rec, ok := read(types.BackboneArea, src.headers[1]); ok {
			t.Fatalf("%v: record for a 64-bit prefix accepted as %v: it MUST NOT be used", lsType, rec.Prefix)
		}
	}
}

// RFC requirement: RFC5838-2.8-1 negative -- when THIS router advertises only a link-local
// address (fe80::1) in the transit area and the neighbor advertises the global 2001:db8:2::2,
// the virtual-link endpoint does not resolve: no global IPv6 address is associated with the
// local end, so the virtual link is not formed on a link-local source.
func TestRFC5838VirtualLinkRequiresLocalGlobalAddress(t *testing.T) {
	// Goal: the local end needs a global address too. Method: the mirror of
	// TestV6VirtualEndpointRequiresGlobalAddress, with the link-local on this router.
	e := newV6OriginEngine()
	transit := vlArea(t, "0.0.0.1")
	self := vlRID(t, "10.0.0.1")
	neighbor := vlRID(t, "10.0.0.2")
	e.cfg.RouterID = self
	installV6IntraPrefix(t, e.lsdb, transit, self, "fe80::1/128")
	installV6IntraPrefix(t, e.lsdb, transit, neighbor, "2001:db8:2::2/128")

	rt := &virtualLinkRuntime{cfg: virtualLinkConfig{TransitArea: transit, RemoteRouterID: neighbor}}
	if src, dst, ok := e.v6ResolveVirtualEndpointLocked(rt); ok {
		t.Fatalf("virtual-link endpoint resolved src=%v dst=%v with no global IPv6 address on this router", src, dst)
	}
}
