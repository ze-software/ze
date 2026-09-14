// Design: docs/architecture/core-design.md — address forms permitted in the MP_REACH next-hop field
// RFC: rfc/short/rfc2545.md
// RFC: rfc/short/draft-ietf-idr-linklocal-capability.md — the 16-octet Link-Local-only form
// Overview: mpnlri.go — MP_REACH_NLRI encode, decode and next-hop length rules

package attribute

import (
	"errors"
	"fmt"
	"net/netip"
)

// nextHopLenIPv6 is the Length of Next Hop Network Address octet of a field
// holding ONE IPv6 address (RFC 4760 Section 3).
const nextHopLenIPv6 = 16

// ErrLinkLocalNextHop reports an IPv6 link-local address offered as the global
// next hop of an MP_REACH_NLRI attribute.
//
// RFC 2545 Section 2: a link-local address is "not ... well suited to be used as
// next hop attributes in BGP-4", which is why Section 3 carries it as a second
// address after the global one rather than in place of it.
var ErrLinkLocalNextHop = errors.New("link-local address cannot be the global next hop")

// ValidateGlobalNextHop reports whether addr can occupy the Network Address of
// Next Hop field of an MP_REACH_NLRI attribute.
//
// RFC 2545 Section 3: "A BGP speaker shall advertise to its peer in the Network
// Address of Next Hop field the global IPv6 address of the next hop, potentially
// followed by the link-local IPv6 address of the next hop." Section 2 divides
// IPv6 unicast into "link-local" and "global"/"non-link-local", and folds
// site-local into the second, so every address except a link-local one is
// permitted in that field.
//
// The zero Addr returns nil: an unset next hop is the caller's own parse
// failure, and reporting it here would name the wrong defect.
func ValidateGlobalNextHop(addr netip.Addr) error {
	if !addr.IsValid() {
		return nil
	}
	if addr.Unmap().Is4() {
		return nil
	}
	if addr.IsLinkLocalUnicast() {
		return fmt.Errorf("next-hop %s: %w (RFC 2545 Section 3)", addr, ErrLinkLocalNextHop)
	}
	return nil
}

// IsLinkLocalOnlyNextHop reports whether a Network Address of Next Hop field
// carries a Link-Local-only Next Hop.
//
// draft-ietf-idr-linklocal-capability Section 3: "A BGP speaker receiving an
// MP_REACH_NLRI with the Next Hop field length set to 16 classifies the address
// as follows. If the address is in fe80::/10, the Next Hop is a Link-Local-only
// Next Hop as defined in this document. Otherwise, the address is treated as a
// Global IPv6 Next Hop per [RFC2545]."
//
// Both halves of that sentence are read: the LENGTH decides which slot the
// address occupies, and the address decides what it is. A 32-octet field is the
// RFC 2545 Section 3 pair and is never this form, whatever its second address,
// because its first address is the global one.
//
// It is the one classification, and both sides use it: the send side asks
// whether the form it is about to write is this one, and the reflection gate
// asks the same of the field it is about to relay
// (nextHopValue.linkLocalOnly, internal/component/bgp/reactor/forward_next_hop.go).
func IsLinkLocalOnlyNextHop(field []byte) bool {
	if len(field) != nextHopLenIPv6 {
		return false
	}
	return netip.AddrFrom16([16]byte(field)).IsLinkLocalUnicast()
}

// LinkLocalOnlyNextHopPermitted reports whether this session may send a
// 16-octet Link-Local-only Next Hop field for an NLRI of the named family.
//
// Two sentences decide it, and each owns one of the arguments.
//
// draft-ietf-idr-linklocal-capability Section 2: "In this document, all
// procedures described are applicable only when the capability described herein
// has been successfully advertised by both BGP speakers; i.e., negotiated. When
// the capability has not been negotiated, the procedures in this document do not
// apply." The Section 3 form is one of those procedures, so llnhNegotiated is
// a precondition for every family.
//
// draft-ietf-idr-linklocal-capability Section 5: "When both the Link-Local Next
// Hop Capability defined in this document and the Extended Next Hop Encoding
// capability ([RFC8950]) have been negotiated between two peers, a 16-octet
// Link-Local-only Next Hop is permitted for IPv4 NLRI carried with an IPv6 Next
// Hop, provided the receiving peer is directly attached. When this combination
// has not been negotiated, a sender MUST follow the rules in Section 3 of
// [RFC8950] and encode the Next Hop as 32 octets." IPv4 NLRI therefore needs
// BOTH, and extendedNextHopNegotiated is what says the second one was reached.
//
// It answers false where it cannot answer: an unread negotiation is not a
// permission, and the form it gates is one RFC 2545 Section 3 forbids outright.
func LinkLocalOnlyNextHopPermitted(ipv4NLRI, llnhNegotiated, extendedNextHopNegotiated bool) bool {
	if !llnhNegotiated {
		return false
	}
	if !ipv4NLRI {
		return true
	}
	return extendedNextHopNegotiated
}
