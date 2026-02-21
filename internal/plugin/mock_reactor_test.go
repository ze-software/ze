package plugin

import (
	"errors"
	"net/netip"
)

// ErrPeerNotFound is a test error matching reactor.ErrPeerNotFound.
// Cannot import reactor due to import cycle (reactor imports api).
var ErrPeerNotFound = errors.New("peer not found")

// mockReactor implements ReactorLifecycle for testing.
type mockReactor struct {
	peers         []PeerInfo
	stats         ReactorStats
	stopped       bool
	teardownCalls []struct {
		addr    netip.Addr
		subcode uint8
	}
	addedPeers   []DynamicPeerConfig
	removedPeers []netip.Addr
}

func (m *mockReactor) Peers() []PeerInfo {
	return m.peers
}

func (m *mockReactor) Stats() ReactorStats {
	return m.stats
}

func (m *mockReactor) GetPeerProcessBindings(_ netip.Addr) []PeerProcessBinding {
	return nil
}

func (m *mockReactor) GetPeerCapabilityConfigs() []PeerCapabilityConfig {
	return nil
}

func (m *mockReactor) Stop() {
	m.stopped = true
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

func (m *mockReactor) RegisterCacheConsumer(_ string) {}

func (m *mockReactor) UnregisterCacheConsumer(_ string) {}
