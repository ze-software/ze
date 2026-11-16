// Design: docs/architecture/config/syntax.md -- scoped TCP endpoints and dynamic groups
// Related: reactor_connection.go -- inbound attribution must precede dynamic peer publication
// Related: reactor_dynamic.go -- longest prefix among groups eligible for the receiving socket

package reactor

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDynamicScopedInboundSelectsReceivingGroup drives both shared accept paths
// with equal ranges on different interfaces and checks the published callback
// settings, scoped identity and counters of the group that owns the local socket.
func TestDynamicScopedInboundSelectsReceivingGroup(t *testing.T) {
	for _, withContext := range []bool{false, true} {
		t.Run(fmt.Sprintf("listener=%v", withContext), func(t *testing.T) {
			r := newTestReactor(t)
			foreign := dynamicInboundGroup("foreign", "fe80::/64", "fe80::1%if0")
			wanted := dynamicInboundGroup("receiving", "fe80::/64", "fe80::1%if1")
			r.dynamicGroups = []*DynamicGroupConfig{foreign, wanted}
			conn := dynamicInboundConnection(t, "fe80::1%if1", "fe80::2%if1")
			dynamicInboundAccepted(t, r, conn, withContext, wanted)
			assert.Zero(t, foreign.ActivePeers.Load(), "the other interface must not gain a member")
		})
	}
}

// TestDynamicScopedInboundRefusesForeignContext requires a wrong receiving
// interface or local address to close the real pipe before any dynamic member
// or group counter is published, through either shared accept entry point.
func TestDynamicScopedInboundRefusesForeignContext(t *testing.T) {
	for _, withContext := range []bool{false, true} {
		for _, local := range []string{"fe80::1%if1", "fe80::9%if0"} {
			t.Run(fmt.Sprintf("listener=%v/local=%s", withContext, local), func(t *testing.T) {
				r := newTestReactor(t)
				group := dynamicInboundGroup("other-socket", "fe80::/64", "fe80::1%if0")
				r.dynamicGroups = []*DynamicGroupConfig{group}
				incoming := netip.MustParseAddr(local)
				conn := dynamicInboundConnection(t, local, "fe80::2%"+incoming.Zone())
				accepted := 0
				r.connCallback = func(net.Conn, *PeerSettings) { accepted++ }
				dynamicInboundHandle(r, conn, withContext)
				assert.Zero(t, accepted, "no group owns this receiving socket")
				assert.Empty(t, r.peers, "a refused socket must not leave an installed member")
				assert.Zero(t, group.ActivePeers.Load(), "refusal must precede group accounting")
				assert.True(t, conn.closed, "the underlying pipe must be closed on refusal")
			})
		}
	}
}

// TestDynamicScopedInboundLongestPrefixWithinContext checks that an ineligible
// /128 cannot shadow the receiving interface's /120, while its eligible /64
// sibling remains available and uncounted after the useful more-specific accept.
func TestDynamicScopedInboundLongestPrefixWithinContext(t *testing.T) {
	for _, withContext := range []bool{false, true} {
		t.Run(fmt.Sprintf("listener=%v", withContext), func(t *testing.T) {
			r := newTestReactor(t)
			foreign := dynamicInboundGroup("foreign-specific", "fe80::22/128", "fe80::1%if0")
			broad := dynamicInboundGroup("receiving-broad", "fe80::/64", "fe80::1%if1")
			wanted := dynamicInboundGroup("receiving-specific", "fe80::/120", "fe80::1%if1")
			r.dynamicGroups = []*DynamicGroupConfig{foreign, broad, wanted}
			conn := dynamicInboundConnection(t, "fe80::1%if1", "fe80::22%if1")
			dynamicInboundAccepted(t, r, conn, withContext, wanted)
			assert.Zero(t, foreign.ActivePeers.Load(), "foreign specificity gives no socket ownership")
			assert.Zero(t, broad.ActivePeers.Load(), "the eligible longest prefix must win")
		})
	}
}

// TestDynamicInboundContextPreservesGlobalAndAuto proves useful global and auto
// template survivors through both handlers, including an auto scoped member.
// A more-specific group for a different local address cannot suppress them.
func TestDynamicInboundContextPreservesGlobalAndAuto(t *testing.T) {
	cases := []struct {
		name         string
		local        string
		remote       string
		prefix       string
		foreignLocal string
		foreignRange string
		auto         bool
	}{
		{"ipv4-single", "192.0.2.1", "198.51.100.2", "198.51.100.0/24", "", "", false},
		{"ipv6-single", "2001:db8:1::1", "2001:db8:2::2", "2001:db8:2::/64", "", "", false},
		{"auto-scoped-single", "fe80::1%if1", "fe80::2%if1", "fe80::/64", "", "", true},
		{"ipv4", "192.0.2.1", "198.51.100.2", "198.51.100.0/24", "192.0.2.9", "198.51.100.2/32", false},
		{"ipv6", "2001:db8:1::1", "2001:db8:2::2", "2001:db8:2::/64", "2001:db8:1::9", "2001:db8:2::2/128", false},
		{"auto-ipv4", "192.0.2.1", "198.51.100.2", "198.51.100.0/24", "192.0.2.9", "198.51.100.2/32", true},
		{"auto-ipv6", "2001:db8:1::1", "2001:db8:2::2", "2001:db8:2::/64", "2001:db8:1::9", "2001:db8:2::2/128", true},
		{"auto-scoped", "fe80::1%if1", "fe80::2%if1", "fe80::/64", "fe80::1%if0", "fe80::2/128", true},
	}
	for _, withContext := range []bool{false, true} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("listener=%v/%s", withContext, tc.name), func(t *testing.T) {
				r := newTestReactor(t)
				wanted := dynamicInboundGroup("receiving", tc.prefix, tc.local)
				if tc.auto {
					wanted.Settings.LocalAddress = netip.Addr{}
				}
				r.dynamicGroups = []*DynamicGroupConfig{wanted}
				var foreign *DynamicGroupConfig
				if tc.foreignRange != "" {
					foreign = dynamicInboundGroup("foreign", tc.foreignRange, tc.foreignLocal)
					r.dynamicGroups = []*DynamicGroupConfig{foreign, wanted}
				}
				conn := dynamicInboundConnection(t, tc.local, tc.remote)
				dynamicInboundAccepted(t, r, conn, withContext, wanted)
				if foreign != nil {
					assert.Zero(t, foreign.ActivePeers.Load())
				}
			})
		}
	}
}

// TestDynamicInboundContextUsesReceivingPort attributes new dynamic members by
// the actual socket's local port, before publishing them. Static peers retain
// address-based shared attribution and sole-listener direct attribution; an
// ephemeral incoming source port is never the configured remote dial port.
func TestDynamicInboundContextUsesReceivingPort(t *testing.T) {
	for _, withContext := range []bool{false, true} {
		for _, configuredPort := range []uint16{0, DefaultBGPPort, 2179} {
			t.Run(fmt.Sprintf("listener=%v/select-port=%d", withContext, configuredPort), func(t *testing.T) {
				r := newTestReactor(t)
				r.config.Port = 2179
				foreign := dynamicInboundGroup("other-port", "fe80::/64", "fe80::1%if0")
				wanted := dynamicInboundGroup("receiving-port", "fe80::/64", "fe80::1%if0")
				wanted.Settings.LocalPort = configuredPort
				r.dynamicGroups = []*DynamicGroupConfig{foreign, wanted}
				conn := dynamicInboundConnection(t, "fe80::1%if0", "fe80::2%if0")
				conn.local.Port = 2179
				dynamicInboundAccepted(t, r, conn, withContext, wanted)
				assert.Zero(t, foreign.ActivePeers.Load(), "another port must not gain a member")
			})
		}
		t.Run(fmt.Sprintf("listener=%v/refuse-port", withContext), func(t *testing.T) {
			r := newTestReactor(t)
			group := dynamicInboundGroup("other-port", "fe80::/64", "fe80::1%if0")
			r.dynamicGroups = []*DynamicGroupConfig{group}
			conn := dynamicInboundConnection(t, "fe80::1%if0", "fe80::2%if0")
			conn.local.Port = 3179
			accepted := 0
			r.connCallback = func(net.Conn, *PeerSettings) { accepted++ }
			dynamicInboundHandle(r, conn, withContext)
			assert.Zero(t, accepted, "no dynamic group owns the receiving port")
			assert.Empty(t, r.peers, "a refused port must not leave an installed member")
			assert.Zero(t, group.ActivePeers.Load(), "port admission precedes group accounting")
			assert.True(t, conn.closed)
		})
		for _, socketPort := range []int{DefaultBGPPort, 1179} {
			t.Run(fmt.Sprintf("listener=%v/known-peer/socket=%d", withContext, socketPort), func(t *testing.T) {
				r := newTestReactor(t)
				settings := NewPeerSettings(netip.MustParseAddr("198.51.100.2"), 65000, 65001, 0x01020304)
				settings.LocalAddress = netip.MustParseAddr("192.0.2.1")
				settings.LocalPort = 1179
				settings.Port = 2179
				settings.Connection = ConnectionPassive
				r.peers[settings.PeerKey()] = NewPeer(settings)
				fallback := dynamicInboundGroup("fallback", "198.51.100.0/24", "192.0.2.1")
				r.dynamicGroups = []*DynamicGroupConfig{fallback}
				conn := dynamicInboundConnection(t, "192.0.2.1", "198.51.100.2")
				conn.local.Port = socketPort
				accepted := 0
				r.connCallback = func(got net.Conn, selected *PeerSettings) {
					accepted++
					assert.Same(t, conn, got)
					assert.Same(t, settings, selected)
					assert.Equal(t, netip.MustParseAddrPort("198.51.100.2:2179"), selected.PeerKey())
				}
				dynamicInboundHandle(r, conn, withContext)
				assert.Equal(t, 1, accepted, "the known peer precedes dynamic selection")
				assert.Len(t, r.peers, 1)
				assert.Zero(t, fallback.ActivePeers.Load())
				assert.False(t, conn.closed)
			})
		}
	}
	t.Run("sole-custom-listener", func(t *testing.T) {
		r := newTestReactor(t)
		settings := NewPeerSettings(netip.MustParseAddr("198.51.100.2"), 65000, 65001, 0x01020304)
		settings.LocalAddress = netip.MustParseAddr("192.0.2.1")
		settings.LocalPort = 1179
		settings.Port = 2179
		settings.Connection = ConnectionPassive
		r.peers[settings.PeerKey()] = NewPeer(settings)
		conn := dynamicInboundConnection(t, "192.0.2.1", "198.51.100.99")
		accepted := 0
		r.connCallback = func(got net.Conn, selected *PeerSettings) {
			accepted++
			assert.Same(t, conn, got)
			assert.Same(t, settings, selected)
		}
		r.handleListenerConnection(conn, settings.LocalAddress, 1179)
		assert.Equal(t, 1, accepted, "a sole custom listener identifies its configured peer")
		assert.Len(t, r.peers, 1)
		assert.False(t, conn.closed)
	})
}

// TestDynamicInboundExistingMemberKeepsReceivingContext seeds a useful member,
// then presents its same remote identity on a different local address or port.
// Reuse must refuse the socket while preserving the admitted member and counter.
func TestDynamicInboundExistingMemberKeepsReceivingContext(t *testing.T) {
	for _, withContext := range []bool{false, true} {
		for _, wrongPort := range []bool{false, true} {
			t.Run(fmt.Sprintf("listener=%v/wrong-port=%v", withContext, wrongPort), func(t *testing.T) {
				r := newTestReactor(t)
				group := dynamicInboundGroup("receiving", "fe80::/64", "fe80::1%if0")
				r.dynamicGroups = []*DynamicGroupConfig{group}
				seed := dynamicInboundConnection(t, "fe80::1%if0", "fe80::2%if0")
				dynamicInboundAccepted(t, r, seed, withContext, group)
				key := netip.MustParseAddrPort("[fe80::2%if0]:179")
				original := r.peers[key]
				local := "fe80::9%if0"
				if wrongPort {
					local = "fe80::1%if0"
				}
				conn := dynamicInboundConnection(t, local, "fe80::2%if0")
				if wrongPort {
					conn.local.Port = 3179
				}
				accepted := 0
				r.connCallback = func(net.Conn, *PeerSettings) { accepted++ }
				dynamicInboundHandle(r, conn, withContext)
				assert.Zero(t, accepted, "an existing member does not authorize another local socket")
				assert.True(t, conn.closed)
				require.Len(t, r.peers, 1)
				assert.Same(t, original, r.peers[key], "refusal preserves the admitted member")
				assert.Equal(t, int32(1), group.ActivePeers.Load())
			})
		}
	}
}

// TestDynamicInboundWildcardLocalContext keeps explicit wildcard TCP listeners
// useful for their concrete accepted endpoints, while their port still bounds
// admission. Go's tcp listener can accept both families on either wildcard form.
func TestDynamicInboundWildcardLocalContext(t *testing.T) {
	for _, withContext := range []bool{false, true} {
		for _, wildcard := range []string{"0.0.0.0", "::"} {
			for _, ipv6 := range []bool{false, true} {
				for _, wrongPort := range []bool{false, true} {
					t.Run(fmt.Sprintf("listener=%v/wildcard=%s/ipv6=%v/wrong-port=%v", withContext, wildcard, ipv6, wrongPort), func(t *testing.T) {
						r := newTestReactor(t)
						local := "192.0.2.1"
						remote := "198.51.100.2"
						prefix := "198.51.100.0/24"
						if ipv6 {
							local = "2001:db8:1::1"
							remote = "2001:db8:2::2"
							prefix = "2001:db8:2::/64"
						}
						group := dynamicInboundGroup("wildcard", prefix, wildcard)
						r.dynamicGroups = []*DynamicGroupConfig{group}
						conn := dynamicInboundConnection(t, local, remote)
						if wrongPort {
							conn.local.Port = 3179
						}
						var accepted *PeerSettings
						calls := 0
						r.connCallback = func(got net.Conn, settings *PeerSettings) {
							calls++
							assert.Same(t, conn, got)
							accepted = settings
						}
						if withContext {
							r.handleConnectionWithContext(conn, group.Settings.LocalAddress)
						} else {
							r.handleConnection(conn)
						}
						if wrongPort {
							assert.Zero(t, calls)
							assert.Empty(t, r.peers)
							assert.Zero(t, group.ActivePeers.Load())
							assert.True(t, conn.closed)
							return
						}
						assert.Equal(t, 1, calls, "the wildcard listener must admit its concrete endpoint")
						require.NotNil(t, accepted)
						key := netip.AddrPortFrom(netip.MustParseAddr(remote), DefaultBGPPort)
						assert.Equal(t, key, accepted.PeerKey())
						assert.Equal(t, group.Settings.LocalAddress, accepted.LocalAddress)
						assert.Equal(t, group.GroupName, accepted.GroupName)
						require.Len(t, r.peers, 1)
						require.Contains(t, r.peers, key)
						assert.Same(t, accepted, r.peers[key].Settings())
						assert.Equal(t, int32(1), group.ActivePeers.Load())
						assert.False(t, conn.closed)
					})
				}
			}
		}
	}
}

// TestDynamicInboundDefaultPortFollowsGlobalListener starts the existing global
// ListenAddr API on an assigned port. Auto-local templates inherit that actual
// socket for default ports, and must produce a useful member before shutdown.
func TestDynamicInboundDefaultPortFollowsGlobalListener(t *testing.T) {
	for _, configuredPort := range []uint16{0, DefaultBGPPort} {
		t.Run(fmt.Sprintf("configured-port=%d", configuredPort), func(t *testing.T) {
			r := New(&Config{ListenAddr: "127.0.0.1:0", LocalAS: 65000, Standalone: true})
			group := newTestDynamicGroup("global", []string{"127.0.0.0/8"}, 10)
			group.Settings.Connection = ConnectionPassive
			group.Settings.LocalPort = configuredPort
			r.SetDynamicGroups([]*DynamicGroupConfig{group})
			type admission struct {
				settings *PeerSettings
				local    netip.AddrPort
			}
			accepted := make(chan admission, 1)
			r.connCallback = func(conn net.Conn, settings *PeerSettings) {
				local, ok := conn.LocalAddr().(*net.TCPAddr)
				assert.True(t, ok)
				if !ok {
					assert.NoError(t, conn.Close())
					return
				}
				accepted <- admission{settings: settings, local: local.AddrPort()}
				assert.NoError(t, conn.Close())
			}
			require.NoError(t, r.Start())
			t.Cleanup(func() { stopAndWait(t, r) })
			bound, ok := r.ListenAddr().(*net.TCPAddr)
			require.True(t, ok)
			require.Positive(t, bound.Port)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", bound.String())
			require.NoError(t, err)
			t.Cleanup(func() { assert.NoError(t, conn.Close()) })
			var got admission
			select {
			case got = <-accepted:
			case <-ctx.Done():
				t.Fatal("the actual global listener did not deliver the default-port dynamic member")
			}
			require.NotNil(t, got.settings)
			assert.Equal(t, bound.AddrPort(), got.local)
			assert.Equal(t, group.GroupName, got.settings.GroupName)
			assert.Equal(t, configuredPort, got.settings.LocalPort)
			assert.Equal(t, group.Settings.LocalAddress, got.settings.LocalAddress)
			key := netip.MustParseAddrPort("127.0.0.1:179")
			assert.Equal(t, key, got.settings.PeerKey())
			r.mu.RLock()
			count := len(r.peers)
			peer := r.peers[key]
			r.mu.RUnlock()
			assert.Equal(t, 1, count)
			require.NotNil(t, peer)
			assert.Same(t, got.settings, peer.Settings())
			assert.Equal(t, int32(1), group.ActivePeers.Load())
		})
	}
}

// dynamicInboundAccepted observes the actual entry point callback and installed
// member together: dropping all sockets or constructing an uninstalled settings
// value cannot satisfy its useful acceptance oracle.
func dynamicInboundAccepted(t *testing.T, r *Reactor, conn *dynamicInboundConn, withContext bool, group *DynamicGroupConfig) {
	t.Helper()
	var accepted *PeerSettings
	calls := 0
	r.connCallback = func(got net.Conn, settings *PeerSettings) {
		calls++
		assert.Same(t, conn, got)
		accepted = settings
	}
	dynamicInboundHandle(r, conn, withContext)
	assert.Equal(t, 1, calls, "the receiving group must deliver the socket")
	require.NotNil(t, accepted)
	address := conn.remote.AddrPort().Addr()
	key := netip.AddrPortFrom(address, DefaultBGPPort)
	assert.Equal(t, address, accepted.Address, "retain the incoming scoped address")
	assert.Equal(t, key, accepted.PeerKey(), "retain its original zone and canonical BGP port")
	assert.Equal(t, "dyn-"+address.String(), accepted.Name)
	assert.Equal(t, group.GroupName, accepted.GroupName)
	assert.Equal(t, group.Settings.LocalAddress, accepted.LocalAddress)
	assert.Equal(t, group.Settings.LocalPort, accepted.LocalPort)
	assert.Equal(t, ConnectionPassive, accepted.Connection)
	assert.True(t, accepted.IsDynamic)
	require.Len(t, r.peers, 1)
	require.Contains(t, r.peers, key)
	assert.Same(t, accepted, r.peers[key].Settings())
	assert.Equal(t, int32(1), group.ActivePeers.Load())
	assert.False(t, conn.closed, "a useful selected socket remains open for the callback")
}

func dynamicInboundGroup(name, prefix, local string) *DynamicGroupConfig {
	group := newTestDynamicGroup(name, []string{prefix}, 10)
	group.Settings.LocalAddress = netip.MustParseAddr(local)
	group.Settings.LocalPort = 1179
	group.Settings.Connection = ConnectionPassive
	return group
}

func dynamicInboundHandle(r *Reactor, conn *dynamicInboundConn, withContext bool) {
	if withContext {
		r.handleConnectionWithContext(conn, conn.local.AddrPort().Addr())
		return
	}
	r.handleConnection(conn)
}

// dynamicInboundConn replaces only the endpoints of a real pipe. Its close
// observation changes only after the underlying connection actually closes.
// All accesses are synchronous within one inbound handler and test goroutine.
type dynamicInboundConn struct {
	net.Conn
	local  *net.TCPAddr
	remote *net.TCPAddr
	closed bool
}

func (c *dynamicInboundConn) LocalAddr() net.Addr  { return c.local }
func (c *dynamicInboundConn) RemoteAddr() net.Addr { return c.remote }

func (c *dynamicInboundConn) Close() error {
	if err := c.Conn.Close(); err != nil {
		return err
	}
	c.closed = true
	return nil
}

func dynamicInboundConnection(t *testing.T, local, remote string) *dynamicInboundConn {
	t.Helper()
	near, far := net.Pipe()
	t.Cleanup(func() {
		assert.NoError(t, near.Close())
		assert.NoError(t, far.Close())
	})
	return &dynamicInboundConn{
		Conn: near,
		local: net.TCPAddrFromAddrPort(netip.AddrPortFrom(
			netip.MustParseAddr(local), 1179)),
		remote: net.TCPAddrFromAddrPort(netip.AddrPortFrom(
			netip.MustParseAddr(remote), 40770)),
	}
}

// TestDynamicInboundMixedListenerContext keeps eligible longest-prefix selection
// useful when the chosen template and the bound listener have different wildcard
// declarations. The actual accepted TCP endpoint determines dynamic ownership.
func TestDynamicInboundMixedListenerContext(t *testing.T) {
	for _, ipv6 := range []bool{false, true} {
		for _, wildcardTemplate := range []bool{false, true} {
			t.Run(fmt.Sprintf("ipv6=%v/wildcard-template=%v", ipv6, wildcardTemplate), func(t *testing.T) {
				local := "192.0.2.1"
				remote := "198.51.100.2"
				wildcard := "0.0.0.0"
				broadRange := "198.51.100.0/24"
				wantedRange := "198.51.100.2/32"
				if ipv6 {
					local = "2001:db8:1::1"
					remote = "2001:db8:2::2"
					wildcard = "::"
					broadRange = "2001:db8:2::/64"
					wantedRange = "2001:db8:2::2/128"
				}
				listenerLocal := wildcard
				wantedLocal := local
				if wildcardTemplate {
					listenerLocal = local
					wantedLocal = wildcard
				}
				r := newTestReactor(t)
				broad := dynamicInboundGroup("eligible-broad", broadRange, listenerLocal)
				wanted := dynamicInboundGroup("eligible-specific", wantedRange, wantedLocal)
				r.dynamicGroups = []*DynamicGroupConfig{broad, wanted}
				conn := dynamicInboundConnection(t, local, remote)
				var accepted *PeerSettings
				calls := 0
				r.connCallback = func(got net.Conn, settings *PeerSettings) {
					calls++
					assert.Same(t, conn, got)
					accepted = settings
				}
				r.handleListenerConnection(conn, netip.MustParseAddr(listenerLocal), 1179)
				assert.Equal(t, 1, calls, "the actual-local-qualified member must reach the callback")
				assert.False(t, conn.closed, "a useful accepted socket remains open")
				assert.Zero(t, broad.ActivePeers.Load(), "the eligible longest remote prefix wins")
				assert.Equal(t, int32(1), wanted.ActivePeers.Load())
				key := netip.AddrPortFrom(netip.MustParseAddr(remote), DefaultBGPPort)
				require.Len(t, r.peers, 1)
				require.Contains(t, r.peers, key)
				assert.Equal(t, wanted.GroupName, r.peers[key].Settings().GroupName)
				require.NotNil(t, accepted)
				assert.Same(t, accepted, r.peers[key].Settings())
				assert.Equal(t, key, accepted.PeerKey())
				assert.Equal(t, netip.MustParseAddr(remote), accepted.Address)
				assert.Equal(t, "dyn-"+remote, accepted.Name)
				assert.Equal(t, wanted.GroupName, accepted.GroupName)
				assert.Equal(t, wanted.Settings.LocalAddress, accepted.LocalAddress)
				assert.Equal(t, wanted.Settings.LocalPort, accepted.LocalPort)
				assert.Equal(t, ConnectionPassive, accepted.Connection)
				assert.True(t, accepted.IsDynamic)
			})
		}
	}
}

// TestDynamicInboundMixedListenerContextRefusesForeignSocket keeps the actual
// local address and port guard independent of the bound listener declaration.
// Neither a wildcard context nor a matching context may authorize another socket.
func TestDynamicInboundMixedListenerContextRefusesForeignSocket(t *testing.T) {
	for _, ipv6 := range []bool{false, true} {
		for _, wildcardContext := range []bool{false, true} {
			for _, wrongPort := range []bool{false, true} {
				t.Run(fmt.Sprintf("ipv6=%v/wildcard-context=%v/wrong-port=%v", ipv6, wildcardContext, wrongPort), func(t *testing.T) {
					local := "192.0.2.1"
					foreignLocal := "192.0.2.9"
					remote := "198.51.100.2"
					wildcard := "0.0.0.0"
					prefix := "198.51.100.0/24"
					if ipv6 {
						local = "2001:db8:1::1"
						foreignLocal = "2001:db8:1::9"
						remote = "2001:db8:2::2"
						wildcard = "::"
						prefix = "2001:db8:2::/64"
					}
					listenerLocal := local
					if wildcardContext {
						listenerLocal = wildcard
					}
					r := newTestReactor(t)
					group := dynamicInboundGroup("other-socket", prefix, local)
					r.dynamicGroups = []*DynamicGroupConfig{group}
					if wrongPort {
						foreignLocal = local
					}
					conn := dynamicInboundConnection(t, foreignLocal, remote)
					if wrongPort {
						conn.local.Port = 3179
					}
					calls := 0
					r.connCallback = func(net.Conn, *PeerSettings) { calls++ }
					r.handleListenerConnection(conn, netip.MustParseAddr(listenerLocal), 1179)
					assert.Zero(t, calls, "listener context cannot authorize the actual foreign endpoint")
					assert.Empty(t, r.peers, "refusal must precede member publication")
					assert.Zero(t, group.ActivePeers.Load(), "refusal must precede group accounting")
					assert.True(t, conn.closed, "the refused socket must close")
				})
			}
		}
	}
}
