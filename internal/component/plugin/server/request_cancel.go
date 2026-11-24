// Design: docs/architecture/api/process-protocol.md -- connection-scoped request cancellation.
// Related: pkg/plugin/rpc/request_receiver.go -- shared reader enrollment and join.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type requestScope = rpc.RequestScope
type requestIDCollision = rpc.RequestIDCollision

func requestScopeFor[T comparable](method string, identity T, deadline time.Time) requestScope {
	return rpc.NewRequestScope(method, identity, deadline)
}

type runtimeRequestKey struct{}
type runtimeRequestContext struct {
	requests *runtimeRequests
	id       uint64
}
type socketAdmission struct {
	ctx    context.Context
	finish func()
}

type runtimeRequests struct {
	receiver  *rpc.RequestReceiver
	stop      func()
	cancelAll func()
}

func newRuntimeRequests(parent context.Context, scope func(*engineOp, json.RawMessage) (requestScope, error)) *runtimeRequests {
	receiver := rpc.NewRequestReceiver(parent, func(request *rpc.Request) (rpc.RequestScope, error) {
		operation := lookupEngineOp(request.Method)
		if operation == nil {
			return rpc.RequestScope{}, nil
		}
		if operation.cancellationScope == nil {
			return rpc.RequestScope{}, nil
		}
		return scope(operation, request.Params)
	})
	return &runtimeRequests{receiver: receiver, stop: receiver.Stop, cancelAll: receiver.CancelAll}
}

func (requests *runtimeRequests) admit(request *rpc.Request) (*socketAdmission, error) {
	admission, err := requests.receiver.Admit(request)
	if admission == nil {
		return nil, err
	}
	ctx := context.WithValue(admission.Context, runtimeRequestKey{}, runtimeRequestContext{requests, request.ID})
	return &socketAdmission{ctx: ctx, finish: admission.Finish}, err
}

func (requests *runtimeRequests) cancelAndJoin(ctx context.Context, id uint64, method string) (*rpc.CancelRequestOutput, error) {
	return requests.receiver.CancelAndJoin(ctx, id, method)
}

func selectedCancellationScope(s *Server, proc *process.Process, params json.RawMessage) (requestScope, error) {
	destination, deadline, err := selectedCancellationHeader(params)
	if err != nil {
		return requestScope{}, err
	}
	if !s.DeliveryGraph().MaySend(destination.String(), proc.Name(), "update") {
		var problem, fix textbuf.Buffer
		problem.Str("select-routes: endpoint ").Quoted(proc.Name()).Str(" is not permitted to send bgp:update to ").Str(destination.String())
		fix.Str("bus { endpoint ").Str(proc.Name()).Str(" { send { allow [ bgp:update ]; } } }")
		return requestScope{}, opererr.New(
			problem.String(),
			"the compiled bus filter does not allow bgp:update from this endpoint to the requested destination",
			fix.String())
	}
	return requestScopeFor(rpc.MethodSelectRoutes, destination, deadline), nil
}

func selectedCancellationHeader(params json.RawMessage) (netip.Addr, time.Time, error) {
	// Decode the scalar envelope only; route arrays remain raw for the shared
	// selected constructor. The transport already bounds the complete message.
	var envelope struct {
		Destination string `json:"destination"`
		DeadlineMS  int64  `json:"deadline-ms"`
	}
	if err := json.Unmarshal(params, &envelope); err != nil {
		return netip.Addr{}, time.Time{}, err
	}
	destination, err := netip.ParseAddr(envelope.Destination)
	if err != nil {
		return netip.Addr{}, time.Time{}, err
	}
	if envelope.DeadlineMS <= 0 {
		return netip.Addr{}, time.Time{}, errors.New("select-routes: positive deadline-ms is required")
	}
	return destination, time.UnixMilli(envelope.DeadlineMS), nil
}

func (s *Server) opCancelRequest(ctx context.Context, _ *process.Process, params json.RawMessage) (any, error) {
	var input rpc.CancelRequestInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, err
	}
	current, found := ctx.Value(runtimeRequestKey{}).(runtimeRequestContext)
	if !found {
		return nil, errors.New("cancel-request requires its authenticated socket connection")
	}
	if input.RequestID == 0 {
		return nil, errors.New("cancel-request requires a nonzero request-id")
	}
	if input.RequestID >= current.id {
		return nil, errors.New("cancel-request cannot target itself or a future request")
	}
	operation := lookupEngineOp(input.Method)
	if operation == nil {
		return nil, errors.New("cancel-request names an unknown operation")
	}
	if operation.cancellationScope == nil {
		return nil, errors.New("cancel-request operation does not support acknowledged cancellation")
	}
	return current.requests.cancelAndJoin(ctx, input.RequestID, input.Method)
}
