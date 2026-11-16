// Design: docs/architecture/api/ipc_protocol.md -- acknowledged request cancellation.
package rpc

import (
	"context"
	"fmt"
	"time"
)

// MethodCancelRequest cancels only an admitted opt-in request on this same
// authenticated connection. Its answer follows the target's final settlement.
const MethodCancelRequest = "ze-plugin-engine:cancel-request"

// CancellableRequestTimeout bounds opt-in work even if its caller supplied
// no deadline. RequestCancelTimeout separately bounds the cancellation join.
const (
	CancellableRequestTimeout = 30 * time.Second
	RequestCancelTimeout      = 5 * time.Second
)

// CancelRequestInput names transport correlation, never process authority.
type CancelRequestInput struct {
	RequestID uint64 `json:"request-id"`
	Method    string `json:"method"`
}

// CancelRequestOutcome distinguishes a joined request from one no longer live.
// NotActive is safe only because the connection reader admits before dispatch
// and reads the cancellation after the complete original request frame.
type CancelRequestOutcome uint8

const (
	CancelRequestUnspecified CancelRequestOutcome = iota
	CancelRequestSettled
	CancelRequestNotActive
)

// CancelRequestOutput carries no completion claim for another connection.
type CancelRequestOutput struct {
	RequestID uint64               `json:"request-id"`
	Outcome   CancelRequestOutcome `json:"outcome"`
}

// CancellationUnsettledError means the caller stopped waiting but could not
// prove that remote work joined. A lifecycle barrier MUST report this failure,
// not treat it as a completed cancellation. It preserves the caller's cause.
type CancellationUnsettledError struct {
	cause      error
	settlement error
}

func (err *CancellationUnsettledError) Error() string {
	return fmt.Sprintf("request cancellation did not settle remotely: %v: %v", err.cause, err.settlement)
}

func (err *CancellationUnsettledError) Unwrap() error { return err.cause }

// SelectedCallContext bounds one selected call and carries its absolute work
// deadline to the socket recipient. The caller MUST invoke the returned cancel;
// consumers MUST honor this context and finish their writer before returning.
func SelectedCallContext(ctx context.Context, input *SelectRoutesInput) (context.Context, context.CancelFunc) {
	work, cancel := context.WithTimeout(ctx, CancellableRequestTimeout)
	if input.DeadlineMS > 0 {
		remote := time.UnixMilli(input.DeadlineMS)
		deadline, _ := work.Deadline()
		if remote.Before(deadline) {
			cancel()
			work, cancel = context.WithDeadline(ctx, remote)
		}
	}
	deadline, _ := work.Deadline()
	input.DeadlineMS = deadline.UnixMilli()
	return work, cancel
}
