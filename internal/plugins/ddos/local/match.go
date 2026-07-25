// Design: plan/learned/1011-cp-survival-5-detect-0-umbrella.md -- vector to firewall term

package local

import (
	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/ddosevent"
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
	if v.TCPFlags != 0 {
		// Match packets whose set flags include the vector's flags (SYN for a
		// SYN flood). Mask == Flags means "examine exactly these bits, require
		// them set" -- an exact match on the discriminating flags (AC-9).
		f := firewall.TCPFlags(v.TCPFlags)
		matches = append(matches, firewall.MatchTCPFlags{Flags: f, Mask: f})
	}
	return firewall.Term{
		Name:    name,
		Matches: matches,
		Actions: []firewall.Action{firewall.Counter{}, firewall.Drop{}},
	}
}
