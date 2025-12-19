package nlri

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// BGP-LS errors.
var (
	ErrBGPLSTruncated   = errors.New("bgp-ls: truncated data")
	ErrBGPLSInvalidType = errors.New("bgp-ls: invalid NLRI type")
)

// BGPLSNLRIType identifies the type of BGP-LS NLRI (RFC 7752).
type BGPLSNLRIType uint16

// BGP-LS NLRI types (RFC 7752 Section 3.2).
const (
	BGPLSNodeNLRI     BGPLSNLRIType = 1 // Node NLRI
	BGPLSLinkNLRI     BGPLSNLRIType = 2 // Link NLRI
	BGPLSPrefixV4NLRI BGPLSNLRIType = 3 // IPv4 Topology Prefix NLRI
	BGPLSPrefixV6NLRI BGPLSNLRIType = 4 // IPv6 Topology Prefix NLRI
	BGPLSSRv6SIDNLRI  BGPLSNLRIType = 6 // SRv6 SID NLRI
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

// BGPLSProtocolID identifies the IGP protocol (RFC 7752 Section 3.2).
type BGPLSProtocolID uint8

// Protocol IDs.
const (
	ProtoISISL1  BGPLSProtocolID = 1 // IS-IS Level 1
	ProtoISISL2  BGPLSProtocolID = 2 // IS-IS Level 2
	ProtoOSPFv2  BGPLSProtocolID = 3 // OSPFv2
	ProtoDirect  BGPLSProtocolID = 4 // Direct
	ProtoStatic  BGPLSProtocolID = 5 // Static
	ProtoOSPFv3  BGPLSProtocolID = 6 // OSPFv3
	ProtoBGP     BGPLSProtocolID = 7 // BGP
	ProtoRSVPTE  BGPLSProtocolID = 8 // RSVP-TE
	ProtoSegment BGPLSProtocolID = 9 // Segment Routing
)

// String returns a human-readable protocol name.
func (p BGPLSProtocolID) String() string {
	switch p {
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
const (
	TLVLocalNodeDesc  uint16 = 256 // Local Node Descriptors
	TLVRemoteNodeDesc uint16 = 257 // Remote Node Descriptors

	// Node Descriptor Sub-TLVs
	TLVAutonomousSystem uint16 = 512 // Autonomous System
	TLVBGPLSIdentifier  uint16 = 513 // BGP-LS Identifier
	TLVOSPFAreaID       uint16 = 514 // OSPF Area ID
	TLVIGPRouterID      uint16 = 515 // IGP Router ID
)

// BGP-LS TLV types for link descriptors.
const (
	TLVLinkDescriptors   uint16 = 258 // Link Descriptors
	TLVLinkLocalRemoteID uint16 = 258 // Link Local/Remote Identifiers
	TLVIPv4InterfaceAddr uint16 = 259 // IPv4 Interface Address
	TLVIPv4NeighborAddr  uint16 = 260 // IPv4 Neighbor Address
	TLVIPv6InterfaceAddr uint16 = 261 // IPv6 Interface Address
	TLVIPv6NeighborAddr  uint16 = 262 // IPv6 Neighbor Address
	TLVMultiTopologyID   uint16 = 263 // Multi-Topology ID
)

// BGP-LS TLV types for prefix descriptors.
const (
	TLVPrefixDescriptors  uint16 = 264 // Prefix Descriptors
	TLVOSPFRouteType      uint16 = 264 // OSPF Route Type
	TLVIPReachabilityInfo uint16 = 265 // IP Reachability Information
)

// BGPLSNLRI is the interface for BGP-LS NLRI types.
type BGPLSNLRI interface {
	NLRI
	NLRIType() BGPLSNLRIType
	ProtocolID() BGPLSProtocolID
	Identifier() uint64
}

// NodeDescriptor contains node identification information.
type NodeDescriptor struct {
	ASN             uint32 // Autonomous System Number
	BGPLSIdentifier uint32 // BGP-LS Identifier (Domain ID)
	OSPFAreaID      uint32 // OSPF Area ID
	IGPRouterID     []byte // IGP Router ID (4 or 6 bytes)
}

// Bytes encodes the node descriptor as TLVs.
func (nd *NodeDescriptor) Bytes() []byte {
	var data []byte

	// ASN TLV
	if nd.ASN != 0 {
		data = append(data, tlv(TLVAutonomousSystem, uint32ToBytes(nd.ASN))...)
	}

	// BGP-LS Identifier TLV
	if nd.BGPLSIdentifier != 0 {
		data = append(data, tlv(TLVBGPLSIdentifier, uint32ToBytes(nd.BGPLSIdentifier))...)
	}

	// OSPF Area ID TLV
	if nd.OSPFAreaID != 0 {
		data = append(data, tlv(TLVOSPFAreaID, uint32ToBytes(nd.OSPFAreaID))...)
	}

	// IGP Router ID TLV
	if len(nd.IGPRouterID) > 0 {
		data = append(data, tlv(TLVIGPRouterID, nd.IGPRouterID)...)
	}

	return data
}

// LinkDescriptor contains link identification information.
type LinkDescriptor struct {
	LinkLocalID        uint32 // Link Local Identifier
	LinkRemoteID       uint32 // Link Remote Identifier
	LocalInterfaceAddr []byte // IPv4/IPv6 Interface Address
	NeighborAddr       []byte // IPv4/IPv6 Neighbor Address
	MultiTopologyID    uint16 // Multi-Topology ID
}

// Bytes encodes the link descriptor as TLVs.
func (ld *LinkDescriptor) Bytes() []byte {
	var data []byte

	if ld.LinkLocalID != 0 || ld.LinkRemoteID != 0 {
		val := make([]byte, 8)
		binary.BigEndian.PutUint32(val[0:4], ld.LinkLocalID)
		binary.BigEndian.PutUint32(val[4:8], ld.LinkRemoteID)
		data = append(data, tlv(TLVLinkLocalRemoteID, val)...)
	}

	if len(ld.LocalInterfaceAddr) == 4 {
		data = append(data, tlv(TLVIPv4InterfaceAddr, ld.LocalInterfaceAddr)...)
	} else if len(ld.LocalInterfaceAddr) == 16 {
		data = append(data, tlv(TLVIPv6InterfaceAddr, ld.LocalInterfaceAddr)...)
	}

	if len(ld.NeighborAddr) == 4 {
		data = append(data, tlv(TLVIPv4NeighborAddr, ld.NeighborAddr)...)
	} else if len(ld.NeighborAddr) == 16 {
		data = append(data, tlv(TLVIPv6NeighborAddr, ld.NeighborAddr)...)
	}

	return data
}

// PrefixDescriptor contains prefix identification information.
type PrefixDescriptor struct {
	MultiTopologyID    uint16 // Multi-Topology ID
	OSPFRouteType      uint8  // OSPF Route Type
	IPReachabilityInfo []byte // IP Reachability Information
}

// Bytes encodes the prefix descriptor as TLVs.
func (pd *PrefixDescriptor) Bytes() []byte {
	var data []byte

	if len(pd.IPReachabilityInfo) > 0 {
		data = append(data, tlv(TLVIPReachabilityInfo, pd.IPReachabilityInfo)...)
	}

	return data
}

// BGP-LS SAFI
const SAFIBGPLinkState SAFI = 71

// bgplsBase contains common fields for all BGP-LS NLRI types.
type bgplsBase struct {
	nlriType   BGPLSNLRIType
	protocolID BGPLSProtocolID
	identifier uint64
	cached     []byte
}

func (b *bgplsBase) Family() Family {
	return Family{AFI: AFIBGPLS, SAFI: SAFIBGPLinkState}
}

func (b *bgplsBase) NLRIType() BGPLSNLRIType     { return b.nlriType }
func (b *bgplsBase) ProtocolID() BGPLSProtocolID { return b.protocolID }
func (b *bgplsBase) Identifier() uint64          { return b.identifier }
func (b *bgplsBase) PathID() uint32              { return 0 }
func (b *bgplsBase) HasPathID() bool             { return false }

// BGPLSNode represents a Node NLRI.
type BGPLSNode struct {
	bgplsBase
	LocalNode NodeDescriptor
}

// NewBGPLSNode creates a new Node NLRI.
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
func (n *BGPLSNode) Bytes() []byte {
	if n.cached != nil {
		return n.cached
	}

	// Encode local node descriptors
	localNodeData := n.LocalNode.Bytes()
	localNodeTLV := tlv(TLVLocalNodeDesc, localNodeData)

	// Build NLRI body
	body := make([]byte, 9+len(localNodeTLV))
	body[0] = byte(n.protocolID)
	binary.BigEndian.PutUint64(body[1:9], n.identifier)
	copy(body[9:], localNodeTLV)

	// Build full NLRI with type and length
	n.cached = make([]byte, 4+len(body))
	binary.BigEndian.PutUint16(n.cached[0:2], uint16(n.nlriType))
	binary.BigEndian.PutUint16(n.cached[2:4], uint16(len(body)))
	copy(n.cached[4:], body)

	return n.cached
}

// Len returns the length in bytes.
func (n *BGPLSNode) Len() int { return len(n.Bytes()) }

// String returns a human-readable representation.
func (n *BGPLSNode) String() string {
	return fmt.Sprintf("bgp-ls:node(asn=%d)", n.LocalNode.ASN)
}

// BGPLSLink represents a Link NLRI.
type BGPLSLink struct {
	bgplsBase
	LocalNode  NodeDescriptor
	RemoteNode NodeDescriptor
	LinkDesc   LinkDescriptor
}

// NewBGPLSLink creates a new Link NLRI.
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

// Bytes returns the wire-format encoding.
func (l *BGPLSLink) Bytes() []byte {
	if l.cached != nil {
		return l.cached
	}

	localNodeTLV := tlv(TLVLocalNodeDesc, l.LocalNode.Bytes())
	remoteNodeTLV := tlv(TLVRemoteNodeDesc, l.RemoteNode.Bytes())
	linkDescTLV := tlv(TLVLinkDescriptors, l.LinkDesc.Bytes())

	bodyLen := 9 + len(localNodeTLV) + len(remoteNodeTLV) + len(linkDescTLV)
	body := make([]byte, bodyLen)
	body[0] = byte(l.protocolID)
	binary.BigEndian.PutUint64(body[1:9], l.identifier)
	offset := 9
	copy(body[offset:], localNodeTLV)
	offset += len(localNodeTLV)
	copy(body[offset:], remoteNodeTLV)
	offset += len(remoteNodeTLV)
	copy(body[offset:], linkDescTLV)

	l.cached = make([]byte, 4+len(body))
	binary.BigEndian.PutUint16(l.cached[0:2], uint16(l.nlriType))
	binary.BigEndian.PutUint16(l.cached[2:4], uint16(len(body)))
	copy(l.cached[4:], body)

	return l.cached
}

// Len returns the length in bytes.
func (l *BGPLSLink) Len() int { return len(l.Bytes()) }

// String returns a human-readable representation.
func (l *BGPLSLink) String() string {
	return fmt.Sprintf("bgp-ls:link(%d->%d)", l.LocalNode.ASN, l.RemoteNode.ASN)
}

// BGPLSPrefix represents a Prefix NLRI (v4 or v6).
type BGPLSPrefix struct {
	bgplsBase
	LocalNode  NodeDescriptor
	PrefixDesc PrefixDescriptor
}

// NewBGPLSPrefixV4 creates a new IPv4 Prefix NLRI.
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

// Bytes returns the wire-format encoding.
func (p *BGPLSPrefix) Bytes() []byte {
	if p.cached != nil {
		return p.cached
	}

	localNodeTLV := tlv(TLVLocalNodeDesc, p.LocalNode.Bytes())
	prefixDescTLV := tlv(TLVPrefixDescriptors, p.PrefixDesc.Bytes())

	bodyLen := 9 + len(localNodeTLV) + len(prefixDescTLV)
	body := make([]byte, bodyLen)
	body[0] = byte(p.protocolID)
	binary.BigEndian.PutUint64(body[1:9], p.identifier)
	offset := 9
	copy(body[offset:], localNodeTLV)
	offset += len(localNodeTLV)
	copy(body[offset:], prefixDescTLV)

	p.cached = make([]byte, 4+len(body))
	binary.BigEndian.PutUint16(p.cached[0:2], uint16(p.nlriType))
	binary.BigEndian.PutUint16(p.cached[2:4], uint16(len(body)))
	copy(p.cached[4:], body)

	return p.cached
}

// Len returns the length in bytes.
func (p *BGPLSPrefix) Len() int { return len(p.Bytes()) }

// String returns a human-readable representation.
func (p *BGPLSPrefix) String() string {
	return fmt.Sprintf("bgp-ls:prefix(%s)", p.nlriType)
}

// ParseBGPLS parses a BGP-LS NLRI from wire format.
func ParseBGPLS(data []byte) (BGPLSNLRI, error) {
	if len(data) < 4 {
		return nil, ErrBGPLSTruncated
	}

	nlriType := BGPLSNLRIType(binary.BigEndian.Uint16(data[0:2]))
	nlriLen := int(binary.BigEndian.Uint16(data[2:4]))

	if len(data) < 4+nlriLen {
		return nil, ErrBGPLSTruncated
	}

	if nlriLen < 9 {
		return nil, ErrBGPLSTruncated
	}

	body := data[4 : 4+nlriLen]
	proto := BGPLSProtocolID(body[0])
	identifier := binary.BigEndian.Uint64(body[1:9])

	switch nlriType {
	case BGPLSNodeNLRI:
		node := &BGPLSNode{
			bgplsBase: bgplsBase{
				nlriType:   nlriType,
				protocolID: proto,
				identifier: identifier,
			},
		}
		// Parse local node descriptor TLVs
		if err := parseNodeDescriptorTLVs(body[9:], &node.LocalNode); err != nil {
			return nil, err
		}
		node.cached = data[:4+nlriLen]
		return node, nil

	case BGPLSLinkNLRI:
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
		prefix := &BGPLSPrefix{
			bgplsBase: bgplsBase{
				nlriType:   nlriType,
				protocolID: proto,
				identifier: identifier,
			},
		}
		prefix.cached = data[:4+nlriLen]
		return prefix, nil

	default:
		return nil, ErrBGPLSInvalidType
	}
}

// parseNodeDescriptorTLVs parses TLVs into a NodeDescriptor.
func parseNodeDescriptorTLVs(data []byte, nd *NodeDescriptor) error {
	for len(data) >= 4 {
		tlvType := binary.BigEndian.Uint16(data[0:2])
		tlvLen := int(binary.BigEndian.Uint16(data[2:4]))

		if len(data) < 4+tlvLen {
			return ErrBGPLSTruncated
		}

		value := data[4 : 4+tlvLen]

		// Check if this is the Local Node Descriptor container
		if tlvType == TLVLocalNodeDesc {
			return parseNodeDescriptorTLVs(value, nd)
		}

		switch tlvType {
		case TLVAutonomousSystem:
			if len(value) >= 4 {
				nd.ASN = binary.BigEndian.Uint32(value)
			}
		case TLVBGPLSIdentifier:
			if len(value) >= 4 {
				nd.BGPLSIdentifier = binary.BigEndian.Uint32(value)
			}
		case TLVOSPFAreaID:
			if len(value) >= 4 {
				nd.OSPFAreaID = binary.BigEndian.Uint32(value)
			}
		case TLVIGPRouterID:
			nd.IGPRouterID = make([]byte, len(value))
			copy(nd.IGPRouterID, value)
		}

		data = data[4+tlvLen:]
	}

	return nil
}

// Helper functions

func tlv(t uint16, v []byte) []byte {
	data := make([]byte, 4+len(v))
	binary.BigEndian.PutUint16(data[0:2], t)
	binary.BigEndian.PutUint16(data[2:4], uint16(len(v)))
	copy(data[4:], v)
	return data
}

func uint32ToBytes(v uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return b
}
