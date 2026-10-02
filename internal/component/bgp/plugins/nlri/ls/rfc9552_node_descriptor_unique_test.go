// Design: docs/architecture/wire/nlri-bgpls.md -- BGP-LS descriptor TLV encoding
// RFC: rfc/short/rfc9552.md -- RFC9552-5.2.1.4-1, the sender half
// Related: types_descriptor.go -- NodeDescriptor.WriteTo, the producer under test
//
// VALIDATES: the Node Descriptor encoder writes each sub-TLV type at most once,
// with every field set and when the NLRI around it carries an SRv6 SID.
// PREVENTS: a repeated sub-TLV type in one Node Descriptor, which the encoder
// once produced by writing one TLV 518 per stored SRv6 SID.
package ls

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// countDescriptorTLVTypes walks one TLV level and counts each type.
func countDescriptorTLVTypes(t *testing.T, encoded []byte) map[uint16]int {
	t.Helper()
	seen := make(map[uint16]int)
	for off := 0; off < len(encoded); {
		if off+4 > len(encoded) {
			t.Fatalf("truncated TLV header at %d: % x", off, encoded)
		}
		kind := binary.BigEndian.Uint16(encoded[off : off+2])
		length := int(binary.BigEndian.Uint16(encoded[off+2 : off+4]))
		seen[kind]++
		off += 4 + length
	}
	return seen
}

// TestNodeDescriptorNeverRepeatsASubTLVType sets every field the Node
// Descriptor has, so every sub-TLV type it can write is present, and checks
// each appears exactly once.
//
// RFC requirement: RFC9552-5.2.1.4-1 positive -- a Node Descriptor Ze encodes
// with every sub-TLV field set carries each of TLVs 512 to 517 exactly once.
func TestNodeDescriptorNeverRepeatsASubTLVType(t *testing.T) {
	nd := &NodeDescriptor{
		ASN:                65001,
		BGPLSIdentifier:    7,
		HasBGPLSIdentifier: true,
		OSPFAreaID:         1,
		HasOSPFAreaID:      true,
		IGPRouterID:        []byte{10, 0, 0, 1},
		BGPRouterID:        0x0a000001,
		ConfedMember:       65002,
	}
	buf := make([]byte, nd.Len())
	written := nd.WriteTo(buf, 0)

	seen := countDescriptorTLVTypes(t, buf[:written])
	for _, kind := range []uint16{
		TLVAutonomousSystem, TLVBGPLSIdentifier, TLVOSPFAreaID,
		TLVIGPRouterID, TLVBGPRouterID, TLVConfedMember,
	} {
		if seen[kind] != 1 {
			t.Errorf("sub-TLV type %d written %d times, want exactly 1: % x. "+
				"RFC 9552 Section 5.2.1.4: \"At most, there MUST be one instance of each "+
				"sub-TLV type present in any Node Descriptor.\"", kind, seen[kind], buf[:written])
		}
	}
	if len(seen) != 6 {
		t.Errorf("%d sub-TLV types written, want 6: % x", len(seen), buf[:written])
	}
}

// TestSRv6SIDNLRIKeepsItsSIDOutOfTheNodeDescriptor pushes the input toward
// the violation: an SRv6 SID NLRI carries an SRv6 SID beside its node, the
// value Ze once stored inside the Node Descriptor and wrote there once per SID.
// The encoded Local Node Descriptors container must still hold no TLV 518, and
// the SID is written once, at NLRI level, as RFC 9514 Section 6 places it.
//
// RFC requirement: RFC9552-5.2.1.4-1 negative -- an SRv6 SID NLRI's SRv6 SID
// Information TLV is never written into the Node Descriptor, whose sub-TLV
// types each appear at most once.
func TestSRv6SIDNLRIKeepsItsSIDOutOfTheNodeDescriptor(t *testing.T) {
	sid := bytes.Repeat([]byte{0xfd}, 16)
	nlri := NewBGPLSSRv6SID(ProtoISISL2, 0,
		NodeDescriptor{ASN: 65001, IGPRouterID: []byte{10, 0, 0, 1}},
		SRv6SIDDescriptor{SRv6SID: sid})

	encoded := nlri.Bytes()
	descriptors := encoded[4+9:]
	if got := binary.BigEndian.Uint16(descriptors[0:2]); got != TLVLocalNodeDesc {
		t.Fatalf("first descriptor TLV is %d, want the Local Node Descriptors %d: % x",
			got, TLVLocalNodeDesc, encoded)
	}
	containerLen := int(binary.BigEndian.Uint16(descriptors[2:4]))
	container := descriptors[4 : 4+containerLen]

	inside := countDescriptorTLVTypes(t, container)
	for kind, count := range inside {
		if count > 1 {
			t.Errorf("sub-TLV type %d written %d times in one Node Descriptor: % x",
				kind, count, container)
		}
	}
	if inside[TLVSRv6SID] != 0 {
		t.Errorf("TLV 518 written inside the Node Descriptor: % x", container)
	}

	outside := countDescriptorTLVTypes(t, descriptors[4+containerLen:])
	if outside[TLVSRv6SID] != 1 {
		t.Errorf("TLV 518 written %d times at NLRI level, want 1: % x", outside[TLVSRv6SID], encoded)
	}
}
