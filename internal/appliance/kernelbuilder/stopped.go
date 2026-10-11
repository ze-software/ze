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
	return "the " + LimitWords(l.limit) + " limit Ze gives one kernel build"
}

// WithBuildLimit answers a context that ends after limit, the limit Ze gives
// one kernel build. A build stopped by it says it did not finish within that
// limit and names it; a deadline a caller sets with context.WithTimeout is
// reported as the caller's. The caller MUST call the CancelFunc.
func WithBuildLimit(parent context.Context, limit time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeoutCause(parent, limit, buildLimit{limit: limit})
}

// LimitWords writes a limit the way an operator reads one: a whole number of
// hours as "2-hour", a whole number of minutes as "10-minute", and any other
// duration in Go's own form ("300ms"). It is the one formatter for every limit
// a build step names (the kernel build here, the firmware download and the ISO
// tools in internal/appliance), so the three never word a limit differently.
func LimitWords(limit time.Duration) string {
	if limit >= time.Hour && limit%time.Hour == 0 {
		return strconv.Itoa(int(limit/time.Hour)) + "-hour"
	}
	if limit >= time.Minute && limit%time.Minute == 0 {
		return strconv.Itoa(int(limit/time.Minute)) + "-minute"
	}
	return limit.String()
}

// errBuildStopped reports a build step that ended because ctx did, for a
// step whose stop leaves nothing running (errBuildStoppedLeaving). The caller
// MUST check ctx.Err() first: the step's own error ("signal: killed") reads as
// a failed step, and it is not one. stopped names what stopped and when ("the
// build VM was stopped while it set up SSH"). A canceled build is reported as
// canceled, with no next step, because the operator stopped it. A build out of
// Ze's limit (WithBuildLimit) says it did not finish within that limit, and a
// build out of a deadline its caller set says so; both end with next, the
// step's own next step, on lines of their own.
func errBuildStopped(ctx context.Context, stopped, next string) error {
	return errBuildStoppedLeaving(ctx, stopped, next, "")
}

// errBuildStoppedLeaving is errBuildStopped for a step whose stop can leave
// work running that the next build trips over (the kernel container, which a
// stop of the Docker CLI does not stop). left says what is left and how to
// clear it. It is cleanup, not a retry, so it is printed on every branch,
// after next, a canceled build included.
func errBuildStoppedLeaving(ctx context.Context, stopped, next, left string) error {
	cleanup := ""
	if left != "" {
		cleanup = "\n" + left
	}
	ctxErr := ctx.Err()
	if errors.Is(ctxErr, context.Canceled) {
		return &buildStopped{fmt.Errorf("%s: the kernel build was canceled (%w)%s", stopped, ctxErr, cleanup)}
	}
	if limit, ok := errors.AsType[buildLimit](context.Cause(ctx)); ok {
		return &buildStopped{fmt.Errorf("%s: the kernel build did not finish within %s (%w)\n%s%s", stopped, limit.Error(), ctxErr, next, cleanup)}
	}
	return &buildStopped{fmt.Errorf("%s: the kernel build ran out of the time its caller gave it (%w)\n%s%s", stopped, ctxErr, next, cleanup)}
}

// buildStopped is the report errBuildStoppedLeaving writes. Build reads the
// type to tell a step that named its own stop from one that returned the raw
// error its library gave (stopReport).
type buildStopped struct{ report error }

func (s *buildStopped) Error() string { return s.report.Error() }
func (s *buildStopped) Unwrap() error { return s.report }

// stopReport answers err as Build reports it, for SelectBuilder's error and
// for every backend error alike. A step that ended because ctx did and named
// the stop itself (errBuildStopped) is answered as it is. Any other step that
// returned under a stop returned what its library said:
// net/http answers context.Cause(ctx), so under Ze's limit the whole error
// would be the limit's own noun phrase, and exec answers "signal: killed".
// That error is kept as evidence after the stop is named, so no stop is ever
// reported by its cause marker alone.
func stopReport(ctx context.Context, err error) error {
	if ctx.Err() == nil {
		return err
	}
	if _, named := errors.AsType[*buildStopped](err); named {
		return err
	}
	return errBuildStopped(ctx, "the kernel build was stopped part way (its last step said: "+err.Error()+")",
		"the build output above shows the last step it reached")
}
