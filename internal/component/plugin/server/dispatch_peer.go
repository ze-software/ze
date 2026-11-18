// Design: docs/architecture/api/process-protocol.md -- original physical peer command dispatch.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func dispatchPeerCancellationScope(_ *Server, proc *process.Process, raw json.RawMessage) (requestScope, error) {
	var input rpc.DispatchCommandInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return requestScope{}, err
	}
	if input.DestinationBound() {
		if proc == nil {
			return requestScope{}, errors.New("destination command requires its authenticated process")
		}
		if err := input.ValidateDestination(time.Now()); err != nil {
			return requestScope{}, err
		}
		return rpc.NewRequestCancellationScope(rpc.MethodDispatchCommand, time.UnixMilli(input.DeadlineMS)), nil
	}
	if !input.PeerBound() {
		return requestScope{}, nil
	}
	if proc == nil {
		return requestScope{}, errors.New("physical peer command requires its authenticated plugin process")
	}
	if err := input.ValidatePeer(time.Now()); err != nil {
		return requestScope{}, err
	}
	return rpc.NewRequestCancellationScope(rpc.MethodDispatchCommand, time.UnixMilli(input.DeadlineMS)), nil
}

func (s *Server) nativeDispatchPeerCommand(ctx context.Context, proc *process.Process, value any) (any, error) {
	input, ok := value.(rpc.DispatchCommandInput)
	if !ok {
		return nil, errors.New("physical peer command native input has the wrong type; send DispatchCommandInput")
	}
	return s.dispatchPeerCommand(ctx, proc, input)
}

func (s *Server) dispatchPeerCommand(ctx context.Context, proc *process.Process, input rpc.DispatchCommandInput) (*rpc.DispatchCommandOutput, error) {
	if input.DestinationBound() {
		return s.dispatchDestinationCommand(ctx, proc, input)
	}
	if proc == nil {
		return nil, errors.New("physical peer command requires its authenticated plugin process")
	}
	if err := input.ValidatePeer(time.Now()); err != nil {
		return nil, err
	}
	work, cancel := context.WithDeadline(ctx, time.UnixMilli(input.DeadlineMS))
	defer cancel()
	if err := work.Err(); err != nil {
		return nil, err
	}
	response, err := s.dispatchCommandPeerResponse(work, proc, input.Command, input.PeerOwner, input.ConnectionID, 0, 0)
	if err != nil {
		return nil, err
	}
	output := responseToDispatchOutput(response)
	output.PeerOwner = input.PeerOwner
	output.ConnectionID = input.ConnectionID
	output.DeadlineMS = input.DeadlineMS
	return output, nil
}
