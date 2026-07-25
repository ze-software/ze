// VALIDATES: spec-ospf-af-unify -- OSPFv3 self-origination. The engine originates the
// address-free Router-LSA (adjacencies, RFC 5340 App A.4.3) and the Intra-Area-Prefix-LSA
// (interface prefixes, App A.4.10) through the LSDB's AF-neutral OriginateSelf seam, and
// the bytes round-trip through the OSPFv3 codec with a valid Fletcher checksum.
// PREVENTS: eng6 flooding OSPFv2-encoded self-LSAs that FRR ospf6d would reject (so FRR
// never learns Ze's Router-LSA or routes to Ze's prefixes), and the LSDB mis-routing the
// 16-bit scope-typed v6 LS Types (0x2001/0x2009) away from the area store.
package ospf

import (
	"net/netip"
	"testing"
	"time"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func newV6OriginEngine() *engine {
	return &engine{lsdb: ospflsdb.New(func() time.Time { return time.Unix(0, 0) })}
}

func v6P2PInterface(area types.AreaID, self, neighbor types.RouterID) ospflsdb.InterfaceInfo {
	return ospflsdb.InterfaceInfo{
		Name:        "eth0",
		AreaID:      area,
		NetworkType: ospflsdb.NetworkPointToPoint,
		State:       "point-to-point",
		Cost:        10,
		RouterID:    self,
		InterfaceID: 7, // the OS ifindex, advertised in both the Hello and the Router-LSA link
		Neighbors: []ospflsdb.NeighborInfo{{
			RouterID:    neighbor,
			Address:     netip.MustParseAddr("fe80::1"),
			State:       ospflsdb.NeighborStateFull,
			InterfaceID: 11, // the neighbor's advertised Interface ID, echoed in the link
		}},
	}
}

// v6VirtualInterface is a synthetic OSPFv3 virtual-link interface in the backbone with a
// Full neighbor. InterfaceID 42 is the local virtual-interface ID; the neighbor advertises
// Interface ID 99.
func v6VirtualInterface(area types.AreaID, self, neighbor types.RouterID, cost uint16) ospflsdb.InterfaceInfo {
	return ospflsdb.InterfaceInfo{
		Name:               "*vlink-0.0.0.1-" + neighbor.String(),
		AreaID:             area,
		NetworkType:        ospflsdb.NetworkVirtual,
		State:              "point-to-point",
		VirtualTransitArea: types.AreaID{0, 0, 0, 1},
		Cost:               cost,
		RouterID:           self,
		InterfaceID:        42,
		Neighbors: []ospflsdb.NeighborInfo{{
			RouterID:    neighbor,
			Address:     netip.MustParseAddr("2001:db8:cafe::2"),
			State:       ospflsdb.NeighborStateFull,
			InterfaceID: 99,
		}},
	}
}

func v6VirtualRecord(t *testing.T, links []ospfv3packet.RouterLink) (ospfv3packet.RouterLink, bool) {
	t.Helper()
	for _, l := range links {
		if l.Type == ospfv3packet.RouterLinkTypeVirtual {
			return l, true
		}
	}
	return ospfv3packet.RouterLink{}, false
}

func v6DecodeBackboneRouter(t *testing.T, e *engine, area types.AreaID, router types.RouterID) ospfv3packet.RouterLSA {
	t.Helper()
	lsa, ok := e.lsdb.LookupLSA(area, v6RouterKey(router))
	if !ok {
		t.Fatalf("Router-LSA not installed")
	}
	if !ospfv3packet.VerifyLSAChecksum(lsa.RawBytes) {
		t.Fatalf("v6 Router-LSA Fletcher checksum invalid")
	}
	decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
	if err != nil {
		t.Fatalf("DecodeLSA: %v", err)
	}
	body, err := decoded.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter: %v", err)
	}
	return body
}

// VALIDATES: spec-ospf-ext-7 AC-9 / R-5 -- a Full virtual link adds a RouterLinkTypeVirtual
// record (Interface ID, Neighbor Interface ID, Neighbor Router ID, transit metric) to the
// BACKBONE Router-LSA, WITHOUT the V-bit there (the V-bit belongs to the transit area's
// Router-LSA per RFC 5340 App A.4.3 / RFC 2328 App A.4.2).
func TestVirtualRecordInBackboneRouterLSA(t *testing.T) {
	e := newV6OriginEngine()
	router := types.RouterID{172, 30, 0, 2}
	neighbor := types.RouterID{172, 30, 0, 1}
	opts := ospfv3types.OptV6 | ospfv3types.OptR
	ifaces := []ospflsdb.InterfaceInfo{v6VirtualInterface(types.BackboneArea, router, neighbor, 25)}
	if _, ok := e.v6OriginateRouter(types.BackboneArea, router, opts, ifaces, false, true, false, false); !ok {
		t.Fatalf("v6OriginateRouter returned false")
	}
	body := v6DecodeBackboneRouter(t, e, types.BackboneArea, router)
	if body.Flags&ospfv3packet.RouterFlagV != 0 {
		t.Fatalf("V-bit must NOT be set on the backbone Router-LSA: flags = %#x", body.Flags)
	}
	link, ok := v6VirtualRecord(t, body.Links)
	if !ok {
		t.Fatalf("no RouterLinkTypeVirtual record in %+v", body.Links)
	}
	if link.NeighborRouterID != ospfv3types.RouterID(neighbor) {
		t.Fatalf("virtual neighbor = %v, want %v", link.NeighborRouterID, neighbor)
	}
	if link.InterfaceID != ospfv3types.InterfaceID(42) || link.NeighborInterfaceID != ospfv3types.InterfaceID(99) {
		t.Fatalf("virtual interface IDs = (%d,%d), want (42,99)", link.InterfaceID, link.NeighborInterfaceID)
	}
}

// VALIDATES: spec-ospf-ext-7 AC-9 -- the V-bit is set in the TRANSIT area's Router-LSA
// (virtualEndpoint), the signal a far ABR reads to set TransitCapability (RFC 2328 section
// 16.3); no virtual record appears in the transit-area Router-LSA.
func TestV6VirtualLinkVBitInTransitArea(t *testing.T) {
	e := newV6OriginEngine()
	router := types.RouterID{172, 30, 0, 2}
	transit := types.AreaID{0, 0, 0, 1}
	opts := ospfv3types.OptV6 | ospfv3types.OptR
	// The transit area's Router-LSA is built from its real interfaces (no virtual iface),
	// with virtualEndpoint=true.
	realIface := v6P2PInterface(transit, router, types.RouterID{172, 30, 0, 3})
	if _, ok := e.v6OriginateRouter(transit, router, opts, []ospflsdb.InterfaceInfo{realIface}, false, true, false, true); !ok {
		t.Fatalf("v6OriginateRouter returned false")
	}
	lsa, ok := e.lsdb.LookupLSA(transit, v6RouterKey(router))
	if !ok {
		t.Fatalf("transit Router-LSA missing")
	}
	decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
	if err != nil {
		t.Fatalf("DecodeLSA: %v", err)
	}
	body, err := decoded.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter: %v", err)
	}
	if body.Flags&ospfv3packet.RouterFlagV == 0 {
		t.Fatalf("V-bit not set in the transit-area Router-LSA: flags = %#x", body.Flags)
	}
	if _, ok := v6VirtualRecord(t, body.Links); ok {
		t.Fatalf("virtual record leaked into the transit-area Router-LSA: %+v", body.Links)
	}
}

// VALIDATES: spec-ospf-ext-7 AC-14 / A-13 -- the advertised virtual-link metric equals the
// transit-area path cost, never a configured cost.
func TestVirtualLinkCostEqualsTransitCost(t *testing.T) {
	e := newV6OriginEngine()
	router := types.RouterID{172, 30, 0, 2}
	neighbor := types.RouterID{172, 30, 0, 1}
	ifaces := []ospflsdb.InterfaceInfo{v6VirtualInterface(types.BackboneArea, router, neighbor, 33)}
	if _, ok := e.v6OriginateRouter(types.BackboneArea, router, ospfv3types.OptV6|ospfv3types.OptR, ifaces, false, true, false, false); !ok {
		t.Fatalf("v6OriginateRouter returned false")
	}
	body := v6DecodeBackboneRouter(t, e, types.BackboneArea, router)
	link, ok := v6VirtualRecord(t, body.Links)
	if !ok {
		t.Fatalf("no virtual record")
	}
	if link.Metric != 33 {
		t.Fatalf("virtual metric = %d, want the transit cost 33", link.Metric)
	}
}

// VALIDATES: spec-ospf-ext-7 AC-5 -- a virtual link whose adjacency is not Full originates
// no virtual record (the record is withdrawn when the link is down / re-forming).
func TestVirtualLinkWithdrawnWhenDown(t *testing.T) {
	e := newV6OriginEngine()
	router := types.RouterID{172, 30, 0, 2}
	neighbor := types.RouterID{172, 30, 0, 1}
	iface := v6VirtualInterface(types.BackboneArea, router, neighbor, 25)
	iface.Neighbors[0].State = ospflsdb.NeighborStateExchange
	if _, ok := e.v6OriginateRouter(types.BackboneArea, router, ospfv3types.OptV6|ospfv3types.OptR, []ospflsdb.InterfaceInfo{iface}, false, true, false, false); !ok {
		t.Fatalf("v6OriginateRouter returned false")
	}
	body := v6DecodeBackboneRouter(t, e, types.BackboneArea, router)
	if _, ok := v6VirtualRecord(t, body.Links); ok {
		t.Fatalf("virtual record originated for a non-Full link: %+v", body.Links)
	}
}

func TestOSPFv6OriginateRouterLSA(t *testing.T) {
	e := newV6OriginEngine()
	router := types.RouterID{172, 30, 0, 2}
	neighbor := types.RouterID{172, 30, 0, 1}
	area := types.BackboneArea
	opts := ospfv3types.OptV6 | ospfv3types.OptR
	ifaces := []ospflsdb.InterfaceInfo{v6P2PInterface(area, router, neighbor)}

	h, ok := e.v6OriginateRouter(area, router, opts, ifaces, false, false, false, false)
	if !ok {
		t.Fatalf("v6OriginateRouter returned false")
	}
	if h.Type != types.LSType(ospfv3types.LSTypeRouter) {
		t.Errorf("neutral type = %#x, want 0x2001", uint16(h.Type))
	}
	if h.Checksum == 0 || h.Length == 0 {
		t.Errorf("header Length/Checksum not finalized: %+v", h)
	}

	lsa, found := e.lsdb.LookupLSA(area, h.Key())
	if !found {
		t.Fatalf("originated Router-LSA not installed (store routing for v6 0x2001)")
	}
	if !ospfv3packet.VerifyLSAChecksum(lsa.RawBytes) {
		t.Fatalf("originated v6 Router-LSA Fletcher checksum invalid")
	}
	decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
	if err != nil {
		t.Fatalf("DecodeLSA: %v", err)
	}
	body, err := decoded.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter: %v", err)
	}
	if body.Options != opts {
		t.Errorf("options = %#x, want %#x", uint32(body.Options), uint32(opts))
	}
	if len(body.Links) != 1 {
		t.Fatalf("links = %d, want 1 (one Full p2p neighbor)", len(body.Links))
	}
	l := body.Links[0]
	if l.Type != ospfv3packet.RouterLinkTypeP2P {
		t.Errorf("link type = %d, want p2p", l.Type)
	}
	if l.NeighborRouterID != ospfv3types.RouterID(neighbor) {
		t.Errorf("neighbor router id = %v, want %v", l.NeighborRouterID, neighbor)
	}
	if l.Metric != 10 {
		t.Errorf("metric = %d, want 10", l.Metric)
	}
	if l.InterfaceID != ospfv3types.InterfaceID(7) {
		t.Errorf("link interface id = %d, want 7 (the interface ifindex, matching the Hello)", l.InterfaceID)
	}
	if l.NeighborInterfaceID != ospfv3types.InterfaceID(11) {
		t.Errorf("neighbor interface id = %d, want 11 (the neighbor's advertised Interface ID)", l.NeighborInterfaceID)
	}

	// An unchanged topology must re-originate nothing (idempotent, no needless flood).
	if _, ok := e.v6OriginateRouter(area, router, opts, ifaces, false, false, false, false); ok {
		t.Errorf("second v6OriginateRouter re-originated an unchanged Router-LSA")
	}
}

func TestOSPFv6OriginateRouterLSAABRNtBits(t *testing.T) {
	// RFC 5340 App A.4.3: OSPFv3 keeps the ABR, ASBR, and NSSA-translator
	// indicators in the Router-LSA flag byte. The v6 builder must not drop the
	// B or Nt bits while reusing the v4 NSSA translator policy.
	e := newV6OriginEngine()
	router := types.RouterID{172, 30, 0, 2}
	area := types.AreaID{0, 0, 0, 9}
	opts := ospfv3types.OptV6 | ospfv3types.OptR | ospfv3types.OptN

	h, ok := e.v6OriginateRouter(area, router, opts, nil, false, true, true, false)
	if !ok {
		t.Fatal("v6OriginateRouter returned false")
	}
	lsa, ok := e.lsdb.LookupLSA(area, h.Key())
	if !ok {
		t.Fatal("Router-LSA not installed")
	}
	decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
	if err != nil {
		t.Fatalf("DecodeLSA: %v", err)
	}
	body, err := decoded.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter: %v", err)
	}
	if body.Flags&ospfv3packet.RouterFlagB == 0 {
		t.Fatalf("B-bit not set for OSPFv3 ABR: flags = %#x", body.Flags)
	}
	if body.Flags&ospfv3packet.RouterFlagNt == 0 {
		t.Fatalf("Nt-bit not set for OSPFv3 NSSA translator: flags = %#x", body.Flags)
	}
	if body.Flags&ospfv3packet.RouterFlagE != 0 {
		t.Fatalf("E-bit set without an external self-LSA: flags = %#x", body.Flags)
	}
}

func TestOSPFv6OriginateRouterLSAMaxMetric(t *testing.T) {
	// RFC 6987: max-metric sets every non-stub link to 0xffff so transit drains away.
	e := newV6OriginEngine()
	router := types.RouterID{172, 30, 0, 2}
	area := types.BackboneArea
	ifaces := []ospflsdb.InterfaceInfo{v6P2PInterface(area, router, types.RouterID{172, 30, 0, 1})}

	h, ok := e.v6OriginateRouter(area, router, ospfv3types.OptV6|ospfv3types.OptR, ifaces, true, false, false, false)
	if !ok {
		t.Fatalf("v6OriginateRouter returned false")
	}
	lsa, _ := e.lsdb.LookupLSA(area, h.Key())
	decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
	if err != nil {
		t.Fatalf("DecodeLSA: %v", err)
	}
	body, err := decoded.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter: %v", err)
	}
	if len(body.Links) != 1 || body.Links[0].Metric != v6MaxLinkMetric {
		t.Errorf("max-metric link metric = %v, want %#x", body.Links, v6MaxLinkMetric)
	}
}

func TestOSPFv6OriginateRouterLSANoFullNeighbor(t *testing.T) {
	// A p2p interface whose neighbor is not Full contributes no link (RFC 5340 App
	// A.4.3): the Router-LSA is still originated, with zero links.
	e := newV6OriginEngine()
	router := types.RouterID{172, 30, 0, 2}
	area := types.BackboneArea
	iface := v6P2PInterface(area, router, types.RouterID{172, 30, 0, 1})
	iface.Neighbors[0].State = ospflsdb.NeighborStateExchange

	h, ok := e.v6OriginateRouter(area, router, ospfv3types.OptV6|ospfv3types.OptR, []ospflsdb.InterfaceInfo{iface}, false, false, false, false)
	if !ok {
		t.Fatalf("v6OriginateRouter returned false")
	}
	lsa, _ := e.lsdb.LookupLSA(area, h.Key())
	decoded, _ := ospfv3packet.DecodeLSA(lsa.RawBytes)
	body, err := decoded.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter: %v", err)
	}
	if len(body.Links) != 0 {
		t.Errorf("links = %d, want 0 (neighbor not Full)", len(body.Links))
	}
}

func TestOSPFv6OriginateIntraAreaPrefix(t *testing.T) {
	e := newV6OriginEngine()
	router := types.RouterID{172, 30, 0, 2}
	area := types.BackboneArea

	p, ok := netipToV6Prefix(netip.MustParsePrefix("2001:db8:2::/64"), 10)
	if !ok {
		t.Fatalf("netipToV6Prefix failed")
	}
	body := ospfv3packet.IntraAreaPrefixLSA{
		ReferencedLSType:    ospfv3types.LSTypeRouter,
		ReferencedAdvRouter: ospfv3types.RouterID(router),
		Prefixes:            []ospfv3packet.Prefix{p},
	}
	bodyBytes := make([]byte, body.EncodedLen())
	body.WriteTo(bodyBytes, 0)
	key := types.LSAKey{Type: types.LSType(ospfv3types.LSTypeIntraAreaPrefix), AdvertisingRouter: router}

	h, ok := e.lsdb.OriginateSelf(area, key, bodyBytes, func(seq types.LSSequenceNumber, purge bool) packet.LSA {
		return v6SelfLSA(ospfv3packet.LSA{
			Header:       v6OriginHeader(ospfv3types.LSTypeIntraAreaPrefix, ospfv3types.LinkStateID{}, router, seq, purge),
			IntraAreaPfx: &body,
		})
	})
	if !ok {
		t.Fatalf("OriginateSelf(Intra-Area-Prefix) returned false")
	}
	if h.Type != types.LSType(ospfv3types.LSTypeIntraAreaPrefix) {
		t.Errorf("neutral type = %#x, want 0x2009", uint16(h.Type))
	}

	lsa, found := e.lsdb.LookupLSA(area, h.Key())
	if !found {
		t.Fatalf("Intra-Area-Prefix-LSA not installed (store routing for v6 0x2009)")
	}
	if !ospfv3packet.VerifyLSAChecksum(lsa.RawBytes) {
		t.Fatalf("Intra-Area-Prefix-LSA Fletcher checksum invalid")
	}
	decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
	if err != nil {
		t.Fatalf("DecodeLSA: %v", err)
	}
	iap, err := decoded.DecodeIntraAreaPrefix()
	if err != nil {
		t.Fatalf("DecodeIntraAreaPrefix: %v", err)
	}
	if iap.ReferencedLSType != ospfv3types.LSTypeRouter {
		t.Errorf("referenced LS type = %#x, want Router", uint16(iap.ReferencedLSType))
	}
	if iap.ReferencedAdvRouter != ospfv3types.RouterID(router) {
		t.Errorf("referenced adv router = %v, want %v", iap.ReferencedAdvRouter, router)
	}
	if len(iap.Prefixes) != 1 {
		t.Fatalf("prefixes = %d, want 1", len(iap.Prefixes))
	}
	gotPfx, ok := v6PrefixToNetip(iap.Prefixes[0], afIPv6Unicast)
	if !ok || gotPfx != netip.MustParsePrefix("2001:db8:2::/64") {
		t.Errorf("prefix = %s, want 2001:db8:2::/64", gotPfx)
	}
	if iap.Prefixes[0].Field16 != 10 {
		t.Errorf("prefix metric = %d, want 10", iap.Prefixes[0].Field16)
	}
}

func TestOSPFv6OriginateFlushesStale(t *testing.T) {
	// After the interface's prefixes are all withdrawn, the previously-originated
	// Intra-Area-Prefix-LSA must be MaxAge-flushed (RFC 2328 sec 14.1), not left to age
	// out; the Router-LSA (still in the kept set) stays live.
	e := newV6OriginEngine()
	router := types.RouterID{172, 30, 0, 2}
	area := types.BackboneArea
	ifaces := []ospflsdb.InterfaceInfo{v6P2PInterface(area, router, types.RouterID{172, 30, 0, 1})}

	if _, ok := e.v6OriginateRouter(area, router, ospfv3types.OptV6|ospfv3types.OptR, ifaces, false, false, false, false); !ok {
		t.Fatalf("v6OriginateRouter returned false")
	}
	p, _ := netipToV6Prefix(netip.MustParsePrefix("2001:db8:2::/64"), 10)
	hIAP, ok := e.v6OriginateIntraAreaPrefix(area, router, []ospfv3packet.Prefix{p})
	if !ok {
		t.Fatalf("v6OriginateIntraAreaPrefix returned false")
	}
	if hIAP.Type != types.LSType(ospfv3types.LSTypeIntraAreaPrefix) {
		t.Errorf("originated header type = %#x, want 0x2009", uint16(hIAP.Type))
	}

	// Keep only the Router-LSA (the prefix set is now empty).
	keep := map[ospflsdb.SelfLSARef]struct{}{
		{Area: area, Key: v6RouterKey(router)}: {},
	}
	if n := e.lsdb.FlushStaleSelfLSAs(router, v6ManagedSelfTypes, keep); n != 1 {
		t.Fatalf("FlushStaleSelfLSAs flushed %d, want 1 (the Intra-Area-Prefix-LSA)", n)
	}

	iap, ok := e.lsdb.LookupLSA(area, v6IntraAreaPrefixKey(router))
	if !ok || !iap.Header.Age.IsMaxAge() {
		t.Errorf("Intra-Area-Prefix-LSA age = %v (ok=%v), want MaxAge (flushed)", iap.Header.Age, ok)
	}
	rtr, ok := e.lsdb.LookupLSA(area, v6RouterKey(router))
	if !ok || rtr.Header.Age.IsMaxAge() {
		t.Errorf("Router-LSA age = %v (ok=%v), want live (kept)", rtr.Header.Age, ok)
	}

	// Idempotent: a second sweep with the same keep set flushes nothing more.
	if n := e.lsdb.FlushStaleSelfLSAs(router, v6ManagedSelfTypes, keep); n != 0 {
		t.Errorf("second FlushStaleSelfLSAs flushed %d, want 0", n)
	}
}

func TestOSPFv6NetipToV6PrefixRoundTrip(t *testing.T) {
	// The OSPFv3 prefix is word-padded (RFC 5340 App A.4.1): the on-wire address is
	// ((bits+31)/32)*4 octets, and v6PrefixToNetip is the exact inverse.
	for _, cidr := range []string{"2001:db8::/32", "2001:db8:1200::/56", "2001:db8:1:2::/64", "2001:db8::1/128"} {
		pfx := netip.MustParsePrefix(cidr)
		p, ok := netipToV6Prefix(pfx, 7)
		if !ok {
			t.Fatalf("%s: netipToV6Prefix failed", cidr)
		}
		if p.Field16 != 7 {
			t.Errorf("%s: field16 = %d, want 7", cidr, p.Field16)
		}
		if len(p.Address) != p.Length.ByteLen() {
			t.Errorf("%s: address length = %d, want word-padded %d", cidr, len(p.Address), p.Length.ByteLen())
		}
		got, ok := v6PrefixToNetip(p, afIPv6Unicast)
		if !ok || got != pfx.Masked() {
			t.Errorf("%s: round-trip = %s, want %s", cidr, got, pfx.Masked())
		}
	}
}
