package server

import (
	"context"
	"errors"
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
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

func (m *mockReactor) SignalPeerAPIReady(_ string) {}

func (m *mockReactor) PausePeer(_ netip.Addr) error  { return nil }
func (m *mockReactor) ResumePeer(_ netip.Addr) error { return nil }

func (m *mockReactor) FlushForwardPool(_ context.Context) error               { return nil }
func (m *mockReactor) FlushForwardPoolPeer(_ context.Context, _ string) error { return nil }

func (m *mockReactor) RegisterCacheConsumer(_ string, _ bool) {}

func (m *mockReactor) UnregisterCacheConsumer(_ string) {}
