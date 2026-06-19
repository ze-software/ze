package firewall

import "testing"

// TestProtocolNumber pins the single protocol-name -> IANA-number table that
// all firewall backends share.
//
// VALIDATES: ProtocolNumber maps every supported L4 protocol name to its IANA
// number and reports ok=false for unknown names.
// PREVENTS: the nft, VPP-classify, and VPP-NAT backends keeping divergent
// private copies of this table -- the NAT copy previously handled only tcp/udp
// and silently programmed protocol 0 for every other protocol.
func TestProtocolNumber(t *testing.T) {
	want := map[string]uint8{
		"tcp": 6, "udp": 17, "icmp": 1, "icmpv6": 58,
		"sctp": 132, "gre": 47, "esp": 50, "ah": 51,
		"ospf": 89, "vrrp": 112,
	}
	for name, num := range want {
		got, ok := ProtocolNumber(name)
		if !ok || got != num {
			t.Errorf("ProtocolNumber(%q) = (%d, %v), want (%d, true)", name, got, ok, num)
		}
	}
	for _, name := range []string{"", "TCP", "bogus", "ip", "0"} {
		if got, ok := ProtocolNumber(name); ok {
			t.Errorf("ProtocolNumber(%q) = (%d, true), want ok=false", name, got)
		}
	}
}
