// Design: docs/architecture/config/transaction-protocol.md -- shutdown while a transaction runs
// Related: orchestrator.go -- rollBack, the section rollback that runs under a rollback context
// Related: executor.go -- rollbackApplied, the operation undo that runs under one

package transaction

import (
	"context"
	"errors"
	"time"
)

// shutdownScopeKey holds the scope context WithShutdownScope installs.
type shutdownScopeKey struct{}

// rollbackParentKey holds the context a rollback context was detached from.
type rollbackParentKey struct{}

// WithShutdownScope returns a context a shutdown can still reach after an
// ordinary cancellation has ended it.
//
// A context keeps the cause of its FIRST cancellation. A reload whose own
// deadline expired therefore reads as "deadline exceeded" for good, and a
// shutdown that arrives while its rollback runs would be invisible through
// context.Cause alone. The scope is a second context, carried as a value, that
// only the returned cancel function ends: calling it with ErrShutdown marks
// every context derived from the returned one as canceled by shutdown,
// whatever ended them first. A scope opened under another scope ends with it.
//
// The owner of a daemon-lifetime or transaction-lifetime context opens the
// scope, and cancels with ErrShutdown to stand the transaction down. The
// caller MUST call the returned function, with nil when the work simply ended,
// to release the scope.
func WithShutdownScope(parent context.Context) (context.Context, context.CancelCauseFunc) {
	base := context.WithoutCancel(parent)
	if outer := shutdownScope(parent); outer != nil {
		base = outer
	}
	scope, endScope := context.WithCancelCause(base)
	ctx, cancel := context.WithCancelCause(context.WithValue(parent, shutdownScopeKey{}, scope))
	return ctx, func(cause error) {
		endScope(cause)
		cancel(cause)
	}
}

// shutdownScope returns the innermost scope ctx was derived under, or nil
// when no owner opened one.
func shutdownScope(ctx context.Context) context.Context {
	scope, ok := ctx.Value(shutdownScopeKey{}).(context.Context)
	if !ok {
		return nil
	}
	return scope
}

// canceledByShutdown reports whether a shutdown stood ctx down: ErrShutdown is
// its cancellation cause, or the cause of the scope it was derived under
// (WithShutdownScope), or it is a rollback context detached from a context a
// shutdown stood down. context.Cause returns the ctx error itself when no
// cause was set, so a plain cancel and a deadline both answer false here.
//
// The recursion follows one rollback context to the context it was detached
// from. A rollback context is derived at most twice in a transaction (the
// operation undo, then nothing: the section restore shares it), so the depth
// is bounded by that nesting.
func canceledByShutdown(ctx context.Context) bool {
	if errors.Is(context.Cause(ctx), ErrShutdown) {
		return true
	}
	if scope := shutdownScope(ctx); scope != nil {
		if errors.Is(context.Cause(scope), ErrShutdown) {
			return true
		}
	}
	parent, detached := ctx.Value(rollbackParentKey{}).(context.Context)
	if !detached {
		return false
	}
	return canceledByShutdown(parent)
}

// RollbackContext returns the context a rollback runs under. It differs from
// its parent in three ways:
//
//   - an ordinary end of parent (a cancel, an expired deadline) does not end
//     it: a rollback exists because something ended early, and the daemon that
//     keeps running must still be told to undo the change;
//   - it ends after budget, the one bound the whole rollback shares;
//   - a shutdown of parent ends it at once, with ErrShutdown as its cause,
//     because every participant is about to be stopped and nothing is left to
//     restore.
//
// The caller MUST call the returned function when the rollback is over.
func RollbackContext(parent context.Context, budget time.Duration) (context.Context, context.CancelFunc) {
	detached := context.WithValue(context.WithoutCancel(parent), rollbackParentKey{}, parent)
	cancelable, end := context.WithCancelCause(detached)
	ctx, cancel := context.WithTimeout(cancelable, budget)

	// An ordinary end of parent or of its scope is not a shutdown, so each
	// watcher asks before it ends the rollback.
	endOnShutdown := func() {
		if canceledByShutdown(parent) {
			end(ErrShutdown)
		}
	}
	stopParentWatch := context.AfterFunc(parent, endOnShutdown)
	stopScopeWatch := func() bool { return true }
	if scope := shutdownScope(parent); scope != nil {
		stopScopeWatch = context.AfterFunc(scope, endOnShutdown)
	}
	return ctx, func() {
		stopParentWatch()
		stopScopeWatch()
		cancel()
		end(nil)
	}
}
