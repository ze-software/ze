// Design: plan/learned/1005-cp-survival-2-copp-port179.md -- coppPolicy to firewall.Table translation

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

	limitActions := []firewall.Action{
		firewall.Limit{
			Rate:      policy.Rate,
			Unit:      policy.RateUnit,
			Dimension: policy.Dimension,
			Burst:     policy.Burst,
		},
		firewall.Accept{},
	}

	terms = append(terms, firewall.Term{
		Name: "rate-limit",
		Matches: []firewall.Match{
			firewall.MatchProtocol{Protocol: "tcp"},
			firewall.MatchDestinationPort{Ranges: portRanges(policy.ProtectedPorts)},
			firewall.MatchConnState{States: firewall.ConnStateNew},
		},
		Actions: limitActions,
	})

	chainPolicy := firewall.PolicyAccept
	if policy.OverPolicy == overPolicyDrop {
		chainPolicy = firewall.PolicyDrop
	}

	return firewall.Table{
		Name:   coppTableName,
		Family: firewall.FamilyInet,
		Chains: []firewall.Chain{{
			Name:     "input",
			IsBase:   true,
			Type:     firewall.ChainFilter,
			Hook:     firewall.HookInput,
			Priority: 0,
			Policy:   chainPolicy,
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
