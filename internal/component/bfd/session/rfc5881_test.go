// VALIDATES: RFC 5881 sec 3 single-hop role. A single-hop session takes the
// Active role by default and arms its periodic TX timer at creation, so both
// endpoints transmit initial Control packets (with Your Discriminator = 0)
// without waiting to receive one first.
// PREVENTS: a single-hop endpoint that stays silent at creation (never taking
// the Active role), which would stall the symmetric single-hop handshake.
package session

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/bfd/api"
	"codeberg.org/thomas-mangin/ze/internal/core/clock"
)

// newSingleHopMachine builds a single-hop session with the requested passive
// flag. Passive=false is the RFC 5881 sec 3 Active default; Passive=true is
// used only to prove the role field genuinely drives TX arming.
func newSingleHopMachine(t *testing.T, clk clock.Clock, passive bool) *Machine {
	t.Helper()
	req := api.SessionRequest{
		Peer:                  netip.MustParseAddr("192.0.2.2"),
		Local:                 netip.MustParseAddr("192.0.2.1"),
		Interface:             "eth0",
		Mode:                  api.SingleHop,
		DesiredMinTxInterval:  300_000,
		RequiredMinRxInterval: 300_000,
		DetectMult:            3,
		Passive:               passive,
	}
	m := &Machine{}
	m.Init(req, 0xBEEF, clk, nil)
	return m
}

// RFC requirement: RFC5881-3-1 positive -- both sides of a single-hop session
// MUST take the Active role. Init (internal/component/bfd/session/session.go:224)
// defaults m.role to RoleActive for a single-hop request, and (session.go:262-263)
// arms nextTxAt to createdAt, so the session transmits its initial Control
// packet immediately rather than waiting for the peer.
func TestRFC5881SingleHopTakesActiveRole(t *testing.T) {
	m := newSingleHopMachine(t, newFakeClock(), false)
	if m.role != RoleActive {
		t.Fatalf("single-hop default role = %v, want RoleActive (RFC 5881 sec 3)", m.role)
	}
	if m.NextTxDeadline().IsZero() {
		t.Fatal("Active single-hop session did not arm its TX timer; NextTxDeadline is zero")
	}
}

// RFC requirement: RFC5881-3-1 negative -- taking the Active role is what makes
// the session transmit at creation; it is not unconditional. A Passive session
// (session.go:225-227) does NOT arm nextTxAt at Init (the `if m.role ==
// RoleActive` guard at session.go:262 is skipped), so NextTxDeadline stays zero.
// Without this contrast the positive test could pass on code that always arms
// TX regardless of role.
func TestRFC5881NonActiveStaysSilent(t *testing.T) {
	m := newSingleHopMachine(t, newFakeClock(), true)
	if m.role != RolePassive {
		t.Fatalf("passive-configured role = %v, want RolePassive", m.role)
	}
	if !m.NextTxDeadline().IsZero() {
		t.Fatalf("non-Active session armed TX at Init (NextTxDeadline=%v); only the Active role transmits at creation", m.NextTxDeadline())
	}
}
