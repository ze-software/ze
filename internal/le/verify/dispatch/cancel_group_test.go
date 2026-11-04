// Design: docs/architecture/testing/runner-architecture.md -- owned stage process groups.
// Related: dispatch_test.go -- the registered probe and real child dispatcher.

package verifydispatch

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	leroot "github.com/ze-software/ze/internal/le/le/root"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// cancellationProbe starts a TERM-ignoring child in the stage's group. The child
// publishes its PID only after installing the trap, so cancellation has no race
// with handler startup. An escaped child deliberately retains the output pipe.
func cancellationProbe(args []string) (any, int) {
	ready := args[1]
	if args[0] == "cancel-slot" {
		return cancellationSlotProbe(ready)
	}
	words := []string{"sh", "-c", "trap '' TERM; echo $$ $PPID > \"$1\"; exec sleep 60", "sh", ready}
	if args[0] == "cancel-escaped" {
		words = append([]string{"setsid"}, words...)
	}
	child := exec.CommandContext(context.Background(), words[0], words[1:]...)
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	if err := child.Start(); err != nil {
		return probeText(err.Error()), 2
	}
	// Wait remains the sole direct-child reap on an ordinary completion. A
	// canceled parent stage is killed with its child by the dispatcher owner.
	if err := child.Wait(); err != nil {
		return probeText(err.Error()), 1
	}
	return nil, 0
}

// TestDispatchCancellationReapsInflightStage calls the real dispatch boundary
// after the registered stage's child announces readiness. It checks the direct
// Wait, group death, a bounded inherited pipe, and survival of foreign work.
func TestDispatchCancellationReapsInflightStage(t *testing.T) {
	for _, mode := range []string{"cancel-group", "cancel-escaped", "cancel-slot"} {
		t.Run(mode, func(t *testing.T) {
			if mode == "cancel-escaped" {
				if _, err := exec.LookPath("setsid"); err != nil {
					t.Skip("setsid is required to make an escaped pipe holder")
				}
			}
			root := t.TempDir()
			if mode == "cancel-slot" {
				initialize := exec.CommandContext(t.Context(), "git", "init", "-q", root)
				if output, err := initialize.CombinedOutput(); err != nil {
					t.Fatalf("initialize admission fixture: %v: %s", err, output)
				}
				// Admission fingerprints HEAD before it can occupy or wait for a
				// slot. Seed only this private fixture; an unborn repository is
				// correctly refused by the production fingerprint producer.
				seed := exec.CommandContext(t.Context(), "git", "-C", root,
					"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid",
					"-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false",
					"commit", "--allow-empty", "--quiet", "-m", "admission fixture")
				if output, err := seed.CombinedOutput(); err != nil {
					t.Fatalf("seed admission fixture HEAD: %v: %s", err, output)
				}
			}
			ready := filepath.Join(root, "ready")
			foreign := exec.CommandContext(t.Context(), "sleep", "60")
			if err := foreign.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() {
				foreign.Process.Kill() //nolint:errcheck // The fixture owns this sentinel.
				foreign.Wait() //nolint:errcheck // A killed sentinel has no successful exit.
			}()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			registerProbe()
			started := make(chan *exec.Cmd, 1)
			spawn := func(ctx context.Context, words []string) (*exec.Cmd, error) {
				cmd, err := testSpawn(ctx, words)
				if err == nil {
					started <- cmd
				}
				return cmd, err
			}
			answer := make(chan verifyengine.ActionResult, 1)
			go func() {
				answer <- dispatch(ctx, root, verifyengine.Identity{Name: mode, Command: probeRoot, Args: []string{mode, ready}}, leroot.Owns, leroot.LookupCommand, spawn)
			}()
			cmd := <-started
			joined := false
			defer func() {
				cancel()
				if !joined {
					<-answer
				}
			}()
			pid, stagePID := waitCancellationPID(t, ready, answer)
			defer syscall.Kill(pid, syscall.SIGKILL) //nolint:errcheck // Own leaked or deliberately escaped child only.
			began := time.Now()
			cancel()
			select {
			case result := <-answer:
				joined = true
				if result.Completed || result.Code != verifyengine.Interrupted || result.Failure == nil || result.Failure.Kind != "interrupted" {
					t.Errorf("cancellation result = %+v", result)
				}
				if cmd.ProcessState == nil {
					t.Error("dispatcher returned before its sole Wait completed")
				}
			case <-time.After(5 * time.Second):
				syscall.Kill(stagePID, syscall.SIGKILL) //nolint:errcheck // End only the stage named by its own child.
				syscall.Kill(pid, syscall.SIGKILL) //nolint:errcheck // End the owned pipe holder before joining.
				<-answer
				joined = true
				t.Fatal("dispatcher did not join cancellation within five seconds")
			}
			if time.Since(began) > 5*time.Second {
				t.Error("cancellation exceeded the five-second fixture bound")
			}
			if mode == "cancel-group" && !cancellationPIDGone(pid) {
				t.Errorf("TERM-ignoring child %d survived its stage", pid)
			}
			if err := foreign.Process.Signal(syscall.Signal(0)); err != nil {
				t.Errorf("foreign silent process was stopped: %v", err)
			}
			if mode == "cancel-slot" {
				if _, err := os.Stat(ready + ".ran"); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("queued work started before slot-wait cancellation: %v", err)
				}
			}
			later := runProbe(ctx, t, root, verifyengine.Identity{Name: "later", Command: probeRoot})
			if later.Completed || later.Failure == nil || later.Failure.Kind != "interrupted" {
				t.Errorf("a later stage started after cancellation: %+v", later)
			}
		})
	}
}

// TestDispatchCancellationBeforeStart refuses a registered stage without
// spawning anything when its caller's root has already been canceled.
func TestDispatchCancellationBeforeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	result := runProbe(ctx, t, t.TempDir(), verifyengine.Identity{Name: "before-start", Command: probeRoot})
	if result.Completed || result.Registered || result.Output != "" || result.Code != verifyengine.Interrupted || result.Failure == nil || result.Failure.Kind != "interrupted" {
		t.Fatalf("canceled root started a stage: %+v", result)
	}
}

func waitCancellationPID(t *testing.T, path string, answer chan verifyengine.ActionResult) (int, int) {
	t.Helper()
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		body, err := os.ReadFile(path)
		if err == nil {
			fields := strings.Fields(string(body))
			if len(fields) == 2 {
				pid, err := strconv.Atoi(fields[0])
				stagePID, stageErr := strconv.Atoi(fields[1])
				if err == nil && stageErr == nil && pid > 1 && stagePID > 1 {
					return pid, stagePID
				}
			}
		}
		select {
		case result := <-answer:
			answer <- result
			t.Fatalf("registered stage exited before child readiness: %+v", result)
		case <-deadline.C:
			t.Fatalf("registered stage did not announce child readiness; marker=%q read-error=%v", body, err)
		case <-tick.C:
		}
	}
}

func cancellationPIDGone(pid int) bool {
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for range 40 {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return true
		}
		<-tick.C
	}
	return false
}
