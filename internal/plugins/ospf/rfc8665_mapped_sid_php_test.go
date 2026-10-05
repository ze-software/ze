// Design: docs/architecture/wire/ospf.md -- OSPF Segment Routing forwarding.
// Related: sr_install.go -- srMappedSIDAction and srAttachedAdvertisers, the producers.
//
// VALIDATES: RFC 8665 section 5 PHP for a Mapping Server Prefix-SID (M-Flag set): its NP and
// E are ignored, and the label is popped only in the three cases the section lists
// (intra-area toward the prefix originator; inter-area toward an ABR, and external toward an
// ASBR, generating the Extended Prefix TLV with the A-Flag for the prefix). Everywhere else
// the label is kept.
// PREVENTS: a mapped SID that always keeps its label, and one popped toward a router the
// section does not name.
package ospf

import (
	"net/netip"
	"slices"
	"testing"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

var (
	rfc8665MappingServer = types.RouterID{10, 0, 0, 50}
	rfc8665Downstream    = types.RouterID{10, 0, 0, 1}
	rfc8665Other         = types.RouterID{10, 0, 0, 2}
)

// rfc8665MappedInstall installs fec through one next-hop router nh for a Prefix-SID index 9
// the Mapping Server advertised with flags, and reports what reached the mpls-fib toward nh:
// the pushed label (push true), and whether the transit entry for this node's label 18009
// is a pop (PHP) rather than a swap.
func rfc8665MappedInstall(t *testing.T, route srRoute, nh types.RouterID, flags sr.SIDFlags) (label uint32, push, pop bool) {
	t.Helper()
	bus := &srCaptureBus{}
	inst := newTestInstaller(bus)
	nhAddr := netip.MustParseAddr("10.0.9.9")
	route.NextHops = []srNextHop{{Addr: nhAddr, Router: nh}}
	srgb := sr.NewSRGB([]sr.LabelRange{{Base: 16000, Size: 100}})
	sids := map[netip.Prefix]srRemotePrefixSID{route.Prefix: {Originator: rfc8665MappingServer, SID: sr.PrefixSID{Flags: flags, Index: 9}}}
	caps := map[types.RouterID]sr.SRGB{nh: srgb, rfc8665MappingServer: srgb}
	algos := map[types.RouterID][]uint8{rfc8665MappingServer: {0}}
	inst.installRoutes([]srRoute{route}, sids, caps, algos, sr.NewSRGB([]sr.LabelRange{{Base: 18000, Size: 100}}))
	transit := false
	for _, e := range bus.entries {
		if e.Action != mplsfibevents.ActionAdd || e.NextHop != nhAddr {
			continue
		}
		switch e.Op {
		case mplsfibevents.OpPush:
			if len(e.OutLabels) != 1 {
				t.Fatalf("push toward %s carries %d labels, want 1: %+v", nh, len(e.OutLabels), e)
			}
			label, push = e.OutLabels[0], true
		case mplsfibevents.OpPop:
			transit, pop = true, e.InLabel == 18000+9
		case mplsfibevents.OpSwap:
			transit = true
		case mplsfibevents.OpUnspecified:
			t.Fatalf("unexpected mpls-fib op toward %s: %+v", nh, e)
		default:
			panic("BUG: rfc8665MappedInstall: invalid locally emitted MPLS operation")
		}
	}
	if !transit {
		t.Fatalf("no transit entry toward %s for %+v: %+v", nh, route, bus.entries)
	}
	return label, push, pop
}

// RFC requirement: RFC8665-5-14 positive -- for a Prefix-SID with the M-Flag set, the label is
// popped (no push, transit pop of 18009) in each listed case: an intra-area prefix toward its
// originator, which is not the Mapping Server; an inter-area prefix toward an ABR whose
// A-Flag Extended Prefix TLV names it; an external prefix (Type 1 and Type 2) toward such an
// ASBR. NP=1 with E=1 on the SID changes nothing, since both are ignored.
func TestRFC8665MappedSIDPHPInListedCases(t *testing.T) {
	// Goal: PHP follows the route, not the Mapping Server's flags. Method: one route per
	// case through the installer, the mpls-fib entries toward the downstream router read.
	fec := netip.MustParsePrefix("10.1.0.0/24")
	cases := []struct {
		name  string
		route srRoute
	}{
		{"intra-area-originator", srRoute{Prefix: fec, Origin: rfc8665Downstream, Type: ospfspf.RouteIntraArea}},
		{"inter-area-abr-a-flag", srRoute{Prefix: fec, Origin: rfc8665Downstream, Type: ospfspf.RouteInterArea, Attached: []types.RouterID{rfc8665Other, rfc8665Downstream}}},
		{"external-1-asbr-a-flag", srRoute{Prefix: fec, Origin: rfc8665Downstream, Type: ospfspf.RouteExternalType1, Attached: []types.RouterID{rfc8665Downstream}}},
		{"external-2-asbr-a-flag", srRoute{Prefix: fec, Origin: rfc8665Downstream, Type: ospfspf.RouteExternalType2, Attached: []types.RouterID{rfc8665Downstream}}},
	}
	for _, c := range cases {
		for _, flags := range []sr.SIDFlags{{M: true}, {M: true, NP: true, E: true}} {
			label, push, pop := rfc8665MappedInstall(t, c.route, rfc8665Downstream, flags)
			if push {
				t.Fatalf("%s %+v: pushed label %d, want PHP (no push)", c.name, flags, label)
			}
			if !pop {
				t.Fatalf("%s %+v: transit entry for 18009 is not a pop", c.name, flags)
			}
		}
	}
}

// RFC requirement: RFC8665-5-14 negative -- outside the listed cases a Prefix-SID with the
// M-Flag set keeps its label (push 16009, transit swap): an intra-area prefix toward a
// router that is not its originator (the Mapping Server itself included); an inter-area or
// external prefix toward a router no A-Flag Extended Prefix TLV names, even when it is the
// route's advertising router; a route whose type is unspecified. NP=0 alone would mean PHP,
// and it is ignored.
func TestRFC8665MappedSIDKeepsLabelOutsideListedCases(t *testing.T) {
	// Goal: no PHP the section does not list. Method: the positive's routes with one fact
	// of each listed case broken, the pushed label read toward the downstream router.
	fec := netip.MustParsePrefix("10.1.0.0/24")
	cases := []struct {
		name  string
		route srRoute
		nh    types.RouterID
	}{
		{"intra-area-transit", srRoute{Prefix: fec, Origin: rfc8665Other, Type: ospfspf.RouteIntraArea}, rfc8665Downstream},
		{"intra-area-mapping-server", srRoute{Prefix: fec, Origin: rfc8665Other, Type: ospfspf.RouteIntraArea}, rfc8665MappingServer},
		{"inter-area-no-a-flag", srRoute{Prefix: fec, Origin: rfc8665Downstream, Type: ospfspf.RouteInterArea}, rfc8665Downstream},
		{"inter-area-a-flag-elsewhere", srRoute{Prefix: fec, Origin: rfc8665Downstream, Type: ospfspf.RouteInterArea, Attached: []types.RouterID{rfc8665Other}}, rfc8665Downstream},
		{"external-no-a-flag", srRoute{Prefix: fec, Origin: rfc8665Downstream, Type: ospfspf.RouteExternalType2}, rfc8665Downstream},
		{"unspecified-type", srRoute{Prefix: fec, Origin: rfc8665Downstream, Attached: []types.RouterID{rfc8665Downstream}}, rfc8665Downstream},
	}
	for _, c := range cases {
		label, push, pop := rfc8665MappedInstall(t, c.route, c.nh, sr.SIDFlags{M: true})
		if !push {
			t.Fatalf("%s: no label pushed toward %s, want 16009 kept", c.name, c.nh)
		}
		if label != 16009 {
			t.Fatalf("%s: pushed label %d, want 16009", c.name, label)
		}
		if pop {
			t.Fatalf("%s: transit entry for 18009 is a pop, want a swap", c.name)
		}
	}
}

// rfc8665InstallAttachLSA installs one Extended Prefix Opaque LSA from adv advertising
// 10.1.0.0/24 with routeType and flags, and no sub-TLV.
func rfc8665InstallAttachLSA(t *testing.T, eng *engine, adv types.RouterID, routeType, flags uint8) {
	t.Helper()
	body := packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{Prefixes: []packet.ExtPrefixTLV{{
		RouteType:     routeType,
		PrefixLength:  24,
		AF:            packet.ExtPrefixAFIPv4Unicast,
		Flags:         flags,
		AddressPrefix: [4]byte{10, 1, 0, 0},
	}}})
	if _, ok := eng.lsdb.OriginateOpaque(ospflsdb.OpaqueOriginateInput{
		Router:     adv,
		OpaqueType: packet.ExtPrefixOpaqueType,
		OpaqueID:   1,
		Scope:      types.LSTypeOpaqueArea,
		Area:       types.BackboneArea,
		Options:    types.OptionO,
		Body:       body,
	}); !ok {
		t.Fatalf("installing the Extended Prefix Opaque LSA of %s failed", adv)
	}
}

// RFC requirement: RFC8665-5-14 positive -- the ABR and ASBR of the inter-area and external
// cases are read from received Extended Prefix LSAs: a router whose TLV for the prefix has
// the A-Flag and Route Type 3 is an inter-area attacher, Route Type 5 or 7 an external one;
// a TLV without the A-Flag, or with the A-Flag on an intra-area Route Type, names no one.
func TestRFC8665AttachedAdvertisersFromAFlag(t *testing.T) {
	// Goal: the A-Flag set the PHP decision reads is the received one. Method: five routers'
	// Extended Prefix LSAs in a real LSDB, srAttachedAdvertisers read per prefix and class.
	srTestReset(t)
	eng, _ := newRedistEngine(t, extOrigCfg)
	abr := types.RouterID{10, 0, 0, 11}
	asbr := types.RouterID{10, 0, 0, 12}
	nssa := types.RouterID{10, 0, 0, 13}
	plain := types.RouterID{10, 0, 0, 14}
	intra := types.RouterID{10, 0, 0, 15}
	rfc8665InstallAttachLSA(t, eng, abr, packet.ExtRouteTypeInterArea, packet.ExtPrefixFlagA)
	rfc8665InstallAttachLSA(t, eng, asbr, packet.ExtRouteTypeASExternal, packet.ExtPrefixFlagA)
	rfc8665InstallAttachLSA(t, eng, nssa, packet.ExtRouteTypeNSSAExternal, packet.ExtPrefixFlagA)
	rfc8665InstallAttachLSA(t, eng, plain, packet.ExtRouteTypeInterArea, 0)
	rfc8665InstallAttachLSA(t, eng, intra, packet.ExtRouteTypeIntraArea, packet.ExtPrefixFlagA)
	fec := netip.MustParsePrefix("10.1.0.0/24")
	attached := eng.srAttachedAdvertisers()
	inter := attached[srAttachKey{prefix: fec}]
	if !slices.Equal(inter, []types.RouterID{abr}) {
		t.Fatalf("inter-area A-Flag advertisers of %s = %v, want [%s]", fec, inter, abr)
	}
	external := slices.Clone(attached[srAttachKey{prefix: fec, external: true}])
	slices.SortFunc(external, func(a, b types.RouterID) int { return slices.Compare(a[:], b[:]) })
	if !slices.Equal(external, []types.RouterID{asbr, nssa}) {
		t.Fatalf("external A-Flag advertisers of %s = %v, want [%s %s]", fec, external, asbr, nssa)
	}
	if len(attached) != 2 {
		t.Fatalf("A-Flag index holds %d keys, want 2 (no intra-area or flagless entry): %v", len(attached), attached)
	}
}
