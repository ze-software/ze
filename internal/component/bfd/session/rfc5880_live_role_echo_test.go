// VALIDATES: a profile edit of passive or of the echo interval reaches a live
// session (owner decision 2026-10-12, "Apply live"). ApplyRole changes the
// session's role, which decides whether it may transmit while it has no
// remote discriminator (RFC 5880 Sections 6.1 and 6.8.7); ApplyEcho changes
// bfd.DesiredMinEchoTxInterval and the Required Min Echo RX Interval the
// session advertises (RFC 5880 Section 6.8.13), and moves a pending echo to
// the new interval (Section 6.8.9).
// PREVENTS: an edit written into the client's stored request that never
// reaches the session, so the session keeps its old role or echo interval
// until it is torn down.
package session

import (
	"testing"
	"time"
)

// TestApplyRolePassiveStopsDownTransmission makes a Down session that has
// learned no remote discriminator passive: it stops transmitting at once, and
// made active again, transmits at once.
// MUTATION: ApplyRole leaving nextTxAt alone keeps the passive session
// sending, so this test goes red.
func TestApplyRolePassiveStopsDownTransmission(t *testing.T) {
	clk := newFakeClock()
	m, _ := newMachine(t, clk)
	if m.NextTxDeadline().IsZero() {
		t.Fatal("precondition: an active Down session has a TX deadline")
	}

	m.ApplyRole(true)
	if m.role != RolePassive {
		t.Fatalf("role %v, want passive", m.role)
	}
	if !m.NextTxDeadline().IsZero() {
		t.Fatal("a passive session with no remote discriminator still transmits")
	}

	clk.advance(time.Second)
	m.ApplyRole(false)
	if m.role != RoleActive {
		t.Fatalf("role %v, want active", m.role)
	}
	if got := m.NextTxDeadline(); !got.Equal(clk.Now()) {
		t.Fatalf("active again: next TX %v, want now %v", got, clk.Now())
	}
}

// TestApplyRolePassiveOnUpKeepsTransmitting makes an Up session passive. It
// knows the remote discriminator, so RFC 5880 Section 6.8.7 still permits it
// to transmit, and it keeps its TX deadline.
// MUTATION: ApplyRole clearing the deadline for every passive session stops
// an Up session's packets, so this test goes red.
func TestApplyRolePassiveOnUpKeepsTransmitting(t *testing.T) {
	m := upMachine(t, newFakeClock())
	deadline := m.NextTxDeadline()

	m.ApplyRole(true)

	if m.role != RolePassive {
		t.Fatalf("role %v, want passive", m.role)
	}
	if got := m.NextTxDeadline(); !got.Equal(deadline) {
		t.Fatalf("next TX %v, want the unchanged %v", got, deadline)
	}
}

// TestApplyEchoReachesLiveSession gives an Up session whose peer loops echo
// back (Required Min Echo RX 50ms) an echo interval of 200ms, then 400ms
// after one echo, then none. The advertised Required Min Echo RX Interval and
// the echo cadence follow each edit, the pending echo moves to the new
// interval, and an echo interval of zero disables echo.
// MUTATION: ApplyEcho not writing bfd.DesiredMinEchoTxInterval leaves echo
// disabled, and not moving the pending echo keeps the 200ms deadline, so this
// test goes red.
func TestApplyEchoReachesLiveSession(t *testing.T) {
	clk := newFakeClock()
	m := upMachine(t, clk)
	m.vars.RemoteMinEchoRxInterval = 50_000

	m.ApplyEcho(200_000)
	if c := m.Build(); c.RequiredMinEchoRxInterval != 200_000 {
		t.Fatalf("advertised Required Min Echo RX %d, want 200000", c.RequiredMinEchoRxInterval)
	}
	if got := m.EchoInterval(); got != 200*time.Millisecond {
		t.Fatalf("echo interval %v, want 200ms", got)
	}

	m.PrimeEcho(clk.Now())
	m.AdvanceEcho(clk.Now())
	sent := clk.Now()
	m.ApplyEcho(400_000)
	if got := m.NextEchoTxDeadline(); !got.Equal(sent.Add(400 * time.Millisecond)) {
		t.Fatalf("next echo %v, want the last echo plus 400ms, %v", got, sent.Add(400*time.Millisecond))
	}

	m.ApplyEcho(0)
	if m.EchoEnabled() {
		t.Fatal("an echo interval of zero left echo enabled")
	}
	if c := m.Build(); c.RequiredMinEchoRxInterval != 0 {
		t.Fatalf("advertised Required Min Echo RX %d, want 0", c.RequiredMinEchoRxInterval)
	}
	if !m.NextEchoTxDeadline().IsZero() {
		t.Fatal("echo disabled, yet an echo is still scheduled")
	}
}
