// Design: docs/architecture/api/process-protocol.md -- selected-route final completion.
// Related: dispatch_registry.go -- one registration wires every transport.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func (s *Server) opSelectRoutes(ctx context.Context, proc *process.Process, params json.RawMessage) (any, error) {
	var input rpc.SelectRoutesInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("select-routes: invalid parameters: %w", err)
	}
	return s.selectRoutes(ctx, proc, input)
}

func (s *Server) selectRoutes(ctx context.Context, proc *process.Process, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
	ctx, cancel := rpc.SelectedCallContext(ctx, &input)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if proc == nil {
		return nil, errors.New("select-routes: caller process is unavailable")
	}
	owner, err := proc.SelectedOwner()
	if err != nil {
		return nil, err
	}
	batch, err := plugin.NewSelectedBatch(input)
	if err != nil {
		return nil, err
	}
	reactor, supported := s.reactor.(plugin.ReactorSelectedCoordinator)
	if !supported {
		return nil, errors.New("select-routes: reactor does not support selected output")
	}
	output, err := reactor.SelectRoutes(ctx, batch, owner)
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
