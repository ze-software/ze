package reactor

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// RFC 7752 Section 3.1: "Unknown and unsupported types MUST be preserved and
// propagated within both the NLRI and the BGP-LS Attribute."
// RFC requirement: RFC7752-3.1-1 positive -- actual forwarding preserves unknown Node Descriptor sub-TLVs and unknown BGP-LS attribute TLVs toward internal and external peers.
// RFC requirement: RFC7752-3.1-1 negative -- an attribute consisting entirely of unsupported TLVs is still propagated; absence of a recognized TLV does not authorize dropping it.
func TestRFC7752UnknownTLVsReachForwardDestinations(t *testing.T) {
	for _, allUnknown := range []bool{false, true} {
		payload := rfc9552UnknownTypesPayload()
		wantAttr := bytes.Clone(rfc9552UnknownAttrValue)
		if allUnknown {
			binary.BigEndian.PutUint16(wantAttr[:2], 65001)
			binary.BigEndian.PutUint16(wantAttr[15:17], 65002)
			copy(payload[len(payload)-len(wantAttr):], wantAttr)
		}
		for _, peerAS := range []uint32{65000, 65002} {
			peer := rfc9552Destination(t, "192.0.2.2", peerAS)
			blocks := rfc9552Forward(t, payload, peer)
			if len(blocks) != 1 {
				t.Fatalf("allUnknown=%v peerAS=%d: forwarded %d updates, want one", allUnknown, peerAS, len(blocks))
			}
			nlriBytes, attrs, _ := rfc9552PropagatedHalves(t, blocks[0])
			if !bytes.Equal(nlriBytes, rfc9552UnknownSubTLVNodeNLRI) || !bytes.Equal(attrs, wantAttr) {
				t.Fatalf("allUnknown=%v peerAS=%d: unknown NLRI or attribute TLVs changed", allUnknown, peerAS)
			}
		}
	}
}
