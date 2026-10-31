// Design: docs/architecture/behavior/fsm.md — the read loop's owed Event 18 and timer retire.
package reactor

import (
	"net"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
)

// newOwedWorkSession builds a Session in Connect, so Event 18 is a real
// transition that reaches the FSM callback.
func newOwedWorkSession(t *testing.T) *Session {
	t.Helper()
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	session := NewSession(settings)
	if err := session.fsm.Event(fsm.EventManualStart); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.retireTimers() })
	return session
}

// panicCloseConn panics on Close: the owed Event 18 delivery closes the
// connection (handleConnectionCloseLocked -> closeConn), so it panics there.
type panicCloseConn struct{ net.Conn }

func (panicCloseConn) Close() error { panic("BUG: test connection panics on Close") }

// sendHoldArmed reports whether the Send Hold Timer is armed.
func sendHoldArmed(s *Session) bool {
	s.sendHoldMu.Lock()
	defer s.sendHoldMu.Unlock()
	return s.sendHoldTimer != nil
}

// TestOwedEvent18PanicReleasesOpenMu checks that a panic in the owed Event 18
// delivery leaves openMu free. Method: install a connection whose Close
// panics, which the Event 18 delivery reaches through closeConn, run the
// owed work from each of its two runners (a releasing holder and the read
// loop with the lock free), recover the panic, then require that openMu can
// be taken and that the Peer's final stopSessionTimers still runs the retire
// the panic left owed. A lock left held blocks that final join for ever.
func TestOwedEvent18PanicReleasesOpenMu(t *testing.T) {
	runners := map[string]func(*Session){
		"holder releases": func(s *Session) {
			s.openMu.Lock()
			s.closeOwed.Store(true)
			s.retireOwed.Store(true)
			s.unlockOpen()
		},
		"read loop with the lock free": func(s *Session) {
			s.retireOwed.Store(true)
			s.handleConnectionClose()
		},
	}
	for name, run := range runners {
		t.Run(name, func(t *testing.T) {
			session := newOwedWorkSession(t)
			session.timers.StartKeepaliveTimer()
			local, remote := net.Pipe()
			t.Cleanup(func() {
				_ = local.Close()
				_ = remote.Close()
			})
			session.mu.Lock()
			session.conn = panicCloseConn{Conn: local}
			session.mu.Unlock()

			recovered := func() (value any) {
				defer func() { value = recover() }()
				run(session)
				return nil
			}()
			if recovered == nil {
				t.Fatal("the owed Event 18 did not reach the panicking Close")
			}
			if !session.openMu.TryLock() {
				t.Fatal("openMu stayed locked after a panic in the owed Event 18")
			}
			session.openMu.Unlock()

			session.stopSessionTimers()
			if session.retireOwed.Load() {
				t.Error("stopSessionTimers returned with the retire still owed")
			}
			if session.timers.IsKeepaliveTimerRunning() {
				t.Error("keepalive timer still armed after stopSessionTimers")
			}
		})
	}
}

// TestOwedRetireStopsSendHoldAndSeals checks the retire Run owes to an openMu
// holder. Method: hold openMu as a BFD release would, let Run's exit retire
// become owed, arm the keepalive and Send Hold Timers from inside the held
// lock as the release's Established branch does, then release. No timer may
// remain armed, and an arm after the retire must arm nothing.
func TestOwedRetireStopsSendHoldAndSeals(t *testing.T) {
	session := newOwedWorkSession(t)

	session.openMu.Lock()
	session.retireSessionTimers()
	if !session.retireOwed.Load() {
		session.openMu.Unlock()
		t.Fatal("retire ran while openMu was held")
	}
	session.timers.StartKeepaliveTimer()
	session.startSendHoldTimer()
	session.unlockOpen()

	if sendHoldArmed(session) {
		t.Error("Send Hold Timer armed after the owed retire: it fires on a dead session")
	}
	if session.timers.IsKeepaliveTimerRunning() {
		t.Error("keepalive timer armed after the owed retire")
	}

	session.startSendHoldTimer()
	session.timers.StartHoldTimer()
	session.timers.StartKeepaliveTimer()
	if sendHoldArmed(session) {
		t.Error("startSendHoldTimer armed the Send Hold Timer after the retire")
	}
	if session.timers.IsHoldTimerRunning() {
		t.Error("StartHoldTimer armed the HoldTimer after the retire")
	}
	if session.timers.IsKeepaliveTimerRunning() {
		t.Error("StartKeepaliveTimer armed the keepalive timer after the retire")
	}
}
