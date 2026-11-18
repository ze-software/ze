// Design: docs/architecture/api/process-protocol.md -- typed read-only callback lifetime.
// Related: bridge_current_selection.go, bridge_origin_admission_lookup.go -- distinct contracts.
package rpc

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

type bridgeReadInput interface {
	Validate(time.Time) error
}

// bridgeReadSnapshot joins delivery for both typed snapshot contracts. The
// caller MUST pass the installed handler's address, never read its value first.
// This helper MUST acquire readiness and presence before reading that value,
// preserving SetReady's publication of the handler. Output ownership and
// validation remain with the concrete protocol.
func bridgeReadSnapshot[Input bridgeReadInput, Output any, Handler ~func(context.Context, Input) (*Output, error)](
	bridge *DirectBridge, ctx context.Context, input Input,
	callContext func(context.Context, *Input) (context.Context, context.CancelFunc),
	present *atomic.Bool, installed *Handler, missingHandler string,
	validate func(*Output, Input) error) (*Output, error) {
	ctx, cancel := callContext(ctx, &input)
	defer cancel()
	if !bridge.beginDelivery() {
		return nil, ErrBridgeClosed
	}
	defer bridge.deliveryWG.Done()
	if !bridge.ready.Load() {
		return nil, ErrBridgeClosed
	}
	if !present.Load() {
		return nil, errors.New(missingHandler)
	}
	handler := *installed
	if err := input.Validate(time.Now()); err != nil {
		return nil, err
	}
	output, err := handler(ctx, input)
	if err != nil {
		return nil, err
	}
	if err := validate(output, input); err != nil {
		return nil, err
	}
	return output, nil
}
