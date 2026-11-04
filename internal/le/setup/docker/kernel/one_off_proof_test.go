package dockerkernel

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

const installSignalFixture = "INSTALL_SIGNAL_FIXTURE"

// TestInstallSecondSignalEndsWaitingEntry drives the actual signal entry and
// runInstallStep with an owned subprocess that acknowledges but ignores TERM.
// No host preflight, sudo, kernel build, installation or reboot is performed.
func TestInstallSecondSignalEndsWaitingEntry(t *testing.T) {
	if mode := os.Getenv(installSignalFixture); mode != "" {
		runInstallSignalFixture(t, mode)
		return
	}
	for _, first := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(first.String(), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			self, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			command := exec.CommandContext(ctx, self, "-test.run=^TestInstallSecondSignalEndsWaitingEntry$")
			command.Env = append(os.Environ(), installSignalFixture+"=entry")
			command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			stderr, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer stderr.Close() //nolint:errcheck // The fixture's read-only pipe.
			defer writer.Close() //nolint:errcheck // Also closes the pipe when Start fails.
			command.Stderr = writer
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			events := make(chan string, 16)
			readDone := make(chan error, 1)
			go readInstallSignalEvents(stderr, events, readDone)
			waited := make(chan error, 1)
			go func() { waited <- command.Wait() }()
			reaped := false
			// The step remains alive after the entry's second signal. Stop only
			// this private fixture group, then join both owned workers.
			defer func() {
				if err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
					t.Error(err)
				}
				if !reaped {
					<-waited
				}
				if err := <-readDone; err != nil {
					t.Error(err)
				}
			}()
			awaitInstallSignalEvent(t, ctx, events, "INSTALL_STEP_READY")
			if err := command.Process.Signal(first); err != nil {
				t.Fatal(err)
			}
			awaitInstallSignalEvent(t, ctx, events, "INSTALL_STEP_TERM")
			if err := command.Process.Signal(syscall.SIGINT); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-waited:
				reaped = true
				var exit *exec.ExitError
				if !errors.As(err, &exit) {
					t.Fatalf("second interrupt did not terminate entry by signal: %v", err)
				}
				status, ok := exit.Sys().(syscall.WaitStatus)
				if !ok || !status.Signaled() || status.Signal() != syscall.SIGINT {
					t.Fatalf("second interrupt exit = %v, want SIGINT", exit)
				}
			case <-time.After(2 * time.Second):
				if err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL); err != nil {
					t.Error(err)
				}
				<-waited
				reaped = true
				t.Error("second interrupt was suppressed while the owned step ignored TERM")
			}
		})
	}
}

func runInstallSignalFixture(t *testing.T, mode string) {
	t.Helper()
	if mode == "step" {
		stopped := make(chan os.Signal, 1)
		signal.Notify(stopped, syscall.SIGTERM)
		defer signal.Stop(stopped)
		fmt.Fprintln(os.Stderr, "INSTALL_STEP_READY")
		<-stopped
		fmt.Fprintln(os.Stderr, "INSTALL_STEP_TERM")
		<-stopped
		t.Fatal("fixture unexpectedly received another TERM")
	}
	if mode != "entry" {
		t.Fatalf("unknown installer signal fixture mode %q", mode)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(installSignalFixture, "step")
	installKernel = func(ctx context.Context, _ leaction.Arguments) (any, int) {
		err := runInstallStep(ctx, installStep{Why: "run the isolated signal fixture", Argv: []string{
			self, "-test.run=^TestInstallSecondSignalEndsWaitingEntry$",
		}})
		t.Fatalf("TERM-ignoring step unexpectedly returned: %v", err)
		return nil, 1
	}
	runDockerKernelInstall(leaction.Arguments{})
	t.Fatal("installer signal entry unexpectedly returned")
}

func readInstallSignalEvents(stderr interface{ Read([]byte) (int, error) }, events chan<- string, done chan<- error) {
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "INSTALL_STEP_") {
			events <- line
		}
	}
	done <- scanner.Err()
}

func awaitInstallSignalEvent(t *testing.T, ctx context.Context, events <-chan string, want string) {
	t.Helper()
	select {
	case got := <-events:
		if got != want {
			t.Fatalf("installer signal event = %q, want %q", got, want)
		}
	case <-ctx.Done():
		t.Fatalf("waiting for %s: %v", want, ctx.Err())
	}
}
