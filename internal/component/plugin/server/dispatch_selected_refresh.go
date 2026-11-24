// Design: docs/architecture/api/process-protocol.md -- authenticated selected markers.
// Related: dispatch_selected.go -- the same captured selecting process token.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func (s *Server) opSelectRefresh(ctx context.Context, proc *process.Process, params json.RawMessage) (any, error) {
	var input rpc.SelectRefreshInput
	if err := DecodeEngineOperationInput(params, &input, "Selected refresh request"); err != nil {
		return nil, err
	}
	return s.selectRefresh(ctx, proc, input)
}

func (s *Server) selectRefreshNative(ctx context.Context, proc *process.Process, value any) (any, error) {
	input, valid := value.(rpc.SelectRefreshInput)
	if !valid {
		return nil, errors.New("select-refresh input has the wrong type; send its declared request value")
	}
	return s.selectRefresh(ctx, proc, input)
}

func (s *Server) selectRefresh(ctx context.Context, proc *process.Process, input rpc.SelectRefreshInput) (*rpc.SelectRefreshOutput, error) {
	request, err := plugin.NewSelectedRefresh(input)
	if err != nil {
		return nil, err
	}
	ctx, cancel := rpc.SelectRefreshCallContext(ctx, &input)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if proc == nil {
		return nil, errors.New("select-refresh caller is unavailable; reconnect the selecting process")
	}
	owner, err := proc.SelectedOwner()
	if err != nil {
		return nil, err
	}
	reactor, supported := s.reactor.(plugin.ReactorSelectedRefresher)
	if !supported {
		return nil, errors.New("select-refresh reactor is unavailable; start the BGP subsystem before replay")
	}
	output, err := reactor.SelectRefresh(ctx, request, owner)
	if err != nil {
		return nil, err
	}
	if err := owner.Current(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := output.Validate(input); err != nil {
		return nil, err
	}
	return output, nil
}

func selectedRefreshCancellationScope(s *Server, proc *process.Process, params json.RawMessage) (requestScope, error) {
	var input rpc.SelectRefreshInput
	if err := DecodeEngineOperationInput(params, &input, "Selected refresh request"); err != nil {
		return requestScope{}, err
	}
	request, err := plugin.NewSelectedRefresh(input)
	if err != nil {
		return requestScope{}, err
	}
	if proc == nil {
		return requestScope{}, errors.New("select-refresh caller is unavailable; reconnect the selecting process")
	}
	if !s.DeliveryGraph().MaySend(request.Destination().String(), proc.Name(), "update") {
		var problem, fix textbuf.Buffer
		problem.Str("select-refresh: endpoint ").Quoted(proc.Name()).
			Str(" is not permitted to send bgp:update to ").Str(request.Destination().String())
		fix.Str("bus { endpoint ").Str(proc.Name()).Str(" { send { allow [ bgp:update ]; } } }")
		return requestScope{}, opererr.New(
			problem.String(),
			"the compiled bus filter does not allow bgp:update from this endpoint to the requested destination",
			fix.String())
	}
	// Bound admission by the already granted address, as select-routes does.
	// Claimed owner/session values narrow final authority; they MUST NOT mint
	// additional concurrent resources in this connection's live-request table.
	return requestScopeFor(rpc.MethodSelectRefresh, request.Destination(),
		time.UnixMilli(input.DeadlineMS)), nil
}
