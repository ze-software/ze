// Design: docs/architecture/wire/nlri.md — MUP NLRI plugin
// RFC: rfc/short/draft-ietf-bess-mup-safi.md
//
// Package bgp_mup implements Mobile User Plane NLRI (draft-mpmz-bess-mup-safi, SAFI 85).
package mup

import (
	"encoding/binary"
	"errors"
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// Type aliases for shared nlri types.
type (
	Family             = family.Family
	AFI                = family.AFI
	SAFI               = family.SAFI
	NLRI               = nlri.NLRI
	RouteDistinguisher = nlri.RouteDistinguisher
)

// Re-export constants.
const (
	AFIIPv4 = family.AFIIPv4
	AFIIPv6 = family.AFIIPv6
	SAFIMUP = family.SAFIMUP
	RDType0 = nlri.RDType0
	RDType1 = nlri.RDType1
)

// Family registrations for MUP.
var (
	IPv4MUP = family.MustRegister(AFIIPv4, SAFIMUP, "ipv4", "mup")
	IPv6MUP = family.MustRegister(AFIIPv6, SAFIMUP, "ipv6", "mup")
)

var ParseRDString = nlri.ParseRDString

// Errors for MUP parsing.
var (
	ErrMUPTruncated   = errors.New("mup: truncated data")
	ErrMUPInvalidType = errors.New("mup: invalid route type")
)

// MUPRouteType identifies the type of MUP route.
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
type MUPArchType uint8

// MUP architecture types per draft-mpmz-bess-mup-safi.
const (
	MUPArch3GPP5G MUPArchType = 1 // 3GPP 5G architecture
)

// MUP represents a Mobile User Plane NLRI (draft-mpmz-bess-mup-safi).
type MUP struct {
	rd        RouteDistinguisher
	data      []byte
	afi       AFI
	archType  MUPArchType
	routeType MUPRouteType
}

// NewMUP creates a new MUP NLRI.
func NewMUP(routeType MUPRouteType, data []byte) *MUP {
	return &MUP{
		data:      data,
		afi:       AFIIPv4,
		archType:  MUPArch3GPP5G,
		routeType: routeType,
	}
}

// NewMUPFull creates a MUP NLRI with all fields.
func NewMUPFull(afi AFI, archType MUPArchType, routeType MUPRouteType, rd RouteDistinguisher, data []byte) *MUP {
	return &MUP{
		rd:        rd,
		data:      data,
		afi:       afi,
		archType:  archType,
		routeType: routeType,
	}
}

// ParseMUP parses a MUP NLRI from wire format.
//
// draft-mpmz-bess-mup-safi defines the BGP-MUP NLRI format:
//
//	Architecture Type (1 octet) + Route Type (2 octets) + Length (1 octet) + data
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
	}

	nlriData := data[4 : 4+nlriLen]

	if nlriLen >= 8 {
		rd, err := nlri.ParseRouteDistinguisher(nlriData[:8])
		if err == nil {
			mup.rd = rd
			mup.data = nlriData[8:]
		} else {
			mup.data = nlriData
		}
	} else {
		mup.data = nlriData
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

// Bytes allocates a standalone slice and delegates to WriteTo; hot-path
// senders should call WriteTo directly with a pool buffer.
func (m *MUP) Bytes() []byte {
	buf := make([]byte, m.Len())
	m.WriteTo(buf, 0)
	return buf
}

// Len returns the wire-format length in bytes.
//
// 4-byte MUP header (arch + route type + length) + optional 8-byte RD +
// route-type-specific data carried in m.data.
func (m *MUP) Len() int {
	n := 4 + len(m.data)
	if hasRD(m.rd) {
		n += 8
	}
	return n
}

// PathID returns 0.
func (m *MUP) PathID() uint32 { return 0 }

// HasPathID returns false.
func (m *MUP) HasPathID() bool { return false }

// SupportsAddPath returns false - MUP doesn't support ADD-PATH.
func (m *MUP) SupportsAddPath() bool { return false }

// String returns command-style format for API round-trip compatibility.
func (m *MUP) String() string {
	if hasRD(m.rd) {
		return fmt.Sprintf("%s rd %s", m.routeType, m.rd)
	}
	return m.routeType.String()
}

// WriteTo writes the MUP NLRI directly to buf at offset. Zero-alloc:
// RD is written in place via RouteDistinguisher.WriteTo.
func (m *MUP) WriteTo(buf []byte, off int) int {
	pos := off

	dataLen := len(m.data)
	if hasRD(m.rd) {
		dataLen += 8
	}

	buf[pos] = byte(m.archType)
	binary.BigEndian.PutUint16(buf[pos+1:], uint16(m.routeType))
	buf[pos+3] = byte(dataLen & 0xFF)
	pos += 4

	if hasRD(m.rd) {
		pos += m.rd.WriteTo(buf, pos)
	}

	copy(buf[pos:], m.data)
	pos += len(m.data)

	return pos - off
}

// hasRD returns true if the RD is non-zero.
func hasRD(rd RouteDistinguisher) bool {
	return rd.Type != 0 || rd.Value != [6]byte{}
}
