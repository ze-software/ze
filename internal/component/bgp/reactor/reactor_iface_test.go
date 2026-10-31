package reactor

import (
	"context"
	"net"
	"net/netip"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
)

// newIfaceTestReactor creates a reactor with a context set for handler tests.
//
// The daemon port is a free one rather than 0: a listener opens at the port
// peerListenPort answers for its peer, and that answer for a daemon port of 0
// is the default 179, which a test cannot bind (listenPortFor).
func newIfaceTestReactor(t *testing.T) *Reactor {
	t.Helper()
	r := New(&Config{Port: freePort(t)})
	ctx, cancel := context.WithCancel(context.Background())
	r.ctx = ctx
	t.Cleanup(func() {
		r.mu.Lock()
		r.stopAllListeners()
		r.mu.Unlock()
		cancel()
	})
	return r
}

func TestBGPAddrAddedReaction(t *testing.T) {
	// VALIDATES: AC-4 - an added address that is a peer's LocalAddress starts a listener.
	// PREVENTS: BGP ignoring interface events, never binding to addresses.
	r := newIfaceTestReactor(t)

	s := NewPeerSettings(netip.MustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	s.LocalAddress = netip.MustParseAddr("127.0.0.1")
	r.peers[s.PeerKey()] = NewPeer(s)

	r.handleAddressAdded(netip.MustParseAddr("127.0.0.1"))

	r.mu.RLock()
	listenerCount := len(r.listeners)
	r.mu.RUnlock()

	if listenerCount == 0 {
		t.Error("expected listener to be started for matching address, got 0 listeners")
	}
}

func TestBGPAddrAddedNoMatch(t *testing.T) {
	// VALIDATES: an added address no peer is bound to does not start a listener.
	// PREVENTS: Listeners created for irrelevant addresses.
	r := newIfaceTestReactor(t)

	s := NewPeerSettings(netip.MustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	s.LocalAddress = netip.MustParseAddr("10.0.0.1")
	r.peers[s.PeerKey()] = NewPeer(s)

	r.handleAddressAdded(netip.MustParseAddr("192.168.1.1"))

	r.mu.RLock()
	listenerCount := len(r.listeners)
	r.mu.RUnlock()

	if listenerCount != 0 {
		t.Errorf("expected 0 listeners for non-matching address, got %d", listenerCount)
	}
}

func TestBGPAddrRemovedReaction(t *testing.T) {
	// VALIDATES: AC-5 - a removed address stops the listener for that address.
	// PREVENTS: Stale listeners after interface address removal.
	r := newIfaceTestReactor(t)

	s := NewPeerSettings(netip.MustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	s.LocalAddress = netip.MustParseAddr("127.0.0.1")
	r.peers[s.PeerKey()] = NewPeer(s)

	// Add address to create a listener.
	r.handleAddressAdded(netip.MustParseAddr("127.0.0.1"))

	r.mu.RLock()
	hasListener := len(r.listeners) > 0
	r.mu.RUnlock()
	if !hasListener {
		t.Fatal("prerequisite: listener should exist after addr added")
	}

	// Remove the address.
	r.handleAddressRemoved(netip.MustParseAddr("127.0.0.1"))

	r.mu.RLock()
	listenerCount := len(r.listeners)
	r.mu.RUnlock()

	if listenerCount != 0 {
		t.Errorf("expected 0 listeners after addr removed, got %d", listenerCount)
	}
}

func TestBGPSharedListener(t *testing.T) {
	// VALIDATES: AC-14 - Multiple peers with same LocalAddress share one listener.
	// PREVENTS: Duplicate listeners for shared addresses.
	r := newIfaceTestReactor(t)

	localAddr := netip.MustParseAddr("127.0.0.1")

	s1 := NewPeerSettings(netip.MustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	s1.LocalAddress = localAddr
	r.peers[s1.PeerKey()] = NewPeer(s1)

	s2 := NewPeerSettings(netip.MustParseAddr("10.0.0.3"), 65000, 65002, 0x01010101)
	s2.LocalAddress = localAddr
	r.peers[s2.PeerKey()] = NewPeer(s2)

	r.handleAddressAdded(netip.MustParseAddr("127.0.0.1"))

	r.mu.RLock()
	listenerCount := len(r.listeners)
	r.mu.RUnlock()

	if listenerCount != 1 {
		t.Errorf("expected exactly 1 shared listener, got %d", listenerCount)
	}
}

// listeningAt reports whether r holds a listener on addr:port.
func listeningAt(r *Reactor, addr netip.Addr, port int) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.listeners[net.JoinHostPort(addr.String(), strconv.Itoa(port))]
	return ok
}

// ownPortPeer returns a passive peer bound to local that listens on a port of
// its own (connection > local > port), distinct from r's daemon port.
func ownPortPeer(t *testing.T, r *Reactor, local netip.Addr) (*PeerSettings, int) {
	t.Helper()
	own := freePort(t)
	require.NotEqual(t, r.config.Port, own, "the test needs the peer's port to differ from the daemon's")
	s := NewPeerSettings(netip.MustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	s.LocalAddress = local
	s.LocalPort = uint16(own) //nolint:gosec // freePort answers a TCP port, which fits a uint16
	s.Connection = ConnectionPassive
	return s, own
}

// TestAddressAddedListensOnThePeersOwnPort proves an added address opens the
// listener of a peer that listens on a port of its own at that port, the one
// the start opens it on (peerListenPort), and not at the daemon's port.
//
// MUTATION: open the listener at r.config.Port in handleAddressAdded: the
// listener lands on the daemon's port and the peer's own port has none.
func TestAddressAddedListensOnThePeersOwnPort(t *testing.T) {
	r := newIfaceTestReactor(t)
	local := netip.MustParseAddr("127.0.0.1")
	s, own := ownPortPeer(t, r, local)
	r.peers[s.PeerKey()] = NewPeer(s)

	require.True(t, r.handleAddressAdded(local), "a bindable address is not retried")

	assert.True(t, listeningAt(r, local, own), "the peer's own port got its listener")
	assert.False(t, listeningAt(r, local, r.config.Port), "the daemon's port got no listener nothing asked for")
}

// TestAddressRemovedStopsThePeersOwnPortListener proves a removed address
// stops the listener the start opened for a peer on a port of its own: the
// listener leaves the reactor and its socket stops accepting.
//
// The listener is opened by the start path (startMultiListeners), so the test
// holds whatever the add handler does and judges the removal alone.
//
// MUTATION: look the listener up at r.config.Port in handleAddressRemoved: no
// listener is found there, and the own-port one keeps accepting.
func TestAddressRemovedStopsThePeersOwnPortListener(t *testing.T) {
	r := newIfaceTestReactor(t)
	local := netip.MustParseAddr("127.0.0.1")
	s, own := ownPortPeer(t, r, local)
	r.peers[s.PeerKey()] = NewPeer(s)

	r.mu.Lock()
	startErr := r.startMultiListeners()
	r.mu.Unlock()
	require.NoError(t, startErr)
	require.True(t, listeningAt(r, local, own), "prerequisite: the start opened the peer's own-port listener")

	r.handleAddressRemoved(local)

	assert.False(t, listeningAt(r, local, own), "the removed address lost its listener")
	target := net.JoinHostPort(local.String(), strconv.Itoa(own))
	var dialer net.Dialer
	dialCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, dialErr := dialer.DialContext(dialCtx, "tcp", target)
	if dialErr == nil {
		_ = conn.Close() //nolint:errcheck // the test only learns that the dial succeeded
	}
	assert.Error(t, dialErr, "the stopped listener's socket still accepts on %s", target)
}

// TestAddressAddedOpensNoListenerForAnActiveOnlyPeer proves an added address
// opens nothing for a peer that only dials out, as the start opens nothing for
// it (listenerSpecs reads Connection.IsPassive).
//
// MUTATION: count every peer bound to the address in handleAddressAdded,
// passive or not: the active-only peer gets a listener.
func TestAddressAddedOpensNoListenerForAnActiveOnlyPeer(t *testing.T) {
	r := newIfaceTestReactor(t)
	local := netip.MustParseAddr("127.0.0.1")
	s := NewPeerSettings(netip.MustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	s.LocalAddress = local
	s.Connection = ConnectionActive
	r.peers[s.PeerKey()] = NewPeer(s)

	r.handleAddressAdded(local)

	r.mu.RLock()
	listenerCount := len(r.listeners)
	r.mu.RUnlock()
	assert.Equal(t, 0, listenerCount, "a peer that only dials out accepts nothing, so it gets no listener")
}

// TestAddressAddedReadiesAnAddressOnlyActivePeersUse proves an added address
// that only peers dialling out are bound to emits (bgp, listener-ready), with
// no listener. It owes none, so it is ready as soon as it appears, and
// MigrateInterface (iface/migrate_linux.go) waits for that event before it
// removes the old address.
//
// MUTATION: return before the emit when listenPortsAt answers no port: no
// event goes out, and a migration of the address times out and rolls back.
func TestAddressAddedReadiesAnAddressOnlyActivePeersUse(t *testing.T) {
	r := newIfaceTestReactor(t)
	bus := &operationEventBus{}
	r.SetEventBus(bus)
	local := netip.MustParseAddr("127.0.0.1")
	s := NewPeerSettings(netip.MustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	s.LocalAddress = local
	s.Connection = ConnectionActive
	r.peers[s.PeerKey()] = NewPeer(s)

	require.True(t, r.handleAddressAdded(local), "an address owing no listener is not retried")

	r.mu.RLock()
	listenerCount := len(r.listeners)
	r.mu.RUnlock()
	assert.Equal(t, 0, listenerCount, "a peer that only dials out gets no listener")
	require.Len(t, bus.events, 1, "the address is ready, so listener-ready goes out once")
	assert.Equal(t, bgpevents.EventListenerReady, bus.events[0].EventType)
	payload, ok := bus.events[0].Payload.(string)
	require.True(t, ok, "the payload is the JSON string handleAddressAdded marshals")
	assert.JSONEq(t, `{"address":"127.0.0.1"}`, payload)
}

// TestAddressRemovedStopsAListenerNoClaimantNeeds proves a removed address
// stops a listener on it that no peer or group claims any more, at a port that
// is not the daemon's. A reload leaves such a listener in place
// (installDynamicGroupsLocked leaves a removed group's socket open, because it
// may be shared with a peer and closing it is the peer-removal path's
// decision), and nothing can accept on an address the host no longer holds.
//
// MUTATION: stop only the ports listenPortsAt answers for the address: no
// claimant names this one, so it keeps accepting.
func TestAddressRemovedStopsAListenerNoClaimantNeeds(t *testing.T) {
	r := newIfaceTestReactor(t)
	local := netip.MustParseAddr("127.0.0.1")
	leftover := freePort(t)
	require.NotEqual(t, r.config.Port, leftover, "the test needs the leftover's port to differ from the daemon's")
	r.mu.Lock()
	startErr := r.startListenerForAddressPort(local, leftover)
	r.mu.Unlock()
	require.NoError(t, startErr)
	require.True(t, listeningAt(r, local, leftover), "prerequisite: the leftover listener is up")

	r.handleAddressRemoved(local)

	assert.False(t, listeningAt(r, local, leftover), "the removed address lost the listener no claimant needs")
}

// TestInterfaceAddrEventRefreshesEveryPeerLinkScope drives RFC 2545 Section 3's
// interface-table snapshot from the address watch's entry point
// (addressSync.apply, which each watch signal runs).
//
// RFC 2545 Section 3: "The link-local address shall be included in the Next Hop
// field if and only if the BGP speaker shares a common subnet with the entity
// identified by the global IPv6 address carried in the Network Address of Next
// Hop field and the peer the route is being advertised to." That condition is
// answered from a snapshot taken at session establishment. An address added to or
// removed from an interface OTHER than the session's moves the answer while the
// TCP session survives, so the snapshot has to be re-settled for EVERY peer, not
// only the one bound to the address that moved.
//
// VALIDATES: an added and a removed 192.168.1.1, an address no peer here is
// bound to, rebuild the forwarding snapshot of a peer whose local address is
// ::1, taking its next-hop wire form from the 16-octet form back to the
// 32-octet one.
// PREVENTS: a surviving session keeping a stale answer, which appends a link-local
// Section 3 now forbids or omits one it now requires.
func TestInterfaceAddrEventRefreshesEveryPeerLinkScope(t *testing.T) {
	other := netip.MustParsePrefix("192.168.1.1/24")
	for _, tc := range []struct {
		name          string
		before, after []netip.Prefix
	}{
		{"addr-added", nil, []netip.Prefix{other}},
		{"addr-removed", []netip.Prefix{other}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			table := &fakeAddressTable{prefixes: tc.before}
			table.install(t)
			r := newIfaceTestReactor(t)

			s := NewPeerSettings(netip.MustParseAddr("::1"), 65000, 65000, 0x01010101)
			s.LocalAddress = netip.MustParseAddr("::1")
			s.LinkLocal = netip.MustParseAddr("fe80::1")
			s.NextHopMode = NextHopSelf
			peer := NewPeer(s)
			r.peers[s.PeerKey()] = peer

			sync := &addressSync{r: r}
			sync.apply() // the first read records the table

			// A snapshot that has read no interface table: Section 3's condition is
			// unproven, so nothing is appended and the field is 16 octets.
			peer.llScope.Store(&linkScope{})
			peer.fwdFacts.Store(peer.buildForwardFacts())
			require.Equal(t, nhModeSelfV6, peer.forwardFacts().nhMode,
				"prerequisite: the stale snapshot answers Section 3 negatively")

			// The change names an address no peer here is bound to, so the
			// listener path skips every peer.
			table.set(tc.after)
			sync.apply()

			assert.Equal(t, nhModeSelfV6LL, peer.forwardFacts().nhMode,
				"the re-read table puts ::1 and the peer on the loopback subnet, so Section 3 includes the link-local")
			ll := netip.MustParseAddr("fe80::1").As16()
			assert.Equal(t, ll[:], peer.forwardFacts().nhGlobalLL[16:],
				"RFC 2545 Section 3: the link-local address is the SECOND of the two")
		})
	}
}

// TestInterfaceAddrEventLeavesDownPeerWithoutSnapshot verifies the fan-out keeps
// the down-peer gate.
//
// VALIDATES: a peer with no forwarding snapshot still has none after an address
// change.
// PREVENTS: an address change resurrecting the snapshot of a peer with no
// session, which the forwarding rails read as "established".
func TestInterfaceAddrEventLeavesDownPeerWithoutSnapshot(t *testing.T) {
	r := newIfaceTestReactor(t)

	s := NewPeerSettings(netip.MustParseAddr("::1"), 65000, 65000, 0x01010101)
	s.LocalAddress = netip.MustParseAddr("::1")
	peer := NewPeer(s)
	r.peers[s.PeerKey()] = peer
	require.Nil(t, peer.forwardFacts(), "prerequisite: no session, no snapshot")

	table := &fakeAddressTable{}
	table.install(t)
	sync := &addressSync{r: r}
	sync.apply()
	table.set([]netip.Prefix{netip.MustParsePrefix("192.168.1.1/24")})
	sync.apply()

	assert.Nil(t, peer.forwardFacts(), "a down peer must not gain a snapshot")
}
