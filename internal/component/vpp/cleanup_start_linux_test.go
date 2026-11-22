//go:build integration && linux

// Design: docs/research/vpp-deployment-reference.md -- original managed child cleanup
// Related: process_managed_linux.go -- actual failed-start kill and Wait outcomes
package vpp

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// The stdin handshake keeps this exact child alive while Start captures its
// owned pidfd. Its small output fits in the stdout pipe while the parent's
// copy writer waits for observed child exit, so a writer refusal cannot turn
// a clean helper exit into SIGPIPE from the test binary's final PASS output.
func TestVPPManagedStartCleanupWriterChild(t *testing.T) {
	if os.Args[len(os.Args)-1] != "vpp-cleanup-writer-child" {
		return
	}
	var start [1]byte
	if _, err := io.ReadFull(os.Stdin, start[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stdout.WriteString("READY\n"); err != nil {
		t.Fatal(err)
	}
}

// This is an actual exec.Cmd copy writer. Only its return is gated; it never
// fabricates a Wait result, a ProcessState or a native process descriptor.
type cleanupStartFailWriter struct {
	seen    chan string
	release chan struct{}
	once    sync.Once
	cause   error
}

func (w *cleanupStartFailWriter) Write(value []byte) (int, error) {
	w.seen <- string(value)
	<-w.release
	return 0, w.cause
}

func (w *cleanupStartFailWriter) unblock() { w.once.Do(func() { close(w.release) }) }

// Keep the same actual child, original pidfd and gated copy-writer setup for
// failed-start reap and the normal already-exited Wait select arm.
func newCleanupStartCopyFixture(t *testing.T, copyCause error) (*exec.Cmd, *cleanupStartFailWriter) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	lifetime, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)
	writer := &cleanupStartFailWriter{seen: make(chan string, 1), release: make(chan struct{}), cause: copyCause}
	command := exec.CommandContext(lifetime, executable, "-test.run=^TestVPPManagedStartCleanupWriterChild$", "--", "vpp-cleanup-writer-child")
	command.Stdout = writer
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	witness, err := startManagedProcess(command)
	if err != nil {
		if closeErr := closeManagedFixturePipe(input); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// Release the copy goroutine before joining, including assertion failures.
		writer.unblock()
		inputErr := closeManagedFixturePipe(input)
		var cleanup error
		if command.ProcessState == nil {
			killErr := command.Process.Kill()
			if errors.Is(killErr, os.ErrProcessDone) {
				killErr = nil
			}
			waitErr := command.Wait()
			if exit, ok := errors.AsType[*exec.ExitError](waitErr); ok {
				if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() && status.Signal() == syscall.SIGKILL {
					waitErr = nil // this teardown deliberately killed its own child
				}
			}
			cleanup = errors.Join(killErr, waitErr)
		}
		// The original witness is retained through the actual Wait, even if
		// that Wait returns the intentionally injected writer failure.
		if err := errors.Join(inputErr, cleanup, witness.release()); err != nil {
			t.Error(err)
		}
	})
	if _, err := input.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	// This unbound witness has no lifecycle worker or second owner. Keep its
	// mutex through Poll to honor descriptor-use serialization nevertheless.
	witness.mu.Lock()
	descriptor, ok := witness.descriptor.(*linuxProcessDescriptor)
	if !ok || witness.references != 1 || witness.generation != 0 {
		witness.mu.Unlock()
		t.Fatal("managed Start did not retain its original unbound Linux descriptor")
	}
	poll := [1]unix.PollFd{{Fd: int32(descriptor.fd), Events: unix.POLLIN}}
	_, pollErr := unix.Poll(poll[:], 5000)
	witness.mu.Unlock()
	if pollErr != nil {
		t.Fatal(pollErr)
	}
	if poll[0].Revents&(unix.POLLNVAL|unix.POLLERR) != 0 || poll[0].Revents&unix.POLLIN == 0 {
		t.Fatalf("original child did not confirm actual exit before Wait: events=%#x", poll[0].Revents)
	}
	writer.unblock()
	return command, writer
}

// The real Wait returns the stdout copy error alongside a successful native
// exit. Startup's primary cause and the separately typed cleanup cause MUST
// both survive; an untyped errors.Join is insufficient for the outer run loop.
func TestVPPManagedStartCleanupRetainsActualWaitCopyFailure(t *testing.T) {
	copyCause := errors.New("original managed stdout writer refused output")
	command, writer := newCleanupStartCopyFixture(t, copyCause)
	primary := errors.New("original managed startup refused")
	err := reapManagedStart(command, primary)
	if command.ProcessState == nil || !command.ProcessState.Success() {
		t.Fatalf("writer fixture did not retain its actual clean child exit: %v", command.ProcessState)
	}
	select {
	case output := <-writer.seen:
		if !strings.Contains(output, "READY\n") {
			t.Fatalf("actual writer did not receive its child's readiness: %q", output)
		}
	default:
		t.Fatal("actual stdout copy writer was not reached")
	}
	if !errors.Is(err, primary) || !errors.Is(err, copyCause) {
		t.Fatalf("failed-start cleanup lost the original primary or actual Wait copy cause: %v", err)
	}
	var cleanup nativeCleanupError
	if !errors.As(err, &cleanup) || !errors.Is(cleanup, copyCause) {
		t.Fatalf("actual Wait copy failure lost its native cleanup classification: %v", err)
	}
}

// The already-exited Wait arm has no Kill or second Wait. Its actual copy
// failure still requires the same classification that prevents child restart.
func TestVPPManagedWaitCleanupRetainsActualCopyFailure(t *testing.T) {
	copyCause := errors.New("original running managed stdout writer refused output")
	command, writer := newCleanupStartCopyFixture(t, copyCause)
	err := waitVPPConnection(t.Context(), command, make(chan error))
	if command.ProcessState == nil || !command.ProcessState.Success() {
		t.Fatalf("Wait-arm fixture did not retain its actual clean child exit: %v", command.ProcessState)
	}
	select {
	case output := <-writer.seen:
		if !strings.Contains(output, "READY\n") {
			t.Fatalf("actual Wait-arm writer did not receive its child's readiness: %q", output)
		}
	default:
		t.Fatal("actual Wait-arm stdout copy writer was not reached")
	}
	if !errors.Is(err, copyCause) {
		t.Fatalf("already-exited Wait arm lost the actual copy failure: %v", err)
	}
	var cleanup nativeCleanupError
	if !errors.As(err, &cleanup) || !errors.Is(cleanup, copyCause) {
		t.Fatalf("already-exited Wait copy failure lost its cleanup classification: %v", err)
	}
}

// Deliberately killing and reaping this exact owned child is successful
// cleanup. Its actual SIGKILL status MUST NOT acquire cleanup-failure meaning.
func TestVPPManagedStartCleanupNormalizesOnlyOwnedKill(t *testing.T) {
	child := newCleanupOutcomeChild(t)
	primary := errors.New("original managed startup refused")
	err := reapManagedStart(child, primary)
	if child.ProcessState == nil {
		t.Fatal("failed-start cleanup returned before reaping its exact owned child")
	}
	status, ok := child.ProcessState.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
		t.Fatalf("positive cleanup did not kill and reap its own actual child: %v", child.ProcessState)
	}
	if !errors.Is(err, primary) {
		t.Fatalf("healthy cleanup lost the primary startup refusal: %v", err)
	}
	if _, ok := errors.AsType[*exec.ExitError](err); ok {
		t.Fatalf("healthy owned kill retained its expected exit error: %v", err)
	}
	if _, ok := errors.AsType[nativeCleanupError](err); ok {
		t.Fatalf("healthy owned kill became a native cleanup failure: %v", err)
	}
}
