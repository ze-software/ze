package api

import (
	"encoding/binary"
	"net/netip"

	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
)

// MPReachWire wraps MP_REACH_NLRI attribute bytes for zero-copy lazy parsing.
// RFC 4760 Section 3: AFI(2) + SAFI(1) + NH_Len(1) + NextHop + Reserved(1) + NLRI
//
// This is a view into the original wire bytes - do not modify.
type MPReachWire []byte

// AFI returns the Address Family Identifier (2 octets at offset 0).
// Returns 0 if data is too short.
func (m MPReachWire) AFI() uint16 {
	if len(m) < 2 {
		return 0
	}
	return binary.BigEndian.Uint16(m[0:2])
}

// SAFI returns the Subsequent Address Family Identifier (1 octet at offset 2).
// Returns 0 if data is too short.
func (m MPReachWire) SAFI() uint8 {
	if len(m) < 3 {
		return 0
	}
	return m[2]
}

// Family returns the combined AFI/SAFI as an nlri.Family.
func (m MPReachWire) Family() nlri.Family {
	return nlri.Family{
		AFI:  nlri.AFI(m.AFI()),
		SAFI: nlri.SAFI(m.SAFI()),
	}
}

// NextHop returns the first next-hop address from the attribute.
// RFC 4760 Section 3: Next Hop Network Address field.
// Returns invalid Addr if data is malformed or too short.
func (m MPReachWire) NextHop() netip.Addr {
	if len(m) < 4 {
		return netip.Addr{}
	}

	nhLen := int(m[3])
	if len(m) < 4+nhLen {
		return netip.Addr{}
	}

	nhBytes := m[4 : 4+nhLen]

	// Parse based on AFI and NH length
	afi := m.AFI()
	switch {
	case afi == 1 && nhLen >= 4:
		// IPv4: 4 bytes
		var addr [4]byte
		copy(addr[:], nhBytes[:4])
		return netip.AddrFrom4(addr)
	case afi == 2 && nhLen >= 16:
		// IPv6: 16 bytes (may have link-local after, take first)
		var addr [16]byte
		copy(addr[:], nhBytes[:16])
		return netip.AddrFrom16(addr)
	default:
		return netip.Addr{}
	}
}

// Prefixes parses and returns all NLRI prefixes from the attribute.
// RFC 4760 Section 3: NLRI field follows NextHop + Reserved byte.
// Returns nil if data is malformed.
func (m MPReachWire) Prefixes() []netip.Prefix {
	if len(m) < 4 {
		return nil
	}

	nhLen := int(m[3])
	// NLRI starts after: AFI(2) + SAFI(1) + NHLen(1) + NextHop(nhLen) + Reserved(1)
	nlriOffset := 4 + nhLen + 1
	if nlriOffset > len(m) {
		return nil
	}

	nlriBytes := m[nlriOffset:]
	afi := m.AFI()

	switch afi {
	case 1: // IPv4
		return parseIPv4Prefixes(nlriBytes)
	case 2: // IPv6
		return parseIPv6Prefixes(nlriBytes)
	default:
		return nil
	}
}

// MPUnreachWire wraps MP_UNREACH_NLRI attribute bytes for zero-copy lazy parsing.
// RFC 4760 Section 4: AFI(2) + SAFI(1) + Withdrawn Routes
//
// This is a view into the original wire bytes - do not modify.
type MPUnreachWire []byte

// AFI returns the Address Family Identifier (2 octets at offset 0).
// Returns 0 if data is too short.
func (m MPUnreachWire) AFI() uint16 {
	if len(m) < 2 {
		return 0
	}
	return binary.BigEndian.Uint16(m[0:2])
}

// SAFI returns the Subsequent Address Family Identifier (1 octet at offset 2).
// Returns 0 if data is too short.
func (m MPUnreachWire) SAFI() uint8 {
	if len(m) < 3 {
		return 0
	}
	return m[2]
}

// Family returns the combined AFI/SAFI as an nlri.Family.
func (m MPUnreachWire) Family() nlri.Family {
	return nlri.Family{
		AFI:  nlri.AFI(m.AFI()),
		SAFI: nlri.SAFI(m.SAFI()),
	}
}

// Prefixes parses and returns all withdrawn prefixes from the attribute.
// RFC 4760 Section 4: Withdrawn Routes field follows AFI + SAFI.
// Returns nil if data is malformed.
func (m MPUnreachWire) Prefixes() []netip.Prefix {
	if len(m) < 3 {
		return nil
	}

	// Withdrawn routes start after AFI(2) + SAFI(1)
	withdrawnBytes := m[3:]
	afi := m.AFI()

	switch afi {
	case 1: // IPv4
		return parseIPv4Prefixes(withdrawnBytes)
	case 2: // IPv6
		return parseIPv6Prefixes(withdrawnBytes)
	default:
		return nil
	}
}

// IPv4Reach holds zero-copy slices into UPDATE body for legacy IPv4 unicast.
// RFC 4271: IPv4 unicast uses body structure, not MP attributes.
type IPv4Reach struct {
	nh   []byte // slice to NEXT_HOP attribute value (4 bytes)
	nlri []byte // slice to body NLRI section
}

// NextHop returns the next-hop address from the NEXT_HOP attribute.
// Returns invalid Addr if nh is nil or wrong size.
func (r IPv4Reach) NextHop() netip.Addr {
	if len(r.nh) < 4 {
		return netip.Addr{}
	}
	var addr [4]byte
	copy(addr[:], r.nh[:4])
	return netip.AddrFrom4(addr)
}

// Prefixes parses and returns all IPv4 prefixes from the NLRI section.
// Returns nil if nlri is nil or empty.
func (r IPv4Reach) Prefixes() []netip.Prefix {
	if len(r.nlri) == 0 {
		return nil
	}
	return parseIPv4Prefixes(r.nlri)
}

// IPv4Withdraw holds zero-copy slice into UPDATE body for withdrawn routes.
type IPv4Withdraw struct {
	withdrawn []byte // slice to body withdrawn section
}

// Prefixes parses and returns all withdrawn IPv4 prefixes.
func (w IPv4Withdraw) Prefixes() []netip.Prefix {
	if len(w.withdrawn) == 0 {
		return nil
	}
	return parseIPv4Prefixes(w.withdrawn)
}
