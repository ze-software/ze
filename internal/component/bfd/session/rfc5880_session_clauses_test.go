// VALIDATES: RFC 5880 Section 6 clauses of the session Machine that its
// older tests left without a case: the bfd.RemoteDiscr clear when only
// invalid or unauthenticated packets arrive, the one-second floor in every
// state other than Up, a Poll for each interval changed alone, the Detection
// Time following a remote parameter change at once, detection expiry from Up,
// the larger of the two transmit intervals, echo ceasing on the LAST
// advertisement, and a Poll carried on the scheduled packet.
// PREVENTS: a Receive that re-arms detection before its discard checks, a
// floor set only on AdminDown, a Poll guard that watches one interval, a
// detection deadline computed from stale parameters, an expiry scoped to
// Init, a transmit interval taken from the remote value alone, an echo latch,
// and a Poll that adds a packet to the periodic schedule.
//
// Method: each test drives the Machine through Receive, CheckDetection,
// ApplyEchoSlowdown, AdvanceTxWithJitter and PrimeEcho on a fake clock, and
// reads the result through the accessors the engine uses.
package session

import (
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/packet"
)

// rfc5880ClauseMachine initializes a Machine with the given configured
// Desired Min TX, in microseconds, Required Min RX 300 ms and Detect Mult 3.
func rfc5880ClauseMachine(clk *fakeClock, txUs uint32) *Machine {
	req := api.SessionRequest{
		Peer:                  netip.MustParseAddr("192.0.2.2"),
		Local:                 netip.MustParseAddr("192.0.2.1"),
		Mode:                  api.SingleHop,
		Interface:             "eth0",
		DesiredMinTxInterval:  txUs,
		RequiredMinRxInterval: 300_000,
		DetectMult:            3,
	}
	m := &Machine{}
	m.Init(req, 0x5880C1, clk, nil)
	return m
}

// rfc5880BringUp moves m from Down to Up with one peer Init packet and ends
// the Poll the Up transition starts.
func rfc5880BringUp(t *testing.T, m *Machine) {
	t.Helper()
	if err := m.Receive(recv(packet.StateInit, peerLearnedDiscr)); err != nil {
		t.Fatalf("Receive Init: %v", err)
	}
	if m.State() != packet.StateUp {
		t.Fatalf("precondition: expected Up, got %v", m.State())
	}
	settlePoll(t, m)
}

// RFC requirement: RFC5880-6.8.1-5 positive -- on an authenticated Up
// session, a packet without the A bit (unauthenticated, Receive returns
// ErrAuthMismatch) and a packet with Your Discriminator zero and State Up
// (invalid, ErrYourDiscriminatorReset), both delivered inside the Detection
// Time, do not stop CheckDetection at the end of that Detection Time from
// setting bfd.RemoteDiscr to zero.
func TestRFC5880RemoteDiscrClearedDespiteInvalidPackets(t *testing.T) {
	clk := newFakeClock()
	m := rfc5880ClauseMachine(clk, 300_000)
	m.SetAuth(rfc5880AuthPair(t))

	up := recv(packet.StateInit, peerLearnedDiscr)
	up.Auth = true
	if err := m.Receive(up); err != nil {
		t.Fatalf("Receive authenticated Init: %v", err)
	}
	if m.State() != packet.StateUp || m.RemoteDiscriminator() == 0 {
		t.Fatalf("precondition: expected Up with a learned discriminator, got %v and %d",
			m.State(), m.RemoteDiscriminator())
	}
	detect := m.DetectionInterval()

	clk.advance(detect / 2)
	unauthenticated := recv(packet.StateUp, peerLearnedDiscr)
	unauthenticated.MyDiscriminator = 77
	if err := m.Receive(unauthenticated); !errors.Is(err, ErrAuthMismatch) {
		t.Fatalf("unauthenticated packet: got %v, want ErrAuthMismatch", err)
	}
	invalid := recv(packet.StateUp, 0)
	invalid.Auth = true
	invalid.MyDiscriminator = 77
	if err := m.Receive(invalid); !errors.Is(err, ErrYourDiscriminatorReset) {
		t.Fatalf("invalid packet: got %v, want ErrYourDiscriminatorReset", err)
	}

	clk.advance(detect - detect/2)
	if !m.CheckDetection(clk.Now()) {
		t.Fatal("a Detection Time of invalid and unauthenticated packets did not expire detection")
	}
	if got := m.RemoteDiscriminator(); got != 0 {
		t.Fatalf("bfd.RemoteDiscr = %d after a Detection Time without a valid authenticated packet, want 0", got)
	}
}

// RFC requirement: RFC5880-6.8.3-1 positive -- with 300 ms configured,
// bfd.DesiredMinTxInterval and the Desired Min TX field Build writes are at
// least 1,000,000 microseconds in Init, after Up goes Down on detection
// expiry, after Up goes Down on a peer Down, and after Up goes Down on a peer
// AdminDown; in Up between them they are 300 ms, so the floor is re-applied
// on each exit from Up.
func TestRFC5880SlowStartFloorInEveryStateButUp(t *testing.T) {
	requireFloor := func(t *testing.T, m *Machine, when string) {
		t.Helper()
		if m.State() == packet.StateUp {
			t.Fatalf("%s: precondition: session is still Up", when)
		}
		if got := m.DesiredMinTxIntervalUs(); got < SlowStartIntervalUs {
			t.Fatalf("%s: bfd.DesiredMinTxInterval = %d, want at least 1000000", when, got)
		}
		if got := m.Build().DesiredMinTxInterval; got < SlowStartIntervalUs {
			t.Fatalf("%s: transmitted Desired Min TX = %d, want at least 1000000", when, got)
		}
	}
	requireUp := func(t *testing.T, m *Machine) {
		t.Helper()
		rfc5880BringUp(t, m)
		if got := m.DesiredMinTxIntervalUs(); got != 300_000 {
			t.Fatalf("precondition: Up with bfd.DesiredMinTxInterval %d, want 300000", got)
		}
	}

	clk := newFakeClock()
	m := rfc5880ClauseMachine(clk, 300_000)
	if err := m.Receive(recv(packet.StateDown, 0)); err != nil {
		t.Fatalf("Receive Down: %v", err)
	}
	if m.State() != packet.StateInit {
		t.Fatalf("precondition: expected Init, got %v", m.State())
	}
	requireFloor(t, m, "Init")

	m = rfc5880ClauseMachine(clk, 300_000)
	requireUp(t, m)
	clk.advance(m.DetectionInterval())
	if !m.CheckDetection(clk.Now()) {
		t.Fatal("precondition: detection did not expire")
	}
	requireFloor(t, m, "Down on detection expiry")

	for _, peerState := range []packet.State{packet.StateDown, packet.StateAdminDown} {
		m = rfc5880ClauseMachine(clk, 300_000)
		requireUp(t, m)
		if err := m.Receive(recv(peerState, peerLearnedDiscr)); err != nil {
			t.Fatalf("Receive %v: %v", peerState, err)
		}
		requireFloor(t, m, "Down on peer "+peerState.String())
	}
}

// RFC requirement: RFC5880-6.8.3-2 positive -- a Poll Sequence is initiated
// when bfd.DesiredMinTxInterval changes while bfd.RequiredMinRxInterval stays
// the same (1 s to 300 ms on the move to Up, RX 300 ms throughout), and when
// bfd.RequiredMinRxInterval changes while bfd.DesiredMinTxInterval stays the
// same (the echo slow-down moves RX 300 ms to 1 s with TX configured at 2 s);
// the packet Build writes carries P in both cases.
func TestRFC5880PollInitiatedForEachIntervalAlone(t *testing.T) {
	clk := newFakeClock()
	m := rfc5880ClauseMachine(clk, 300_000)
	if m.vars.RequiredMinRxInterval != 300_000 {
		t.Fatalf("precondition: bfd.RequiredMinRxInterval %d before Up, want 300000", m.vars.RequiredMinRxInterval)
	}
	if err := m.Receive(recv(packet.StateInit, peerLearnedDiscr)); err != nil {
		t.Fatalf("Receive Init: %v", err)
	}
	if m.DesiredMinTxIntervalUs() != 300_000 || m.vars.RequiredMinRxInterval != 300_000 {
		t.Fatalf("precondition: only TX changes: tx=%d rx=%d", m.DesiredMinTxIntervalUs(), m.vars.RequiredMinRxInterval)
	}
	if !m.PollOutstanding() || !m.Build().Poll {
		t.Fatal("bfd.DesiredMinTxInterval changed alone and no Poll Sequence was initiated")
	}

	m = rfc5880ClauseMachine(clk, 2_000_000)
	rfc5880BringUp(t, m)
	txBefore := m.DesiredMinTxIntervalUs()
	m.ApplyEchoSlowdown()
	if m.DesiredMinTxIntervalUs() != txBefore || m.vars.RequiredMinRxInterval == 300_000 {
		t.Fatalf("precondition: only RX changes: tx %d to %d, rx=%d",
			txBefore, m.DesiredMinTxIntervalUs(), m.vars.RequiredMinRxInterval)
	}
	if !m.PollOutstanding() || !m.Build().Poll {
		t.Fatal("bfd.RequiredMinRxInterval changed alone and no Poll Sequence was initiated")
	}
}

// RFC requirement: RFC5880-6.8.3-8 positive -- on an Up session, a received
// Desired Min TX of 600 ms moves the Detection Time from 900 ms to 1.8 s at
// once (CheckDetection does not fire 1 ms past the old 900 ms and fires at
// 1.8 s), and a received Detect Mult of 5 moves it to 1.5 s at once.
func TestRFC5880RemoteTimingChangeMovesDetectionTimeAtOnce(t *testing.T) {
	clk := newFakeClock()
	m := rfc5880ClauseMachine(clk, 300_000)
	rfc5880BringUp(t, m)
	if got := m.DetectionInterval(); got != 900*time.Millisecond {
		t.Fatalf("precondition: Detection Time %v, want 900ms", got)
	}

	slower := recv(packet.StateUp, peerLearnedDiscr)
	slower.DesiredMinTxInterval = 600_000
	if err := m.Receive(slower); err != nil {
		t.Fatalf("Receive 600 ms: %v", err)
	}
	if got := m.DetectionInterval(); got != 1800*time.Millisecond {
		t.Fatalf("Detection Time %v after the peer's 600 ms, want 1.8s", got)
	}
	start := clk.Now()
	clk.advance(901 * time.Millisecond)
	if m.CheckDetection(clk.Now()) {
		t.Fatal("detection expired on the old 900 ms Detection Time")
	}
	clk.t = start.Add(1800 * time.Millisecond)
	if !m.CheckDetection(clk.Now()) {
		t.Fatal("detection did not expire at the new 1.8 s Detection Time")
	}

	m = rfc5880ClauseMachine(clk, 300_000)
	rfc5880BringUp(t, m)
	mult := recv(packet.StateUp, peerLearnedDiscr)
	mult.DetectMult = 5
	if err := m.Receive(mult); err != nil {
		t.Fatalf("Receive Detect Mult 5: %v", err)
	}
	if got := m.DetectionInterval(); got != 1500*time.Millisecond {
		t.Fatalf("Detection Time %v after the peer's Detect Mult 5, want 1.5s", got)
	}
}

// RFC requirement: RFC5880-6.8.4-1 positive -- with local Demand mode not
// active (Ze never sets bfd.DemandMode), an Up session that receives nothing
// for a Detection Time moves to Down with bfd.LocalDiag 1 and reports that
// transition, both when the last packet had the D bit clear and when it had
// the D bit set; 1 ns earlier nothing changes.
func TestRFC5880DetectionExpiryFromUpDownDiagOne(t *testing.T) {
	for _, remoteDemand := range []bool{false, true} {
		clk := newFakeClock()
		m, rec := newMachine(t, clk)
		rfc5880BringUp(t, m)
		last := recv(packet.StateUp, peerLearnedDiscr)
		last.Demand = remoteDemand
		if err := m.Receive(last); err != nil {
			t.Fatalf("D=%v: Receive: %v", remoteDemand, err)
		}
		if m.vars.DemandMode {
			t.Fatalf("D=%v: precondition: local Demand mode is active", remoteDemand)
		}
		seen := len(rec.transitions)

		clk.advance(m.DetectionInterval() - time.Nanosecond)
		if m.CheckDetection(clk.Now()) || m.State() != packet.StateUp {
			t.Fatalf("D=%v: session left Up before a Detection Time passed", remoteDemand)
		}
		clk.advance(time.Nanosecond)
		if !m.CheckDetection(clk.Now()) {
			t.Fatalf("D=%v: detection did not expire from Up", remoteDemand)
		}
		if m.State() != packet.StateDown || m.LocalDiag() != packet.DiagControlDetectExpired {
			t.Fatalf("D=%v: after expiry from Up: state %v diag %v, want Down and 1", remoteDemand, m.State(), m.LocalDiag())
		}
		if len(rec.transitions) != seen+1 || rec.transitions[seen] != (transition{packet.StateDown, packet.DiagControlDetectExpired}) {
			t.Fatalf("D=%v: reported transitions %v, want one Down with diag 1", remoteDemand, rec.transitions[seen:])
		}
	}
}

// RFC requirement: RFC5880-6.8.7-1 positive -- the next periodic Control
// packet is scheduled one interval of the LARGER of bfd.DesiredMinTxInterval
// and bfd.RemoteMinRxInterval after the last, whichever of the two is larger:
// 700 ms for 700 ms local and 300 ms remote, 700 ms for 300 ms local and
// 700 ms remote, and never less than 75 percent of it with a 25 percent
// jitter reduction.
func TestRFC5880TransmitDeadlineUsesLargerOfBothIntervals(t *testing.T) {
	for _, tc := range []struct{ localUs, remoteUs uint32 }{{700_000, 300_000}, {300_000, 700_000}} {
		clk := newFakeClock()
		m, _ := newMachine(t, clk)
		m.vars.SessionState = packet.StateUp
		m.vars.DesiredMinTxInterval = tc.localUs
		m.vars.RemoteMinRxInterval = tc.remoteUs

		now := clk.Now()
		m.AdvanceTxWithJitter(now, 0)
		if got := m.NextTxDeadline().Sub(now); got != 700*time.Millisecond {
			t.Fatalf("local %d remote %d: next TX in %v, want 700ms", tc.localUs, tc.remoteUs, got)
		}
		m.AdvanceTxWithJitter(now, 175*time.Millisecond)
		if got := m.NextTxDeadline().Sub(now); got != 525*time.Millisecond {
			t.Fatalf("local %d remote %d: next TX in %v with 25%% jitter, want 525ms", tc.localUs, tc.remoteUs, got)
		}
	}
}

// RFC requirement: RFC5880-6.8.9-2 negative -- echo follows the LAST
// received Control packet: after a packet advertising 50 ms enabled and
// scheduled echo, a later packet advertising Required Min Echo RX 0 leaves
// EchoEnabled false and PrimeEcho with no deadline, and a further packet
// advertising 50 ms enables it again.
func TestRFC5880EchoFollowsLastAdvertisement(t *testing.T) {
	clk := newFakeClock()
	m := rfc5880EchoMachine(t, clk)
	for i, echoRxUs := range []uint32{50_000, 0, 50_000} {
		c := recv(packet.StateUp, peerLearnedDiscr)
		if i == 0 {
			c.State = packet.StateInit
		}
		c.RequiredMinEchoRxInterval = echoRxUs
		if err := m.Receive(c); err != nil {
			t.Fatalf("packet %d: Receive: %v", i, err)
		}
		m.PrimeEcho(clk.Now())
		scheduled := !m.NextEchoTxDeadline().IsZero()
		want := echoRxUs != 0
		if m.EchoEnabled() != want || scheduled != want {
			t.Fatalf("packet %d advertising %d: EchoEnabled %v scheduled %v, want %v",
				i, echoRxUs, m.EchoEnabled(), scheduled, want)
		}
		m.ClearEchoSchedule()
	}
}

// RFC requirement: RFC5880-6.5-2 positive -- on an Up session already
// sending periodic packets, a Poll raised by the echo slow-down leaves the
// scheduled periodic deadline where it was, so no additional packet is
// scheduled, and the packet Build writes for that scheduled transmission
// carries P.
func TestRFC5880PollRidesTheScheduledPacket(t *testing.T) {
	clk := newFakeClock()
	m := rfc5880ClauseMachine(clk, 300_000)
	rfc5880BringUp(t, m)
	m.AdvanceTxWithJitter(clk.Now(), 0)
	scheduled := m.NextTxDeadline()
	if !scheduled.After(clk.Now()) {
		t.Fatal("precondition: no periodic deadline ahead")
	}

	clk.advance(100 * time.Millisecond)
	m.ApplyEchoSlowdown()
	if !m.PollOutstanding() {
		t.Fatal("precondition: the slow-down raised no Poll")
	}
	if got := m.NextTxDeadline(); !got.Equal(scheduled) {
		t.Fatalf("Poll moved the next transmission from %v to %v", scheduled, got)
	}
	clk.t = scheduled
	if !m.Build().Poll {
		t.Fatal("the scheduled periodic packet does not carry P")
	}
}

// RFC requirement: RFC5880-6.8.3-2 positive -- a Poll Sequence is initiated
// when leaving Up changes bfd.DesiredMinTxInterval: an Up session at 300 ms
// that receives a peer Down has its bfd.DesiredMinTxInterval set to the 1 s
// floor, and PollOutstanding is true after the transition. The RFC text makes
// no exception for the move to Down.
func TestRFC5880PollInitiatedWhenDownEntryRaisesDesiredMinTx(t *testing.T) {
	clk := newFakeClock()
	m := rfc5880ClauseMachine(clk, 300_000)
	rfc5880BringUp(t, m)
	if err := m.Receive(recv(packet.StateDown, peerLearnedDiscr)); err != nil {
		t.Fatalf("Receive Down: %v", err)
	}
	if m.DesiredMinTxIntervalUs() != SlowStartIntervalUs {
		t.Fatalf("precondition: bfd.DesiredMinTxInterval %d, want the 1 s floor", m.DesiredMinTxIntervalUs())
	}
	if !m.PollOutstanding() {
		t.Fatal("bfd.DesiredMinTxInterval changed from 300 ms to 1 s on entry to Down and no Poll Sequence was initiated")
	}
}

// RFC requirement: RFC5880-6.8.1-4 negative -- the input that would violate the
// rule is a Machine that already holds a nonzero bfd.RemoteDiscr when it is
// initialized: the machine learns the peer's discriminator (1) from a received
// packet, is initialized again for a new session, and bfd.RemoteDiscr is zero
// after that Init. An Init that carried the learned value over would leave 1.
func TestRFC5880InitClearsLearnedRemoteDiscr(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	if err := m.Receive(recv(packet.StateDown, 0)); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if got := m.RemoteDiscriminator(); got != 1 {
		t.Fatalf("precondition: bfd.RemoteDiscr = %d after the first packet, want the peer's 1", got)
	}

	m.Init(m.configReq, 0xBEEF, clk, nil)
	if got := m.RemoteDiscriminator(); got != 0 {
		t.Fatalf("bfd.RemoteDiscr after Init = %d, want 0", got)
	}
}
