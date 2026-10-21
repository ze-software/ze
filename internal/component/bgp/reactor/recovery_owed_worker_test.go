// Design: docs/architecture/update-building.md -- owed keys and the recovery worker
// Related: recovery_owed.go -- recoveryWorker
// Related: session_ownership.go -- adjOutWrite.owesWithdrawal
//
// VALIDATES: an owner withdrawal at a non-ADD-PATH destination is owed to the
// destination's recovery worker, which asks the RIB and writes its decision.
// PREVENTS: a bare withdrawal that leaves a client with no route while the RIB
// holds another candidate, and a writer that waits on the election.
package reactor

import (
	"bytes"
	"maps"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
)

// owedAnswer is the RIB's answer to one gated lookup; nil answers WITHDRAW.
type owedAnswer func(ribevents.RecoveryRequest) []ribevents.RecoveryOutcome

// owedRail is a destination with a running recovery worker, a registered
// recovery owner, and a lookup double the test gates.
type owedRail struct {
	r           *Reactor
	source      *Peer
	destination *Peer
	conn        *recordingConn
	requests    chan ribevents.RecoveryRequest
	release     chan owedAnswer
}

func newOwedRail(t *testing.T) *owedRail {
	t.Helper()
	r, source, destination, conn, _ := newSyncOrderRail(t)
	r.clock = clock.RealClock{}
	r.api = flowForwardPluginServer(t, New(&Config{ListenAddr: "127.0.0.1:0"}))
	lowEventually(t, func() bool {
		command := r.api.Dispatcher().Registry().Lookup(recoveryCommandName)
		return command != nil && command.Process != nil
	}, "recovery command owner registered")
	destination.reactor = r
	destination.sendingInitialRoutes.Store(0)
	attachRecoveryOwner(t, r, source)
	rail := &owedRail{r: r, source: source, destination: destination, conn: conn,
		requests: make(chan ribevents.RecoveryRequest, 8), release: make(chan owedAnswer, 8)}
	owner := ribevents.PublishRecovery(func(request ribevents.RecoveryRequest) ([]ribevents.RecoveryOutcome, error) {
		rail.requests <- request
		if answer := <-rail.release; answer != nil {
			return answer(request), nil
		}
		return recoveryWithdrawAnswer(request), nil
	})
	session := destination.currentSession()
	session.startRecovery(destination)
	// The provider is released before the join, so a gated lookup cannot
	// wedge it (the in-process lookup takes no context).
	t.Cleanup(func() {
		close(rail.release)
		session.joinRecovery()
		owner.Close()
	})
	return rail
}

// attachRecoveryOwner publishes a delivery graph in which the registered
// recovery owner receives each source's received UPDATEs, as an `attach
// process bgp-rib { receive [ update-received ] }` block on each source does.
func attachRecoveryOwner(t *testing.T, r *Reactor, sources ...*Peer) {
	t.Helper()
	command := r.api.Dispatcher().Registry().Lookup(recoveryCommandName)
	require.NotNil(t, command)
	settings := make([]*PeerSettings, 0, len(sources))
	for _, source := range sources {
		source.settings.ProcessBindings = append(source.settings.ProcessBindings, ProcessBinding{
			PluginName: command.Process.Name(),
			Receive:    map[string]events.Direction{bgpevents.EventUpdate: events.DirReceived},
		})
		settings = append(settings, source.settings)
	}
	r.api.UpdateDeliveryGraph(bgpevents.Namespace, deliveryPeersFromSettings(settings))
}

func (rail *owedRail) forward(t *testing.T, withdraw bool, raw []byte) {
	t.Helper()
	body := ownershipRailBody(family.IPv4Unicast, withdraw, raw, 0)
	require.NoError(t, ownershipWriterForward(t, rail.destination, rail.source, body, false))
}

func (rail *owedRail) owed() map[adjOutKey]*adjOutOwed {
	table := &rail.destination.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()
	owed := make(map[adjOutKey]*adjOutOwed, len(table.owed))
	maps.Copy(owed, table.owed)
	return owed
}

func (rail *owedRail) request(t *testing.T) ribevents.RecoveryRequest {
	t.Helper()
	select {
	case request := <-rail.requests:
		return request
	case <-time.After(5 * time.Second):
		t.Fatal("the recovery worker asked the RIB nothing")
		return ribevents.RecoveryRequest{}
	}
}

// TestAdjOutOwnerWithdrawalOwedToRecovery proves the writer leaves a forwarded
// owner withdrawal to the destination's recovery worker. Method: the source
// announces P, then withdraws it. The writer emits nothing for the withdrawal,
// records P in the owed set with the destination session and the
// destination-framed NLRI, and the worker asks the RIB about P with no
// excluded source. Only the RIB's WITHDRAW answer reaches the wire, and the
// decided record leaves the set. The announcement installs the source's
// session generation as the owner's.
func TestAdjOutOwnerWithdrawalOwedToRecovery(t *testing.T) {
	rail := newOwedRail(t)
	p := []byte{24, 192, 0, 2}

	rail.forward(t, false, p)
	require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 1)
	table := &rail.destination.adjOut
	table.mu.Lock()
	for _, entry := range table.routes {
		require.Equal(t, rail.source.forwardGeneration.Load(), entry.owner.generation,
			"the owner's session generation is recorded with the path")
	}
	table.mu.Unlock()

	rail.forward(t, true, p)
	require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 1, "an owner withdrawal is owed, not written")
	owed := rail.owed()
	require.Len(t, owed, 1)
	for _, record := range owed {
		require.Same(t, rail.destination.currentSession(), record.session)
		require.Equal(t, family.IPv4Unicast, record.family)
		require.Equal(t, p, record.nlri)
	}

	request := rail.request(t)
	require.Len(t, request.Keys, 1)
	require.Equal(t, p, request.Keys[0].NLRI)
	require.Empty(t, request.Keys[0].Excluded, "an owner withdrawal excludes no source")
	rail.release <- nil

	lowEventually(t, func() bool {
		return len(recoveryWrittenUpdates(t, rail.conn.written())) == 2
	}, "the RIB's WITHDRAW decision is written")
	updates := recoveryWrittenUpdates(t, rail.conn.written())
	require.True(t, bytes.Equal(p, updates[1].WithdrawnRoutes), "the decision withdraws P")
	lowEventually(t, func() bool { return len(rail.owed()) == 0 }, "the decided record leaves the owed set")
}

// TestOwedVPNWithdrawalAsksInAnnouncementFraming proves the RIB is asked
// about an owed labeled key in the framing ribevents.RecoveryRequest names:
// native announcement framing. Method: the source announces a VPN route with
// a one-label stack, then withdraws it with the RFC 8277 Section 2.4
// Compatibility field 0x800000, whose S bit is clear. A request carrying the
// withdrawal's bytes reads as a label stack that never ends, and bgp-rib
// refused it with "recovery: invalid NLRI for ipv4/mpls-vpn" and retired the
// destination (test/plugin/vpn-withdrawal-inventory-rr-ipv4.ci). The request's
// NLRI must parse as an announcement and name the announced route's key, and
// the RIB's WITHDRAW decision must still reach the wire.
func TestOwedVPNWithdrawalAsksInAnnouncementFraming(t *testing.T) {
	rail := newOwedRail(t)
	vpn := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}
	rail.destination.negotiated.Load().families[vpn] = true
	announce := ownershipRailNative(vpn, 0, 1, false, 0)[4:]
	withdraw := ownershipRailNative(vpn, 0, 1, true, 0x80)[4:]

	require.NoError(t, ownershipWriterForward(t, rail.destination, rail.source, ownershipRailBody(vpn, false, announce, 0), false))
	require.NoError(t, ownershipWriterForward(t, rail.destination, rail.source, ownershipRailBody(vpn, true, withdraw, 0), false))
	require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 1, "an owner withdrawal is owed, not written")

	request := rail.request(t)
	require.Len(t, request.Keys, 1)
	var askedScratch, announcedScratch [nlrisplit.PrefixKeyScratchSize]byte
	asked, err := nlrisplit.GetPrefixKey(vpn)(request.Keys[0].NLRI, askedScratch[:], false)
	require.NoError(t, err, "the RIB reads the request's NLRI as an announcement")
	announced, err := nlrisplit.GetPrefixKey(vpn)(announce, announcedScratch[:], false)
	require.NoError(t, err)
	require.Equal(t, announced, asked, "the request names the announced route")
	rail.release <- nil

	lowEventually(t, func() bool {
		return len(recoveryWrittenUpdates(t, rail.conn.written())) == 2
	}, "the RIB's WITHDRAW decision is written")
	updates := recoveryWrittenUpdates(t, rail.conn.written())
	require.True(t, bytes.Contains(updates[1].PathAttributes, withdraw),
		"the decision withdraws the route with the Compatibility field 0x800000")
	lowEventually(t, func() bool { return len(rail.owed()) == 0 }, "the decided record leaves the owed set")
}

// TestOwnerWithdrawalWrittenWhenOwnerDoesNotReceiveSource proves a
// recovery owner counts only for a source it receives. Method: bgp-rib is
// registered, as an auto-loaded RIB is, but no attach block grants it the
// source's received UPDATEs (test/plugin/mup-t1st-malformed-treat-as-withdraw.ci).
// The source announces P and withdraws it: the withdrawal is written at once,
// nothing is owed, and the worker asks the RIB nothing, so a later UPDATE from
// the source cannot overtake it. Attaching the owner to the source then makes
// the next owner withdrawal owed, so the test is not passing on a worker that
// never owes.
func TestOwnerWithdrawalWrittenWhenOwnerDoesNotReceiveSource(t *testing.T) {
	r, source, destination, conn, _ := newSyncOrderRail(t)
	r.clock = clock.RealClock{}
	r.api = flowForwardPluginServer(t, New(&Config{ListenAddr: "127.0.0.1:0"}))
	lowEventually(t, func() bool {
		command := r.api.Dispatcher().Registry().Lookup(recoveryCommandName)
		return command != nil && command.Process != nil
	}, "recovery command owner registered")
	destination.reactor = r
	destination.sendingInitialRoutes.Store(0)
	requests := make(chan ribevents.RecoveryRequest, 8)
	owner := ribevents.PublishRecovery(func(request ribevents.RecoveryRequest) ([]ribevents.RecoveryOutcome, error) {
		requests <- request
		return recoveryWithdrawAnswer(request), nil
	})
	session := destination.currentSession()
	session.startRecovery(destination)
	t.Cleanup(func() {
		session.joinRecovery()
		owner.Close()
	})
	rail := &owedRail{r: r, source: source, destination: destination, conn: conn}
	p := []byte{24, 192, 0, 2}

	rail.forward(t, false, p)
	rail.forward(t, true, p)
	updates := recoveryWrittenUpdates(t, conn.written())
	require.Len(t, updates, 2, "an owner that does not receive the source owes nothing: the withdrawal is written")
	require.True(t, bytes.Equal(p, updates[1].WithdrawnRoutes), "the write withdraws P")
	require.Empty(t, rail.owed())
	select {
	case request := <-requests:
		t.Fatalf("the worker asked the RIB about %d keys it does not hold", len(request.Keys))
	case <-time.After(50 * time.Millisecond):
	}

	attachRecoveryOwner(t, r, source)
	rail.forward(t, false, p)
	rail.forward(t, true, p)
	require.Len(t, recoveryWrittenUpdates(t, conn.written()), 3, "an attached owner's withdrawal is owed, not written")
	request := <-requests
	require.Len(t, request.Keys, 1)
	lowEventually(t, func() bool {
		return len(recoveryWrittenUpdates(t, conn.written())) == 4
	}, "the attached owner's WITHDRAW decision is written")
}

// TestOwedKeyHandOffNeverBlocksWriter proves the writer never waits on the
// worker. Method: the lookup double holds the worker inside its first round;
// meanwhile the source withdraws a second owned key. The writer returns at
// once, the key is owed, and the skipped wake loses nothing: a later round
// asks about it, and both keys are decided.
func TestOwedKeyHandOffNeverBlocksWriter(t *testing.T) {
	rail := newOwedRail(t)
	p := []byte{24, 192, 0, 2}
	q := []byte{24, 192, 0, 3}
	rail.forward(t, false, p)
	rail.forward(t, false, q)
	rail.forward(t, true, p)
	first := rail.request(t) // The worker is now held inside the lookup.
	require.Equal(t, p, first.Keys[0].NLRI)

	returned := make(chan struct{})
	go func() {
		defer close(returned)
		rail.forward(t, true, q)
		rail.forward(t, true, q) // A second wake finds the slot full and skips.
	}()
	select {
	case <-returned:
	case <-time.After(5 * time.Second):
		t.Fatal("the writer waited on the recovery worker")
	}
	require.Len(t, rail.owed(), 2)
	rail.release <- nil

	// The writes that owed Q moved the destination's send sequence, so the
	// first admission may retry and the next round may ask about P again.
	second := rail.request(t)
	asked := false
	for i := range second.Keys {
		asked = asked || bytes.Equal(q, second.Keys[i].NLRI)
	}
	require.True(t, asked, "the next round asks about the key owed while the worker was busy")
	for range 4 {
		rail.release <- nil
	}
	lowEventually(t, func() bool { return len(rail.owed()) == 0 }, "both owed keys are decided")
}

// TestOwedKeyWorkerLifecycle proves the worker's start, stop and join. Method:
// a key owed before the worker starts is decided by the first round with no
// wake; seal stops a running worker and join returns; stop and join are
// idempotent; a worker started on a sealed session stops at once.
func TestOwedKeyWorkerLifecycle(t *testing.T) {
	t.Run("first-round-without-wake", func(t *testing.T) {
		r, source, destination, _, _ := newSyncOrderRail(t)
		r.clock = clock.RealClock{}
		r.api = flowForwardPluginServer(t, New(&Config{ListenAddr: "127.0.0.1:0"}))
		lowEventually(t, func() bool {
			command := r.api.Dispatcher().Registry().Lookup(recoveryCommandName)
			return command != nil && command.Process != nil
		}, "recovery command owner registered")
		destination.reactor = r
		destination.sendingInitialRoutes.Store(0)
		asked := make(chan ribevents.RecoveryRequest, 1)
		owner := ribevents.PublishRecovery(func(request ribevents.RecoveryRequest) ([]ribevents.RecoveryOutcome, error) {
			asked <- request
			return recoveryWithdrawAnswer(request), nil
		})
		t.Cleanup(owner.Close)
		require.NoError(t, ownershipWriterForward(t, destination, source,
			ownershipRailBody(family.IPv4Unicast, false, []byte{24, 192, 0, 2}, 0), false))
		session := destination.currentSession()
		table := &destination.adjOut
		table.mu.Lock()
		for key := range table.routes {
			table.owed = map[adjOutKey]*adjOutOwed{key: {session: session, family: family.IPv4Unicast,
				nlri: []byte{24, 192, 0, 2}}}
		}
		table.mu.Unlock()

		session.startRecovery(destination)
		select {
		case request := <-asked:
			require.Equal(t, []byte{24, 192, 0, 2}, request.Keys[0].NLRI)
		case <-time.After(5 * time.Second):
			t.Fatal("a key owed before the worker started was stranded")
		}
		session.seal()
		session.joinRecovery()
		session.joinRecovery() // Idempotent.
		session.stopRecovery()
	})
	t.Run("sealed-before-start", func(t *testing.T) {
		r, _, destination, _, _ := newSyncOrderRail(t)
		destination.reactor = r
		session := destination.currentSession()
		session.seal()
		session.startRecovery(destination)
		joined := make(chan struct{})
		go func() {
			defer close(joined)
			session.joinRecovery()
		}()
		select {
		case <-joined:
		case <-time.After(5 * time.Second):
			t.Fatal("a worker started on a sealed session kept running")
		}
	})
	t.Run("no-reactor-no-worker", func(t *testing.T) {
		_, _, destination, _, _ := newSyncOrderRail(t)
		destination.reactor = nil
		session := destination.currentSession()
		session.startRecovery(destination)
		require.Nil(t, session.recovery.Load())
		require.False(t, session.recoveryOwed(), "with no worker an owner withdrawal is written as before")
		session.joinRecovery()
	})
}

// owedRailExpected writes body as a bgp-rib lifecycle removal: an
// adjOutExpected item naming the sent revision it removes.
func owedRailExpected(t *testing.T, target *Peer, body []byte, revision uint64) {
	t.Helper()
	s := target.currentSession()
	item := fwdItem{peer: target, session: s, authority: adjOutExpected, expectedMessage: revision}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.sentForward = &item
	defer func() { s.sentForward = nil }()
	require.NoError(t, s.writeRawUpdateBody(body))
	require.NoError(t, s.flushWrites())
}

// TestAdjOutOwedAuthorities proves which withdrawals the writer owes. Method:
// a bgp-rib lifecycle removal (adjOutExpected) of a source-owned key at a
// non-ADD-PATH destination is owed and not written, while a forwarded owner
// withdrawal at a destination that negotiated ADD-PATH send for the family is
// written as before and owes nothing.
func TestAdjOutOwedAuthorities(t *testing.T) {
	t.Run("lifecycle-removal-owed", func(t *testing.T) {
		rail := newOwedRail(t)
		p := []byte{24, 192, 0, 2}
		rail.forward(t, false, p)
		table := &rail.destination.adjOut
		table.mu.Lock()
		var revision uint64
		for _, entry := range table.routes {
			revision = entry.revision
		}
		table.mu.Unlock()
		require.NotZero(t, revision)

		owedRailExpected(t, rail.destination, ownershipRailBody(family.IPv4Unicast, true, p, 0), revision)
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 1, "a lifecycle removal is owed, not written")
		require.Len(t, rail.owed(), 1)
		request := rail.request(t)
		require.Equal(t, p, request.Keys[0].NLRI)
		require.Empty(t, request.Keys[0].Excluded, "a lifecycle removal excludes no source")
		rail.release <- nil
		lowEventually(t, func() bool { return len(rail.owed()) == 0 }, "the decided record leaves the owed set")
	})
	t.Run("add-path-destination-written", func(t *testing.T) {
		rail := newOwedRail(t)
		ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true})
		ctxID, err := bgpctx.Registry.Register(ctx)
		require.NoError(t, err)
		rail.destination.currentSession().setSendCtxID(ctxID)
		p := []byte{0, 0, 0, 1, 24, 192, 0, 2}
		rail.forward(t, false, p)
		rail.forward(t, true, p)
		require.Empty(t, rail.owed(), "an ADD-PATH destination keeps every path; nothing is owed")
		raw := rail.conn.written()
		require.Len(t, recoveryWrittenUpdates(t, raw), 2, "the owner withdrawal is written as before")
	})
}

// TestOwedKeyNoRecoveryOwnerWritesWithdrawal proves AC-10's last sentence and
// AC-11's ownerless departure. The destination runs a recovery worker, but no
// process registers `request bgp rib recovery` (an external forwarder running
// without bgp-rib), so nothing can elect a replacement. Each owner withdrawal
// of three prefixes is then written in order, as before this spec, nothing is
// owed and the RIB is never asked. A departure in the same state cannot tell
// the client which route replaces the departed one, so it retires the
// destination session. The one warning per destination session is the
// worker's ownerLog sync.Once; the subsystem logger writes to the process's
// stderr and has no test seam, so the count is carried by that structure.
//
// VALIDATES: AC-10 (no recovery owner: the withdrawal is written as today) and
// AC-11 (a departure with no owner retires the destination).
// PREVENTS: a withdrawal held for an election nobody can run.
func TestOwedKeyNoRecoveryOwnerWritesWithdrawal(t *testing.T) {
	r, source, destination, conn, _ := newSyncOrderRail(t)
	r.clock = clock.RealClock{}
	r.api = newBorrowedPluginServer(t, New(&Config{ListenAddr: "127.0.0.1:0"}))
	require.Nil(t, r.api.Dispatcher().Registry().Lookup(recoveryCommandName), "guard: no recovery owner")
	destination.reactor = r
	destination.sendingInitialRoutes.Store(0)
	requests := make(chan ribevents.RecoveryRequest, 8)
	owner := ribevents.PublishRecovery(func(request ribevents.RecoveryRequest) ([]ribevents.RecoveryOutcome, error) {
		requests <- request
		return recoveryWithdrawAnswer(request), nil
	})
	session := destination.currentSession()
	session.startRecovery(destination)
	t.Cleanup(func() {
		session.joinRecovery()
		owner.Close()
	})
	require.NotNil(t, session.recovery.Load(), "guard: the destination runs a worker")
	rail := &owedRail{r: r, source: source, destination: destination, conn: conn}

	prefixes := [][]byte{{24, 192, 0, 2}, {24, 192, 0, 3}, {24, 192, 0, 4}}
	for _, p := range prefixes {
		rail.forward(t, false, p)
		rail.forward(t, true, p)
	}
	updates := recoveryWrittenUpdates(t, conn.written())
	require.Len(t, updates, 2*len(prefixes), "every announcement and every withdrawal is written")
	for i, p := range prefixes {
		require.True(t, bytes.Equal(p, updates[2*i+1].WithdrawnRoutes), "write %d withdraws %x", 2*i+1, p)
	}
	require.Empty(t, rail.owed(), "nothing is owed without a recovery owner")
	select {
	case request := <-requests:
		t.Fatalf("the worker asked the RIB about %d keys with no recovery owner", len(request.Keys))
	case <-time.After(50 * time.Millisecond):
	}

	rail.forward(t, false, prefixes[0])
	err := recoveryDepart(t, r, destination, source.Settings().Address, "192.0.2.0/24")
	require.Error(t, err, "an ownerless departure reports that no owner can elect")
	require.True(t, session.tearingDown.Load(), "an ownerless departure retires the destination")
}

// TestOwedKeyOwnerGoneAfterOweRetires proves AC-11's third failure: the
// recovery owner leaves after a key was owed. P is owed and its round is held
// inside the lookup; Q is owed meanwhile; then `request bgp rib recovery` is
// unregistered, as when bgp-rib's process dies. P's held round still lands,
// and the round for Q finds no owner, so no election can decide Q: the
// destination is retired with Cease, never left holding Q's withdrawn route
// on a session that stays up, and Q's withdrawal is never written bare.
//
// VALIDATES: AC-11, the owner gone between owe and lookup retires the
// destination.
// PREVENTS: a key left owed forever, or decided without an election.
func TestOwedKeyOwnerGoneAfterOweRetires(t *testing.T) {
	rail := newOwedRail(t)
	p := []byte{24, 192, 0, 2}
	q := []byte{24, 192, 0, 3}
	rail.forward(t, false, p)
	rail.forward(t, false, q)
	rail.forward(t, true, p)
	rail.request(t) // P's round, held.
	rail.forward(t, true, q)
	require.Len(t, rail.owed(), 2, "P and Q are owed")

	registry := rail.r.api.Dispatcher().Registry()
	command := registry.Lookup(recoveryCommandName)
	require.NotNil(t, command)
	registry.Unregister(command.Process, []string{recoveryCommandName})
	require.Nil(t, registry.Lookup(recoveryCommandName), "guard: the owner is gone")

	rail.release <- nil // P's round answers WITHDRAW.
	session := rail.destination.currentSession()
	lowEventually(t, func() bool { return session.tearingDown.Load() }, "the destination is retired")
	// RFC 4271 Section 4.1: each frame carries its own length; the retirement
	// leaves a NOTIFICATION after the UPDATEs.
	raw := rail.conn.written()
	for len(raw) >= message.HeaderLen {
		length := int(raw[16])<<8 | int(raw[17])
		require.GreaterOrEqual(t, length, message.HeaderLen)
		require.LessOrEqual(t, length, len(raw))
		if raw[18] == 2 { // UPDATE
			update, err := message.UnpackUpdate(raw[message.HeaderLen:length])
			require.NoError(t, err)
			require.False(t, bytes.Equal(q, update.WithdrawnRoutes), "Q's withdrawal is never written without an election")
		}
		raw = raw[length:]
	}
	select {
	case request := <-rail.requests:
		t.Fatalf("the RIB was asked about %d keys after its owner left", len(request.Keys))
	case <-time.After(50 * time.Millisecond):
	}
}
