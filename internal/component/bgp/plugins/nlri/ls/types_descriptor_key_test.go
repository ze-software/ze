// Related: types_descriptor.go — the node descriptor sub-TLV encoder
//
// VALIDATES: a node descriptor carrying a zero-valued OSPF Area-ID or BGP-LS
// Identifier encodes that sub-TLV rather than eliding it, so two nodes that
// differ only in those fields do not collapse to one key.
// PREVENTS: the RFC 9552 Section 5.2.1.1 violation "Two different nodes MUST
// NOT be represented by the same key", which the value-only encoder produced
// for every OSPF backbone node.
package ls

import (
	"bytes"
	"testing"
)

// TestNodeDescriptorEncodesLegalZeroKeyFields is the positive case. Area
// 0.0.0.0 is the OSPF backbone and Section 5.2.1.4 RECOMMENDS 0 as the default
// BGP-LS Identifier, so both must reach the wire when they are present.
//
// RFC requirement: RFC9552-5.2.1.1-2 positive -- every key field a node carries
// reaches its key, a legal zero included, which is what lets two nodes that
// differ only in that field be told apart (§5.2.1.1).
func TestNodeDescriptorEncodesLegalZeroKeyFields(t *testing.T) {
	backbone := &NodeDescriptor{
		ASN:                65001,
		OSPFAreaID:         0,
		HasOSPFAreaID:      true,
		BGPLSIdentifier:    0,
		HasBGPLSIdentifier: true,
	}

	encoded := backbone.Bytes()
	for _, want := range []struct {
		name string
		tlv  uint16
	}{
		{"OSPF Area-ID", TLVOSPFAreaID},
		{"BGP-LS Identifier", TLVBGPLSIdentifier},
	} {
		needle := []byte{byte(want.tlv >> 8), byte(want.tlv), 0, 4, 0, 0, 0, 0}
		if !bytes.Contains(encoded, needle) {
			t.Errorf("a present %s of zero was elided: % x", want.name, encoded)
		}
	}

	if got, want := len(encoded), backbone.Len(); got != want {
		t.Fatalf("Bytes wrote %d octets and Len promised %d; the two must agree "+
			"or WriteTo overruns the buffer CheckedWriteTo sized", got, want)
	}
}

// TestNodeDescriptorKeepsBackboneDistinctFromAreaLess is the discrimination
// case, and it is the one that fails against the old encoder. A backbone node
// and a node with no Area-ID at all are DIFFERENT nodes, so their keys must
// differ.
//
// RFC requirement: RFC9552-5.2.1.1-2 negative -- the forbidden outcome is
// asserted absent: two different nodes do not collapse onto one key, so they
// cannot "look like one node" (§5.2.1.1).
func TestNodeDescriptorKeepsBackboneDistinctFromAreaLess(t *testing.T) {
	backbone := &NodeDescriptor{ASN: 65001, OSPFAreaID: 0, HasOSPFAreaID: true}
	areaLess := &NodeDescriptor{ASN: 65001}

	if bytes.Equal(backbone.Bytes(), areaLess.Bytes()) {
		t.Fatal("an OSPF backbone node and a node carrying no Area-ID encode to " +
			"the same key, which RFC 9552 Section 5.2.1.1 forbids")
	}
}

// TestNodeDescriptorStillElidesReservedZeros pins the negative half. AS 0 is
// reserved by RFC 7607 and a BGP Router-ID of 0.0.0.0 identifies nothing, so
// for those fields zero and absent mean the same thing and the sub-TLV stays
// off the wire. Without this the fix would read as "encode every zero".
func TestNodeDescriptorStillElidesReservedZeros(t *testing.T) {
	empty := &NodeDescriptor{}
	if got := empty.Bytes(); len(got) != 0 {
		t.Fatalf("a descriptor with no field set encoded % x, want nothing", got)
	}
	if got := empty.Len(); got != 0 {
		t.Fatalf("Len promised %d octets for an empty descriptor, want 0", got)
	}
}

// TestPrefixDescriptorEmitsTheKeyTLVs covers the second half of the same
// defect. RFC 9552 Section 5.2.1.1 names the Multi-Topology Identifier as part
// of the key, and the descriptor declared MT-ID and OSPF Route Type while
// encoding neither: only TLV 265 reached the wire.
func TestPrefixDescriptorEmitsTheKeyTLVs(t *testing.T) {
	pd := &PrefixDescriptor{
		MultiTopologyID:    0,
		HasMultiTopologyID: true,
		OSPFRouteType:      3,
		IPReachabilityInfo: []byte{24, 10, 0, 0},
	}

	encoded := pd.Bytes()
	for _, want := range []struct {
		name string
		tlv  uint16
	}{
		{"Multi-Topology Identifier", TLVMultiTopologyID},
		{"OSPF Route Type", TLVOSPFRouteType},
		{"IP Reachability", TLVIPReachabilityInfo},
	} {
		if !bytes.Contains(encoded, []byte{byte(want.tlv >> 8), byte(want.tlv)}) {
			t.Errorf("%s (TLV %d) is absent: % x", want.name, want.tlv, encoded)
		}
	}

	if got, want := len(encoded), pd.Len(); got != want {
		t.Fatalf("Bytes wrote %d octets and Len promised %d", got, want)
	}

	buf := make([]byte, pd.Len())
	if n := pd.WriteTo(buf, 0); n != len(encoded) || !bytes.Equal(buf, encoded) {
		t.Fatalf("WriteTo produced % x (%d octets), Bytes produced % x", buf, n, encoded)
	}
}

// TestDescriptorTLVsAreAscending pins RFC 9552 Section 5.1: "all TLVs within
// the NLRI MUST be ordered in ascending order by TLV Type." A descriptor that
// carries the right TLVs in the wrong order is still malformed, and the two
// new TLVs are the ones most likely to be appended carelessly.
func TestDescriptorTLVsAreAscending(t *testing.T) {
	prefix := (&PrefixDescriptor{
		HasMultiTopologyID: true,
		OSPFRouteType:      3,
		IPReachabilityInfo: []byte{24, 10, 0, 0},
	}).Bytes()
	link := (&LinkDescriptor{
		LinkLocalID:        1,
		LocalInterfaceAddr: []byte{10, 0, 0, 1},
		NeighborAddr:       []byte{10, 0, 0, 2},
		HasMultiTopologyID: true,
	}).Bytes()

	for name, encoded := range map[string][]byte{"prefix": prefix, "link": link} {
		previous := -1
		for off := 0; off+4 <= len(encoded); {
			kind := int(encoded[off])<<8 | int(encoded[off+1])
			length := int(encoded[off+2])<<8 | int(encoded[off+3])
			if kind < previous {
				t.Errorf("%s descriptor: TLV %d follows %d, which Section 5.1 forbids: % x",
					name, kind, previous, encoded)
			}
			previous = kind
			off += 4 + length
		}
	}
}

// TestNodeDescriptorWriteToMatchesBytes keeps the three encoders in step. They
// are separate implementations of one format, so a fix applied to one and not
// the others is a length mismatch rather than a missing field.
//
// RFC requirement: RFC9552-5.2.1.1-1 negative -- one node does not acquire a
// second key from the choice of encoder, so it cannot "look like two nodes"
// (§5.2.1.1).
func TestNodeDescriptorWriteToMatchesBytes(t *testing.T) {
	nd := &NodeDescriptor{
		ASN:                65001,
		OSPFAreaID:         0,
		HasOSPFAreaID:      true,
		BGPLSIdentifier:    0,
		HasBGPLSIdentifier: true,
		IGPRouterID:        []byte{1, 2, 3, 4},
	}

	want := nd.Bytes()
	buf := make([]byte, nd.Len())
	written := nd.WriteTo(buf, 0)

	if written != len(want) {
		t.Fatalf("WriteTo wrote %d octets, Bytes produced %d", written, len(want))
	}
	if !bytes.Equal(buf, want) {
		t.Fatalf("WriteTo produced % x, Bytes produced % x", buf, want)
	}
}

// TestSameNodeHasOneKeyWhateverTheStorageOrder covers requirement (A) directly.
// A node's repeated sub-TLVs are held in a slice, and slice order is an
// artifact of how the node was built rather than a property of the node, so
// encoding it must not depend on that order.
//
// RFC requirement: RFC9552-5.2.1.1-1 positive -- one node encodes to exactly one
// key however its repeated sub-TLVs happen to be stored (§5.2.1.1).
func TestSameNodeHasOneKeyWhateverTheStorageOrder(t *testing.T) {
	first := bytes.Repeat([]byte{0xfd}, 16)
	second := bytes.Repeat([]byte{0xfe}, 16)

	oneWay := &NodeDescriptor{
		ASN:           65001,
		HasOSPFAreaID: true,
		SRv6SIDs:      [][]byte{first, second},
	}
	theOther := &NodeDescriptor{
		ASN:           65001,
		HasOSPFAreaID: true,
		SRv6SIDs:      [][]byte{second, first},
	}

	if !bytes.Equal(oneWay.Bytes(), theOther.Bytes()) {
		t.Fatalf("the same node encoded to two keys, % x and % x. RFC 9552 Section "+
			"5.2.1.1 (A): \"The same node MUST NOT be represented by two keys "+
			"(otherwise, one node will look like two nodes)\"",
			oneWay.Bytes(), theOther.Bytes())
	}
}
