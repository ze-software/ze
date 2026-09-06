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
// VALIDATES: peerListenPort answers from LocalPort, falls back to the daemon's
//
//	port, and returns DefaultBGPPort when neither is set. The remote port
//	never reaches the listener: the last case pins that.
//
// PREVENTS: Listener binding to port 0 (OS-assigned random port) instead of 179,
//
//	and a listener answering on the port Ze dials rather than the port the
//	operator asked to listen on.
func TestPeerListenPort(t *testing.T) {
	tests := []struct {
		name       string
		localPort  uint16
		remotePort uint16
		configPort int
		want       int
	}{
		{"custom local port", 1179, 0, 0, 1179},
		{"config port, no local port", 0, 0, 10179, 10179},
		{"config port, local port is the default", DefaultBGPPort, 0, 10179, 10179},
		{"no port set anywhere", 0, 0, 0, DefaultBGPPort},
		{"local port is the default, config zero", DefaultBGPPort, 0, 0, DefaultBGPPort},
		{"a remote port of its own moves no listener", 0, 1179, 0, DefaultBGPPort},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reactor{config: &Config{Port: tt.configPort}}
			s := &PeerSettings{LocalPort: tt.localPort, Port: tt.remotePort}
			assert.Equal(t, tt.want, r.peerListenPort(s))
		})
	}
}
