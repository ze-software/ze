// Design: docs/architecture/api/process-protocol.md -- native owner engine operations.

package rpc

import "context"

// SetDispatchNative installs the generic native owner dispatcher. The engine
// MUST call it before SetReady and MUST NOT replace it after activation.
// The dispatcher returns handled=false only when an operation has no native form.
func (b *DirectBridge) SetDispatchNative(fn func(context.Context, string, any) (any, bool, error)) {
	b.dispatchNative = fn
}

// DispatchNative passes self-contained owner values without JSON conversion.
// Owners validate payload types and values before mutation. Callers retain no
// borrowed engine state in input, and owners return no borrowed state in result.
// A false handled result with no error permits the caller's public RPC path.
// Startup, cancellation, failure and shutdown never report a native miss.
func (b *DirectBridge) DispatchNative(ctx context.Context, method string, input any) (any, bool, error) {
	if err := b.callbackFailure(); err != nil {
		return nil, true, err
	}
	if !b.beginDispatch() {
		return nil, true, ErrBridgeClosed
	}
	defer b.endDispatch()
	if err := ctx.Err(); err != nil {
		return nil, true, err
	}
	if !b.ready.Load() {
		return nil, true, ErrBridgeClosed
	}
	if b.dispatchNative == nil {
		return nil, false, nil
	}
	return b.dispatchNative(ctx, method, input)
}
