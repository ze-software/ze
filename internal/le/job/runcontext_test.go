// Design: docs/architecture/testing/runner-architecture.md -- a verb's signal root reaches its admitted builds
// Related: contention_test.go -- the admission, holder and entry helpers these cases reuse.

package job

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// runContextBound is how long a case waits for RunContext to answer before
// it calls the wait unbounded. A wait the root ends answers within one poll.
const runContextBound = 10 * time.Second

// VALIDATES: RunContext under a done root starts no child, and answers a
// failure code rather than 0.
// METHOD: an admission over a free registry runs a command that records it
// ran, under a ctx canceled before the call.
// PREVENTS: a verb whose root a SIGTERM already ended starting a cross-compile
// it will not wait for.
func TestRunContextStartsNoChildUnderADoneRoot(t *testing.T) {
	detach(t)
	root := fixtureRepo(t)
	ran := filepath.Join(root, "tmp", "ran")
	canceled, cancel := context.WithCancel(t.Context())
	cancel()

	_, code := admission(t, root).RunContext(canceled, "probe", []string{"sh", "-c", record(ran, "admitted")}, root, nil)
	if code == 0 {
		t.Error("RunContext under a canceled root answered 0")
	}
	if _, err := os.Stat(ran); err == nil {
		t.Error("RunContext started the child under a canceled root")
	}
}

// VALIDATES: RunContext stops waiting for a slot when its root ends, so a
// SIGTERM that a verb's signal root captured is not held up by the queue.
// METHOD: a live stand-in holds the only slot; the waiter's root is canceled
// after a few polls, and RunContext must answer well inside runContextBound,
// with no admission in its report. A queue on context.Background waits out
// the bound.
// PREVENTS: a verb rooted on signals waiting up to an hour for a slot after
// the operator stopped it, because NotifyContext took SIGTERM from Go's
// default action and the queue never asked the root.
func TestRunContextEndsTheWaitForASlot(t *testing.T) {
	detach(t)
	root := fixtureRepo(t)
	ran := filepath.Join(root, "tmp", "ran")
	holder := sleeper(t)
	heldEntry(t, root, "holder", holder.Process.Pid)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	timer := time.AfterFunc(3*helperPoll, cancel)
	defer timer.Stop()

	answered := make(chan Report, 1)
	go func() {
		report, _ := admission(t, root).RunContext(ctx, "probe", []string{"sh", "-c", record(ran, "admitted")}, root, nil)
		answered <- report
	}()

	select {
	case report := <-answered:
		if report.Code == 0 {
			t.Error("RunContext answered 0 for a wait its root ended")
		}
		if report.Admission != KindUnspecified.String() {
			t.Errorf("RunContext reported admission %q for a wait its root ended, want %q",
				report.Admission, KindUnspecified.String())
		}
	case <-time.After(runContextBound):
		t.Fatal("RunContext kept waiting for a slot after its root ended")
	}
	if _, err := os.Stat(ran); err == nil {
		t.Error("RunContext ran the child although the slot was never free")
	}
}

// awaitFile blocks until path exists, so a case acts only once the child it
// started is running.
func awaitFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(runContextBound)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(helperPoll)
	}
	t.Fatalf("%s never appeared: the child never started", path)
}

// VALIDATES: RunContext sends a running child SIGTERM when its root ends, and
// answers once the child has gone, rather than waiting for it to finish.
// METHOD: an admission over a free registry runs a shell that records SIGTERM
// in its trap, holding a 30 s sleep; the root is canceled once the shell is
// up. RunContext must answer well inside runContextBound and the trap must
// have fired. A child started on context.Background never sees the cancel
// and outlasts the bound; one canceled with SIGKILL never runs its trap.
// PREVENTS: a verb whose root a SIGTERM ended still waiting out a running
// cross-compile, or killing it with no chance to release its own children.
func TestRunContextStopsARunningChildWithSIGTERM(t *testing.T) {
	detach(t)
	root := fixtureRepo(t)
	up := filepath.Join(root, "tmp", "up")
	termed := filepath.Join(root, "tmp", "termed")
	script := "trap '" + record(termed, "term") + "; kill $child; exit 143' TERM; " +
		"sleep 30 >/dev/null 2>&1 & child=$!; " + record(up, "up") + "; wait"

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	answered := make(chan int, 1)
	go func() {
		_, code := admission(t, root).RunContext(ctx, "probe", []string{"sh", "-c", script}, root, nil)
		answered <- code
	}()
	awaitFile(t, up)
	cancel()

	select {
	case code := <-answered:
		if code == 0 {
			t.Error("RunContext answered 0 for a child its root stopped")
		}
	case <-time.After(runContextBound):
		t.Fatal("RunContext kept waiting for a running child after its root ended")
	}
	if got := read(t, termed); got != "term\n" {
		t.Errorf("the child recorded %q, want SIGTERM to have reached its trap", got)
	}
}

// VALIDATES: RunContext stops following a shared run when its root ends, and
// runs nothing of its own.
// METHOD: a live stand-in holds the slot under an entry naming this exact
// work (label, inputs and command), so the asker attaches and follows it. The
// root is canceled after a few polls; RunContext must answer well inside
// runContextBound, and the command must not have run. A follow on
// context.Background watches the stand-in until it exits, past the bound.
// PREVENTS: a verb whose root a SIGTERM ended watching another session's
// build to its end, because the follow never asked the root.
func TestRunContextEndsTheFollowOfASharedRun(t *testing.T) {
	detach(t)
	t.Setenv(AttachKey, "1")
	root := fixtureRepo(t)
	ran := filepath.Join(root, "tmp", "ran")
	argv := []string{"sh", "-c", record(ran, "admitted")}
	holder := sleeper(t)
	entry, _ := heldEntry(t, root, "probe", holder.Process.Pid)
	shareEntry(t, root, entry, argv)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	timer := time.AfterFunc(3*helperPoll, cancel)
	defer timer.Stop()

	answered := make(chan int, 1)
	go func() {
		_, code := admission(t, root).RunContext(ctx, "probe", argv, root, nil)
		answered <- code
	}()

	select {
	case code := <-answered:
		if code == 0 {
			t.Error("RunContext answered 0 for a follow its root ended")
		}
	case <-time.After(runContextBound):
		t.Fatal("RunContext kept following the shared run after its root ended")
	}
	if _, err := os.Stat(ran); err == nil {
		t.Error("RunContext ran its own copy of the work after its root ended")
	}
}

// shareEntry rewrites a fabricated holder entry so it names the work argv
// asks for over root's inputs: the entry an identical asker attaches to.
func shareEntry(t *testing.T, root, entry string, argv []string) {
	t.Helper()
	body, err := os.ReadFile(entry) //nolint:gosec // the test's own fixture entry
	if err != nil {
		t.Fatalf("read the holder entry: %v", err)
	}
	tree, err := InputHash(root, "probe")
	if err != nil {
		t.Fatalf("fingerprint the fixture: %v", err)
	}
	shared := strings.Replace(string(body), "KEY=fabricated\n", "KEY="+jobKey(argv)+"\n", 1)
	shared = strings.Replace(shared, "TREE="+TreeHash(root)+"\n", "TREE="+tree+"\n", 1)
	if shared == string(body) {
		t.Fatal("the holder entry carried neither the fabricated key nor the tree hash")
	}
	if err := os.WriteFile(entry, []byte(shared), 0o644); err != nil { //nolint:gosec // the registry is world-readable by design
		t.Fatalf("write the holder entry: %v", err)
	}
}
