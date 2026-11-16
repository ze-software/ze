// Design: docs/architecture/api/process-protocol.md -- selected-route final completion.
// Related: bridge.go -- dispatch admission and process-lifetime draining.
package rpc

import (
	"context"
	"errors"
)

// SelectRoutesHandler MUST return final writer receipts, never queue admission.
// Input is borrowed until return; the handler MUST NOT retain caller-owned bytes.
type SelectRoutesHandler func(context.Context, SelectRoutesInput) (*SelectRoutesOutput, error)

// SetSelectRoutes installs the selected-operation slot before bridge activation.
func (b *DirectBridge) SetSelectRoutes(handler SelectRoutesHandler) {
	b.selectRoutes = handler
	b.hasSelectRoutes.Store(handler != nil)
}

// HasSelectRoutes reports whether an active bridge has the selected operation.
func (b *DirectBridge) HasSelectRoutes() bool {
	return b.ready.Load() && b.hasSelectRoutes.Load()
}

// SelectRoutes keeps dispatch admission until the final receipt returns, so
// process cleanup cannot overtake an admitted selected write.
func (b *DirectBridge) SelectRoutes(ctx context.Context, input SelectRoutesInput) (*SelectRoutesOutput, error) {
	ctx, cancel := SelectedCallContext(ctx, &input)
	defer cancel()
	if !b.beginDispatch() {
		return nil, ErrBridgeClosed
	}
	defer b.endDispatch()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !b.hasSelectRoutes.Load() {
		return nil, errors.New("select-routes handler not set")
	}
	output, err := b.selectRoutes(ctx, input)
	if err != nil {
		return nil, err
	}
	if err := output.Validate(input); err != nil {
		return nil, err
	}
	return output, nil
}
