// Design: docs/architecture/wire/nlri.md — MVPN NLRI plugin
// RFC: rfc/short/rfc6514.md -- MCAST-VPN NLRI (SAFI 5), Sections 4, 4.5 and 4.6
// Related: rfc7606.go -- the RFC 7606 Section 5.4 ruling over the route types declared here
//
// Package bgp_mvpn implements Multicast VPN NLRI (RFC 6514, SAFI 5).
package mvpn

import (
	"encoding/binary"
	"errors"
	"fmt"
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

// Re-export constants and variables.
const (
	AFIIPv4  = family.AFIIPv4
	AFIIPv6  = family.AFIIPv6
	SAFIMVPN = family.SAFIMVPN
	RDType0  = nlri.RDType0
	RDType1  = nlri.RDType1
)

// Family registrations for MVPN.
var (
	IPv4MVPN = family.MustRegister(AFIIPv4, SAFIMVPN, "ipv4", "mvpn")
	IPv6MVPN = family.MustRegister(AFIIPv6, SAFIMVPN, "ipv6", "mvpn")
)

// Errors for MVPN parsing.
var ErrMVPNTruncated = errors.New("mvpn: truncated data")

// MVPNRouteType identifies the type of MVPN route.
// RFC 6514 Section 4 defines the route types for MCAST-VPN NLRI.
type MVPNRouteType uint8

// MVPN route types per RFC 6514 Section 4.
const (
	MVPNIntraASIPMSIAD MVPNRouteType = 1 // Intra-AS I-PMSI A-D route.
	MVPNInterASIPMSIAD MVPNRouteType = 2 // Inter-AS I-PMSI A-D route.
	MVPNSPMSIAD        MVPNRouteType = 3 // S-PMSI A-D route.
	MVPNLeafAD         MVPNRouteType = 4 // Leaf A-D route.
	MVPNSourceActive   MVPNRouteType = 5 // Source Active A-D route.
	MVPNSharedTreeJoin MVPNRouteType = 6 // Shared Tree Join route (C-*,C-G).
	MVPNSourceTreeJoin MVPNRouteType = 7 // Source Tree Join route (C-S,C-G).
)

// String returns a human-readable route type name.
func (t MVPNRouteType) String() string {
	switch t {
	case MVPNIntraASIPMSIAD:
		return "intra-as-i-pmsi-ad"
	case MVPNInterASIPMSIAD:
		return "inter-as-i-pmsi-ad"
	case MVPNSPMSIAD:
		return "s-pmsi-ad"
	case MVPNLeafAD:
		return "leaf-ad"
	case MVPNSourceActive:
		return "source-active"
	case MVPNSharedTreeJoin:
		return "shared-tree-join"
	case MVPNSourceTreeJoin:
		return "source-tree-join"
	default:
		return "type(" + textbuf.StringUint8(uint8(t)) + ")"
	}
}

// bodyParsed reports whether ze reads the route type specific field of this
// route type.
//
// RFC 6514 Section 4.5 defines the Source Active A-D route body and Section 4.6
// the two C-multicast route bodies. Sections 4.1 to 4.4 define the four A-D
// route bodies, which ze keeps as octets. ExaBGP draws the same line (sourcead.py
// code 5, sharedjoin.py code 6, sourcejoin.py code 7, and GenericMVPN for the
// rest), so the two implementations publish the same members for the same bytes.
func (t MVPNRouteType) bodyParsed() bool {
	switch t {
	case MVPNSourceActive, MVPNSharedTreeJoin, MVPNSourceTreeJoin:
		return true
	case MVPNIntraASIPMSIAD, MVPNInterASIPMSIAD, MVPNSPMSIAD, MVPNLeafAD:
		return false
	}
	return false
}

// hasSourceAS reports whether this route type carries the Source AS field.
//
// RFC 6514 Section 4.6 gives a Shared Tree Join and a Source Tree Join route
// "Source AS (4 octets)" between the RD and the Multicast Source Length.
// Section 4.5 gives the Source Active A-D route no such field, so this decides
// the offset of every field behind it.
func (t MVPNRouteType) hasSourceAS() bool {
	return t == MVPNSharedTreeJoin || t == MVPNSourceTreeJoin
}

// name returns the route type's long name, as the "name" JSON member.
//
// The spellings are ExaBGP's NAME class variables, copied letter for letter,
// including the lower-case "route" the two C-multicast names end on. ExaBGP
// decides this member, so a prettier spelling here would be a difference a
// reader of both feeds has to reconcile. A route type ze does not parse has no
// name, and publishes none.
func (t MVPNRouteType) name() string {
	switch t {
	case MVPNSourceActive:
		return "Source Active A-D Route"
	case MVPNSharedTreeJoin:
		return "C-Multicast Shared Tree Join route"
	case MVPNSourceTreeJoin:
		return "C-Multicast Source Tree Join route"
	case MVPNIntraASIPMSIAD, MVPNInterASIPMSIAD, MVPNSPMSIAD, MVPNLeafAD:
		return ""
	}
	return ""
}

// MVPN represents one Multicast VPN NLRI (RFC 6514).
//
// RFC 6514 Section 4: "Route Type (1 octet), Length (1 octet), Route Type
// specific (variable)". packed holds those octets exactly as they arrived, the
// two header octets included. Every other field is a view parseMVPN filled
// from it, so the wire form and the rendered form cannot disagree: Bytes()
// reproduces what was read rather than re-encoding it, and the JSON writers
// publish the same octets as "raw".
type MVPN struct {
	packed    []byte
	rd        RouteDistinguisher
	source    netip.Addr
	group     netip.Addr
	sourceAS  uint32
	afi       AFI
	rdPresent bool
}

// rdLen is the octet count RFC 4364 gives a Route Distinguisher, which
// RFC 6514 Sections 4.5 and 4.6 place first in the route type specific field.
const rdLen = 8

// sourceASLen is the octet count RFC 6514 Section 4.6 gives the Source AS
// field of a Shared Tree Join and a Source Tree Join route.
const sourceASLen = 4

// maxMVPNBodyLen is the largest route type specific field the one-octet
// RFC 6514 Section 4 Length field can describe.
const maxMVPNBodyLen = 255

var (
	// ErrMVPNAddressLength reports a Multicast Source Length or Multicast Group
	// Length field that names neither an IPv4 nor an IPv6 address.
	ErrMVPNAddressLength = errors.New("mvpn: multicast address length is neither 32 nor 128 bits")

	// ErrMVPNTrailingOctets reports a Length field that describes more octets
	// than the route type's own fields account for.
	ErrMVPNTrailingOctets = errors.New("mvpn: route body carries octets the route type does not define")
)

// frameMVPN joins a route type and its route type specific field into the wire
// form of RFC 6514 Section 4.
//
// The Length field is one octet, so a body over 255 octets has no wire form at
// all. No MVPN is built for one, rather than one whose Length octet contradicts
// the bytes behind it (ai/rules/principles.md).
func frameMVPN(routeType MVPNRouteType, body []byte) []byte {
	if len(body) > maxMVPNBodyLen {
		return nil
	}
	packed := make([]byte, 0, 2+len(body))
	packed = append(packed, byte(routeType), byte(len(body)))
	return append(packed, body...)
}

// NewMVPN creates a new MVPN NLRI carrying body as its route type specific
// field. It answers nil for a body the one-octet Length field cannot describe.
func NewMVPN(routeType MVPNRouteType, body []byte) *MVPN {
	packed := frameMVPN(routeType, body)
	if packed == nil {
		return nil
	}
	return &MVPN{packed: packed, afi: AFIIPv4}
}

// newMVPNWithRD creates a new MVPN NLRI whose route type specific field opens
// with a Route Distinguisher. It answers nil for a body the one-octet Length
// field cannot describe.
func newMVPNWithRD(afi AFI, routeType MVPNRouteType, rd RouteDistinguisher, data []byte) *MVPN {
	body := make([]byte, rdLen, rdLen+len(data))
	rd.WriteTo(body, 0)
	body = append(body, data...)

	packed := frameMVPN(routeType, body)
	if packed == nil {
		return nil
	}
	return &MVPN{packed: packed, rd: rd, rdPresent: true, afi: afi}
}

// parseMVPN parses one MVPN NLRI from wire format and answers the octets that
// follow it.
//
// RFC 6514 Section 4 defines the MCAST-VPN NLRI format:
//
//	Route Type (1 octet) + Length (1 octet) + Route Type specific (variable)
//
// The caller owns the rest of the section. MP_REACH_NLRI packs as many NLRIs as
// fit into one attribute, so a caller that drops the second answer publishes the
// first route and discards every route behind it
// (plan/journal/validated-value-discarded-by-its-caller.md).
func parseMVPN(afi AFI, data []byte) (*MVPN, []byte, error) {
	if len(data) < 2 {
		return nil, nil, ErrMVPNTruncated
	}

	routeType := MVPNRouteType(data[0])
	bodyLen := int(data[1])
	if len(data) < 2+bodyLen {
		return nil, nil, ErrMVPNTruncated
	}

	m := &MVPN{afi: afi, packed: data[:2+bodyLen]}
	if err := m.parseBody(routeType, data[2:2+bodyLen]); err != nil {
		return nil, nil, err
	}
	return m, data[2+bodyLen:], nil
}

// parseBody reads the route type specific field of the route types ze parses,
// and records nothing but the Route Distinguisher for the rest.
//
// A route type ze does not parse keeps its octets and is published as
// parsed:false, which is what ExaBGP's GenericMVPN does with the same bytes.
// The Route Distinguisher is still read where the body is long enough to hold
// one, because String() names it and the command round-trip reads that name.
func (m *MVPN) parseBody(routeType MVPNRouteType, body []byte) error {
	if !routeType.bodyParsed() {
		if len(body) >= rdLen {
			if rd, err := nlri.ParseRouteDistinguisher(body[:rdLen]); err == nil {
				m.rd, m.rdPresent = rd, true
			}
		}
		return nil
	}

	// RFC 6514 Section 4.5 and Section 4.6: "The RD is encoded as described in
	// [RFC4364]."
	if len(body) < rdLen {
		return ErrMVPNTruncated
	}
	rd, err := nlri.ParseRouteDistinguisher(body[:rdLen])
	if err != nil {
		return err
	}
	m.rd, m.rdPresent = rd, true
	off := rdLen

	// RFC 6514 Section 4.6: "The Source AS contains an ASN. Two-octet ASNs are
	// encoded in the low-order two octets of the Source AS field." Section 4.5
	// gives the Source Active A-D route no such field, so the offset of every
	// field behind it depends on the route type.
	if routeType.hasSourceAS() {
		if len(body) < off+sourceASLen {
			return ErrMVPNTruncated
		}
		m.sourceAS = binary.BigEndian.Uint32(body[off : off+sourceASLen])
		off += sourceASLen
	}

	if m.source, off, err = parseMVPNAddress(body, off); err != nil {
		return err
	}
	if m.group, off, err = parseMVPNAddress(body, off); err != nil {
		return err
	}

	// The Length octet must describe exactly the fields the route type defines.
	// Octets left over mean the Length field and the body disagree about what
	// this route is, so nothing is published for it rather than a route filled
	// from the part that happened to parse (ai/rules/principles.md).
	if off != len(body) {
		return ErrMVPNTrailingOctets
	}
	return nil
}

// parseMVPNAddress reads one Multicast Source or Multicast Group field: a
// length in BITS, then the address.
//
// RFC 6514 Section 4.5: "If the Multicast Source field contains an IPv4
// address, then the value of the Multicast Source Length field is 32. If the
// Multicast Source field contains an IPv6 address, then the value of the
// Multicast Source Length field is 128." Any other length names an address ze
// cannot read, including the zero length the same section puts outside the
// document's scope, so it is refused rather than guessed at.
//
// The length octet decides the address family on its own. RFC 6514 Section 4
// says the NLRI's AFI decides it too, and the two are required to agree; where
// they do not, ze renders what the octets in front of it really say. ExaBGP
// reads the same field the same way, so both implementations answer alike.
func parseMVPNAddress(body []byte, off int) (netip.Addr, int, error) {
	if off >= len(body) {
		return netip.Addr{}, 0, ErrMVPNTruncated
	}

	var size int
	switch bits := body[off]; bits {
	case 32:
		size = 4
	case 128:
		size = 16
	default:
		return netip.Addr{}, 0, fmt.Errorf("%w: %d", ErrMVPNAddressLength, bits)
	}
	off++

	if off+size > len(body) {
		return netip.Addr{}, 0, ErrMVPNTruncated
	}
	addr, ok := netip.AddrFromSlice(body[off : off+size])
	if !ok {
		return netip.Addr{}, 0, ErrMVPNTruncated
	}
	return addr, off + size, nil
}

// Family returns the address family.
func (m *MVPN) Family() Family {
	return Family{AFI: m.afi, SAFI: SAFIMVPN}
}

// RouteType returns the MVPN route type, read from the wire octet that carries
// it so no second copy exists to drift from it.
func (m *MVPN) RouteType() MVPNRouteType { return MVPNRouteType(m.packed[0]) }

// RD returns the Route Distinguisher.
func (m *MVPN) RD() RouteDistinguisher { return m.rd }

// Bytes returns the wire-format encoding. It aliases the octets the NLRI was
// parsed from, so a parsed NLRI re-encodes to exactly what arrived.
func (m *MVPN) Bytes() []byte { return m.packed }

// Len returns the length in bytes.
func (m *MVPN) Len() int { return len(m.packed) }

// PathID returns 0 (MVPN doesn't typically use ADD-PATH).
func (m *MVPN) PathID() uint32 { return 0 }

// HasPathID returns false.
func (m *MVPN) HasPathID() bool { return false }

// SupportsAddPath returns false - MVPN doesn't support ADD-PATH.
func (m *MVPN) SupportsAddPath() bool { return false }

// String returns command-style format for API round-trip compatibility.
func (m *MVPN) String() string {
	if m.rdPresent {
		return m.RouteType().String() + " rd " + m.rd.String()
	}
	return m.RouteType().String()
}

// WriteTo writes the MVPN NLRI directly to buf at offset.
func (m *MVPN) WriteTo(buf []byte, off int) int {
	return copy(buf[off:], m.packed)
}
