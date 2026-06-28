// Design: plan/spec-cp-survival-5-detect-2-local-responder.md -- vector to firewall term

package local

import (
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
	"codeberg.org/thomas-mangin/ze/internal/core/ddosevent"
)

var protoName = map[uint8]string{
	6:  "tcp",
	17: "udp",
	1:  "icmp",
	58: "icmpv6",
}

func buildDropTerm(name string, v ddosevent.VectorTuple) firewall.Term {
	var matches []firewall.Match
	if v.DstPrefix.IsValid() {
		matches = append(matches, firewall.MatchDestinationAddress{Prefix: v.DstPrefix})
	}
	if p, ok := protoName[v.Proto]; ok {
		matches = append(matches, firewall.MatchProtocol{Protocol: p})
	}
	if v.DstPort != 0 {
		matches = append(matches, firewall.MatchDestinationPort{
			Ranges: []firewall.PortRange{{Lo: v.DstPort, Hi: v.DstPort}},
		})
	}
	if v.SrcPort != 0 {
		matches = append(matches, firewall.MatchSourcePort{
			Ranges: []firewall.PortRange{{Lo: v.SrcPort, Hi: v.SrcPort}},
		})
	}
	return firewall.Term{
		Name:    name,
		Matches: matches,
		Actions: []firewall.Action{firewall.Counter{}, firewall.Drop{}},
	}
}

func shouldMitigate(v ddosevent.VectorTuple, allowlist []netip.Prefix) bool {
	if !v.DstPrefix.IsValid() {
		return false
	}
	for _, allow := range allowlist {
		if allow.Overlaps(v.DstPrefix) {
			return false
		}
	}
	return true
}
