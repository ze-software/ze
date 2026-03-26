package reactor

import (
	"net/netip"
	"testing"
)

// TestParsePeerAddrToKey verifies address-to-map-key conversion for peer lookup.
//
// VALIDATES: parsePeerAddrToKey handles bare IPs, IPs with ports, IPv6, and invalid input.
// PREVENTS: Peer lookup failures from malformed address strings.
func TestParsePeerAddrToKey(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  netip.AddrPort
	}{
		{"bare IPv4", "10.0.0.1", netip.MustParseAddrPort("10.0.0.1:179")},
		{"IPv4 with default port", "10.0.0.1:179", netip.MustParseAddrPort("10.0.0.1:179")},
		{"IPv4 with custom port", "10.0.0.1:1790", netip.MustParseAddrPort("10.0.0.1:1790")},
		{"bare IPv6", "2001:db8::1", netip.MustParseAddrPort("[2001:db8::1]:179")},
		{"IPv6 with port", "[2001:db8::1]:8179", netip.MustParseAddrPort("[2001:db8::1]:8179")},
		{"empty string", "", netip.AddrPort{}},
		{"invalid", "not-an-ip", netip.AddrPort{}},
		{"loopback", "127.0.0.1", netip.MustParseAddrPort("127.0.0.1:179")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePeerAddrToKey(tt.input)
			if got != tt.want {
				t.Errorf("parsePeerAddrToKey(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
