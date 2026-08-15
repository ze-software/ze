package server

import (
	"context"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ErrPeerNotFound is a test error matching reactor.ErrPeerNotFound.
// Cannot import reactor due to import cycle (reactor imports api).
var ErrPeerNotFound = errors.New("peer not found")

// mockReactor implements ReactorLifecycle for testing.
type mockReactor struct {
	peers         []plugin.PeerInfo
	stats         plugin.ReactorStats
	stopped       bool
	teardownCalls []struct {
		addr    netip.Addr
		subcode uint8
		message string
	}
	removedPeers []netip.Addr

	// relayCalls records every RelayStoredRoute the server dispatched, so a
	// wiring test can prove the RPC reached the coordinator with the payload
	// intact rather than merely returning nil.
	relayCalls []mockRelayCall
	relayErr   error

	// forwardCalls records every ForwardUpdatesDirect the server dispatched, for
	// the same reason.
	forwardCalls []mockForwardCall
}

// mockRelayCall is one recorded RelayStoredRoute dispatch.
//
// sender is recorded because it is the AUTHORITY the reactor judges: the peer's
// attach block is looked up by that name (bgp/reactor/send_permission.go,
// filterPermittedPeers). An op that dropped it, or that named the operator
// instead of the calling process, would look identical here without this field.
type mockRelayCall struct {
	destination netip.Addr
	routes      []rpc.StoredRoute
	sender      plugin.Sender
}

// mockForwardCall is one recorded ForwardUpdatesDirect dispatch.
type mockForwardCall struct {
	ids          []uint64
	destinations []netip.AddrPort
	pluginName   string
	sender       plugin.Sender
}

func (m *mockReactor) Peers() []plugin.PeerInfo {
	return m.peers
}

func (m *mockReactor) Stats() plugin.ReactorStats {
	return m.stats
}

func (m *mockReactor) GetPeerProcessBindings(_ netip.Addr) []plugin.PeerProcessBinding {
	return nil
}

func (m *mockReactor) GetPeerCapabilityConfigs() []plugin.PeerCapabilityConfig {
	return nil
}

func (m *mockReactor) PeerNegotiatedCapabilities(_ netip.Addr) *plugin.PeerCapabilitiesInfo {
	return nil
}

func (m *mockReactor) Stop() {
	m.stopped = true
}

func (m *mockReactor) TeardownPeer(addr netip.Addr, subcode uint8, shutdownMsg string) error {
	m.teardownCalls = append(m.teardownCalls, struct {
		addr    netip.Addr
		subcode uint8
		message string
	}{addr, subcode, shutdownMsg})
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

func (m *mockReactor) RemovePeer(addr netip.Addr) error {
	m.removedPeers = append(m.removedPeers, addr)
	return nil
}

func (m *mockReactor) AddDynamicPeer(_ netip.Addr, _ map[string]any) error { return nil }

func (m *mockReactor) GetConfigTree() map[string]any {
	return nil
}

func (m *mockReactor) SetConfigTree(_ map[string]any) {}

func (m *mockReactor) SignalAPIReady() {}

func (m *mockReactor) AddAPIProcessCount(_ int) {}

func (m *mockReactor) SignalPluginStartupComplete() {}

func (m *mockReactor) SignalPeerAPIReady(_ string)      {}
func (m *mockReactor) SetPeerUpBarrier(_ string, _ int) {}
func (m *mockReactor) SignalPeerUpBarrier(_ string)     {}

func (m *mockReactor) PausePeer(_ netip.Addr) error  { return nil }
func (m *mockReactor) ResumePeer(_ netip.Addr) error { return nil }

func (m *mockReactor) FlushForwardPool(_ context.Context) error               { return nil }
func (m *mockReactor) FlushForwardPoolPeer(_ context.Context, _ string) error { return nil }
func (m *mockReactor) DrainPeerSync(_ context.Context) error                  { return nil }

func (m *mockReactor) RegisterCacheConsumer(_ string, _ bool) {}

func (m *mockReactor) UnregisterCacheConsumer(_ string) {}

// ForwardUpdatesDirect implements plugin.ReactorCacheCoordinator, recording the
// dispatch so a wiring test can assert what crossed the RPC boundary.
func (m *mockReactor) ForwardUpdatesDirect(ids []uint64, destinations []netip.AddrPort, pluginName string, sender plugin.Sender) error {
	m.forwardCalls = append(m.forwardCalls, mockForwardCall{
		ids: ids, destinations: destinations, pluginName: pluginName, sender: sender,
	})
	return nil
}

func (m *mockReactor) ReleaseUpdates(_ []uint64, _ string) error { return nil }

// RelayStoredRoute implements plugin.ReactorRelayCoordinator, recording the
// dispatch so a wiring test can assert the payload survived the RPC boundary.
func (m *mockReactor) RelayStoredRoute(destination netip.Addr, routes []rpc.StoredRoute, sender plugin.Sender) error {
	m.relayCalls = append(m.relayCalls, mockRelayCall{destination: destination, routes: routes, sender: sender})
	return m.relayErr
}
