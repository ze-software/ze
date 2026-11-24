// Design: docs/architecture/config/syntax.md -- scoped TCP endpoint admission
// Related: reactor_peers.go -- AddPeer validates settings after config parsing
// Related: reactor_connection.go -- shared listeners attribute inbound sockets by scoped peer address

package reactor

import (
	"fmt"
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/opererr"
)

// TestReactorAdmitsConfiguredScopedTransport follows real config text through
// PeersFromTree and AddPeer. The admitted peer and passive listener retain scope
// independently of the configured route next-hop mode.
func TestReactorAdmitsConfiguredScopedTransport(t *testing.T) {
	for _, passive := range []bool{false, true} {
		for _, mode := range []string{"unchanged", "self", "auto"} {
			t.Run(fmt.Sprintf("passive=%v/%s", passive, mode), func(t *testing.T) {
				settings, err := reactorTransportPeer(t, "fe80::1%if0", "fe80::2%if0", mode, passive)
				require.NoError(t, err)
				r := New(&Config{LocalAS: 65000})
				require.NoError(t, r.AddPeer(settings))
				r.mu.RLock()
				peer := r.peers[netip.MustParseAddrPort("[fe80::2%if0]:2179")]
				specs := r.listenerSpecs()
				r.mu.RUnlock()
				require.NotNil(t, peer, "the scoped peer must be installed under its exact key")
				assert.Equal(t, netip.MustParseAddr("fe80::1%if0"), peer.Settings().LocalAddress)
				assert.Equal(t, netip.MustParseAddr("fe80::2%if0"), peer.Settings().Address)
				if passive {
					require.Len(t, specs, 1)
					assert.Equal(t, netip.MustParseAddr("fe80::1%if0"), specs[0].addr)
					assert.Equal(t, 1179, specs[0].port)
					return
				}
				assert.Empty(t, specs, "an active-only peer opens no listener")
			})
		}
	}
}

// TestReactorTransportAdmissionKeepsAddressChecks preserves TCP tuple checks
// while scoped link-local transport becomes usable. No real socket is opened.
func TestReactorTransportAdmissionKeepsAddressChecks(t *testing.T) {
	for _, tc := range []struct {
		name      string
		local     string
		remote    string
		wantError string
	}{
		{"self-scoped", "fe80::1%if0", "fe80::1%if0", "cannot equal local-address"},
		{"self-mapped", "::ffff:192.0.2.1", "192.0.2.1", "cannot equal local-address"},
		{"ipv4-peer-ipv6-source", "2001:db8::1", "192.0.2.2", "family mismatch"},
		{"ipv6-peer-ipv4-source", "192.0.2.1", "2001:db8::2", "family mismatch"},
		{"global-ipv6", "2001:db8::1", "2001:db8::2", ""},
		{"ipv4", "192.0.2.1", "192.0.2.2", ""},
		{"mapped-ipv4", "::ffff:192.0.2.1", "::ffff:192.0.2.2", ""},
		{"loopback-self", "::1", "::1", ""},
		{"auto-source", "auto", "fe80::2%if0", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings, err := reactorTransportPeer(t, tc.local, tc.remote, "unchanged", false)
			require.NoError(t, err)
			r := New(&Config{LocalAS: 65000})
			err = r.AddPeer(settings)
			if tc.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantError)
				assert.Empty(t, r.peers, "invalid transport must not install a peer")
				return
			}
			require.NoError(t, err)
			require.Len(t, r.peers, 1)
			assert.Equal(t, settings.Address.Unmap(), settings.Address)
			assert.Equal(t, settings.LocalAddress.Unmap(), settings.LocalAddress)
		})
	}
}

// TestReactorUnscopedLocalTransportNamesTheFix keeps an unusable unscoped local
// bind refused, with an operator error that identifies the missing interface.
func TestReactorUnscopedLocalTransportNamesTheFix(t *testing.T) {
	settings, err := reactorTransportPeer(t, "fe80::1", "fe80::2%if0", "unchanged", false)
	require.NoError(t, err)
	r := New(&Config{LocalAS: 65000})
	err = r.AddPeer(settings)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "link-local")
	var refusal *opererr.Error
	require.ErrorAs(t, err, &refusal)
	assert.Contains(t, refusal.Problem(), "fe80::1")
	assert.Contains(t, refusal.Why(), "interface")
	assert.Equal(t, "set bgp peer scoped connection local ip fe80::1%if0", refusal.Fix())
	assert.Empty(t, r.peers)
}

// TestReactorTransportConfigKeepsExplicitGlobalNextHopGuard makes a valid
// scoped TCP source independent from an invalid explicit global next hop.
func TestReactorTransportConfigKeepsExplicitGlobalNextHopGuard(t *testing.T) {
	settings, err := reactorTransportPeer(t, "fe80::1%if0", "fe80::2%if0", "auto", false)
	require.NoError(t, err)
	require.Equal(t, netip.MustParseAddr("fe80::1%if0"), settings.LocalAddress)
	err = ValidatePeerGlobalNextHop("scoped", "next-hop", netip.MustParseAddr("fe80::9"))
	require.ErrorIs(t, err, attribute.ErrLinkLocalNextHop)
	assert.Contains(t, err.Error(), "next-hop")
}

// TestReactorScopedInboundLookupRetainsZone isolates the next admission boundary
// after peer installation. Both shared-listener paths must choose the exact
// scoped peer, and a socket from another interface must not select it.
func TestReactorScopedInboundLookupRetainsZone(t *testing.T) {
	for _, listenerContext := range []bool{false, true} {
		for _, zone := range []string{"if0", "other"} {
			t.Run(fmt.Sprintf("listener=%v/zone=%s", listenerContext, zone), func(t *testing.T) {
				settings, err := reactorTransportPeer(t, "fe80::1%if0", "fe80::2%if0", "unchanged", true)
				require.NoError(t, err)
				r := New(&Config{LocalAS: 65000})
				// Install directly to isolate inbound attribution from AddPeer's
				// separately tested local-address admission.
				r.peers[settings.PeerKey()] = NewPeer(settings)
				local, remote := net.Pipe()
				t.Cleanup(func() {
					assert.NoError(t, local.Close())
					assert.NoError(t, remote.Close())
				})
				conn := &reactorTransportConn{Conn: local, remote: &net.TCPAddr{
					IP: net.ParseIP("fe80::2"), Zone: zone, Port: 40770,
				}}
				accepted := 0
				r.connCallback = func(got net.Conn, peer *PeerSettings) {
					accepted++
					assert.Same(t, conn, got)
					assert.Same(t, settings, peer)
				}
				if listenerContext {
					r.handleConnectionWithContext(conn, netip.MustParseAddr("fe80::1%if0"))
				} else {
					r.handleConnection(conn)
				}
				if zone == "if0" {
					assert.Equal(t, 1, accepted, "the actual scoped peer must receive the socket")
					return
				}
				assert.Zero(t, accepted, "another interface is a different peer identity")
			})
		}
	}
}

type reactorTransportConn struct {
	net.Conn
	remote *net.TCPAddr
}

func (c *reactorTransportConn) RemoteAddr() net.Addr { return c.remote }

func reactorTransportPeer(t *testing.T, localIP, remoteIP, nextHop string, passive bool) (*PeerSettings, error) {
	t.Helper()
	tree, err := config.ParseTreeForValidation(fmt.Sprintf(`bgp {
    router-id 1.2.3.4;
    peer scoped {
        connection {
            local { ip %s; port 1179; accept %v; }
            remote { ip %s; port 2179; connect true; }
        }
        session {
            asn { local 65000; remote 65001; }
        }
        filter { export { filters [ next-hop:%s ]; final accept; } }
    }
}`, localIP, passive, remoteIP, nextHop))
	if err != nil {
		return nil, err
	}
	bgp := tree.GetContainer("bgp")
	require.NotNil(t, bgp)
	peers, err := PeersFromTree(bgp.ToMap())
	if err != nil {
		return nil, err
	}
	require.Len(t, peers, 1)
	return peers[0], nil
}
