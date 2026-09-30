// VALIDATES: RFC 5881 Section 5 through the engine's receive path: a
// single-hop Control packet that handleInbound demultiplexes to a session,
// by the first-packet tuple or by Your Discriminator, reaches the session's
// state machine only when its TTL is 255.
// PREVENTS: a TTL predicate that is correct in isolation while handleInbound
// no longer consults it, or consults it on one demultiplexing path only.
package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/packet"
)

// rfc5881InboundTTL delivers a peer Control packet in state with Your
// Discriminator yd, arriving with the given TTL.
func rfc5881InboundTTL(l *Loop, key api.Key, yd uint32, state packet.State, ttl uint8) {
	in := rfc5880Inbound(key.Peer, key.Local, key.Interface, yd, func(c *packet.Control) { c.State = state })
	in.TTL = ttl
	l.handleInbound(in)
}

// RFC requirement: RFC5881-5-2 positive -- a single-hop packet with TTL 255 is
// not discarded on either demultiplexing path: the first packet (Your
// Discriminator 0, peer Down) installs bfd.RemoteDiscr and moves the session to
// Init, and the next packet, demultiplexed by Your Discriminator with the peer
// in Init, moves it to Up.
func TestRFC5881TTL255ReachesTheSessionOnBothDemuxPaths(t *testing.T) {
	l, _, key := newSingleHopLoop(t)
	m := machineFor(t, l, key)

	rfc5881InboundTTL(l, key, 0, packet.StateDown, 255)
	if got := m.RemoteDiscriminator(); got != peerMyDiscr {
		t.Fatalf("bfd.RemoteDiscr after a TTL 255 first packet = %d, want %d", got, peerMyDiscr)
	}
	if got := m.State(); got != packet.StateInit {
		t.Fatalf("state after a TTL 255 first packet = %s, want Init", got)
	}
	rfc5881InboundTTL(l, key, m.LocalDiscriminator(), packet.StateInit, 255)
	if got := m.State(); got != packet.StateUp {
		t.Fatalf("state after a TTL 255 packet demultiplexed by Your Discriminator = %s, want Up", got)
	}
}

// RFC requirement: RFC5881-5-2 negative -- a single-hop packet whose TTL is
// not 255 is discarded on both demultiplexing paths. First packets (Your
// Discriminator 0, peer Down) with TTL 254, 128, 1 and 0 leave bfd.RemoteDiscr
// at 0 and the session Down. After a TTL 255 first packet brings the session
// to Init, a packet demultiplexed by Your Discriminator with the peer in Init
// and TTL 254 leaves the session in Init, where TTL 255 would move it to Up.
func TestRFC5881TTLNot255DiscardedOnBothDemuxPaths(t *testing.T) {
	l, _, key := newSingleHopLoop(t)
	m := machineFor(t, l, key)

	for _, ttl := range []uint8{254, 128, 1, 0} {
		rfc5881InboundTTL(l, key, 0, packet.StateDown, ttl)
		if got := m.RemoteDiscriminator(); got != 0 {
			t.Fatalf("TTL %d first packet installed bfd.RemoteDiscr %d; it must be discarded", ttl, got)
		}
		if got := m.State(); got != packet.StateDown {
			t.Fatalf("TTL %d first packet moved the session to %s; it must be discarded", ttl, got)
		}
	}

	rfc5881InboundTTL(l, key, 0, packet.StateDown, 255)
	if got := m.State(); got != packet.StateInit {
		t.Fatalf("precondition: state after a TTL 255 first packet = %s, want Init", got)
	}
	rfc5881InboundTTL(l, key, m.LocalDiscriminator(), packet.StateInit, 254)
	if got := m.State(); got != packet.StateInit {
		t.Fatalf("TTL 254 packet demultiplexed by Your Discriminator moved the session to %s; it must be discarded", got)
	}
}
