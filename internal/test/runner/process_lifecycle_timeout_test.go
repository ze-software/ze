// Design: docs/architecture/testing/runner-architecture.md -- the foreground-timeout return joins its owned helpers.
// Related: process_lifecycle_test.go -- TestRunnerOwnedReturnPaths, the return arms this one slows down.

package runner

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// ownedReapDelay holds the background helper's reaper between its exit and its
// reap. It is far longer than runTest needs to return once nothing waits.
const ownedReapDelay = 500 * time.Millisecond

// TestForegroundTimeoutJoinsOwnedHelperBeforeRelease proves the P-3 obligation
// on the foreground-timeout return: the run stops AND waits for its owned
// background helper before it releases the port lease and returns.
//
// The foreground-timeout arm of TestRunnerOwnedReturnPaths cannot tell a join
// from a race. The test deadline that ends the foreground helper also SIGKILLs
// the background group through its context, about 200ms before runTest
// returns, so the helper's own reaper has normally collected it by the time
// anyone looks, whether or not the run waited for it.
//
// Method: the observe hook delays the background helper's reaper after its
// exit and before Cmd.Wait, so the child stays an unreaped zombie for
// ownedReapDelay. A run that joins its owner returns only after that reap,
// with the lease still held during the delay; a run that does not join
// returns first, releasing the lease while the helper is unreaped.
func TestForegroundTimeoutJoinsOwnedHelperBeforeRelease(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	termFile := filepath.Join(t.TempDir(), "term")
	script := filepath.Join(t.TempDir(), "child.sh")
	if err := os.WriteFile(script, []byte(`trap 'echo TERM > "$OWNED_TERM_FILE"; exit 0' TERM
echo $$ > "$OWNED_PID_FILE"
echo owned-ready
while :; do sleep 0.05; done
`), 0o600); err != nil {
		t.Fatal(err)
	}
	r, rec := lastHelperRecord(t, "foreground-timeout-join", "200ms", &RunCommand{
		Mode: modeBackground, Seq: 1, Name: "owned", Ready: "owned-ready",
		Exec: `sh "` + script + `"`,
	})
	rec.EnvVars = append(rec.EnvVars, "OWNED_PID_FILE="+pidFile, "OWNED_TERM_FILE="+termFile)
	rec.RunCommands = append(rec.RunCommands, RunCommand{Mode: modeForeground, Seq: 2, Exec: "sleep 60"})

	// leaseAtReap records whether the run still held its port lease when the
	// delayed reaper finished waiting. The hook writes it before closing
	// observed, and the test reads it only after observed closes.
	var leaseAtReap struct {
		released bool
		err      error
	}
	observed := make(chan struct{})
	var observeOnce sync.Once
	r.observeProcessExit = func(p *ownedProcess) error {
		exitErr := p.beforeReap()
		if !strings.Contains(strings.Join(p.cmd.Args, " "), script) {
			return exitErr
		}
		time.Sleep(ownedReapDelay)
		reservation, acquired, err := LockPort(rec.Port)
		if acquired {
			reservation.Release()
		}
		observeOnce.Do(func() {
			leaseAtReap.released = acquired
			leaseAtReap.err = err
			close(observed)
		})
		return exitErr
	}

	if r.runTest(t.Context(), rec, &RunOptions{}) {
		t.Fatalf("a foreground helper past the deadline passed: %v", rec.Error)
	}
	if rec.Error == nil || !strings.Contains(rec.Error.Error(), "foreground helper did not exit within 200ms") {
		t.Errorf("error=%v, want the foreground-timeout error", rec.Error)
	}
	assertOwnedPIDReaped(t, pidFile)

	// A run that did not join returns while the reaper still sleeps, so wait
	// for the observation rather than reading it early.
	deadline := time.NewTimer(10 * ownedReapDelay)
	defer deadline.Stop()
	select {
	case <-observed:
	case <-deadline.C:
		t.Fatal("the background helper's reaper never ran the delayed observation, so the join was not exercised")
	}
	if leaseAtReap.err != nil {
		t.Fatalf("could not probe the port lease at reap: %v", leaseAtReap.err)
	}
	if leaseAtReap.released {
		t.Errorf("port lease %d was released before the owned helper was reaped", rec.Port)
	}
	if _, err := os.Stat(termFile); err == nil {
		t.Errorf("the deadline teardown ran the helper's SIGTERM handler instead of killing it")
	}
}
