// VALIDATES: RFC 5880 Section 6.1 roles on the engine's transmit path: the
// timer tick, which turns a session's transmit deadline into a packet handed to
// the transport, sends for an Active session whatever it has received, and
// sends nothing for a Passive session until its peer has spoken.
// PREVENTS: a transmit gate in the tick (on reception, on the remote
// discriminator, or none at all for Passive) that the session-level
// NextTxDeadline tests cannot see.
// Method: an unstarted Loop over a transport that counts Sends, a stepped
// clock, tick every 10 ms, and handleInbound for the peer's packet.
package engine

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/transport"
)

// countingTransport counts the Control packets the loop hands to Send. The
// loop is never started, so RX is never read.
type countingTransport struct{ sent int }

func (*countingTransport) Start() error                    { return nil }
func (*countingTransport) Stop() error                     { return nil }
func (c *countingTransport) Send(transport.Outbound) error { c.sent++; return nil }
func (*countingTransport) RX() <-chan transport.Inbound    { return nil }

// roleTick is the tick step every test here uses.
const roleTick = 10 * time.Millisecond

// roleLoop builds an unstarted Loop over a counting transport and a stepped
// clock, with one single-hop session to addrB on "loop" in the given role.
func roleLoop(t *testing.T, passive bool) (*Loop, *countingTransport, *steppedClock) {
	t.Helper()
	ct := &countingTransport{}
	clk := &steppedClock{now: time.Unix(1_000_000, 0)}
	l := NewLoop(ct, clk)
	req := reqFor(addrB, addrA)
	req.Passive = passive
	if _, err := l.EnsureSession(req); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	return l, ct, clk
}

// peerDown is a first packet from the peer: State Down, Your Discriminator 0.
func peerDown() transport.Inbound {
	return inboundControl(netip.MustParseAddr(addrB), netip.MustParseAddr(addrA), "loop", 0)
}

// RFC requirement: RFC5880-6.1-1 positive -- an Active session that has
// received nothing sends Control packets: the engine's timer tick hands the
// transport packets for it within two seconds.
func TestRFC5880ActiveSessionSendsBeforeAnyReception(t *testing.T) {
	l, ct, clk := roleLoop(t, false)
	tickFor(l, clk, 2*time.Second, roleTick)
	if ct.sent == 0 {
		t.Fatalf("an Active session sent nothing in 2s without reception")
	}
}

// RFC requirement: RFC5880-6.1-1 negative -- an Active session whose peer
// spoke once and then fell silent for far longer than the Detection Time keeps
// sending: nothing the session has or has not received gates its transmission.
func TestRFC5880ActiveSessionSendsAfterThePeerFallsSilent(t *testing.T) {
	l, ct, clk := roleLoop(t, false)
	l.handleInbound(peerDown())
	tickFor(l, clk, 5*time.Second, roleTick)
	before := ct.sent
	tickFor(l, clk, 3*time.Second, roleTick)
	if ct.sent == before {
		t.Fatalf("an Active session stopped sending once its peer fell silent (%d packets before, none in the next 3s)", before)
	}
}

// RFC requirement: RFC5880-6.1-2 positive -- a Passive session that has
// received nothing sends no Control packet: five seconds of timer ticks hand
// the transport none.
func TestRFC5880PassiveSessionSilentThroughTicksUntilReception(t *testing.T) {
	l, ct, clk := roleLoop(t, true)
	tickFor(l, clk, 5*time.Second, roleTick)
	if ct.sent != 0 {
		t.Fatalf("a Passive session sent %d packets before receiving any", ct.sent)
	}
}

// RFC requirement: RFC5880-6.1-2 negative -- the same Passive session, once a
// packet from its peer arrives through handleInbound, sends: the silence is
// lifted by reception and by nothing else.
func TestRFC5880PassiveSessionSendsOnceThePeerSpoke(t *testing.T) {
	l, ct, clk := roleLoop(t, true)
	tickFor(l, clk, 2*time.Second, roleTick)
	if ct.sent != 0 {
		t.Fatalf("precondition: a Passive session sent %d packets before reception", ct.sent)
	}
	l.handleInbound(peerDown())
	tickFor(l, clk, 2*time.Second, roleTick)
	if ct.sent == 0 {
		t.Fatalf("a Passive session sent nothing after its peer's first packet")
	}
}
