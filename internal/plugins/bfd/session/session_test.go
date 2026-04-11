package session

import (
	"errors"
	"net/netip"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/packet"
)

// fakeClock is a controllable clock used by every test in this file.
type fakeClock struct{ t time.Time }

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, time.April, 11, 0, 0, 0, 0, time.UTC)}
}

func (f *fakeClock) Now() time.Time                              { return f.t }
func (f *fakeClock) Sleep(d time.Duration)                       { f.t = f.t.Add(d) }
func (f *fakeClock) After(d time.Duration) <-chan time.Time      { return time.After(d) }
func (f *fakeClock) AfterFunc(time.Duration, func()) clock.Timer { return nil }
func (f *fakeClock) NewTimer(time.Duration) clock.Timer          { return nil }
func (f *fakeClock) NewTicker(time.Duration) clock.Ticker        { return nil }

// advance moves the fake clock forward.
func (f *fakeClock) advance(d time.Duration) { f.t = f.t.Add(d) }

func newMachine(t *testing.T, clk *fakeClock) (*Machine, *stateRecorder) {
	t.Helper()
	req := api.SessionRequest{
		Peer:                  netip.MustParseAddr("192.0.2.2"),
		Local:                 netip.MustParseAddr("192.0.2.1"),
		Mode:                  api.SingleHop,
		Interface:             "eth0",
		DesiredMinTxInterval:  300_000,
		RequiredMinRxInterval: 300_000,
		DetectMult:            3,
	}
	rec := &stateRecorder{}
	m := &Machine{}
	m.Init(req, 0xCAFEBABE, clk, rec.notify)
	return m, rec
}

// stateRecorder captures notify callbacks for assertion.
type stateRecorder struct {
	transitions []transition
}

type transition struct {
	State packet.State
	Diag  packet.Diag
}

func (r *stateRecorder) notify(s packet.State, d packet.Diag) {
	r.transitions = append(r.transitions, transition{s, d})
}

// recv builds a synthetic peer Control packet. Tests pretend the peer's
// MyDiscriminator is always 1; yourDisc is whatever the local session is
// expected to recognize (0 during the first packet, the local discriminator
// thereafter).
func recv(state packet.State, yourDisc uint32) packet.Control {
	return packet.Control{
		Version:                   packet.Version,
		State:                     state,
		DetectMult:                3,
		Length:                    packet.MandatoryLen,
		MyDiscriminator:           1,
		YourDiscriminator:         yourDisc,
		DesiredMinTxInterval:      300_000,
		RequiredMinRxInterval:     300_000,
		RequiredMinEchoRxInterval: 0,
	}
}

// VALIDATES: Init produces a session in Down state with slow-start TX
// interval and the configured RX interval.
// PREVENTS: regression where Init forgets the slow-start floor or copies
// configured TX into the active TX field too early.
func TestInitSlowStart(t *testing.T) {
	m, _ := newMachine(t, newFakeClock())
	if m.State() != packet.StateDown {
		t.Fatalf("state: got %v want Down", m.State())
	}
	if m.vars.DesiredMinTxInterval != SlowStartIntervalUs {
		t.Fatalf("DesiredMinTxInterval: got %d want %d", m.vars.DesiredMinTxInterval, SlowStartIntervalUs)
	}
	if m.vars.ConfiguredDesiredMinTxInterval != 300_000 {
		t.Fatalf("ConfiguredDesiredMinTxInterval: got %d want 300_000", m.vars.ConfiguredDesiredMinTxInterval)
	}
	if m.vars.LocalDiscr == 0 {
		t.Fatalf("LocalDiscr is zero")
	}
}

// VALIDATES: every cell of the RFC 5880 Section 6.8.6 transition table
// produces the documented next state.
// PREVENTS: regression where a cell falls through silently.
func TestTransitionTable(t *testing.T) {
	type cell struct {
		from packet.State
		recv packet.State
		want packet.State
	}
	cells := []cell{
		// Local Down
		{packet.StateDown, packet.StateAdminDown, packet.StateDown},
		{packet.StateDown, packet.StateDown, packet.StateInit},
		{packet.StateDown, packet.StateInit, packet.StateUp},
		{packet.StateDown, packet.StateUp, packet.StateDown},
		// Local Init
		{packet.StateInit, packet.StateAdminDown, packet.StateDown},
		{packet.StateInit, packet.StateDown, packet.StateInit},
		{packet.StateInit, packet.StateInit, packet.StateUp},
		{packet.StateInit, packet.StateUp, packet.StateUp},
		// Local Up
		{packet.StateUp, packet.StateAdminDown, packet.StateDown},
		{packet.StateUp, packet.StateDown, packet.StateDown},
		{packet.StateUp, packet.StateInit, packet.StateUp},
		{packet.StateUp, packet.StateUp, packet.StateUp},
	}
	for _, c := range cells {
		t.Run(c.from.String()+"_recv_"+c.recv.String(), func(t *testing.T) {
			clk := newFakeClock()
			m, _ := newMachine(t, clk)
			// Force the local FSM into the desired starting state.
			m.vars.SessionState = c.from
			pkt := recv(c.recv, m.vars.LocalDiscr)
			if c.from == packet.StateDown {
				// Down accepts YourDisc=0 to avoid the reset rule.
				pkt.YourDiscriminator = 0
			}
			if err := m.Receive(pkt); err != nil {
				t.Fatalf("Receive: %v", err)
			}
			if m.State() != c.want {
				t.Fatalf("from %v recv %v: got %v want %v",
					c.from, c.recv, m.State(), c.want)
			}
		})
	}
}

// VALIDATES: a packet with YourDiscriminator=0 is rejected when the local
// session is not in Down/AdminDown.
// PREVENTS: the trivial reset attack described in RFC 5880 Section 6.8.6.
func TestReceiveZeroYourDiscRejected(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	// Force into Init.
	m.vars.SessionState = packet.StateInit
	pkt := recv(packet.StateInit, 0)
	err := m.Receive(pkt)
	if !errors.Is(err, ErrYourDiscriminatorReset) {
		t.Fatalf("got err %v want ErrYourDiscriminatorReset", err)
	}
	if m.State() != packet.StateInit {
		t.Fatalf("state mutated to %v after rejection", m.State())
	}
}

// VALIDATES: a session reaching Up applies configured (faster) intervals
// and starts a Poll Sequence.
// PREVENTS: regression where the slow-start interval persists after Up.
func TestSlowStartToFastViaPoll(t *testing.T) {
	clk := newFakeClock()
	m, rec := newMachine(t, clk)
	// Drive Down -> Init via received Down.
	if err := m.Receive(recv(packet.StateDown, 0)); err != nil {
		t.Fatal(err)
	}
	if m.State() != packet.StateInit {
		t.Fatalf("expected Init, got %v", m.State())
	}
	// Drive Init -> Up via received Init.
	if err := m.Receive(recv(packet.StateInit, m.vars.LocalDiscr)); err != nil {
		t.Fatal(err)
	}
	if m.State() != packet.StateUp {
		t.Fatalf("expected Up, got %v", m.State())
	}
	if m.vars.DesiredMinTxInterval != 300_000 {
		t.Fatalf("DesiredMinTxInterval not switched: got %d", m.vars.DesiredMinTxInterval)
	}
	if !m.vars.PollOutstanding {
		t.Fatalf("Poll Sequence not initiated on transition to Up")
	}
	if len(rec.transitions) != 2 {
		t.Fatalf("expected 2 transitions (Init, Up), got %d: %+v", len(rec.transitions), rec.transitions)
	}
}

// VALIDATES: a Final-bit reply to an outstanding Poll clears the Poll
// flag.
// PREVENTS: regression where the Poll Sequence never terminates.
func TestPollFinalSequence(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	m.vars.SessionState = packet.StateUp
	m.vars.PollOutstanding = true
	pkt := recv(packet.StateUp, m.vars.LocalDiscr)
	pkt.Final = true
	if err := m.Receive(pkt); err != nil {
		t.Fatal(err)
	}
	if m.vars.PollOutstanding {
		t.Fatalf("Poll still outstanding after F=1")
	}
}

// VALIDATES: detection-time expiry while Up transitions to Down with
// diagnostic 1.
// PREVENTS: regression where CheckDetection ignores the timer or sets
// the wrong diagnostic.
func TestDetectionTimerExpiry(t *testing.T) {
	clk := newFakeClock()
	m, rec := newMachine(t, clk)
	m.vars.SessionState = packet.StateUp
	m.vars.RemoteDetectMult = 3
	m.vars.RemoteMinRxInterval = 300_000
	m.vars.RequiredMinRxInterval = 300_000
	m.armDetectionLocked(clk.Now())

	// Not yet due.
	if m.CheckDetection(clk.Now()) {
		t.Fatalf("detection fired before deadline")
	}
	// Advance past the deadline.
	clk.advance(2 * time.Second)
	if !m.CheckDetection(clk.Now()) {
		t.Fatalf("detection did not fire after deadline")
	}
	if m.State() != packet.StateDown {
		t.Fatalf("state after timeout: got %v want Down", m.State())
	}
	if m.vars.LocalDiag != packet.DiagControlDetectExpired {
		t.Fatalf("diag after timeout: got %v want DiagControlDetectExpired", m.vars.LocalDiag)
	}
	if len(rec.transitions) != 1 || rec.transitions[0].Diag != packet.DiagControlDetectExpired {
		t.Fatalf("notify history wrong: %+v", rec.transitions)
	}
}

// VALIDATES: AdminDown enters AdminDown with the supplied diagnostic and
// AdminEnable returns to Down with cleared diagnostic.
// PREVENTS: regression where the admin path skips notification or fails
// to clear the diagnostic on re-enable.
func TestAdminDownEnable(t *testing.T) {
	clk := newFakeClock()
	m, rec := newMachine(t, clk)
	m.vars.SessionState = packet.StateUp
	m.AdminDown(packet.DiagAdminDown)
	if m.State() != packet.StateAdminDown {
		t.Fatalf("after AdminDown: got %v", m.State())
	}
	if m.vars.LocalDiag != packet.DiagAdminDown {
		t.Fatalf("diag after AdminDown: got %v", m.vars.LocalDiag)
	}
	m.AdminEnable()
	if m.State() != packet.StateDown {
		t.Fatalf("after AdminEnable: got %v", m.State())
	}
	if m.vars.LocalDiag != packet.DiagNone {
		t.Fatalf("diag after AdminEnable: got %v", m.vars.LocalDiag)
	}
	if len(rec.transitions) != 2 {
		t.Fatalf("expected 2 transitions, got %d", len(rec.transitions))
	}
}

// VALIDATES: refcount Acquire/Release behavior.
// PREVENTS: regression where Release decrements below zero or Acquire
// fails to compose.
func TestRefcount(t *testing.T) {
	m, _ := newMachine(t, newFakeClock())
	if m.Refcount() != 1 {
		t.Fatalf("initial refcount: got %d want 1", m.Refcount())
	}
	if got := m.Acquire(); got != 2 {
		t.Fatalf("Acquire: got %d want 2", got)
	}
	if got := m.Release(); got != 1 {
		t.Fatalf("Release: got %d want 1", got)
	}
	if got := m.Release(); got != 0 {
		t.Fatalf("Release: got %d want 0", got)
	}
	if got := m.Release(); got != 0 {
		t.Fatalf("Release below zero: got %d want 0", got)
	}
}

// VALIDATES: Build produces a packet whose fields mirror the current
// session state.
// PREVENTS: regression where Build forgets a field or sets the wrong flag.
func TestBuildOutgoingPacket(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	m.vars.SessionState = packet.StateUp
	m.vars.RemoteSessionState = packet.StateUp
	m.vars.RemoteDiscr = 0xDEADBEEF
	m.vars.PollOutstanding = true

	c := m.Build()
	if c.Version != packet.Version {
		t.Fatalf("Version: got %d want %d", c.Version, packet.Version)
	}
	if c.State != packet.StateUp {
		t.Fatalf("State: got %v want Up", c.State)
	}
	if c.MyDiscriminator != m.vars.LocalDiscr {
		t.Fatalf("MyDiscriminator: got %#x want %#x", c.MyDiscriminator, m.vars.LocalDiscr)
	}
	if c.YourDiscriminator != 0xDEADBEEF {
		t.Fatalf("YourDiscriminator: got %#x", c.YourDiscriminator)
	}
	if !c.Poll {
		t.Fatalf("Poll bit not set on outgoing packet")
	}
	if c.Final {
		t.Fatalf("Final bit set on Build")
	}
	if c.Length != packet.MandatoryLen {
		t.Fatalf("Length: got %d want %d", c.Length, packet.MandatoryLen)
	}

	final := m.BuildFinal()
	if final.Poll || !final.Final {
		t.Fatalf("BuildFinal flags wrong: poll=%v final=%v", final.Poll, final.Final)
	}
}

// VALIDATES: while the session is not Up, TransmitInterval observes the
// slow-start floor (RFC 5880 Section 6.8.3) regardless of the configured
// faster operating value.
// PREVENTS: regression where Init forgets the floor and the engine ships
// 50ms-rate packets at slow-start time.
func TestTransmitIntervalSlowStart(t *testing.T) {
	m, _ := newMachine(t, newFakeClock())
	tx := m.TransmitInterval()
	if tx < time.Second {
		t.Fatalf("TransmitInterval below slow-start floor: %v", tx)
	}
}

// VALIDATES: DetectionInterval reflects the negotiated RX floor and the
// remote's detect multiplier.
// PREVENTS: regression where DetectionInterval returns zero or uses the
// local multiplier instead of the remote one.
func TestDetectionIntervalNegotiated(t *testing.T) {
	m, _ := newMachine(t, newFakeClock())
	m.vars.RequiredMinRxInterval = 300_000
	m.vars.RemoteMinRxInterval = 1
	m.vars.RemoteDetectMult = 5
	det := m.DetectionInterval()
	want := 5 * 300 * time.Millisecond
	if det != want {
		t.Fatalf("DetectionInterval: got %v want %v", det, want)
	}
}
