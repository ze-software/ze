// Design: docs/architecture/wire/nlri.md — MVPN NLRI plugin
//
// Package bgp_mvpn implements Multicast VPN NLRI (RFC 6514, SAFI 5).
package bgp_nlri_mvpn

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
)

// Type aliases for shared nlri types.
type (
	Family             = nlri.Family
	AFI                = nlri.AFI
	SAFI               = nlri.SAFI
	NLRI               = nlri.NLRI
	RouteDistinguisher = nlri.RouteDistinguisher
)

// Re-export constants and variables.
const (
	AFIIPv4  = nlri.AFIIPv4
	AFIIPv6  = nlri.AFIIPv6
	SAFIMVPN = nlri.SAFIMVPN
	RDType0  = nlri.RDType0
	RDType1  = nlri.RDType1
)

var (
	IPv4MVPN = nlri.IPv4MVPN
	IPv6MVPN = nlri.IPv6MVPN
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
	cached    []byte
	cacheOnce sync.Once
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
		cached:    data[:2+nlriLen],
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
	if m.cached != nil {
		return m.cached
	}

	m.cacheOnce.Do(func() {
		totalData := m.buildData()
		m.cached = make([]byte, 2+len(totalData))
		m.cached[0] = byte(m.routeType)
		m.cached[1] = byte(len(totalData))
		copy(m.cached[2:], totalData)
	})

	return m.cached
}

// Len returns the length in bytes.
func (m *MVPN) Len() int { return len(m.Bytes()) }

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
	if m.cached != nil {
		return copy(buf[off:], m.cached)
	}

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

// buildData returns rd+data for Bytes() caching.
func (m *MVPN) buildData() []byte {
	if hasRD(m.rd) {
		rdBytes := make([]byte, 10)
		binary.BigEndian.PutUint16(rdBytes[:2], uint16(m.rd.Type))
		copy(rdBytes[2:8], m.rd.Value[:])
		return append(rdBytes[:8], m.data...)
	}
	result := make([]byte, len(m.data))
	copy(result, m.data)
	return result
}

// hasRD returns true if the RD is non-zero.
func hasRD(rd RouteDistinguisher) bool {
	return rd.Type != 0 || rd.Value != [6]byte{}
}
