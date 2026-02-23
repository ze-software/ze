// Design: docs/architecture/wire/nlri-evpn.md — EVPN NLRI plugin
// RFC: rfc/short/rfc7432.md
//
// Package evpn implements EVPN NLRI types for the evpn plugin.
// RFC 7432: BGP MPLS-Based Ethernet VPN
// RFC 9136: IP Prefix Advertisement in EVPN
package bgp_nlri_evpn

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
)

// Type aliases for nlri types used by EVPN.
type (
	Family             = nlri.Family
	RouteDistinguisher = nlri.RouteDistinguisher
)

// Re-export constants from nlri for local use.
var L2VPNEVPN = nlri.L2VPNEVPN

// Re-export parsing functions.
var (
	ParseRouteDistinguisher = nlri.ParseRouteDistinguisher
	ParseLabelStack         = nlri.ParseLabelStack
	EncodeLabelStack        = nlri.EncodeLabelStack
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
const (
	RouteNameEthernetAutoDiscovery = "ethernet-auto-discovery"
	RouteNameMACIPAdvertisement    = "mac-ip-advertisement"
	RouteNameInclusiveMulticast    = "inclusive-multicast"
	RouteNameEthernetSegment       = "ethernet-segment"
	RouteNameIPPrefix              = "ip-prefix"
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
	return fmt.Sprintf("evpn-type-%d", t)
}

// ESI represents a 10-byte Ethernet Segment Identifier.
// RFC 7432 Section 5 defines the ESI format and types.
type ESI [10]byte

// IsZero returns true if ESI is all zeros.
func (e ESI) IsZero() bool { return e == ESI{} }

// String returns hex representation.
func (e ESI) String() string {
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x",
		e[0], e[1], e[2], e[3], e[4], e[5], e[6], e[7], e[8], e[9])
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
		evpn = &EVPNGeneric{routeType: routeType, data: nlriData, pathID: pathID, hasPath: addpath}
	}

	// Handle any other route type as generic
	if evpn == nil && err == nil {
		evpn = &EVPNGeneric{routeType: routeType, data: nlriData, pathID: pathID, hasPath: addpath}
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

func (e *EVPNType1) Bytes() []byte {
	labelBytes := EncodeLabelStack(e.labels)
	payloadLen := 8 + 10 + 4 + len(labelBytes)

	buf := make([]byte, 2+payloadLen)
	buf[0] = byte(EVPNRouteType1)
	buf[1] = byte(payloadLen)

	offset := 2
	copy(buf[offset:], e.rd.Bytes())
	offset += 8
	copy(buf[offset:], e.esi[:])
	offset += 10
	binary.BigEndian.PutUint32(buf[offset:], e.ethernetTag)
	offset += 4
	copy(buf[offset:], labelBytes)

	return buf
}

func (e *EVPNType1) Len() int {
	return 8 + 10 + 4 + len(e.labels)*3 + 2
}

func (e *EVPNType1) String() string {
	var sb strings.Builder
	sb.WriteString("ethernet-ad rd set ")
	sb.WriteString(e.rd.String())
	sb.WriteString(" esi set ")
	sb.WriteString(e.esi.String())
	sb.WriteString(" etag set ")
	fmt.Fprintf(&sb, "%d", e.ethernetTag)
	if len(e.labels) > 0 {
		sb.WriteString(" label set ")
		fmt.Fprintf(&sb, "%d", e.labels[0])
		for _, l := range e.labels[1:] {
			fmt.Fprintf(&sb, ",%d", l)
		}
	}
	return sb.String()
}

func (e *EVPNType1) WriteTo(buf []byte, off int) int { return copy(buf[off:], e.Bytes()) }

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
	case 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31,
		33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48,
		49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63, 64,
		65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79, 80,
		81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95, 96,
		97, 98, 99, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110,
		111, 112, 113, 114, 115, 116, 117, 118, 119, 120, 121, 122, 123,
		124, 125, 126, 127:
		// Invalid IP lengths per RFC 7432 Section 7.2
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

func (e *EVPNType2) Bytes() []byte {
	labelBytes := EncodeLabelStack(e.labels)

	ipLen := 0
	var ipBytes []byte
	if e.ip.IsValid() {
		if e.ip.Is4() {
			ipLen = 32
			ip4 := e.ip.As4()
			ipBytes = ip4[:]
		} else {
			ipLen = 128
			ip6 := e.ip.As16()
			ipBytes = ip6[:]
		}
	}

	payloadLen := 8 + 10 + 4 + 1 + 6 + 1 + len(ipBytes) + len(labelBytes)

	buf := make([]byte, 2+payloadLen)
	buf[0] = byte(EVPNRouteType2)
	buf[1] = byte(payloadLen)

	offset := 2
	copy(buf[offset:], e.rd.Bytes())
	offset += 8
	copy(buf[offset:], e.esi[:])
	offset += 10
	binary.BigEndian.PutUint32(buf[offset:], e.ethernetTag)
	offset += 4
	buf[offset] = 48
	offset++
	copy(buf[offset:], e.mac[:])
	offset += 6
	buf[offset] = byte(ipLen)
	offset++
	if len(ipBytes) > 0 {
		copy(buf[offset:], ipBytes)
		offset += len(ipBytes)
	}
	copy(buf[offset:], labelBytes)

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
	var sb strings.Builder
	sb.WriteString("mac-ip rd set ")
	sb.WriteString(e.rd.String())
	sb.WriteString(" mac set ")
	fmt.Fprintf(&sb, "%02x:%02x:%02x:%02x:%02x:%02x",
		e.mac[0], e.mac[1], e.mac[2], e.mac[3], e.mac[4], e.mac[5])
	if e.ip.IsValid() {
		sb.WriteString(" ip set ")
		sb.WriteString(e.ip.String())
	}
	if e.ethernetTag != 0 {
		fmt.Fprintf(&sb, " etag set %d", e.ethernetTag)
	}
	if len(e.labels) > 0 {
		fmt.Fprintf(&sb, " label set %d", e.labels[0])
		for _, l := range e.labels[1:] {
			fmt.Fprintf(&sb, ",%d", l)
		}
	}
	return sb.String()
}

func (e *EVPNType2) WriteTo(buf []byte, off int) int { return copy(buf[off:], e.Bytes()) }

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

func (e *EVPNType3) Bytes() []byte {
	var ipLen int
	var ipBytes []byte
	if e.originatorIP.Is4() {
		ipLen = 32
		ip4 := e.originatorIP.As4()
		ipBytes = ip4[:]
	} else {
		ipLen = 128
		ip6 := e.originatorIP.As16()
		ipBytes = ip6[:]
	}

	payloadLen := 8 + 4 + 1 + len(ipBytes)

	buf := make([]byte, 2+payloadLen)
	buf[0] = byte(EVPNRouteType3)
	buf[1] = byte(payloadLen)

	offset := 2
	copy(buf[offset:], e.rd.Bytes())
	offset += 8
	binary.BigEndian.PutUint32(buf[offset:], e.ethernetTag)
	offset += 4
	buf[offset] = byte(ipLen)
	offset++
	copy(buf[offset:], ipBytes)

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
	var sb strings.Builder
	sb.WriteString("multicast rd set ")
	sb.WriteString(e.rd.String())
	sb.WriteString(" ip set ")
	sb.WriteString(e.originatorIP.String())
	if e.ethernetTag != 0 {
		fmt.Fprintf(&sb, " etag set %d", e.ethernetTag)
	}
	return sb.String()
}

func (e *EVPNType3) WriteTo(buf []byte, off int) int { return copy(buf[off:], e.Bytes()) }

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
	case 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31,
		33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48,
		49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63, 64,
		65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79, 80,
		81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95, 96,
		97, 98, 99, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110,
		111, 112, 113, 114, 115, 116, 117, 118, 119, 120, 121, 122, 123,
		124, 125, 126, 127:
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

func (e *EVPNType4) Bytes() []byte {
	var ipLen int
	var ipBytes []byte
	if e.originatorIP.Is4() {
		ipLen = 32
		ip4 := e.originatorIP.As4()
		ipBytes = ip4[:]
	} else {
		ipLen = 128
		ip6 := e.originatorIP.As16()
		ipBytes = ip6[:]
	}

	payloadLen := 8 + 10 + 1 + len(ipBytes)

	buf := make([]byte, 2+payloadLen)
	buf[0] = byte(EVPNRouteType4)
	buf[1] = byte(payloadLen)

	offset := 2
	copy(buf[offset:], e.rd.Bytes())
	offset += 8
	copy(buf[offset:], e.esi[:])
	offset += 10
	buf[offset] = byte(ipLen)
	offset++
	copy(buf[offset:], ipBytes)

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
	var sb strings.Builder
	sb.WriteString("ethernet-segment rd set ")
	sb.WriteString(e.rd.String())
	sb.WriteString(" esi set ")
	sb.WriteString(e.esi.String())
	sb.WriteString(" ip set ")
	sb.WriteString(e.originatorIP.String())
	return sb.String()
}

func (e *EVPNType4) WriteTo(buf []byte, off int) int { return copy(buf[off:], e.Bytes()) }

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

func (e *EVPNType5) Bytes() []byte {
	labelBytes := EncodeLabelStack(e.labels)

	var prefixSize int
	if e.prefix.Addr().Is4() {
		prefixSize = 4
	} else {
		prefixSize = 16
	}

	payloadLen := 8 + 10 + 4 + 1 + prefixSize + prefixSize + len(labelBytes)

	buf := make([]byte, 2+payloadLen)
	buf[0] = byte(EVPNRouteType5)
	buf[1] = byte(payloadLen)

	offset := 2
	copy(buf[offset:], e.rd.Bytes())
	offset += 8
	copy(buf[offset:], e.esi[:])
	offset += 10
	binary.BigEndian.PutUint32(buf[offset:], e.ethernetTag)
	offset += 4
	buf[offset] = byte(e.prefix.Bits())
	offset++

	if prefixSize == 4 {
		ip4 := e.prefix.Addr().As4()
		copy(buf[offset:], ip4[:])
		offset += 4
		if e.gateway.IsValid() {
			gw4 := e.gateway.As4()
			copy(buf[offset:], gw4[:])
		}
		offset += 4
	} else {
		ip6 := e.prefix.Addr().As16()
		copy(buf[offset:], ip6[:])
		offset += 16
		if e.gateway.IsValid() {
			gw6 := e.gateway.As16()
			copy(buf[offset:], gw6[:])
		}
		offset += 16
	}

	copy(buf[offset:], labelBytes)

	return buf
}

func (e *EVPNType5) Len() int {
	if e.prefix.Addr().Is4() {
		return 34 + 2
	}
	return 58 + 2
}

func (e *EVPNType5) String() string {
	var sb strings.Builder
	sb.WriteString("ip-prefix rd set ")
	sb.WriteString(e.rd.String())
	sb.WriteString(" prefix set ")
	sb.WriteString(e.prefix.String())
	if !e.esi.IsZero() {
		sb.WriteString(" esi set ")
		sb.WriteString(e.esi.String())
	}
	if e.ethernetTag != 0 {
		fmt.Fprintf(&sb, " etag set %d", e.ethernetTag)
	}
	if e.gateway.IsValid() && !e.gateway.IsUnspecified() {
		sb.WriteString(" gateway set ")
		sb.WriteString(e.gateway.String())
	}
	if len(e.labels) > 0 {
		fmt.Fprintf(&sb, " label set %d", e.labels[0])
		for _, l := range e.labels[1:] {
			fmt.Fprintf(&sb, ",%d", l)
		}
	}
	return sb.String()
}

func (e *EVPNType5) WriteTo(buf []byte, off int) int { return copy(buf[off:], e.Bytes()) }

// EVPNGeneric holds unparsed EVPN routes.
type EVPNGeneric struct {
	routeType EVPNRouteType
	data      []byte
	pathID    uint32
	hasPath   bool
}

func (e *EVPNGeneric) Family() Family           { return L2VPNEVPN }
func (e *EVPNGeneric) RouteType() EVPNRouteType { return e.routeType }
func (e *EVPNGeneric) RD() RouteDistinguisher   { return RouteDistinguisher{} }
func (e *EVPNGeneric) PathID() uint32           { return e.pathID }
func (e *EVPNGeneric) HasPathID() bool          { return e.hasPath }
func (e *EVPNGeneric) SupportsAddPath() bool    { return true }
func (e *EVPNGeneric) Bytes() []byte            { return e.data }
func (e *EVPNGeneric) Len() int                 { return len(e.data) + 2 }
func (e *EVPNGeneric) String() string           { return fmt.Sprintf("evpn-type%d", e.routeType) }
func (e *EVPNGeneric) WriteTo(buf []byte, off int) int {
	return copy(buf[off:], e.Bytes())
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

// NewEVPNType5 creates an IP Prefix route (Type 5).
func NewEVPNType5(rd RouteDistinguisher, esi [10]byte, ethernetTag uint32, prefix netip.Prefix, gateway netip.Addr, labels []uint32) *EVPNType5 {
	return &EVPNType5{rd: rd, esi: esi, ethernetTag: ethernetTag, prefix: prefix, gateway: gateway, labels: labels}
}

// EVPNFamilies returns the address families this plugin can decode.
func EVPNFamilies() []string {
	return []string{"l2vpn/evpn"}
}
