// Related: reactor_api.go — reconcilePeersJournaled, the diff under test
// Related: reactor_dynamic.go — SetDynamicGroups, which owns the dynamic population
package reactor

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
)

// newDynamicReloadReactor builds a reactor holding ONE established dynamic peer
// accepted from a group template, plus a ReloadFunc returning newPeers.
//
// No peer goroutine runs: the peer is built through createDynamicPeer with
// r.running false, so its state field is written by the test alone.
func newDynamicReloadReactor(t *testing.T, group *DynamicGroupConfig, addr netip.Addr, newPeers []*PeerSettings) (*Reactor, *Peer) {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "config.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(emptyConfig), 0o600))

	r := New(&Config{ConfigPath: configPath, ListenAddr: "127.0.0.1:0", Standalone: true})
	r.dynamicGroups = []*DynamicGroupConfig{group}
	r.SetReloadFunc(func(string) ([]*PeerSettings, error) {
		return newPeers, nil
	})

	r.mu.Lock()
	peer, err := r.createDynamicPeer(group, addr)
	r.mu.Unlock()
	require.NoError(t, err)
	require.NotNil(t, peer)

	peer.state.Store(int32(PeerStateEstablished))
	return r, peer
}

// dynamicReloadGroup is the group template every test below starts from. It
// carries a filter chain because the chain is what resolveDynamicPeerSettings
// rewrites on the running peer, which is what makes a running dynamic peer
// un-diffable against configuration.
func dynamicReloadGroup() *DynamicGroupConfig {
	dg := newTestDynamicGroup("ix", []string{"185.1.69.0/24"}, 100)
	dg.Settings.Connection = ConnectionPassive
	dg.Settings.ImportFilters = []filterapi.FilterRef{{Name: "policy:in-$remote_as"}}
	return dg
}

// TestReloadKeepsAnEstablishedDynamicPeer is the defect this file exists for.
//
// VALIDATES: a config reload leaves an established dynamic session alone.
// PREVENTS: the teardown of EVERY dynamic session on EVERY reload.
// reconcilePeersJournaled (reactor_api.go) categorized peers by asking whether
// each key of r.peers appears in the config-derived map. A dynamic peer's key is
// the AddrPort it connected FROM and configuration names a template and a prefix
// range instead (createDynamicPeer, reactor_dynamic.go), so every dynamic peer
// read as "no longer configured" and landed in toRemove. An operator running an
// IXP fabric lost every session on a reload that changed nothing about them.
//
// DISCRIMINATION: the assertions are peer OBJECT IDENTITY, session object
// identity and r.peerGeneration, never "a peer exists at this key". The defect
// deletes the map entry, so identity cannot pass against it, and the reconnection
// that follows in production would rebuild both objects.
func TestReloadKeepsAnEstablishedDynamicPeer(t *testing.T) {
	addr := netip.MustParseAddr("185.1.69.42")
	// The reload's config names one unrelated peer, so the dynamic peer's key is
	// absent from newPeerSettings -- exactly the input the defect mis-read.
	other := NewPeerSettings(mustParseAddr("10.0.0.1"), 65001, 65002, 0x01020304)
	other.Connection = ConnectionPassive

	r, peer := newDynamicReloadReactor(t, dynamicReloadGroup(), addr, []*PeerSettings{other})

	// The establishment goroutine has already resolved the template on this peer:
	// PeerAS learned from the OPEN, $remote_as substituted in the chain. This is
	// the state that makes the running peer un-diffable against configuration.
	session := NewSession(peer.settings)
	peer.mu.Lock()
	peer.session = session
	peer.settings.PeerAS = 65010
	peer.settings.ImportFilters = []filterapi.FilterRef{{Name: "policy:in-65010"}}
	peer.mu.Unlock()

	adapter := &reactorAPIAdapter{r: r}
	require.NoError(t, adapter.Reload())

	key := netip.AddrPortFrom(addr, DefaultBGPPort)
	r.mu.RLock()
	after := r.peers[key]
	peerCount := len(r.peers)
	r.mu.RUnlock()

	// Object identity, not r.peerGeneration: that counter is reactor-wide and this
	// reload legitimately adds a peer, so it moves by one whether or not the
	// dynamic peer was torn down. Identity of the peer AND of its session is the
	// assertion the defect cannot pass.
	require.NotNil(t, after, "the dynamic peer must survive a reload that does not name it")
	assert.True(t, after == peer, "a reload must not replace the dynamic peer object")
	assert.True(t, after.currentSession() == session, "the established session must be the same session")
	assert.Equal(t, PeerStateEstablished, after.State(),
		"the session must still be established after the reload")
	assert.Equal(t, 2, peerCount, "the reload must leave the dynamic peer and add the configured one")

	// The resolved chain is still the running one. A swap would have written the
	// template over it, which is the write plan/deferrals/
	// fixit-dynamic-peer-settings-unlocked-read.md races against.
	assert.Equal(t, []filterapi.FilterRef{{Name: "policy:in-65010"}}, after.ImportFilters(),
		"the resolved filter chain must not be overwritten by a reload")
	assert.Equal(t, uint32(65010), after.PeerAS(),
		"the AS learned from the OPEN must not be reset by a reload")

	// Control: the reload still did its own work.
	r.mu.RLock()
	added := r.peers[other.PeerKey()]
	r.mu.RUnlock()
	assert.NotNil(t, added, "the configured peer in the new config must still be added")
}

// TestReloadRemovesAConfiguredPeerTheConfigDropped is the control for the test
// above: the dynamic guard must not turn the reconcile into "never remove".
//
// VALIDATES: a configured peer absent from the new config is still torn down.
// DISCRIMINATION: widening the guard to skip every peer makes this test red.
func TestReloadRemovesAConfiguredPeerTheConfigDropped(t *testing.T) {
	initial := swapTestPeerSettings()
	kept := NewPeerSettings(mustParseAddr("10.0.0.2"), 65001, 65002, 0x01020304)
	kept.Connection = ConnectionPassive

	r, peer := newSwapTestReactor(t, initial, kept)
	peer.state.Store(int32(PeerStateEstablished))

	adapter := &reactorAPIAdapter{r: r}
	require.NoError(t, adapter.Reload())

	r.mu.RLock()
	dropped := r.peers[initial.PeerKey()]
	survivor := r.peers[kept.PeerKey()]
	r.mu.RUnlock()

	assert.Nil(t, dropped, "a configured peer the new config drops must be removed")
	assert.NotNil(t, survivor, "the peer the new config names must be added")
}

// TestReloadReplacesADynamicPeerTheConfigNowNames covers the one overlap: the
// operator adds a peer entry for the exact address a dynamic peer connected from.
//
// VALIDATES: the explicit configuration entry wins, and it wins by RESTART. The
// running settings were resolved at establishment, so they are not the template
// a config entry can be diffed against, and a swap must never run on a dynamic
// peer (plan/deferrals/fixit-dynamic-peer-settings-unlocked-read.md).
// PREVENTS: the guard swallowing the operator's new peer entry -- skipping every
// dynamic peer unconditionally would leave the entry unapplied until the session
// happened to drop.
//
// DISCRIMINATION: object identity again. "A peer exists at this key" passes
// against the swallowed entry, because the dynamic peer is still sitting there.
func TestReloadReplacesADynamicPeerTheConfigNowNames(t *testing.T) {
	addr := netip.MustParseAddr("185.1.69.42")
	configured := NewPeerSettings(addr, 65000, 65010, 0x01020304)
	configured.Connection = ConnectionPassive
	configured.Name = "ix-peer-1"

	r, peer := newDynamicReloadReactor(t, dynamicReloadGroup(), addr, []*PeerSettings{configured})
	generationBefore := r.peerGeneration.Load()

	adapter := &reactorAPIAdapter{r: r}
	require.NoError(t, adapter.Reload())

	r.mu.RLock()
	after := r.peers[configured.PeerKey()]
	r.mu.RUnlock()

	require.NotNil(t, after, "the configured peer must be added at the same key")
	assert.False(t, after == peer, "the template-built peer must be replaced, not kept")
	assert.False(t, after.Settings().IsDynamic, "the surviving peer must be the configured one")
	assert.Equal(t, "ix-peer-1", after.Settings().Name)
	assert.Greater(t, r.peerGeneration.Load(), generationBefore,
		"a remove/re-add advances peerGeneration")
}

// TestReloadNeverSwapsSettingsOntoADynamicPeer drives the same overlap with an
// entry that the swap plan WOULD accept, which is the only input that separates
// the dynamic branch from the ordinary diff.
//
// VALIDATES: applyHotSwappableSettings never runs on a dynamic peer. That is the
// premise plan/deferrals/fixit-dynamic-peer-settings-unlocked-read.md rests on:
// resolveDynamicPeerSettings writes ImportFilters and ExportFilters from the
// ESTABLISHMENT goroutine, and a reload writing them from its own goroutine is
// the race that shard records. The fix for the reload teardown must not arm it.
// PREVENTS: the branch decaying into "the fields happen to differ anyway". Today
// Name and IsDynamic guarantee a restart verdict for every entry the config
// parser can produce (validatePeerName reserves the dyn- prefix,
// bgp/config/resolve.go), so nothing else in the tree holds this invariant.
//
// The entry differs from the running peer in a hot-swappable field ALONE, which
// is what makes peerSettingsSwapPlan return a swap. No config parser emits it;
// the guard is driven from its entry point with the input that isolates it
// (ai/rules/evidence.md).
func TestReloadNeverSwapsSettingsOntoADynamicPeer(t *testing.T) {
	addr := netip.MustParseAddr("185.1.69.42")
	group := dynamicReloadGroup()

	r, peer := newDynamicReloadReactor(t, group, addr, nil)
	swappableOnly := *peer.Settings()
	swappableOnly.ImportFilters = []filterapi.FilterRef{{Name: "policy:in-strict"}}
	require.NotEmpty(t, peer.Settings().ImportFilters, "the fixture needs a chain to change")

	// The plan agrees this entry is swappable: without the dynamic branch the
	// reload would deliver it to the running peer.
	apply, reason := peerSettingsSwapPlan(peer.Settings(), &swappableOnly, peer.currentSession())
	require.Empty(t, reason, "the fixture must be an entry the swap plan accepts")
	require.NotNil(t, apply)

	r.SetReloadFunc(func(string) ([]*PeerSettings, error) {
		return []*PeerSettings{&swappableOnly}, nil
	})
	adapter := &reactorAPIAdapter{r: r}
	require.NoError(t, adapter.Reload())

	r.mu.RLock()
	after := r.peers[netip.AddrPortFrom(addr, DefaultBGPPort)]
	r.mu.RUnlock()

	require.NotNil(t, after)
	assert.False(t, after == peer, "a configured entry must restart the dynamic peer, never swap onto it")
	assert.Equal(t, []filterapi.FilterRef{{Name: "policy:in-$remote_as"}}, peer.ImportFilters(),
		"the outgoing dynamic peer must not have been written to by the reload goroutine")
}

// TestPeerDiffCountIgnoresADynamicPeerTheConfigDoesNotName keeps the budget
// estimate agreeing with the reconcile it estimates.
//
// VALIDATES: peerDiffCount (reactor_api.go) counts the changes reconcile makes.
// PREVENTS: a transaction budget sized for a teardown that no longer happens.
func TestPeerDiffCountIgnoresADynamicPeerTheConfigDoesNotName(t *testing.T) {
	addr := netip.MustParseAddr("185.1.69.42")
	r, _ := newDynamicReloadReactor(t, dynamicReloadGroup(), addr, nil)

	adapter := &reactorAPIAdapter{r: r}
	count, err := adapter.peerDiffCount(map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, 0, count, "a dynamic peer nobody removes is not a change")
}

// TestSetDynamicGroupsKeepsThePeerWhenTheTemplateIsUnchanged proves the reload
// path that OWNS the dynamic population leaves an unchanged one alone.
//
// VALIDATES: reload case 1 at its owning layer. createReloadFunc
// (bgp/config/loader_create.go) calls SetDynamicGroups on every reload with a
// freshly built group, so "unchanged" means structurally equal rather than the
// same pointer. A comparison that answered "changed" for a re-parse of the same
// text would reintroduce the teardown this work removes, one layer down.
//
// DISCRIMINATION: object identity, and the group is rebuilt rather than reused.
func TestSetDynamicGroupsKeepsThePeerWhenTheTemplateIsUnchanged(t *testing.T) {
	addr := netip.MustParseAddr("185.1.69.42")
	r, peer := newDynamicReloadReactor(t, dynamicReloadGroup(), addr, nil)

	r.SetDynamicGroups([]*DynamicGroupConfig{dynamicReloadGroup()})

	r.mu.RLock()
	after := r.peers[netip.AddrPortFrom(addr, DefaultBGPPort)]
	r.mu.RUnlock()

	require.NotNil(t, after, "a re-parse of the same template must not remove the peer")
	assert.True(t, after == peer, "the peer object must survive an unchanged template")
	assert.Equal(t, int32(1), r.dynamicGroups[0].ActivePeers.Load(),
		"the surviving peer must be counted on the new group")
}

// TestSetDynamicGroupsRemovesThePeerWhenTheTemplateChanges covers reload case 2.
//
// VALIDATES: a changed template reaches the peer, by restart. The swap path is
// unreachable here on purpose: a running dynamic peer's settings were rewritten
// at establishment (resolveDynamicPeerSettings), so they are not the template
// the new one can be compared against, and applyHotSwappableSettings must never
// run on a dynamic peer.
// PREVENTS: an operator's edit to a dynamic group being discarded for the life
// of every session accepted under the old template.
func TestSetDynamicGroupsRemovesThePeerWhenTheTemplateChanges(t *testing.T) {
	addr := netip.MustParseAddr("185.1.69.42")
	r, _ := newDynamicReloadReactor(t, dynamicReloadGroup(), addr, nil)

	changed := dynamicReloadGroup()
	changed.Settings.ImportFilters = []filterapi.FilterRef{{Name: "policy:in-strict"}}
	r.SetDynamicGroups([]*DynamicGroupConfig{changed})

	r.mu.RLock()
	after := r.peers[netip.AddrPortFrom(addr, DefaultBGPPort)]
	r.mu.RUnlock()

	assert.Nil(t, after, "a changed group template must restart the sessions built from it")
}

// TestSetDynamicGroupsIgnoresChangesThePeerDoesNotHold keeps the restart narrow.
//
// VALIDATES: a wider range and a raised max-peers change the GROUP and change
// nothing a running peer holds, so neither may bounce a session.
// PREVENTS: the reload bounce coming back through the template comparison --
// accepting more peers is the one edit an IXP makes most often.
func TestSetDynamicGroupsIgnoresChangesThePeerDoesNotHold(t *testing.T) {
	addr := netip.MustParseAddr("185.1.69.42")
	r, peer := newDynamicReloadReactor(t, dynamicReloadGroup(), addr, nil)

	wider := newTestDynamicGroup("ix", []string{"185.1.0.0/16"}, 500)
	wider.Settings.Connection = ConnectionPassive
	wider.Settings.ImportFilters = []filterapi.FilterRef{{Name: "policy:in-$remote_as"}}
	r.SetDynamicGroups([]*DynamicGroupConfig{wider})

	r.mu.RLock()
	after := r.peers[netip.AddrPortFrom(addr, DefaultBGPPort)]
	r.mu.RUnlock()

	require.NotNil(t, after, "a wider range must not remove a peer it still covers")
	assert.True(t, after == peer, "raising max-peers must not restart a session")
}

// TestSetDynamicGroupsRemovesThePeerWhenTheRangeNoLongerCoversIt covers reload
// case 3: narrowing the range past a running peer's address.
//
// VALIDATES: removal is still correct when the configuration stops accepting the
// address. Dynamic peers are not unconditionally immortal.
func TestSetDynamicGroupsRemovesThePeerWhenTheRangeNoLongerCoversIt(t *testing.T) {
	addr := netip.MustParseAddr("185.1.69.42")
	r, _ := newDynamicReloadReactor(t, dynamicReloadGroup(), addr, nil)

	narrowed := newTestDynamicGroup("ix", []string{"185.1.69.0/28"}, 100)
	narrowed.Settings.Connection = ConnectionPassive
	narrowed.Settings.ImportFilters = []filterapi.FilterRef{{Name: "policy:in-$remote_as"}}
	r.SetDynamicGroups([]*DynamicGroupConfig{narrowed})

	r.mu.RLock()
	after := r.peers[netip.AddrPortFrom(addr, DefaultBGPPort)]
	r.mu.RUnlock()

	assert.Nil(t, after, "a peer outside every range of its group must be removed")
}
