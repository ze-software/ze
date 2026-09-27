// VALIDATES: RFC 8666 Section 10 on the OSPFv3 Extended-LSA reception path: a Prefix-SID
// sub-TLV of invalid length makes the whole carrying LSA malformed, so no Prefix-SID from
// it is received. RFC 8666 Section 6: only multiple Prefix-SIDs for the same prefix,
// topology AND algorithm are ignored; Prefix-SIDs for different algorithms are not.
// PREVENTS: v6PrefixSIDFromPrefixTLV dropping only its own TLV while the other TLVs of a
// malformed LSA are still consumed, and a second Prefix-SID sub-TLV in one TLV being
// silently dropped (first wins) or colliding with a Prefix-SID of another algorithm.
package ospf

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// v6TestSubTLV encodes one OSPFv3 Prefix-SID sub-TLV: Type(2) Length(2) Value, padded to 4 octets.
func v6TestSubTLV(value []byte) []byte {
	b := make([]byte, 4+(len(value)+3)/4*4)
	binary.BigEndian.PutUint16(b[0:], sr.V6TypePrefixSID)
	binary.BigEndian.PutUint16(b[2:], uint16(len(value)))
	copy(b[4:], value)
	return b
}

// v6TestPrefixTLV returns an Intra-Area-Prefix TLV for pfx whose sub-TLV region is subs.
func v6TestPrefixTLV(t *testing.T, pfx netip.Prefix, subs ...[]byte) ospfv3packet.ExtendedTLV {
	t.Helper()
	base, ok := v6IntraAreaPrefixTLV(sr.PrefixSIDConfig{Prefix: pfx})
	if !ok {
		t.Fatalf("building the Intra-Area-Prefix TLV for %s failed", pfx)
	}
	value := append([]byte(nil), base.Value[:8+v6PrefixTLVWordBytes(uint8(pfx.Bits()))]...)
	for _, s := range subs {
		value = append(value, s...)
	}
	return ospfv3packet.ExtendedTLV{Type: extTLVIntraAreaPrefix, Value: value}
}

// v6TestPrefixSID returns a Prefix-SID sub-TLV value for index under algorithm.
func v6TestPrefixSID(index uint32, algorithm uint8) []byte {
	return sr.EncodePrefixSIDValueV6(sr.PrefixSID{Algorithm: algorithm, Index: index})
}

// installRemoteV6ExtTLVs installs one E-Intra-Area-Prefix LSA from originator carrying tlvs.
func installRemoteV6ExtTLVs(t *testing.T, eng *engine, originator types.RouterID, tlvs ...ospfv3packet.ExtendedTLV) {
	t.Helper()
	ext := ospfv3packet.EncodeExtendedLSABody(ospfv3packet.ExtendedLSA{TLVs: tlvs})
	body := make([]byte, eIntraPrefixHeaderLen+len(ext))
	binary.BigEndian.PutUint16(body[2:], uint16(ospfv3types.LSTypeRouter))
	copy(body[8:12], originator[:])
	copy(body[eIntraPrefixHeaderLen:], ext)
	enc := v6SelfExtEncoder(ospfv3types.LSTypeEIntraAreaPrefix, ospfv3types.LinkStateID(v6SummaryLSID(1)), originator, body)
	if !eng.lsdb.Install(types.BackboneArea, enc(types.LSSequenceNumber(0x80000001), false)) {
		t.Fatal("install remote E-Intra-Area-Prefix LSA failed")
	}
}

// TestRFC8666LengthInvalidSubTLVIgnoresExtendedLSA installs an E-Intra-Area-Prefix LSA with
// two prefix TLVs, first well-formed, then with one Prefix-SID sub-TLV one octet too long.
func TestRFC8666LengthInvalidSubTLVIgnoresExtendedLSA(t *testing.T) {
	r5 := types.RouterID{5, 5, 5, 5}
	a := netip.MustParsePrefix("2001:db8::5/128")
	b := netip.MustParsePrefix("2001:db8::6/128")
	good := v6TestSubTLV(v6TestPrefixSID(6, 0))

	// RFC requirement: RFC8666-10-1 positive -- an Extended-LSA whose Prefix-SID sub-TLVs
	// all carry a valid length is applied: both Prefix-SIDs are received.
	eng := newV6RIEngine(t)
	installRemoteV6ExtTLVs(t, eng, r5,
		v6TestPrefixTLV(t, a, v6TestSubTLV(v6TestPrefixSID(5, 0))), v6TestPrefixTLV(t, b, good))
	sids := eng.srRemotePrefixSIDsV6()
	if sids[a].SID.Index != 5 || sids[b].SID.Index != 6 {
		t.Fatalf("well-formed Extended-LSA not applied: %+v", sids)
	}

	// RFC requirement: RFC8666-10-1 negative -- a Prefix-SID sub-TLV one octet too long
	// makes the LSA malformed, and it MUST be ignored: the well-formed Prefix-SID in the
	// other TLV of the same LSA is not received either.
	eng = newV6RIEngine(t)
	installRemoteV6ExtTLVs(t, eng, r5,
		v6TestPrefixTLV(t, a, v6TestSubTLV(srLengthInvalid(v6TestPrefixSID(5, 0)))), v6TestPrefixTLV(t, b, good))
	if sids := eng.srRemotePrefixSIDsV6(); len(sids) != 0 {
		t.Fatalf("a Prefix-SID from a malformed Extended-LSA was received: %+v", sids)
	}
}

// TestRFC8666PrefixSIDsKeyedByAlgorithm installs one Intra-Area-Prefix TLV carrying two
// differing Prefix-SIDs for Algorithm 0, then one carrying an Algorithm 1 and an Algorithm 0
// Prefix-SID, and reads the aggregated Prefix-SID for the prefix.
func TestRFC8666PrefixSIDsKeyedByAlgorithm(t *testing.T) {
	r5 := types.RouterID{5, 5, 5, 5}
	loop := netip.MustParsePrefix("2001:db8::5/128")

	// RFC requirement: RFC8666-6-7 positive -- one router advertising two differing
	// Prefix-SIDs for the same prefix and algorithm (two sub-TLVs of one TLV) has all of
	// them ignored: the prefix is marked Duplicate, not resolved to the first sub-TLV.
	eng := newV6RIEngine(t)
	installRemoteV6ExtTLVs(t, eng, r5, v6TestPrefixTLV(t, loop,
		v6TestSubTLV(v6TestPrefixSID(5, 0)), v6TestSubTLV(v6TestPrefixSID(9, 0))))
	if rs := eng.srRemotePrefixSIDsV6()[loop]; !rs.Duplicate {
		t.Fatalf("two differing Algorithm 0 Prefix-SIDs from one router must be marked Duplicate: %+v", rs)
	}

	// RFC requirement: RFC8666-6-7 negative -- Prefix-SIDs for the same prefix under
	// DIFFERENT algorithms are not ignored: the Algorithm 0 Prefix-SID is kept even when an
	// Algorithm 1 Prefix-SID precedes it.
	eng = newV6RIEngine(t)
	installRemoteV6ExtTLVs(t, eng, r5, v6TestPrefixTLV(t, loop,
		v6TestSubTLV(v6TestPrefixSID(7, 1)), v6TestSubTLV(v6TestPrefixSID(5, 0))))
	if rs := eng.srRemotePrefixSIDsV6()[loop]; rs.Duplicate || rs.SID.Algorithm != 0 || rs.SID.Index != 5 {
		t.Fatalf("the Algorithm 0 Prefix-SID must be kept beside an Algorithm 1 one: %+v", rs)
	}
}

// v6PrefixSIDFromTLV returns the prefix and the FIRST Prefix-SID of one Extended-LSA TLV,
// for the tests that build a TLV carrying exactly one Prefix-SID. Reception reads every
// Prefix-SID through v6PrefixSIDsFromTLV.
func v6PrefixSIDFromTLV(tlv ospfv3packet.ExtendedTLV) (netip.Prefix, sr.PrefixSID, bool) {
	pfx, sids := v6PrefixSIDsFromTLV(tlv, nil)
	if len(sids) == 0 {
		return netip.Prefix{}, sr.PrefixSID{}, false
	}
	return pfx, sids[0], true
}

// v6PrefixSIDFromPrefixTLV is v6PrefixSIDFromTLV for a bare Intra/Inter/External Prefix
// TLV value.
func v6PrefixSIDFromPrefixTLV(value []byte) (netip.Prefix, sr.PrefixSID, bool) {
	pfx, sids := v6PrefixSIDsFromPrefixTLV(value, nil)
	if len(sids) == 0 {
		return netip.Prefix{}, sr.PrefixSID{}, false
	}
	return pfx, sids[0], true
}
