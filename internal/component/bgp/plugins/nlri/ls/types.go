// Design: docs/architecture/wire/nlri-bgpls.md — BGP-LS NLRI plugin
// RFC: rfc/short/rfc7752.md
// Detail: types_descriptor.go — node, link, and prefix descriptor TLV encoding
// Detail: types_nlri.go — concrete NLRI types (Node, Link, Prefix)
// Detail: types_srv6.go — SRv6 SID NLRI and descriptor (RFC 9514)
// Detail: attr.go — BGP-LS attribute TLV framework (type 29)
//
// Package bgp_ls implements BGP-LS family types and plugin for ze.
// RFC 7752: North-Bound Distribution of Link-State and TE Information Using BGP
// RFC 9085: BGP-LS Extensions for Segment Routing
// RFC 9514: BGP-LS Extensions for SRv6
package ls

import (
	"encoding/binary"
	"errors"
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// Type aliases for nlri types used by BGP-LS.
type (
	Family = family.Family
	AFI    = family.AFI
	SAFI   = family.SAFI
	NLRI   = nlri.NLRI
)

// Re-export constants from nlri for local use.
const (
	AFIBGPLS            = family.AFIBGPLS
	SAFIBGPLinkState    = family.SAFIBGPLinkState
	SAFIBGPLinkStateVPN = family.SAFIBGPLinkStateVPN
)

// Family registrations for BGP-LS.
var (
	BGPLSFamily    = family.MustRegister(AFIBGPLS, SAFIBGPLinkState, "bgp-ls", "bgp-ls")
	BGPLSVPNFamily = family.MustRegister(AFIBGPLS, SAFIBGPLinkStateVPN, "bgp-ls", "bgp-ls-vpn")
)

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
	default: // format unknown NLRI types numerically
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
	default: // format unknown protocols numerically
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
	TLVBGPRouterID      uint16 = 516 // BGP Router-ID (RFC 9086 Section 4.1)
	TLVConfedMember     uint16 = 517 // BGP Confederation Member (RFC 9086 Section 4.2)
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

// cachedBytes returns the original wire bytes if ParseBGPLS set them, or nil
// for programmatically-constructed NLRIs. Used by the AppendJSON fast path
// (json.go) to skip a fresh WriteTo allocation on every encode.
func (b *bgplsBase) cachedBytes() []byte { return b.cached }

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

	default: // unknown NLRI type
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
		// Unwrap container by continuing iteration on its contents (avoids recursion).
		if tlvType == TLVLocalNodeDesc {
			data = value
			continue
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
		case TLVBGPRouterID: // TLV 516 - 4 bytes (RFC 9086 Section 4.1)
			if len(value) >= 4 {
				nd.BGPRouterID = binary.BigEndian.Uint32(value)
			}
		case TLVConfedMember: // TLV 517 - 4 bytes (RFC 9086 Section 4.2)
			if len(value) >= 4 {
				nd.ConfedMember = binary.BigEndian.Uint32(value)
			}
		case TLVSRv6SID: // TLV 518 - 16 bytes (RFC 9514)
			sid := make([]byte, len(value))
			copy(sid, value)
			nd.SRv6SIDs = append(nd.SRv6SIDs, sid)
		}

		data = data[4+tlvLen:]
	}

	return nil
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
