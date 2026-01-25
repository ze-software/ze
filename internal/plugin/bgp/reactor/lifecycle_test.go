package reactor

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// mockLifecycleObserver records callback invocations for testing.
type mockLifecycleObserver struct {
	mu              sync.Mutex
	established     []*Peer
	closed          []closedEvent
	establishedFunc func(*Peer)
	closedFunc      func(*Peer, string)
}

type closedEvent struct {
	peer   *Peer
	reason string
}

func (m *mockLifecycleObserver) OnPeerEstablished(peer *Peer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.established = append(m.established, peer)
	if m.establishedFunc != nil {
		m.establishedFunc(peer)
	}
}

func (m *mockLifecycleObserver) OnPeerClosed(peer *Peer, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = append(m.closed, closedEvent{peer: peer, reason: reason})
	if m.closedFunc != nil {
		m.closedFunc(peer, reason)
	}
}

func (m *mockLifecycleObserver) establishedCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.established)
}

func (m *mockLifecycleObserver) closedCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.closed)
}

func (m *mockLifecycleObserver) lastClosedReason() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.closed) == 0 {
		return ""
	}
	return m.closed[len(m.closed)-1].reason
}

// TestPeerLifecycleObserverRegistration verifies observers can be registered.
//
// VALIDATES: AddPeerObserver adds observer to reactor.
// PREVENTS: Observer registration failures.
func TestPeerLifecycleObserverRegistration(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)
	obs := &mockLifecycleObserver{}

	// Should not panic
	reactor.AddPeerObserver(obs)
}

// TestPeerLifecycleCallbacks verifies OnPeerEstablished/OnPeerClosed are called.
//
// VALIDATES: Observer callbacks are invoked on state transitions.
// PREVENTS: Missing state notifications to plugins/API.
func TestPeerLifecycleCallbacks(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)
	obs := &mockLifecycleObserver{}
	reactor.AddPeerObserver(obs)

	// Create a peer
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	peer := NewPeer(settings)
	peer.SetReactor(reactor)

	// Simulate established transition
	reactor.notifyPeerEstablished(peer)
	require.Equal(t, 1, obs.establishedCount(), "OnPeerEstablished should be called")

	// Simulate closed transition
	reactor.notifyPeerClosed(peer, "session closed")
	require.Equal(t, 1, obs.closedCount(), "OnPeerClosed should be called")
}

// TestPeerLifecycleCallbackOrder verifies observers are called in registration order.
//
// VALIDATES: Observers called in registration order.
// PREVENTS: Non-deterministic callback ordering.
func TestPeerLifecycleCallbackOrder(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)

	var callOrder []int
	var mu sync.Mutex

	obs1 := &mockLifecycleObserver{
		establishedFunc: func(_ *Peer) {
			mu.Lock()
			callOrder = append(callOrder, 1)
			mu.Unlock()
		},
	}
	obs2 := &mockLifecycleObserver{
		establishedFunc: func(_ *Peer) {
			mu.Lock()
			callOrder = append(callOrder, 2)
			mu.Unlock()
		},
	}
	obs3 := &mockLifecycleObserver{
		establishedFunc: func(_ *Peer) {
			mu.Lock()
			callOrder = append(callOrder, 3)
			mu.Unlock()
		},
	}

	// Register in order
	reactor.AddPeerObserver(obs1)
	reactor.AddPeerObserver(obs2)
	reactor.AddPeerObserver(obs3)

	// Create a peer and trigger callback
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	peer := NewPeer(settings)

	reactor.notifyPeerEstablished(peer)

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []int{1, 2, 3}, callOrder, "observers must be called in registration order")
}

// TestPeerClosedReason verifies correct reason is passed to OnPeerClosed.
//
// VALIDATES: Correct reason passed to OnPeerClosed.
// PREVENTS: Misleading close reasons in logs/API.
func TestPeerClosedReason(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)
	obs := &mockLifecycleObserver{}
	reactor.AddPeerObserver(obs)

	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	peer := NewPeer(settings)

	// Test various reasons
	reactor.notifyPeerClosed(peer, "connection lost")
	require.Equal(t, "connection lost", obs.lastClosedReason())

	reactor.notifyPeerClosed(peer, "session closed")
	require.Equal(t, "session closed", obs.lastClosedReason())
}

// TestPeerSetReactor verifies Peer.SetReactor sets the reactor reference.
//
// VALIDATES: SetReactor stores reactor reference in peer.
// PREVENTS: Nil reactor causing missed notifications.
func TestPeerSetReactor(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)

	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	peer := NewPeer(settings)

	// Initially nil
	peer.mu.RLock()
	require.Nil(t, peer.reactor, "reactor should be nil initially")
	peer.mu.RUnlock()

	// Set reactor
	peer.SetReactor(reactor)

	peer.mu.RLock()
	require.NotNil(t, peer.reactor, "reactor should be set after SetReactor")
	require.Equal(t, reactor, peer.reactor)
	peer.mu.RUnlock()
}

// TestMultipleObserversAllCalled verifies all registered observers receive callbacks.
//
// VALIDATES: All observers receive callbacks.
// PREVENTS: Observers being skipped.
func TestMultipleObserversAllCalled(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)

	obs1 := &mockLifecycleObserver{}
	obs2 := &mockLifecycleObserver{}
	obs3 := &mockLifecycleObserver{}

	reactor.AddPeerObserver(obs1)
	reactor.AddPeerObserver(obs2)
	reactor.AddPeerObserver(obs3)

	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	peer := NewPeer(settings)

	reactor.notifyPeerEstablished(peer)

	require.Equal(t, 1, obs1.establishedCount())
	require.Equal(t, 1, obs2.establishedCount())
	require.Equal(t, 1, obs3.establishedCount())
}
