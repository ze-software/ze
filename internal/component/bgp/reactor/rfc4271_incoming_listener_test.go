// Design: docs/architecture/behavior/peer-lifecycle.md — incoming connection ownership.
// Related: delay_open_test.go — protocol timer clock and wire helpers.
// RFC: rfc/short/rfc4271.md — Sections 8.2.1 and 8.2.2.
// RFC naming: untagged -- listener-level incoming FSM defect probes and DelayOpen controls await baseline and discrimination proof.

package reactor

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/test/sim"
)

// incomingListenerPeer starts the Reactor's real listener and peer lifecycle.
// The outbound variant holds its dial until the session's timer clock is set.
// Only protocol timers use fake time; socket deadlines and shutdown stay real.
func incomingListenerPeer(t *testing.T, outbound bool, delay time.Duration) (*Reactor, *Peer, *Session, net.Conn, *sim.FakeClock) {
	t.Helper()
	settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101)
	settings.Connection = ConnectionPassive
	settings.DelayOpen = delay
	var remoteListener *net.TCPListener
	var dialer *heldDialer
	if outbound {
		settings.Connection = ConnectionBoth
		var lc net.ListenConfig
		listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { closeConnQuietlyListener(listener) })
		var ok bool
		remoteListener, ok = listener.(*net.TCPListener)
		if !ok {
			t.Fatal("outbound control listener is not TCP")
		}
		addr, ok := listener.Addr().(*net.TCPAddr)
		if !ok {
			t.Fatal("outbound control address is not TCP")
		}
		settings.Port = uint16(addr.Port)
		if err := remoteListener.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatal(err)
		}
		dialer = newHeldDialer()
	}
	r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
	if err := r.AddPeer(settings); err != nil {
		t.Fatal(err)
	}
	peer := r.Peers()[0]
	peer.setReconnectDelay(time.Hour, time.Hour)
	if dialer != nil {
		peer.SetDialer(dialer)
	}
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		r.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := r.Wait(ctx); err != nil {
			t.Errorf("incoming listener reactor did not stop: %v", err)
		}
	})
	var session *Session
	delayOpenAwait(t, func() bool {
		session = peer.currentSession()
		return session != nil && session.State() != fsm.StateIdle
	})
	fc := sim.NewFakeClock(time.Now())
	session.timers.SetClock(fc)
	var outgoing net.Conn
	if dialer != nil {
		close(dialer.release)
		var err error
		outgoing, err = remoteListener.Accept()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { closeConnQuietly(outgoing) })
	}
	return r, peer, session, outgoing, fc
}

func incomingListenerDial(t *testing.T, r *Reactor) net.Conn {
	t.Helper()
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(t.Context(), "tcp", r.ListenAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeConnQuietly(conn) })
	return conn
}

func incomingListenerEstablish(t *testing.T, peer *Peer, conn net.Conn) {
	t.Helper()
	delayOpenWrite(t, conn, delayOpenRemote(0x02020202))
	delayOpenReadType(t, conn, 4)
	delayOpenWrite(t, conn, &message.Keepalive{})
	delayOpenAwait(t, func() bool { return peer.SessionState() == fsm.StateEstablished })
}

// VALIDATES: with DelayOpen FALSE, an initial incoming TCP connection receives
// Ze's OPEN without first sending its own OPEN, then completes a valid handshake.
// RFC 4271 Section 8.2.2, Active: "If the DelayOpen attribute is set to FALSE,
// the local system:" includes "sends the OPEN message to its peer,".
func TestRFC4271ListenerIncomingDefaultOpen(t *testing.T) {
	r, peer, _, _, _ := incomingListenerPeer(t, false, 0)
	conn := incomingListenerDial(t, r)
	delayOpenReadType(t, conn, 1)
	incomingListenerEstablish(t, peer, conn)
}

// VALIDATES: an incoming connection while the outgoing connection is OpenSent
// has its own default (DelayOpen FALSE) OPEN exchange. The outgoing connection
// already received a real OPEN; the incoming peer deliberately sends no OPEN
// until it receives Ze's, so a pending reader cannot impersonate its FSM.
// RFC 4271 Section 8.2.1: "For each incoming connection, a state machine MUST
// be instantiated." Section 8.2.1.2: "There is one FSM per BGP connection."
func TestRFC4271ListenerIncomingDuringOutgoingOpenSentSendsOpen(t *testing.T) {
	r, peer, session, outgoing, _ := incomingListenerPeer(t, true, 0)
	delayOpenReadType(t, outgoing, 1)
	delayOpenAwait(t, func() bool { return session.State() == fsm.StateOpenSent })
	incoming := incomingListenerDial(t, r)
	delayOpenReadType(t, incoming, 1)
	// The remote Identifier is higher, so this incoming connection wins the
	// later collision; merely writing OPEN on every socket cannot pass.
	incomingListenerEstablish(t, peer, incoming)
}

// VALIDATES: a configured DelayOpen on the actual incoming listener withholds
// OPEN until its timer expires, then sends OPEN and permits establishment.
// RFC 4271 Section 8.2.2, Active: "If the local system receives a
// DelayOpenTimer_Expires event (Event 12), the local system:" includes "sends
// the OPEN message to its remote peer," and "changes its state to OpenSent."
func TestRFC4271ListenerIncomingDelayOpenExpiry(t *testing.T) {
	const delay = 10 * time.Second
	r, peer, session, _, fc := incomingListenerPeer(t, false, delay)
	conn := incomingListenerDial(t, r)
	delayOpenAwait(t, session.timers.IsDelayOpenTimerRunning)
	if session.State() != fsm.StateActive {
		t.Fatalf("held incoming state %s, want Active", session.State())
	}
	fc.Add(delay - time.Nanosecond)
	delayOpenNoBytes(t, conn)
	fc.Add(time.Nanosecond)
	delayOpenReadType(t, conn, 1)
	if session.State() != fsm.StateOpenSent {
		t.Fatalf("expired incoming state %s, want OpenSent", session.State())
	}
	incomingListenerEstablish(t, peer, conn)
}

// VALIDATES: a valid peer OPEN received on the listener while DelayOpen runs
// releases local OPEN followed by KEEPALIVE without waiting for timer expiry.
// RFC 4271 Section 8.2.2, Active: "If an OPEN message is received and the
// DelayOpenTimer is running (Event 20), the local system:" includes "sends an
// OPEN message," and "sends a KEEPALIVE message,".
func TestRFC4271ListenerIncomingDelayOpenReceivedOpen(t *testing.T) {
	r, peer, session, _, _ := incomingListenerPeer(t, false, 10*time.Second)
	conn := incomingListenerDial(t, r)
	delayOpenAwait(t, session.timers.IsDelayOpenTimerRunning)
	delayOpenNoBytes(t, conn)
	delayOpenWrite(t, conn, delayOpenRemote(0x02020202))
	delayOpenReadType(t, conn, 1)
	delayOpenReadType(t, conn, 4)
	delayOpenAwait(t, func() bool { return session.State() == fsm.StateOpenConfirm })
	if session.timers.IsDelayOpenTimerRunning() {
		t.Fatal("received OPEN did not clear the incoming DelayOpenTimer")
	}
	delayOpenWrite(t, conn, &message.Keepalive{})
	delayOpenAwait(t, func() bool { return peer.SessionState() == fsm.StateEstablished })
}
