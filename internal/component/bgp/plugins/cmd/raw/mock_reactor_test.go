package raw

import (
	"context"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/pkg/plugin/rpc"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

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

// mockReactor implements plugin.ReactorLifecycle + bgptypes.BGPReactor for handler tests.
type mockReactor struct {
	rawMessages []rawCall
}

// errUnnamedSender is what this mock returns for the zero Sender, standing in
// for the reactor's errSendNoSender (bgp/reactor/send_permission.go), which is
// unexported and lives in a package this one cannot import.
var errUnnamedSender = errors.New("send refused: the command names no sender, so no attach block can permit it")

// rawCall is one SendRawMessage the handler reached the reactor with. The sender
// is recorded because it is the authority the real rail judges: a handler that
// dropped it, or replaced it with the operator's, would look identical here
// without this field.
type rawCall struct {
	addr    netip.Addr
	msgType uint8
	payload []byte
	sender  plugin.Sender
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
func (m *mockReactor) ForwardUpdatesDirect(_ []uint64, _ []netip.AddrPort, _ string, _ plugin.Sender) error {
	return nil
}

// RelayStoredRoute satisfies plugin.ReactorRelayCoordinator; this stub relays
// nothing because these tests exercise command dispatch, not the forward rail.
func (m *mockReactor) RelayStoredRoute(_ netip.Addr, _ []rpc.StoredRoute, _ plugin.Sender) error {
	return nil
}
func (m *mockReactor) ReleaseUpdates(_ []uint64, _ string) error { return nil }

func (m *mockReactor) PausePeer(_ netip.Addr) error                           { return nil }
func (m *mockReactor) ResumePeer(_ netip.Addr) error                          { return nil }
func (m *mockReactor) FlushForwardPool(_ context.Context) error               { return nil }
func (m *mockReactor) FlushForwardPoolPeer(_ context.Context, _ string) error { return nil }
func (m *mockReactor) DrainPeerSync(_ context.Context) error                  { return nil }
func (m *mockReactor) TeardownPeer(_ netip.Addr, _ uint8, _ string) error     { return nil }
func (m *mockReactor) RemovePeer(_ netip.Addr) error                          { return nil }
func (m *mockReactor) AddDynamicPeer(_ netip.Addr, _ map[string]any) error    { return nil }
func (m *mockReactor) AnnounceEOR(_ *selector.Selector, _ uint16, _ uint8, _ plugin.Sender) error {
	return nil
}
func (m *mockReactor) AnnounceNLRIBatch(_ *selector.Selector, _ bgptypes.NLRIBatch, _ plugin.Sender) error {
	return nil
}
func (m *mockReactor) WithdrawNLRIBatch(_ *selector.Selector, _ bgptypes.NLRIBatch, _ plugin.Sender) error {
	return nil
}
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

func (m *mockReactor) RetainUpdate(_ uint64) error            { return nil }
func (m *mockReactor) ReleaseUpdate(_ uint64, _ string) error { return nil }
func (m *mockReactor) DeleteUpdate(_ uint64) error            { return nil }
func (m *mockReactor) ForwardUpdate(_ *selector.Selector, _ uint64, _ string, _ plugin.Sender) error {
	return nil
}
func (m *mockReactor) ListUpdates() []uint64 { return nil }

// SendRawMessage records the call, and refuses an unnamed sender exactly where
// the real rail refuses it.
//
// The rule is the reactor's, not this mock's: (*reactorAPIAdapter).SendRawMessage
// (bgp/reactor/reactor_api.go) refuses the zero Sender before it looks the peer
// up, because there is no attach block to consult for a command nobody claimed.
// This package cannot call that function -- the reactor imports it, so the
// dependency runs one way -- and the real guard is proven over real peers in
// TestPeerRawEntryPointRefusesAnUnattachedProcess
// (bgp/reactor/send_permission_rails_test.go). What the mirror buys here is the
// handler's own obligation: the dispatch chain must carry ctx.Sender to the rail
// unchanged, and a handler that drops it now fails at this entry point instead
// of looking like a success.
func (m *mockReactor) SendRawMessage(addr netip.Addr, msgType uint8, payload []byte, sender plugin.Sender) error {
	if !sender.IsSet() {
		return errUnnamedSender
	}
	m.rawMessages = append(m.rawMessages, rawCall{addr, msgType, payload, sender})
	return nil
}

func (m *mockReactor) SendRefresh(_ *selector.Selector, _ uint16, _ uint8, _ plugin.Sender) error {
	return nil
}
func (m *mockReactor) SendBoRR(_ *selector.Selector, _ uint16, _ uint8, _ plugin.Sender) error {
	return nil
}
func (m *mockReactor) SendEoRR(_ *selector.Selector, _ uint16, _ uint8, _ plugin.Sender) error {
	return nil
}
func (m *mockReactor) SoftClearPeer(_ *selector.Selector, _ plugin.Sender) ([]string, error) {
	return nil, nil
}

// newTestContext creates a CommandContext backed by a mock reactor, issued by an
// operator.
//
// The sender is stated rather than left zero because every production path
// states one (cmd/ze/hub/main_servers.go for an operator, the plugin server's
// dispatch paths for a process), and the zero value is the third state: a
// command nobody claimed, which the rail refuses. A test about hex decoding must
// not sit on that state by accident.
func newTestContext(reactor plugin.ReactorLifecycle) *pluginserver.CommandContext {
	server, _ := pluginserver.NewServer(&pluginserver.ServerConfig{}, reactor)
	return &pluginserver.CommandContext{Server: server, Sender: plugin.OperatorSender()}
}
