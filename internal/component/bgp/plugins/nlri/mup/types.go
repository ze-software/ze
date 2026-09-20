// Design: docs/architecture/wire/nlri.md — MUP NLRI plugin
// RFC: rfc/short/draft-ietf-bess-mup-safi.md
// Related: rfc7606.go -- the RFC 7606 Section 5.4 ruling over the types declared here
// Related: json.go -- the two JSON writers that read the fields parsed here
//
// Package bgp_mup implements Mobile User Plane NLRI (draft-ietf-bess-mup-safi, SAFI 85).
package mup

import (
	"encoding/binary"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Type aliases for shared nlri types.
type (
	Family             = family.Family
	AFI                = family.AFI
	SAFI               = family.SAFI
	NLRI               = nlri.NLRI
	RouteDistinguisher = nlri.RouteDistinguisher
)

// Re-export constants.
const (
	AFIIPv4 = family.AFIIPv4
	AFIIPv6 = family.AFIIPv6
	SAFIMUP = family.SAFIMUP
	RDType0 = nlri.RDType0
	RDType1 = nlri.RDType1
)

// Family registrations for MUP.
var (
	IPv4MUP = family.MustRegister(AFIIPv4, SAFIMUP, "ipv4", "mup")
	IPv6MUP = family.MustRegister(AFIIPv6, SAFIMUP, "ipv6", "mup")
)

var ParseRDString = nlri.ParseRDString

// Errors for MUP parsing. Each one names the field that did not add up, so an
// operator reading the log knows which octet to look at.
var (
	ErrMUPTruncated       = errors.New("mup: truncated data")
	ErrMUPInvalidType     = errors.New("mup: invalid route type")
	ErrMUPPrefixLength    = errors.New("mup: prefix length exceeds the address size of the AFI")
	ErrMUPBodyLength      = errors.New("mup: route type length disagrees with the octets present")
	ErrMUPAddressLength   = errors.New("mup: address length does not match the AFI")
	ErrMUPEndpointLength  = errors.New("mup: endpoint address length is neither 32 nor 128")
	ErrMUPSourceLength    = errors.New("mup: source address length is neither 0, 32 nor 128")
	ErrMUPEndpointTooLong = errors.New("mup: endpoint length leaves more than a 4-octet TEID")
	ErrMUPTEIDZero        = errors.New("mup: TEID is zero")
	ErrMUPTLV             = errors.New("mup: TLV does not fit the octets present")
)

// Wire layout of every BGP-MUP NLRI, draft-ietf-bess-mup-safi Section 3.1:
//
//	+-----------------------------------+
//	|    Architecture Type (1 octet)    |  offset 0
//	+-----------------------------------+
//	|       Route Type (2 octets)       |  offset 1
//	+-----------------------------------+
//	|         Length (1 octet)          |  offset 3
//	+-----------------------------------+
//	|  Route Type specific (variable)   |  offset 4, Length octets
//	+-----------------------------------+
//
// Every route type this document defines opens its Route Type specific field
// with an 8-octet RD encoded as described in RFC 4364.
const (
	mupHeaderLen = 4
	mupRDLen     = 8
)

// MUPRouteType identifies the type of MUP route.
type MUPRouteType uint16

// MUP route types per draft-ietf-bess-mup-safi.
const (
	MUPISD  MUPRouteType = 1 // Interwork Segment Discovery route
	MUPDSD  MUPRouteType = 2 // Direct Segment Discovery route
	MUPT1ST MUPRouteType = 3 // Type 1 Session Transformed route
	MUPT2ST MUPRouteType = 4 // Type 2 Session Transformed route
)

// String returns a human-readable route type name.
func (t MUPRouteType) String() string {
	switch t {
	case MUPISD:
		return "isd"
	case MUPDSD:
		return "dsd"
	case MUPT1ST:
		return "t1st"
	case MUPT2ST:
		return "t2st"
	default:
		return "type(" + textbuf.StringUint8(uint8(t)) + ")"
	}
}

// MUPArchType identifies the MUP architecture type.
type MUPArchType uint8

// MUP architecture types per draft-ietf-bess-mup-safi.
const (
	MUPArch3GPP5G MUPArchType = 1 // 3GPP 5G architecture
)

// MUP represents a Mobile User Plane NLRI (draft-ietf-bess-mup-safi).
//
// body holds the Route Type specific field exactly as it arrived, the RD
// included, so WriteTo reproduces the received octets and a relayed route is
// byte-identical. The parsed fields are read out of body once, at parse time,
// and are never written back from: nothing re-serializes them.
//
// Which parsed field carries meaning is decided by routeType, and parsed says
// whether any of them were read at all. A route type ze does not implement
// keeps its octets and reports parsed false, so a reader never sees a
// half-filled route that looks like a real one.
type MUP struct {
	rd   RouteDistinguisher
	body []byte

	prefix   netip.Addr // ISD, T1ST: prefix address, zero-padded to the AFI width
	address  netip.Addr // DSD: address of the originating BGP speaker
	endpoint netip.Addr // T1ST, T2ST: endpoint address
	source   netip.Addr // T1ST: source address, invalid when none is carried

	teid uint32 // T1ST, T2ST

	afi          AFI
	archType     MUPArchType
	routeType    MUPRouteType
	prefixBits   uint8 // ISD, T1ST
	endpointBits uint8 // T1ST endpoint address length, T2ST endpoint length
	sourceBits   uint8 // T1ST, 0 when no source address is carried
	qfi          uint8 // T1ST
	teidOctets   uint8 // T2ST: octets the TEID occupies, 0 to 4
	parsed       bool
}

// ParseMUP parses one MUP NLRI from wire format and returns the octets that
// follow it in the same MP_REACH_NLRI or MP_UNREACH_NLRI attribute.
//
// draft-ietf-bess-mup-safi Section 3.1: "The Length field indicates the length
// in octets of the Route Type specific field of the BGP-MUP NLRI." The Length
// field is what finds the next NLRI, so it is read before the body is judged.
//
// A body that does not add up is an error rather than a partly filled route:
// the caller cannot tell a zero prefix length from a prefix length ze failed to
// read (ai/rules/principles.md).
func ParseMUP(afi AFI, data []byte) (*MUP, []byte, error) {
	if len(data) < mupHeaderLen {
		return nil, nil, ErrMUPTruncated
	}

	archType := MUPArchType(data[0])
	routeType := MUPRouteType(binary.BigEndian.Uint16(data[1:3]))
	bodyLen := int(data[3])

	if len(data) < mupHeaderLen+bodyLen {
		return nil, nil, ErrMUPTruncated
	}

	mup := &MUP{
		body:      data[mupHeaderLen : mupHeaderLen+bodyLen],
		afi:       afi,
		archType:  archType,
		routeType: routeType,
	}
	rest := data[mupHeaderLen+bodyLen:]

	// Section 3.1: "Any other Route Types MUST be silently ignored upon a
	// receipt if a BGP speaker supports only 3gpp-5G architecture type." Ignored
	// means the octets are skipped by their Length, which the walk above already
	// did, and the route is reported as one ze did not read.
	if !routeType.Implemented(archType) {
		return mup, rest, nil
	}

	if err := mup.parseBody(); err != nil {
		return nil, nil, err
	}
	mup.parsed = true

	return mup, rest, nil
}

// parseBody reads the Route Type specific field into the parsed fields.
//
// The RD is common to all four route types (Sections 3.1.1 to 3.1.4), so it is
// read here and each route type parser starts after it.
func (m *MUP) parseBody() error {
	if len(m.body) < mupRDLen {
		return ErrMUPTruncated
	}
	rd, err := nlri.ParseRouteDistinguisher(m.body[:mupRDLen])
	if err != nil {
		return err
	}
	m.rd = rd

	rest := m.body[mupRDLen:]
	switch m.routeType {
	case MUPISD:
		return m.parseBodyISD(rest)
	case MUPDSD:
		return m.parseBodyDSD(rest)
	case MUPT1ST:
		return m.parseBodyT1ST(rest)
	case MUPT2ST:
		return m.parseBodyT2ST(rest)
	}
	// Unreachable: Implemented gates the call, and it answers for these four.
	return ErrMUPInvalidType
}

// parseBodyISD reads an Interwork Segment Discovery route, Section 3.1.1:
//
//	RD (8 octets) | Prefix Length (1 octet) | Prefix (variable)
//
// No architecture specific field and no TLVs follow, so any octet past the
// prefix is a Length that disagrees with the route type.
func (m *MUP) parseBodyISD(rest []byte) error {
	addr, bits, used, err := m.parsePrefixField(rest)
	if err != nil {
		return err
	}
	if used != len(rest) {
		return ErrMUPBodyLength
	}
	m.prefix = addr
	m.prefixBits = bits
	return nil
}

// parseBodyDSD reads a Direct Segment Discovery route, Section 3.1.2:
//
//	RD (8 octets) | Address (4 or 16 octets)
//
// Section 3.1.2: "If the AFI is IPv4 then the address length is 4 octets
// otherwise it is considered as a malformed NLRI.  If the AFI is IPv6 then the
// address length is 16 octets otherwise it is considered as a malformed NLRI."
// The AFI alone decides the width here, so the length is checked and not read.
func (m *MUP) parseBodyDSD(rest []byte) error {
	if len(rest) != addrOctets(m.afi) {
		return ErrMUPAddressLength
	}
	m.address = addrFromOctets(m.afi, rest)
	return nil
}

// parseBodyT1ST reads a Type 1 Session Transformed route, Sections 3.1.3 and
// 3.1.3.1:
//
//	RD (8) | Prefix Length (1) | Prefix (variable) | TEID (4) | QFI (1) |
//	Endpoint Address Length (1) | Endpoint Address (variable) |
//	Source Address Length (1) | Source Address (variable) | TLVs (variable)
//
// The Source Address Length octet is absent when nothing follows the endpoint
// address. The draft states the field but lets a sender stop after the endpoint
// (Section 3.1.3.1, "Source Address Length is 0 bytes then the Source address
// is not carried within the NLRI"), and that is the shape ExaBGP puts on the
// wire, so both spellings of "no source address" are read here.
func (m *MUP) parseBodyT1ST(rest []byte) error {
	addr, bits, used, err := m.parsePrefixField(rest)
	if err != nil {
		return err
	}
	m.prefix = addr
	m.prefixBits = bits
	rest = rest[used:]

	// TEID (4 octets) + QFI (1 octet) + Endpoint Address Length (1 octet).
	const fixedLen = 6
	if len(rest) < fixedLen {
		return ErrMUPTruncated
	}

	// Section 3.1.3.1: "The TEID value of 0 is considered as an invalid and a
	// malformed TEID.  A BGP speaker MUST handle such a malformed NLRI as a
	// "Treat-as-withdraw" [RFC7606]."
	m.teid = binary.BigEndian.Uint32(rest[:4])
	if m.teid == 0 {
		return ErrMUPTEIDZero
	}
	m.qfi = rest[4]

	// Section 3.1.3.1: "Endpoint Address field contains of an IPv4 address, then
	// the value of the Endpoint Address Length field is 32.  If the Endpoint
	// Address field contains of an IPv6 Address, then the value of the Endpoint
	// Address Length field is 128.  Any other value is considered as an invalid
	// and a malformed Endpoint Address."
	m.endpointBits = rest[5]
	endpointOctets, ok := addressBitsOctets(m.endpointBits)
	if !ok {
		return ErrMUPEndpointLength
	}
	rest = rest[fixedLen:]
	if len(rest) < endpointOctets {
		return ErrMUPTruncated
	}
	m.endpoint = addrFromOctets(bitsAFI(m.endpointBits), rest[:endpointOctets])
	rest = rest[endpointOctets:]

	if len(rest) == 0 {
		return nil
	}

	// Section 3.1.3.1 gives the source address the same two lengths as the
	// endpoint address, and 0 for "not carried".
	m.sourceBits = rest[0]
	rest = rest[1:]
	if m.sourceBits != 0 {
		sourceOctets, sourceOK := addressBitsOctets(m.sourceBits)
		if !sourceOK {
			return ErrMUPSourceLength
		}
		if len(rest) < sourceOctets {
			return ErrMUPTruncated
		}
		m.source = addrFromOctets(bitsAFI(m.sourceBits), rest[:sourceOctets])
		rest = rest[sourceOctets:]
	}

	return validateTLVs(rest)
}

// parseBodyT2ST reads a Type 2 Session Transformed route, Sections 3.1.4 and
// 3.1.4.1:
//
//	RD (8) | Endpoint Length (1) | Endpoint Address (4 or 16) |
//	TEID (0 to 4 octets) | TLVs (variable)
//
// Section 3.1.4: "If the AFI is IPv4, then the maximum Endpoint length is 64
// otherwise it is considered as a malformed NLRI.  If the AFI is IPv6, then the
// maximum Endpoint length is 160 otherwise it is considered as a malformed
// NLRI." The endpoint address takes the whole width of the AFI, so the bits
// above it are the architecture specific endpoint identifier, the TEID.
func (m *MUP) parseBodyT2ST(rest []byte) error {
	if len(rest) < 1 {
		return ErrMUPTruncated
	}
	m.endpointBits = rest[0]

	addressBits := uint8(addrOctets(m.afi) * 8) //nolint:gosec // 4 or 16 octets
	if m.endpointBits < addressBits {
		return ErrMUPEndpointLength
	}
	teidBits := m.endpointBits - addressBits
	if teidBits > 32 {
		return ErrMUPEndpointTooLong
	}

	rest = rest[1:]
	if len(rest) < addrOctets(m.afi) {
		return ErrMUPTruncated
	}
	m.endpoint = addrFromOctets(m.afi, rest[:addrOctets(m.afi)])
	rest = rest[addrOctets(m.afi):]

	m.teidOctets = (teidBits + 7) / 8
	if len(rest) < int(m.teidOctets) {
		return ErrMUPTruncated
	}
	for _, octet := range rest[:m.teidOctets] {
		m.teid = m.teid<<8 | uint32(octet)
	}
	// Section 3.1.4.1: "The TEID value of 0 is considered as an invalid and a
	// malformed TEID." A TEID of zero octets carries no value at all, which
	// Section 3.1.4.1 permits ("The TEID length is zero or more bytes as
	// determined by the Endpoint Length value"), so only a present TEID is held
	// to the rule.
	if m.teidOctets > 0 && m.teid == 0 {
		return ErrMUPTEIDZero
	}

	return validateTLVs(rest[m.teidOctets:])
}

// parsePrefixField reads the Prefix Length and Prefix pair that opens the ISD
// and T1ST route types, and returns the address, its length in bits, and the
// octets it consumed.
//
// Section 3.1.1: "If the AFI is IPv4, then the maximum value of the Prefix
// Length is 32 bits otherwise it is considered as a malformed NLRI.  If the AFI
// is IPv6, then the maximum value of of the Prefix length is 128 bits otherwise
// it is considered as a malformed NLRI."
//
// The prefix carries only the significant octets, so the address is zero-padded
// back to the width of the AFI.
func (m *MUP) parsePrefixField(data []byte) (netip.Addr, uint8, int, error) {
	if len(data) < 1 {
		return netip.Addr{}, 0, 0, ErrMUPTruncated
	}
	bits := data[0]
	maxBits := addrOctets(m.afi) * 8
	if int(bits) > maxBits {
		return netip.Addr{}, 0, 0, ErrMUPPrefixLength
	}
	octets := (int(bits) + 7) / 8
	if len(data) < 1+octets {
		return netip.Addr{}, 0, 0, ErrMUPTruncated
	}
	return addrFromOctets(m.afi, data[1:1+octets]), bits, 1 + octets, nil
}

// validateTLVs checks the framing of the optional TLVs that can follow the
// mandatory fields of a session transformed route.
//
// Section 3.1.3.1 and Section 3.1.4.1 decide by the count of remaining octets:
// "- 0: no TLVs are present.", "- 1: encoding is invalid (a valid TLV requires
// at minimum a Type byte and a Length byte); MUST be treated as Treat-as-
// withdraw.", "- >= 2: one or more TLVs are present."
//
// Section 3.1.5 gives every TLV a 1-octet Type, a 1-octet Length and Length
// octets of Value. The values are not read: "unknown TLV types MUST be ignored
// for local processing and MUST be propagated unchanged when re-advertising the
// route", and the whole Route Type specific field is kept for that. The framing
// is still walked, because "any TLV parsing error MUST result in
// Treat-as-withdraw".
func validateTLVs(data []byte) error {
	for off := 0; off < len(data); {
		if off+2 > len(data) {
			return ErrMUPTLV
		}
		off += 2 + int(data[off+1])
		if off > len(data) {
			return ErrMUPTLV
		}
	}
	return nil
}

// addrOctets returns the width of an address under afi: 4 octets for IPv4, 16
// for IPv6.
func addrOctets(afi AFI) int {
	if afi == AFIIPv6 {
		return 16
	}
	return 4
}

// addressBitsOctets converts an endpoint or source address length field to the
// octets it covers, and reports whether the draft allows the value.
//
// Sections 3.1.3.1 names 32 and 128 for both fields, and nothing else.
func addressBitsOctets(bits uint8) (int, bool) {
	switch bits {
	case 32:
		return 4, true
	case 128:
		return 16, true
	}
	return 0, false
}

// bitsAFI answers which AFI an address of bits bits belongs to. The endpoint
// and source addresses of a T1ST route carry their own length, so an IPv4
// endpoint can appear under the IPv6 AFI and the address width follows the
// field rather than the family.
func bitsAFI(bits uint8) AFI {
	if bits == 128 {
		return AFIIPv6
	}
	return AFIIPv4
}

// addrFromOctets builds an address of the AFI width from the leading octets of
// a prefix, zero-padding the rest. data MUST NOT be longer than the AFI width,
// which every caller checks first.
func addrFromOctets(afi AFI, data []byte) netip.Addr {
	if afi == AFIIPv6 {
		var full [16]byte
		copy(full[:], data)
		return netip.AddrFrom16(full)
	}
	var full [4]byte
	copy(full[:], data)
	return netip.AddrFrom4(full)
}

// Family returns the address family.
func (m *MUP) Family() Family {
	return Family{AFI: m.afi, SAFI: SAFIMUP}
}

// ArchType returns the MUP architecture type.
func (m *MUP) ArchType() MUPArchType { return m.archType }

// RouteType returns the MUP route type.
func (m *MUP) RouteType() MUPRouteType { return m.routeType }

// RD returns the Route Distinguisher. It is the zero RD for a route type ze
// does not implement, whose octets are carried and re-encoded but not read.
//
// The `parsed` field says which of the two an NLRI is. It has no accessor,
// because every reader of it is in this package: the JSON writers, which
// publish it as the `parsed` member ExaBGP writes. A consumer outside the
// package holds an opaque NLRI rather than a *MUP.
func (m *MUP) RD() RouteDistinguisher { return m.rd }

// Bytes allocates a standalone slice and delegates to WriteTo; hot-path
// senders should call WriteTo directly with a pool buffer.
func (m *MUP) Bytes() []byte {
	buf := make([]byte, m.Len())
	m.WriteTo(buf, 0)
	return buf
}

// Len returns the wire-format length in bytes: the 4-octet header plus the
// Route Type specific field.
func (m *MUP) Len() int {
	return mupHeaderLen + len(m.body)
}

// PathID returns 0.
func (m *MUP) PathID() uint32 { return 0 }

// HasPathID returns false.
func (m *MUP) HasPathID() bool { return false }

// SupportsAddPath returns false - MUP doesn't support ADD-PATH.
func (m *MUP) SupportsAddPath() bool { return false }

// String returns command-style format for API round-trip compatibility.
func (m *MUP) String() string {
	if m.parsed {
		return m.routeType.String() + " rd " + m.rd.String()
	}
	return m.routeType.String()
}

// WriteTo writes the MUP NLRI directly to buf at offset. Zero-alloc.
//
// The Route Type specific field is copied as it arrived, so a route ze relays
// leaves as the octets that reached it, TLVs and all.
func (m *MUP) WriteTo(buf []byte, off int) int {
	buf[off] = byte(m.archType)
	binary.BigEndian.PutUint16(buf[off+1:], uint16(m.routeType))
	buf[off+3] = byte(len(m.body)) //nolint:gosec // body came from a 1-octet Length
	copy(buf[off+mupHeaderLen:], m.body)
	return mupHeaderLen + len(m.body)
}
