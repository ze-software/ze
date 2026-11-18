// Design: docs/architecture/api/process-protocol.md -- final output settlement evidence.
package rpc

import (
	"context"
	"errors"
)

type CachedWriteFailureCause uint8

const (
	CachedWriteFailureUnspecified CachedWriteFailureCause = iota
	CachedWriteFailureCanceled
	CachedWriteFailureDeadline
)

// CachedWriteJoinedError is returned only from a validated exact completion
// receipt. No original-request writer remains queued or running. Requested
// output or cleanup still failed. Its zero value is not a completion receipt.
type CachedWriteJoinedError struct {
	reason string
	cause  error
}

func (err *CachedWriteJoinedError) Error() string { return err.reason }
func (err *CachedWriteJoinedError) Unwrap() error { return err.cause }

// CachedWriteNotDispatchedError is a local SDK refusal before engine dispatch.
// It is not a remote outcome and says nothing about earlier requests.
type CachedWriteNotDispatchedError struct{ cause error }

func (err *CachedWriteNotDispatchedError) Error() string { return err.cause.Error() }
func (err *CachedWriteNotDispatchedError) Unwrap() error { return err.cause }

// CachedWriteNotDispatched requires an actual local refusal before dispatch.
// A nil cause returns an ordinary error, never success or settlement evidence.
func CachedWriteNotDispatched(cause error) error {
	if cause == nil {
		return errors.New("cached write refusal has no cause, so dispatch remains unknown; supply the actual local refusal")
	}
	return &CachedWriteNotDispatchedError{cause: cause}
}

// CompletedCachedWrite is the server's output AFTER all admitted target writers
// joined, or after its evaluator refused before admission. The caller MUST NOT
// invoke it for an enqueued, running or unknown writer. It stores no history.
func CompletedCachedWrite(deadline int64, failure error) *CachedWriteOutput {
	output := &CachedWriteOutput{Completed: true, DeadlineMS: deadline}
	if failure == nil {
		return output
	}
	output.Failure = failure.Error()
	switch {
	case errors.Is(failure, context.Canceled):
		output.FailureCause = CachedWriteFailureCanceled
	case errors.Is(failure, context.DeadlineExceeded):
		output.FailureCause = CachedWriteFailureDeadline
	}
	return output
}
func cachedWriteFailureCause(value CachedWriteFailureCause) (error, error) {
	switch value {
	case CachedWriteFailureUnspecified:
		return nil, nil
	case CachedWriteFailureCanceled:
		return context.Canceled, nil
	case CachedWriteFailureDeadline:
		return context.DeadlineExceeded, nil
	default:
		return nil, errors.New("cached write failure cause is unknown; preserve the unsettled output obligation")
	}
}
