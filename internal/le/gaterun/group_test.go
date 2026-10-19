package gaterun

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// leaderExitsFirst runs a child that heads its own group, leaves a member
// sleeping for sleep, and exits at once, answering the group and the member's
// pid: a group that outlives its reaped leader, as a SIGKILLed `go test`
// group does while its test binaries are still exiting.
func leaderExitsFirst(t *testing.T, sleep string) (ProcessGroup, int) {
	t.Helper()
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "member")
	code, group := StreamContext(t.Context(),
		[]string{"sh", "-c", "sleep " + sleep + " & echo $! > " + pidFile}, dir, os.Environ())
	if code != 0 {
		t.Fatalf("leader answered %d, want 0", code)
	}
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("member pid: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("member pid %q: %v", raw, err)
	}
	return group, pid
}

// VALIDATES: StreamContext answers the group its child headed, and AwaitExit
// returns only once every member is gone, not when the leader is reaped.
// METHOD: a leader that exits at once leaves a member sleeping 500 ms; the
// group must still be present on return from StreamContext, and absent
// (kill(-group, 0) answers ESRCH) once AwaitExit returns nil.
// PREVENTS: the post-signal lab sweep judging a still-exiting test binary
// alive and skipping its containers.
func TestAwaitExitWaitsForEveryMember(t *testing.T) {
	group, member := leaderExitsFirst(t, "0.5")
	leader, started := group.Leader()
	if !started {
		t.Fatal("StreamContext answered no group for a child that ran")
	}
	if !groupPresent(leader) {
		t.Fatalf("group %d already empty: the member %d did not outlive the leader", leader, member)
	}
	if err := group.AwaitExit(t.Context(), 10*time.Second); err != nil {
		t.Fatalf("AwaitExit: %v", err)
	}
	if err := syscall.Kill(-leader, 0); !errors.Is(err, syscall.ESRCH) {
		t.Errorf("kill(-%d, 0) = %v after AwaitExit, want ESRCH", leader, err)
	}
}

// VALIDATES: AwaitExit stops at its bound and names the pids still present.
func TestAwaitExitNamesMembersLeftAtTheBound(t *testing.T) {
	group, member := leaderExitsFirst(t, "5")
	t.Cleanup(func() {
		if err := syscall.Kill(member, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			t.Errorf("kill member %d: %v", member, err)
		}
	})
	began := time.Now()
	err := group.AwaitExit(t.Context(), 200*time.Millisecond)
	if elapsed := time.Since(began); elapsed > 3*time.Second {
		t.Errorf("AwaitExit came back after %v, want the 200ms bound", elapsed)
	}
	if err == nil || !strings.Contains(err.Error(), strconv.Itoa(member)) {
		t.Errorf("AwaitExit = %v, want an error naming member %d", err, member)
	}
}

// VALIDATES: a child that never started heads no group, and AwaitExit on it
// answers nil at once rather than probing kill(0, 0), this process's own group.
func TestAwaitExitWithoutGroup(t *testing.T) {
	code, group := StreamContext(t.Context(), []string{filepath.Join(t.TempDir(), "absent")}, t.TempDir(), os.Environ())
	if code != CannotStart {
		t.Fatalf("code %d, want CannotStart", code)
	}
	if _, started := group.Leader(); started {
		t.Error("a child that never started answered a group")
	}
	if err := group.AwaitExit(t.Context(), time.Second); err != nil {
		t.Errorf("AwaitExit = %v, want nil for no group", err)
	}
}
