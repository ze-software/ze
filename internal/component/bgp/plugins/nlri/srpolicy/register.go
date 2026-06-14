// Design: docs/architecture/wire/nlri.md -- SR-Policy NLRI plugin registration
// RFC: rfc/short/rfc9830.md -- SAFI 73 (SR-Policy)
// Related: types.go -- SRPolicy NLRI type
// Related: split.go -- NLRI splitter

package srpolicy

import (
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri/nlrisplit"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// Family registrations for SR-Policy (SAFI 73).
var (
	IPv4SRPolicy = family.MustRegister(family.AFIIPv4, family.SAFISRPolicy, "ipv4", "sr-policy")
	IPv6SRPolicy = family.MustRegister(family.AFIIPv6, family.SAFISRPolicy, "ipv6", "sr-policy")
)

func init() {
	nlrisplit.Register(IPv4SRPolicy, SplitSRPolicy)
	nlrisplit.Register(IPv6SRPolicy, SplitSRPolicy)
}
