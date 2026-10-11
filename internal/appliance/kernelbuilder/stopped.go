// Design: docs/architecture/appliance/kernel-profiles.md -- a build stopped by its context
// Related: driver.go -- runDocker, the Docker steps a stop names
// Related: qemu.go -- runQEMUBuild and buildGuestWorker, the QEMU steps a stop names

package kernelbuilder

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// buildLimit is the cause WithBuildLimit attaches to its deadline, so a build
// that runs out of it is told apart from one whose caller set a deadline of
// its own: only the first is Ze's limit, and only it is named as one.
type buildLimit struct{ limit time.Duration }

func (l buildLimit) Error() string {
	return "the " + limitWords(l.limit) + " limit Ze gives one kernel build"
}

// WithBuildLimit answers a context that ends after limit, the limit Ze gives
// one kernel build. A build stopped by it says it did not finish within that
// limit and names it; a deadline a caller sets with context.WithTimeout is
// reported as the caller's. The caller MUST call the CancelFunc.
func WithBuildLimit(parent context.Context, limit time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeoutCause(parent, limit, buildLimit{limit: limit})
}

// limitWords writes a whole number of hours as "2-hour", the way an operator
// reads a limit, and any other duration in Go's own form.
func limitWords(limit time.Duration) string {
	if limit >= time.Hour && limit%time.Hour == 0 {
		return strconv.Itoa(int(limit/time.Hour)) + "-hour"
	}
	return limit.String()
}

// errBuildStopped reports a build step that ended because ctx did. The caller
// MUST check ctx.Err() first: the step's own error ("signal: killed") reads as
// a failed step, and it is not one. stopped names what stopped and when ("the
// build VM was stopped while it set up SSH"). A canceled build is reported as
// canceled, with no next step, because the operator stopped it. A build out of
// Ze's limit (WithBuildLimit) says it did not finish within that limit, and a
// build out of a deadline its caller set says so; both end with next, the
// step's own next step, on lines of their own.
func errBuildStopped(ctx context.Context, stopped, next string) error {
	ctxErr := ctx.Err()
	if errors.Is(ctxErr, context.Canceled) {
		return fmt.Errorf("%s: the kernel build was canceled (%w)", stopped, ctxErr)
	}
	if limit, ok := errors.AsType[buildLimit](context.Cause(ctx)); ok {
		return fmt.Errorf("%s: the kernel build did not finish within %s (%w)\n%s", stopped, limit.Error(), ctxErr, next)
	}
	return fmt.Errorf("%s: the kernel build ran out of the time its caller gave it (%w)\n%s", stopped, ctxErr, next)
}
