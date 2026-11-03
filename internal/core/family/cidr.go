// Design: docs/architecture/wire/nlri.md -- BGP address family types
// Related: family.go -- the AFI and SAFI constants this file classifies

package family

// cidrSAFIs is the set of SAFIs whose NLRI wire format is a plain CIDR prefix
// behind an optional label stack. Declared as a map so the exhaustive linter
// does not flag the bounded check in PlainCIDR.
var cidrSAFIs = map[SAFI]struct{}{
	SAFIUnicast:   {},
	SAFIMulticast: {},
	SAFIMPLSLabel: {},
}

// PlainCIDR reports whether the family's NLRI wire format is a plain CIDR
// prefix: IPv4/IPv6 unicast, multicast, and mpls-label (RFC 8277). It is the
// one declaration of that set, read by the engine and by the filters alike:
//
//   - the engine rewrites the MP block of such a family from a filter's answer,
//     and refuses a changed block of every other family (EVPN, Flowspec, VPN,
//     BGP-LS, MVPN, MUP, RTC, ...);
//   - the per-prefix overlay of a policy chain and the CIDR NLRI walk over the
//     wire serve such a family only.
func (f Family) PlainCIDR() bool {
	if f.AFI != AFIIPv4 && f.AFI != AFIIPv6 {
		return false
	}
	_, ok := cidrSAFIs[f.SAFI]
	return ok
}
