// Design: docs/architecture/wire/nlri-flowspec.md -- transport component semantics
package flowspecfirewall

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/flowspec"
	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/family"
)

func transportProtocols(fs *flowspec.FlowSpec, current []firewall.MatchProtocol) ([]firewall.MatchProtocol, error) {
	ports, tcp, icmp := false, false, false
	for _, c := range fs.Components() {
		switch c.Type() {
		case flowspec.FlowPort, flowspec.FlowDestPort, flowspec.FlowSourcePort:
			ports = true
		case flowspec.FlowTCPFlags:
			tcp = true
		case flowspec.FlowICMPType, flowspec.FlowICMPCode:
			icmp = true
		}
	}
	if !ports && !tcp && !icmp {
		return current, nil
	}
	icmpName := "icmp"
	if fs.Family().AFI == family.AFIIPv6 {
		icmpName = "icmpv6"
	}
	if len(current) == 0 {
		current = []firewall.MatchProtocol{{Protocol: "tcp"}, {Protocol: "udp"}, {Protocol: icmpName}}
	}
	out := current[:0]
	// RFC 8955 Sections 4.2.2.4-9: transport components never match a packet
	// of a different protocol. The constraints are intersected, even when
	// the NLRI omits the explicit IP-protocol component.
	for _, p := range current {
		if ports && p.Protocol != "tcp" && p.Protocol != "udp" {
			continue
		}
		if tcp && p.Protocol != "tcp" {
			continue
		}
		if icmp && p.Protocol != icmpName {
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: conflicting transport components", errUnsupportedComponent)
	}
	return out, nil
}

// tcpFlagsMatch preserves the bitmask operator rather than interpreting it as
// numeric equality. RFC 8955 Section 4.2.1.2 -- see rfc/short/rfc8955.md.
func tcpFlagsMatch(comp flowspec.FlowComponent) (firewall.MatchTCPFlags, error) {
	list, ok := comp.(interface{ Matches() []flowspec.FlowMatch })
	if !ok {
		return firewall.MatchTCPFlags{}, errUnreadableValue
	}
	matches := list.Matches()
	if len(matches) == 0 {
		return firewall.MatchTCPFlags{}, errUnreadableValue
	}
	if len(matches) != 1 {
		return firewall.MatchTCPFlags{}, fmt.Errorf("%w: tcp-flags requires one bitmask", errUnsupportedComponent)
	}
	match := matches[0]
	if match.Value > 255 {
		return firewall.MatchTCPFlags{}, fmt.Errorf("%w: tcp-flags exceeds the firewall flag field", errUnsupportedComponent)
	}
	flags := firewall.TCPFlags(match.Value)
	switch match.Op {
	case flowspec.FlowOpMatch:
		return firewall.MatchTCPFlags{Flags: flags, Mask: flags}, nil
	case flowspec.FlowOpNot:
		return firewall.MatchTCPFlags{Mask: flags}, nil
	case 0, flowspec.FlowOpNot | flowspec.FlowOpMatch:
		// Any-bit-set and not-all-bits-set reduce to one masked equality
		// only for a single selected bit.
		if flags == 0 {
			break
		}
		if flags&(flags-1) != 0 {
			break
		}
		if match.Op == 0 {
			return firewall.MatchTCPFlags{Flags: flags, Mask: flags}, nil
		}
		return firewall.MatchTCPFlags{Mask: flags}, nil
	}
	return firewall.MatchTCPFlags{}, fmt.Errorf("%w: tcp-flags bitmask cannot be represented exactly", errUnsupportedComponent)
}
