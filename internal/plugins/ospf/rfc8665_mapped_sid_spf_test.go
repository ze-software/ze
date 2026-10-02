// Design: docs/architecture/wire/ospf.md -- OSPF Segment Routing forwarding.
// Related: sr_install.go -- srInstallFromRoutes and srRoutes, the production join.
//
// VALIDATES: RFC 8665 section 5 PHP for a Mapping Server Prefix-SID on the post-SPF path:
// srInstallFromRoutes reads the SPF route table, and srRoutes copies each route's type and
// the routers whose Extended Prefix TLV carries the A-Flag for that prefix and route class
// into the route the installer judges. An inter-area and an external route through an ABR
// and ASBR that generated those TLVs pop the label; when the A-Flags name the wrong class,
// or another router, the label is kept.
// PREVENTS: the A-Flag advertisers or the route type dropped between the SPF and the
// installer, which keeps every mapped inter-area and external label, and the inter-area
// and external A-Flag sets read as one.
package ospf

import (
	"net/netip"
	"testing"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

var (
	rfc8665SPFBorder = types.RouterID{10, 0, 0, 2}
	rfc8665SPFOther  = types.RouterID{10, 0, 0, 3}
	rfc8665SPFInter  = netip.MustParsePrefix("10.1.0.0/24")
	rfc8665SPFExtern = netip.MustParsePrefix("10.2.0.0/24")
)

// rfc8665SPFOutcome is what reached the mpls-fib for one FEC: the pushed label (push
// true), and whether the transit entry for this node's label is a pop or a swap.
type rfc8665SPFOutcome struct {
	label uint32
	push  bool
	pop   bool
	swap  bool
}

// rfc8665SPFAttach is one Extended Prefix TLV with the A-Flag set: the router generating
// it, the prefix, and its Route Type.
type rfc8665SPFAttach struct {
	router    types.RouterID
	prefix    [4]byte
	routeType uint8
}

// rfc8665SPFOriginate installs one area-scoped opaque LSA of opaqueType from router.
func rfc8665SPFOriginate(t *testing.T, eng *engine, router types.RouterID, opaqueType uint8, opaqueID uint32, body []byte) {
	t.Helper()
	if _, ok := eng.lsdb.OriginateOpaque(ospflsdb.OpaqueOriginateInput{
		Router: router, OpaqueType: opaqueType, OpaqueID: opaqueID,
		Scope: types.LSTypeOpaqueArea, Area: types.BackboneArea, Options: types.OptionO, Body: body,
	}); !ok {
		t.Fatalf("installing opaque type %d from %s failed", opaqueType, router)
	}
}

// rfc8665SPFInstall builds router 10.0.0.1 with a real SPF computer and LSDB: a
// point-to-point link to 10.0.0.2, an ABR and ASBR (B and E bits) that originates a
// Summary-LSA for 10.1.0.0/24 and an AS-external LSA (Type 2) for 10.2.0.0/24. 10.0.0.2
// and the Mapping Server 10.0.0.50 advertise SRGB 16000/100; the Mapping Server binds index
// 9 to 10.1.0.0/24 and index 10 to 10.2.0.0/24 with the M-Flag set. Each entry of attach
// becomes an Extended Prefix TLV with the A-Flag. SPF runs, srInstallFromRoutes runs, and
// the mpls-fib entries toward 10.0.0.2 are read per FEC.
func rfc8665SPFInstall(t *testing.T, attach []rfc8665SPFAttach) map[netip.Prefix]rfc8665SPFOutcome {
	t.Helper()
	srTestReset(t)
	eng := rfc5250ReceiverEngine(t)
	self := types.RouterID{10, 0, 0, 1}
	srWire.set(self, sr.SRConfig{Enabled: true, SRGB: []sr.LabelRange{{Base: 18000, Size: 100}}})
	bus := &srCaptureBus{}
	eng.srInstaller = newTestInstaller(bus)

	toSelf := packet.RouterLink{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID(self),
		LinkData: [4]byte{10, 0, 0, 2}, Metric: 10}
	border := bgplsReachabilityRouter(rfc8665SPFBorder, toSelf)
	border.Router.Flags = packet.RouterFlagB | packet.RouterFlagE
	summary := packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeSummaryNetwork,
		LinkStateID: types.LinkStateID{10, 1, 0, 0}, AdvertisingRouter: rfc8665SPFBorder, Sequence: types.InitialSequenceNumber},
		Summary: &packet.SummaryLSA{NetworkMask: [4]byte{255, 255, 255, 0}, Metric: 20}}
	external := packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeASExternal,
		LinkStateID: types.LinkStateID{10, 2, 0, 0}, AdvertisingRouter: rfc8665SPFBorder, Sequence: types.InitialSequenceNumber},
		External: &packet.ExternalLSA{NetworkMask: [4]byte{255, 255, 255, 0}, ExternalType2: true, Metric: 20}}
	for _, lsa := range []packet.LSA{border, summary, external} {
		if !eng.lsdb.Install(types.BackboneArea, lsa) {
			t.Fatalf("installing %v from %s failed", lsa.Header.Type, lsa.Header.AdvertisingRouter)
		}
	}

	ri := packet.EncodeRITLVs([]packet.RITLV{
		{Type: sr.V4TypeSRAlgorithm, Value: []byte{0}},
		{Type: sr.V4TypeSRGB, Value: sr.EncodeRangeValue(sr.LabelRange{Base: 16000, Size: 100})},
	})
	rfc8665SPFOriginate(t, eng, rfc8665SPFBorder, packet.RIOpaqueType, 0, ri)
	rfc8665SPFOriginate(t, eng, rfc8665MappingServer, packet.RIOpaqueType, 0, ri)

	mapped := func(prefix [4]byte, index uint32) packet.ExtPrefixTLV {
		sid := sr.PrefixSID{Flags: sr.SIDFlags{M: true}, Index: index}
		return packet.ExtPrefixTLV{RouteType: packet.ExtRouteTypeInterArea, PrefixLength: 24,
			AF: packet.ExtPrefixAFIPv4Unicast, AddressPrefix: prefix,
			SubTLVs: []packet.ExtSubTLV{{Type: sr.V4TypePrefixSID, Value: sr.EncodePrefixSIDValue(sid)}}}
	}
	rfc8665SPFOriginate(t, eng, rfc8665MappingServer, packet.ExtPrefixOpaqueType, 1, packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{
		Prefixes: []packet.ExtPrefixTLV{mapped([4]byte{10, 1, 0, 0}, 9), mapped([4]byte{10, 2, 0, 0}, 10)},
	}))
	for i, a := range attach {
		tlv := packet.ExtPrefixTLV{RouteType: a.routeType, PrefixLength: 24, AF: packet.ExtPrefixAFIPv4Unicast,
			Flags: packet.ExtPrefixFlagA, AddressPrefix: a.prefix}
		rfc8665SPFOriginate(t, eng, a.router, packet.ExtPrefixOpaqueType, uint32(10+i),
			packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{Prefixes: []packet.ExtPrefixTLV{tlv}}))
	}

	eng.spf.Run()
	eng.srInstallFromRoutes()
	return rfc8665SPFRead(t, bus.entries)
}

// rfc8665SPFRead sorts the mpls-fib entries toward 10.0.0.2 by FEC: a push names its FEC,
// and a transit entry is keyed by this node's label, 18009 for 10.1.0.0/24 and 18010 for
// 10.2.0.0/24.
func rfc8665SPFRead(t *testing.T, entries []mplsfibevents.Entry) map[netip.Prefix]rfc8665SPFOutcome {
	t.Helper()
	nh := netip.MustParseAddr("10.0.0.2")
	byLabel := map[uint32]netip.Prefix{18009: rfc8665SPFInter, 18010: rfc8665SPFExtern}
	out := make(map[netip.Prefix]rfc8665SPFOutcome)
	for _, e := range entries {
		if e.Action != mplsfibevents.ActionAdd || e.NextHop != nh {
			continue
		}
		switch e.Op {
		case mplsfibevents.OpPush:
			if len(e.OutLabels) != 1 {
				t.Fatalf("push for %s carries %d labels, want 1: %+v", e.FEC, len(e.OutLabels), e)
			}
			o := out[e.FEC]
			o.label, o.push = e.OutLabels[0], true
			out[e.FEC] = o
		case mplsfibevents.OpPop:
			o := out[byLabel[e.InLabel]]
			o.pop = true
			out[byLabel[e.InLabel]] = o
		case mplsfibevents.OpSwap:
			o := out[byLabel[e.InLabel]]
			o.swap = true
			out[byLabel[e.InLabel]] = o
		default:
			t.Fatalf("unexpected mpls-fib op toward %s: %+v", nh, e)
		}
	}
	if _, ok := out[netip.Prefix{}]; ok {
		t.Fatalf("transit entry for a label other than 18009 or 18010: %+v", entries)
	}
	return out
}

// RFC requirement: RFC8665-5-14 positive -- on the post-SPF install path, a Mapping Server
// Prefix-SID (M-Flag) for an inter-area route through the ABR 10.0.0.2 whose Extended Prefix
// TLV for the prefix has the A-Flag and Route Type 3, and for an external (Type 2) route
// through the ASBR 10.0.0.2 whose TLV has the A-Flag and Route Type 5, is popped: no label
// pushed for either FEC, and the transit entries for 18009 and 18010 are pops.
func TestRFC8665MappedSIDPHPThroughSPFRoutes(t *testing.T) {
	// Goal: the route type and A-Flag advertisers reach the PHP decision from the SPF route
	// table. Method: a real LSDB and SPF run, srInstallFromRoutes, the mpls-fib entries read.
	got := rfc8665SPFInstall(t, []rfc8665SPFAttach{
		{router: rfc8665SPFBorder, prefix: [4]byte{10, 1, 0, 0}, routeType: packet.ExtRouteTypeInterArea},
		{router: rfc8665SPFBorder, prefix: [4]byte{10, 2, 0, 0}, routeType: packet.ExtRouteTypeASExternal},
	})
	for _, fec := range []netip.Prefix{rfc8665SPFInter, rfc8665SPFExtern} {
		o, ok := got[fec]
		if !ok {
			t.Fatalf("%s: nothing installed toward 10.0.0.2: %+v", fec, got)
		}
		if o.push {
			t.Fatalf("%s: pushed label %d, want PHP (no push)", fec, o.label)
		}
		if !o.pop || o.swap {
			t.Fatalf("%s: transit entry pop=%v swap=%v, want a pop", fec, o.pop, o.swap)
		}
	}
}

// RFC requirement: RFC8665-5-14 negative -- on the same path, the label is kept (push 16009
// for 10.1.0.0/24 and 16010 for 10.2.0.0/24, transit swaps) when the next hop's A-Flag TLVs
// name the other route class (Route Type 5 for the inter-area prefix, 3 for the external
// one), and the correct A-Flag TLVs come from 10.0.0.3, which is not the next hop.
func TestRFC8665MappedSIDKeepsLabelThroughSPFRoutesOutsideListedCases(t *testing.T) {
	// Goal: the class of each A-Flag TLV, and its advertiser, are kept through the join.
	// Method: the positive's network with the A-Flag TLVs moved, the mpls-fib entries read.
	got := rfc8665SPFInstall(t, []rfc8665SPFAttach{
		{router: rfc8665SPFBorder, prefix: [4]byte{10, 1, 0, 0}, routeType: packet.ExtRouteTypeASExternal},
		{router: rfc8665SPFBorder, prefix: [4]byte{10, 2, 0, 0}, routeType: packet.ExtRouteTypeInterArea},
		{router: rfc8665SPFOther, prefix: [4]byte{10, 1, 0, 0}, routeType: packet.ExtRouteTypeInterArea},
		{router: rfc8665SPFOther, prefix: [4]byte{10, 2, 0, 0}, routeType: packet.ExtRouteTypeASExternal},
	})
	want := map[netip.Prefix]uint32{rfc8665SPFInter: 16009, rfc8665SPFExtern: 16010}
	for fec, label := range want {
		o := got[fec]
		if !o.push {
			t.Fatalf("%s: no label pushed toward 10.0.0.2, want %d kept: %+v", fec, label, got)
		}
		if o.label != label {
			t.Fatalf("%s: pushed label %d, want %d", fec, o.label, label)
		}
		if o.pop || !o.swap {
			t.Fatalf("%s: transit entry pop=%v swap=%v, want a swap", fec, o.pop, o.swap)
		}
	}
}
