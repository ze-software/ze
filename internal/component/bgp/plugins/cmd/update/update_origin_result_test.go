// Design: docs/architecture/api/update-syntax.md -- accepted origin batch counts.
package update

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/route"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type originResultReactor struct {
	*mockReactorBatch
	announceResult *rpc.UpdateRouteOutput
	withdrawResult *rpc.UpdateRouteOutput
	withdrawCause  error
}

func (r *originResultReactor) AnnounceNLRIBatchResult(context.Context, *selector.Selector, bgptypes.NLRIBatch, plugin.Sender) (*rpc.UpdateRouteOutput, error) {
	return r.announceResult, nil
}
func (r *originResultReactor) WithdrawNLRIBatchResult(context.Context, *selector.Selector, bgptypes.NLRIBatch, plugin.Sender) (*rpc.UpdateRouteOutput, error) {
	return r.withdrawResult, r.withdrawCause
}

// This test compiles against P1 alone: the extra result methods are ordinary Go
// methods on its fake. Old DispatchNLRIGroups calls the inherited error-only
// method and incorrectly reports Announced=1, discarding the actual loser.
func TestUpdateRouteConsumesSuppressedOriginBatchResult(t *testing.T) {
	receipt := rpc.RouteAdmission{Family: "ipv4/unicast", KeyNLRI: []byte{24, 10, 0, 0}, Destination: "192.0.2.7", PeerOwner: 17, Generation: 3,
		Outcome: rpc.RouteAdmissionRetained, Disposition: rpc.RouteAdmissionSuppressedSelection}
	reactor := &originResultReactor{mockReactorBatch: &mockReactorBatch{}, announceResult: &rpc.UpdateRouteOutput{Admissions: []rpc.RouteAdmission{receipt}}}
	ctx := &pluginserver.CommandContext{Server: mustNewServer(&pluginserver.ServerConfig{}, reactor), Peer: "192.0.2.7"}
	response, err := handleUpdateText(ctx, []string{"nhop", "self", "nlri", "add", "10.0.0.0/24"})
	require.NoError(t, err)
	require.NotNil(t, response)
	result, ok := response.Data.(*plugin.RouteResult)
	require.True(t, ok)
	require.Zero(t, result.Announced, "accepted loser must retain the actual zero-output count")
	require.Equal(t, []rpc.RouteAdmission{receipt}, result.Admissions)
}

func TestUpdateRouteOriginResultKeepsWithheldAndPartialFailure(t *testing.T) {
	for _, withheld := range []bool{false, true} {
		t.Run(map[bool]string{false: "true-failure", true: "withheld-completion"}[withheld], func(t *testing.T) {
			cause := errors.New("origin withdrawal transport unavailable")
			receipt := rpc.RouteAdmission{Family: "ipv4/unicast", KeyNLRI: []byte{24, 10, 0, 0}, Destination: "192.0.2.7", PeerOwner: 17, Generation: 3,
				Outcome: rpc.RouteAdmissionRetained, Disposition: rpc.RouteAdmissionSuppressedSelection}
			reactor := &originResultReactor{mockReactorBatch: &mockReactorBatch{}, announceResult: &rpc.UpdateRouteOutput{Admissions: []rpc.RouteAdmission{receipt}}, withdrawCause: cause}
			if withheld {
				reactor.withdrawCause = route.ErrWithdrawWithheld
				reactor.withdrawResult = &rpc.UpdateRouteOutput{Withdrawn: 1}
			}
			ctx := &pluginserver.CommandContext{Server: mustNewServer(&pluginserver.ServerConfig{}, reactor), Peer: "192.0.2.7"}
			response, err := handleUpdateText(ctx, []string{"nhop", "self", "nlri", "add", "10.0.0.0/24", "del", "10.0.1.0/24"})
			require.NotNil(t, response)
			result, ok := response.Data.(*plugin.RouteResult)
			require.True(t, ok)
			require.Zero(t, result.Announced)
			require.Equal(t, []rpc.RouteAdmission{receipt}, result.Admissions)
			if withheld {
				require.NoError(t, err)
				require.Equal(t, uint32(1), result.Withdrawn)
				require.NotEmpty(t, result.Warnings)
			} else {
				require.ErrorIs(t, err, cause)
				require.Zero(t, result.Withdrawn)
				require.Equal(t, cause.Error(), result.Failure)
			}
		})
	}
}
