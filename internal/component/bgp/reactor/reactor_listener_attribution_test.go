// RFC: rfc/short/rfc4271.md — Section 6.8, the inbound connection an accept must attribute
package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

const attrTestPort = 1790

// attrReactor builds a reactor holding the given passive peers and dynamic
// groups, all bound to local. Every peer speaks on attrTestPort unless the case
// says otherwise, which is what a listener with more than one claimant needs.
func attrReactor(t *testing.T, peers, groups []*PeerSettings) *Reactor {
	t.Helper()
	r := &Reactor{
		config: &Config{Port: attrTestPort},
		peers:  make(map[netip.AddrPort]*Peer),
	}
	for _, s := range peers {
		r.peers[s.PeerKey()] = NewPeer(s)
	}
	for i, s := range groups {
		r.dynamicGroups = append(r.dynamicGroups, &DynamicGroupConfig{
			GroupName: "g" + string(rune('a'+i)),
			Settings:  s,
		})
	}
	return r
}

func attrPeer(addr string, port uint16, mode ConnectionMode) *PeerSettings {
	s := NewPeerSettings(netip.MustParseAddr(addr), 65533, 65010, 0x01020304)
	s.LocalAddress = netip.MustParseAddr("127.0.0.1")
	s.Port = port
	s.Connection = mode
	return s
}

// TestListenerPeerKeyRoutesDirectlyForASoleClaimant covers the one shape in which
// a listener attributes by its own identity instead of by the connection's
// source address.
//
// VALIDATES: listenerPeerKey returns a peer key only when that peer is the single
//
//	claimant of the (local address, port) pair and speaks on a port of
//	its own.
//
// PREVENTS: handleDirectConnection serving a connection under the policy of a peer
//
//	it was not opened for. It reads no remote address, so a socket a
//	second peer or a dynamic group also accepts on cannot be attributed
//	that way at all.
func TestListenerPeerKeyRoutesDirectlyForASoleClaimant(t *testing.T) {
	local := netip.MustParseAddr("127.0.0.1")
	group := attrPeer("0.0.0.0", 0, ConnectionPassive)
	group.IsDynamic = true

	tests := []struct {
		name   string
		peers  []*PeerSettings
		groups []*PeerSettings
		want   netip.AddrPort
	}{
		{
			name:  "sole custom-port peer",
			peers: []*PeerSettings{attrPeer("127.0.0.2", attrTestPort, ConnectionPassive)},
			want:  netip.MustParseAddrPort("127.0.0.2:1790"),
		},
		{
			name:  "sole peer on the default port shares the listener with every other one",
			peers: []*PeerSettings{attrPeer("127.0.0.2", DefaultBGPPort, ConnectionPassive)},
			want:  netip.AddrPort{},
		},
		{
			name: "two peers on one socket",
			peers: []*PeerSettings{
				attrPeer("127.0.0.2", attrTestPort, ConnectionPassive),
				attrPeer("127.0.0.3", attrTestPort, ConnectionPassive),
			},
			want: netip.AddrPort{},
		},
		{
			name:   "a dynamic group accepts on the same socket",
			peers:  []*PeerSettings{attrPeer("127.0.0.2", attrTestPort, ConnectionPassive)},
			groups: []*PeerSettings{group},
			want:   netip.AddrPort{},
		},
		{
			name:   "the group is the only claimant",
			groups: []*PeerSettings{group},
			want:   netip.AddrPort{},
		},
		{
			name:  "an active-only peer claims no listener",
			peers: []*PeerSettings{attrPeer("127.0.0.2", attrTestPort, ConnectionActive)},
			want:  netip.AddrPort{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := attrReactor(t, tt.peers, tt.groups)
			assert.Equal(t, tt.want, r.listenerPeerKey(local, attrTestPort))
		})
	}
}

// TestListenerClaimantsCountsGroupsSoTheSocketOutlivesTheLastPeer pins the count
// doRemovePeer reads before it closes a listening socket.
//
// VALIDATES: listenerClaimants counts a dynamic group, so removing the last static
//
//	peer on an address leaves a group still accepting there.
//
// PREVENTS: a route server going deaf on the reload that removes its last
//
//	bilateral peer. No peer entry represents a group, so a count over
//	r.peers alone reads zero while the group is still configured.
func TestListenerClaimantsCountsGroupsSoTheSocketOutlivesTheLastPeer(t *testing.T) {
	local := netip.MustParseAddr("127.0.0.1")
	group := attrPeer("0.0.0.0", 0, ConnectionPassive)
	group.IsDynamic = true

	r := attrReactor(t, nil, []*PeerSettings{group})
	count, only := r.listenerClaimants(local, attrTestPort)
	assert.Equal(t, 1, count, "the group claims the listener on its own")
	assert.Nil(t, only, "a group is never the peer a direct route attributes to")

	elsewhere := netip.MustParseAddr("127.0.0.9")
	count, _ = r.listenerClaimants(elsewhere, attrTestPort)
	assert.Equal(t, 0, count, "a group claims only the address it accepts on")
}
