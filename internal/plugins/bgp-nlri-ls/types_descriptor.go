// Design: docs/architecture/wire/nlri-bgpls.md — BGP-LS descriptor TLV encoding
// RFC: rfc/short/rfc7752.md — BGP-LS node and link descriptors
// Overview: types.go — core types, TLV constants, and helper functions
// Related: types_nlri.go — NLRI types that embed these descriptors
// Related: types_srv6.go — SRv6 SID descriptor (RFC 9514)
package bgp_nlri_ls

import (
	"encoding/binary"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/wire"
)

// NodeDescriptor contains node identification information.
// RFC 7752 Section 3.2.1.4 defines the node descriptor sub-TLVs.
type NodeDescriptor struct {
	ASN             uint32 // Autonomous System (TLV 512, RFC 7752 Section 3.2.1.4)
	BGPLSIdentifier uint32 // BGP-LS Identifier (TLV 513, RFC 7752 Section 3.2.1.4)
	OSPFAreaID      uint32 // OSPF Area-ID (TLV 514, RFC 7752 Section 3.2.1.4)
	IGPRouterID     []byte // IGP Router-ID (TLV 515, RFC 7752 Section 3.2.1.4)
}

// Bytes encodes the node descriptor as TLVs.
// RFC 7752 Section 3.2.1.4 specifies the encoding of node descriptor sub-TLVs.
func (nd *NodeDescriptor) Bytes() []byte {
	var data []byte

	// ASN TLV (512) - RFC 7752 Section 3.2.1.4
	if nd.ASN != 0 {
		data = append(data, tlv(TLVAutonomousSystem, uint32ToBytes(nd.ASN))...)
	}

	// BGP-LS Identifier TLV (513) - RFC 7752 Section 3.2.1.4
	if nd.BGPLSIdentifier != 0 {
		data = append(data, tlv(TLVBGPLSIdentifier, uint32ToBytes(nd.BGPLSIdentifier))...)
	}

	// OSPF Area-ID TLV (514) - RFC 7752 Section 3.2.1.4
	if nd.OSPFAreaID != 0 {
		data = append(data, tlv(TLVOSPFAreaID, uint32ToBytes(nd.OSPFAreaID))...)
	}

	// IGP Router-ID TLV (515) - RFC 7752 Section 3.2.1.4
	if len(nd.IGPRouterID) > 0 {
		data = append(data, tlv(TLVIGPRouterID, nd.IGPRouterID)...)
	}

	return data
}

// Len returns the TLV-encoded length in bytes.
func (nd *NodeDescriptor) Len() int {
	n := 0
	if nd.ASN != 0 {
		n += 4 + 4 // TLV header + 4-byte value
	}
	if nd.BGPLSIdentifier != 0 {
		n += 4 + 4
	}
	if nd.OSPFAreaID != 0 {
		n += 4 + 4
	}
	if len(nd.IGPRouterID) > 0 {
		n += 4 + len(nd.IGPRouterID)
	}
	return n
}

// WriteTo writes the node descriptor TLVs directly to buf at offset.
// Returns bytes written.
func (nd *NodeDescriptor) WriteTo(buf []byte, off int) int {
	pos := off

	if nd.ASN != 0 {
		pos += writeTLV(buf, pos, TLVAutonomousSystem, 4)
		binary.BigEndian.PutUint32(buf[pos-4:], nd.ASN)
	}
	if nd.BGPLSIdentifier != 0 {
		pos += writeTLV(buf, pos, TLVBGPLSIdentifier, 4)
		binary.BigEndian.PutUint32(buf[pos-4:], nd.BGPLSIdentifier)
	}
	if nd.OSPFAreaID != 0 {
		pos += writeTLV(buf, pos, TLVOSPFAreaID, 4)
		binary.BigEndian.PutUint32(buf[pos-4:], nd.OSPFAreaID)
	}
	if len(nd.IGPRouterID) > 0 {
		pos += writeTLVBytes(buf, pos, TLVIGPRouterID, nd.IGPRouterID)
	}

	return pos - off
}

// CheckedWriteTo validates capacity before writing.
func (nd *NodeDescriptor) CheckedWriteTo(buf []byte, off int) (int, error) {
	needed := nd.Len()
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return nd.WriteTo(buf, off), nil
}

// LinkDescriptor contains link identification information.
// RFC 7752 Section 3.2.2 defines the link descriptor TLVs.
type LinkDescriptor struct {
	LinkLocalID        uint32 // Link Local ID (TLV 258, RFC 7752 Section 3.2.2)
	LinkRemoteID       uint32 // Link Remote ID (TLV 258, RFC 7752 Section 3.2.2)
	LocalInterfaceAddr []byte // IPv4 (TLV 259) or IPv6 (TLV 261) Interface Address
	NeighborAddr       []byte // IPv4 (TLV 260) or IPv6 (TLV 262) Neighbor Address
	MultiTopologyID    uint16 // Multi-Topology ID (TLV 263, RFC 7752 Section 3.2.2)
}

// Bytes encodes the link descriptor as TLVs.
// RFC 7752 Section 3.2.2 specifies the encoding of link descriptor TLVs.
func (ld *LinkDescriptor) Bytes() []byte {
	var data []byte

	// Link Local/Remote Identifiers (TLV 258) - RFC 7752 Section 3.2.2
	// Format: 4-byte Local ID + 4-byte Remote ID = 8 bytes total
	if ld.LinkLocalID != 0 || ld.LinkRemoteID != 0 {
		val := make([]byte, 8)
		binary.BigEndian.PutUint32(val[0:4], ld.LinkLocalID)
		binary.BigEndian.PutUint32(val[4:8], ld.LinkRemoteID)
		data = append(data, tlv(TLVLinkLocalRemoteID, val)...)
	}

	// IPv4 Interface Address (TLV 259) or IPv6 Interface Address (TLV 261)
	// RFC 7752 Section 3.2.2
	if len(ld.LocalInterfaceAddr) == 4 {
		data = append(data, tlv(TLVIPv4InterfaceAddr, ld.LocalInterfaceAddr)...)
	} else if len(ld.LocalInterfaceAddr) == 16 {
		data = append(data, tlv(TLVIPv6InterfaceAddr, ld.LocalInterfaceAddr)...)
	}

	// IPv4 Neighbor Address (TLV 260) or IPv6 Neighbor Address (TLV 262)
	// RFC 7752 Section 3.2.2
	if len(ld.NeighborAddr) == 4 {
		data = append(data, tlv(TLVIPv4NeighborAddr, ld.NeighborAddr)...)
	} else if len(ld.NeighborAddr) == 16 {
		data = append(data, tlv(TLVIPv6NeighborAddr, ld.NeighborAddr)...)
	}

	return data
}

// Len returns the TLV-encoded length in bytes.
func (ld *LinkDescriptor) Len() int {
	n := 0
	if ld.LinkLocalID != 0 || ld.LinkRemoteID != 0 {
		n += 4 + 8 // TLV header + 8-byte value
	}
	if len(ld.LocalInterfaceAddr) > 0 {
		n += 4 + len(ld.LocalInterfaceAddr)
	}
	if len(ld.NeighborAddr) > 0 {
		n += 4 + len(ld.NeighborAddr)
	}
	return n
}

// WriteTo writes the link descriptor TLVs directly to buf at offset.
// Returns bytes written.
func (ld *LinkDescriptor) WriteTo(buf []byte, off int) int {
	pos := off

	if ld.LinkLocalID != 0 || ld.LinkRemoteID != 0 {
		pos += writeTLV(buf, pos, TLVLinkLocalRemoteID, 8)
		binary.BigEndian.PutUint32(buf[pos-8:], ld.LinkLocalID)
		binary.BigEndian.PutUint32(buf[pos-4:], ld.LinkRemoteID)
	}

	if len(ld.LocalInterfaceAddr) == 4 {
		pos += writeTLVBytes(buf, pos, TLVIPv4InterfaceAddr, ld.LocalInterfaceAddr)
	} else if len(ld.LocalInterfaceAddr) == 16 {
		pos += writeTLVBytes(buf, pos, TLVIPv6InterfaceAddr, ld.LocalInterfaceAddr)
	}

	if len(ld.NeighborAddr) == 4 {
		pos += writeTLVBytes(buf, pos, TLVIPv4NeighborAddr, ld.NeighborAddr)
	} else if len(ld.NeighborAddr) == 16 {
		pos += writeTLVBytes(buf, pos, TLVIPv6NeighborAddr, ld.NeighborAddr)
	}

	return pos - off
}

// CheckedWriteTo validates capacity before writing.
func (ld *LinkDescriptor) CheckedWriteTo(buf []byte, off int) (int, error) {
	needed := ld.Len()
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return ld.WriteTo(buf, off), nil
}

// PrefixDescriptor contains prefix identification information.
// RFC 7752 Section 3.2.3 defines the prefix descriptor TLVs.
type PrefixDescriptor struct {
	MultiTopologyID    uint16 // Multi-Topology ID (TLV 263, RFC 7752 Section 3.2.3)
	OSPFRouteType      uint8  // OSPF Route Type (TLV 264, RFC 7752 Section 3.2.3)
	IPReachabilityInfo []byte // IP Reachability Information (TLV 265, RFC 7752 Section 3.2.3)
}

// Bytes encodes the prefix descriptor as TLVs.
// RFC 7752 Section 3.2.3 specifies the encoding of prefix descriptor TLVs.
func (pd *PrefixDescriptor) Bytes() []byte {
	var data []byte

	// IP Reachability Information (TLV 265) - RFC 7752 Section 3.2.3
	if len(pd.IPReachabilityInfo) > 0 {
		data = append(data, tlv(TLVIPReachabilityInfo, pd.IPReachabilityInfo)...)
	}

	return data
}

// Len returns the TLV-encoded length in bytes.
func (pd *PrefixDescriptor) Len() int {
	if len(pd.IPReachabilityInfo) > 0 {
		return 4 + len(pd.IPReachabilityInfo)
	}
	return 0
}

// WriteTo writes the prefix descriptor TLVs directly to buf at offset.
// Returns bytes written.
func (pd *PrefixDescriptor) WriteTo(buf []byte, off int) int {
	if len(pd.IPReachabilityInfo) > 0 {
		return writeTLVBytes(buf, off, TLVIPReachabilityInfo, pd.IPReachabilityInfo)
	}
	return 0
}

// CheckedWriteTo validates capacity before writing.
func (pd *PrefixDescriptor) CheckedWriteTo(buf []byte, off int) (int, error) {
	needed := pd.Len()
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return pd.WriteTo(buf, off), nil
}
