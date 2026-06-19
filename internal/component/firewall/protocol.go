// Design: docs/architecture/core-design.md -- Firewall data model types
// Related: model.go -- MatchProtocol carries the protocol name this resolves

package firewall

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
