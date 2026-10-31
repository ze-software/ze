// RFC naming: untagged -- tests the D-1 import relaxation RFC 8212 default reject relies on; rows RFC8212-3-1/3-2 are {gap}, which refuse a proof tag until the spec closes them (AC-11).
package reactor

import (
	"context"
	"errors"
	"maps"
	"net"
	"net/netip"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// relaxNegotiated is a published negotiation with or without Route Refresh,
// over IPv4 and IPv6 unicast.
func relaxNegotiated(routeRefresh bool) *NegotiatedCapabilities {
	return &NegotiatedCapabilities{RouteRefresh: routeRefresh,
		families: map[family.Family]bool{family.IPv4Unicast: true, family.IPv6Unicast: true}}
}

// newRelaxFixture is the transaction fixture (newRFC8212ImportFixture) with
// the peer's import policy absent, so its EBGP default rejects, and the
// session's negotiation published with or without Route Refresh.
func newRelaxFixture(t *testing.T, routeRefresh bool) *rfc8212ImportFixture {
	t.Helper()
	fx := newRFC8212ImportFixture(t, false)
	fx.peer.mu.Lock()
	fx.peer.settings.ImportPolicy = filterapi.Policy{}
	fx.peer.mu.Unlock()
	fx.peer.negotiated.Store(relaxNegotiated(routeRefresh))
	return fx
}

// relax lifts the peer's rejecting import decision through the journaled swap.
func relax(fx *rfc8212ImportFixture, j configJournal) error {
	next := fx.peer.settingsSnapshot()
	accept, err := filterapi.NewPolicy(nil, routepolicy.FinalAccept)
	if err != nil {
		return err
	}
	next.ImportPolicy = accept
	copyImport := func(dst, src *PeerSettings) { dst.ImportPolicy = src.ImportPolicy }
	return (&reactorAPIAdapter{r: fx.r}).swapPeerSettingsJournaled([]peerSettingsSwap{{
		key: fx.peer.Settings().PeerKey(), next: next, apply: copyImport}}, j)
}

func relaxJournal(fx *rfc8212ImportFixture, txID string) *rfc8212TxJournal {
	return &rfc8212TxJournal{Journal: sdk.NewJournal(), txID: txID, deadline: fx.f.clock.Now().Add(30 * time.Second)}
}

func importAccepts(p *Peer) bool {
	s := p.settingsSnapshot()
	return s.ImportPolicy.AcceptsAtEnd(s.IsEBGP())
}

// refreshesOnWire counts the ROUTE-REFRESH messages Ze wrote to the session
// until no message arrives for a short while.
func refreshesOnWire(t *testing.T, fx *rfc8212ImportFixture) int {
	t.Helper()
	count := 0
	for {
		select {
		case typ := <-fx.f.wire:
			if typ == msgtype.TypeROUTEREFRESH {
				count++
			}
		case <-time.After(300 * time.Millisecond):
			return count
		}
	}
}

// TestRFC8212RelaxWithoutRouteRefreshRefused proves D-1's refusal at both
// stages. Method: an Established EBGP peer whose import default rejects and
// whose session did not negotiate Route Refresh is offered an accepting
// import policy, first at verify (the operation's verify, and the section
// path's VerifyConfig over a reload function), then at apply.
// VALIDATES: every stage refuses naming the disable, commit, enable sequence;
// the running policy still rejects; no source-policy owner was called and
// the journal recorded nothing.
// PREVENTS: an accepting policy reported live while the routes the default
// discarded never return (RFC 8212 Section 3, D-1).
func TestRFC8212RelaxWithoutRouteRefreshRefused(t *testing.T) {
	fx := newRelaxFixture(t, false)
	next := fx.peer.settingsSnapshot()
	next.ImportPolicy = mustPolicy(t, nil, routepolicy.FinalAccept)
	adapter := &reactorAPIAdapter{r: fx.r}
	const sequence = "`request peer 192.0.2.1 disable`, wait until the session is down, commit this change, then `request peer 192.0.2.1 enable`"

	require.ErrorContains(t, adapter.verifyImportRelaxation(next, true), sequence)
	fx.r.config.ConfigPath = "relax.conf"
	fx.r.SetReloadFunc(func(string) ([]*PeerSettings, Globals, []*DynamicGroupConfig, error) {
		return []*PeerSettings{next}, Globals{}, nil, nil
	})
	require.ErrorContains(t, adapter.VerifyConfig(map[string]any{"bgp": map[string]any{}}), sequence)

	j := relaxJournal(fx, "tx-no-refresh")
	err := relax(fx, j)
	require.ErrorContains(t, err, sequence)
	require.False(t, importAccepts(fx.peer), "the refused change was published")
	require.Zero(t, j.Len(), "the refused change was journaled")
	ops, _, eligible, _ := fx.owner.seen()
	require.Empty(t, ops, "a source-policy owner was called for a refused change")
	require.True(t, eligible)
	require.Zero(t, refreshesOnWire(t, fx), "ROUTE-REFRESH sent to a peer that did not negotiate it")
}

// TestRFC8212RelaxRequestsEveryFamily proves D-1's recovery. Method: the same
// peer with Route Refresh negotiated over IPv4 and IPv6 unicast takes the
// accepting policy through a transaction, which then commits.
// VALIDATES: the policy accepts, exactly one ROUTE-REFRESH per negotiated
// family reached the session's wire, the session and its generation are
// unchanged, and no owner was asked to invalidate anything.
// PREVENTS: an accepting policy whose denied routes are never asked for
// again (RFC 7313 Section 3.2, normal route refresh request).
func TestRFC8212RelaxRequestsEveryFamily(t *testing.T) {
	fx := newRelaxFixture(t, true)
	j := relaxJournal(fx, "tx-refresh")
	err := relax(fx, j)
	require.NoError(t, err)
	require.True(t, importAccepts(fx.peer))
	require.Equal(t, 2, refreshesOnWire(t, fx), "one ROUTE-REFRESH per negotiated family")
	require.Same(t, fx.f.s, fx.peer.currentSession())
	require.Equal(t, fx.generation, fx.peer.forwardGeneration.Load())
	require.NoError(t, fx.r.CompleteSourcePolicyTx(t.Context(), j.txID, true))
	j.Discard()
	ops, _, _, _ := fx.owner.seen()
	require.Empty(t, ops, "an accepted relaxation invalidated the source")
}

// TestRFC8212RelaxReplacedSessionRefused proves the apply judges the session
// it holds, not the one verify judged. Method: verify passes on a session
// with Route Refresh; the session is then replaced by one that negotiated
// none, as a reconnect between verify and apply does.
// VALIDATES: the apply refuses, the policy still rejects, no ROUTE-REFRESH
// went out, and no owner was called.
// PREVENTS: a recovery request sent to, or skipped on, a session nobody judged.
func TestRFC8212RelaxReplacedSessionRefused(t *testing.T) {
	fx := newRelaxFixture(t, true)
	next := fx.peer.settingsSnapshot()
	next.ImportPolicy = mustPolicy(t, nil, routepolicy.FinalAccept)
	require.NoError(t, (&reactorAPIAdapter{r: fx.r}).verifyImportRelaxation(next, true))

	successor := newAdmissionSession(t)
	fx.peer.mu.Lock()
	fx.peer.session = successor.s
	fx.peer.mu.Unlock()
	fx.peer.negotiated.Store(relaxNegotiated(false))

	err := relax(fx, relaxJournal(fx, "tx-replaced"))
	require.ErrorContains(t, err, "did not negotiate Route Refresh")
	require.False(t, importAccepts(fx.peer))
	require.Zero(t, refreshesOnWire(t, fx))
	ops, _, _, _ := fx.owner.seen()
	require.Empty(t, ops)
}

// relaxBudgetConn fails every write once its budget, when armed, is spent,
// and runs onFail, when set, before each failed write returns.
type relaxBudgetConn struct {
	net.Conn
	armed  atomic.Bool
	budget atomic.Int32
	onFail func()
}

func (c *relaxBudgetConn) Write(b []byte) (int, error) {
	if c.armed.Load() && c.budget.Add(-1) < 0 {
		if c.onFail != nil {
			c.onFail()
		}
		return 0, errors.New("injected write failure")
	}
	return c.Conn.Write(b)
}

// TestRFC8212RelaxPartialSendFails proves a partial recovery fails the apply
// and leaves nothing accepting. Method: the session's connection lets one
// more write through, then fails, so the first family's ROUTE-REFRESH is
// sent and the second is not.
// VALIDATES: the apply fails naming the family whose request was not sent;
// the policy rejects again and nothing was journaled. The failed write also
// ended the session's ability to pause its input, so the undo reports its
// compensation as incomplete, naming the hold-down sequence, rather than as a
// rollback (the apply's own report never says the change was undone), and no
// owner was asked to prepare.
// PREVENTS: a commit reported applied while half the denied routes can never
// return, a failed relaxation that leaves the accepting policy behind, or a
// compensation that failed reported as complete.
func TestRFC8212RelaxPartialSendFails(t *testing.T) {
	fx := newRelaxFixture(t, true)
	var conn *relaxBudgetConn
	fx.f = newAdmissionSessionWithConn(t, func(c net.Conn) net.Conn {
		conn = &relaxBudgetConn{Conn: c}
		return conn
	})
	fx.peer.mu.Lock()
	fx.peer.session = fx.f.s
	fx.peer.mu.Unlock()
	fx.owner.onPrepare, fx.owner.onApply = nil, nil
	conn.budget.Store(1)
	conn.armed.Store(true)

	j := relaxJournal(fx, "tx-partial")
	err := relax(fx, j)
	require.ErrorContains(t, err, "ROUTE-REFRESH for ipv6/unicast not sent")
	require.ErrorContains(t, err, "could not be withdrawn and stay eligible; run `request peer 192.0.2.1 disable`")
	require.ErrorContains(t, err, "undoing the change did not complete")
	require.NotContains(t, err.Error(), "so the change was undone", "an incomplete undo reported as complete")
	require.False(t, importAccepts(fx.peer), "a failed relaxation left the accepting policy")
	require.Zero(t, j.Len(), "a failed apply was journaled")
	require.NoError(t, fx.r.CompleteSourcePolicyTx(t.Context(), j.txID, false))
	ops, _, _, _ := fx.owner.seen()
	require.Empty(t, ops, "an owner was prepared although the input could not be paused")
	require.False(t, fx.f.s.IsPaused(), "input left paused")
}

// TestRFC8212RelaxUndoFencesAdmittedPaths proves the "Failed import
// relaxation" boundary. Method: the relaxation applies with Route Refresh
// (the owner's path stands for a route admitted under it); the transaction
// then rolls back. While the undo holds input paused, the peer sends one more
// UPDATE, a late answer to the ROUTE-REFRESH; the rejection receipt follows.
// VALIDATES: the owner made the admitted path ineligible (prepare, apply,
// finalize) under a fresh epoch; the late UPDATE was admitted once, under the
// restored rejecting policy and that epoch; the session was kept.
// PREVENTS: routes admitted under a rolled-back accepting policy staying
// eligible, or a late refresh answer repopulating them (RFC 8212 Section 3).
func TestRFC8212RelaxUndoFencesAdmittedPaths(t *testing.T) {
	fx := newRelaxFixture(t, true)
	j := relaxJournal(fx, "tx-undo")
	err := relax(fx, j)
	require.NoError(t, err)
	require.True(t, importAccepts(fx.peer))
	require.Equal(t, 2, refreshesOnWire(t, fx))

	require.Empty(t, j.Rollback())
	require.True(t, fx.f.s.IsPaused(), "input resumed before the transaction's receipt")
	require.NoError(t, fx.r.CompleteSourcePolicyTx(t.Context(), j.txID, false))
	ops, _, eligible, epoch := fx.owner.seen()
	require.Equal(t, []rpc.SourcePolicyOperation{rpc.SourcePolicyPrepare, rpc.SourcePolicyApply, rpc.SourcePolicyFinalize}, ops)
	require.False(t, eligible, "a path admitted under the undone relaxation stays eligible")
	require.Equal(t, fx.original, epoch, "the owner restamped a path it should have excluded")
	require.Greater(t, fx.peer.sourcePolicyEpoch.Load(), fx.original, "the undo did not fence with a fresh epoch")
	fx.settled(t, false)
}

// TestRFC8212DynamicTemplateRelaxation proves a dynamic group's template
// takes the same import transitions as a configured peer. Method: one group
// template edit at a time, through the verify check and the reload's dynamic
// step on a journal of no config transaction, on an Established dynamic EBGP
// peer and on a dynamic peer that is not yet Established. The Established
// rows cover a session with no published negotiation, one whose negotiation
// lacks Route Refresh, and a fall.
// VALIDATES: on the Established peer, lifting the rejection is refused at
// verify and at apply (with no negotiation, as unjudgeable; without Route
// Refresh, naming the disable, commit, enable sequence), and removing the
// accepting policy is refused at apply, each leaving the running policy and
// the group set; on the peer that is not Established both edits publish,
// since it admits nothing yet.
// PREVENTS: a dynamic template edit bypassing D-1 and D-2 that a configured
// peer's identical edit takes.
func TestRFC8212DynamicTemplateRelaxation(t *testing.T) {
	accept := mustPolicy(t, nil, routepolicy.FinalAccept)
	for _, tc := range []struct {
		name        string
		from, to    filterapi.Policy
		established bool
		negotiated  *NegotiatedCapabilities
		refused     string
	}{
		{name: "established rise", to: accept, established: true, refused: "not negotiated capabilities"},
		{
			name: "established rise without route refresh", to: accept, established: true, negotiated: relaxNegotiated(false),
			refused: "`request peer 185.1.69.42 disable`, wait until the session is down, commit this change, then `request peer 185.1.69.42 enable`",
		},
		{name: "established fall", from: accept, established: true, refused: "needs a config transaction operation"},
		{name: "idle rise", to: accept},
		{name: "idle fall", from: accept},
	} {
		t.Run(tc.name, func(t *testing.T) {
			group := dynamicReloadGroup(t)
			group.Settings.ImportPolicy = tc.from
			r, peer := newDynamicReloadReactor(t, group, netip.MustParseAddr("185.1.69.42"), nil)
			if tc.established {
				establishDynamicPeer(t, r, peer)
				peer.state.Store(int32(PeerStateEstablished))
				if tc.negotiated != nil {
					peer.negotiated.Store(tc.negotiated)
				}
			} else {
				peer.state.Store(int32(PeerStateActive))
			}
			next := dynamicReloadGroup(t)
			next.Settings.ImportPolicy = tc.to
			adapter := &reactorAPIAdapter{r: r}
			verify := adapter.verifyDynamicRelaxations(dynamicGroupsReadAs([]*DynamicGroupConfig{next}))
			err := adapter.reconcileDynamicGroupsJournaled([]*DynamicGroupConfig{next}, "test", &internalJournal{})
			template := peer.readDynamicTemplate()
			if tc.refused == "" {
				require.NoError(t, verify)
				require.NoError(t, err)
				require.Equal(t, tc.to, template.importTemplate)
				return
			}
			if tc.to.AcceptsAtEnd(true) {
				require.ErrorContains(t, verify, tc.refused)
			}
			require.ErrorContains(t, err, tc.refused)
			require.Equal(t, tc.from, template.importTemplate, "the refused template was published")
			r.mu.RLock()
			groups := r.dynamicGroups
			r.mu.RUnlock()
			require.Equal(t, []*DynamicGroupConfig{group}, groups, "the refused step left the new group set")
			require.Same(t, peer, r.Peers()[0])
		})
	}
}

// TestRFC8212RelaxUndoHoldExpires proves the pause of a relaxation's undo is
// bounded. Method: the relaxation applies with Route Refresh, the
// transaction rolls back, and no receipt ever arrives; the session's fake
// clock is advanced past the receipt's completion deadline.
// VALIDATES: input stays paused until the deadline and resumes after it; the
// owner was finalized, so the admitted path stays ineligible under the
// restored rejecting policy; the late receipt reports the expiry.
// PREVENTS: a lost rejection receipt leaving the session's input paused until
// its HoldTimer kills it.
func TestRFC8212RelaxUndoHoldExpires(t *testing.T) {
	fx := newRelaxFixture(t, true)
	j := relaxJournal(fx, "tx-undo-expiry")
	err := relax(fx, j)
	require.NoError(t, err)
	require.Equal(t, 2, refreshesOnWire(t, fx))

	require.Empty(t, j.Rollback())
	held := fx.r.sourcePolicy.transaction(j.txID)
	require.Len(t, held, 1)
	completion := held[0].receipt.CompletionDeadline()
	require.True(t, fx.f.s.IsPaused(), "input resumed before the transaction's receipt")
	fx.f.clock.Add(completion.Sub(fx.f.clock.Now()) - time.Millisecond)
	require.True(t, fx.f.s.IsPaused(), "hold released before its completion deadline")
	fx.f.clock.Add(2 * time.Millisecond)
	require.Eventually(t, func() bool { return !fx.f.s.IsPaused() }, 5*time.Second, 5*time.Millisecond,
		"input stayed paused past the completion deadline")
	err = fx.r.CompleteSourcePolicyTx(t.Context(), j.txID, false)
	require.ErrorContains(t, err, "completion deadline")
	ops, _, eligible, _ := fx.owner.seen()
	require.Equal(t, []rpc.SourcePolicyOperation{rpc.SourcePolicyPrepare, rpc.SourcePolicyApply, rpc.SourcePolicyFinalize}, ops)
	require.False(t, eligible, "the expired undo restored the path it excluded")
	require.False(t, importAccepts(fx.peer))
	fx.settled(t, false)
}

// TestRFC8212RelaxUndoApplyFailureKeepsRejection proves a relaxation's undo
// whose invalidation fails after it published the rejecting policy keeps that
// policy and reports the paths it could not exclude. Method: the relaxation
// applies with Route Refresh; on rollback the owner refuses the
// invalidation's apply every time, as an owner that lost its pin does. The
// fixture owner refuses a finalize no apply preceded, as every real owner's
// ledger does (sourcepolicy Owner.ordered).
// VALIDATES: the undo retries the apply, then aborts the owner with a zero
// target (it never applied, so it cannot be finalized); the rollback names
// that owner, says its routes stay eligible and gives the hold-down sequence;
// the policy still rejects; input stays paused until the rejection receipt,
// which succeeds and resumes it.
// PREVENTS: a failed undo re-publishing the accepting candidate while the
// config rolled back to reject; a report claiming the paths leave
// eligibility when the transaction ends while an owner never excluded them;
// a receipt finalizing an owner that never applied, which every real owner
// refuses, leaving its ledger open.
func TestRFC8212RelaxUndoApplyFailureKeepsRejection(t *testing.T) {
	fx := newRelaxFixture(t, true)
	fx.owner.onApply = func() {
		fx.owner.mu.Lock()
		fx.owner.pinned = false
		fx.owner.mu.Unlock()
	}
	j := relaxJournal(fx, "tx-undo-fails")
	err := relax(fx, j)
	require.NoError(t, err)
	require.Equal(t, 2, refreshesOnWire(t, fx))

	errs := j.Rollback()
	require.Len(t, errs, 1)
	require.ErrorContains(t, errs[0], "the rejecting import policy is restored, but source-policy owner "+
		rfc8212ImportOwnerName+" did not withdraw the routes the undone change admitted, so they stay eligible")
	require.ErrorContains(t, errs[0], "`request peer 192.0.2.1 disable`, then `request peer 192.0.2.1 enable`")
	require.NotContains(t, errs[0].Error(), "once the transaction ends", "an unexcluded path reported as leaving eligibility")
	require.False(t, importAccepts(fx.peer), "the failed undo re-published the accepting policy")
	require.True(t, fx.f.s.IsPaused(), "input resumed before the transaction's receipt")
	require.NoError(t, fx.r.CompleteSourcePolicyTx(t.Context(), j.txID, false))
	ops, targets, _, _ := fx.owner.seen()
	require.Equal(t, []rpc.SourcePolicyOperation{rpc.SourcePolicyPrepare, rpc.SourcePolicyApply,
		rpc.SourcePolicyApply, rpc.SourcePolicyAbort}, ops)
	require.Equal(t, targets[1], targets[2], "the retry named another epoch than the apply")
	require.Zero(t, targets[3], "the never-applied owner was aborted with a target epoch")
	require.False(t, importAccepts(fx.peer))
	require.False(t, fx.f.s.IsPaused(), "input left paused after the receipt")
}

// TestRFC8212RelaxRefreshFailureOnDownSessionPurges proves the paths Graceful
// Restart retained for a peer whose session went down while its relaxation's
// ROUTE-REFRESH failed are purged. Method: a plugin answering the RIB's
// purge-stale command holds one retained path for the peer; the session's
// connection fails the first ROUTE-REFRESH write and, as it fails, the peer
// loses its session, as a teardown does.
// VALIDATES: the apply fails reporting the change undone; the policy rejects;
// the retained path was purged once, after the rejecting policy was restored.
// PREVENTS: candidate-admitted paths kept stale and eligible by Graceful
// Restart under the restored rejecting policy (RFC 8212 Section 3).
func TestRFC8212RelaxRefreshFailureOnDownSessionPurges(t *testing.T) {
	const name = "rfc8212-relax-retaining-rib"
	rib := &rfc8212RetainingRIB{retained: map[string]bool{"192.0.2.1": true}}
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	require.NoError(t, registry.Register(registry.Registration{
		Name: name, Description: "RFC 8212 relaxation retained-path RIB fixture",
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(name, conn)
			p.OnExecuteCommand(func(_, _ string, args []string, _ string) (string, any, error) {
				rib.purge(args)
				return rpc.StatusDone, map[string]any{"purged": 1}, nil
			})
			if err := p.Run(context.Background(), &sdk.Registration{
				Commands: []sdk.CommandDecl{{Name: retainedPurgeCommand}},
			}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))
	fx := newRFC8212ImportFixtureWith(t, false, name)
	fx.peer.mu.Lock()
	fx.peer.settings.ImportPolicy = filterapi.Policy{}
	fx.peer.mu.Unlock()
	fx.peer.negotiated.Store(relaxNegotiated(true))
	rib.mu.Lock()
	rib.peer = fx.peer
	rib.mu.Unlock()
	process := fx.r.api.ProcessManager().GetProcess(name)
	require.NotNil(t, process)
	lowEventually(t, process.Bridge().HasExecuteCommand, "purge-stale callback published")

	var conn *relaxBudgetConn
	fx.f = newAdmissionSessionWithConn(t, func(c net.Conn) net.Conn {
		conn = &relaxBudgetConn{Conn: c}
		return conn
	})
	fx.peer.mu.Lock()
	fx.peer.session = fx.f.s
	fx.peer.mu.Unlock()
	// A teardown leaves Established before it clears the session
	// (Peer.sessionDown, then runOnce's deferred clear).
	conn.onFail = func() {
		fx.peer.setState(PeerStateConnecting)
		fx.peer.mu.Lock()
		fx.peer.session = nil
		fx.peer.mu.Unlock()
	}
	conn.armed.Store(true)

	j := relaxJournal(fx, "tx-down")
	err := relax(fx, j)
	require.ErrorContains(t, err, "not sent")
	require.ErrorContains(t, err, "so the change was undone")
	require.False(t, importAccepts(fx.peer))
	require.Zero(t, j.Len())
	rib.mu.Lock()
	defer rib.mu.Unlock()
	require.Empty(t, rib.retained, "a path the candidate admitted stays retained and eligible")
	require.Equal(t, []bool{false}, rib.accepts, "purge ran before the rejecting policy was restored, or more than once")
}

// TestRFC8212DynamicRefusalTearsNothingDown proves the dynamic step refuses an
// import transition before any irreversible step. Method: one step removes a
// whole group, whose dynamic peer it would stop, and lifts the rejecting
// import policy of another group's Established peer whose session has no
// negotiation, so the rise is refused.
// VALIDATES: the step fails naming the refusal; the removed group's peer is
// still the same object in the peer map, and the group set is unchanged.
// PREVENTS: peers torn down by a step whose import transition is refused only
// at apply, after its stops already ran.
func TestRFC8212DynamicRefusalTearsNothingDown(t *testing.T) {
	group := dynamicReloadGroup(t)
	group.Settings.ImportPolicy = filterapi.Policy{}
	r, peer := newDynamicReloadReactor(t, group, netip.MustParseAddr("185.1.69.42"), nil)
	establishDynamicPeer(t, r, peer)
	peer.state.Store(int32(PeerStateEstablished))

	gone := newTestDynamicGroup("gone", []string{"198.51.100.0/24"}, 10)
	gone.Settings.Connection = ConnectionPassive
	r.mu.Lock()
	r.dynamicGroups = append(r.dynamicGroups, gone)
	doomed, err := r.createDynamicPeer(gone, netip.MustParseAddr("198.51.100.7"))
	r.mu.Unlock()
	require.NoError(t, err)

	next := dynamicReloadGroup(t)
	next.Settings.ImportPolicy = mustPolicy(t, nil, routepolicy.FinalAccept)
	err = (&reactorAPIAdapter{r: r}).reconcileDynamicGroupsJournaled([]*DynamicGroupConfig{next}, "test", &internalJournal{})
	require.ErrorContains(t, err, "not negotiated capabilities")
	r.mu.RLock()
	groups := r.dynamicGroups
	kept := slices.Contains(slices.Collect(maps.Values(r.peers)), doomed)
	r.mu.RUnlock()
	require.True(t, kept, "the refused step removed another group's peer")
	require.Equal(t, []*DynamicGroupConfig{group, gone}, groups, "the refused step left the new group set")
	require.Equal(t, filterapi.Policy{}, peer.readDynamicTemplate().importTemplate)
}

// makeDynamic turns the fixture's peer into a member of dynamic group "ix",
// whose template carries import policy from, installed as the reactor's only
// group, and returns that group.
func makeDynamic(t *testing.T, fx *rfc8212ImportFixture, from filterapi.Policy) *DynamicGroupConfig {
	t.Helper()
	group := dynamicNext(fx, from)
	fx.peer.mu.Lock()
	fx.peer.settings.IsDynamic = true
	fx.peer.settings.GroupName = group.GroupName
	fx.peer.dynImportPolicy = from
	fx.peer.mu.Unlock()
	fx.r.mu.Lock()
	fx.r.dynamicGroups = []*DynamicGroupConfig{group}
	fx.r.mu.Unlock()
	return group
}

// dynamicNext is the template of group "ix" with import policy to. It carries
// the fixture peer's process bindings, so a swap that delivers the template's
// bindings keeps the source-policy owner attached.
func dynamicNext(fx *rfc8212ImportFixture, to filterapi.Policy) *DynamicGroupConfig {
	next := newTestDynamicGroup("ix", []string{"192.0.2.0/24"}, 10)
	next.Settings.Connection = ConnectionPassive
	next.Settings.ImportPolicy = to
	next.Settings.ProcessBindings = fx.peer.Settings().ProcessBindings
	return next
}

// TestRFC8212DynamicRelaxUnderTransaction proves a dynamic template's rise
// takes D-1's recovery and undo inside a config transaction. Method: an
// Established dynamic EBGP peer whose session negotiated Route Refresh over
// IPv4 and IPv6 unicast takes its group's accepting template through the
// dynamic step on a transaction journal; the transaction then rolls back
// while one late UPDATE arrives, and the rejection receipt follows.
// VALIDATES: the policy accepts, one ROUTE-REFRESH per negotiated family went
// out on the same session; the rollback restores rejection through the
// in-place invalidation (prepare, apply, finalize under a fresh epoch), keeps
// the peer and its session, and admits the late UPDATE once under reject.
// PREVENTS: a dynamic rise skipping the recovery or the undo a configured
// peer's identical change takes (RFC 8212 Section 3).
func TestRFC8212DynamicRelaxUnderTransaction(t *testing.T) {
	fx := newRelaxFixture(t, true)
	group := makeDynamic(t, fx, filterapi.Policy{})
	j := relaxJournal(fx, "tx-dynamic-rise")
	adapter := &reactorAPIAdapter{r: fx.r}
	require.NoError(t, adapter.reconcileDynamicGroupsJournaled(
		[]*DynamicGroupConfig{dynamicNext(fx, mustPolicy(t, nil, routepolicy.FinalAccept))}, "test", j))
	require.True(t, importAccepts(fx.peer))
	require.Equal(t, 2, refreshesOnWire(t, fx), "one ROUTE-REFRESH per negotiated family")

	require.Empty(t, j.Rollback())
	require.True(t, fx.f.s.IsPaused(), "input resumed before the transaction's receipt")
	require.NoError(t, fx.r.CompleteSourcePolicyTx(t.Context(), j.txID, false))
	ops, _, eligible, _ := fx.owner.seen()
	require.Equal(t, []rpc.SourcePolicyOperation{rpc.SourcePolicyPrepare, rpc.SourcePolicyApply, rpc.SourcePolicyFinalize}, ops)
	require.False(t, eligible, "a path admitted under the undone relaxation stays eligible")
	require.Greater(t, fx.peer.sourcePolicyEpoch.Load(), fx.original, "the undo did not fence with a fresh epoch")
	require.Same(t, fx.peer, fx.r.Peers()[0])
	fx.r.mu.RLock()
	require.Equal(t, []*DynamicGroupConfig{group}, fx.r.dynamicGroups)
	fx.r.mu.RUnlock()
	fx.settled(t, false)
}

// TestRFC8212DynamicFallUnderTransaction proves a dynamic template's fall takes
// D-2's in-place invalidation inside a config transaction. Method: an
// Established dynamic EBGP peer whose template accepts takes its group's
// template with no import policy through the dynamic step on a transaction
// journal, which then rolls back; the rejection receipt follows.
// VALIDATES: the policy rejects while input is paused; the rollback aborts
// the invalidation (prepare, apply, abort) and restores the accepting
// template, keeping the peer and its session, and the paused UPDATE is
// admitted once under the restored policy.
// PREVENTS: a dynamic fall refused, or applied by restarting the session,
// where a configured peer's identical change invalidates in place.
func TestRFC8212DynamicFallUnderTransaction(t *testing.T) {
	fx := newRFC8212ImportFixture(t, false)
	accept := fx.peer.Settings().ImportPolicy
	group := makeDynamic(t, fx, accept)
	j := relaxJournal(fx, "tx-dynamic-fall")
	adapter := &reactorAPIAdapter{r: fx.r}
	require.NoError(t, adapter.reconcileDynamicGroupsJournaled(
		[]*DynamicGroupConfig{dynamicNext(fx, filterapi.Policy{})}, "test", j))
	require.False(t, importAccepts(fx.peer))
	require.True(t, fx.f.s.IsPaused(), "input resumed before the transaction's receipt")

	require.Empty(t, j.Rollback())
	require.True(t, fx.f.s.IsPaused(), "input resumed before the whole rollback completed")
	require.NoError(t, fx.r.CompleteSourcePolicyTx(t.Context(), j.txID, false))
	ops, _, eligible, _ := fx.owner.seen()
	require.Equal(t, []rpc.SourcePolicyOperation{rpc.SourcePolicyPrepare, rpc.SourcePolicyApply, rpc.SourcePolicyAbort}, ops)
	require.True(t, eligible, "the rollback left the accepted path ineligible")
	require.Equal(t, accept, fx.peer.readDynamicTemplate().importTemplate)
	require.Same(t, fx.peer, fx.r.Peers()[0])
	fx.r.mu.RLock()
	require.Equal(t, []*DynamicGroupConfig{group}, fx.r.dynamicGroups)
	fx.r.mu.RUnlock()
	fx.settled(t, true)
}
