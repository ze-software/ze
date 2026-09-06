// RFC: rfc/short/rfc4271.md — Section 6.8, the inbound connection an accept must attribute
package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

const attrTestPort = 1790

// attrReactor builds a reactor holding the given passive peers and dynamic
// groups, all bound to local, with configPort as the daemon's own listen port.
func attrReactor(t *testing.T, configPort int, peers, groups []*PeerSettings) *Reactor {
	t.Helper()
	r := &Reactor{
		config: &Config{Port: configPort},
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

// attrPeer builds one passive-or-active peer bound to 127.0.0.1. localPort is
// connection > local > port, the port a listener of its own answers on, and
// remotePort is connection > remote > port, the port Ze dials. Zero leaves each
// at its default, which is the whole point of the pair: the two endpoints hold
// different numbers and only the first one decides a listener.
func attrPeer(addr string, localPort, remotePort uint16, mode ConnectionMode) *PeerSettings {
	s := NewPeerSettings(netip.MustParseAddr(addr), 65533, 65010, 0x01020304)
	s.LocalAddress = netip.MustParseAddr("127.0.0.1")
	s.LocalPort = localPort
	if remotePort != 0 {
		s.Port = remotePort
	}
	s.Connection = mode
	return s
}

// TestListenerPeerKeyRoutesDirectlyForASoleClaimant covers the one shape in which
// a listener attributes by its own identity instead of by the connection's
// source address.
//
// VALIDATES: listenerPeerKey returns a peer key only when that peer is the single
//
//	claimant of the (local address, port) pair and that pair is not the
//	well-known BGP port. The claim is decided by the LISTEN port, and the
//	key it returns carries the peer's remote port, because that is what
//	the peer map is keyed on.
//
// PREVENTS: handleDirectConnection serving a connection under the policy of a peer
//
//	it was not opened for. It reads no remote address, so a socket a
//	second peer or a dynamic group also accepts on cannot be attributed
//	that way at all. It also prevents the port Ze DIALS deciding which
//	socket a peer claims, which is what one merged field did until
//	2026-09-06.
func TestListenerPeerKeyRoutesDirectlyForASoleClaimant(t *testing.T) {
	local := netip.MustParseAddr("127.0.0.1")

	dynGroup := func(localPort uint16) *PeerSettings {
		g := attrPeer("0.0.0.0", localPort, 0, ConnectionPassive)
		g.IsDynamic = true
		return g
	}

	tests := []struct {
		name       string
		configPort int
		listenPort int
		peers      []*PeerSettings
		groups     []*PeerSettings
		want       netip.AddrPort
	}{
		{
			name:       "sole custom-port peer",
			listenPort: attrTestPort,
			peers:      []*PeerSettings{attrPeer("127.0.0.2", attrTestPort, 2790, ConnectionPassive)},
			want:       netip.MustParseAddrPort("127.0.0.2:2790"),
		},
		{
			name:       "the daemon's own port carries a sole claimant",
			configPort: attrTestPort,
			listenPort: attrTestPort,
			peers:      []*PeerSettings{attrPeer("127.0.0.2", 0, 0, ConnectionPassive)},
			want:       netip.MustParseAddrPort("127.0.0.2:179"),
		},
		{
			name:       "a peer on the default-port listener shares it with every other one",
			listenPort: DefaultBGPPort,
			peers:      []*PeerSettings{attrPeer("127.0.0.2", 0, 0, ConnectionPassive)},
			want:       netip.AddrPort{},
		},
		{
			name:       "a remote port of its own claims no listener",
			listenPort: attrTestPort,
			peers:      []*PeerSettings{attrPeer("127.0.0.2", 0, attrTestPort, ConnectionPassive)},
			want:       netip.AddrPort{},
		},
		{
			name:       "two peers on one socket",
			listenPort: attrTestPort,
			peers: []*PeerSettings{
				attrPeer("127.0.0.2", attrTestPort, 0, ConnectionPassive),
				attrPeer("127.0.0.3", attrTestPort, 0, ConnectionPassive),
			},
			want: netip.AddrPort{},
		},
		{
			name:       "a dynamic group accepts on the same socket",
			listenPort: attrTestPort,
			peers:      []*PeerSettings{attrPeer("127.0.0.2", attrTestPort, 0, ConnectionPassive)},
			groups:     []*PeerSettings{dynGroup(attrTestPort)},
			want:       netip.AddrPort{},
		},
		{
			name:       "the group is the only claimant",
			listenPort: attrTestPort,
			groups:     []*PeerSettings{dynGroup(attrTestPort)},
			want:       netip.AddrPort{},
		},
		{
			name:       "an active-only peer claims no listener",
			listenPort: attrTestPort,
			peers:      []*PeerSettings{attrPeer("127.0.0.2", attrTestPort, 0, ConnectionActive)},
			want:       netip.AddrPort{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := attrReactor(t, tt.configPort, tt.peers, tt.groups)
			assert.Equal(t, tt.want, r.listenerPeerKey(local, tt.listenPort))
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
	group := attrPeer("0.0.0.0", attrTestPort, 0, ConnectionPassive)
	group.IsDynamic = true

	r := attrReactor(t, 0, nil, []*PeerSettings{group})
	count, only := r.listenerClaimants(local, attrTestPort)
	assert.Equal(t, 1, count, "the group claims the listener on its own")
	assert.Nil(t, only, "a group is never the peer a direct route attributes to")

	elsewhere := netip.MustParseAddr("127.0.0.9")
	count, _ = r.listenerClaimants(elsewhere, attrTestPort)
	assert.Equal(t, 0, count, "a group claims only the address it accepts on")
}
