package ospf

import (
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// VALIDATES: RFC 7684 Section 2.1, an ABR propagates another router's N-Flag between areas.
// PREVENTS: the N-Flag dropped on, or invented for, an inter-area Extended Prefix TLV.

// nflagInterAreaTLV delivers one received Extended Prefix LSA from 2.2.2.2 in source (route
// type, prefix length, flags as given), originates this ABR's Type-3 summary for the prefix
// into area 0.0.0.1, and returns the inter-area Extended Prefix TLV this router originates
// for that summary. It fails the test when no such TLV is originated.
func nflagInterAreaTLV(t *testing.T, source types.AreaID, bits, flags uint8, prefix [4]byte) packet.ExtPrefixTLV {
	t.Helper()
	router := types.RouterID{1, 1, 1, 1}
	topo := []ospflsdb.InterfaceInfo{extStubIface("eth0", [4]byte{10, 0, 0, 1}, [4]byte{255, 255, 255, 0})}
	eng, _ := extEngineWithTopology(t, topo)
	eng.extPrefixOnReceive(opaqueReceived{
		OpaqueType: packet.ExtPrefixOpaqueType, OpaqueID: 1, Scope: OpaqueScopeArea, Area: source,
		AdvertisingRouter: types.RouterID{2, 2, 2, 2},
		Body:              extPrefixBody(packet.ExtRouteTypeIntraArea, bits, flags, prefix), Reachable: true,
	})
	mask := [4]byte{255, 255, 255, 255}
	if bits == 24 {
		mask = [4]byte{255, 255, 255, 0}
	}
	area1 := types.AreaID{0, 0, 0, 1}
	eng.lsdb.OriginateSummary(area1, router, 0, types.LSTypeSummaryNetwork, types.LinkStateID(prefix), mask, 20)

	for _, o := range eng.extPrefixOnOriginate(router) {
		if o.Withdraw {
			continue
		}
		p := decodeOnePrefix(t, o)
		if p.RouteType != packet.ExtRouteTypeInterArea {
			continue
		}
		if p.AddressPrefix != prefix {
			continue
		}
		return p
	}
	t.Fatalf("no inter-area Extended Prefix TLV originated for %v/%d", prefix, bits)
	return packet.ExtPrefixTLV{}
}

// TestRFC7684NFlagOfAnotherRouterPreservedInterArea proves the ABR carries another router's
// N-Flag onto the inter-area advertisement. RFC 7684 Section 2.1: "The flag is preserved when
// the OSPFv2 Extended Prefix Opaque LSA is propagated between areas."
//
// Goal: an ABR that summarizes into area 1 a host prefix another router advertised in area 0
// with the N-Flag keeps that N-Flag on its inter-area Extended Prefix TLV.
// Method: router 2.2.2.2's intra-area Extended Prefix LSA for 10.2.2.2/32 with N reaches this
// router through the opaque reception hook with its source area 0; this router then
// originates the Type-3 Summary for 10.2.2.2/32 into area 1, and the inter-area Extended
// Prefix TLV it originates for that summary is decoded. The prefix is not connected to this
// router, so the flag can only come from the received advertisement.
func TestRFC7684NFlagOfAnotherRouterPreservedInterArea(t *testing.T) {
	// RFC requirement: RFC7684-2.1-2 positive -- another router's host prefix advertised with
	// the N-Flag in area 0 keeps the N-Flag on this ABR's inter-area Extended Prefix TLV into
	// area 1.
	p := nflagInterAreaTLV(t, types.BackboneArea, 32, packet.ExtPrefixFlagN, [4]byte{10, 2, 2, 2})
	if !p.HasFlag(packet.ExtPrefixFlagN) {
		t.Fatalf("N-Flag advertised by 2.2.2.2 in area 0 not preserved on the inter-area Extended Prefix TLV: %+v", p)
	}
}

// TestRFC7684NFlagNotInventedInterArea proves the ABR sets the N-Flag on the inter-area TLV only
// when a flag it propagates exists. RFC 7684 Section 2.1: "The flag is preserved when the
// OSPFv2 Extended Prefix Opaque LSA is propagated between areas."
//
// Goal: preserving is not inventing. The inter-area TLV into area 1 carries no N-Flag when
// the received advertisement lacks it, when the only advertisement carrying it is in area 1
// itself (nothing is propagated between areas), or when it was set on a non-host prefix
// (where RFC 7684 Section 2.1 says it "MUST be ignored").
// Method: as the positive test, one received advertisement per case, each on a fresh engine.
func TestRFC7684NFlagNotInventedInterArea(t *testing.T) {
	// RFC requirement: RFC7684-2.1-2 negative -- no N-Flag on the inter-area TLV into area 1
	// when the area-0 advertisement lacks it, when the N-Flag advertisement is in area 1
	// itself, or when N was set on a non-host /24.
	area1 := types.AreaID{0, 0, 0, 1}
	cases := []struct {
		name   string
		source types.AreaID
		bits   uint8
		flags  uint8
		prefix [4]byte
	}{
		{"area 0 advertisement without N", types.BackboneArea, 32, 0, [4]byte{10, 2, 2, 2}},
		{"N advertised only in the target area", area1, 32, packet.ExtPrefixFlagN, [4]byte{10, 2, 2, 2}},
		{"N on a non-host prefix in area 0", types.BackboneArea, 24, packet.ExtPrefixFlagN, [4]byte{10, 2, 2, 0}},
	}
	for _, c := range cases {
		p := nflagInterAreaTLV(t, c.source, c.bits, c.flags, c.prefix)
		if p.HasFlag(packet.ExtPrefixFlagN) {
			t.Fatalf("%s: inter-area Extended Prefix TLV into area 1 carries an N-Flag nothing propagated: %+v", c.name, p)
		}
	}
}
