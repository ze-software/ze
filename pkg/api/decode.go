package api

import (
	"encoding/binary"
	"net/netip"

	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
)

// DecodedUpdate holds parsed UPDATE message contents.
type DecodedUpdate struct {
	Announced []ReceivedRoute // Announced routes (NLRI + MP_REACH_NLRI)
	Withdrawn []netip.Prefix  // Withdrawn prefixes (Withdrawn Routes + MP_UNREACH_NLRI)
}

// DecodeUpdate parses raw UPDATE bytes into announced and withdrawn routes.
// Handles both IPv4 unicast NLRI and MP_REACH_NLRI for IPv6.
func DecodeUpdate(body []byte) DecodedUpdate {
	result := DecodedUpdate{}

	if len(body) < 4 {
		return result
	}

	// Parse UPDATE structure: withdrawn_len (2) + withdrawn + attr_len (2) + attrs + nlri
	withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
	offset := 2

	// Parse withdrawn routes (IPv4)
	if withdrawnLen > 0 && offset+withdrawnLen <= len(body) {
		result.Withdrawn = parseIPv4Prefixes(body[offset : offset+withdrawnLen])
	}
	offset += withdrawnLen

	if offset+2 > len(body) {
		return result
	}

	attrLen := int(binary.BigEndian.Uint16(body[offset : offset+2]))
	offset += 2
	if offset+attrLen > len(body) {
		return result
	}

	pathAttrs := body[offset : offset+attrLen]
	nlriOffset := offset + attrLen
	nlriLen := len(body) - nlriOffset

	// Parse path attributes including MP extensions
	attrs := parsePathAttributes(pathAttrs)

	// Add MP_UNREACH_NLRI withdrawals
	result.Withdrawn = append(result.Withdrawn, attrs.mpWithdrawn...)

	// Parse IPv4 NLRI and add to announced
	if nlriLen > 0 {
		ipv4Routes := parseIPv4NLRI(body[nlriOffset:], attrs)
		result.Announced = append(result.Announced, ipv4Routes...)
	}

	// Add MP_REACH_NLRI announcements
	result.Announced = append(result.Announced, attrs.mpAnnounced...)

	return result
}

// DecodeUpdateRoutes parses raw UPDATE bytes into ReceivedRoute structs.
// This is used for on-demand parsing when format=parsed or format=full.
// For full UPDATE parsing including withdrawals, use DecodeUpdate instead.
func DecodeUpdateRoutes(body []byte) []ReceivedRoute {
	return DecodeUpdate(body).Announced
}

// parsedAttrs holds attributes extracted from UPDATE path attributes.
type parsedAttrs struct {
	origin      string
	localPref   uint32
	med         uint32
	nextHop     netip.Addr
	asPath      []uint32
	mpAnnounced []ReceivedRoute
	mpWithdrawn []netip.Prefix
}

// parsePathAttributes extracts path attributes from UPDATE.
func parsePathAttributes(pathAttrs []byte) parsedAttrs {
	attrs := parsedAttrs{
		origin:    "igp",
		localPref: 100, // Default for iBGP
	}

	for i := 0; i < len(pathAttrs); {
		if i+2 > len(pathAttrs) {
			break
		}
		flags := pathAttrs[i]
		typeCode := pathAttrs[i+1]
		attrLenBytes := 1
		if flags&0x10 != 0 { // Extended length
			attrLenBytes = 2
		}
		if i+2+attrLenBytes > len(pathAttrs) {
			break
		}
		var attrValueLen int
		if attrLenBytes == 1 {
			attrValueLen = int(pathAttrs[i+2])
			i += 3
		} else {
			attrValueLen = int(binary.BigEndian.Uint16(pathAttrs[i+2 : i+4]))
			i += 4
		}
		if i+attrValueLen > len(pathAttrs) {
			break
		}
		attrValue := pathAttrs[i : i+attrValueLen]
		i += attrValueLen

		switch typeCode {
		case 1: // ORIGIN
			if o, err := attribute.ParseOrigin(attrValue); err == nil {
				attrs.origin = o.String()
			}
		case 2: // AS_PATH
			if ap, err := attribute.ParseASPath(attrValue, true); err == nil {
				for _, seg := range ap.Segments {
					attrs.asPath = append(attrs.asPath, seg.ASNs...)
				}
			}
		case 3: // NEXT_HOP
			if nh, err := attribute.ParseNextHop(attrValue); err == nil {
				attrs.nextHop = nh.Addr
			}
		case 4: // MED
			if m, err := attribute.ParseMED(attrValue); err == nil {
				attrs.med = uint32(m)
			}
		case 5: // LOCAL_PREF
			if lp, err := attribute.ParseLocalPref(attrValue); err == nil {
				attrs.localPref = uint32(lp)
			}
		case 14: // MP_REACH_NLRI
			attrs.mpAnnounced = parseMPReachNLRI(attrValue, attrs)
		case 15: // MP_UNREACH_NLRI
			attrs.mpWithdrawn = parseMPUnreachNLRI(attrValue)
		}
	}

	return attrs
}

// parseIPv4Prefixes parses a sequence of IPv4 prefixes (used for withdrawn routes).
func parseIPv4Prefixes(data []byte) []netip.Prefix {
	var prefixes []netip.Prefix
	for i := 0; i < len(data); {
		if i >= len(data) {
			break
		}
		prefixLen := int(data[i])
		i++
		prefixBytes := (prefixLen + 7) / 8
		if i+prefixBytes > len(data) {
			break
		}
		var addrBytes [4]byte
		copy(addrBytes[:], data[i:i+prefixBytes])
		i += prefixBytes

		addr := netip.AddrFrom4(addrBytes)
		prefix := netip.PrefixFrom(addr, prefixLen)
		prefixes = append(prefixes, prefix)
	}
	return prefixes
}

// parseIPv4NLRI parses IPv4 NLRI into ReceivedRoute structs.
func parseIPv4NLRI(data []byte, attrs parsedAttrs) []ReceivedRoute {
	var routes []ReceivedRoute
	for i := 0; i < len(data); {
		if i >= len(data) {
			break
		}
		prefixLen := int(data[i])
		i++
		prefixBytes := (prefixLen + 7) / 8
		if i+prefixBytes > len(data) {
			break
		}
		var addrBytes [4]byte
		copy(addrBytes[:], data[i:i+prefixBytes])
		i += prefixBytes

		addr := netip.AddrFrom4(addrBytes)
		prefix := netip.PrefixFrom(addr, prefixLen)
		routes = append(routes, ReceivedRoute{
			Prefix:          prefix,
			NextHop:         attrs.nextHop,
			Origin:          attrs.origin,
			LocalPreference: attrs.localPref,
			MED:             attrs.med,
			ASPath:          attrs.asPath,
		})
	}
	return routes
}

// parseMPReachNLRI parses MP_REACH_NLRI attribute (RFC 4760).
// Handles IPv6 unicast announcements.
func parseMPReachNLRI(data []byte, attrs parsedAttrs) []ReceivedRoute {
	// MP_REACH_NLRI format:
	// AFI (2) + SAFI (1) + NH Length (1) + Next Hop + Reserved (1) + NLRI
	if len(data) < 5 {
		return nil
	}

	afi := binary.BigEndian.Uint16(data[0:2])
	safi := data[2]
	nhLen := int(data[3])

	if len(data) < 4+nhLen+1 {
		return nil
	}

	// Parse next hop (IPv6: 16 or 32 bytes for link-local)
	var nextHop netip.Addr
	if afi == 2 && nhLen >= 16 { // AFI_IPV6
		var addrBytes [16]byte
		copy(addrBytes[:], data[4:4+16])
		nextHop = netip.AddrFrom16(addrBytes)
	}

	// Skip to NLRI (after next hop + reserved byte)
	nlriOffset := 4 + nhLen + 1
	if nlriOffset >= len(data) {
		return nil
	}
	nlriData := data[nlriOffset:]

	// Only handle IPv6 unicast for now
	if afi != 2 || safi != 1 {
		return nil
	}

	return parseIPv6NLRI(nlriData, nextHop, attrs)
}

// parseMPUnreachNLRI parses MP_UNREACH_NLRI attribute (RFC 4760).
// Handles IPv6 unicast withdrawals.
func parseMPUnreachNLRI(data []byte) []netip.Prefix {
	// MP_UNREACH_NLRI format:
	// AFI (2) + SAFI (1) + Withdrawn Routes
	if len(data) < 3 {
		return nil
	}

	afi := binary.BigEndian.Uint16(data[0:2])
	safi := data[2]

	// Only handle IPv6 unicast for now
	if afi != 2 || safi != 1 {
		return nil
	}

	return parseIPv6Prefixes(data[3:])
}

// parseIPv6NLRI parses IPv6 NLRI into ReceivedRoute structs.
func parseIPv6NLRI(data []byte, nextHop netip.Addr, attrs parsedAttrs) []ReceivedRoute {
	var routes []ReceivedRoute
	for i := 0; i < len(data); {
		if i >= len(data) {
			break
		}
		prefixLen := int(data[i])
		i++
		prefixBytes := (prefixLen + 7) / 8
		if i+prefixBytes > len(data) {
			break
		}
		var addrBytes [16]byte
		copy(addrBytes[:], data[i:i+prefixBytes])
		i += prefixBytes

		addr := netip.AddrFrom16(addrBytes)
		prefix := netip.PrefixFrom(addr, prefixLen)
		routes = append(routes, ReceivedRoute{
			Prefix:          prefix,
			NextHop:         nextHop,
			Origin:          attrs.origin,
			LocalPreference: attrs.localPref,
			MED:             attrs.med,
			ASPath:          attrs.asPath,
		})
	}
	return routes
}

// parseIPv6Prefixes parses a sequence of IPv6 prefixes (used for MP_UNREACH).
func parseIPv6Prefixes(data []byte) []netip.Prefix {
	var prefixes []netip.Prefix
	for i := 0; i < len(data); {
		if i >= len(data) {
			break
		}
		prefixLen := int(data[i])
		i++
		prefixBytes := (prefixLen + 7) / 8
		if i+prefixBytes > len(data) {
			break
		}
		var addrBytes [16]byte
		copy(addrBytes[:], data[i:i+prefixBytes])
		i += prefixBytes

		addr := netip.AddrFrom16(addrBytes)
		prefix := netip.PrefixFrom(addr, prefixLen)
		prefixes = append(prefixes, prefix)
	}
	return prefixes
}
