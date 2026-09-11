package reactor

import (
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/packet"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
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

// VALIDATES: a BFD Down StateChange on a peer with a live Established session
// closes that session with RFC 9384 Cease subcode 10.
//
// PREVENTS: regression where the subscriber drops events, uses the wrong
// subcode, or fails to bridge to BGP teardown.
//
// It asserts the wire rather than the opQueue, which is what the case used to
// read. draft-ietf-idr-bgp-bfd-strict-mode Section 8.2 makes every BFD event
// IGNORED in Idle, and a peer with no session is Idle, so the queued teardown
// the old assertion looked for is now a teardown of nothing: it would fire the
// NOTIFICATION at whichever session established next.
func TestBFDClient_TeardownOnDown(t *testing.T) {
	svc := &fakeBFDService{}
	p, cleanup := newBFDTestPeer(t, &BFDSettings{Enabled: true}, svc)
	defer cleanup()

	session, messages := newEstablishedSessionForPeer(t, p)

	p.startBFDClient()
	defer p.stopBFDClient()

	svc.handle.emit(t, packet.StateDown, packet.DiagControlDetectExpired)

	select {
	case msg := <-messages:
		if msg[18] != byte(msgtype.TypeNOTIFICATION) {
			t.Fatalf("message type = %d, want NOTIFICATION", msg[18])
		}
		if msg[19] != byte(message.NotifyCease) || msg[20] != message.NotifyCeaseBFDDown {
			t.Fatalf("code/subcode = %d/%d, want Cease/BFD Down", msg[19], msg[20])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected a Cease / BFD Down NOTIFICATION after the BFD Down event")
	}

	// Session.teardown writes the NOTIFICATION before it fires the FSM event, so
	// the state follows the wire rather than leading it. Poll rather than read
	// once (internal/component/bgp/reactor/session_connection.go, teardown).
	deadline := time.Now().Add(2 * time.Second)
	for session.State() != fsm.StateIdle && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if state := session.State(); state != fsm.StateIdle {
		t.Fatalf("session state = %s, want IDLE", state)
	}
}

// VALIDATES: a BFD AdminDown StateChange leaves an Established session alone.
//
// PREVENTS: an operator disabling BFD for maintenance dropping every BGP
// session that used it. draft-ietf-idr-bgp-bfd-strict-mode Section 8.7.1: "The
// BfdAdminDown, BfdDisabled, and BfdUp events are ignored in the Established
// state", which restates RFC 5882 Section 4.2: "If a BFD session transitions
// from Up state to AdminDown ... clients SHOULD NOT take any control protocol
// action." Ze tore the session down on AdminDown until this test existed.
func TestBFDClientAdminDownDoesNotTeardown(t *testing.T) {
	svc := &fakeBFDService{}
	p, cleanup := newBFDTestPeer(t, &BFDSettings{Enabled: true}, svc)
	defer cleanup()

	session, messages := newEstablishedSessionForPeer(t, p)

	p.startBFDClient()
	defer p.stopBFDClient()

	svc.handle.emit(t, packet.StateAdminDown, packet.DiagAdminDown)

	select {
	case msg := <-messages:
		t.Fatalf("AdminDown owes the peer no message, got % x", msg)
	case <-time.After(300 * time.Millisecond):
	}

	if state := session.State(); state != fsm.StateEstablished {
		t.Fatalf("session state = %s, want ESTABLISHED", state)
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

// VALIDATES: runBFDSubscriber re-stamps the BFD entry time only on a state
// CHANGE, so an Up, Down, Up sequence leaves the entry time at the SECOND Up
// and the draft-ietf-idr-bgp-bfd-strict-mode Section 10 hold-down interval is
// owed again in full.
//
// PREVENTS: a flapping link establishing early by accumulating credit across
// its own outages, which would make the damping report success on exactly the
// link it exists to refuse. This drives the real producer: an earlier version
// of this case set the entry time by hand and asserted what it had just
// written, so the re-stamp never executed and the claim was unproven.
func TestBFDClientReStampsTheEntryTimeOnlyOnAChange(t *testing.T) {
	svc := &fakeBFDService{}
	p, cleanup := newBFDTestPeer(t, &BFDSettings{Enabled: true, Strict: true, HoldDown: 400}, svc)
	defer cleanup()

	p.startBFDClient()
	defer p.stopBFDClient()

	read := func() (api.State, time.Time) {
		t.Helper()
		state, since, live := p.bfdSessionState()
		if !live {
			t.Fatal("the BFD session is not live")
		}
		return state, since
	}

	waitFor := func(want api.State) time.Time {
		t.Helper()
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if state, since := read(); state == want {
				return since
			}
			time.Sleep(5 * time.Millisecond)
		}
		state, _ := read()
		t.Fatalf("BFD state = %v, want %v", state, want)
		return time.Time{}
	}

	svc.handle.emit(t, packet.StateUp, packet.DiagNone)
	firstUp := waitFor(api.StateUp)

	// The SAME state again must not move the entry time: Section 10 measures
	// how long the session has been Up, and a repeated report is not a new Up.
	svc.handle.emit(t, packet.StateUp, packet.DiagNone)
	time.Sleep(20 * time.Millisecond)
	if _, since := read(); !since.Equal(firstUp) {
		t.Fatalf("a repeated Up moved the entry time from %v to %v", firstUp, since)
	}

	// The flap. Down, then Up again: the interval restarts at the second Up.
	svc.handle.emit(t, packet.StateDown, packet.DiagControlDetectExpired)
	waitFor(api.StateDown)
	svc.handle.emit(t, packet.StateUp, packet.DiagNone)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		state, since := read()
		if state == api.StateUp && since.After(firstUp) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	_, since := read()
	t.Fatalf("after a flap the entry time is still %v (first Up was %v), so the interval would count time the session spent down",
		since, firstUp)
}
