// VALIDATES: RFC 5880 base-protocol session behavior -- the Section 6.8.1
// state-variable initialization and clearing rules, the Section 6.1 role
// rules, the Section 6.8.3 slow-start and Poll-Sequence rules, the Section
// 6.5 Poll/Final bit rules, the Section 6.6 Demand-bit guard, the Section
// 6.8.6 reception guards that live above the codec, the Section 6.8.7
// transmit-interval arithmetic, and the Section 6.8.4/6.8.5/6.8.16
// state-transition triggers.
// PREVENTS: a session that starts Up, a slow-start floor that leaks away, a
// Poll that never rides a scheduled packet, a D bit set on a half-open
// session, a zero-discriminator reset of a live session, and echo packets
// leaving faster than the peer asked for.
package session

import (
	"encoding/binary"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/auth"
	"github.com/ze-software/ze/internal/component/bfd/packet"
)

// rfc5880LocalEchoTxUs is the local echo transmit target used by the
// echo-negotiation tests: 50 ms, faster than the peer floors they exercise.
const rfc5880LocalEchoTxUs uint32 = 50_000

// rfc5880EchoMachine builds a session with echo configured locally so the
// echo-negotiation tests can drive RemoteMinEchoRxInterval independently.
func rfc5880EchoMachine(t *testing.T, clk *fakeClock) *Machine {
	t.Helper()
	req := api.SessionRequest{
		Peer:                     netip.MustParseAddr("192.0.2.2"),
		Local:                    netip.MustParseAddr("192.0.2.1"),
		Mode:                     api.SingleHop,
		Interface:                "eth0",
		DesiredMinTxInterval:     300_000,
		RequiredMinRxInterval:    300_000,
		DetectMult:               3,
		DesiredMinEchoTxInterval: rfc5880LocalEchoTxUs,
	}
	m := &Machine{}
	m.Init(req, 0xBEEF01, clk, nil)
	return m
}

// rfc5880AuthPair builds a Keyed SHA1 signer/verifier bundle for the
// authenticated-session tests.
func rfc5880AuthPair(t *testing.T) *AuthPair {
	t.Helper()
	cfg := auth.Settings{
		Type:   packet.AuthTypeKeyedSHA1,
		KeyID:  9,
		Secret: []byte("rfc5880-session-key"),
	}
	signer, err := auth.NewSigner(cfg)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	verifier, err := auth.NewVerifier(cfg)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	return &AuthPair{Signer: signer, Verifier: verifier}
}

// ---------------------------------------------------------------------
// Section 6.8.1 -- state variable initialization
// ---------------------------------------------------------------------

// RFC requirement: RFC5880-6.8.1-1 positive -- bfd.SessionState is
// initialized to Down. Init (internal/component/bfd/session/session.go:246)
// assigns SessionState: packet.StateDown before any packet is exchanged.
// RFC requirement: RFC5880-6.8.1-2 positive -- bfd.RemoteSessionState is
// initialized to Down by the same producer (session.go:247).
func TestRFC5880InitStatesAreDown(t *testing.T) {
	m, _ := newMachine(t, newFakeClock())
	if m.State() != packet.StateDown {
		t.Fatalf("bfd.SessionState after Init = %v, want Down", m.State())
	}
	if m.vars.RemoteSessionState != packet.StateDown {
		t.Fatalf("bfd.RemoteSessionState after Init = %v, want Down", m.vars.RemoteSessionState)
	}
}

// RFC requirement: RFC5880-6.8.1-4 positive -- bfd.RemoteDiscr is initialized
// to zero. Init (session.go:249) assigns RemoteDiscr: 0.
// RFC requirement: RFC5880-6.8.1-6 positive -- bfd.LocalDiag is initialized to
// zero: Init (session.go:250) assigns LocalDiag: packet.DiagNone.
// RFC requirement: RFC5880-6.8.1-8 positive -- bfd.RemoteMinRxInterval is
// initialized to 1: Init (session.go:253) assigns RemoteMinRxInterval: 1.
// RFC requirement: RFC5880-6.8.1-9 positive -- bfd.RemoteDemandMode is
// initialized to zero: the Vars literal (session.go:245-259) omits
// RemoteDemandMode, leaving the false zero value.
func TestRFC5880InitVariableDefaults(t *testing.T) {
	m, _ := newMachine(t, newFakeClock())
	if m.RemoteDiscriminator() != 0 {
		t.Fatalf("bfd.RemoteDiscr after Init = %d, want 0", m.RemoteDiscriminator())
	}
	if m.LocalDiag() != packet.DiagNone {
		t.Fatalf("bfd.LocalDiag after Init = %v, want no-diagnostic", m.LocalDiag())
	}
	if m.vars.RemoteMinRxInterval != 1 {
		t.Fatalf("bfd.RemoteMinRxInterval after Init = %d, want 1", m.vars.RemoteMinRxInterval)
	}
	if m.vars.RemoteDemandMode {
		t.Fatalf("bfd.RemoteDemandMode after Init = true, want false")
	}
}

// The test also checks that bfd.RemoteDiscr's zero is an initial value, not a
// constant: the first received packet installs the peer's My Discriminator via
// Receive (internal/component/bfd/session/fsm.go).
// RFC requirement: RFC5880-6.8.1-6 negative -- LocalDiag likewise moves off
// zero when a transition records a reason (fsm.go:83).
// RFC requirement: RFC5880-6.8.1-8 negative -- RemoteMinRxInterval is replaced
// by the peer's advertised value (fsm.go:59), so the 1 is an initial value and
// not a hardcoded constant.
// RFC requirement: RFC5880-6.8.1-9 negative -- RemoteDemandMode follows the
// received D bit (fsm.go:58) rather than staying false forever.
func TestRFC5880InitVariablesAreNotConstants(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)

	first := recv(packet.StateDown, 0)
	first.Demand = true
	first.RequiredMinRxInterval = 700_000
	if err := m.Receive(first); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if m.RemoteDiscriminator() != 1 {
		t.Fatalf("bfd.RemoteDiscr = %d after first packet, want the peer's 1", m.RemoteDiscriminator())
	}
	if !m.vars.RemoteDemandMode {
		t.Fatalf("bfd.RemoteDemandMode still false after receiving D=1")
	}
	if m.vars.RemoteMinRxInterval != 700_000 {
		t.Fatalf("bfd.RemoteMinRxInterval = %d, want the advertised 700000", m.vars.RemoteMinRxInterval)
	}

	// Drive Up then have the peer signal Down so LocalDiag records a reason.
	m.vars.SessionState = packet.StateUp
	if err := m.Receive(recv(packet.StateDown, m.vars.LocalDiscr)); err != nil {
		t.Fatalf("Receive Down: %v", err)
	}
	if m.LocalDiag() != packet.DiagNeighborSignaledDown {
		t.Fatalf("bfd.LocalDiag = %v, want neighbor-signaled-session-down", m.LocalDiag())
	}
}

// RFC requirement: RFC5880-6.8.1-5 positive -- bfd.RemoteDiscr is set to zero
// when no valid packet has been received for one Detection Time. CheckDetection
// (internal/component/bfd/session/timers.go:76-83) records
// DiagControlDetectExpired and onStateChange (fsm.go:159-162) clears
// RemoteDiscr for that diagnostic.
func TestRFC5880RemoteDiscrClearedOnDetectionExpiry(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	if err := m.Receive(recv(packet.StateInit, peerLearnedDiscr)); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if m.State() != packet.StateUp {
		t.Fatalf("precondition: expected Up, got %v", m.State())
	}
	if m.RemoteDiscriminator() == 0 {
		t.Fatal("precondition: RemoteDiscr must be learned before the timeout")
	}

	clk.advance(10 * time.Second)
	if !m.CheckDetection(clk.Now()) {
		t.Fatal("detection timer did not fire after one Detection Time of silence")
	}
	if got := m.RemoteDiscriminator(); got != 0 {
		t.Fatalf("bfd.RemoteDiscr = %d after detection expiry, want 0", got)
	}
}

// RFC requirement: RFC5880-6.8.1-5 negative -- the clear is bound to the
// detection-time event, not to every entry into Down. A peer-signaled Down
// leaves the peer reachable, and onStateChange (fsm.go:159-162) only clears
// RemoteDiscr for DiagControlDetectExpired or DiagEchoFailed, so a Down
// carrying DiagNeighborSignaledDown keeps the learned discriminator. Without
// this the positive could pass on code that cleared unconditionally.
func TestRFC5880RemoteDiscrKeptOnNeighborSignaledDown(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	if err := m.Receive(recv(packet.StateInit, peerLearnedDiscr)); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	learned := m.RemoteDiscriminator()
	if learned == 0 {
		t.Fatal("precondition: RemoteDiscr must be learned")
	}

	if err := m.Receive(recv(packet.StateDown, m.vars.LocalDiscr)); err != nil {
		t.Fatalf("Receive Down: %v", err)
	}
	if m.State() != packet.StateDown {
		t.Fatalf("state = %v, want Down", m.State())
	}
	if got := m.RemoteDiscriminator(); got != learned {
		t.Fatalf("bfd.RemoteDiscr = %d after a peer-signaled Down, want the learned %d", got, learned)
	}
}

// RFC requirement: RFC5880-6.8.1-7 positive -- bfd.DesiredMinTxInterval is
// initialized to at least 1 000 000 microseconds. Init
// (internal/component/bfd/session/session.go:251) assigns
// SlowStartIntervalUs (session.go:27 = 1 000 000) regardless of the faster
// configured value, which is parked in ConfiguredDesiredMinTxInterval.
// RFC requirement: RFC5880-6.8.3-1 positive -- the same producer keeps the
// floor for as long as the session is not Up.
func TestRFC5880SlowStartFloorWhileNotUp(t *testing.T) {
	m, _ := newMachine(t, newFakeClock())
	if m.DesiredMinTxIntervalUs() < 1_000_000 {
		t.Fatalf("bfd.DesiredMinTxInterval = %d us at Init, want >= 1000000", m.DesiredMinTxIntervalUs())
	}
	if m.vars.ConfiguredDesiredMinTxInterval != 300_000 {
		t.Fatalf("configured operating value lost: %d", m.vars.ConfiguredDesiredMinTxInterval)
	}
	if got := m.TransmitInterval(); got < time.Second {
		t.Fatalf("TransmitInterval while Down = %v, want >= 1s", got)
	}
}

// RFC requirement: RFC5880-6.8.1-7 negative -- the floor is scoped to the
// not-Up states rather than pinned forever: onStateChange
// (internal/component/bfd/session/fsm.go:139-144) swaps in the configured
// 300 ms value once the session reaches Up.
// RFC requirement: RFC5880-6.8.3-1 negative -- the same producer proves the
// floor is state-conditional, so a blanket 1 s implementation would fail here.
func TestRFC5880SlowStartFloorLiftedWhenUp(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	if err := m.Receive(recv(packet.StateInit, peerLearnedDiscr)); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if m.State() != packet.StateUp {
		t.Fatalf("precondition: expected Up, got %v", m.State())
	}
	if got := m.DesiredMinTxIntervalUs(); got != 300_000 {
		t.Fatalf("bfd.DesiredMinTxInterval after Up = %d, want the configured 300000", got)
	}
}

// RFC requirement: RFC5880-6.8.1-10 positive -- bfd.DetectMult is a nonzero
// integer. Init (session.go:232-235) takes the requested multiplier and
// substitutes DefaultDetectMult (session.go:31 = 3) when the request carries
// zero, so the variable is never zero.
func TestRFC5880DetectMultConfiguredValue(t *testing.T) {
	m, _ := newMachine(t, newFakeClock())
	if m.DetectMult() != 3 {
		t.Fatalf("bfd.DetectMult = %d, want the configured 3", m.DetectMult())
	}
}

// RFC requirement: RFC5880-6.8.1-10 negative -- a request carrying a zero
// multiplier does not produce a zero bfd.DetectMult: the fallback at
// session.go:233-235 substitutes DefaultDetectMult. A zero would make the
// detection time zero and tear every session down instantly.
func TestRFC5880DetectMultZeroRequestSubstituted(t *testing.T) {
	req := api.SessionRequest{
		Peer:                  netip.MustParseAddr("192.0.2.2"),
		Local:                 netip.MustParseAddr("192.0.2.1"),
		Mode:                  api.SingleHop,
		Interface:             "eth0",
		DesiredMinTxInterval:  300_000,
		RequiredMinRxInterval: 300_000,
		DetectMult:            0,
	}
	m := &Machine{}
	m.Init(req, 0x1234, newFakeClock(), nil)
	if m.DetectMult() == 0 {
		t.Fatal("bfd.DetectMult is zero after a zero-multiplier request")
	}
	if m.DetectMult() != DefaultDetectMult {
		t.Fatalf("bfd.DetectMult = %d, want DefaultDetectMult %d", m.DetectMult(), DefaultDetectMult)
	}
}

// RFC requirement: RFC5880-6.8.1-14 positive -- session state is preserved for
// at least one Detection Time after the last valid packet. CheckDetection
// (internal/component/bfd/session/timers.go:69-71) returns without touching the
// FSM while now is before nextDetectAt, which armDetectionLocked (timers.go:55)
// set to lastRx + DetectionInterval.
func TestRFC5880StatePreservedForOneDetectionTime(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	if err := m.Receive(recv(packet.StateInit, peerLearnedDiscr)); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	detect := m.DetectionInterval()
	if detect <= 0 {
		t.Fatalf("DetectionInterval = %v, want > 0", detect)
	}

	clk.advance(detect - time.Millisecond)
	if m.CheckDetection(clk.Now()) {
		t.Fatal("detection fired before one Detection Time had elapsed")
	}
	if m.State() != packet.StateUp {
		t.Fatalf("state = %v just before the deadline, want Up", m.State())
	}
}

// RFC requirement: RFC5880-6.8.4-1 positive -- once now reaches nextDetectAt,
// CheckDetection (timers.go:69-84) tears the session down: the expiry sets
// bfd.SessionState to Down and bfd.LocalDiag to 1 (timers.go:76-77) when the
// session was Init or Up.
func TestRFC5880DetectionExpiryDownDiagOne(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	if err := m.Receive(recv(packet.StateInit, peerLearnedDiscr)); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	clk.advance(m.DetectionInterval())
	if !m.CheckDetection(clk.Now()) {
		t.Fatal("detection did not fire at the deadline")
	}
	if m.State() != packet.StateDown {
		t.Fatalf("state after expiry = %v, want Down", m.State())
	}
	if m.LocalDiag() != packet.DiagControlDetectExpired {
		t.Fatalf("bfd.LocalDiag after expiry = %v, want control-detection-time-expired (1)", m.LocalDiag())
	}
}

// RFC requirement: RFC5880-6.8.4-1 negative -- the transition is scoped to the
// Init and Up states. CheckDetection (timers.go:72-74) returns false for a
// session already Down or AdminDown, so an expired deadline does not overwrite
// an existing diagnostic with code 1.
func TestRFC5880DetectionExpiryIgnoredWhenDown(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	if err := m.Receive(recv(packet.StateInit, peerLearnedDiscr)); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	// Force back to Down with a different diagnostic, keeping the armed
	// detection deadline in place.
	m.vars.SessionState = packet.StateDown
	m.vars.LocalDiag = packet.DiagNeighborSignaledDown

	clk.advance(10 * time.Second)
	if m.CheckDetection(clk.Now()) {
		t.Fatal("detection fired while the session was already Down")
	}
	if m.LocalDiag() != packet.DiagNeighborSignaledDown {
		t.Fatalf("bfd.LocalDiag overwritten to %v while Down", m.LocalDiag())
	}
}

// ---------------------------------------------------------------------
// Section 6.1 -- Active and Passive roles
// ---------------------------------------------------------------------

// RFC requirement: RFC5880-6.1-1 positive -- a system taking the Active role
// sends BFD Control packets regardless of whether any have been received. Init
// (internal/component/bfd/session/session.go:262-264) arms nextTxAt to
// createdAt for RoleActive, so the periodic-TX deadline is due immediately and
// the engine tick transmits without any prior reception.
// RFC requirement: RFC5880-6.1-3 positive -- a default session takes the
// Active role: Init (session.go:224) sets RoleActive and only flips to
// RolePassive when the request opts in (session.go:225-227).
// RFC requirement: RFC5880-6.8.7-5 negative -- the "do not transmit" rule is
// scoped to the Passive role: an Active session with bfd.RemoteDiscr still
// zero DOES arm its TX timer, so the suppression is role-conditional rather
// than blanket.
func TestRFC5880ActiveRoleTransmitsWithoutReception(t *testing.T) {
	m, _ := newMachine(t, newFakeClock())
	if m.role != RoleActive {
		t.Fatalf("default role = %v, want RoleActive", m.role)
	}
	if m.RemoteDiscriminator() != 0 {
		t.Fatalf("precondition: RemoteDiscr must still be zero, got %d", m.RemoteDiscriminator())
	}
	if m.NextTxDeadline().IsZero() {
		t.Fatal("Active session did not arm its periodic TX timer before receiving anything")
	}
}

// RFC requirement: RFC5880-6.1-2 positive -- a system taking the Passive role
// does not begin sending until it has received a Control packet. Init
// (session.go:262-264) skips arming nextTxAt for RolePassive, leaving
// NextTxDeadline zero, and the engine tick (engine/loop.go:192-195) skips a
// session whose deadline is zero.
// RFC requirement: RFC5880-6.8.7-5 positive -- the same producer implements
// "MUST NOT transmit while bfd.RemoteDiscr is zero and the system is Passive":
// the Passive session has RemoteDiscr zero and no TX deadline.
// RFC requirement: RFC5880-6.1-1 negative -- sending without reception is the
// Active role's obligation, not every session's: an explicit Passive request
// produces RolePassive, and Init leaves its periodic-TX deadline unarmed while
// RemoteDiscr is still zero. Without this contrast the Active positive could
// pass on code that armed the timer for every role.
func TestRFC5880PassiveRoleSilentUntilReception(t *testing.T) {
	m := newPassiveMachine(t, newFakeClock())
	if m.role != RolePassive {
		t.Fatalf("role = %v, want RolePassive", m.role)
	}
	if m.RemoteDiscriminator() != 0 {
		t.Fatalf("precondition: RemoteDiscr must be zero, got %d", m.RemoteDiscriminator())
	}
	if !m.NextTxDeadline().IsZero() {
		t.Fatalf("Passive session armed TX before receiving anything: %v", m.NextTxDeadline())
	}
}

// RFC requirement: RFC5880-6.1-1 negative -- transmission is armed by the
// role, not unconditionally: a Passive session stays silent at Init and only
// starts once a packet arrives (onStateChange, fsm.go:172, sets nextTxAt).
// Without this contrast the Active positive could pass on code that always
// armed the timer.
// RFC requirement: RFC5880-6.1-2 negative -- the silence is lifted by the
// first received packet rather than permanent, which is what makes the Passive
// end usable at all.
func TestRFC5880PassiveRoleTransmitsAfterReception(t *testing.T) {
	clk := newFakeClock()
	m := newPassiveMachine(t, clk)
	if !m.NextTxDeadline().IsZero() {
		t.Fatalf("precondition: Passive session must be silent, got %v", m.NextTxDeadline())
	}
	if err := m.Receive(recv(packet.StateDown, 0)); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if m.NextTxDeadline().IsZero() {
		t.Fatal("Passive session still silent after receiving a Control packet")
	}
}

// ---------------------------------------------------------------------
// Section 6.8.3 -- Poll Sequence on parameter change
// ---------------------------------------------------------------------

// RFC requirement: RFC5880-6.8.3-2 positive -- a change to
// bfd.DesiredMinTxInterval or bfd.RequiredMinRxInterval initiates a Poll
// Sequence. onStateChange (internal/component/bfd/session/fsm.go:139-144)
// swaps the slow-start values for the configured ones and sets
// PollOutstanding in the same step.
// RFC requirement: RFC5880-6.8.3-6 positive -- both parameter changes are
// carried in a SINGLE packet: one PollOutstanding covers the simultaneous
// DesiredMinTxInterval and RequiredMinRxInterval change (fsm.go:141-143), so
// no second overlapping Poll Sequence is started.
func TestRFC5880PollInitiatedOnIntervalChange(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	if m.PollOutstanding() {
		t.Fatal("precondition: no Poll should be outstanding at Init")
	}
	if err := m.Receive(recv(packet.StateInit, peerLearnedDiscr)); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if m.State() != packet.StateUp {
		t.Fatalf("precondition: expected Up, got %v", m.State())
	}
	if !m.PollOutstanding() {
		t.Fatal("no Poll Sequence initiated after the interval change")
	}
	if m.DesiredMinTxIntervalUs() != 300_000 || m.vars.RequiredMinRxInterval != 300_000 {
		t.Fatalf("both intervals must change together: tx=%d rx=%d",
			m.DesiredMinTxIntervalUs(), m.vars.RequiredMinRxInterval)
	}
	c := m.Build()
	if !c.Poll {
		t.Fatal("the outgoing packet carrying both new intervals does not set P")
	}
}

// RFC requirement: RFC5880-6.8.3-2 negative -- the Poll is tied to an actual
// change: a session whose configured intervals already equal the slow-start
// values reaches Up without setting PollOutstanding, because the guard at
// fsm.go:139-140 compares the live values against the configured ones.
func TestRFC5880NoPollWhenIntervalsUnchanged(t *testing.T) {
	req := api.SessionRequest{
		Peer:                  netip.MustParseAddr("192.0.2.2"),
		Local:                 netip.MustParseAddr("192.0.2.1"),
		Mode:                  api.SingleHop,
		Interface:             "eth0",
		DesiredMinTxInterval:  SlowStartIntervalUs,
		RequiredMinRxInterval: SlowStartIntervalUs,
		DetectMult:            3,
	}
	m := &Machine{}
	m.Init(req, 0x4242, newFakeClock(), nil)
	if err := m.Receive(recv(packet.StateInit, peerLearnedDiscr)); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if m.State() != packet.StateUp {
		t.Fatalf("precondition: expected Up, got %v", m.State())
	}
	if m.PollOutstanding() {
		t.Fatal("Poll Sequence initiated even though neither interval changed")
	}
}

// RFC requirement: RFC5880-6.8.3-5 positive -- when the local system reduces
// its transmit interval because bfd.RemoteMinRxInterval was reduced, the new
// interval is honored immediately. TransmitInterval
// (internal/component/bfd/session/timers.go:43-49) recomputes
// max(DesiredMinTxInterval, RemoteMinRxInterval) from the live variables that
// Receive (fsm.go:59) just updated, so the very next call returns the shorter
// interval with no Poll in between.
// RFC requirement: RFC5880-6.8.7-4 positive -- the same producer recalculates
// the transmit interval whenever bfd.RemoteMinRxInterval changes.
// RFC requirement: RFC5880-6.8.3-8 positive -- a timing-parameter change with
// no explicit exception is effected immediately by that live recomputation.
func TestRFC5880RemoteMinRxReductionHonoredImmediately(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	m.vars.SessionState = packet.StateUp
	m.vars.DesiredMinTxInterval = 300_000

	slow := recv(packet.StateUp, m.vars.LocalDiscr)
	slow.RequiredMinRxInterval = 900_000
	if err := m.Receive(slow); err != nil {
		t.Fatalf("Receive slow: %v", err)
	}
	if got := m.TransmitInterval(); got != 900*time.Millisecond {
		t.Fatalf("TransmitInterval with remote 900ms = %v, want 900ms", got)
	}

	fast := recv(packet.StateUp, m.vars.LocalDiscr)
	fast.RequiredMinRxInterval = 400_000
	if err := m.Receive(fast); err != nil {
		t.Fatalf("Receive fast: %v", err)
	}
	if got := m.TransmitInterval(); got != 400*time.Millisecond {
		t.Fatalf("TransmitInterval after the remote reduction = %v, want 400ms immediately", got)
	}
}

// RFC requirement: RFC5880-6.8.3-5 negative -- honoring the remote reduction
// is bounded by the local bfd.DesiredMinTxInterval: TransmitInterval
// (timers.go:44) takes the MAXIMUM of the two, so a remote value below the
// local target does not shorten transmission. Without this the positive could
// pass on code that blindly adopted whatever the peer advertised.
// RFC requirement: RFC5880-6.8.7-4 negative -- the recalculation is driven by
// the two interval variables only: changing bfd.RequiredMinRxInterval (a
// receive-side variable) leaves the transmit interval untouched.
func TestRFC5880TransmitIntervalFlooredByLocalDesired(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	m.vars.SessionState = packet.StateUp
	m.vars.DesiredMinTxInterval = 300_000

	tiny := recv(packet.StateUp, m.vars.LocalDiscr)
	tiny.RequiredMinRxInterval = 1
	if err := m.Receive(tiny); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if got := m.TransmitInterval(); got != 300*time.Millisecond {
		t.Fatalf("TransmitInterval = %v with a 1us remote value, want the local 300ms floor", got)
	}

	before := m.TransmitInterval()
	m.vars.RequiredMinRxInterval = 50_000
	if after := m.TransmitInterval(); after != before {
		t.Fatalf("TransmitInterval changed from %v to %v when only the receive-side variable moved", before, after)
	}
}

// ---------------------------------------------------------------------
// Section 6.5 -- Poll Sequence bit rules
// ---------------------------------------------------------------------

// RFC requirement: RFC5880-6.5-1 positive -- a Control packet never carries
// both P and F. Build (internal/component/bfd/session/fsm.go:217-218) sets
// Poll from PollOutstanding and hardcodes Final: false, so a scheduled packet
// during a Poll Sequence has P=1 and F=0.
// RFC requirement: RFC5880-6.5-2 positive -- the Poll Sequence is performed by
// setting the P bit on the scheduled transmissions themselves: the same Build
// is what the engine tick calls (engine/loop.go:197) on the periodic deadline.
func TestRFC5880PollPacketHasNoFinalBit(t *testing.T) {
	m, _ := newMachine(t, newFakeClock())
	m.vars.PollOutstanding = true
	c := m.Build()
	if !c.Poll {
		t.Fatal("scheduled packet during a Poll Sequence does not set P")
	}
	if c.Final {
		t.Fatal("scheduled Poll packet also set F; P and F must never both be set")
	}
}

// RFC requirement: RFC5880-6.5-1 negative -- the Final reply clears P rather
// than inheriting it. BuildFinal (fsm.go:236-241) copies Build's output and
// then forces Poll=false, Final=true, so even with a Poll outstanding the
// reply is F=1/P=0 and never both.
// RFC requirement: RFC5880-6.5-2 negative -- the P bit tracks the Poll state
// rather than being always-on: with no Poll outstanding, Build emits P=0, so
// the Poll rides only the transmissions that belong to a sequence.
func TestRFC5880FinalReplyClearsPollBit(t *testing.T) {
	m, _ := newMachine(t, newFakeClock())
	m.vars.PollOutstanding = true
	f := m.BuildFinal()
	if !f.Final {
		t.Fatal("BuildFinal did not set F")
	}
	if f.Poll {
		t.Fatal("BuildFinal left P set; P and F must never both be set")
	}

	m.vars.PollOutstanding = false
	if c := m.Build(); c.Poll {
		t.Fatal("Build set P with no Poll Sequence in flight")
	}
}

// RFC requirement: RFC5880-6.8.6-13 positive -- when a Poll Sequence is in
// flight and a packet with F=1 arrives, the Poll Sequence terminates. Receive
// (internal/component/bfd/session/fsm.go:65-67) clears PollOutstanding on
// c.Final.
func TestRFC5880FinalTerminatesPoll(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	m.vars.SessionState = packet.StateUp
	m.vars.PollOutstanding = true

	p := recv(packet.StateUp, m.vars.LocalDiscr)
	p.Final = true
	if err := m.Receive(p); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if m.PollOutstanding() {
		t.Fatal("Poll Sequence still outstanding after receiving F=1")
	}
}

// RFC requirement: RFC5880-6.8.6-13 negative -- the termination is driven by
// the F bit and nothing else: a packet with F=0 leaves the Poll Sequence in
// flight (the guard at fsm.go:65 requires c.Final). Without this the positive
// could pass on code that cleared the Poll on any received packet.
func TestRFC5880NonFinalDoesNotTerminatePoll(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	m.vars.SessionState = packet.StateUp
	m.vars.PollOutstanding = true

	if err := m.Receive(recv(packet.StateUp, m.vars.LocalDiscr)); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if !m.PollOutstanding() {
		t.Fatal("Poll Sequence terminated by a packet with F=0")
	}
}

// ---------------------------------------------------------------------
// Section 6.6 -- Demand bit guard
// ---------------------------------------------------------------------

// RFC requirement: RFC5880-6.6-1 positive -- the D bit is set only when
// bfd.DemandMode is 1 and both bfd.SessionState and bfd.RemoteSessionState are
// Up. canSetDemand (internal/component/bfd/session/fsm.go:246-250) requires
// all three, and Build (fsm.go:221) uses it for the Demand field.
func TestRFC5880DemandBitSetWhenAllConditionsHold(t *testing.T) {
	m, _ := newMachine(t, newFakeClock())
	m.vars.DemandMode = true
	m.vars.SessionState = packet.StateUp
	m.vars.RemoteSessionState = packet.StateUp
	if !m.Build().Demand {
		t.Fatal("D bit not set with DemandMode=1 and both ends Up")
	}
}

// RFC requirement: RFC5880-6.6-1 negative -- each of the three conditions is
// load-bearing: dropping bfd.DemandMode, the local Up, or the remote Up makes
// canSetDemand (fsm.go:247-249) false and the D bit stays clear. Without this
// the positive could pass on code that always set D.
func TestRFC5880DemandBitClearWhenAnyConditionFails(t *testing.T) {
	cases := []struct {
		name        string
		demand      bool
		local       packet.State
		remoteState packet.State
	}{
		{"demand mode off", false, packet.StateUp, packet.StateUp},
		{"local not up", true, packet.StateInit, packet.StateUp},
		{"remote not up", true, packet.StateUp, packet.StateDown},
		{"neither up", true, packet.StateDown, packet.StateDown},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			m, _ := newMachine(t, newFakeClock())
			m.vars.DemandMode = tt.demand
			m.vars.SessionState = tt.local
			m.vars.RemoteSessionState = tt.remoteState
			if m.Build().Demand {
				t.Fatalf("D bit set with demand=%v local=%v remote=%v", tt.demand, tt.local, tt.remoteState)
			}
		})
	}
}

// ---------------------------------------------------------------------
// Section 6.8.6 -- reception guards above the codec
// ---------------------------------------------------------------------

// RFC requirement: RFC5880-6.8.6-8 negative -- a packet whose Your
// Discriminator is zero MUST be discarded when its own State field is neither
// Down nor AdminDown, whatever the local bfd.SessionState. Receive
// (internal/component/bfd/session/fsm.go) tests the received State and
// returns ErrYourDiscriminatorReset, mutating nothing. The local Down case is
// the one a gate on bfd.SessionState would let through.
func TestRFC5880ZeroYourDiscriminatorDiscardedWhenLive(t *testing.T) {
	for _, local := range []packet.State{packet.StateDown, packet.StateInit, packet.StateUp} {
		for _, sent := range []packet.State{packet.StateInit, packet.StateUp} {
			clk := newFakeClock()
			m, _ := newMachine(t, clk)
			m.vars.SessionState = local
			m.vars.RemoteDiscr = 0x55

			err := m.Receive(recv(sent, 0))
			if !errors.Is(err, ErrYourDiscriminatorReset) {
				t.Fatalf("local %v, packet %v: got err %v, want ErrYourDiscriminatorReset", local, sent, err)
			}
			if m.State() != local {
				t.Fatalf("local %v, packet %v: state mutated to %v by a discarded packet", local, sent, m.State())
			}
			if m.RemoteDiscriminator() != 0x55 {
				t.Fatalf("local %v, packet %v: RemoteDiscr mutated to %d by a discarded packet", local, sent, m.RemoteDiscriminator())
			}
		}
	}
}

// RFC requirement: RFC5880-6.8.6-8 positive -- the discard is scoped to a
// packet whose State field is Up or Init: a zero Your Discriminator in a
// packet whose State is Down or AdminDown is legitimate in every local state.
// It is how a session bootstraps, and how a restarted peer takes a live
// session Down. Receive (fsm.go) lets both through, and the live session
// follows the packet to Down with diagnostic Neighbor Signaled Session Down.
func TestRFC5880ZeroYourDiscriminatorAcceptedWhenDown(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	if err := m.Receive(recv(packet.StateDown, 0)); err != nil {
		t.Fatalf("first packet with Your Discriminator 0 rejected while Down: %v", err)
	}
	if m.State() != packet.StateInit {
		t.Fatalf("state after the bootstrap packet = %v, want Init", m.State())
	}

	clk2 := newFakeClock()
	m2, _ := newMachine(t, clk2)
	m2.vars.SessionState = packet.StateAdminDown
	if err := m2.Receive(recv(packet.StateDown, 0)); err != nil {
		t.Fatalf("packet with Your Discriminator 0 rejected while AdminDown: %v", err)
	}

	for _, sent := range []packet.State{packet.StateDown, packet.StateAdminDown} {
		m3, _ := newMachine(t, newFakeClock())
		m3.vars.SessionState = packet.StateUp
		if err := m3.Receive(recv(sent, 0)); err != nil {
			t.Fatalf("Up session: packet %v with Your Discriminator 0 rejected: %v", sent, err)
		}
		if m3.State() != packet.StateDown {
			t.Fatalf("Up session: state after packet %v = %v, want Down", sent, m3.State())
		}
		if m3.LocalDiag() != packet.DiagNeighborSignaledDown {
			t.Fatalf("Up session: diag after packet %v = %v, want Neighbor Signaled Session Down", sent, m3.LocalDiag())
		}
	}
}

// RFC requirement: RFC5880-6.8.6-9 positive -- an unauthenticated session
// (bfd.AuthType zero) accepts a packet with A=0. Receive
// (internal/component/bfd/session/fsm.go:50-53) compares the received A bit
// against `m.vars.AuthType != 0` and only rejects on a mismatch.
func TestRFC5880UnauthenticatedSessionAcceptsUnauthenticatedPacket(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	if m.vars.AuthType != 0 {
		t.Fatalf("precondition: bfd.AuthType must be 0, got %d", m.vars.AuthType)
	}
	if err := m.Receive(recv(packet.StateDown, 0)); err != nil {
		t.Fatalf("A=0 packet rejected by an unauthenticated session: %v", err)
	}
}

// RFC requirement: RFC5880-6.8.6-9 negative -- a packet with A=1 arriving at a
// session whose bfd.AuthType is zero is discarded with ErrAuthMismatch
// (fsm.go:50-53), so an attacker cannot switch a session into authenticated
// processing by asserting the bit.
func TestRFC5880UnauthenticatedSessionDiscardsAuthenticatedPacket(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	p := recv(packet.StateDown, 0)
	p.Auth = true
	if err := m.Receive(p); !errors.Is(err, ErrAuthMismatch) {
		t.Fatalf("got err %v, want ErrAuthMismatch", err)
	}
	if m.State() != packet.StateDown {
		t.Fatalf("state mutated to %v by a discarded A=1 packet", m.State())
	}
}

// RFC requirement: RFC5880-6.8.6-10 positive -- an authenticated session
// (bfd.AuthType nonzero, installed by SetAuth,
// internal/component/bfd/session/auth.go:31-41) accepts a packet whose A bit
// is set, because the comparison at fsm.go:50-53 matches.
func TestRFC5880AuthenticatedSessionAcceptsAuthenticatedPacket(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	m.SetAuth(rfc5880AuthPair(t))
	if m.vars.AuthType == 0 {
		t.Fatal("precondition: SetAuth must install a nonzero bfd.AuthType")
	}
	p := recv(packet.StateDown, 0)
	p.Auth = true
	if err := m.Receive(p); err != nil {
		t.Fatalf("A=1 packet rejected by an authenticated session: %v", err)
	}
}

// RFC requirement: RFC5880-6.8.6-10 negative -- a packet with A=0 arriving at
// a session whose bfd.AuthType is nonzero is discarded (fsm.go:50-53), so an
// attacker cannot strip authentication off a protected session by clearing the
// bit.
func TestRFC5880AuthenticatedSessionDiscardsUnauthenticatedPacket(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	m.SetAuth(rfc5880AuthPair(t))
	if err := m.Receive(recv(packet.StateDown, 0)); !errors.Is(err, ErrAuthMismatch) {
		t.Fatalf("got err %v, want ErrAuthMismatch", err)
	}
	if m.State() != packet.StateDown {
		t.Fatalf("state mutated to %v by a discarded A=0 packet", m.State())
	}
}

// RFC requirement: RFC5880-6.8.6-12 positive -- when the received Required Min
// Echo RX Interval is zero, echo transmission ceases. Receive
// (internal/component/bfd/session/fsm.go:62) stores the advertised value in
// RemoteMinEchoRxInterval and EchoEnabled (timers.go:136-139) then reports
// false, which makes EchoInterval zero and PrimeEcho (timers.go:178-186) clear
// the echo deadline so no further echo packet is scheduled.
func TestRFC5880EchoCeasesWhenPeerAdvertisesZero(t *testing.T) {
	clk := newFakeClock()
	m := rfc5880EchoMachine(t, clk)

	on := recv(packet.StateInit, peerLearnedDiscr)
	on.RequiredMinEchoRxInterval = 50_000
	if err := m.Receive(on); err != nil {
		t.Fatalf("Receive echo-capable: %v", err)
	}
	m.PrimeEcho(clk.Now())
	if !m.EchoEnabled() || m.NextEchoTxDeadline().IsZero() {
		t.Fatal("precondition: echo must be scheduled before the peer withdraws it")
	}

	off := recv(packet.StateUp, m.vars.LocalDiscr)
	off.RequiredMinEchoRxInterval = 0
	if err := m.Receive(off); err != nil {
		t.Fatalf("Receive echo-withdraw: %v", err)
	}
	if m.EchoEnabled() {
		t.Fatal("echo still enabled after the peer advertised Required Min Echo RX 0")
	}
	m.PrimeEcho(clk.Now())
	if !m.NextEchoTxDeadline().IsZero() {
		t.Fatalf("echo still scheduled at %v after the peer advertised zero", m.NextEchoTxDeadline())
	}
	if m.EchoInterval() != 0 {
		t.Fatalf("EchoInterval = %v after the peer advertised zero, want 0", m.EchoInterval())
	}
}

// RFC requirement: RFC5880-6.8.6-12 negative -- the cease is driven by the
// advertised zero, not by a blanket refusal to echo: a nonzero Required Min
// Echo RX Interval keeps echo enabled and schedulable (timers.go:136-139).
// RFC requirement: RFC5880-6.8.9-2 positive -- echo packets are transmitted
// only once the remote Required Min Echo RX Interval is nonzero, which is
// exactly the condition EchoEnabled encodes.
func TestRFC5880EchoEnabledWhenPeerAdvertisesNonZero(t *testing.T) {
	clk := newFakeClock()
	m := rfc5880EchoMachine(t, clk)

	on := recv(packet.StateInit, peerLearnedDiscr)
	on.RequiredMinEchoRxInterval = 75_000
	if err := m.Receive(on); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if !m.EchoEnabled() {
		t.Fatal("echo not enabled after the peer advertised a nonzero Required Min Echo RX")
	}
	m.PrimeEcho(clk.Now())
	if m.NextEchoTxDeadline().IsZero() {
		t.Fatal("echo deadline not armed with echo negotiated")
	}
}

// RFC requirement: RFC5880-6.8.9-2 negative -- with the peer advertising no
// echo capability (RemoteMinEchoRxInterval zero) EchoEnabled
// (internal/component/bfd/session/timers.go:136-139) is false and PrimeEcho
// (timers.go:179-182) refuses to arm the deadline, so no echo packet is ever
// scheduled however eager the local configuration is.
func TestRFC5880EchoNotTransmittedWithoutPeerAdvertisement(t *testing.T) {
	clk := newFakeClock()
	m := rfc5880EchoMachine(t, clk)

	silent := recv(packet.StateInit, peerLearnedDiscr)
	silent.RequiredMinEchoRxInterval = 0
	if err := m.Receive(silent); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if m.EchoEnabled() {
		t.Fatal("echo enabled although the peer advertised Required Min Echo RX 0")
	}
	m.PrimeEcho(clk.Now())
	if !m.NextEchoTxDeadline().IsZero() {
		t.Fatalf("echo scheduled at %v with no peer advertisement", m.NextEchoTxDeadline())
	}
}

// RFC requirement: RFC5880-6.8.9-3 positive -- the echo transmit interval is
// never shorter than the peer's Required Min Echo RX Interval. EchoInterval
// (internal/component/bfd/session/timers.go:144-153) returns the MAXIMUM of
// the local DesiredMinEchoTxInterval and the peer's advertised value, so a
// peer asking for 200 ms gets 200 ms even when the local target is 50 ms.
func TestRFC5880EchoIntervalHonorsPeerFloor(t *testing.T) {
	clk := newFakeClock()
	m := rfc5880EchoMachine(t, clk)

	p := recv(packet.StateInit, peerLearnedDiscr)
	p.RequiredMinEchoRxInterval = 200_000
	if err := m.Receive(p); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if got := m.EchoInterval(); got != 200*time.Millisecond {
		t.Fatalf("EchoInterval = %v, want the peer's 200ms floor", got)
	}
}

// RFC requirement: RFC5880-6.8.9-3 negative -- the peer's value is a floor and
// not an override: when the peer asks for 20 ms and the local target is 50 ms,
// the max() at timers.go:148-151 keeps 50 ms, which still satisfies "not less
// than the remote Required Min Echo RX Interval". Without this the positive
// could pass on code that simply copied the peer's number.
func TestRFC5880EchoIntervalNotBelowLocalTarget(t *testing.T) {
	clk := newFakeClock()
	m := rfc5880EchoMachine(t, clk)

	p := recv(packet.StateInit, peerLearnedDiscr)
	p.RequiredMinEchoRxInterval = 20_000
	if err := m.Receive(p); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	got := m.EchoInterval()
	if got != 50*time.Millisecond {
		t.Fatalf("EchoInterval = %v, want the local 50ms target", got)
	}
	if got < 20*time.Millisecond {
		t.Fatalf("EchoInterval %v is below the peer's 20ms Required Min Echo RX", got)
	}
}

// RFC requirement: RFC5880-6.8.8-2 positive -- a means of detecting missing
// Echo packets is implemented. RegisterEchoTx
// (internal/component/bfd/session/timers.go:263-282) records every echo TX in
// the outstanding ring and EchoDetectionExpired (timers.go:333-349) reports
// true once an unreturned entry is older than DetectMult * EchoInterval.
// RFC requirement: RFC5880-6.8.5-1 positive -- EchoFail (timers.go:362-374)
// then sets bfd.SessionState to Down and bfd.LocalDiag to 2 (echo function
// failed).
func TestRFC5880EchoMissDetectedAndReported(t *testing.T) {
	clk := newFakeClock()
	m := rfc5880EchoMachine(t, clk)

	p := recv(packet.StateInit, peerLearnedDiscr)
	p.RequiredMinEchoRxInterval = 50_000
	if err := m.Receive(p); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if m.State() != packet.StateUp {
		t.Fatalf("precondition: expected Up, got %v", m.State())
	}

	m.RegisterEchoTx(1, clk.Now())
	clk.advance(m.EchoDetectInterval() + time.Millisecond)
	if !m.EchoDetectionExpired(clk.Now()) {
		t.Fatal("an unreturned echo older than the echo detection time was not detected")
	}

	m.EchoFail()
	if m.State() != packet.StateDown {
		t.Fatalf("state after EchoFail = %v, want Down", m.State())
	}
	if m.LocalDiag() != packet.DiagEchoFailed {
		t.Fatalf("bfd.LocalDiag after EchoFail = %v, want echo-function-failed (2)", m.LocalDiag())
	}
}

// RFC requirement: RFC5880-6.8.8-2 negative -- the detector reports a miss
// only for echoes that did not come back: MatchEchoRx
// (internal/component/bfd/session/timers.go:292-302) clears the ring slot on a
// returning echo, so EchoDetectionExpired stays false however long the session
// then runs.
// RFC requirement: RFC5880-6.8.5-1 negative -- EchoFail (timers.go:363-365) is
// scoped to Init and Up: calling it on a session already Down leaves the state
// and the existing diagnostic alone, so it cannot manufacture a spurious
// diagnostic 2.
func TestRFC5880EchoReturnClearsMissAndFailIsScoped(t *testing.T) {
	clk := newFakeClock()
	m := rfc5880EchoMachine(t, clk)

	p := recv(packet.StateInit, peerLearnedDiscr)
	p.RequiredMinEchoRxInterval = 50_000
	if err := m.Receive(p); err != nil {
		t.Fatalf("Receive: %v", err)
	}

	m.RegisterEchoTx(7, clk.Now())
	clk.advance(10 * time.Millisecond)
	if _, ok := m.MatchEchoRx(7, clk.Now()); !ok {
		t.Fatal("returning echo was not matched against the outstanding ring")
	}
	clk.advance(m.EchoDetectInterval() * 3)
	if m.EchoDetectionExpired(clk.Now()) {
		t.Fatal("a returned echo was still counted as missing")
	}

	m.vars.SessionState = packet.StateDown
	m.vars.LocalDiag = packet.DiagNeighborSignaledDown
	m.EchoFail()
	if m.LocalDiag() != packet.DiagNeighborSignaledDown {
		t.Fatalf("EchoFail overwrote the diagnostic of an already-Down session: %v", m.LocalDiag())
	}
}

// ---------------------------------------------------------------------
// Section 6.8.7 -- transmit interval floor
// ---------------------------------------------------------------------

// RFC requirement: RFC5880-6.8.7-1 positive -- the periodic transmit interval
// is max(bfd.DesiredMinTxInterval, bfd.RemoteMinRxInterval) less the jitter
// reduction. AdvanceTxWithJitter
// (internal/component/bfd/session/timers.go:110-116) sets nextTxAt to
// now + TransmitInterval() - reduction, so a zero reduction gives the full
// negotiated interval and a 25% reduction never goes further.
func TestRFC5880TransmitDeadlineUsesNegotiatedInterval(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	m.vars.SessionState = packet.StateUp
	m.vars.DesiredMinTxInterval = 300_000
	m.vars.RemoteMinRxInterval = 700_000

	base := m.TransmitInterval()
	if base != 700*time.Millisecond {
		t.Fatalf("TransmitInterval = %v, want max(300ms, 700ms)", base)
	}
	now := clk.Now()
	m.AdvanceTxWithJitter(now, 0)
	if got := m.NextTxDeadline().Sub(now); got != base {
		t.Fatalf("next TX in %v with no jitter, want the full %v", got, base)
	}

	quarter := base / 4
	m.AdvanceTxWithJitter(now, quarter)
	if got := m.NextTxDeadline().Sub(now); got != base-quarter {
		t.Fatalf("next TX in %v with a 25%% reduction, want %v", got, base-quarter)
	}
}

// RFC requirement: RFC5880-6.8.7-1 negative -- the deadline can never land
// earlier than one full interval by way of an out-of-range reduction:
// AdvanceTxWithJitter (timers.go:111-114) clamps a negative or
// interval-or-larger reduction back to zero, so a caller bug cannot make the
// session transmit faster than the negotiated rate.
func TestRFC5880TransmitDeadlineClampsBadJitter(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	m.vars.SessionState = packet.StateUp
	m.vars.DesiredMinTxInterval = 300_000
	m.vars.RemoteMinRxInterval = 300_000

	base := m.TransmitInterval()
	now := clk.Now()
	for _, bad := range []time.Duration{-time.Second, base, base * 2} {
		m.AdvanceTxWithJitter(now, bad)
		if got := m.NextTxDeadline().Sub(now); got != base {
			t.Fatalf("reduction %v produced a %v gap, want the clamped full %v", bad, got, base)
		}
	}
}

// ---------------------------------------------------------------------
// Section 6.8.16 -- administrative control
// ---------------------------------------------------------------------

// RFC requirement: RFC5880-6.8.16-5 positive -- the disable steps held by the
// Machine: AdminDown (internal/component/bfd/session/fsm.go) sets
// bfd.SessionState to AdminDown and bfd.LocalDiag to the operator's diagnostic
// (Administratively Down). The third step, ceasing Echo transmission, is the
// engine's and is asserted by engine TestRFC5880AdminDisableCeasesEchoes.
// RFC requirement: RFC5880-6.8.16-4 positive -- the enable step: AdminEnable
// (fsm.go) sets bfd.SessionState to Down from AdminDown, clears
// bfd.LocalDiag, and restores the slow-start transmit interval so the
// handshake restarts from the beginning. Exactly two notifications fire, one
// per transition.
func TestRFC5880AdministrativeDisableEnable(t *testing.T) {
	clk := newFakeClock()
	m, rec := newMachine(t, clk)
	m.vars.SessionState = packet.StateUp
	m.vars.DesiredMinTxInterval = 300_000

	m.AdminDown(packet.DiagAdminDown)
	if m.State() != packet.StateAdminDown {
		t.Fatalf("state after AdminDown = %v, want AdminDown", m.State())
	}
	if m.LocalDiag() != packet.DiagAdminDown {
		t.Fatalf("bfd.LocalDiag after AdminDown = %v, want administratively-down", m.LocalDiag())
	}

	m.AdminEnable()
	if m.State() != packet.StateDown {
		t.Fatalf("state after AdminEnable = %v, want Down", m.State())
	}
	if m.LocalDiag() != packet.DiagNone {
		t.Fatalf("bfd.LocalDiag after AdminEnable = %v, want no-diagnostic", m.LocalDiag())
	}
	if m.DesiredMinTxIntervalUs() != SlowStartIntervalUs {
		t.Fatalf("slow-start interval not restored on enable: %d", m.DesiredMinTxIntervalUs())
	}
	if len(rec.transitions) != 2 {
		t.Fatalf("expected exactly 2 notifications (AdminDown, Down), got %+v", rec.transitions)
	}
}

// VALIDATES: the administrative calls are guarded. AdminEnable on a session
// that is merely Down (fsm.go) keeps its diagnostic (Control Detection Time
// Expired) and notifies nothing, and a second AdminDown on an AdminDown
// session notifies nothing and keeps the first diagnostic. Neither guard is an
// RFC statement; the RFC5880-6.8.16-4 negative is
// TestRFC5880EnableLandsOnDownWhilePeerSaysUpOrInit.
func TestRFC5880AdministrativeCallsAreGuarded(t *testing.T) {
	clk := newFakeClock()
	m, rec := newMachine(t, clk)

	// AdminEnable on a session that is merely Down must do nothing.
	m.vars.LocalDiag = packet.DiagControlDetectExpired
	m.AdminEnable()
	if m.LocalDiag() != packet.DiagControlDetectExpired {
		t.Fatalf("AdminEnable cleared the diagnostic of a non-AdminDown session: %v", m.LocalDiag())
	}
	if len(rec.transitions) != 0 {
		t.Fatalf("AdminEnable notified on a non-AdminDown session: %+v", rec.transitions)
	}

	m.AdminDown(packet.DiagAdminDown)
	before := len(rec.transitions)
	m.AdminDown(packet.DiagPathDown)
	if len(rec.transitions) != before {
		t.Fatalf("second AdminDown notified again: %+v", rec.transitions)
	}
	if m.LocalDiag() != packet.DiagAdminDown {
		t.Fatalf("second AdminDown overwrote the diagnostic: %v", m.LocalDiag())
	}
}

// RFC requirement: RFC5880-6.8.16-4 negative -- the inputs are pushed toward
// the violation: an Up session is disabled, the peer keeps sending Up (or
// Init) while it is AdminDown, so bfd.RemoteSessionState says Up (or Init)
// when AdminEnable runs. The session still lands on Down, never Up or Init,
// and the next Control packet it builds announces Down.
func TestRFC5880EnableLandsOnDownWhilePeerSaysUpOrInit(t *testing.T) {
	for _, peer := range []packet.State{packet.StateUp, packet.StateInit} {
		clk := newFakeClock()
		m, _ := newMachine(t, clk)
		if err := m.Receive(recv(packet.StateDown, 0)); err != nil {
			t.Fatalf("peer Down: %v", err)
		}
		if err := m.Receive(recv(packet.StateInit, peerLearnedDiscr)); err != nil {
			t.Fatalf("peer Init: %v", err)
		}
		if m.State() != packet.StateUp {
			t.Fatalf("precondition: state after the handshake = %v, want Up", m.State())
		}

		m.AdminDown(packet.DiagAdminDown)
		if err := m.Receive(recv(peer, peerLearnedDiscr)); err != nil {
			t.Fatalf("peer %v while AdminDown: %v", peer, err)
		}
		if m.vars.RemoteSessionState != peer {
			t.Fatalf("precondition: bfd.RemoteSessionState = %v, want %v", m.vars.RemoteSessionState, peer)
		}

		m.AdminEnable()
		if m.State() != packet.StateDown {
			t.Fatalf("peer says %v: state after AdminEnable = %v, want Down", peer, m.State())
		}
		if got := m.Build().State; got != packet.StateDown {
			t.Fatalf("peer says %v: first packet after AdminEnable announces %v, want Down", peer, got)
		}
	}
}

// ---------------------------------------------------------------------
// Section 6.7 -- authentication sequence number in the outgoing packet
// ---------------------------------------------------------------------

// RFC requirement: RFC5880-6.7.4-11 positive -- under Keyed SHA1
// (rfc5880AuthPair) the Sequence Number field of the authentication section
// is set to bfd.XmitAuthSeq. Machine.Sign
// (internal/component/bfd/session/auth.go:66-78) reads m.vars.XmitAuthSeq and
// hands it to the signer, which writes it big-endian at offset 4 of the auth
// section (internal/component/bfd/auth/sha1.go:84).
func TestRFC5880AuthSequenceFieldIsXmitAuthSeq(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	m.SetAuth(rfc5880AuthPair(t))
	m.vars.XmitAuthSeq = 0x11223344

	buf := make([]byte, packet.MandatoryLen+m.authBodyLen())
	c := m.Build()
	c.WriteTo(buf, 0)
	n := m.Sign(buf, packet.MandatoryLen)
	if n != m.authBodyLen() {
		t.Fatalf("Sign wrote %d bytes, want %d", n, m.authBodyLen())
	}
	if got := binary.BigEndian.Uint32(buf[packet.MandatoryLen+4:]); got != 0x11223344 {
		t.Fatalf("Sequence Number field = %#x, want bfd.XmitAuthSeq %#x", got, uint32(0x11223344))
	}
}

// RFC requirement: RFC5880-6.7.4-11 negative -- under Keyed SHA1
// (rfc5880AuthPair) the field tracks the variable rather than being a
// constant: AdvanceAuthSeq
// (internal/component/bfd/session/auth.go:86-94) bumps bfd.XmitAuthSeq and the
// next Sign writes the new value, so a signer that emitted a fixed number
// would fail here.
func TestRFC5880AuthSequenceFieldFollowsAdvance(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	m.SetAuth(rfc5880AuthPair(t))
	m.vars.XmitAuthSeq = 100

	buf := make([]byte, packet.MandatoryLen+m.authBodyLen())
	c := m.Build()
	c.WriteTo(buf, 0)
	m.Sign(buf, packet.MandatoryLen)
	first := binary.BigEndian.Uint32(buf[packet.MandatoryLen+4:])

	m.AdvanceAuthSeq()
	m.Sign(buf, packet.MandatoryLen)
	second := binary.BigEndian.Uint32(buf[packet.MandatoryLen+4:])

	if second == first {
		t.Fatalf("Sequence Number stayed %#x across an AdvanceAuthSeq", first)
	}
	if second != first+1 {
		t.Fatalf("Sequence Number = %d after advance, want %d", second, first+1)
	}
}

// ---------------------------------------------------------------------
// Owner decision D-8 fixes (2026-09-27)
// ---------------------------------------------------------------------

// RFC requirement: RFC5880-6.8.1-5 positive -- the clear holds in every
// state, not only on the Init or Up expiry. A peer that signals Down keeps its
// discriminator; when it then stays silent for a Detection Time, CheckDetection
// (internal/component/bfd/session/timers.go) clears bfd.RemoteDiscr while the
// session stays Down and reports no state change. Before the Detection Time
// passes the discriminator is kept.
func TestRFC5880RemoteDiscrClearedAfterSilenceWhileDown(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	if err := m.Receive(recv(packet.StateInit, peerLearnedDiscr)); err != nil {
		t.Fatalf("Receive Init: %v", err)
	}
	if err := m.Receive(recv(packet.StateDown, m.vars.LocalDiscr)); err != nil {
		t.Fatalf("Receive Down: %v", err)
	}
	learned := m.RemoteDiscriminator()
	if m.State() != packet.StateDown || learned == 0 {
		t.Fatalf("precondition: want Down with a learned RemoteDiscr, got %v and %d", m.State(), learned)
	}

	clk.advance(m.DetectionInterval() - time.Millisecond)
	m.CheckDetection(clk.Now())
	if got := m.RemoteDiscriminator(); got != learned {
		t.Fatalf("bfd.RemoteDiscr = %d before a Detection Time passed, want %d", got, learned)
	}

	clk.advance(2 * time.Millisecond)
	if m.CheckDetection(clk.Now()) {
		t.Fatal("CheckDetection reported a state change for a session already Down")
	}
	if got := m.RemoteDiscriminator(); got != 0 {
		t.Fatalf("bfd.RemoteDiscr = %d after a Detection Time of silence while Down, want 0", got)
	}
	if m.LocalDiag() != packet.DiagNeighborSignaledDown {
		t.Fatalf("bfd.LocalDiag overwritten to %v", m.LocalDiag())
	}
}

// RFC requirement: RFC5880-6.8.3-1 positive -- the one-second floor returns on
// EVERY entry into a state other than Up. onStateChange
// (internal/component/bfd/session/fsm.go) restores it on AdminDown entered
// from Up, and an echo slow-down that was active does not survive the engine's
// ClearEchoSchedule to put the configured sub-second value back. The Up Poll
// settles first, because the slow-down waits for it (RFC 5880 Section 6.8.3).
func TestRFC5880SlowStartFloorRestoredOnAdminDown(t *testing.T) {
	for _, echo := range []bool{false, true} {
		clk := newFakeClock()
		m, _ := newMachine(t, clk)
		if err := m.Receive(recv(packet.StateInit, peerLearnedDiscr)); err != nil {
			t.Fatalf("Receive: %v", err)
		}
		if m.State() != packet.StateUp || m.DesiredMinTxIntervalUs() != 300_000 {
			t.Fatalf("precondition: want Up at 300000 us, got %v at %d", m.State(), m.DesiredMinTxIntervalUs())
		}
		if echo {
			settlePoll(t, m)
			m.ApplyEchoSlowdown()
			if got := m.DesiredMinTxIntervalUs(); got != EchoSlowdownIntervalUs {
				t.Fatalf("precondition: echo slow-down not applied, bfd.DesiredMinTxInterval = %d", got)
			}
		}
		m.AdminDown(packet.DiagAdminDown)
		m.ClearEchoSchedule()
		if got := m.DesiredMinTxIntervalUs(); got < 1_000_000 {
			t.Fatalf("echo %v: bfd.DesiredMinTxInterval = %d us in AdminDown, want >= 1000000", echo, got)
		}
	}
}

// RFC requirement: RFC5880-6.8.3-5 positive -- a reduced
// bfd.RemoteMinRxInterval is honored at once: Receive calls
// rescheduleTxLocked (internal/component/bfd/session/timers.go), which moves
// the pending deadline to the previous transmission plus the new interval, and
// to a deadline already past when that interval has elapsed. The jitter drawn
// for the last packet keeps its share of the new interval.
// RFC requirement: RFC5880-6.8.7-4 positive -- the transmit interval is
// recalculated when bfd.RemoteMinRxInterval changes in either direction: an
// increase moves the pending deadline later, so the next packet does not go
// out at the old, shorter interval.
func TestRFC5880RemoteMinRxChangeReschedulesNextTx(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	m.vars.SessionState = packet.StateUp
	m.vars.DesiredMinTxInterval = 300_000

	advertise := func(requiredMinRxUs uint32) {
		t.Helper()
		c := recv(packet.StateUp, m.vars.LocalDiscr)
		c.RequiredMinRxInterval = requiredMinRxUs
		if err := m.Receive(c); err != nil {
			t.Fatalf("Receive %d us: %v", requiredMinRxUs, err)
		}
	}

	advertise(900_000)
	sent := clk.Now()
	m.AdvanceTxWithJitter(sent, 0)
	clk.advance(100 * time.Millisecond)
	advertise(400_000)
	if got, want := m.NextTxDeadline(), sent.Add(400*time.Millisecond); !got.Equal(want) {
		t.Fatalf("after the reduction to 400ms the next TX is due %v after the last, want 400ms", got.Sub(sent))
	}

	sent = clk.Now()
	m.AdvanceTxWithJitter(sent, 0)
	advertise(1_200_000)
	if got, want := m.NextTxDeadline(), sent.Add(1200*time.Millisecond); !got.Equal(want) {
		t.Fatalf("after the increase to 1.2s the next TX is due %v after the last, want 1.2s", got.Sub(sent))
	}

	sent = clk.Now()
	m.AdvanceTxWithJitter(sent, 300*time.Millisecond)
	clk.advance(700 * time.Millisecond)
	advertise(400_000)
	if got, want := m.NextTxDeadline(), sent.Add(300*time.Millisecond); !got.Equal(want) {
		t.Fatalf("jittered wait of 900ms of 1.2s rescaled to %v, want 300ms of 400ms", got.Sub(sent))
	}
	if !m.NextTxDeadline().Before(clk.Now()) {
		t.Fatal("a deadline the new interval has already passed is not due at once")
	}
}

// RFC requirement: RFC5880-6.8.7-4 positive -- the transmit interval is
// recalculated when bfd.DesiredMinTxInterval changes: reverting the echo
// slow-down while Up (ClearEchoSchedule, revertEchoSlowdownLocked in
// internal/component/bfd/session/timers.go) lowers bfd.DesiredMinTxInterval
// from 1s to the configured 300ms, and the pending deadline moves from 1s to
// 300ms after the previous transmission, the greater of the new
// bfd.DesiredMinTxInterval and bfd.RemoteMinRxInterval (100ms). The slow-down
// Poll settles before the revert, which waits for it (RFC 5880 Section 6.8.3).
func TestRFC5880DesiredMinTxChangeReschedulesNextTx(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	m.vars.SessionState = packet.StateUp
	m.vars.ConfiguredDesiredMinTxInterval = 300_000
	m.vars.DesiredMinTxInterval = 300_000
	m.vars.RemoteMinRxInterval = 100_000

	m.ApplyEchoSlowdown()
	// The raised interval is in force only once its Poll has terminated
	// (RFC 5880 Section 6.8.3, RFC5880-6.8.3-3), so the Poll settles first.
	settlePoll(t, m)
	sent := clk.Now()
	m.AdvanceTxWithJitter(sent, 0)
	if got, want := m.NextTxDeadline(), sent.Add(time.Second); !got.Equal(want) {
		t.Fatalf("precondition: under the echo slow-down the next TX is due %v after the last, want 1s", got.Sub(sent))
	}
	clk.advance(100 * time.Millisecond)
	m.ClearEchoSchedule()
	if m.vars.DesiredMinTxInterval != 300_000 {
		t.Fatalf("precondition: bfd.DesiredMinTxInterval = %d after the revert, want 300000", m.vars.DesiredMinTxInterval)
	}
	if got, want := m.NextTxDeadline(), sent.Add(300*time.Millisecond); !got.Equal(want) {
		t.Fatalf("after bfd.DesiredMinTxInterval fell to 300ms the next TX is due %v after the last, want 300ms", got.Sub(sent))
	}
}

// RFC requirement: RFC5880-6.8.3-5 negative -- rescheduling is bound to a
// change of the interval: a packet that carries the same Required Min RX
// Interval leaves the pending deadline where AdvanceTxWithJitter put it.
func TestRFC5880UnchangedRemoteMinRxKeepsNextTx(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	m.vars.SessionState = packet.StateUp
	m.vars.DesiredMinTxInterval = 300_000
	c := recv(packet.StateUp, m.vars.LocalDiscr)
	c.RequiredMinRxInterval = 900_000
	if err := m.Receive(c); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	sent := clk.Now()
	m.AdvanceTxWithJitter(sent, 100*time.Millisecond)
	clk.advance(50 * time.Millisecond)
	if err := m.Receive(c); err != nil {
		t.Fatalf("Receive again: %v", err)
	}
	if got, want := m.NextTxDeadline(), sent.Add(800*time.Millisecond); !got.Equal(want) {
		t.Fatalf("next TX moved to %v after the last with no interval change, want 800ms", got.Sub(sent))
	}
}

// RFC requirement: RFC5880-6.8.7-5 positive -- the Passive silence holds
// whenever bfd.RemoteDiscr is zero, not only at Init. A Passive session that
// times out, from Up or while Down after a peer-signaled Down, loses the
// discriminator and schedules no transmission: onStateChange and
// CheckDetection (internal/component/bfd/session/fsm.go, timers.go) both ask
// transmitPermitted before arming nextTxAt.
func TestRFC5880PassiveSilentOnceRemoteDiscrCleared(t *testing.T) {
	clk := newFakeClock()
	m := newPassiveMachine(t, clk)
	if err := m.Receive(recv(packet.StateInit, peerLearnedDiscr)); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if m.State() != packet.StateUp || m.NextTxDeadline().IsZero() {
		t.Fatalf("precondition: want Up and transmitting, got %v, deadline %v", m.State(), m.NextTxDeadline())
	}
	clk.advance(10 * time.Second)
	if !m.CheckDetection(clk.Now()) {
		t.Fatal("precondition: detection did not fire")
	}
	if m.RemoteDiscriminator() != 0 {
		t.Fatal("precondition: RemoteDiscr not cleared by the detection expiry")
	}
	if !m.NextTxDeadline().IsZero() {
		t.Fatal("Passive session with bfd.RemoteDiscr zero scheduled a transmission after the timeout")
	}

	m2 := newPassiveMachine(t, clk)
	if err := m2.Receive(recv(packet.StateInit, peerLearnedDiscr)); err != nil {
		t.Fatalf("Receive Init: %v", err)
	}
	if err := m2.Receive(recv(packet.StateDown, m2.vars.LocalDiscr)); err != nil {
		t.Fatalf("Receive Down: %v", err)
	}
	clk.advance(10 * time.Second)
	m2.CheckDetection(clk.Now())
	if !m2.NextTxDeadline().IsZero() {
		t.Fatal("Passive session kept transmitting after a Detection Time of silence while Down")
	}
}

// RFC requirement: RFC5880-6.8.7-5 negative -- the silence is lifted by the
// peer: once a packet from the peer sets bfd.RemoteDiscr again, the Passive
// session that fell silent transmits.
func TestRFC5880PassiveTransmitsAgainOnceRemoteDiscrLearned(t *testing.T) {
	clk := newFakeClock()
	m := newPassiveMachine(t, clk)
	if err := m.Receive(recv(packet.StateInit, peerLearnedDiscr)); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	clk.advance(10 * time.Second)
	m.CheckDetection(clk.Now())
	if !m.NextTxDeadline().IsZero() {
		t.Fatal("precondition: Passive session must be silent after the timeout")
	}
	if err := m.Receive(recv(packet.StateDown, 0)); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if m.NextTxDeadline().IsZero() {
		t.Fatal("Passive session stayed silent after the peer's packet set bfd.RemoteDiscr")
	}
}

// RFC requirement: RFC5880-6.8.3-3 positive -- an increase of
// bfd.DesiredMinTxInterval while Up leaves the actual transmission interval
// unchanged until the Poll Sequence it started has terminated. Two increases
// are driven: the echo slow-down (ApplyEchoSlowdown raises 300ms to 1s) and
// the Up transition to a configured interval above the one-second slow start
// (onStateChange raises 1s to 2s). Both paths that compute the interval are
// asserted while the Poll is outstanding: AdvanceTxWithJitter, which schedules
// the next packet, and Receive, whose rescheduleTxLocked would move a pending
// deadline to the raised interval.
// MUTATION: TransmitInterval reading m.vars.DesiredMinTxInterval in place of
// desiredMinTxInForceUs turns every assertion below red.
func TestRFC5880RaisedDesiredMinTxHeldUntilPollTerminates(t *testing.T) {
	t.Run("echo slow-down", func(t *testing.T) {
		clk := newFakeClock()
		m, _ := newMachine(t, clk)
		m.vars.SessionState = packet.StateUp
		m.vars.ConfiguredDesiredMinTxInterval = 300_000
		m.vars.DesiredMinTxInterval = 300_000
		m.vars.RemoteMinRxInterval = 100_000

		m.ApplyEchoSlowdown()
		if !m.PollOutstanding() || m.vars.DesiredMinTxInterval != 1_000_000 {
			t.Fatalf("precondition: slow-down not applied (poll=%v desired=%d)", m.PollOutstanding(), m.vars.DesiredMinTxInterval)
		}
		if got := m.TransmitInterval(); got != 300*time.Millisecond {
			t.Fatalf("transmit interval %v while the slow-down Poll is outstanding, want the pre-Poll 300ms", got)
		}
		sent := clk.Now()
		m.AdvanceTxWithJitter(sent, 0)
		if got, want := m.NextTxDeadline(), sent.Add(300*time.Millisecond); !got.Equal(want) {
			t.Fatalf("next TX due %v after the last while the Poll is outstanding, want 300ms", got.Sub(sent))
		}
		clk.advance(100 * time.Millisecond)
		c := recv(packet.StateUp, m.vars.LocalDiscr)
		c.RequiredMinRxInterval = 100_000
		if err := m.Receive(c); err != nil {
			t.Fatal(err)
		}
		if got, want := m.NextTxDeadline(), sent.Add(300*time.Millisecond); !got.Equal(want) {
			t.Fatalf("a packet without Final moved the next TX to %v after the last, want 300ms", got.Sub(sent))
		}
	})

	t.Run("Up transition", func(t *testing.T) {
		clk := newFakeClock()
		m, _ := newMachine(t, clk)
		m.vars.ConfiguredDesiredMinTxInterval = 2_000_000
		if err := m.Receive(recv(packet.StateInit, m.vars.LocalDiscr)); err != nil {
			t.Fatal(err)
		}
		if m.State() != packet.StateUp || !m.PollOutstanding() || m.vars.DesiredMinTxInterval != 2_000_000 {
			t.Fatalf("precondition: state %v poll=%v desired=%d, want Up with a Poll carrying 2s", m.State(), m.PollOutstanding(), m.vars.DesiredMinTxInterval)
		}
		if got := m.TransmitInterval(); got != time.Second {
			t.Fatalf("transmit interval %v while the Up Poll is outstanding, want the pre-Poll 1s", got)
		}
	})
}

// RFC requirement: RFC5880-6.8.3-3 negative -- the hold ends with the Poll
// Sequence: the Final that terminates it makes the raised interval the actual
// one, and moves the pending deadline to it, so the hold never pins the
// session at the old rate. A session that leaves Up drops the hold with the
// Poll.
func TestRFC5880RaisedDesiredMinTxAppliedAtFinal(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	m.vars.SessionState = packet.StateUp
	m.vars.ConfiguredDesiredMinTxInterval = 300_000
	m.vars.DesiredMinTxInterval = 300_000
	m.vars.RemoteMinRxInterval = 100_000

	m.ApplyEchoSlowdown()
	sent := clk.Now()
	m.AdvanceTxWithJitter(sent, 0)
	clk.advance(100 * time.Millisecond)
	c := recv(packet.StateUp, m.vars.LocalDiscr)
	c.RequiredMinRxInterval = 100_000
	c.Final = true
	if err := m.Receive(c); err != nil {
		t.Fatal(err)
	}
	if got := m.TransmitInterval(); got != time.Second {
		t.Fatalf("transmit interval %v after the Final, want the raised 1s", got)
	}
	if got, want := m.NextTxDeadline(), sent.Add(time.Second); !got.Equal(want) {
		t.Fatalf("after the Final the next TX is due %v after the last, want 1s", got.Sub(sent))
	}

	m2, _ := newMachine(t, clk)
	m2.vars.SessionState = packet.StateUp
	m2.vars.ConfiguredDesiredMinTxInterval = 300_000
	m2.vars.DesiredMinTxInterval = 300_000
	m2.ApplyEchoSlowdown()
	down := recv(packet.StateDown, m2.vars.LocalDiscr)
	if err := m2.Receive(down); err != nil {
		t.Fatal(err)
	}
	if m2.State() != packet.StateDown {
		t.Fatalf("precondition: state %v, want Down", m2.State())
	}
	if got := m2.TransmitInterval(); got != time.Second {
		t.Fatalf("transmit interval %v after leaving Up, want the 1s slow-start floor", got)
	}
}
