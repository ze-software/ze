package engine

import "testing"

// RFC requirement: RFC5880-9-2 positive -- the receive half of "checked to be
// equal to the maximum value on reception": a single-hop Control packet whose
// TTL (or IPv6 Hop Limit, carried in the same field) is 255 passes the check
// and is delivered, so the session's bfd.RemoteDiscr becomes the peer's My
// Discriminator.
//
// VALIDATES: the receive TTL gate admits the maximum value.
// PREVENTS: a gate that discards every single-hop packet, which the
// not-maximum negative TestRFC5880SingleHopReceiveTTLNotMaxDiscarded alone
// would pass.
func TestRFC5880SingleHopReceiveTTLMaxAccepted(t *testing.T) {
	l, _, key := newSingleHopLoop(t)
	m := machineFor(t, l, key)

	in := rfc5880Inbound(key.Peer, key.Local, key.Interface, m.LocalDiscriminator(), nil)
	in.TTL = 255
	l.handleInbound(in)

	l.mu.Lock()
	got := m.RemoteDiscriminator()
	l.mu.Unlock()
	if got != peerMyDiscr {
		t.Fatalf("TTL 255 packet not delivered: RemoteDiscr = %d, want %d", got, peerMyDiscr)
	}
}
