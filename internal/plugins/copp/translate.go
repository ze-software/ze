// Design: docs/architecture/traffic/cp-survival-2-copp-port179.md -- coppPolicy to firewall.Table translation

package copp

import (
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// coppTableName carries the "ze_" ownership prefix that the firewall backend
// uses to recognize ze-managed kernel tables (mirrors policyroute's "ze_pr"
// and the firewall engine's tableNamePrefix). Without it the kernel table is
// named "copp": the backend's reconcile never sees it as ze-owned, so it is
// neither found by `nft list table inet ze_copp` nor withdrawn on removal.
const coppTableName = "ze_copp"

// The names below are stable across reloads, so `nft list table inet ze_copp`
// reads the same from one apply to the next and the doctor check can name a
// term the kernel lost.
const (
	inputChainName      = "input"
	neighborChainName   = "bgp-neighbor"
	neighborSetIPv4     = "bgp-neighbor-ipv4"
	neighborSetIPv6     = "bgp-neighbor-ipv6"
	establishedTermName = "established"
	rateLimitTermName   = "rate-limit"
	neighborAcceptName  = "neighbor-accept"
	unknownSourceDrop   = "unknown-source-drop"
)

// matchTCP is the protocol match every term on the protected ports carries.
var matchTCP = firewall.MatchProtocol{Protocol: "tcp"}

// translatePolicy builds the ze_copp table: an ACL on TCP to the protected
// ports, with the optional limiter on the accepted neighbors.
//
// The input chain, in order:
//
//  1. established: an established or related connection is accepted, so no
//     later term can close a session that is already up.
//  2. trusted-<prefix>: each trusted-source is accepted, ahead of the limiter.
//  3. bgp-neighbor-ipv4, bgp-neighbor-ipv6: TCP to a protected port from a
//     source in the family's neighbor set goes to the bgp-neighbor chain.
//  4. unknown-source-drop: TCP to a protected port from any other source is
//     counted and dropped, in any connection state.
//
// The bgp-neighbor chain holds the limiter, when a rate is configured, then
// accepts. One limiter term serves both families, so `rate` is one budget for
// all neighbors, as the operator wrote it, and not one budget per family.
//
// The chain policy stays accept: the terms name TCP to the protected ports and
// nothing else, so SSH, ICMP and every service this table never names stay
// reachable.
func translatePolicy(policy coppPolicy) firewall.Table {
	ports := portRanges(policy.ProtectedPorts)
	v4, v6 := splitByFamily(policy.Neighbors)

	terms := make([]firewall.Term, 0, 4+len(policy.TrustedSources))
	terms = append(terms, firewall.Term{
		Name: establishedTermName,
		Matches: []firewall.Match{
			firewall.MatchConnState{States: firewall.ConnStateEstablished | firewall.ConnStateRelated},
		},
		Actions: []firewall.Action{firewall.Accept{}},
	})

	var tb textbuf.Buffer
	for _, prefix := range policy.TrustedSources {
		terms = append(terms, firewall.Term{
			Name: trustedTermName(&tb, prefix),
			Matches: []firewall.Match{
				matchTCP,
				firewall.MatchDestinationPort{Ranges: ports},
				firewall.MatchSourceAddress{Prefix: prefix},
			},
			Actions: []firewall.Action{firewall.Accept{}},
		})
	}

	var sets []firewall.Set
	if len(v4) > 0 {
		sets = append(sets, neighborSet(neighborSetIPv4, firewall.SetTypeIPv4, v4))
		terms = append(terms, neighborTerm(neighborSetIPv4, ports))
	}
	if len(v6) > 0 {
		sets = append(sets, neighborSet(neighborSetIPv6, firewall.SetTypeIPv6, v6))
		terms = append(terms, neighborTerm(neighborSetIPv6, ports))
	}

	// RFC 7454 Section 4: "This protection SHOULD be achieved by an Access
	// Control List (ACL) that would discard all packets directed to TCP port
	// 179 on the local device and sourced from an address not known or
	// permitted to become a BGP neighbor."
	terms = append(terms, firewall.Term{
		Name: unknownSourceDrop,
		Matches: []firewall.Match{
			matchTCP,
			firewall.MatchDestinationPort{Ranges: ports},
		},
		Actions: []firewall.Action{firewall.Counter{}, firewall.Drop{}},
	})

	chains := []firewall.Chain{{
		Name:     inputChainName,
		IsBase:   true,
		Type:     firewall.ChainFilter,
		Hook:     firewall.HookInput,
		Priority: 0,
		Policy:   firewall.PolicyAccept,
		Terms:    terms,
	}}
	if len(sets) > 0 {
		chains = append(chains, firewall.Chain{
			Name:  neighborChainName,
			Terms: neighborChainTerms(policy.RateLimit),
		})
	}

	return firewall.Table{
		Name:   coppTableName,
		Family: firewall.FamilyInet,
		Chains: chains,
		Sets:   sets,
	}
}

// trustedTermName names the accept term of one trusted-source prefix. A term
// name allows letters, digits, hyphens, underscores and dots
// (firewall.ValidateName), so the prefix's "/" and an IPv6 ":" are written as
// "-": 192.0.2.0/24 is trusted-192.0.2.0-24. A prefix has one canonical text
// with its length last, so two prefixes never share a name.
func trustedTermName(tb *textbuf.Buffer, prefix netip.Prefix) string {
	text := tb.Reset().Prefix(prefix).String()
	tb.Reset().Str("trusted-")
	for i := range len(text) {
		c := text[i]
		if c == '/' || c == ':' {
			c = '-'
		}
		tb.Byte(c)
	}
	return tb.String()
}

// neighborTerm sends TCP to a protected port from a source in setName to the
// bgp-neighbor chain.
func neighborTerm(setName string, ports []firewall.PortRange) firewall.Term {
	return firewall.Term{
		Name: setName,
		Matches: []firewall.Match{
			matchTCP,
			firewall.MatchDestinationPort{Ranges: ports},
			firewall.MatchInSet{SetName: setName, MatchField: firewall.SetFieldSourceAddr},
		},
		Actions: []firewall.Action{firewall.Jump{Target: neighborChainName}},
	}
}

// neighborChainTerms is the bgp-neighbor chain: the limiter when a rate is
// configured, then the accept. Only a source the input chain found in a
// neighbor set reaches it, so an unknown source never spends the budget.
//
// RFC 7454 Section 4: "In addition to strict filtering, rate-limiting MAY be
// configured for accepted BGP traffic."
func neighborChainTerms(rateLimit *coppRateLimit) []firewall.Term {
	accept := firewall.Term{
		Name:    neighborAcceptName,
		Actions: []firewall.Action{firewall.Accept{}},
	}
	if rateLimit == nil {
		return []firewall.Term{accept}
	}

	// over-limit-policy governs the packets that EXCEED the rate, and nothing
	// else. nftables spells the two cases with one limiter and its inversion
	// flag. "limit rate N accept" matches while the token bucket has credit,
	// so an over-limit packet falls through to the accept term after it.
	// "limit rate over N drop" matches only once the bucket is empty, so an
	// over-limit packet is dropped and everything under the rate falls through
	// to that same accept.
	limit := firewall.Limit{
		Rate:      rateLimit.Rate,
		Unit:      rateLimit.RateUnit,
		Dimension: rateLimit.Dimension,
		Burst:     rateLimit.Burst,
	}
	overAction := firewall.Action(firewall.Accept{})
	if rateLimit.OverPolicy == overPolicyDrop {
		limit.Over = true
		overAction = firewall.Drop{}
	}
	return []firewall.Term{
		{
			Name: rateLimitTermName,
			Matches: []firewall.Match{
				firewall.MatchConnState{States: firewall.ConnStateNew},
			},
			Actions: []firewall.Action{limit, overAction},
		},
		accept,
	}
}

// neighborSet builds one family's interval set of neighbor prefixes.
func neighborSet(name string, setType firewall.SetType, prefixes []netip.Prefix) firewall.Set {
	return firewall.Set{
		Name:     name,
		Type:     setType,
		Flags:    firewall.SetFlagInterval,
		Elements: intervalElements(outermostPrefixes(prefixes)),
	}
}

// outermostPrefixes drops every prefix another prefix in the list contains: a
// static peer inside a dynamic group's range is an ordinary configuration, and
// the kernel refuses an interval set whose intervals overlap.
//
// Two aligned prefixes are either disjoint or one holds the other, so after a
// sort by first address, shorter prefix first, a contained prefix comes after
// its container and before anything outside it. One pass keeping the last
// prefix kept then sees every containment.
func outermostPrefixes(prefixes []netip.Prefix) []netip.Prefix {
	sorted := make([]netip.Prefix, len(prefixes))
	for i, prefix := range prefixes {
		sorted[i] = prefix.Masked()
	}
	slices.SortFunc(sorted, func(a, b netip.Prefix) int {
		if order := a.Addr().Compare(b.Addr()); order != 0 {
			return order
		}
		return a.Bits() - b.Bits()
	})

	kept := sorted[:0]
	for _, prefix := range sorted {
		if len(kept) > 0 && kept[len(kept)-1].Contains(prefix.Addr()) {
			continue
		}
		kept = append(kept, prefix)
	}
	return kept
}

// intervalElements encodes each prefix as an interval: its first address, then
// the address after its last one, marked as the exclusive end. A prefix that
// reaches the top of its address space has no address after it, so it gets no
// end element: in an interval set a start with no end after it runs to the
// top of the space, which is what the prefix covers.
//
// The caller passes prefixes that do not overlap (config.BGPNeighborSources
// sorts and dedups them); overlapping intervals are refused by the kernel, and
// the apply then reports the error.
func intervalElements(prefixes []netip.Prefix) []firewall.SetElement {
	elements := make([]firewall.SetElement, 0, 2*len(prefixes))
	for _, prefix := range prefixes {
		first := prefix.Masked().Addr()
		elements = append(elements, firewall.SetElement{Value: first.String()})
		after, ok := addressAfter(prefix)
		if !ok {
			continue
		}
		elements = append(elements, firewall.SetElement{Value: after.String(), IntervalEnd: true})
	}
	return elements
}

// addressAfter answers the address that follows the last address of prefix,
// and false when the prefix reaches the top of its address space.
func addressAfter(prefix netip.Prefix) (netip.Addr, bool) {
	last := lastAddress(prefix)
	next := last.Next()
	if !next.IsValid() {
		return netip.Addr{}, false
	}
	return next, true
}

// lastAddress sets every host bit of prefix.
func lastAddress(prefix netip.Prefix) netip.Addr {
	bytes := prefix.Masked().Addr().AsSlice()
	for bit := prefix.Bits(); bit < len(bytes)*8; bit++ {
		bytes[bit/8] |= 0x80 >> (bit % 8)
	}
	addr, _ := netip.AddrFromSlice(bytes) //nolint:errcheck // AsSlice of a valid address is 4 or 16 bytes, which AddrFromSlice always accepts
	return addr
}

// splitByFamily separates IPv4 from IPv6 prefixes, each set holding one
// family's addresses.
func splitByFamily(prefixes []netip.Prefix) (v4, v6 []netip.Prefix) {
	for _, prefix := range prefixes {
		if prefix.Addr().Is4() {
			v4 = append(v4, prefix)
			continue
		}
		v6 = append(v6, prefix)
	}
	return v4, v6
}

func portRanges(ports []uint16) []firewall.PortRange {
	ranges := make([]firewall.PortRange, len(ports))
	for i, p := range ports {
		ranges[i] = firewall.PortRange{Lo: p, Hi: p}
	}
	return ranges
}
