//go:build linux

// Design: docs/functional-tests.md -- prepared native execution and joined cleanup.

package teststressrepro

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
)

func TestNativeStressProcessUsesPreparedEnvironment(t *testing.T) {
	root := t.TempDir()
	le := filepath.Join(root, "le")
	body := "#!/bin/sh\nprintf '%s\\n' \"$PATH\" \"$ZE_RUN_JOB\" \"$ZE_BIN\" \"$ZE_TEST_NETNS\" \"$STRESS_PREPARED_ONLY\"\n"
	if err := os.WriteFile(le, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	spec := invocation{
		root: root, harness: []string{"bgp", "plugin", "--draft"}, test: "native-selected",
		testBin: le, zeBin: "/prepared/ze", timeout: 2 * time.Second,
		environment: []string{"PATH=/prepared/bin:/usr/bin", "ZE_RUN_JOB=/prepared/parent", "ZE_BIN=/prepared/ze", "ZE_TEST_NETNS=1", "STRESS_PREPARED_ONLY=retained"},
	}
	result := (realProcessRunner{}).Invoke(context.Background(), spec)
	if !result.succeeded() {
		t.Fatalf("prepared native invocation failed: %+v", result)
	}
	want := "/prepared/bin:/usr/bin\n/prepared/parent\n/prepared/ze\n1\nretained\n"
	if result.output != want {
		t.Fatalf("prepared namespace/job/binary environment lost:\n got %q\nwant %q", result.output, want)
	}
}

func TestNativeStressProcessJoinsDescendantsAfterLeaderExit(t *testing.T) {
	root := t.TempDir()
	pidFile := filepath.Join(root, "descendant")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result := runCommand(ctx, root, os.Environ(), "/bin/sh", "-c", "sleep 30 </dev/null >/dev/null 2>&1 & echo $! > \"$1\"", "fixture", pidFile)
	if !result.succeeded() {
		t.Fatalf("controlled leader failed before cleanup: %+v", result)
	}
	pid := stressDescendantPID(t, pidFile)
	t.Cleanup(func() { stopStressDescendant(t, pid) })
	if stressDescendantLive(t, pid) {
		t.Fatal("successful invocation returned while its native descendant still runs")
	}
}

func TestNativeStressProcessCancellationJoinsDescendants(t *testing.T) {
	root := t.TempDir()
	pidFile := filepath.Join(root, "descendant")
	le := filepath.Join(root, "le")
	body := "#!/bin/sh\nsleep 30 </dev/null >/dev/null 2>&1 &\necho $! > " + strconv.Quote(pidFile) + "\nwait\n"
	if err := os.WriteFile(le, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result := make(chan processResult, 1)
	go invokeStressCancellation(ctx, invocation{root: root, harness: []string{"bgp", "plugin"}, test: "selected", testBin: le, zeBin: "/prepared/ze", timeout: time.Minute}, result)
	t.Cleanup(func() { cancel() })
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(pidFile); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			cancel()
			<-result
			t.Fatal(err)
		}
		select {
		case <-ctx.Done():
			<-result
			t.Fatal("controlled descendant never reached its ready marker")
		case <-ticker.C:
		}
	}
	pid := stressDescendantPID(t, pidFile)
	t.Cleanup(func() { stopStressDescendant(t, pid) })
	cancel()
	joined := <-result
	if joined.code == 0 {
		t.Fatalf("canceled invocation reported success: %+v", joined)
	}
	if stressDescendantLive(t, pid) {
		t.Fatal("canceled invocation returned while its native descendant still runs")
	}
}

func invokeStressCancellation(ctx context.Context, spec invocation, result chan<- processResult) {
	result <- (realProcessRunner{}).Invoke(ctx, spec)
}

func stressDescendantPID(t *testing.T, path string) int {
	t.Helper()
	value, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(value)))
	if err != nil {
		t.Fatal(err)
	}
	if pid <= 1 {
		t.Fatalf("invalid controlled descendant PID %d", pid)
	}
	return pid
}

func stressDescendantLive(t *testing.T, pid int) bool {
	t.Helper()
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	// Linux proc_single_show returns ESRCH if the task disappears before the read.
	// https://github.com/torvalds/linux/blob/master/fs/proc/base.c
	if errors.Is(err, syscall.ESRCH) {
		return false
	}
	if err != nil {
		t.Fatal(err)
	}
	_, tail, found := strings.Cut(string(stat), ") ")
	if !found {
		t.Fatalf("cannot read controlled descendant state: %q", stat)
	}
	return !strings.HasPrefix(tail, "Z ")
}

func stopStressDescendant(t *testing.T, pid int) {
	t.Helper()
	if !stressDescendantLive(t, pid) {
		return
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		if !errors.Is(err, syscall.ESRCH) {
			t.Errorf("stop controlled descendant: %v", err)
		}
	}
}
