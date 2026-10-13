package reactor

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bfd/packet"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// awaitBFDHandleCleared waits until the peer holds no BFD handle, the state the
// subscriber leaves behind when its channel closes under it.
func awaitBFDHandleCleared(t *testing.T, p *Peer) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		p.bfd.mu.Lock()
		held := p.bfd.handle != nil
		p.bfd.mu.Unlock()
		if !held {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("peer still holds its BFD handle 2s after the BFD engine closed the subscription")
}

// VALIDATES: when the BFD engine stops, which closes every subscriber channel,
// the peer forgets the handle, stops reporting a live session, and the next
// startBFDClient asks the service for a new session.
//
// PREVENTS: a peer keeping the handle of a stopped engine forever. Its
// startBFDClient then returned on "handle != nil" and never asked again, and
// bfdSessionState kept reporting a live session whose last reading was the
// one before the engine went away. The fake closes the channel the way
// engine.Loop.Stop does.
func TestBFDClientEngineStopClearsTheHandle(t *testing.T) {
	svc := &fakeBFDService{}
	p, cleanup := newBFDTestPeer(t, &BFDSettings{Enabled: true}, svc)
	defer cleanup()

	p.startBFDClient()
	defer p.stopBFDClient()
	if _, _, live := p.bfdSessionState(); !live {
		t.Fatal("no live BFD session after startBFDClient")
	}

	close(svc.handle.ch)
	awaitBFDHandleCleared(t, p)

	if _, _, live := p.bfdSessionState(); live {
		t.Fatal("bfdSessionState reports a live session after the engine stopped")
	}
	if got := svc.release.Load(); got != 0 {
		t.Fatalf("ReleaseSession calls = %d after the engine stopped, want 0: no loop is left to release into", got)
	}

	// The engine is back: a fresh handle, and the peer asks for it.
	svc.handle = nil
	p.startBFDClient()
	if got := svc.ensure.Load(); got != 2 {
		t.Fatalf("EnsureSession calls = %d, want 2: the peer did not ask again after the engine stopped", got)
	}
	if _, _, live := p.bfdSessionState(); !live {
		t.Fatal("no live BFD session after the reopen")
	}
}

// VALIDATES: the BFD engine stopping under an Established session sends the
// peer nothing and leaves the session Established.
//
// PREVENTS: a BFD shutdown read as a path failure. draft-ietf-idr-bgp-bfd-strict-mode
// Section 4, Event 35: "When BFD has been disabled, the local system will
// trigger a BfdAdminDown event instead", and Section 8.7.1: "The BfdAdminDown,
// BfdDisabled, and BfdUp events are ignored in the Established state.".
func TestBFDClientEngineStopKeepsEstablished(t *testing.T) {
	svc := &fakeBFDService{}
	p, cleanup := newBFDTestPeer(t, &BFDSettings{Enabled: true}, svc)
	defer cleanup()

	session, messages := newEstablishedSessionForPeer(t, p)

	p.startBFDClient()
	defer p.stopBFDClient()

	close(svc.handle.ch)
	awaitBFDHandleCleared(t, p)

	select {
	case msg := <-messages:
		t.Fatalf("an engine stop owes the peer no message, got % x", msg)
	case <-time.After(300 * time.Millisecond):
	}
	if state := session.State(); state != fsm.StateEstablished {
		t.Fatalf("session state = %s, want ESTABLISHED", state)
	}
}

// VALIDATES: a strict peer whose BFD engine stopped is held at its next OPEN,
// the same as a strict peer whose BFD plugin never ran.
//
// PREVENTS: a strict session establishing on a forwarding-path check nobody
// performs, because the last reading before the engine stopped was Up.
func TestBFDClientEngineStopHoldsStrict(t *testing.T) {
	svc := &fakeBFDService{}
	p, cleanup := newBFDTestPeer(t, &BFDSettings{Enabled: true, Strict: true}, svc)
	defer cleanup()

	p.startBFDClient()
	defer p.stopBFDClient()

	// The engine's last word before it stopped was Up, so only the vanished
	// session, not the reading, can hold the gate.
	svc.handle.emit(t, packet.StateUp, packet.DiagNone)
	session := p.newSession()
	session.fsm.SetBFDStrict(true)
	session.setBFDStateReader(p.bfdSessionState)
	deadline := time.Now().Add(2 * time.Second)
	for session.bfdStrictHolds() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if session.bfdStrictHolds() {
		t.Fatal("bfdStrictHolds = true with the BFD session Up; the gate cannot discriminate")
	}

	close(svc.handle.ch)
	awaitBFDHandleCleared(t, p)

	if !session.bfdStrictHolds() {
		t.Fatal("bfdStrictHolds = false after the BFD engine stopped, want the strict gate to hold")
	}
}

// VALIDATES: the BFD engine stopping under a strict session waiting in
// OpenSentBfdUpPending delivers BfdAdminDown, and the session leaves the
// sub-state for OpenConfirm with the withheld KEEPALIVE on the wire.
//
// PREVENTS: the engine-stop path forgetting to tell the FSM. Without the event
// the session waits in OpenSent for a BfdUp no engine will ever send.
// draft-ietf-idr-bgp-bfd-strict-mode Section 4, Event 35: "When BFD has been
// disabled, the local system will trigger a BfdAdminDown event instead", and
// Section 8.5.1: "If the FSM is in the OpenSentBfdUpPending sub-state, the
// local system: changes its state to OpenConfirm".
func TestBFDClientEngineStopDeliversAdminDown(t *testing.T) {
	svc := &fakeBFDService{}
	p, cleanup := newBFDTestPeer(t, &BFDSettings{Enabled: true, Strict: true, HoldTime: 5}, svc)
	defer cleanup()
	p.settings.Connection = ConnectionPassive
	p.settings.ReceiveHoldTime = 90 * time.Second
	p.settings.Capabilities = []capability.Capability{&capability.BFDStrictMode{}}

	// The fake delivers no Initial snapshot, so the reading is Down and live:
	// the OPEN below parks the session in OpenSentBfdUpPending.
	p.startBFDClient()
	defer p.stopBFDClient()

	session := p.newSession()
	session.setBFDStateReader(p.bfdSessionState)
	require.NoError(t, session.Start())
	client, server := net.Pipe()
	t.Cleanup(func() {
		client.Close() //nolint:errcheck // test cleanup
		server.Close() //nolint:errcheck // test cleanup
	})
	_ = acceptWithReader(t, session, server, client)
	messages := drainMessages(t, client)
	require.NoError(t, session.handleOpen(openBodyWithBFDStrict(90)))
	require.Equal(t, fsm.SubStateOpenSentBfdUpPending, session.fsm.BfdSubState())

	p.mu.Lock()
	p.session = session
	p.mu.Unlock()

	close(svc.handle.ch)
	awaitBFDHandleCleared(t, p)

	// The event follows the handle clearing, outside the lock, so wait for it.
	deadline := time.Now().Add(2 * time.Second)
	for session.State() != fsm.StateOpenConfirm && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	require.Equal(t, fsm.StateOpenConfirm, session.State(), "BfdAdminDown did not reach the FSM")
	select {
	case msg := <-messages:
		require.Equal(t, byte(msgtype.TypeKEEPALIVE), msg[18], "the withheld KEEPALIVE is what goes out")
	case <-time.After(time.Second):
		t.Fatal("no KEEPALIVE reached the wire after the BFD engine stopped")
	}
}

// endVanishedAndWait closes the peer's BFD subscription the way engine.Loop.Stop
// does and waits for the subscriber goroutine to return, so every line
// endVanishedBFDSession logs is written before the caller reads the sink.
func endVanishedAndWait(t *testing.T, p *Peer, svc *fakeBFDService) {
	t.Helper()
	p.bfd.mu.Lock()
	done := p.bfd.done
	p.bfd.mu.Unlock()
	if done == nil {
		t.Fatal("no BFD subscriber running")
	}
	close(svc.handle.ch)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("BFD subscriber still running 2s after the engine closed the subscription")
	}
}

// warnsRunsWithoutBFD answers whether the sink holds the warning that the peer
// now runs without BFD.
func warnsRunsWithoutBFD(sink []string) bool {
	for _, line := range sink {
		if strings.Contains(line, "runs without BFD") {
			return true
		}
	}
	return false
}

// VALIDATES: the BFD engine stopping while the daemon runs still warns that
// the peer runs without BFD, and the same close during a daemon stop does not,
// while both clear the handle. The daemon stop is a plugin server whose Stop
// has run, which is what pluginserver.Server.Stop leaves before it closes any
// plugin connection: a zero Server reports Running false the same way.
//
// PREVENTS: every daemon stop that happens to end the BFD plugin before the
// reactor logging a warning that is false, because the peer stops a moment
// later (test/reload/bgp-bfd-profile-reload-peer-and-profile.ci rejects it).
// And the opposite: a guard that silences the warning when the engine stops
// under a running daemon, where the peer really does run without BFD.
func TestBFDClientEngineStopWarnsOnlyWhileDaemonRuns(t *testing.T) {
	cases := []struct {
		name     string
		reactor  func(t *testing.T) *Reactor
		wantWarn bool
	}{
		{"no reactor", func(*testing.T) *Reactor { return nil }, true},
		{"reactor with no plugin server", newTestReactor, true},
		{"plugin server stopped", func(t *testing.T) *Reactor {
			r := newTestReactor(t)
			r.api = &pluginserver.Server{}
			return r
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sink := rfc5882WarnSink(t)
			svc := &fakeBFDService{}
			p, cleanup := newBFDTestPeer(t, &BFDSettings{Enabled: true}, svc)
			defer cleanup()
			if r := tc.reactor(t); r != nil {
				p.SetReactor(r)
			}

			p.startBFDClient()
			defer p.stopBFDClient()
			endVanishedAndWait(t, p, svc)

			if _, _, live := p.bfdSessionState(); live {
				t.Fatal("bfdSessionState reports a live session after the engine stopped")
			}
			if got := warnsRunsWithoutBFD(rfc5882WarnLines(sink)); got != tc.wantWarn {
				t.Fatalf("warned runs without BFD = %v, want %v; log: %q", got, tc.wantWarn, sink.String())
			}
		})
	}
}
