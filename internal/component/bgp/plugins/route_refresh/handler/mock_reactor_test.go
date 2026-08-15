package handler

import (
	"context"
	"net/netip"

	"github.com/ze-software/ze/pkg/plugin/rpc"

	"github.com/ze-software/ze/internal/component/bgp/rib"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/selector"
)

// requireBGPReactor type-asserts at RUNTIME, so a mock that drifts out of
// bgptypes.BGPReactor still COMPILES and every handler under test silently takes
// the "BGP reactor not available" branch instead. This line makes that drift a
// build failure. It was added when the send permission put a process name on
// eight of the interface's methods and seven mocks went stale in silence.
var _ bgptypes.BGPReactor = (*mockReactor)(nil)

// mockReactor implements plugin.ReactorLifecycle and bgptypes.BGPReactor
// for handler tests in this package.
type mockReactor struct {
	peers    []plugin.PeerInfo
	stats    plugin.ReactorStats
	peerCaps *plugin.PeerCapabilitiesInfo

	sendRefreshCalled bool
	sendBoRRCalled    bool
	sendEoRRCalled    bool

	// Soft clear tracking
	softClearCalls []string // peer selectors
}

func (m *mockReactor) Peers() []plugin.PeerInfo                                        { return m.peers }
func (m *mockReactor) Stats() plugin.ReactorStats                                      { return m.stats }
func (m *mockReactor) Stop()                                                           {}
func (m *mockReactor) Reload() error                                                   { return nil }
func (m *mockReactor) DrainPeerSync(_ context.Context) error                           { return nil }
func (m *mockReactor) VerifyConfig(_ map[string]any) error                             { return nil }
func (m *mockReactor) ApplyConfigDiff(_ map[string]any) error                          { return nil }
func (m *mockReactor) GetPeerProcessBindings(_ netip.Addr) []plugin.PeerProcessBinding { return nil }
func (m *mockReactor) GetPeerCapabilityConfigs() []plugin.PeerCapabilityConfig         { return nil }
func (m *mockReactor) PeerNegotiatedCapabilities(_ netip.Addr) *plugin.PeerCapabilitiesInfo {
	return m.peerCaps
}
func (m *mockReactor) GetConfigTree() map[string]any          { return nil }
func (m *mockReactor) SetConfigTree(_ map[string]any)         {}
func (m *mockReactor) SignalAPIReady()                        {}
func (m *mockReactor) AddAPIProcessCount(_ int)               {}
func (m *mockReactor) SignalPluginStartupComplete()           {}
func (m *mockReactor) SignalPeerAPIReady(_ string)            {}
func (m *mockReactor) SetPeerUpBarrier(_ string, _ int)       {}
func (m *mockReactor) SignalPeerUpBarrier(_ string)           {}
func (m *mockReactor) RegisterCacheConsumer(_ string, _ bool) {}
func (m *mockReactor) UnregisterCacheConsumer(_ string)       {}
func (m *mockReactor) ForwardUpdatesDirect(_ []uint64, _ []netip.AddrPort, _ string, _ plugin.Sender) error {
	return nil
}

// RelayStoredRoute satisfies plugin.ReactorRelayCoordinator; this stub relays
// nothing because these tests exercise command dispatch, not the forward rail.
func (m *mockReactor) RelayStoredRoute(_ netip.Addr, _ []rpc.StoredRoute, _ plugin.Sender) error {
	return nil
}
func (m *mockReactor) ReleaseUpdates(_ []uint64, _ string) error { return nil }

func (m *mockReactor) PausePeer(_ netip.Addr) error  { return nil }
func (m *mockReactor) ResumePeer(_ netip.Addr) error { return nil }

func (m *mockReactor) FlushForwardPool(_ context.Context) error               { return nil }
func (m *mockReactor) FlushForwardPoolPeer(_ context.Context, _ string) error { return nil }

func (m *mockReactor) TeardownPeer(_ netip.Addr, _ uint8, _ string) error  { return nil }
func (m *mockReactor) RemovePeer(_ netip.Addr) error                       { return nil }
func (m *mockReactor) AddDynamicPeer(_ netip.Addr, _ map[string]any) error { return nil }

// BGP reactor stubs.
func (m *mockReactor) AnnounceEOR(_ *selector.Selector, _ uint16, _ uint8, _ plugin.Sender) error {
	return nil
}
func (m *mockReactor) AnnounceNLRIBatch(_ *selector.Selector, _ bgptypes.NLRIBatch, _ plugin.Sender) error {
	return nil
}

func (m *mockReactor) WithdrawNLRIBatch(_ *selector.Selector, _ bgptypes.NLRIBatch, _ plugin.Sender) error {
	return nil
}

// RIB stubs.
func (m *mockReactor) RIBInRoutes(_ string) []rib.RouteJSON { return nil }
func (m *mockReactor) RIBStats() bgptypes.RIBStatsInfo      { return bgptypes.RIBStatsInfo{} }
func (m *mockReactor) ClearRIBIn() int                      { return 0 }

func (m *mockReactor) SendRoutes(_ *selector.Selector, _ []*rib.Route, _ []nlri.NLRI, _ bool, _ plugin.Sender) (bgptypes.TransactionResult, error) {
	return bgptypes.TransactionResult{}, nil
}

// Cache operations.
func (m *mockReactor) RetainUpdate(_ uint64) error            { return nil }
func (m *mockReactor) ReleaseUpdate(_ uint64, _ string) error { return nil }
func (m *mockReactor) DeleteUpdate(_ uint64) error            { return nil }
func (m *mockReactor) ForwardUpdate(_ *selector.Selector, _ uint64, _ string, _ plugin.Sender) error {
	return nil
}
func (m *mockReactor) ListUpdates() []uint64 { return nil }

// Raw message sending.
func (m *mockReactor) SendRawMessage(_ netip.Addr, _ uint8, _ []byte, _ plugin.Sender) error {
	return nil
}

func (m *mockReactor) SendRefresh(_ *selector.Selector, _ uint16, _ uint8, _ plugin.Sender) error {
	m.sendRefreshCalled = true
	return nil
}

func (m *mockReactor) SendBoRR(_ *selector.Selector, _ uint16, _ uint8, _ plugin.Sender) error {
	m.sendBoRRCalled = true
	return nil
}

func (m *mockReactor) SendEoRR(_ *selector.Selector, _ uint16, _ uint8, _ plugin.Sender) error {
	m.sendEoRRCalled = true
	return nil
}

func (m *mockReactor) SoftClearPeer(sel *selector.Selector, _ plugin.Sender) ([]string, error) {
	m.softClearCalls = append(m.softClearCalls, sel.String())
	return []string{"ipv4/unicast", "ipv6/unicast"}, nil
}

// newTestContext creates a CommandContext backed by a mock reactor.
func newTestContext(reactor plugin.ReactorLifecycle) *pluginserver.CommandContext {
	server, _ := pluginserver.NewServer(&pluginserver.ServerConfig{}, reactor)
	return &pluginserver.CommandContext{Server: server}
}
