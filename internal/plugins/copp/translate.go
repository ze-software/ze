// Design: docs/architecture/traffic/cp-survival-2-copp-port179.md -- coppPolicy to firewall.Table translation

package copp

import (
	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// coppTableName carries the "ze_" ownership prefix that the firewall backend
// uses to recognize ze-managed kernel tables (mirrors policyroute's "ze_pr"
// and the firewall engine's tableNamePrefix). Without it the kernel table is
// named "copp": the backend's reconcile never sees it as ze-owned, so it is
// neither found by `nft list table inet ze_copp` nor withdrawn on removal.
const coppTableName = "ze_copp"

func translatePolicy(policy coppPolicy) firewall.Table {
	terms := make([]firewall.Term, 0, 2+len(policy.TrustedSources))
	var tb textbuf.Buffer

	terms = append(terms, firewall.Term{
		Name: "established",
		Matches: []firewall.Match{
			firewall.MatchConnState{States: firewall.ConnStateEstablished | firewall.ConnStateRelated},
		},
		Actions: []firewall.Action{firewall.Accept{}},
	})

	for _, prefix := range policy.TrustedSources {
		terms = append(terms, firewall.Term{
			Name: tb.Reset().Str("trusted-").Prefix(prefix).String(),
			Matches: []firewall.Match{
				firewall.MatchProtocol{Protocol: "tcp"},
				firewall.MatchDestinationPort{Ranges: portRanges(policy.ProtectedPorts)},
				firewall.MatchSourceAddress{Prefix: prefix},
			},
			Actions: []firewall.Action{firewall.Accept{}},
		})
	}

	// over-limit-policy governs the packets that EXCEED the rate, and nothing
	// else. It is expressed on the rate-limit term rather than on the chain
	// policy: a base input chain with policy drop discards every packet that
	// reaches its end, so SSH, ICMP and every service this table never mentions
	// would go with it, and the operator would lose the box.
	//
	// nftables spells the two cases with one limiter and its inversion flag.
	// "limit rate N accept" matches while the token bucket has credit, so an
	// over-limit packet falls through to the accept policy. "limit rate over N
	// drop" matches only once the bucket is empty, so an over-limit packet is
	// dropped and everything under the rate falls through to that same policy.
	limit := firewall.Limit{
		Rate:      policy.Rate,
		Unit:      policy.RateUnit,
		Dimension: policy.Dimension,
		Burst:     policy.Burst,
	}
	overAction := firewall.Action(firewall.Accept{})
	if policy.OverPolicy == overPolicyDrop {
		limit.Over = true
		overAction = firewall.Drop{}
	}

	terms = append(terms, firewall.Term{
		Name: "rate-limit",
		Matches: []firewall.Match{
			firewall.MatchProtocol{Protocol: "tcp"},
			firewall.MatchDestinationPort{Ranges: portRanges(policy.ProtectedPorts)},
			firewall.MatchConnState{States: firewall.ConnStateNew},
		},
		Actions: []firewall.Action{limit, overAction},
	})

	return firewall.Table{
		Name:   coppTableName,
		Family: firewall.FamilyInet,
		Chains: []firewall.Chain{{
			Name:     "input",
			IsBase:   true,
			Type:     firewall.ChainFilter,
			Hook:     firewall.HookInput,
			Priority: 0,
			Policy:   firewall.PolicyAccept,
			Terms:    terms,
		}},
	}
}

func portRanges(ports []uint16) []firewall.PortRange {
	ranges := make([]firewall.PortRange, len(ports))
	for i, p := range ports {
		ranges[i] = firewall.PortRange{Lo: p, Hi: p}
	}
	return ranges
}
