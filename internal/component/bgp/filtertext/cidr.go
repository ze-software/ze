// Design: docs/architecture/api/process-protocol.md -- NLRI blocks in the filter text protocol
// Related: aspath.go -- another reading of the policy filter text format

package filtertext

import "github.com/ze-software/ze/internal/core/family"

// cidrSAFIs is the set of SAFIs whose NLRI wire format is a plain CIDR prefix
// behind an optional label stack. Declared as a map so the exhaustive linter
// does not flag the bounded check in CIDRFamily.
var cidrSAFIs = map[family.SAFI]struct{}{
	family.SAFIUnicast:   {},
	family.SAFIMulticast: {},
	family.SAFIMPLSLabel: {},
}

// CIDRFamily reports whether fam is an address family whose NLRI wire format is
// a plain CIDR prefix: IPv4/IPv6 unicast, multicast, and mpls-label (RFC 8277).
// It is the one declaration of the filter text protocol's CIDR families, read
// by the engine and by the filters alike:
//
//   - the engine renders the prefixes of such a family's MP block inline, and
//     every other family (EVPN, Flowspec, VPN, BGP-LS, MVPN, MUP, RTC, ...) as
//     a marker block or its own projection (reactor filter_format.go);
//   - the engine rewrites the MP block of such a family from a modify answer,
//     and refuses a changed block of every other family (reactor
//     filter_nlri_delta.go), so a filter asks this before it drops a prefix
//     from an MP block by modify.
func CIDRFamily(fam family.Family) bool {
	if fam.AFI != family.AFIIPv4 && fam.AFI != family.AFIIPv6 {
		return false
	}
	_, ok := cidrSAFIs[fam.SAFI]
	return ok
}
