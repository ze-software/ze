// Design: docs/architecture/wire/nlri.md — MVPN NLRI plugin
//
// Package bgp_mvpn implements Multicast VPN NLRI (RFC 6514, SAFI 5).
package mvpn

import (
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

// Re-export constants and variables.
const (
	AFIIPv4  = family.AFIIPv4
	AFIIPv6  = family.AFIIPv6
	SAFIMVPN = family.SAFIMVPN
	RDType0  = nlri.RDType0
	RDType1  = nlri.RDType1
)

// Family registrations for MVPN.
var (
	IPv4MVPN = family.MustRegister(AFIIPv4, SAFIMVPN, "ipv4", "mvpn")
	IPv6MVPN = family.MustRegister(AFIIPv6, SAFIMVPN, "ipv6", "mvpn")
)

// Errors for MVPN parsing.
var ErrMVPNTruncated = errors.New("mvpn: truncated data")

// MVPNRouteType identifies the type of MVPN route.
// RFC 6514 Section 4 defines the route types for MCAST-VPN NLRI.
type MVPNRouteType uint8

// MVPN route types per RFC 6514 Section 4.
const (
	MVPNIntraASIPMSIAD MVPNRouteType = 1 // Intra-AS I-PMSI A-D route.
	MVPNInterASIPMSIAD MVPNRouteType = 2 // Inter-AS I-PMSI A-D route.
	MVPNSPMSIAD        MVPNRouteType = 3 // S-PMSI A-D route.
	MVPNLeafAD         MVPNRouteType = 4 // Leaf A-D route.
	MVPNSourceActive   MVPNRouteType = 5 // Source Active A-D route.
	MVPNSharedTreeJoin MVPNRouteType = 6 // Shared Tree Join route (C-*,C-G).
	MVPNSourceTreeJoin MVPNRouteType = 7 // Source Tree Join route (C-S,C-G).
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
	rd        RouteDistinguisher
	data      []byte
	afi       AFI
	routeType MVPNRouteType
}

// NewMVPN creates a new MVPN NLRI.
func NewMVPN(routeType MVPNRouteType, data []byte) *MVPN {
	return &MVPN{
		data:      data,
		afi:       AFIIPv4,
		routeType: routeType,
	}
}

// NewMVPNWithRD creates a new MVPN NLRI with Route Distinguisher.
func NewMVPNWithRD(afi AFI, routeType MVPNRouteType, rd RouteDistinguisher, data []byte) *MVPN {
	return &MVPN{
		rd:        rd,
		data:      data,
		afi:       afi,
		routeType: routeType,
	}
}

// ParseMVPN parses an MVPN NLRI from wire format.
//
// RFC 6514 Section 4 defines the MCAST-VPN NLRI format:
//
//	Route Type (1 octet) + Length (1 octet) + Route Type specific (variable)
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
	}

	// Parse RD if present (all route types except default have RD)
	if nlriLen >= 8 {
		rd, err := nlri.ParseRouteDistinguisher(nlriData[:8])
		if err == nil {
			mvpn.rd = rd
			mvpn.data = nlriData[8:] // Zero-copy slice
		} else {
			mvpn.data = nlriData
		}
	} else {
		mvpn.data = nlriData
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
	buf := make([]byte, m.Len())
	m.WriteTo(buf, 0)
	return buf
}

// Len returns the length in bytes.
func (m *MVPN) Len() int {
	n := 2 + len(m.data)
	if hasRD(m.rd) {
		n += 8
	}
	return n
}

// PathID returns 0 (MVPN doesn't typically use ADD-PATH).
func (m *MVPN) PathID() uint32 { return 0 }

// HasPathID returns false.
func (m *MVPN) HasPathID() bool { return false }

// SupportsAddPath returns false - MVPN doesn't support ADD-PATH.
func (m *MVPN) SupportsAddPath() bool { return false }

// String returns command-style format for API round-trip compatibility.
func (m *MVPN) String() string {
	if hasRD(m.rd) {
		return fmt.Sprintf("%s rd %s", m.routeType, m.rd)
	}
	return m.routeType.String()
}

// WriteTo writes the MVPN NLRI directly to buf at offset.
func (m *MVPN) WriteTo(buf []byte, off int) int {
	pos := off

	dataLen := len(m.data)
	if hasRD(m.rd) {
		dataLen += 8
	}

	buf[pos] = byte(m.routeType)
	buf[pos+1] = byte(dataLen)
	pos += 2

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
