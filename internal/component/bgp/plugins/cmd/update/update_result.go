// Design: docs/architecture/api/update-syntax.md -- complete partial command results.
package update

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/component/bgp/route"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A failure cannot erase earlier counts, warnings or admitted loser receipts.
func updateRouteAdmissionFailure(announced, withdrawn int, warnings []string, admissions []rpc.RouteAdmission, cause error) (*plugin.Response, error) {
	return &plugin.Response{Status: plugin.StatusError, Error: cause.Error(), Data: &plugin.RouteResult{
		Announced: uint32(announced), Withdrawn: uint32(withdrawn), //nolint:gosec // bounded parsed native count
		Warnings: warnings, Admissions: admissions, Failure: cause.Error(),
	}}, cause
}

func announceBatchResult(ctx context.Context, reactor bgptypes.BGPReactor, sel *selector.Selector, batch bgptypes.NLRIBatch, sender plugin.Sender) (*rpc.UpdateRouteOutput, error) {
	if complete, ok := reactor.(bgptypes.BatchResultReactor); ok {
		output, err := complete.AnnounceNLRIBatchResult(ctx, sel, batch, sender)
		if output == nil && err == nil {
			return nil, errors.New("announce batch returned no result")
		}
		return output, err
	}
	err := reactor.AnnounceNLRIBatch(ctx, sel, batch, sender)
	if err != nil {
		return nil, err
	}
	return &rpc.UpdateRouteOutput{Announced: uint32(len(batch.NLRIs))}, nil //nolint:gosec // bounded parsed native count
}

func withdrawBatchResult(ctx context.Context, reactor bgptypes.BGPReactor, sel *selector.Selector, batch bgptypes.NLRIBatch, sender plugin.Sender) (*rpc.UpdateRouteOutput, error) {
	if complete, ok := reactor.(bgptypes.BatchResultReactor); ok {
		output, err := complete.WithdrawNLRIBatchResult(ctx, sel, batch, sender)
		if output == nil && err == nil {
			return nil, errors.New("withdraw batch returned no result")
		}
		return output, err
	}
	err := reactor.WithdrawNLRIBatch(ctx, sel, batch, sender)
	if err != nil && !errors.Is(err, route.ErrWithdrawWithheld) {
		return nil, err
	}
	return &rpc.UpdateRouteOutput{Withdrawn: uint32(len(batch.NLRIs))}, err //nolint:gosec // completed withheld command count
}
