// Design: docs/architecture/wire/nlri.md — MUP NLRI plugin
// RFC: rfc/short/draft-ietf-bess-mup-safi.md
//
// Package bgp_mup implements Mobile User Plane NLRI (draft-mpmz-bess-mup-safi, SAFI 85).
package bgp_nlri_mup

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/nlri"
)

// Type aliases for shared nlri types.
type (
	Family             = nlri.Family
	AFI                = nlri.AFI
	SAFI               = nlri.SAFI
	NLRI               = nlri.NLRI
	RouteDistinguisher = nlri.RouteDistinguisher
)

// Re-export constants.
const (
	AFIIPv4 = nlri.AFIIPv4
	AFIIPv6 = nlri.AFIIPv6
	SAFIMUP = nlri.SAFIMUP
	RDType0 = nlri.RDType0
	RDType1 = nlri.RDType1
)

var (
	IPv4MUP       = nlri.IPv4MUP
	IPv6MUP       = nlri.IPv6MUP
	ParseRDString = nlri.ParseRDString
)

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
	cached    []byte
	cacheOnce sync.Once
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
		cached:    data[:4+nlriLen],
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

// Bytes returns the wire-format encoding.
func (m *MUP) Bytes() []byte {
	if m.cached != nil {
		return m.cached
	}

	m.cacheOnce.Do(func() {
		totalData := m.buildData()
		m.cached = make([]byte, 4+len(totalData))
		m.cached[0] = byte(m.archType)
		binary.BigEndian.PutUint16(m.cached[1:3], uint16(m.routeType))
		m.cached[3] = byte(len(totalData) & 0xFF)
		copy(m.cached[4:], totalData)
	})

	return m.cached
}

// Len returns the length in bytes.
func (m *MUP) Len() int { return len(m.Bytes()) }

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

// WriteTo writes the MUP NLRI directly to buf at offset.
func (m *MUP) WriteTo(buf []byte, off int) int {
	if m.cached != nil {
		return copy(buf[off:], m.cached)
	}

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

// buildData returns rd+data for Bytes() caching.
func (m *MUP) buildData() []byte {
	if hasRD(m.rd) {
		return append(m.rd.Bytes(), m.data...)
	}
	result := make([]byte, len(m.data))
	copy(result, m.data)
	return result
}

// hasRD returns true if the RD is non-zero.
func hasRD(rd RouteDistinguisher) bool {
	return rd.Type != 0 || rd.Value != [6]byte{}
}
