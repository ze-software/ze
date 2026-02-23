// Design: docs/architecture/wire/nlri-bgpls.md — SRv6 SID NLRI (RFC 9514)
// RFC: rfc/short/rfc9514.md — BGP-LS SRv6 SID NLRI
// RFC: rfc/short/rfc7752.md — base BGP-LS framework
// Related: types.go — core types, bgplsBase, TLV constants, parser functions
// Related: types_descriptor.go — NodeDescriptor embedded in BGPLSSRv6SID
// Related: types_nlri.go — RFC 7752 NLRI types (Node, Link, Prefix)
package bgp_nlri_ls

import (
	"encoding/binary"
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/wire"
)

// ============================================================================
// SRv6 SID NLRI (RFC 9514)
// Note: This is NOT part of RFC 7752 but extends BGP-LS for Segment Routing v6.
// ============================================================================

// TLV types for SRv6.
// Note: These are defined in RFC 9514, not RFC 7752.
const (
	TLVSRv6SID uint16 = 518 // SRv6 SID (RFC 9514, not RFC 7752)
)

// SRv6SIDDescriptor contains SRv6 SID identification information.
// RFC 9514 defines the SRv6 SID NLRI extension to BGP-LS.
type SRv6SIDDescriptor struct {
	MultiTopologyID uint16 // Multi-Topology ID (TLV 263)
	SRv6SID         []byte // 16 bytes IPv6 address (TLV 518)
}

// Bytes encodes the SRv6 SID descriptor as TLVs.
// RFC 9514 defines the SRv6 SID TLV encoding.
func (sd *SRv6SIDDescriptor) Bytes() []byte {
	var data []byte

	// RFC 9514 - SRv6 SID TLV (518)
	if len(sd.SRv6SID) > 0 {
		data = append(data, tlv(TLVSRv6SID, sd.SRv6SID)...)
	}

	return data
}

// Len returns the TLV-encoded length in bytes.
func (sd *SRv6SIDDescriptor) Len() int {
	if len(sd.SRv6SID) > 0 {
		return 4 + len(sd.SRv6SID)
	}
	return 0
}

// WriteTo writes the SRv6 SID descriptor TLVs directly to buf at offset.
// Returns bytes written.
func (sd *SRv6SIDDescriptor) WriteTo(buf []byte, off int) int {
	if len(sd.SRv6SID) > 0 {
		return writeTLVBytes(buf, off, TLVSRv6SID, sd.SRv6SID)
	}
	return 0
}

// CheckedWriteTo validates capacity before writing.
func (sd *SRv6SIDDescriptor) CheckedWriteTo(buf []byte, off int) (int, error) {
	needed := sd.Len()
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return sd.WriteTo(buf, off), nil
}

// BGPLSSRv6SID represents an SRv6 SID NLRI.
// RFC 9514 - SRv6 SID NLRI (Type 6).
type BGPLSSRv6SID struct {
	bgplsBase
	LocalNode NodeDescriptor    // RFC 7752 Section 3.2.1.2 - Local Node Descriptors
	SRv6SID   SRv6SIDDescriptor // RFC 9514 - SRv6 SID Descriptor
}

// NewBGPLSSRv6SID creates a new SRv6 SID NLRI.
// RFC 9514 - SRv6 SID NLRI (Type 6).
func NewBGPLSSRv6SID(proto BGPLSProtocolID, id uint64, node NodeDescriptor, sid SRv6SIDDescriptor) *BGPLSSRv6SID {
	return &BGPLSSRv6SID{
		bgplsBase: bgplsBase{
			nlriType:   BGPLSSRv6SIDNLRI,
			protocolID: proto,
			identifier: id,
		},
		LocalNode: node,
		SRv6SID:   sid,
	}
}

// Bytes returns the wire-format encoding.
// Uses RFC 7752 NLRI header format with RFC 9514 SRv6 SID descriptor.
//
//nolint:dupl // Similar structure to BGPLSPrefix.Bytes() is intentional
func (s *BGPLSSRv6SID) Bytes() []byte {
	if s.cached != nil {
		return s.cached
	}

	// RFC 7752 Section 3.2.1.2 - Local Node Descriptors (TLV 256)
	localNodeTLV := tlv(TLVLocalNodeDesc, s.LocalNode.Bytes())
	// RFC 9514 - SRv6 SID descriptor TLVs
	sidTLV := s.SRv6SID.Bytes()

	bodyLen := 9 + len(localNodeTLV) + len(sidTLV)
	body := make([]byte, bodyLen)
	body[0] = byte(s.protocolID)                        // Protocol-ID (1 byte)
	binary.BigEndian.PutUint64(body[1:9], s.identifier) // Identifier (8 bytes)
	offset := 9
	copy(body[offset:], localNodeTLV)
	offset += len(localNodeTLV)
	copy(body[offset:], sidTLV)

	// RFC 7752 Section 3.2 - NLRI header format
	s.cached = make([]byte, 4+len(body))
	binary.BigEndian.PutUint16(s.cached[0:2], uint16(s.nlriType)) // NLRI Type (2 bytes)
	binary.BigEndian.PutUint16(s.cached[2:4], uint16(len(body)))  //nolint:gosec // Total NLRI Length (2 bytes)
	copy(s.cached[4:], body)

	return s.cached
}

// Len returns the length in bytes.
// Calculates arithmetically without allocating, matching WriteTo logic:
// NLRI header (4) + body (9) + local node TLV (4+len) + SRv6 SID desc.
func (s *BGPLSSRv6SID) Len() int {
	if s.cached != nil {
		return len(s.cached)
	}
	return 4 + 9 + (4 + s.LocalNode.Len()) + s.SRv6SID.Len()
}

// String returns command-style format for API round-trip compatibility.
// Format: srv6-sid protocol set <proto> asn set <n>.
func (s *BGPLSSRv6SID) String() string {
	return fmt.Sprintf("srv6-sid protocol set %s asn set %d", s.protocolID, s.LocalNode.ASN)
}

// WriteTo writes the SRv6 SID NLRI directly to buf at offset.
//
//nolint:dupl // Similar structure to BGPLSPrefix.WriteTo is intentional
func (s *BGPLSSRv6SID) WriteTo(buf []byte, off int) int {
	// Fallback: use cached bytes if present
	if s.cached != nil {
		return copy(buf[off:], s.cached)
	}

	pos := off

	// Calculate body length
	localNodeLen := s.LocalNode.Len()
	sidDescLen := s.SRv6SID.Len()
	bodyLen := 9 + (4 + localNodeLen) + sidDescLen

	// Write NLRI header
	binary.BigEndian.PutUint16(buf[pos:], uint16(s.nlriType))
	binary.BigEndian.PutUint16(buf[pos+2:], uint16(bodyLen)) //nolint:gosec // Length validated
	pos += 4

	// Write body: Protocol-ID + Identifier
	buf[pos] = byte(s.protocolID)
	binary.BigEndian.PutUint64(buf[pos+1:], s.identifier)
	pos += 9

	// Write Local Node TLV (256)
	binary.BigEndian.PutUint16(buf[pos:], TLVLocalNodeDesc)
	binary.BigEndian.PutUint16(buf[pos+2:], uint16(localNodeLen)) //nolint:gosec // Length validated
	pos += 4
	pos += s.LocalNode.WriteTo(buf, pos)

	// Write SRv6 SID Descriptor TLVs
	pos += s.SRv6SID.WriteTo(buf, pos)

	return pos - off
}
