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

			assert.True(t, peerSettingsRestartRequired(current, next), "the change must force a restart")
			assert.Equal(t, tc.want, peerSettingsRestartReason(current, next))
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
	assert.Empty(t, peerSettingsRestartReason(current, next))
	assert.False(t, peerSettingsRestartRequired(current, next))
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

			assert.True(t, peerSettingsRestartRequired(current, next),
				"an unclassified field must force a restart, never a silent swap")
		})
	}
}

// TestPeerDiffCountCountsSwapAsOneChange verifies the reload budget estimate still
// reports a swap-only edit as a change.
//
// VALIDATES: AC-2 — an import-policy edit must not read as "0 peer changes".
// PREVENTS: the silent-success failure. A swap costs one apply, not the two of a
// remove plus a re-add, so it counts 1 rather than 2.
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
