package nlri

import (
	"encoding/binary"
	"fmt"
)

// Additional SAFI values for specialized NLRI types.
const (
	SAFIMVPN SAFI = 5   // Multicast VPN
	SAFIVPLS SAFI = 65  // VPLS
	SAFIMUP  SAFI = 85  // Mobile User Plane
	SAFIRTC  SAFI = 132 // Route Target Constraint
)

// ============================================================================
// MVPN - Multicast VPN (RFC 6514)
// ============================================================================

// MVPNRouteType identifies the type of MVPN route.
type MVPNRouteType uint8

// MVPN route types (RFC 6514 Section 4).
const (
	MVPNIntraASIPMSIAD MVPNRouteType = 1 // Intra-AS I-PMSI A-D
	MVPNInterASIPMSIAD MVPNRouteType = 2 // Inter-AS I-PMSI A-D
	MVPNSPMSIAD        MVPNRouteType = 3 // S-PMSI A-D
	MVPNLeafAD         MVPNRouteType = 4 // Leaf A-D
	MVPNSourceActive   MVPNRouteType = 5 // Source Active A-D
	MVPNSharedTreeJoin MVPNRouteType = 6 // Shared Tree Join
	MVPNSourceTreeJoin MVPNRouteType = 7 // Source Tree Join
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

// MVPN represents a Multicast VPN NLRI.
type MVPN struct {
	routeType MVPNRouteType
	data      []byte // Route-type specific data
}

// NewMVPN creates a new MVPN NLRI.
func NewMVPN(routeType MVPNRouteType, data []byte) *MVPN {
	return &MVPN{
		routeType: routeType,
		data:      data,
	}
}

// Family returns the address family.
func (m *MVPN) Family() Family {
	return Family{AFI: AFIIPv4, SAFI: SAFIMVPN}
}

// RouteType returns the MVPN route type.
func (m *MVPN) RouteType() MVPNRouteType { return m.routeType }

// Bytes returns the wire-format encoding.
func (m *MVPN) Bytes() []byte {
	data := make([]byte, 1+len(m.data))
	data[0] = byte(m.routeType)
	copy(data[1:], m.data)
	return data
}

// Len returns the length in bytes.
func (m *MVPN) Len() int { return len(m.Bytes()) }

// PathID returns 0 (MVPN doesn't typically use ADD-PATH).
func (m *MVPN) PathID() uint32 { return 0 }

// HasPathID returns false.
func (m *MVPN) HasPathID() bool { return false }

// String returns a human-readable representation.
func (m *MVPN) String() string {
	return fmt.Sprintf("mvpn:%s", m.routeType)
}

// ============================================================================
// VPLS - Virtual Private LAN Service (RFC 4761)
// ============================================================================

// VPLS represents a VPLS NLRI.
type VPLS struct {
	rd            RouteDistinguisher
	veBlockOffset uint16
	veBlockSize   uint16
	labelBase     []byte
}

// NewVPLS creates a new VPLS NLRI.
func NewVPLS(rd RouteDistinguisher, veBlockOffset, veBlockSize uint16, labelBase []byte) *VPLS {
	return &VPLS{
		rd:            rd,
		veBlockOffset: veBlockOffset,
		veBlockSize:   veBlockSize,
		labelBase:     labelBase,
	}
}

// Family returns the address family.
func (v *VPLS) Family() Family {
	return Family{AFI: AFIL2VPN, SAFI: SAFIVPLS}
}

// RD returns the route distinguisher.
func (v *VPLS) RD() RouteDistinguisher { return v.rd }

// VEBlockOffset returns the VE block offset.
func (v *VPLS) VEBlockOffset() uint16 { return v.veBlockOffset }

// VEBlockSize returns the VE block size.
func (v *VPLS) VEBlockSize() uint16 { return v.veBlockSize }

// Bytes returns the wire-format encoding.
func (v *VPLS) Bytes() []byte {
	// NLRI length (2 bytes) + RD (8 bytes) + VE ID (2 bytes) + VE Block Offset (2 bytes) +
	// VE Block Size (2 bytes) + Label Base (3 bytes)
	data := make([]byte, 2+8+2+2+2+len(v.labelBase))
	binary.BigEndian.PutUint16(data[0:2], uint16(len(data)-2)) //nolint:gosec // Fixed size struct

	// RD
	rdBytes := v.rd.Bytes()
	copy(data[2:10], rdBytes)

	// VE ID (typically 0)
	binary.BigEndian.PutUint16(data[10:12], 0)

	// VE Block Offset
	binary.BigEndian.PutUint16(data[12:14], v.veBlockOffset)

	// VE Block Size
	binary.BigEndian.PutUint16(data[14:16], v.veBlockSize)

	// Label Base
	copy(data[16:], v.labelBase)

	return data
}

// Len returns the length in bytes.
func (v *VPLS) Len() int { return len(v.Bytes()) }

// PathID returns 0.
func (v *VPLS) PathID() uint32 { return 0 }

// HasPathID returns false.
func (v *VPLS) HasPathID() bool { return false }

// String returns a human-readable representation.
func (v *VPLS) String() string {
	return fmt.Sprintf("vpls:%s", v.rd)
}

// ============================================================================
// RTC - Route Target Constraint (RFC 4684)
// ============================================================================

// RouteTarget represents a Route Target extended community.
type RouteTarget struct {
	Type  uint16
	Value []byte
}

// RTC represents a Route Target Constraint NLRI.
type RTC struct {
	originAS    uint32
	routeTarget RouteTarget
}

// NewRTC creates a new RTC NLRI.
func NewRTC(originAS uint32, rt RouteTarget) *RTC {
	return &RTC{
		originAS:    originAS,
		routeTarget: rt,
	}
}

// Family returns the address family.
func (r *RTC) Family() Family {
	return Family{AFI: AFIIPv4, SAFI: SAFIRTC}
}

// OriginAS returns the origin AS number.
func (r *RTC) OriginAS() uint32 { return r.originAS }

// RouteTarget returns the route target.
func (r *RTC) RouteTarget() RouteTarget { return r.routeTarget }

// Bytes returns the wire-format encoding.
func (r *RTC) Bytes() []byte {
	// Format: prefix-length (1 byte) + origin-as (4 bytes) + route-target (8 bytes)
	// But prefix-length can vary based on RT presence
	if len(r.routeTarget.Value) == 0 {
		// Default route (matches all RTs)
		return []byte{0}
	}

	data := make([]byte, 1+4+2+len(r.routeTarget.Value))
	data[0] = byte((4 + 2 + len(r.routeTarget.Value)) * 8) // prefix length in bits

	binary.BigEndian.PutUint32(data[1:5], r.originAS)
	binary.BigEndian.PutUint16(data[5:7], r.routeTarget.Type)
	copy(data[7:], r.routeTarget.Value)

	return data
}

// Len returns the length in bytes.
func (r *RTC) Len() int { return len(r.Bytes()) }

// PathID returns 0.
func (r *RTC) PathID() uint32 { return 0 }

// HasPathID returns false.
func (r *RTC) HasPathID() bool { return false }

// String returns a human-readable representation.
func (r *RTC) String() string {
	return fmt.Sprintf("rtc:as%d", r.originAS)
}

// ============================================================================
// MUP - Mobile User Plane (draft-ietf-bess-bgp-mup-safi)
// ============================================================================

// MUPRouteType identifies the type of MUP route.
type MUPRouteType uint16

// MUP route types.
const (
	MUPISD  MUPRouteType = 1 // Interwork Segment Discovery
	MUPDSD  MUPRouteType = 2 // Direct Segment Discovery
	MUPT1ST MUPRouteType = 3 // Type 1 Session Transformed
	MUPT2ST MUPRouteType = 4 // Type 2 Session Transformed
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

// MUP represents a Mobile User Plane NLRI.
type MUP struct {
	routeType MUPRouteType
	data      []byte
}

// NewMUP creates a new MUP NLRI.
func NewMUP(routeType MUPRouteType, data []byte) *MUP {
	return &MUP{
		routeType: routeType,
		data:      data,
	}
}

// Family returns the address family.
func (m *MUP) Family() Family {
	return Family{AFI: AFIIPv4, SAFI: SAFIMUP}
}

// RouteType returns the MUP route type.
func (m *MUP) RouteType() MUPRouteType { return m.routeType }

// Bytes returns the wire-format encoding.
func (m *MUP) Bytes() []byte {
	data := make([]byte, 3+len(m.data))
	data[0] = 0 // Architecture type (placeholder)
	binary.BigEndian.PutUint16(data[1:3], uint16(m.routeType))
	copy(data[3:], m.data)
	return data
}

// Len returns the length in bytes.
func (m *MUP) Len() int { return len(m.Bytes()) }

// PathID returns 0.
func (m *MUP) PathID() uint32 { return 0 }

// HasPathID returns false.
func (m *MUP) HasPathID() bool { return false }

// String returns a human-readable representation.
func (m *MUP) String() string {
	return fmt.Sprintf("mup:%s", m.routeType)
}
