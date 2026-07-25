// VALIDATES: spec-ospf-ext-5 AC-1/AC-3/AC-12/AC-20/AC-23 (IPv6) -- the OSPFv3 SR
// origination builds RFC 8362 Extended LSAs that carry an Adj-SID (type 5) under a
// Router-Link TLV in the E-Router-LSA and a Prefix-SID (type 4) under an Intra-Area
// Prefix TLV in the E-Intra-Area-Prefix-LSA, using the OSPFv3 registry type codes, and
// the E-LSA types are self-managed so the stale flush withdraws them when SR is off.
// PREVENTS: an E-Router-LSA that never carries the Adj-SID, a node Prefix-SID lost in
// carriage, an SR Extended LSA that lingers after SR is disabled.
package ospf

import (
	"net/netip"
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// RFC requirement: RFC8666-8.4.1-1 negative -- the withdrawal is conditional on the
// adjacency going down: while the neighbor is up, the E-Router-LSA keeps advertising the
// Adj-SID sub-TLV for that adjacency.
func TestOSPFv3ERouterBodyCarriesAdjSID(t *testing.T) {
	eng := newV6RIEngine(t)
	nbr := types.RouterID{2, 2, 2, 2}
	const adjLabel uint32 = 40001
	eng.srAdj = &srAdjManager{labels: map[srAdjKey]srAdjRecord{
		{iface: "eth0", router: nbr}: {label: adjLabel, adj: sr.AdjSID{
			Flags: sr.AdjSIDFlags{V: true, L: true}, Label: adjLabel, IsLabel: true,
		}},
	}}
	ifaces := []ospflsdb.InterfaceInfo{{
		Name: "eth0", NetworkType: ospflsdb.NetworkPointToPoint, InterfaceID: 5, Cost: 10,
		Neighbors: []ospflsdb.NeighborInfo{{RouterID: nbr, State: ospflsdb.NeighborStateFull, InterfaceID: 6}},
	}}

	body, ok := eng.v6BuildERouterBody(ifaces)
	if !ok {
		t.Fatalf("E-Router body not built for an adjacency with an Adj-SID")
	}
	// Skip the 4-byte E-Router fixed header, then parse the Router-Link TLV stream.
	ext, err := ospfv3packet.DecodeExtendedLSABody(body[eRouterHeaderLen:])
	if err != nil || len(ext.TLVs) != 1 || ext.TLVs[0].Type != extTLVRouterLink {
		t.Fatalf("E-Router TLVs = %+v, err %v", ext.TLVs, err)
	}
	// The Router-Link TLV carries the neighbor Router ID at offset 12 and an Adj-SID
	// sub-TLV after the 16-byte fixed fields.
	rl := ext.TLVs[0].Value
	if [4]byte(rl[12:16]) != [4]byte(nbr) {
		t.Fatalf("Router-Link neighbor id = %v want %v", rl[12:16], nbr)
	}
	subs, err := ospfv3packet.SubTLVsAt(rl, 16)
	if err != nil || len(subs) != 1 || subs[0].Type != sr.V6TypeAdjSID {
		t.Fatalf("Adj-SID sub-TLV = %+v, err %v", subs, err)
	}
	a, err := sr.DecodeAdjSIDValueV6(subs[0].Value)
	if err != nil || !a.IsLabel || a.Label != adjLabel {
		t.Fatalf("decoded Adj-SID = %+v, err %v", a, err)
	}
}

func TestOSPFv3ERouterBodyEmptyWithoutAdjSID(t *testing.T) {
	eng := newV6RIEngine(t)
	eng.srAdj = &srAdjManager{labels: map[srAdjKey]srAdjRecord{}}
	ifaces := []ospflsdb.InterfaceInfo{{
		Name: "eth0", NetworkType: ospflsdb.NetworkPointToPoint, InterfaceID: 5,
		Neighbors: []ospflsdb.NeighborInfo{{RouterID: types.RouterID{2, 2, 2, 2}, State: ospflsdb.NeighborStateFull}},
	}}
	if _, ok := eng.v6BuildERouterBody(ifaces); ok {
		t.Fatalf("E-Router body must not be originated with no Adj-SID")
	}
}

func TestOSPFv3EIntraAreaPrefixRoundTrip(t *testing.T) {
	router := types.RouterID{1, 1, 1, 1}
	loop := netip.MustParsePrefix("2001:db8::1/128")
	sids := []sr.PrefixSIDConfig{{Prefix: loop, Index: 42, NodeSID: true, NoPHP: true}}

	body := v6EIntraAreaPrefixBody(router, sids)
	// The 12-byte referenced header points at this router's Router-LSA.
	if [4]byte(body[8:12]) != [4]byte(router) {
		t.Fatalf("referenced advertising router = %v want %v", body[8:12], router)
	}
	ext, err := ospfv3packet.DecodeExtendedLSABody(body[eIntraPrefixHeaderLen:])
	if err != nil || len(ext.TLVs) != 1 || ext.TLVs[0].Type != extTLVIntraAreaPrefix {
		t.Fatalf("Intra-Area-Prefix TLVs = %+v, err %v", ext.TLVs, err)
	}
	gotPfx, ps, ok := v6PrefixSIDFromTLV(ext.TLVs[0])
	if !ok {
		t.Fatalf("Prefix-SID not recovered from the Intra-Area-Prefix TLV")
	}
	if gotPfx != loop {
		t.Fatalf("prefix round-trip = %v want %v", gotPfx, loop)
	}
	if ps.Index != 42 || ps.IsLabel || !ps.Flags.NP || ps.Algorithm != 0 {
		t.Fatalf("Prefix-SID round-trip = %+v", ps)
	}
}

func TestOSPFv3ManagedSelfTypesIncludeSR(t *testing.T) {
	for _, want := range []ospfv3types.LSType{
		ospfv3types.LSTypeERouter, ospfv3types.LSTypeEIntraAreaPrefix, ospfv3types.LSTypeEInterAreaPrefix,
	} {
		if _, ok := v6ManagedSelfTypes[types.LSType(want)]; !ok {
			t.Fatalf("SR Extended LSA type %#04x not in v6ManagedSelfTypes (AC-20 flush)", uint16(want))
		}
	}
}

// installRemoteV6EPrefix builds a remote E-prefix LSA carrying a single node Prefix-SID
// and installs it into the engine's LSDB for the reception tests.
func installRemoteV6EPrefix(t *testing.T, eng *engine, area types.AreaID, originator types.RouterID, lsid uint32, sids []sr.PrefixSIDConfig) {
	t.Helper()
	body := v6EIntraAreaPrefixBody(originator, sids)
	enc := v6SelfExtEncoder(ospfv3types.LSTypeEIntraAreaPrefix, ospfv3types.LinkStateID(v6SummaryLSID(lsid)), originator, body)
	lsa := enc(types.LSSequenceNumber(0x80000001), false)
	if !eng.lsdb.Install(area, lsa) {
		t.Fatalf("install remote E-Intra-Area-Prefix LSA failed")
	}
}

func TestOSPFv3SRSnapshot(t *testing.T) {
	// AC-17: show ospf ipv6 segment-routing renders the configured SRGB/SRLB and node
	// Prefix-SIDs for the IPv6 family, keyed to this engine's Router ID.
	eng := newV6RIEngine(t)
	eng.setConfig(v6RIConfig(t, "area"))
	router := eng.cfg.RouterID
	srWire.set(router, sr.SRConfig{
		Enabled:  true,
		SRGB:     []sr.LabelRange{{Base: 16000, Size: 8000}},
		SRLB:     []sr.LabelRange{{Base: 15000, Size: 1000}},
		Prefixes: []sr.PrefixSIDConfig{{Prefix: netip.MustParsePrefix("2001:db8::1/128"), Index: 1}},
	})
	defer srWire.set(router, sr.SRConfig{})

	view := eng.srSnapshot(interfaceFamilyIPv6)
	if !view.Enabled || view.Family != interfaceFamilyIPv6 {
		t.Fatalf("v6 SR snapshot = %+v", view)
	}
	if len(view.SRGB) != 1 || view.SRGB[0].LowerBound != 16000 {
		t.Fatalf("SRGB row wrong: %+v", view.SRGB)
	}
	if len(view.PrefixSIDs) != 1 || view.PrefixSIDs[0].Prefix != "2001:db8::1/128" {
		t.Fatalf("Prefix-SID row wrong: %+v", view.PrefixSIDs)
	}
	if len(view.Algorithms) != 1 || view.Algorithms[0] != 0 {
		t.Fatalf("SR-Algorithm 0 must be advertised: %+v", view.Algorithms)
	}
}

func TestOSPFv3SRDisabledOriginatesNothing(t *testing.T) {
	// AC-20: with SR disabled, the SR Extended-LSA origination pass originates nothing.
	eng := newV6RIEngine(t)
	router := types.RouterID{7, 7, 7, 7}
	srWire.set(router, sr.SRConfig{}) // disabled
	byArea := map[types.AreaID][]ospflsdb.InterfaceInfo{types.BackboneArea: {{Name: "eth0"}}}
	keep := map[ospflsdb.SelfLSARef]struct{}{}
	if n := eng.v6OriginateSR(router, byArea, false, []types.AreaID{types.BackboneArea}, keep); n != 0 {
		t.Fatalf("SR disabled must originate no Extended LSAs, got %d", n)
	}
	if len(keep) != 0 {
		t.Fatalf("SR disabled must keep no self-LSA refs, got %d", len(keep))
	}
}

func TestOSPFv3ReceivedPrefixSIDsFromLSDB(t *testing.T) {
	eng := newV6RIEngine(t)
	originator := types.RouterID{9, 9, 9, 9}
	loop := netip.MustParsePrefix("2001:db8::9/128")
	installRemoteV6EPrefix(t, eng, types.BackboneArea, originator, 7,
		[]sr.PrefixSIDConfig{{Prefix: loop, Index: 9}})

	got := eng.v6ReceivedPrefixSIDs()
	if len(got) != 1 {
		t.Fatalf("v6ReceivedPrefixSIDs = %+v, want 1 entry", got)
	}
	if got[0].Prefix != loop || got[0].SID.Index != 9 || got[0].Originator != originator {
		t.Fatalf("received Prefix-SID = %+v", got[0])
	}
	if got[0].LSType != ospfv3types.LSTypeEIntraAreaPrefix {
		t.Fatalf("received LS type = %#04x", uint16(got[0].LSType))
	}
}
