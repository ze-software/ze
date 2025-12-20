// Package attribute implements BGP path attributes.
package attribute

import (
	"encoding/binary"
	"errors"
	"net/netip"
)

// Errors for MP NLRI parsing.
var (
	ErrInvalidNextHopLen = errors.New("attribute: invalid next-hop length")
	ErrUnsupportedAFI    = errors.New("attribute: unsupported AFI")
)

// AFI represents Address Family Identifier.
//
// RFC 4760 Section 3: "Address Family Identifier (AFI): This field in
// combination with the Subsequent Address Family Identifier field identifies
// the set of Network Layer protocols to which the address carried in the
// Next Hop field must belong..."
//
// Values are defined in IANA's Address Family Numbers registry.
type AFI uint16

// Address Family Identifiers (IANA registry).
const (
	AFIIPv4  AFI = 1
	AFIIPv6  AFI = 2
	AFIL2VPN AFI = 25
)

// SAFI represents Subsequent Address Family Identifier.
//
// RFC 4760 Section 6: Defines SAFI values 1 (unicast) and 2 (multicast).
// Additional values are registered in IANA's SAFI registry.
type SAFI uint8

// Subsequent Address Family Identifiers.
//
// RFC 4760 Section 6:
//   - 1: Network Layer Reachability Information used for unicast forwarding
//   - 2: Network Layer Reachability Information used for multicast forwarding
//
// Other values (70, 128, 133) are defined in separate RFCs.
const (
	SAFIUnicast   SAFI = 1   // RFC 4760 Section 6
	SAFIMulticast SAFI = 2   // RFC 4760 Section 6
	SAFIEVPN      SAFI = 70  // RFC 7432
	SAFIVPN       SAFI = 128 // RFC 4364
	SAFIFlowSpec  SAFI = 133 // RFC 5575
)

// MPReachNLRI represents the MP_REACH_NLRI attribute (Type Code 14).
//
// RFC 4760 Section 3: "This is an optional non-transitive attribute that can
// be used for the following purposes:
//
//	(a) to advertise a feasible route to a peer
//	(b) to permit a router to advertise the Network Layer address of the
//	    router that should be used as the next hop to the destinations
//	    listed in the Network Layer Reachability Information field"
//
// Wire format (RFC 4760 Section 3):
//
//	+---------------------------------------------------------+
//	| Address Family Identifier (2 octets)                    |
//	+---------------------------------------------------------+
//	| Subsequent Address Family Identifier (1 octet)          |
//	+---------------------------------------------------------+
//	| Length of Next Hop Network Address (1 octet)            |
//	+---------------------------------------------------------+
//	| Network Address of Next Hop (variable)                  |
//	+---------------------------------------------------------+
//	| Reserved (1 octet)                                      |
//	+---------------------------------------------------------+
//	| Network Layer Reachability Information (variable)       |
//	+---------------------------------------------------------+
type MPReachNLRI struct {
	AFI      AFI          // RFC 4760 Section 3: Address Family Identifier (2 octets)
	SAFI     SAFI         // RFC 4760 Section 3: Subsequent Address Family Identifier (1 octet)
	NextHops []netip.Addr // RFC 4760 Section 3: Network Address of Next Hop (variable)
	NLRI     []byte       // RFC 4760 Section 3: Network Layer Reachability Information (variable)
}

// Code returns AttrMPReachNLRI (Type Code 14).
// RFC 4760 Section 3: MP_REACH_NLRI has Type Code 14.
func (m *MPReachNLRI) Code() AttributeCode { return AttrMPReachNLRI }

// Flags returns FlagOptional.
// RFC 4760 Section 3: "This is an optional non-transitive attribute"
func (m *MPReachNLRI) Flags() AttributeFlags { return FlagOptional }

// Len returns the packed length in bytes.
//
// RFC 4760 Section 3 wire format:
// AFI(2) + SAFI(1) + NH_Len(1) + NextHops(variable) + Reserved(1) + NLRI(variable)
func (m *MPReachNLRI) Len() int {
	nhLen := m.nextHopLen()
	return 2 + 1 + 1 + nhLen + 1 + len(m.NLRI)
}

// nextHopLen calculates the total next-hop length in bytes.
//
// RFC 4760 Section 3: "Length of Next Hop Network Address: A 1-octet field
// whose value expresses the length of the 'Network Address of Next Hop' field,
// measured in octets."
func (m *MPReachNLRI) nextHopLen() int {
	total := 0
	for _, nh := range m.NextHops {
		if nh.Is4() {
			total += 4
		} else {
			total += 16
		}
	}
	return total
}

// Pack serializes the MP_REACH_NLRI attribute value per RFC 4760 Section 3.
func (m *MPReachNLRI) Pack() []byte {
	nhLen := m.nextHopLen()
	buf := make([]byte, m.Len())

	// RFC 4760 Section 3: Address Family Identifier (2 octets)
	binary.BigEndian.PutUint16(buf[0:2], uint16(m.AFI))

	// RFC 4760 Section 3: Subsequent Address Family Identifier (1 octet)
	buf[2] = byte(m.SAFI)

	// RFC 4760 Section 3: Length of Next Hop Network Address (1 octet)
	buf[3] = byte(nhLen)

	// RFC 4760 Section 3: Network Address of Next Hop (variable)
	offset := 4
	for _, nh := range m.NextHops {
		nhBytes := nh.AsSlice()
		copy(buf[offset:], nhBytes)
		offset += len(nhBytes)
	}

	// RFC 4760 Section 3: Reserved (1 octet) - "MUST be set to 0"
	buf[offset] = 0
	offset++

	// RFC 4760 Section 3: Network Layer Reachability Information (variable)
	copy(buf[offset:], m.NLRI)

	return buf
}

// ParseMPReachNLRI parses an MP_REACH_NLRI attribute value per RFC 4760 Section 3.
//
// RFC 4760 Section 3: The Reserved octet "SHOULD be ignored upon receipt."
func ParseMPReachNLRI(data []byte) (*MPReachNLRI, error) {
	// Minimum: AFI(2) + SAFI(1) + NH_Len(1) + Reserved(1) = 5 octets
	if len(data) < 5 {
		return nil, ErrShortData
	}

	// RFC 4760 Section 3: Parse AFI and SAFI
	m := &MPReachNLRI{
		AFI:  AFI(binary.BigEndian.Uint16(data[0:2])),
		SAFI: SAFI(data[2]),
	}

	// RFC 4760 Section 3: Length of Next Hop Network Address (1 octet)
	nhLen := int(data[3])
	if len(data) < 4+nhLen+1 { // +1 for reserved byte
		return nil, ErrShortData
	}

	// RFC 4760 Section 3: Network Address of Next Hop (variable)
	nhData := data[4 : 4+nhLen]
	nextHops, err := parseNextHops(m.AFI, nhData)
	if err != nil {
		return nil, err
	}
	m.NextHops = nextHops

	// RFC 4760 Section 3: Reserved (1 octet) - "SHOULD be ignored upon receipt"
	nlriOffset := 4 + nhLen + 1

	// RFC 4760 Section 3: Network Layer Reachability Information (variable)
	if nlriOffset < len(data) {
		m.NLRI = make([]byte, len(data)-nlriOffset)
		copy(m.NLRI, data[nlriOffset:])
	}

	return m, nil
}

// parseNextHops parses next-hop address(es) based on AFI.
//
// RFC 4760 Section 3: "Network Address of Next Hop: A variable-length field
// that contains the Network Address of the next router on the path to the
// destination system. The Network Layer protocol associated with the Network
// Address of the Next Hop is identified by a combination of <AFI, SAFI>
// carried in the attribute."
//
// For IPv6 (AFI=2), RFC 2545 specifies that the next-hop can be either:
//   - 16 octets: global IPv6 address only
//   - 32 octets: global IPv6 address followed by link-local address
func parseNextHops(afi AFI, data []byte) ([]netip.Addr, error) {
	if len(data) == 0 {
		return nil, nil
	}

	var hops []netip.Addr

	switch afi {
	case AFIIPv4:
		// IPv4: 4 bytes per next-hop
		if len(data)%4 != 0 {
			return nil, ErrInvalidNextHopLen
		}
		for i := 0; i < len(data); i += 4 {
			var ip [4]byte
			copy(ip[:], data[i:i+4])
			hops = append(hops, netip.AddrFrom4(ip))
		}

	case AFIIPv6:
		// IPv6: 16 bytes per next-hop, or 32 bytes for global+link-local (RFC 2545)
		switch len(data) {
		case 16:
			var ip [16]byte
			copy(ip[:], data)
			hops = append(hops, netip.AddrFrom16(ip))
		case 32:
			// Global + link-local (RFC 2545 Section 3)
			var ip1, ip2 [16]byte
			copy(ip1[:], data[0:16])
			copy(ip2[:], data[16:32])
			hops = append(hops, netip.AddrFrom16(ip1), netip.AddrFrom16(ip2))
		default:
			return nil, ErrInvalidNextHopLen
		}

	case AFIL2VPN:
		// L2VPN (EVPN, RFC 7432): typically 4 or 16 bytes
		switch len(data) {
		case 4:
			var ip [4]byte
			copy(ip[:], data)
			hops = append(hops, netip.AddrFrom4(ip))
		case 16:
			var ip [16]byte
			copy(ip[:], data)
			hops = append(hops, netip.AddrFrom16(ip))
		default:
			return nil, ErrInvalidNextHopLen
		}

	default:
		// Unknown AFI - try to preserve the raw data as best we can
		// Return empty slice, the raw data is still in the attribute
		return nil, nil
	}

	return hops, nil
}

// MPUnreachNLRI represents the MP_UNREACH_NLRI attribute (Type Code 15).
//
// RFC 4760 Section 4: "This is an optional non-transitive attribute that can
// be used for the purpose of withdrawing multiple unfeasible routes from service."
//
// Wire format (RFC 4760 Section 4):
//
//	+---------------------------------------------------------+
//	| Address Family Identifier (2 octets)                    |
//	+---------------------------------------------------------+
//	| Subsequent Address Family Identifier (1 octet)          |
//	+---------------------------------------------------------+
//	| Withdrawn Routes (variable)                             |
//	+---------------------------------------------------------+
//
// RFC 4760 Section 4: "An UPDATE message that contains the MP_UNREACH_NLRI
// is not required to carry any other path attributes."
type MPUnreachNLRI struct {
	AFI  AFI    // RFC 4760 Section 4: Address Family Identifier (2 octets)
	SAFI SAFI   // RFC 4760 Section 4: Subsequent Address Family Identifier (1 octet)
	NLRI []byte // RFC 4760 Section 4: Withdrawn Routes (variable)
}

// Code returns AttrMPUnreachNLRI (Type Code 15).
// RFC 4760 Section 4: MP_UNREACH_NLRI has Type Code 15.
func (m *MPUnreachNLRI) Code() AttributeCode { return AttrMPUnreachNLRI }

// Flags returns FlagOptional.
// RFC 4760 Section 4: "This is an optional non-transitive attribute"
func (m *MPUnreachNLRI) Flags() AttributeFlags { return FlagOptional }

// Len returns the packed length in bytes.
//
// RFC 4760 Section 4 wire format: AFI(2) + SAFI(1) + Withdrawn Routes(variable)
func (m *MPUnreachNLRI) Len() int {
	return 2 + 1 + len(m.NLRI)
}

// Pack serializes the MP_UNREACH_NLRI attribute value per RFC 4760 Section 4.
func (m *MPUnreachNLRI) Pack() []byte {
	buf := make([]byte, m.Len())

	// RFC 4760 Section 4: Address Family Identifier (2 octets)
	binary.BigEndian.PutUint16(buf[0:2], uint16(m.AFI))

	// RFC 4760 Section 4: Subsequent Address Family Identifier (1 octet)
	buf[2] = byte(m.SAFI)

	// RFC 4760 Section 4: Withdrawn Routes (variable)
	copy(buf[3:], m.NLRI)

	return buf
}

// ParseMPUnreachNLRI parses an MP_UNREACH_NLRI attribute value per RFC 4760 Section 4.
func ParseMPUnreachNLRI(data []byte) (*MPUnreachNLRI, error) {
	// Minimum: AFI(2) + SAFI(1) = 3 octets
	if len(data) < 3 {
		return nil, ErrShortData
	}

	// RFC 4760 Section 4: Parse AFI and SAFI
	m := &MPUnreachNLRI{
		AFI:  AFI(binary.BigEndian.Uint16(data[0:2])),
		SAFI: SAFI(data[2]),
	}

	// RFC 4760 Section 4: Withdrawn Routes (variable)
	if len(data) > 3 {
		m.NLRI = make([]byte, len(data)-3)
		copy(m.NLRI, data[3:])
	}

	return m, nil
}

// IsEndOfRIB returns true if this MP_UNREACH_NLRI represents an End-of-RIB marker.
//
// RFC 4724 Section 2: "An UPDATE message with no reachable Network Layer
// Reachability Information (NLRI) and empty Withdrawn NLRI is specified as
// the End-of-RIB marker that can be used by a BGP speaker to indicate to
// its peer the completion of the initial routing update after the session
// is established."
//
// For MP-BGP, an MP_UNREACH_NLRI with empty Withdrawn Routes signals End-of-RIB
// for that <AFI, SAFI>.
func (m *MPUnreachNLRI) IsEndOfRIB() bool {
	return len(m.NLRI) == 0
}

// NewMPUnreachEndOfRIB creates an End-of-RIB marker for the given address family.
//
// RFC 4724: End-of-RIB is signaled by an MP_UNREACH_NLRI with empty Withdrawn Routes.
func NewMPUnreachEndOfRIB(afi AFI, safi SAFI) *MPUnreachNLRI {
	return &MPUnreachNLRI{
		AFI:  afi,
		SAFI: safi,
		NLRI: nil, // Empty NLRI signals End-of-RIB (RFC 4724)
	}
}
