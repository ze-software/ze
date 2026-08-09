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
// is enforceRFC7606, reactor/session_read.go:162). Classifying shape is not
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
		switch code { //nolint:exhaustive // only the two MP attributes bear NLRI
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
