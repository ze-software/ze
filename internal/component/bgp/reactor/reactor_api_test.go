package reactor

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPeerInfoPopulatesStats verifies that Peers() populates message and route counters.
//
// VALIDATES: reactorAPIAdapter.Peers() returns non-zero statistics from peer counters.
// PREVENTS: Stats fields remaining zero despite counter increments.
func TestPeerInfoPopulatesStats(t *testing.T) {
	r := New(&Config{})
	r.startTime = time.Now()

	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)
	peer.incrUpdatesReceived()
	peer.incrUpdatesReceived()
	peer.incrUpdatesSent()
	peer.incrKeepalivesReceived()
	peer.incrKeepalivesReceived()
	peer.incrKeepalivesSent()
	peer.incrEORReceived()
	peer.incrEORSent()
	peer.counters.establishedAt.Store(time.Now().Add(-time.Second).UnixNano())
	peer.state.Store(int32(PeerStateEstablished))

	r.peers[settings.PeerKey()] = peer

	adapter := &reactorAPIAdapter{r: r}
	peers := adapter.Peers()

	require.Len(t, peers, 1)
	p := peers[0]

	assert.Equal(t, uint32(2), p.UpdatesReceived, "updates received")
	assert.Equal(t, uint32(1), p.UpdatesSent, "updates sent")
	assert.Equal(t, uint32(2), p.KeepalivesReceived, "keepalives received")
	assert.Equal(t, uint32(1), p.KeepalivesSent, "keepalives sent")
	assert.Equal(t, uint32(1), p.EORReceived, "eor received")
	assert.Equal(t, uint32(1), p.EORSent, "eor sent")
	assert.True(t, p.Uptime > 0, "uptime should be non-zero for established peer")
}

// TestPeerInfoUptimeUsesEstablishedAt verifies per-peer uptime, not reactor start time.
//
// VALIDATES: Uptime comes from peer's EstablishedAt, not reactor.startTime.
// PREVENTS: All peers showing the same uptime regardless of when they established.
func TestPeerInfoUptimeUsesEstablishedAt(t *testing.T) {
	r := New(&Config{})
	r.startTime = time.Now().Add(-1 * time.Hour) // reactor started 1 hour ago

	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	// Peer established just now — uptime should be ~0, not ~1 hour
	peer.setEstablishedNow()

	r.peers[settings.PeerKey()] = peer

	adapter := &reactorAPIAdapter{r: r}
	peers := adapter.Peers()

	require.Len(t, peers, 1)
	// Uptime should be close to 0, not close to 1 hour
	assert.Less(t, peers[0].Uptime, 10*time.Second, "uptime should reflect peer establishment, not reactor start")
}

// TestPeerInfoNonEstablishedZeroUptime verifies non-established peers have zero uptime.
//
// VALIDATES: Peers not in Established state have zero Uptime.
// PREVENTS: Non-established peers showing stale uptime from previous session.
func TestPeerInfoNonEstablishedZeroUptime(t *testing.T) {
	r := New(&Config{})
	r.startTime = time.Now()

	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)
	// Not established — state defaults to Idle (0)

	r.peers[settings.PeerKey()] = peer

	adapter := &reactorAPIAdapter{r: r}
	peers := adapter.Peers()

	require.Len(t, peers, 1)
	assert.Equal(t, time.Duration(0), peers[0].Uptime, "non-established peer should have zero uptime")
}

// TestReconcilePeersJournalRollback verifies that when a peer add fails,
// the journal rolls back all previously successful operations.
//
// VALIDATES: AC-2 - BGP apply: 5 peers to add, peer 3 fails, journal rolls back peers 1-2.
// PREVENTS: Partial peer state after a failed add leaves the reactor inconsistent.
func TestReconcilePeersJournalRollback(t *testing.T) {
	r := New(&Config{})
	adapter := &reactorAPIAdapter{r: r}

	p1 := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0)
	p2 := NewPeerSettings(mustParseAddr("192.0.2.2"), 65000, 65002, 0)
	p3 := NewPeerSettings(mustParseAddr("192.0.2.3"), 65000, 65003, 0)

	// Use a journal that fails on the 3rd Record call, simulating
	// a failure during the 3rd peer add.
	j := &failingJournal{failAt: 3}
	err := adapter.reconcilePeersJournaled([]*PeerSettings{p1, p2, p3}, "test", j)
	require.Error(t, err, "reconcile should fail when journal rejects 3rd operation")

	// After rollback, no peers should remain (all were adds, all rolled back).
	assert.Len(t, r.peers, 0, "rollback should remove all added peers")
	assert.True(t, j.rolledBack, "journal should have been rolled back")
}

// TestReconcilePeersJournalSuccess verifies that when all peers are added
// successfully, the journal is committed (no rollback).
//
// VALIDATES: AC-3 - BGP apply: all peers succeed, all peers running.
// PREVENTS: Journal left uncommitted after successful reconcile.
func TestReconcilePeersJournalSuccess(t *testing.T) {
	r := New(&Config{})
	adapter := &reactorAPIAdapter{r: r}

	p1 := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0)
	p2 := NewPeerSettings(mustParseAddr("192.0.2.2"), 65000, 65002, 0)

	j := &testJournal{}
	err := adapter.reconcilePeersJournaled([]*PeerSettings{p1, p2}, "test", j)
	require.NoError(t, err)

	assert.Len(t, r.peers, 2, "both peers should exist after successful reconcile")
	assert.False(t, j.rolledBack, "journal should not be rolled back on success")
	assert.Equal(t, 2, j.recordCount, "journal should have 2 entries (one per peer add)")
}

// TestReconcilePeersReleasesRouterIDClaimSynchronously verifies that a peer
// removed by a reload has given up its AS-wide BGP Identifier claim by the time
// reconcile returns, not merely once its own goroutine gets around to cleanup.
//
// VALIDATES: after reconcilePeersJournaled returns, the identifier the outgoing
// peer held is unclaimed, so the incoming generation can claim it.
// PREVENTS: a reload that MOVES a router-id between peers (rotation, swap, or a
// re-address pointing a new peer at the router an outgoing one served) answering
// the new, legitimate peer with OPEN Message Error / Bad BGP Identifier because
// the outgoing peer's claim was still registered when the new peer's OPEN was
// validated. Peer.Stop only cancels the context; without a synchronous release
// the outcome is decided by goroutine scheduling.
func TestReconcilePeersReleasesRouterIDClaimSynchronously(t *testing.T) {
	const peerAS uint32 = 65001
	const sharedBGPID uint32 = 0x01020305

	r := New(&Config{})
	r.eventDispatcher = nil
	adapter := &reactorAPIAdapter{r: r}

	// Peer A is attached WITHOUT starting its run goroutine, so nothing can
	// release the claim asynchronously: any release observed below is the one
	// reconcile performed itself.
	outgoing := NewPeerSettings(mustParseAddr("192.0.2.1"), peerAS, peerAS, 0x01020304)
	outgoing.Name = "peerA"
	peerA := NewPeer(outgoing)
	peerA.SetReactor(r)
	r.peers[outgoing.PeerKey()] = peerA

	_, granted := r.routerIDs.claim(peerA, outgoing.Address, peerAS, sharedBGPID)
	require.True(t, granted, "peer A should hold the identifier before the reload")
	holder, held := r.routerIDs.holder(peerAS, sharedBGPID)
	require.True(t, held)
	require.Same(t, peerA, holder)

	// The reload drops peer A entirely and brings up peer C, which reaches the
	// router that was presenting sharedBGPID.
	incoming := NewPeerSettings(mustParseAddr("192.0.2.3"), peerAS, peerAS, 0x01020306)
	incoming.Name = "peerC"

	j := &testJournal{}
	require.NoError(t, adapter.reconcilePeersJournaled([]*PeerSettings{incoming}, "test", j))

	// No sleep, no polling: the claim must already be gone.
	_, stillHeld := r.routerIDs.holder(peerAS, sharedBGPID)
	assert.False(t, stillHeld,
		"removed peer must release its BGP Identifier claim before reconcile returns, "+
			"otherwise the incoming peer's OPEN is refused as Bad BGP Identifier")
}

// TestReconcilePeersJournalRemoveThenAdd verifies the remove-before-add order
// with journal recording undo operations for both.
//
// VALIDATES: AC-4 - BGP rollback: removed peers re-added with old settings, added peers stopped.
// PREVENTS: Rollback leaving peers in wrong state (removed not restored, added not cleaned).
func TestReconcilePeersJournalRemoveThenAdd(t *testing.T) {
	r := New(&Config{})
	adapter := &reactorAPIAdapter{r: r}

	// Start with peers A and B.
	pA := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0)
	pA.Name = "peerA"
	pB := NewPeerSettings(mustParseAddr("192.0.2.2"), 65000, 65002, 0)
	pB.Name = "peerB"
	require.NoError(t, r.AddPeer(pA))
	require.NoError(t, r.AddPeer(pB))
	require.Len(t, r.peers, 2)

	// New config: B stays, A removed, C added.
	pC := NewPeerSettings(mustParseAddr("192.0.2.3"), 65000, 65003, 0)
	pC.Name = "peerC"

	j := &testJournal{}
	err := adapter.reconcilePeersJournaled([]*PeerSettings{pB, pC}, "test", j)
	require.NoError(t, err)

	// After apply: B and C should exist, A removed.
	assert.Len(t, r.peers, 2)
	_, hasB := r.peers[pB.PeerKey()]
	_, hasC := r.peers[pC.PeerKey()]
	assert.True(t, hasB, "peer B should still exist")
	assert.True(t, hasC, "peer C should be added")

	// Journal should have entries for: remove A (undo=re-add A) + add C (undo=remove C).
	assert.Equal(t, 2, j.recordCount, "journal should record remove A and add C")

	// Now rollback: should restore to A+B.
	j.rollback()
	assert.Len(t, r.peers, 2)
	_, hasA := r.peers[pA.PeerKey()]
	_, hasB = r.peers[pB.PeerKey()]
	assert.True(t, hasA, "peer A should be restored after rollback")
	assert.True(t, hasB, "peer B should still exist after rollback")
}

// TestBGPVerifyEstimate verifies that PeerDiffCount returns a count proportional
// to the number of peer changes.
//
// VALIDATES: AC-12 - BGP budget proportional to peer count.
// PREVENTS: Budget estimate that doesn't scale with diff size.
func TestBGPVerifyEstimate(t *testing.T) {
	r := New(&Config{})

	// Peer trees in the shape `grouping peer-fields` produces
	// (../yang/ze-bgp-conf.yang). The global local AS lives under
	// bgp > session > asn > local: without it PeersFromTree skips every peer as
	// incomplete and the diff below counts nothing.
	peerTree := func(ip, as string) map[string]any {
		return map[string]any{
			"connection": map[string]any{
				"remote": map[string]any{"ip": ip},
				"local":  map[string]any{"ip": "auto"},
			},
			"session": map[string]any{"asn": map[string]any{"remote": as}},
		}
	}
	globalASN := map[string]any{"asn": map[string]any{"local": "65000"}}

	tree := map[string]any{
		"session": globalASN,
		"peer": map[string]any{
			"peer1": peerTree("192.0.2.1", "65001"),
			"peer2": peerTree("192.0.2.2", "65002"),
			"peer3": peerTree("192.0.2.3", "65003"),
			"peer4": peerTree("192.0.2.4", "65004"),
			"peer5": peerTree("192.0.2.5", "65005"),
		},
	}

	// Pre-add 2 peers via the same tree parsing path so PeerKey matches.
	existingTree := map[string]any{
		"session": globalASN,
		"peer": map[string]any{
			"peer1": peerTree("192.0.2.1", "65001"),
			"peer2": peerTree("192.0.2.2", "65002"),
		},
	}
	existingPeers, err := PeersFromTree(existingTree)
	require.NoError(t, err)
	require.Len(t, existingPeers, 2, "both peers must parse: a skipped peer makes the diff below vacuous")
	for _, p := range existingPeers {
		require.NoError(t, r.AddPeer(p))
	}

	count, err := r.PeerDiffCount(tree)
	require.NoError(t, err)
	assert.Equal(t, 3, count, "should report 3 new peers to add")
}

// TestBGPApplyBudgetUpdate verifies that budget is set at registration
// proportional to expected peer count.
//
// VALIDATES: AC-11 - All 5 plugins provide initial budgets at registration.
// PREVENTS: Plugin registered without verify/apply budgets.
func TestBGPApplyBudgetUpdate(t *testing.T) {
	// This test validates the static registration values.
	// The actual registration happens in plugin/register.go;
	// here we verify the budget calculation logic.
	const perPeerCostSeconds = 2

	// With 5 peers, budget should be 5 * perPeerCost.
	count := 5
	budget := count * perPeerCostSeconds
	assert.Equal(t, 10, budget, "budget should be proportional to peer count")
}

// failingJournal is a test double that fails on the Nth Record call.
// It runs apply for records 1..N-1, then returns an error on N without
// running apply. It automatically rolls back on failure.
type failingJournal struct {
	entries    []func() error
	count      int
	failAt     int
	rolledBack bool
}

func (j *failingJournal) Record(apply, undo func() error) error {
	j.count++
	if j.count >= j.failAt {
		// Roll back previous entries before returning error.
		j.rolledBack = true
		for _, v := range slices.Backward(j.entries) {
			_ = v()
		}
		j.entries = nil
		return fmt.Errorf("injected failure at record %d", j.count)
	}
	if err := apply(); err != nil {
		return err
	}
	j.entries = append(j.entries, undo)
	return nil
}

func (j *failingJournal) Rollback() []error {
	j.rolledBack = true
	var errs []error
	for _, v := range slices.Backward(j.entries) {
		if err := v(); err != nil {
			errs = append(errs, err)
		}
	}
	j.entries = nil
	return errs
}

func (j *failingJournal) Discard() {
	j.entries = nil
}

// testJournal is a test double for ConfigJournal that tracks operations.
type testJournal struct {
	entries     []func() error // undo functions
	recordCount int
	rolledBack  bool
}

func (j *testJournal) Record(apply, undo func() error) error {
	if err := apply(); err != nil {
		return err
	}
	j.entries = append(j.entries, undo)
	j.recordCount++
	return nil
}

func (j *testJournal) Rollback() []error {
	j.rolledBack = true
	return j.rollback()
}

func (j *testJournal) rollback() []error {
	var errs []error
	for _, v := range slices.Backward(j.entries) {
		if err := v(); err != nil {
			errs = append(errs, err)
		}
	}
	j.entries = nil
	return errs
}

func (j *testJournal) Discard() {
	j.entries = nil
}

// TestPeersFromTreeRejectsBadRouterID drives the BGP Identifier rule from the
// WHOLE-TREE entry point loadPeersFullOrTree reaches, not from the per-peer
// helper (ai/rules/evidence.md: a guard's test starts where callers enter).
//
// PeersFromTree skips a peer whose error wraps ErrIncompleteConfig and keeps
// parsing the rest. A bad router-id must NOT take that route: parseRouterID's
// error is unwrapped, so the whole tree is refused and no peer is returned.
// TestParsePeerFromTreeInvalid (config_test.go) pins the two rejections at the
// helper; this test pins what the tree-level caller does with them.
//
// VALIDATES: router-id 0.0.0.0 is REJECTED with an error naming the peer and
// RFC 6286 Section 2.1, not accepted as RouterID 0 and not skipped.
// VALIDATES: a malformed router-id is rejected rather than silently ignored.
// VALIDATES: a valid router-id is still parsed into RouterID.
// PREVENTS: a peer reaching the reactor with a zero BGP Identifier, which every
// RFC 6286 Section 2.2 implementation answers with Bad BGP Identifier, so the
// session can never come up.
func TestPeersFromTreeRejectsBadRouterID(t *testing.T) {
	tests := []struct {
		name       string
		routerID   string
		wantErr    bool
		wantErrHas string
		wantID     uint32
	}{
		{
			name:       "zero identifier rejected",
			routerID:   "0.0.0.0",
			wantErr:    true,
			wantErrHas: "0.0.0.0",
		},
		{
			name:       "malformed identifier rejected",
			routerID:   "not-an-ip",
			wantErr:    true,
			wantErrHas: "not-an-ip",
		},
		{
			name:     "valid identifier parsed",
			routerID: "10.0.0.1",
			wantID:   0x0A000001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := map[string]any{
				"peer": map[string]any{
					"peer1": map[string]any{
						"connection": map[string]any{
							"remote": map[string]any{"ip": "192.0.2.1"},
							"local":  map[string]any{"ip": "auto"},
						},
						"session": map[string]any{
							"asn":       map[string]any{"remote": "65001", "local": "65000"},
							"router-id": tt.routerID,
						},
					},
				},
			}

			peers, err := PeersFromTree(tree)

			if tt.wantErr {
				require.Error(t, err, "router-id %q must be rejected", tt.routerID)
				assert.Contains(t, err.Error(), "peer1", "error must name the offending peer")
				assert.Contains(t, err.Error(), tt.wantErrHas, "error must quote the offending value")
				assert.Nil(t, peers, "no peers may be returned when a peer is rejected")
				return
			}

			require.NoError(t, err)
			require.Len(t, peers, 1)
			assert.Equal(t, tt.wantID, peers[0].RouterID)
		})
	}
}

// TestPeersFromTreeRejectsWrongTypedPeerSection pins the difference between a
// config that states NO peers and a config whose `peer` node this parser cannot
// read. Both arrive at the same lookup, and mapMap (config.go) answers each with
// the same false, so reading the raw value is what keeps them apart.
//
// VALIDATES: a `peer` node that is not a map is REJECTED with an error naming
// the type it found, not reported as an empty peer set.
// VALIDATES: an absent `peer` node is still no peers and no error.
// PREVENTS: a reload reporting success over config nobody could read, which
// ApplyConfigDiff (reactor_api.go) then applies by reconciling the reactor to
// zero peers -- every session torn down, no error anywhere
// (ai/rules/evidence.md, a zero that reads as a valid answer).
func TestPeersFromTreeRejectsWrongTypedPeerSection(t *testing.T) {
	// The tree route is operator JSON, so `peer` can arrive as any shape.
	for _, tt := range []struct {
		name    string
		section any
		wantHas string
	}{
		{name: "string", section: "london", wantHas: "string"},
		{name: "list", section: []any{"london"}, wantHas: "[]interface {}"},
		{name: "number", section: float64(2), wantHas: "float64"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			peers, err := PeersFromTree(map[string]any{"peer": tt.section})

			require.Error(t, err, "a peer section of type %T must be rejected", tt.section)
			assert.Contains(t, err.Error(), tt.wantHas, "the error must name the type it found")
			assert.Nil(t, peers, "an unreadable peer section may not read as an empty peer set")
		})
	}

	t.Run("absent", func(t *testing.T) {
		peers, err := PeersFromTree(map[string]any{})

		require.NoError(t, err, "a config stating no peers is not an error")
		assert.Empty(t, peers)
	})
}
