// Design: docs/architecture/api/process-protocol.md -- authoritative routing views.
// Related: ../rpc/route_view.go -- the public value contract.

package sdk

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// RouteViewOpen acquires an authenticated routing-view session.
func (p *Plugin) RouteViewOpen(ctx context.Context, input *rpc.RouteViewOpenInput) (*rpc.RouteViewOpenOutput, error) {
	return routeViewResult[rpc.RouteViewOpenOutput](p, ctx, rpc.MethodRouteViewOpen, input)
}

// RouteViewRead reads one bounded snapshot or delta page.
func (p *Plugin) RouteViewRead(ctx context.Context, input *rpc.RouteViewReadInput) (*rpc.RouteViewReadOutput, error) {
	return routeViewResult[rpc.RouteViewReadOutput](p, ctx, rpc.MethodRouteViewRead, input)
}

// RouteViewCandidates reads the owner's separately pinned candidate snapshot.
func (p *Plugin) RouteViewCandidates(ctx context.Context, input *rpc.RouteViewCandidatesInput) (*rpc.RouteViewCandidatesOutput, error) {
	return routeViewResult[rpc.RouteViewCandidatesOutput](p, ctx, rpc.MethodRouteViewCandidates, input)
}

// RouteViewDistances awaits authoritative application of a distance declaration.
func (p *Plugin) RouteViewDistances(ctx context.Context, input *rpc.RouteViewDistancesInput) (*rpc.RouteViewDistancesOutput, error) {
	return routeViewResult[rpc.RouteViewDistancesOutput](p, ctx, rpc.MethodRouteViewDistances, input)
}

// RouteViewPublish awaits the active session's forwarding acknowledgment.
func (p *Plugin) RouteViewPublish(ctx context.Context, input *rpc.RouteViewPublishInput) (*rpc.RouteViewPublishOutput, error) {
	return routeViewResult[rpc.RouteViewPublishOutput](p, ctx, rpc.MethodRouteViewPublish, input)
}

// RouteViewClose awaits fencing and cleanup of the caller's routing session.
func (p *Plugin) RouteViewClose(ctx context.Context, input *rpc.RouteViewCloseInput) (*rpc.RouteViewCloseOutput, error) {
	return routeViewResult[rpc.RouteViewCloseOutput](p, ctx, rpc.MethodRouteViewClose, input)
}

func routeViewResult[T any](p *Plugin, ctx context.Context, method string, input any) (*T, error) {
	result, err := p.callEngineWithResult(ctx, method, input)
	if err != nil {
		return nil, err
	}
	// DirectBridge has no frame decoder. Keep its reply bound equal to RPC's.
	if len(result) > rpc.RouteViewResultBudget() {
		return nil, fmt.Errorf("%s result exceeds maximum message size", method)
	}
	var out T
	if err := json.Unmarshal(result, &out); err != nil {
		return nil, fmt.Errorf("decode %s result: %w", method, err)
	}
	return &out, nil
}
