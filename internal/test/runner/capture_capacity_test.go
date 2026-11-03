// Design: docs/architecture/testing/runner-architecture.md -- complete output capture.

package runner

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// TestCaptureBeyondPipeCapacity requires exact capture of a write larger than
// a pipe can buffer. The watchdog closes only this capture's writer on a stall,
// so the old write-before-read ordering fails rather than hanging the package.
func TestCaptureBeyondPipeCapacity(t *testing.T) {
	want := strings.Repeat("capture-boundary\x00\xff\n", 1<<18)
	var writeErr error
	var written int
	var expired bool
	got := captureStdout(t, func() {
		writer := os.Stdout
		done := make(chan struct{})
		joined := make(chan struct{})
		go func() {
			defer close(joined)
			timer := time.NewTimer(5 * time.Second)
			defer timer.Stop()
			select {
			case <-done:
			case <-timer.C:
				expired = true
				t.Error("capture blocked its writer before draining the pipe")
				if err := writer.Close(); err != nil {
					t.Errorf("close stalled capture writer: %v", err)
				}
			}
		}()
		written, writeErr = writer.WriteString(want)
		close(done)
		<-joined
	})
	if expired {
		t.Fatal("capture blocked its writer before draining the pipe")
	}
	if writeErr != nil {
		t.Fatalf("write capture: %v", writeErr)
	}
	if written != len(want) {
		t.Fatalf("wrote %d bytes, want %d", written, len(want))
	}
	if got != want {
		t.Fatalf("capture differs: received %d bytes, want %d", len(got), len(want))
	}
}

// TestCapturePanicRestoresStdout checks that unwinding closes the pipe and joins
// its reader, then leaves the original stdout usable by the next capture.
func TestCapturePanicRestoresStdout(t *testing.T) {
	original := os.Stdout
	var writer *os.File
	const sentinel = "BUG: capture test panic"
	func() {
		defer func() {
			if got := recover(); got != sentinel {
				t.Errorf("panic = %v, want %q", got, sentinel)
			}
		}()
		captureStdout(t, func() {
			writer = os.Stdout
			if _, err := os.Stdout.WriteString("before panic"); err != nil {
				t.Fatal(err)
			}
			panic(sentinel)
		})
	}()
	if os.Stdout != original {
		t.Fatal("capture did not restore stdout after panic")
	}
	if _, err := writer.WriteString("must be closed"); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("capture writer survived panic: %v", err)
	}
	if got := captureStdout(t, func() {
		if _, err := os.Stdout.WriteString("after panic"); err != nil {
			t.Fatal(err)
		}
	}); got != "after panic" {
		t.Fatalf("next capture = %q", got)
	}
}
