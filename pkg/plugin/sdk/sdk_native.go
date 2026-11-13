// Design: docs/plugin-development/handlers.md -- native owner engine operations.

package sdk

import "context"

// TryEngineNative passes self-contained values to an owner's native operation.
// Owners supply both native and public RPC adapters to the same evaluator.
// A false handled result with no error means the native form is unavailable;
// the caller chooses its public RPC method and input. Startup and external
// processes take that public path. After bridge activation, shutdown and failure
// return an error and MUST NOT return to the closed startup connection.
// Owners validate input, including nil and typed nil, and return owned values.
func (p *Plugin) TryEngineNative(ctx context.Context, method string, input any) (any, bool, error) {
	if !p.engineBridgeActive() {
		return nil, false, nil
	}
	return p.bridge.DispatchNative(ctx, method, input)
}

// engineBridgeActive keeps bridge selection after activation. Ready is also
// observed because SetReady publishes it before sticky activation.
func (p *Plugin) engineBridgeActive() bool {
	if p.bridge == nil {
		return false
	}
	if p.bridge.Ready() {
		return true
	}
	return p.bridge.Activated()
}
