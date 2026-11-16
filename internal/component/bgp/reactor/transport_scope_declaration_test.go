// Design: docs/architecture/config/syntax.md -- declared scoped TCP endpoints
// Related: transport_scope.go -- runtime interface names precede numeric indices

package reactor

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/network"
)

// TestTransportScopeFutureNumericNames uses immutable before/after tables to
// prove why socket fallback cannot decide the identity of future declarations.
// The runtime resolver must retain Go's numeric fallback before creation and
// switch to each literal name once the two distinct devices exist.
func TestTransportScopeFutureNumericNames(t *testing.T) {
	a := netip.MustParseAddr("fe80::2%3")
	b := netip.MustParseAddr("fe80::2%03")
	before := []net.Interface{{Name: "unrelated", Index: 17}}
	assert.True(t, sameTransportAddressOn(a, b, before), "runtime numeric fallback remains available even without index 3")
	after := []net.Interface{{Name: "3", Index: 21}, {Name: "03", Index: 22}}
	assert.False(t, sameTransportAddressOn(a, b, after), "created literal names select distinct devices")
	assert.True(t, sameTransportAddressOn(a, a.WithZone("21"), after))
	assert.True(t, sameTransportAddressOn(b, b.WithZone("22"), after))
}

// TestTransportScopePhysicalKeyBoundaries fixes the runtime validation keys
// against an immutable name table, including name shadowing and unresolved or
// default scopes. Configured values remain unchanged by key construction.
func TestTransportScopePhysicalKeyBoundaries(t *testing.T) {
	interfaces := []net.Interface{
		{Name: "port", Index: 3},
		{Name: "3", Index: 7},
		{Name: "0", Index: 9},
	}
	for _, test := range []struct {
		zone string
		want string
	}{
		{zone: "port", want: "3"},
		{zone: "03", want: "3"},
		{zone: "3suffix", want: "3"},
		{zone: "3", want: "7"},
		{zone: "0", want: "9"},
		{zone: "00", want: "00"},
		{zone: "future", want: "future"},
		{zone: "", want: ""},
		{zone: "999999999999suffix", want: "16777215"},
	} {
		t.Run("zone="+test.zone, func(t *testing.T) {
			original := netip.MustParseAddr("fe80::2").WithZone(test.zone)
			assert.Equal(t, original.WithZone(test.want), transportAddressKeyOn(original, interfaces))
			assert.Equal(t, test.zone, original.Zone(), "key construction cannot rewrite the configured scope")
		})
	}
}

// TestTransportScopeDeclaredNumericPeers follows a configuration declaring two
// future dummy names through parsing and AddPeer, retaining both exact keys and
// their real Session dialer sources. It creates no host device or socket.
func TestTransportScopeDeclaredNumericPeers(t *testing.T) {
	zones := futureNumericScopeNames(t)
	peers := declaredNumericScopePeers(t, zones)
	r := New(&Config{LocalAS: 65000})
	for _, settings := range peers {
		key := settings.PeerKey()
		require.NoError(t, r.AddPeer(settings), "different declared devices must survive before interface apply")
		require.Contains(t, r.peers, key)
		assert.Same(t, settings, r.peers[key].Settings())
		assert.Equal(t, key, settings.PeerKey())
		session := NewSession(settings)
		dialer, ok := session.dialer.(*network.RealDialer)
		require.True(t, ok)
		assertSessionTCPSource(t, &net.TCPAddr{IP: net.ParseIP("fe80::1"), Zone: settings.LocalAddress.Zone()}, dialer.LocalAddr)
	}
	require.Len(t, r.peers, 2)
	assert.ErrorIs(t, r.AddPeer(peers[0]), ErrPeerExists, "an exact configured duplicate is still refused")
	assert.Len(t, r.peers, 2)
}

// TestTransportScopeDeclaredNumericSelfCheck retains distinct prospective
// interfaces even when their address bits match, while the exact same scoped
// local and remote endpoint remains a configuration error.
func TestTransportScopeDeclaredNumericSelfCheck(t *testing.T) {
	zones := futureNumericScopeNames(t)
	for _, sameZone := range []bool{false, true} {
		t.Run(fmt.Sprintf("same-zone=%v", sameZone), func(t *testing.T) {
			remoteZone := zones[1]
			if sameZone {
				remoteZone = zones[0]
			}
			settings, err := reactorTransportPeer(t, "fe80::1%"+zones[0], "fe80::1%"+remoteZone, "unchanged", false)
			require.NoError(t, err)
			r := New(&Config{LocalAS: 65000})
			err = r.AddPeer(settings)
			if sameZone {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "address cannot equal local-address")
				assert.Empty(t, r.peers)
				return
			}
			require.NoError(t, err, "future literal names must not become a self-address through numeric fallback")
			assert.Contains(t, r.peers, settings.PeerKey())
		})
	}
}

// TestTransportScopeDeclaredNumericListeners retains the listening claims of
// two future numeric names for peers and dynamic groups. An identical spelling
// still contributes only one declaration, with its original local port.
func TestTransportScopeDeclaredNumericListeners(t *testing.T) {
	zones := futureNumericScopeNames(t)
	for _, owner := range []string{"peer", "group", "exact-spelling"} {
		t.Run(owner, func(t *testing.T) {
			peers := declaredNumericScopePeers(t, zones)
			r := New(&Config{LocalAS: 65000})
			r.peers[peers[0].PeerKey()] = NewPeer(peers[0])
			if owner == "group" {
				r.dynamicGroups = []*DynamicGroupConfig{{GroupName: "future", Settings: peers[1]}}
			} else {
				if owner == "exact-spelling" {
					peers[1].LocalAddress = peers[0].LocalAddress
				}
				r.peers[peers[1].PeerKey()] = NewPeer(peers[1])
			}
			specs := r.listenerSpecs()
			wantCount := 2
			if owner == "exact-spelling" {
				wantCount = 1
			}
			require.Len(t, specs, wantCount, "a declaration cannot collapse devices that have not been created")
			for _, settings := range peers {
				assert.Contains(t, specs, listenerSpec{addr: settings.LocalAddress, port: 1179})
			}
		})
	}
}

// TestTransportScopeStartRejectsPhysicalConflicts seeds declared settings and
// crosses the actual reactor start boundary. Runtime duplicate/self aliases
// must be refused before a listener, worker or startup context is created.
// A failing listener factory makes a missing guard visible without binding.
func TestTransportScopeStartRejectsPhysicalConflicts(t *testing.T) {
	zones := scopeAliasInterfaces(t)
	for _, aliases := range zones.aliases() {
		for _, conflict := range []string{"duplicate", "self"} {
			t.Run(aliases.name+"/"+conflict, func(t *testing.T) {
				r := New(&Config{LocalAS: 65000, Standalone: true})
				t.Cleanup(r.Stop)
				listenErr := errors.New("fixture: transport validation must precede listener creation")
				r.SetListenerFactory(startupFailingListenerFactory{err: listenErr})
				first := scopeAliasSettings(aliases.configured, zones.name, DefaultBGPPort)
				if conflict == "self" {
					first.Address = first.LocalAddress.WithZone(aliases.configured)
					first.LocalAddress = first.LocalAddress.WithZone(aliases.observed)
				} else {
					second := scopeAliasSettings(aliases.observed, zones.name, DefaultBGPPort)
					r.peers[second.PeerKey()] = NewPeer(second)
				}
				r.peers[first.PeerKey()] = NewPeer(first)
				err := r.StartWithContext(t.Context())
				if conflict == "duplicate" {
					assert.ErrorIs(t, err, ErrPeerExists)
				} else {
					require.Error(t, err)
					assert.Contains(t, err.Error(), "address cannot equal local-address")
				}
				assert.NotErrorIs(t, err, listenErr)
				assert.Nil(t, r.ctx, "transport validation must run before startup resources exist")
				assert.False(t, r.Running())
				assert.Empty(t, r.listeners)
			})
		}
	}
}

// TestTransportScopeLiveAdmissionRejectsPhysicalConflicts opens the real empty
// listener pass, then adds active-only peers while the peer-start barrier is
// still closed. A live addition in this window must use physical identity;
// distinct remote ports and interfaces remain useful admitted controls.
func TestTransportScopeLiveAdmissionRejectsPhysicalConflicts(t *testing.T) {
	zones := scopeAliasInterfaces(t)
	for _, aliases := range zones.aliases() {
		for _, identity := range []string{"alias", "self", "other-port", "other-interface"} {
			t.Run(aliases.name+"/"+identity, func(t *testing.T) {
				r := New(&Config{LocalAS: 65000})
				require.NoError(t, r.startMultiListeners())
				require.True(t, r.listenersStarted)
				require.False(t, r.running, "this is the listener-open, peer-held startup window")
				first := scopeAliasSettings(aliases.configured, zones.name, DefaultBGPPort)
				first.Connection = ConnectionActive
				require.NoError(t, r.AddPeer(first))
				second := scopeAliasSettings(aliases.observed, zones.name, DefaultBGPPort)
				second.Connection = ConnectionActive
				if identity == "self" {
					second.Address = second.LocalAddress.WithZone(aliases.configured)
					second.LocalAddress = second.LocalAddress.WithZone(aliases.observed)
				}
				if identity == "other-port" {
					second.Port = scopeAliasPort
				}
				if identity == "other-interface" {
					second.Address = second.Address.WithZone(zones.wrong)
				}
				err := r.AddPeer(second)
				if identity == "alias" {
					assert.ErrorIs(t, err, ErrPeerExists)
					assert.Len(t, r.peers, 1)
					return
				}
				if identity == "self" {
					require.Error(t, err)
					assert.Contains(t, err.Error(), "address cannot equal local-address")
					assert.Len(t, r.peers, 1)
					return
				}
				require.NoError(t, err)
				assert.Len(t, r.peers, 2)
			})
		}
	}
}

// futureNumericScopeNames chooses two valid, absent literal names with the same
// Go numeric fallback. Usually these are 3 and 03; another task's existing
// numeric device is respected. The immutable proof above fixes those spellings
// independently of this read-only host precondition.
func futureNumericScopeNames(t *testing.T) [2]string {
	t.Helper()
	interfaces, err := net.Interfaces()
	require.NoError(t, err)
	var zones [2]string
	found := 0
	for padding := range 15 {
		candidate := strings.Repeat("0", padding) + "3"
		present := false
		for _, iface := range interfaces {
			if iface.Name == candidate {
				present = true
				break
			}
		}
		if present {
			continue
		}
		zones[found] = candidate
		found++
		if found == len(zones) {
			return zones
		}
	}
	t.Fatal("fixture needs two unused numeric interface names of at most 15 bytes")
	return zones
}

// declaredNumericScopePeers parses the declarations without running iface's
// apply callback. Created kinds use their declared names as kernel names.
func declaredNumericScopePeers(t *testing.T, zones [2]string) []*PeerSettings {
	t.Helper()
	tree, err := config.ParseTreeForValidation(fmt.Sprintf(`interface {
    dummy %s {}
    dummy %s {}
}
bgp {
    router-id 1.2.3.4;
    peer first {
        connection { local { ip fe80::1%%%s; port 1179; accept true; } remote { ip fe80::2%%%s; port 2179; connect true; } }
        session { asn { local 65000; remote 65001; } next-hop unchanged; }
    }
    peer second {
        connection { local { ip fe80::1%%%s; port 1179; accept true; } remote { ip fe80::2%%%s; port 2179; connect true; } }
        session { asn { local 65000; remote 65001; } next-hop unchanged; }
    }
}`, zones[0], zones[1], zones[0], zones[0], zones[1], zones[1]))
	require.NoError(t, err)
	require.NotNil(t, tree.GetContainer("interface"))
	bgp := tree.GetContainer("bgp")
	require.NotNil(t, bgp)
	peers, err := PeersFromTree(bgp.ToMap())
	require.NoError(t, err)
	require.Len(t, peers, 2)
	return peers
}
