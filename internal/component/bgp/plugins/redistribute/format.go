// Design: docs/architecture/core-design.md -- canonical update-route command shape
// Related: redistribute.go -- handler that dispatches the formatted commands
//
// The canonical text-mode command shape lives in
// internal/component/bgp/format.go (FormatAnnounceCommand /
// FormatWithdrawCommand). bgp-redistribute mirrors that contract so the
// reactor's announce parser accepts what we emit:
//
//	announce: update text origin <o> nhop <addr|self> nlri <fam> add <prefix>
//	withdraw: update text nlri <fam> del <prefix>
//
// The literal `nhop self` token is what triggers reactor per-peer
// `LocalAddress` substitution -- see
// internal/component/bgp/plugins/cmd/update/update_wire.go:209-211 and
// internal/component/bgp/reactor/peer.go:562-591. A non-zero NextHop is
// passed verbatim as `nhop <addr>`.
//
// strings.Builder is used here per spec Decision (matches format.go's
// existing builder pattern for the same command shape; see Critical Review
// Checklist row "buffer-first"). `fmt.Sprintf` is forbidden on this path.

package redistribute

import (
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
)

// originIncomplete is the canonical ORIGIN value for locally-originated
// routes per RFC 4271 S5.1.1 -- the route did not enter via BGP (IGP) and
// did not enter via EGP, so the originator is "INCOMPLETE".
const originIncomplete = "incomplete"

// formatAnnounce builds the canonical text-mode announce command for one
// entry. Caller passes the family string (already stringified once per
// batch) and the entry by pointer to avoid copying.
//
// Examples:
//
//	formatAnnounce("ipv4/unicast", &entry{Action: ActionAdd, Prefix: 10.0.0.0/24, NextHop: zero})
//	  -> "update text origin incomplete nhop self nlri ipv4/unicast add 10.0.0.0/24"
//
//	formatAnnounce("ipv6/unicast", &entry{NextHop: 2001:db8::1, Prefix: 2001:db8::/64})
//	  -> "update text origin incomplete nhop 2001:db8::1 nlri ipv6/unicast add 2001:db8::/64"
func formatAnnounce(family string, entry *redistevents.RouteChangeEntry) string {
	var sb strings.Builder
	// Pre-grow to a typical line size to avoid intermediate allocation.
	sb.Grow(80)

	sb.WriteString("update text origin ")
	sb.WriteString(originIncomplete)

	sb.WriteString(" nhop ")
	if entry.NextHop.IsValid() {
		sb.WriteString(entry.NextHop.String())
	} else {
		sb.WriteString("self")
	}

	sb.WriteString(" nlri ")
	sb.WriteString(family)
	sb.WriteString(" add ")
	sb.WriteString(entry.Prefix.String())

	return sb.String()
}

// formatWithdraw builds the canonical text-mode withdraw command for one
// entry. Withdrawals carry no attributes.
//
// Example:
//
//	formatWithdraw("ipv4/unicast", &entry{Action: ActionRemove, Prefix: 10.0.0.0/24})
//	  -> "update text nlri ipv4/unicast del 10.0.0.0/24"
func formatWithdraw(family string, entry *redistevents.RouteChangeEntry) string {
	var sb strings.Builder
	sb.Grow(64)

	sb.WriteString("update text nlri ")
	sb.WriteString(family)
	sb.WriteString(" del ")
	sb.WriteString(entry.Prefix.String())

	return sb.String()
}
