// Design: docs/architecture/api/process-protocol.md -- destination-bound existing dispatch.
package sdk

import (
	"context"
	"encoding/json"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"time"
)

// DispatchDestinationCommand waits for the existing command evaluator. Its
// registration MUST opt in; it grants no send or readiness permission.
func (p *Plugin) DispatchDestinationCommand(ctx context.Context, input rpc.DispatchCommandInput) (string, json.RawMessage, error) {
	ctx, cancel := rpc.PeerCommandCallContext(ctx, &input)
	defer cancel()
	if err := input.ValidateDestination(time.Now()); err != nil {
		return "", nil, err
	}
	output, err := cancellableEngineResult[rpc.DispatchCommandOutput](p, ctx, rpc.MethodDispatchCommand, input)
	if err != nil {
		return "", nil, err
	}
	if err := output.ValidateDestinationAnswer(input); err != nil {
		return "", nil, err
	}
	result, err := dispatchDirectCommandResult(output, nil)
	return result.Status, result.Data, err
}
