// Package nlri provides EVPN NLRI parsing and encoding.
//
// EVPN is defined in RFC 7432 (BGP MPLS-Based Ethernet VPN).
// EVPN Type 5 (IP Prefix) is defined in RFC 9136.
//
// RFC 7432 Section 7: BGP EVPN Routes
// RFC 7432 Section 7.1: Ethernet Auto-discovery Route (Type 1)
// RFC 7432 Section 7.2: MAC/IP Advertisement Route (Type 2)
// RFC 7432 Section 7.3: Inclusive Multicast Ethernet Tag Route (Type 3)
// RFC 7432 Section 7.4: Ethernet Segment Route (Type 4)
// RFC 9136 Section 3: IP Prefix Route (Type 5)
package nlri

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

// EVPNRouteType identifies the EVPN route type.
// RFC 7432 Section 7 defines the EVPN NLRI format:
//
//	+-----------------------------------+
//	|    Route Type (1 octet)           |
//	+-----------------------------------+
//	|     Length (1 octet)              |
//	+-----------------------------------+
//	| Route Type specific (variable)    |
//	+-----------------------------------+
type EVPNRouteType uint8

// EVPN Route Types per RFC 7432 Section 7 and RFC 9136.
const (
	// EVPNRouteType1 is Ethernet Auto-Discovery (RFC 7432 Section 7.1).
	EVPNRouteType1 EVPNRouteType = 1
	// EVPNRouteType2 is MAC/IP Advertisement (RFC 7432 Section 7.2).
	EVPNRouteType2 EVPNRouteType = 2
	// EVPNRouteType3 is Inclusive Multicast Ethernet Tag (RFC 7432 Section 7.3).
	EVPNRouteType3 EVPNRouteType = 3
	// EVPNRouteType4 is Ethernet Segment (RFC 7432 Section 7.4).
	EVPNRouteType4 EVPNRouteType = 4
	// EVPNRouteType5 is IP Prefix (RFC 9136 Section 3).
	EVPNRouteType5 EVPNRouteType = 5
)

// String returns the route type name.
func (t EVPNRouteType) String() string {
	switch t {
	case EVPNRouteType1:
		return "ethernet-auto-discovery"
	case EVPNRouteType2:
		return "mac-ip-advertisement"
	case EVPNRouteType3:
		return "inclusive-multicast"
	case EVPNRouteType4:
		return "ethernet-segment"
	case EVPNRouteType5:
		return "ip-prefix"
	default:
		return fmt.Sprintf("evpn-type-%d", t)
	}
}

// ESI represents a 10-byte Ethernet Segment Identifier.
// RFC 7432 Section 5 defines the ESI format and types.
type ESI [10]byte

// IsZero returns true if ESI is all zeros.
func (e ESI) IsZero() bool {
	return e == ESI{}
}

// String returns hex representation.
func (e ESI) String() string {
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x",
		e[0], e[1], e[2], e[3], e[4], e[5], e[6], e[7], e[8], e[9])
}

// ParseESIString parses an Ethernet Segment Identifier from string format.
//
// RFC 7432 Section 5 defines ESI as a 10-byte identifier.
// Supports: "0" (all zeros), plain hex (20 chars), colon-separated (10 bytes).
func ParseESIString(s string) (ESI, error) {
	var esi ESI

	// Handle "0" or empty for all zeros
	if s == "0" || s == "" {
		return esi, nil
	}

	// Try colon-separated format (00:11:22:33:44:55:66:77:88:99)
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

	// Try plain hex format (20 chars)
	if len(s) == 20 {
		decoded, err := hex.DecodeString(s)
		if err != nil {
			return esi, fmt.Errorf("invalid ESI hex: %w", err)
		}
		copy(esi[:], decoded)
		return esi, nil
	}

	return esi, fmt.Errorf("invalid ESI format: %s (expected 0, 20 hex chars, or colon-separated)", s)
}

// EVPN is the interface for all EVPN route types.
// All EVPN routes carry a Route Distinguisher (RFC 7432 Section 7).
type EVPN interface {
	NLRI
	RouteType() EVPNRouteType
	RD() RouteDistinguisher
}

// ParseEVPN parses an EVPN NLRI from wire format.
// RFC 7432 Section 7: The EVPN NLRI is carried in BGP using BGP Multiprotocol
// Extensions with AFI 25 (L2VPN) and SAFI 70 (EVPN).
func ParseEVPN(data []byte, addpath bool) (NLRI, []byte, error) {
	if len(data) < 2 {
		return nil, nil, ErrShortRead
	}

	offset := 0
	var pathID uint32

	if addpath {
		if len(data) < 4 {
			return nil, nil, ErrShortRead
		}
		pathID = binary.BigEndian.Uint32(data[:4])
		offset = 4
	}

	if offset >= len(data) {
		return nil, nil, ErrShortRead
	}

	routeType := EVPNRouteType(data[offset])
	offset++

	if offset >= len(data) {
		return nil, nil, ErrShortRead
	}

	length := int(data[offset])
	offset++

	if offset+length > len(data) {
		return nil, nil, ErrShortRead
	}

	nlriData := data[offset : offset+length]

	var nlri NLRI
	var err error

	switch routeType { //nolint:exhaustive // Unsupported types handled as EVPNGeneric
	case EVPNRouteType1:
		nlri, err = parseEVPNType1(nlriData, pathID, addpath)
	case EVPNRouteType2:
		nlri, err = parseEVPNType2(nlriData, pathID, addpath)
	case EVPNRouteType3:
		nlri, err = parseEVPNType3(nlriData, pathID, addpath)
	case EVPNRouteType4:
		nlri, err = parseEVPNType4(nlriData, pathID, addpath)
	case EVPNRouteType5:
		nlri, err = parseEVPNType5(nlriData, pathID, addpath)
	default:
		// Generic EVPN for unsupported types
		nlri = &EVPNGeneric{
			routeType: routeType,
			data:      nlriData,
			pathID:    pathID,
			hasPath:   addpath,
		}
	}

	if err != nil {
		return nil, nil, err
	}

	return nlri, data[offset+length:], nil
}

// EVPNType1 represents an Ethernet Auto-Discovery route.
// RFC 7432 Section 7.1 defines the wire format:
//
//	+---------------------------------------+
//	|  Route Distinguisher (RD) (8 octets)  |
//	+---------------------------------------+
//	|Ethernet Segment Identifier (10 octets)|
//	+---------------------------------------+
//	|  Ethernet Tag ID (4 octets)           |
//	+---------------------------------------+
//	|  MPLS Label (3 octets)                |
//	+---------------------------------------+
//
// This route is used for multihoming fast convergence and aliasing.
// Per RFC 7432, only ESI and Ethernet Tag are part of the route key;
// the MPLS Label is a route attribute.
type EVPNType1 struct {
	rd          RouteDistinguisher
	esi         ESI
	ethernetTag uint32
	labels      []uint32
	pathID      uint32
	hasPath     bool
}

// parseEVPNType1 parses an Ethernet Auto-Discovery route per RFC 7432 Section 7.1.
func parseEVPNType1(data []byte, pathID uint32, hasPath bool) (*EVPNType1, error) {
	// RFC 7432 Section 7.1: RD (8) + ESI (10) + EthTag (4) + Label (3+)
	if len(data) < 8+10+4 {
		return nil, ErrShortRead
	}

	e := &EVPNType1{pathID: pathID, hasPath: hasPath}

	offset := 0

	// RD
	rd, err := ParseRouteDistinguisher(data[offset : offset+8])
	if err != nil {
		return nil, err
	}
	e.rd = rd
	offset += 8

	// ESI
	copy(e.esi[:], data[offset:offset+10])
	offset += 10

	// Ethernet Tag
	e.ethernetTag = binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	// Labels (remaining bytes)
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

func (e *EVPNType1) Bytes() []byte {
	// RFC 7432 Section 7.1: RD (8) + ESI (10) + EthTag (4) + Labels (3+)
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
	n := 8 + 10 + 4
	n += len(e.labels) * 3
	return n + 2 // +2 for type and length
}

func (e *EVPNType1) String() string {
	var sb strings.Builder
	sb.WriteString("ethernet-ad rd set ")
	sb.WriteString(e.rd.String())
	sb.WriteString(" esi set ")
	sb.WriteString(e.esi.String())
	sb.WriteString(" etag set ")
	sb.WriteString(fmt.Sprintf("%d", e.ethernetTag))
	if len(e.labels) > 0 {
		sb.WriteString(" label set ")
		sb.WriteString(fmt.Sprintf("%d", e.labels[0]))
		for _, l := range e.labels[1:] {
			sb.WriteString(",")
			sb.WriteString(fmt.Sprintf("%d", l))
		}
	}
	return sb.String()
}

// EVPNType2 represents a MAC/IP Advertisement route.
// RFC 7432 Section 7.2 defines the wire format:
//
//	+---------------------------------------+
//	|  RD (8 octets)                        |
//	+---------------------------------------+
//	|Ethernet Segment Identifier (10 octets)|
//	+---------------------------------------+
//	|  Ethernet Tag ID (4 octets)           |
//	+---------------------------------------+
//	|  MAC Address Length (1 octet)         |
//	+---------------------------------------+
//	|  MAC Address (6 octets)               |
//	+---------------------------------------+
//	|  IP Address Length (1 octet)          |
//	+---------------------------------------+
//	|  IP Address (0, 4, or 16 octets)      |
//	+---------------------------------------+
//	|  MPLS Label1 (3 octets)               |
//	+---------------------------------------+
//	|  MPLS Label2 (0 or 3 octets)          |
//	+---------------------------------------+
//
// Both MAC and IP address lengths are in bits (RFC 7432 Section 7.2).
// MAC Address Length MUST be 48 for Ethernet MACs.
// IP Address Length is 0 (none), 32 (IPv4), or 128 (IPv6).
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

// parseEVPNType2 parses a MAC/IP Advertisement route per RFC 7432 Section 7.2.
func parseEVPNType2(data []byte, pathID uint32, hasPath bool) (*EVPNType2, error) {
	// RFC 7432 Section 7.2: RD (8) + ESI (10) + EthTag (4) + MACLen (1) + MAC (6) + IPLen (1)
	if len(data) < 8+10+4+1+6+1 {
		return nil, ErrShortRead
	}

	e := &EVPNType2{pathID: pathID, hasPath: hasPath}

	offset := 0

	// RD
	rd, err := ParseRouteDistinguisher(data[offset : offset+8])
	if err != nil {
		return nil, err
	}
	e.rd = rd
	offset += 8

	// ESI
	copy(e.esi[:], data[offset:offset+10])
	offset += 10

	// Ethernet Tag
	e.ethernetTag = binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	// MAC Length in bits (RFC 7432 Section 7.2: MUST be 48 for Ethernet)
	macLen := data[offset]
	offset++
	if macLen != 48 {
		return nil, ErrInvalidAddress
	}

	// MAC
	copy(e.mac[:], data[offset:offset+6])
	offset += 6

	// IP Length in bits (RFC 7432 Section 7.2: 0, 32, or 128)
	ipLen := data[offset]
	offset++

	// IP (optional per RFC 7432 Section 10)
	switch ipLen {
	case 0:
		// No IP
	case 32:
		if offset+4 > len(data) {
			return nil, ErrShortRead
		}
		e.ip = netip.AddrFrom4([4]byte(data[offset : offset+4]))
		offset += 4
	case 128:
		if offset+16 > len(data) {
			return nil, ErrShortRead
		}
		e.ip = netip.AddrFrom16([16]byte(data[offset : offset+16]))
		offset += 16
	default:
		return nil, ErrInvalidAddress
	}

	// Labels (remaining bytes)
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

func (e *EVPNType2) Bytes() []byte {
	// RFC 7432 Section 7.2: RD (8) + ESI (10) + EthTag (4) + MACLen (1) + MAC (6) +
	// IPLen (1) + IP (0/4/16) + Labels (3+)
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

	buf[offset] = 48 // MAC length in bits (always 48 for Ethernet)
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
	n += len(e.labels) * 3
	return n + 2 // +2 for type and length
}

func (e *EVPNType2) String() string {
	var sb strings.Builder
	sb.WriteString("mac-ip rd set ")
	sb.WriteString(e.rd.String())
	sb.WriteString(" mac set ")
	sb.WriteString(fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		e.mac[0], e.mac[1], e.mac[2], e.mac[3], e.mac[4], e.mac[5]))
	if e.ip.IsValid() {
		sb.WriteString(" ip set ")
		sb.WriteString(e.ip.String())
	}
	if e.ethernetTag != 0 {
		sb.WriteString(" etag set ")
		sb.WriteString(fmt.Sprintf("%d", e.ethernetTag))
	}
	if len(e.labels) > 0 {
		sb.WriteString(" label set ")
		sb.WriteString(fmt.Sprintf("%d", e.labels[0]))
		for _, l := range e.labels[1:] {
			sb.WriteString(",")
			sb.WriteString(fmt.Sprintf("%d", l))
		}
	}
	return sb.String()
}

// EVPNType3 represents an Inclusive Multicast Ethernet Tag route.
// RFC 7432 Section 7.3 defines the wire format:
//
//	+---------------------------------------+
//	|  RD (8 octets)                        |
//	+---------------------------------------+
//	|  Ethernet Tag ID (4 octets)           |
//	+---------------------------------------+
//	|  IP Address Length (1 octet)          |
//	+---------------------------------------+
//	|  Originating Router's IP Address      |
//	|          (4 or 16 octets)             |
//	+---------------------------------------+
//
// IP Address Length is in bits (32 or 128 per RFC 7432 Section 7.3).
type EVPNType3 struct {
	rd           RouteDistinguisher
	ethernetTag  uint32
	originatorIP netip.Addr
	pathID       uint32
	hasPath      bool
}

// parseEVPNType3 parses an Inclusive Multicast Ethernet Tag route per RFC 7432 Section 7.3.
func parseEVPNType3(data []byte, pathID uint32, hasPath bool) (*EVPNType3, error) {
	// RFC 7432 Section 7.3: RD (8) + EthTag (4) + IPLen (1) + IP
	if len(data) < 8+4+1 {
		return nil, ErrShortRead
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

	switch ipLen {
	case 32:
		if offset+4 > len(data) {
			return nil, ErrShortRead
		}
		e.originatorIP = netip.AddrFrom4([4]byte(data[offset : offset+4]))
	case 128:
		if offset+16 > len(data) {
			return nil, ErrShortRead
		}
		e.originatorIP = netip.AddrFrom16([16]byte(data[offset : offset+16]))
	default:
		return nil, ErrInvalidAddress
	}

	return e, nil
}

func (e *EVPNType3) Family() Family           { return L2VPNEVPN }
func (e *EVPNType3) RouteType() EVPNRouteType { return EVPNRouteType3 }
func (e *EVPNType3) RD() RouteDistinguisher   { return e.rd }
func (e *EVPNType3) EthernetTag() uint32      { return e.ethernetTag }
func (e *EVPNType3) OriginatorIP() netip.Addr { return e.originatorIP }
func (e *EVPNType3) PathID() uint32           { return e.pathID }
func (e *EVPNType3) HasPathID() bool          { return e.hasPath }

func (e *EVPNType3) Bytes() []byte {
	// RFC 7432 Section 7.3: RD (8) + EthTag (4) + IPLen (1) + IP (4/16)
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
	return n + 2 // +2 for type and length
}

func (e *EVPNType3) String() string {
	var sb strings.Builder
	sb.WriteString("multicast rd set ")
	sb.WriteString(e.rd.String())
	sb.WriteString(" ip set ")
	sb.WriteString(e.originatorIP.String())
	if e.ethernetTag != 0 {
		sb.WriteString(" etag set ")
		sb.WriteString(fmt.Sprintf("%d", e.ethernetTag))
	}
	return sb.String()
}

// EVPNType4 represents an Ethernet Segment route.
// RFC 7432 Section 7.4 defines the wire format:
//
//	+---------------------------------------+
//	|  RD (8 octets)                        |
//	+---------------------------------------+
//	|Ethernet Segment Identifier (10 octets)|
//	+---------------------------------------+
//	|  IP Address Length (1 octet)          |
//	+---------------------------------------+
//	|  Originating Router's IP Address      |
//	|          (4 or 16 octets)             |
//	+---------------------------------------+
//
// This route is used for Designated Forwarder (DF) election in multihoming.
// IP Address Length is in bits (32 for IPv4, 128 for IPv6).
type EVPNType4 struct {
	rd           RouteDistinguisher
	esi          ESI
	originatorIP netip.Addr
	pathID       uint32
	hasPath      bool
}

// parseEVPNType4 parses an Ethernet Segment route per RFC 7432 Section 7.4.
func parseEVPNType4(data []byte, pathID uint32, hasPath bool) (*EVPNType4, error) {
	// RFC 7432 Section 7.4: RD (8) + ESI (10) + IPLen (1) + IP (4/16)
	if len(data) < 8+10+1 {
		return nil, ErrShortRead
	}

	e := &EVPNType4{pathID: pathID, hasPath: hasPath}

	offset := 0

	// RD
	rd, err := ParseRouteDistinguisher(data[offset : offset+8])
	if err != nil {
		return nil, err
	}
	e.rd = rd
	offset += 8

	// ESI
	copy(e.esi[:], data[offset:offset+10])
	offset += 10

	// IP Address Length in bits (RFC 7432 Section 7.4: 32 or 128)
	ipLen := data[offset]
	offset++

	switch ipLen {
	case 32:
		if offset+4 > len(data) {
			return nil, ErrShortRead
		}
		e.originatorIP = netip.AddrFrom4([4]byte(data[offset : offset+4]))
	case 128:
		if offset+16 > len(data) {
			return nil, ErrShortRead
		}
		e.originatorIP = netip.AddrFrom16([16]byte(data[offset : offset+16]))
	default:
		return nil, ErrInvalidAddress
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

func (e *EVPNType4) Bytes() []byte {
	// RFC 7432 Section 7.4: RD (8) + ESI (10) + IPLen (1) + IP (4/16)
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
	return n + 2 // +2 for type and length
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

// EVPNType5 represents an IP Prefix route.
// RFC 9136 Section 3 defines the wire format:
//
//	+---------------------------------------+
//	|      RD (8 octets)                    |
//	+---------------------------------------+
//	|Ethernet Segment Identifier (10 octets)|
//	+---------------------------------------+
//	|  Ethernet Tag ID (4 octets)           |
//	+---------------------------------------+
//	|  IP Prefix Length (1 octet)           |
//	+---------------------------------------+
//	|  IP Prefix (4 or 16 octets)           |
//	+---------------------------------------+
//	|  GW IP Address (4 or 16 octets)       |
//	+---------------------------------------+
//	|  MPLS Label (3 octets)                |
//	+---------------------------------------+
//
// RFC 9136: IP Prefix Length is 0-32 for IPv4, 0-128 for IPv6.
// Total length is 34 octets for IPv4 or 58 octets for IPv6.
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

// parseEVPNType5 parses an IP Prefix route per RFC 9136 Section 3.1.
// RFC 9136 specifies fixed-size NLRI:
//   - IPv4: Length = 34 bytes (RD:8 + ESI:10 + ETag:4 + IPLen:1 + IP:4 + GW:4 + Label:3)
//   - IPv6: Length = 58 bytes (RD:8 + ESI:10 + ETag:4 + IPLen:1 + IP:16 + GW:16 + Label:3)
func parseEVPNType5(data []byte, pathID uint32, hasPath bool) (*EVPNType5, error) {
	// RFC 9136 Section 3.1: Length MUST be 34 (IPv4) or 58 (IPv6)
	switch len(data) {
	case 34:
		// IPv4 Type 5
	case 58:
		// IPv6 Type 5
	default:
		return nil, ErrInvalidAddress
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

	// RFC 9136 Section 3.1: IP Prefix is FIXED 4 octets (IPv4) or 16 octets (IPv6)
	// Determined by total NLRI length, not prefix length
	var addr netip.Addr
	if len(data) == 34 {
		// IPv4: Fixed 4-byte prefix field
		if ipLen > 32 {
			return nil, ErrInvalidPrefix
		}
		var ip [4]byte
		copy(ip[:], data[offset:offset+4])
		addr = netip.AddrFrom4(ip)
		offset += 4

		// Gateway: Fixed 4 bytes
		e.gateway = netip.AddrFrom4([4]byte(data[offset : offset+4]))
		offset += 4
	} else {
		// IPv6: Fixed 16-byte prefix field
		if ipLen > 128 {
			return nil, ErrInvalidPrefix
		}
		var ip [16]byte
		copy(ip[:], data[offset:offset+16])
		addr = netip.AddrFrom16(ip)
		offset += 16

		// Gateway: Fixed 16 bytes
		e.gateway = netip.AddrFrom16([16]byte(data[offset : offset+16]))
		offset += 16
	}

	prefix, err := addr.Prefix(ipLen)
	if err != nil {
		return nil, ErrInvalidPrefix
	}
	e.prefix = prefix

	// Labels (remaining 3 bytes)
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

func (e *EVPNType5) Bytes() []byte {
	// RFC 9136 Section 3.1: IP Prefix route encoding
	// IPv4: RD (8) + ESI (10) + EthTag (4) + PrefixLen (1) + Prefix (4) + GW (4) + Labels (3) = 34
	// IPv6: RD (8) + ESI (10) + EthTag (4) + PrefixLen (1) + Prefix (16) + GW (16) + Labels (3) = 58
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

	// RFC 9136: Prefix field is FIXED size (4 or 16 bytes), not variable
	// RFC 9136 Section 3.1: Gateway IP is 0 when not used
	if prefixSize == 4 {
		ip4 := e.prefix.Addr().As4()
		copy(buf[offset:], ip4[:])
		offset += 4

		if e.gateway.IsValid() {
			gw4 := e.gateway.As4()
			copy(buf[offset:], gw4[:])
		}
		// else: leave as zeros (gateway not used)
		offset += 4
	} else {
		ip6 := e.prefix.Addr().As16()
		copy(buf[offset:], ip6[:])
		offset += 16

		if e.gateway.IsValid() {
			gw6 := e.gateway.As16()
			copy(buf[offset:], gw6[:])
		}
		// else: leave as zeros (gateway not used)
		offset += 16
	}

	copy(buf[offset:], labelBytes)

	return buf
}

func (e *EVPNType5) Len() int {
	// RFC 9136: Fixed lengths (34 for IPv4, 58 for IPv6)
	if e.prefix.Addr().Is4() {
		return 34 + 2 // +2 for type and length header
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
		sb.WriteString(" etag set ")
		sb.WriteString(fmt.Sprintf("%d", e.ethernetTag))
	}
	if e.gateway.IsValid() && !e.gateway.IsUnspecified() {
		sb.WriteString(" gateway set ")
		sb.WriteString(e.gateway.String())
	}
	if len(e.labels) > 0 {
		sb.WriteString(" label set ")
		sb.WriteString(fmt.Sprintf("%d", e.labels[0]))
		for _, l := range e.labels[1:] {
			sb.WriteString(",")
			sb.WriteString(fmt.Sprintf("%d", l))
		}
	}
	return sb.String()
}

// EVPNGeneric holds unparsed EVPN routes.
// Used for route types not yet implemented (e.g., Type 1, Type 4).
// RFC 7432 Section 7.1: Type 1 - Ethernet Auto-discovery per-ES and per-EVI.
// RFC 7432 Section 7.4: Type 4 - Ethernet Segment route for DF election.
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
func (e *EVPNGeneric) Bytes() []byte            { return e.data }
func (e *EVPNGeneric) Len() int                 { return len(e.data) + 2 }
func (e *EVPNGeneric) String() string           { return fmt.Sprintf("evpn-type%d", e.routeType) }

// WriteTo methods for EVPN types - write payload without path ID.
// RFC 7911 Section 3: Path ID is NOT written by these methods.
// Use WriteNLRI() for ADD-PATH encoding with path identifier.

func (e *EVPNType1) WriteTo(buf []byte, off int) int   { return copy(buf[off:], e.Bytes()) }
func (e *EVPNType2) WriteTo(buf []byte, off int) int   { return copy(buf[off:], e.Bytes()) }
func (e *EVPNType3) WriteTo(buf []byte, off int) int   { return copy(buf[off:], e.Bytes()) }
func (e *EVPNType4) WriteTo(buf []byte, off int) int   { return copy(buf[off:], e.Bytes()) }
func (e *EVPNType5) WriteTo(buf []byte, off int) int   { return copy(buf[off:], e.Bytes()) }
func (e *EVPNGeneric) WriteTo(buf []byte, off int) int { return copy(buf[off:], e.Bytes()) }

// NewEVPNType1 creates an Ethernet Auto-Discovery route (Type 1).
// RFC 7432 Section 7.1.
func NewEVPNType1(rd RouteDistinguisher, esi [10]byte, ethernetTag uint32, labels []uint32) *EVPNType1 {
	return &EVPNType1{
		rd:          rd,
		esi:         esi,
		ethernetTag: ethernetTag,
		labels:      labels,
	}
}

// NewEVPNType2 creates a MAC/IP Advertisement route (Type 2).
// RFC 7432 Section 7.2.
func NewEVPNType2(rd RouteDistinguisher, esi [10]byte, ethernetTag uint32, mac [6]byte, ip netip.Addr, labels []uint32) *EVPNType2 {
	return &EVPNType2{
		rd:          rd,
		esi:         esi,
		ethernetTag: ethernetTag,
		mac:         mac,
		ip:          ip,
		labels:      labels,
	}
}

// NewEVPNType3 creates an Inclusive Multicast Ethernet Tag route (Type 3).
// RFC 7432 Section 7.3.
func NewEVPNType3(rd RouteDistinguisher, ethernetTag uint32, originatorIP netip.Addr) *EVPNType3 {
	return &EVPNType3{
		rd:           rd,
		ethernetTag:  ethernetTag,
		originatorIP: originatorIP,
	}
}

// NewEVPNType4 creates an Ethernet Segment route (Type 4).
// RFC 7432 Section 7.4.
func NewEVPNType4(rd RouteDistinguisher, esi [10]byte, originatorIP netip.Addr) *EVPNType4 {
	return &EVPNType4{
		rd:           rd,
		esi:          esi,
		originatorIP: originatorIP,
	}
}

// NewEVPNType5 creates an IP Prefix route (Type 5).
// RFC 9136 Section 3.1.
func NewEVPNType5(rd RouteDistinguisher, esi [10]byte, ethernetTag uint32, prefix netip.Prefix, gateway netip.Addr, labels []uint32) *EVPNType5 {
	return &EVPNType5{
		rd:          rd,
		esi:         esi,
		ethernetTag: ethernetTag,
		prefix:      prefix,
		gateway:     gateway,
		labels:      labels,
	}
}
