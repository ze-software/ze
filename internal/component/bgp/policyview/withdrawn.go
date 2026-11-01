// Design: docs/architecture/core-design.md -- the BGP route view
// Related: readers.go -- nlriSections, the announced half
//
// The withdrawn half of an UPDATE's NLRI, for a member that judges every
// prefix the UPDATE names: a match's prefix-list drops a denied withdrawal as
// it drops a denied announcement.

package policyview

import (
	"errors"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// Withdrawn answers the body Withdrawn Routes and the MP_UNREACH_NLRI
// attribute, nil when the UPDATE carries none. Both are borrowed for the run,
// as received: no member's set changes them. A section that does not decode
// wraps ErrMalformed.
//
// RFC 4271 Section 4.3: "This is a variable-length field that contains a list of IP
// address prefixes for the routes that are being withdrawn from
// service."
func (v *View) Withdrawn() ([]byte, wireu.MPUnreachWire, error) {
	if v.update == nil {
		return nil, nil, ErrNoUpdate
	}
	body, err := v.update.Withdrawn()
	if err != nil {
		return nil, nil, errors.Join(ErrMalformed, err)
	}
	mpUnreach, err := v.update.MPUnreach()
	if err != nil {
		return nil, nil, errors.Join(ErrMalformed, err)
	}
	return body, mpUnreach, nil
}
