// Design: docs/architecture/testing/runner-architecture.md -- graceful owned server cleanup.
package runner

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestServerProcessJourneyStop drives real normal and TERM-resistant direct
// children through Stop, then observes the sole reap and cached cleanup verdict.
// The resistant child outlives the grace unless Stop actually escalates.
func TestServerProcessJourneyStop(t *testing.T) {
	for _, mode := range []string{"normal", "resistant", "signal-error"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			ready := filepath.Join(t.TempDir(), "ready")
			script := `printf ready > "$1"; exec sleep 60`
			if mode == "resistant" {
				script = `trap '' TERM; printf ready > "$1"; exec sleep 60`
			}
			cmd := exec.CommandContext(ctx, "/bin/sh", "-c", script, "owned-stop", ready)
			cmd.WaitDelay = 2 * time.Second
			server, err := StartServerProcess(cmd)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = server.Close() }) //nolint:errcheck // Exact cached verdicts are asserted below; cleanup also covers an earlier failure.
			if cmd.WaitDelay != 2*time.Second {
				t.Errorf("owner replaced caller pipe-drain bound: %s", cmd.WaitDelay)
			}
			for {
				if _, err := os.Stat(ready); err == nil {
					break
				} else if !os.IsNotExist(err) {
					t.Fatal(err)
				}
				select {
				case <-ctx.Done():
					t.Fatal("owned child did not rendezvous")
				case <-time.After(10 * time.Millisecond):
				}
			}
			if mode == "signal-error" {
				server.process.signalMu.Lock()
				signal := server.process.groupSignal
				server.process.groupSignal = func(pid int, sig syscall.Signal) error {
					if err := signal(pid, sig); err != nil {
						return err
					}
					if sig == syscall.SIGTERM {
						return syscall.EPERM
					}
					return nil
				}
				server.process.signalMu.Unlock()
			}
			started := time.Now()
			err = server.Stop(syscall.SIGTERM, TeardownGraceTimeout, 0)
			if time.Since(started) > TeardownGraceTimeout+2*time.Second {
				t.Error("Stop did not enforce its separate cleanup grace")
			}
			if mode == "signal-error" {
				if !errors.Is(err, syscall.EPERM) || !errors.Is(server.Close(), syscall.EPERM) {
					t.Errorf("cleanup lost its signal error or cached result: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			select {
			case <-server.Done():
			default:
				t.Fatal("Stop returned before Done closed")
			}
			if cmd.ProcessState == nil {
				t.Fatal("Stop returned without its sole Wait result")
			}
			if mode == "resistant" {
				status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
				if !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
					t.Fatalf("TERM-resistant child was not killed: %v", cmd.ProcessState)
				}
			}
			var status syscall.WaitStatus
			if pid, err := syscall.Wait4(cmd.Process.Pid, &status, syscall.WNOHANG, nil); !errors.Is(err, syscall.ECHILD) {
				t.Errorf("Stop left an unjoined child: pid=%d error=%v", pid, err)
			}
			first, second := server.Wait(), server.Wait()
			if first != second && (first == nil || second == nil || first.Error() != second.Error()) {
				t.Errorf("Wait observers saw different cached exits: %v / %v", first, second)
			}
		})
	}
}
