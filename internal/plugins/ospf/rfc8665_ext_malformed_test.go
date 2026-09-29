// VALIDATES: RFC 8665 Section 9 on the Extended Prefix and Extended Link Opaque LSA paths:
// an SR TLV or sub-TLV of invalid length makes the whole carrying LSA malformed, so no
// Prefix-SID or Adj-SID from it is used, including the well-formed ones beside the bad one.
// Also RFC 8665 Section 5: Prefix-SIDs of DIFFERENT algorithms for one prefix are not the
// "multiple Prefix-SIDs for the same prefix, topology, and algorithm" that are ignored.
// PREVENTS: the reception, TI-LFA and BGP-LS readers skipping only the bad sub-TLV and
// applying the rest of a malformed LSA.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/linkstateevents"
	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// srOriginateTestOpaque installs one area-scoped opaque LSA body as if received from adv.
func srOriginateTestOpaque(t *testing.T, eng *engine, adv types.RouterID, opaqueType uint8, body []byte) {
	t.Helper()
	if _, ok := eng.lsdb.OriginateOpaque(ospflsdb.OpaqueOriginateInput{
		Router:     adv,
		OpaqueType: opaqueType,
		OpaqueID:   1,
		Scope:      types.LSTypeOpaqueArea,
		Area:       types.BackboneArea,
		Options:    types.OptionO,
		Body:       body,
	}); !ok {
		t.Fatalf("installing opaque LSA type %d failed", opaqueType)
	}
}

// srTestPrefixTLV returns an intra-area Extended Prefix TLV for a /32 carrying the sub-TLVs.
func srTestPrefixTLV(addr [4]byte, subs ...packet.ExtSubTLV) packet.ExtPrefixTLV {
	return packet.ExtPrefixTLV{
		RouteType:     packet.ExtRouteTypeIntraArea,
		PrefixLength:  32,
		AF:            packet.ExtPrefixAFIPv4Unicast,
		AddressPrefix: addr,
		SubTLVs:       subs,
	}
}

// srTestPrefixSIDSub returns a Prefix-SID sub-TLV for index under algorithm.
func srTestPrefixSIDSub(index uint32, algorithm uint8) packet.ExtSubTLV {
	value := sr.EncodePrefixSIDValue(sr.PrefixSID{Flags: sr.SIDFlags{NP: true}, Algorithm: algorithm, Index: index})
	return packet.ExtSubTLV{Type: sr.V4TypePrefixSID, Value: value}
}

// srLengthInvalid appends one octet to a well-formed value, so its length is no longer
// one the RFC allows for the field layout its flags select.
func srLengthInvalid(value []byte) []byte {
	return append(append([]byte(nil), value...), 0)
}

// TestRFC8665LengthInvalidSubTLVIgnoresExtendedPrefixLSA installs an Extended Prefix LSA
// with two prefixes, first well-formed, then with one Prefix-SID sub-TLV (or one Extended
// Prefix Range TLV) of invalid length, and reads the received Prefix-SIDs.
func TestRFC8665LengthInvalidSubTLVIgnoresExtendedPrefixLSA(t *testing.T) {
	srTestReset(t)
	adv := types.RouterID{10, 0, 0, 9}
	a, b := [4]byte{10, 0, 0, 9}, [4]byte{10, 0, 0, 10}
	fecA := netip.PrefixFrom(netip.AddrFrom4(a), 32)
	fecB := netip.PrefixFrom(netip.AddrFrom4(b), 32)
	good := srTestPrefixSIDSub(10, 0)
	rangeValue := sr.EncodeExtPrefixRangeValueV4(32, [4]byte{10, 0, 1, 0}, 4, false, sr.PrefixSID{Index: 40})

	// RFC requirement: RFC8665-9-1 positive -- an Extended Prefix LSA whose SR TLVs and
	// sub-TLVs all carry a valid length is applied: both Prefix-SIDs are received.
	eng, _ := newRedistEngine(t, extOrigCfg)
	srOriginateTestOpaque(t, eng, adv, packet.ExtPrefixOpaqueType, packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{
		Prefixes: []packet.ExtPrefixTLV{srTestPrefixTLV(a, srTestPrefixSIDSub(9, 0)), srTestPrefixTLV(b, good)},
		Ranges:   []packet.ExtPrefixRangeTLV{{Value: rangeValue}},
	}))
	sids := eng.srRemotePrefixSIDs()
	if sids[fecA].SID.Index != 9 || sids[fecB].SID.Index != 10 {
		t.Fatalf("well-formed Extended Prefix LSA not applied: %+v", sids)
	}

	// RFC requirement: RFC8665-9-1 negative -- one SR sub-TLV or TLV of invalid length (a
	// Prefix-SID one octet too long, an Extended Prefix Range TLV cut inside its fixed part)
	// makes the Extended Prefix LSA malformed, and it MUST be ignored: the well-formed
	// Prefix-SID for the other prefix in the same LSA is not received either.
	invalid := map[string]packet.ExtPrefixLSA{
		"prefix-sid-long": {Prefixes: []packet.ExtPrefixTLV{
			srTestPrefixTLV(a, packet.ExtSubTLV{Type: sr.V4TypePrefixSID, Value: srLengthInvalid(good.Value)}),
			srTestPrefixTLV(b, good),
		}},
		"range-short": {
			Prefixes: []packet.ExtPrefixTLV{srTestPrefixTLV(b, good)},
			Ranges:   []packet.ExtPrefixRangeTLV{{Value: rangeValue[:10]}},
		},
	}
	for name, lsa := range invalid {
		eng, _ := newRedistEngine(t, extOrigCfg)
		srOriginateTestOpaque(t, eng, adv, packet.ExtPrefixOpaqueType, packet.EncodeExtPrefixLSA(lsa))
		if sids := eng.srRemotePrefixSIDs(); len(sids) != 0 {
			t.Fatalf("%s: a Prefix-SID from a malformed Extended Prefix LSA was received: %+v", name, sids)
		}
	}
}

// TestRFC8665LengthInvalidAdjSIDIgnoresExtendedLinkLSA installs an Extended Link LSA for a
// point-to-point link carrying a well-formed label Adj-SID, then the same LSA with a second
// Adj-SID sub-TLV of invalid length, and asks the TI-LFA reader for the Adj-SID label.
func TestRFC8665LengthInvalidAdjSIDIgnoresExtendedLinkLSA(t *testing.T) {
	srTestReset(t)
	adv := types.RouterID{10, 0, 0, 9}
	neighbor := types.RouterID{10, 0, 0, 2}
	label := sr.EncodeAdjSIDValue(sr.AdjSID{Flags: sr.AdjSIDFlags{V: true, L: true}, Label: 24001, IsLabel: true})
	link := func(subs ...packet.ExtSubTLV) []byte {
		return packet.EncodeExtLinkLSA(packet.ExtLinkTLV{
			LinkType: packet.RouterLinkTypeP2P, LinkID: [4]byte(neighbor), LinkData: [4]byte{10, 1, 0, 1}, SubTLVs: subs,
		})
	}

	// RFC requirement: RFC8665-9-1 positive -- an Extended Link LSA whose Adj-SID sub-TLVs
	// carry a valid length is applied: the Adj-SID label toward the neighbor is used.
	eng, _ := newRedistEngine(t, extOrigCfg)
	srOriginateTestOpaque(t, eng, adv, packet.ExtLinkOpaqueType, link(packet.ExtSubTLV{Type: sr.V4TypeAdjSID, Value: label}))
	if got, ok := eng.srRemoteAdjSID(adv, neighbor); !ok || got != 24001 {
		t.Fatalf("well-formed Extended Link LSA not applied: label %d ok %v", got, ok)
	}

	// RFC requirement: RFC8665-9-1 negative -- a second Adj-SID sub-TLV one octet too long
	// makes the Extended Link LSA malformed, so the well-formed Adj-SID beside it is ignored.
	eng, _ = newRedistEngine(t, extOrigCfg)
	srOriginateTestOpaque(t, eng, adv, packet.ExtLinkOpaqueType, link(
		packet.ExtSubTLV{Type: sr.V4TypeAdjSID, Value: label},
		packet.ExtSubTLV{Type: sr.V4TypeAdjSID, Value: srLengthInvalid(label)},
	))
	if got, ok := eng.srRemoteAdjSID(adv, neighbor); ok {
		t.Fatalf("an Adj-SID from a malformed Extended Link LSA was used: label %d", got)
	}
}

// TestRFC8665LengthInvalidSubTLVIgnoredByBGPLSExport exports an Extended Prefix LSA whose
// only valid Prefix-SID sits beside a length-invalid Prefix-SID in another prefix TLV, and
// checks that BGP-LS carries no Prefix-SID from it. The control half exports the same LSA
// without the bad sub-TLV and finds the Prefix-SID, so the absence is the malformed verdict.
//
// RFC requirement: RFC8665-9-1 positive -- the BGP-LS export of a well-formed Extended Prefix
// LSA carries its Prefix-SID.
// RFC requirement: RFC8665-9-1 negative -- the BGP-LS export of an Extended Prefix LSA holding
// a length-invalid Prefix-SID sub-TLV (alone, or with invalid flags too) carries no Prefix-SID
// from that LSA, including the valid Prefix-SID in its other prefix TLV.
func TestRFC8665LengthInvalidSubTLVIgnoredByBGPLSExport(t *testing.T) {
	valid := sr.EncodePrefixSIDValue(sr.PrefixSID{MTID: 9, Index: 77})
	first := packet.ExtPrefixTLV{RouteType: packet.ExtRouteTypeIntraArea, PrefixLength: 24, AddressPrefix: [4]byte{192, 0, 2, 0},
		SubTLVs: []packet.ExtSubTLV{{Type: sr.V4TypePrefixSID, Value: valid}}}
	second := packet.ExtPrefixTLV{RouteType: packet.ExtRouteTypeIntraArea, PrefixLength: 24, AddressPrefix: [4]byte{198, 51, 100, 0},
		SubTLVs: []packet.ExtSubTLV{{Type: sr.V4TypePrefixSID, Value: srLengthInvalid(valid)}}}
	// A well-formed marker LSA installed after the one under test: the snapshot that carries
	// the marker also reflects the LSA under test, whichever way it was judged.
	marker := netip.MustParsePrefix("203.0.113.0/24")
	markerBody := packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{Prefixes: []packet.ExtPrefixTLV{{
		RouteType: packet.ExtRouteTypeIntraArea, PrefixLength: 24, AddressPrefix: [4]byte{203, 0, 113, 0},
		SubTLVs: []packet.ExtSubTLV{{Type: sr.V4TypePrefixSID, Value: sr.EncodePrefixSIDValue(sr.PrefixSID{Index: 5})}},
	}}})
	exported := func(prefixes ...packet.ExtPrefixTLV) bool {
		e, bus, events := bgplsTestSource(t, false)
		if !e.lsdb.Install(types.BackboneArea, bgplsTestRouter(10, types.InitialSequenceNumber)) {
			t.Fatal("install base router")
		}
		bgplsInstallOpaque(t, e, packet.ExtPrefixOpaqueType, packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{Prefixes: prefixes}))
		if !e.lsdb.Install(types.BackboneArea, packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeOpaqueArea,
			LinkStateID: types.LinkStateID{packet.ExtPrefixOpaqueType, 0, 0, 2}, AdvertisingRouter: types.RouterID{2, 2, 2, 2},
			Sequence: types.InitialSequenceNumber}, Body: markerBody}) {
			t.Fatal("install marker LSA")
		}
		if _, err := linkstateevents.Request.Emit(bus); err != nil {
			t.Fatal(err)
		}
		snapshot := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
			for _, p := range s.Prefixes {
				if p.Prefix == marker && bgplsTestAttribute(p.Attributes, 1158) != nil {
					return true
				}
			}
			return false
		})
		for _, p := range snapshot.Prefixes {
			if p.Prefix != marker && bgplsTestAttribute(p.Attributes, 1158) != nil {
				return true
			}
		}
		return false
	}
	if !exported(first) {
		t.Fatal("control: the well-formed Extended Prefix LSA must export its Prefix-SID")
	}
	if exported(first, second) {
		t.Fatal("BGP-LS exported a Prefix-SID from a malformed Extended Prefix LSA")
	}
	// A Prefix-SID whose length AND V/L-Flags are invalid (V set, L clear, with a 4-octet
	// SID where the V-Flag makes the length 7) is a length error, so the LSA is not exported.
	lengthAndFlags := second
	lengthAndFlags.SubTLVs = []packet.ExtSubTLV{{Type: sr.V4TypePrefixSID,
		Value: sr.EncodePrefixSIDValue(sr.PrefixSID{Flags: sr.SIDFlags{V: true}, MTID: 9, Index: 78})}}
	if exported(first, lengthAndFlags) {
		t.Fatal("BGP-LS exported a Prefix-SID from an Extended Prefix LSA whose sub-TLV has an invalid length and invalid flags")
	}
}

// TestRFC8665PrefixSIDsOfDifferentAlgorithmsNotDuplicate installs one Extended Prefix TLV
// carrying a Prefix-SID for Algorithm 1 and one for Algorithm 0, then a TLV carrying two
// differing Prefix-SIDs for Algorithm 0.
func TestRFC8665PrefixSIDsOfDifferentAlgorithmsNotDuplicate(t *testing.T) {
	srTestReset(t)
	adv := types.RouterID{10, 0, 0, 9}
	addr := [4]byte{10, 0, 0, 9}
	fec := netip.PrefixFrom(netip.AddrFrom4(addr), 32)

	// RFC requirement: RFC8665-5-6 positive -- Prefix-SIDs for one prefix under DIFFERENT
	// algorithms are not "multiple Prefix-SIDs for the same prefix, topology, and
	// algorithm": the Algorithm 0 Prefix-SID is kept, not marked Duplicate.
	eng, _ := newRedistEngine(t, extOrigCfg)
	srOriginateTestOpaque(t, eng, adv, packet.ExtPrefixOpaqueType, packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{
		Prefixes: []packet.ExtPrefixTLV{srTestPrefixTLV(addr, srTestPrefixSIDSub(7, 1), srTestPrefixSIDSub(9, 0))},
	}))
	if rs := eng.srRemotePrefixSIDs()[fec]; rs.Duplicate || rs.SID.Algorithm != 0 || rs.SID.Index != 9 {
		t.Fatalf("the Algorithm 0 Prefix-SID must be kept beside an Algorithm 1 one: %+v", rs)
	}

	// RFC requirement: RFC8665-5-6 negative -- two differing Prefix-SIDs for the same prefix
	// and algorithm in one TLV are all ignored: the prefix is marked Duplicate.
	eng, _ = newRedistEngine(t, extOrigCfg)
	srOriginateTestOpaque(t, eng, adv, packet.ExtPrefixOpaqueType, packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{
		Prefixes: []packet.ExtPrefixTLV{srTestPrefixTLV(addr, srTestPrefixSIDSub(9, 0), srTestPrefixSIDSub(11, 0))},
	}))
	if rs := eng.srRemotePrefixSIDs()[fec]; !rs.Duplicate {
		t.Fatalf("two differing Algorithm 0 Prefix-SIDs must be marked Duplicate: %+v", rs)
	}
}
