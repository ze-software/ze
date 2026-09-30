// VALIDATES: RFC 5881 Section 6, "subsequent BFD packets MUST be
// demultiplexed solely by the Your Discriminator field", over every other
// field handleInbound sees: the source address, the local address and the
// ingress interface.
// PREVENTS: a Your Discriminator lookup that also requires the packet's
// addressing or its link to match the session, and one that lets the
// addressing pick the session when the discriminator names another.
package engine

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/core/clock"
)

// rfc5881TwoSessionLoop builds an unstarted Loop holding two single-hop
// sessions: one for addrB on "loop" and one for 203.0.113.9 on "loop2".
func rfc5881TwoSessionLoop(t *testing.T) (*Loop, api.Key, api.Key) {
	t.Helper()
	l := NewLoop(&captureTransport{}, clock.RealClock{})
	first := reqFor(addrB, addrA)
	second := reqFor("203.0.113.9", addrA)
	second.Interface = "loop2"
	for _, req := range []api.SessionRequest{first, second} {
		if _, err := l.EnsureSession(req); err != nil {
			t.Fatalf("EnsureSession %s: %v", req.Peer, err)
		}
	}
	return l, first.Key(), second.Key()
}

// RFC requirement: RFC5881-6-5 positive -- a packet carrying the session's
// Your Discriminator is delivered to that session when every other field
// differs from it: another source address (198.51.100.7), another local
// address (192.0.2.77) and another ingress interface ("eth9").
func TestRFC5881DiscriminatorAloneSelectsTheSession(t *testing.T) {
	l, _, key := newSingleHopLoop(t)
	m := machineFor(t, l, key)

	source := netip.MustParseAddr("198.51.100.7")
	local := netip.MustParseAddr("192.0.2.77")
	l.handleInbound(inboundControl(source, local, "eth9", m.LocalDiscriminator()))

	if got := m.RemoteDiscriminator(); got != peerMyDiscr {
		t.Fatalf("packet with the session's Your Discriminator but other source, local address and interface was not delivered: RemoteDiscriminator = %d, want %d", got, peerMyDiscr)
	}
}

// RFC requirement: RFC5881-6-5 negative -- the addressing does not take part:
// a packet whose source, local address and interface are exactly the first
// session's, but whose Your Discriminator is the second session's, is
// delivered to the second session and leaves the first untouched.
func TestRFC5881AddressingNeverOverridesTheDiscriminator(t *testing.T) {
	l, firstKey, secondKey := rfc5881TwoSessionLoop(t)
	first := machineFor(t, l, firstKey)
	second := machineFor(t, l, secondKey)

	l.handleInbound(inboundControl(firstKey.Peer, firstKey.Local, firstKey.Interface, second.LocalDiscriminator()))

	if got := first.RemoteDiscriminator(); got != 0 {
		t.Fatalf("the session whose addressing the packet carried took it: RemoteDiscriminator = %d, want 0", got)
	}
	if got := second.RemoteDiscriminator(); got != peerMyDiscr {
		t.Fatalf("the session the Your Discriminator names did not get the packet: RemoteDiscriminator = %d, want %d", got, peerMyDiscr)
	}
}
