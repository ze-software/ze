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
	leader, started := group.LeaderForTest()
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
	if _, started := group.LeaderForTest(); started {
		t.Error("a child that never started answered a group")
	}
	if err := group.AwaitExit(t.Context(), time.Second); err != nil {
		t.Errorf("AwaitExit = %v, want nil for no group", err)
	}
}

// VALIDATES: a group whose last member exits between the final poll and the
// ps listing answers nil, the group being empty, rather than a "still holds
// pids" warning that names no pid still running.
// METHOD: a fake ps on PATH prints the member's line, then SIGKILLs it and
// waits (at most 3 s) for the group to empty, which stages the race: the
// listing saw a member the group no longer holds. The bound is 0, so
// AwaitExit lists at once.
// PREVENTS: the docker-peers post-signal summary warning about killed tests
// that had in fact all exited.
func TestAwaitExitGroupEmptiedDuringTheListing(t *testing.T) {
	group, member := leaderExitsFirst(t, "5")
	t.Cleanup(func() {
		if err := syscall.Kill(member, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			t.Errorf("kill member %d: %v", member, err)
		}
	})
	leader, _ := group.LeaderForTest()
	bin := t.TempDir()
	script := "#!/bin/sh\n" +
		"echo \"" + strconv.Itoa(member) + " " + strconv.Itoa(leader) + "\"\n" +
		"kill -KILL " + strconv.Itoa(member) + "\n" +
		"tries=0\n" +
		"while kill -0 -" + strconv.Itoa(leader) + " 2>/dev/null; do\n" +
		"  tries=$((tries + 1)); [ $tries -gt 300 ] && exit 0; sleep 0.01\n" +
		"done\n"
	if err := os.WriteFile(filepath.Join(bin, "ps"), []byte(script), 0o755); err != nil {
		t.Fatalf("fake ps: %v", err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	left := group.AwaitExit(t.Context(), 0)
	// The staging is checked first: a member left as a zombie (a subreaper
	// that has not reaped it) keeps the group present, and the nil assertion
	// would then fail for a reason it does not name.
	if err := syscall.Kill(-leader, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("kill(-%d, 0) = %v, want ESRCH: the fake ps did not empty the group, so the race was not staged",
			leader, err)
	}
	if left != nil {
		t.Errorf("AwaitExit = %v, want nil: the group emptied during the listing", left)
	}
}

// VALIDATES: a group still present whose members ps does not list answers the
// "members that ps did not list" error, naming the group and the bound,
// rather than nil or an empty "pids" list.
// METHOD: a fake ps on PATH prints nothing and succeeds while the member
// sleeps, so the group stays present across the listing. The bound is 0, so
// AwaitExit lists at once.
// PREVENTS: a member hidden from ps (another user's, or one that joined after
// the listing) being reported as no member at all.
func TestAwaitExitGroupPsListsNoMember(t *testing.T) {
	group, member := leaderExitsFirst(t, "5")
	t.Cleanup(func() {
		if err := syscall.Kill(member, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			t.Errorf("kill member %d: %v", member, err)
		}
	})
	leader, _ := group.LeaderForTest()
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "ps"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("fake ps: %v", err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	left := group.AwaitExit(t.Context(), 0)
	if !groupPresent(leader) {
		t.Fatalf("group %d emptied during the test: the member %d did not stay, so the branch was not reached",
			leader, member)
	}
	want := "process group " + strconv.Itoa(leader) + " still holds members that ps did not list" +
		" (one that joined after the listing, or one hidden from this user) after 0s"
	if left == nil || left.Error() != want {
		t.Errorf("AwaitExit = %v, want %q", left, want)
	}
}
