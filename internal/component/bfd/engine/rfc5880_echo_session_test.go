package engine

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/packet"
	"github.com/ze-software/ze/internal/component/bfd/session"
	"github.com/ze-software/ze/internal/component/bfd/transport"
	"github.com/ze-software/ze/internal/core/clock"
)

// echoSessionFixture is one single-hop session with echo configured, driven by
// Control packets from its peer that advertise a Required Min Echo RX Interval,
// so echo is negotiated whatever state the session ends in.
type echoSessionFixture struct {
	key     api.Key
	machine *session.Machine
}

// echoSessionAdvertisesEcho sets the peer's Required Min Echo RX Interval to
// 50 ms on a Control packet.
func echoSessionAdvertisesEcho(c *packet.Control) { c.RequiredMinEchoRxInterval = 50_000 }

// echoSessionOpen creates the session to peer from local and delivers the
// peer's Down packet, which moves the session to Init with echo negotiated.
func echoSessionOpen(t *testing.T, l *Loop, peer string) echoSessionFixture {
	t.Helper()
	req := echoReqFor(peer, addrA)
	if _, err := l.EnsureSession(req); err != nil {
		t.Fatalf("EnsureSession %s: %v", peer, err)
	}
	key := req.Key()
	m := machineFor(t, l, key)
	l.handleInbound(rfc5880Inbound(key.Peer, key.Local, key.Interface, 0, echoSessionAdvertisesEcho))
	if m.State() != packet.StateInit {
		t.Fatalf("precondition: %s state = %v, want Init", peer, m.State())
	}
	if !m.EchoEnabled() {
		t.Fatalf("precondition: echo must be negotiated on %s", peer)
	}
	return echoSessionFixture{key: key, machine: m}
}

// echoSessionUp opens the session and delivers the peer's Init packet, which
// moves it to Up.
func echoSessionUp(t *testing.T, l *Loop, peer string) echoSessionFixture {
	t.Helper()
	f := echoSessionOpen(t, l, peer)
	l.handleInbound(rfc5880Inbound(f.key.Peer, f.key.Local, f.key.Interface, f.machine.LocalDiscriminator(), func(c *packet.Control) {
		c.State = packet.StateInit
		echoSessionAdvertisesEcho(c)
	}))
	if f.machine.State() != packet.StateUp {
		t.Fatalf("precondition: %s state = %v, want Up", peer, f.machine.State())
	}
	return f
}

// echoSessionTickSends runs one echo scheduler pass and answers whether it
// transmitted an echo packet.
func echoSessionTickSends(l *Loop, echoCT *captureTransport) bool {
	echoCT.sent = false
	rfc5880EchoTick(l, time.Now())
	return echoCT.sent
}

// RFC requirement: RFC5880-6.8.9-1 negative -- "not Up" includes Init and
// Down, not only AdminDown: a session in Init with echo negotiated, and a
// session that was Up with echo negotiated and fell to Down on the peer's Down
// packet, each transmit no echo packet on an echo scheduler pass and hold no
// armed echo schedule. The same pass on an Up session in the same loop does
// transmit, so the scheduler is live.
//
// VALIDATES: echoTickLocked gates echo transmission on bfd.SessionState == Up.
// PREVENTS: a gate that skipped AdminDown alone, which the AdminDown-only
// negative TestRFC5880NoEchoTransmittedWhenNotUp would not catch.
func TestRFC5880NoEchoTransmittedInInitOrDown(t *testing.T) {
	cases := []struct {
		name  string
		state packet.State
		drive func(t *testing.T, l *Loop) echoSessionFixture
	}{
		{name: "Init", state: packet.StateInit, drive: func(t *testing.T, l *Loop) echoSessionFixture {
			t.Helper()
			return echoSessionOpen(t, l, addrB)
		}},
		{name: "Down after Up", state: packet.StateDown, drive: func(t *testing.T, l *Loop) echoSessionFixture {
			t.Helper()
			f := echoSessionUp(t, l, addrB)
			l.handleInbound(rfc5880Inbound(f.key.Peer, f.key.Local, f.key.Interface, f.machine.LocalDiscriminator(), echoSessionAdvertisesEcho))
			return f
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			echoCT := &captureTransport{}
			l := NewLoopWithEcho(&captureTransport{}, echoCT, clock.RealClock{})
			f := tc.drive(t, l)
			if got := f.machine.State(); got != tc.state {
				t.Fatalf("precondition: state = %v, want %v", got, tc.state)
			}
			if !f.machine.EchoEnabled() {
				t.Fatal("precondition: echo must still be negotiated")
			}
			if echoSessionTickSends(l, echoCT) {
				t.Fatalf("an echo packet was transmitted by a session in %v", tc.state)
			}
			if !f.machine.NextEchoTxDeadline().IsZero() {
				t.Fatalf("echo schedule armed at %v on a session in %v", f.machine.NextEchoTxDeadline(), tc.state)
			}

			echoSessionUp(t, l, "203.0.113.9")
			if !echoSessionTickSends(l, echoCT) {
				t.Fatal("control: an Up session in the same loop transmitted no echo")
			}
		})
	}
}

// RFC requirement: RFC5880-6.8.8-1 positive -- with two Up echo sessions in
// one loop, a returning echo packet carrying the second session's Local
// Discriminator from the second peer records its round-trip time on the second
// session and leaves the first untouched, and the same packet shape for the
// first session records on the first only.
//
// VALIDATES: handleEchoInbound demultiplexes a returning echo to the session
// its discriminator names, among several.
// PREVENTS: an echo delivered to whichever session the loop holds, which a
// loop of one session cannot tell apart.
func TestRFC5880EchoDemultiplexedAmongSessions(t *testing.T) {
	l := NewLoopWithEcho(&captureTransport{}, &captureTransport{}, clock.RealClock{})
	first := echoSessionUp(t, l, addrB)
	second := echoSessionUp(t, l, "203.0.113.9")

	returnEcho := func(to echoSessionFixture, age time.Duration) {
		buf := make([]byte, packet.EchoLen)
		packet.WriteEcho(buf, 0, packet.Echo{
			LocalDiscriminator: to.machine.LocalDiscriminator(),
			Sequence:           7,
			TimestampMs:        uint32(time.Now().Add(-age).UnixMilli()),
		})
		l.handleEchoInbound(transport.Inbound{
			From:      to.key.Peer,
			Local:     to.key.Local,
			Interface: to.key.Interface,
			Mode:      api.SingleHop,
			TTL:       255,
			Bytes:     buf,
		})
	}

	returnEcho(second, 40*time.Millisecond)
	l.mu.Lock()
	firstRTT, secondRTT := first.machine.LastEchoRTT(), second.machine.LastEchoRTT()
	l.mu.Unlock()
	if secondRTT < 40*time.Millisecond {
		t.Fatalf("echo for the second session: its RTT = %v, want >= 40ms", secondRTT)
	}
	if firstRTT != 0 {
		t.Fatalf("echo for the second session recorded RTT %v on the first", firstRTT)
	}

	returnEcho(first, 80*time.Millisecond)
	l.mu.Lock()
	firstRTT = first.machine.LastEchoRTT()
	l.mu.Unlock()
	if firstRTT < 80*time.Millisecond {
		t.Fatalf("echo for the first session: its RTT = %v, want >= 80ms", firstRTT)
	}
}
