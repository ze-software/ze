// Design: docs/architecture/api/process-protocol.md -- original physical peer command dispatch.
package sdk

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// DispatchPeerCommand binds the existing command evaluator to one original
// peer and physical connection. The command registration MUST opt in. It waits
// for final handler completion and acknowledged remote cancellation on both
// native and framed transports. It never derives identity from an address.
func (p *Plugin) DispatchPeerCommand(ctx context.Context, input rpc.DispatchCommandInput) (string, json.RawMessage, error) {
	ctx, cancel := rpc.PeerCommandCallContext(ctx, &input)
	defer cancel()
	if err := input.ValidatePeer(time.Now()); err != nil {
		return "", nil, err
	}
	output, err := cancellableEngineResult[rpc.DispatchCommandOutput](p, ctx, rpc.MethodDispatchCommand, input)
	if err != nil {
		return "", nil, err
	}
	if err := output.ValidatePeerAnswer(input); err != nil {
		return "", nil, err
	}
	result, err := dispatchDirectCommandResult(output, nil)
	return result.Status, result.Data, err
}
