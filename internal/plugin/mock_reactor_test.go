package plugin

import (
	"errors"
	"net/netip"

	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/rib"
	"codeberg.org/thomas-mangin/ze/internal/selector"
)

// ErrPeerNotFound is a test error matching reactor.ErrPeerNotFound.
// Cannot import reactor due to import cycle (reactor imports api).
var ErrPeerNotFound = errors.New("peer not found")

// mockReactor implements ReactorInterface for testing.
type mockReactor struct {
	peers           []PeerInfo
	stats           ReactorStats
	stopped         bool
	announcedRoutes []struct {
		selector string
		route    bgptypes.RouteSpec
	}
	withdrawnRoutes []struct {
		selector string
		prefix   netip.Prefix
	}
	announcedL3VPNRoutes []struct {
		selector string
		route    bgptypes.L3VPNRoute
	}
	withdrawnL3VPNRoutes []struct {
		selector string
		route    bgptypes.L3VPNRoute
	}
	announcedLabeledUnicastRoutes []struct {
		selector string
		route    bgptypes.LabeledUnicastRoute
	}
	withdrawnLabeledUnicastRoutes []struct {
		selector string
		route    bgptypes.LabeledUnicastRoute
	}
	teardownCalls []struct {
		addr    netip.Addr
		subcode uint8
	}
	rawMessages []struct {
		addr    netip.Addr
		msgType uint8
		payload []byte
	}
	// RIB operation tracking
	ribInCleared  bool
	ribOutCleared bool
	ribOutFlushed bool

	// NLRI batch tracking for wire mode tests
	announcedBatches []struct {
		selector string
		batch    bgptypes.NLRIBatch
	}
	withdrawnBatches []struct {
		selector string
		batch    bgptypes.NLRIBatch
	}

	// Dynamic peer management tracking
	addedPeers   []DynamicPeerConfig
	removedPeers []netip.Addr
}

func (m *mockReactor) Peers() []PeerInfo {
	return m.peers
}

func (m *mockReactor) Stats() ReactorStats {
	return m.stats
}

func (m *mockReactor) Stop() {
	m.stopped = true
}

func (m *mockReactor) AnnounceRoute(selector string, route bgptypes.RouteSpec) error {
	m.announcedRoutes = append(m.announcedRoutes, struct {
		selector string
		route    bgptypes.RouteSpec
	}{selector, route})
	return nil
}

func (m *mockReactor) WithdrawRoute(selector string, prefix netip.Prefix) error {
	m.withdrawnRoutes = append(m.withdrawnRoutes, struct {
		selector string
		prefix   netip.Prefix
	}{selector, prefix})
	return nil
}

func (m *mockReactor) TeardownPeer(addr netip.Addr, subcode uint8) error {
	m.teardownCalls = append(m.teardownCalls, struct {
		addr    netip.Addr
		subcode uint8
	}{addr, subcode})
	return nil
}

func (m *mockReactor) Reload() error {
	return nil
}

func (m *mockReactor) VerifyConfig(_ map[string]any) error {
	return nil
}

func (m *mockReactor) ApplyConfigDiff(_ map[string]any) error {
	return nil
}

func (m *mockReactor) AddDynamicPeer(config DynamicPeerConfig) error {
	m.addedPeers = append(m.addedPeers, config)
	return nil
}

func (m *mockReactor) RemovePeer(addr netip.Addr) error {
	m.removedPeers = append(m.removedPeers, addr)
	return nil
}

func (m *mockReactor) AnnounceFlowSpec(_ string, _ bgptypes.FlowSpecRoute) error {
	return nil
}

func (m *mockReactor) WithdrawFlowSpec(_ string, _ bgptypes.FlowSpecRoute) error {
	return nil
}

func (m *mockReactor) AnnounceVPLS(_ string, _ bgptypes.VPLSRoute) error {
	return nil
}

func (m *mockReactor) WithdrawVPLS(_ string, _ bgptypes.VPLSRoute) error {
	return nil
}

func (m *mockReactor) AnnounceL2VPN(_ string, _ bgptypes.L2VPNRoute) error {
	return nil
}

func (m *mockReactor) AnnounceL3VPN(selector string, route bgptypes.L3VPNRoute) error {
	m.announcedL3VPNRoutes = append(m.announcedL3VPNRoutes, struct {
		selector string
		route    bgptypes.L3VPNRoute
	}{selector, route})
	return nil
}

func (m *mockReactor) WithdrawL3VPN(selector string, route bgptypes.L3VPNRoute) error {
	m.withdrawnL3VPNRoutes = append(m.withdrawnL3VPNRoutes, struct {
		selector string
		route    bgptypes.L3VPNRoute
	}{selector, route})
	return nil
}

func (m *mockReactor) AnnounceLabeledUnicast(selector string, route bgptypes.LabeledUnicastRoute) error {
	m.announcedLabeledUnicastRoutes = append(m.announcedLabeledUnicastRoutes, struct {
		selector string
		route    bgptypes.LabeledUnicastRoute
	}{selector, route})
	return nil
}

func (m *mockReactor) WithdrawLabeledUnicast(selector string, route bgptypes.LabeledUnicastRoute) error {
	m.withdrawnLabeledUnicastRoutes = append(m.withdrawnLabeledUnicastRoutes, struct {
		selector string
		route    bgptypes.LabeledUnicastRoute
	}{selector, route})
	return nil
}

func (m *mockReactor) AnnounceMUPRoute(_ string, _ bgptypes.MUPRouteSpec) error {
	return nil
}

func (m *mockReactor) WithdrawMUPRoute(_ string, _ bgptypes.MUPRouteSpec) error {
	return nil
}

func (m *mockReactor) AnnounceEOR(_ string, _ uint16, _ uint8) error {
	return nil
}

func (m *mockReactor) RIBInRoutes(_ string) []rib.RouteJSON {
	return nil
}

func (m *mockReactor) RIBOutRoutes() []rib.RouteJSON {
	return nil
}

func (m *mockReactor) RIBStats() RIBStatsInfo {
	return RIBStatsInfo{}
}

func (m *mockReactor) ClearRIBIn() int {
	m.ribInCleared = true
	return 5 // Mock: pretend we cleared 5 routes
}

func (m *mockReactor) ClearRIBOut() int {
	m.ribOutCleared = true
	return 3 // Mock: pretend we withdrew 3 routes
}

func (m *mockReactor) FlushRIBOut() int {
	m.ribOutFlushed = true
	return 7 // Mock: pretend we flushed 7 routes
}

func (m *mockReactor) GetPeerProcessBindings(_ netip.Addr) []PeerProcessBinding {
	return nil // Mock: no API bindings configured
}

func (m *mockReactor) GetPeerCapabilityConfigs() []PeerCapabilityConfig {
	return nil // Mock: no capability configs
}

func (m *mockReactor) GetConfigTree() map[string]any {
	return nil // Mock: no config tree
}

func (m *mockReactor) SetConfigTree(_ map[string]any) {}

func (m *mockReactor) WithdrawL2VPN(_ string, _ bgptypes.L2VPNRoute) error {
	return nil
}

// Transaction stubs (base mock doesn't support transactions).
func (m *mockReactor) BeginTransaction(_, _ string) error {
	return bgptypes.ErrNoTransaction
}

func (m *mockReactor) CommitTransaction(_ string) (bgptypes.TransactionResult, error) {
	return bgptypes.TransactionResult{}, bgptypes.ErrNoTransaction
}

func (m *mockReactor) CommitTransactionWithLabel(_, _ string) (bgptypes.TransactionResult, error) {
	return bgptypes.TransactionResult{}, bgptypes.ErrNoTransaction
}

func (m *mockReactor) RollbackTransaction(_ string) (bgptypes.TransactionResult, error) {
	return bgptypes.TransactionResult{}, bgptypes.ErrNoTransaction
}

func (m *mockReactor) InTransaction(_ string) bool {
	return false
}

func (m *mockReactor) TransactionID(_ string) string {
	return ""
}

func (m *mockReactor) SendRoutes(_ string, routes []*rib.Route, withdrawals []nlri.NLRI, _ bool) (bgptypes.TransactionResult, error) {
	return bgptypes.TransactionResult{
		RoutesAnnounced: len(routes),
		RoutesWithdrawn: len(withdrawals),
		UpdatesSent:     1,
	}, nil
}

func (m *mockReactor) AnnounceWatchdog(_, _ string) error {
	return nil
}

func (m *mockReactor) WithdrawWatchdog(_, _ string) error {
	return nil
}

func (m *mockReactor) AddWatchdogRoute(_ bgptypes.RouteSpec, _ string) error {
	return nil
}

func (m *mockReactor) RemoveWatchdogRoute(_, _ string) error {
	return nil
}

func (m *mockReactor) ForwardUpdate(_ *selector.Selector, _ uint64) error {
	return nil
}

func (m *mockReactor) DeleteUpdate(_ uint64) error {
	return nil
}

func (m *mockReactor) RetainUpdate(_ uint64) error {
	return nil
}

func (m *mockReactor) ReleaseUpdate(_ uint64) error {
	return nil
}

func (m *mockReactor) ListUpdates() []uint64 {
	return nil
}

func (m *mockReactor) SignalAPIReady() {}

func (m *mockReactor) AddAPIProcessCount(_ int)     {}
func (m *mockReactor) SignalPluginStartupComplete() {}

func (m *mockReactor) SignalPeerAPIReady(_ string) {}

func (m *mockReactor) AnnounceNLRIBatch(selector string, batch bgptypes.NLRIBatch) error {
	m.announcedBatches = append(m.announcedBatches, struct {
		selector string
		batch    bgptypes.NLRIBatch
	}{selector, batch})
	return nil
}

func (m *mockReactor) WithdrawNLRIBatch(selector string, batch bgptypes.NLRIBatch) error {
	m.withdrawnBatches = append(m.withdrawnBatches, struct {
		selector string
		batch    bgptypes.NLRIBatch
	}{selector, batch})
	return nil
}

func (m *mockReactor) SendRawMessage(addr netip.Addr, msgType uint8, payload []byte) error {
	m.rawMessages = append(m.rawMessages, struct {
		addr    netip.Addr
		msgType uint8
		payload []byte
	}{addr, msgType, payload})
	return nil
}

func (m *mockReactor) SendBoRR(_ string, _ uint16, _ uint8) error {
	return nil
}

func (m *mockReactor) SendEoRR(_ string, _ uint16, _ uint8) error {
	return nil
}

// mockReactorRawError embeds mockReactor but returns error from SendRawMessage.
type mockReactorRawError struct {
	mockReactor
	err error
}

func (m *mockReactorRawError) SendRawMessage(_ netip.Addr, _ uint8, _ []byte) error {
	return m.err
}
