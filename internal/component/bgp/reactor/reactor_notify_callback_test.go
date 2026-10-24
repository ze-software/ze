package reactor

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

// recordingLifecycle keeps what a registry.PeerLifecycleCallback was handed.
type recordingLifecycle struct {
	established any
	closed      any
}

func (r *recordingLifecycle) OnPeerEstablished(peer any)      { r.established = peer }
func (r *recordingLifecycle) OnPeerClosed(peer any, _ string) { r.closed = peer }

// TestPeerLifecycleCallbackReceivesPeerInfo proves the reactor hands a
// registered lifecycle callback the *plugin.PeerInfo its contract names. MRT's
// handlers assert that type; a *Peer made them return in silence, so MRT
// never recorded a BGP state change. Method: drive the adapter the reactor
// registers for AddPeerLifecycleCallback with a real *Peer and read the type
// and address the callback received on both events.
func TestPeerLifecycleCallbackReceivesPeerInfo(t *testing.T) {
	addr := netip.MustParseAddr("192.0.2.7")
	peer := NewPeer(NewPeerSettings(addr, 65000, 65001, 0xc0000201))
	rec := &recordingLifecycle{}
	adapter := &callbackAdapter{cb: rec}

	adapter.OnPeerEstablished(peer)
	adapter.OnPeerClosed(peer, sessionClose{})

	for event, got := range map[string]any{"established": rec.established, "closed": rec.closed} {
		info, ok := got.(*plugin.PeerInfo)
		if !ok {
			t.Fatalf("%s: callback received %T, want *plugin.PeerInfo", event, got)
		}
		if info.Address != addr {
			t.Fatalf("%s: callback received address %v, want %v", event, info.Address, addr)
		}
	}
}
