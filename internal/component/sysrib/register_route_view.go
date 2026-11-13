// Design: docs/architecture/rib/unified-locrib.md -- authoritative selected views.
// Related: route_view.go -- the shared authenticated session evaluator.

package sysrib

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"

	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// routeViews is the owner's registration-backed daemon service. Declaration
// mode initializes no RIB, connection, timer, subscription or worker.
var routeViews routeViewAuthority

func init() {
	for _, operation := range []pluginserver.EngineOperation{
		{Method: rpc.MethodRouteViewOpen, Handler: routeViewOpenHandler, Startup: pluginserver.EngineOperationConfigureSafe},
		{Method: rpc.MethodRouteViewRead, Handler: routeViewReadHandler, Startup: pluginserver.EngineOperationRuntimeOnly},
		{Method: rpc.MethodRouteViewClose, Handler: routeViewCloseHandler, Startup: pluginserver.EngineOperationRuntimeOnly},
	} {
		if err := pluginserver.RegisterEngineOp(operation); err != nil {
			panic("BUG: selected routing view registration failed: " + err.Error())
		}
	}
	if err := pluginserver.RegisterProcessGenerationCleanup(routeViewCleanup); err != nil {
		panic("BUG: selected routing cleanup registration failed: " + err.Error())
	}
}

func routeViewOpenHandler(_ *pluginserver.Server, ctx context.Context, proc *process.Process, params json.RawMessage) (any, error) {
	var input rpc.RouteViewOpenInput
	if len(params) > 0 {
		if err := routeViewDecode(params, &input); err != nil {
			return nil, err
		}
	}
	return routeViews.open(ctx, proc, locrib.Default(), input)
}

func routeViewReadHandler(_ *pluginserver.Server, ctx context.Context, proc *process.Process, params json.RawMessage) (any, error) {
	var input rpc.RouteViewReadInput
	if err := routeViewDecode(params, &input); err != nil {
		return nil, err
	}
	return routeViews.read(ctx, proc, input)
}

func routeViewCloseHandler(_ *pluginserver.Server, _ context.Context, proc *process.Process, params json.RawMessage) (any, error) {
	var input rpc.RouteViewCloseInput
	if err := routeViewDecode(params, &input); err != nil {
		return nil, err
	}
	return routeViews.close(proc, input.Session)
}

func routeViewDecode(params json.RawMessage, input any) error {
	const fix = "Resend one complete JSON object containing only the declared routing-view input fields"
	if len(params) > rpc.MaxMessageSize {
		return routeViewRefusal("Routing-view request exceeds the message size limit", "Direct and external requests have the same frame bound", fix)
	}
	decoder := json.NewDecoder(bytes.NewReader(params))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(input); err != nil {
		return opererr.Wrap(err, "Routing-view request is malformed", "No part of this request was applied", fix)
	}
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return routeViewRefusal("Routing-view request contains trailing JSON", "Exactly one input object is accepted per request", fix)
}

func routeViewCleanup(proc *process.Process) {
	if err := routeViews.cleanup(proc); err != nil {
		reportRouteViewFailure(proc, err)
	}
}

func reportRouteViewFailure(proc *process.Process, err error) {
	var failure *opererr.Error
	if !errors.As(err, &failure) {
		failure = opererr.Wrap(err, "System routing view failed for plugin "+proc.Name(), "The view cannot authorize new output until resynchronization or cleanup succeeds", "ze request plugin restart "+proc.Name())
	}
	logger().LogAttrs(context.Background(), slog.LevelError, failure.Problem(), failure.LogAttrs(slog.String("plugin", proc.Name()), slog.String("code", routeViewDoctorCode))...)
}
