// Design: docs/architecture/api/process-protocol.md -- same destination command evaluator.
package server

import (
	"context"
	"errors"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func (s *Server) dispatchDestinationCommand(ctx context.Context, proc *process.Process, input rpc.DispatchCommandInput) (*rpc.DispatchCommandOutput, error) {
	if proc == nil {
		return nil, errors.New("destination command has no authenticated process; use its original connection")
	}
	if err := input.ValidateDestination(time.Now()); err != nil {
		return nil, err
	}
	work, cancel := context.WithDeadline(ctx, time.UnixMilli(input.DeadlineMS))
	defer cancel()
	if err := work.Err(); err != nil {
		return nil, err
	}
	response, err := s.dispatchCommandPeerResponse(work, proc, input.Command, 0, 0, input.DestinationOwner, input.DestinationSession)
	if err != nil {
		return nil, err
	}
	output := responseToDispatchOutput(response)
	output.DestinationOwner = input.DestinationOwner
	output.DestinationSession = input.DestinationSession
	output.DeadlineMS = input.DeadlineMS
	return output, nil
}
