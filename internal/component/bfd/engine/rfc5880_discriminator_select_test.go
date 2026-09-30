package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/core/clock"
)

// RFC requirement: RFC5880-6.8.6-7 positive -- with two sessions to one peer
// on two interfaces, a packet that arrives on the FIRST session's interface
// but carries the SECOND session's discriminator in Your Discriminator is
// delivered to the second session (its bfd.RemoteDiscr becomes the peer's My
// Discriminator) and leaves the first untouched; the mirror packet, on the
// second interface naming the first, is delivered to the first.
//
// VALIDATES: a nonzero Your Discriminator, not the arrival tuple, selects the
// session.
// PREVENTS: selection by source address or ingress interface when Your
// Discriminator is nonzero, which a loop of one session cannot tell apart.
func TestRFC5880YourDiscriminatorSelectsAmongSessions(t *testing.T) {
	l := NewLoop(&captureTransport{}, clock.RealClock{})
	firstReq := reqFor(addrB, addrA)
	firstReq.Interface = "eth0"
	secondReq := reqFor(addrB, addrA)
	secondReq.Interface = "eth1"
	if _, err := l.EnsureSession(firstReq); err != nil {
		t.Fatalf("EnsureSession on eth0: %v", err)
	}
	if _, err := l.EnsureSession(secondReq); err != nil {
		t.Fatalf("EnsureSession on eth1: %v", err)
	}
	first := machineFor(t, l, firstReq.Key())
	second := machineFor(t, l, secondReq.Key())
	if first == second {
		t.Fatal("precondition: the two interfaces must give two sessions")
	}

	l.handleInbound(rfc5880Inbound(firstReq.Peer, firstReq.Local, firstReq.Interface, second.LocalDiscriminator(), nil))
	l.mu.Lock()
	firstRemote, secondRemote := first.RemoteDiscriminator(), second.RemoteDiscriminator()
	l.mu.Unlock()
	if secondRemote != peerMyDiscr {
		t.Fatalf("packet naming the second session was not delivered to it: RemoteDiscr = %d, want %d", secondRemote, peerMyDiscr)
	}
	if firstRemote != 0 {
		t.Fatalf("packet naming the second session was delivered to the first, whose interface it arrived on: RemoteDiscr = %d", firstRemote)
	}

	l.handleInbound(rfc5880Inbound(secondReq.Peer, secondReq.Local, secondReq.Interface, first.LocalDiscriminator(), nil))
	l.mu.Lock()
	firstRemote = first.RemoteDiscriminator()
	l.mu.Unlock()
	if firstRemote != peerMyDiscr {
		t.Fatalf("packet naming the first session was not delivered to it: RemoteDiscr = %d, want %d", firstRemote, peerMyDiscr)
	}
}
