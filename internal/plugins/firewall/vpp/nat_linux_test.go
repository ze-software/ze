//go:build linux

package firewallvpp

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
)

// TestBuildDNATMappingProtocol checks the DNAT static mapping carries the IANA
// protocol number resolved from the shared firewall.ProtocolNumber table.
//
// VALIDATES: buildDNATMapping sets the VPP NAT44 mapping Protocol field from the
// MatchProtocol name for every recognized protocol, and leaves it 0 when no
// protocol match is present or the name is unknown.
// PREVENTS: the regression where the former inline tcp/udp-only switch silently
// programmed protocol 0 for every other protocol (icmp, sctp, gre, ...).
func TestBuildDNATMappingProtocol(t *testing.T) {
	dnat := firewall.DNAT{Address: netip.MustParseAddr("10.0.0.1"), Port: 8443}

	tests := []struct {
		proto string
		want  uint8
	}{
		{"tcp", 6},
		{"udp", 17},
		{"sctp", 132}, // was 0 before the fix
		{"icmp", 1},   // was 0 before the fix
		{"gre", 47},   // was 0 before the fix
		{"bogus", 0},  // unknown stays 0
	}
	for _, tt := range tests {
		term := &firewall.Term{Matches: []firewall.Match{
			firewall.MatchProtocol{Protocol: tt.proto},
			firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 443, Hi: 443}}},
		}}
		if m := buildDNATMapping(dnat, term, "test/dnat"); m.Protocol != tt.want {
			t.Errorf("protocol %q: mapping.Protocol = %d, want %d", tt.proto, m.Protocol, tt.want)
		}
	}

	// No MatchProtocol at all leaves Protocol at 0 (unchanged behavior).
	term := &firewall.Term{Matches: []firewall.Match{
		firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 443, Hi: 443}}},
	}}
	if m := buildDNATMapping(dnat, term, "test/dnat"); m.Protocol != 0 {
		t.Errorf("no MatchProtocol: mapping.Protocol = %d, want 0", m.Protocol)
	}
}
