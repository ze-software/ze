// Design: docs/architecture/core-design.md — address forms permitted in the MP_REACH next-hop field
// RFC: rfc/short/rfc2545.md
// Overview: mpnlri.go — MP_REACH_NLRI encode, decode and next-hop length rules

package attribute

import (
	"errors"
	"fmt"
	"net/netip"
)

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
