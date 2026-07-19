package engine

import (
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/ike/transport"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// VALIDATES: R-5 / coupling #1. routeInbound keys the owner-loop hand-off on SA
// identity (ps.ownedSA == packet's SA), not the peer name. A packet for a DIFFERENT
// SA of the same peer -- the parallel half-open re-init -- is NOT delivered to the
// established SA's owner loop (where it would decrypt under the wrong keys); it is
// handled inline instead. RFC 7296 Section 2.4.
// PREVENTS: the naive one-line responderBusy fix, which would misroute the accepted
// re-init to the old SA and silently fail to decrypt.
func TestRouteInboundKeysOnSANotPeer(t *testing.T) {
	log := slogutil.DiscardLogger()
	table := NewSATable()

	owned := testSA()
	owned.PeerName = "ze"
	owned.State = StateEstablished
	other := testSA() // a distinct SA of the SAME peer (the parallel handshake)
	other.PeerName = "ze"
	other.State = StateSAInitReceived

	ps := &PeerSession{peerName: "ze", inbound: make(chan transport.Packet, inboundQueueDepth)}
	ps.ownedSA.Store(owned)
	setActivePeers(map[string]*PeerSession{"ze": ps})
	t.Cleanup(func() { setActivePeers(nil) })

	// A packet for the parallel SA must NOT reach the owner loop. Short data makes the
	// inline handleInbound return without side effects; we only assert the routing.
	routeInbound(other, transport.Packet{Data: make([]byte, 8)}, table, nil, log)
	select {
	case <-ps.inbound:
		t.Fatal("a parallel-SA packet was wrongly delivered to the established SA's owner loop")
	default:
	}

	// A packet for the owned SA does reach the owner loop.
	routeInbound(owned, transport.Packet{Data: []byte("owned-sa-packet")}, table, nil, log)
	select {
	case <-ps.inbound:
	case <-time.After(time.Second):
		t.Fatal("the owned SA's packet did not reach the owner inbound queue")
	}
}
