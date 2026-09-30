// VALIDATES: RFC 5880 transmit and Echo obligations at the engine, where the
// packets actually leave: the express-loop tick and echoTickLocked, driven on a
// clock the test steps, with a transport that records every send.
// PREVENTS: a session-level rule (a deadline, a detector, a floor) that holds
// in isolation while the engine that should consult it ignores it.
package engine

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/packet"
	"github.com/ze-software/ze/internal/component/bfd/transport"
)

// sendLog is a transport that records the stepped-clock time of every send.
type sendLog struct {
	clk *steppedClock
	at  []time.Time
}

func (*sendLog) Start() error                    { return nil }
func (*sendLog) Stop() error                     { return nil }
func (*sendLog) RX() <-chan transport.Inbound    { return nil }
func (s *sendLog) Send(transport.Outbound) error { s.at = append(s.at, s.clk.now); return nil }

// tickFor calls tick every step from the clock's current time until span has
// passed, moving the clock each time.
func tickFor(l *Loop, clk *steppedClock, span, step time.Duration) {
	end := clk.now.Add(span)
	for clk.now.Before(end) {
		l.tick()
		clk.now = clk.now.Add(step)
	}
}

// RFC requirement: RFC5880-6.8.7-1 positive -- the gap between two Control
// packets the engine tick actually sends is not less than the larger of
// bfd.DesiredMinTxInterval and bfd.RemoteMinRxInterval, less applied jitter. The
// session is Down, so bfd.DesiredMinTxInterval is the one-second floor, and the
// peer advertises a Required Min RX Interval of 2 s, the larger. Ticking every
// millisecond for 20 s, every gap between consecutive sends lies between 1.5 s
// (2 s less the 25% jitter ceiling for Detect Mult 3) and 2 s plus one tick,
// and at least eight gaps are measured. A transmitter that used
// bfd.DesiredMinTxInterval alone would space packets 0.75 s to 1 s apart.
func TestRFC5880TickSpacesControlPacketsByLargerInterval(t *testing.T) {
	clk := &steppedClock{now: time.Unix(1_000_000, 0)}
	sent := &sendLog{clk: clk}
	l := NewLoop(sent, clk)
	req := reqFor(addrB, addrA)
	if _, err := l.EnsureSession(req); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	key := req.Key()
	m := machineFor(t, l, key)
	l.handleInbound(rfc5880Inbound(key.Peer, key.Local, key.Interface, m.LocalDiscriminator(), func(c *packet.Control) {
		c.State = packet.StateAdminDown
		c.RequiredMinRxInterval = 2_000_000
	}))
	if got := m.State(); got != packet.StateDown {
		t.Fatalf("precondition: state = %s, want Down", got)
	}
	if got := m.RemoteMinRxInterval(); got != 2*time.Second {
		t.Fatalf("precondition: bfd.RemoteMinRxInterval = %v, want 2s", got)
	}

	tickFor(l, clk, 20*time.Second, time.Millisecond)
	if len(sent.at) < 9 {
		t.Fatalf("the engine sent %d Control packets in 20 s, want at least 9", len(sent.at))
	}
	for i := 1; i < len(sent.at); i++ {
		gap := sent.at[i].Sub(sent.at[i-1])
		if gap < 1500*time.Millisecond {
			t.Fatalf("gap %d between Control packets is %v, below 2 s less 25%% jitter", i, gap)
		}
		if gap > 2*time.Second+time.Millisecond {
			t.Fatalf("gap %d between Control packets is %v, above the 2 s interval", i, gap)
		}
	}
}

// RFC requirement: RFC5880-6.8.7-5 positive -- the engine tick sends nothing
// for a Passive session whose bfd.RemoteDiscr is zero: ticking every 10 ms for
// 5 s leaves the transport untouched. Once a packet from the peer installs
// bfd.RemoteDiscr, the same tick sends, so the silence is the Passive rule and
// not a tick that never runs.
func TestRFC5880PassiveTickSendsNothingWithoutRemoteDiscr(t *testing.T) {
	clk := &steppedClock{now: time.Unix(1_000_000, 0)}
	sent := &sendLog{clk: clk}
	l := NewLoop(sent, clk)
	req := reqFor(addrB, addrA)
	req.Passive = true
	if _, err := l.EnsureSession(req); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	key := req.Key()
	m := machineFor(t, l, key)

	tickFor(l, clk, 5*time.Second, 10*time.Millisecond)
	if len(sent.at) != 0 {
		t.Fatalf("a Passive session with bfd.RemoteDiscr zero sent %d Control packets", len(sent.at))
	}

	l.handleInbound(rfc5880Inbound(key.Peer, key.Local, key.Interface, 0, nil))
	if got := m.RemoteDiscriminator(); got != peerMyDiscr {
		t.Fatalf("precondition: bfd.RemoteDiscr = %#x, want %#x", got, peerMyDiscr)
	}
	tickFor(l, clk, 10*time.Millisecond, 10*time.Millisecond)
	if len(sent.at) == 0 {
		t.Fatal("the Passive session sent nothing after learning bfd.RemoteDiscr")
	}
}

// RFC requirement: RFC5880-6.8.5-1 positive -- when Echo packets stop coming
// back, the running engine takes the session down: echoTickLocked sends an
// echo, nothing returns it, and the tick after the echo detection time leaves
// bfd.SessionState Down with bfd.LocalDiag 2 (Echo Function Failed). EchoFail
// is not called by the test.
// RFC requirement: RFC5880-6.8.8-2 positive -- the engine consults the missing
// echo detector on its own tick: the same unreturned echo is what brings the
// session down.
func TestRFC5880EngineFailsSessionOnMissingEchoes(t *testing.T) {
	l, echoCT, key := rfc5880EchoLoop(t)
	m := machineFor(t, l, key)
	t0 := time.Now()
	rfc5880EchoTick(l, t0)
	if !echoCT.sent {
		t.Fatal("precondition: the first echo must leave on the first tick")
	}

	rfc5880EchoTick(l, t0.Add(m.EchoDetectInterval()+time.Millisecond))
	if got := m.State(); got != packet.StateDown {
		t.Fatalf("state after the echo detection time with no echo returned = %s, want Down", got)
	}
	if got := m.LocalDiag(); got != packet.DiagEchoFailed {
		t.Fatalf("bfd.LocalDiag = %v, want Echo Function Failed (2)", got)
	}
}

// RFC requirement: RFC5880-6.8.8-2 negative -- the detector processes the
// echoes that come back: the engine sends an echo every Echo interval, each one
// is returned through handleEchoInbound, and after three echo detection times
// the session is still Up. A detector that did not match returned echoes would
// count every one as missing and fail the session.
func TestRFC5880EngineReturnedEchoesKeepSessionUp(t *testing.T) {
	l, echoCT, key := rfc5880EchoLoop(t)
	m := machineFor(t, l, key)
	step := m.EchoInterval()
	span := 3 * m.EchoDetectInterval()
	t0 := time.Now()
	for at := time.Duration(0); at <= span; at += step {
		echoCT.sent = false
		rfc5880EchoTick(l, t0.Add(at))
		if echoCT.sent {
			rfc5880ReturnEcho(l, key, echoCT)
		}
	}
	if got := m.State(); got != packet.StateUp {
		t.Fatalf("state after %v of returned echoes = %s (diag %v), want Up", span, got, m.LocalDiag())
	}
}

// RFC requirement: RFC5880-6.8.9-3 positive -- in steady state, with no change
// of the peer's value, the engine sends no echo before the peer's Required Min
// Echo RX Interval (50 ms) has passed since the previous one, although the
// local bfd.DesiredMinEchoTxInterval is 10 ms: ticks every millisecond from 1 to
// 49 ms send nothing, and the tick at 50 ms sends.
func TestRFC5880EchoSteadySpacingHonorsPeerFloor(t *testing.T) {
	l, echoCT, key := rfc5880EchoLoop(t)
	t0 := time.Now()
	rfc5880EchoTick(l, t0)
	if !echoCT.sent {
		t.Fatal("precondition: the first echo must leave on the first tick")
	}
	rfc5880ReturnEcho(l, key, echoCT)

	for at := time.Millisecond; at < 50*time.Millisecond; at += time.Millisecond {
		echoCT.sent = false
		rfc5880EchoTick(l, t0.Add(at))
		if echoCT.sent {
			t.Fatalf("echo sent %v after the previous one, below the peer's 50ms Required Min Echo RX", at)
		}
	}
	echoCT.sent = false
	rfc5880EchoTick(l, t0.Add(50*time.Millisecond))
	if !echoCT.sent {
		t.Fatal("no echo 50ms after the previous one")
	}
}

// RFC requirement: RFC5880-6.8.16-5 positive -- the administrative disable
// steps at the engine: after handle.Shutdown the session is AdminDown with
// bfd.LocalDiag 7 (Administratively Down), and the transmission of Echo packets
// ceases. The first echo is returned before the disable, so the echo detector
// stays clear and a tick has no reason to skip the send other than the state:
// ticks every millisecond for 100 ms, twice the peer's 50 ms Required Min Echo
// RX, send nothing, and the session is still AdminDown with diag 7 after them.
// An engine that kept the echo schedule of an AdminDown session would send at
// 50 ms.
func TestRFC5880AdminDisableCeasesEchoes(t *testing.T) {
	l, echoCT, key := rfc5880EchoLoop(t)
	m := machineFor(t, l, key)
	t0 := time.Now()
	rfc5880EchoTick(l, t0)
	if !echoCT.sent {
		t.Fatal("precondition: the first echo must leave on the first tick")
	}
	rfc5880ReturnEcho(l, key, echoCT)

	h := &handle{loop: l, key: key}
	if err := h.Shutdown(); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if got := m.State(); got != packet.StateAdminDown {
		t.Fatalf("state after Shutdown = %s, want AdminDown", got)
	}
	if got := m.LocalDiag(); got != packet.DiagAdminDown {
		t.Fatalf("bfd.LocalDiag after Shutdown = %v, want Administratively Down (7)", got)
	}
	for at := time.Millisecond; at <= 100*time.Millisecond; at += time.Millisecond {
		echoCT.sent = false
		rfc5880EchoTick(l, t0.Add(at))
		if echoCT.sent {
			t.Fatalf("an echo left %v after the session was administratively disabled", at)
		}
	}
	if got := m.State(); got != packet.StateAdminDown {
		t.Fatalf("state after the echo ticks = %s, want AdminDown", got)
	}
	if got := m.LocalDiag(); got != packet.DiagAdminDown {
		t.Fatalf("bfd.LocalDiag after the echo ticks = %v, want Administratively Down (7)", got)
	}
}

// RFC requirement: RFC5880-6.8.16-5 negative -- the peer keeps asking for
// echoes after the local disable: every 10 ms it sends Up with the
// discriminator it learned and a 50 ms Required Min Echo RX Interval, the input
// that sets an echo schedule on an Up session. The disabled session still
// ceases Echo transmission: ticks every millisecond for 100 ms send nothing,
// and the session stays AdminDown with diag 7, so the peer's packets neither
// re-enable the session nor restart its echoes.
func TestRFC5880AdminDisableIgnoresPeerEchoRequest(t *testing.T) {
	l, echoCT, key := rfc5880EchoLoop(t)
	m := machineFor(t, l, key)
	t0 := time.Now()
	rfc5880EchoTick(l, t0)
	if !echoCT.sent {
		t.Fatal("precondition: the first echo must leave on the first tick")
	}
	rfc5880ReturnEcho(l, key, echoCT)

	h := &handle{loop: l, key: key}
	if err := h.Shutdown(); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	for at := time.Millisecond; at <= 100*time.Millisecond; at += time.Millisecond {
		if at%(10*time.Millisecond) == 0 {
			rfc5880PeerUp(l, key, m.LocalDiscriminator(), false, 50_000)
		}
		echoCT.sent = false
		rfc5880EchoTick(l, t0.Add(at))
		if echoCT.sent {
			t.Fatalf("an echo left %v after the disable, on the peer's request", at)
		}
	}
	if got := m.State(); got != packet.StateAdminDown {
		t.Fatalf("state after the peer's Up packets = %s, want AdminDown", got)
	}
	if got := m.LocalDiag(); got != packet.DiagAdminDown {
		t.Fatalf("bfd.LocalDiag after the peer's Up packets = %v, want Administratively Down (7)", got)
	}
}
