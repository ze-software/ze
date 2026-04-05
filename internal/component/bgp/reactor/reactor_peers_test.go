package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
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

// TestPeerListenPort verifies the port fallback logic for peer listeners.
//
// VALIDATES: peerListenPort returns DefaultBGPPort when neither peer nor config set a port.
// PREVENTS: Listener binding to port 0 (OS-assigned random port) instead of 179.
func TestPeerListenPort(t *testing.T) {
	tests := []struct {
		name       string
		peerPort   uint16
		configPort int
		want       int
	}{
		{"custom peer port", 1179, 0, 1179},
		{"config port, no peer port", 0, 10179, 10179},
		{"config port, peer has default", DefaultBGPPort, 10179, 10179},
		{"no port set anywhere", 0, 0, DefaultBGPPort},
		{"peer has default, config zero", DefaultBGPPort, 0, DefaultBGPPort},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reactor{config: &Config{Port: tt.configPort}}
			s := &PeerSettings{Port: tt.peerPort}
			assert.Equal(t, tt.want, r.peerListenPort(s))
		})
	}
}
