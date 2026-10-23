// Design: docs/architecture/core-design.md — section 18a, a dynamic peer takes the configured peer's swap
// Related: reactor_dynamic.go — reconcileDynamicGroupsJournaled, the step under test
// Related: peer_settings_apply.go — applyHotSwappableSettings, the copier and ordered delivery it shares

package reactor

import (
	"net/netip"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// reloadDynamicGroups runs the reload's journaled dynamic step over groups, the
// entry point a reload reaches the dynamic population through, and returns the
// journal so a test can roll it back.
func reloadDynamicGroups(t *testing.T, r *Reactor, groups ...*DynamicGroupConfig) *internalJournal {
	t.Helper()
	j := &internalJournal{}
	require.NoError(t, (&reactorAPIAdapter{r: r}).reconcileDynamicGroupsJournaled(groups, "test", j))
	return j
}

// dynamicPeerRemoteAS is the AS every dynamic peer in this file learns from its
// OPEN, so each resolved `$remote_as` filter name in the assertions ends in it.
const dynamicPeerRemoteAS uint16 = 65010

// establishDynamicPeer gives peer an Established session whose OPEN carried
// dynamicPeerRemoteAS, resolves the template against it as the establishment
// goroutine does, and wires a recorder in place of the event dispatcher.
func establishDynamicPeer(t *testing.T, r *Reactor, peer *Peer) *bindRecorder {
	t.Helper()
	srv := &pluginserver.Server{}
	r.api = srv
	rec := &bindRecorder{t: t, srv: srv, conn: &recordingConn{}}
	r.bindEvents = rec

	session := NewSession(peer.settings)
	session.mu.Lock()
	session.peerOpen = &message.Open{MyAS: dynamicPeerRemoteAS, BGPIdentifier: 0x0a0b0c0d}
	session.mu.Unlock()
	peer.mu.Lock()
	peer.session = session
	// The plugins were told the session is up (Peer.announceEstablished).
	peer.upSession = session
	peer.upPhase = sessionUpDelivered
	peer.mu.Unlock()
	peer.resolveDynamicPeerSettings(session)

	r.mu.Lock()
	r.publishDeliveryGraphLocked()
	r.mu.Unlock()
	return rec
}

// TestDynamicTemplateReceiveOnlySwap proves AC-9 at the step: a dynamic group
// template that gains a receive-only binding keeps the dynamic session, and the
// gained process is told the session is up with reason attached. The second
// pass the transaction reload runs (ApplyConfigDiff after the commit) compares
// templates that are now equal and delivers nothing (N5).
//
// VALIDATES: AC-9, the dynamic step swaps a receive-only binding in place.
// PREVENTS: every dynamic peer of a group restarting when an attach is added.
// DISCRIMINATION: peer object identity and session identity; a restart deletes
// the map entry. The recorder counts the targeted ups, so a second pass that
// re-delivered would read two.
func TestDynamicTemplateReceiveOnlySwap(t *testing.T) {
	addr := netip.MustParseAddr("185.1.69.42")
	r, peer := newDynamicReloadReactor(t, dynamicReloadGroup(t), addr, nil)
	rec := establishDynamicPeer(t, r, peer)
	session := peer.currentSession()

	changed := dynamicReloadGroup(t)
	changed.Settings.ProcessBindings = []ProcessBinding{operatorBinding(t, "monitor", "update-received state", "")}
	reloadDynamicGroups(t, r, changed)

	key := netip.AddrPortFrom(addr, DefaultBGPPort)
	r.mu.RLock()
	after := r.peers[key]
	r.mu.RUnlock()
	require.True(t, after == peer, "a receive-only template binding must not restart the dynamic peer")
	assert.True(t, after.currentSession() == session, "the session must be the same session")
	require.Len(t, peer.settingsSnapshot().ProcessBindings, 1, "the binding must reach the running peer")
	assert.Equal(t, "monitor", peer.settingsSnapshot().ProcessBindings[0].PluginName)
	assert.Equal(t, []filterapi.FilterRef{{Name: "policy:in-65010"}}, peer.ImportPolicy().Filters(),
		"the running chain must stay resolved against the learned AS")

	steps := rec.recorded()
	require.Len(t, steps, 1, "one targeted up for the gained process")
	assert.Equal(t, "up", steps[0].event)
	assert.Equal(t, rpc.ReasonAttached, steps[0].reason)
	assert.Equal(t, []string{"monitor"}, steps[0].processes)

	// The second pass: the same template again, rebuilt.
	again := dynamicReloadGroup(t)
	again.Settings.ProcessBindings = []ProcessBinding{operatorBinding(t, "monitor", "update-received state", "")}
	reloadDynamicGroups(t, r, again)
	assert.Len(t, rec.recorded(), 1, "the second pass must deliver no second up")
	r.mu.RLock()
	assert.True(t, r.peers[key] == peer, "the second pass must not restart the peer")
	r.mu.RUnlock()
}

// TestDynamicFilterSwapResolvesAgainstLearnedAS proves AC-22 and R-14: a group
// template whose import chain changed swaps onto the running dynamic peer, and
// the chain the peer enforces is the new template resolved against the AS the
// peer learned from its OPEN, while the template pair holds the new chain
// unresolved for the next establishment. It replaces
// TestSetDynamicGroupsRemovesThePeerWhenTheTemplateChanges's filter case, which
// pinned a restart (owner decision OQ-2, option A).
//
// A peer that has not learned its AS keeps the chain unresolved, and its
// establishment resolves it from the new pair.
//
// VALIDATES: the swap resolves under p.mu against the AS current at the write.
// PREVENTS: an unresolved $remote_as chain enforced on an established peer.
func TestDynamicFilterSwapResolvesAgainstLearnedAS(t *testing.T) {
	strict := func() *DynamicGroupConfig {
		g := dynamicReloadGroup(t)
		g.Settings.ImportPolicy = mustPolicy(t, []filterapi.FilterRef{{Name: "policy:strict-$remote_as"}}, filterapi.FinalAccept)
		return g
	}

	t.Run("learned", func(t *testing.T) {
		addr := netip.MustParseAddr("185.1.69.42")
		r, peer := newDynamicReloadReactor(t, dynamicReloadGroup(t), addr, nil)
		establishDynamicPeer(t, r, peer)

		reloadDynamicGroups(t, r, strict())

		r.mu.RLock()
		after := r.peers[netip.AddrPortFrom(addr, DefaultBGPPort)]
		r.mu.RUnlock()
		require.True(t, after == peer, "a filter template change must swap, not restart")
		assert.Equal(t, []filterapi.FilterRef{{Name: "policy:strict-65010"}}, peer.ImportPolicy().Filters())
		peer.mu.RLock()
		template := peer.dynImportPolicy.Filters()
		peer.mu.RUnlock()
		assert.Equal(t, []filterapi.FilterRef{{Name: "policy:strict-$remote_as"}}, template,
			"the template pair must hold the new chain unresolved")
	})

	t.Run("not yet learned", func(t *testing.T) {
		addr := netip.MustParseAddr("185.1.69.43")
		r, peer := newDynamicReloadReactor(t, dynamicReloadGroup(t), addr, nil)
		session := NewSession(peer.settings)
		peer.mu.Lock()
		peer.session = session
		peer.mu.Unlock()

		reloadDynamicGroups(t, r, strict())
		assert.Equal(t, []filterapi.FilterRef{{Name: "policy:strict-$remote_as"}}, peer.ImportPolicy().Filters(),
			"a peer with no AS yet keeps the chain unresolved")

		session.mu.Lock()
		session.peerOpen = &message.Open{MyAS: 65020, BGPIdentifier: 0x0a0b0c0d}
		session.mu.Unlock()
		peer.resolveDynamicPeerSettings(session)
		assert.Equal(t, []filterapi.FilterRef{{Name: "policy:strict-65020"}}, peer.ImportPolicy().Filters(),
			"establishment must resolve the chain from the new template pair")
	})
}

// TestDynamicPolicyResolutionPreservesFinal checks both empty policies and
// variable-bearing child policies through construction and OPEN resolution.
func TestDynamicPolicyResolutionPreservesFinal(t *testing.T) {
	for _, tc := range []struct {
		name       string
		importRefs []filterapi.FilterRef
		exportRefs []filterapi.FilterRef
		wantImport []filterapi.FilterRef
		wantExport []filterapi.FilterRef
	}{
		{name: "final only"},
		{
			name:       "substitute child variables",
			importRefs: frefs("policy:in-$remote_as"),
			exportRefs: frefs("policy:out-$remote_as"),
			wantImport: frefs("policy:in-65010"),
			wantExport: frefs("policy:out-65010"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			group := dynamicReloadGroup(t)
			group.Settings.ImportPolicy = mustPolicy(t, tc.importRefs, filterapi.FinalReject)
			group.Settings.ExportPolicy = mustPolicy(t, tc.exportRefs, filterapi.FinalReject)
			r, peer := newDynamicReloadReactor(t, group, netip.MustParseAddr("185.1.69.42"), nil)
			assert.Equal(t, group.Settings.ImportPolicy, peer.ImportPolicy())
			assert.Equal(t, group.Settings.ExportPolicy, peer.ExportPolicy())

			establishDynamicPeer(t, r, peer)
			peer.refreshForwardFacts()
			wantImport := mustPolicy(t, tc.wantImport, filterapi.FinalReject)
			wantExport := mustPolicy(t, tc.wantExport, filterapi.FinalReject)
			assert.Equal(t, wantImport, peer.ImportPolicy())
			assert.Equal(t, wantExport, peer.ExportPolicy())
			require.NotNil(t, peer.forwardFacts())
			assert.Equal(t, wantExport, peer.forwardFacts().exportPolicy)
			template := peer.readDynamicTemplate()
			assert.Equal(t, group.Settings.ImportPolicy, template.importTemplate)
			assert.Equal(t, group.Settings.ExportPolicy, template.exportTemplate)
			assert.Equal(t, tc.importRefs, group.Settings.ImportPolicy.Filters(), "resolution must not mutate the group")
			assert.Equal(t, tc.exportRefs, group.Settings.ExportPolicy.Filters(), "resolution must not mutate the group")
		})
	}
}

// TestDynamicPolicyFinalSwapRollbackPreservesSession checks empty policy
// transitions and removal without replacing the learned peer or its session.
func TestDynamicPolicyFinalSwapRollbackPreservesSession(t *testing.T) {
	accept := mustPolicy(t, nil, filterapi.FinalAccept)
	reject := mustPolicy(t, nil, filterapi.FinalReject)
	for _, tc := range []struct {
		name string
		from filterapi.Policy
		to   filterapi.Policy
	}{
		{name: "accept to reject", from: accept, to: reject},
		{name: "reject to accept", from: reject, to: accept},
		{name: "remove reject", from: reject},
		{name: "remove accept", from: accept},
		{name: "configure reject", to: reject},
		{name: "configure accept", to: accept},
	} {
		t.Run(tc.name, func(t *testing.T) {
			group := dynamicReloadGroup(t)
			group.Settings.ImportPolicy = tc.from
			group.Settings.ExportPolicy = tc.from
			r, peer := newDynamicReloadReactor(t, group, netip.MustParseAddr("185.1.69.42"), nil)
			establishDynamicPeer(t, r, peer)
			session := peer.currentSession()
			peer.refreshForwardFacts()
			before := peer.forwardFacts()
			require.NotNil(t, before)
			generation := r.peerGeneration.Load()

			next := dynamicReloadGroup(t)
			next.Settings.ImportPolicy = tc.to
			next.Settings.ExportPolicy = tc.to
			j := reloadDynamicGroups(t, r, next)
			require.Equal(t, tc.to, peer.ImportPolicy())
			require.Equal(t, tc.to, peer.ExportPolicy())
			require.NotNil(t, peer.forwardFacts())
			assert.Equal(t, tc.to, peer.forwardFacts().exportPolicy)
			assert.Equal(t, tc.from, before.exportPolicy, "the old snapshot must remain immutable")
			template := peer.readDynamicTemplate()
			assert.Equal(t, tc.to, template.importTemplate)
			assert.Equal(t, tc.to, template.exportTemplate)
			require.Len(t, r.Peers(), 1)
			assert.True(t, r.Peers()[0] == peer, "a final-only edit must keep the same dynamic peer")
			assert.True(t, peer.currentSession() == session)
			assert.Equal(t, PeerStateEstablished, peer.State())
			assert.Equal(t, generation, r.peerGeneration.Load())

			require.Empty(t, j.Rollback())
			assert.Equal(t, tc.from, peer.ImportPolicy())
			assert.Equal(t, tc.from, peer.ExportPolicy())
			require.NotNil(t, peer.forwardFacts())
			assert.Equal(t, tc.from, peer.forwardFacts().exportPolicy)
			template = peer.readDynamicTemplate()
			assert.Equal(t, tc.from, template.importTemplate)
			assert.Equal(t, tc.from, template.exportTemplate)
			require.Len(t, r.Peers(), 1)
			assert.True(t, r.Peers()[0] == peer, "rollback must keep the same dynamic peer")
			assert.True(t, peer.currentSession() == session)
			assert.Equal(t, PeerStateEstablished, peer.State())
			assert.Equal(t, generation, r.peerGeneration.Load())
		})
	}
}

// TestFailingReloadRollsBackDynamicSwap proves AC-20 for the binding and filter
// halves: a reload whose later step fails rolls the journal back, and the
// dynamic peer is the same peer on the same session with its pre-reload
// bindings, filter chain, BFD opt-in and client, template pair and group set.
// The gained process is
// told down for the rolled-back binding. No Cease is sent: the peer stays in
// r.peers throughout.
//
// VALIDATES: the step's undo restores groups, bindings and the template pair.
// PREVENTS: a rolled-back reload leaving a half-applied dynamic swap.
func TestFailingReloadRollsBackDynamicSwap(t *testing.T) {
	addr := netip.MustParseAddr("185.1.69.42")
	before := dynamicReloadGroup(t)
	r, peer := newDynamicReloadReactor(t, before, addr, nil)
	rec := establishDynamicPeer(t, r, peer)
	session := peer.currentSession()

	svc := &swapBFDService{}
	api.SetService(svc)
	defer api.SetService(nil)

	changed := dynamicReloadGroup(t)
	changed.Settings.ProcessBindings = []ProcessBinding{operatorBinding(t, "monitor", "update-received state", "")}
	changed.Settings.ImportPolicy = mustPolicy(t, []filterapi.FilterRef{{Name: "policy:strict-$remote_as"}}, filterapi.FinalAccept)
	changed.Settings.BFD = nonStrictBFD("fast")
	j := reloadDynamicGroups(t, r, changed)
	require.Len(t, peer.settingsSnapshot().ProcessBindings, 1, "the fixture must swap before it rolls back")
	live, _ := svc.state()
	require.Equal(t, 1, live, "the fixture must open the BFD client before it rolls back")

	require.Empty(t, j.Rollback(), "the rollback must not fail")

	r.mu.RLock()
	after := r.peers[netip.AddrPortFrom(addr, DefaultBGPPort)]
	groups := r.dynamicGroups
	r.mu.RUnlock()
	require.True(t, after == peer, "the rollback must keep the same peer")
	assert.True(t, after.currentSession() == session, "the rollback must keep the same session")
	assert.Empty(t, peer.settingsSnapshot().ProcessBindings, "the bindings must be the pre-reload ones")
	assert.Equal(t, []filterapi.FilterRef{{Name: "policy:in-65010"}}, peer.ImportPolicy().Filters(),
		"the running chain must be the pre-reload one, still resolved")
	peer.mu.RLock()
	template := peer.dynImportPolicy.Filters()
	peer.mu.RUnlock()
	assert.Equal(t, []filterapi.FilterRef{{Name: "policy:in-$remote_as"}}, template,
		"the template pair must be the pre-reload one, unresolved")
	require.Len(t, groups, 1)
	assert.True(t, groups[0] == before, "the group set must be the pre-reload one")
	assert.Equal(t, int32(1), before.ActivePeers.Load(), "the restored group must count the peer")
	assert.Nil(t, peer.bfdSettings(), "the BFD opt-in must be the pre-reload one")
	live, _ = svc.state()
	assert.Equal(t, 0, live, "the rollback must release the BFD client the swap opened")

	steps := rec.recorded()
	require.Len(t, steps, 2, "an up for the swap, then a down for its undo")
	assert.Equal(t, "down", steps[1].event)
	assert.Equal(t, []string{"monitor"}, steps[1].processes)
}

// TestDynamicTemplateSwapRacesEstablishment proves the template pair is read
// and written under p.mu while a resolution races a swap. The race detector is
// the verdict, so the test is meant for -race. The closing assertion reads the
// chain the race left, with no resolution after the writers stop: the last
// swap resolved its own template, and every resolution that read an older one
// was refused, so the chain is the last template's. A discriminating proof of
// the refusal, which the race detector cannot see, is
// TestDynamicResolutionRefusedAfterSwap.
//
// VALIDATES: AC-21, no race between the swap and the establishment resolution.
// PREVENTS: a resolution publishing chains from a replaced template.
func TestDynamicTemplateSwapRacesEstablishment(t *testing.T) {
	addr := netip.MustParseAddr("185.1.69.42")
	r, peer := newDynamicReloadReactor(t, dynamicReloadGroup(t), addr, nil)
	establishDynamicPeer(t, r, peer)
	session := peer.currentSession()

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			peer.resolveDynamicPeerSettings(session)
		}
	})

	names := []string{"policy:a-$remote_as", "policy:b-$remote_as"}
	for i := range 200 {
		g := dynamicReloadGroup(t)
		g.Settings.ImportPolicy = mustPolicy(t, []filterapi.FilterRef{{Name: names[i%2]}}, filterapi.FinalAccept)
		reloadDynamicGroups(t, r, g)
	}
	close(stop)
	wg.Wait()

	assert.Equal(t, []filterapi.FilterRef{{Name: "policy:b-65010"}}, peer.ImportPolicy().Filters(),
		"the chain must be resolved from the template the last swap installed")
}

// TestDynamicResolutionRefusedAfterSwap proves the generation check
// deterministically: a resolution reads the template, a swap replaces it, and
// the resolution's publish is refused, so the running chain stays the one the
// swap resolved from the new template.
//
// Method: the two halves of one resolution pass are driven by hand, with the
// swap through the reload's dynamic step between them.
// VALIDATES: AC-21, publishDynamicResolution refuses a replaced template.
// PREVENTS: chains built from the replaced template overwriting the new ones.
func TestDynamicResolutionRefusedAfterSwap(t *testing.T) {
	addr := netip.MustParseAddr("185.1.69.42")
	r, peer := newDynamicReloadReactor(t, dynamicReloadGroup(t), addr, nil)
	establishDynamicPeer(t, r, peer)

	read := peer.readDynamicTemplate()
	stale := resolveFilterVars(read.importTemplate.Filters(), peer.settings.LocalAS, uint32(dynamicPeerRemoteAS), addr.String())

	swapped := dynamicReloadGroup(t)
	swapped.Settings.ImportPolicy = mustPolicy(t, []filterapi.FilterRef{{Name: "policy:b-$remote_as"}}, filterapi.FinalAccept)
	reloadDynamicGroups(t, r, swapped)

	published := peer.publishDynamicResolution(read, uint32(dynamicPeerRemoteAS), read.importTemplate.WithFilters(stale), filterapi.Policy{})
	assert.False(t, published, "a resolution of the replaced template must be refused")
	assert.Equal(t, []filterapi.FilterRef{{Name: "policy:b-65010"}}, peer.ImportPolicy().Filters(),
		"the chain must stay the one resolved from the swapped template")
}

// TestDynamicTemplatesCapturedAtConstruction proves AC-21's capture: the
// template pair is set from the group template when the peer is built, before
// any establishment, so an establishment after a filter swap resolves the
// swapped template and never the first one the peer was built with.
//
// VALIDATES: NewPeer captures dynImportPolicy / dynExportPolicy.
// PREVENTS: a lazy capture on first resolution taking the running chain, or a
// pre-swap template, as the template.
func TestDynamicTemplatesCapturedAtConstruction(t *testing.T) {
	addr := netip.MustParseAddr("185.1.69.44")
	r, peer := newDynamicReloadReactor(t, dynamicReloadGroup(t), addr, nil)

	read := peer.readDynamicTemplate()
	assert.Equal(t, []filterapi.FilterRef{{Name: "policy:in-$remote_as"}}, read.importTemplate.Filters(),
		"the template pair is the group template from construction")

	session := NewSession(peer.settings)
	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()
	swapped := dynamicReloadGroup(t)
	swapped.Settings.ImportPolicy = mustPolicy(t, []filterapi.FilterRef{{Name: "policy:swapped-$remote_as"}}, filterapi.FinalAccept)
	reloadDynamicGroups(t, r, swapped)

	session.mu.Lock()
	session.peerOpen = &message.Open{MyAS: dynamicPeerRemoteAS, BGPIdentifier: 0x0a0b0c0d}
	session.mu.Unlock()
	peer.resolveDynamicPeerSettings(session)
	assert.Equal(t, []filterapi.FilterRef{{Name: "policy:swapped-65010"}}, peer.ImportPolicy().Filters(),
		"establishment must resolve the swapped template")
}

// establishInsideApply returns a copier that runs copier and, on its first
// call, which applyHotSwappableSettings makes in step 1 after reading the
// session's up phase, brings the dynamic peer to Established the way the FSM
// does: the OPEN, the published state, the resolution and the announced up.
func establishInsideApply(peer *Peer, copier settingsCopier) settingsCopier {
	calls := 0
	return func(dst, src *PeerSettings) {
		calls++
		if calls == 1 {
			session := peer.currentSession()
			session.mu.Lock()
			session.peerOpen = &message.Open{MyAS: dynamicPeerRemoteAS, BGPIdentifier: 0x0a0b0c0d}
			session.mu.Unlock()
			peer.state.Store(int32(PeerStateEstablished))
			peer.sendingInitialRoutes.Store(1)
			peer.resolveDynamicPeerSettings(session)
			setUpPhase(peer, sessionUpDelivered)
		}
		copier(dst, src)
	}
}

// TestDynamicFilterSwapRacingEstablishmentKeepsSession proves I8: a dynamic
// peer reaching Established between the swap's plan and its write keeps its
// session for a filter-only swap, with the chain resolved from the swapped
// template, and restarts when the same swap gains a process, because the FSM
// up it raced may have read either graph.
//
// Method: the swap is applied with a copier that establishes the peer inside
// step 1, after the apply judged the session not yet up.
func TestDynamicFilterSwapRacingEstablishmentKeepsSession(t *testing.T) {
	for _, tc := range []struct {
		name    string
		bind    bool
		restart bool
	}{
		{"filter only", false, false},
		{"gained process", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			addr := netip.MustParseAddr("185.1.69.45")
			r, peer := newDynamicReloadReactor(t, dynamicReloadGroup(t), addr, nil)
			rec := establishDynamicPeer(t, r, peer)
			session := peer.currentSession()
			peer.state.Store(int32(PeerStateConnecting))
			session.mu.Lock()
			session.peerOpen = nil
			session.mu.Unlock()
			setUpPhase(peer, sessionUpUnspecified)

			next := peer.settingsSnapshot()
			next.ImportPolicy = mustPolicy(t, []filterapi.FilterRef{{Name: "policy:strict-$remote_as"}}, filterapi.FinalAccept)
			if tc.bind {
				next.ProcessBindings = []ProcessBinding{operatorBinding(t, "monitor", "update-received state", "")}
			}
			copier, reason := peerSettingsSwapPlan(peer.settingsSnapshot(), next, session)
			require.Empty(t, reason, "the swap must be planned in place")
			mustApplyHotSwappableSettings(t, peer, next, establishInsideApply(peer, copier))

			assert.True(t, peer.currentSession() == session, "the session object is kept")
			assert.Equal(t, []filterapi.FilterRef{{Name: "policy:strict-65010"}}, peer.ImportPolicy().Filters(),
				"the chain is resolved from the swapped template")
			assert.Empty(t, rec.recorded(), "no targeted up: the apply judged the session not yet up")
			peer.mu.RLock()
			queued := slices.ContainsFunc(peer.opQueue, func(op peerOp) bool { return op.Type == PeerOpTeardown })
			peer.mu.RUnlock()
			assert.Equal(t, tc.restart, queued, "a restart only when a process was gained")
		})
	}
}

// TestReloadFuncAppliesNoDynamicGroups proves AC-23: a verify, a budget
// estimate and an operation's candidate lookup each run the ReloadFunc over a
// candidate that drops the group, and none of them changes the dynamic
// population or the group set. Only the journaled reconcile applies groups.
//
// VALIDATES: the ReloadFunc returns groups and installs none.
// PREVENTS: a verify tearing dynamic peers down before the commit is decided.
func TestReloadFuncAppliesNoDynamicGroups(t *testing.T) {
	addr := netip.MustParseAddr("185.1.69.42")
	group := dynamicReloadGroup(t)
	r, peer := newDynamicReloadReactor(t, group, addr, nil)
	r.SetReloadFunc(func(string) ([]*PeerSettings, Globals, []*DynamicGroupConfig, error) {
		return nil, Globals{}, nil, nil
	})
	adapter := &reactorAPIAdapter{r: r}

	require.NoError(t, adapter.VerifyConfig(map[string]any{}))
	_, err := adapter.peerDiffCount(map[string]any{})
	require.NoError(t, err)
	_, err = adapter.peerSettingsFromReloadConfig(&rpc.ConfigOperation{Params: rpc.ConfigOperationParams{Peer: "any"}})
	require.Error(t, err, "no peer of that name is in the candidate")

	key := netip.AddrPortFrom(addr, DefaultBGPPort)
	r.mu.RLock()
	after := r.peers[key]
	groups := r.dynamicGroups
	r.mu.RUnlock()
	assert.True(t, after == peer, "a verify, an estimate or a lookup must not remove the dynamic peer")
	require.Len(t, groups, 1)
	assert.True(t, groups[0] == group, "the group set must be the running one")

	// Control: the reconcile does apply the candidate's empty group set.
	require.NoError(t, adapter.Reload())
	r.mu.RLock()
	_, kept := r.peers[key]
	r.mu.RUnlock()
	assert.False(t, kept, "the reload must remove the peer of the dropped group")
}

// TestTreeRouteLeavesDynamicGroupsUnchanged proves the tree route (no
// ReloadFunc) answers "groups not read": ApplyConfigDiff leaves the group set
// and every dynamic peer as they are, rather than reading the absent groups as
// an empty set and tearing every dynamic peer down.
//
// VALIDATES: dynamicGroupsNotRead leaves the population alone.
// PREVENTS: a tree-route apply reading absent groups as none.
func TestTreeRouteLeavesDynamicGroupsUnchanged(t *testing.T) {
	addr := netip.MustParseAddr("185.1.69.42")
	group := dynamicReloadGroup(t)
	r, peer := newDynamicReloadReactor(t, group, addr, nil)
	r.mu.Lock()
	r.reloadFunc = nil
	r.mu.Unlock()

	require.NoError(t, (&reactorAPIAdapter{r: r}).ApplyConfigDiff(map[string]any{"bgp": map[string]any{}}))

	r.mu.RLock()
	after := r.peers[netip.AddrPortFrom(addr, DefaultBGPPort)]
	groups := r.dynamicGroups
	r.mu.RUnlock()
	assert.True(t, after == peer, "the tree route must not remove a dynamic peer")
	require.Len(t, groups, 1)
	assert.True(t, groups[0] == group, "the tree route must not replace the group set")
}

// TestDynamicNonStrictBFDOptInSwaps proves AC-19 at the step: a group template
// that gains a non-strict BFD opt-in keeps the running dynamic peer and opens
// its BFD client, and losing the opt-in releases the client, again on the same
// peer and session.
//
// VALIDATES: the dynamic step delivers a non-strict opt-in through the swap.
// PREVENTS: an IXP operator toggling BFD on a group and resetting every
// dynamic session in it.
// DISCRIMINATION: peer and session identity, and the fake service's live hold
// count; with BFD outside the hot-swap set the step restarts the peer.
func TestDynamicNonStrictBFDOptInSwaps(t *testing.T) {
	svc := &swapBFDService{}
	api.SetService(svc)
	defer api.SetService(nil)
	addr := netip.MustParseAddr("185.1.69.42")
	r, peer := newDynamicReloadReactor(t, dynamicReloadGroup(t), addr, nil)
	establishDynamicPeer(t, r, peer)
	session := peer.currentSession()
	key := netip.AddrPortFrom(addr, DefaultBGPPort)

	gained := dynamicReloadGroup(t)
	gained.Settings.BFD = nonStrictBFD("fast")
	reloadDynamicGroups(t, r, gained)
	r.mu.RLock()
	after := r.peers[key]
	r.mu.RUnlock()
	require.True(t, after == peer, "a non-strict opt-in must not restart the dynamic peer")
	assert.True(t, after.currentSession() == session, "the session must be the same session")
	live, profiles := svc.state()
	require.Equal(t, 1, live, "the swap must open the BFD client")
	assert.Equal(t, []string{"fast"}, profiles)

	reloadDynamicGroups(t, r, dynamicReloadGroup(t))
	r.mu.RLock()
	after = r.peers[key]
	r.mu.RUnlock()
	require.True(t, after == peer, "removing the opt-in must not restart the dynamic peer")
	assert.True(t, after.currentSession() == session, "the session must be the same session")
	live, _ = svc.state()
	assert.Equal(t, 0, live, "removing the opt-in must release the BFD client")
}

// TestDynamicStrictBFDChangeRestarts proves AC-19's strict half at the step: a
// template change with strict mode on either side restarts the dynamic peer,
// because strict mode lives in the OPEN and the OpenConfirm gate.
//
// VALIDATES: the dynamic step refuses to swap a strict-mode change.
// PREVENTS: a running dynamic session keeping an OPEN built for the old mode.
func TestDynamicStrictBFDChangeRestarts(t *testing.T) {
	svc := &swapBFDService{}
	api.SetService(svc)
	defer api.SetService(nil)
	addr := netip.MustParseAddr("185.1.69.42")
	r, peer := newDynamicReloadReactor(t, dynamicReloadGroup(t), addr, nil)
	establishDynamicPeer(t, r, peer)

	strict := dynamicReloadGroup(t)
	strict.Settings.BFD = strictBFD("fast")
	reloadDynamicGroups(t, r, strict)
	r.mu.RLock()
	after := r.peers[netip.AddrPortFrom(addr, DefaultBGPPort)]
	r.mu.RUnlock()
	assert.False(t, after == peer, "a strict-mode opt-in must restart the dynamic peer")
}
