// Design: docs/architecture/api/process-protocol.md -- constrained original peer commands.
package rpc

import (
	"context"
	"errors"
	"time"
)

// PeerBound distinguishes explicit physical constraints from ordinary command
// dispatch. A partial tuple is bound but invalid; it never selects an operator
// path. Safe for concurrent use while the caller leaves the raw input unchanged.
func (input DispatchCommandInput) PeerBound() bool {
	if input.PeerOwner != 0 {
		return true
	}
	if input.ConnectionID != 0 {
		return true
	}
	return input.DeadlineMS != 0
}

func (input DispatchCommandInput) ValidatePeer(now time.Time) error {
	if input.DestinationBound() {
		return errors.New("physical command mixes destination constraints; preserve only the original physical pair")
	}
	if input.PeerOwner == 0 {
		return errors.New("physical peer command has no original peer-owner; preserve the received owner")
	}
	if input.ConnectionID == 0 {
		return errors.New("physical peer command has no original connection-id; preserve the received physical connection")
	}
	if input.Command == "" {
		return errors.New("physical peer command is empty; send the registered peer command")
	}
	if input.DeadlineMS <= now.UnixMilli() {
		return errors.New("physical peer command deadline expired; retry with a live bounded deadline")
	}
	if input.DeadlineMS > now.Add(CancellableRequestTimeout).UnixMilli() {
		return errors.New("physical peer command deadline exceeds thirty seconds; send a bounded request")
	}
	return nil
}

func (output *DispatchCommandOutput) ValidatePeerAnswer(input DispatchCommandInput) error {
	if output == nil {
		return errors.New("physical peer command answer is absent; return its settled owner, connection and deadline")
	}
	if output.PeerOwner == 0 {
		return errors.New("physical peer command answer has no original owner; echo the settled original peer-owner")
	}
	if output.ConnectionID == 0 {
		return errors.New("physical peer command answer has no original connection; echo the settled original connection-id")
	}
	if output.DeadlineMS <= 0 {
		return errors.New("physical peer command answer has no bounded deadline; echo the exact settled deadline-ms")
	}
	if output.PeerOwner != input.PeerOwner {
		return errors.New("physical peer command answer names another owner; echo the settled original peer-owner")
	}
	if output.ConnectionID != input.ConnectionID {
		return errors.New("physical peer command answer names another connection; echo the settled original connection-id")
	}
	if output.DeadlineMS != input.DeadlineMS {
		return errors.New("physical peer command answer names another deadline; echo the exact settled deadline-ms")
	}
	return nil
}

// PeerCommandCallContext normalizes one bounded physical command deadline. Its
// cancel MUST follow the existing remote cancellation join, never precede it.
func PeerCommandCallContext(ctx context.Context, input *DispatchCommandInput) (context.Context, context.CancelFunc) {
	work, cancel := context.WithTimeout(ctx, CancellableRequestTimeout)
	if input.DeadlineMS > 0 {
		deadline, bounded := work.Deadline()
		if bounded {
			if time.UnixMilli(input.DeadlineMS).Before(deadline) {
				cancel()
				work, cancel = context.WithDeadline(ctx, time.UnixMilli(input.DeadlineMS))
			}
		}
	}
	deadline, bounded := work.Deadline()
	if bounded {
		input.DeadlineMS = deadline.UnixMilli()
	}
	return work, cancel
}
