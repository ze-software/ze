// Related: dockerpeers.go -- the docker-peers and live-rpki children this test signals

package testintegration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// sleeperArgv is a child holding a grandchild, both sleeping 30 s, so a stop
// that reaches only the direct child still leaves the run waiting.
var sleeperArgv = []string{"sh", "-c", "sleep 30 & sleep 30"}

// VALIDATES: AC-8 and AC-22 for docker-peers and live-rpki: SIGINT and SIGTERM
// reach the `go test` child through the signal-cancelled root, so le kills the
// child's process group and answers non-zero at once, rather than waiting out
// the child's own 900 s bound.
// METHOD: the verb's runner drives a child that would sleep for 30 s; the test
// signals itself 300 ms in and requires the runner back within 10 s, non-zero.
// G-2: the SIGKILL of the child's group means labtest's t.Cleanup never
// runs, so after the signalled exit the verb runs one non-dry orphan sweep,
// on a context the signal did not cancel, in the same invocation.
// PREVENTS: the root's NotifyContext swallowing the signal while the child,
// run on context.Background, runs on; and a killed run's containers waiting
// for the next sweep.
func TestDockerChildStopsOnSignal(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("checkout root: %v", err)
	}
	for _, delivered := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		cleaner := &fakeCleaner{}
		run := nativeRunner(root, func(ctx context.Context, root string) (any, int) {
			timer := time.AfterFunc(300*time.Millisecond, func() {
				if err := syscall.Kill(os.Getpid(), delivered); err != nil {
					t.Errorf("signal self: %v", err)
				}
			})
			defer timer.Stop()
			return runRequiringDocker(ctx, dockerPeersVerb, sleeperArgv, root, cleaner)
		})
		began := time.Now()
		_, code := run()
		if elapsed := time.Since(began); elapsed > 10*time.Second {
			t.Errorf("%v: the runner came back after %v, want the child stopped on the signal", delivered, elapsed)
		}
		if code == 0 {
			t.Errorf("%v: code 0, want a non-zero answer for a signalled run", delivered)
		}
		if !slices.Equal(cleaner.dry, []bool{false}) {
			t.Errorf("%v: CleanupOrphans calls = %v, want one non-dry sweep after the signal", delivered, cleaner.dry)
		}
		if slices.Contains(cleaner.canceled, true) {
			t.Errorf("%v: the sweep ran on a context the signal had canceled", delivered)
		}
	}
}

// VALIDATES: a docker-peers child that ends on its own runs no sweep: the
// sweep is for the signal path only, where t.Cleanup was skipped.
func TestDockerChildExitRunsNoSweep(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("checkout root: %v", err)
	}
	cleaner := &fakeCleaner{}
	if _, code := runRequiringDocker(t.Context(), dockerPeersVerb, []string{"true"}, root, cleaner); code != 0 {
		t.Fatalf("code %d, want 0 for a child that exits 0", code)
	}
	if len(cleaner.dry) != 0 {
		t.Errorf("CleanupOrphans calls = %v, want none for an unsignalled exit", cleaner.dry)
	}
}

// VALIDATES: the post-signal sweep reports its summary, and a sweep the
// daemon refuses says so with the fix rather than passing in silence.
func TestSignalSweepReports(t *testing.T) {
	removed := &fakeCleaner{report: interoplab.OrphanReport{
		Removed: []interoplab.OrphanResource{{Name: "lab-peer"}},
	}}
	if text := sweepAfterSignal(t.Context(), dockerPeersVerb, removed, gaterun.ProcessGroup{}, groupExitTimeout); !strings.Contains(text, "1 removed") {
		t.Errorf("summary = %q, want the report's removed count", text)
	}
	refused := &fakeCleaner{err: errors.New("daemon gone")}
	text := sweepAfterSignal(t.Context(), dockerPeersVerb, refused, gaterun.ProcessGroup{}, groupExitTimeout)
	for _, want := range []string{"daemon gone", "./le test integration cleanup"} {
		if !strings.Contains(text, want) {
			t.Errorf("refusal %q lacks %q", text, want)
		}
	}
}

// groupOutlivingLeader runs, through the verb's own runner, a child that
// leaves a member sleeping for sleep and exits at once, answering its group
// and the member's pid: the shape a SIGKILLed `go test` group has while its
// test binaries are still exiting after le reaped `go test`.
func groupOutlivingLeader(t *testing.T, sleep string) (gaterun.ProcessGroup, int) {
	t.Helper()
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "member")
	_, code, group := gaterun.RunContext(t.Context(), dockerPeersVerb,
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

// VALIDATES: the post-signal sweep runs only once the killed child's process
// group is empty, not as soon as the leader is reaped.
// METHOD: a real group whose member outlives its reaped leader by 500 ms; the
// fake cleaner records whether the group still had a member when it swept.
// PREVENTS: the sweep judging a still-exiting test binary a live owner and
// leaving its containers.
func TestSignalSweepWaitsForTheGroup(t *testing.T) {
	group, member := groupOutlivingLeader(t, "0.5")
	leader, started := group.Leader()
	if !started {
		t.Fatal("RunContext answered no group for a child that ran")
	}
	cleaner := &fakeCleaner{group: leader}
	text := sweepAfterSignal(t.Context(), dockerPeersVerb, cleaner, group, groupExitTimeout)
	if !slices.Equal(cleaner.groupLive, []bool{false}) {
		t.Errorf("group live at sweep = %v, want one sweep after member %d exited", cleaner.groupLive, member)
	}
	if strings.Contains(text, "had not all exited") {
		t.Errorf("summary %q reports members left for a group that emptied", text)
	}
}

// VALIDATES: a group still holding members at its bound is swept anyway, and
// the summary names the pid left and the fix.
func TestSignalSweepNamesMembersLeft(t *testing.T) {
	group, member := groupOutlivingLeader(t, "5")
	t.Cleanup(func() {
		if err := syscall.Kill(member, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			t.Errorf("kill member %d: %v", member, err)
		}
	})
	cleaner := &fakeCleaner{}
	text := sweepAfterSignal(t.Context(), dockerPeersVerb, cleaner, group, 200*time.Millisecond)
	if !slices.Equal(cleaner.dry, []bool{false}) {
		t.Errorf("CleanupOrphans calls = %v, want one non-dry sweep past the bound", cleaner.dry)
	}
	for _, want := range []string{strconv.Itoa(member), "had not all exited", "live-owned: process group ", "./le test integration cleanup"} {
		if !strings.Contains(text, want) {
			t.Errorf("summary %q lacks %q", text, want)
		}
	}
}
