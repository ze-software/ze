// Design: docs/architecture/api/process-protocol.md -- typed mutable origin intake.
// Related: bridge.go -- delivery admission and lifetime join.
package rpc

import (
	"context"
	"errors"
	"time"
)

func (b *DirectBridge) SetOriginIntake(handler OriginIntakeHandler) {
	b.originIntake = handler
	b.hasOriginIntake.Store(handler != nil)
}

func (b *DirectBridge) HasOriginIntake() bool { return b.ready.Load() && b.hasOriginIntake.Load() }

// OriginIntake returns only after the registered worker joins the mutation,
// including canceled work. A published bridge never falls back to the socket.
func (b *DirectBridge) OriginIntake(ctx context.Context, input OriginIntakeInput) (*UpdateRouteOutput, error) {
	ctx, cancel := OriginIntakeCallContext(ctx, &input)
	defer cancel()
	if !b.beginDelivery() {
		return nil, ErrBridgeClosed
	}
	defer b.deliveryWG.Done()
	if !b.ready.Load() {
		return nil, ErrBridgeClosed
	}
	if !b.hasOriginIntake.Load() {
		return nil, errors.New("origin-intake handler is not installed; register OnOriginIntake before Run")
	}
	if err := input.Validate(time.Now()); err != nil {
		return nil, err
	}
	output, err := b.originIntake(ctx, input)
	if output == nil {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("origin-intake handler returned no admission result")
	}
	if validation := ValidateOriginIntakeOutput(input, output); validation != nil {
		return nil, validation
	}
	resultErr := errors.Join(err, output.Err())
	return output, resultErr
}
