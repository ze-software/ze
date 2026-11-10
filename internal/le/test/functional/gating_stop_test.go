// Design: docs/architecture/testing/ci-format.md -- what a gating run is
// Related: run.go -- runPrepared and Execute, the stop this file proves

package testfunctional

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
)

// stopStub is the le a stub suite runs through. It ignores SIGTERM, starts a
// grandchild that inherits the ignore, records both pids and which suite
// started, then waits, so only a kill of the whole process group stops it.
const stopStub = `#!/bin/sh
trap '' TERM
echo "$2" >> "%DIR%/started"
sleep 300 &
echo $! > "%DIR%/grandchild.pid"
echo $$ > "%DIR%/child.pid"
wait
`

// TestGatingStopReapsSuiteGroupAndReleasesSet proves that a canceled gating
// run stops where it is and leaves nothing behind.
//
// Method: two stub suites run under the real commandLine (`timeout` around
// the set's le) through runPrepared, the loop runGating hands a built set. The
// first suite's le ignores SIGTERM and forks a grandchild that ignores it
// too. Once both have written their pids, ctx is canceled, as the signal root
// does on SIGINT or SIGTERM. Within five seconds both pids must be gone, the
// throwaway set's directory and the suite's raw coverage directory removed,
// the run answered with the interrupt's code, and the second suite never
// started.
//
// MUTATION: Execute back on gaterun.Stream (no ctx, no group kill) leaves both
// pids running to the 300 s sleep and the run never returns within the bound;
// drop `defer Release(set)` from runPrepared and the testbin directory
// survives; drop the `removeTree(cover)` of the interrupted arm and the raw
// coverage directory survives; drop both ctx checks of the loop and the run
// records the killed suite and answers 1, not 130. Each goes red here.
func TestGatingStopReapsSuiteGroupAndReleasesSet(t *testing.T) {
	scratch := t.TempDir()
	testbin := filepath.Join(scratch, "testbin-pid-1-functional")
	set := BinarySet{Dir: filepath.Join(testbin, "bin"), Remove: true}
	if err := os.MkdirAll(set.Dir, 0o750); err != nil {
		t.Fatal(err)
	}
	stub := strings.ReplaceAll(stopStub, "%DIR%", scratch)
	if err := os.WriteFile(set.lePath(), []byte(stub), 0o700); err != nil { //nolint:gosec // an executable stub is the subject
		t.Fatal(err)
	}
	covers := filepath.Join(scratch, "covdata")
	suites := []Suite{{Name: "stopfirst", Args: []string{"stopfirst"}}, {Name: "stopsecond", Args: []string{"stopsecond"}}}
	tc := gotoolchain.Toolchain{Root: scratch}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	type answer struct{ code int }
	done := make(chan answer, 1)
	go func() {
		_, code := runPrepared(ctx, tc, suites, set, covers, NewRun(len(suites)), newSuiteRecording("test"))
		done <- answer{code: code}
	}()

	child := waitForPid(t, filepath.Join(scratch, "child.pid"))
	grandchild := waitForPid(t, filepath.Join(scratch, "grandchild.pid"))
	// A red run must not leave the 300 s sleepers behind it.
	t.Cleanup(func() {
		_ = syscall.Kill(child, syscall.SIGKILL)
		_ = syscall.Kill(grandchild, syscall.SIGKILL)
	})
	cancel()

	var got answer
	select {
	case got = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the canceled gating run did not return within 10s")
	}
	if got.code != interruptedCode {
		t.Errorf("code = %d, want the interrupt's %d", got.code, interruptedCode)
	}
	for name, pid := range map[string]int{"suite child": child, "grandchild": grandchild} {
		if !processGone(pid, 5*time.Second) {
			t.Errorf("%s %d is still running after the stop", name, pid)
		}
	}
	for _, left := range []string{testbin, filepath.Join(covers, "stopfirst")} {
		if _, err := os.Lstat(left); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("the stop left %s behind: %v", left, err)
		}
	}
	started, err := os.ReadFile(filepath.Join(scratch, "started"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(started)) != "stopfirst" {
		t.Errorf("suites started = %q, want only stopfirst", started)
	}
}

// waitForPid reads the pid the stub writes, failing after ten seconds.
func waitForPid(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(path)
		if err == nil {
			if pid, parseErr := strconv.Atoi(strings.TrimSpace(string(raw))); parseErr == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the stub suite never wrote %s", path)
	return 0
}

// processGone polls kill(pid, 0) until it answers ESRCH or bound passes. An
// orphaned member is reparented and reaped by init, so its zombie is brief.
func processGone(pid int, bound time.Duration) bool {
	deadline := time.Now().Add(bound)
	for time.Now().Before(deadline) {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}
