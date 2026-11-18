// Design: docs/architecture/api/process-protocol.md -- captured cached-write constraints.
package rpc

import (
	"context"
	"errors"
	"time"
)

// ValidateCachedWrite validates boundary scalars, never grants. Both zero keep
// the intentional unbound operation. A partial tuple never selects a successor.
func ValidateCachedWrite(owner, session uint64, deadline int64) error {
	if owner == 0 {
		if session != 0 {
			return errors.New("cached write destination-owner is missing; supply both original destination identifiers")
		}
		if deadline != 0 {
			return errors.New("unbound cached write has a deadline; supply its original destination identifiers or omit all constraints")
		}
		return nil
	}
	if session == 0 {
		return errors.New("cached write destination-session is missing; supply both original destination identifiers")
	}
	if deadline <= 0 {
		return errors.New("bound cached write deadline is missing; supply its positive absolute deadline")
	}
	return nil
}

// CachedWriteContext normalizes only the bounded variant. The caller MUST call
// cancel after its final receipt or joined cancellation has returned.
func CachedWriteContext(ctx context.Context, owner, session uint64, deadline *int64) (context.Context, context.CancelFunc, error) {
	if deadline == nil {
		return nil, nil, errors.New("cached write deadline storage is missing")
	}
	if *deadline < 0 {
		return nil, nil, errors.New("cached write deadline is negative")
	}
	if owner == 0 {
		if err := ValidateCachedWrite(owner, session, *deadline); err != nil {
			return nil, nil, err
		}
		return ctx, func() {}, nil
	}
	if session == 0 {
		return nil, nil, errors.New("cached write destination-session is missing")
	}
	return BoundCachedWriteContext(ctx, deadline)
}

// BoundCachedWriteContext normalizes a validated bound operation deadline.
// The caller MUST cancel only after final completion or joined cancellation.
func BoundCachedWriteContext(ctx context.Context, deadline *int64) (context.Context, context.CancelFunc, error) {
	if deadline == nil {
		return nil, nil, errors.New("cached write deadline storage is missing")
	}
	if *deadline < 0 {
		return nil, nil, errors.New("cached write deadline is negative")
	}
	work, cancel := context.WithTimeout(ctx, CancellableRequestTimeout)
	if *deadline > 0 {
		current, ok := work.Deadline()
		if ok {
			if remote := time.UnixMilli(*deadline); remote.Before(current) {
				cancel()
				work, cancel = context.WithDeadline(ctx, remote)
			}
		}
	}
	limit, ok := work.Deadline()
	if !ok {
		cancel()
		return nil, nil, errors.New("cached write work deadline could not be established")
	}
	*deadline = limit.UnixMilli()
	return work, cancel, nil
}

// CachedWriteOutput proves the bound evaluator joined its final flush. It is
// not send authority or a source ownership receipt. Nil/null never acknowledges
// a bound request, and an enqueue-only peer cannot return this scoped receipt.
type CachedWriteOutput struct {
	Completed    bool                    `json:"completed"`
	DeadlineMS   int64                   `json:"deadline-ms"`
	Failure      string                  `json:"failure,omitempty"`
	FailureCause CachedWriteFailureCause `json:"failure-cause,omitempty"`
}

func (output *CachedWriteOutput) Validate(deadline int64) error {
	if output == nil {
		return errors.New("bound cached write has no completion receipt")
	}
	if !output.Completed {
		return errors.New("bound cached write did not complete its final writer")
	}
	if deadline <= 0 {
		return errors.New("bound cached write request has no deadline")
	}
	if output.DeadlineMS != deadline {
		return errors.New("bound cached write completion belongs to another request deadline")
	}
	if output.Failure != "" {
		cause, err := cachedWriteFailureCause(output.FailureCause)
		if err != nil {
			return err
		}
		return &CachedWriteJoinedError{reason: output.Failure, cause: cause}
	}
	if output.FailureCause != CachedWriteFailureUnspecified {
		return errors.New("cached completion cause has no failure; return the complete settled outcome")
	}
	return nil
}
