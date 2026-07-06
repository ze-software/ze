package engine

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/ike/transport"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/wire"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// VALIDATES: a packet for an ESTABLISHED SA is routed to the owning session's
// maintainSA inbound queue (spec-ipsec-13 Alt-A single-owner model) rather than
// handled on the shared dispatch goroutine.
// PREVENTS: inbound CREATE_CHILD_SA rekey messages bypassing the owner loop and
// racing childSA/SA state.
func TestInboundRekeyRoutedToOwner(t *testing.T) {
	log := slogutil.DiscardLogger()
	table := NewSATable()

	sa := testSA()
	sa.State = StateEstablished

	ps := &PeerSession{peerName: sa.PeerName, inbound: make(chan transport.Packet, inboundQueueDepth)}
	ps.established.Store(true) // maintainSA owns the SA
	setActivePeers(map[string]*PeerSession{sa.PeerName: ps})
	t.Cleanup(func() { setActivePeers(nil) })

	pkt := transport.Packet{Data: []byte("create-child-sa-request")}
	routeInbound(sa, pkt, table, nil, log)

	select {
	case got := <-ps.inbound:
		if !bytes.Equal(got.Data, pkt.Data) {
			t.Fatalf("owner received %q, want %q", got.Data, pkt.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("established-SA packet did not reach the owner inbound queue")
	}
}

// VALIDATES: a packet for a not-yet-established SA is handled inline (the owner
// loop is not consuming during the handshake), never queued to the owner.
// PREVENTS: handshake packets being silently parked on an idle owner channel.
func TestInboundPreEstablishedNotRoutedToOwner(t *testing.T) {
	log := slogutil.DiscardLogger()
	table := NewSATable()

	sa := testSA()
	sa.State = StateSAInitSent

	ps := &PeerSession{peerName: sa.PeerName, inbound: make(chan transport.Packet, inboundQueueDepth)}
	setActivePeers(map[string]*PeerSession{sa.PeerName: ps})
	t.Cleanup(func() { setActivePeers(nil) })

	// Too short to parse as an IKE message: handleInbound returns without action.
	pkt := transport.Packet{Data: make([]byte, 8)}
	routeInbound(sa, pkt, table, nil, log)

	select {
	case <-ps.inbound:
		t.Fatal("pre-established packet must not reach the owner inbound queue")
	default:
	}
}

// VALIDATES: routeInbound does not read sa.State (it uses the atomic ps.established
// flag), so it can run on the dispatch goroutine concurrently with owner-side
// sa.State writes without a data race. Run under `go test -race`.
// PREVENTS: the sa.State cross-goroutine race that the owner-loop DELETE handler
// introduced.
func TestRouteInboundNoStateRace(t *testing.T) {
	log := slogutil.DiscardLogger()
	table := NewSATable()
	sa := testSA()
	sa.State = StateEstablished

	ps := &PeerSession{peerName: sa.PeerName, inbound: make(chan transport.Packet, inboundQueueDepth)}
	ps.established.Store(true)
	setActivePeers(map[string]*PeerSession{sa.PeerName: ps})
	t.Cleanup(func() { setActivePeers(nil) })

	done := make(chan struct{})
	go func() { // drain the owner queue so routeInbound never blocks
		for {
			select {
			case <-ps.inbound:
			case <-done:
				return
			}
		}
	}()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { // owner goroutine writes sa.State (as handleDeletePayload does)
		defer wg.Done()
		for range 1000 {
			sa.State = StateDead
			sa.State = StateEstablished
		}
	}()
	go func() { // dispatch goroutine routes packets
		defer wg.Done()
		for range 1000 {
			routeInbound(sa, transport.Packet{Data: []byte("x")}, table, nil, log)
		}
	}()
	wg.Wait()
	close(done)
}

// VALIDATES: an authenticated INFORMATIONAL response that matches no pending
// exchange (a DPD/Delete ack) still reports peerAlive, so maintainSA clears the
// DPD wait instead of false-timing-out the peer.
// PREVENTS: DPD responses being dropped as out-of-window with awaitReply never cleared.
func TestOwnedInboundInformationalResponseIsLiveness(t *testing.T) {
	log := slogutil.DiscardLogger()
	sa := testSAWithGCMKeys(t)
	// Symmetric SK so a message sealed with SK_ei decrypts under the response key
	// SK_er, simulating the peer's encrypted INFORMATIONAL response.
	sa.SKKeys.SK_er = append([]byte(nil), sa.SKKeys.SK_ei...)

	resp, err := buildEncryptedMessageEx(sa, nil, 99, wire.ExchangeInformational, wire.FlagInitiator|wire.FlagResponse)
	if err != nil {
		t.Fatalf("build informational response: %v", err)
	}

	ps := &PeerSession{peerName: sa.PeerName}
	// msgID 99 matches no pending exchange (out of window): the response is reported
	// with its message ID so the caller can correlate it against the DPD probe.
	out := ps.handleOwnedInbound(sa, transport.Packet{Data: resp}, nil, nil, log)
	if !out.dpdResp || out.dpdRespMsgID != 99 {
		t.Errorf("authenticated INFORMATIONAL response: dpdResp=%v msgID=%d, want true/99", out.dpdResp, out.dpdRespMsgID)
	}
}

// VALIDATES: DPD liveness is credited only for the response that matches the
// outstanding probe's message ID; a replayed/out-of-window response ID is rejected.
// PREVENTS: an attacker replaying a captured INFORMATIONAL response to mask a dead
// peer from DPD (RFC 7296 §2.3 message-ID correlation).
func TestDPDMatchesProbeRejectsReplay(t *testing.T) {
	dpd := &dpdState{awaitReply: true, probeMsgID: 7}

	if !dpd.matchesProbe(7) {
		t.Error("the outstanding probe's message ID must match")
	}
	if dpd.matchesProbe(6) {
		t.Error("a non-matching (replayed/out-of-window) message ID must NOT match")
	}

	dpd.awaitReply = false // no probe outstanding
	if dpd.matchesProbe(7) {
		t.Error("no match when no probe is outstanding")
	}

	if (*dpdState)(nil).matchesProbe(7) {
		t.Error("nil dpdState must not match")
	}
}
