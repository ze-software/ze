// Design: docs/architecture/api/update-syntax.md -- partial route command results.
package update

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

var errUpdateResultWithdrawal = errors.New("withdraw transport unavailable")

// An accepted announcement remains counted when a later withdrawal fails.
func TestUpdateRouteFailureKeepsEarlierCounts(t *testing.T) {
	reactor := &mockReactorBatch{withdrawError: errUpdateResultWithdrawal}
	ctx := &pluginserver.CommandContext{Server: mustNewServer(&pluginserver.ServerConfig{}, reactor), Peer: "192.0.2.7"}
	resp, err := handleUpdateText(ctx, []string{"nhop", "self", "nlri", "add", "10.0.0.0/24", "del", "10.0.1.0/24"})
	require.ErrorIs(t, err, errUpdateResultWithdrawal)
	require.NotNil(t, resp, "the failing command still returns its answer")
	require.Len(t, reactor.announceCalls, 1, "the earlier parsed announcement was accepted")
	require.Len(t, reactor.withdrawCalls, 1, "the later withdrawal reached its failure")
	result, ok := resp.Data.(*plugin.RouteResult)
	require.True(t, ok, "failure lost the partial route result")
	require.Equal(t, uint32(1), result.Announced)
	require.Zero(t, result.Withdrawn)
	require.Equal(t, errUpdateResultWithdrawal.Error(), resp.Error)
}
