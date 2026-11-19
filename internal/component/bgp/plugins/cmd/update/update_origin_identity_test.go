// Design: docs/architecture/api/update-syntax.md -- original accepted identity at dispatch.
package update

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// Inspect the actual native batch before a reactor could recover a lost field
// from context. Announce is the paired positive control for the withdrawal.
// MUTATION: remove OriginAdmissionID from the withdrawal batch constructor.
func TestUpdateRouteWithdrawalPreservesAcceptedID(t *testing.T) {
	reactor := &mockReactorBatch{}
	ctx := &pluginserver.CommandContext{
		Server: mustNewServer(&pluginserver.ServerConfig{}, reactor),
		Peer:   "*", RequestContext: plugin.WithOriginAdmissionID(t.Context(), 51),
	}
	_, err := handleUpdateText(ctx, []string{"nhop", "self", "nlri", "add", "10.0.0.0/24", "del", "10.0.1.0/24"})
	require.NoError(t, err)
	require.Len(t, reactor.announceCalls, 1)
	require.Len(t, reactor.withdrawCalls, 1)
	require.Equal(t, uint64(51), reactor.announceCalls[0].OriginAdmissionID)
	require.Equal(t, uint64(51), reactor.withdrawCalls[0].OriginAdmissionID)
}
