// Design: docs/architecture/api/process-protocol.md -- joined engine requests.
// Related: sdk_native.go -- native availability and sticky bridge activation.
package sdk

import (
	"context"
	"encoding/json"
	"fmt"
)

// cancellableEngineResult shares transport selection and result decoding for
// operations whose public RPC cancellation joins their final work. The caller
// MUST validate its input and bind the deadline before calling this helper.
// The caller MUST validate the returned receipt before reporting success.
func cancellableEngineResult[T any](p *Plugin, ctx context.Context, method string, input any) (*T, error) {
	value, handled, err := p.TryEngineNative(ctx, method, input)
	if err != nil {
		return nil, err
	}
	if handled {
		output, valid := value.(*T)
		if !valid {
			return nil, fmt.Errorf("%s answer has the wrong type; restart the selecting plugin", method)
		}
		return output, nil
	}
	var encoded json.RawMessage
	if p.engineBridgeActive() {
		encoded, err = p.callEngineWithResult(ctx, method, input)
	} else {
		encoded, err = p.engineMux.CallRPCCancellable(ctx, method, input)
	}
	if err != nil {
		return nil, err
	}
	var output *T
	if err := json.Unmarshal(encoded, &output); err != nil {
		return nil, err
	}
	return output, nil
}
