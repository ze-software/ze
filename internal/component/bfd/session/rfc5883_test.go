// VALIDATES: RFC 5883 sec 4.3 unidirectional-link roles. A default session
// takes the Active role and arms its periodic TX timer immediately, while a
// Passive session (the unidirectional receiver) transmits nothing until it
// has received a Control packet from the peer, then begins transmitting.
// PREVENTS: a Passive receiver leaking Control packets before it learns the
// peer's discriminator, and an Active sender that fails to start periodic
// transmission from session creation.
package session

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/bfd/api"
	"codeberg.org/thomas-mangin/ze/internal/component/bfd/packet"
	"codeberg.org/thomas-mangin/ze/internal/core/clock"
)

// newPassiveMachine builds a session whose SessionRequest carries Passive=true,
// so Init (internal/component/bfd/session/session.go:225-227) selects
// RolePassive. Mirrors newMachine in session_test.go but flips the one field
// under test.
func newPassiveMachine(t *testing.T, clk clock.Clock) *Machine {
	t.Helper()
	req := api.SessionRequest{
		Peer:                  netip.MustParseAddr("192.0.2.2"),
		Local:                 netip.MustParseAddr("192.0.2.1"),
		Mode:                  api.MultiHop,
		DesiredMinTxInterval:  300_000,
		RequiredMinRxInterval: 300_000,
		DetectMult:            3,
		Passive:               true,
	}
	m := &Machine{}
	m.Init(req, 0xC0FFEE, clk, nil)
	return m
}

// RFC requirement: RFC5883-4.3-1 positive -- a system MUST take the Active
// role and transmit periodically. Init (internal/component/bfd/session/session.go:224)
// defaults m.role to RoleActive and (session.go:262-263) arms nextTxAt to
// createdAt, so NextTxDeadline is non-zero the moment the session exists.
func TestRFC5883DefaultSessionActiveArmsTx(t *testing.T) {
	m, _ := newMachine(t, newFakeClock())
	if m.role != RoleActive {
		t.Fatalf("default role = %v, want RoleActive (RFC 5883 sec 4.3)", m.role)
	}
	if m.NextTxDeadline().IsZero() {
		t.Fatalf("Active session did not arm its TX timer; NextTxDeadline is zero")
	}
}

// RFC requirement: RFC5883-4.3-1 negative -- arming TX is conditional on the
// role, not unconditional: a Passive session (session.go:225-227) does NOT
// arm nextTxAt at Init (the `if m.role == RoleActive` guard at session.go:262
// is not taken), so NextTxDeadline stays zero. Without this contrast the
// positive test could pass on code that always arms TX.
func TestRFC5883PassiveSessionDoesNotArmTx(t *testing.T) {
	m := newPassiveMachine(t, newFakeClock())
	if m.role != RolePassive {
		t.Fatalf("passive-configured role = %v, want RolePassive", m.role)
	}
	if !m.NextTxDeadline().IsZero() {
		t.Fatalf("Passive session armed TX at Init (NextTxDeadline=%v); it must stay silent until first RX", m.NextTxDeadline())
	}
}

// RFC requirement: RFC5883-4.3-2 positive -- a Passive system MUST NOT send
// until it has received a packet. Init (session.go:212-213,225-227) leaves
// nextTxAt zero for RolePassive, so before any Receive the session schedules
// no periodic TX (NextTxDeadline is zero).
func TestRFC5883PassiveSessionSilentUntilRx(t *testing.T) {
	m := newPassiveMachine(t, newFakeClock())
	if !m.NextTxDeadline().IsZero() {
		t.Fatalf("Passive session scheduled TX before receiving any packet: NextTxDeadline=%v", m.NextTxDeadline())
	}
}

// RFC requirement: RFC5883-4.3-2 negative -- the silence is lifted by the
// first received packet, not permanent. Receiving a Control packet drives the
// FSM out of Down; onStateChange (internal/component/bfd/session/fsm.go:172)
// sets nextTxAt = now, so the Passive session begins transmitting. A blanket
// "never transmit" implementation would fail this.
func TestRFC5883PassiveSessionTransmitsAfterRx(t *testing.T) {
	clk := newFakeClock()
	m := newPassiveMachine(t, clk)
	if !m.NextTxDeadline().IsZero() {
		t.Fatalf("precondition: Passive session must be silent before RX, got NextTxDeadline=%v", m.NextTxDeadline())
	}
	if err := m.Receive(recv(packet.StateDown, 0)); err != nil {
		t.Fatalf("Receive first packet: %v", err)
	}
	if m.NextTxDeadline().IsZero() {
		t.Fatalf("Passive session did not begin transmitting after first RX; NextTxDeadline still zero")
	}
}
