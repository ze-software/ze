package plugin

import (
	"context"
	"errors"
	"net/netip"
	"testing"
)

// VALIDATES: Coordinator implements ReactorLifecycle and ProtocolReactor.
// PREVENTS: Missing interface method causes compile failure.
func TestCoordinatorImplementsReactorLifecycle(t *testing.T) {
	var _ ReactorLifecycle = (*Coordinator)(nil)
	var _ ProtocolReactor = (*Coordinator)(nil)
}

// VALIDATES: RegisterReactor stores and retrieves named protocol reactors.
// PREVENTS: Multi-protocol reactor registration broken.
func TestCoordinatorMultiReactor(t *testing.T) {
	c := NewCoordinator(map[string]any{})

	// No reactor registered
	if r := c.Reactor("ospf"); r != nil {
		t.Errorf("expected nil, got %v", r)
	}

	// Register a reactor
	dummy := "ospf-reactor"
	c.RegisterReactor("ospf", dummy)
	if r := c.Reactor("ospf"); r != dummy {
		t.Errorf("expected ospf-reactor, got %v", r)
	}

	// SetReactor also registers under "bgp"
	m := &mockReactor{}
	if err := c.SetReactor(m); err != nil {
		t.Fatal(err)
	}
	if r := c.Reactor("bgp"); r == nil {
		t.Error("expected bgp reactor in generic map")
	}

	// Unregister
	c.RegisterReactor("ospf", nil)
	if r := c.Reactor("ospf"); r != nil {
		t.Errorf("expected nil after unregister, got %v", r)
	}

	// SetReactor(nil) clears both
	if err := c.SetReactor(nil); err != nil {
		t.Fatal(err)
	}
	if r := c.Reactor("bgp"); r != nil {
		t.Errorf("expected nil after SetReactor(nil), got %v", r)
	}
}

// VALIDATES: BGP methods return ErrNoReactor when no reactor is set.
// PREVENTS: Nil dereference when BGP is not loaded.
func TestCoordinatorWithoutReactor(t *testing.T) {
	c := NewCoordinator(map[string]any{"interface": map[string]any{}})

	// Introspector: returns zero values
	if peers := c.Peers(); peers != nil {
		t.Errorf("expected nil peers, got %v", peers)
	}
	if caps := c.GetPeerCapabilityConfigs(); caps != nil {
		t.Errorf("expected nil caps, got %v", caps)
	}

	// PeerController: returns ErrNoReactor
	addr := netip.MustParseAddr("10.0.0.1")
	if err := c.TeardownPeer(addr, 2, ""); !errors.Is(err, ErrNoReactor) {
		t.Errorf("expected ErrNoReactor, got %v", err)
	}
	if err := c.PausePeer(addr); !errors.Is(err, ErrNoReactor) {
		t.Errorf("expected ErrNoReactor, got %v", err)
	}
	if err := c.RemovePeer(addr); !errors.Is(err, ErrNoReactor) {
		t.Errorf("expected ErrNoReactor, got %v", err)
	}

	// Configurator: config tree works
	tree := c.GetConfigTree()
	if _, ok := tree["interface"]; !ok {
		t.Error("expected interface in config tree")
	}

	// Startup coordinator: no-ops (no panic)
	c.SignalAPIReady()
	c.AddAPIProcessCount(1)
	c.SignalPluginStartupComplete()
	c.SignalPeerAPIReady("10.0.0.1")

	// Cache coordinator: no-ops (no panic)
	c.RegisterCacheConsumer("test", false)
	c.UnregisterCacheConsumer("test")

	// Stop: no-op
	c.Stop()
}

// VALIDATES: Coordinator delegates to reactor when set.
// PREVENTS: Reactor methods bypassed after SetReactor.
func TestCoordinatorWithReactor(t *testing.T) {
	c := NewCoordinator(map[string]any{})
	m := &mockReactor{}
	if err := c.SetReactor(m); err != nil {
		t.Fatal(err)
	}

	c.Peers()
	if !m.peersCalled {
		t.Error("expected Peers() to delegate to reactor")
	}

	addr := netip.MustParseAddr("10.0.0.1")
	_ = c.TeardownPeer(addr, 2, "")
	if !m.teardownCalled {
		t.Error("expected TeardownPeer to delegate to reactor")
	}

	c.SignalAPIReady()
	if !m.apiReadyCalled {
		t.Error("expected SignalAPIReady to delegate to reactor")
	}

	_ = c.Reload()
	if !m.reloadCalled {
		t.Error("expected Reload to delegate to reactor")
	}
}

// VALIDATES: SetReactor(nil) reverts to stub behavior.
// PREVENTS: Stale reactor reference after BGP unloads.
func TestCoordinatorUnsetReactor(t *testing.T) {
	c := NewCoordinator(map[string]any{})
	m := &mockReactor{}
	if err := c.SetReactor(m); err != nil {
		t.Fatal(err)
	}

	// Delegates
	c.Peers()
	if !m.peersCalled {
		t.Fatal("expected delegation")
	}

	// Unset
	if err := c.SetReactor(nil); err != nil {
		t.Fatal(err)
	}
	addr := netip.MustParseAddr("10.0.0.1")
	if err := c.TeardownPeer(addr, 2, ""); !errors.Is(err, ErrNoReactor) {
		t.Errorf("expected ErrNoReactor after unset, got %v", err)
	}
}

// mockReactor tracks which methods are called.
type mockReactor struct {
	peersCalled    bool
	teardownCalled bool
	apiReadyCalled bool
	reloadCalled   bool
}

func (m *mockReactor) Peers() []PeerInfo {
	m.peersCalled = true
	return nil
}
func (m *mockReactor) Stats() ReactorStats { return ReactorStats{} }
func (m *mockReactor) PeerNegotiatedCapabilities(netip.Addr) *PeerCapabilitiesInfo {
	return nil
}
func (m *mockReactor) GetPeerProcessBindings(netip.Addr) []PeerProcessBinding { return nil }
func (m *mockReactor) GetPeerCapabilityConfigs() []PeerCapabilityConfig       { return nil }
func (m *mockReactor) Stop()                                                  {}
func (m *mockReactor) TeardownPeer(netip.Addr, uint8, string) error {
	m.teardownCalled = true
	return nil
}
func (m *mockReactor) PausePeer(netip.Addr) error                         { return nil }
func (m *mockReactor) ResumePeer(netip.Addr) error                        { return nil }
func (m *mockReactor) AddDynamicPeer(netip.Addr, map[string]any) error    { return nil }
func (m *mockReactor) RemovePeer(netip.Addr) error                        { return nil }
func (m *mockReactor) FlushForwardPool(context.Context) error             { return nil }
func (m *mockReactor) FlushForwardPoolPeer(context.Context, string) error { return nil }
func (m *mockReactor) Reload() error {
	m.reloadCalled = true
	return nil
}
func (m *mockReactor) VerifyConfig(map[string]any) error    { return nil }
func (m *mockReactor) ApplyConfigDiff(map[string]any) error { return nil }
func (m *mockReactor) GetConfigTree() map[string]any        { return nil }
func (m *mockReactor) SetConfigTree(map[string]any)         {}
func (m *mockReactor) SignalAPIReady() {
	m.apiReadyCalled = true
}
func (m *mockReactor) AddAPIProcessCount(int)             {}
func (m *mockReactor) SignalPluginStartupComplete()       {}
func (m *mockReactor) SignalPeerAPIReady(string)          {}
func (m *mockReactor) RegisterCacheConsumer(string, bool) {}
func (m *mockReactor) UnregisterCacheConsumer(string)     {}
func (m *mockReactor) ForwardUpdatesDirect([]uint64, []netip.AddrPort, string) error {
	return nil
}
func (m *mockReactor) ReleaseUpdates([]uint64, string) error { return nil }
