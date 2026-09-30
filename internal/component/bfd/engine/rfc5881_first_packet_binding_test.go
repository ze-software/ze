// VALIDATES: RFC 5881 Section 3, the zero-Your-Discriminator association, with
// the interface and the protocol varied independently of the remote system.
// One Loop holds three single-hop sessions: the IPv4 peer on "loop", the SAME
// IPv4 peer on "loop2", and an IPv6 peer on "loop2". A first packet from the
// IPv4 peer arriving on "loop2" must reach the IPv4 session on "loop2" and no
// other.
// PREVENTS: a first packet associated by remote address alone, which would
// select the session on another interface or of another protocol.
package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/core/clock"
)

// RFC requirement: RFC5881-3-2 positive -- a Your Discriminator zero packet
// from the IPv4 peer that arrived on "loop2" is associated with the session
// bound to that peer, "loop2" and IPv4: its Remote Discriminator becomes the
// packet's My Discriminator.
// RFC requirement: RFC5881-3-2 negative -- the same packet is not associated
// with the session bound to the same peer on "loop" (another interface), nor
// with the IPv6 session on "loop2" (another protocol): both keep Remote
// Discriminator zero.
func TestRFC5881FirstPacketBoundToRemoteInterfaceAndProtocol(t *testing.T) {
	l := NewLoop(&captureTransport{}, clock.RealClock{})
	onLoop := reqFor(addrB, addrA)
	onLoop2 := reqFor(addrB, addrA)
	onLoop2.Interface = "loop2"
	v6 := reqFor("2001:db8::2", "2001:db8::1")
	v6.Interface = "loop2"
	if _, err := l.EnsureSession(onLoop); err != nil {
		t.Fatalf("EnsureSession loop: %v", err)
	}
	if _, err := l.EnsureSession(onLoop2); err != nil {
		t.Fatalf("EnsureSession loop2: %v", err)
	}
	if _, err := l.EnsureSession(v6); err != nil {
		t.Fatalf("EnsureSession loop2 v6: %v", err)
	}
	mLoop := machineFor(t, l, onLoop.Key())
	mLoop2 := machineFor(t, l, onLoop2.Key())
	mV6 := machineFor(t, l, v6.Key())

	l.handleInbound(inboundControl(onLoop2.Peer, onLoop2.Local, "loop2", 0))

	if got := mLoop2.RemoteDiscriminator(); got != peerMyDiscr {
		t.Fatalf("the session bound to (%s, loop2, IPv4) was not selected: RemoteDiscriminator = %d, want %d", onLoop2.Peer, got, peerMyDiscr)
	}
	if got := mLoop.RemoteDiscriminator(); got != 0 {
		t.Fatalf("the session on interface loop was selected by a packet that arrived on loop2: RemoteDiscriminator = %d", got)
	}
	if got := mV6.RemoteDiscriminator(); got != 0 {
		t.Fatalf("the IPv6 session on loop2 was selected by an IPv4 packet: RemoteDiscriminator = %d", got)
	}
}
