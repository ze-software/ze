package plugin

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgpconfig "github.com/ze-software/ze/internal/component/bgp/config"
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	tx "github.com/ze-software/ze/internal/component/config/transaction"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// mixedApplyRoot is a bgp root the reactor loads: peer edge in group ixp,
// whose import layer is importLayer, and peer transit in group transit at
// holdTime seconds. The group order sits beside the list under its reserved
// key, the delivered shape (configorder): a multi-entry list without it has no
// order, and the config tree rebuilt from it walks no group.
func mixedApplyRoot(importLayer, holdTime string) string {
	peer := func(remote, asn string) string {
		return `{"connection":{"remote":{"ip":"` + remote + `"},"local":{"ip":"auto"}},"session":{"asn":{"remote":"` + asn + `"}}}`
	}
	return `{"bgp":{"router-id":"10.0.0.1","session":{"asn":{"local":"65000"}},"group":{` +
		`"ixp":{` + importLayer + `"peer":{"edge":` + peer("203.0.113.1", "65001") + `}},` +
		`"transit":{"timer":{"receive-hold-time":"` + holdTime + `"},"peer":{"transit":` + peer("203.0.113.2", "65002") + `}}},"@group":["ixp","transit"]}}`
}

// mixedPeerState is what the test reads back from the reactor.
type mixedPeerState struct {
	edgeImportConfigured bool
	transitHold          time.Duration
}

func readMixedPeers(t *testing.T, r *reactor.Reactor) mixedPeerState {
	t.Helper()
	var state mixedPeerState
	seen := 0
	var found []string
	for _, peer := range r.Peers() {
		settings := peer.Settings()
		found = append(found, settings.Name+"@"+settings.Address.String())
		switch settings.Address.String() {
		case "203.0.113.1":
			state.edgeImportConfigured = settings.ImportPolicy.AcceptsAtEnd(true)
			seen++
		case "203.0.113.2":
			state.transitHold = settings.ReceiveHoldTime
			seen++
		}
	}
	require.Equal(t, 2, seen, "both peers configured, found %v", found)
	return state
}

// TestBGPOperationMixedDiffAppliesAndRollsBack proves a mixed bgp diff takes
// effect whole on the operation path and rolls back whole. Method: the reactor
// boots from the active root through the section reconcile; the decomposition
// of the commit (a group's import-layer removal plus another group's
// hold-time) is applied through ApplyConfigOperation in graph order, one
// journal per operation, as the executor does; a later failure then rolls the
// journals back in reverse order, as the executor's per-operation rollback does.
// VALIDATES: the import change and the hold-time both apply, and both revert.
// PREVENTS: the hold-time of another group dropped because the diff carried an
// operation for the import change.
func TestBGPOperationMixedDiffAppliesAndRollsBack(t *testing.T) {
	activeData := mixedApplyRoot(`"filter":{"import":{"final":"accept"}},`, "90")
	candidateData := mixedApplyRoot("", "30")
	activeTree, err := parseBGPSection(activeData)
	require.NoError(t, err)
	candidateTree, err := parseBGPSection(candidateData)
	require.NoError(t, err)
	// file is the bgp root the config file holds. The reload function resolves
	// peers from it the way the daemon's does (createReloadFunc, bgp/config):
	// a config tree, ResolveBGPTree for the globals, and PeersFromConfigTree
	// for the peers, so group inheritance and the filter policies are read by
	// the production code rather than by a test copy of it.
	file := activeTree
	r := reactor.New(&reactor.Config{ConfigPath: "mixed.conf"})
	r.SetReloadFunc(func(string) ([]*reactor.PeerSettings, reactor.Globals, []*reactor.DynamicGroupConfig, error) {
		return reloadMixedConfig(file)
	})
	require.NoError(t, r.ReconcilePeersWithJournal(activeTree, sdk.NewJournal()))
	require.Equal(t, mixedPeerState{edgeImportConfigured: true, transitHold: 90 * time.Second}, readMixedPeers(t, r))

	ops := decomposeImportLayer(t, activeData, candidateData,
		`{"bgp/group/ixp/filter/import/final":{"old":"accept","new":""},"bgp/group/transit/timer/receive-hold-time":{"old":"90","new":"30"}}`)
	graph, err := tx.BuildOperationGraph(ops, tx.ConstraintRules())
	require.NoError(t, err)
	sorted, err := tx.TopologicalSort(graph)
	require.NoError(t, err)
	file = candidateTree
	journals := make([]*sdk.Journal, 0, len(sorted))
	for i := range sorted {
		require.NoError(t, r.VerifyConfigOperation("tx-residual", &sorted[i]), "verify %s", sorted[i].ID)
		j := sdk.NewJournal()
		_, err := r.ApplyConfigOperation(&sorted[i], j)
		require.NoError(t, err, "apply %s", sorted[i].ID)
		journals = append(journals, j)
	}
	require.Equal(t, mixedPeerState{edgeImportConfigured: false, transitHold: 30 * time.Second}, readMixedPeers(t, r),
		"the commit did not take effect whole")

	file = activeTree
	for _, j := range slices.Backward(journals) {
		require.Empty(t, j.Rollback())
	}
	require.Equal(t, mixedPeerState{edgeImportConfigured: true, transitHold: 90 * time.Second}, readMixedPeers(t, r),
		"the rollback did not restore the whole active config")
}

// reloadMixedConfig resolves the bgp root through the config pipeline the
// daemon's reload function runs: the delivered map rebuilt as a config tree,
// the resolved tree for the globals, PeersFromConfigTree for the peers.
func reloadMixedConfig(root map[string]any) ([]*reactor.PeerSettings, reactor.Globals, []*reactor.DynamicGroupConfig, error) {
	tree, err := config.TreeFromPluginMap(map[string]any{configRootBGP: root})
	if err != nil {
		return nil, reactor.Globals{}, nil, err
	}
	resolved, err := bgpconfig.ResolveBGPTree(tree)
	if err != nil {
		return nil, reactor.Globals{}, nil, err
	}
	globals, err := reactor.GlobalsFromTree(resolved)
	if err != nil {
		return nil, reactor.Globals{}, nil, err
	}
	peers, err := bgpconfig.PeersFromConfigTree(tree)
	return peers, globals, nil, err
}
