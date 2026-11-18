// Design: docs/architecture/api/process-protocol.md -- final cached-write receipts.
package sdk

import (
	"context"
	"encoding/json"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func (p *Plugin) cachedWriteCall(ctx context.Context, method string, input any, bound bool, deadline int64) error {
	var result json.RawMessage
	var err error
	if !bound {
		_, err = p.callEngineWithResult(ctx, method, input)
		return err
	}
	if p.engineBridgeActive() {
		result, err = p.callEngineWithResult(ctx, method, input)
	} else {
		result, err = p.engineMux.CallRPCCancellable(ctx, method, input)
	}
	if err != nil {
		return err
	}
	var output *rpc.CachedWriteOutput
	if err := json.Unmarshal(result, &output); err != nil {
		return err
	}
	if err := output.Validate(deadline); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return rpc.CompletedCachedWrite(deadline, err).Validate(deadline)
	}
	return nil
}
