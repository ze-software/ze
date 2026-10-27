// Design: docs/architecture/behavior/peer-lifecycle.md — DelayOpen through real Peer cycles.
package reactor

import (
	"errors"
	"io"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/test/sim"
)

func delayOpenAwait(t *testing.T, predicate func() bool) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for !predicate() {
		select {
		case <-deadline.C:
			t.Fatal("DelayOpen state did not arrive")
		case <-tick.C:
		}
	}
}

// startDelayOpenPeer uses the actual run loop, outbound dial or inbound accept,
// and an injected timer clock. No synthetic FSM state is installed.
func startDelayOpenPeer(t *testing.T, passive bool, delay time.Duration, configure ...func(*Peer)) (*Peer, *Session, net.Conn, *sim.FakeClock) {
	t.Helper()
	settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101)
	settings.DelayOpen = delay
	settings.Connection = ConnectionBoth
	var far net.Conn
	var listener net.Listener
	if passive {
		settings.Connection = ConnectionPassive
	} else {
		var lc net.ListenConfig
		var err error
		listener, err = lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { closeConnQuietlyListener(listener) })
		settings.Port = uint16(listener.Addr().(*net.TCPAddr).Port)
		if err := listener.(*net.TCPListener).SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	peer := NewPeer(settings)
	fc := sim.NewFakeClock(time.Now())
	dialer := newHeldDialer()
	if !passive {
		peer.SetDialer(dialer)
	}
	for _, apply := range configure {
		apply(peer)
	}
	peer.setReconnectDelay(time.Hour, time.Hour)
	t.Cleanup(startAndStop(t, peer))
	var session *Session
	delayOpenAwait(t, func() bool {
		session = peer.currentSession()
		return session != nil && session.State() != fsm.StateIdle
	})
	// Keep the outer Peer/read-loop clock real; only the protocol timers
	// advance virtually. The dial is held until that timer owner is installed.
	session.timers.SetClock(fc)
	if !passive {
		close(dialer.release)
	}
	if passive {
		local, remote := net.Pipe()
		far = remote
		accepted := make(chan error, 1)
		go func() { accepted <- peer.acceptConnection(local) }()
		if delay == 0 {
			delayOpenReadType(t, far, 1)
		}
		select {
		case err := <-accepted:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("accept blocked")
		}
	} else {
		var err error
		far, err = listener.Accept()
		if err != nil {
			t.Fatal(err)
		}
		if delay == 0 {
			delayOpenReadType(t, far, 1)
		}
	}
	t.Cleanup(func() { closeConnQuietly(far) })
	delayOpenAwait(t, func() bool {
		if delay == 0 {
			return session.State() == fsm.StateOpenSent
		}
		return session.timers.IsDelayOpenTimerRunning()
	})
	return peer, session, far, fc
}

func closeConnQuietlyListener(listener net.Listener) {
	_ = listener.Close() // Test cleanup; no live owner remains.
}

func delayOpenReadType(t *testing.T, conn net.Conn, kind byte) []byte {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	packet, err := core4271ReadMessage(conn)
	if err != nil {
		t.Fatal(err)
	}
	if packet[18] != kind {
		t.Fatalf("wire type %d, want %d: %x", packet[18], kind, packet)
	}
	return packet
}

func delayOpenNoBytes(t *testing.T, conn net.Conn) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(20 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	var octet [1]byte
	n, err := conn.Read(octet[:])
	var timeout net.Error
	if n != 0 || !errors.As(err, &timeout) || !timeout.Timeout() {
		t.Fatalf("held connection produced %d bytes, error %v", n, err)
	}
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
}

func delayOpenWrite(t *testing.T, conn net.Conn, msg message.Message) {
	t.Helper()
	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write(message.PackTo(msg, nil)); err != nil {
		t.Fatal(err)
	}
}

func delayOpenRemote(id uint32) *message.Open {
	return &message.Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: id}
}

// TestRFC4271DelayOpenHeldConnectionWritesNoOpen proves Event 17 holds in both
// states, then drives Event 20 over the actual read loop, with no OpenSent stop.
func TestRFC4271DelayOpenHeldConnectionWritesNoOpen(t *testing.T) {
	for _, passive := range []bool{false, true} {
		t.Run(map[bool]string{false: "Connect", true: "Active"}[passive], func(t *testing.T) {
			peer, s, far, _ := startDelayOpenPeer(t, passive, 240*time.Second)
			delayOpenNoBytes(t, far)
			if s.timers.IsConnectRetryTimerRunning() {
				t.Fatal("retry timer still running")
			}
			delayOpenWrite(t, far, delayOpenRemote(0x02020202))
			delayOpenReadType(t, far, 1)
			delayOpenReadType(t, far, 4)
			delayOpenAwait(t, func() bool { return s.State() == fsm.StateOpenConfirm })
			from := fsm.StateConnect
			if passive {
				from = fsm.StateActive
			}
			delayOpenDirectConfirm(t, peer, from)
			if s.timers.IsDelayOpenTimerRunning() || !s.timers.IsHoldTimerRunning() || !s.timers.IsKeepaliveTimerRunning() {
				t.Fatal("Event 20 did not replace DelayOpen with negotiated timers")
			}
		})
	}
}

// TestRFC4271DelayOpenTimerExpiry drives the installed callback, not Event 12
// directly. Both peers stay silent until the exact configured deadline.
func TestRFC4271DelayOpenTimerExpiry(t *testing.T) {
	for _, passive := range []bool{false, true} {
		t.Run(map[bool]string{false: "Connect", true: "Active"}[passive], func(t *testing.T) {
			_, s, far, fc := startDelayOpenPeer(t, passive, 240*time.Second)
			fc.Add(240*time.Second - time.Nanosecond)
			delayOpenNoBytes(t, far)
			advanced := make(chan struct{})
			go func() { fc.Add(time.Nanosecond); close(advanced) }()
			delayOpenReadType(t, far, 1)
			<-advanced
			if s.State() != fsm.StateOpenSent || s.timers.IsDelayOpenTimerRunning() || !s.timers.IsHoldTimerRunning() {
				t.Fatal("Event 12 did not enter bounded OpenSent")
			}
		})
	}
}

// TestRFC4271ActiveEvent18WithDelayOpen reaches Active's Event 18 through a
// running passive Peer. The disabled case goes to OpenSent before any failure.
func TestRFC4271ActiveEvent18WithDelayOpen(t *testing.T) {
	for _, delay := range []time.Duration{0, 240 * time.Second} {
		t.Run(delay.String(), func(t *testing.T) {
			peer, s, far, _ := startDelayOpenPeer(t, true, delay)
			peer.connectRetryCounter.Increment()
			peer.connectRetryCounter.Increment()
			closeConnQuietly(far)
			delayOpenAwait(t, func() bool { return peer.currentSession() != s })
			want, counter := fsm.StateIdle, uint32(3)
			if delay == 0 {
				want, counter = fsm.StateActive, 2
			}
			if s.State() != want || peer.ConnectRetryCounter() != counter || s.Conn() != nil || s.timers.IsDelayOpenTimerRunning() {
				t.Fatalf("delay %s: state=%s counter=%d conn=%v", delay, s.State(), peer.ConnectRetryCounter(), s.Conn())
			}
		})
	}
}

// TestRFC4271DelayOpenVersionError checks the running-timer Event 24 branch
// before cleanup can erase the fact that selects its no-increment action.
func TestRFC4271DelayOpenVersionError(t *testing.T) {
	for _, passive := range []bool{false, true} {
		peer, s, far, _ := startDelayOpenPeer(t, passive, 240*time.Second)
		peer.connectRetryCounter.Increment()
		delayOpenWrite(t, far, &message.Notification{ErrorCode: message.NotifyOpenMessage, ErrorSubcode: message.NotifyOpenUnsupportedVersion, Data: []byte{0, 4}})
		delayOpenAwait(t, func() bool { return peer.currentSession() != s })
		if peer.ConnectRetryCounter() != 1 || s.State() != fsm.StateIdle || s.timers.IsDelayOpenTimerRunning() {
			t.Fatal("Event 24 incremented retry history or retained DelayOpen")
		}
	}
}

// TestRFC4271DelayOpenBothSidesHold sends a second connection through the real
// reactor gate. Its first OPEN must retire the OPEN-less socket without Cease,
// and the next Peer cycle accepts the buffered OPEN with the counter preserved.
func TestRFC4271DelayOpenBothSidesHold(t *testing.T) {
	for _, passive := range []bool{false, true} {
		for _, id := range []uint32{0x01010100, 0x02020202} {
			peer, s, far, _ := startDelayOpenPeer(t, passive, 240*time.Second)
			peer.connectRetryCounter.Increment()
			local, pending := net.Pipe()
			t.Cleanup(func() { closeConnQuietly(pending) })
			r := &Reactor{clock: clock.RealClock{}}
			r.acceptOrReject(local, peer, nil)
			if !peer.hasPendingConnection() {
				t.Fatal("second OPEN-less socket bypassed pending slot")
			}
			delayOpenNoBytes(t, far)
			thirdLocal, thirdRemote := net.Pipe()
			t.Cleanup(func() { closeConnQuietly(thirdRemote) })
			rejected := make(chan struct{})
			go func() { r.acceptOrReject(thirdLocal, peer, nil); close(rejected) }()
			delayOpenReadCollisionClose(t, thirdRemote)
			select {
			case <-rejected:
			case <-time.After(5 * time.Second):
				t.Fatal("third connection rejection did not finish")
			}
			delayOpenWrite(t, pending, delayOpenRemote(id))
			if err := far.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			var octet [1]byte
			n, err := far.Read(octet[:])
			if n != 0 || !errors.Is(err, io.EOF) {
				t.Fatalf("OPEN-less loser received bytes or stayed open: n=%d err=%v", n, err)
			}
			delayOpenReadType(t, pending, 1)
			delayOpenReadType(t, pending, 4)
			delayOpenAwait(t, func() bool { return peer.currentSession() != s && peer.SessionState() == fsm.StateOpenConfirm })
			if s.State() != fsm.StateIdle || peer.ConnectRetryCounter() != 2 {
				t.Fatalf("collision did not count exactly once: state=%s counter=%d", s.State(), peer.ConnectRetryCounter())
			}
			if reason := s.closeReason.Load(); reason == nil || !errors.Is(*reason, ErrCollisionDump) {
				t.Fatal("collision lost close reason")
			}
			delayOpenWrite(t, pending, &message.Keepalive{})
			delayOpenAwait(t, func() bool { return peer.SessionState() == fsm.StateEstablished })
		}
	}
}

// TestDelayOpenConfigBoundary exercises the parser and the reflection-based
// restart decision, so changing this timer cannot hot-swap a live wait.
func TestDelayOpenConfigBoundary(t *testing.T) {
	for _, value := range []string{"0", "1", "240", "241"} {
		tree := map[string]any{
			"connection": map[string]any{"remote": map[string]any{"ip": "10.0.0.1"}, "local": map[string]any{"ip": "auto"}},
			"session":    map[string]any{"asn": map[string]any{"remote": "65001"}},
			"timer":      map[string]any{"delay-open": value},
		}
		settings, err := parsePeerFromTree("peer1", tree, 65000, 0)
		if value == "241" {
			if err == nil || !strings.Contains(err.Error(), "0..240") {
				t.Fatalf("invalid delay accepted: %v", err)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		want, err := time.ParseDuration(value + "s")
		if err != nil {
			t.Fatal(err)
		}
		if settings.DelayOpen != want {
			t.Fatalf("delay %s parsed as %s", value, settings.DelayOpen)
		}
		next := *settings
		next.DelayOpen = 2 * time.Second
		if !peerSettingsRestartRequired(settings, &next, nil) {
			t.Fatal("delay change did not restart session")
		}
	}
}

// TestRFC4271DelayOpenConnectFailureClearsTimer follows the held outbound
// socket through the real read-loop error and Peer cleanup. Firing its old
// clock afterward must not send OPEN or rearm any session timer.
func TestRFC4271DelayOpenConnectFailureClearsTimer(t *testing.T) {
	peer, session, far, clock := startDelayOpenPeer(t, false, 240*time.Second)
	peer.connectRetryCounter.Increment()
	closeConnQuietly(far)
	delayOpenAwait(t, func() bool { return peer.currentSession() != session })
	clock.Add(240 * time.Second)
	if session.State() != fsm.StateActive || peer.ConnectRetryCounter() != 1 {
		t.Fatalf("Connect failure: state=%s counter=%d", session.State(), peer.ConnectRetryCounter())
	}
	if session.Conn() != nil || session.timers.IsDelayOpenTimerRunning() ||
		session.timers.IsConnectRetryTimerRunning() || session.timers.IsHoldTimerRunning() ||
		session.timers.IsKeepaliveTimerRunning() {
		t.Fatal("retired session retained a socket or timer")
	}
}

func delayOpenDirectConfirm(t *testing.T, peer *Peer, from fsm.State) {
	t.Helper()
	found := false
	for _, transition := range peer.fSMHistory() {
		if transition.To == fsm.StateOpenSent.String() {
			t.Fatal("early OPEN took an intermediate OpenSent transition")
		}
		if transition.To == fsm.StateOpenConfirm.String() {
			found = true
			if transition.From != from.String() {
				t.Fatalf("OpenConfirm came from %s, want %s", transition.From, from)
			}
		}
	}
	if !found {
		t.Fatal("peer history omitted direct OpenConfirm transition")
	}
}
