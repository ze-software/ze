// Design: docs/architecture/core-design.md — zero-allocation wire UPDATE builders
// Overview: reactor.go — Reactor struct, lifecycle, and connection management
// Related: reactor_api.go — reactorAPIAdapter for plugin integration
// Related: reactor_api_forward.go — forwarding uses wire builders

package reactor

import (
	"encoding/binary"
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"
)

// Zero-allocation attribute writers.
// These functions write attributes directly to the buffer without allocating structs.

// writeOriginAttr writes ORIGIN attribute directly to buf.
// RFC 4271 §5.1.1: Well-known mandatory, 1 byte value.
func writeOriginAttr(buf []byte, off int, origin uint8) int {
	// Header: Transitive(0x40) | code(1) | len(1)
	buf[off] = byte(attribute.FlagTransitive)
	buf[off+1] = byte(attribute.AttrOrigin)
	buf[off+2] = 1
	buf[off+3] = origin
	return 4
}

// writeASPathAttr writes AS_PATH attribute directly to buf.
// RFC 4271 §5.1.2: Well-known mandatory.
// RFC 6793: asn4 determines 2-byte vs 4-byte AS numbers.
// RFC 4271 §4.3: Handles segment splitting for >255 ASNs and extended length.
func writeASPathAttr(buf []byte, off int, asns []uint32, asn4 bool) int {
	start := off
	asnSize := 2
	if asn4 {
		asnSize = 4
	}

	// RFC 4271: Max 255 ASNs per segment, split if needed
	// Calculate total value length accounting for segment splitting
	var valueLen int
	remaining := len(asns)
	for remaining > 0 {
		chunk := min(remaining, attribute.MaxASPathSegmentLength)
		valueLen += 2 + chunk*asnSize // type(1) + count(1) + asns
		remaining -= chunk
	}
	// Empty AS_PATH for iBGP has valueLen=0

	// RFC 4271 §4.3: Use extended length if > 255 bytes
	if valueLen > 255 {
		buf[off] = byte(attribute.FlagTransitive | attribute.FlagExtLength)
		buf[off+1] = byte(attribute.AttrASPath)
		binary.BigEndian.PutUint16(buf[off+2:], uint16(valueLen)) //nolint:gosec // valueLen validated ≤ max attr len
		off += 4
	} else {
		buf[off] = byte(attribute.FlagTransitive)
		buf[off+1] = byte(attribute.AttrASPath)
		buf[off+2] = byte(valueLen)
		off += 3
	}

	// Value: write segments, splitting at 255 ASNs
	remaining = len(asns)
	idx := 0
	for remaining > 0 {
		chunk := min(remaining, attribute.MaxASPathSegmentLength)

		buf[off] = byte(attribute.ASSequence) // Type
		buf[off+1] = byte(chunk)              // Count
		off += 2

		for i := range chunk {
			asn := asns[idx+i]
			if asn4 {
				binary.BigEndian.PutUint32(buf[off:], asn)
				off += 4
			} else {
				// RFC 6793: Map to AS_TRANS if > 65535
				if asn > 65535 {
					binary.BigEndian.PutUint16(buf[off:], 23456) // AS_TRANS
				} else {
					binary.BigEndian.PutUint16(buf[off:], uint16(asn)) //nolint:gosec // asn checked ≤ 65535 in else branch
				}
				off += 2
			}
		}

		idx += chunk
		remaining -= chunk
	}

	return off - start
}

// writeNextHopAttr writes NEXT_HOP attribute directly to buf.
// RFC 4271 §5.1.3: Well-known mandatory, 4 bytes for IPv4.
func writeNextHopAttr(buf []byte, off int, addr netip.Addr) int {
	// Header: Transitive(0x40) | code(3) | len(4)
	buf[off] = byte(attribute.FlagTransitive)
	buf[off+1] = byte(attribute.AttrNextHop)
	buf[off+2] = 4
	a4 := addr.As4()
	copy(buf[off+3:], a4[:])
	return 7
}

// writeMEDAttr writes MED attribute directly to buf.
// RFC 4271 §5.1.4: Optional non-transitive, 4 bytes.
func writeMEDAttr(buf []byte, off int, med uint32) int {
	// Header: Optional(0x80) | code(4) | len(4)
	buf[off] = byte(attribute.FlagOptional)
	buf[off+1] = byte(attribute.AttrMED)
	buf[off+2] = 4
	binary.BigEndian.PutUint32(buf[off+3:], med)
	return 7
}

// writeLocalPrefAttr writes LOCAL_PREF attribute directly to buf.
// RFC 4271 §5.1.5: Well-known for iBGP, 4 bytes.
func writeLocalPrefAttr(buf []byte, off int, localPref uint32) int {
	// Header: Transitive(0x40) | code(5) | len(4)
	buf[off] = byte(attribute.FlagTransitive)
	buf[off+1] = byte(attribute.AttrLocalPref)
	buf[off+2] = 4
	binary.BigEndian.PutUint32(buf[off+3:], localPref)
	return 7
}

// writeCommunitiesAttr writes COMMUNITIES attribute directly to buf.
// RFC 1997: Optional transitive, 4 bytes per community.
// RFC 4271 §4.3: Uses extended length for >63 communities (>255 bytes).
func writeCommunitiesAttr(buf []byte, off int, communities []uint32) int {
	start := off
	valueLen := len(communities) * 4

	// RFC 4271 §4.3: Use extended length if > 255 bytes
	flags := attribute.FlagOptional | attribute.FlagTransitive
	if valueLen > 255 {
		buf[off] = byte(flags | attribute.FlagExtLength)
		buf[off+1] = byte(attribute.AttrCommunity)
		binary.BigEndian.PutUint16(buf[off+2:], uint16(valueLen)) //nolint:gosec // valueLen validated ≤ max attr len
		off += 4
	} else {
		buf[off] = byte(flags)
		buf[off+1] = byte(attribute.AttrCommunity)
		buf[off+2] = byte(valueLen)
		off += 3
	}

	for _, c := range communities {
		binary.BigEndian.PutUint32(buf[off:], c)
		off += 4
	}

	return off - start
}

// WriteAnnounceUpdate writes a complete BGP UPDATE message for announcing a route
// directly into buf at offset off. Returns total bytes written.
//
// True zero-allocation: writes all attributes directly to the buffer.
//
// RFC 4271 Section 4.3 - UPDATE message format.
// RFC 7911: addPath indicates ADD-PATH capability for NLRI encoding.
// RFC 6793: asn4 determines 2-byte vs 4-byte AS numbers in AS_PATH.
func WriteAnnounceUpdate(buf []byte, off int, route bgptypes.RouteSpec, localAS uint32, isIBGP, asn4, addPath bool) int {
	start := off

	// RFC 4271 Section 4.1 - BGP Header: 16-byte marker (all 0xFF)
	for i := range message.MarkerLen {
		buf[off+i] = 0xFF
	}
	off += message.MarkerLen

	// Length placeholder (backfill after body)
	lengthPos := off
	off += 2

	// Type = UPDATE
	buf[off] = byte(message.TypeUPDATE)
	off++

	// RFC 4271 Section 4.3 - Withdrawn Routes Length = 0 (announce, not withdraw)
	buf[off] = 0
	buf[off+1] = 0
	off += 2

	// Path Attributes Length placeholder (backfill after attrs)
	attrLenPos := off
	off += 2
	attrStart := off

	// Extract attributes from Wire (wire-first approach)
	origin := uint8(attribute.OriginIGP)
	var med *uint32
	var localPref *uint32
	var communities []uint32
	var largeCommunities []attribute.LargeCommunity
	var extCommunities []attribute.ExtendedCommunity
	var userASPath []uint32

	if route.Wire != nil {
		// Extract ORIGIN
		if originAttr, err := route.Wire.Get(attribute.AttrOrigin); err == nil && originAttr != nil {
			if o, ok := originAttr.(attribute.Origin); ok {
				origin = uint8(o)
			}
		}
		// Extract AS_PATH (all segments)
		if asPathAttr, err := route.Wire.Get(attribute.AttrASPath); err == nil {
			if asp, ok := asPathAttr.(*attribute.ASPath); ok {
				for _, seg := range asp.Segments {
					userASPath = append(userASPath, seg.ASNs...)
				}
			}
		}
		// Extract MED
		if medAttr, err := route.Wire.Get(attribute.AttrMED); err == nil && medAttr != nil {
			if m, ok := medAttr.(attribute.MED); ok {
				v := uint32(m)
				med = &v
			}
		}
		// Extract LOCAL_PREF
		if lpAttr, err := route.Wire.Get(attribute.AttrLocalPref); err == nil && lpAttr != nil {
			if lp, ok := lpAttr.(attribute.LocalPref); ok {
				v := uint32(lp)
				localPref = &v
			}
		}
		// Extract COMMUNITY
		if commAttr, err := route.Wire.Get(attribute.AttrCommunity); err == nil {
			if comms, ok := commAttr.(attribute.Communities); ok {
				communities = make([]uint32, len(comms))
				for i, c := range comms {
					communities[i] = uint32(c)
				}
			}
		}
		// Extract LARGE_COMMUNITY
		if lcAttr, err := route.Wire.Get(attribute.AttrLargeCommunity); err == nil {
			if lc, ok := lcAttr.(attribute.LargeCommunities); ok {
				largeCommunities = lc
			}
		}
		// Extract EXTENDED_COMMUNITIES
		if ecAttr, err := route.Wire.Get(attribute.AttrExtCommunity); err == nil {
			if ec, ok := ecAttr.(attribute.ExtendedCommunities); ok {
				extCommunities = ec
			}
		}
	}

	// 1. ORIGIN - RFC 4271 §5.1.1: Well-known mandatory attribute.
	off += writeOriginAttr(buf, off, origin)

	// 2. AS_PATH - RFC 4271 §5.1.2: Well-known mandatory attribute.
	// Zero-alloc: write directly without creating ASPath struct.
	var asPathASNs []uint32
	switch {
	case len(userASPath) > 0:
		asPathASNs = userASPath // Use caller's slice directly
	case isIBGP:
		asPathASNs = nil // Empty AS_PATH for iBGP
	default: // eBGP: prepend local AS - use stack-allocated array
		asPathASNs = []uint32{localAS}
	}
	off += writeASPathAttr(buf, off, asPathASNs, asn4)

	isIPv6 := route.Prefix.Addr().Is6()
	nhAddr := route.NextHop.Addr

	// 3. NEXT_HOP - RFC 4271 §5.1.3 (IPv4 only; IPv6 uses MP_REACH_NLRI)
	if !isIPv6 {
		off += writeNextHopAttr(buf, off, nhAddr)
	}

	// 4. MED - RFC 4271 §5.1.4: Optional non-transitive attribute.
	if med != nil {
		off += writeMEDAttr(buf, off, *med)
	}

	// 5. LOCAL_PREF - RFC 4271 §5.1.5: Well-known attribute for iBGP only.
	if isIBGP {
		lpVal := uint32(100)
		if localPref != nil {
			lpVal = *localPref
		}
		off += writeLocalPrefAttr(buf, off, lpVal)
	}

	// 6. COMMUNITY - RFC 1997: Optional transitive attribute.
	if len(communities) > 0 {
		off += writeCommunitiesAttr(buf, off, communities)
	}

	// 7. LARGE_COMMUNITY - RFC 8092: Optional transitive attribute.
	// Type conversion only, no allocation.
	if len(largeCommunities) > 0 {
		lcomms := attribute.LargeCommunities(largeCommunities)
		off += attribute.WriteAttrTo(lcomms, buf, off)
	}

	// 8. EXTENDED_COMMUNITIES - RFC 4360: Optional transitive attribute.
	// Type conversion only, no allocation.
	if len(extCommunities) > 0 {
		extComms := attribute.ExtendedCommunities(extCommunities)
		off += attribute.WriteAttrTo(extComms, buf, off)
	}

	// NLRI handling - MP_REACH_NLRI (14) goes at end per our pattern
	if !isIPv6 {
		// IPv4: Write NLRI directly after attributes (zero-alloc)
		// Backfill attr length first
		attrLen := off - attrStart
		buf[attrLenPos] = byte(attrLen >> 8)
		buf[attrLenPos+1] = byte(attrLen)

		// RFC 7911: WriteNLRI handles ADD-PATH encoding
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, route.Prefix, 0)
		off += nlri.WriteNLRI(inet, buf, off, addPath)
	} else {
		// RFC 4760 Section 3 - IPv6: Write MP_REACH_NLRI directly (zero-alloc)
		// Wire format: AFI(2) + SAFI(1) + NH_Len(1) + NextHop(16) + Reserved(1) + NLRI(var)
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}, route.Prefix, 0)
		nlriPayloadLen := nlri.LenWithContext(inet, addPath)
		nhLen := 16 // IPv6 next-hop
		mpValueLen := 2 + 1 + 1 + nhLen + 1 + nlriPayloadLen

		// RFC 4760 Section 3 - Attribute header (Optional, non-transitive)
		off += attribute.WriteHeaderTo(buf, off, attribute.FlagOptional, attribute.AttrMPReachNLRI, uint16(mpValueLen)) //nolint:gosec // mpValueLen bounded by UPDATE max

		// RFC 4760 Section 3 - AFI (2 octets)
		buf[off] = 0
		buf[off+1] = byte(attribute.AFIIPv6)
		off += 2

		// RFC 4760 Section 3 - SAFI (1 octet)
		buf[off] = byte(attribute.SAFIUnicast)
		off++

		// RFC 4760 Section 3 - Length of Next Hop (1 octet)
		buf[off] = byte(nhLen)
		off++

		// RFC 4760 Section 3 - Network Address of Next Hop (variable)
		off += copy(buf[off:], nhAddr.AsSlice())

		// RFC 4760 Section 3 - Reserved (1 octet, MUST be 0)
		buf[off] = 0
		off++

		// RFC 4760 Section 3 - NLRI (variable)
		// RFC 7911: WriteNLRI handles ADD-PATH encoding when negotiated
		off += nlri.WriteNLRI(inet, buf, off, addPath)

		// Backfill attr length (no inline NLRI for IPv6)
		attrLen := off - attrStart
		buf[attrLenPos] = byte(attrLen >> 8)
		buf[attrLenPos+1] = byte(attrLen)
	}

	// Backfill total message length
	totalLen := off - start
	buf[lengthPos] = byte(totalLen >> 8)
	buf[lengthPos+1] = byte(totalLen)

	return totalLen
}

// WriteWithdrawUpdate writes a complete BGP UPDATE message for withdrawing a route
// directly into buf at offset off. Returns total bytes written.
//
// Eliminates large buffer allocations by writing directly to the provided buffer.
//
// RFC 4271 Section 4.3 - UPDATE message format.
// RFC 4760 Section 4: IPv6 withdrawals use MP_UNREACH_NLRI attribute.
// RFC 7911: addPath indicates ADD-PATH capability for NLRI encoding.
func WriteWithdrawUpdate(buf []byte, off int, prefix netip.Prefix, addPath bool) int {
	start := off

	// RFC 4271 Section 4.1 - BGP Header: 16-byte marker (all 0xFF)
	for i := range message.MarkerLen {
		buf[off+i] = 0xFF
	}
	off += message.MarkerLen

	// Length placeholder
	lengthPos := off
	off += 2

	// Type = UPDATE
	buf[off] = byte(message.TypeUPDATE)
	off++

	if prefix.Addr().Is4() {
		// RFC 4271 Section 4.3 - IPv4: Use WithdrawnRoutes field (zero-alloc)
		// Withdrawn Routes Length placeholder
		withdrawnLenPos := off
		off += 2
		withdrawnStart := off

		// RFC 4271 Section 4.3 - Withdrawn Routes: list of IP address prefixes
		// RFC 7911: WriteNLRI handles ADD-PATH encoding when negotiated
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, prefix, 0)
		off += nlri.WriteNLRI(inet, buf, off, addPath)

		// RFC 4271 Section 4.3 - Backfill Withdrawn Routes Length
		withdrawnLen := off - withdrawnStart
		buf[withdrawnLenPos] = byte(withdrawnLen >> 8)
		buf[withdrawnLenPos+1] = byte(withdrawnLen)

		// RFC 4271 Section 4.3 - Total Path Attribute Length = 0 (withdrawal only)
		buf[off] = 0
		buf[off+1] = 0
		off += 2
	} else {
		// RFC 4760 Section 4 - IPv6: Use MP_UNREACH_NLRI attribute (zero-alloc)
		// RFC 4271 Section 4.3 - Withdrawn Routes Length = 0 (using MP_UNREACH instead)
		buf[off] = 0
		buf[off+1] = 0
		off += 2

		// RFC 4271 Section 4.3 - Path Attributes Length placeholder
		attrLenPos := off
		off += 2
		attrStart := off

		// RFC 4760 Section 4 - MP_UNREACH_NLRI wire format:
		//   AFI(2) + SAFI(1) + Withdrawn_NLRI(var)
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}, prefix, 0)
		nlriPayloadLen := nlri.LenWithContext(inet, addPath)
		mpValueLen := 2 + 1 + nlriPayloadLen

		// RFC 4760 Section 4 - Attribute header (Optional, non-transitive)
		off += attribute.WriteHeaderTo(buf, off, attribute.FlagOptional, attribute.AttrMPUnreachNLRI, uint16(mpValueLen)) //nolint:gosec // mpValueLen bounded by UPDATE max

		// RFC 4760 Section 4 - AFI (2 octets)
		buf[off] = 0
		buf[off+1] = byte(attribute.AFIIPv6)
		off += 2

		// RFC 4760 Section 4 - SAFI (1 octet)
		buf[off] = byte(attribute.SAFIUnicast)
		off++

		// RFC 4760 Section 4 - Withdrawn Routes (variable)
		// RFC 7911: WriteNLRI handles ADD-PATH encoding when negotiated
		off += nlri.WriteNLRI(inet, buf, off, addPath)

		// Backfill attr length
		attrLen := off - attrStart
		buf[attrLenPos] = byte(attrLen >> 8)
		buf[attrLenPos+1] = byte(attrLen)
	}

	// Backfill total message length
	totalLen := off - start
	buf[lengthPos] = byte(totalLen >> 8)
	buf[lengthPos+1] = byte(totalLen)

	return totalLen
}
