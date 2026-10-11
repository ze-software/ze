// Design: docs/architecture/core-design.md — section 18a, a dynamic peer takes the configured peer's swap
// Related: reactor_dynamic.go — reconcileDynamicGroupsJournaled, the step under test
// Related: peer_settings_apply.go — applyHotSwappableSettings, the copier and ordered delivery it shares

package reactor

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// establishDynamicPeer gives peer an Established session whose OPEN carried
// remoteAS, resolves the template against it as the establishment goroutine
// does, and wires a recorder in place of the event dispatcher.
func establishDynamicPeer(t *testing.T, r *Reactor, peer *Peer, remoteAS uint16) *bindRecorder {
	t.Helper()
	srv := &pluginserver.Server{}
	r.api = srv
	rec := &bindRecorder{t: t, srv: srv, conn: &recordingConn{}}
	r.bindEvents = rec

	session := NewSession(peer.settings)
	session.mu.Lock()
	session.peerOpen = &message.Open{MyAS: remoteAS, BGPIdentifier: 0x0a0b0c0d}
	session.mu.Unlock()
	peer.mu.Lock()
	peer.session = session
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
	r, peer := newDynamicReloadReactor(t, dynamicReloadGroup(), addr, nil)
	rec := establishDynamicPeer(t, r, peer, 65010)
	session := peer.currentSession()

	changed := dynamicReloadGroup()
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
	assert.Equal(t, []filterapi.FilterRef{{Name: "policy:in-65010"}}, peer.ImportFilters(),
		"the running chain must stay resolved against the learned AS")

	steps := rec.recorded()
	require.Len(t, steps, 1, "one targeted up for the gained process")
	assert.Equal(t, "up", steps[0].event)
	assert.Equal(t, rpc.ReasonAttached, steps[0].reason)
	assert.Equal(t, []string{"monitor"}, steps[0].processes)

	// The second pass: the same template again, rebuilt.
	again := dynamicReloadGroup()
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
		g := dynamicReloadGroup()
		g.Settings.ImportFilters = []filterapi.FilterRef{{Name: "policy:strict-$remote_as"}}
		return g
	}

	t.Run("learned", func(t *testing.T) {
		addr := netip.MustParseAddr("185.1.69.42")
		r, peer := newDynamicReloadReactor(t, dynamicReloadGroup(), addr, nil)
		establishDynamicPeer(t, r, peer, 65010)

		reloadDynamicGroups(t, r, strict())

		r.mu.RLock()
		after := r.peers[netip.AddrPortFrom(addr, DefaultBGPPort)]
		r.mu.RUnlock()
		require.True(t, after == peer, "a filter template change must swap, not restart")
		assert.Equal(t, []filterapi.FilterRef{{Name: "policy:strict-65010"}}, peer.ImportFilters())
		peer.mu.RLock()
		template := peer.dynImportFilters
		peer.mu.RUnlock()
		assert.Equal(t, []filterapi.FilterRef{{Name: "policy:strict-$remote_as"}}, template,
			"the template pair must hold the new chain unresolved")
	})

	t.Run("not yet learned", func(t *testing.T) {
		addr := netip.MustParseAddr("185.1.69.43")
		r, peer := newDynamicReloadReactor(t, dynamicReloadGroup(), addr, nil)
		session := NewSession(peer.settings)
		peer.mu.Lock()
		peer.session = session
		peer.mu.Unlock()

		reloadDynamicGroups(t, r, strict())
		assert.Equal(t, []filterapi.FilterRef{{Name: "policy:strict-$remote_as"}}, peer.ImportFilters(),
			"a peer with no AS yet keeps the chain unresolved")

		session.mu.Lock()
		session.peerOpen = &message.Open{MyAS: 65020, BGPIdentifier: 0x0a0b0c0d}
		session.mu.Unlock()
		peer.resolveDynamicPeerSettings(session)
		assert.Equal(t, []filterapi.FilterRef{{Name: "policy:strict-65020"}}, peer.ImportFilters(),
			"establishment must resolve the chain from the new template pair")
	})
}

// TestFailingReloadRollsBackDynamicSwap proves AC-20 for the binding and filter
// halves: a reload whose later step fails rolls the journal back, and the
// dynamic peer is the same peer on the same session with its pre-reload
// bindings, filter chain, template pair and group set. The gained process is
// told down for the rolled-back binding. No Cease is sent: the peer stays in
// r.peers throughout.
//
// VALIDATES: the step's undo restores groups, bindings and the template pair.
// PREVENTS: a rolled-back reload leaving a half-applied dynamic swap.
func TestFailingReloadRollsBackDynamicSwap(t *testing.T) {
	addr := netip.MustParseAddr("185.1.69.42")
	before := dynamicReloadGroup()
	r, peer := newDynamicReloadReactor(t, before, addr, nil)
	rec := establishDynamicPeer(t, r, peer, 65010)
	session := peer.currentSession()

	changed := dynamicReloadGroup()
	changed.Settings.ProcessBindings = []ProcessBinding{operatorBinding(t, "monitor", "update-received state", "")}
	changed.Settings.ImportFilters = []filterapi.FilterRef{{Name: "policy:strict-$remote_as"}}
	j := reloadDynamicGroups(t, r, changed)
	require.Len(t, peer.settingsSnapshot().ProcessBindings, 1, "the fixture must swap before it rolls back")

	require.Empty(t, j.Rollback(), "the rollback must not fail")

	r.mu.RLock()
	after := r.peers[netip.AddrPortFrom(addr, DefaultBGPPort)]
	groups := r.dynamicGroups
	r.mu.RUnlock()
	require.True(t, after == peer, "the rollback must keep the same peer")
	assert.True(t, after.currentSession() == session, "the rollback must keep the same session")
	assert.Empty(t, peer.settingsSnapshot().ProcessBindings, "the bindings must be the pre-reload ones")
	assert.Equal(t, []filterapi.FilterRef{{Name: "policy:in-65010"}}, peer.ImportFilters(),
		"the running chain must be the pre-reload one, still resolved")
	peer.mu.RLock()
	template := peer.dynImportFilters
	peer.mu.RUnlock()
	assert.Equal(t, []filterapi.FilterRef{{Name: "policy:in-$remote_as"}}, template,
		"the template pair must be the pre-reload one, unresolved")
	require.Len(t, groups, 1)
	assert.True(t, groups[0] == before, "the group set must be the pre-reload one")
	assert.Equal(t, int32(1), before.ActivePeers.Load(), "the restored group must count the peer")

	steps := rec.recorded()
	require.Len(t, steps, 2, "an up for the swap, then a down for its undo")
	assert.Equal(t, "down", steps[1].event)
	assert.Equal(t, []string{"monitor"}, steps[1].processes)
}

// TestDynamicTemplateSwapRacesEstablishment proves the template pair is read
// and written under p.mu, and that a resolution racing a swap never publishes a
// chain built from the template the swap replaced. The race detector is the
// first verdict, so the test is meant for -race; the closing assertion is the
// second: once both writers stop, a last resolution leaves the chain resolved
// from the template the last swap installed.
//
// VALIDATES: dynGeneration makes a racing resolution start again.
// PREVENTS: a resolution publishing chains from a replaced template.
func TestDynamicTemplateSwapRacesEstablishment(t *testing.T) {
	addr := netip.MustParseAddr("185.1.69.42")
	r, peer := newDynamicReloadReactor(t, dynamicReloadGroup(), addr, nil)
	establishDynamicPeer(t, r, peer, 65010)
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
		g := dynamicReloadGroup()
		g.Settings.ImportFilters = []filterapi.FilterRef{{Name: names[i%2]}}
		reloadDynamicGroups(t, r, g)
	}
	close(stop)
	wg.Wait()

	peer.resolveDynamicPeerSettings(session)
	assert.Equal(t, []filterapi.FilterRef{{Name: "policy:b-65010"}}, peer.ImportFilters(),
		"the chain must be resolved from the template the last swap installed")
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
	group := dynamicReloadGroup()
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
	group := dynamicReloadGroup()
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
