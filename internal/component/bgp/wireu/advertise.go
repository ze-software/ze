// Design: docs/architecture/wire/messages.md -- wire UPDATE lazy parsing
// RFC: rfc/short/rfc4271.md -- Section 4.3, a withdraw-only UPDATE carries no path attributes
// RFC: rfc/short/rfc4760.md -- Section 3, an advertisement can ride in MP_REACH_NLRI
// Related: aspath_slot.go -- ASPathEdit.Record, the caller this predicate gates

package wireu

import (
	"encoding/binary"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// PayloadAdvertisesNLRI reports whether an UPDATE body advertises reachable
// NLRI: a non-empty Network Layer Reachability Information field (RFC 4271
// Section 4.3, IPv4 unicast) or an MP_REACH_NLRI attribute (RFC 4760
// Section 3).
//
// It answers ONE question: is there a route for a per-destination rule to act
// on? RFC 4271 Section 4.3 states the shape it separates: "An UPDATE message
// might advertise only routes that are to be withdrawn from service, in which
// case the message will not include path attributes or Network Layer
// Reachability Information."
//
// DO NOT LOOSEN THIS. An egress rule that adds an attribute to an UPDATE
// advertising nothing produces a well-known attribute set that is incomplete by
// construction, and RFC 4271 Section 6.3 makes that a wire error: "If any of the
// well-known mandatory attributes are not present, then the Error Subcode MUST
// be set to Missing Well-known Attribute." Section 6.3's tolerance clause does
// not cover it either -- it covers "an UPDATE message that contains correct path
// attributes, but no NLRI", and a lone AS_PATH is not a correct set. RFC 7606
// Section 5.2 lets a receiver answer the same shape with a session reset.
//
// The predicate fires on positive evidence that a route IS carried, so a
// truncated or unreadable payload reports false and no rule acts on it
// (ai/rules/evidence.md).
//
// The role plugin's OTC gate asks the same question for RFC 9234 Section 5 and
// delegates here (internal/component/bgp/plugins/role/otc.go).
func PayloadAdvertisesNLRI(payload []byte) bool {
	if len(payload) < 4 {
		return false
	}
	withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrLenOff := 2 + withdrawnLen
	if len(payload) < attrLenOff+2 {
		return false
	}
	attrLen := int(binary.BigEndian.Uint16(payload[attrLenOff : attrLenOff+2]))
	attrsStart := attrLenOff + 2
	if len(payload) < attrsStart+attrLen {
		return false
	}
	// RFC 4271 Section 4.3: the NLRI field is whatever trails the attributes.
	if len(payload) > attrsStart+attrLen {
		return true
	}
	// No native NLRI, but an advertisement can still ride in MP_REACH_NLRI.
	iter := attribute.NewAttrIterator(payload[attrsStart : attrsStart+attrLen])
	for code, _, _, ok := iter.Next(); ok; code, _, _, ok = iter.Next() {
		if code == attribute.AttrMPReachNLRI {
			return true
		}
	}
	return false
}
