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
	"fmt"
	"net/netip"
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
	case EVPNRouteType2:
		nlri, err = parseEVPNType2(nlriData, pathID, addpath)
	case EVPNRouteType3:
		nlri, err = parseEVPNType3(nlriData, pathID, addpath)
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
	// TODO: implement encoding
	return nil
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
	mac := fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		e.mac[0], e.mac[1], e.mac[2], e.mac[3], e.mac[4], e.mac[5])
	if e.ip.IsValid() {
		return fmt.Sprintf("type2 RD:%s MAC:%s IP:%s", e.rd, mac, e.ip)
	}
	return fmt.Sprintf("type2 RD:%s MAC:%s", e.rd, mac)
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
func (e *EVPNType3) Bytes() []byte            { return nil }
func (e *EVPNType3) Len() int                 { return 0 }

func (e *EVPNType3) String() string {
	return fmt.Sprintf("type3 RD:%s originator:%s", e.rd, e.originatorIP)
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

// parseEVPNType5 parses an IP Prefix route per RFC 9136 Section 3.
func parseEVPNType5(data []byte, pathID uint32, hasPath bool) (*EVPNType5, error) {
	// RFC 9136: RD (8) + ESI (10) + EthTag (4) + IPLen (1) + IP + GW + Label
	if len(data) < 8+10+4+1 {
		return nil, ErrShortRead
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

	prefixBytes := (ipLen + 7) / 8
	if offset+prefixBytes > len(data) {
		return nil, ErrShortRead
	}

	// RFC 9136: Determine IPv4 or IPv6 based on prefix length (0-32 = IPv4, >32 = IPv6)
	var addr netip.Addr
	if ipLen <= 32 {
		var ip [4]byte
		copy(ip[:], data[offset:offset+prefixBytes])
		addr = netip.AddrFrom4(ip)
	} else {
		var ip [16]byte
		copy(ip[:], data[offset:offset+prefixBytes])
		addr = netip.AddrFrom16(ip)
	}
	offset += prefixBytes

	prefix, err := addr.Prefix(ipLen)
	if err != nil {
		return nil, ErrInvalidPrefix
	}
	e.prefix = prefix

	// RFC 9136: Gateway IP (same address family as prefix)
	gwBytes := 4
	if ipLen > 32 {
		gwBytes = 16
	}
	if offset+gwBytes > len(data) {
		return nil, ErrShortRead
	}
	if gwBytes == 4 {
		e.gateway = netip.AddrFrom4([4]byte(data[offset : offset+4]))
	} else {
		e.gateway = netip.AddrFrom16([16]byte(data[offset : offset+16]))
	}
	offset += gwBytes

	// Labels
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
func (e *EVPNType5) Bytes() []byte            { return nil }
func (e *EVPNType5) Len() int                 { return 0 }

func (e *EVPNType5) String() string {
	return fmt.Sprintf("type5 RD:%s prefix:%s", e.rd, e.prefix)
}

// EVPNGeneric holds unparsed EVPN routes.
// Used for route types not yet implemented (e.g., Type 1, Type 4).
// RFC 7432 Section 7.1: Type 1 - Ethernet Auto-discovery
// RFC 7432 Section 7.4: Type 4 - Ethernet Segment
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
