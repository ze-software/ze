// Design: docs/architecture/mrt.md -- BGP message parsing for offline MRT analysis

package mrt

import (
	"encoding/binary"
	"errors"
	"net/netip"
)

var (
	errShortMessage = errors.New("mrt: BGP message too short")
	errBadMarker    = errors.New("mrt: invalid BGP marker")
	errBadMsgType   = errors.New("mrt: unknown BGP message type")
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
		update, err := parseUpdate(body)
		if err != nil {
			return nil, err
		}
		msg.Update = update
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

func parseUpdate(body []byte) (*ParsedUpdate, error) {
	if len(body) < 4 {
		return nil, errShortMessage
	}
	u := &ParsedUpdate{}

	wdLen := int(binary.BigEndian.Uint16(body[0:2]))
	off := 2
	if off+wdLen > len(body) {
		return nil, errShortMessage
	}
	u.WithdrawnPrefixes = parsePrefixes(body[off:off+wdLen], false)
	off += wdLen

	if off+2 > len(body) {
		return nil, errShortMessage
	}
	attrLen := int(binary.BigEndian.Uint16(body[off : off+2]))
	off += 2
	if off+attrLen > len(body) {
		return nil, errShortMessage
	}
	u.Attributes = parseAttributes(body[off : off+attrLen])
	off += attrLen

	if off < len(body) {
		u.AnnouncedPrefixes = parsePrefixes(body[off:], false)
	}
	return u, nil
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

// ParsePrefixes parses packed NLRI prefixes into netip.Prefix values.
// Set addPath=true when the NLRI includes a 4-byte Path Identifier per prefix.
func ParsePrefixes(data []byte, addPath bool) []netip.Prefix {
	return parsePrefixes(data, addPath)
}

func parsePrefixes(data []byte, addPath bool) []netip.Prefix {
	var prefixes []netip.Prefix
	off := 0
	for off < len(data) {
		if addPath {
			if off+4 >= len(data) {
				break
			}
			off += 4
		}
		if off >= len(data) {
			break
		}
		pfxLen := int(data[off])
		off++
		byteLen := (pfxLen + 7) / 8
		if off+byteLen > len(data) {
			break
		}
		var addr [4]byte
		copy(addr[:], data[off:off+byteLen])
		off += byteLen
		ip := netip.AddrFrom4(addr)
		prefixes = append(prefixes, netip.PrefixFrom(ip, pfxLen))
	}
	return prefixes
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
			return nil, errShortData
		}
		segType := data[off]
		count := int(data[off+1])
		off += 2
		needed := count * asnSize
		if off+needed > len(data) {
			return nil, errShortData
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
