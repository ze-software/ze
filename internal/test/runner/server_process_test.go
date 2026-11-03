// Design: docs/architecture/testing/runner-architecture.md -- server ownership.
package runner

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// TestServerProcessCloseReaps verifies normal stop and canceled startup share
// one Wait result, and cannot publish cleanup while the command remains unreaped.
func TestServerProcessCloseReaps(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", "exec sleep 60")
	server, err := StartServerProcess(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	}()
	cancel()
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	if cmd.ProcessState == nil {
		t.Fatal("cleanup returned before the sole Wait completed")
	}
	if err := server.Close(); err != nil {
		t.Fatalf("cached cleanup result changed: %v", err)
	}
}

// TestServerProcessSignalOutcomes keeps a real child and sole reaper while
// injecting the signal syscall verdict after actually stopping the owned group.
// It distinguishes an absent group from a genuine signal failure.
func TestServerProcessSignalOutcomes(t *testing.T) {
	for _, verdict := range []error{syscall.ESRCH, syscall.EPERM} {
		cmd := exec.CommandContext(context.Background(), "/bin/sh", "-c", "exec sleep 60")
		server, err := StartServerProcess(cmd)
		if err != nil {
			t.Fatal(err)
		}
		server.process.signalMu.Lock()
		signal := server.process.groupSignal
		server.process.groupSignal = func(pid int, sig syscall.Signal) error {
			if err := signal(pid, sig); err != nil {
				return err
			}
			return verdict
		}
		server.process.signalMu.Unlock()
		err = server.Close()
		if errors.Is(verdict, syscall.ESRCH) {
			if err != nil {
				t.Fatal(err)
			}
		} else if !errors.Is(err, verdict) {
			t.Fatalf("signal failure lost: %v", err)
		}
		if cmd.ProcessState == nil {
			t.Fatal("signal verdict bypassed owned join")
		}
	}
}

// TestServerProcessNaturalExitSignalOutcomes releases a real child through stdin
// only after installing the final group-cleanup verdict. Close must retain a
// genuine retirement failure after the sole reap, but an absent group is benign.
func TestServerProcessNaturalExitSignalOutcomes(t *testing.T) {
	for _, test := range []struct {
		name    string
		verdict error
		wantErr error
	}{
		{name: "permission-denied", verdict: syscall.EPERM, wantErr: syscall.EPERM},
		{name: "absent-group", verdict: syscall.ESRCH},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "/bin/sh", "-c", "IFS= read -r release; exit 0")
			release, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				err := release.Close()
				if errors.Is(err, os.ErrClosed) {
					return
				}
				if err != nil {
					t.Error(err)
				}
			})
			server, err := StartServerProcess(cmd)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				// The exact first and repeated Close verdicts are asserted below.
				_ = server.Close() //nolint:errcheck // Cleanup must also join after an earlier assertion aborts.
			})
			var signals atomic.Int32
			server.process.signalMu.Lock()
			signal := server.process.groupSignal
			server.process.groupSignal = func(pid int, sig syscall.Signal) error {
				signals.Add(1)
				if err := signal(pid, sig); err != nil {
					if !errors.Is(err, syscall.ESRCH) {
						return err
					}
				}
				return test.verdict
			}
			server.process.signalMu.Unlock()
			if _, err := io.WriteString(release, "exit\n"); err != nil {
				t.Fatal(err)
			}
			select {
			case <-server.process.done:
			case <-ctx.Done():
				t.Fatal("natural child exit was not reaped before the test deadline")
			}
			if cmd.ProcessState == nil {
				t.Fatal("owner completed without reaping the child")
			}
			if !cmd.ProcessState.Success() {
				t.Fatalf("child did not exit naturally with success: %v", cmd.ProcessState)
			}
			var status syscall.WaitStatus
			if pid, err := syscall.Wait4(cmd.Process.Pid, &status, syscall.WNOHANG, nil); !errors.Is(err, syscall.ECHILD) {
				t.Errorf("sole owner left an unreaped child: wait4 = %d, %v", pid, err)
			}
			for attempt := range 2 {
				err := server.Close()
				if test.wantErr != nil {
					if !errors.Is(err, test.wantErr) {
						t.Errorf("Close attempt %d lost retirement failure: %v", attempt+1, err)
					}
					continue
				}
				if err != nil {
					t.Errorf("Close attempt %d rejected an absent group: %v", attempt+1, err)
				}
			}
			if got := signals.Load(); got != 1 {
				t.Errorf("numeric signaling continued after retirement: got %d calls, want 1", got)
			}
		})
	}
}

// TestServerProcessReportsUnexpectedExit waits for the owner's observed exit,
// then proves cleanup cannot turn a failed server into a passing case.
func TestServerProcessReportsUnexpectedExit(t *testing.T) {
	cmd := exec.CommandContext(context.Background(), "/bin/sh", "-c", "exit 13")
	server, err := StartServerProcess(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := server.Close(); err == nil {
			t.Error("repeated cleanup lost unexpected server exit")
		}
	}()
	<-server.process.done
	err = server.Close()
	var exited *exec.ExitError
	if !errors.As(err, &exited) || exited.ExitCode() != 13 {
		t.Fatalf("cleanup lost unexpected server exit: %v", err)
	}
	if !strings.Contains(err.Error(), "cleanup failed") {
		t.Fatalf("missing cleanup diagnostic: %v", err)
	}
}

// TestServerProcessStartFailure gives an unavailable executable to the real
// persistent spawner; no owner may be published for an unstarted command.
func TestServerProcessStartFailure(t *testing.T) {
	cmd := exec.CommandContext(context.Background(), "/no-such-owned-server")
	server, err := StartServerProcess(cmd)
	if err == nil || server != nil {
		t.Fatalf("failed start published an owner: %v %v", server, err)
	}
}
