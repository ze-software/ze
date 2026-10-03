// Related: types.go — parseNodeDescriptorTLVsAt and parseSRv6SIDDescriptorTLVs,
// the two walks over an SRv6 SID NLRI's descriptors
//
// VALIDATES: the SRv6 SID Information TLV that follows the Local Node
// Descriptors container at NLRI level is read into the NLRI's SRv6 SID
// Descriptor, never into the Node Descriptor, and a container nested past the
// bound costs only its own bytes.
// PREVENTS: the walk replacing its iteration buffer with the container's value,
// which discarded every octet after the container, and the RFC 9514 Section 6
// TLV being filed as a Node Descriptor sub-TLV, which RFC 9552 Section 5.2.1.4
// then sees repeated.
package ls

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// tlv2 builds one TLV: type, 2-octet length, value.
func tlv2(kind uint16, value ...byte) []byte {
	out := []byte{byte(kind >> 8), byte(kind & 0xff), byte(len(value) >> 8), byte(len(value) & 0xff)}
	return append(out, value...)
}

// srv6SIDNLRI wraps a descriptor section in an RFC 9514 SRv6 SID NLRI header:
// type 6, length, Protocol-ID 2 (IS-IS L2), Identifier 0.
func srv6SIDNLRI(section []byte) []byte {
	out := make([]byte, 4+9+len(section))
	binary.BigEndian.PutUint16(out[0:2], uint16(BGPLSSRv6SIDNLRI))
	binary.BigEndian.PutUint16(out[2:4], uint16(9+len(section))) //nolint:gosec // test fixture, small
	out[4] = 2
	copy(out[13:], section)
	return out
}

// parseSRv6SIDNLRI parses an SRv6 SID NLRI through the production entry point.
func parseSRv6SIDNLRI(t *testing.T, section []byte) *BGPLSSRv6SID {
	t.Helper()
	parsed, err := parseBGPLS(srv6SIDNLRI(section))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	srv6, ok := parsed.(*BGPLSSRv6SID)
	if !ok {
		t.Fatalf("parsed %T, want *BGPLSSRv6SID", parsed)
	}
	return srv6
}

// TestATLVAfterTheContainerIsStillRead is the case the old walk lost. The SRv6
// SID Information TLV sits after the Local Node Descriptors container, at the
// same level, so a walk that descends and does not come back never sees it.
func TestATLVAfterTheContainerIsStillRead(t *testing.T) {
	sid := bytes.Repeat([]byte{0xfd}, 16)
	container := tlv2(TLVLocalNodeDesc, tlv2(TLVAutonomousSystem, 0x00, 0x00, 0xfd, 0xe9)...)
	section := make([]byte, 0, len(container)+4+len(sid))
	section = append(section, container...)
	section = append(section, tlv2(TLVSRv6SID, sid...)...)

	srv6 := parseSRv6SIDNLRI(t, section)

	if srv6.LocalNode.ASN != 65001 {
		t.Errorf("the container's own sub-TLV was lost: ASN = %d, want 65001", srv6.LocalNode.ASN)
	}
	if !bytes.Equal(srv6.SRv6SID.SRv6SID, sid) {
		t.Fatalf("the TLV after the container was discarded: SRv6SID = % x. The walk "+
			"must resume where the container ended, not stop inside it", srv6.SRv6SID.SRv6SID)
	}
	if got := srv6.LocalNode.Len(); got != 8 {
		t.Errorf("the Node Descriptor re-encodes to %d octets, want 8 (TLV 512 only): "+
			"TLV 518 is an SRv6 SID Descriptor, not a Node Descriptor sub-TLV", got)
	}
}

// TestASecondSRv6SIDInformationTLVKeepsTheFirst covers the repeat RFC 9514
// Section 6 forbids a sender to emit ("MUST contain a single SRv6 SID
// Information TLV"). RFC 9552 Section 8.2.2 forbids refusing the NLRI for it, so
// the first is kept and the NLRI still parses.
func TestASecondSRv6SIDInformationTLVKeepsTheFirst(t *testing.T) {
	first := bytes.Repeat([]byte{0xfd}, 16)
	second := bytes.Repeat([]byte{0xfe}, 16)
	section := tlv2(TLVLocalNodeDesc, tlv2(TLVAutonomousSystem, 0x00, 0x00, 0xfd, 0xe9)...)
	section = append(section, tlv2(TLVSRv6SID, first...)...)
	section = append(section, tlv2(TLVSRv6SID, second...)...)

	srv6 := parseSRv6SIDNLRI(t, section)

	if !bytes.Equal(srv6.SRv6SID.SRv6SID, first) {
		t.Fatalf("SRv6SID = % x, want the first TLV's % x", srv6.SRv6SID.SRv6SID, first)
	}
}

// TestTheContainersOwnSubTLVsAreStillRead is the half the old walk got right,
// kept so the fix cannot be "stop descending".
func TestTheContainersOwnSubTLVsAreStillRead(t *testing.T) {
	section := tlv2(TLVLocalNodeDesc,
		append(
			tlv2(TLVAutonomousSystem, 0x00, 0x00, 0xfd, 0xe9),
			tlv2(TLVOSPFAreaID, 0x00, 0x00, 0x00, 0x00)...,
		)...,
	)

	var nd NodeDescriptor
	if err := parseNodeDescriptorTLVs(section, &nd); err != nil {
		t.Fatalf("parse: %v", err)
	}

	if nd.ASN != 65001 {
		t.Errorf("ASN = %d, want 65001", nd.ASN)
	}
	if !nd.HasOSPFAreaID {
		t.Error("a present OSPF Area-ID of zero inside the container was not recorded")
	}
}

// TestANestedContainerCostsOnlyItsOwnBytes bounds the descent. The walk follows
// lengths a peer chose, so a container whose value is another container must not
// recurse without limit, and must not abandon the rest of the section either.
func TestANestedContainerCostsOnlyItsOwnBytes(t *testing.T) {
	nested := tlv2(TLVLocalNodeDesc,
		tlv2(TLVLocalNodeDesc, tlv2(TLVAutonomousSystem, 0x00, 0x00, 0xfd, 0xe9)...)...,
	)
	sid := bytes.Repeat([]byte{0xfe}, 16)
	section := make([]byte, 0, len(nested)+4+len(sid))
	section = append(section, nested...)
	section = append(section, tlv2(TLVSRv6SID, sid...)...)

	srv6 := parseSRv6SIDNLRI(t, section)

	if !bytes.Equal(srv6.SRv6SID.SRv6SID, sid) {
		t.Fatalf("the TLV after a nested container was discarded: % x", srv6.SRv6SID.SRv6SID)
	}
	if srv6.LocalNode.ASN != 0 {
		t.Errorf("the walk descended past the bound: ASN = %d, want 0", srv6.LocalNode.ASN)
	}
}

// TestATruncatedTLVIsStillRefused keeps the length check. Without it the fix
// could be read as "trust every length".
func TestATruncatedTLVIsStillRefused(t *testing.T) {
	section := []byte{0x01, 0x00, 0x00, 0x40, 0xaa}

	var nd NodeDescriptor
	if err := parseNodeDescriptorTLVs(section, &nd); err == nil {
		t.Fatal("a TLV whose declared length runs past the section was accepted")
	}
}
