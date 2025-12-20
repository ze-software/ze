package nlri

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Additional SAFI values for specialized NLRI types.
const (
	SAFIMVPN        SAFI = 5   // Multicast VPN
	SAFIVPLS        SAFI = 65  // VPLS
	SAFIMUP         SAFI = 85  // Mobile User Plane
	SAFIRTC         SAFI = 132 // Route Target Constraint
	SAFIFlowSpecVPN SAFI = 134 // FlowSpec VPN (RFC 5575 Section 6)
)

// Common address families for P2 NLRI types.
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

// Errors for P2 NLRI parsing.
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

// MVPN represents a Multicast VPN NLRI (RFC 6514).
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
// MVPN format: route-type (1) + length (1) + route-type-specific data.
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

// VPLS represents a VPLS NLRI (RFC 4761).
type VPLS struct {
	rd            RouteDistinguisher
	veID          uint16 // VE ID
	veBlockOffset uint16
	veBlockSize   uint16
	labelBase     uint32 // 20-bit label
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
// VPLS format: length (2) + RD (8) + VE ID (2) + VE Block Offset (2) + VE Block Size (2) + Label Base (3).
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
func (v *VPLS) Bytes() []byte {
	if v.cached != nil {
		return v.cached
	}

	// NLRI length (2 bytes) + RD (8 bytes) + VE ID (2 bytes) + VE Block Offset (2 bytes) +
	// VE Block Size (2 bytes) + Label Base (3 bytes) = 19 bytes total
	v.cached = make([]byte, 19)
	binary.BigEndian.PutUint16(v.cached[0:2], 17) // length excludes length field

	// RD
	copy(v.cached[2:10], v.rd.Bytes())

	// VE ID
	binary.BigEndian.PutUint16(v.cached[10:12], v.veID)

	// VE Block Offset
	binary.BigEndian.PutUint16(v.cached[12:14], v.veBlockOffset)

	// VE Block Size
	binary.BigEndian.PutUint16(v.cached[14:16], v.veBlockSize)

	// Label Base (3 bytes, 20-bit label with BOS)
	v.cached[16] = byte(v.labelBase >> 12)
	v.cached[17] = byte(v.labelBase >> 4)
	v.cached[18] = byte(v.labelBase<<4) | 0x01 // BOS bit

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

// RouteTarget represents a Route Target extended community.
type RouteTarget struct {
	Type  uint16
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
func (rt RouteTarget) String() string {
	switch rt.Type >> 8 { // High byte indicates type
	case 0x00: // 2-byte ASN
		asn := binary.BigEndian.Uint16(rt.Value[:2])
		assigned := binary.BigEndian.Uint32(rt.Value[2:6])
		return fmt.Sprintf("%d:%d", asn, assigned)
	case 0x01: // 4-byte IP
		ip := fmt.Sprintf("%d.%d.%d.%d", rt.Value[0], rt.Value[1], rt.Value[2], rt.Value[3])
		assigned := binary.BigEndian.Uint16(rt.Value[4:6])
		return fmt.Sprintf("%s:%d", ip, assigned)
	case 0x02: // 4-byte ASN
		asn := binary.BigEndian.Uint32(rt.Value[:4])
		assigned := binary.BigEndian.Uint16(rt.Value[4:6])
		return fmt.Sprintf("%d:%d", asn, assigned)
	default:
		return fmt.Sprintf("rt-type%d:%x", rt.Type, rt.Value)
	}
}

// RTC represents a Route Target Constraint NLRI (RFC 4684).
type RTC struct {
	originAS    uint32
	routeTarget RouteTarget
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
// RTC format: prefix-length (1 byte) + [origin-as (4 bytes) + route-target (8 bytes)].
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
func (r *RTC) IsDefault() bool {
	return r.originAS == 0 && r.routeTarget.Type == 0 && r.routeTarget.Value == [6]byte{}
}

// Bytes returns the wire-format encoding.
func (r *RTC) Bytes() []byte {
	if r.cached != nil {
		return r.cached
	}

	// Default route (matches all RTs)
	if r.IsDefault() {
		r.cached = []byte{0}
		return r.cached
	}

	// Full RTC: prefix-length + origin-as (4) + route-target (8) = 13 bytes
	r.cached = make([]byte, 13)
	r.cached[0] = 96 // 12 bytes * 8 bits

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

// MUPArchType identifies the MUP architecture type.
type MUPArchType uint8

// MUP architecture types.
const (
	MUPArch3GPP5G MUPArchType = 1 // 3GPP 5G
)

// MUP represents a Mobile User Plane NLRI.
type MUP struct {
	afi       AFI
	archType  MUPArchType
	routeType MUPRouteType
	rd        RouteDistinguisher
	data      []byte
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
// MUP format: arch-type (1) + route-type (2) + length (1) + route-type-specific data.
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
