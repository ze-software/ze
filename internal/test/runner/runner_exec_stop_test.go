// Design: docs/architecture/testing/ci-format.md -- test execution utilities
// Related: runner_exec_util.go -- foregroundHelperStopError
// VALIDATES: a foreground helper killed because the run ended is reported as
//            the run ending, not as the test's own timeout.
// PREVENTS: a suite-budget kill read as a hung helper.

package runner

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestForegroundHelperStopErrorNamesWhoseClockRanOut proves the runner tells a
// suite-budget kill apart from the test's own timeout. Method: cancel the
// parent context (what the suite budget's SIGTERM does) and require the "run
// ended" diagnosis; expire only a child deadline and require the timeout one.
// The 2026-10-16 CI log blamed a "45m0s" test timeout for a 600s suite kill.
func TestForegroundHelperStopErrorNamesWhoseClockRanOut(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	cancel()
	ended := foregroundHelperStopError(parent, 1, "le test fixture ui/x", 45*time.Minute).Error()
	if !strings.Contains(ended, "the run ended before this foreground helper exited") {
		t.Errorf("parent canceled: error does not say the run ended first: %s", ended)
	}
	if strings.Contains(ended, "did not exit within") {
		t.Errorf("parent canceled: error still blames the test timeout: %s", ended)
	}

	live := context.Background()
	child, childCancel := context.WithTimeout(live, time.Nanosecond)
	defer childCancel()
	<-child.Done()
	own := foregroundHelperStopError(live, 2, "sh helper.sh", 15*time.Second).Error()
	if !strings.Contains(own, "cmd seq=2 (sh helper.sh): foreground helper did not exit within 15s") {
		t.Errorf("own timeout: error does not name the test timeout: %s", own)
	}
}
