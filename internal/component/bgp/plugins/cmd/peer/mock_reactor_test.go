package peer

import (
	"context"
	"net/netip"

	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/rib"
	"codeberg.org/thomas-mangin/ze/internal/core/selector"
)

// mockReactor implements plugin.ReactorLifecycle for handler tests.
type mockReactor struct {
	peers    []plugin.PeerInfo
	stats    plugin.ReactorStats
	peerCaps *plugin.PeerCapabilitiesInfo

	rawMessages []struct {
		addr    netip.Addr
		msgType uint8
		payload []byte
	}

	sendRefreshCalled bool
	sendBoRRCalled    bool
	sendEoRRCalled    bool

	// Soft clear tracking
	softClearCalls []string // peer selectors

	// Peer operations tracking
	teardownCalls []struct {
		addr    netip.Addr
		subcode uint8
		message string
	}
	configTree     map[string]any   // returned by GetConfigTree
	appliedConfigs []map[string]any // captured by ApplyConfigDiff
	removedPeers   []netip.Addr
	pausedPeers    []netip.Addr
	resumedPeers   []netip.Addr

	// NLRI batch tracking (used by update_wire integration tests)
	announcedBatches []struct {
		peer  string
		batch bgptypes.NLRIBatch
	}
	withdrawnBatches []struct {
		peer  string
		batch bgptypes.NLRIBatch
	}

	// Cache tracking
	cachedIDs []uint64 // returned by ListUpdates
	retainedIDs,
	releasedIDs,
	deletedIDs []uint64
	forwardedUpdates []struct {
		sel *selector.Selector
		id  uint64
	}
}

func (m *mockReactor) Peers() []plugin.PeerInfo            { return m.peers }
func (m *mockReactor) Stats() plugin.ReactorStats          { return m.stats }
func (m *mockReactor) Stop()                               {}
func (m *mockReactor) Reload() error                       { return nil }
func (m *mockReactor) VerifyConfig(_ map[string]any) error { return nil }
func (m *mockReactor) ApplyConfigDiff(tree map[string]any) error {
	m.appliedConfigs = append(m.appliedConfigs, tree)
	return nil
}
func (m *mockReactor) GetPeerProcessBindings(_ netip.Addr) []plugin.PeerProcessBinding { return nil }
func (m *mockReactor) GetPeerCapabilityConfigs() []plugin.PeerCapabilityConfig         { return nil }
func (m *mockReactor) PeerNegotiatedCapabilities(_ netip.Addr) *plugin.PeerCapabilitiesInfo {
	return m.peerCaps
}
func (m *mockReactor) GetConfigTree() map[string]any          { return m.configTree }
func (m *mockReactor) SetConfigTree(_ map[string]any)         {}
func (m *mockReactor) SignalAPIReady()                        {}
func (m *mockReactor) AddAPIProcessCount(_ int)               {}
func (m *mockReactor) SignalPluginStartupComplete()           {}
func (m *mockReactor) SignalPeerAPIReady(_ string)            {}
func (m *mockReactor) RegisterCacheConsumer(_ string, _ bool) {}
func (m *mockReactor) UnregisterCacheConsumer(_ string)       {}

func (m *mockReactor) PausePeer(addr netip.Addr) error {
	m.pausedPeers = append(m.pausedPeers, addr)
	return nil
}

func (m *mockReactor) ResumePeer(addr netip.Addr) error {
	m.resumedPeers = append(m.resumedPeers, addr)
	return nil
}

func (m *mockReactor) FlushForwardPool(_ context.Context) error               { return nil }
func (m *mockReactor) FlushForwardPoolPeer(_ context.Context, _ string) error { return nil }

func (m *mockReactor) TeardownPeer(addr netip.Addr, subcode uint8, shutdownMsg string) error {
	m.teardownCalls = append(m.teardownCalls, struct {
		addr    netip.Addr
		subcode uint8
		message string
	}{addr, subcode, shutdownMsg})
	return nil
}

func (m *mockReactor) RemovePeer(addr netip.Addr) error {
	m.removedPeers = append(m.removedPeers, addr)
	return nil
}

func (m *mockReactor) AddDynamicPeer(addr netip.Addr, tree map[string]any) error {
	m.appliedConfigs = append(m.appliedConfigs, tree)
	return nil
}

// BGP reactor stubs (not tracked unless needed).
func (m *mockReactor) AnnounceEOR(_ string, _ uint16, _ uint8) error { return nil }
func (m *mockReactor) AnnounceNLRIBatch(peer string, batch bgptypes.NLRIBatch) error {
	m.announcedBatches = append(m.announcedBatches, struct {
		peer  string
		batch bgptypes.NLRIBatch
	}{peer, batch})
	return nil
}

func (m *mockReactor) WithdrawNLRIBatch(peer string, batch bgptypes.NLRIBatch) error {
	m.withdrawnBatches = append(m.withdrawnBatches, struct {
		peer  string
		batch bgptypes.NLRIBatch
	}{peer, batch})
	return nil
}

// RIB stubs.
func (m *mockReactor) RIBInRoutes(_ string) []rib.RouteJSON { return nil }
func (m *mockReactor) RIBStats() bgptypes.RIBStatsInfo      { return bgptypes.RIBStatsInfo{} }
func (m *mockReactor) ClearRIBIn() int                      { return 0 }

func (m *mockReactor) SendRoutes(_ string, routes []*rib.Route, withdrawals []nlri.NLRI, _ bool) (bgptypes.TransactionResult, error) {
	return bgptypes.TransactionResult{
		RoutesAnnounced: len(routes),
		RoutesWithdrawn: len(withdrawals),
		UpdatesSent:     1,
	}, nil
}

// Cache operations (tracked).
func (m *mockReactor) RetainUpdate(id uint64) error {
	m.retainedIDs = append(m.retainedIDs, id)
	return nil
}

func (m *mockReactor) ReleaseUpdate(id uint64, _ string) error {
	m.releasedIDs = append(m.releasedIDs, id)
	return nil
}

func (m *mockReactor) DeleteUpdate(id uint64) error {
	m.deletedIDs = append(m.deletedIDs, id)
	return nil
}

func (m *mockReactor) ForwardUpdate(sel *selector.Selector, id uint64, _ string) error {
	m.forwardedUpdates = append(m.forwardedUpdates, struct {
		sel *selector.Selector
		id  uint64
	}{sel, id})
	return nil
}

func (m *mockReactor) ListUpdates() []uint64 { return m.cachedIDs }

// Raw message sending (tracked).
func (m *mockReactor) SendRawMessage(addr netip.Addr, msgType uint8, payload []byte) error {
	m.rawMessages = append(m.rawMessages, struct {
		addr    netip.Addr
		msgType uint8
		payload []byte
	}{addr, msgType, payload})
	return nil
}

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

func (m *mockReactor) SoftClearPeer(selector string) ([]string, error) {
	m.softClearCalls = append(m.softClearCalls, selector)
	return []string{"ipv4/unicast", "ipv6/unicast"}, nil
}

// newTestContext creates a CommandContext backed by a mock reactor.
func newTestContext(reactor plugin.ReactorLifecycle) *pluginserver.CommandContext {
	server, _ := pluginserver.NewServer(&pluginserver.ServerConfig{}, reactor)
	return &pluginserver.CommandContext{Server: server}
}

// newTestContextWithConfig creates a CommandContext with a config path set.
func newTestContextWithConfig(reactor plugin.ReactorLifecycle, configPath string) *pluginserver.CommandContext {
	server, _ := pluginserver.NewServer(&pluginserver.ServerConfig{
		ConfigPath: configPath,
	}, reactor)
	return &pluginserver.CommandContext{Server: server}
}
