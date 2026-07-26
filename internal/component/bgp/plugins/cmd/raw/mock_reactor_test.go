package raw

import (
	"context"
	"net/netip"

	"github.com/ze-software/ze/pkg/plugin/rpc"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

	"github.com/ze-software/ze/internal/component/bgp/rib"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/selector"
)

// mockReactor implements plugin.ReactorLifecycle + bgptypes.BGPReactor for handler tests.
type mockReactor struct {
	rawMessages []struct {
		addr    netip.Addr
		msgType uint8
		payload []byte
	}
}

func (m *mockReactor) Peers() []plugin.PeerInfo                                        { return nil }
func (m *mockReactor) Stats() plugin.ReactorStats                                      { return plugin.ReactorStats{} }
func (m *mockReactor) Stop()                                                           {}
func (m *mockReactor) Reload() error                                                   { return nil }
func (m *mockReactor) VerifyConfig(_ map[string]any) error                             { return nil }
func (m *mockReactor) ApplyConfigDiff(_ map[string]any) error                          { return nil }
func (m *mockReactor) GetPeerProcessBindings(_ netip.Addr) []plugin.PeerProcessBinding { return nil }
func (m *mockReactor) GetPeerCapabilityConfigs() []plugin.PeerCapabilityConfig         { return nil }
func (m *mockReactor) PeerNegotiatedCapabilities(_ netip.Addr) *plugin.PeerCapabilitiesInfo {
	return nil
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
func (m *mockReactor) ForwardUpdatesDirect(_ []uint64, _ []netip.AddrPort, _ string) error {
	return nil
}

// RelayStoredRoute satisfies plugin.ReactorRelayCoordinator; this stub relays
// nothing because these tests exercise command dispatch, not the forward rail.
func (m *mockReactor) RelayStoredRoute(_ netip.Addr, _ []rpc.StoredRoute) error {
	return nil
}
func (m *mockReactor) ReleaseUpdates(_ []uint64, _ string) error { return nil }

func (m *mockReactor) PausePeer(_ netip.Addr) error                                       { return nil }
func (m *mockReactor) ResumePeer(_ netip.Addr) error                                      { return nil }
func (m *mockReactor) FlushForwardPool(_ context.Context) error                           { return nil }
func (m *mockReactor) FlushForwardPoolPeer(_ context.Context, _ string) error             { return nil }
func (m *mockReactor) DrainPeerSync(_ context.Context) error                              { return nil }
func (m *mockReactor) TeardownPeer(_ netip.Addr, _ uint8, _ string) error                 { return nil }
func (m *mockReactor) RemovePeer(_ netip.Addr) error                                      { return nil }
func (m *mockReactor) AddDynamicPeer(_ netip.Addr, _ map[string]any) error                { return nil }
func (m *mockReactor) AnnounceEOR(_ *selector.Selector, _ uint16, _ uint8) error          { return nil }
func (m *mockReactor) AnnounceNLRIBatch(_ *selector.Selector, _ bgptypes.NLRIBatch) error { return nil }
func (m *mockReactor) WithdrawNLRIBatch(_ *selector.Selector, _ bgptypes.NLRIBatch) error { return nil }
func (m *mockReactor) RIBInRoutes(_ string) []rib.RouteJSON                               { return nil }
func (m *mockReactor) RIBStats() bgptypes.RIBStatsInfo                                    { return bgptypes.RIBStatsInfo{} }
func (m *mockReactor) ClearRIBIn() int                                                    { return 0 }

func (m *mockReactor) SendRoutes(_ *selector.Selector, routes []*rib.Route, withdrawals []nlri.NLRI, _ bool) (bgptypes.TransactionResult, error) {
	return bgptypes.TransactionResult{
		RoutesAnnounced: len(routes),
		RoutesWithdrawn: len(withdrawals),
		UpdatesSent:     1,
	}, nil
}

func (m *mockReactor) RetainUpdate(_ uint64) error                                  { return nil }
func (m *mockReactor) ReleaseUpdate(_ uint64, _ string) error                       { return nil }
func (m *mockReactor) DeleteUpdate(_ uint64) error                                  { return nil }
func (m *mockReactor) ForwardUpdate(_ *selector.Selector, _ uint64, _ string) error { return nil }
func (m *mockReactor) ListUpdates() []uint64                                        { return nil }

func (m *mockReactor) SendRawMessage(addr netip.Addr, msgType uint8, payload []byte) error {
	m.rawMessages = append(m.rawMessages, struct {
		addr    netip.Addr
		msgType uint8
		payload []byte
	}{addr, msgType, payload})
	return nil
}

func (m *mockReactor) SendRefresh(_ *selector.Selector, _ uint16, _ uint8) error { return nil }
func (m *mockReactor) SendBoRR(_ *selector.Selector, _ uint16, _ uint8) error    { return nil }
func (m *mockReactor) SendEoRR(_ *selector.Selector, _ uint16, _ uint8) error    { return nil }
func (m *mockReactor) SoftClearPeer(_ *selector.Selector) ([]string, error)      { return nil, nil }

// newTestContext creates a CommandContext backed by a mock reactor.
func newTestContext(reactor plugin.ReactorLifecycle) *pluginserver.CommandContext {
	server, _ := pluginserver.NewServer(&pluginserver.ServerConfig{}, reactor)
	return &pluginserver.CommandContext{Server: server}
}
