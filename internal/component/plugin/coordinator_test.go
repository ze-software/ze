package plugin

import (
	"context"
	"errors"
	"net/netip"
	"reflect"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// Bootstrap round-trip fakes: satisfy the callback/storage interfaces without
// implementing behavior (methods are never called in the test).
type fakeStore struct{ storage.Storage }
type fakePeerCB struct{}

func (fakePeerCB) OnPeerEstablished(any)    {}
func (fakePeerCB) OnPeerClosed(any, string) {}

// TestBootstrapRoundTrip proves the coordinator stores and returns the typed
// BGPBootstrap struct that replaced the string-keyed extra bag, preserving all
// its fields (seven since feature-gate-11 moved the two MRT callbacks to the
// registry seam), and that *Coordinator still satisfies CoordinatorAccessor.
//
// VALIDATES: P2 AC-3 (bootstrap delivered via a typed struct; the string-keyed
// extra bag and its per-read assertions are gone).
// PREVENTS: a field being dropped or mistyped when threading hub -> reactor factory.
func TestBootstrapRoundTrip(t *testing.T) {
	var _ registry.CoordinatorAccessor = (*Coordinator)(nil)

	peerCB := fakePeerCB{}
	want := registry.BGPBootstrap{
		ConfigPath:         "/etc/ze.conf",
		CLIPlugins:         []string{"bgp-rs", "bgp-gr"},
		ConfigData:         []byte("bgp {}"),
		Store:              fakeStore{},
		ChaosSeed:          42,
		ChaosRate:          0.25,
		HealthPeerCallback: peerCB,
		// MRTMessageCallback / MRTPeerCallback were removed from BGPBootstrap:
		// MRT now self-registers those bridges into the registry seam, verified by
		// registry.TestMRTCallbackSeam. This struct no longer carries them.
	}
	c := NewCoordinator(map[string]any{})
	c.SetBootstrap(want)
	if got := c.Bootstrap(); !reflect.DeepEqual(got, want) {
		t.Errorf("Bootstrap() = %+v, want %+v", got, want)
	}
	if got := NewCoordinator(nil).Bootstrap(); !reflect.DeepEqual(got, registry.BGPBootstrap{}) {
		t.Errorf("fresh coordinator Bootstrap() = %+v, want zero value", got)
	}
}

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
	c.SetPeerUpBarrier("10.0.0.1", 1)
	c.SignalPeerUpBarrier("10.0.0.1")

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

	// The peer-up barrier pair, asserted by ARGUMENT and not merely by "a method
	// ran": a body that delegated to the wrong reactor call (SignalPeerAPIReady
	// in place of SignalPeerUpBarrier, say) compiles, and would release the wrong
	// barrier at run time.
	c.SetPeerUpBarrier("10.0.0.2", 3)
	if m.barrierSet != "10.0.0.2" || m.barrierExpected != 3 {
		t.Errorf("expected SetPeerUpBarrier to delegate peer and count, got %q/%d",
			m.barrierSet, m.barrierExpected)
	}

	c.SignalPeerUpBarrier("10.0.0.3")
	if m.barrierSignaled != "10.0.0.3" {
		t.Errorf("expected SignalPeerUpBarrier to delegate the peer, got %q", m.barrierSignaled)
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

// TestCacheConsumerDeclaredBeforeReactorReachesIt drives the startup order the
// daemon actually uses: every plugin declares in Stage 1, and the BGP reactor is
// built in the bgp plugin's Stage 2 OnConfigure (bgp/plugin/register.go), so a
// cache-consumer declaration is ALWAYS made while no reactor is attached.
//
// VALIDATES: the declaration, and its ack ordering, survive that gap.
// PREVENTS: route loss. Dropping the declaration left bgp-rs unknown to
// RecentUpdateCache, which then applied FIFO cumulative-ack semantics to a
// plugin that declared CacheConsumerUnordered. One ack of a later message id
// then evicted an entry bgp-rs had batched but not yet forwarded, and the
// UPDATE reached nobody: "BUG: ForwardUpdatesDirect: msgID missing from cache"
// (reactor/reactor_api_forward_batch.go), measured in interop scenario 54.
func TestCacheConsumerDeclaredBeforeReactorReachesIt(t *testing.T) {
	c := NewCoordinator(map[string]any{})

	// Stage 1: the plugins declare. No reactor exists yet.
	c.RegisterCacheConsumer("bgp-rs", true)
	c.RegisterCacheConsumer("bgp-persist", false)

	// Stage 2: the bgp plugin builds the reactor and attaches it.
	m := &mockReactor{}
	if err := c.SetReactor(m); err != nil {
		t.Fatal(err)
	}

	unordered, ok := m.cacheConsumers["bgp-rs"]
	if !ok {
		t.Fatal("bgp-rs declared cache-consumer in Stage 1 and the reactor was never told")
	}
	if !unordered {
		t.Error("bgp-rs reached the reactor as an ordered consumer; it declared unordered")
	}
	if fifo, ok := m.cacheConsumers["bgp-persist"]; !ok || fifo {
		t.Errorf("bgp-persist = (%v, present=%v), want (false, present=true)", fifo, ok)
	}
	if n := m.cacheRegisterCalls["bgp-rs"]; n != 1 {
		t.Errorf("bgp-rs registered %d times, want 1: the cache logs a BUG on a repeat", n)
	}
}

// VALIDATES: a declaration made after the reactor is attached still reaches it,
// and a consumer that unregisters before the reactor attaches is not replayed.
// PREVENTS: the recorded set turning into a leak that resurrects a dead plugin.
//
// bgp-rs is declared beside bgp-rr and never unregistered, so "bgp-rr is
// absent" cannot pass by nothing ever being replayed.
func TestCacheConsumerRegistrationTracksLifecycle(t *testing.T) {
	c := NewCoordinator(map[string]any{})

	c.RegisterCacheConsumer("bgp-rr", true)
	c.RegisterCacheConsumer("bgp-rs", true)
	c.UnregisterCacheConsumer("bgp-rr")

	m := &mockReactor{}
	if err := c.SetReactor(m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.cacheConsumers["bgp-rs"]; !ok {
		t.Fatal("bgp-rs was declared and never unregistered, but the reactor was not told")
	}
	if _, ok := m.cacheConsumers["bgp-rr"]; ok {
		t.Error("bgp-rr unregistered before attach but was replayed onto the reactor")
	}

	c.RegisterCacheConsumer("bgp-rs", true)
	if unordered, ok := m.cacheConsumers["bgp-rs"]; !ok || !unordered {
		t.Errorf("post-attach declaration = (%v, present=%v), want (true, present=true)", unordered, ok)
	}

	c.UnregisterCacheConsumer("bgp-rs")
	if _, ok := m.cacheConsumers["bgp-rs"]; ok {
		t.Error("UnregisterCacheConsumer did not reach the attached reactor")
	}
}

// VALIDATES: RegisterReactor("bgp") replays the recorded declarations, because
// it and SetReactor write the same map row.
// PREVENTS: the fixed defect coming back through the other door. Only
// SetReactor carried the replay, and RegisterReactor is on the exported
// CoordinatorAccessor surface, so a caller reaching for it would attach the BGP
// reactor with every cache consumer unknown to it.
func TestRegisterReactorBGPReplaysCacheConsumers(t *testing.T) {
	c := NewCoordinator(map[string]any{})
	c.RegisterCacheConsumer("bgp-rs", true)

	m := &mockReactor{}
	c.RegisterReactor("bgp", m)

	unordered, ok := m.cacheConsumers["bgp-rs"]
	if !ok {
		t.Fatal("RegisterReactor(\"bgp\") attached the reactor without replaying the declarations")
	}
	if !unordered {
		t.Error("bgp-rs reached the reactor as an ordered consumer; it declared unordered")
	}
	if r := c.Reactor("bgp"); r == nil {
		t.Error("RegisterReactor(\"bgp\") did not store the reactor")
	}

	// A non-BGP name keeps the plain store, with no replay.
	other := &mockReactor{}
	c.RegisterReactor("ospf", other)
	if len(other.cacheConsumers) != 0 {
		t.Errorf("ospf reactor was told about %d cache consumers; cache consumers are BGP's", len(other.cacheConsumers))
	}
}

// mockReactor tracks which methods are called.
type mockReactor struct {
	peersCalled    bool
	teardownCalled bool
	apiReadyCalled bool

	barrierSet      string
	barrierExpected int
	barrierSignaled string
	reloadCalled    bool

	// cacheConsumers records name -> unordered for every RegisterCacheConsumer
	// this reactor was told about; unregistered names are deleted.
	cacheConsumers map[string]bool
	// cacheRegisterCalls counts registrations per name. The real cache logs
	// "BUG: duplicate RegisterConsumer" on a repeat, so once is the contract.
	cacheRegisterCalls map[string]int

	// relay* record the last RelayStoredRoute this reactor was handed, and
	// relayErr is what it answers with.
	relayCalled bool
	relayDest   netip.Addr
	relayRoutes []rpc.StoredRoute
	relayErr    error
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
func (m *mockReactor) DrainPeerSync(context.Context) error                { return nil }
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
func (m *mockReactor) AddAPIProcessCount(int)       {}
func (m *mockReactor) SignalPluginStartupComplete() {}
func (m *mockReactor) SignalPeerAPIReady(string)    {}
func (m *mockReactor) SetPeerUpBarrier(peer string, expected int) {
	m.barrierSet = peer
	m.barrierExpected = expected
}

func (m *mockReactor) SignalPeerUpBarrier(peer string) {
	m.barrierSignaled = peer
}

func (m *mockReactor) RegisterCacheConsumer(name string, unordered bool) {
	if m.cacheConsumers == nil {
		m.cacheConsumers = make(map[string]bool)
		m.cacheRegisterCalls = make(map[string]int)
	}
	m.cacheConsumers[name] = unordered
	m.cacheRegisterCalls[name]++
}

func (m *mockReactor) UnregisterCacheConsumer(name string) {
	delete(m.cacheConsumers, name)
}
func (m *mockReactor) ForwardUpdatesDirect([]uint64, []netip.AddrPort, string, Sender) error {
	return nil
}

// RelayStoredRoute satisfies plugin.ReactorRelayCoordinator. It relays nothing
// because these tests exercise the delegation, not the forward rail, and records
// what it was handed so a test can prove the arguments crossed unchanged.
func (m *mockReactor) RelayStoredRoute(destination netip.Addr, routes []rpc.StoredRoute, _ Sender) error {
	m.relayCalled = true
	m.relayDest = destination
	m.relayRoutes = routes
	return m.relayErr
}
func (m *mockReactor) ReleaseUpdates([]uint64, string) error { return nil }

// TestCoordinatorRelayStoredRoute verifies both branches of the relay
// delegation: no reactor answers ErrNoReactor, and a registered reactor receives
// the destination and the routes unchanged and its answer is returned.
//
// VALIDATES: the peer-up replay's engine entry point. The plugin server holds
// the Coordinator as its ReactorLifecycle, so this method is what every
// relay-stored-route RPC from bgp-adj-rib-in passes through.
// PREVENTS: the delegation silently dropping a route slice or swallowing the
// reactor's error, either of which turns a failed replay into a reported one.
func TestCoordinatorRelayStoredRoute(t *testing.T) {
	dest := netip.MustParseAddr("192.0.2.7")
	routes := []rpc.StoredRoute{
		{SourcePeer: "198.51.100.1", Family: "ipv4/unicast", NLRIHex: "180a0000"},
		{SourcePeer: "198.51.100.2", Family: "ipv4/unicast", NLRIHex: "10c000"},
	}

	c := NewCoordinator(map[string]any{})
	if err := c.RelayStoredRoute(dest, routes, Sender{}); !errors.Is(err, ErrNoReactor) {
		t.Fatalf("without a reactor: expected ErrNoReactor, got %v", err)
	}

	m := &mockReactor{}
	if err := c.SetReactor(m); err != nil {
		t.Fatal(err)
	}
	if err := c.RelayStoredRoute(dest, routes, Sender{}); err != nil {
		t.Fatalf("with a reactor: expected the reactor's nil, got %v", err)
	}
	if !m.relayCalled {
		t.Fatal("expected RelayStoredRoute to reach the reactor")
	}
	if m.relayDest != dest {
		t.Errorf("destination = %v, want %v", m.relayDest, dest)
	}
	if !reflect.DeepEqual(m.relayRoutes, routes) {
		t.Errorf("routes = %+v, want %+v", m.relayRoutes, routes)
	}

	// The reactor's failure is the caller's failure: a replay that could not be
	// delivered must not read as one that was.
	m.relayErr = errors.New("relay refused")
	if err := c.RelayStoredRoute(dest, routes, Sender{}); !errors.Is(err, m.relayErr) {
		t.Errorf("expected the reactor's error back, got %v", err)
	}
}
