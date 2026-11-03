// Design: docs/architecture/testing/runner-architecture.md -- one browser case budget.
package webtesting

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/ze-software/ze/internal/test/runner"
)

type caseBudgetKey struct{}

// CaseContext starts the case deadline before server startup. The scheduler
// MUST supply its concurrency and MUST cancel the returned context after cleanup.
// Zero authored duration means unspecified; negative or overflowing values fail.
func CaseContext(parent context.Context, authored time.Duration, concurrency int) (context.Context, context.CancelFunc, error) {
	if concurrency < 1 {
		return nil, nil, errors.New("browser concurrency must be positive")
	}
	if authored == 0 {
		authored = defaultWBTimeout
	}
	factor := time.Duration(1)
	if concurrency > 1 {
		factor = runner.ParallelTimeoutHeadroom
	}
	if authored < 0 {
		return nil, nil, errors.New("browser timeout must be positive")
	}
	if authored > math.MaxInt64/factor {
		return nil, nil, errors.New("browser timeout overflows with concurrency headroom")
	}
	ctx := context.WithValue(parent, caseBudgetKey{}, factor)
	ctx, cancel := context.WithTimeoutCause(ctx, authored*factor,
		fmt.Errorf("option=timeout budget %s exhausted: %w", authored*factor, context.DeadlineExceeded))
	return ctx, cancel, nil
}

// Bound scales an inner limit once using the scheduler's case policy. A caller
// MUST use this with the case context so an inner timeout cannot extend it.
func Bound(ctx context.Context, limit time.Duration) time.Duration {
	factor, ok := ctx.Value(caseBudgetKey{}).(time.Duration)
	if !ok {
		return limit // A standalone browser command has no scheduler contention.
	}
	return limit * factor
}

// bounded scopes synchronous operations on this browser to an inner deadline.
// Browser is single-owner and MUST NOT be used concurrently. The caller MUST
// restore its parent context after the operation.
func (b *Browser) bounded(limit time.Duration) func() {
	parent := b.ctx
	ctx, cancel := context.WithTimeout(parent, Bound(parent, limit))
	b.ctx = ctx
	return func() {
		cancel()
		b.ctx = parent
	}
}

func waitContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-timer.C:
		return ctx.Err()
	}
}
