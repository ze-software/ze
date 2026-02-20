// Design: docs/architecture/wire/nlri-bgpls.md — BGP-LS NLRI plugin
// Design: rfc/short/rfc7752.md
//
// Package bgp_ls implements BGP-LS family types and plugin for ze.
// RFC 7752: North-Bound Distribution of Link-State and TE Information Using BGP
// RFC 9085: BGP-LS Extensions for Segment Routing
// RFC 9514: BGP-LS Extensions for SRv6
package bgp_nlri_ls

import (
	"encoding/binary"
	"errors"
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/wire"
)

// Type aliases for nlri types used by BGP-LS.
type (
	Family = nlri.Family
	AFI    = nlri.AFI
	SAFI   = nlri.SAFI
	NLRI   = nlri.NLRI
)

// Re-export constants from nlri for local use.
const (
	AFIBGPLS            = nlri.AFIBGPLS
	SAFIBGPLinkState    = nlri.SAFIBGPLinkState
	SAFIBGPLinkStateVPN = nlri.SAFIBGPLinkStateVPN
)

// BGPLSFamily is the address family for BGP-LS.
var BGPLSFamily = nlri.Family{AFI: AFIBGPLS, SAFI: SAFIBGPLinkState}

// BGP-LS errors.
var (
	ErrBGPLSTruncated   = errors.New("bgp-ls: truncated data")
	ErrBGPLSInvalidType = errors.New("bgp-ls: invalid NLRI type")
)

// BGPLSNLRIType identifies the type of BGP-LS NLRI.
// RFC 7752 Section 3.2, Table 1 - NLRI Types defines Node, Link, and Prefix types.
type BGPLSNLRIType uint16

// BGP-LS NLRI types.
// RFC 7752 Section 3.2, Table 1:
//
//	+------+---------------------------+
//	| Type | NLRI Type                 |
//	+------+---------------------------+
//	|   1  | Node NLRI                 |
//	|   2  | Link NLRI                 |
//	|   3  | IPv4 Topology Prefix NLRI |
//	|   4  | IPv6 Topology Prefix NLRI |
//	+------+---------------------------+
//
// Note: Type 6 (SRv6 SID NLRI) is defined in RFC 9514, not RFC 7752.
const (
	BGPLSNodeNLRI     BGPLSNLRIType = 1 // Node NLRI (RFC 7752 Section 3.2)
	BGPLSLinkNLRI     BGPLSNLRIType = 2 // Link NLRI (RFC 7752 Section 3.2)
	BGPLSPrefixV4NLRI BGPLSNLRIType = 3 // IPv4 Topology Prefix NLRI (RFC 7752 Section 3.2)
	BGPLSPrefixV6NLRI BGPLSNLRIType = 4 // IPv6 Topology Prefix NLRI (RFC 7752 Section 3.2)
	BGPLSSRv6SIDNLRI  BGPLSNLRIType = 6 // SRv6 SID NLRI (RFC 9514)
)

// String returns a human-readable NLRI type name.
func (t BGPLSNLRIType) String() string {
	switch t {
	case BGPLSNodeNLRI:
		return "node"
	case BGPLSLinkNLRI:
		return "link"
	case BGPLSPrefixV4NLRI:
		return "prefix-v4"
	case BGPLSPrefixV6NLRI:
		return "prefix-v6"
	case BGPLSSRv6SIDNLRI:
		return "srv6-sid"
	default:
		return fmt.Sprintf("type(%d)", t)
	}
}

// BGPLSProtocolID identifies the IGP protocol source of link-state information.
// RFC 7752 Section 3.2, Table 2 - Protocol Identifiers for IS-IS, OSPF, etc.
type BGPLSProtocolID uint8

// Protocol IDs.
// RFC 7752 Section 3.2, Table 2:
//
//	+-------------+----------------------------------+
//	| Protocol-ID | NLRI information source protocol |
//	+-------------+----------------------------------+
//	|      1      | IS-IS Level 1                    |
//	|      2      | IS-IS Level 2                    |
//	|      3      | OSPFv2                           |
//	|      4      | Direct                           |
//	|      5      | Static configuration             |
//	|      6      | OSPFv3                           |
//	+-------------+----------------------------------+
//
// Note: Protocol IDs 7-9 are not defined in RFC 7752 but are used by implementations.
const (
	ProtoISISL1  BGPLSProtocolID = 1 // IS-IS Level 1 (RFC 7752 Section 3.2)
	ProtoISISL2  BGPLSProtocolID = 2 // IS-IS Level 2 (RFC 7752 Section 3.2)
	ProtoOSPFv2  BGPLSProtocolID = 3 // OSPFv2 (RFC 7752 Section 3.2)
	ProtoDirect  BGPLSProtocolID = 4 // Direct (RFC 7752 Section 3.2)
	ProtoStatic  BGPLSProtocolID = 5 // Static configuration (RFC 7752 Section 3.2)
	ProtoOSPFv3  BGPLSProtocolID = 6 // OSPFv3 (RFC 7752 Section 3.2)
	ProtoBGP     BGPLSProtocolID = 7 // BGP (implementation extension)
	ProtoRSVPTE  BGPLSProtocolID = 8 // RSVP-TE (implementation extension)
	ProtoSegment BGPLSProtocolID = 9 // Segment Routing (implementation extension)
)

// String returns a human-readable protocol name.
func (p BGPLSProtocolID) String() string {
	switch p { //nolint:exhaustive // Unknown protocols formatted in default
	case ProtoISISL1:
		return "isis-l1"
	case ProtoISISL2:
		return "isis-l2"
	case ProtoOSPFv2:
		return "ospfv2"
	case ProtoDirect:
		return "direct"
	case ProtoStatic:
		return "static"
	case ProtoOSPFv3:
		return "ospfv3"
	case ProtoBGP:
		return "bgp"
	default:
		return fmt.Sprintf("proto(%d)", p)
	}
}

// BGP-LS TLV types for node descriptors.
// RFC 7752 Section 3.2.1, Table 3 - Node Descriptor TLVs:
//
//	+------------+---------------------+
//	| TLV Code   | Description         |
//	+------------+---------------------+
//	|    256     | Local Node Desc     |
//	|    257     | Remote Node Desc    |
//	+------------+---------------------+
const (
	TLVLocalNodeDesc  uint16 = 256 // Local Node Descriptors (RFC 7752 Section 3.2.1.2)
	TLVRemoteNodeDesc uint16 = 257 // Remote Node Descriptors (RFC 7752 Section 3.2.1.3)
)

// Node Descriptor Sub-TLVs.
// RFC 7752 Section 3.2.1.4, Table 4:
//
//	+------------+----------------------+----------+
//	| TLV Code   | Description          | Length   |
//	+------------+----------------------+----------+
//	|    512     | Autonomous System    | 4 bytes  |
//	|    513     | BGP-LS Identifier    | 4 bytes  |
//	|    514     | OSPF Area-ID         | 4 bytes  |
//	|    515     | IGP Router-ID        | Variable |
//	+------------+----------------------+----------+
const (
	TLVAutonomousSystem uint16 = 512 // Autonomous System (RFC 7752 Section 3.2.1.4)
	TLVBGPLSIdentifier  uint16 = 513 // BGP-LS Identifier (RFC 7752 Section 3.2.1.4)
	TLVOSPFAreaID       uint16 = 514 // OSPF Area-ID (RFC 7752 Section 3.2.1.4)
	TLVIGPRouterID      uint16 = 515 // IGP Router-ID (RFC 7752 Section 3.2.1.4)
)

// BGP-LS TLV types for link descriptors.
// RFC 7752 Section 3.2.2, Table 5:
//
//	+------------+----------------------------+----------+
//	| TLV Code   | Description                | Length   |
//	+------------+----------------------------+----------+
//	|    258     | Link Local/Remote ID       | 8 bytes  |
//	|    259     | IPv4 interface address     | 4 bytes  |
//	|    260     | IPv4 neighbor address      | 4 bytes  |
//	|    261     | IPv6 interface address     | 16 bytes |
//	|    262     | IPv6 neighbor address      | 16 bytes |
//	|    263     | Multi-Topology Identifier  | 2 bytes  |
//	+------------+----------------------------+----------+
//
// NOTE: TLVLinkDescriptors (258) is a VIOLATION - RFC 7752 does not define
// a "Link Descriptors" container TLV at 258. The code incorrectly treats
// Link Local/Remote ID (258) as a container. Per RFC 7752 Section 3.2.2,
// link descriptor TLVs appear directly in the Link NLRI, not wrapped.
const (
	TLVLinkDescriptors   uint16 = 258 // VIOLATION: Not in RFC 7752 - should not be used as container
	TLVLinkLocalRemoteID uint16 = 258 // Link Local/Remote Identifiers (RFC 7752 Section 3.2.2)
	TLVIPv4InterfaceAddr uint16 = 259 // IPv4 interface address (RFC 7752 Section 3.2.2)
	TLVIPv4NeighborAddr  uint16 = 260 // IPv4 neighbor address (RFC 7752 Section 3.2.2)
	TLVIPv6InterfaceAddr uint16 = 261 // IPv6 interface address (RFC 7752 Section 3.2.2)
	TLVIPv6NeighborAddr  uint16 = 262 // IPv6 neighbor address (RFC 7752 Section 3.2.2)
	TLVMultiTopologyID   uint16 = 263 // Multi-Topology Identifier (RFC 7752 Section 3.2.2)
)

// BGP-LS TLV types for prefix descriptors.
// RFC 7752 Section 3.2.3, Table 6:
//
//	+------------+----------------------------+----------+
//	| TLV Code   | Description                | Length   |
//	+------------+----------------------------+----------+
//	|    263     | Multi-Topology Identifier  | 2 bytes  |
//	|    264     | OSPF Route Type            | 1 byte   |
//	|    265     | IP Reachability Information| Variable |
//	+------------+----------------------------+----------+
//
// NOTE: TLVPrefixDescriptors (264) is a VIOLATION - RFC 7752 does not define
// a "Prefix Descriptors" container TLV at 264. The code incorrectly reuses
// OSPF Route Type (264) as a container. Per RFC 7752 Section 3.2.3,
// prefix descriptor TLVs appear directly in the Prefix NLRI, not wrapped.
const (
	TLVPrefixDescriptors  uint16 = 264 // VIOLATION: Not in RFC 7752 - should not be used as container
	TLVOSPFRouteType      uint16 = 264 // OSPF Route Type (RFC 7752 Section 3.2.3)
	TLVIPReachabilityInfo uint16 = 265 // IP Reachability Information (RFC 7752 Section 3.2.3)
)

// BGPLSNLRI is the interface for BGP-LS NLRI types.
// RFC 7752 Section 3.2 defines the common NLRI header format.
type BGPLSNLRI interface {
	NLRI
	NLRIType() BGPLSNLRIType     // RFC 7752 Section 3.2 - NLRI Type (2 bytes)
	ProtocolID() BGPLSProtocolID // RFC 7752 Section 3.2 - Protocol-ID (1 byte)
	Identifier() uint64          // RFC 7752 Section 3.2 - Identifier (8 bytes)
}

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

// BGP-LS SAFIs are defined in nlri.go (SAFIBGPLinkState, SAFIBGPLinkStateVPN).

// bgplsBase contains common fields for all BGP-LS NLRI types.
// RFC 7752 Section 3.2 defines the common NLRI header:
//
//	+------------------+
//	| NLRI Type (2)    |  2 bytes - NLRI type (Table 1)
//	+------------------+
//	| Total NLRI Len   |  2 bytes - length of NLRI body
//	+------------------+
//	| Protocol-ID (1)  |  1 byte - source protocol (Table 2)
//	+------------------+
//	| Identifier (8)   |  8 bytes - routing universe ID
//	+------------------+
//	| Descriptors      |  Variable - TLV encoded descriptors
//	+------------------+
type bgplsBase struct {
	nlriType   BGPLSNLRIType   // RFC 7752 Section 3.2 - NLRI Type
	protocolID BGPLSProtocolID // RFC 7752 Section 3.2 - Protocol-ID
	identifier uint64          // RFC 7752 Section 3.2 - Identifier
	cached     []byte
}

// Family returns the AFI/SAFI for BGP-LS.
// RFC 7752 Section 3.1: AFI 16388 (BGP-LS), SAFI 71 (Link-State).
func (b *bgplsBase) Family() Family {
	return Family{AFI: AFIBGPLS, SAFI: SAFIBGPLinkState}
}

func (b *bgplsBase) NLRIType() BGPLSNLRIType     { return b.nlriType }
func (b *bgplsBase) ProtocolID() BGPLSProtocolID { return b.protocolID }
func (b *bgplsBase) Identifier() uint64          { return b.identifier }
func (b *bgplsBase) PathID() uint32              { return 0 }
func (b *bgplsBase) HasPathID() bool             { return false }
func (b *bgplsBase) SupportsAddPath() bool       { return false }

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
	if n.cached != nil {
		return n.cached
	}

	// RFC 7752 Section 3.2.1.2 - Local Node Descriptors (TLV 256)
	localNodeData := n.LocalNode.Bytes()
	localNodeTLV := tlv(TLVLocalNodeDesc, localNodeData)

	// Build NLRI body per RFC 7752 Section 3.2
	body := make([]byte, 9+len(localNodeTLV))
	body[0] = byte(n.protocolID)                        // Protocol-ID (1 byte)
	binary.BigEndian.PutUint64(body[1:9], n.identifier) // Identifier (8 bytes)
	copy(body[9:], localNodeTLV)

	// Build full NLRI with type and length per RFC 7752 Section 3.2
	n.cached = make([]byte, 4+len(body))
	binary.BigEndian.PutUint16(n.cached[0:2], uint16(n.nlriType)) // NLRI Type (2 bytes)
	binary.BigEndian.PutUint16(n.cached[2:4], uint16(len(body)))  //nolint:gosec // Total NLRI Length (2 bytes)
	copy(n.cached[4:], body)

	return n.cached
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
// Format: node protocol set <proto> asn set <n>.
func (n *BGPLSNode) String() string {
	return fmt.Sprintf("node protocol set %s asn set %d", n.protocolID, n.LocalNode.ASN)
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
	if l.cached != nil {
		return l.cached
	}

	// RFC 7752 Section 3.2.1.2 - Local Node Descriptors (TLV 256)
	localNodeTLV := tlv(TLVLocalNodeDesc, l.LocalNode.Bytes())
	// RFC 7752 Section 3.2.1.3 - Remote Node Descriptors (TLV 257)
	remoteNodeTLV := tlv(TLVRemoteNodeDesc, l.RemoteNode.Bytes())
	// RFC 7752 Section 3.2.2 - Link descriptor TLVs appear directly (not wrapped)
	linkDescBytes := l.LinkDesc.Bytes()

	bodyLen := 9 + len(localNodeTLV) + len(remoteNodeTLV) + len(linkDescBytes)
	body := make([]byte, bodyLen)
	body[0] = byte(l.protocolID)                        // Protocol-ID (1 byte)
	binary.BigEndian.PutUint64(body[1:9], l.identifier) // Identifier (8 bytes)
	offset := 9
	copy(body[offset:], localNodeTLV)
	offset += len(localNodeTLV)
	copy(body[offset:], remoteNodeTLV)
	offset += len(remoteNodeTLV)
	copy(body[offset:], linkDescBytes)

	// RFC 7752 Section 3.2 - NLRI header
	l.cached = make([]byte, 4+len(body))
	binary.BigEndian.PutUint16(l.cached[0:2], uint16(l.nlriType)) // NLRI Type (2 bytes)
	binary.BigEndian.PutUint16(l.cached[2:4], uint16(len(body)))  //nolint:gosec // Total NLRI Length (2 bytes)
	copy(l.cached[4:], body)

	return l.cached
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
// Format: link protocol set <proto> local-asn set <n> remote-asn set <m>.
func (l *BGPLSLink) String() string {
	return fmt.Sprintf("link protocol set %s local-asn set %d remote-asn set %d", l.protocolID, l.LocalNode.ASN, l.RemoteNode.ASN)
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
//
//nolint:dupl // Similar structure to BGPLSSRv6SID.Bytes() is intentional
func (p *BGPLSPrefix) Bytes() []byte {
	if p.cached != nil {
		return p.cached
	}

	// RFC 7752 Section 3.2.1.2 - Local Node Descriptors (TLV 256)
	localNodeTLV := tlv(TLVLocalNodeDesc, p.LocalNode.Bytes())
	// RFC 7752 Section 3.2.3 - Prefix descriptor TLVs appear directly (not wrapped)
	prefixDescBytes := p.PrefixDesc.Bytes()

	bodyLen := 9 + len(localNodeTLV) + len(prefixDescBytes)
	body := make([]byte, bodyLen)
	body[0] = byte(p.protocolID)                        // Protocol-ID (1 byte)
	binary.BigEndian.PutUint64(body[1:9], p.identifier) // Identifier (8 bytes)
	offset := 9
	copy(body[offset:], localNodeTLV)
	offset += len(localNodeTLV)
	copy(body[offset:], prefixDescBytes)

	// RFC 7752 Section 3.2 - NLRI header
	p.cached = make([]byte, 4+len(body))
	binary.BigEndian.PutUint16(p.cached[0:2], uint16(p.nlriType)) // NLRI Type (2 bytes)
	binary.BigEndian.PutUint16(p.cached[2:4], uint16(len(body)))  //nolint:gosec // Total NLRI Length (2 bytes)
	copy(p.cached[4:], body)

	return p.cached
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
// Format: prefix protocol set <proto> type set <type> asn set <n>.
func (p *BGPLSPrefix) String() string {
	return fmt.Sprintf("prefix protocol set %s type set %s asn set %d", p.protocolID, p.nlriType, p.LocalNode.ASN)
}

// ParseBGPLS parses a BGP-LS NLRI from wire format.
// RFC 7752 Section 3.2 defines the NLRI encoding:
//
//	+------------------+
//	| NLRI Type (2)    |  <- data[0:2]
//	+------------------+
//	| Total NLRI Len   |  <- data[2:4]
//	+------------------+
//	| Protocol-ID (1)  |  <- body[0]
//	+------------------+
//	| Identifier (8)   |  <- body[1:9]
//	+------------------+
//	| Descriptors      |  <- body[9:]
//	+------------------+
func ParseBGPLS(data []byte) (BGPLSNLRI, error) {
	// RFC 7752 Section 3.2 - minimum 4 bytes for Type + Length header
	if len(data) < 4 {
		return nil, ErrBGPLSTruncated
	}

	// RFC 7752 Section 3.2 - NLRI Type (2 bytes)
	nlriType := BGPLSNLRIType(binary.BigEndian.Uint16(data[0:2]))
	// RFC 7752 Section 3.2 - Total NLRI Length (2 bytes)
	nlriLen := int(binary.BigEndian.Uint16(data[2:4]))

	if len(data) < 4+nlriLen {
		return nil, ErrBGPLSTruncated
	}

	// RFC 7752 Section 3.2 - minimum 9 bytes for Protocol-ID + Identifier
	if nlriLen < 9 {
		return nil, ErrBGPLSTruncated
	}

	body := data[4 : 4+nlriLen]
	proto := BGPLSProtocolID(body[0])                // Protocol-ID (1 byte)
	identifier := binary.BigEndian.Uint64(body[1:9]) // Identifier (8 bytes)

	switch nlriType { //nolint:exhaustive // Unsupported types handled in default
	case BGPLSNodeNLRI:
		// RFC 7752 Section 3.2.1 - Node NLRI (Type 1)
		node := &BGPLSNode{
			bgplsBase: bgplsBase{
				nlriType:   nlriType,
				protocolID: proto,
				identifier: identifier,
			},
		}
		// RFC 7752 Section 3.2.1.2 - Parse Local Node Descriptors
		if err := parseNodeDescriptorTLVs(body[9:], &node.LocalNode); err != nil {
			return nil, err
		}
		node.cached = data[:4+nlriLen]
		return node, nil

	case BGPLSLinkNLRI:
		// RFC 7752 Section 3.2.2 - Link NLRI (Type 2)
		link := &BGPLSLink{
			bgplsBase: bgplsBase{
				nlriType:   nlriType,
				protocolID: proto,
				identifier: identifier,
			},
		}
		link.cached = data[:4+nlriLen]
		return link, nil

	case BGPLSPrefixV4NLRI, BGPLSPrefixV6NLRI:
		// RFC 7752 Section 3.2.3 - Prefix NLRI (Types 3 and 4)
		prefix := &BGPLSPrefix{
			bgplsBase: bgplsBase{
				nlriType:   nlriType,
				protocolID: proto,
				identifier: identifier,
			},
		}
		prefix.cached = data[:4+nlriLen]
		return prefix, nil

	case BGPLSSRv6SIDNLRI:
		// RFC 9514 - SRv6 SID NLRI (Type 6)
		srv6 := &BGPLSSRv6SID{
			bgplsBase: bgplsBase{
				nlriType:   nlriType,
				protocolID: proto,
				identifier: identifier,
			},
		}
		// Parse Local Node Descriptors (same format as RFC 7752)
		if err := parseNodeDescriptorTLVs(body[9:], &srv6.LocalNode); err != nil {
			return nil, err
		}
		srv6.cached = data[:4+nlriLen]
		return srv6, nil

	default:
		return nil, ErrBGPLSInvalidType
	}
}

// parseNodeDescriptorTLVs parses TLVs into a NodeDescriptor.
// RFC 7752 Section 3.2.1.4 defines the node descriptor sub-TLV format:
//
//	+------------------+
//	| Type (2 bytes)   |
//	+------------------+
//	| Length (2 bytes) |
//	+------------------+
//	| Value (variable) |
//	+------------------+
func parseNodeDescriptorTLVs(data []byte, nd *NodeDescriptor) error {
	for len(data) >= 4 {
		tlvType := binary.BigEndian.Uint16(data[0:2])     // TLV Type (2 bytes)
		tlvLen := int(binary.BigEndian.Uint16(data[2:4])) // TLV Length (2 bytes)

		if len(data) < 4+tlvLen {
			return ErrBGPLSTruncated
		}

		value := data[4 : 4+tlvLen]

		// RFC 7752 Section 3.2.1.2 - Local Node Descriptor container (TLV 256)
		if tlvType == TLVLocalNodeDesc {
			return parseNodeDescriptorTLVs(value, nd)
		}

		// RFC 7752 Section 3.2.1.4 - Node Descriptor Sub-TLVs
		switch tlvType {
		case TLVAutonomousSystem: // TLV 512 - 4 bytes
			if len(value) >= 4 {
				nd.ASN = binary.BigEndian.Uint32(value)
			}
		case TLVBGPLSIdentifier: // TLV 513 - 4 bytes
			if len(value) >= 4 {
				nd.BGPLSIdentifier = binary.BigEndian.Uint32(value)
			}
		case TLVOSPFAreaID: // TLV 514 - 4 bytes
			if len(value) >= 4 {
				nd.OSPFAreaID = binary.BigEndian.Uint32(value)
			}
		case TLVIGPRouterID: // TLV 515 - variable length
			nd.IGPRouterID = make([]byte, len(value))
			copy(nd.IGPRouterID, value)
		}

		data = data[4+tlvLen:]
	}

	return nil
}

// Helper functions

// tlv encodes a Type-Length-Value structure.
// RFC 7752 uses TLV encoding throughout:
//
//	+------------------+
//	| Type (2 bytes)   |
//	+------------------+
//	| Length (2 bytes) |
//	+------------------+
//	| Value (variable) |
//	+------------------+
func tlv(t uint16, v []byte) []byte {
	data := make([]byte, 4+len(v))
	binary.BigEndian.PutUint16(data[0:2], t)              // Type (2 bytes)
	binary.BigEndian.PutUint16(data[2:4], uint16(len(v))) //nolint:gosec // Length (2 bytes)
	copy(data[4:], v)                                     // Value
	return data
}

func uint32ToBytes(v uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return b
}

// writeTLV writes a TLV header and reserves space for the value.
// Returns total bytes written (4 + valueLen).
// Caller must write value data to buf[off+4:off+4+valueLen].
func writeTLV(buf []byte, off int, tlvType uint16, valueLen int) int {
	binary.BigEndian.PutUint16(buf[off:], tlvType)
	binary.BigEndian.PutUint16(buf[off+2:], uint16(valueLen)) //nolint:gosec // Length validated by caller
	return 4 + valueLen
}

// writeTLVBytes writes a complete TLV with value bytes.
// Returns total bytes written.
func writeTLVBytes(buf []byte, off int, tlvType uint16, value []byte) int {
	binary.BigEndian.PutUint16(buf[off:], tlvType)
	binary.BigEndian.PutUint16(buf[off+2:], uint16(len(value))) //nolint:gosec // Length validated by caller
	copy(buf[off+4:], value)
	return 4 + len(value)
}

// ============================================================================
// SRv6 SID NLRI (RFC 9514)
// Note: This is NOT part of RFC 7752 but extends BGP-LS for Segment Routing v6.
// ============================================================================

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

// TLV types for SRv6.
// Note: These are defined in RFC 9514, not RFC 7752.
const (
	TLVSRv6SID uint16 = 518 // SRv6 SID (RFC 9514, not RFC 7752)
)

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

// WriteTo methods for BGP-LS types (zero-alloc).

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

// ParseBGPLSWithRest parses a single BGP-LS NLRI and returns the remaining data.
// This enables parsing multiple packed NLRIs from MP_REACH/MP_UNREACH.
// RFC 7752 Section 3.2: NLRI format is Type(2) + Length(2) + Value(Length).
func ParseBGPLSWithRest(data []byte) (BGPLSNLRI, []byte, error) {
	// Need at least 4 bytes for Type + Length header
	if len(data) < 4 {
		return nil, nil, ErrBGPLSTruncated
	}

	// RFC 7752 Section 3.2 - Total NLRI Length (bytes 2-3)
	nlriLen := int(data[2])<<8 | int(data[3])
	totalLen := 4 + nlriLen

	if len(data) < totalLen {
		return nil, nil, ErrBGPLSTruncated
	}

	// Parse just this NLRI
	parsed, err := ParseBGPLS(data[:totalLen])
	if err != nil {
		return nil, nil, err
	}

	return parsed, data[totalLen:], nil
}
