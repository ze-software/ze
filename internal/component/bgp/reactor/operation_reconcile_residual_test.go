package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/configop"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestReconcilePeersOperationAppliesGlobalsAndDynamicGroups proves the
// reconcile-peers residual (bgpResidualOperation in the BGP plugin) applies
// what no peer operation carries, and rolls it back with its journal.
// Method: a reactor whose reload function answers the candidate (router-id
// 10.0.0.2, dynamic group ix) applies one reconcile-peers operation under an
// operation journal; the journal is then rolled back, as the executor does
// when a later participant fails.
// VALIDATES: the global router-id and the dynamic group set move to the
// candidate on apply, and back to the active config on rollback.
// PREVENTS: a commit that changes `bgp router-id` or a dynamic group beside an
// import-layer change applying only the peer operations.
func TestReconcilePeersOperationAppliesGlobalsAndDynamicGroups(t *testing.T) {
	active := Globals{RouterID: 0x0a000001, LocalAS: 65000}
	candidate := Globals{RouterID: 0x0a000002, LocalAS: 65000}
	group := dynamicReloadGroup(t)
	r := New(&Config{ListenAddr: "127.0.0.1:0", LocalAS: 65000, RouterID: active.RouterID, ConfigPath: "residual.conf"})
	r.SetReloadFunc(func(string) ([]*PeerSettings, Globals, []*DynamicGroupConfig, error) {
		return nil, candidate, []*DynamicGroupConfig{group}, nil
	})
	require.Equal(t, active, r.globals())
	require.Empty(t, r.dynamicGroups)

	op := &rpc.ConfigOperation{ID: "bgp-reconcile-peers", Type: configop.ReconcilePeers,
		Params: rpc.ConfigOperationParams{Config: []byte(`{"router-id":"10.0.0.2"}`)}}
	require.NoError(t, r.VerifyConfigOperation("tx-residual", op))
	j := sdk.NewJournal()
	_, err := r.ApplyConfigOperation(op, j)
	require.NoError(t, err)
	require.Equal(t, candidate, r.globals(), "router-id not applied by the residual")
	r.mu.RLock()
	groups := r.dynamicGroups
	r.mu.RUnlock()
	require.Equal(t, []*DynamicGroupConfig{group}, groups, "dynamic group not applied by the residual")

	require.Empty(t, j.Rollback())
	require.Equal(t, active, r.globals(), "router-id not rolled back")
	r.mu.RLock()
	groups = r.dynamicGroups
	r.mu.RUnlock()
	require.Empty(t, groups, "dynamic group not rolled back")
}
