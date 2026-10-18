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

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/family"
)

// owedRail is a destination with a running recovery worker, a registered
// recovery owner, and a lookup double the test gates.
type owedRail struct {
	r           *Reactor
	source      *Peer
	destination *Peer
	conn        *recordingConn
	requests    chan ribevents.RecoveryRequest
	release     chan struct{}
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
	rail := &owedRail{r: r, source: source, destination: destination, conn: conn,
		requests: make(chan ribevents.RecoveryRequest, 8), release: make(chan struct{}, 8)}
	owner := ribevents.PublishRecovery(func(request ribevents.RecoveryRequest) ([]ribevents.RecoveryOutcome, error) {
		rail.requests <- request
		<-rail.release
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
	rail.release <- struct{}{}

	lowEventually(t, func() bool {
		return len(recoveryWrittenUpdates(t, rail.conn.written())) == 2
	}, "the RIB's WITHDRAW decision is written")
	updates := recoveryWrittenUpdates(t, rail.conn.written())
	require.True(t, bytes.Equal(p, updates[1].WithdrawnRoutes), "the decision withdraws P")
	lowEventually(t, func() bool { return len(rail.owed()) == 0 }, "the decided record leaves the owed set")
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
	rail.release <- struct{}{}

	// The writes that owed Q moved the destination's send sequence, so the
	// first admission may retry and the next round may ask about P again.
	second := rail.request(t)
	asked := false
	for i := range second.Keys {
		asked = asked || bytes.Equal(q, second.Keys[i].NLRI)
	}
	require.True(t, asked, "the next round asks about the key owed while the worker was busy")
	for range 4 {
		rail.release <- struct{}{}
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
		rail.release <- struct{}{}
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
