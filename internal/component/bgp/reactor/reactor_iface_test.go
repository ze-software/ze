package reactor

import (
	"context"
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newIfaceTestReactor creates a reactor with a context set for handler tests.
func newIfaceTestReactor(t *testing.T) *Reactor {
	t.Helper()
	r := New(&Config{Port: 0})
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

// makeAddrPayload constructs the JSON payload string the EventBus delivery
// path produces. Tests use this to drive the address-handler functions
// directly without going through the full subscribe/emit cycle.
func makeAddrPayload(address string, unit int) interfaceAddrPayload {
	return interfaceAddrPayload{
		Name:         "eth0",
		Unit:         unit,
		Index:        5,
		Address:      address,
		PrefixLength: 24,
		Family:       "ipv4",
	}
}

// makeAddrPayloadJSON marshals an interfaceAddrPayload to the on-the-wire
// JSON string used by the (interface, addr-*) event types.
//
// The unit is always 0: no handler on the JSON path reads it, and a test that
// needs a specific unit builds the payload with makeAddrPayload directly.
func makeAddrPayloadJSON(t *testing.T, address string) string {
	t.Helper()
	data, err := json.Marshal(makeAddrPayload(address, 0))
	if err != nil {
		t.Fatalf("marshal addr payload: %v", err)
	}
	return string(data)
}

func TestBGPAddrAddedReaction(t *testing.T) {
	// VALIDATES: AC-4 - addr-added for a peer's LocalAddress starts a listener.
	// PREVENTS: BGP ignoring interface events, never binding to addresses.
	r := newIfaceTestReactor(t)

	s := NewPeerSettings(netip.MustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	s.LocalAddress = netip.MustParseAddr("127.0.0.1")
	r.peers[s.PeerKey()] = NewPeer(s)

	r.handleAddrAddedPayload(makeAddrPayload("127.0.0.1", 0))

	r.mu.RLock()
	listenerCount := len(r.listeners)
	r.mu.RUnlock()

	if listenerCount == 0 {
		t.Error("expected listener to be started for matching address, got 0 listeners")
	}
}

func TestBGPAddrAddedNoMatch(t *testing.T) {
	// VALIDATES: addr-added for non-matching address does not start a listener.
	// PREVENTS: Listeners created for irrelevant addresses.
	r := newIfaceTestReactor(t)

	s := NewPeerSettings(netip.MustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	s.LocalAddress = netip.MustParseAddr("10.0.0.1")
	r.peers[s.PeerKey()] = NewPeer(s)

	r.handleAddrAddedPayload(makeAddrPayload("192.168.1.1", 0))

	r.mu.RLock()
	listenerCount := len(r.listeners)
	r.mu.RUnlock()

	if listenerCount != 0 {
		t.Errorf("expected 0 listeners for non-matching address, got %d", listenerCount)
	}
}

func TestBGPAddrRemovedReaction(t *testing.T) {
	// VALIDATES: AC-5 - addr-removed stops the listener for that address.
	// PREVENTS: Stale listeners after interface address removal.
	r := newIfaceTestReactor(t)

	s := NewPeerSettings(netip.MustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	s.LocalAddress = netip.MustParseAddr("127.0.0.1")
	r.peers[s.PeerKey()] = NewPeer(s)

	// Add address to create a listener.
	r.handleAddrAddedPayload(makeAddrPayload("127.0.0.1", 0))

	r.mu.RLock()
	hasListener := len(r.listeners) > 0
	r.mu.RUnlock()
	if !hasListener {
		t.Fatal("prerequisite: listener should exist after addr added")
	}

	// Remove the address.
	r.handleAddrRemovedPayload(makeAddrPayload("127.0.0.1", 0))

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

	r.handleAddrAddedPayload(makeAddrPayload("127.0.0.1", 100))

	r.mu.RLock()
	listenerCount := len(r.listeners)
	r.mu.RUnlock()

	if listenerCount != 1 {
		t.Errorf("expected exactly 1 shared listener, got %d", listenerCount)
	}
}

func TestOnInterfaceAddrJSONDispatch(t *testing.T) {
	// VALIDATES: onInterfaceAddrAdded/Removed parse the EventBus JSON payload
	// and dispatch correctly. Malformed payloads do not panic.
	// PREVENTS: Events silently dropped or panicking the engine on bad input.
	r := newIfaceTestReactor(t)

	s := NewPeerSettings(netip.MustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	s.LocalAddress = netip.MustParseAddr("127.0.0.1")
	r.peers[s.PeerKey()] = NewPeer(s)

	// Valid JSON: triggers the listener startup path.
	r.onInterfaceAddrAdded(makeAddrPayloadJSON(t, "127.0.0.1"))
	r.mu.RLock()
	hasListener := len(r.listeners) > 0
	r.mu.RUnlock()
	if !hasListener {
		t.Fatal("expected listener after valid addr-added payload")
	}

	// Malformed JSON: must not panic.
	r.onInterfaceAddrAdded("not json")
	r.onInterfaceAddrRemoved("not json either")

	// Valid removal: undoes the listener.
	r.onInterfaceAddrRemoved(makeAddrPayloadJSON(t, "127.0.0.1"))
	r.mu.RLock()
	listenerCount := len(r.listeners)
	r.mu.RUnlock()
	if listenerCount != 0 {
		t.Errorf("expected 0 listeners after addr-removed, got %d", listenerCount)
	}
}

// TestInterfaceAddrEventRefreshesEveryPeerLinkScope drives RFC 2545 Section 3's
// interface-table snapshot from the EventBus entry point.
//
// RFC 2545 Section 3: "The link-local address shall be included in the Next Hop
// field if and only if the BGP speaker shares a common subnet with the entity
// identified by the global IPv6 address carried in the Network Address of Next
// Hop field and the peer the route is being advertised to." That condition is
// answered from a snapshot taken at session establishment. An address added to or
// removed from an interface OTHER than the session's moves the answer while the
// TCP session survives, so the snapshot has to be re-settled for EVERY peer, not
// only the one bound to the address in the event.
//
// VALIDATES: an addr-added and an addr-removed event for 192.168.1.1, an address
// no peer here is bound to, rebuild the forwarding snapshot of a peer whose local
// address is ::1, taking its next-hop wire form from the 16-octet form back to the
// 32-octet one.
// PREVENTS: a surviving session keeping a stale answer, which appends a link-local
// Section 3 now forbids or omits one it now requires.
func TestInterfaceAddrEventRefreshesEveryPeerLinkScope(t *testing.T) {
	for _, tc := range []struct {
		name string
		fire func(r *Reactor, payload string)
	}{
		{"addr-added", func(r *Reactor, payload string) { r.onInterfaceAddrAdded(payload) }},
		{"addr-removed", func(r *Reactor, payload string) { r.onInterfaceAddrRemoved(payload) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newIfaceTestReactor(t)

			s := NewPeerSettings(netip.MustParseAddr("::1"), 65000, 65000, 0x01010101)
			s.LocalAddress = netip.MustParseAddr("::1")
			s.LinkLocal = netip.MustParseAddr("fe80::1")
			s.NextHopMode = NextHopSelf
			peer := NewPeer(s)
			r.peers[s.PeerKey()] = peer

			// A snapshot that has read no interface table: Section 3's condition is
			// unproven, so nothing is appended and the field is 16 octets.
			peer.llScope.Store(&linkScope{})
			peer.fwdFacts.Store(peer.buildForwardFacts())
			require.Equal(t, nhModeSelfV6, peer.forwardFacts().nhMode,
				"prerequisite: the stale snapshot answers Section 3 negatively")

			// The event names an address no peer here is bound to, so the listener
			// path skips every peer.
			tc.fire(r, makeAddrPayloadJSON(t, "192.168.1.1"))

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
// VALIDATES: a peer with no forwarding snapshot still has none after the event.
// PREVENTS: an interface event resurrecting the snapshot of a peer with no
// session, which the forwarding rails read as "established".
func TestInterfaceAddrEventLeavesDownPeerWithoutSnapshot(t *testing.T) {
	r := newIfaceTestReactor(t)

	s := NewPeerSettings(netip.MustParseAddr("::1"), 65000, 65000, 0x01010101)
	s.LocalAddress = netip.MustParseAddr("::1")
	peer := NewPeer(s)
	r.peers[s.PeerKey()] = peer
	require.Nil(t, peer.forwardFacts(), "prerequisite: no session, no snapshot")

	r.onInterfaceAddrAdded(makeAddrPayloadJSON(t, "192.168.1.1"))

	assert.Nil(t, peer.forwardFacts(), "a down peer must not gain a snapshot")
}
