// VALIDATES: RFC 8665 Section 5 and RFC 8666 Section 6 read literally: "If an OSPF router
// advertises multiple Prefix-SIDs for the same prefix, topology, and algorithm, all of
// them MUST be ignored." The rule is per advertising router, and it holds when the
// multiple Prefix-SIDs bind the SAME SID. It does not reach Prefix-SIDs of another
// topology (MT-ID), of another algorithm, of another router (an anycast prefix, RFC 8660
// Section 2.5), or of the same router in another area (an ABR, RFC 8665 Section 7.2).
// PREVENTS: a router's repeated Prefix-SID being accepted because its values happen to be
// equal, and the per-router rule being widened into a rule over every advertiser.
package ospf

import (
	"net/netip"
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// srInstallPrefixSIDs installs one area-scoped Extended Prefix LSA from adv in area,
// advertising 10.0.0.9/32 with one Prefix-SID sub-TLV per entry of sids.
func srInstallPrefixSIDs(t *testing.T, eng *engine, adv types.RouterID, area types.AreaID, opaqueID uint32, sids ...sr.PrefixSID) {
	t.Helper()
	subs := make([]packet.ExtSubTLV, 0, len(sids))
	for _, ps := range sids {
		subs = append(subs, packet.ExtSubTLV{Type: sr.V4TypePrefixSID, Value: sr.EncodePrefixSIDValue(ps)})
	}
	body := packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{Prefixes: []packet.ExtPrefixTLV{srTestPrefixTLV([4]byte{10, 0, 0, 9}, subs...)}})
	if _, ok := eng.lsdb.OriginateOpaque(ospflsdb.OpaqueOriginateInput{
		Router: adv, OpaqueType: packet.ExtPrefixOpaqueType, OpaqueID: opaqueID,
		Scope: types.LSTypeOpaqueArea, Area: area, Options: types.OptionO, Body: body,
	}); !ok {
		t.Fatalf("installing Extended Prefix LSA %d in area %s failed", opaqueID, area)
	}
}

// TestRFC8665OneRouterMultiplePrefixSIDsAllIgnored installs, for 10.0.0.9/32, Prefix-SIDs
// that one router repeats in one scope, then Prefix-SIDs that differ only by topology,
// router or area, and reads the received Prefix-SID for the prefix.
func TestRFC8665OneRouterMultiplePrefixSIDsAllIgnored(t *testing.T) {
	srTestReset(t)
	r9, r10 := types.RouterID{10, 0, 0, 9}, types.RouterID{10, 0, 0, 10}
	area1 := types.AreaID{0, 0, 0, 1}
	fec := netip.MustParsePrefix("10.0.0.9/32")
	nine := sr.PrefixSID{Flags: sr.SIDFlags{NP: true}, Index: 9}
	nineTopology5 := nine
	nineTopology5.MTID = 5

	// RFC requirement: RFC8665-5-6 negative -- one router advertising the SAME Prefix-SID
	// twice for one prefix, topology and algorithm, in two LSAs or in two sub-TLVs of one
	// TLV, has all of them ignored: the prefix is marked Duplicate.
	for name, install := range map[string]func(*engine){
		"two LSAs": func(eng *engine) {
			srInstallPrefixSIDs(t, eng, r9, types.BackboneArea, 1, nine)
			srInstallPrefixSIDs(t, eng, r9, types.BackboneArea, 2, nine)
		},
		"two sub-TLVs": func(eng *engine) { srInstallPrefixSIDs(t, eng, r9, types.BackboneArea, 1, nine, nine) },
	} {
		eng, _ := newRedistEngine(t, extOrigCfg)
		install(eng)
		if rs := eng.srRemotePrefixSIDs()[fec]; !rs.Duplicate {
			t.Fatalf("%s: one router's repeated Prefix-SID must be ignored: %+v", name, rs)
		}
	}

	// RFC requirement: RFC8665-5-6 positive -- the rule stops at its scope: the same SID
	// under another topology (MT-ID 5) from the same router, the same SID from another
	// router, the same SID from the same router in another area, and a Prefix-SID the same
	// router advertises for another prefix are not multiple
	// Prefix-SIDs for one prefix, topology and algorithm, so the Prefix-SID is kept.
	for name, install := range map[string]func(*engine){
		"another topology": func(eng *engine) { srInstallPrefixSIDs(t, eng, r9, types.BackboneArea, 1, nine, nineTopology5) },
		"another router": func(eng *engine) {
			srInstallPrefixSIDs(t, eng, r9, types.BackboneArea, 1, nine)
			srInstallPrefixSIDs(t, eng, r10, types.BackboneArea, 1, nine)
		},
		"another area": func(eng *engine) {
			srInstallPrefixSIDs(t, eng, r9, types.BackboneArea, 1, nine)
			srInstallPrefixSIDs(t, eng, r9, area1, 1, nine)
		},
		"another prefix": func(eng *engine) {
			srOriginateTestOpaque(t, eng, r9, packet.ExtPrefixOpaqueType, packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{
				Prefixes: []packet.ExtPrefixTLV{srTestPrefixTLV([4]byte{10, 0, 0, 9}, srTestPrefixSIDSub(9, 0)),
					srTestPrefixTLV([4]byte{10, 0, 0, 10}, srTestPrefixSIDSub(10, 0))},
			}))
		},
	} {
		eng, _ := newRedistEngine(t, extOrigCfg)
		install(eng)
		rs, ok := eng.srRemotePrefixSIDs()[fec]
		if !ok || rs.Duplicate || rs.SID.MTID != 0 || rs.SID.Index != 9 {
			t.Fatalf("%s: the topology 0 Prefix-SID must be kept: %+v (present %v)", name, rs, ok)
		}
	}
}

// TestRFC8666OneRouterMultiplePrefixSIDsAllIgnored installs OSPFv3 E-Intra-Area-Prefix
// LSAs in which one router repeats a Prefix-SID for one prefix and algorithm, then ones in
// which the same SID comes from another router or another area.
func TestRFC8666OneRouterMultiplePrefixSIDsAllIgnored(t *testing.T) {
	r5, r6 := types.RouterID{5, 5, 5, 5}, types.RouterID{6, 6, 6, 6}
	area1 := types.AreaID{0, 0, 0, 1}
	loop := netip.MustParsePrefix("2001:db8::5/128")
	five := []sr.PrefixSIDConfig{{Prefix: loop, Index: 5}}

	// RFC requirement: RFC8666-6-7 positive -- one router advertising the SAME Prefix-SID
	// twice for one prefix and algorithm, in two sub-TLVs of one TLV or in two LSAs, has
	// all of them ignored: the prefix is marked Duplicate.
	eng := newV6RIEngine(t)
	installRemoteV6ExtTLVs(t, eng, r5, v6TestPrefixTLV(t, loop,
		v6TestSubTLV(v6TestPrefixSID(5, 0)), v6TestSubTLV(v6TestPrefixSID(5, 0))))
	if rs := eng.srRemotePrefixSIDsV6()[loop]; !rs.Duplicate {
		t.Fatalf("two sub-TLVs: one router's repeated Prefix-SID must be ignored: %+v", rs)
	}
	eng = newV6RIEngine(t)
	installRemoteV6EPrefix(t, eng, types.BackboneArea, r5, 1, five)
	installRemoteV6EPrefix(t, eng, types.BackboneArea, r5, 2, five)
	if rs := eng.srRemotePrefixSIDsV6()[loop]; !rs.Duplicate {
		t.Fatalf("two LSAs: one router's repeated Prefix-SID must be ignored: %+v", rs)
	}

	// RFC requirement: RFC8666-6-7 negative -- the same SID from another router, or from
	// the same router in another area, and a Prefix-SID the same router advertises for
	// another prefix, are not multiple Prefix-SIDs from one router: the Prefix-SID is kept.
	for name, install := range map[string]func(*engine){
		"another router": func(eng *engine) {
			installRemoteV6EPrefix(t, eng, types.BackboneArea, r5, 1, five)
			installRemoteV6EPrefix(t, eng, types.BackboneArea, r6, 1, five)
		},
		"another area": func(eng *engine) {
			installRemoteV6EPrefix(t, eng, types.BackboneArea, r5, 1, five)
			installRemoteV6EPrefix(t, eng, area1, r5, 1, five)
		},
		"another prefix": func(eng *engine) {
			installRemoteV6ExtTLVs(t, eng, r5, v6TestPrefixTLV(t, loop, v6TestSubTLV(v6TestPrefixSID(5, 0))),
				v6TestPrefixTLV(t, netip.MustParsePrefix("2001:db8::6/128"), v6TestSubTLV(v6TestPrefixSID(6, 0))))
		},
	} {
		eng := newV6RIEngine(t)
		install(eng)
		rs, ok := eng.srRemotePrefixSIDsV6()[loop]
		if !ok || rs.Duplicate || rs.SID.Index != 5 {
			t.Fatalf("%s: the Prefix-SID must be kept: %+v (present %v)", name, rs, ok)
		}
	}
}
