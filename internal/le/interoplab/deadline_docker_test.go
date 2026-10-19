//go:build docker

// Design: docs/architecture/testing/interop.md -- the docker-tagged tests.
package interoplab

import (
	"context"
	"testing"
	"time"
)

// testReportMargin is how long before the test binary's deadline a docker
// test's context ends, so the step that was waiting fails with what it waited
// for instead of the harness panicking over every goroutine's stack.
const testReportMargin = 10 * time.Second

// testDeadlineContext answers the test's context, ending testReportMargin
// before t.Deadline(), or the test's context alone when the run declares no
// deadline (-timeout 0). A docker test sets no bound of its own on a build or
// a run: the wrapper's bounds stop a wedged daemon and are not a budget
// (dockerBuildTimeoutDefault), and a constant here would be a budget that a
// population running every docker package in parallel overruns. The run's
// own deadline is the one budget, and the caller of `go test` sets it.
func testDeadlineContext(t *testing.T) context.Context {
	t.Helper()
	deadline, ok := t.Deadline()
	if !ok {
		return t.Context()
	}
	ctx, cancel := context.WithDeadline(t.Context(), deadline.Add(-testReportMargin))
	t.Cleanup(cancel)
	return ctx
}
