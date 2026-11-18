// Design: docs/architecture/web-interface.md -- closed private artifact command inputs.
package artifact_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestArtifactRPCRefusesUnclaimedTokensBeforeAdmission drives the actual
// dispatcher with undeclared tails and observes the sole owner's files and
// transfers. An ordinary begin/abort remains the accepted-input control.
func TestArtifactRPCRefusesUnclaimedTokensBeforeAdmission(t *testing.T) {
	fixture := newPolicyHTTP(t, nil, false)
	caller := plugin.CallerIdentity{Username: "bob"}
	const begin = "request config artifact upload begin name private.zefs total 1"
	for _, tail := range []string{"account alice", "unclaimed value", "unused"} {
		response, err := fixture.dispatch(t.Context(), caller, begin+" "+tail)
		require.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, plugin.StatusError, response.Status)
		state := fixture.owner.FixtureState()
		require.Empty(t, state.Workspaces)
		require.Zero(t, state.Bytes)
		require.Zero(t, state.Uploads)
		require.Zero(t, state.Sources)
		require.Zero(t, state.Views)
		require.Empty(t, state.Root, "refused arguments must not allocate private files")
	}
	response, err := fixture.dispatch(t.Context(), caller, begin)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status, response.Error)
	data, ok := response.Data.(plugin.Map)
	require.True(t, ok)
	transfer, ok := data["transfer"].(string)
	require.True(t, ok)
	require.Len(t, transfer, 64)
	require.Equal(t, 1, fixture.owner.FixtureState().Uploads)
	response, err = fixture.dispatch(t.Context(), caller, "request config artifact upload abort transfer "+transfer)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status, response.Error)
	require.Zero(t, fixture.owner.FixtureState().Uploads)
}
