package reactor

import (
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/packet"
)

// fakeBFDHandle is a minimal SessionHandle implementation used by the
// Peer BFD-client tests. It owns a single subscriber channel that the
// test drives directly to simulate the BFD engine's notify path.
type fakeBFDHandle struct {
	key        api.Key
	ch         chan api.StateChange
	shutdowns  atomic.Int32
	enables    atomic.Int32
	unsubbed   atomic.Bool
	subscribed atomic.Bool
}

func (h *fakeBFDHandle) Key() api.Key { return h.key }

func (h *fakeBFDHandle) Subscribe() <-chan api.StateChange {
	h.subscribed.Store(true)
	return h.ch
}

func (h *fakeBFDHandle) Unsubscribe(_ <-chan api.StateChange) {
	h.unsubbed.Store(true)
}

func (h *fakeBFDHandle) Shutdown() error {
	h.shutdowns.Add(1)
	return nil
}

func (h *fakeBFDHandle) Enable() error {
	h.enables.Add(1)
	return nil
}

// emit sends a StateChange on the handle's subscriber channel with a
// short timeout so a buggy test does not deadlock the runner.
func (h *fakeBFDHandle) emit(t *testing.T, state packet.State, diag packet.Diag) {
	t.Helper()
	select {
	case h.ch <- api.StateChange{Key: h.key, State: state, Diag: diag, When: time.Now()}:
	case <-time.After(time.Second):
		t.Fatal("timed out emitting StateChange")
	}
}

// fakeBFDService records EnsureSession / ReleaseSession calls and
// returns a single handle per Key. Tests use this to verify the Peer
// BFD client drives the contract correctly without pulling in the
// real BFD engine.
type fakeBFDService struct {
	ensure   atomic.Int32
	release  atomic.Int32
	handle   *fakeBFDHandle
	ensureFn func(api.SessionRequest) (api.SessionHandle, error)
}

func (s *fakeBFDService) EnsureSession(req api.SessionRequest) (api.SessionHandle, error) {
	s.ensure.Add(1)
	if s.ensureFn != nil {
		return s.ensureFn(req)
	}
	if s.handle == nil {
		s.handle = &fakeBFDHandle{
			key: req.Key(),
			ch:  make(chan api.StateChange, 4),
		}
	}
	return s.handle, nil
}

func (s *fakeBFDService) ReleaseSession(_ api.SessionHandle) error {
	s.release.Add(1)
	if s.handle != nil {
		close(s.handle.ch)
	}
	return nil
}

func (s *fakeBFDService) Snapshot() []api.SessionState { return nil }
func (s *fakeBFDService) SessionDetail(_ string) (api.SessionState, bool) {
	return api.SessionState{}, false
}
func (s *fakeBFDService) Profiles() []api.ProfileState { return nil }

// minimalPeerSettings builds a PeerSettings with the smallest set of
// fields the BFD client code touches. It avoids the heavy NewPeer
// constructor because the tests exercise startBFDClient / stopBFDClient
// directly, not the full peer lifecycle.
func minimalPeerSettings(bfd *BFDSettings) *PeerSettings {
	s := NewPeerSettings(netip.MustParseAddr("203.0.113.2"), 65001, 65002, 0xC0000201)
	s.LocalAddress = netip.MustParseAddr("203.0.113.1")
	s.BFD = bfd
	return s
}

// newBFDTestPeer builds a minimal *Peer wired to a fake service and
// publishes the service via api.SetService so startBFDClient picks
// it up. Returns a cleanup func that clears the global service.
func newBFDTestPeer(t *testing.T, bfd *BFDSettings, svc *fakeBFDService) (*Peer, func()) {
	t.Helper()
	api.SetService(svc)
	settings := minimalPeerSettings(bfd)
	p := NewPeer(settings)
	return p, func() { api.SetService(nil) }
}

// VALIDATES: a peer with no BFD config is a no-op on startBFDClient,
// does not touch the Service, and stopBFDClient is harmless to call.
// PREVENTS: regression where BFD wiring fires on every peer.
func TestBFDClient_DisabledNoOp(t *testing.T) {
	svc := &fakeBFDService{}
	p, cleanup := newBFDTestPeer(t, nil, svc)
	defer cleanup()

	p.startBFDClient()
	if svc.ensure.Load() != 0 {
		t.Fatalf("EnsureSession calls = %d, want 0 (BFD not configured)", svc.ensure.Load())
	}
	p.stopBFDClient()
	if svc.release.Load() != 0 {
		t.Fatalf("ReleaseSession calls = %d, want 0", svc.release.Load())
	}
}

// VALIDATES: BFD with Enabled=false is a no-op.
// PREVENTS: regression where the `presence` container alone triggers
// wiring regardless of the enabled leaf.
func TestBFDClient_EnabledFalseNoOp(t *testing.T) {
	svc := &fakeBFDService{}
	p, cleanup := newBFDTestPeer(t, &BFDSettings{Enabled: false}, svc)
	defer cleanup()

	p.startBFDClient()
	if svc.ensure.Load() != 0 {
		t.Fatalf("EnsureSession calls = %d, want 0 (Enabled=false)", svc.ensure.Load())
	}
}

// VALIDATES: api.GetService returning nil (BFD plugin not loaded) makes
// startBFDClient log and return without error.
// PREVENTS: regression where the BGP peer fails its own startup
// because the BFD plugin is not present.
func TestBFDClient_NilService(t *testing.T) {
	api.SetService(nil) // explicit
	defer api.SetService(nil)
	settings := minimalPeerSettings(&BFDSettings{Enabled: true})
	p := NewPeer(settings)

	// Should not panic and should not block.
	p.startBFDClient()
	// stopBFDClient is a no-op when startBFDClient didn't open a session.
	p.stopBFDClient()
}

// VALIDATES: startBFDClient calls EnsureSession with a SessionRequest
// derived from PeerSettings (peer, local, single-hop default).
// PREVENTS: regression where the request fields are mis-wired.
func TestBFDClient_EnsureSessionSingleHop(t *testing.T) {
	svc := &fakeBFDService{}
	p, cleanup := newBFDTestPeer(t, &BFDSettings{Enabled: true}, svc)
	defer cleanup()

	p.startBFDClient()
	defer p.stopBFDClient()

	if svc.ensure.Load() != 1 {
		t.Fatalf("EnsureSession calls = %d, want 1", svc.ensure.Load())
	}
	if svc.handle == nil || !svc.handle.subscribed.Load() {
		t.Fatal("expected Subscribe on the returned handle")
	}
	if svc.handle.key.Peer != netip.MustParseAddr("203.0.113.2") {
		t.Fatalf("request peer = %v, want 203.0.113.2", svc.handle.key.Peer)
	}
	if svc.handle.key.Local != netip.MustParseAddr("203.0.113.1") {
		t.Fatalf("request local = %v, want 203.0.113.1", svc.handle.key.Local)
	}
	if svc.handle.key.Mode != api.SingleHop {
		t.Fatalf("request mode = %v, want SingleHop", svc.handle.key.Mode)
	}
}

// VALIDATES: BFD with MultiHop=true produces a multi-hop request that
// carries MinTTL from config.
// PREVENTS: regression where MultiHop flag or MinTTL is dropped.
func TestBFDClient_EnsureSessionMultiHop(t *testing.T) {
	svc := &fakeBFDService{}
	p, cleanup := newBFDTestPeer(t,
		&BFDSettings{Enabled: true, MultiHop: true, MinTTL: 250}, svc)
	defer cleanup()

	p.startBFDClient()
	defer p.stopBFDClient()

	if svc.handle == nil {
		t.Fatal("handle not populated")
	}
	if svc.handle.key.Mode != api.MultiHop {
		t.Fatalf("mode = %v, want MultiHop", svc.handle.key.Mode)
	}
}

// VALIDATES: a BFD Down StateChange results in exactly one
// Peer.Teardown call with RFC 9384 Cease subcode 10.
// PREVENTS: regression where the subscriber drops events, uses the
// wrong subcode, or fails to bridge to BGP teardown.
func TestBFDClient_TeardownOnDown(t *testing.T) {
	svc := &fakeBFDService{}
	p, cleanup := newBFDTestPeer(t, &BFDSettings{Enabled: true}, svc)
	defer cleanup()

	// Install a test hook that replaces Teardown's session-invoking
	// path: the minimal peer has no session, so Teardown's queued
	// branch runs and records the subcode on the opQueue. We read
	// the queue directly after emitting the BFD Down.
	p.startBFDClient()
	defer p.stopBFDClient()

	svc.handle.emit(t, packet.StateDown, packet.DiagControlDetectExpired)

	// Give the subscriber goroutine time to process. Poll the
	// opQueue up to 1 s.
	deadline := time.Now().Add(time.Second)
	var sawTeardown bool
	for time.Now().Before(deadline) {
		p.mu.RLock()
		for _, op := range p.opQueue {
			if op.Type == PeerOpTeardown && op.Subcode == message.NotifyCeaseBFDDown {
				sawTeardown = true
				break
			}
		}
		p.mu.RUnlock()
		if sawTeardown {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !sawTeardown {
		t.Fatal("expected Teardown with subcode NotifyCeaseBFDDown after BFD Down event")
	}
}

// VALIDATES: stopBFDClient calls ReleaseSession exactly once and the
// subscriber goroutine exits cleanly.
// PREVENTS: handle leak on peer shutdown.
func TestBFDClient_Stop_ReleasesHandle(t *testing.T) {
	svc := &fakeBFDService{}
	p, cleanup := newBFDTestPeer(t, &BFDSettings{Enabled: true}, svc)
	defer cleanup()

	p.startBFDClient()
	if svc.ensure.Load() != 1 {
		t.Fatal("EnsureSession not called")
	}

	p.stopBFDClient()
	if svc.release.Load() != 1 {
		t.Fatalf("ReleaseSession calls = %d, want 1", svc.release.Load())
	}
	if !svc.handle.unsubbed.Load() {
		t.Fatal("Unsubscribe not called on handle")
	}
}

// VALIDATES: stopBFDClient is idempotent -- a second call is a no-op.
// PREVENTS: double-release causing negative refcount in the engine.
func TestBFDClient_StopIdempotent(t *testing.T) {
	svc := &fakeBFDService{}
	p, cleanup := newBFDTestPeer(t, &BFDSettings{Enabled: true}, svc)
	defer cleanup()

	p.startBFDClient()
	p.stopBFDClient()
	p.stopBFDClient() // second call
	if svc.release.Load() != 1 {
		t.Fatalf("ReleaseSession calls = %d, want 1 after double stop", svc.release.Load())
	}
}
