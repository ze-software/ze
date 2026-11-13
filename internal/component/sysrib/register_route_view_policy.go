// Design: docs/architecture/rib/unified-locrib.md -- authoritative policy views.
// Related: register_route_view.go -- the owner's shared daemon session service.

package sysrib

import (
	"context"
	"encoding/json"

	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func init() {
	for _, operation := range []pluginserver.EngineOperation{
		{Method: rpc.MethodRouteViewCandidates, Handler: routeViewCandidatesHandler, Startup: pluginserver.EngineOperationRuntimeOnly},
		{Method: rpc.MethodRouteViewDistances, Handler: routeViewDistancesHandler, Startup: pluginserver.EngineOperationConfigureSafe},
	} {
		if err := pluginserver.RegisterEngineOp(operation); err != nil {
			panic("BUG: routing policy view registration failed: " + err.Error())
		}
	}
}

func routeViewCandidatesHandler(_ *pluginserver.Server, ctx context.Context, owner *process.Process, params json.RawMessage) (any, error) {
	var input rpc.RouteViewCandidatesInput
	if err := routeViewDecode(params, &input); err != nil {
		return nil, err
	}
	return routeViews.readCandidates(ctx, owner, input)
}

func routeViewDistancesHandler(_ *pluginserver.Server, ctx context.Context, owner *process.Process, params json.RawMessage) (any, error) {
	var input rpc.RouteViewDistancesInput
	if err := routeViewDecode(params, &input); err != nil {
		return nil, err
	}
	return routeViews.applyDistances(ctx, owner, input)
}
