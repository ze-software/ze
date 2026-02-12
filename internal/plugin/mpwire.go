package plugin

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
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
//
// Note: This method does NOT preserve ADD-PATH path-id. Use NLRIs() instead.
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

// NLRIs parses and returns all NLRIs from the attribute, preserving path-id.
// RFC 7911 Section 3: When hasAddPath is true, each NLRI is prefixed with 4-byte path-id.
// Returns error if wire bytes are malformed.
func (m MPReachWire) NLRIs(hasAddPath bool) ([]nlri.NLRI, error) {
	if len(m) < 5 {
		return nil, fmt.Errorf("MP_REACH_NLRI too short: %d bytes", len(m))
	}

	nhLen := int(m[3])
	// NLRI starts after: AFI(2) + SAFI(1) + NHLen(1) + NextHop(nhLen) + Reserved(1)
	nlriOffset := 4 + nhLen + 1
	if nlriOffset > len(m) {
		return nil, fmt.Errorf("invalid next-hop length: %d", nhLen)
	}

	nlriBytes := m[nlriOffset:]
	family := m.Family()

	return parseNLRIs(nlriBytes, family, hasAddPath)
}

// NLRIIterator returns a zero-allocation iterator over the NLRI section.
// RFC 7911 Section 3: When addPath is true, each NLRI is prefixed with 4-byte path-id.
// Returns nil if data is malformed or empty.
//
// Use this for zero-copy iteration instead of NLRIs() which allocates a slice.
func (m MPReachWire) NLRIIterator(addPath bool) *nlri.NLRIIterator {
	if len(m) < 5 {
		return nil
	}

	nhLen := int(m[3])
	// NLRI starts after: AFI(2) + SAFI(1) + NHLen(1) + NextHop(nhLen) + Reserved(1)
	nlriOffset := 4 + nhLen + 1
	if nlriOffset > len(m) {
		return nil
	}

	nlriBytes := m[nlriOffset:]
	if len(nlriBytes) == 0 {
		return nil
	}

	return nlri.NewNLRIIterator(nlriBytes, addPath)
}

// NLRIBytes returns the raw NLRI bytes without parsing.
// Returns the bytes after NextHop + Reserved, or nil if malformed.
// Use for raw wire byte extraction (pool storage).
func (m MPReachWire) NLRIBytes() []byte {
	if len(m) < 5 {
		return nil
	}

	nhLen := int(m[3])
	// NLRI starts after: AFI(2) + SAFI(1) + NHLen(1) + NextHop(nhLen) + Reserved(1)
	nlriOffset := 4 + nhLen + 1
	if nlriOffset > len(m) {
		return nil
	}

	return m[nlriOffset:]
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
//
// Note: This method does NOT preserve ADD-PATH path-id. Use NLRIs() instead.
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

// NLRIs parses and returns all withdrawn NLRIs, preserving path-id.
// RFC 7911 Section 3: When hasAddPath is true, each NLRI is prefixed with 4-byte path-id.
// Returns error if wire bytes are malformed.
func (m MPUnreachWire) NLRIs(hasAddPath bool) ([]nlri.NLRI, error) {
	if len(m) < 3 {
		return nil, fmt.Errorf("MP_UNREACH_NLRI too short: %d bytes", len(m))
	}

	// Withdrawn routes start after AFI(2) + SAFI(1)
	withdrawnBytes := m[3:]
	family := m.Family()

	return parseNLRIs(withdrawnBytes, family, hasAddPath)
}

// NLRIIterator returns a zero-allocation iterator over the withdrawn NLRI section.
// RFC 7911 Section 3: When addPath is true, each NLRI is prefixed with 4-byte path-id.
// Returns nil if data is malformed or empty.
//
// Use this for zero-copy iteration instead of NLRIs() which allocates a slice.
func (m MPUnreachWire) NLRIIterator(addPath bool) *nlri.NLRIIterator {
	if len(m) < 3 {
		return nil
	}

	// Withdrawn routes start after AFI(2) + SAFI(1)
	withdrawnBytes := m[3:]
	if len(withdrawnBytes) == 0 {
		return nil
	}

	return nlri.NewNLRIIterator(withdrawnBytes, addPath)
}

// WithdrawnBytes returns the raw withdrawn NLRI bytes without parsing.
// Returns the bytes after AFI(2) + SAFI(1), or nil if malformed.
// Use for raw wire byte extraction (pool storage).
func (m MPUnreachWire) WithdrawnBytes() []byte {
	if len(m) < 3 {
		return nil
	}
	return m[3:]
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
//
// Note: This method does NOT preserve ADD-PATH path-id. Use NLRIs() instead.
func (r IPv4Reach) Prefixes() []netip.Prefix {
	if len(r.nlri) == 0 {
		return nil
	}
	return parseIPv4Prefixes(r.nlri)
}

// NLRIs parses and returns all NLRIs, preserving path-id.
// RFC 7911 Section 3: When hasAddPath is true, each NLRI is prefixed with 4-byte path-id.
// Returns error if bytes are malformed.
func (r IPv4Reach) NLRIs(hasAddPath bool) ([]nlri.NLRI, error) {
	if len(r.nlri) == 0 {
		return nil, nil
	}
	return parseNLRIs(r.nlri, nlri.IPv4Unicast, hasAddPath)
}

// NLRIIterator returns a zero-allocation iterator over the NLRI section.
// Returns nil if nlri is empty.
func (r IPv4Reach) NLRIIterator(addPath bool) *nlri.NLRIIterator {
	if len(r.nlri) == 0 {
		return nil
	}
	return nlri.NewNLRIIterator(r.nlri, addPath)
}

// IPv4Withdraw holds zero-copy slice into UPDATE body for withdrawn routes.
type IPv4Withdraw struct {
	withdrawn []byte // slice to body withdrawn section
}

// Prefixes parses and returns all withdrawn IPv4 prefixes.
//
// Note: This method does NOT preserve ADD-PATH path-id. Use NLRIs() instead.
func (w IPv4Withdraw) Prefixes() []netip.Prefix {
	if len(w.withdrawn) == 0 {
		return nil
	}
	return parseIPv4Prefixes(w.withdrawn)
}

// NLRIs parses and returns all withdrawn NLRIs, preserving path-id.
// RFC 7911 Section 3: When hasAddPath is true, each NLRI is prefixed with 4-byte path-id.
// Returns error if bytes are malformed.
func (w IPv4Withdraw) NLRIs(hasAddPath bool) ([]nlri.NLRI, error) {
	if len(w.withdrawn) == 0 {
		return nil, nil
	}
	return parseNLRIs(w.withdrawn, nlri.IPv4Unicast, hasAddPath)
}

// NLRIIterator returns a zero-allocation iterator over the withdrawn section.
// Returns nil if withdrawn is empty.
func (w IPv4Withdraw) NLRIIterator(addPath bool) *nlri.NLRIIterator {
	if len(w.withdrawn) == 0 {
		return nil
	}
	return nlri.NewNLRIIterator(w.withdrawn, addPath)
}

// parseNLRIs parses a sequence of NLRIs using the nlri package.
// RFC 7911 Section 3: When hasAddPath is true, each NLRI is prefixed with 4-byte path-id.
// Supports IPv4/IPv6 unicast/multicast. Other families return error.
func parseNLRIs(data []byte, family nlri.Family, hasAddPath bool) ([]nlri.NLRI, error) {
	var result []nlri.NLRI
	originalLen := len(data)

	for len(data) > 0 {
		offset := originalLen - len(data)
		var n nlri.NLRI
		var rest []byte
		var err error

		switch {
		case family.AFI == nlri.AFIIPv4 && family.SAFI == nlri.SAFIUnicast:
			n, rest, err = nlri.ParseINET(nlri.AFIIPv4, nlri.SAFIUnicast, data, hasAddPath)
		case family.AFI == nlri.AFIIPv6 && family.SAFI == nlri.SAFIUnicast:
			n, rest, err = nlri.ParseINET(nlri.AFIIPv6, nlri.SAFIUnicast, data, hasAddPath)
		case family.AFI == nlri.AFIIPv4 && family.SAFI == nlri.SAFIMulticast:
			n, rest, err = nlri.ParseINET(nlri.AFIIPv4, nlri.SAFIMulticast, data, hasAddPath)
		case family.AFI == nlri.AFIIPv6 && family.SAFI == nlri.SAFIMulticast:
			n, rest, err = nlri.ParseINET(nlri.AFIIPv6, nlri.SAFIMulticast, data, hasAddPath)
		default:
			// For other families, return what we have so far
			// TODO: Add support for VPN, EVPN, FlowSpec, etc.
			return result, fmt.Errorf("unsupported family for NLRI parsing: %s", family)
		}

		if err != nil {
			return result, fmt.Errorf("parsing NLRI at offset %d: %w", offset, err)
		}

		result = append(result, n)
		data = rest
	}

	return result, nil
}
