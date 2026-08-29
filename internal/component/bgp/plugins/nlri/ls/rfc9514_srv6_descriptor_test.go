// Related: types.go — parseNodeDescriptorTLVsAt, the descriptor walk
//
// VALIDATES: a TLV that follows the Local Node Descriptors container at NLRI
// level is read, and a container nested past the bound costs only its own bytes.
// PREVENTS: the walk replacing its iteration buffer with the container's value,
// which discarded every octet after the container — so an RFC 9514 SRv6 SID
// Information TLV was never read for any wire-decoded NLRI.
package ls

import (
	"bytes"
	"testing"
)

// tlv2 builds one TLV: type, 2-octet length, value.
func tlv2(kind uint16, value ...byte) []byte {
	out := []byte{byte(kind >> 8), byte(kind & 0xff), byte(len(value) >> 8), byte(len(value) & 0xff)}
	return append(out, value...)
}

// TestATLVAfterTheContainerIsStillRead is the case the old walk lost. The SRv6
// SID sub-TLV sits after the Local Node Descriptors container, at the same
// level, so a walk that descends and does not come back never sees it.
func TestATLVAfterTheContainerIsStillRead(t *testing.T) {
	sid := bytes.Repeat([]byte{0xfd}, 16)
	container := tlv2(TLVLocalNodeDesc, tlv2(TLVAutonomousSystem, 0x00, 0x00, 0xfd, 0xe9)...)
	section := make([]byte, 0, len(container)+4+len(sid))
	section = append(section, container...)
	section = append(section, tlv2(TLVSRv6SID, sid...)...)

	var nd NodeDescriptor
	if err := parseNodeDescriptorTLVs(section, &nd); err != nil {
		t.Fatalf("parse: %v", err)
	}

	if nd.ASN != 65001 {
		t.Errorf("the container's own sub-TLV was lost: ASN = %d, want 65001", nd.ASN)
	}
	if len(nd.SRv6SIDs) != 1 || !bytes.Equal(nd.SRv6SIDs[0], sid) {
		t.Fatalf("the TLV after the container was discarded: SRv6SIDs = %v. The walk "+
			"must resume where the container ended, not stop inside it", nd.SRv6SIDs)
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

	var nd NodeDescriptor
	if err := parseNodeDescriptorTLVs(section, &nd); err != nil {
		t.Fatalf("a nested container was refused: %v. RFC 9552 Section 8.2.2 forbids "+
			"judging an NLRI malformed on which TLVs it includes", err)
	}

	if len(nd.SRv6SIDs) != 1 || !bytes.Equal(nd.SRv6SIDs[0], sid) {
		t.Fatalf("the TLV after a nested container was discarded: %v", nd.SRv6SIDs)
	}
	if nd.ASN != 0 {
		t.Errorf("the walk descended past the bound: ASN = %d, want 0", nd.ASN)
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
