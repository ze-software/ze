// Design: docs/functional-tests.md -- canceled native invocations retain cleanup failures.

package teststressrepro

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

// TestCanceledStressCommandRetainsCleanupError cancels after the owned leader
// was reaped while the test retains a duplicate of its output writer. The
// existing output-drain bound produces a real owner error alongside cancellation.
func TestCanceledStressCommandRetainsCleanupError(t *testing.T) {
	root := t.TempDir()
	executable := stressFixtureProgram(t)
	listener, socket := stressDescriptorSocket(t)
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan processResult, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		result <- runCommand(ctx, root, os.Environ(), executable,
			"-test.run=^TestStressProcessHelper$", "--", "stress-process-fixture", "output", socket)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	ready, stopReady := context.WithTimeout(t.Context(), 5*time.Second)
	defer stopReady()
	descriptor := receiveStressDescriptor(t, ready, listener)
	info, err := descriptor.file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeNamedPipe == 0 {
		t.Fatal("helper did not transfer its output pipe")
	}
	descriptor.acknowledge(t)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat("/proc/" + strconv.Itoa(descriptor.leader)); errors.Is(err, os.ErrNotExist) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		select {
		case joined := <-result:
			t.Fatalf("owner ended before cancellation: %+v", joined)
		case <-ready.Done():
			t.Fatal("owned leader was not reaped")
		case <-ticker.C:
		}
	}
	cancel()
	joined := <-result
	if joined.code != 1 {
		t.Fatalf("canceled command code=%d, want 1: %+v", joined.code, joined)
	}
	if !errors.Is(joined.err, context.Canceled) {
		t.Fatalf("cancellation cause was lost: %v", joined.err)
	}
	if !errors.Is(joined.err, exec.ErrWaitDelay) {
		t.Fatalf("owner output-drain failure was lost: %v", joined.err)
	}
}
