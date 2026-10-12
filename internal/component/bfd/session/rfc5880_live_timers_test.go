// VALIDATES: RFC 5880 Section 6.8.3 timer manipulation on a live session --
// ApplyTimers changes bfd.DesiredMinTxInterval, bfd.RequiredMinRxInterval and
// bfd.DetectMult on a session in any state, an Up session through a Poll
// Sequence, keeping the old transmission interval for an increase and the old
// receive interval in the Detection Time for a reduction until the Poll ends.
// PREVENTS: a profile edit that cannot reach an Up session (Reconfigure panics
// there), a timer change that skips its Poll, a second Poll started before the
// first has settled, and a Detection Time cut before the peer speeds up.
package session

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/packet"
)

// upMachine brings a newMachine session Up through the three-way handshake and
// settles the Poll that the Up transition starts, so a test starts from an Up
// session at its configured 300ms intervals with no Poll in flight.
func upMachine(t *testing.T, clk *fakeClock) *Machine {
	t.Helper()
	m, _ := newMachine(t, clk)
	if err := m.Receive(recv(packet.StateInit, m.vars.LocalDiscr)); err != nil {
		t.Fatal(err)
	}
	if m.State() != packet.StateUp {
		t.Fatalf("precondition: state %v, want Up", m.State())
	}
	settlePoll(t, m)
	if m.vars.DesiredMinTxInterval != 300_000 || m.vars.RequiredMinRxInterval != 300_000 {
		t.Fatalf("precondition: desired %d required %d, want 300000 both", m.vars.DesiredMinTxInterval, m.vars.RequiredMinRxInterval)
	}
	return m
}

// timers builds the request ApplyTimers reads: only its three timer fields.
func timers(desiredMinTx, requiredMinRx uint32, detectMult uint8) api.SessionRequest {
	return api.SessionRequest{
		DesiredMinTxInterval:  desiredMinTx,
		RequiredMinRxInterval: requiredMinRx,
		DetectMult:            detectMult,
	}
}

// Goal: a faster profile applied to an Up session reaches the packets it
// sends, in a Poll, with the faster transmit interval in force at once.
// Method: ApplyTimers on a settled Up session, then read Build and the timers.
func TestApplyTimersUpStartsPoll(t *testing.T) {
	clk := newFakeClock()
	m := upMachine(t, clk)
	m.vars.RemoteMinRxInterval = 50_000

	m.ApplyTimers(timers(100_000, 100_000, 5))

	if !m.PollOutstanding() {
		t.Fatal("a timer change on an Up session started no Poll Sequence")
	}
	c := m.Build()
	if !c.Poll || c.DesiredMinTxInterval != 100_000 || c.RequiredMinRxInterval != 100_000 || c.DetectMult != 5 {
		t.Fatalf("Build carries poll=%v tx=%d rx=%d mult=%d, want a Poll carrying 100000/100000/5", c.Poll, c.DesiredMinTxInterval, c.RequiredMinRxInterval, c.DetectMult)
	}
	if m.State() != packet.StateUp {
		t.Fatalf("state %v after a timer change, want Up", m.State())
	}
	// RFC 5880 Section 6.8.3: a decrease is not called out, so it is
	// effected immediately.
	if got := m.TransmitInterval(); got != 100*time.Millisecond {
		t.Fatalf("transmit interval %v after a decrease, want 100ms at once", got)
	}
}

// RFC requirement: RFC5880-6.8.3-3 positive -- an increase of
// bfd.DesiredMinTxInterval that a profile edit makes on an Up session leaves
// the actual transmission interval unchanged until the Poll Sequence it
// started has terminated, then takes effect at the Final.
func TestApplyTimersIncreaseHeldUntilFinal(t *testing.T) {
	clk := newFakeClock()
	m := upMachine(t, clk)
	m.vars.RemoteMinRxInterval = 50_000

	m.ApplyTimers(timers(1_000_000, 300_000, 3))

	if got := m.TransmitInterval(); got != 300*time.Millisecond {
		t.Fatalf("transmit interval %v while the Poll is outstanding, want the pre-Poll 300ms", got)
	}
	c := recv(packet.StateUp, m.vars.LocalDiscr)
	c.RequiredMinRxInterval = 50_000
	c.Final = true
	if err := m.Receive(c); err != nil {
		t.Fatal(err)
	}
	if got := m.TransmitInterval(); got != time.Second {
		t.Fatalf("transmit interval %v after the Final, want the raised 1s", got)
	}
}

// RFC requirement: RFC5880-6.8.3-4 positive -- "If bfd.RequiredMinRxInterval
// is reduced and bfd.SessionState is Up, the previous value of
// bfd.RequiredMinRxInterval MUST be used when calculating the Detection Time
// for the remote system until the Poll Sequence described above has
// terminated." A profile edit and the echo slow-down revert both reduce it.
// MUTATION: DetectionInterval reading m.vars.RequiredMinRxInterval in place of
// requiredMinRxInForceUs turns both subtests red.
func TestRequiredMinRxReductionHeldUntilFinal(t *testing.T) {
	t.Run("profile edit", func(t *testing.T) {
		clk := newFakeClock()
		m := upMachine(t, clk)
		m.vars.RemoteDesiredMinTx = 10_000

		m.ApplyTimers(timers(300_000, 100_000, 3))

		if m.vars.RequiredMinRxInterval != 100_000 || !m.PollOutstanding() {
			t.Fatalf("precondition: required %d poll=%v, want 100000 in a Poll", m.vars.RequiredMinRxInterval, m.PollOutstanding())
		}
		if got := m.DetectionInterval(); got != 900*time.Millisecond {
			t.Fatalf("Detection Time %v while the Poll is outstanding, want 3 x the previous 300ms", got)
		}
	})

	t.Run("echo slow-down revert", func(t *testing.T) {
		clk := newFakeClock()
		m := upMachine(t, clk)
		m.vars.RemoteDesiredMinTx = 10_000
		m.ApplyEchoSlowdown()
		settlePoll(t, m)
		if got := m.DetectionInterval(); got != 3*time.Second {
			t.Fatalf("precondition: Detection Time %v under the slow-down, want 3s", got)
		}

		m.revertEchoSlowdownLocked()

		if m.vars.RequiredMinRxInterval != 300_000 || !m.PollOutstanding() {
			t.Fatalf("precondition: required %d poll=%v, want 300000 in a Poll", m.vars.RequiredMinRxInterval, m.PollOutstanding())
		}
		if got := m.DetectionInterval(); got != 3*time.Second {
			t.Fatalf("Detection Time %v while the revert's Poll is outstanding, want 3 x the previous 1s", got)
		}
	})
}

// RFC requirement: RFC5880-6.8.3-4 negative -- the previous value is used only
// until the Poll Sequence terminates: the Final makes the reduced
// bfd.RequiredMinRxInterval part of the Detection Time, and a session leaving
// Up drops the hold with its Poll.
func TestRequiredMinRxReductionAppliedAtFinal(t *testing.T) {
	clk := newFakeClock()
	m := upMachine(t, clk)
	m.vars.RemoteDesiredMinTx = 10_000
	m.ApplyTimers(timers(300_000, 100_000, 3))

	c := recv(packet.StateUp, m.vars.LocalDiscr)
	c.DesiredMinTxInterval = 10_000
	c.Final = true
	if err := m.Receive(c); err != nil {
		t.Fatal(err)
	}
	if got := m.DetectionInterval(); got != 300*time.Millisecond {
		t.Fatalf("Detection Time %v after the Final, want 3 x the reduced 100ms", got)
	}

	m2 := upMachine(t, clk)
	m2.vars.RemoteDesiredMinTx = 10_000
	m2.ApplyTimers(timers(300_000, 100_000, 3))
	down := recv(packet.StateDown, m2.vars.LocalDiscr)
	down.DesiredMinTxInterval = 10_000
	if err := m2.Receive(down); err != nil {
		t.Fatal(err)
	}
	if got := m2.DetectionInterval(); got != 300*time.Millisecond {
		t.Fatalf("Detection Time %v after leaving Up, want 3 x the reduced 100ms", got)
	}
}

// Goal: a second timer change made while the first one's Poll is in flight is
// not lost and does not start a second Poll before the first has settled
// (RFC 5880 Section 6.8.3 choice 3). Method: two ApplyTimers calls, then the
// Final and a packet with F clear.
func TestApplyTimersWaitsForPollToSettle(t *testing.T) {
	clk := newFakeClock()
	m := upMachine(t, clk)

	m.ApplyTimers(timers(200_000, 200_000, 3))
	m.ApplyTimers(timers(100_000, 100_000, 4))

	if m.vars.DesiredMinTxInterval != 200_000 || m.vars.RequiredMinRxInterval != 200_000 {
		t.Fatalf("the second change rode the first Poll: desired %d required %d, want 200000 both", m.vars.DesiredMinTxInterval, m.vars.RequiredMinRxInterval)
	}
	c := recv(packet.StateUp, m.vars.LocalDiscr)
	c.Final = true
	if err := m.Receive(c); err != nil {
		t.Fatal(err)
	}
	if m.PollOutstanding() {
		t.Fatal("the deferred change started a Poll before a packet with the Final bit clear arrived")
	}
	c.Final = false
	if err := m.Receive(c); err != nil {
		t.Fatal(err)
	}
	if !m.PollOutstanding() {
		t.Fatal("the deferred change started no Poll once the first had settled")
	}
	if m.vars.DesiredMinTxInterval != 100_000 || m.vars.RequiredMinRxInterval != 100_000 || m.DetectMult() != 4 {
		t.Fatalf("after settling: desired %d required %d mult %d, want 100000/100000/4", m.vars.DesiredMinTxInterval, m.vars.RequiredMinRxInterval, m.DetectMult())
	}
}

// Goal: an unchanged request is a no-op, and a change of Detect Mult alone is
// effected at once with no Poll (RFC 5880 Section 6.8.3 obliges a Poll only
// for the two intervals). Method: ApplyTimers on a settled Up session.
func TestApplyTimersWithoutIntervalChangeStartsNoPoll(t *testing.T) {
	clk := newFakeClock()
	m := upMachine(t, clk)

	m.ApplyTimers(timers(300_000, 300_000, 3))
	if m.PollOutstanding() {
		t.Fatal("an unchanged request started a Poll")
	}
	m.ApplyTimers(timers(300_000, 300_000, 7))
	if m.PollOutstanding() {
		t.Fatal("a Detect Mult change started a Poll")
	}
	if got := m.Build().DetectMult; got != 7 {
		t.Fatalf("Detect Mult %d in the next packet, want 7", got)
	}
}

// Goal: a timer change on a session that is not Up keeps the one-second
// transmit floor and takes the new values when the session comes Up.
// Method: ApplyTimers on a Down session, then the handshake.
func TestApplyTimersDownTakesEffectAtUp(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)

	m.ApplyTimers(timers(100_000, 200_000, 4))

	if m.vars.DesiredMinTxInterval != SlowStartIntervalUs {
		t.Fatalf("desired %d on a Down session, want the one-second floor", m.vars.DesiredMinTxInterval)
	}
	if m.vars.RequiredMinRxInterval != 200_000 || !m.PollOutstanding() {
		t.Fatalf("required %d poll=%v on a Down session, want 200000 in a Poll", m.vars.RequiredMinRxInterval, m.PollOutstanding())
	}
	if err := m.Receive(recv(packet.StateInit, m.vars.LocalDiscr)); err != nil {
		t.Fatal(err)
	}
	if m.vars.DesiredMinTxInterval != 100_000 || m.DetectMult() != 4 {
		t.Fatalf("at Up: desired %d mult %d, want 100000 and 4", m.vars.DesiredMinTxInterval, m.DetectMult())
	}
}
