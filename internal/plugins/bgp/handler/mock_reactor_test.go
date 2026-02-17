package handler

import (
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/commit"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/rib"
	"codeberg.org/thomas-mangin/ze/internal/selector"
)

// mockReactor implements plugin.ReactorLifecycle for handler tests.
type mockReactor struct {
	peers []plugin.PeerInfo
	stats plugin.ReactorStats

	rawMessages []struct {
		addr    netip.Addr
		msgType uint8
		payload []byte
	}

	sendRefreshCalled bool
	sendBoRRCalled    bool
	sendEoRRCalled    bool

	// Peer operations tracking
	teardownCalls []struct {
		addr    netip.Addr
		subcode uint8
	}
	addedPeers   []plugin.DynamicPeerConfig
	removedPeers []netip.Addr

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

func (m *mockReactor) Peers() []plugin.PeerInfo                                        { return m.peers }
func (m *mockReactor) Stats() plugin.ReactorStats                                      { return m.stats }
func (m *mockReactor) Stop()                                                           {}
func (m *mockReactor) Reload() error                                                   { return nil }
func (m *mockReactor) VerifyConfig(_ map[string]any) error                             { return nil }
func (m *mockReactor) ApplyConfigDiff(_ map[string]any) error                          { return nil }
func (m *mockReactor) GetPeerProcessBindings(_ netip.Addr) []plugin.PeerProcessBinding { return nil }
func (m *mockReactor) GetPeerCapabilityConfigs() []plugin.PeerCapabilityConfig         { return nil }
func (m *mockReactor) GetConfigTree() map[string]any                                   { return nil }
func (m *mockReactor) SetConfigTree(_ map[string]any)                                  {}
func (m *mockReactor) SignalAPIReady()                                                 {}
func (m *mockReactor) AddAPIProcessCount(_ int)                                        {}
func (m *mockReactor) SignalPluginStartupComplete()                                    {}
func (m *mockReactor) SignalPeerAPIReady(_ string)                                     {}

func (m *mockReactor) TeardownPeer(addr netip.Addr, subcode uint8) error {
	m.teardownCalls = append(m.teardownCalls, struct {
		addr    netip.Addr
		subcode uint8
	}{addr, subcode})
	return nil
}

func (m *mockReactor) AddDynamicPeer(config plugin.DynamicPeerConfig) error {
	m.addedPeers = append(m.addedPeers, config)
	return nil
}

func (m *mockReactor) RemovePeer(addr netip.Addr) error {
	m.removedPeers = append(m.removedPeers, addr)
	return nil
}

// BGP reactor stubs (not tracked unless needed).
func (m *mockReactor) AnnounceRoute(_ string, _ bgptypes.RouteSpec) error        { return nil }
func (m *mockReactor) WithdrawRoute(_ string, _ netip.Prefix) error              { return nil }
func (m *mockReactor) AnnounceFlowSpec(_ string, _ bgptypes.FlowSpecRoute) error { return nil }
func (m *mockReactor) WithdrawFlowSpec(_ string, _ bgptypes.FlowSpecRoute) error { return nil }
func (m *mockReactor) AnnounceVPLS(_ string, _ bgptypes.VPLSRoute) error         { return nil }
func (m *mockReactor) WithdrawVPLS(_ string, _ bgptypes.VPLSRoute) error         { return nil }
func (m *mockReactor) AnnounceL2VPN(_ string, _ bgptypes.L2VPNRoute) error       { return nil }
func (m *mockReactor) WithdrawL2VPN(_ string, _ bgptypes.L2VPNRoute) error       { return nil }
func (m *mockReactor) AnnounceL3VPN(_ string, _ bgptypes.L3VPNRoute) error       { return nil }
func (m *mockReactor) WithdrawL3VPN(_ string, _ bgptypes.L3VPNRoute) error       { return nil }
func (m *mockReactor) AnnounceEOR(_ string, _ uint16, _ uint8) error             { return nil }
func (m *mockReactor) AnnounceWatchdog(_, _ string) error                        { return nil }
func (m *mockReactor) WithdrawWatchdog(_, _ string) error                        { return nil }
func (m *mockReactor) AddWatchdogRoute(_ bgptypes.RouteSpec, _ string) error     { return nil }
func (m *mockReactor) RemoveWatchdogRoute(_, _ string) error                     { return nil }
func (m *mockReactor) AnnounceLabeledUnicast(_ string, _ bgptypes.LabeledUnicastRoute) error {
	return nil
}
func (m *mockReactor) WithdrawLabeledUnicast(_ string, _ bgptypes.LabeledUnicastRoute) error {
	return nil
}
func (m *mockReactor) AnnounceMUPRoute(_ string, _ bgptypes.MUPRouteSpec) error { return nil }
func (m *mockReactor) WithdrawMUPRoute(_ string, _ bgptypes.MUPRouteSpec) error { return nil }
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
func (m *mockReactor) RIBOutRoutes() []rib.RouteJSON        { return nil }
func (m *mockReactor) RIBStats() bgptypes.RIBStatsInfo      { return bgptypes.RIBStatsInfo{} }
func (m *mockReactor) ClearRIBIn() int                      { return 0 }
func (m *mockReactor) ClearRIBOut() int                     { return 0 }
func (m *mockReactor) FlushRIBOut() int                     { return 0 }

// Transaction stubs.
func (m *mockReactor) BeginTransaction(_, _ string) error { return rib.ErrNoTransaction }
func (m *mockReactor) CommitTransaction(_ string) (bgptypes.TransactionResult, error) {
	return bgptypes.TransactionResult{}, rib.ErrNoTransaction
}
func (m *mockReactor) CommitTransactionWithLabel(_, _ string) (bgptypes.TransactionResult, error) {
	return bgptypes.TransactionResult{}, rib.ErrNoTransaction
}
func (m *mockReactor) RollbackTransaction(_ string) (bgptypes.TransactionResult, error) {
	return bgptypes.TransactionResult{}, rib.ErrNoTransaction
}
func (m *mockReactor) InTransaction(_ string) bool   { return false }
func (m *mockReactor) TransactionID(_ string) string { return "" }

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

func (m *mockReactor) ReleaseUpdate(id uint64) error {
	m.releasedIDs = append(m.releasedIDs, id)
	return nil
}

func (m *mockReactor) DeleteUpdate(id uint64) error {
	m.deletedIDs = append(m.deletedIDs, id)
	return nil
}

func (m *mockReactor) ForwardUpdate(sel *selector.Selector, id uint64) error {
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

// newTestContext creates a CommandContext backed by a mock reactor.
func newTestContext(reactor plugin.ReactorLifecycle) *plugin.CommandContext {
	server := plugin.NewServer(&plugin.ServerConfig{
		CommitManager: commit.NewCommitManager(),
	}, reactor)
	return &plugin.CommandContext{Server: server}
}
