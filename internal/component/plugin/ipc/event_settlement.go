// Design: docs/architecture/api/process-protocol.md -- applied event settlement.
// Related: rpc.go -- PluginConn owns the exact callback connection.
package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// SendEventSettlement waits for completed asynchronous work on this connection.
// The caller MUST first await event application and retain this exact process
// lifetime until return. A callback receipt MUST confirm its effective deadline;
// transport acceptance, malformed replies and expired success never complete it.
// The work deadline and bounded reply wait are separate: an expired failure may
// name unfinished work, but reply waiting MUST never outlive the caller.
// Safe for concurrent use; no Process or delivery lock may be held by its caller.
func (pc *PluginConn) SendEventSettlement(ctx context.Context, input rpc.EventSettlementInput) error {
	if err := input.Validate(time.Now()); err != nil {
		return err
	}
	// The plugin's work cannot exceed an earlier caller ceiling. The reply
	// may follow work expiry, within that same ceiling and the worker's
	// existing answer-write allowance.
	if deadline, ok := ctx.Deadline(); ok {
		input.DeadlineMS = min(input.DeadlineMS, deadline.UnixMilli())
	}
	if err := input.Validate(time.Now()); err != nil {
		return err
	}
	deadline := time.UnixMilli(input.DeadlineMS)
	reply, cancel := context.WithDeadline(ctx, deadline.Add(rpc.CallbackAnswerWriteMax))
	defer cancel()
	if err := reply.Err(); err != nil {
		return err
	}
	data, err := pc.CallRPC(reply, rpc.MethodEventSettlement, input)
	if err != nil {
		return err
	}
	var output rpc.EventSettlementOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return fmt.Errorf("decode event-settlement receipt: %w", err)
	}
	if err := output.Validate(input); err != nil {
		return err
	}
	if output.Status != rpc.StatusOK {
		if deadline.After(time.Now()) {
			return fmt.Errorf("event settlement failed: %s", output.Error)
		}
		return fmt.Errorf("event settlement failed: %s: %w", output.Error, context.DeadlineExceeded)
	}
	if err := reply.Err(); err != nil {
		return err
	}
	if err := pc.Err(); err != nil {
		return err
	}
	if !time.Now().Before(deadline) {
		return context.DeadlineExceeded
	}
	return nil
}
