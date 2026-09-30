// VALIDATES: RFC 8665 Section 9 and RFC 8666 Section 10 on every OSPF reader of an SR
// sub-TLV: a Prefix-SID, Adj-SID, LAN Adj-SID or Extended Prefix Range whose length is
// invalid condemns the whole carrying LSA even when its V/L-Flags are invalid too, and a
// sub-TLV whose length is valid and whose flags alone are invalid is ignored by itself.
// The readers covered are the IPv4 Prefix-SID install (srRemotePrefixSIDs), the IPv4
// Extended Prefix receiver (extPrefixOnReceive, which feeds ResolvedPrefixes), the IPv4
// TI-LFA Adj-SID lookup (srRemoteAdjSID), and the OSPFv3 Extended-LSA gate every IPv6
// reader asks (srRemotePrefixSIDsV6, srV3ExtendedLengthInvalid).
// PREVENTS: a sub-TLV with a bad length and bad flags being judged a flag error, so the
// rest of a malformed LSA is applied.
package ospf

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
)

// srTestTLVBytes encodes one TLV or sub-TLV: Type(2) Length(2) Value, padded to 4 octets.
func srTestTLVBytes(typ uint16, value []byte) []byte {
	b := make([]byte, 4+(len(value)+3)/4*4)
	binary.BigEndian.PutUint16(b[0:], typ)
	binary.BigEndian.PutUint16(b[2:], uint16(len(value)))
	copy(b[4:], value)
	return b
}

// srTestBadFlags returns SID flags with V set and L clear: invalid V/L-Flags. The encoders
// size the SID by V AND L, so a value built with them is also one octet too long for the
// length the V-Flag selects. srTestFlagsOnly returns L set and V clear: invalid flags at
// a valid length.
func srTestBadFlags() sr.SIDFlags       { return sr.SIDFlags{V: true} }
func srTestFlagsOnly() sr.SIDFlags      { return sr.SIDFlags{L: true} }
func srTestBadAdjFlags() sr.AdjSIDFlags { return sr.AdjSIDFlags{V: true} }

// TestRFC8665LengthBeforeFlagsIgnoresLSA installs IPv4 Extended Prefix and Extended Link
// LSAs whose SR sub-TLVs carry an invalid length together with invalid flags, and reads
// them through the install, receiver and TI-LFA paths.
func TestRFC8665LengthBeforeFlagsIgnoresLSA(t *testing.T) {
	srTestReset(t)
	adv := types.RouterID{10, 0, 0, 9}
	neighbor := types.RouterID{10, 0, 0, 2}
	a, b := [4]byte{10, 0, 0, 9}, [4]byte{10, 0, 0, 10}
	fecA := netip.PrefixFrom(netip.AddrFrom4(a), 32)
	fecB := netip.PrefixFrom(netip.AddrFrom4(b), 32)
	good := srTestPrefixSIDSub(10, 0)
	sub := func(flags sr.SIDFlags) packet.ExtSubTLV {
		return packet.ExtSubTLV{Type: sr.V4TypePrefixSID, Value: sr.EncodePrefixSIDValue(sr.PrefixSID{Flags: flags, Index: 9})}
	}

	// RFC requirement: RFC8665-9-1 positive -- a Prefix-SID whose length is valid and whose
	// V/L-Flags alone are invalid is not a length error: it is ignored by itself, and the
	// other Prefix-SID of the same LSA is still received.
	eng, _ := newRedistEngine(t, extOrigCfg)
	srOriginateTestOpaque(t, eng, adv, packet.ExtPrefixOpaqueType, packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{
		Prefixes: []packet.ExtPrefixTLV{srTestPrefixTLV(a, sub(srTestFlagsOnly())), srTestPrefixTLV(b, good)},
	}))
	sids := eng.srRemotePrefixSIDs()
	if _, seen := sids[fecA]; seen || sids[fecB].SID.Index != 10 {
		t.Fatalf("a flag-only error must drop its own Prefix-SID and keep the LSA: %+v", sids)
	}

	// RFC requirement: RFC8665-9-1 negative -- a Prefix-SID or Extended Prefix Range whose
	// length is invalid makes the LSA malformed even when its V/L-Flags are invalid too, or
	// when a flag error comes first in the same range: no Prefix-SID from it is received.
	rangeFlagThenLength := append(
		sr.EncodeExtPrefixRangeValueV4(32, [4]byte{10, 0, 1, 0}, 4, false, sr.PrefixSID{Flags: srTestFlagsOnly(), Index: 40}),
		srTestTLVBytes(sr.V4TypePrefixSID, srLengthInvalid(sr.EncodePrefixSIDValue(sr.PrefixSID{Index: 41})))...)
	invalid := map[string]packet.ExtPrefixLSA{
		"prefix-sid length and flags": {Prefixes: []packet.ExtPrefixTLV{srTestPrefixTLV(a, sub(srTestBadFlags())), srTestPrefixTLV(b, good)}},
		"prefix-sid too short": {Prefixes: []packet.ExtPrefixTLV{
			srTestPrefixTLV(a, packet.ExtSubTLV{Type: sr.V4TypePrefixSID, Value: []byte{0, 0, 0}}), srTestPrefixTLV(b, good),
		}},
		"range length and flags": {
			Prefixes: []packet.ExtPrefixTLV{srTestPrefixTLV(b, good)},
			Ranges: []packet.ExtPrefixRangeTLV{{Value: sr.EncodeExtPrefixRangeValueV4(32, [4]byte{10, 0, 1, 0}, 4, false,
				sr.PrefixSID{Flags: srTestBadFlags(), Index: 40})}},
		},
		"range flag error then length error": {
			Prefixes: []packet.ExtPrefixTLV{srTestPrefixTLV(b, good)},
			Ranges:   []packet.ExtPrefixRangeTLV{{Value: rangeFlagThenLength}},
		},
	}
	for name, lsa := range invalid {
		eng, _ := newRedistEngine(t, extOrigCfg)
		srOriginateTestOpaque(t, eng, adv, packet.ExtPrefixOpaqueType, packet.EncodeExtPrefixLSA(lsa))
		if sids := eng.srRemotePrefixSIDs(); len(sids) != 0 {
			t.Fatalf("%s: a Prefix-SID from a malformed Extended Prefix LSA was received: %+v", name, sids)
		}
		// The Extended Prefix receiver ignores the same LSA, and withdraws what the
		// previous instance of it applied.
		recv := extRecvEngine(t)
		deliver := func(body []byte) {
			recv.extPrefixOnReceive(opaqueReceived{
				OpaqueType: packet.ExtPrefixOpaqueType, OpaqueID: 1, Scope: OpaqueScopeArea, AdvertisingRouter: adv,
				Body: body, Reachable: true,
			})
		}
		deliver(packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{Prefixes: []packet.ExtPrefixTLV{srTestPrefixTLV(b, good)}}))
		if _, ok := recv.extRecv.lookupPrefix(adv, types.BackboneArea, [5]byte{b[0], b[1], b[2], b[3], 32}); !ok {
			t.Fatalf("%s: control: the well-formed Extended Prefix LSA must be applied", name)
		}
		deliver(packet.EncodeExtPrefixLSA(lsa))
		if _, ok := recv.extRecv.lookupPrefix(adv, types.BackboneArea, [5]byte{b[0], b[1], b[2], b[3], 32}); ok {
			t.Fatalf("%s: the Extended Prefix receiver applied a prefix from a malformed LSA", name)
		}
	}

	// The Extended Link LSA: an Adj-SID or LAN Adj-SID with an invalid length and invalid
	// flags beside a well-formed Adj-SID makes the TI-LFA reader use no Adj-SID from it.
	label := sr.EncodeAdjSIDValue(sr.AdjSID{Flags: sr.AdjSIDFlags{V: true, L: true}, Label: 24001, IsLabel: true})
	for name, bad := range map[string]packet.ExtSubTLV{
		"adj-sid length and flags":     {Type: sr.V4TypeAdjSID, Value: sr.EncodeAdjSIDValue(sr.AdjSID{Flags: srTestBadAdjFlags(), Index: 3})},
		"lan-adj-sid length and flags": {Type: sr.V4TypeLANAdjSID, Value: sr.EncodeLANAdjSIDValue(sr.AdjSID{Flags: srTestBadAdjFlags(), Index: 3})},
		"adj-sid too short":            {Type: sr.V4TypeAdjSID, Value: []byte{0, 0, 0}},
	} {
		eng, _ := newRedistEngine(t, extOrigCfg)
		srOriginateTestOpaque(t, eng, adv, packet.ExtLinkOpaqueType, packet.EncodeExtLinkLSA(packet.ExtLinkTLV{
			LinkType: packet.RouterLinkTypeP2P, LinkID: [4]byte(neighbor), LinkData: [4]byte{10, 1, 0, 1},
			SubTLVs: []packet.ExtSubTLV{{Type: sr.V4TypeAdjSID, Value: label}, bad},
		}))
		if got, ok := eng.srRemoteAdjSID(adv, neighbor); ok {
			t.Fatalf("%s: an Adj-SID from a malformed Extended Link LSA was used: label %d", name, got)
		}
	}
}

// TestRFC8666LengthBeforeFlagsIgnoresLSA installs OSPFv3 E-Intra-Area-Prefix LSAs whose SR
// sub-TLVs carry an invalid length together with invalid flags, reads them through the
// IPv6 Prefix-SID reception, and asks the Extended-LSA gate about Router-Link TLVs.
func TestRFC8666LengthBeforeFlagsIgnoresLSA(t *testing.T) {
	r5 := types.RouterID{5, 5, 5, 5}
	a := netip.MustParsePrefix("2001:db8::5/128")
	b := netip.MustParsePrefix("2001:db8::6/128")
	good := v6TestSubTLV(v6TestPrefixSID(6, 0))
	withFlags := func(flags sr.SIDFlags) []byte {
		return v6TestSubTLV(sr.EncodePrefixSIDValueV6(sr.PrefixSID{Flags: flags, Index: 5}))
	}
	routerLink := func(typ uint16, value []byte) ospfv3packet.ExtendedTLV {
		return ospfv3packet.ExtendedTLV{Type: extTLVRouterLink, Value: append(make([]byte, 16), srTestTLVBytes(typ, value)...)}
	}

	// RFC requirement: RFC8666-10-1 positive -- a Prefix-SID whose length is valid and whose
	// V/L-Flags alone are invalid is ignored by itself: the other TLV's Prefix-SID is still
	// received, and a Router-Link TLV whose Adj-SID has only its flags wrong does not
	// condemn the LSA.
	eng := newV6RIEngine(t)
	installRemoteV6ExtTLVs(t, eng, r5, v6TestPrefixTLV(t, a, withFlags(srTestFlagsOnly())), v6TestPrefixTLV(t, b, good))
	sids := eng.srRemotePrefixSIDsV6()
	if _, seen := sids[a]; seen || sids[b].SID.Index != 6 {
		t.Fatalf("a flag-only error must drop its own Prefix-SID and keep the LSA: %+v", sids)
	}
	flagsOnlyAdj := sr.EncodeAdjSIDValueV6(sr.AdjSID{Flags: sr.AdjSIDFlags{L: true}, Index: 3})
	if srV3ExtendedLengthInvalid([]ospfv3packet.ExtendedTLV{routerLink(sr.V6TypeAdjSID, flagsOnlyAdj)}) {
		t.Fatal("an Adj-SID with a valid length and invalid flags must not condemn the LSA")
	}

	// RFC requirement: RFC8666-10-1 negative -- a Prefix-SID, an Extended Prefix Range, an
	// Adj-SID or a LAN Adj-SID whose length is invalid makes the LSA malformed even when its
	// V/L-Flags are invalid too: no Prefix-SID from it is received, and the gate every
	// OSPFv3 reader asks condemns the Router-Link TLV that carries the Adj-SID.
	invalid := map[string][]ospfv3packet.ExtendedTLV{
		"prefix-sid length and flags": {v6TestPrefixTLV(t, a, withFlags(srTestBadFlags())), v6TestPrefixTLV(t, b, good)},
		"prefix-sid too short":        {v6TestPrefixTLV(t, a, v6TestSubTLV([]byte{0, 0, 0})), v6TestPrefixTLV(t, b, good)},
		"range length and flags": {v6TestPrefixTLV(t, b, good), {Type: extTLVExtPrefixRange,
			Value: sr.EncodeExtPrefixRangeValueV6(128, make([]byte, 16), 4, sr.PrefixSID{Flags: srTestBadFlags(), Index: 40})}},
	}
	for name, tlvs := range invalid {
		eng := newV6RIEngine(t)
		installRemoteV6ExtTLVs(t, eng, r5, tlvs...)
		if sids := eng.srRemotePrefixSIDsV6(); len(sids) != 0 {
			t.Fatalf("%s: a Prefix-SID from a malformed Extended-LSA was received: %+v", name, sids)
		}
	}
	badAdj := sr.EncodeAdjSIDValueV6(sr.AdjSID{Flags: srTestBadAdjFlags(), Index: 3})
	badLAN := sr.EncodeLANAdjSIDValueV6(sr.AdjSID{Flags: srTestBadAdjFlags(), Index: 3})
	for name, tlv := range map[string]ospfv3packet.ExtendedTLV{
		"adj-sid length and flags":     routerLink(sr.V6TypeAdjSID, badAdj),
		"lan-adj-sid length and flags": routerLink(sr.V6TypeLANAdjSID, badLAN),
		"adj-sid too short":            routerLink(sr.V6TypeAdjSID, []byte{0, 0, 0}),
	} {
		if !srV3ExtendedLengthInvalid([]ospfv3packet.ExtendedTLV{tlv}) {
			t.Fatalf("%s: the Router-Link TLV must condemn the Extended-LSA", name)
		}
	}
}
