// Design: docs/architecture/config/syntax.md -- scoped TCP endpoint identity
// Related: reactor_peers.go -- lookup and admission keep configured peer keys
// Related: reactor_connection.go -- shared and dedicated listener ownership

package reactor

import (
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/network"
)

const (
	scopeAliasLocalIP = "fe80::1"
	scopeAliasPeerIP  = "fe80::2"
	scopeAliasOtherIP = "fe80::9"
	scopeAliasPort    = 2179
)

// TestScopedTransportAliasPeerLookup uses the host's existing loopback name and
// index to identify one interface without changing the host. Both lookup forms
// must return the installed peer and its original, port-bearing key.
func TestScopedTransportAliasPeerLookup(t *testing.T) {
	zones := scopeAliasInterfaces(t)
	for _, aliases := range zones.aliases() {
		for _, port := range []uint16{DefaultBGPPort, scopeAliasPort} {
			t.Run(fmt.Sprintf("%s/port=%d", aliases.name, port), func(t *testing.T) {
				r := New(&Config{LocalAS: 65000})
				settings := scopeAliasSettings(aliases.configured, zones.name, port)
				peer := NewPeer(settings)
				key := settings.PeerKey()
				r.peers[key] = peer
				observed := netip.MustParseAddr(scopeAliasPeerIP).WithZone(aliases.observed)

				got, found := r.findPeerByAddr(observed)
				assert.True(t, found, "name and index identify the same interface")
				assert.Same(t, peer, got)
				gotKey, keyedPeer, keyed := r.findPeerKeyByAddr(observed)
				assert.True(t, keyed)
				assert.Same(t, peer, keyedPeer)
				assert.Equal(t, key, gotKey, "lookup must return the configured spelling and remote port")
				assert.Equal(t, key, settings.PeerKey())

				wrongZone := observed.WithZone(zones.wrong)
				_, found = r.findPeerByAddr(wrongZone)
				assert.False(t, found, "another interface is another transport identity")
				_, _, keyed = r.findPeerKeyByAddr(wrongZone)
				assert.False(t, keyed)
				_, found = r.findPeerByAddr(netip.MustParseAddr(scopeAliasOtherIP).WithZone(aliases.observed))
				assert.False(t, found, "interface aliases do not alias address bits")
			})
		}
	}
}

// TestScopedTransportAliasSharedAccept follows the observed TCPAddr through
// both shared accept rails. A configured alias wins before dynamic creation;
// local listener context accepts aliases and refuses another interface.
func TestScopedTransportAliasSharedAccept(t *testing.T) {
	zones := scopeAliasInterfaces(t)
	for _, aliases := range zones.aliases() {
		for _, context := range []bool{false, true} {
			for _, wrongLocal := range []bool{false, true} {
				if !context && wrongLocal {
					continue
				}
				t.Run(fmt.Sprintf("%s/context=%v/wrong-local=%v", aliases.name, context, wrongLocal), func(t *testing.T) {
					r := New(&Config{LocalAS: 65000})
					settings := scopeAliasSettings(aliases.configured, aliases.configured, scopeAliasPort)
					key := settings.PeerKey()
					r.peers[key] = NewPeer(settings)
					group := newTestDynamicGroup("fallback", []string{"fe80::/10"}, 10)
					group.Settings.LocalAddress = settings.LocalAddress
					r.dynamicGroups = []*DynamicGroupConfig{group}
					conn := scopeAliasConnection(t, scopeAliasPeerIP, aliases.observed)
					accepted := 0
					r.connCallback = func(got net.Conn, peer *PeerSettings) {
						accepted++
						assert.Same(t, conn, got)
						assert.Same(t, settings, peer, "an existing configured peer wins before a dynamic group")
					}
					if context {
						localZone := aliases.observed
						if wrongLocal {
							localZone = zones.wrong
						}
						r.handleConnectionWithContext(conn, netip.MustParseAddr(scopeAliasLocalIP).WithZone(localZone))
					} else {
						r.handleConnection(conn)
					}
					if wrongLocal {
						assert.Zero(t, accepted)
					} else {
						assert.Equal(t, 1, accepted, "the scoped configured peer must receive a useful socket")
					}
					assert.Len(t, r.peers, 1, "an alias must not create a second dynamic peer")
					assert.Zero(t, group.ActivePeers.Load())
					assert.Same(t, settings, r.peers[key].Settings())
					assert.Equal(t, key, settings.PeerKey())
				})
			}
		}
	}
}

// TestScopedTransportAliasListenerOwnership retains distinct declarations and
// makes their runtime aliases reuse one bound socket. Direct attribution remains
// available only to its sole passive peer on a custom listen port; another peer
// or a dynamic group removes that right.
func TestScopedTransportAliasListenerOwnership(t *testing.T) {
	zones := scopeAliasInterfaces(t)
	for _, aliases := range zones.aliases() {
		for _, owner := range []string{"sole", "peer", "group", "other-port", "other-interface"} {
			t.Run(aliases.name+"/"+owner, func(t *testing.T) {
				r := New(&Config{LocalAS: 65000})
				settings := scopeAliasSettings(zones.name, aliases.configured, scopeAliasPort)
				r.peers[settings.PeerKey()] = NewPeer(settings)
				wantCount := 1
				wantSpecs := 1
				if owner != "sole" {
					second := scopeAliasSettings(zones.name, aliases.observed, DefaultBGPPort)
					second.Address = netip.MustParseAddr(scopeAliasOtherIP).WithZone(zones.name)
					switch owner {
					case "peer":
						wantCount = 2
						wantSpecs = 2
						r.peers[second.PeerKey()] = NewPeer(second)
					case "group":
						wantCount = 2
						wantSpecs = 2
						r.dynamicGroups = []*DynamicGroupConfig{{GroupName: "co-owner", Settings: second}}
					case "other-port":
						wantSpecs = 2
						second.LocalPort++
						r.peers[second.PeerKey()] = NewPeer(second)
					case "other-interface":
						wantSpecs = 2
						second.LocalAddress = second.LocalAddress.WithZone(zones.wrong)
						r.peers[second.PeerKey()] = NewPeer(second)
					}
				}
				observedLocal := netip.MustParseAddr(scopeAliasLocalIP).WithZone(aliases.observed)
				count, only, _ := r.listenerClaimants(observedLocal, attrTestPort)
				assert.Equal(t, wantCount, count)
				assert.Len(t, r.listenerSpecs(), wantSpecs, "distinct configured spellings retain their declarations")
				switch owner {
				case "sole", "peer", "group":
					boundKey := netip.AddrPortFrom(observedLocal, attrTestPort).String()
					bound := NewListener(boundKey)
					bound.listener = scopeAliasBoundListener{address: &net.TCPAddr{
						IP: observedLocal.AsSlice(), Zone: aliases.observed, Port: attrTestPort,
					}}
					r.listeners[boundKey] = bound
					require.NoError(t, r.startMultiListeners(), "all declarations must reuse the bound physical socket")
					require.Len(t, r.listeners, 1)
					assert.Same(t, bound, r.listeners[boundKey])
				}
				if wantCount == 1 {
					assert.Same(t, settings, only)
					assert.Equal(t, settings.PeerKey(), r.listenerPeerKey(observedLocal, attrTestPort))
				} else {
					assert.Nil(t, only)
					assert.False(t, r.listenerPeerKey(observedLocal, attrTestPort).IsValid())
				}
				assert.False(t, r.listenerPeerKey(observedLocal, DefaultBGPPort).IsValid())
				assert.True(t, r.addressBound(observedLocal), "interface address events must find an aliased local binding")
				assert.Contains(t, r.listenPortsAt(observedLocal), attrTestPort)

				// A sole dedicated listener attributes by the socket, including
				// remote addresses absent from config. Co-owners require lookup.
				accepted := 0
				r.connCallback = func(_ net.Conn, got *PeerSettings) {
					accepted++
					assert.Same(t, settings, got)
				}
				conn := scopeAliasConnection(t, "fe80::99", zones.name)
				r.handleListenerConnection(conn, observedLocal, attrTestPort)
				if wantCount == 1 {
					assert.Equal(t, 1, accepted)
				} else {
					assert.Zero(t, accepted, "a shared listener must not attribute an unknown remote to its first claimant")
				}
			})
		}
	}
}

// TestScopedTransportAliasAdmissionGuards preserves configured keys before
// interface apply, then refuses physical duplicate and self-address aliases at
// reactor startup before resources exist. Different ports and interfaces remain
// distinct declarations; live admission is covered by its dedicated root.
func TestScopedTransportAliasAdmissionGuards(t *testing.T) {
	zones := scopeAliasInterfaces(t)
	for _, aliases := range zones.aliases() {
		t.Run(aliases.name+"/self", func(t *testing.T) {
			r := New(&Config{LocalAS: 65000})
			settings := scopeAliasSettings(aliases.configured, aliases.observed, DefaultBGPPort)
			t.Cleanup(r.Stop)
			r.SetListenerFactory(startupFailingListenerFactory{err: fmt.Errorf("fixture: physical validation must precede listeners")})
			settings.Address = netip.MustParseAddr(scopeAliasLocalIP).WithZone(aliases.configured)
			require.NoError(t, r.AddPeer(settings), "physical identity is checked after interface apply")
			err := r.StartWithContext(t.Context())
			require.Error(t, err)
			assert.Contains(t, err.Error(), "address cannot equal local-address")
			assert.Len(t, r.peers, 1, "the declaration remains available for correction")
			assert.Nil(t, r.ctx, "no worker, listener or startup context may precede validation")
			assert.False(t, r.Running())
		})
		for _, secondIdentity := range []string{"alias", "other-port", "other-interface"} {
			t.Run(aliases.name+"/"+secondIdentity, func(t *testing.T) {
				r := New(&Config{LocalAS: 65000})
				first := scopeAliasSettings(aliases.configured, zones.name, DefaultBGPPort)
				key := first.PeerKey()
				require.NoError(t, r.AddPeer(first))
				second := scopeAliasSettings(aliases.observed, zones.name, DefaultBGPPort)
				switch secondIdentity {
				case "alias":
					t.Cleanup(r.Stop)
					r.SetListenerFactory(startupFailingListenerFactory{err: fmt.Errorf("fixture: physical validation must precede listeners")})
					require.NoError(t, r.AddPeer(second), "physical identity is checked after interface apply")
					assert.Len(t, r.peers, 2)
					assert.ErrorIs(t, r.StartWithContext(t.Context()), ErrPeerExists)
					assert.Nil(t, r.ctx, "no worker, listener or startup context may precede validation")
					assert.False(t, r.Running())
				case "other-port":
					second.Port = scopeAliasPort
					assert.NoError(t, r.AddPeer(second))
					assert.Len(t, r.peers, 2)
				case "other-interface":
					second.Address = second.Address.WithZone(zones.wrong)
					assert.NoError(t, r.AddPeer(second))
					assert.Len(t, r.peers, 2)
				}
				assert.Same(t, first, r.peers[key].Settings())
				assert.Equal(t, key, first.PeerKey())
			})
		}
	}
}

// TestScopedTransportDefaultZoneAdmissionDefersToBind proves a declarative zone
// name is retained until socket construction. A numeric name may identify an
// interface that the same configuration creates after peer settings are built.
// The kernel, rather than AddPeer's current host view, decides whether it binds.
func TestScopedTransportDefaultZoneAdmissionDefersToBind(t *testing.T) {
	for _, zone := range []string{"0", "00", "0suffix"} {
		t.Run(zone, func(t *testing.T) {
			settings, err := reactorTransportPeer(t, scopeAliasLocalIP+"%"+zone,
				scopeAliasPeerIP+"%"+zone, "unchanged", true)
			require.NoError(t, err)
			r := New(&Config{LocalAS: 65000})
			key := settings.PeerKey()
			require.NoError(t, r.AddPeer(settings), "a configured interface may not exist when peers are built")
			require.Len(t, r.peers, 1)
			assert.Same(t, settings, r.peers[key].Settings())
			assert.Equal(t, key, settings.PeerKey())
			session := NewSession(settings)
			dialer, ok := session.dialer.(*network.RealDialer)
			require.True(t, ok)
			assertSessionTCPSource(t, &net.TCPAddr{IP: net.ParseIP(scopeAliasLocalIP), Zone: zone}, dialer.LocalAddr)
		})
	}
}

type scopeAliasInterface struct {
	name  string
	index string
	wrong string
	names map[string]bool
}

type scopeAliasPair struct {
	name       string
	configured string
	observed   string
}

func (s scopeAliasInterface) aliases() []scopeAliasPair {
	pairs := []scopeAliasPair{
		{name: "name-to-index", configured: s.name, observed: s.index},
		{name: "index-to-name", configured: s.index, observed: s.name},
		{name: "padded-index-to-name", configured: "0" + s.index, observed: s.name},
		{name: "name-to-padded-index", configured: s.name, observed: "0" + s.index},
	}
	if !s.names[s.index+"suffix"] {
		pairs = append(pairs, scopeAliasPair{
			name: "numeric-prefix-to-name", configured: s.index + "suffix", observed: s.name,
		})
	}
	return pairs
}

// scopeAliasInterfaces reads an existing loopback interface. No address is
// installed, no socket is bound to it, and no host interface is changed.
func scopeAliasInterfaces(t *testing.T) scopeAliasInterface {
	t.Helper()
	interfaces, err := net.Interfaces()
	require.NoError(t, err)
	zones := scopeAliasInterface{names: make(map[string]bool)}
	maxIndex := 0
	for _, iface := range interfaces {
		zones.names[iface.Name] = true
		maxIndex = max(maxIndex, iface.Index)
		if iface.Flags&net.FlagLoopback != 0 {
			zones.name = iface.Name
			zones.index = strconv.Itoa(iface.Index)
		}
	}
	require.NotEmpty(t, zones.name, "the transport identity test requires an existing loopback interface")
	for _, iface := range interfaces {
		if iface.Name == zones.index {
			if strconv.Itoa(iface.Index) != zones.index {
				t.Skip("a numeric interface name shadows the loopback's numeric index")
			}
		}
		if iface.Name != zones.name {
			zones.wrong = strconv.Itoa(iface.Index)
		}
	}
	if zones.wrong == "" {
		zones.wrong = strconv.Itoa(maxIndex + 1)
	}
	return zones
}

func scopeAliasSettings(remoteZone, localZone string, port uint16) *PeerSettings {
	settings := NewPeerSettings(netip.MustParseAddr(scopeAliasPeerIP).WithZone(remoteZone), 65000, 65001, 0x01020304)
	settings.LocalAddress = netip.MustParseAddr(scopeAliasLocalIP).WithZone(localZone)
	settings.LocalPort = attrTestPort
	settings.Port = port
	settings.Connection = ConnectionPassive
	return settings
}

func scopeAliasConnection(t *testing.T, address, zone string) *reactorTransportConn {
	t.Helper()
	local, remote := net.Pipe()
	t.Cleanup(func() {
		assert.NoError(t, local.Close())
		assert.NoError(t, remote.Close())
	})
	return &reactorTransportConn{Conn: local, remote: &net.TCPAddr{
		IP: net.ParseIP(address), Zone: zone, Port: scopeAliasPort,
	}}
}
