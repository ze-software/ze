// Design: docs/architecture/wire/attributes.md — path attribute encoding
// RFC: rfc/short/rfc4760.md — MP_REACH_NLRI / MP_UNREACH_NLRI attributes
// Related: nexthop_form.go — which address forms may occupy the next-hop field
//
// Package attribute implements BGP path attributes.
package attribute

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// Errors for MP NLRI parsing.
var (
	ErrInvalidNextHopLen = errors.New("attribute: invalid next-hop length")
	ErrUnsupportedAFI    = errors.New("attribute: unsupported AFI")
)

// ErrUnencodableNextHop reports a next hop that has no wire form, in either
// attribute that carries one.
//
// RFC 4760 Section 3 gives MP_REACH_NLRI a "Network Address of Next Hop" field and
// a "Length of Next Hop Network Address" octet that counts it. The zero netip.Addr
// names no address, so no octet count encodes it, and ValidNextHopLens admits no
// length that would. RFC 4271 Section 5.1.3 gives NEXT_HOP an IP address and no
// zero-length form at all. An attribute carrying one cannot go on the wire.
//
// Each wrapper names its own attribute, because the two ask the operator for
// different things: (*MPReachNLRI).ValidateNextHops adds MP_REACH and the AFI/SAFI
// pair, (*NextHop).ValidateNextHops adds NEXT_HOP.
var ErrUnencodableNextHop = errors.New("attribute: next hop has no wire form")

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
	SAFIMVPN      SAFI = 5   // RFC 6514
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
	NextHops NextHopAddrs // RFC 4760 Section 3: Network Address of Next Hop (max 2, inline)
	NLRI     []byte       // RFC 4760 Section 3: Network Layer Reachability Information (variable)
}

// NextHopAddrs stores up to 2 next-hop addresses inline without slice allocation.
// Assignable from []netip.Addr via the implicit conversion in struct literals.
type NextHopAddrs struct {
	addrs [2]netip.Addr
	count uint8
}

// NewNextHopAddrs creates NextHopAddrs from a slice.
func NewNextHopAddrs(hops []netip.Addr) NextHopAddrs {
	var n NextHopAddrs
	n.count = uint8(min(len(hops), 2))
	for i := range n.count {
		n.addrs[i] = hops[i]
	}
	return n
}

// Slice returns the addresses as a slice (no allocation, backed by inline array).
func (n *NextHopAddrs) Slice() []netip.Addr { return n.addrs[:n.count] }

// Len returns the number of next-hops.
func (n *NextHopAddrs) Len() int { return int(n.count) }

// NewMPReachNLRI creates an MPReachNLRI with inline next-hop storage.
func NewMPReachNLRI(afi AFI, safi SAFI, nextHops []netip.Addr, nlri []byte) *MPReachNLRI {
	return &MPReachNLRI{AFI: afi, SAFI: safi, NextHops: NewNextHopAddrs(nextHops), NLRI: nlri}
}

// Code returns AttrMPReachNLRI (Type Code 14).
// RFC 4760 Section 3: MP_REACH_NLRI has Type Code 14.
func (m *MPReachNLRI) Code() AttributeCode { return AttrMPReachNLRI }

// Flags returns FlagOptional (non-transitive) per RFC 4760 Section 3.
func (m *MPReachNLRI) Flags() AttributeFlags { return FlagOptional }

// Len returns the packed length in bytes.
// RFC 4760 Section 3 wire format: AFI(2) + SAFI(1) + NH_Len(1) + NextHops + Reserved(1) + NLRI.
func (m *MPReachNLRI) Len() int {
	nhLen := m.nextHopLen()
	return 2 + 1 + 1 + nhLen + 1 + len(m.NLRI)
}

// nextHopLen calculates the total next-hop length in bytes per RFC 4760 Section 3.
func (m *MPReachNLRI) nextHopLen() int {
	total := 0
	for _, nh := range m.NextHops.Slice() {
		total += m.nextHopOctets(nh)
	}
	return total
}

// nextHopOctets counts the selected address and the profile's framing. Invalid
// addresses contribute no RD, so sizing and writing agree even before the
// separate presence and family admission checks run.
func (m *MPReachNLRI) nextHopOctets(nh netip.Addr) int {
	return MPNextHopProfile(m.AFI, m.SAFI).NextHopOctets(m.wireNextHop(nh))
}

// wireNextHop selects the typed codec's address representation. Permission to
// admit mapped input does not request conversion. RFC 4659 Section 3.2.1.2
// requires the VPN-IPv6 default, declared only by that built-in profile.
func (m *MPReachNLRI) wireNextHop(nh netip.Addr) netip.Addr {
	return MPNextHopProfile(m.AFI, m.SAFI).SelectAddress(nh, false)
}

// ValidateNextHops reports whether every next hop of this attribute has a wire
// form, and is the refusal half of the rule nextHopOctets states.
//
// RFC 4760 Section 3 requires the Network Address of Next Hop field to carry "the
// Network Address of the next router on the path to the destination system". The
// zero netip.Addr is not such an address. Encoding it would put a next-hop length
// of zero on the wire, which ValidNextHopLens admits for no AFI/SAFI pair, so the
// peer would treat the UPDATE as malformed (RFC 7606 Section 7.11).
//
// This is a presence check, not family admission. Callers must separately
// validate the field width, address roles and negotiated permissions.
// MPNextHopProfile supplies the family-specific field contract; session
// capabilities are not part of this attribute.
func (m *MPReachNLRI) ValidateNextHops() error {
	for _, nh := range m.NextHops.Slice() {
		if !nh.IsValid() {
			return fmt.Errorf("%w: MP_REACH AFI %d SAFI %d (RFC 4760 Section 3)",
				ErrUnencodableNextHop, m.AFI, m.SAFI)
		}
	}
	return nil
}

// WriteTo writes the MP_REACH_NLRI attribute value into buf at offset.
func (m *MPReachNLRI) WriteTo(buf []byte, off int) int {
	profile := MPNextHopProfile(m.AFI, m.SAFI)
	var first, second netip.Addr
	if m.NextHops.count > 0 {
		first = profile.SelectAddress(m.NextHops.addrs[0], false)
	}
	if m.NextHops.count > 1 {
		second = profile.SelectAddress(m.NextHops.addrs[1], false)
	}
	nhLen := profile.NextHopLen(first, second)

	// RFC 4760 Section 3: Address Family Identifier (2 octets)
	binary.BigEndian.PutUint16(buf[off:], uint16(m.AFI))

	// RFC 4760 Section 3: Subsequent Address Family Identifier (1 octet)
	buf[off+2] = byte(m.SAFI)

	// RFC 4760 Section 3: Length of Next Hop Network Address (1 octet)
	buf[off+3] = byte(nhLen)

	// RFC 4760 Section 3: Network Address of Next Hop (variable)
	pos := off + 4 + profile.WriteNextHops(buf, off+4, first, second)

	// RFC 4760 Section 3: Reserved (1 octet) - "MUST be set to 0"
	buf[pos] = 0
	pos++

	// RFC 4760 Section 3: Network Layer Reachability Information (variable)
	n := copy(buf[pos:], m.NLRI)
	pos += n

	return pos - off
}

// WriteToWithContext writes MP_REACH_NLRI - context-independent.
func (m *MPReachNLRI) WriteToWithContext(buf []byte, off int, _, _ *bgpctx.EncodingContext) int {
	return m.WriteTo(buf, off)
}

// CheckedWriteTo validates the next hops and the capacity before writing.
//
// It is NOT the announce rails' entry point and cannot become one: it takes no
// *bgpctx.EncodingContext, while announceAttrs.add (reactor/announce_build.go)
// writes through WriteToWithContext because that context decides the AS_PATH ASN
// width (RFC 6793 Section 4.1). Routing a plan through this signature would
// four-octet-encode AS_PATH toward a two-octet peer. The rails therefore refuse
// where the plan is built, calling the same ValidateNextHops this does; the
// capacity half they get from announceAttrs.reserve.
func (m *MPReachNLRI) CheckedWriteTo(buf []byte, off int) (int, error) {
	if err := m.ValidateNextHops(); err != nil {
		return 0, err
	}
	needed := m.Len()
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return m.WriteTo(buf, off), nil
}

// ParseMPReachNLRI parses an MP_REACH_NLRI attribute value per RFC 4760 Section 3.
// The Reserved octet is ignored per RFC 4760.
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
	nextHops, err := parseNextHops(m.AFI, m.SAFI, nhData)
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

// RDSize is the size of Route Distinguisher in VPN next-hops.
// RFC 4364 Section 4.3.4: VPN next-hop includes 8-byte RD prefix (set to zero).
const RDSize = 8

// SAFIMPLSLabel is SAFI 4 (RFC 8277: MPLS labeled unicast).
const SAFIMPLSLabel SAFI = 4

// SAFISRPolicy is SAFI 73 (RFC 9830: SR Policy).
const SAFISRPolicy SAFI = 73

// NextHopProfile holds the family-specific field contract. Its zero value names
// an unknown layout and imposes no new admission on plugin-defined fields.
// Lengths is shared read-only storage; callers MUST NOT mutate it.
type NextHopProfile struct {
	Lengths      []int
	IPv6Roles    bool
	MappedIPv4   bool
	ExtendedIPv6 bool
	// RDPerAddress prefixes each address with RDSize octets.
	RDPerAddress bool
	// EncodeIPv4Mapped selects mapped output by default, independently of
	// MappedIPv4's permission to admit an already mapped address.
	EncodeIPv4Mapped bool
	// DecodeUnprefixedIPv6 preserves codec-only legacy VPN128 decoding.
	DecodeUnprefixedIPv6 bool
}

// MPNextHopProfile supplies known field widths, RD framing, IPv6 address roles,
// interworking forms and RFC 8950 capability scope.
// RFC 8950 Section 3: "This field is to be constructed as per Section 3 of
// [RFC2545]." RFC 9830 Section 2.1: "If the next-hop length is 32, then it has
// a global IPv6 address followed by a link-local IPv6 address."
// VPN fields retain their RD(8)+address layouts, not plain IPv6 roles.
func MPNextHopProfile(afi AFI, safi SAFI) NextHopProfile {
	if profile, found := mpNextHopProfiles[uint32(afi)<<8|uint32(safi)]; found {
		return profile
	}
	return builtinMPNextHopProfile(afi, safi)
}

// builtinMPNextHopProfile holds only contracts owned by the core. Plugin
// contracts enter through RegisterMPNextHopProfile and cannot shadow these.
func builtinMPNextHopProfile(afi AFI, safi SAFI) NextHopProfile {
	switch afi {
	case AFIIPv4:
		switch safi {
		case SAFIUnicast, SAFIMulticast, SAFIMPLSLabel:
			// RFC 8950 Section 3, for <1/1>, <1/2> and <1/4>: "Length of Next
			// Hop Address = 16 or 32" and "Next Hop Address = IPv6 address of a
			// next hop (potentially followed by the link-local IPv6 address of
			// the next hop). This field is to be constructed as per Section 3 of
			// [RFC2545]." So the two-address form of RFC 2545 Section 3 is a
			// length this AFI/SAFI carries, beside the plain IPv4 one.
			return NextHopProfile{Lengths: ipv4ExtendedNextHopLens, IPv6Roles: true, ExtendedIPv6: true}
		case SAFIVPN:
			// RFC 8950 Section 3, for <1/128> and <1/129>: "Length of Next Hop
			// Address = 24 or 48", the VPN-IPv6 address with its zero RD,
			// "potentially followed by the link-local VPN-IPv6 address of the
			// next hop with an 8-octet RD set to zero".
			return NextHopProfile{Lengths: vpnIPv4NextHopLens, ExtendedIPv6: true, RDPerAddress: true, DecodeUnprefixedIPv6: true}
		case SAFI(129):
			// RFC 8950 names SAFI 129; no Ze decoder implements its layout.
			return NextHopProfile{ExtendedIPv6: true}
		case SAFISRPolicy:
			return NextHopProfile{Lengths: ipv4ExtendedNextHopLens, IPv6Roles: true}
		case SAFIMVPN:
			return NextHopProfile{Lengths: mvpnNextHopLens}
		case SAFIFlowSpec, SAFIEVPN:
			// FlowSpec: permissive (no test coverage yet for strict validation)
			// EVPN: uses AFI L2VPN (25), not IPv4
		default:
			// The wire SAFI set is open; unknown combinations have no length table.
		}
	case AFIIPv6:
		switch safi {
		case SAFIUnicast, SAFIMulticast, SAFIMPLSLabel:
			// RFC 8950 Section 1: the IPv4-mapped IPv6 format can be used
			// "when the <AFI/SAFI> is <2/1>, <2/2>, or <2/4>".
			// This recognizes the field, not a 6PE dataplane.
			return NextHopProfile{Lengths: ipv6NextHopLens, IPv6Roles: true, MappedIPv4: true}
		case SAFIVPN:
			// RFC 4659 Section 3.2.1.2 defines zero RD plus mapped IPv6.
			return NextHopProfile{Lengths: vpnIPv6NextHopLens, MappedIPv4: true, RDPerAddress: true, EncodeIPv4Mapped: true, DecodeUnprefixedIPv6: true}
		case SAFISRPolicy:
			return NextHopProfile{Lengths: ipv4ExtendedNextHopLens, IPv6Roles: true}
		case SAFIMVPN:
			return NextHopProfile{Lengths: mvpnNextHopLens}
		case SAFIFlowSpec, SAFIEVPN:
			// FlowSpec: permissive (no test coverage yet for strict validation)
			// EVPN: uses AFI L2VPN (25), not IPv6
		default:
			// The wire SAFI set is open; unknown combinations have no length table.
		}
	case AFIL2VPN:
		switch safi {
		case SAFIEVPN:
			return NextHopProfile{Lengths: dualAFINextHopLens}
		case SAFIUnicast, SAFIMulticast, SAFIMPLSLabel, SAFIVPN, SAFIFlowSpec, SAFISRPolicy, SAFIMVPN:
			// These SAFIs don't apply to L2VPN AFI
		default:
			// The wire SAFI set is open; unknown combinations have no length table.
		}
	default:
		// The wire AFI set is open; unknown combinations have no length table.
	}
	return NextHopProfile{}
}

// ValidNextHopLens returns the profile's shared read-only lengths. Callers MUST
// NOT mutate them. Unknown or independently defined layouts return nil.
func ValidNextHopLens(afi AFI, safi SAFI) []int {
	return MPNextHopProfile(afi, safi).Lengths
}

// Shared read-only storage keeps length admission allocation-free on the wire
// path, without requiring ValidNextHopLens to be inlined.
var (
	ipv4ExtendedNextHopLens = []int{4, 16, 32}
	ipv6NextHopLens         = []int{16, 32}
	vpnIPv4NextHopLens      = []int{12, 24, 48}
	vpnIPv6NextHopLens      = []int{24, 48}
	dualAFINextHopLens      = []int{4, 16}
)

// mvpnNextHopLens are the next-hop lengths an MCAST-VPN route carries, under
// either AFI.
//
// RFC 6514 Section 5: "The Next Hop field of the MP_REACH_NLRI attribute of the
// route MUST be set to the same IP address as the one carried in the
// Originating Router's IP Address field", and that field holds an IPv4 or an
// IPv6 address whichever AFI the NLRI uses. So the AFI of an MCAST-VPN route
// does not decide the family of its next hop, and RFC 8950 Section 3 says what
// does: the length field, "out of the set of protocols allowed by the AFI/SAFI
// definition". This slice IS that set for SAFI 5.
var mvpnNextHopLens = []int{4, 16, 32}

// parseNextHops uses the family profile's framing and preserves the received
// native or mapped representation in inline address storage.
func parseNextHops(afi AFI, safi SAFI, data []byte) (NextHopAddrs, error) {
	return MPNextHopProfile(afi, safi).DecodeNextHops(afi, data)
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
// is not required to carry any other path attributes.".
type MPUnreachNLRI struct {
	AFI  AFI    // RFC 4760 Section 4: Address Family Identifier (2 octets)
	SAFI SAFI   // RFC 4760 Section 4: Subsequent Address Family Identifier (1 octet)
	NLRI []byte // RFC 4760 Section 4: Withdrawn Routes (variable)
}

// Code returns AttrMPUnreachNLRI (Type Code 15).
// RFC 4760 Section 4: MP_UNREACH_NLRI has Type Code 15.
func (m *MPUnreachNLRI) Code() AttributeCode { return AttrMPUnreachNLRI }

// Flags returns FlagOptional (non-transitive) per RFC 4760 Section 4.
func (m *MPUnreachNLRI) Flags() AttributeFlags { return FlagOptional }

// Len returns the packed length in bytes (AFI + SAFI + NLRI).
func (m *MPUnreachNLRI) Len() int {
	return 2 + 1 + len(m.NLRI)
}

// WriteTo writes the MP_UNREACH_NLRI attribute value into buf at offset.
func (m *MPUnreachNLRI) WriteTo(buf []byte, off int) int {
	// RFC 4760 Section 4: Address Family Identifier (2 octets)
	binary.BigEndian.PutUint16(buf[off:], uint16(m.AFI))

	// RFC 4760 Section 4: Subsequent Address Family Identifier (1 octet)
	buf[off+2] = byte(m.SAFI)

	// RFC 4760 Section 4: Withdrawn Routes (variable)
	n := copy(buf[off+3:], m.NLRI)

	return 3 + n
}

// WriteToWithContext writes MP_UNREACH_NLRI - context-independent.
func (m *MPUnreachNLRI) WriteToWithContext(buf []byte, off int, _, _ *bgpctx.EncodingContext) int {
	return m.WriteTo(buf, off)
}

// CheckedWriteTo validates capacity before writing.
func (m *MPUnreachNLRI) CheckedWriteTo(buf []byte, off int) (int, error) {
	needed := m.Len()
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return m.WriteTo(buf, off), nil
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
