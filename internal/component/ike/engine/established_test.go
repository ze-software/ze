package engine

import (
	"bytes"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/ike/dataplane"
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
	ps.ownedSA.Store(sa) // maintainSA owns this exact SA (routeInbound keys on identity)
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

// VALIDATES: routeInbound does not read sa.State (it uses the atomic ps.ownedSA
// pointer), so it can run on the dispatch goroutine concurrently with owner-side
// sa.State writes without a data race. Run under `go test -race`.
// PREVENTS: the sa.State cross-goroutine race that the owner-loop DELETE handler
// introduced.
func TestRouteInboundNoStateRace(t *testing.T) {
	log := slogutil.DiscardLogger()
	table := NewSATable()
	sa := testSA()
	sa.State = StateEstablished

	ps := &PeerSession{peerName: sa.PeerName, inbound: make(chan transport.Packet, inboundQueueDepth)}
	ps.ownedSA.Store(sa)
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

// runStopCase drives maintainSA once through its stopCh path against a loopback peer
// and returns the raw bytes the peer received (or nil within the window). It uses the
// ze.test.ike.port seam so sa.remoteUDPAddr() dials the loopback receiver.
func runStopCase(t *testing.T, graceful bool) (peerGot []byte, ini *SA) {
	t.Helper()
	log := slogutil.DiscardLogger()
	var resp *SA
	ini, resp, ps := establishPSK(t)

	// Loopback receiver standing in for the peer, plus our own sender socket.
	peerTr, err := transport.NewUDPTransport("127.0.0.1:0", log)
	if err != nil {
		t.Fatalf("peer transport: %v", err)
	}
	t.Cleanup(func() { _ = peerTr.Close() })
	go peerTr.Run()
	myTr, err := transport.NewUDPTransport("127.0.0.1:0", log)
	if err != nil {
		t.Fatalf("sender transport: %v", err)
	}
	t.Cleanup(func() { _ = myTr.Close() })

	addr, ok := peerTr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("peer transport local address is not *net.UDPAddr")
	}
	port := addr.Port
	oldPortFn := ikeTestPortFn
	ikeTestPortFn = func() string { return strconv.Itoa(port) }
	t.Cleanup(func() { ikeTestPortFn = oldPortFn })
	resp.PeerCfg.RemoteAddress = "127.0.0.1"

	ps.stopCh = make(chan struct{})
	ps.supersede = make(chan struct{}, 1)
	if graceful {
		ps.graceful.Store(true)
	}
	close(ps.stopCh) // the very first select takes the stop path

	finished := make(chan struct{})
	go func() {
		_ = ps.maintainSA(resp, nil, newLifetimeState(3600), newLifetimeState(3600),
			testIKEGroup(), NewSATable(), dataplane.Get(), myTr, nil, log)
		close(finished)
	}()

	var got []byte
	select {
	case pkt := <-peerTr.Recv():
		got = pkt.Data
	case <-time.After(2 * time.Second):
	}
	// Wait for maintainSA to return before the test's Cleanup restores the
	// ikeTestPortFn global, or the goroutine's ikeAddr read races that restore.
	<-finished
	return got, ini
}

// VALIDATES: Phase A. On an operator clear (graceful stop) the owner loop sends an
// authenticated INFORMATIONAL Delete for the IKE SA (RFC 7296 Section 1.4) so the peer
// tears down at once; a plain config-change Stop sends nothing. The Delete is decrypted
// with the peer's SA to prove it is well-formed and carries an IKE Delete payload.
func TestClearSendsIKEDelete(t *testing.T) {
	// Graceful: a Delete arrives and decrypts to an IKE Delete payload.
	got, ini := runStopCase(t, true)
	if got == nil {
		t.Fatal("graceful clear did not send an INFORMATIONAL Delete")
	}
	msg := parseMsg(t, got)
	if msg.Header.ExchangeType != wire.ExchangeInformational {
		t.Fatalf("sent exchange = %d, want INFORMATIONAL", msg.Header.ExchangeType)
	}
	inner, err := decryptAndParse(ini, msg, got)
	if err != nil {
		t.Fatalf("peer could not decrypt the Delete: %v", err)
	}
	var sawIKEDelete bool
	for i := range inner {
		if del, ok := inner[i].Payload.(*wire.PayloadDelete); ok && del.ProtocolID == wire.ProtocolIKE {
			sawIKEDelete = true
		}
	}
	if !sawIKEDelete {
		t.Error("graceful clear message carried no IKE Delete payload")
	}
}

// VALIDATES: Phase A negative. A plain (non-graceful) Stop -- a config change -- stays
// silent: it must not start emitting Deletes (R-6, keep Stop()'s meaning distinct).
func TestPlainStopSendsNoDelete(t *testing.T) {
	got, _ := runStopCase(t, false)
	if got != nil {
		t.Errorf("a plain config-change Stop wrongly sent %d bytes to the peer", len(got))
	}
}

// VALIDATES: review fix (concurrency ISSUE). reapStalePending must NOT reap a parallel
// SA that authenticated right at the timeout boundary (State==StateEstablished): doing
// so would destroy the freshly installed make-before-break child and, with the supersede
// token still buffered, tear the old SA down too with nothing to promote. A genuinely
// abandoned half-open pending past the timeout IS reaped. Mirrors reapStaleHandshake.
func TestReapStalePendingSkipsEstablished(t *testing.T) {
	log := slogutil.DiscardLogger()
	table := NewSATable()
	ps := &PeerSession{peerName: "ze"}
	ps.responderBusy.Store(true)

	pending := testSA()
	pending.IsInitiator = false
	pending.InitiatorSPI = [8]byte{1, 1, 1, 1, 1, 1, 1, 1}
	pending.ResponderSPI = [8]byte{2, 2, 2, 2, 2, 2, 2, 2}
	pending.CreatedAt = time.Now().Add(-2 * responderHandshakeTimeout) // stale timestamp
	table.Insert(pending)
	ps.setPendingSA(pending)
	ps.setPendingChild(&ChildSA{})

	// Authenticated at the boundary: must NOT be reaped.
	pending.State = StateEstablished
	ps.reapStalePending(time.Now(), table, nil, log)
	if ps.getPendingSA() == nil || ps.getPendingChild() == nil {
		t.Fatal("an established pending was wrongly reaped (child/SA destroyed)")
	}
	if table.Len() != 1 || !ps.responderBusy.Load() {
		t.Error("established pending state disturbed by reap")
	}

	// Genuinely abandoned half-open past the timeout: reaped, gate freed.
	pending.State = StateSAInitReceived
	ps.reapStalePending(time.Now(), table, nil, log)
	if ps.getPendingSA() != nil {
		t.Error("abandoned half-open pending was not reaped")
	}
	if ps.responderBusy.Load() {
		t.Error("responderBusy not cleared after reaping -- a future re-init would wedge")
	}
	if table.Len() != 0 {
		t.Errorf("table has %d entries after reap, want 0", table.Len())
	}
}

// VALIDATES: review fix (correctness ISSUE -- dataplane Child SA leak). The
// operator-clear + parallel-authenticated race: maintainSA takes stopCh over the
// supersede case, so the session is stopping while a parallel SA authenticated (pendingSA
// established, pendingChild installed). resolvePendingAfterOwnerLoop MUST free the pending
// child (no leak) and MUST NOT promote it into childSA (which the stop path never cleans).
func TestResolvePendingCleansPromotedChildOnStop(t *testing.T) {
	log := slogutil.DiscardLogger()
	table := NewSATable()
	ps := &PeerSession{peerName: "ze", stopCh: make(chan struct{})}
	ps.responderBusy.Store(true)

	pending := testSA()
	pending.State = StateEstablished // authenticated in the race
	pending.InitiatorSPI = [8]byte{5, 5, 5, 5, 5, 5, 5, 5}
	pending.ResponderSPI = [8]byte{6, 6, 6, 6, 6, 6, 6, 6}
	table.Insert(pending)
	ps.setPendingSA(pending)
	ps.setPendingChild(&ChildSA{})

	close(ps.stopCh) // the session is stopping (operator clear / config change)

	if got := ps.resolvePendingAfterOwnerLoop(table, nil, nil, log); got != pendingReturn {
		t.Fatalf("stop path returned %v, want pendingReturn (must not promote)", got)
	}
	if ps.getChildSA() != nil {
		t.Error("a parallel Child SA was promoted into childSA on stop -- it would leak (stop path never cleans childSA)")
	}
	if ps.getPendingChild() != nil {
		t.Error("pending Child SA not freed on stop -- DATAPLANE LEAK")
	}
	if ps.getPendingSA() != nil || table.Len() != 0 {
		t.Error("pending SA / SATable entry not freed on stop")
	}
}

// VALIDATES: AC-3 (supersede) promotion. Not stopping + an authenticated parallel SA:
// resolvePendingAfterOwnerLoop promotes it into the primary slot (pendingSA -> sa,
// pendingChild -> childSA) so the poll loop adopts it and it is not orphaned.
func TestResolvePendingPromotesOnSupersede(t *testing.T) {
	log := slogutil.DiscardLogger()
	table := NewSATable()
	ps := &PeerSession{peerName: "ze", stopCh: make(chan struct{})} // open: NOT stopping
	ps.responderBusy.Store(true)

	pending := testSA()
	pending.State = StateEstablished
	pending.InitiatorSPI = [8]byte{7, 7, 7, 7, 7, 7, 7, 7}
	pending.ResponderSPI = [8]byte{8, 8, 8, 8, 8, 8, 8, 8}
	child := &ChildSA{}
	table.Insert(pending)
	ps.setPendingSA(pending)
	ps.setPendingChild(child)

	if got := ps.resolvePendingAfterOwnerLoop(table, nil, nil, log); got != pendingContinue {
		t.Fatalf("supersede path returned %v, want pendingContinue", got)
	}
	if ps.getSA() != pending {
		t.Error("authenticated parallel SA not promoted to the primary slot")
	}
	if ps.getChildSA() != child {
		t.Error("parallel Child SA not moved to childSA on promotion")
	}
	if ps.getPendingSA() != nil || ps.getPendingChild() != nil {
		t.Error("second slot not cleared after promotion")
	}
}

// VALIDATES: review fix (leak ISSUE). cleanupPendingSA frees BOTH second slots (the
// SATable entry for pendingSA and the make-before-break pendingChild) so a session torn
// down (operator clear, config change, engine shutdown) while a parallel handshake is in
// flight does not leak the second-slot SA/child.
func TestCleanupPendingSA(t *testing.T) {
	log := slogutil.DiscardLogger()
	table := NewSATable()
	ps := &PeerSession{peerName: "ze"}
	ps.responderBusy.Store(true)

	pending := testSA()
	pending.InitiatorSPI = [8]byte{3, 3, 3, 3, 3, 3, 3, 3}
	pending.ResponderSPI = [8]byte{4, 4, 4, 4, 4, 4, 4, 4}
	table.Insert(pending)
	ps.setPendingSA(pending)
	ps.setPendingChild(&ChildSA{})

	ps.cleanupPendingSA(table, nil, nil, log)

	if ps.getPendingSA() != nil {
		t.Error("pendingSA not cleared")
	}
	if ps.getPendingChild() != nil {
		t.Error("pendingChild not cleared")
	}
	if table.Len() != 0 {
		t.Errorf("pending SATable entry not removed: %d left", table.Len())
	}
}

// VALIDATES: AC-3 (supersede) owner-loop half. When a parallel IKE_SA_INIT
// authenticated, finishResponderEstablish signals ps.supersede; maintainSA must
// relinquish the old SA -- returning without error and removing ONLY the old Child SA
// (the new one is already staged in pendingChild, make-before-break, R-2).
func TestMaintainSARelinquishesOnSupersede(t *testing.T) {
	log := slogutil.DiscardLogger()
	_, old, ps := establishPSK(t)
	if ps.getChildSA() == nil {
		t.Fatal("setup: established responder has no child SA")
	}
	ps.stopCh = make(chan struct{})
	ps.supersede = make(chan struct{}, 1)
	ps.supersede <- struct{}{} // a parallel SA authenticated

	done := make(chan error, 1)
	go func() {
		done <- ps.maintainSA(old, nil, newLifetimeState(3600), newLifetimeState(3600),
			testIKEGroup(), NewSATable(), dataplane.Get(), nil, nil, log)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("maintainSA returned %v on supersede, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("maintainSA did not relinquish the owner loop on the supersede signal")
	}
	if ps.getChildSA() != nil {
		t.Error("old Child SA not cleaned up after supersede")
	}
}
