// Design: docs/architecture/testing/qemu-integration.md -- actual ISO poweroff.

package testqemu

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// VALIDATES: completion text from a guest that is still running is not
// poweroff: the capture answers its deadline error, and its output holds the
// completion marker the guest printed before that deadline.
// METHOD: the fake guest prints InstallMarkISODone and then sleeps. The
// deadline reaches the capture only after the parent has read the marker from
// the guest's output, through a parent context that ends with
// context.DeadlineExceeded, so "marker printed, then deadline, guest still
// running" holds at any host load. The capture's own timeout is a guard far
// above it: it fires only when the guest never prints, and the marker
// assertion then names that.
// PREVENTS: a capture that answers success once the guest prints the
// completion marker without powering off.
func TestISOCaptureWaitsForActualPoweroff(t *testing.T) {
	deadline := newGuestDeadline()
	installer := installCaptureProcess(t, "hang")
	start := installer.ops.Start
	installer.ops.Start = func(command *exec.Cmd) error {
		watch := &markerWatch{target: command.Stdout, marker: InstallMarkISODone, seen: deadline.expire}
		// One writer for both streams keeps exec on one shared pipe, as the
		// capture's own serial writer does.
		command.Stdout, command.Stderr = watch, watch
		return start(command)
	}
	output, err := installer.runCaptureUntilExit(deadline, []string{"fixture-qemu"}, time.Minute)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("completion text from a still-running guest counted as poweroff: %v", err)
	}
	if !strings.Contains(output, InstallMarkISODone) {
		t.Fatalf("guest did not reach the completed-write state before its deadline: %q", output)
	}
}

// VALIDATES: the capture's own timeout ends a guest that never powers off,
// with the deadline error.
// METHOD: a hung guest and a 250 ms timeout. Only the error is asserted, so
// the verdict holds whether or not the guest printed anything by then, and
// whether the deadline met it before or after it started.
// PREVENTS: a capture whose timeout argument no longer bounds the guest.
func TestISOCaptureTimeoutStopsAHungGuest(t *testing.T) {
	installer := installCaptureProcess(t, "hang")
	_, err := installer.runCaptureUntilExit(context.Background(), []string{"fixture-qemu"}, 250*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a hung guest outlived the capture's timeout: %v", err)
	}
}

// guestDeadline is a parent context whose deadline the test delivers when it
// chooses, with the context.DeadlineExceeded a timer would give. A context
// derived from it with context.WithTimeout ends with that same error. Safe for
// concurrent use: expire may be called from any goroutine, more than once.
type guestDeadline struct {
	context.Context
	done chan struct{}
	once sync.Once
}

func newGuestDeadline() *guestDeadline {
	return &guestDeadline{Context: context.Background(), done: make(chan struct{})}
}

func (deadline *guestDeadline) expire() { deadline.once.Do(func() { close(deadline.done) }) }

func (deadline *guestDeadline) Done() <-chan struct{} { return deadline.done }

func (deadline *guestDeadline) Err() error {
	select {
	case <-deadline.done:
		return context.DeadlineExceeded
	default:
		return nil
	}
}

// markerWatch forwards the guest's output to the capture's own writer, then
// calls seen once the marker has passed through, so the marker is in the
// capture's output before seen runs. Safe for concurrent use.
type markerWatch struct {
	target io.Writer
	marker string
	seen   func()
	mu     sync.Mutex
	text   strings.Builder
}

func (watch *markerWatch) Write(chunk []byte) (int, error) {
	count, err := watch.target.Write(chunk)
	watch.mu.Lock()
	_, _ = watch.text.Write(chunk[:count]) // strings.Builder.Write never fails.
	found := strings.Contains(watch.text.String(), watch.marker)
	watch.mu.Unlock()
	if found {
		watch.seen()
	}
	return count, err
}

func TestISOCaptureRejectsGuestFailureAfterCompletionText(t *testing.T) {
	installer := installCaptureProcess(t, "fail")
	output, err := installer.runCaptureUntilExit(context.Background(), []string{"fixture-qemu"}, 5*time.Second)
	if err == nil {
		t.Fatal("guest failure after completion text counted as safe poweroff")
	}
	if !strings.Contains(output, InstallMarkISODone) {
		t.Fatalf("failure lost the guest's completed-write diagnostic: %q", output)
	}
}

func TestISOCaptureAcceptsCleanPoweroff(t *testing.T) {
	installer := installCaptureProcess(t, "success")
	output, err := installer.runCaptureUntilExit(context.Background(), []string{"fixture-qemu"}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, InstallMarkISODone) {
		t.Fatalf("clean guest output lost: %q", output)
	}
}

func installCaptureProcess(t *testing.T, mode string) *Installer {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	installer := installTestInstaller(t, InstallKindISO)
	installer.ops.Start = func(command *exec.Cmd) error {
		command.Path = executable
		command.Args = []string{executable, "-test.run=^TestISOCaptureGuestProcess$"}
		command.Env = append(os.Environ(), "ZE_ISO_CAPTURE_TEST="+mode)
		// Replace the lookup result as well as the path: CommandContext
		// already tried to resolve the deliberately fake QEMU name.
		command.Err = nil
		return command.Start()
	}
	return installer
}

func TestISOCaptureGuestProcess(t *testing.T) {
	mode := os.Getenv("ZE_ISO_CAPTURE_TEST")
	if mode == "" {
		return
	}
	if _, err := fmt.Fprintln(os.Stdout, InstallMarkISODone); err != nil {
		t.Fatal(err)
	}
	switch mode {
	case "success":
		return
	case "fail":
		os.Exit(7)
	case "hang":
		// This guest never exits within the proof's deadline. The parent
		// delivers that deadline only after it read the marker above, and
		// asserts the deadline error, not a duration or scheduling gap.
		time.Sleep(time.Hour)
	default:
		t.Fatalf("unknown fixture mode %q", mode)
	}
}
