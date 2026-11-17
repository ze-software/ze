// Design: docs/architecture/config/syntax.md -- scoped TCP endpoint identity
// Related: reactor_connection.go -- dedicated listener attribution
// Related: reactor_peers.go -- shared listener retention during removal

package reactor

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScopedTransportReadFailureCannotGrantDedicatedOwnership reaches both
// attribution rails: losing the interface table must not turn a shared socket
// into authority to serve an unrelated remote under its exact-zone claimant.
func TestScopedTransportReadFailureCannotGrantDedicatedOwnership(t *testing.T) {
	for _, owner := range []string{"sole", "peer", "group", "other-interface", "other-port", "default-port"} {
		for _, unreadable := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/unreadable=%v", owner, unreadable), func(t *testing.T) {
				r := scopeReadFailureReactor(unreadable)
				first := scopeAliasSettings("if0", "if0", scopeAliasPort)
				if owner == "default-port" {
					first.LocalPort = DefaultBGPPort
				}
				firstKey := first.PeerKey()
				r.peers[firstKey] = NewPeer(first)
				var group *DynamicGroupConfig
				if owner != "sole" && owner != "default-port" {
					second := scopeAliasSettings("if0", "7", DefaultBGPPort)
					second.Address = netip.MustParseAddr("fe80::3%if0")
					switch owner {
					case "group":
						group = newTestDynamicGroup("co-owner", []string{"fe80::/120"}, 10)
						group.Settings.LocalAddress = second.LocalAddress
						group.Settings.LocalPort = second.LocalPort
						group.Settings.Connection = ConnectionPassive
						r.dynamicGroups = []*DynamicGroupConfig{group}
					case "other-interface":
						second.LocalAddress = second.LocalAddress.WithZone("if1")
						r.peers[second.PeerKey()] = NewPeer(second)
					case "other-port":
						second.LocalPort++
						r.peers[second.PeerKey()] = NewPeer(second)
					case "peer":
						r.peers[second.PeerKey()] = NewPeer(second)
					}
				}
				local := first.LocalAddress
				port := int(first.LocalPort)
				wantDirect := owner == "sole" || owner == "other-port" || (owner == "other-interface" && !unreadable)
				key := r.listenerPeerKey(local, port)
				if wantDirect {
					assert.Equal(t, firstKey, key)
				} else {
					assert.False(t, key.IsValid(), "uncertain co-ownership cannot grant dedicated attribution")
				}

				var accepted []*PeerSettings
				r.connCallback = func(_ net.Conn, settings *PeerSettings) {
					accepted = append(accepted, settings)
				}
				peerCount := len(r.peers)
				foreign := dynamicInboundConnection(t, local.String(), "fe80::400%if0")
				foreign.local.Port = port
				r.handleListenerConnection(foreign, local, port)
				if wantDirect {
					require.Len(t, accepted, 1)
					assert.Same(t, first, accepted[0])
					assert.False(t, foreign.closed)
				} else {
					assert.Empty(t, accepted, "a foreign remote must not receive the first claimant's policy")
					assert.True(t, foreign.closed)
				}
				assert.Len(t, r.peers, peerCount, "refusal must not create an unknown dynamic member")
				assert.Same(t, first, r.peers[firstKey].Settings())
				assert.Equal(t, firstKey, first.PeerKey())
				if group != nil {
					assert.Zero(t, group.ActivePeers.Load())
				}

				// Exact configured attribution stays useful even when an alias
				// comparison cannot establish the socket's exclusive owner.
				before := len(accepted)
				known := dynamicInboundConnection(t, local.String(), first.Address.String())
				known.local.Port = port
				r.handleListenerConnection(known, local, port)
				require.Len(t, accepted, before+1)
				assert.Same(t, first, accepted[before])
				assert.False(t, known.closed)

				if group != nil && !unreadable {
					dynamic := dynamicInboundConnection(t, local.String(), "fe80::4%if0")
					dynamic.local.Port = port
					dynamic.local.Zone = group.Settings.LocalAddress.Zone()
					r.handleListenerConnection(dynamic, local, port)
					require.Len(t, accepted, before+2)
					assert.True(t, accepted[before+1].IsDynamic)
					assert.Equal(t, "co-owner", accepted[before+1].GroupName)
					assert.False(t, dynamic.closed)
					assert.EqualValues(t, 1, group.ActivePeers.Load())
					assert.Len(t, r.peers, peerCount+1)
				}
			})
		}
	}
}

// TestScopedTransportReadFailureKeepsAliasedCoOwnerListener removes an exact
// claimant through RemovePeer. Unknown alias ownership must not close the socket
// still claimed by a peer or group; removing its true last owner must close it.
func TestScopedTransportReadFailureKeepsAliasedCoOwnerListener(t *testing.T) {
	for _, owner := range []string{"sole", "peer", "group"} {
		for _, unreadable := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/unreadable=%v", owner, unreadable), func(t *testing.T) {
				r := scopeReadFailureReactor(unreadable)
				first := scopeAliasSettings("if0", "if0", scopeAliasPort)
				firstKey := first.PeerKey()
				r.peers[firstKey] = NewPeer(first)
				second := scopeAliasSettings("if0", "7", DefaultBGPPort)
				second.Address = netip.MustParseAddr("fe80::3%if0")
				var group *DynamicGroupConfig
				switch owner {
				case "peer":
					r.peers[second.PeerKey()] = NewPeer(second)
				case "group":
					group = newTestDynamicGroup("co-owner", []string{"fe80::/120"}, 10)
					group.Settings.LocalAddress = second.LocalAddress
					group.Settings.LocalPort = second.LocalPort
					group.Settings.Connection = ConnectionPassive
					r.dynamicGroups = []*DynamicGroupConfig{group}
				}
				boundKey := netip.AddrPortFrom(first.LocalAddress, first.LocalPort).String()
				bound := NewListener(boundKey)
				socket := &scopeReadFailureListener{address: net.TCPAddrFromAddrPort(
					netip.AddrPortFrom(first.LocalAddress, first.LocalPort))}
				bound.listener = socket
				r.listeners[boundKey] = bound

				require.NoError(t, r.RemovePeer(first.Address))
				assert.NotContains(t, r.peers, firstKey)
				if owner == "sole" {
					assert.Empty(t, r.listeners)
					assert.Equal(t, 1, socket.closes, "a listener without any claimant must be released")
					return
				}
				require.Len(t, r.listeners, 1, "uncertain alias ownership cannot release a co-owned socket")
				assert.Same(t, bound, r.listeners[boundKey], "retain the original socket and configured map key")
				assert.Zero(t, socket.closes)
				assert.Equal(t, firstKey, first.PeerKey())
				if owner == "peer" {
					require.Len(t, r.peers, 1)
					assert.Same(t, second, r.peers[second.PeerKey()].Settings())
				} else {
					assert.Empty(t, r.peers)
					require.Len(t, r.dynamicGroups, 1)
					assert.Same(t, group, r.dynamicGroups[0])
					assert.Zero(t, group.ActivePeers.Load())
				}
				if !unreadable {
					var accepted *PeerSettings
					r.connCallback = func(_ net.Conn, settings *PeerSettings) { accepted = settings }
					remote := second.Address.String()
					if group != nil {
						remote = "fe80::4%if0"
					}
					conn := dynamicInboundConnection(t, second.LocalAddress.String(), remote)
					conn.local.Port = int(first.LocalPort)
					r.handleListenerConnection(conn, second.LocalAddress, int(first.LocalPort))
					require.NotNil(t, accepted, "the surviving owner must still receive useful connections")
					assert.False(t, conn.closed)
					if group == nil {
						assert.Same(t, second, accepted)
					} else {
						assert.True(t, accepted.IsDynamic)
						assert.EqualValues(t, 1, group.ActivePeers.Load())
					}
					assert.Zero(t, socket.closes)
				}
			})
		}
	}
}

// Each reactor receives its immutable source before any handler runs. The real
// constructor retains net.Interfaces; no process-wide hook or host change is used.
func scopeReadFailureReactor(unreadable bool) *Reactor {
	r := New(&Config{LocalAS: 65000})
	if unreadable {
		r.readInterfaces = func() ([]net.Interface, error) {
			return nil, errors.New("fixture: transport interface table unavailable")
		}
		return r
	}
	r.readInterfaces = func() ([]net.Interface, error) {
		return []net.Interface{{Index: 7, Name: "if0"}, {Index: 8, Name: "if1"}}, nil
	}
	return r
}

// The listener never starts an accept goroutine. Only the real removal path's
// call to Listener.Stop can close this observed socket.
type scopeReadFailureListener struct {
	address *net.TCPAddr
	closes  int
}

func (l *scopeReadFailureListener) Accept() (net.Conn, error) {
	return nil, errors.New("fixture: no accept loop")
}

func (l *scopeReadFailureListener) Close() error {
	l.closes++
	return nil
}

func (l *scopeReadFailureListener) Addr() net.Addr { return l.address }
