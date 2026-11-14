//go:build integration && linux

// Design: docs/functional-tests.md -- joined privileged executable inspection.

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// This Go-test-only flag selects the root subprocess of the single test below;
// no daemon, le action or fixture command gains an argument.
var sysribExecutableRootReady = flag.String("sysrib-executable-root-ready", "", "private ready file for the stopped root inspection test child")

type sysribExecutableRootIdentity struct {
	PID   int `json:"pid"`
	UID   int `json:"uid"`
	Group int `json:"group"`
}

// A non-root fixture owns a real sudo child which becomes root and stops itself.
// Canceling the actual inspection runner MUST join and leave that same root
// identity dead. The pidfd cleanup MUST reclaim it even if sudo detached it and
// the assertion fails; numeric PID reuse cannot turn cleanup into another kill.
func TestSysribExecutablePrivilegedCancellationJoinsRootChild(t *testing.T) {
	if *sysribExecutableRootReady != "" {
		sysribExecutableStoppedRoot(t, *sysribExecutableRootReady)
		return
	}
	if os.Geteuid() == 0 {
		t.Skip("run as the non-root native guest user with the fixture CAP_KILL grant")
	}
	if err := sysribExecutableChildControl(); err != nil {
		t.Skipf("requires the native fixture capability grant: %v", err)
	}
	if _, err := exec.LookPath("sudo"); err != nil {
		t.Skipf("requires the native guest's passwordless sudo: %v", err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	readyPath := filepath.Join(t.TempDir(), "root-ready.json")
	ctx, cancel := context.WithTimeout(t.Context(), min(WaitBudget(10, 15*time.Second), 15*time.Second))
	type commandResult struct {
		output []byte
		err    error
	}
	done := make(chan commandResult, 1)
	joined := false
	pidfd := -1
	// ownedGroup is the process group this test proved it owns, zero until then.
	ownedGroup := 0
	t.Cleanup(func() {
		cancel()
		if pidfd >= 0 {
			exited, err := sysribExecutableRootExited(pidfd)
			if err != nil {
				t.Error(err)
			}
			if !exited {
				if err := unix.PidfdSendSignal(pidfd, unix.SIGKILL, nil, 0); err != nil {
					if !errors.Is(err, unix.ESRCH) {
						t.Errorf("reclaim held root child: %v", err)
					}
				}
			}
		}
		if !joined {
			if ownedGroup > 0 {
				// sudo suspends itself when its command stops, so killing the
				// root child alone leaves a stopped sudo that never exits and
				// the reaper below never returns. The group's leader is this
				// test's own unreaped descendant, so its number cannot be reused.
				if err := unix.Kill(-ownedGroup, unix.SIGKILL); err != nil {
					if !errors.Is(err, unix.ESRCH) {
						t.Errorf("reclaim owned process group %d: %v", ownedGroup, err)
					}
				}
			}
			// Failure cannot leave the request goroutine owning the process.
			// The controlled stopped child is killable through the pinned fd;
			// after reclaiming it, the actual sole reaper MUST finish its join.
			<-done
			joined = true
		}
		if pidfd >= 0 {
			if err := unix.Close(pidfd); err != nil {
				t.Error(err)
			}
		}
	})
	argv := []string{"sudo", "-n", self, "-test.run", "^TestSysribExecutablePrivilegedCancellationJoinsRootChild$", "-test.timeout", "1m", "-sysrib-executable-root-ready", readyPath}
	// This one test request owns the goroutine until actual runner completion;
	// Cleanup MUST cancel it, reclaim the pinned root identity and join done.
	go func() {
		output, err := runSysribExecutableCommand(ctx, argv)
		done <- commandResult{output: output, err: err}
	}()
	var root sysribExecutableRootIdentity
	var observation error
	if !Poll(ctx, WaitAttempts(10, 20*time.Millisecond, 750), 20*time.Millisecond, func() bool {
		select {
		case result := <-done:
			joined = true
			if result.err != nil {
				observation = fmt.Errorf("privileged command ended before its root barrier: %w; output=%q", result.err, result.output)
				return true
			}
			observation = fmt.Errorf("privileged command ended before its root barrier: <nil>; output=%q", result.output)
			return true
		default:
		}
		raw, err := os.ReadFile(readyPath)
		if errors.Is(err, os.ErrNotExist) {
			return false
		}
		if err != nil {
			observation = err
			return true
		}
		if err := json.Unmarshal(raw, &root); err != nil {
			observation = err
			return true
		}
		return true
	}) {
		t.Fatalf("root child did not reach its ready barrier: %v", ctx.Err())
	}
	if observation != nil {
		t.Fatal(observation)
	}
	if root.PID <= 0 {
		t.Fatalf("invalid root child PID: %+v", root)
	}
	if root.UID != 0 {
		t.Fatalf("sudo child never became root: %+v", root)
	}
	if root.Group <= 0 {
		t.Fatalf("invalid root child group: %+v", root)
	}
	pidfd, err = unix.PidfdOpen(root.PID, 0)
	if err != nil {
		t.Fatal(err)
	}
	parents, err := sysribExecutionParentCensus(ctx, "/proc")
	if err != nil {
		t.Fatal(err)
	}
	front := root.PID
	owned := false
	for range 16 {
		parent, exists := parents[front]
		if !exists {
			t.Fatalf("held root ancestry lost PID %d", front)
		}
		if parent == os.Getpid() {
			owned = true
			break
		}
		if parent < 2 {
			break
		}
		front = parent
	}
	if !owned {
		t.Fatalf("root child %d is not this test's descendant", root.PID)
	}
	if root.Group != front {
		t.Fatalf("sudo detached root PID %d into group %d, owned front PID is %d", root.PID, root.Group, front)
	}
	ownedGroup = root.Group
	group, err := unix.Getpgid(root.PID)
	if err != nil {
		t.Fatal(err)
	}
	if group != root.Group {
		t.Fatalf("root child changed process group: actual=%d ready=%d", group, root.Group)
	}
	if !Poll(ctx, WaitAttempts(10, 20*time.Millisecond, 750), 20*time.Millisecond, func() bool {
		state, answered, err := procProcessState(root.PID)
		if err != nil {
			observation = err
			return true
		}
		if !answered {
			observation = errors.New("native root proof requires procfs")
			return true
		}
		return state == "T"
	}) {
		t.Fatalf("root child never entered its controlled stop: %v", ctx.Err())
	}
	if observation != nil {
		t.Fatal(observation)
	}
	if err := ctx.Err(); err != nil {
		t.Fatalf("inspection expired before explicit cancellation: %v", err)
	}
	cancel()
	join, stop := context.WithTimeout(context.WithoutCancel(t.Context()), WaitBudget(10, 10*time.Second))
	defer stop()
	select {
	case result := <-done:
		joined = true
		if !errors.Is(result.err, context.Canceled) {
			t.Fatalf("canceled privileged inspection returned %v; output=%q", result.err, result.output)
		}
		if errors.Is(result.err, unix.EPERM) {
			t.Fatalf("owner lacked permission to stop its root child: %v", result.err)
		}
	case <-join.Done():
		t.Fatal("actual inspection runner did not join its stopped root child on cancellation")
	}
	exited, err := sysribExecutableRootExited(pidfd)
	if err != nil {
		t.Fatal(err)
	}
	if !exited {
		t.Fatal("inspection joined while its pinned root child remained alive")
	}
	t.Logf("actual privileged inspection canceled and joined stopped root PID %d in owned group %d", root.PID, root.Group)
}

func sysribExecutableStoppedRoot(t *testing.T, readyPath string) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Fatal("privileged inspection helper did not become root")
	}
	group, err := unix.Getpgid(0)
	if err != nil {
		t.Fatal(err)
	}
	identity := sysribExecutableRootIdentity{PID: os.Getpid(), UID: os.Geteuid(), Group: group}
	raw, err := json.Marshal(identity)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(readyPath+".pending", raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(readyPath+".pending", readyPath); err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintf(os.Stdout, "ROOTREADY pid=%d uid=%d group=%d\n", identity.PID, identity.UID, identity.Group); err != nil {
		t.Fatal(err)
	}
	if err := unix.Kill(identity.PID, unix.SIGSTOP); err != nil {
		t.Fatal(err)
	}
	t.Fatal("held root child unexpectedly resumed without being reclaimed")
}

func sysribExecutableRootExited(pidfd int) (bool, error) {
	fds := []unix.PollFd{{Fd: int32(pidfd), Events: unix.POLLIN}}
	if _, err := unix.Poll(fds, 0); err != nil {
		return false, err
	}
	if fds[0].Revents&unix.POLLNVAL != 0 {
		return false, errors.New("root child pidfd is invalid")
	}
	return fds[0].Revents&unix.POLLIN != 0, nil
}
