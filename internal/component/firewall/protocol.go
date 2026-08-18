// Design: docs/architecture/core-design.md -- Firewall data model types
// Related: model.go -- MatchProtocol carries the protocol name this resolves

package firewall

import "sort"

// ianaProtocolNumbers maps each L4 protocol name carried by MatchProtocol to
// its IANA protocol number (the value a backend programs into its L4-protocol
// field). It is the single source of truth shared by every firewall backend --
// the nftables L4PROTO match, the VPP classify table, and the VPP NAT44 static
// mapping -- so the backends must not keep private copies that can drift apart.
var ianaProtocolNumbers = map[string]uint8{
	"tcp": 6, "udp": 17, "icmp": 1, "icmpv6": 58,
	"sctp": 132, "gre": 47, "esp": 50, "ah": 51,
	"ospf": 89, "vrrp": 112,
}

// ProtocolNumber returns the IANA protocol number for a MatchProtocol name and
// true, or (0, false) when the name is not a recognized protocol. Backends use
// the boolean to reject the rule (nft, VPP classify) or skip protocol matching
// (VPP NAT) rather than silently programming protocol 0.
func ProtocolNumber(name string) (uint8, bool) {
	num, ok := ianaProtocolNumbers[name]
	return num, ok
}

// protocolNames is the reverse of ianaProtocolNumbers, derived from it so the
// two directions can never drift. A producer that starts from a wire protocol
// number (a FlowSpec type 3 component, a DDoS vector tuple) resolves the name
// here rather than keeping its own table.
var protocolNames = buildProtocolNames()

func buildProtocolNames() map[uint8]string {
	names := make(map[uint8]string, len(ianaProtocolNumbers))
	for name, num := range ianaProtocolNumbers {
		names[num] = name
	}
	return names
}

// ProtocolName returns the MatchProtocol name for an IANA protocol number and
// true, or ("", false) when no name in the canonical table carries that number.
// A producer MUST use the boolean to refuse the rule: MatchProtocol carries a
// name, every backend resolves it through ProtocolNumber, and a number rendered
// as digits is a rule no backend can lower.
func ProtocolName(num uint8) (string, bool) {
	name, ok := protocolNames[num]
	return name, ok
}

// ProtocolNames returns every protocol name the canonical table carries, sorted,
// so a validator or an error message can state what it accepts without spelling
// the list a second time.
func ProtocolNames() []string {
	names := make([]string, 0, len(ianaProtocolNumbers))
	for name := range ianaProtocolNumbers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
