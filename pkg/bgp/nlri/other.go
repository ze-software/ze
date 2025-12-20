// Package nlri provides NLRI (Network Layer Reachability Information) types.
//
// This file implements specialized NLRI types for various BGP extensions:
//   - MVPN: Multicast VPN (RFC 6514)
//   - VPLS: Virtual Private LAN Service (RFC 4761)
//   - RTC: Route Target Constraint (RFC 4684)
//   - MUP: Mobile User Plane (draft-mpmz-bess-mup-safi)
//   - FlowSpec VPN: Flow Specification for VPN (RFC 8955)
package nlri

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Additional SAFI values for specialized NLRI types.
//
// RFC 4760 Section 3 defines the SAFI (Subsequent Address Family Identifier)
// as a one-octet field that provides additional information about the type
// of NLRI being carried.
//
// SAFI allocations are maintained by IANA in the "Subsequent Address Family
// Identifiers (SAFI) Parameters" registry.
const (
	SAFIMVPN        SAFI = 5   // Multicast VPN - RFC 6514
	SAFIVPLS        SAFI = 65  // VPLS - RFC 4761 Section 3.2.2
	SAFIMUP         SAFI = 85  // Mobile User Plane - draft-mpmz-bess-mup-safi
	SAFIRTC         SAFI = 132 // Route Target Constraint - RFC 4684 Section 4
	SAFIFlowSpecVPN SAFI = 134 // FlowSpec VPN - RFC 8955 (obsoletes RFC 5575)
)

// Common address families for specialized NLRI types.
//
// These combine AFI and SAFI values to identify specific BGP address families:
//   - MVPN uses AFI 1 (IPv4) or 2 (IPv6) with SAFI 5 (RFC 6514)
//   - VPLS uses AFI 25 (L2VPN) with SAFI 65 (RFC 4761)
//   - RTC uses AFI 1 (IPv4) with SAFI 132 (RFC 4684)
//   - MUP uses AFI 1 or 2 with SAFI 85 (draft-mpmz-bess-mup-safi)
//   - FlowSpec VPN uses AFI 1 or 2 with SAFI 134 (RFC 8955)
var (
	IPv4MVPN        = Family{AFI: AFIIPv4, SAFI: SAFIMVPN}
	IPv6MVPN        = Family{AFI: AFIIPv6, SAFI: SAFIMVPN}
	L2VPNVPLS       = Family{AFI: AFIL2VPN, SAFI: SAFIVPLS}
	IPv4RTC         = Family{AFI: AFIIPv4, SAFI: SAFIRTC}
	IPv4MUP         = Family{AFI: AFIIPv4, SAFI: SAFIMUP}
	IPv6MUP         = Family{AFI: AFIIPv6, SAFI: SAFIMUP}
	IPv4FlowSpecVPN = Family{AFI: AFIIPv4, SAFI: SAFIFlowSpecVPN}
	IPv6FlowSpecVPN = Family{AFI: AFIIPv6, SAFI: SAFIFlowSpecVPN}
)

// Errors for specialized NLRI parsing.
var (
	ErrMVPNTruncated  = errors.New("mvpn: truncated data")
	ErrVPLSTruncated  = errors.New("vpls: truncated data")
	ErrRTCTruncated   = errors.New("rtc: truncated data")
	ErrMUPTruncated   = errors.New("mup: truncated data")
	ErrMUPInvalidType = errors.New("mup: invalid route type")
)

// ============================================================================
// MVPN - Multicast VPN (RFC 6514)
// ============================================================================
//
// RFC 6514 defines BGP encodings and procedures for Multicast in MPLS/BGP
// IP VPNs. It introduces the MCAST-VPN NLRI for exchanging multicast VPN
// routing information.
//
// MCAST-VPN NLRI Format (RFC 6514 Section 4):
//
//	+-----------------------------------+
//	| Route Type (1 octet)              |
//	+-----------------------------------+
//	| Length (1 octet)                  |
//	+-----------------------------------+
//	| Route Type specific (variable)    |
//	+-----------------------------------+
//
// The Route Type field determines the encoding of the rest of the NLRI.
// Most route types include a Route Distinguisher (RD) as the first 8 bytes.
// ============================================================================

// MVPNRouteType identifies the type of MVPN route.
// RFC 6514 Section 4 defines the route types for MCAST-VPN NLRI.
type MVPNRouteType uint8

// MVPN route types per RFC 6514 Section 4.
//
// A-D routes (Auto-Discovery) are used for MVPN autodiscovery and
// binding MVPN to P-Multicast Service Interface (PMSI) tunnels.
// C-multicast routes carry customer multicast routing information.
const (
	// A-D (Auto-Discovery) route types - RFC 6514 Section 4.1-4.5
	MVPNIntraASIPMSIAD MVPNRouteType = 1 // Intra-AS I-PMSI A-D route
	MVPNInterASIPMSIAD MVPNRouteType = 2 // Inter-AS I-PMSI A-D route
	MVPNSPMSIAD        MVPNRouteType = 3 // S-PMSI A-D route
	MVPNLeafAD         MVPNRouteType = 4 // Leaf A-D route
	MVPNSourceActive   MVPNRouteType = 5 // Source Active A-D route

	// C-multicast route types - RFC 6514 Section 4.6-4.7
	MVPNSharedTreeJoin MVPNRouteType = 6 // Shared Tree Join route (C-*,C-G)
	MVPNSourceTreeJoin MVPNRouteType = 7 // Source Tree Join route (C-S,C-G)
)

// String returns a human-readable route type name.
func (t MVPNRouteType) String() string {
	switch t {
	case MVPNIntraASIPMSIAD:
		return "intra-as-i-pmsi-ad"
	case MVPNInterASIPMSIAD:
		return "inter-as-i-pmsi-ad"
	case MVPNSPMSIAD:
		return "s-pmsi-ad"
	case MVPNLeafAD:
		return "leaf-ad"
	case MVPNSourceActive:
		return "source-active"
	case MVPNSharedTreeJoin:
		return "shared-tree-join"
	case MVPNSourceTreeJoin:
		return "source-tree-join"
	default:
		return fmt.Sprintf("type(%d)", t)
	}
}

// MVPN represents a Multicast VPN NLRI (RFC 6514).
//
// RFC 6514 Section 4 defines the MCAST-VPN NLRI structure. Each route type
// has its own specific format, but most include a Route Distinguisher.
type MVPN struct {
	afi       AFI
	routeType MVPNRouteType
	rd        RouteDistinguisher
	data      []byte // Route-type specific data after RD
	cached    []byte
}

// NewMVPN creates a new MVPN NLRI.
func NewMVPN(routeType MVPNRouteType, data []byte) *MVPN {
	return &MVPN{
		afi:       AFIIPv4,
		routeType: routeType,
		data:      data,
	}
}

// NewMVPNWithRD creates a new MVPN NLRI with Route Distinguisher.
func NewMVPNWithRD(afi AFI, routeType MVPNRouteType, rd RouteDistinguisher, data []byte) *MVPN {
	return &MVPN{
		afi:       afi,
		routeType: routeType,
		rd:        rd,
		data:      data,
	}
}

// ParseMVPN parses an MVPN NLRI from wire format.
//
// RFC 6514 Section 4 defines the MCAST-VPN NLRI format:
//
//	+-----------------------------------+
//	| Route Type (1 octet)              |
//	+-----------------------------------+
//	| Length (1 octet)                  |
//	+-----------------------------------+
//	| Route Type specific (variable)    |
//	+-----------------------------------+
//
// The Length field indicates the length in octets of the Route Type
// specific field. Most route types include an 8-byte Route Distinguisher
// as the first part of the route-type-specific data.
func ParseMVPN(afi AFI, data []byte) (*MVPN, []byte, error) {
	if len(data) < 2 {
		return nil, nil, ErrMVPNTruncated
	}

	routeType := MVPNRouteType(data[0])
	nlriLen := int(data[1])

	if len(data) < 2+nlriLen {
		return nil, nil, ErrMVPNTruncated
	}

	nlriData := data[2 : 2+nlriLen]

	mvpn := &MVPN{
		afi:       afi,
		routeType: routeType,
		cached:    data[:2+nlriLen],
	}

	// Parse RD if present (all route types except default have RD)
	if nlriLen >= 8 {
		rd, err := ParseRouteDistinguisher(nlriData[:8])
		if err == nil {
			mvpn.rd = rd
			mvpn.data = make([]byte, len(nlriData)-8)
			copy(mvpn.data, nlriData[8:])
		} else {
			mvpn.data = make([]byte, len(nlriData))
			copy(mvpn.data, nlriData)
		}
	} else {
		mvpn.data = make([]byte, len(nlriData))
		copy(mvpn.data, nlriData)
	}

	return mvpn, data[2+nlriLen:], nil
}

// Family returns the address family.
func (m *MVPN) Family() Family {
	return Family{AFI: m.afi, SAFI: SAFIMVPN}
}

// RouteType returns the MVPN route type.
func (m *MVPN) RouteType() MVPNRouteType { return m.routeType }

// RD returns the Route Distinguisher.
func (m *MVPN) RD() RouteDistinguisher { return m.rd }

// Bytes returns the wire-format encoding.
func (m *MVPN) Bytes() []byte {
	if m.cached != nil {
		return m.cached
	}

	// Calculate total data length
	var totalData []byte
	if m.rd.Type != 0 || m.rd.Value != [6]byte{} {
		totalData = append(m.rd.Bytes(), m.data...)
	} else {
		totalData = m.data
	}

	m.cached = make([]byte, 2+len(totalData))
	m.cached[0] = byte(m.routeType)
	m.cached[1] = byte(len(totalData))
	copy(m.cached[2:], totalData)

	return m.cached
}

// Len returns the length in bytes.
func (m *MVPN) Len() int { return len(m.Bytes()) }

// PathID returns 0 (MVPN doesn't typically use ADD-PATH).
func (m *MVPN) PathID() uint32 { return 0 }

// HasPathID returns false.
func (m *MVPN) HasPathID() bool { return false }

// String returns a human-readable representation.
func (m *MVPN) String() string {
	if m.rd.Type != 0 || m.rd.Value != [6]byte{} {
		return fmt.Sprintf("mvpn:%s rd=%s", m.routeType, m.rd)
	}
	return fmt.Sprintf("mvpn:%s", m.routeType)
}

// ============================================================================
// VPLS - Virtual Private LAN Service (RFC 4761)
// ============================================================================
//
// RFC 4761 defines Virtual Private LAN Service (VPLS) using BGP for
// Auto-Discovery and Signaling. VPLS provides a multipoint Layer 2 VPN
// service that appears as an Ethernet LAN to customers.
//
// VPLS BGP NLRI Format (RFC 4761 Section 3.2.2):
//
//	+------------------------------------+
//	|  Length (2 octets)                 |
//	+------------------------------------+
//	|  Route Distinguisher  (8 octets)   |
//	+------------------------------------+
//	|  VE ID (2 octets)                  |
//	+------------------------------------+
//	|  VE Block Offset (2 octets)        |
//	+------------------------------------+
//	|  VE Block Size (2 octets)          |
//	+------------------------------------+
//	|  Label Base (3 octets)             |
//	+------------------------------------+
//
// The AFI is L2VPN (25), and the SAFI is VPLS (65).
// ============================================================================

// VPLS represents a VPLS NLRI (RFC 4761 Section 3.2.2).
//
// A VPLS BGP NLRI contains information for establishing pseudowires
// between VPLS Edge (VE) devices. The label block mechanism allows a PE
// to send a single Update message that contains demultiplexors for all
// remote PEs, reducing control plane load.
type VPLS struct {
	rd            RouteDistinguisher
	veID          uint16 // VE ID - unique identifier within a VPLS
	veBlockOffset uint16 // Starting VE ID for the label block
	veBlockSize   uint16 // Number of labels in the block
	labelBase     uint32 // 20-bit MPLS label base
	cached        []byte
}

// NewVPLS creates a new VPLS NLRI.
func NewVPLS(rd RouteDistinguisher, veBlockOffset, veBlockSize uint16, labelBase []byte) *VPLS {
	var label uint32
	if len(labelBase) >= 3 {
		label = uint32(labelBase[0])<<12 | uint32(labelBase[1])<<4 | uint32(labelBase[2]>>4)
	}
	return &VPLS{
		rd:            rd,
		veBlockOffset: veBlockOffset,
		veBlockSize:   veBlockSize,
		labelBase:     label,
	}
}

// NewVPLSFull creates a VPLS NLRI with all fields.
func NewVPLSFull(rd RouteDistinguisher, veID, veBlockOffset, veBlockSize uint16, labelBase uint32) *VPLS {
	return &VPLS{
		rd:            rd,
		veID:          veID,
		veBlockOffset: veBlockOffset,
		veBlockSize:   veBlockSize,
		labelBase:     labelBase,
	}
}

// ParseVPLS parses a VPLS NLRI from wire format.
//
// RFC 4761 Section 3.2.2 defines the VPLS NLRI format:
//
//	+------------------------------------+
//	|  Length (2 octets)                 |  <- Length of remaining fields
//	+------------------------------------+
//	|  Route Distinguisher (8 octets)    |
//	+------------------------------------+
//	|  VE ID (2 octets)                  |
//	+------------------------------------+
//	|  VE Block Offset (2 octets)        |
//	+------------------------------------+
//	|  VE Block Size (2 octets)          |
//	+------------------------------------+
//	|  Label Base (3 octets)             |  <- 20-bit label + BOS bit
//	+------------------------------------+
//
// The minimum NLRI length is 17 bytes (8+2+2+2+3).
func ParseVPLS(data []byte) (*VPLS, []byte, error) {
	if len(data) < 2 {
		return nil, nil, ErrVPLSTruncated
	}

	nlriLen := int(binary.BigEndian.Uint16(data[:2]))
	if len(data) < 2+nlriLen {
		return nil, nil, ErrVPLSTruncated
	}

	// Minimum VPLS NLRI is 17 bytes (8 RD + 2 VE ID + 2 offset + 2 size + 3 label)
	if nlriLen < 17 {
		return nil, nil, ErrVPLSTruncated
	}

	nlriData := data[2 : 2+nlriLen]

	rd, err := ParseRouteDistinguisher(nlriData[:8])
	if err != nil {
		return nil, nil, err
	}

	vpls := &VPLS{
		rd:            rd,
		veID:          binary.BigEndian.Uint16(nlriData[8:10]),
		veBlockOffset: binary.BigEndian.Uint16(nlriData[10:12]),
		veBlockSize:   binary.BigEndian.Uint16(nlriData[12:14]),
		cached:        data[:2+nlriLen],
	}

	// Parse label base (3 bytes -> 20-bit label)
	if nlriLen >= 17 {
		vpls.labelBase = uint32(nlriData[14])<<12 | uint32(nlriData[15])<<4 | uint32(nlriData[16]>>4)
	}

	return vpls, data[2+nlriLen:], nil
}

// Family returns the address family.
func (v *VPLS) Family() Family {
	return Family{AFI: AFIL2VPN, SAFI: SAFIVPLS}
}

// RD returns the route distinguisher.
func (v *VPLS) RD() RouteDistinguisher { return v.rd }

// VEID returns the VE ID.
func (v *VPLS) VEID() uint16 { return v.veID }

// VEBlockOffset returns the VE block offset.
func (v *VPLS) VEBlockOffset() uint16 { return v.veBlockOffset }

// VEBlockSize returns the VE block size.
func (v *VPLS) VEBlockSize() uint16 { return v.veBlockSize }

// LabelBase returns the label base value.
func (v *VPLS) LabelBase() uint32 { return v.labelBase }

// Bytes returns the wire-format encoding.
//
// RFC 4761 Section 3.2.2: The Label Base is encoded as a 3-octet MPLS label
// with the Bottom of Stack (BOS) bit set to 1.
func (v *VPLS) Bytes() []byte {
	if v.cached != nil {
		return v.cached
	}

	// RFC 4761 Section 3.2.2:
	// NLRI length (2 bytes) + RD (8 bytes) + VE ID (2 bytes) + VE Block Offset (2 bytes) +
	// VE Block Size (2 bytes) + Label Base (3 bytes) = 19 bytes total
	v.cached = make([]byte, 19)
	binary.BigEndian.PutUint16(v.cached[0:2], 17) // length excludes length field

	// Route Distinguisher (RFC 4364)
	copy(v.cached[2:10], v.rd.Bytes())

	// VE ID - unique identifier for this VE within the VPLS
	binary.BigEndian.PutUint16(v.cached[10:12], v.veID)

	// VE Block Offset - starting VE ID for label block
	binary.BigEndian.PutUint16(v.cached[12:14], v.veBlockOffset)

	// VE Block Size - number of labels in block
	binary.BigEndian.PutUint16(v.cached[14:16], v.veBlockSize)

	// Label Base (3 bytes, 20-bit label with BOS bit)
	// RFC 3032: MPLS label format is 20-bit label + 3-bit TC + 1-bit BOS + 8-bit TTL
	// For signaling, only label and BOS are used
	v.cached[16] = byte(v.labelBase >> 12)
	v.cached[17] = byte(v.labelBase >> 4)
	v.cached[18] = byte(v.labelBase<<4) | 0x01 // BOS bit set to 1

	return v.cached
}

// Len returns the length in bytes.
func (v *VPLS) Len() int { return len(v.Bytes()) }

// PathID returns 0.
func (v *VPLS) PathID() uint32 { return 0 }

// HasPathID returns false.
func (v *VPLS) HasPathID() bool { return false }

// String returns a human-readable representation.
func (v *VPLS) String() string {
	return fmt.Sprintf("vpls:%s ve=%d label=%d", v.rd, v.veID, v.labelBase)
}

// ============================================================================
// RTC - Route Target Constraint (RFC 4684)
// ============================================================================
//
// RFC 4684 defines Route Target Membership NLRI for constrained distribution
// of VPN routing information. This allows BGP speakers to advertise which
// Route Targets they are interested in, enabling efficient VPN route
// distribution.
//
// Route Target Membership NLRI Format (RFC 4684 Section 4):
//
//	+-------------------------------+
//	| origin as        (4 octets)   |
//	+-------------------------------+
//	| route target     (8 octets)   |
//	+                               +
//	|                               |
//	+-------------------------------+
//
// The NLRI is encoded as a prefix of 0 to 96 bits. A zero-length prefix
// represents the default route target, indicating willingness to receive
// all VPN route advertisements.
//
// The AFI is IPv4 (1), and the SAFI is RTC (132).
// ============================================================================

// RouteTarget represents a Route Target extended community.
//
// RFC 4360 defines extended communities as 8-octet values. Route Targets
// are a type of extended community used to control VPN route distribution.
type RouteTarget struct {
	Type  uint16  // Extended community type (2 bytes)
	Value [6]byte // Extended community value (6 bytes)
}

// Bytes returns the wire format of the route target (8 bytes).
func (rt RouteTarget) Bytes() []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint16(buf[:2], rt.Type)
	copy(buf[2:], rt.Value[:])
	return buf
}

// String returns a human-readable route target.
//
// RFC 4360 Section 3 defines extended community types. The high-order octet
// of the Type field determines the format:
//   - 0x00: Two-octet AS specific (2-byte ASN + 4-byte local admin)
//   - 0x01: IPv4 address specific (4-byte IP + 2-byte local admin)
//   - 0x02: Four-octet AS specific (4-byte ASN + 2-byte local admin)
func (rt RouteTarget) String() string {
	switch rt.Type >> 8 { // High byte indicates type
	case 0x00: // 2-byte ASN (RFC 4360 Section 3.1)
		asn := binary.BigEndian.Uint16(rt.Value[:2])
		assigned := binary.BigEndian.Uint32(rt.Value[2:6])
		return fmt.Sprintf("%d:%d", asn, assigned)
	case 0x01: // IPv4 address (RFC 4360 Section 3.2)
		ip := fmt.Sprintf("%d.%d.%d.%d", rt.Value[0], rt.Value[1], rt.Value[2], rt.Value[3])
		assigned := binary.BigEndian.Uint16(rt.Value[4:6])
		return fmt.Sprintf("%s:%d", ip, assigned)
	case 0x02: // 4-byte ASN (RFC 5668)
		asn := binary.BigEndian.Uint32(rt.Value[:4])
		assigned := binary.BigEndian.Uint16(rt.Value[4:6])
		return fmt.Sprintf("%d:%d", asn, assigned)
	default:
		return fmt.Sprintf("rt-type%d:%x", rt.Type, rt.Value)
	}
}

// RTC represents a Route Target Constraint NLRI (RFC 4684 Section 4).
//
// The RTC NLRI is used to advertise interest in receiving VPN routes
// with specific Route Targets. This enables constrained route distribution,
// reducing the amount of VPN routing information that needs to be
// maintained by route reflectors and ASBRs.
type RTC struct {
	originAS    uint32      // Origin AS number (4 bytes)
	routeTarget RouteTarget // Route Target extended community (8 bytes)
	cached      []byte
}

// NewRTC creates a new RTC NLRI.
func NewRTC(originAS uint32, rt RouteTarget) *RTC {
	return &RTC{
		originAS:    originAS,
		routeTarget: rt,
	}
}

// ParseRTC parses an RTC NLRI from wire format.
//
// RFC 4684 Section 4 defines the RTC NLRI format as a prefix of 0 to 96 bits:
//
//	+-------------------------------+
//	| prefix-length (1 octet)       |
//	+-------------------------------+
//	| origin as      (4 octets)     |  <- if prefix-length >= 32
//	+-------------------------------+
//	| route target   (8 octets)     |  <- if prefix-length > 32
//	+                               +
//	|                               |
//	+-------------------------------+
//
// A prefix-length of 0 represents the default route target, indicating
// willingness to receive all VPN route advertisements.
// The minimum prefix-length (except for default) is 32 bits.
func ParseRTC(data []byte) (*RTC, []byte, error) {
	if len(data) < 1 {
		return nil, nil, ErrRTCTruncated
	}

	prefixLen := int(data[0])
	prefixBytes := (prefixLen + 7) / 8

	if len(data) < 1+prefixBytes {
		return nil, nil, ErrRTCTruncated
	}

	rtc := &RTC{
		cached: data[:1+prefixBytes],
	}

	// Default route (prefix-length = 0)
	if prefixLen == 0 {
		return rtc, data[1:], nil
	}

	// Parse origin AS (4 bytes)
	if prefixBytes >= 4 {
		rtc.originAS = binary.BigEndian.Uint32(data[1:5])
	}

	// Parse route target (up to 8 bytes)
	if prefixBytes >= 6 {
		rtc.routeTarget.Type = binary.BigEndian.Uint16(data[5:7])
	}
	if prefixBytes >= 12 {
		copy(rtc.routeTarget.Value[:], data[7:13])
	} else if prefixBytes > 6 {
		copy(rtc.routeTarget.Value[:prefixBytes-6], data[7:1+prefixBytes])
	}

	return rtc, data[1+prefixBytes:], nil
}

// Family returns the address family.
func (r *RTC) Family() Family {
	return Family{AFI: AFIIPv4, SAFI: SAFIRTC}
}

// OriginAS returns the origin AS number.
func (r *RTC) OriginAS() uint32 { return r.originAS }

// RouteTarget returns the route target.
func (r *RTC) RouteTarget() RouteTarget { return r.routeTarget }

// IsDefault returns true if this is the default RTC (matches all RTs).
//
// RFC 4684 Section 4: A zero-length prefix (prefix-length = 0) represents
// the default route target, indicating willingness to receive all VPN
// route advertisements, such as from a route reflector to its PE clients.
func (r *RTC) IsDefault() bool {
	return r.originAS == 0 && r.routeTarget.Type == 0 && r.routeTarget.Value == [6]byte{}
}

// Bytes returns the wire-format encoding.
//
// RFC 4684 Section 4: The NLRI is encoded as a prefix per RFC 4760 Section 5.
// The prefix-length is in bits: 96 bits = 12 bytes (4 origin-as + 8 route-target).
func (r *RTC) Bytes() []byte {
	if r.cached != nil {
		return r.cached
	}

	// Default route (matches all RTs) - RFC 4684 Section 4
	if r.IsDefault() {
		r.cached = []byte{0}
		return r.cached
	}

	// Full RTC: prefix-length + origin-as (4) + route-target (8) = 13 bytes
	// RFC 4684 Section 4: prefix is 96 bits (12 bytes * 8 bits/byte)
	r.cached = make([]byte, 13)
	r.cached[0] = 96 // prefix-length in bits

	binary.BigEndian.PutUint32(r.cached[1:5], r.originAS)
	binary.BigEndian.PutUint16(r.cached[5:7], r.routeTarget.Type)
	copy(r.cached[7:13], r.routeTarget.Value[:])

	return r.cached
}

// Len returns the length in bytes.
func (r *RTC) Len() int { return len(r.Bytes()) }

// PathID returns 0.
func (r *RTC) PathID() uint32 { return 0 }

// HasPathID returns false.
func (r *RTC) HasPathID() bool { return false }

// String returns a human-readable representation.
func (r *RTC) String() string {
	if r.IsDefault() {
		return "rtc:default"
	}
	return fmt.Sprintf("rtc:as%d:%s", r.originAS, r.routeTarget)
}

// ============================================================================
// MUP - Mobile User Plane (draft-mpmz-bess-mup-safi)
// ============================================================================
//
// The BGP-MUP SAFI enables BGP-based signaling for Mobile User Plane (MUP)
// architectures, particularly for 5G mobile networks using SRv6.
//
// BGP-MUP NLRI Format (draft-mpmz-bess-mup-safi):
//
//	+-----------------------------------+
//	| Architecture Type (1 octet)       |
//	+-----------------------------------+
//	| Route Type (2 octets)             |
//	+-----------------------------------+
//	| Length (1 octet)                  |
//	+-----------------------------------+
//	| Route Type specific (variable)    |
//	+-----------------------------------+
//
// The Architecture Type field determines the mobile network architecture.
// Currently defined: 1 = 3GPP 5G.
// ============================================================================

// MUPRouteType identifies the type of MUP route.
// draft-mpmz-bess-mup-safi defines route types for different MUP functions.
type MUPRouteType uint16

// MUP route types per draft-mpmz-bess-mup-safi.
const (
	MUPISD  MUPRouteType = 1 // Interwork Segment Discovery route
	MUPDSD  MUPRouteType = 2 // Direct Segment Discovery route
	MUPT1ST MUPRouteType = 3 // Type 1 Session Transformed route
	MUPT2ST MUPRouteType = 4 // Type 2 Session Transformed route
)

// String returns a human-readable route type name.
func (t MUPRouteType) String() string {
	switch t {
	case MUPISD:
		return "isd"
	case MUPDSD:
		return "dsd"
	case MUPT1ST:
		return "t1st"
	case MUPT2ST:
		return "t2st"
	default:
		return fmt.Sprintf("type(%d)", t)
	}
}

// MUPArchType identifies the MUP architecture type.
// draft-mpmz-bess-mup-safi defines architecture types for different mobile networks.
type MUPArchType uint8

// MUP architecture types per draft-mpmz-bess-mup-safi.
const (
	MUPArch3GPP5G MUPArchType = 1 // 3GPP 5G architecture
)

// MUP represents a Mobile User Plane NLRI (draft-mpmz-bess-mup-safi).
//
// MUP enables SRv6-based mobile user plane signaling through BGP.
// Each route type carries specific information for mobile network functions.
type MUP struct {
	afi       AFI
	archType  MUPArchType
	routeType MUPRouteType
	rd        RouteDistinguisher
	data      []byte // Route-type specific data after RD
	cached    []byte
}

// NewMUP creates a new MUP NLRI.
func NewMUP(routeType MUPRouteType, data []byte) *MUP {
	return &MUP{
		afi:       AFIIPv4,
		archType:  MUPArch3GPP5G,
		routeType: routeType,
		data:      data,
	}
}

// NewMUPFull creates a MUP NLRI with all fields.
func NewMUPFull(afi AFI, archType MUPArchType, routeType MUPRouteType, rd RouteDistinguisher, data []byte) *MUP {
	return &MUP{
		afi:       afi,
		archType:  archType,
		routeType: routeType,
		rd:        rd,
		data:      data,
	}
}

// ParseMUP parses a MUP NLRI from wire format.
//
// draft-mpmz-bess-mup-safi defines the BGP-MUP NLRI format:
//
//	+-----------------------------------+
//	| Architecture Type (1 octet)       |
//	+-----------------------------------+
//	| Route Type (2 octets)             |
//	+-----------------------------------+
//	| Length (1 octet)                  |
//	+-----------------------------------+
//	| Route Type specific (variable)    |
//	+-----------------------------------+
//
// Most route types include an 8-byte Route Distinguisher as the first
// part of the route-type-specific data.
func ParseMUP(afi AFI, data []byte) (*MUP, []byte, error) {
	if len(data) < 4 {
		return nil, nil, ErrMUPTruncated
	}

	archType := MUPArchType(data[0])
	routeType := MUPRouteType(binary.BigEndian.Uint16(data[1:3]))
	nlriLen := int(data[3])

	if len(data) < 4+nlriLen {
		return nil, nil, ErrMUPTruncated
	}

	mup := &MUP{
		afi:       afi,
		archType:  archType,
		routeType: routeType,
		cached:    data[:4+nlriLen],
	}

	nlriData := data[4 : 4+nlriLen]

	// Parse RD if present (most route types have RD)
	if nlriLen >= 8 {
		rd, err := ParseRouteDistinguisher(nlriData[:8])
		if err == nil {
			mup.rd = rd
			mup.data = make([]byte, len(nlriData)-8)
			copy(mup.data, nlriData[8:])
		} else {
			mup.data = make([]byte, len(nlriData))
			copy(mup.data, nlriData)
		}
	} else {
		mup.data = make([]byte, len(nlriData))
		copy(mup.data, nlriData)
	}

	return mup, data[4+nlriLen:], nil
}

// Family returns the address family.
func (m *MUP) Family() Family {
	return Family{AFI: m.afi, SAFI: SAFIMUP}
}

// ArchType returns the MUP architecture type.
func (m *MUP) ArchType() MUPArchType { return m.archType }

// RouteType returns the MUP route type.
func (m *MUP) RouteType() MUPRouteType { return m.routeType }

// RD returns the Route Distinguisher.
func (m *MUP) RD() RouteDistinguisher { return m.rd }

// Bytes returns the wire-format encoding.
func (m *MUP) Bytes() []byte {
	if m.cached != nil {
		return m.cached
	}

	// Calculate total data length
	var totalData []byte
	if m.rd.Type != 0 || m.rd.Value != [6]byte{} {
		totalData = append(m.rd.Bytes(), m.data...)
	} else {
		totalData = m.data
	}

	m.cached = make([]byte, 4+len(totalData))
	m.cached[0] = byte(m.archType)
	binary.BigEndian.PutUint16(m.cached[1:3], uint16(m.routeType))
	m.cached[3] = byte(len(totalData))
	copy(m.cached[4:], totalData)

	return m.cached
}

// Len returns the length in bytes.
func (m *MUP) Len() int { return len(m.Bytes()) }

// PathID returns 0.
func (m *MUP) PathID() uint32 { return 0 }

// HasPathID returns false.
func (m *MUP) HasPathID() bool { return false }

// String returns a human-readable representation.
func (m *MUP) String() string {
	if m.rd.Type != 0 || m.rd.Value != [6]byte{} {
		return fmt.Sprintf("mup:%s rd=%s", m.routeType, m.rd)
	}
	return fmt.Sprintf("mup:%s", m.routeType)
}
