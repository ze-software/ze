// Design: docs/architecture/api/process-protocol.md -- failed-retention compensation.
// Related: origin_intake.go -- complete callback join precedes compensation.
package reactor

import (
	"context"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// compensateOriginIntake settles a possibly retained original after the joined
// announce callback failed. Caller cancellation cannot abandon its membership.
// All identity/build fields came from the issuer before the callback; no reply
// or name lookup can redirect cleanup to another generation or process.
func (a *reactorAPIAdapter) compensateOriginIntake(ctx context.Context, owner plugin.SelectedOwner, input rpc.OriginIntakeInput) error {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), rpc.CancellableRequestTimeout)
	defer cancel()
	// Revocation publishes before waiting or asking the provider to re-elect.
	// Even when the real-write join fails, the provider removal is attempted and
	// both failures remain explicit. The canceled cleanup context grants no ACK.
	retired := a.r.originReceipts.retireOriginReceipt(cleanup, input.OriginReceipt)
	removed := withdrawOriginIntake(cleanup, owner, input)
	if err := errors.Join(retired, removed); err != nil {
		return fmt.Errorf("origin intake candidate cleanup did not settle: %w", err)
	}
	return nil
}

// withdrawOriginIntake removes only this engine-captured original through its
// immutable authenticated connection. It may reach a still-live old owner after
// manager replacement, but never a same-name successor. Done/Ended or a failed
// Current check alone do not prove candidate destruction or a joined stop; that
// case stays an explicit failure for the lifecycle owner to settle.
// C2 retirement callers keep their own tracked cancellation context intact.
func withdrawOriginIntake(ctx context.Context, owner plugin.SelectedOwner, input rpc.OriginIntakeInput) error {
	input.Operation = rpc.OriginIntakeWithdraw
	input.DeadlineMS = 0 // A failed/canceled announcement deadline is not cleanup's deadline.
	work, cancel := rpc.OriginIntakeCallContext(ctx, &input)
	defer cancel()
	output, err := owner.OriginIntake(work, input)
	if err != nil {
		return err
	}
	if err := work.Err(); err != nil {
		return err
	}
	if err := rpc.ValidateOriginIntakeOutput(input, output); err != nil {
		return err
	}
	return output.Err()
}
