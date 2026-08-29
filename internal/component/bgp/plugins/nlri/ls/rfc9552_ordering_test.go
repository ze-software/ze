// Related: types_descriptor.go — addressTLVs, srv6SIDsOrdered
//
// VALIDATES: the descriptor encoders emit sub-TLVs in the canonical order RFC
// 9552 Section 5.1 defines, ascending by type and, among repeated types,
// ascending by Length then by Value.
// PREVENTS: an NLRI a BGP-LS Propagator must treat as malformed, and the
// Section 5.2.1.1 breach where one node acquires two keys because its repeated
// sub-TLVs were stored in a different order.
package ls

import (
	"bytes"
	"testing"
)

// tlvTypes walks an encoded descriptor and returns the TLV types in the order
// they appear. It reads only the header of each TLV, so an unknown type is
// walked like any other.
func tlvTypes(t *testing.T, encoded []byte) []uint16 {
	t.Helper()

	var types []uint16
	for off := 0; off < len(encoded); {
		if off+4 > len(encoded) {
			t.Fatalf("truncated TLV header at offset %d in % x", off, encoded)
		}
		kind := uint16(encoded[off])<<8 | uint16(encoded[off+1])
		length := int(encoded[off+2])<<8 | int(encoded[off+3])
		types = append(types, kind)
		off += 4 + length
	}

	return types
}

// TestLinkDescriptorOrdersMixedFamilyAddressesAscending is the case where field
// order and type order disagree, and the only one the old encoder got wrong.
//
// RFC requirement: RFC9552-5.1-1 positive -- an IPv6 interface address (TLV 261)
// beside an IPv4 neighbor address (TLV 260) emits 260 first, so the NLRI is in
// ascending type order however the two addresses are typed (§5.1)
// RFC requirement: RFC7752-3.1-2 positive -- the predecessor states the same
// ascending-Type rule, and RFC9552-5.1-1 restates it (Section 3.1).
func TestLinkDescriptorOrdersMixedFamilyAddressesAscending(t *testing.T) {
	ld := &LinkDescriptor{
		LocalInterfaceAddr: bytes.Repeat([]byte{0x20}, 16),
		NeighborAddr:       []byte{10, 0, 0, 2},
	}

	encoded := ld.Bytes()
	got := tlvTypes(t, encoded)
	want := []uint16{TLVIPv4NeighborAddr, TLVIPv6InterfaceAddr}

	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("TLV order %v, want %v: Section 5.1 requires ascending type order, "+
			"and a Propagator must treat a descending NLRI as malformed", got, want)
	}

	if len(encoded) != ld.Len() {
		t.Fatalf("Bytes wrote %d octets and Len promised %d", len(encoded), ld.Len())
	}

	buf := make([]byte, ld.Len())
	if n := ld.WriteTo(buf, 0); n != len(encoded) || !bytes.Equal(buf, encoded) {
		t.Fatalf("WriteTo produced % x, Bytes produced % x", buf, encoded)
	}
}

// TestNoDescriptorEmitsADescendingTLVSequence sweeps the input space rather than
// one fixture, because an encoder that happens to be ascending for the case
// above can still descend for another combination.
//
// RFC requirement: RFC9552-5.1-1 negative -- no combination of address families
// or descriptor fields produces a TLV whose type is lower than the one before
// it (§5.1)
// RFC requirement: RFC7752-3.1-2 negative -- the same sweep over the same
// encoders (Section 3.1).
func TestNoDescriptorEmitsADescendingTLVSequence(t *testing.T) {
	v4 := []byte{10, 0, 0, 1}
	v6 := bytes.Repeat([]byte{0x20}, 16)

	encodings := map[string][]byte{}
	for ifaceName, iface := range map[string][]byte{"none": nil, "v4": v4, "v6": v6} {
		for nbrName, nbr := range map[string][]byte{"none": nil, "v4": v4, "v6": v6} {
			ld := &LinkDescriptor{
				LinkLocalID:        1,
				LocalInterfaceAddr: iface,
				NeighborAddr:       nbr,
				HasMultiTopologyID: true,
			}
			encodings["link iface="+ifaceName+" neighbor="+nbrName] = ld.Bytes()
		}
	}
	encodings["node"] = (&NodeDescriptor{
		ASN:                65001,
		HasBGPLSIdentifier: true,
		HasOSPFAreaID:      true,
		IGPRouterID:        []byte{1, 2, 3, 4},
		BGPRouterID:        0x0a000001,
		ConfedMember:       65002,
		SRv6SIDs:           [][]byte{bytes.Repeat([]byte{0xfd}, 16)},
	}).Bytes()
	encodings["prefix"] = (&PrefixDescriptor{
		HasMultiTopologyID: true,
		OSPFRouteType:      3,
		IPReachabilityInfo: []byte{24, 10, 0, 0},
	}).Bytes()

	for name, encoded := range encodings {
		types := tlvTypes(t, encoded)
		for i := 1; i < len(types); i++ {
			if types[i] < types[i-1] {
				t.Errorf("%s: TLV %d follows %d, a descending pair Section 5.1 forbids: % x",
					name, types[i], types[i-1], encoded)
			}
		}
	}
}

// TestNodeDescriptorOrdersRepeatedSRv6SIDs covers the one sub-TLV a descriptor
// can repeat. Section 5.1 orders repeated types by Length and then by Value, so
// slice order is not a free choice.
//
// RFC requirement: RFC9552-5.1-2 positive -- repeated TLV 518 sub-TLVs are
// emitted ascending by Length and then ascending by Value, whatever order they
// were stored in (§5.1)
// RFC requirement: RFC7752-3.1-3 positive -- the predecessor orders same-type
// TLVs by value alone, which ascending Length then Value satisfies (Section 3.1).
func TestNodeDescriptorOrdersRepeatedSRv6SIDs(t *testing.T) {
	high := bytes.Repeat([]byte{0xfe}, 16)
	low := bytes.Repeat([]byte{0xfd}, 16)
	short := []byte{0xff, 0xff}

	nd := &NodeDescriptor{SRv6SIDs: [][]byte{high, short, low}}

	encoded := nd.Bytes()
	kind := TLVSRv6SID
	off := 0
	for _, want := range [][]byte{short, low, high} {
		header := []byte{
			byte(kind >> 8), byte(kind & 0xff),
			byte(len(want) >> 8), byte(len(want) & 0xff),
		}
		end := off + 4 + len(want)
		if end > len(encoded) {
			t.Fatalf("encoding ends at %d, short of the sub-TLV for % x: % x", len(encoded), want, encoded)
		}
		if !bytes.Equal(encoded[off:end], append(header, want...)) {
			t.Fatalf("at offset %d the encoder emitted % x, want the sub-TLV for % x; "+
				"full encoding % x", off, encoded[off:end], want, encoded)
		}
		off = end
	}

	if off != len(encoded) {
		t.Fatalf("%d trailing octets after the three SID sub-TLVs: % x", len(encoded)-off, encoded)
	}

	// The caller's slice is an input, not scratch space. Reordering it in place
	// would give the same node a different key on a later encode.
	if !bytes.Equal(nd.SRv6SIDs[0], high) {
		t.Error("the encoder reordered the caller's SRv6SIDs slice in place")
	}
}

// TestSRv6SIDOrderIsLengthBeforeValue is the discrimination case for the
// comparison itself. A plain lexicographic sort passes the test above and fails
// this one, because Section 5.1 makes Length the first key.
//
// RFC requirement: RFC9552-5.1-2 negative -- a shorter SID whose value sorts
// after a longer one is still emitted first, so value order never overrides
// Length order (§5.1)
// RFC requirement: RFC7752-3.1-3 negative -- value order alone never decides
// the emitted order (Section 3.1).
func TestSRv6SIDOrderIsLengthBeforeValue(t *testing.T) {
	longer := make([]byte, 16)
	shorter := []byte{0xff, 0xff}

	nd := &NodeDescriptor{SRv6SIDs: [][]byte{longer, shorter}}
	encoded := nd.Bytes()

	const firstValueStart = 4
	if !bytes.Equal(encoded[firstValueStart:firstValueStart+len(shorter)], shorter) {
		t.Fatalf("the longer SID was emitted first: % x. Section 5.1 orders repeated "+
			"TLVs \"in ascending order based on the Length field followed by ascending "+
			"order based on the Value field\", so Length decides before Value does", encoded)
	}
}
