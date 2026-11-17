// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- complete directional state scope.
// Related: child.go -- expanded policies and publication of both Child SA states.
// RFC: rfc/short/rfc4301.md -- negotiated selectors in the SAD (Section 4.4.2).

package engine

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// childStateScope preserves every expanded directional policy record without
// combining prefixes or borrowing the Child SA's mutable selector storage.
// RFC 4301 Section 4.4.2: "For each of the selectors defined in Section 4.4.1.1,
// the entry for an inbound SA in the SAD MUST be initially populated with the value
// or values negotiated at the time the SA was created."
func childStateScope(policies []dataplane.SPParams, mode uint8) (*dataplane.SAStateScope, error) {
	if mode == modeTransport {
		if len(policies) != 1 {
			return nil, fmt.Errorf("%w: a transport-mode Child SA needs exactly one expanded selector, got %d",
				dataplane.ErrSAOptionRefused, len(policies))
		}
	}
	selectors := make([]dataplane.SASelector, len(policies))
	for i := range policies {
		p := &policies[i]
		selectors[i] = dataplane.SASelector{
			Src: p.Src, Dst: p.Dst, UpperProto: p.UpperProto,
			SrcPort: p.SrcPort, DstPort: p.DstPort, IfIndex: p.IfIndex,
		}
	}
	// RFC 7296 Section 1.3.1: "It is enabled only if NON_FIRST_FRAGMENTS_ALSO
	// notification is included in both the request proposing an SA and the response
	// accepting it." Ze does not negotiate that notification, so permission is false.
	// RFC 4301 Section 4.4.2.
	return dataplane.NewSAStateScope(selectors, false)
}
