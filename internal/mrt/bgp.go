// Design: docs/architecture/mrt.md -- BGP message parsing for offline MRT analysis
// RFC: rfc/short/rfc6396.md -- AS width and MP_REACH constraints per record type
// Related: bgp_attribute.go -- typed path-attribute decoders (MP_REACH, aggregator, communities)
// Related: format.go -- attribute string rendering for display and matching

package mrt

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
)

var (
	errShortMessage = errors.New("mrt: BGP message too short")
	errBadMarker    = errors.New("mrt: invalid BGP marker")
	errBadMsgType   = errors.New("mrt: unknown BGP message type")
	errBadPrefixLen = errors.New("mrt: prefix length out of range")
)

// ParsedMessage is a parsed BGP message from an MRT record.
type ParsedMessage struct {
	Type         uint8
	Open         *ParsedOpen
	Update       *ParsedUpdate
	Notification *ParsedNotification
}

// ParsedOpen contains fields from a BGP OPEN message.
type ParsedOpen struct {
	Version  uint8
	ASN      uint32
	HoldTime uint16
	RouterID [4]byte
	Caps     []Capability
}

// Capability is a single BGP capability from an OPEN message.
type Capability struct {
	Code  uint8
	Value []byte
}

// ParsedUpdate contains fields from a BGP UPDATE message.
type ParsedUpdate struct {
	WithdrawnPrefixes []netip.Prefix
	Attributes        []PathAttribute
	AnnouncedPrefixes []netip.Prefix
}

// PathAttribute is a single parsed path attribute.
type PathAttribute struct {
	Flags uint8
	Code  uint8
	Value []byte
}

// ParsedNotification contains fields from a BGP NOTIFICATION message.
type ParsedNotification struct {
	Code    uint8
	Subcode uint8
	Data    []byte
}

// ParseBGPMessage parses a complete BGP message (including 19-byte header).
//
// For an UPDATE it may return a non-nil message TOGETHER with a non-nil error:
// the message holds everything that decoded and the error names what did not
// (see parseUpdate). Callers that only want fully-clean records check err
// first, as before; callers that render records check the value too.
func ParseBGPMessage(data []byte) (*ParsedMessage, error) {
	if len(data) < 19 {
		return nil, errShortMessage
	}
	for i := range 16 {
		if data[i] != 0xff {
			return nil, errBadMarker
		}
	}

	msgLen := int(binary.BigEndian.Uint16(data[16:18]))
	if msgLen < 19 || msgLen > len(data) {
		return nil, errShortMessage
	}

	msgType := data[18]
	body := data[19:msgLen]

	msg := &ParsedMessage{Type: msgType}
	switch msgType {
	case 1:
		open, err := parseOpen(body)
		if err != nil {
			return nil, err
		}
		msg.Open = open
	case 2:
		// Salvage: a UPDATE whose withdrawn/NLRI field is damaged still returns
		// the fields that decoded, together with the error. Only a body too
		// broken to yield anything (updateSections failed) returns nil.
		update, uerr := parseUpdate(body)
		if update == nil {
			return nil, uerr
		}
		msg.Update = update
		if uerr != nil {
			return msg, uerr
		}
	case 3:
		msg.Notification = parseNotification(body)
	case 4:
		// KEEPALIVE has no body
	case 5:
		// ROUTE-REFRESH: not parsed further
	default:
		return nil, errBadMsgType
	}
	return msg, nil
}

func parseOpen(body []byte) (*ParsedOpen, error) {
	if len(body) < 10 {
		return nil, errShortMessage
	}
	o := &ParsedOpen{
		Version:  body[0],
		HoldTime: binary.BigEndian.Uint16(body[3:5]),
	}
	asn16 := binary.BigEndian.Uint16(body[1:3])
	o.ASN = uint32(asn16)
	copy(o.RouterID[:], body[5:9])

	optLen := int(body[9])
	if 10+optLen > len(body) {
		return nil, errShortMessage
	}
	o.Caps = parseCapabilities(body[10 : 10+optLen])

	for _, cap := range o.Caps {
		if cap.Code == 65 && len(cap.Value) == 4 {
			o.ASN = binary.BigEndian.Uint32(cap.Value)
		}
	}
	return o, nil
}

func parseCapabilities(optParams []byte) []Capability {
	var caps []Capability
	off := 0
	for off+2 <= len(optParams) {
		pType := optParams[off]
		pLen := int(optParams[off+1])
		off += 2
		if off+pLen > len(optParams) {
			break
		}
		if pType != 2 {
			off += pLen
			continue
		}
		capData := optParams[off : off+pLen]
		cOff := 0
		for cOff+2 <= len(capData) {
			code := capData[cOff]
			cLen := int(capData[cOff+1])
			cOff += 2
			if cOff+cLen > len(capData) {
				break
			}
			val := make([]byte, cLen)
			copy(val, capData[cOff:cOff+cLen])
			caps = append(caps, Capability{Code: code, Value: val})
			cOff += cLen
		}
		off += pLen
	}
	return caps
}

// UpdateAttributeBytes returns the raw Path Attributes section of an UPDATE
// body, without decoding the individual attributes.
//
// Callers that need the attribute bytes verbatim (to re-pack them, hash them,
// or hand them to a matcher) use this; callers that want decoded attributes use
// ParseUpdateBody. Both share one implementation of the field offsets, so the
// two can never disagree about where the section starts.
func UpdateAttributeBytes(body []byte) ([]byte, error) {
	_, attrs, _, err := updateSections(body)
	return attrs, err
}

// UpdateSections splits an UPDATE body into its withdrawn-routes, path-attribute
// and NLRI sections (RFC 4271 Section 4.3).
//
// This is the exported face of the single source of truth for the UPDATE field
// layout. A caller that needs more than the attribute bytes -- counting the
// prefixes in all four NLRI locations, for instance -- MUST come through here
// rather than re-deriving the offsets, because a second copy of the layout is a
// second thing to get wrong and it will not be wrong in the same way.
func UpdateSections(body []byte) (withdrawn, attrs, nlri []byte, err error) {
	return updateSections(body)
}

// updateSections splits an UPDATE body into its three variable-length fields
// (RFC 4271 Section 4.3):
//
//	Withdrawn Routes Length(2) + Withdrawn Routes(var) +
//	Total Path Attribute Length(2) + Path Attributes(var) + NLRI(var)
//
// This is the single source of truth for the UPDATE field layout.
func updateSections(body []byte) (withdrawn, attrs, nlri []byte, err error) {
	const fixedFields = 4 // the two 2-octet length fields
	if len(body) < fixedFields {
		return nil, nil, nil, fmt.Errorf("%w: UPDATE body needs at least %d octets for its length fields, have %d", errShortMessage, fixedFields, len(body))
	}

	wdLen := int(binary.BigEndian.Uint16(body[0:2]))
	off := 2
	if off+wdLen > len(body) {
		return nil, nil, nil, fmt.Errorf("%w: withdrawn-routes length %d exceeds %d remaining octets", errShortMessage, wdLen, len(body)-off)
	}
	withdrawn = body[off : off+wdLen]
	off += wdLen

	if off+2 > len(body) {
		return nil, nil, nil, fmt.Errorf("%w: UPDATE truncated before the path-attribute length field at offset %d", errShortMessage, off)
	}
	attrLen := int(binary.BigEndian.Uint16(body[off : off+2]))
	off += 2
	if off+attrLen > len(body) {
		return nil, nil, nil, fmt.Errorf("%w: path-attribute length %d exceeds %d remaining octets", errShortMessage, attrLen, len(body)-off)
	}
	attrs = body[off : off+attrLen]
	off += attrLen

	if off < len(body) {
		nlri = body[off:]
	}
	return withdrawn, attrs, nlri, nil
}

// parseUpdate decodes an UPDATE body.
//
// A damaged withdrawn-routes or NLRI field returns the ParsedUpdate decoded so
// far ALONGSIDE the error, so a caller that renders a record (ze-analyze show)
// can print what survived and mark it damaged, while a caller that needs
// correctness still sees the failure. Both fields are attempted even when the
// first fails: they are independent, and reporting only the earlier one would
// hide a second fault.
func parseUpdate(body []byte) (*ParsedUpdate, error) {
	withdrawn, attrs, nlri, err := updateSections(body)
	if err != nil {
		return nil, err
	}

	withdrawnPrefixes, wErr := ParsePrefixes(withdrawn, false)
	if wErr != nil {
		wErr = fmt.Errorf("UPDATE withdrawn routes: %w", wErr)
	}
	announcedPrefixes, aErr := ParsePrefixes(nlri, false)
	if aErr != nil {
		aErr = fmt.Errorf("UPDATE NLRI: %w", aErr)
	}

	return &ParsedUpdate{
		WithdrawnPrefixes: withdrawnPrefixes,
		Attributes:        parseAttributes(attrs),
		AnnouncedPrefixes: announcedPrefixes,
	}, errors.Join(wErr, aErr)
}

func parseNotification(body []byte) *ParsedNotification {
	n := &ParsedNotification{}
	if len(body) >= 1 {
		n.Code = body[0]
	}
	if len(body) >= 2 {
		n.Subcode = body[1]
	}
	if len(body) > 2 {
		n.Data = make([]byte, len(body)-2)
		copy(n.Data, body[2:])
	}
	return n
}

// ParseAttributes extracts path attributes from raw attribute bytes.
func ParseAttributes(data []byte) []PathAttribute {
	return parseAttributes(data)
}

func parseAttributes(data []byte) []PathAttribute {
	var attrs []PathAttribute
	off := 0
	for off < len(data) {
		if off+2 > len(data) {
			break
		}
		flags := data[off]
		code := data[off+1]
		off += 2

		var attrLen int
		if flags&0x10 != 0 {
			if off+2 > len(data) {
				break
			}
			attrLen = int(binary.BigEndian.Uint16(data[off : off+2]))
			off += 2
		} else {
			if off >= len(data) {
				break
			}
			attrLen = int(data[off])
			off++
		}
		if off+attrLen > len(data) {
			break
		}
		val := make([]byte, attrLen)
		copy(val, data[off:off+attrLen])
		attrs = append(attrs, PathAttribute{Flags: flags, Code: code, Value: val})
		off += attrLen
	}
	return attrs
}

// ParsePrefixes parses packed IPv4 NLRI prefixes into netip.Prefix values.
// Set addPath=true when the NLRI includes a 4-byte Path Identifier per prefix.
//
// The withdrawn-routes and NLRI fields of a BGP UPDATE are always IPv4
// (RFC 4271 Section 4.3); IPv6 reachability travels in MP_REACH_NLRI. Use
// ParsePrefixesAFI for those.
func ParsePrefixes(data []byte, addPath bool) ([]netip.Prefix, error) {
	return ParsePrefixesAFI(data, AFIIPv4, addPath)
}

// ParsePrefixesAFI parses packed NLRI prefixes for the given address family.
//
// Malformed input is reported, never silently dropped. A damaged NLRI field
// returns the prefixes decoded so far together with an error naming the offset
// and the offending value, so a caller can both salvage the good entries and
// tell the operator the record is damaged. Returning the short list alone would
// make "fewer routes than the file contains" indistinguishable from "the file
// has fewer routes" (ai/rules/fail-closed-guards.md).
//
// A prefix length beyond the family width is never emitted: netip's zero Prefix
// reads downstream as a default route.
//
// An unrecognized AFI yields no prefixes and an error, per RFC 6396
// Section 4.3.3 ("SHOULD discard the remainder of the MRT record").
func ParsePrefixesAFI(data []byte, afi uint16, addPath bool) ([]netip.Prefix, error) {
	var maxBits int
	switch afi {
	case AFIIPv4:
		maxBits = 32
	case AFIIPv6:
		maxBits = 128
	default:
		return nil, fmt.Errorf("%w: %d, want %d (IPv4) or %d (IPv6)", ErrBadAFI, afi, AFIIPv4, AFIIPv6)
	}

	var prefixes []netip.Prefix
	off := 0
	for off < len(data) {
		if addPath {
			if off+4 >= len(data) {
				return prefixes, fmt.Errorf("%w: NLRI truncated inside the add-path Path Identifier at offset %d", ErrShortData, off)
			}
			off += 4
		}
		pfxLen := int(data[off])
		off++
		if pfxLen > maxBits {
			return prefixes, fmt.Errorf("%w: prefix length %d at offset %d exceeds %d bits for AFI %d", errBadPrefixLen, pfxLen, off-1, maxBits, afi)
		}
		byteLen := (pfxLen + 7) / 8
		if off+byteLen > len(data) {
			return prefixes, fmt.Errorf("%w: prefix at offset %d needs %d octets, %d remain", ErrShortData, off, byteLen, len(data)-off)
		}

		var buf [16]byte
		copy(buf[:byteLen], data[off:off+byteLen])
		off += byteLen

		ip := netip.AddrFrom16(buf)
		if afi == AFIIPv4 {
			ip = netip.AddrFrom4([4]byte(buf[:4]))
		}
		prefixes = append(prefixes, netip.PrefixFrom(ip, pfxLen))
	}
	return prefixes, nil
}

// ParseASPath parses AS_PATH value bytes into a list of (segType, []asn) pairs.
// fourByte=true for 4-byte ASN encoding (TABLE_DUMP_V2, BGP4MP_MESSAGE_AS4).
func ParseASPath(data []byte, fourByte bool) ([]ASPathSegment, error) {
	var segments []ASPathSegment
	asnSize := 2
	if fourByte {
		asnSize = 4
	}
	off := 0
	for off < len(data) {
		if off+2 > len(data) {
			return nil, ErrShortData
		}
		segType := data[off]
		count := int(data[off+1])
		off += 2
		needed := count * asnSize
		if off+needed > len(data) {
			return nil, ErrShortData
		}
		asns := make([]uint32, count)
		for i := range count {
			if fourByte {
				asns[i] = binary.BigEndian.Uint32(data[off:])
			} else {
				asns[i] = uint32(binary.BigEndian.Uint16(data[off:]))
			}
			off += asnSize
		}
		segments = append(segments, ASPathSegment{Type: segType, ASNs: asns})
	}
	return segments, nil
}

// ASPathSegment is a segment in an AS path (for offline parsing).
type ASPathSegment struct {
	Type uint8
	ASNs []uint32
}

// FindAttribute returns the first attribute with the given type code, or nil.
func FindAttribute(attrs []PathAttribute, code uint8) *PathAttribute {
	for i := range attrs {
		if attrs[i].Code == code {
			return &attrs[i]
		}
	}
	return nil
}

// ExtractNextHop returns the next-hop address from path attributes.
// Checks NEXT_HOP (type 3) first, then MP_REACH_NLRI (type 14).
func ExtractNextHop(attrs []PathAttribute) netip.Addr {
	if nh := FindAttribute(attrs, 3); nh != nil && len(nh.Value) == 4 {
		return netip.AddrFrom4([4]byte(nh.Value))
	}
	if mp := FindAttribute(attrs, 14); mp != nil {
		return parseMPReachNextHop(mp.Value)
	}
	return netip.Addr{}
}

func parseMPReachNextHop(data []byte) netip.Addr {
	// MP_REACH_NLRI: AFI(2) + SAFI(1) + NH_Len(1) + NH(var) + Reserved(1) + NLRI(var)
	if len(data) < 4 {
		return netip.Addr{}
	}
	nhLen := int(data[3])
	if 4+nhLen > len(data) {
		return netip.Addr{}
	}
	nh := data[4 : 4+nhLen]
	switch nhLen {
	case 4:
		return netip.AddrFrom4([4]byte(nh))
	case 16:
		return netip.AddrFrom16([16]byte(nh))
	case 32:
		return netip.AddrFrom16([16]byte(nh[:16]))
	}
	return netip.Addr{}
}

// ExtractOrigin returns the ORIGIN value (0=IGP, 1=EGP, 2=INCOMPLETE).
func ExtractOrigin(attrs []PathAttribute) (uint8, bool) {
	if a := FindAttribute(attrs, 1); a != nil && len(a.Value) == 1 {
		return a.Value[0], true
	}
	return 0, false
}

// ExtractLocalPref returns the LOCAL_PREF value.
func ExtractLocalPref(attrs []PathAttribute) (uint32, bool) {
	if a := FindAttribute(attrs, 5); a != nil && len(a.Value) == 4 {
		return binary.BigEndian.Uint32(a.Value), true
	}
	return 0, false
}

// ExtractMED returns the MULTI_EXIT_DISC value.
func ExtractMED(attrs []PathAttribute) (uint32, bool) {
	if a := FindAttribute(attrs, 4); a != nil && len(a.Value) == 4 {
		return binary.BigEndian.Uint32(a.Value), true
	}
	return 0, false
}

// ExtractCommunities returns standard communities as (high:low) uint32 pairs.
func ExtractCommunities(attrs []PathAttribute) []uint32 {
	a := FindAttribute(attrs, 8)
	if a == nil || len(a.Value)%4 != 0 {
		return nil
	}
	comms := make([]uint32, len(a.Value)/4)
	for i := range comms {
		comms[i] = binary.BigEndian.Uint32(a.Value[i*4:])
	}
	return comms
}
