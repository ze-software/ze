// Design: docs/plugin-development/protocol.md -- selected-route final receipts.
// Related: sdk_engine.go -- legacy stored-route relay remains source-owned.
package sdk

import (
	"context"
	"encoding/json"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// SelectRoutes submits one bounded destination batch. The caller MUST keep its
// input unchanged until return. Success means every route has a final writer
// receipt; sent-event delivery is independent and cannot complete this call.
func (p *Plugin) SelectRoutes(ctx context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
	ctx, cancel := rpc.SelectedCallContext(ctx, &input)
	defer cancel()
	if p.engineBridgeActive() {
		if p.bridge.HasSelectRoutes() {
			return p.bridge.SelectRoutes(ctx, input)
		}
	}
	var result json.RawMessage
	var err error
	if p.engineBridgeActive() {
		result, err = p.callEngineWithResult(ctx, rpc.MethodSelectRoutes, &input)
	} else {
		result, err = p.engineMux.CallRPCCancellable(ctx, rpc.MethodSelectRoutes, &input)
	}
	if err != nil {
		return nil, err
	}
	var output *rpc.SelectRoutesOutput
	if err := json.Unmarshal(result, &output); err != nil {
		return nil, err
	}
	if err := output.Validate(input); err != nil {
		return nil, err
	}
	return output, nil
}
