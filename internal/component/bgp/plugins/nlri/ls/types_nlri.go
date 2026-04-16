// Design: docs/architecture/wire/nlri-bgpls.md — BGP-LS concrete NLRI types
// RFC: rfc/short/rfc7752.md — BGP-LS NLRI types (Node, Link, Prefix)
// Overview: types.go — core types, bgplsBase, TLV constants, parser functions
// Related: types_descriptor.go — descriptor structs embedded in these NLRI types
// Related: types_srv6.go — SRv6 SID NLRI type (RFC 9514)
package ls

import (
	"encoding/binary"
	"fmt"
)

// BGPLSNode represents a Node NLRI.
// RFC 7752 Section 3.2.1 defines the Node NLRI format (NLRI Type 1).
type BGPLSNode struct {
	bgplsBase
	LocalNode NodeDescriptor // RFC 7752 Section 3.2.1.2 - Local Node Descriptors
}

// NewBGPLSNode creates a new Node NLRI.
// RFC 7752 Section 3.2.1 - Node NLRI (Type 1).
func NewBGPLSNode(proto BGPLSProtocolID, id uint64, localNode NodeDescriptor) *BGPLSNode {
	return &BGPLSNode{
		bgplsBase: bgplsBase{
			nlriType:   BGPLSNodeNLRI,
			protocolID: proto,
			identifier: id,
		},
		LocalNode: localNode,
	}
}

// Bytes returns the wire-format encoding.
// RFC 7752 Section 3.2 - NLRI encoding format:
//   - Type (2 bytes) + Length (2 bytes) + Protocol-ID (1 byte) + Identifier (8 bytes) + Descriptors
func (n *BGPLSNode) Bytes() []byte {
	buf := make([]byte, n.Len())
	n.WriteTo(buf, 0)
	return buf
}

// Len returns the length in bytes.
// Calculates arithmetically without allocating, matching WriteTo logic:
// NLRI header (4) + Protocol-ID (1) + Identifier (8) + TLV 256 header (4) + local node.
func (n *BGPLSNode) Len() int {
	if n.cached != nil {
		return len(n.cached)
	}
	return 4 + 9 + 4 + n.LocalNode.Len()
}

// String returns command-style format for API round-trip compatibility.
// Format: node protocol <proto> asn <n>.
func (n *BGPLSNode) String() string {
	return fmt.Sprintf("node protocol %s asn %d", n.protocolID, n.LocalNode.ASN)
}

// WriteTo writes the Node NLRI directly to buf at offset.
func (n *BGPLSNode) WriteTo(buf []byte, off int) int {
	// Fallback: use cached bytes if present
	if n.cached != nil {
		return copy(buf[off:], n.cached)
	}

	pos := off

	// Calculate body length: proto (1) + id (8) + local node TLV
	localNodeLen := n.LocalNode.Len()
	localNodeTLVLen := 4 + localNodeLen // TLV 256 header + content
	bodyLen := 9 + localNodeTLVLen

	// Write NLRI header
	binary.BigEndian.PutUint16(buf[pos:], uint16(n.nlriType))
	binary.BigEndian.PutUint16(buf[pos+2:], uint16(bodyLen)) //nolint:gosec // Length validated
	pos += 4

	// Write body: Protocol-ID + Identifier
	buf[pos] = byte(n.protocolID)
	binary.BigEndian.PutUint64(buf[pos+1:], n.identifier)
	pos += 9

	// Write Local Node TLV (256)
	binary.BigEndian.PutUint16(buf[pos:], TLVLocalNodeDesc)
	binary.BigEndian.PutUint16(buf[pos+2:], uint16(localNodeLen)) //nolint:gosec // Length validated
	pos += 4
	pos += n.LocalNode.WriteTo(buf, pos)

	return pos - off
}

// BGPLSLink represents a Link NLRI.
// RFC 7752 Section 3.2.2 defines the Link NLRI format (NLRI Type 2).
type BGPLSLink struct {
	bgplsBase
	LocalNode  NodeDescriptor // RFC 7752 Section 3.2.1.2 - Local Node Descriptors
	RemoteNode NodeDescriptor // RFC 7752 Section 3.2.1.3 - Remote Node Descriptors
	LinkDesc   LinkDescriptor // RFC 7752 Section 3.2.2 - Link Descriptors
}

// NewBGPLSLink creates a new Link NLRI.
// RFC 7752 Section 3.2.2 - Link NLRI (Type 2).
func NewBGPLSLink(proto BGPLSProtocolID, id uint64, local, remote NodeDescriptor, link LinkDescriptor) *BGPLSLink {
	return &BGPLSLink{
		bgplsBase: bgplsBase{
			nlriType:   BGPLSLinkNLRI,
			protocolID: proto,
			identifier: id,
		},
		LocalNode:  local,
		RemoteNode: remote,
		LinkDesc:   link,
	}
}

// Bytes returns the wire-format encoding per RFC 7752 Section 3.2.2.
// Link descriptor TLVs (258-263) appear directly in the NLRI body after
// the Remote Node Descriptors, NOT wrapped in a container TLV.
func (l *BGPLSLink) Bytes() []byte {
	buf := make([]byte, l.Len())
	l.WriteTo(buf, 0)
	return buf
}

// Len returns the length in bytes.
// Calculates arithmetically without allocating, matching WriteTo logic:
// NLRI header (4) + body (9) + local TLV (4+len) + remote TLV (4+len) + link desc.
func (l *BGPLSLink) Len() int {
	if l.cached != nil {
		return len(l.cached)
	}
	return 4 + 9 + (4 + l.LocalNode.Len()) + (4 + l.RemoteNode.Len()) + l.LinkDesc.Len()
}

// String returns command-style format for API round-trip compatibility.
// Format: link protocol <proto> local-asn <n> remote-asn <m>.
func (l *BGPLSLink) String() string {
	return fmt.Sprintf("link protocol %s local-asn %d remote-asn %d", l.protocolID, l.LocalNode.ASN, l.RemoteNode.ASN)
}

// WriteTo writes the Link NLRI directly to buf at offset.
func (l *BGPLSLink) WriteTo(buf []byte, off int) int {
	// Fallback: use cached bytes if present
	if l.cached != nil {
		return copy(buf[off:], l.cached)
	}

	pos := off

	// Calculate body length
	localNodeLen := l.LocalNode.Len()
	remoteNodeLen := l.RemoteNode.Len()
	linkDescLen := l.LinkDesc.Len()
	bodyLen := 9 + (4 + localNodeLen) + (4 + remoteNodeLen) + linkDescLen

	// Write NLRI header
	binary.BigEndian.PutUint16(buf[pos:], uint16(l.nlriType))
	binary.BigEndian.PutUint16(buf[pos+2:], uint16(bodyLen)) //nolint:gosec // Length validated
	pos += 4

	// Write body: Protocol-ID + Identifier
	buf[pos] = byte(l.protocolID)
	binary.BigEndian.PutUint64(buf[pos+1:], l.identifier)
	pos += 9

	// Write Local Node TLV (256)
	binary.BigEndian.PutUint16(buf[pos:], TLVLocalNodeDesc)
	binary.BigEndian.PutUint16(buf[pos+2:], uint16(localNodeLen)) //nolint:gosec // Length validated
	pos += 4
	pos += l.LocalNode.WriteTo(buf, pos)

	// Write Remote Node TLV (257)
	binary.BigEndian.PutUint16(buf[pos:], TLVRemoteNodeDesc)
	binary.BigEndian.PutUint16(buf[pos+2:], uint16(remoteNodeLen)) //nolint:gosec // Length validated
	pos += 4
	pos += l.RemoteNode.WriteTo(buf, pos)

	// Write Link Descriptor TLVs (directly, not wrapped)
	pos += l.LinkDesc.WriteTo(buf, pos)

	return pos - off
}

// BGPLSPrefix represents a Prefix NLRI (v4 or v6).
// RFC 7752 Section 3.2.3 defines the Prefix NLRI format (NLRI Types 3 and 4).
type BGPLSPrefix struct {
	bgplsBase
	LocalNode  NodeDescriptor   // RFC 7752 Section 3.2.1.2 - Local Node Descriptors
	PrefixDesc PrefixDescriptor // RFC 7752 Section 3.2.3 - Prefix Descriptors
}

// NewBGPLSPrefixV4 creates a new IPv4 Prefix NLRI.
// RFC 7752 Section 3.2.3 - IPv4 Topology Prefix NLRI (Type 3).
func NewBGPLSPrefixV4(proto BGPLSProtocolID, id uint64, node NodeDescriptor, prefix PrefixDescriptor) *BGPLSPrefix {
	return &BGPLSPrefix{
		bgplsBase: bgplsBase{
			nlriType:   BGPLSPrefixV4NLRI,
			protocolID: proto,
			identifier: id,
		},
		LocalNode:  node,
		PrefixDesc: prefix,
	}
}

// NewBGPLSPrefixV6 creates a new IPv6 Prefix NLRI.
// RFC 7752 Section 3.2.3 - IPv6 Topology Prefix NLRI (Type 4).
func NewBGPLSPrefixV6(proto BGPLSProtocolID, id uint64, node NodeDescriptor, prefix PrefixDescriptor) *BGPLSPrefix {
	return &BGPLSPrefix{
		bgplsBase: bgplsBase{
			nlriType:   BGPLSPrefixV6NLRI,
			protocolID: proto,
			identifier: id,
		},
		LocalNode:  node,
		PrefixDesc: prefix,
	}
}

// Bytes returns the wire-format encoding per RFC 7752 Section 3.2.3.
// Prefix descriptor TLVs (263-265) appear directly in the NLRI body after
// the Local Node Descriptors, NOT wrapped in a container TLV.
func (p *BGPLSPrefix) Bytes() []byte {
	buf := make([]byte, p.Len())
	p.WriteTo(buf, 0)
	return buf
}

// Len returns the length in bytes.
// Calculates arithmetically without allocating, matching WriteTo logic:
// NLRI header (4) + body (9) + local node TLV (4+len) + prefix desc.
func (p *BGPLSPrefix) Len() int {
	if p.cached != nil {
		return len(p.cached)
	}
	return 4 + 9 + (4 + p.LocalNode.Len()) + p.PrefixDesc.Len()
}

// String returns command-style format for API round-trip compatibility.
// Format: reachability protocol <proto> type <type> asn <n>.
func (p *BGPLSPrefix) String() string {
	return fmt.Sprintf("reachability protocol %s type %s asn %d", p.protocolID, p.nlriType, p.LocalNode.ASN)
}

// WriteTo writes the Prefix NLRI directly to buf at offset.
//
//nolint:dupl // Similar structure to BGPLSSRv6SID.WriteTo is intentional
func (p *BGPLSPrefix) WriteTo(buf []byte, off int) int {
	// Fallback: use cached bytes if present
	if p.cached != nil {
		return copy(buf[off:], p.cached)
	}

	pos := off

	// Calculate body length
	localNodeLen := p.LocalNode.Len()
	prefixDescLen := p.PrefixDesc.Len()
	bodyLen := 9 + (4 + localNodeLen) + prefixDescLen

	// Write NLRI header
	binary.BigEndian.PutUint16(buf[pos:], uint16(p.nlriType))
	binary.BigEndian.PutUint16(buf[pos+2:], uint16(bodyLen)) //nolint:gosec // Length validated
	pos += 4

	// Write body: Protocol-ID + Identifier
	buf[pos] = byte(p.protocolID)
	binary.BigEndian.PutUint64(buf[pos+1:], p.identifier)
	pos += 9

	// Write Local Node TLV (256)
	binary.BigEndian.PutUint16(buf[pos:], TLVLocalNodeDesc)
	binary.BigEndian.PutUint16(buf[pos+2:], uint16(localNodeLen)) //nolint:gosec // Length validated
	pos += 4
	pos += p.LocalNode.WriteTo(buf, pos)

	// Write Prefix Descriptor TLVs (directly, not wrapped)
	pos += p.PrefixDesc.WriteTo(buf, pos)

	return pos - off
}
