// Design: docs/architecture/wire/nlri-evpn.md — EVPN NLRI plugin
// RFC: rfc/short/rfc7432.md
//
// Package evpn implements EVPN NLRI types for the evpn plugin.
// RFC 7432: BGP MPLS-Based Ethernet VPN
// RFC 9136: IP Prefix Advertisement in EVPN
package evpn

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Type aliases for nlri types used by EVPN.
type (
	Family             = family.Family
	RouteDistinguisher = nlri.RouteDistinguisher
)

// Re-export constants from nlri for local use.
const (
	AFIL2VPN = family.AFIL2VPN
	SAFIEVPN = family.SAFIEVPN
)

// Family registration for EVPN.
var L2VPNEVPN = family.MustRegister(AFIL2VPN, SAFIEVPN, "l2vpn", "evpn")

// Re-export parsing functions.
var (
	ParseRouteDistinguisher = nlri.ParseRouteDistinguisher
	ParseLabelStack         = nlri.ParseLabelStack
	EncodeLabelStack        = nlri.EncodeLabelStack
	WriteLabelStack         = nlri.WriteLabelStack
	ParseRDString           = nlri.ParseRDString
)

// EVPN errors.
var (
	ErrEVPNTruncated      = errors.New("evpn: truncated data")
	ErrEVPNInvalidAddress = errors.New("evpn: invalid address")
	ErrEVPNInvalidPrefix  = errors.New("evpn: invalid prefix")
)

// EVPNRouteType identifies the EVPN route type.
// RFC 7432 Section 7 defines the EVPN NLRI format.
type EVPNRouteType uint8

// EVPN Route Types per RFC 7432 Section 7 and RFC 9136.
const (
	EVPNRouteType1 EVPNRouteType = 1 // Ethernet Auto-Discovery (RFC 7432 Section 7.1)
	EVPNRouteType2 EVPNRouteType = 2 // MAC/IP Advertisement (RFC 7432 Section 7.2)
	EVPNRouteType3 EVPNRouteType = 3 // Inclusive Multicast Ethernet Tag (RFC 7432 Section 7.3)
	EVPNRouteType4 EVPNRouteType = 4 // Ethernet Segment (RFC 7432 Section 7.4)
	EVPNRouteType5 EVPNRouteType = 5 // IP Prefix (RFC 9136 Section 3)
)

// Route type name constants — used in String(), encoding, and JSON parsing.
// evpnHeaderLen is the common EVPN NLRI header: Route Type (1) + Length (1), RFC 7432
// Section 7. The Length being a single octet is why a route body cannot exceed 255.
const evpnHeaderLen = 2

// familyNameEVPN is the address family this plugin decodes and encodes.
const familyNameEVPN = "l2vpn/evpn"

const (
	RouteNameEthernetAutoDiscovery = "ethernet-auto-discovery"
	RouteNameMACIPAdvertisement    = "mac-ip-advertisement"
	RouteNameInclusiveMulticast    = "inclusive-multicast"
	RouteNameEthernetSegment       = "ethernet-segment"
	RouteNameIPPrefix              = "ip-prefix"
)

// Short route-type tokens. The encoder accepts these beside the descriptive
// RouteName values above. RFC 7432 Section 7 and RFC 9136 Section 3 assign the
// numbers.
const (
	routeTypeToken1 = "type1"
	routeTypeToken2 = "type2"
	routeTypeToken3 = "type3"
	routeTypeToken4 = "type4"
	routeTypeToken5 = "type5"
)

// Names of the EVPN route fields. Each name is both an encode keyword and a key
// of the decoded JSON object, so the two surfaces cannot drift apart.
const (
	fieldRD          = "rd"
	fieldESI         = "esi"
	fieldEthernetTag = "ethernet-tag"
	fieldMAC         = "mac"
	fieldIP          = "ip"
	fieldPrefix      = "prefix"
	fieldGateway     = "gateway"
	fieldLabel       = "label"
)

// String returns the route type name.
func (t EVPNRouteType) String() string {
	switch t {
	case EVPNRouteType1:
		return RouteNameEthernetAutoDiscovery
	case EVPNRouteType2:
		return RouteNameMACIPAdvertisement
	case EVPNRouteType3:
		return RouteNameInclusiveMulticast
	case EVPNRouteType4:
		return RouteNameEthernetSegment
	case EVPNRouteType5:
		return RouteNameIPPrefix
	}
	return textbuf.StrInt("evpn-type-", int64(t))
}

// ESI represents a 10-byte Ethernet Segment Identifier.
// RFC 7432 Section 5 defines the ESI format and types.
type ESI [10]byte

// IsZero returns true if ESI is all zeros.
func (e ESI) IsZero() bool { return e == ESI{} }

// String returns hex representation.
func (e ESI) String() string {
	return appendColonHex(nil, e[:])
}

func appendColonHex(dst, data []byte) string {
	const hextable = "0123456789abcdef"
	buf := make([]byte, 0, len(data)*3-1)
	buf = append(buf, dst...)
	for i, b := range data {
		if i > 0 {
			buf = append(buf, ':')
		}
		buf = append(buf, hextable[b>>4], hextable[b&0x0f])
	}
	return string(buf)
}

// ParseESIString parses an Ethernet Segment Identifier from string format.
func ParseESIString(s string) (ESI, error) {
	var esi ESI
	if s == "0" || s == "" {
		return esi, nil
	}

	if strings.Contains(s, ":") {
		parts := strings.Split(s, ":")
		if len(parts) != 10 {
			return esi, fmt.Errorf("invalid ESI format: expected 10 parts, got %d", len(parts))
		}
		for i, p := range parts {
			b, err := strconv.ParseUint(p, 16, 8)
			if err != nil {
				return esi, fmt.Errorf("invalid ESI byte %d: %s", i, p)
			}
			esi[i] = byte(b)
		}
		return esi, nil
	}

	if len(s) == 20 {
		decoded, err := hex.DecodeString(s)
		if err != nil {
			return esi, fmt.Errorf("invalid ESI hex: %w", err)
		}
		copy(esi[:], decoded)
		return esi, nil
	}

	return esi, fmt.Errorf("invalid ESI format: %s", s)
}

// parseOriginatorIP parses an originator IP from wire format.
// Returns the parsed IP address and the number of bytes consumed (ipLen/8).
// RFC 7432 Section 7.3/7.4: IP length is in bits (32 or 128).
func parseOriginatorIP(data []byte, offset int, ipLen byte) (netip.Addr, int, error) {
	if ipLen == 32 {
		if offset+4 > len(data) {
			return netip.Addr{}, 0, ErrEVPNTruncated
		}
		return netip.AddrFrom4([4]byte(data[offset : offset+4])), 4, nil
	}
	if ipLen == 128 {
		if offset+16 > len(data) {
			return netip.Addr{}, 0, ErrEVPNTruncated
		}
		return netip.AddrFrom16([16]byte(data[offset : offset+16])), 16, nil
	}
	return netip.Addr{}, 0, ErrEVPNInvalidAddress
}

// EVPN is the interface for all EVPN route types.
type EVPN interface {
	RouteType() EVPNRouteType
	RD() RouteDistinguisher
	Family() Family
	Bytes() []byte
	Len() int
	String() string
	PathID() uint32
	HasPathID() bool
	SupportsAddPath() bool
	WriteTo(buf []byte, off int) int
}

// ParseEVPN parses an EVPN NLRI from wire format.
// RFC 7432 Section 7: AFI 25 (L2VPN) and SAFI 70 (EVPN).
func ParseEVPN(data []byte, addpath bool) (EVPN, []byte, error) {
	if len(data) < 2 {
		return nil, nil, ErrEVPNTruncated
	}

	offset := 0
	var pathID uint32

	if addpath {
		if len(data) < 4 {
			return nil, nil, ErrEVPNTruncated
		}
		pathID = binary.BigEndian.Uint32(data[:4])
		offset = 4
	}

	if offset >= len(data) {
		return nil, nil, ErrEVPNTruncated
	}

	routeType := EVPNRouteType(data[offset])
	offset++

	if offset >= len(data) {
		return nil, nil, ErrEVPNTruncated
	}

	length := int(data[offset])
	offset++

	if offset+length > len(data) {
		return nil, nil, ErrEVPNTruncated
	}

	nlriData := data[offset : offset+length]

	var evpn EVPN
	var err error

	switch routeType {
	case EVPNRouteType1:
		evpn, err = parseEVPNType1(nlriData, pathID, addpath)
	case EVPNRouteType2:
		evpn, err = parseEVPNType2(nlriData, pathID, addpath)
	case EVPNRouteType3:
		evpn, err = parseEVPNType3(nlriData, pathID, addpath)
	case EVPNRouteType4:
		evpn, err = parseEVPNType4(nlriData, pathID, addpath)
	case EVPNRouteType5:
		evpn, err = parseEVPNType5(nlriData, pathID, addpath)
	case 0, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15: // Reserved/unknown route types
		evpn = &eVPNGeneric{routeType: routeType, data: nlriData, pathID: pathID, hasPath: addpath}
	}

	// Handle any other route type as generic
	if evpn == nil && err == nil {
		evpn = &eVPNGeneric{routeType: routeType, data: nlriData, pathID: pathID, hasPath: addpath}
	}

	if err != nil {
		return nil, nil, err
	}

	return evpn, data[offset+length:], nil
}

// EVPNType1 represents an Ethernet Auto-Discovery route (RFC 7432 Section 7.1).
type EVPNType1 struct {
	rd          RouteDistinguisher
	esi         ESI
	ethernetTag uint32
	labels      []uint32
	pathID      uint32
	hasPath     bool
}

func parseEVPNType1(data []byte, pathID uint32, hasPath bool) (*EVPNType1, error) {
	if len(data) < 8+10+4 {
		return nil, ErrEVPNTruncated
	}

	e := &EVPNType1{pathID: pathID, hasPath: hasPath}
	offset := 0

	rd, err := ParseRouteDistinguisher(data[offset : offset+8])
	if err != nil {
		return nil, err
	}
	e.rd = rd
	offset += 8

	copy(e.esi[:], data[offset:offset+10])
	offset += 10

	e.ethernetTag = binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	if offset < len(data) {
		labels, _, err := ParseLabelStack(data[offset:])
		if err != nil {
			return nil, err
		}
		e.labels = labels
	}

	return e, nil
}

func (e *EVPNType1) Family() Family           { return L2VPNEVPN }
func (e *EVPNType1) RouteType() EVPNRouteType { return EVPNRouteType1 }
func (e *EVPNType1) RD() RouteDistinguisher   { return e.rd }
func (e *EVPNType1) ESI() ESI                 { return e.esi }
func (e *EVPNType1) EthernetTag() uint32      { return e.ethernetTag }
func (e *EVPNType1) Labels() []uint32         { return e.labels }
func (e *EVPNType1) PathID() uint32           { return e.pathID }
func (e *EVPNType1) HasPathID() bool          { return e.hasPath }
func (e *EVPNType1) SupportsAddPath() bool    { return true }

// WriteTo encodes the EVPN Type 1 NLRI directly into buf at off. Returns
// bytes written. Zero-alloc primitive: label stack and RD are written
// in place via their own WriteTo helpers, never through an intermediate
// `make`.
func (e *EVPNType1) WriteTo(buf []byte, off int) int {
	payloadLen := 8 + 10 + 4 + len(e.labels)*3
	buf[off] = byte(EVPNRouteType1)
	buf[off+1] = byte(payloadLen)
	pos := off + 2
	pos += e.rd.WriteTo(buf, pos)
	copy(buf[pos:], e.esi[:])
	pos += 10
	binary.BigEndian.PutUint32(buf[pos:], e.ethernetTag)
	pos += 4
	pos += WriteLabelStack(buf, pos, e.labels)
	return pos - off
}

// Bytes returns a standalone wire encoding. Retained for JSON / test
// callers that need an owned slice; hot-path senders should call
// WriteTo directly into a pool buffer.
func (e *EVPNType1) Bytes() []byte {
	buf := make([]byte, e.Len())
	e.WriteTo(buf, 0)
	return buf
}

func (e *EVPNType1) Len() int {
	return 8 + 10 + 4 + len(e.labels)*3 + 2
}

func (e *EVPNType1) String() string {
	var b textbuf.Buffer
	b.Str("ethernet-ad rd ").Str(e.rd.String()).Str(" esi ").Str(e.esi.String()).Str(" etag ").Uint32(e.ethernetTag)
	if len(e.labels) > 0 {
		b.Str(" label ").Uint32(e.labels[0])
		for _, l := range e.labels[1:] {
			b.Byte(',').Uint32(l)
		}
	}
	return b.String()
}

// EVPNType2 represents a MAC/IP Advertisement route (RFC 7432 Section 7.2).
type EVPNType2 struct {
	rd          RouteDistinguisher
	esi         ESI
	ethernetTag uint32
	mac         [6]byte
	ip          netip.Addr
	labels      []uint32
	pathID      uint32
	hasPath     bool
}

func parseEVPNType2(data []byte, pathID uint32, hasPath bool) (*EVPNType2, error) {
	if len(data) < 8+10+4+1+6+1 {
		return nil, ErrEVPNTruncated
	}

	e := &EVPNType2{pathID: pathID, hasPath: hasPath}
	offset := 0

	rd, err := ParseRouteDistinguisher(data[offset : offset+8])
	if err != nil {
		return nil, err
	}
	e.rd = rd
	offset += 8

	copy(e.esi[:], data[offset:offset+10])
	offset += 10

	e.ethernetTag = binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	macLen := data[offset]
	offset++
	if macLen != 48 {
		return nil, ErrEVPNInvalidAddress
	}

	copy(e.mac[:], data[offset:offset+6])
	offset += 6

	ipLen := data[offset]
	offset++

	switch ipLen {
	case 0:
		// No IP address - valid per RFC 7432 Section 7.2
	case 32:
		if offset+4 > len(data) {
			return nil, ErrEVPNTruncated
		}
		e.ip = netip.AddrFrom4([4]byte(data[offset : offset+4]))
		offset += 4
	case 128:
		if offset+16 > len(data) {
			return nil, ErrEVPNTruncated
		}
		e.ip = netip.AddrFrom16([16]byte(data[offset : offset+16]))
		offset += 16
	default:
		// RFC 7432 Section 7.2: the IP Address Length field is expressed in
		// bits and takes exactly one of 0, 32 or 128. Every other value --
		// including values above 128 -- is malformed and is rejected here
		// rather than silently read as "no IP address".
		return nil, ErrEVPNInvalidAddress
	}

	if offset < len(data) {
		labels, _, err := ParseLabelStack(data[offset:])
		if err != nil {
			return nil, err
		}
		e.labels = labels
	}

	return e, nil
}

func (e *EVPNType2) Family() Family           { return L2VPNEVPN }
func (e *EVPNType2) RouteType() EVPNRouteType { return EVPNRouteType2 }
func (e *EVPNType2) RD() RouteDistinguisher   { return e.rd }
func (e *EVPNType2) ESI() ESI                 { return e.esi }
func (e *EVPNType2) EthernetTag() uint32      { return e.ethernetTag }
func (e *EVPNType2) MAC() [6]byte             { return e.mac }
func (e *EVPNType2) IP() netip.Addr           { return e.ip }
func (e *EVPNType2) Labels() []uint32         { return e.labels }
func (e *EVPNType2) PathID() uint32           { return e.pathID }
func (e *EVPNType2) HasPathID() bool          { return e.hasPath }
func (e *EVPNType2) SupportsAddPath() bool    { return true }

// WriteTo encodes the EVPN Type 2 NLRI directly into buf at off.
// Zero-alloc: label stack, RD, and IP are written in place.
func (e *EVPNType2) WriteTo(buf []byte, off int) int {
	ipLen, ipBytesLen := 0, 0
	switch {
	case e.ip.Is4():
		ipLen, ipBytesLen = 32, 4
	case e.ip.Is6():
		ipLen, ipBytesLen = 128, 16
	}
	payloadLen := 8 + 10 + 4 + 1 + 6 + 1 + ipBytesLen + len(e.labels)*3
	buf[off] = byte(EVPNRouteType2)
	buf[off+1] = byte(payloadLen)
	pos := off + 2
	pos += e.rd.WriteTo(buf, pos)
	copy(buf[pos:], e.esi[:])
	pos += 10
	binary.BigEndian.PutUint32(buf[pos:], e.ethernetTag)
	pos += 4
	buf[pos] = 48 // RFC 7432: MAC Addr Length (bits)
	pos++
	copy(buf[pos:], e.mac[:])
	pos += 6
	buf[pos] = byte(ipLen)
	pos++
	switch ipBytesLen {
	case 4:
		ip4 := e.ip.As4()
		copy(buf[pos:], ip4[:])
		pos += 4
	case 16:
		ip6 := e.ip.As16()
		copy(buf[pos:], ip6[:])
		pos += 16
	}
	pos += WriteLabelStack(buf, pos, e.labels)
	return pos - off
}

// Bytes returns a standalone wire encoding. Retained for JSON / test
// callers; hot-path senders should call WriteTo into a pool buffer.
func (e *EVPNType2) Bytes() []byte {
	buf := make([]byte, e.Len())
	e.WriteTo(buf, 0)
	return buf
}

func (e *EVPNType2) Len() int {
	n := 8 + 10 + 4 + 1 + 6 + 1
	if e.ip.IsValid() {
		if e.ip.Is4() {
			n += 4
		} else {
			n += 16
		}
	}
	return n + len(e.labels)*3 + 2
}

func (e *EVPNType2) String() string {
	var b textbuf.Buffer
	b.Str("mac-ip rd ").Str(e.rd.String()).Str(" mac ").MAC(e.mac[:])
	if e.ip.IsValid() {
		b.Str(" ip ").Addr(e.ip)
	}
	if e.ethernetTag != 0 {
		b.Str(" etag ").Uint32(e.ethernetTag)
	}
	if len(e.labels) > 0 {
		b.Str(" label ").Uint32(e.labels[0])
		for _, l := range e.labels[1:] {
			b.Byte(',').Uint32(l)
		}
	}
	return b.String()
}

// EVPNType3 represents an Inclusive Multicast Ethernet Tag route (RFC 7432 Section 7.3).
type EVPNType3 struct {
	rd           RouteDistinguisher
	ethernetTag  uint32
	originatorIP netip.Addr
	pathID       uint32
	hasPath      bool
}

func parseEVPNType3(data []byte, pathID uint32, hasPath bool) (*EVPNType3, error) {
	if len(data) < 8+4+1 {
		return nil, ErrEVPNTruncated
	}

	e := &EVPNType3{pathID: pathID, hasPath: hasPath}
	offset := 0

	rd, err := ParseRouteDistinguisher(data[offset : offset+8])
	if err != nil {
		return nil, err
	}
	e.rd = rd
	offset += 8

	e.ethernetTag = binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	ipLen := data[offset]
	offset++

	ip, _, err := parseOriginatorIP(data, offset, ipLen)
	if err != nil {
		return nil, err
	}
	e.originatorIP = ip

	return e, nil
}

func (e *EVPNType3) Family() Family           { return L2VPNEVPN }
func (e *EVPNType3) RouteType() EVPNRouteType { return EVPNRouteType3 }
func (e *EVPNType3) RD() RouteDistinguisher   { return e.rd }
func (e *EVPNType3) EthernetTag() uint32      { return e.ethernetTag }
func (e *EVPNType3) OriginatorIP() netip.Addr { return e.originatorIP }
func (e *EVPNType3) PathID() uint32           { return e.pathID }
func (e *EVPNType3) HasPathID() bool          { return e.hasPath }
func (e *EVPNType3) SupportsAddPath() bool    { return true }

// WriteTo encodes the EVPN Type 3 NLRI directly into buf at off.
// Zero-alloc: RD and IP are written in place.
func (e *EVPNType3) WriteTo(buf []byte, off int) int {
	ipLen, ipBytesLen := 128, 16
	if e.originatorIP.Is4() {
		ipLen, ipBytesLen = 32, 4
	}
	payloadLen := 8 + 4 + 1 + ipBytesLen
	buf[off] = byte(EVPNRouteType3)
	buf[off+1] = byte(payloadLen)
	pos := off + 2
	pos += e.rd.WriteTo(buf, pos)
	binary.BigEndian.PutUint32(buf[pos:], e.ethernetTag)
	pos += 4
	buf[pos] = byte(ipLen)
	pos++
	switch ipBytesLen {
	case 4:
		ip4 := e.originatorIP.As4()
		copy(buf[pos:], ip4[:])
		pos += 4
	case 16:
		ip6 := e.originatorIP.As16()
		copy(buf[pos:], ip6[:])
		pos += 16
	}
	return pos - off
}

// Bytes returns a standalone wire encoding. Retained for JSON / test
// callers; hot-path senders should call WriteTo into a pool buffer.
func (e *EVPNType3) Bytes() []byte {
	buf := make([]byte, e.Len())
	e.WriteTo(buf, 0)
	return buf
}

func (e *EVPNType3) Len() int {
	n := 8 + 4 + 1
	if e.originatorIP.Is4() {
		n += 4
	} else {
		n += 16
	}
	return n + 2
}

func (e *EVPNType3) String() string {
	var b textbuf.Buffer
	b.Str("multicast rd ").Str(e.rd.String()).Str(" ip ").Addr(e.originatorIP)
	if e.ethernetTag != 0 {
		b.Str(" etag ").Uint32(e.ethernetTag)
	}
	return b.String()
}

// EVPNType4 represents an Ethernet Segment route (RFC 7432 Section 7.4).
type EVPNType4 struct {
	rd           RouteDistinguisher
	esi          ESI
	originatorIP netip.Addr
	pathID       uint32
	hasPath      bool
}

func parseEVPNType4(data []byte, pathID uint32, hasPath bool) (*EVPNType4, error) {
	if len(data) < 8+10+1 {
		return nil, ErrEVPNTruncated
	}

	e := &EVPNType4{pathID: pathID, hasPath: hasPath}
	offset := 0

	rd, err := ParseRouteDistinguisher(data[offset : offset+8])
	if err != nil {
		return nil, err
	}
	e.rd = rd
	offset += 8

	copy(e.esi[:], data[offset:offset+10])
	offset += 10

	ipLen := data[offset]
	offset++

	switch ipLen {
	case 32:
		if offset+4 > len(data) {
			return nil, ErrEVPNTruncated
		}
		e.originatorIP = netip.AddrFrom4([4]byte(data[offset : offset+4]))
	case 128:
		if offset+16 > len(data) {
			return nil, ErrEVPNTruncated
		}
		e.originatorIP = netip.AddrFrom16([16]byte(data[offset : offset+16]))
	default:
		// RFC 7432 Section 7.4: the Ethernet Segment route carries the
		// Originating Router's IP Address, so the length field is 32 or 128
		// bits. Every other value -- including values above 128 -- is
		// malformed and is rejected rather than leaving the address unset.
		return nil, ErrEVPNInvalidAddress
	}

	return e, nil
}

func (e *EVPNType4) Family() Family           { return L2VPNEVPN }
func (e *EVPNType4) RouteType() EVPNRouteType { return EVPNRouteType4 }
func (e *EVPNType4) RD() RouteDistinguisher   { return e.rd }
func (e *EVPNType4) ESI() ESI                 { return e.esi }
func (e *EVPNType4) OriginatorIP() netip.Addr { return e.originatorIP }
func (e *EVPNType4) PathID() uint32           { return e.pathID }
func (e *EVPNType4) HasPathID() bool          { return e.hasPath }
func (e *EVPNType4) SupportsAddPath() bool    { return true }

// WriteTo encodes the EVPN Type 4 NLRI directly into buf at off.
// Zero-alloc: RD and IP are written in place.
func (e *EVPNType4) WriteTo(buf []byte, off int) int {
	ipLen, ipBytesLen := 128, 16
	if e.originatorIP.Is4() {
		ipLen, ipBytesLen = 32, 4
	}
	payloadLen := 8 + 10 + 1 + ipBytesLen
	buf[off] = byte(EVPNRouteType4)
	buf[off+1] = byte(payloadLen)
	pos := off + 2
	pos += e.rd.WriteTo(buf, pos)
	copy(buf[pos:], e.esi[:])
	pos += 10
	buf[pos] = byte(ipLen)
	pos++
	switch ipBytesLen {
	case 4:
		ip4 := e.originatorIP.As4()
		copy(buf[pos:], ip4[:])
		pos += 4
	case 16:
		ip6 := e.originatorIP.As16()
		copy(buf[pos:], ip6[:])
		pos += 16
	}
	return pos - off
}

// Bytes returns a standalone wire encoding. Retained for JSON / test
// callers; hot-path senders should call WriteTo into a pool buffer.
func (e *EVPNType4) Bytes() []byte {
	buf := make([]byte, e.Len())
	e.WriteTo(buf, 0)
	return buf
}

func (e *EVPNType4) Len() int {
	n := 8 + 10 + 1
	if e.originatorIP.Is4() {
		n += 4
	} else {
		n += 16
	}
	return n + 2
}

func (e *EVPNType4) String() string {
	var b textbuf.Buffer
	return b.Str("ethernet-segment rd ").Str(e.rd.String()).Str(" esi ").Str(e.esi.String()).Str(" ip ").Addr(e.originatorIP).String()
}

// EVPNType5 represents an IP Prefix route (RFC 9136 Section 3).
type EVPNType5 struct {
	rd          RouteDistinguisher
	esi         ESI
	ethernetTag uint32
	prefix      netip.Prefix
	gateway     netip.Addr
	labels      []uint32
	pathID      uint32
	hasPath     bool
}

func parseEVPNType5(data []byte, pathID uint32, hasPath bool) (*EVPNType5, error) {
	// RFC 9136: Length MUST be 34 (IPv4) or 58 (IPv6)
	if len(data) != 34 && len(data) != 58 {
		return nil, ErrEVPNInvalidAddress
	}

	e := &EVPNType5{pathID: pathID, hasPath: hasPath}
	offset := 0

	rd, err := ParseRouteDistinguisher(data[offset : offset+8])
	if err != nil {
		return nil, err
	}
	e.rd = rd
	offset += 8

	copy(e.esi[:], data[offset:offset+10])
	offset += 10

	e.ethernetTag = binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	ipLen := int(data[offset])
	offset++

	var addr netip.Addr
	if len(data) == 34 {
		if ipLen > 32 {
			return nil, ErrEVPNInvalidPrefix
		}
		var ip [4]byte
		copy(ip[:], data[offset:offset+4])
		addr = netip.AddrFrom4(ip)
		offset += 4
		e.gateway = netip.AddrFrom4([4]byte(data[offset : offset+4]))
		offset += 4
	} else {
		if ipLen > 128 {
			return nil, ErrEVPNInvalidPrefix
		}
		var ip [16]byte
		copy(ip[:], data[offset:offset+16])
		addr = netip.AddrFrom16(ip)
		offset += 16
		e.gateway = netip.AddrFrom16([16]byte(data[offset : offset+16]))
		offset += 16
	}

	prefix, err := addr.Prefix(ipLen)
	if err != nil {
		return nil, ErrEVPNInvalidPrefix
	}
	e.prefix = prefix

	if offset < len(data) {
		labels, _, err := ParseLabelStack(data[offset:])
		if err != nil {
			return nil, err
		}
		e.labels = labels
	}

	return e, nil
}

func (e *EVPNType5) Family() Family           { return L2VPNEVPN }
func (e *EVPNType5) RouteType() EVPNRouteType { return EVPNRouteType5 }
func (e *EVPNType5) RD() RouteDistinguisher   { return e.rd }
func (e *EVPNType5) ESI() ESI                 { return e.esi }
func (e *EVPNType5) EthernetTag() uint32      { return e.ethernetTag }
func (e *EVPNType5) Prefix() netip.Prefix     { return e.prefix }
func (e *EVPNType5) Gateway() netip.Addr      { return e.gateway }
func (e *EVPNType5) Labels() []uint32         { return e.labels }
func (e *EVPNType5) PathID() uint32           { return e.pathID }
func (e *EVPNType5) HasPathID() bool          { return e.hasPath }
func (e *EVPNType5) SupportsAddPath() bool    { return true }

// WriteTo encodes the EVPN Type 5 NLRI directly into buf at off.
// Zero-alloc: label stack, RD, prefix, and gateway are written in place.
func (e *EVPNType5) WriteTo(buf []byte, off int) int {
	prefixSize := 16
	if e.prefix.Addr().Is4() {
		prefixSize = 4
	}
	payloadLen := 8 + 10 + 4 + 1 + prefixSize + prefixSize + len(e.labels)*3
	buf[off] = byte(EVPNRouteType5)
	buf[off+1] = byte(payloadLen)
	pos := off + 2
	pos += e.rd.WriteTo(buf, pos)
	copy(buf[pos:], e.esi[:])
	pos += 10
	binary.BigEndian.PutUint32(buf[pos:], e.ethernetTag)
	pos += 4
	buf[pos] = byte(e.prefix.Bits())
	pos++
	switch prefixSize {
	case 4:
		ip4 := e.prefix.Addr().As4()
		copy(buf[pos:], ip4[:])
		pos += 4
		if e.gateway.IsValid() {
			gw4 := e.gateway.As4()
			copy(buf[pos:], gw4[:])
		}
		pos += 4
	case 16:
		ip6 := e.prefix.Addr().As16()
		copy(buf[pos:], ip6[:])
		pos += 16
		if e.gateway.IsValid() {
			gw6 := e.gateway.As16()
			copy(buf[pos:], gw6[:])
		}
		pos += 16
	}
	pos += WriteLabelStack(buf, pos, e.labels)
	return pos - off
}

// Bytes returns a standalone wire encoding. Retained for JSON / test
// callers; hot-path senders should call WriteTo into a pool buffer.
func (e *EVPNType5) Bytes() []byte {
	buf := make([]byte, e.Len())
	e.WriteTo(buf, 0)
	return buf
}

func (e *EVPNType5) Len() int {
	if e.prefix.Addr().Is4() {
		return 34 + 2
	}
	return 58 + 2
}

func (e *EVPNType5) String() string {
	var b textbuf.Buffer
	b.Str("ip-prefix rd ").Str(e.rd.String()).Str(" prefix ").Prefix(e.prefix)
	if !e.esi.IsZero() {
		b.Str(" esi ").Str(e.esi.String())
	}
	if e.ethernetTag != 0 {
		b.Str(" etag ").Uint32(e.ethernetTag)
	}
	if e.gateway.IsValid() && !e.gateway.IsUnspecified() {
		b.Str(" gateway ").Addr(e.gateway)
	}
	if len(e.labels) > 0 {
		b.Str(" label ").Uint32(e.labels[0])
		for _, l := range e.labels[1:] {
			b.Byte(',').Uint32(l)
		}
	}
	return b.String()
}

// eVPNGeneric holds unparsed EVPN routes.
type eVPNGeneric struct {
	routeType EVPNRouteType
	data      []byte
	pathID    uint32
	hasPath   bool
}

func (e *eVPNGeneric) Family() Family           { return L2VPNEVPN }
func (e *eVPNGeneric) RouteType() EVPNRouteType { return e.routeType }
func (e *eVPNGeneric) RD() RouteDistinguisher   { return RouteDistinguisher{} }
func (e *eVPNGeneric) PathID() uint32           { return e.pathID }
func (e *eVPNGeneric) HasPathID() bool          { return e.hasPath }
func (e *eVPNGeneric) SupportsAddPath() bool    { return true }
func (e *eVPNGeneric) String() string           { return textbuf.StrInt("evpn-type", int64(e.routeType)) }

// Bytes returns a standalone wire encoding, header included, matching every other EVPN
// route type. It used to return the bare body, so a caller that round-trips an
// unrecognized route through encode.go emitted an NLRI with no [type][length] header.
func (e *eVPNGeneric) Bytes() []byte {
	buf := make([]byte, e.Len())
	e.WriteTo(buf, 0)
	return buf
}

func (e *eVPNGeneric) Len() int { return len(e.data) + evpnHeaderLen }

// WriteTo writes the full [route-type][length][body] encoding, like every other EVPN type,
// and returns the total written.
//
// It used to copy only the body, which broke the two contracts its callers rely on:
// Len() promised len(data)+2, so plugin.go's `make([]byte, Len())` + WriteTo left two
// trailing zero octets on the wire; and appendRawField (json.go) skips `scratch[:2]` to
// drop the header, so the JSON "raw" field lost the first two octets of the body instead.
//
// e.data comes from ParseEVPN, where the length is a single octet, so it cannot exceed 255.
func (e *eVPNGeneric) WriteTo(buf []byte, off int) int {
	buf[off] = byte(e.routeType)
	buf[off+1] = byte(len(e.data))
	pos := off + evpnHeaderLen
	pos += copy(buf[pos:], e.data)
	return pos - off
}

// Constructors for creating EVPN routes.

// NewEVPNType1 creates an Ethernet Auto-Discovery route (Type 1).
func NewEVPNType1(rd RouteDistinguisher, esi [10]byte, ethernetTag uint32, labels []uint32) *EVPNType1 {
	return &EVPNType1{rd: rd, esi: esi, ethernetTag: ethernetTag, labels: labels}
}

// NewEVPNType2 creates a MAC/IP Advertisement route (Type 2).
func NewEVPNType2(rd RouteDistinguisher, esi [10]byte, ethernetTag uint32, mac [6]byte, ip netip.Addr, labels []uint32) *EVPNType2 {
	return &EVPNType2{rd: rd, esi: esi, ethernetTag: ethernetTag, mac: mac, ip: ip, labels: labels}
}

// NewEVPNType3 creates an Inclusive Multicast Ethernet Tag route (Type 3).
func NewEVPNType3(rd RouteDistinguisher, ethernetTag uint32, originatorIP netip.Addr) *EVPNType3 {
	return &EVPNType3{rd: rd, ethernetTag: ethernetTag, originatorIP: originatorIP}
}

// NewEVPNType4 creates an Ethernet Segment route (Type 4).
func NewEVPNType4(rd RouteDistinguisher, esi [10]byte, originatorIP netip.Addr) *EVPNType4 {
	return &EVPNType4{rd: rd, esi: esi, originatorIP: originatorIP}
}

// newEVPNType5 creates an IP Prefix route (Type 5).
func newEVPNType5(rd RouteDistinguisher, esi [10]byte, ethernetTag uint32, prefix netip.Prefix, gateway netip.Addr, labels []uint32) *EVPNType5 {
	return &EVPNType5{rd: rd, esi: esi, ethernetTag: ethernetTag, prefix: prefix, gateway: gateway, labels: labels}
}

// eVPNFamilies returns the address families this plugin can decode.
func eVPNFamilies() []string {
	return []string{familyNameEVPN}
}
