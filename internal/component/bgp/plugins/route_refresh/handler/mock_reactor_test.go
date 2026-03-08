package handler

import (
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/transaction"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/rib"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/selector"
)

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
func (m *mockReactor) RegisterCacheConsumer(_ string, _ bool) {}
func (m *mockReactor) UnregisterCacheConsumer(_ string)       {}

func (m *mockReactor) PausePeer(_ netip.Addr) error  { return nil }
func (m *mockReactor) ResumePeer(_ netip.Addr) error { return nil }

func (m *mockReactor) TeardownPeer(_ netip.Addr, _ uint8) error        { return nil }
func (m *mockReactor) AddDynamicPeer(_ plugin.DynamicPeerConfig) error { return nil }
func (m *mockReactor) RemovePeer(_ netip.Addr) error                   { return nil }

// BGP reactor stubs.
func (m *mockReactor) AnnounceEOR(_ string, _ uint16, _ uint8) error { return nil }
func (m *mockReactor) AnnounceNLRIBatch(_ string, _ bgptypes.NLRIBatch) error {
	return nil
}

func (m *mockReactor) WithdrawNLRIBatch(_ string, _ bgptypes.NLRIBatch) error {
	return nil
}

// RIB stubs.
func (m *mockReactor) RIBInRoutes(_ string) []rib.RouteJSON { return nil }
func (m *mockReactor) RIBStats() bgptypes.RIBStatsInfo      { return bgptypes.RIBStatsInfo{} }
func (m *mockReactor) ClearRIBIn() int                      { return 0 }

func (m *mockReactor) SendRoutes(_ string, _ []*rib.Route, _ []nlri.NLRI, _ bool) (bgptypes.TransactionResult, error) {
	return bgptypes.TransactionResult{}, nil
}

// Cache operations.
func (m *mockReactor) RetainUpdate(_ uint64) error                                  { return nil }
func (m *mockReactor) ReleaseUpdate(_ uint64, _ string) error                       { return nil }
func (m *mockReactor) DeleteUpdate(_ uint64) error                                  { return nil }
func (m *mockReactor) ForwardUpdate(_ *selector.Selector, _ uint64, _ string) error { return nil }
func (m *mockReactor) ListUpdates() []uint64                                        { return nil }

// Raw message sending.
func (m *mockReactor) SendRawMessage(_ netip.Addr, _ uint8, _ []byte) error { return nil }

func (m *mockReactor) SendRefresh(_ string, _ uint16, _ uint8) error {
	m.sendRefreshCalled = true
	return nil
}

func (m *mockReactor) SendBoRR(_ string, _ uint16, _ uint8) error {
	m.sendBoRRCalled = true
	return nil
}

func (m *mockReactor) SendEoRR(_ string, _ uint16, _ uint8) error {
	m.sendEoRRCalled = true
	return nil
}

func (m *mockReactor) SoftClearPeer(sel string) ([]string, error) {
	m.softClearCalls = append(m.softClearCalls, sel)
	return []string{"ipv4/unicast", "ipv6/unicast"}, nil
}

// newTestContext creates a CommandContext backed by a mock reactor.
func newTestContext(reactor plugin.ReactorLifecycle) *pluginserver.CommandContext {
	server := pluginserver.NewServer(&pluginserver.ServerConfig{
		CommitManager: transaction.NewCommitManager(),
	}, reactor)
	return &pluginserver.CommandContext{Server: server}
}
