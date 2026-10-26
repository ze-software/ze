// Related: recordrun.go -- runCatalogScenario, the signal root this test drives.

package feature

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// VALIDATES: a SIGINT while feature run drives an Interop scenario cancels the
// context the scenario runs under, and the run answers the interruption, not
// a timeout and not a verdict.
// METHOD: the test holds its own subscription to SIGINT, so a root that does
// not listen leaves the process alive and the scenario answers "not
// canceled" instead of the default action killing the test binary.
// PREVENTS: a Ctrl-C killing le by the default action while the lab's docker
// clients, in process groups of their own, keep running.
func TestRecordRunScenarioCancelsOnSignal(t *testing.T) {
	held := make(chan os.Signal, 1)
	signal.Notify(held, syscall.SIGINT)
	defer signal.Stop(held)

	canceled := false
	catalog := interoplab.Catalog{Suite: "fixture", RunScenario: func(ctx context.Context, _, _ string) interoplab.SuiteReport {
		if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
			t.Errorf("signal self: %v", err)
			return interoplab.SuiteReport{}
		}
		select {
		case <-ctx.Done():
			canceled = true
		case <-time.After(5 * time.Second):
		}
		return interoplab.SuiteReport{}
	}}
	_, err := runCatalogScenario(catalog, t.TempDir(), "x")
	if !canceled {
		t.Fatal("SIGINT did not cancel the scenario's context")
	}
	if err == nil || !strings.Contains(err.Error(), "interrupted") {
		t.Fatalf("an interrupted scenario answered %v, want it named as interrupted", err)
	}
}
