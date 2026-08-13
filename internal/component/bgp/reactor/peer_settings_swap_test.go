// Related: peer_settings_apply.go — the swap-or-restart split under test
// Related: peer_settings_reload_test.go — the "did anything change?" guard
package reactor

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/report"
)

// swapTestPeerSettings is the baseline a reload starts from. Every swap test
// changes exactly one field on a second copy and asserts what reload does with it.
func swapTestPeerSettings() *PeerSettings {
	ps := NewPeerSettings(mustParseAddr("10.0.0.1"), 65001, 65002, 0x01020304)
	ps.Connection = ConnectionPassive
	ps.ImportFilters = []filterapi.FilterRef{{Name: "policy:old-import"}}
	ps.ExportFilters = []filterapi.FilterRef{{Name: "policy:old-export"}}
	return ps
}

// newSwapTestReactor builds a reactor that can Reload() without starting any peer
// goroutines. The peer is added but never started, so its state field is written
// by the test alone and no background goroutine can overwrite it.
func newSwapTestReactor(t *testing.T, initial, next *PeerSettings) (*Reactor, *Peer) {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "config.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(emptyConfig), 0o600))

	r := New(&Config{ConfigPath: configPath, ListenAddr: "127.0.0.1:0", Standalone: true})
	require.NoError(t, r.AddPeer(initial))
	r.SetReloadFunc(func(string) ([]*PeerSettings, error) {
		return []*PeerSettings{next}, nil
	})

	r.mu.RLock()
	peer := r.peers[initial.PeerKey()]
	r.mu.RUnlock()
	require.NotNil(t, peer, "peer must exist before reload")

	return r, peer
}

// TestReloadImportPolicyKeepsTheSamePeer verifies that a reload whose ONLY change
// is the import filter chain applies the new chain WITHOUT restarting the session.
//
// VALIDATES: the D-1b swap-or-restart split — "no restart unless unavoidable".
// PREVENTS: the bounce that A2 shipped. peerSettingsEqual (reactor_api.go) now
// reports every changed field, and reconcilePeersJournaled applied ANY change by
// removing and re-adding the peer, so an import-policy edit flapped an established
// session.
//
// DISCRIMINATION: the assertions are peer OBJECT IDENTITY and a counter, not the
// absence of something. A restart calls peer.Stop(), deletes the map entry and
// builds a fresh *Peer through Reactor.AddPeer (reactor_peers.go), which bumps
// r.peerGeneration. "A peer exists at this key afterwards" would pass against the
// bouncing code; "it is the same object and the generation did not move" cannot.
func TestReloadImportPolicyKeepsTheSamePeer(t *testing.T) {
	initial := swapTestPeerSettings()
	next := swapTestPeerSettings()
	next.ImportFilters = []filterapi.FilterRef{{Name: "policy:new-import"}}

	r, peer := newSwapTestReactor(t, initial, next)
	peer.state.Store(int32(PeerStateEstablished))
	generationBefore := r.peerGeneration.Load()

	adapter := &reactorAPIAdapter{r: r}
	require.NoError(t, adapter.Reload())

	r.mu.RLock()
	after := r.peers[initial.PeerKey()]
	r.mu.RUnlock()

	require.NotNil(t, after, "the peer must still be configured after reload")
	assert.True(t, after == peer, "an import-policy edit must not replace the peer object")
	assert.Equal(t, generationBefore, r.peerGeneration.Load(),
		"peerGeneration must not advance: a restart re-adds the peer and bumps it")
	assert.Equal(t, PeerStateEstablished, after.State(),
		"the established session must survive an import-policy edit")
	assert.Equal(t, next.ImportFilters, after.ImportFilters(),
		"the new import chain must be published to the running peer")
}

// TestReloadExportPolicyRefreshesForwardFacts verifies that an export-policy edit
// reaches the EGRESS datapath, which reads its filter chain from the
// peerForwardFacts snapshot rather than from PeerSettings.
//
// VALIDATES: the swap actually applies, not merely that it skipped the restart.
// PREVENTS: the original defect in a new costume. peerForwardFacts is built once
// per session by refreshForwardFacts (peer_forward_facts.go) and every egress
// consumer reads facts.exportFilters (egress_inject_filter.go, forward_rs.go,
// reactor_api_forward.go). A swap that updated PeerSettings and left the snapshot
// alone would stop the bounce AND silently keep enforcing the old export chain.
func TestReloadExportPolicyRefreshesForwardFacts(t *testing.T) {
	initial := swapTestPeerSettings()
	next := swapTestPeerSettings()
	next.ExportFilters = []filterapi.FilterRef{{Name: "policy:new-export"}}

	r, peer := newSwapTestReactor(t, initial, next)
	peer.state.Store(int32(PeerStateEstablished))
	// Stand in for the establishment-time build (peer.setEncodingContexts).
	peer.refreshForwardFacts()
	require.NotNil(t, peer.forwardFacts(), "the fixture must start with a facts snapshot")

	adapter := &reactorAPIAdapter{r: r}
	require.NoError(t, adapter.Reload())

	facts := peer.forwardFacts()
	require.NotNil(t, facts, "the swap must not drop the forwarding facts snapshot")
	assert.Equal(t, next.ExportFilters, facts.exportFilters,
		"the egress datapath must see the new export chain after the swap")
}

// TestReloadSwapDoesNotResurrectForwardFacts verifies that swapping settings on a
// peer with no session does not publish a forwarding facts snapshot.
//
// VALIDATES: the swap is inert on a down peer.
// PREVENTS: a nil-facts pointer being used as the "this peer is established" gate
// (forward_rs.go skips a peer whose forwardFacts() is nil). Storing facts from the
// reload goroutine would make a down peer look established to the RS fast path.
func TestReloadSwapDoesNotResurrectForwardFacts(t *testing.T) {
	initial := swapTestPeerSettings()
	next := swapTestPeerSettings()
	next.ExportFilters = []filterapi.FilterRef{{Name: "policy:new-export"}}

	r, peer := newSwapTestReactor(t, initial, next)
	require.Nil(t, peer.forwardFacts(), "a peer that never established has no facts")

	adapter := &reactorAPIAdapter{r: r}
	require.NoError(t, adapter.Reload())

	assert.Nil(t, peer.forwardFacts(),
		"a settings swap must not publish facts for a peer with no session")
	assert.Equal(t, next.ExportFilters, peer.ExportFilters(),
		"the new export chain is still published to the peer settings")
}

// TestReloadSessionFieldRestartsPeer verifies that a change to a field the running
// session cannot take a new value for still restarts the peer.
//
// VALIDATES: the swap path narrowed the restart, it did not remove it.
// PREVENTS: a hot swap that leaves a session running on stale session-scoped
// settings — the failure this whole spec exists to fix, in the opposite direction.
func TestReloadSessionFieldRestartsPeer(t *testing.T) {
	initial := swapTestPeerSettings()
	next := swapTestPeerSettings()
	next.ReceiveHoldTime = 30 * time.Second

	r, peer := newSwapTestReactor(t, initial, next)
	generationBefore := r.peerGeneration.Load()

	adapter := &reactorAPIAdapter{r: r}
	require.NoError(t, adapter.Reload())

	r.mu.RLock()
	after := r.peers[initial.PeerKey()]
	r.mu.RUnlock()

	require.NotNil(t, after, "the peer must be re-added after the restart")
	assert.False(t, after == peer, "a hold-time change must rebuild the peer")
	assert.Greater(t, r.peerGeneration.Load(), generationBefore,
		"a restart re-adds the peer, which advances peerGeneration")
	assert.Equal(t, 30*time.Second, after.Settings().ReceiveHoldTime,
		"the restarted peer must carry the new hold time")
}

// TestPeerSettingsRestartReasonNamesTheChangedFields verifies that the restart
// decision reports WHICH fields forced it.
//
// VALIDATES: B3 — "log which category of change caused a restart when one happens"
// (docs/research/bird-bgp-reference.md). Silent on a hot swap, one named line on a
// restart.
// PREVENTS: an operator seeing a session flap on reload with nothing saying why.
func TestPeerSettingsRestartReasonNamesTheChangedFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*PeerSettings)
		want   string
	}{
		{"hold time", func(p *PeerSettings) { p.ReceiveHoldTime = 30 * time.Second }, "ReceiveHoldTime"},
		{"md5 key", func(p *PeerSettings) { p.MD5Key = "s3cret" }, "MD5Key"},
		{"peer as", func(p *PeerSettings) { p.PeerAS = 65099 }, "PeerAS"},
		{"static routes", func(p *PeerSettings) { p.StaticRoutes = []StaticRoute{{Origin: 1}} }, "StaticRoutes"},
		{"prefix maximum", func(p *PeerSettings) { p.PrefixMaximum = map[string]uint32{"ipv4/unicast": 10} }, "PrefixMaximum"},
		{"route reflector client", func(p *PeerSettings) { p.RouteReflectorClient = true }, "RouteReflectorClient"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			current := swapTestPeerSettings()
			next := swapTestPeerSettings()
			tc.mutate(next)

			// nil session: these fields are not capability fields, so the
			// negotiation probe has nothing to say about them either way.
			assert.True(t, peerSettingsRestartRequired(current, next, nil), "the change must force a restart")
			assert.Equal(t, tc.want, peerSettingsRestartReason(current, next, nil))
		})
	}
}

// TestPeerSettingsRestartReasonEmptyForHotSwappableFields verifies that the two
// declared hot-swappable fields, alone or together, need no restart.
//
// VALIDATES: the swap category is exactly what hotSwappableSettings copies.
// PREVENTS: the two lists drifting apart. peerSettingsRestartReason neutralizes a
// field by calling hotSwappableSettings, the same function the apply path uses, so
// a field can never be classified swappable without also being applied.
func TestPeerSettingsRestartReasonEmptyForHotSwappableFields(t *testing.T) {
	current := swapTestPeerSettings()
	next := swapTestPeerSettings()
	next.ImportFilters = []filterapi.FilterRef{{Name: "policy:new-import"}}
	next.ExportFilters = []filterapi.FilterRef{{Name: "policy:new-export"}}

	require.False(t, peerSettingsEqual(current, next), "the fixture must be a real change")
	assert.Empty(t, peerSettingsRestartReason(current, next, nil))
	assert.False(t, peerSettingsRestartRequired(current, next, nil))
}

// TestPeerSettingsRestartRequiredIsFailClosed verifies that the restart decision
// derives from the whole struct, so a field nobody classified forces a restart.
//
// VALIDATES: AC-6 / A-2 — the guard fails CLOSED for fields it does not know.
// PREVENTS: the hand-maintained-list rot this spec exists to fix. The decision is
// !peerSettingsEqual(hotCopy, next), and peerSettingsEqual is reflect.DeepEqual
// over every field, so a field added to PeerSettings tomorrow is restart-scoped
// until somebody adds it to hotSwappableSettings on purpose.
func TestPeerSettingsRestartRequiredIsFailClosed(t *testing.T) {
	// Fields never named by the old hand-maintained predicate, spread across the
	// scalar / map / slice / pointer shapes the struct holds.
	tests := map[string]func(*PeerSettings){
		"BFD":            func(p *PeerSettings) { p.BFD = &BFDSettings{Enabled: true} },
		"MinTTL":         func(p *PeerSettings) { p.MinTTL = 254 },
		"LinkLocal":      func(p *PeerSettings) { p.LinkLocal = mustParseAddr("fe80::1") },
		"SendCommunity":  func(p *PeerSettings) { p.SendCommunity = []string{"none"} },
		"NextHopMode":    func(p *PeerSettings) { p.NextHopMode = NextHopSelf },
		"PrefixTeardown": func(p *PeerSettings) { p.PrefixTeardown = map[string]bool{"ipv4/unicast": false} },
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			current := swapTestPeerSettings()
			next := swapTestPeerSettings()
			mutate(next)

			assert.True(t, peerSettingsRestartRequired(current, next, nil),
				"an unclassified field must force a restart, never a silent swap")
		})
	}
}

// prefixStaleTestPeer is the address the prefix-date tests below configure. It is
// not the address swapTestPeerSettings uses, so a warning one of these tests leaves
// on the process-wide report bus cannot be mistaken for another test's.
const prefixStaleTestPeer = "10.0.0.9"

// prefixDatedSettings is a peer whose one family carries the given PeeringDB
// refresh date. Every other field is identical across two calls, so a pair of them
// differs in PrefixUpdated and in nothing else.
func prefixDatedSettings(updated string) *PeerSettings {
	ps := NewPeerSettings(mustParseAddr(prefixStaleTestPeer), 65001, 65002, 0x01020304)
	ps.Connection = ConnectionPassive
	ps.PrefixMaximum = map[string]uint32{"ipv4/unicast": 10000}
	ps.PrefixUpdated = map[string]string{"ipv4/unicast": updated}
	return ps
}

// newPrefixStaleTestReactor is newSwapTestReactor with the metrics registry wired
// BEFORE the peer is added, which is the production order: Start registers the
// reactor metrics, and every AddPeer after that publishes ze_bgp_prefix_stale
// through them (reactor.go, reactor_peers.go). The returned gauge vec is that
// metric, so a test can read the value the reload had to correct.
func newPrefixStaleTestReactor(t *testing.T, initial, next *PeerSettings) (*Reactor, *Peer, *spyGaugeVec) {
	t.Helper()

	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	configPath := filepath.Join(t.TempDir(), "config.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(emptyConfig), 0o600))

	reg := newSpyRegistry()
	r := New(&Config{ConfigPath: configPath, ListenAddr: "127.0.0.1:0", Standalone: true})
	r.rmetrics = initReactorMetrics(reg, "1.0.0", "1.2.3.4", "65000")

	require.NoError(t, r.AddPeer(initial))
	r.SetReloadFunc(func(string) ([]*PeerSettings, error) {
		return []*PeerSettings{next}, nil
	})

	r.mu.RLock()
	peer := r.peers[initial.PeerKey()]
	r.mu.RUnlock()
	require.NotNil(t, peer, "peer must exist before reload")

	return r, peer, reg.gaugeVec("ze_bgp_prefix_stale")
}

// prefixStaleRaised reports whether the report bus currently carries a
// bgp/prefix-stale warning for the test peer, which is what `ze show warnings` and
// the login banner read (test/plugin/show-warnings.ci).
func prefixStaleRaised() bool {
	for _, w := range report.Warnings() {
		if w.Source == reportSourceBGP && w.Code == reportCodePrefixStale && w.Subject == prefixStaleTestPeer {
			return true
		}
	}
	return false
}

// freshPrefixDate is today, which IsPrefixDataStale (session_prefix.go) measures
// against a 180-day threshold. The reactor under test runs on clock.RealClock, so
// deriving the date from the same wall clock keeps the fixture valid on any day.
func freshPrefixDate() string {
	return time.Now().UTC().Format(time.DateOnly)
}

// TestReloadPrefixDatesSwapWithoutRestart verifies that a config change touching
// ONLY the per-family prefix `updated` dates reaches the running peer, and reaches
// it without tearing the session down.
//
// VALIDATES: a PeeringDB refresh that bumps only the dates is delivered. Before
// this, peerSettingsEqual (reactor_api.go) neutralized PrefixUpdated on both copies
// before comparing, so reconcilePeersJournaled read the peer as unchanged, neither
// swapped nor restarted it, and the new dates were discarded
// (plan/deferrals/fixit-bgp-per-family-prefix-enforcement.md).
// PREVENTS: both halves of that trade going wrong. Comparing the field without
// classifying it hot-swappable would deliver the dates by BOUNCING every peer on
// every PeeringDB refresh, which is a worse regression than the stale alarm.
//
// DISCRIMINATION: the assertions are peer OBJECT IDENTITY, the generation counter,
// and the date read back off the running peer. "A peer exists at this key" passes
// against both failures; identity plus the delivered date passes against neither.
func TestReloadPrefixDatesSwapWithoutRestart(t *testing.T) {
	fresh := freshPrefixDate()
	initial := prefixDatedSettings("2020-01-01")
	next := prefixDatedSettings(fresh)

	r, peer, _ := newPrefixStaleTestReactor(t, initial, next)
	peer.state.Store(int32(PeerStateEstablished))
	generationBefore := r.peerGeneration.Load()

	adapter := &reactorAPIAdapter{r: r}
	require.NoError(t, adapter.Reload())

	r.mu.RLock()
	after := r.peers[initial.PeerKey()]
	r.mu.RUnlock()

	require.NotNil(t, after, "the peer must still be configured after reload")
	assert.True(t, after == peer, "a prefix-date refresh must not replace the peer object")
	assert.Equal(t, generationBefore, r.peerGeneration.Load(),
		"peerGeneration must not advance: a restart re-adds the peer and bumps it")
	assert.Equal(t, PeerStateEstablished, after.State(),
		"the established session must survive a prefix-date refresh")
	assert.Equal(t, fresh, after.OldestPrefixUpdated(),
		"the new dates must reach the running peer, not be discarded as 'no change'")
}

// TestReloadFreshPrefixDatesClearStaleAlarm verifies that the swap republishes the
// staleness verdict, so a PeeringDB refresh actually clears the alarm it exists to
// clear.
//
// VALIDATES: the operator-visible half of the fix. Both surfaces are asserted
// because both are published from the same dates and each is read by different
// tooling: the report bus warning by `ze show warnings` and the login banner, the
// ze_bgp_prefix_stale gauge by Prometheus.
// PREVENTS: the silent-discard failure moving one layer out. A swap that delivers
// the dates and republishes nothing leaves both alarms answering from the values
// read at AddPeer, so they could only clear on a daemon restart.
//
// DISCRIMINATION: the fixture REQUIREs the alarm raised and the gauge at 1 before
// the reload. Without that precondition, "no warning afterwards" would also pass on
// a run where the alarm was never raised at all.
func TestReloadFreshPrefixDatesClearStaleAlarm(t *testing.T) {
	initial := prefixDatedSettings("2020-01-01")
	next := prefixDatedSettings(freshPrefixDate())

	r, peer, staleGauge := newPrefixStaleTestReactor(t, initial, next)
	peer.state.Store(int32(PeerStateEstablished))

	require.True(t, prefixStaleRaised(), "the fixture must start with the prefix-stale warning raised")
	require.NotNil(t, staleGauge, "ze_bgp_prefix_stale must be registered")
	require.Equal(t, float64(1), staleGauge.get(prefixStaleTestPeer).Value(),
		"the fixture must start with ze_bgp_prefix_stale at 1")

	adapter := &reactorAPIAdapter{r: r}
	require.NoError(t, adapter.Reload())

	assert.False(t, prefixStaleRaised(),
		"refreshed prefix dates must clear the prefix-stale warning without a daemon restart")
	assert.Equal(t, float64(0), staleGauge.get(prefixStaleTestPeer).Value(),
		"refreshed prefix dates must drive ze_bgp_prefix_stale back to 0")
}

// TestReloadStalePrefixDatesRaiseStaleAlarm verifies the other direction: a reload
// whose dates cross the 180-day threshold RAISES the alarm on the running peer.
//
// VALIDATES: the swap republishes the verdict rather than clearing unconditionally.
// PREVENTS: a fix that passes the clearing test by always clearing, which would
// silence a peer whose prefix data has genuinely gone stale.
func TestReloadStalePrefixDatesRaiseStaleAlarm(t *testing.T) {
	initial := prefixDatedSettings(freshPrefixDate())
	next := prefixDatedSettings("2020-01-01")

	r, peer, staleGauge := newPrefixStaleTestReactor(t, initial, next)
	peer.state.Store(int32(PeerStateEstablished))

	require.False(t, prefixStaleRaised(), "the fixture must start with no prefix-stale warning")
	require.Equal(t, float64(0), staleGauge.get(prefixStaleTestPeer).Value(),
		"the fixture must start with ze_bgp_prefix_stale at 0")

	adapter := &reactorAPIAdapter{r: r}
	require.NoError(t, adapter.Reload())

	assert.True(t, prefixStaleRaised(),
		"dates that cross the staleness threshold must raise the warning on the running peer")
	assert.Equal(t, float64(1), staleGauge.get(prefixStaleTestPeer).Value(),
		"dates that cross the staleness threshold must drive ze_bgp_prefix_stale to 1")
}

// TestPeerDiffCountCountsSwapAsOneChange verifies the reload budget estimate still
// reports a swap-only edit as a change.
//
// VALIDATES: AC-2 — an import-policy edit must not read as "0 peer changes".
// PREVENTS: the silent-success failure. A swap costs one apply, not the two of a
// remove plus a re-add, so it counts 1 rather than 2.
//
// DISCRIMINATION: the assertion is an exact 1, so it separates both failures. The
// original omission (ImportFilters neutralized inside peerSettingsEqual,
// reactor_api.go) reads 0, and a change delivered by a bounce reads 2.
func TestPeerDiffCountCountsSwapAsOneChange(t *testing.T) {
	initial := swapTestPeerSettings()
	next := swapTestPeerSettings()
	next.ImportFilters = []filterapi.FilterRef{{Name: "policy:new-import"}}

	r, _ := newSwapTestReactor(t, initial, next)
	adapter := &reactorAPIAdapter{r: r}

	count, err := adapter.peerDiffCount(map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, 1, count, "a hot-swappable edit is one change, not zero and not a bounce")
}

// TestPeerDiffCountIsZeroForAnIdenticalReload verifies the other direction of the
// budget estimate: a reload that changes nothing costs nothing.
//
// VALIDATES: AC-3 / R-2. The guard became reflect.DeepEqual over the whole struct,
// which is strictly broader than the predicate it replaced.
// PREVENTS: over-triggering. A whole-struct comparison that reported a difference
// on identical settings would bounce or swap every peer on every reload, which on a
// large fleet is a self-inflicted outage -- the opposite failure to the one this
// spec fixes, and just as operator-visible.
//
// DISCRIMINATION: the fixture is two SEPARATELY built PeerSettings, not one pointer
// used twice, so the maps and slices inside them are distinct allocations. A guard
// that compared identity rather than value would report a change here.
func TestPeerDiffCountIsZeroForAnIdenticalReload(t *testing.T) {
	initial := swapTestPeerSettings()
	next := swapTestPeerSettings()
	require.False(t, initial == next, "the fixture must be two distinct structs")

	r, _ := newSwapTestReactor(t, initial, next)
	adapter := &reactorAPIAdapter{r: r}

	count, err := adapter.peerDiffCount(map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, 0, count, "a reload that changes nothing must report no peer changes")
}

// TestReloadRouteReflectorClientReachesForwarding verifies that an edit to a field
// the running session cannot take reaches the forwarding datapath through the
// restart, rather than being dropped.
//
// VALIDATES: AC-4. route-reflector-client governs whether ze re-advertises an IBGP
// route and what ORIGINATOR_ID/CLUSTER_LIST it carries (RFC 4456), and the egress
// path reads it from the peerForwardFacts snapshot (peer_forward_facts.go).
// PREVENTS: the restart branch inheriting the original defect. The whole point of
// the swap-or-restart split is that the restart branch delivers what the swap
// cannot, so a restart that re-adds the peer from STALE settings would leave the
// operator with a flapped session AND the old behavior.
//
// DISCRIMINATION: the assertion is the value read off the peer that is running
// AFTER the reload, through the same snapshot the egress path reads. Neutralizing
// RouteReflectorClient inside peerSettingsEqual (reactor_api.go) reproduces the
// original hand-maintained-list omission for this field, and the test then reads
// false off a peer that was never reconciled. "The peer restarted" alone would pass
// against that mutation, because the peer does not restart at all.
func TestReloadRouteReflectorClientReachesForwarding(t *testing.T) {
	initial := swapTestPeerSettings()
	require.False(t, initial.RouteReflectorClient, "the fixture must start with the field off")
	next := swapTestPeerSettings()
	next.RouteReflectorClient = true

	r, peer := newSwapTestReactor(t, initial, next)
	peer.state.Store(int32(PeerStateEstablished))
	generationBefore := r.peerGeneration.Load()

	adapter := &reactorAPIAdapter{r: r}
	require.NoError(t, adapter.Reload())

	r.mu.RLock()
	after := r.peers[initial.PeerKey()]
	r.mu.RUnlock()
	require.NotNil(t, after, "the peer must still be configured after reload")

	assert.Greater(t, r.peerGeneration.Load(), generationBefore,
		"a session-scoped field must restart the peer: the running session cannot take it")

	// Stand in for the establishment-time build (peer.setEncodingContexts).
	after.refreshForwardFacts()
	facts := after.forwardFacts()
	require.NotNil(t, facts, "the re-added peer must build its forwarding facts")
	assert.True(t, facts.rrClient,
		"the egress datapath must see the new route-reflector-client value after reload")
}

// TestReloadImportPolicyDisplayMatchesDatapath verifies that after a policy reload
// the operator-facing display and the enforcing datapath report the SAME chain.
//
// VALIDATES: AC-5. The two paths are independently derived: `bgp peer <ip>` renders
// PeerInfo.ImportFilters (plugins/cmd/peer/peer.go) built by Peers()
// (reactor_api.go), while the ingress datapath reads Peer.ImportFilters()
// (runIngressPolicyChain, filter_ordered.go).
// PREVENTS: the facet that made the original defect invisible. The display already
// showed the operator's new policy while the datapath enforced the old one, so the
// reload looked successful from every surface an operator can see.
//
// DISCRIMINATION: both values are asserted equal to the NEW chain, not merely
// equal to each other. Agreement alone passes against the original defect, where
// both sides read the old chain; agreement ON THE NEW CHAIN passes against neither
// the old defect nor a display re-derived from the re-parsed config.
func TestReloadImportPolicyDisplayMatchesDatapath(t *testing.T) {
	initial := swapTestPeerSettings()
	next := swapTestPeerSettings()
	next.ImportFilters = []filterapi.FilterRef{{Name: "policy:new-import"}}

	r, peer := newSwapTestReactor(t, initial, next)
	peer.state.Store(int32(PeerStateEstablished))

	adapter := &reactorAPIAdapter{r: r}
	require.NoError(t, adapter.Reload())

	infos := adapter.Peers()
	require.Len(t, infos, 1, "the reload must leave exactly the one configured peer")

	want := filterapi.FilterRefStrings(next.ImportFilters)
	assert.Equal(t, want, infos[0].ImportFilters,
		"the displayed import-policy must be the chain the operator just configured")
	assert.Equal(t, want, filterapi.FilterRefStrings(peer.ImportFilters()),
		"the enforcing datapath must read that same chain")
}
