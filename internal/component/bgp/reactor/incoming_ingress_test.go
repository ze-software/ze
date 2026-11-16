// Design: docs/architecture/behavior/peer-lifecycle.md -- validated incoming admission.
// Related: rfc4271_incoming_listener_test.go -- real listener and useful incumbent.
// RFC 4271 Sections 6.1, 6.2 and 8.2.2 -- see rfc/short/rfc4271.md.
package reactor

import (
	"bytes"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/test/sim"
)

// TestIncomingInvalidWinningOpenPreservesIncumbent sends decodable but invalid
// OPENs with a winning Identifier to two real incumbent states. Validation must
// refuse only the candidate, before collision retirement; the original socket
// must still complete its own useful handshake.
func TestIncomingInvalidWinningOpenPreservesIncumbent(t *testing.T) {
	for _, state := range []fsm.State{fsm.StateConnect, fsm.StateOpenConfirm} {
		for _, invalid := range []struct {
			name   string
			change func(*message.Open)
			reply  []byte
		}{
			{name: "version", change: func(open *message.Open) { open.Version = 5 }, reply: []byte{2, 1, 0, 4}},
			{name: "hold-time", change: func(open *message.Open) { open.HoldTime = 1 }, reply: []byte{2, 6, 0, 1}},
			{name: "peer-AS", change: func(open *message.Open) { open.MyAS = 65002 }, reply: []byte{2, 2}},
		} {
			t.Run(state.String()+"/"+invalid.name, func(t *testing.T) {
				delay := time.Duration(0)
				if state == fsm.StateConnect {
					delay = 240 * time.Second
				}
				var candidateClock *sim.FakeClock
				var configure []func(*Peer)
				if delay != 0 {
					candidateClock = sim.NewFakeClock(time.Now())
					configure = append(configure, func(peer *Peer) { peer.SetClock(candidateClock) })
				}
				r, peer, original, outgoing := incomingIngressPeer(t, delay, state, configure...)
				incoming := incomingListenerDial(t, r)
				if delay == 0 {
					delayOpenReadType(t, incoming, 1)
				} else {
					// Exact OPEN-error notifications are unconditional only after
					// the candidate sent its own OPEN. Expire its real timer while
					// the incumbent's separate timer clock remains held.
					delayOpenAwait(t, func() bool {
						peer.mu.RLock()
						defer peer.mu.RUnlock()
						candidate, ok := peer.pending.(*incomingCandidate)
						return ok && candidate.session.holdsOpen()
					})
					advanced := make(chan struct{})
					go func() { defer close(advanced); candidateClock.Add(delay) }()
					delayOpenReadType(t, incoming, 1)
					select {
					case <-advanced:
					case <-time.After(5 * time.Second):
						t.Fatal("candidate DelayOpen expiry did not finish")
					}
					if !original.holdsOpen() {
						t.Fatal("candidate timer advancement released the incumbent")
					}
				}
				open := delayOpenRemote(0x03030303)
				invalid.change(open)
				delayOpenWrite(t, incoming, open)
				if err := incoming.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
					t.Fatal(err)
				}
				reply, err := core4271ReadMessage(incoming)
				if err != nil {
					peer.mu.RLock()
					pending, inbound, handoff, current := peer.pending, peer.inbound, peer.collisionHandoff, peer.session
					peer.mu.RUnlock()
					t.Fatalf("invalid OPEN response: %v; incumbent=%s current=%p pending=%T inbound=%T handoff=%t", err, original.State(), current, pending, inbound, handoff)
				}
				if reply[18] != 3 {
					t.Fatalf("invalid OPEN response type=%d, want NOTIFICATION: %x", reply[18], reply)
				}
				if !bytes.Equal(reply[message.HeaderLen:], invalid.reply) {
					t.Fatalf("invalid OPEN notification=%x, want %x", reply[message.HeaderLen:], invalid.reply)
				}
				incomingIngressEOF(t, incoming)
				if peer.currentSession() != original || original.State() != state {
					t.Fatalf("invalid winning OPEN retired incumbent: current=%p original=%p state=%s, want %s", peer.currentSession(), original, original.State(), state)
				}
				incomingIngressComplete(t, peer, original, outgoing, state)
			})
		}
	}
}

// TestIncomingFirstHeaderErrorsUseSessionDisposition checks every first-header
// refusal before a body is read. The incoming Session must return the exact
// RFC 4271 header error and preserve the original OpenConfirm connection.
func TestIncomingFirstHeaderErrorsUseSessionDisposition(t *testing.T) {
	for _, invalid := range []struct {
		name   string
		marker byte
		length uint16
		kind   byte
		reply  []byte
	}{
		{name: "marker", marker: 0, length: 29, kind: 1, reply: []byte{1, 1}},
		{name: "short-header", marker: 255, length: 18, kind: 1, reply: []byte{1, 2, 0, 18}},
		{name: "short-OPEN", marker: 255, length: 28, kind: 1, reply: []byte{1, 2, 0, 28}},
		{name: "oversized-OPEN", marker: 255, length: 4097, kind: 1, reply: []byte{1, 2, 16, 1}},
		{name: "unknown-type", marker: 255, length: 19, kind: 255, reply: []byte{1, 3, 255}},
	} {
		t.Run(invalid.name, func(t *testing.T) {
			r, peer, original, outgoing := incomingIngressPeer(t, 0, fsm.StateOpenConfirm)
			incoming := incomingListenerDial(t, r)
			delayOpenReadType(t, incoming, 1)
			var header [message.HeaderLen]byte
			for i := range message.MarkerLen {
				header[i] = 255
			}
			header[0] = invalid.marker
			header[16], header[17] = byte(invalid.length>>8), byte(invalid.length)
			header[18] = invalid.kind
			if _, err := incoming.Write(header[:]); err != nil {
				t.Fatal(err)
			}
			reply := delayOpenReadType(t, incoming, 3)
			if !bytes.Equal(reply[message.HeaderLen:], invalid.reply) {
				t.Fatalf("first-header notification=%x, want %x", reply[message.HeaderLen:], invalid.reply)
			}
			incomingIngressEOF(t, incoming)
			incomingIngressComplete(t, peer, original, outgoing, fsm.StateOpenConfirm)
		})
	}
}

// TestIncomingVersionNotificationKeepsRetryHistory receives Event24 on a held
// candidate. Unlike TCP failure, the version-error NOTIFICATION closes silently
// without incrementing history; the original connection still establishes.
func TestIncomingVersionNotificationKeepsRetryHistory(t *testing.T) {
	r, peer, original, outgoing := incomingIngressPeer(t, 240*time.Second, fsm.StateConnect)
	incoming := incomingListenerDial(t, r)
	var candidate *incomingCandidate
	delayOpenAwait(t, func() bool {
		peer.mu.RLock()
		defer peer.mu.RUnlock()
		var ok bool
		candidate, ok = peer.pending.(*incomingCandidate)
		return ok && candidate.session.holdsOpen()
	})
	peer.connectRetryCounter.Increment()
	peer.connectRetryCounter.Increment()
	before := peer.ConnectRetryCounter()
	delayOpenWrite(t, incoming, &message.Notification{ErrorCode: message.NotifyOpenMessage, ErrorSubcode: message.NotifyOpenUnsupportedVersion, Data: []byte{0, 4}})
	incomingIngressEOF(t, incoming)
	delayOpenAwait(t, func() bool {
		peer.mu.RLock()
		defer peer.mu.RUnlock()
		return peer.pending == nil
	})
	if got := peer.ConnectRetryCounter(); got != before {
		t.Fatalf("version-error candidate changed retry history: got %d, want %d", got, before)
	}
	if candidate.session.State() != fsm.StateIdle || candidate.session.timers.IsDelayOpenTimerRunning() {
		t.Fatal("received version error did not retire its own DelayOpen FSM")
	}
	incomingIngressComplete(t, peer, original, outgoing, fsm.StateConnect)
}

// TestIncomingRefusalStopsDelayOpenBeforeNotification holds a completed
// NOTIFICATION write before it returns. The terminal decision must already
// stop/seal DelayOpen, so advancing its clock cannot append OPEN after refusal.
func TestIncomingRefusalStopsDelayOpenBeforeNotification(t *testing.T) {
	fc := sim.NewFakeClock(time.Now())
	r, peer, original, outgoing, _ := incomingListenerPeer(t, true, 240*time.Second, func(peer *Peer) { peer.SetClock(fc) })
	incomingIngressComplete(t, peer, original, outgoing, fsm.StateConnect)
	remote, local := collisionProofTCPPair(t)
	gate := &incomingIngressWriteGate{Conn: local, written: make(chan struct{}), release: make(chan struct{})}
	release := sync.OnceFunc(func() { close(gate.release) })
	t.Cleanup(release)
	r.acceptOrReject(gate, peer, nil)
	var candidate *incomingCandidate
	delayOpenAwait(t, func() bool {
		peer.mu.RLock()
		defer peer.mu.RUnlock()
		var ok bool
		candidate, ok = peer.pending.(*incomingCandidate)
		return ok && candidate.session.holdsOpen()
	})
	delayOpenWrite(t, remote, delayOpenRemote(0x03030303))
	reply := delayOpenReadType(t, remote, 3)
	if !bytes.Equal(reply[message.HeaderLen:], []byte{6, 7}) {
		t.Fatalf("collision refusal=%x, want 0607", reply[message.HeaderLen:])
	}
	select {
	case <-gate.written:
	case <-time.After(5 * time.Second):
		t.Fatal("the refusal did not reach its completed-write gate")
	}
	if candidate.session.timers.IsDelayOpenTimerRunning() {
		t.Fatal("refusal bytes were sent while the candidate still owned a running DelayOpen timer")
	}
	if !candidate.session.tearingDown.Load() {
		t.Fatal("a terminal refusal did not seal its candidate before writing")
	}
	advanced := make(chan struct{})
	go func() { defer close(advanced); fc.Add(240 * time.Second) }()
	release()
	select {
	case <-advanced:
	case <-time.After(5 * time.Second):
		t.Fatal("retired candidate timer did not finish after refusal release")
	}
	incomingIngressEOF(t, remote)
	if peer.currentSession() != original || original.State() != fsm.StateEstablished {
		t.Fatal("candidate refusal changed the useful Established incumbent")
	}
	incomingCaptureWithdrawal(t, peer, outgoing)
}

func incomingIngressPeer(t *testing.T, delay time.Duration, state fsm.State, configure ...func(*Peer)) (*Reactor, *Peer, *Session, net.Conn) {
	t.Helper()
	r, peer, original, outgoing, _ := incomingListenerPeer(t, true, delay, configure...)
	if delay == 0 {
		delayOpenReadType(t, outgoing, 1)
	}
	if state != fsm.StateConnect {
		delayOpenWrite(t, outgoing, delayOpenRemote(0x02020202))
		if delay != 0 {
			delayOpenReadType(t, outgoing, 1)
		}
		delayOpenReadType(t, outgoing, 4)
	}
	delayOpenAwait(t, func() bool { return original.State() == state })
	if state == fsm.StateConnect {
		// StateConnect precedes TCP publication. The second accepted socket
		// must arrive only once the original actually owns its held connection.
		delayOpenAwait(t, original.holdsOpen)
	}
	return r, peer, original, outgoing
}

func incomingIngressComplete(t *testing.T, peer *Peer, original *Session, outgoing net.Conn, from fsm.State) {
	t.Helper()
	if peer.currentSession() != original || original.State() != from {
		t.Fatalf("original connection no longer usable: current=%p original=%p state=%s, want %s", peer.currentSession(), original, original.State(), from)
	}
	if from == fsm.StateConnect {
		delayOpenWrite(t, outgoing, delayOpenRemote(0x02020202))
		delayOpenReadType(t, outgoing, 1)
		delayOpenReadType(t, outgoing, 4)
	}
	delayOpenWrite(t, outgoing, &message.Keepalive{})
	delayOpenAwait(t, func() bool { return original.State() == fsm.StateEstablished })
	if peer.currentSession() != original {
		t.Fatal("useful handshake replaced the original Session")
	}
}

func incomingIngressEOF(t *testing.T, conn net.Conn) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var octet [1]byte
	if count, err := conn.Read(octet[:]); count != 0 || !errors.Is(err, io.EOF) {
		t.Fatalf("terminal candidate disposition: bytes=%d error=%v, want silent EOF", count, err)
	}
}

// incomingIngressWriteGate owns one test transport. Its test MUST release it
// before Reactor cleanup joins the candidate's refusal worker.
type incomingIngressWriteGate struct {
	net.Conn
	written chan struct{}
	release chan struct{}
	once    sync.Once
}

func (c *incomingIngressWriteGate) Write(data []byte) (int, error) {
	count, err := c.Conn.Write(data)
	if len(data) < message.HeaderLen {
		return count, err
	}
	if data[18] != 3 {
		return count, err
	}
	if err != nil {
		return count, err
	}
	if count != len(data) {
		return count, err
	}
	c.once.Do(func() { close(c.written) })
	<-c.release
	return count, err
}
