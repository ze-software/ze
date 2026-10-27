// Design: docs/architecture/behavior/peer-lifecycle.md — bounded DelayOpen collisions.
package reactor

import (
	"errors"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/test/sim"
)

// TestRFC4271DelayOpenPreBufferedOpen starts real Peer cycles with the winning
// socket and OPEN already in the slot. Both modes negotiate the local proposal
// and establish; the zero setting preserves the ordinary OpenSent path.
func TestRFC4271DelayOpenPreBufferedOpen(t *testing.T) {
	for _, mode := range []ConnectionMode{ConnectionBoth, ConnectionPassive} {
		for _, delay := range []time.Duration{0, 240 * time.Second} {
			settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101)
			settings.Connection, settings.DelayOpen = mode, delay
			peer := NewPeer(settings)
			local, far := net.Pipe()
			open := delayOpenRemote(0x02020202)
			peer.mu.Lock()
			peer.storeInboundLocked(inboundConnection{conn: local, open: open, wire: message.PackTo(open, nil)})
			peer.collisionHandoff = true
			peer.mu.Unlock()
			t.Cleanup(startAndStop(t, peer))
			t.Cleanup(func() { closeConnQuietly(far) })
			delayOpenReadType(t, far, 1)
			delayOpenReadType(t, far, 4)
			delayOpenAwait(t, func() bool { return peer.SessionState() == fsm.StateOpenConfirm })
			s := peer.currentSession()
			if s.Negotiated() == nil || s.timers.IsDelayOpenTimerRunning() {
				t.Fatal("buffered OPEN did not negotiate or clear delay")
			}
			if delay != 0 {
				from := fsm.StateConnect
				if mode == ConnectionPassive {
					from = fsm.StateActive
				}
				delayOpenDirectConfirm(t, peer, from)
			}
			delayOpenWrite(t, far, &message.Keepalive{})
			delayOpenAwait(t, func() bool { return peer.SessionState() == fsm.StateEstablished })
		}
	}
}

// TestDelayOpenExistingOpenFirst exercises order (a), with both identifier
// orders, after the pending socket was admitted while both sockets were silent.
func TestDelayOpenExistingOpenFirst(t *testing.T) {
	for _, id := range []uint32{0x01010100, 0x02020202} {
		peer, s, far, _ := startDelayOpenPeer(t, false, 240*time.Second)
		local, pending := net.Pipe()
		t.Cleanup(func() { closeConnQuietly(pending) })
		r := &Reactor{clock: clock.RealClock{}}
		r.acceptOrReject(local, peer, nil)
		if !peer.hasPendingConnection() {
			t.Fatal("pending socket not admitted")
		}
		delayOpenWrite(t, far, delayOpenRemote(id))
		delayOpenReadType(t, far, 1)
		delayOpenReadType(t, far, 4)
		delayOpenAwait(t, func() bool { return s.State() == fsm.StateOpenConfirm })
		delayOpenWrite(t, pending, delayOpenRemote(id))
		winner := far
		if id > peer.settings.RouterID {
			notification := delayOpenReadType(t, far, 3)
			if notification[19] != byte(message.NotifyCease) {
				t.Fatal("existing loser not sent Cease")
			}
			closeConnQuietly(far)
			delayOpenReadType(t, pending, 1)
			delayOpenReadType(t, pending, 4)
			winner = pending
		} else {
			notification := delayOpenReadType(t, pending, 3)
			if notification[19] != byte(message.NotifyCease) {
				t.Fatal("pending loser not sent Cease")
			}
		}
		delayOpenWrite(t, winner, &message.Keepalive{})
		delayOpenAwait(t, func() bool { return peer.SessionState() == fsm.StateEstablished })
	}
}

// TestDelayOpenHandoffRechecksOpenConfirm forces the race between storing the
// inbound winner and closing the held connection, including an already-consumed
// wakeup. The close helper must do nothing once the existing OPEN won.
func TestDelayOpenHandoffRechecksOpenConfirm(t *testing.T) {
	for _, id := range []uint32{0x01010100, 0x02020202} {
		for _, consumeWakeup := range []bool{false, true} {
			peer, s, far, _ := startDelayOpenPeer(t, false, 240*time.Second)
			local, pending := net.Pipe()
			t.Cleanup(func() { closeConnQuietly(pending) })
			open := delayOpenRemote(id)
			peer.mu.Lock()
			peer.storeInboundLocked(inboundConnection{conn: local, open: open, wire: message.PackTo(open, nil)})
			peer.collisionHandoff = true
			if consumeWakeup {
				<-peer.inboundNotify
			}
			peer.mu.Unlock()
			delayOpenWrite(t, far, open)
			delayOpenReadType(t, far, 1)
			delayOpenReadType(t, far, 4)
			delayOpenAwait(t, func() bool { return s.State() == fsm.StateOpenConfirm })
			if s.closeHeldForCollision() {
				t.Fatal("held close retired an OpenConfirm connection")
			}
			finished := make(chan struct{})
			go func() { peer.finishCollisionHandoff(s, local, id); close(finished) }()
			if id < peer.settings.RouterID {
				delayOpenReadCollisionClose(t, pending)
				select {
				case <-finished:
				case <-time.After(5 * time.Second):
					t.Fatal("take-back blocked on consumed wakeup")
				}
				peer.mu.RLock()
				reserved, inbound := peer.collisionHandoff, peer.inbound.conn
				peer.mu.RUnlock()
				if reserved || inbound != nil {
					t.Fatal("take-back retained inbound reservation")
				}
				select {
				case <-peer.inboundNotify:
					t.Fatal("take-back retained wakeup")
				default:
				}
				delayOpenWrite(t, far, &message.Keepalive{})
			} else {
				delayOpenReadType(t, far, 3)
				closeConnQuietly(far)
				<-finished
				// The test consumed the token to model an already-woken run loop;
				// restore that external wakeup before expecting its next cycle.
				if consumeWakeup {
					peer.inboundNotify <- struct{}{}
				}
				delayOpenReadType(t, pending, 1)
				delayOpenReadType(t, pending, 4)
				delayOpenWrite(t, pending, &message.Keepalive{})
			}
			delayOpenAwait(t, func() bool { return peer.SessionState() == fsm.StateEstablished })
		}
	}
}

// TestDelayOpenTimerWinsCollision exercises order (c): Event 12 sends OPEN on
// the existing socket, and the pending OPEN follows the unchanged OpenSent rule.
func TestDelayOpenTimerWinsCollision(t *testing.T) {
	peer, s, far, fc := startDelayOpenPeer(t, false, 240*time.Second)
	local, pending := net.Pipe()
	t.Cleanup(func() { closeConnQuietly(pending) })
	r := &Reactor{clock: clock.RealClock{}}
	r.acceptOrReject(local, peer, nil)
	advanced := make(chan struct{})
	go func() { fc.Add(240 * time.Second); close(advanced) }()
	delayOpenReadType(t, far, 1)
	<-advanced
	if s.State() != fsm.StateOpenSent {
		t.Fatal("expiry did not enter OpenSent")
	}
	delayOpenWrite(t, pending, delayOpenRemote(0x02020202))
	delayOpenReadType(t, pending, 3)
	delayOpenWrite(t, far, delayOpenRemote(0x02020202))
	delayOpenReadType(t, far, 4)
	delayOpenWrite(t, far, &message.Keepalive{})
	delayOpenAwait(t, func() bool { return peer.SessionState() == fsm.StateEstablished })
}

// TestDelayOpenPendingDeadlineOnlyDropsPending gives the pending reader an
// already-expired ReceiveHoldTime deadline; the held session remains untouched.
func TestDelayOpenPendingDeadlineOnlyDropsPending(t *testing.T) {
	peer, s, far, _ := startDelayOpenPeer(t, true, 240*time.Second)
	local, pending := net.Pipe()
	t.Cleanup(func() { closeConnQuietly(pending) })
	r := &Reactor{clock: sim.NewFakeClock(time.Now().Add(-91 * time.Second))}
	r.acceptOrReject(local, peer, nil)
	if err := pending.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var octet [1]byte
	if n, err := pending.Read(octet[:]); n != 0 || !errors.Is(err, io.EOF) {
		t.Fatalf("pending expiry: %d %v", n, err)
	}
	if !s.holdsOpen() || s.Conn() == nil || peer.hasPendingConnection() {
		t.Fatal("pending deadline damaged held connection")
	}
	delayOpenNoBytes(t, far)
}

// TestDelayOpenActiveOnlyRefusesPending keeps the existing ConnectionActive
// admission policy even while an outbound socket is held without an OPEN.
func TestDelayOpenActiveOnlyRefusesPending(t *testing.T) {
	peer, session, far, _ := startDelayOpenPeer(t, false, 240*time.Second, func(peer *Peer) {
		peer.settings.Connection = ConnectionActive
	})
	local, pending := net.Pipe()
	t.Cleanup(func() { closeConnQuietly(pending) })
	r := &Reactor{clock: clock.RealClock{}}
	r.acceptOrReject(local, peer, nil)
	if err := pending.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var octet [1]byte
	if n, err := pending.Read(octet[:]); n != 0 || !errors.Is(err, io.EOF) {
		t.Fatalf("active-only accepted pending socket: bytes=%d error=%v", n, err)
	}
	if peer.hasPendingConnection() || !session.holdsOpen() {
		t.Fatal("active-only changed the held session")
	}
	delayOpenNoBytes(t, far)
}

// TestDelayOpenHandoffAlreadyTaken preserves ownership if the run loop took
// the inbound slot before a keep-existing arbitration answer could take it back.
func TestDelayOpenHandoffAlreadyTaken(t *testing.T) {
	peer, session, far, _ := startDelayOpenPeer(t, false, 240*time.Second)
	open := delayOpenRemote(0x01010100)
	local, pending := net.Pipe()
	t.Cleanup(func() { closeConnQuietly(local); closeConnQuietly(pending) })
	peer.mu.Lock()
	peer.storeInboundLocked(inboundConnection{conn: local, open: open, wire: message.PackTo(open, nil)})
	peer.collisionHandoff = true
	peer.mu.Unlock()
	taken := peer.takeInboundConnection()
	if taken.conn != local {
		t.Fatal("winner was not transferred to the run-loop owner")
	}
	delayOpenWrite(t, far, open)
	delayOpenReadType(t, far, 1)
	delayOpenReadType(t, far, 4)
	peer.finishCollisionHandoff(session, local, open.BGPIdentifier)
	delayOpenNoBytes(t, pending) // EOF would mean the previous owner closed it.
	peer.mu.RLock()
	reserved := peer.collisionHandoff
	peer.mu.RUnlock()
	if !reserved {
		t.Fatal("old owner released the accepting run-loop reservation")
	}
	peer.mu.Lock()
	peer.collisionHandoff = false // this test is the owner of the taken slot
	peer.mu.Unlock()
	delayOpenWrite(t, far, &message.Keepalive{})
	delayOpenAwait(t, func() bool { return peer.SessionState() == fsm.StateEstablished })
}
