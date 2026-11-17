// Design: docs/architecture/api/process-protocol.md -- outer origin request settlement.
// Related: request_cancel.go -- the same enrolled receiver as selected output.
package server

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Only a new caller's explicit deadline opts in. The authenticated connection
// correlates each request independently, preserving parallel identical commands.
// Full parser, sender grants and concrete peer resolution remain in dispatch.
func updateRouteCancellationScope(_ *Server, proc *process.Process, raw json.RawMessage) (requestScope, error) {
	var input rpc.UpdateRouteInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return requestScope{}, err
	}
	if input.DeadlineMS == 0 {
		return requestScope{}, nil
	}
	if proc == nil {
		return requestScope{}, errors.New("update-route cancellation requires its authenticated process")
	}
	if input.Command == "" {
		return requestScope{}, errors.New("update-route command is missing")
	}
	return rpc.NewRequestCancellationScope(rpc.MethodUpdateRoute, time.UnixMilli(input.DeadlineMS)), nil
}
