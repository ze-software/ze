// Package attribute implements BGP path attributes.
package attribute

import (
	"encoding/binary"
	"errors"
	"net/netip"
)

// Errors for MP NLRI parsing.
var (
	ErrInvalidNextHopLen = errors.New("attribute: invalid next-hop length")
	ErrUnsupportedAFI    = errors.New("attribute: unsupported AFI")
)

// AFI represents Address Family Identifier (RFC 4760).
type AFI uint16

// Address Family Identifiers.
const (
	AFIIPv4  AFI = 1
	AFIIPv6  AFI = 2
	AFIL2VPN AFI = 25
)

// SAFI represents Subsequent Address Family Identifier (RFC 4760).
type SAFI uint8

// Subsequent Address Family Identifiers.
const (
	SAFIUnicast   SAFI = 1
	SAFIMulticast SAFI = 2
	SAFIEVPN      SAFI = 70
	SAFIVPN       SAFI = 128
	SAFIFlowSpec  SAFI = 133
)

// MPReachNLRI represents the MP_REACH_NLRI attribute (RFC 4760).
//
// This attribute carries NLRI for address families other than IPv4 unicast.
// It includes the AFI/SAFI, next-hop address(es), and the NLRI itself.
type MPReachNLRI struct {
	AFI      AFI
	SAFI     SAFI
	NextHops []netip.Addr // One or more next-hop addresses
	NLRI     []byte       // Raw NLRI data (parsed separately per family)
}

// Code returns AttrMPReachNLRI.
func (m *MPReachNLRI) Code() AttributeCode { return AttrMPReachNLRI }

// Flags returns FlagOptional (MP_REACH_NLRI is optional non-transitive).
func (m *MPReachNLRI) Flags() AttributeFlags { return FlagOptional }

// Len returns the packed length in bytes.
func (m *MPReachNLRI) Len() int {
	nhLen := m.nextHopLen()
	// AFI(2) + SAFI(1) + NH_Len(1) + NextHops + Reserved(1) + NLRI
	return 2 + 1 + 1 + nhLen + 1 + len(m.NLRI)
}

// nextHopLen calculates the total next-hop length in bytes.
func (m *MPReachNLRI) nextHopLen() int {
	total := 0
	for _, nh := range m.NextHops {
		if nh.Is4() {
			total += 4
		} else {
			total += 16
		}
	}
	return total
}

// Pack serializes the MP_REACH_NLRI attribute value.
func (m *MPReachNLRI) Pack() []byte {
	nhLen := m.nextHopLen()
	buf := make([]byte, m.Len())

	// AFI (2 bytes)
	binary.BigEndian.PutUint16(buf[0:2], uint16(m.AFI))

	// SAFI (1 byte)
	buf[2] = byte(m.SAFI)

	// Next Hop Length (1 byte)
	buf[3] = byte(nhLen)

	// Next Hop(s)
	offset := 4
	for _, nh := range m.NextHops {
		nhBytes := nh.AsSlice()
		copy(buf[offset:], nhBytes)
		offset += len(nhBytes)
	}

	// Reserved (1 byte, must be 0)
	buf[offset] = 0
	offset++

	// NLRI
	copy(buf[offset:], m.NLRI)

	return buf
}

// ParseMPReachNLRI parses an MP_REACH_NLRI attribute value.
func ParseMPReachNLRI(data []byte) (*MPReachNLRI, error) {
	if len(data) < 5 { // AFI(2) + SAFI(1) + NH_Len(1) + Reserved(1) minimum
		return nil, ErrShortData
	}

	m := &MPReachNLRI{
		AFI:  AFI(binary.BigEndian.Uint16(data[0:2])),
		SAFI: SAFI(data[2]),
	}

	nhLen := int(data[3])
	if len(data) < 4+nhLen+1 { // +1 for reserved byte
		return nil, ErrShortData
	}

	// Parse next-hop(s)
	nhData := data[4 : 4+nhLen]
	nextHops, err := parseNextHops(m.AFI, nhData)
	if err != nil {
		return nil, err
	}
	m.NextHops = nextHops

	// Skip reserved byte
	nlriOffset := 4 + nhLen + 1

	// NLRI is the remainder
	if nlriOffset < len(data) {
		m.NLRI = make([]byte, len(data)-nlriOffset)
		copy(m.NLRI, data[nlriOffset:])
	}

	return m, nil
}

// parseNextHops parses next-hop address(es) based on AFI.
func parseNextHops(afi AFI, data []byte) ([]netip.Addr, error) {
	if len(data) == 0 {
		return nil, nil
	}

	var hops []netip.Addr

	switch afi {
	case AFIIPv4:
		// IPv4: 4 bytes per next-hop
		if len(data)%4 != 0 {
			return nil, ErrInvalidNextHopLen
		}
		for i := 0; i < len(data); i += 4 {
			var ip [4]byte
			copy(ip[:], data[i:i+4])
			hops = append(hops, netip.AddrFrom4(ip))
		}

	case AFIIPv6:
		// IPv6: 16 bytes per next-hop, or 32 bytes for global+link-local
		switch len(data) {
		case 16:
			var ip [16]byte
			copy(ip[:], data)
			hops = append(hops, netip.AddrFrom16(ip))
		case 32:
			// Global + link-local
			var ip1, ip2 [16]byte
			copy(ip1[:], data[0:16])
			copy(ip2[:], data[16:32])
			hops = append(hops, netip.AddrFrom16(ip1), netip.AddrFrom16(ip2))
		default:
			return nil, ErrInvalidNextHopLen
		}

	case AFIL2VPN:
		// L2VPN (EVPN): typically 4 or 16 bytes
		switch len(data) {
		case 4:
			var ip [4]byte
			copy(ip[:], data)
			hops = append(hops, netip.AddrFrom4(ip))
		case 16:
			var ip [16]byte
			copy(ip[:], data)
			hops = append(hops, netip.AddrFrom16(ip))
		default:
			return nil, ErrInvalidNextHopLen
		}

	default:
		// Unknown AFI - try to preserve the raw data as best we can
		// Return empty slice, the raw data is still in the attribute
		return nil, nil
	}

	return hops, nil
}

// MPUnreachNLRI represents the MP_UNREACH_NLRI attribute (RFC 4760).
//
// This attribute carries withdrawn NLRI for address families other than
// IPv4 unicast. It's simpler than MP_REACH_NLRI as there's no next-hop.
type MPUnreachNLRI struct {
	AFI  AFI
	SAFI SAFI
	NLRI []byte // Raw withdrawn NLRI data
}

// Code returns AttrMPUnreachNLRI.
func (m *MPUnreachNLRI) Code() AttributeCode { return AttrMPUnreachNLRI }

// Flags returns FlagOptional (MP_UNREACH_NLRI is optional non-transitive).
func (m *MPUnreachNLRI) Flags() AttributeFlags { return FlagOptional }

// Len returns the packed length in bytes.
func (m *MPUnreachNLRI) Len() int {
	// AFI(2) + SAFI(1) + Withdrawn NLRI
	return 2 + 1 + len(m.NLRI)
}

// Pack serializes the MP_UNREACH_NLRI attribute value.
func (m *MPUnreachNLRI) Pack() []byte {
	buf := make([]byte, m.Len())

	// AFI (2 bytes)
	binary.BigEndian.PutUint16(buf[0:2], uint16(m.AFI))

	// SAFI (1 byte)
	buf[2] = byte(m.SAFI)

	// Withdrawn NLRI
	copy(buf[3:], m.NLRI)

	return buf
}

// ParseMPUnreachNLRI parses an MP_UNREACH_NLRI attribute value.
func ParseMPUnreachNLRI(data []byte) (*MPUnreachNLRI, error) {
	if len(data) < 3 { // AFI(2) + SAFI(1) minimum
		return nil, ErrShortData
	}

	m := &MPUnreachNLRI{
		AFI:  AFI(binary.BigEndian.Uint16(data[0:2])),
		SAFI: SAFI(data[2]),
	}

	// Withdrawn NLRI is the remainder
	if len(data) > 3 {
		m.NLRI = make([]byte, len(data)-3)
		copy(m.NLRI, data[3:])
	}

	return m, nil
}

// IsEndOfRIB returns true if this MP_UNREACH_NLRI represents an End-of-RIB marker.
// End-of-RIB is signaled by an MP_UNREACH_NLRI with no withdrawn routes.
func (m *MPUnreachNLRI) IsEndOfRIB() bool {
	return len(m.NLRI) == 0
}

// NewMPUnreachEndOfRIB creates an End-of-RIB marker for the given family.
func NewMPUnreachEndOfRIB(afi AFI, safi SAFI) *MPUnreachNLRI {
	return &MPUnreachNLRI{
		AFI:  afi,
		SAFI: safi,
		NLRI: nil, // Empty NLRI signals End-of-RIB
	}
}
