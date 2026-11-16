// Design: docs/architecture/config/syntax.md -- scoped local TCP endpoints
// Related: session.go -- NewSession and mergedRealDialer construct the actual source address
// Related: internal/core/network/network.go -- RealDialer passes LocalAddr to net.Dialer

package reactor

import (
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/network"
)

// TestNewSessionLocalTransportScope inspects the actual TCP source address,
// including its interface zone. The configured listener port is not a source
// port, and an unset local address leaves source selection to the dialer.
func TestNewSessionLocalTransportScope(t *testing.T) {
	for _, tc := range []struct {
		name  string
		local netip.Addr
		want  *net.TCPAddr
	}{
		{"scoped-link-local", netip.MustParseAddr("fe80::1%sp"), &net.TCPAddr{IP: net.ParseIP("fe80::1"), Zone: "sp"}},
		{"unscoped-link-local", netip.MustParseAddr("fe80::1"), &net.TCPAddr{IP: net.ParseIP("fe80::1")}},
		{"global-ipv6", netip.MustParseAddr("2001:db8::1"), &net.TCPAddr{IP: net.ParseIP("2001:db8::1")}},
		{"ipv4", netip.MustParseAddr("192.0.2.1"), &net.TCPAddr{IP: net.IP{192, 0, 2, 1}}},
		{"unset", netip.Addr{}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr("fe80::2%sp"), 65000, 65001, 0x01020304)
			settings.LocalAddress = tc.local
			settings.LocalPort = 1179
			session := NewSession(settings)
			dialer, ok := session.dialer.(*network.RealDialer)
			require.True(t, ok)
			assertSessionTCPSource(t, tc.want, dialer.LocalAddr)
		})
	}
}

// TestSessionSetDialerLocalTransportScope drives the production merge entry
// point. A configured source overrides the injected source. Without a configured
// source, the injected IP, zone and port remain unchanged, including nil.
func TestSessionSetDialerLocalTransportScope(t *testing.T) {
	for _, tc := range []struct {
		name     string
		local    netip.Addr
		injected *net.TCPAddr
		want     *net.TCPAddr
	}{
		{"scoped-default", netip.MustParseAddr("fe80::1%sp"), nil,
			&net.TCPAddr{IP: net.ParseIP("fe80::1"), Zone: "sp"}},
		{"scoped-overrides-explicit", netip.MustParseAddr("fe80::1%sp"),
			&net.TCPAddr{IP: net.ParseIP("fe80::99"), Zone: "other", Port: 40770},
			&net.TCPAddr{IP: net.ParseIP("fe80::1"), Zone: "sp"}},
		{"unscoped-overrides-explicit", netip.MustParseAddr("fe80::1"),
			&net.TCPAddr{IP: net.ParseIP("fe80::99"), Zone: "other", Port: 40770},
			&net.TCPAddr{IP: net.ParseIP("fe80::1")}},
		{"global-overrides-explicit", netip.MustParseAddr("2001:db8::1"),
			&net.TCPAddr{IP: net.ParseIP("fe80::99"), Zone: "other", Port: 40770},
			&net.TCPAddr{IP: net.ParseIP("2001:db8::1")}},
		{"ipv4", netip.MustParseAddr("192.0.2.1"), nil,
			&net.TCPAddr{IP: net.IP{192, 0, 2, 1}}},
		{"unset-keeps-explicit", netip.Addr{},
			&net.TCPAddr{IP: net.ParseIP("fe80::99"), Zone: "other", Port: 40770},
			&net.TCPAddr{IP: net.ParseIP("fe80::99"), Zone: "other", Port: 40770}},
		{"unset-keeps-default", netip.Addr{}, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr("fe80::2%sp"), 65000, 65001, 0x01020304)
			settings.LocalAddress = tc.local
			settings.LocalPort = 1179
			session := NewSession(settings)
			base := &network.RealDialer{LocalAddr: tc.injected}
			session.SetDialer(base)
			dialer, ok := session.dialer.(*network.RealDialer)
			require.True(t, ok)
			require.NotSame(t, base, dialer, "merging must not change the shared dialer")
			assertSessionTCPSource(t, tc.want, dialer.LocalAddr)
			assert.Same(t, tc.injected, base.LocalAddr, "the injected source must remain unchanged")
			if !tc.local.IsValid() {
				assert.Same(t, tc.injected, dialer.LocalAddr, "an unset peer source retains the injected source")
			}
		})
	}
}

func assertSessionTCPSource(t *testing.T, want, got *net.TCPAddr) {
	t.Helper()
	if want == nil {
		require.Nil(t, got)
		return
	}
	require.NotNil(t, got)
	assert.Equal(t, want.IP, got.IP)
	assert.Equal(t, want.Zone, got.Zone)
	assert.Equal(t, want.Port, got.Port)
}
