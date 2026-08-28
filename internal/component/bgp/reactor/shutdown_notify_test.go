// VALIDATES: an administrative stop of the reactor puts a Cease / Administrative
// Shutdown NOTIFICATION on every live session BEFORE the context cancel closes
// the sockets, in every state that can carry one (RFC 4271 Section 8.2.2,
// ManualStop, Event 2; RFC 4486 subcode 2).
//
// PREVENTS: the shutdown NOTIFICATION going back to being unreachable. It was
// built by a Session.Close that Peer.cleanup called behind `if p.session != nil`,
// and runOnce's own defer had already nil'd p.session, so the send never
// happened and nothing logged that it had not. See
// plan/spec-fixit-bgp-shutdown-cease-notification.md.

package reactor

import (
	"context"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// feedPeer writes one message to the peer end of a net.Pipe from its own
// goroutine, because net.Pipe is unbuffered and the session reads it inline.
// A write error means the session already closed the socket, which the caller
// detects on its own assertion; there is nothing useful to do here.
func feedPeer(conn net.Conn, msg []byte, drain bool) {
	go func() {
		if _, err := conn.Write(msg); err != nil {
			return
		}
		if !drain {
			return
		}
		buf := make([]byte, 4096)
		if _, err := conn.Read(buf); err != nil {
			return
		}
	}()
}

// shutdownTestSession returns a session whose conn is one end of a net.Pipe,
// driven to `want`. The caller reads the peer's view from the returned conn.
//
// OpenSent is what Accept alone reaches: the OPEN has gone out and no answer
// has come back. The two further states are reached by feeding the answers,
// which is the same sequence collision_test.go uses.
func shutdownTestSession(t *testing.T, want fsm.State) (*Session, net.Conn) {
	t.Helper()

	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020304,
	)
	settings.Connection = ConnectionPassive

	session := NewSession(settings)
	require.NoError(t, session.Start())

	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })
	_ = acceptWithReader(t, session, server, client)
	require.Equal(t, fsm.StateOpenSent, session.State())

	if want == fsm.StateOpenSent {
		return session, client
	}

	feedPeer(client, message.PackTo(&message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x01020302,
	}, nil), true)
	require.NoError(t, session.ReadAndProcess())
	require.Equal(t, fsm.StateOpenConfirm, session.State())

	if want == fsm.StateOpenConfirm {
		return session, client
	}

	feedPeer(client, message.PackTo(message.NewKeepalive(), nil), false)
	require.NoError(t, session.ReadAndProcess())
	require.Equal(t, fsm.StateEstablished, session.State())

	return session, client
}

// readOne reads one message off conn. The channel closes without a value when
// the socket ends before anything arrives, which is exactly the defect's
// signature: a bare close with no NOTIFICATION on it.
func readOne(conn net.Conn) <-chan []byte {
	got := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 4096)
		n, err := conn.Read(buf)
		if err != nil && n == 0 {
			close(got)
			return
		}
		got <- buf[:n]
	}()
	return got
}

// attachPeer registers a peer holding this session in the reactor, the way a
// running peer's runOnce would have.
func attachPeer(t *testing.T, r *Reactor, session *Session) {
	t.Helper()

	peer := NewPeer(session.settings)
	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()

	r.mu.Lock()
	r.peers[peerKeyFromAddrPort(session.settings.Address, session.settings.Port)] = peer
	r.mu.Unlock()
}

// TestReactorStopSendsAdminShutdownBeforeCancel is the AC-1 wire assertion at
// the unit level: the peer reads the exact 45 octets of Cease / Administrative
// Shutdown off a session running under a real child of the reactor context.
//
// It is NOT the ordering guard, and it must not be read as one. What it
// observes is the peer's END of the pipe, which races Stop's own cancel(): the
// bytes the notify already put on the pipe are still readable after the cancel
// has fired. Measured with the send moved after cancel(), over `-count=50
// -race`, this test went red 28 times and green 22. The deterministic guard is
// TestReactorStopNotifiesWhileItsContextIsStillLive below, which observes the
// ordering from inside the send instead of from the far end of the socket.
func TestReactorStopSendsAdminShutdownBeforeCancel(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
	require.NoError(t, r.Start())

	session, client := shutdownTestSession(t, fsm.StateEstablished)
	attachPeer(t, r, session)

	r.mu.Lock()
	peerCtx, peerCancel := context.WithCancel(r.ctx)
	r.mu.Unlock()
	t.Cleanup(peerCancel)

	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		_ = session.Run(peerCtx)
	}()

	got := readOne(client)
	r.Stop()

	select {
	case msg, ok := <-got:
		require.True(t, ok, "socket closed with no NOTIFICATION on it")
		require.Equal(t, adminShutdownNotificationWire(), msg)
	case <-time.After(5 * time.Second):
		t.Fatal("reactor stopped without telling the peer why")
	}

	select {
	case <-runDone:
	case <-time.After(5 * time.Second):
		t.Fatal("session did not exit after the reactor stopped")
	}

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer waitCancel()
	_ = r.Wait(waitCtx)
}

// RFC requirement: RFC4271-8.2.2-18 positive -- ManualStop (Event 2) sends the
// Cease NOTIFICATION in every state that holds a connection. OpenSent writes
// "sends the NOTIFICATION with a Cease" and OpenConfirm and Established write
// "sends the NOTIFICATION message with a Cease"; the wording differs, the
// obligation does not, so all three are driven here rather than Established
// alone. An operator who stops the daemon mid-handshake is owed the same reason
// as one who stops it with the session up.
func TestShutdownNotifySendsCeaseFromEveryConnectedState(t *testing.T) {
	for _, state := range []fsm.State{
		fsm.StateOpenSent, fsm.StateOpenConfirm, fsm.StateEstablished,
	} {
		t.Run(state.String(), func(t *testing.T) {
			session, client := shutdownTestSession(t, state)

			peer := NewPeer(session.settings)
			peer.mu.Lock()
			peer.session = session
			peer.mu.Unlock()

			got := readOne(client)
			peer.shutdownNotify()

			select {
			case msg, ok := <-got:
				require.True(t, ok, "socket closed with no NOTIFICATION on it")
				require.Equal(t, adminShutdownNotificationWire(), msg)
			case <-time.After(5 * time.Second):
				t.Fatalf("no NOTIFICATION sent from %s", state)
			}
		})
	}
}

// RFC requirement: RFC4271-8.2.2-18 negative -- the Cease belongs to the STOP,
// not to a session that merely exists. A reactor nobody has stopped has raised
// no ManualStop, so its peer must be told nothing: a speaker that writes Cease
// while the FSM is sitting in Established would end a session RFC 4271 Section
// 8.2.2 never reached, and Section 6.7 forbids Cease where a fatal error is the
// real reason. The assertion is that ZERO octets reach the peer, which is what
// fails if the send is ever hoisted out of the stop path.
func TestRFC4271NoCeaseWithoutAManualStop(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
	require.NoError(t, r.Start())
	t.Cleanup(r.Stop)

	session, client := shutdownTestSession(t, fsm.StateEstablished)
	attachPeer(t, r, session)

	got := readOne(client)

	select {
	case chunk, ok := <-got:
		if ok {
			t.Fatalf("ze wrote %d octets to a peer of a reactor nobody stopped: "+
				"the Cease NOTIFICATION is bound to ManualStop (Event 2), not to a "+
				"session being up", len(chunk))
		}
		t.Fatal("the session's socket closed with no stop having been issued")
	case <-time.After(500 * time.Millisecond):
	}
}

// TestShutdownNotifyWithoutSessionIsQuiet is the AC-2 half that has no socket at
// all: a peer that never connected must not queue a teardown nobody will drain,
// and must not panic on a nil session.
func TestShutdownNotifyWithoutSessionIsQuiet(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.9"),
		65001, 65002, 0x01020304,
	)
	peer := NewPeer(settings)

	peer.shutdownNotify()

	peer.mu.Lock()
	queued := len(peer.opQueue)
	peer.mu.Unlock()
	require.Zero(t, queued,
		"a shutdown notification must not be queued for a drain that never comes")
}

// TestReactorStopStaysInBudgetWithUnreadablePeer is AC-2.
//
// The peer here reads nothing at all, so the NOTIFICATION write blocks on an
// unbuffered net.Pipe until its own control-message deadline, which is 10s at
// the shortest -- more than three times the hub's whole 3s engine budget. Stop
// must abandon that send rather than wait for it.
func TestReactorStopStaysInBudgetWithUnreadablePeer(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
	require.NoError(t, r.Start())

	session, _ := shutdownTestSession(t, fsm.StateEstablished)
	attachPeer(t, r, session)

	started := time.Now()
	r.Stop()
	elapsed := time.Since(started)

	require.Less(t, elapsed, shutdownNotifyBudget+2*time.Second,
		"Stop waited on a peer that cannot be written to")

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer waitCancel()
	_ = r.Wait(waitCtx)
}

// TestReactorStopNotifiesWhileItsContextIsStillLive is the ordering guard, and
// it is deterministic where an assertion about what the PEER reads is not.
//
// The reactor context is read from inside the send, on the goroutine doing it,
// through the onNotifSent hook that fires at the end of a successful
// NOTIFICATION write (session_write.go). Program order fixes the answer,
// nothing schedules it:
//
//   - send before cancel: r.ctx is live when the octets go out, so the hook
//     records a nil error. That is the pass.
//   - send after cancel: either the session's own cancel goroutine has already
//     closed the socket, the write fails and the hook never fires, or the
//     write wins and the hook records context.Canceled. Both are red.
//
// The far-end assertion cannot separate those, because bytes already on the
// pipe stay readable after the cancel. See the note on
// TestReactorStopSendsAdminShutdownBeforeCancel.
func TestReactorStopNotifiesWhileItsContextIsStillLive(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
	require.NoError(t, r.Start())

	r.mu.Lock()
	reactorCtx := r.ctx
	r.mu.Unlock()

	session, client := shutdownTestSession(t, fsm.StateEstablished)

	sent := make(chan error, 1)
	session.onNotifSent = func(code, subcode uint8) {
		if message.NotifyErrorCode(code) != message.NotifyCease ||
			subcode != message.NotifyCeaseAdminShutdown {
			return
		}
		select {
		case sent <- reactorCtx.Err():
		default:
		}
	}

	attachPeer(t, r, session)

	peerCtx, peerCancel := context.WithCancel(reactorCtx)
	t.Cleanup(peerCancel)

	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		_ = session.Run(peerCtx)
	}()

	// net.Pipe is unbuffered, so the send blocks until somebody reads it.
	got := readOne(client)

	r.Stop()

	// Stop waits on the per-peer sends before it cancels, so by the time it
	// returns the hook has either fired or lost its budget. Nothing to poll.
	select {
	case err := <-sent:
		require.NoError(t, err,
			"the Cease was written after Reactor.Stop had already canceled r.ctx: "+
				"the peer's socket is closing underneath the send")
	default:
		t.Fatal("Reactor.Stop returned with no Cease/Administrative Shutdown written")
	}

	<-got

	select {
	case <-runDone:
	case <-time.After(5 * time.Second):
		t.Fatal("session did not exit after the reactor stopped")
	}

	waitCtx2, waitCancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer waitCancel2()
	_ = r.Wait(waitCtx2)
}

// countingPipeDialer hands each dial one end of a net.Pipe and drains the other,
// so a session dialing through it gets its OPEN written and then sits in
// OpenSent with nothing to read. It counts the dials, which is the observable
// that separates a peer that stayed down from one that came back.
type countingPipeDialer struct {
	dials atomic.Int32
}

func (d *countingPipeDialer) DialContext(_ context.Context, _, _ string) (net.Conn, error) {
	d.dials.Add(1)
	local, remote := net.Pipe()
	go func() {
		defer func() { _ = remote.Close() }()
		buf := make([]byte, 4096)
		for {
			if _, err := remote.Read(buf); err != nil {
				return
			}
		}
	}()
	return local, nil
}

// TestReactorStopDoesNotRedialAPeerItHasAlreadyNotified is the two-peer guard on
// the ordering Stop depends on.
//
// Stop notifies before it cancels, because the cancel closes the sockets the
// NOTIFICATION needs (reactor.go). So for the whole shutdownNotifyBudget every
// peer context is still live -- and shutdownNotify tears its session down with
// ErrTeardown, which is the one error Peer.run answers by resetting the delay
// and continuing with NO wait (peer_run.go). A healthy peer therefore re-dialed
// a listener that is still open, could reach Established again, and was then
// killed by the cancel with nothing on the wire to say why: a shutdown flap, and
// a second RFC 4271 Section 8.2.2 ManualStop miss on the same stop.
//
// Two peers are what makes the window observable, which is why no existing test
// caught it. The slow peer reads nothing, so its NOTIFICATION write blocks on
// its own control-message deadline (10s at the shortest, session_write.go) and
// Stop spends the full budget waiting for it. The healthy peer flaps inside that
// second. With one peer the notify returns in microseconds and the cancel lands
// before the loop can get back round.
func TestReactorStopDoesNotRedialAPeerItHasAlreadyNotified(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
	require.NoError(t, r.Start())

	// The slow peer: attached with a live session whose far end never reads.
	slowSession, _ := shutdownTestSession(t, fsm.StateEstablished)
	attachPeer(t, r, slowSession)

	// The healthy peer, with a real run loop, so the reconnect decision is the
	// production one rather than a test's imitation of it.
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.2"),
		65001, 65002, 0x01020305,
	)
	settings.Port = 179
	settings.Connection = ConnectionActive

	dialer := &countingPipeDialer{}
	healthy := NewPeer(settings)
	healthy.SetDialer(dialer)
	healthy.setReconnectDelay(10*time.Millisecond, 50*time.Millisecond)

	r.mu.Lock()
	r.peers[peerKeyFromAddrPort(settings.Address, settings.Port)] = healthy
	reactorCtx := r.ctx
	r.mu.Unlock()

	healthy.StartWithContext(reactorCtx)
	t.Cleanup(healthy.Stop)

	require.Eventually(t, func() bool {
		healthy.mu.Lock()
		session := healthy.session
		healthy.mu.Unlock()
		return session != nil && session.Conn() != nil
	}, 5*time.Second, time.Millisecond, "the healthy peer never got a connection up")
	require.Equal(t, int32(1), dialer.dials.Load(),
		"the healthy peer must be one dial in before the stop")

	started := time.Now()
	r.Stop()
	elapsed := time.Since(started)

	// The fixture assertion: without a stop that actually spends its budget
	// there is no window for a flap to happen in, and a green below would mean
	// nothing. The slow peer's write cannot complete inside the budget, so
	// WaitGroupWait can only return when the budget expires.
	require.GreaterOrEqual(t, elapsed, shutdownNotifyBudget,
		"the slow peer did not hold Stop for its budget, so this test proves nothing")

	require.Equal(t, int32(1), dialer.dials.Load(),
		"a peer Stop had already told the engine was leaving dialed again while "+
			"the reactor context was still live: that session reaches Established "+
			"and dies on the cancel with no NOTIFICATION on it")

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer waitCancel()
	_ = r.Wait(waitCtx)

	peerWaitCtx, peerWaitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer peerWaitCancel()
	require.NoError(t, healthy.Wait(peerWaitCtx), "the healthy peer's run loop did not exit")
}

// TestReactorStopAcceptsNoInboundSessionWhileItNotifies is the INBOUND half of
// the same guard, and Peer.stopping does not cover it.
//
// Stop notifies before it cancels, so for the whole shutdownNotifyBudget the
// reactor context is live and the listeners are up. Listener.acceptLoop runs on
// its own goroutine and reaches acceptOrReject (reactor_connection.go) without
// reading p.stopping or p.ctx, and an address inside a dynamic group builds a
// brand new peer on the way (tryCreateDynamicPeer) -- one whose flag was never
// set, because Stop had taken its snapshot before that peer existed. That
// session reaches OpenSent and dies on the cancel with nothing on the wire to
// say why: the same RFC 4271 Section 8.2.2 ManualStop miss as the outbound flap,
// on the other rail.
//
// The slow peer is the same fixture as the test above: it reads nothing, so its
// NOTIFICATION write cannot finish and Stop spends the whole budget on it. The
// elapsed assertion runs FIRST, because a stop that returned in microseconds
// opens no window and would make everything below vacuous.
func TestReactorStopAcceptsNoInboundSessionWhileItNotifies(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
	require.NoError(t, r.Start())

	// A dynamic group over the loopback, so one TCP connection from this test
	// is the whole input a peer needs to be born and answered with an OPEN.
	dg := newTestDynamicGroup("shutdown-inbound", []string{"127.0.0.0/8"}, 10)
	dg.Settings.Connection = ConnectionPassive
	r.SetDynamicGroups([]*DynamicGroupConfig{dg})

	listenAddr := r.ListenAddr()
	require.NotNil(t, listenAddr, "the reactor is not listening, so there is no inbound rail to test")

	slowSession, _ := shutdownTestSession(t, fsm.StateEstablished)
	attachPeer(t, r, slowSession)

	started := time.Now()
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		r.Stop()
	}()

	// Stop seals the peers and the listeners in one hold of r.mu, so observing
	// the mark under that same lock means the seal is complete and the notify
	// budget has opened. Nothing here polls the socket.
	require.Eventually(t, func() bool {
		r.mu.RLock()
		defer r.mu.RUnlock()
		for _, peer := range r.peers {
			if peer.stopping.Load() {
				return true
			}
		}
		return false
	}, 5*time.Second, time.Millisecond, "Stop never marked its peers, so the window never opened")

	var answered int
	dialCtx, dialCancel := context.WithTimeout(context.Background(), time.Second)
	defer dialCancel()
	conn, dialErr := (&net.Dialer{}).DialContext(dialCtx, "tcp", listenAddr.String())
	if dialErr == nil {
		t.Cleanup(func() { _ = conn.Close() })
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(time.Second)))
		buf := make([]byte, 4096)
		answered, _ = conn.Read(buf)
	}

	<-stopped
	elapsed := time.Since(started)

	require.GreaterOrEqual(t, elapsed, shutdownNotifyBudget,
		"the slow peer did not hold Stop for its budget, so this test proves nothing")

	dynamic := 0
	r.mu.RLock()
	for _, peer := range r.peers {
		if peer.Settings().IsDynamic {
			dynamic++
		}
	}
	r.mu.RUnlock()
	require.Zero(t, dynamic,
		"a stop that had already notified its peers built a NEW one from an inbound "+
			"connection: nothing marked it stopping and nothing will notify it, so its "+
			"session dies on the cancel with no NOTIFICATION on it")

	require.Zero(t, answered,
		"ze answered an inbound connection with %d octets while it was shutting down: "+
			"that session is in OpenSent, which RFC 4271 Section 8.2.2 owes a Cease, and "+
			"the cancel is about to close it in silence", answered)

	require.Error(t, dialErr,
		"the listener was still accepting connections during the shutdown budget")

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer waitCancel()
	_ = r.Wait(waitCtx)
}

// TestTryCreateDynamicPeerRefusesAfterTheStopHasSealed covers the one inbound
// connection the listener close cannot reach: the one a Listener had ALREADY
// accepted when Reactor.stop took r.mu. Its handler goroutine still holds it,
// and it arrives at tryCreateDynamicPeer after the seal, which is what this
// test calls directly -- the seam is the lock, so the handler's own ordering is
// what the direct call reproduces.
//
// Building a peer there is the miss: Stop's snapshot was taken before that peer
// existed, so no shutdownNotify covers it, and the session it would reach
// OpenSent on dies on the cancel with nothing on the wire (RFC 4271 Section
// 8.2.2, ManualStop). Returning nil makes the caller close the connection
// (reactor_connection.go), which owes nothing.
func TestTryCreateDynamicPeerRefusesAfterTheStopHasSealed(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
	require.NoError(t, r.Start())

	dg := newTestDynamicGroup("shutdown-inbound", []string{"185.1.69.0/24"}, 10)
	dg.Settings.Connection = ConnectionPassive
	r.SetDynamicGroups([]*DynamicGroupConfig{dg})

	addr := netip.MustParseAddr("185.1.69.42")
	require.NotNil(t, r.tryCreateDynamicPeer(addr),
		"the group does not admit this address, so the assertion below would be vacuous")

	r.mu.Lock()
	delete(r.peers, peerKeyFromAddrPort(addr, DefaultBGPPort))
	r.mu.Unlock()

	r.Stop()

	// A non-nil Peer here is a RUNNING one, so this asserts a boolean: a
	// value-printing matcher would reflect-walk a struct the peer's own run
	// loop is still writing to, and report a race on top of the failure.
	created := r.tryCreateDynamicPeer(addr)
	require.True(t, created == nil,
		"a stop that has already sealed built a new peer: nothing marked it stopping, "+
			"Stop's snapshot predates it, and the cancel closes its session in silence")

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer waitCancel()
	_ = r.Wait(waitCtx)
}

// TestReactorStopForRestartSendsNoNotification is the restart half of the same
// decision, and the reason the two stops are separate methods.
//
// RFC 4724 Section 5, "Changes to BGP Finite State Machine", replaces RFC 4271
// Section 8.2.2's text: a peer that receives a NOTIFICATION (Event 24 or Event
// 25) "deletes all routes associated with this connection", unconditionally
// (rfc/full/rfc4724.txt, lines 569-585). Retention is the conditional branch --
// a TcpConnectionFails (Event 18) AND the Graceful Restart Capability received
// "with one or more AFIs/SAFIs" (lines 587-604) -- and Ze advertises an empty
// tuple list, so no peer takes it for a Ze session today. That is why the
// assertion below is about the NOTIFICATION and not about the peer's routes:
// silence is the only end a restart can ever be retained across, and Ze does
// not implement RFC 8538 (docs/features/rfc-status.md), so there is no third
// option. `daemon restart` and `daemon reboot` write the restarting-speaker
// marker and then take this path (cmd/ze/hub/infra_setup.go); a Cease here
// would foreclose, for every peer, the retention that marker asks for.
func TestReactorStopForRestartSendsNoNotification(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stop   func(*Reactor)
		notify bool
	}{
		{"shutdown", (*Reactor).Stop, true},
		{"restart", (*Reactor).StopForRestart, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
			require.NoError(t, r.Start())

			session, client := shutdownTestSession(t, fsm.StateEstablished)
			attachPeer(t, r, session)

			r.mu.Lock()
			peerCtx, peerCancel := context.WithCancel(r.ctx)
			r.mu.Unlock()
			t.Cleanup(peerCancel)

			runDone := make(chan struct{})
			go func() {
				defer close(runDone)
				_ = session.Run(peerCtx)
			}()

			got := readOne(client)
			tc.stop(r)

			select {
			case msg, ok := <-got:
				switch {
				case tc.notify:
					require.True(t, ok, "socket closed with no NOTIFICATION on it")
					require.Equal(t, adminShutdownNotificationWire(), msg)
				default:
					require.False(t, ok,
						"a restarting speaker wrote %d octets to its peer; RFC 4724 "+
							"Section 5 makes any NOTIFICATION delete the routes the "+
							"graceful-restart marker asks the peer to keep", len(msg))
				}
			case <-time.After(5 * time.Second):
				t.Fatal("the session's socket neither carried a message nor closed")
			}

			select {
			case <-runDone:
			case <-time.After(5 * time.Second):
				t.Fatal("session did not exit after the reactor stopped")
			}

			waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer waitCancel()
			_ = r.Wait(waitCtx)
		})
	}
}

// TestReactorStopStartsNoListenerForAnAddressAddedWhileItNotifies is the third
// rail into a session, and the two rounds before this one each left it open.
//
// Reactor.stop shuts the listeners under r.mu, but shutting them is not the same
// as keeping them shut, and r.ctx stays live for the whole notify budget.
// A netlink address-added event inside that window reaches
// handleAddrAddedPayload (reactor_iface.go), which takes r.mu and starts a fresh
// Listener on that still-live context. Nothing stops the event arriving: the
// EventBus subscriptions are released by Reactor.cleanup, and cleanup is reached
// from monitor(), which blocks on <-r.ctx.Done(). So the release happens AFTER
// the cancel, one whole budget too late. The daemon accepts again, answers with
// an OPEN, and the cancel closes that socket with nothing on the wire to say why
// (RFC 4271 Section 8.2.2, ManualStop).
//
// The seal is read inside startListenerForAddressPort, which all three callers
// reach with r.mu held, rather than at the callers. Gating the callers is what
// had already failed twice.
//
// The slow peer is the fixture from the tests above: it reads nothing, so its
// NOTIFICATION write cannot finish and Stop spends the whole budget on it. The
// elapsed assertion runs FIRST, because a stop that returned in microseconds
// opens no window and would make everything below vacuous.
func TestReactorStopStartsNoListenerForAnAddressAddedWhileItNotifies(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
	require.NoError(t, r.Start())

	local := netip.MustParseAddr("127.0.0.1")
	lkey := net.JoinHostPort(local.String(), "0")

	// A peer bound to that local address, because handleAddrAddedPayload starts
	// nothing for an address no peer asked for.
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.3"), 65001, 65002, 0x01020306)
	settings.LocalAddress = local
	settings.Connection = ConnectionPassive
	r.mu.Lock()
	r.peers[peerKeyFromAddrPort(settings.Address, settings.Port)] = NewPeer(settings)
	r.mu.Unlock()

	added := interfaceAddrPayload{Name: "lo", Unit: 0, Address: local.String(), PrefixLength: 8}

	// Non-vacuity, measured rather than assumed: the same event DOES open a
	// listener on this reactor before the stop. Take it back down afterwards,
	// because startListenerForAddressPort returns early for a key it already
	// holds and the assertion below would then pass for the wrong reason.
	r.handleAddrAddedPayload(added)
	r.mu.Lock()
	preStop, hadListener := r.listeners[lkey]
	if hadListener {
		preStop.Stop()
		delete(r.listeners, lkey)
	}
	r.mu.Unlock()
	require.True(t, hadListener,
		"this event opens no listener even before the stop, so the assertion below proves nothing")

	slowSession, _ := shutdownTestSession(t, fsm.StateEstablished)
	attachPeer(t, r, slowSession)

	started := time.Now()
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		r.Stop()
	}()

	// Stop seals under r.mu in one hold, so observing the mark under that same
	// lock means the seal is complete and the notify budget has opened.
	require.Eventually(t, func() bool {
		r.mu.RLock()
		defer r.mu.RUnlock()
		return r.stopping
	}, 5*time.Second, time.Millisecond, "Stop never sealed, so the window never opened")

	r.handleAddrAddedPayload(added)

	r.mu.RLock()
	_, reopened := r.listeners[lkey]
	r.mu.RUnlock()

	<-stopped
	elapsed := time.Since(started)

	require.GreaterOrEqual(t, elapsed, shutdownNotifyBudget,
		"the slow peer did not hold Stop for its budget, so this test proves nothing")

	require.False(t, reopened,
		"an address-added event re-opened a listener on a reactor that had already "+
			"sealed: the reactor context is live for the whole notify budget, so that "+
			"listener accepts, answers with an OPEN, and the cancel closes it in silence")

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer waitCancel()
	_ = r.Wait(waitCtx)
}

// TestPeerStartWithContextDoesNotLiftTheStopsSeal covers the companion hole on
// the same rail: a peer STARTED inside the notify budget.
//
// Peer.StartWithContext used to clear Peer.stopping, which is the mark
// Reactor.stop had just set on every peer, and it holds no lock that could order
// that clear against the stop. Reactor.StartPeers reaches it from
// coord.OnPostStartup (internal/component/bgp/plugin/register.go) with r.mu
// released, so a SIGTERM landing during plugin startup re-opened the outbound
// rail on a peer the stop had already marked and notified: run()'s loop top then
// read false, dialed, reached OpenSent, and died on the cancel in silence
// (RFC 4271 Section 8.2.2, ManualStop).
//
// Lifting the seal is now one act, in Reactor.StartWithContext, under the same
// r.mu that sets it. The observable is the dial count: a peer that never dials
// never has a session to owe a NOTIFICATION on.
func TestPeerStartWithContextDoesNotLiftTheStopsSeal(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
	require.NoError(t, r.Start())

	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.4"), 65001, 65002, 0x01020307)
	settings.Port = 179
	settings.Connection = ConnectionActive

	dialer := &countingPipeDialer{}
	late := NewPeer(settings)
	late.SetDialer(dialer)
	late.setReconnectDelay(10*time.Millisecond, 50*time.Millisecond)

	// In r.peers before the stop, so the seal marks it. A peer added after the
	// seal is refused by AddPeer and tryCreateDynamicPeer instead.
	r.mu.Lock()
	r.peers[peerKeyFromAddrPort(settings.Address, settings.Port)] = late
	r.mu.Unlock()

	slowSession, _ := shutdownTestSession(t, fsm.StateEstablished)
	attachPeer(t, r, slowSession)

	started := time.Now()
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		r.Stop()
	}()

	require.Eventually(t, func() bool {
		return late.stopping.Load()
	}, 5*time.Second, time.Millisecond, "Stop never marked the peer, so the window never opened")

	r.mu.RLock()
	reactorCtx := r.ctx
	r.mu.RUnlock()
	late.StartWithContext(reactorCtx)
	t.Cleanup(late.Stop)

	// Give the run loop room to dial if the seal was lifted. The reconnect delay
	// above is 10ms, so a peer that is going to dial has dialed by now.
	require.Never(t, func() bool {
		return dialer.dials.Load() != 0
	}, 200*time.Millisecond, 5*time.Millisecond,
		"a peer started inside the notify budget dialed: Stop had already marked and "+
			"notified it, so that session reaches OpenSent and the cancel closes it "+
			"with no NOTIFICATION on it")

	<-stopped
	elapsed := time.Since(started)

	require.GreaterOrEqual(t, elapsed, shutdownNotifyBudget,
		"the slow peer did not hold Stop for its budget, so this test proves nothing")

	require.Zero(t, dialer.dials.Load(),
		"a peer started inside the notify budget dialed")

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer waitCancel()
	_ = r.Wait(waitCtx)
}

// TestRunOncePublishesNoSessionAfterTheStopHasMarkedThePeer is the fourth rail:
// a connection cycle already PAST run()'s loop-top check when the stop landed.
//
// Round 3 read Peer.stopping at the top of run() and nowhere else, so a cycle
// that had already entered runOnce carried on: it published p.session and dialed,
// and Reactor.stop's shutdownNotify -- whose read of p.session may have got in
// first and seen nil -- never covered it. The session then reached OpenSent and
// the cancel closed it with nothing on the wire (RFC 4271 Section 8.2.2,
// ManualStop).
//
// The seam is the p.mu hold that publishes p.session, which is the same lock
// shutdownNotify reads that field under, so this test drives runOnce directly:
// the direct call reproduces exactly the ordering the run loop would have, the
// way TestTryCreateDynamicPeerRefusesAfterTheStopHasSealed drives
// tryCreateDynamicPeer for the r.mu seam.
func TestRunOncePublishesNoSessionAfterTheStopHasMarkedThePeer(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.5"), 65001, 65002, 0x01020308)
	settings.Port = 179
	settings.Connection = ConnectionActive

	dialer := &countingPipeDialer{}
	peer := NewPeer(settings)
	peer.SetDialer(dialer)

	// A canceled context, so the cycle unwinds as soon as it has dialed. The
	// dialer ignores it, which is what makes the dial itself observable.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	peer.mu.Lock()
	peer.ctx, peer.cancel = ctx, cancel
	peer.mu.Unlock()

	// Non-vacuity first: an unmarked peer DOES dial through this fixture.
	_ = peer.runOnce()
	require.Equal(t, int32(1), dialer.dials.Load(),
		"the fixture never dials even unmarked, so the assertion below proves nothing")

	peer.stopping.Store(true)
	refused := peer.runOnce()

	// The observables come first and the return value last, because a refused
	// cycle and a cycle that ran to a clean end both return nil: what separates
	// them is the dial that did not happen and the session never published.
	require.Equal(t, int32(1), dialer.dials.Load(),
		"a connection cycle that was already past run()'s loop top dialed after the "+
			"stop had marked the peer: Stop's shutdownNotify may have read p.session "+
			"before it was published, so nothing notifies that session and the cancel "+
			"closes it in silence")

	peer.mu.RLock()
	published := peer.session
	peer.mu.RUnlock()
	require.Nil(t, published,
		"a session was published after the seal, so shutdownNotify's snapshot can miss it")

	require.NoError(t, refused,
		"a refused cycle is not a failed one: run() ends the loop at its own top, and "+
			"an error here would send it into the backoff select instead")
}

// TestSessionConnectSendsNoOpenOnceTeardownHasRun is the fifth rail, and the one
// no flag on the peer can close: a dial already IN FLIGHT when the stop landed.
//
// Peer.shutdownNotify tears the session down (peer.go), and Session.teardown
// writes the Cease only when s.conn is already set (session_connection.go). For
// a peer in Connect or Active the conn is still nil, so teardown sets
// s.tearingDown and sends nothing -- and then the dial completes,
// connectionEstablished sends the OPEN, and the cancel closes that socket with
// nothing on the wire to say why (RFC 4271 Section 8.2.2, ManualStop).
// Session.Accept has refused a torn-down session since before this spec;
// Session.Connect did not, and it reaches the wire through the same
// connectionEstablished, which is where the check now sits -- inside the s.mu
// hold that publishes s.conn, so teardown either sees the conn and sends the
// Cease on it, or is seen by this check.
//
// The far end is a real TCP listener rather than a net.Pipe: the kernel buffers,
// so "zero octets arrived" is an assertion about what was sent and not about who
// blocked first.
func TestSessionConnectSendsNoOpenOnceTeardownHasRun(t *testing.T) {
	for _, tc := range []struct {
		name     string
		teardown bool
	}{
		{"live session dials and sends its OPEN", false},
		{"torn-down session dials nothing", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
			require.NoError(t, err)
			t.Cleanup(func() { _ = ln.Close() })

			octets := make(chan int, 1)
			go func() {
				conn, acceptErr := ln.Accept()
				if acceptErr != nil {
					octets <- -1
					return
				}
				defer func() { _ = conn.Close() }()
				_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
				buf := make([]byte, 4096)
				n, _ := conn.Read(buf)
				octets <- n
			}()

			addrPort, ok := ln.Addr().(*net.TCPAddr)
			require.True(t, ok, "the listener is not TCP, so there is no port to dial")
			settings := NewPeerSettings(
				netip.MustParseAddr("127.0.0.1"), 65001, 65002, 0x01020309,
			)
			settings.Port = uint16(addrPort.Port)
			settings.Connection = ConnectionActive

			session := NewSession(settings)
			require.NoError(t, session.Start())

			if tc.teardown {
				// Exactly what shutdownNotify does to a peer whose session has no
				// conn yet: tearingDown is set, and nothing goes on the wire.
				require.NoError(t, session.Teardown(message.NotifyCeaseAdminShutdown, ""))
			}

			connectErr := session.Connect(context.Background())
			t.Cleanup(session.closeConn)

			select {
			case n := <-octets:
				if tc.teardown {
					// The wire comes first: what the peer sees is the obligation,
					// and the error is how Connect reports having honored it.
					require.LessOrEqual(t, n, 0,
						"a session torn down by the shutdown notify still dialed and put "+
							"%d octets on the wire; that connection reaches OpenSent and "+
							"the cancel closes it with no NOTIFICATION on it", n)
					require.ErrorIs(t, connectErr, ErrSessionTearingDown)
					return
				}
				require.NoError(t, connectErr)
				require.Positive(t, n,
					"the fixture sends nothing even on a live session, so the case above proves nothing")
			case <-time.After(5 * time.Second):
				t.Fatal("the listener neither accepted nor timed out")
			}
		})
	}
}

// TestSealedStopAcceptsNoInboundSessionOnAConfiguredPeer drives the accept rail
// against an EXISTING CONFIGURED peer, which is the population every earlier
// inbound guard here misses.
//
// TestReactorStopAcceptsNoInboundSessionWhileItNotifies dials the reactor's own
// listener, and TestTryCreateDynamicPeerRefusesAfterTheStopHasSealed drives the
// dynamic-peer seam. Both end at tryCreateDynamicPeer, and neither runs when
// findPeerByAddr succeeds: acceptOrReject then goes straight to
// Peer.AcceptConnection (reactor_connection.go) and reads no seal of any kind.
// The connection this models is the one a Listener had ALREADY accepted when
// Reactor.stop took r.mu, so closing the listeners does not reach it either.
//
// What must hold is not "no new socket opens" -- that connection already exists
// -- but "no conn becomes a session after the seal". The session it would land
// on is the one already published on p.session, and the gate is the s.tearingDown
// read inside the s.mu hold that publishes s.conn (session_connection.go). Both
// stops are driven, because both take the seal and only one of them notifies:
// on the restart path nothing tears a session down, so before Reactor.stop
// sealed the sessions itself nothing set that flag at all.
//
// Without the gate the peer gets an OPEN, reaches OpenSent, and dies on the
// cancel with nothing on the wire to say why -- the RFC 4271 Section 8.2.2
// ManualStop miss, on the rail that keeps being the last one open.
func TestSealedStopAcceptsNoInboundSessionOnAConfiguredPeer(t *testing.T) {
	for _, tc := range []struct {
		name string
		stop func(*Reactor)
	}{
		{"shutdown", (*Reactor).Stop},
		{"restart", (*Reactor).StopForRestart},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
			require.NoError(t, r.Start())

			// A configured peer at the address this test dials from, with a
			// session published the way a runOnce mid-cycle leaves it: started,
			// no conn yet.
			addr := netip.MustParseAddr("127.0.0.1")
			settings := NewPeerSettings(addr, 65001, 65002, 0x01020304)
			settings.Connection = ConnectionPassive

			session := NewSession(settings)
			require.NoError(t, session.Start())

			peer := NewPeer(settings)
			peer.mu.Lock()
			peer.session = session
			peer.mu.Unlock()

			r.mu.Lock()
			r.peers[peerKeyFromAddrPort(settings.Address, settings.Port)] = peer
			r.mu.Unlock()

			// Non-vacuity first: everything below is worthless if the rail does
			// not actually reach this peer's published session.
			r.mu.RLock()
			found, exists := r.findPeerByAddr(addr)
			callback := r.connCallback
			r.mu.RUnlock()
			require.True(t, exists,
				"findPeerByAddr does not find this peer, so acceptOrReject would take the "+
					"dynamic path this test is not about")
			require.Same(t, peer, found, "the accept rail resolves a different peer")
			require.False(t, found.Settings().IsDynamic,
				"this peer is dynamic, so round 4's tryCreateDynamicPeer seal covers it already")
			require.Nil(t, callback,
				"a connection callback is installed, so acceptOrReject returns before it "+
					"ever reaches the peer")
			require.NotEqual(t, PeerStateEstablished, peer.State(),
				"an Established peer is answered with a collision NOTIFICATION instead")
			require.NotEqual(t, fsm.StateOpenConfirm, peer.SessionState(),
				"an OpenConfirm session takes the pending-collision path instead")
			require.NotNil(t, peer.currentSession(),
				"no session is published, so AcceptConnection refuses on ErrNotConnected "+
					"and the publish site is never reached")
			require.Nil(t, session.Conn(),
				"the session already holds a conn, so Accept refuses on ErrAlreadyConnected "+
					"and the publish site is never reached")

			tc.stop(r)

			r.mu.RLock()
			sealed := r.stopping
			r.mu.RUnlock()
			require.True(t, sealed, "the stop did not seal, so there is no seal to test")
			require.True(t, peer.stopping.Load(), "the stop did not mark this peer")

			// A real TCP connection, handed on the way a Listener's handler
			// goroutine hands one: accepted before the seal, delivered after it.
			ln, lnErr := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
			require.NoError(t, lnErr)
			t.Cleanup(func() { _ = ln.Close() })

			dialed := make(chan net.Conn, 1)
			go func() {
				dialCtx, dialCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer dialCancel()
				far, dialErr := (&net.Dialer{}).DialContext(dialCtx, "tcp", ln.Addr().String())
				if dialErr != nil {
					close(dialed)
					return
				}
				dialed <- far
			}()

			accepted, acceptErr := ln.Accept()
			require.NoError(t, acceptErr, "no connection arrived, so nothing reaches the accept rail")
			far, dialOK := <-dialed
			require.True(t, dialOK, "the dial failed, so nothing reaches the accept rail")
			t.Cleanup(func() { _ = far.Close() })

			r.handleConnection(accepted)

			require.Nil(t, session.Conn(),
				"a stop that had already sealed published a connection on a live session: "+
					"that session sends its OPEN and the cancel then closes it in silence, "+
					"which RFC 4271 Section 8.2.2 owes a Cease")

			require.NoError(t, far.SetReadDeadline(time.Now().Add(250*time.Millisecond)))
			buf := make([]byte, 4096)
			n, _ := far.Read(buf)
			require.Zero(t, n,
				"ze answered an inbound connection with %d octets after it had sealed", n)

			require.NotEqual(t, fsm.StateOpenSent, session.State(),
				"the session reached OpenSent after the seal")

			waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer waitCancel()
			_ = r.Wait(waitCtx)
		})
	}
}

// sealedSessionOnPort returns a sealed session for a peer at 127.0.0.1:port,
// started and holding no conn. Sealed is the state Reactor.stop leaves every
// live session in, on both stops (reactor.go, Peer.sealSession).
func sealedSessionOnPort(t *testing.T, port int) *Session {
	t.Helper()

	settings := NewPeerSettings(
		netip.MustParseAddr("127.0.0.1"), 65001, 65002, 0x01020304,
	)
	settings.Port = uint16(port)

	session := NewSession(settings)
	require.NoError(t, session.Start())
	t.Cleanup(session.closeConn)

	session.seal()
	require.Nil(t, session.Conn(),
		"the session already holds a conn, so every refusal below returns "+
			"ErrAlreadyConnected and the branch under test is never reached")

	return session
}

// TestSealedSessionRefusesAnAcceptWithoutClosingTheCallersConn pins WHO owns a
// connection that connectionEstablished refuses.
//
// The refusal is the s.tearingDown read inside the s.mu hold that publishes
// s.conn (session_connection.go). It used to close the connection on its way
// out, which is right for Connect -- runOnce never sees that socket and cannot
// close it (peer_run.go) -- and wrong for both accept rails, where the CALLER
// keeps it. acceptOrReject buffers it on ErrSessionTearingDown for a passive
// peer and offers it to the next runOnce cycle (reactor_connection.go); with the
// close in place that cycle accepted a dead socket and paid a backoff.
// acceptPendingConnection closed it a second time.
//
// AcceptWithOpen is the row that drives the changed branch, and it does so
// deterministically: it has no entry check of its own, so a sealed session
// reaches connectionEstablished on every call and ErrSessionTearingDown can come
// from nowhere else. Accept refuses one step earlier, at its own check, which
// never closed the connection; that row is here because acceptOrReject takes
// that rail in production, not because it discriminates.
func TestSealedSessionRefusesAnAcceptWithoutClosingTheCallersConn(t *testing.T) {
	for _, tc := range []struct {
		name   string
		accept func(*Session, net.Conn) error
	}{
		{"accept", func(s *Session, conn net.Conn) error {
			return s.Accept(conn)
		}},
		{"accept with open", func(s *Session, conn net.Conn) error {
			return s.AcceptWithOpen(conn, &message.Open{})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ln, lnErr := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
			require.NoError(t, lnErr)
			t.Cleanup(func() { _ = ln.Close() })

			dialed := make(chan net.Conn, 1)
			go func() {
				far, dialErr := (&net.Dialer{}).Dial("tcp", ln.Addr().String())
				if dialErr != nil {
					close(dialed)
					return
				}
				dialed <- far
			}()

			// near is the connection the reactor's accept rail hands on: one a
			// Listener had already accepted when Reactor.stop took r.mu.
			near, acceptErr := ln.Accept()
			require.NoError(t, acceptErr, "no connection arrived, so there is nothing to refuse")
			t.Cleanup(func() { _ = near.Close() })
			far, dialOK := <-dialed
			require.True(t, dialOK, "the dial failed, so there is nothing to refuse")
			t.Cleanup(func() { _ = far.Close() })

			session := sealedSessionOnPort(t, 179)

			require.ErrorIs(t, tc.accept(session, near), ErrSessionTearingDown)
			require.Nil(t, session.Conn(),
				"a sealed session published a conn, so the seal is not a gate at all")

			// The caller's connection is still usable. This is the socket
			// acceptOrReject buffers for the next cycle; a closed one fails that
			// cycle's Accept and costs the peer a backoff.
			probe := []byte("still open")
			written, writeErr := near.Write(probe)
			require.NoError(t, writeErr,
				"the refusal closed a connection it does not own: acceptOrReject then "+
					"buffers a dead socket and the next runOnce cycle pays a backoff on it")
			require.Equal(t, len(probe), written)

			require.NoError(t, far.SetReadDeadline(time.Now().Add(2*time.Second)))
			buf := make([]byte, len(probe))
			for got := 0; got < len(buf); {
				n, readErr := far.Read(buf[got:])
				require.NoError(t, readErr, "the far end lost the connection the refusal handed back")
				got += n
			}
			require.Equal(t, probe, buf)
		})
	}
}

// TestSealedSessionConnectClosesTheConnItDialed is the other half of that
// ownership rule, and it guards the rail the fix above must not regress.
//
// Connect owns what it dialed: runOnce holds no reference to that socket
// (peer_run.go, the session.Connect call site), so nothing else can close it.
// Moving the close out of connectionEstablished therefore has to put it back
// here, or every dial refused by the seal leaks a socket for the rest of the
// process's life -- bounded by the shutdown budget, but held for the whole of it.
//
// The far end reads io.EOF because Connect closed; a leak leaves it blocked to
// the deadline instead. TestSessionConnectSendsNoOpenOnceTeardownHasRun cannot
// tell those apart -- it asserts zero octets, and both produce zero.
//
// The sentinel is matched by its message rather than with errors.Is: importing
// io here would move every line below this file's import block, and two of them
// are cited by line in rfc/requirements/rfc4271.md for the RFC4271-8.2.2-18
// tags. io.EOF's text is fixed, and the failure it has to be told from reads
// "i/o timeout".
func TestSealedSessionConnectClosesTheConnItDialed(t *testing.T) {
	ln, lnErr := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, lnErr)
	t.Cleanup(func() { _ = ln.Close() })

	addrPort, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok, "the listener is not TCP, so there is no port to dial")

	session := sealedSessionOnPort(t, addrPort.Port)

	require.ErrorIs(t, session.Connect(context.Background()), ErrSessionTearingDown)
	require.Nil(t, session.Conn(), "a sealed session published the conn it dialed")

	far, acceptErr := ln.Accept()
	require.NoError(t, acceptErr, "the dial never reached the listener, so nothing was closed or leaked")
	t.Cleanup(func() { _ = far.Close() })

	require.NoError(t, far.SetReadDeadline(time.Now().Add(2*time.Second)))
	buf := make([]byte, 4096)
	read, readErr := far.Read(buf)
	require.Zero(t, read, "a session refused by its own seal put %d octets on the wire", read)
	require.EqualError(t, readErr, "EOF",
		"the far end did not see a FIN, so Connect left the socket it dialed open")
}
