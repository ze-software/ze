package reactor

import (
	"context"
	"encoding/json"
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
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

func makeAddrEvent(topic, address string, unit int) ze.Event {
	payload := iface.AddrPayload{
		Name:         "eth0",
		Unit:         unit,
		Index:        5,
		Address:      address,
		PrefixLength: 24,
		Family:       "ipv4",
	}
	data, _ := json.Marshal(payload)
	return ze.Event{
		Topic:    topic,
		Payload:  data,
		Metadata: map[string]string{"name": "eth0", "address": address},
	}
}

func TestBGPAddrAddedReaction(t *testing.T) {
	// VALIDATES: AC-4 - addr/added for a peer's LocalAddress starts a listener.
	// PREVENTS: BGP ignoring interface events, never binding to addresses.
	r := newIfaceTestReactor(t)

	s := NewPeerSettings(netip.MustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	s.LocalAddress = netip.MustParseAddr("127.0.0.1")
	r.peers[s.PeerKey()] = NewPeer(s)

	ev := makeAddrEvent(iface.TopicAddrAdded, "127.0.0.1", 0)
	r.handleAddrAdded(ev)

	r.mu.RLock()
	listenerCount := len(r.listeners)
	r.mu.RUnlock()

	if listenerCount == 0 {
		t.Error("expected listener to be started for matching address, got 0 listeners")
	}
}

func TestBGPAddrAddedNoMatch(t *testing.T) {
	// VALIDATES: addr/added for non-matching address does not start a listener.
	// PREVENTS: Listeners created for irrelevant addresses.
	r := newIfaceTestReactor(t)

	s := NewPeerSettings(netip.MustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	s.LocalAddress = netip.MustParseAddr("10.0.0.1")
	r.peers[s.PeerKey()] = NewPeer(s)

	ev := makeAddrEvent(iface.TopicAddrAdded, "192.168.1.1", 0)
	r.handleAddrAdded(ev)

	r.mu.RLock()
	listenerCount := len(r.listeners)
	r.mu.RUnlock()

	if listenerCount != 0 {
		t.Errorf("expected 0 listeners for non-matching address, got %d", listenerCount)
	}
}

func TestBGPAddrRemovedReaction(t *testing.T) {
	// VALIDATES: AC-5 - addr/removed stops the listener for that address.
	// PREVENTS: Stale listeners after interface address removal.
	r := newIfaceTestReactor(t)

	s := NewPeerSettings(netip.MustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	s.LocalAddress = netip.MustParseAddr("127.0.0.1")
	r.peers[s.PeerKey()] = NewPeer(s)

	// Add address to create a listener.
	r.handleAddrAdded(makeAddrEvent(iface.TopicAddrAdded, "127.0.0.1", 0))

	r.mu.RLock()
	hasListener := len(r.listeners) > 0
	r.mu.RUnlock()
	if !hasListener {
		t.Fatal("prerequisite: listener should exist after addr added")
	}

	// Remove the address.
	r.handleAddrRemoved(makeAddrEvent(iface.TopicAddrRemoved, "127.0.0.1", 0))

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

	ev := makeAddrEvent(iface.TopicAddrAdded, "127.0.0.1", 100)
	r.handleAddrAdded(ev)

	r.mu.RLock()
	listenerCount := len(r.listeners)
	r.mu.RUnlock()

	if listenerCount != 1 {
		t.Errorf("expected exactly 1 shared listener, got %d", listenerCount)
	}
}

func TestHandleInterfaceEventDispatch(t *testing.T) {
	// VALIDATES: handleInterfaceEvent routes events correctly and handles errors.
	// PREVENTS: Events silently dropped or panic on bad input.
	r := newIfaceTestReactor(t)

	// addr/added with no matching peers: no-op, no panic.
	r.handleInterfaceEvent(makeAddrEvent(iface.TopicAddrAdded, "10.0.0.99", 0))

	// created event: no-op (future work), no panic.
	r.handleInterfaceEvent(ze.Event{Topic: iface.TopicCreated, Payload: []byte(`{}`)})

	// Malformed payload: no panic.
	r.handleInterfaceEvent(ze.Event{Topic: iface.TopicAddrAdded, Payload: []byte(`not json`)})
}
