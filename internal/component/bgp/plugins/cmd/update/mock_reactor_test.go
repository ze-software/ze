package update

import (
	"context"
	"net/netip"

	"github.com/ze-software/ze/pkg/plugin/rpc"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"

	"github.com/ze-software/ze/internal/component/bgp/rib"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/selector"
)

// requireBGPReactor type-asserts at RUNTIME, so a mock that drifts out of
// bgptypes.BGPReactor still COMPILES and every handler under test silently takes
// the "BGP reactor not available" branch instead. This line makes that drift a
// build failure. It was added when the send permission put a process name on
// eight of the interface's methods and seven mocks went stale in silence.
var _ bgptypes.BGPReactor = (*mockReactor)(nil)

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
	removedPeers []netip.Addr
	pausedPeers  []netip.Addr
	resumedPeers []netip.Addr

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

func (m *mockReactor) AddDynamicPeer(_ netip.Addr, _ map[string]any) error { return nil }

// BGP reactor stubs (not tracked unless needed).
func (m *mockReactor) AnnounceEOR(_ *selector.Selector, _ uint16, _ uint8, _ plugin.Sender) error {
	return nil
}
func (m *mockReactor) AnnounceNLRIBatch(sel *selector.Selector, batch bgptypes.NLRIBatch, _ plugin.Sender) error {
	m.announcedBatches = append(m.announcedBatches, struct {
		peer  string
		batch bgptypes.NLRIBatch
	}{sel.String(), batch})
	return nil
}

func (m *mockReactor) WithdrawNLRIBatch(sel *selector.Selector, batch bgptypes.NLRIBatch, _ plugin.Sender) error {
	m.withdrawnBatches = append(m.withdrawnBatches, struct {
		peer  string
		batch bgptypes.NLRIBatch
	}{sel.String(), batch})
	return nil
}

// RIB stubs.
func (m *mockReactor) RIBInRoutes(_ string) []rib.RouteJSON { return nil }
func (m *mockReactor) RIBStats() bgptypes.RIBStatsInfo      { return bgptypes.RIBStatsInfo{} }
func (m *mockReactor) ClearRIBIn() int                      { return 0 }

func (m *mockReactor) SendRoutes(_ *selector.Selector, routes []*rib.Route, withdrawals []nlri.NLRI, _ bool, _ plugin.Sender) (bgptypes.TransactionResult, error) {
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

func (m *mockReactor) ForwardUpdate(sel *selector.Selector, id uint64, _ string, _ plugin.Sender) error {
	m.forwardedUpdates = append(m.forwardedUpdates, struct {
		sel *selector.Selector
		id  uint64
	}{sel, id})
	return nil
}

func (m *mockReactor) ListUpdates() []uint64 { return m.cachedIDs }

// Raw message sending (tracked).
func (m *mockReactor) SendRawMessage(addr netip.Addr, msgType uint8, payload []byte, _ plugin.Sender) error {
	m.rawMessages = append(m.rawMessages, struct {
		addr    netip.Addr
		msgType uint8
		payload []byte
	}{addr, msgType, payload})
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
