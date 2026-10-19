// Design: docs/architecture/wire/rfc7606-relay-shape.md -- one NLRI-bearing field per UPDATE
// RFC: rfc/short/rfc7606.md -- Section 5.1 second bullet (sender-side encoding restriction)
// Related: update_split.go -- Splitter.SplitCompliant, the enforcement point for parsed UPDATEs
// Related: ../wireu/split.go -- SplitWireUpdate, the enforcement point for wire UPDATEs

package message

import (
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// NLRIBearingFieldCount counts how many of the four fields named by RFC 7606
// Section 5.1's second bullet an UPDATE carries: "An UPDATE message MUST NOT
// contain more than one of the following: non-empty Withdrawn Routes field,
// non-empty Network Layer Reachability Information field, MP_REACH_NLRI
// attribute, and MP_UNREACH_NLRI attribute."
//
// The arguments are the UPDATE's three wire sections, so one definition serves
// both a parsed *Update and a lazily-parsed wireu.WireUpdate instead of each
// growing its own notion of "mixed".
//
// A count of 0 or 1 is compliant; higher means the sender must split first.
//
// Path attribute bytes that stop parsing end the walk, and only what parsed is
// counted. Callers have already validated the UPDATE (on the receive path that
// is applyRFC7606 in reactor/session_validation.go). Classifying shape is not
// validating, and a parse failure must not be turned into an invented violation.
func NLRIBearingFieldCount(withdrawn, pathAttrs, nlri []byte) int {
	n := 0
	if len(withdrawn) > 0 {
		n++
	}
	if len(nlri) > 0 {
		n++
	}

	iter := attribute.NewAttrIterator(pathAttrs)
	for code, _, _, ok := iter.Next(); ok; code, _, _, ok = iter.Next() {
		//exhaustive:ignore // Only MP_REACH and MP_UNREACH contribute NLRI-bearing attributes.
		switch code {
		case attribute.AttrMPReachNLRI, attribute.AttrMPUnreachNLRI:
			n++
		}
	}
	return n
}

// MixesNLRIFields reports whether this UPDATE violates RFC 7606 Section 5.1's
// second bullet by carrying more than one NLRI-bearing field.
func (u *Update) MixesNLRIFields() bool {
	return NLRIBearingFieldCount(u.WithdrawnRoutes, u.PathAttributes, u.NLRI) > 1
}

// MPAttributesTrail reports whether pathAttrs break the order Ze composes for RFC
// 7606 Section 5.1's first bullet: "The MP_REACH_NLRI or MP_UNREACH_NLRI
// attribute (if present) SHALL be encoded as the very first path attribute in an
// UPDATE message." An MP attribute trails when any other attribute precedes it,
// or when MP_REACH_NLRI precedes MP_UNREACH_NLRI (Ze writes the withdrawal
// first, as Withdrawn Routes precede NLRI in RFC 4271 Section 4.3). A block with
// no MP attribute does not trail.
//
// Bytes that stop parsing end the walk, for the reason NLRIBearingFieldCount
// gives: classifying order is not validating.
func MPAttributesTrail(pathAttrs []byte) bool {
	otherSeen := false
	reachSeen := false
	iter := attribute.NewAttrIterator(pathAttrs)
	for code, _, _, ok := iter.Next(); ok; code, _, _, ok = iter.Next() {
		//exhaustive:ignore // Only the two MP codes have a required position.
		switch code {
		case attribute.AttrMPUnreachNLRI:
			if otherSeen {
				return true
			}
			if reachSeen {
				return true
			}
		case attribute.AttrMPReachNLRI:
			if otherSeen {
				return true
			}
			reachSeen = true
		default:
			otherSeen = true
		}
	}
	return false
}

// WriteMPAttributesFirst writes pathAttrs into dst in the order Ze composes: every
// MP_UNREACH_NLRI, then every MP_REACH_NLRI, then every other attribute in its
// source order. Each attribute is copied byte for byte, header included, so the
// result differs from the source in attribute order only. Bytes after the last
// attribute that parses are copied unchanged at the end, so the length is kept.
//
// RFC 7606 Section 5.1: "The MP_REACH_NLRI or MP_UNREACH_NLRI attribute (if
// present) SHALL be encoded as the very first path attribute in an UPDATE
// message."
//
// dst MUST hold len(pathAttrs) bytes. Returns the bytes written, which is
// len(pathAttrs).
func WriteMPAttributesFirst(dst, pathAttrs []byte) int {
	n := copyAttrsOfKind(dst, pathAttrs, attrKindMPUnreach)
	n += copyAttrsOfKind(dst[n:], pathAttrs, attrKindMPReach)
	n += copyAttrsOfKind(dst[n:], pathAttrs, attrKindOther)
	return n
}

// attrKind selects which attributes one copyAttrsOfKind pass writes.
type attrKind uint8

const (
	attrKindUnspecified attrKind = iota
	attrKindMPUnreach
	attrKindMPReach
	attrKindOther
)

// kindOfAttr classifies one attribute code for WriteMPAttributesFirst.
func kindOfAttr(code attribute.AttributeCode) attrKind {
	//exhaustive:ignore // Only the two MP codes are moved; every other code keeps its place.
	switch code {
	case attribute.AttrMPUnreachNLRI:
		return attrKindMPUnreach
	case attribute.AttrMPReachNLRI:
		return attrKindMPReach
	default:
		return attrKindOther
	}
}

// copyAttrsOfKind copies every attribute of kind from pathAttrs into dst, whole
// and in source order, and returns the bytes written. The attrKindOther pass also
// copies the unparsed tail, so the three passes together write every byte once.
func copyAttrsOfKind(dst, pathAttrs []byte, kind attrKind) int {
	n := 0
	pos := 0
	iter := attribute.NewAttrIterator(pathAttrs)
	for code, flags, value, ok := iter.Next(); ok; code, flags, value, ok = iter.Next() {
		size := 3 + len(value)
		if flags.IsExtLength() {
			size = 4 + len(value)
		}
		if kindOfAttr(code) == kind {
			n += copy(dst[n:], pathAttrs[pos:pos+size])
		}
		pos += size
	}
	if kind == attrKindOther {
		n += copy(dst[n:], pathAttrs[pos:])
	}
	return n
}
