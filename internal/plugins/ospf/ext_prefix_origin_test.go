// VALIDATES: spec-ospf-ext-4 -- Extended Prefix origination associates each advertised prefix
// with the correct RFC 7684 sec 2.1 Route Type (intra-area 1 from Router-LSA stub links,
// inter-area 3 from self summaries with the A-Flag/N-Flag, AS-external 5 at AS scope), selects
// the right flooding scope (AC-2/AC-4/R-4), preserves the N-Flag on inter-area propagation
// (AC-6), and withdraws a prefix that disappears (AC-13/R-6).
// PREVENTS: a wrong Route Type/scope FRR cannot correlate, a lost A/N flag, or a stale LSA.
package ospf

import (
	"testing"

	ospflsdb "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/lsdb"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

const extOrigCfg = `{"ospf":{"router-id":"1.1.1.1","opaque":true,"extended-prefix":true,"extended-link":true,` +
	`"areas":{"area":{"0":{"area-id":"0"}}},` +
	`"interfaces":{"interface":{"eth0":{"name":"eth0","area":"0"}}}}}`

// decodeOnePrefix decodes an origination body known to carry exactly one Extended Prefix TLV.
func decodeOnePrefix(t *testing.T, o opaqueOrigination) packet.ExtPrefixTLV {
	t.Helper()
	lsa, err := packet.DecodeExtPrefixLSA(o.Body)
	if err != nil {
		t.Fatalf("decode originated prefix body (id %d): %v", o.OpaqueID, err)
	}
	if len(lsa.Prefixes) != 1 {
		t.Fatalf("origination id %d has %d prefix TLVs, want 1", o.OpaqueID, len(lsa.Prefixes))
	}
	return lsa.Prefixes[0]
}

func TestExtPrefixRouteTypeMapping(t *testing.T) {
	router := types.RouterID{1, 1, 1, 1}
	// area 0: a connected /32 loopback (host) and a /24 network.
	topo := []ospflsdb.InterfaceInfo{
		extStubIface("lo0", [4]byte{10, 9, 9, 9}, [4]byte{255, 255, 255, 255}),
		extStubIface("eth0", [4]byte{10, 0, 0, 1}, [4]byte{255, 255, 255, 0}),
	}
	eng, _ := extEngineWithTopology(t, topo)
	// An ABR summary of the /32 into area 1, plus a self Type-5 external.
	area1 := types.AreaID{0, 0, 0, 1}
	eng.lsdb.OriginateSummary(area1, router, 0, types.LSTypeSummaryNetwork, types.LinkStateID{10, 9, 9, 9}, [4]byte{255, 255, 255, 255}, 20)
	_, _, err := eng.lsdb.OriginateExternal(router, [4]byte{198, 51, 100, 0}, [4]byte{255, 255, 255, 0}, 0, true, 30, [4]byte{}, 0)
	if err != nil {
		t.Fatalf("OriginateExternal: %v", err)
	}

	out := eng.extPrefixOnOriginate(router)

	var intra, inter, external *opaqueOrigination
	for i := range out {
		if out[i].Withdraw {
			continue
		}
		p := decodeOnePrefix(t, out[i])
		switch p.RouteType {
		case packet.ExtRouteTypeIntraArea:
			if p.PrefixLength == 32 {
				intra = &out[i]
			}
		case packet.ExtRouteTypeInterArea:
			inter = &out[i]
		case packet.ExtRouteTypeASExternal:
			external = &out[i]
		}
	}
	if intra == nil {
		t.Fatalf("no intra-area (Route Type 1) host prefix originated")
	}
	ip := decodeOnePrefix(t, *intra)
	if ip.AF != packet.ExtPrefixAFIPv4Unicast || ip.AddressPrefix != [4]byte{10, 9, 9, 9} {
		t.Fatalf("intra prefix fields = %+v", ip)
	}
	if !ip.HasFlag(packet.ExtPrefixFlagN) {
		t.Fatalf("intra host /32 must set the N-Flag")
	}
	if intra.Scope != OpaqueScopeArea || intra.Scope.lsType() != types.LSTypeOpaqueArea {
		t.Fatalf("intra scope = %v, want area (LS Type 10)", intra.Scope)
	}
	// AC-4: inter-area prefix connected in another area -> Route Type 3 + A-Flag; N-Flag
	// preserved because the connected prefix is a host (AC-6).
	if inter == nil {
		t.Fatalf("no inter-area (Route Type 3) prefix originated")
	}
	xp := decodeOnePrefix(t, *inter)
	if !xp.HasFlag(packet.ExtPrefixFlagA) {
		t.Fatalf("inter-area prefix attached in another area must set the A-Flag: %+v", xp)
	}
	if !xp.HasFlag(packet.ExtPrefixFlagN) {
		t.Fatalf("N-Flag must be preserved across ABR inter-area propagation (AC-6): %+v", xp)
	}
	if inter.Scope != OpaqueScopeArea {
		t.Fatalf("inter-area scope = %v, want area", inter.Scope)
	}
	// R-4: AS-external prefix -> Route Type 5 at AS scope (LS Type 11).
	if external == nil {
		t.Fatalf("no AS-external (Route Type 5) prefix originated")
	}
	if external.Scope != OpaqueScopeAS || external.Scope.lsType() != types.LSTypeOpaqueAS {
		t.Fatalf("external scope = %v, want AS (LS Type 11)", external.Scope)
	}
}

func TestExtPrefixNFlagPreservedInterArea(t *testing.T) {
	// A host /32 connected in area 0, summarized into area 1: the inter-area Extended Prefix
	// TLV preserves the N-Flag (RFC 7684 sec 2.1).
	router := types.RouterID{1, 1, 1, 1}
	topo := []ospflsdb.InterfaceInfo{extStubIface("lo0", [4]byte{10, 9, 9, 9}, [4]byte{255, 255, 255, 255})}
	eng, _ := extEngineWithTopology(t, topo)
	area1 := types.AreaID{0, 0, 0, 1}
	eng.lsdb.OriginateSummary(area1, router, 0, types.LSTypeSummaryNetwork, types.LinkStateID{10, 9, 9, 9}, [4]byte{255, 255, 255, 255}, 20)

	for _, o := range eng.extPrefixOnOriginate(router) {
		if o.Withdraw {
			continue
		}
		p := decodeOnePrefix(t, o)
		if p.RouteType == packet.ExtRouteTypeInterArea {
			// RFC requirement: RFC7684-2.1-2 positive -- a host /32 summarized between areas
			// keeps the N-Flag on the propagated inter-area Extended Prefix TLV
			// (selfPrefixAdverts, ext_prefix.go:154-157).
			if !p.HasFlag(packet.ExtPrefixFlagN) {
				t.Fatalf("N-Flag not preserved on inter-area host prefix: %+v", p)
			}
			return
		}
	}
	t.Fatalf("no inter-area prefix originated")
}

func TestExtPrefixNFlagNotSetNonHostInterArea(t *testing.T) {
	// A NON-host /24 connected in area 0, summarized into area 1: the inter-area Extended
	// Prefix TLV sets the A-Flag (locally connected in another area) but does NOT carry the
	// N-Flag, because N is preserved only for a host prefix (RFC 7684 sec 2.1). This confines
	// the N-Flag preservation to host prefixes so a non-host inter-area prefix never gains N.
	router := types.RouterID{1, 1, 1, 1}
	topo := []ospflsdb.InterfaceInfo{extStubIface("eth0", [4]byte{10, 0, 0, 1}, [4]byte{255, 255, 255, 0})}
	eng, _ := extEngineWithTopology(t, topo)
	area1 := types.AreaID{0, 0, 0, 1}
	eng.lsdb.OriginateSummary(area1, router, 0, types.LSTypeSummaryNetwork, types.LinkStateID{10, 0, 0, 0}, [4]byte{255, 255, 255, 0}, 20)

	for _, o := range eng.extPrefixOnOriginate(router) {
		if o.Withdraw {
			continue
		}
		p := decodeOnePrefix(t, o)
		if p.RouteType == packet.ExtRouteTypeInterArea {
			// RFC requirement: RFC7684-2.1-2 negative -- a non-host /24 propagated inter-area
			// gets the A-Flag but NOT the N-Flag; preservation carries the flag state through
			// faithfully (absent stays absent), it does not blanket-set N.
			if !p.HasFlag(packet.ExtPrefixFlagA) {
				t.Fatalf("inter-area prefix attached in another area must set the A-Flag: %+v", p)
			}
			if p.HasFlag(packet.ExtPrefixFlagN) {
				t.Fatalf("N-Flag must NOT be set on a non-host inter-area prefix: %+v", p)
			}
			return
		}
	}
	t.Fatalf("no inter-area prefix originated")
}

func TestExtPrefixScopeSelection(t *testing.T) {
	// Pure scope mapping: intra/inter-area -> area (10); external -> AS (11).
	// RFC requirement: RFC7684-2.1-3 positive -- intra-area and inter-area prefixes map to
	// area scope (LS Type 10), the scope that satisfies an area-local prefix (extPrefixScope,
	// ext.go:107).
	// RFC requirement: RFC7684-2.1-3 negative -- AS-external and NSSA-external prefixes map to
	// AS scope (LS Type 11), never area scope, so an AS-wide prefix is not under-flooded.
	for rt, want := range map[uint8]OpaqueScope{
		packet.ExtRouteTypeIntraArea:    OpaqueScopeArea,
		packet.ExtRouteTypeInterArea:    OpaqueScopeArea,
		packet.ExtRouteTypeASExternal:   OpaqueScopeAS,
		packet.ExtRouteTypeNSSAExternal: OpaqueScopeAS,
	} {
		if got := extPrefixScope(rt); got != want {
			t.Errorf("extPrefixScope(%d) = %v, want %v", rt, got, want)
		}
	}
}

func TestExtPrefixWithdrawOnPrefixGone(t *testing.T) {
	router := types.RouterID{1, 1, 1, 1}
	topo := []ospflsdb.InterfaceInfo{extStubIface("eth0", [4]byte{10, 0, 0, 1}, [4]byte{255, 255, 255, 0})}
	eng, _ := extEngineWithTopology(t, topo)

	first := eng.extPrefixOnOriginate(router)
	var origID uint32
	found := false
	for _, o := range first {
		if !o.Withdraw {
			origID = o.OpaqueID
			found = true
		}
	}
	if !found {
		t.Fatalf("no Extended Prefix LSA originated initially")
	}
	// The prefix disappears (interface goes down): the self Router-LSA is regenerated without
	// the stub link, so re-run must withdraw the same Opaque ID.
	down := []ospflsdb.InterfaceInfo{{Name: "eth0", AreaID: types.BackboneArea, Address: [4]byte{10, 0, 0, 1}, NetworkMask: [4]byte{255, 255, 255, 0}, State: ospflsdb.InterfaceStateDown}}
	eng.lsdb.SetTopology(func() []ospflsdb.InterfaceInfo { return down })
	eng.lsdb.OriginateFromTopology(router, false)
	second := eng.extPrefixOnOriginate(router)
	withdrawn := false
	for _, o := range second {
		if o.Withdraw && o.OpaqueID == origID {
			withdrawn = true
		}
	}
	if !withdrawn {
		t.Fatalf("prefix gone but no withdraw for Opaque ID %d: %+v", origID, second)
	}
}
