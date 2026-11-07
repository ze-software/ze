// Design: docs/architecture/diagnostics/crash-capture.md -- the crash directory a test points the package at
//
// VALIDATES: SetCrashDirForTest points the listing at its directory, consumes
//            Init's Once so a later Init cannot overwrite that directory, and
//            leaves os.Stderr and the stderr relay as it found them.
// PREVENTS:  the helper running the full Init mid-binary, which wrote the
//            process-wide os.Stderr while a logger goroutine read it: the data
//            race `go test -race` reported in the dhcpserver panic-guard tests.

package crashlog

import (
	"os"
	"runtime"
	"testing"
)

// TestSetCrashDirForTestLeavesStderrAlone proves the helper never swaps
// os.Stderr. A goroutine reads os.Stderr for the whole call, the way a logger
// goroutine started by an earlier test does, so a helper that writes it is a
// race `-race` reports; the direct comparison afterwards catches the same
// break without the race detector.
func TestSetCrashDirForTestLeavesStderrAlone(t *testing.T) {
	before := os.Stderr
	previousDir := crashDir

	stop := make(chan struct{})
	started := make(chan struct{})
	reads := make(chan int, 1)
	go readStderrUntilStopped(stop, started, reads)
	<-started

	dir := t.TempDir()
	restore := SetCrashDirForTest(dir)

	close(stop)
	if n := <-reads; n == 0 {
		t.Fatalf("the reader goroutine never read os.Stderr, so it proves nothing")
	}

	if os.Stderr != before {
		t.Fatalf("SetCrashDirForTest replaced os.Stderr: a test binary's logger goroutines read it")
	}
	if pipeW != nil {
		t.Fatalf("SetCrashDirForTest started the stderr relay")
	}
	if crashDir != dir {
		t.Fatalf("crashDir = %q, want %q", crashDir, dir)
	}
	if crashKeep < minKeep {
		t.Fatalf("crashKeep = %d, want at least %d: rotation would remove every record", crashKeep, minKeep)
	}

	// The Once is consumed, so the Init the code under test calls is a no-op.
	Init()
	if crashDir != dir {
		t.Fatalf("a later Init overwrote crashDir with %q, want %q", crashDir, dir)
	}
	if os.Stderr != before {
		t.Fatalf("a later Init replaced os.Stderr: SetCrashDirForTest left Init's Once unconsumed")
	}

	restore()
	if crashDir != previousDir {
		t.Fatalf("restore left crashDir = %q, want %q", crashDir, previousDir)
	}
}

// readStderrUntilStopped reads os.Stderr in a loop until stop closes. It closes
// started once it is running and sends on reads how many reads it made.
func readStderrUntilStopped(stop <-chan struct{}, started chan<- struct{}, reads chan<- int) {
	close(started)

	count := 0
	for {
		// This comparison is the read the race detector watches. Its result is
		// counted so the read is part of what the test asserts.
		if os.Stderr != nil {
			count++
		}
		select {
		case <-stop:
			reads <- count
			return
		default:
			runtime.Gosched()
		}
	}
}
