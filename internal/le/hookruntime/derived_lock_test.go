// Design: docs/architecture/core-design.md -- a derived artifact declares its inputs and its rebuild
// Related: bash.go -- the read half, materializeNamed
// Related: postwrite.go -- the write half, invalidateFed
//
// VALIDATES: an invalidation that arrives while a reader's rebuild is between
// two batches leaves the reader's command allowed, and the artifact ends whole
// once the next reader names it; a lock another session holds past the hook's
// wait refuses the reader with "run it again" and reports the invalidation's
// artifact stale and not removed; the waits stay inside the hook timeouts the
// settings file declares.
// PREVENTS: the 2026-09-21 refusal (plan/journal/check-mode-mutates-the-tree.md):
// another session's edit removed rfc/requirements between the batch's first
// temporary and its last rename, the rebuild failed on a directory that was
// gone, and a harmless grep was refused as "is derived and could not be built".
package hookruntime

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/derived"
)

// openDerivedLock opens its own descriptor to root's derived-artifact lock
// file, creating its directory, and leaves the closing to the caller.
func openDerivedLock(t *testing.T, root string) *os.File {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(derived.LockPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create the lock directory: %v", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // a fixture path under t.TempDir()
	if err != nil {
		t.Fatalf("open the lock with a descriptor of its own: %v", err)
	}
	return file
}

// derivedLockHeld opens its own descriptor to root's derived-artifact lock and
// reports whether a non-blocking exclusive lock is refused. flock binds an open
// file description, so this descriptor is excluded exactly as another
// session's process would be.
func derivedLockHeld(t *testing.T, root string) bool {
	t.Helper()
	file := openDerivedLock(t, root)
	defer file.Close() //nolint:errcheck // closing releases whatever the probe took
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		return false
	}
	if !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatalf("probe the lock: %v", err)
	}
	return true
}

// holdDerivedLock takes root's derived-artifact lock on a descriptor of its
// own, as another session's process would, and keeps it until the test ends.
func holdDerivedLock(t *testing.T, root string) {
	t.Helper()
	file := openDerivedLock(t, root)
	t.Cleanup(func() { file.Close() }) //nolint:errcheck // closing releases the lock the test held
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("hold the lock as another session: %v", err)
	}
}

// TestAnInvalidationDuringARebuildNeverRefusesTheReader drives both hook
// halves over one directory artifact whose first rebuild pauses between its
// two batches, the shape of the RFC generator's shards and its marker.
//
// The reader starts the rebuild and the test waits for the pause. The edit's
// invalidation then starts while the batch is in flight. When the lock is
// held, the rebuild is resumed at once: the removal waits for it whatever the
// scheduling. When no lock is held, which is the behavior before the lock,
// nothing makes the removal wait, so the test lets it finish inside the batch
// before resuming, which is the interleaving of the journal row, every run.
func TestAnInvalidationDuringARebuildNeverRefusesTheReader(t *testing.T) {
	root := t.TempDir()
	writeHookFixture(t, root, "family/input.md", "# input\n")
	members := filepath.Join(root, "family", "members")
	marker := filepath.Join(members, ".finished")
	paused := make(chan struct{})
	resume := make(chan struct{})
	var rebuilds atomic.Int32
	artifact := derived.Artifact{
		Path:  "family/members",
		Feeds: func(_, path string) bool { return path == "family/input.md" },
		Rebuild: func(string) error {
			first := rebuilds.Add(1) == 1
			if err := os.MkdirAll(members, 0o750); err != nil {
				return err
			}
			one := derived.File{Path: filepath.Join(members, "one.md"), Content: []byte("one\n")}
			if err := derived.WriteAtomicAll([]derived.File{one}); err != nil {
				return err
			}
			if first {
				close(paused)
				<-resume
			}
			return derived.WriteAtomicAll([]derived.File{
				{Path: filepath.Join(members, "two.md"), Content: []byte("two\n")},
				{Path: marker, Content: []byte("done\n")},
			})
		},
		Complete: func(string) bool {
			_, err := os.Stat(marker)
			return err == nil
		},
		SessionStart: derived.SessionStartDefer,
	}
	artifacts := []derived.Artifact{artifact}
	const command = "grep -rn one family/members"

	read := make(chan *verdict, 1)
	go func() { read <- materializeNamed(root, command, artifacts, time.Minute) }()
	select {
	case <-paused:
	case answer := <-read:
		t.Fatalf("the reader answered before its rebuild paused: %+v", answer)
	}

	invalidated := make(chan *verdict, 1)
	go func() { invalidated <- invalidateFed(root, "family/input.md", artifacts, time.Minute) }()
	var invalidation *verdict
	finished := false
	if !derivedLockHeld(t, root) {
		t.Error("the rebuild holds no derived-artifact lock between its two batches")
		invalidation = <-invalidated
		finished = true
	}
	close(resume)

	if answer := <-read; answer != nil && answer.code == 2 {
		t.Errorf("the reader was refused while an invalidation ran: %s", answer.message)
	}
	if !finished {
		invalidation = <-invalidated
	}
	if invalidation == nil {
		t.Fatal("the invalidation removed nothing, though the artifact was present when it ran")
	}
	if invalidation.code != 0 {
		t.Errorf("the invalidation reported a problem: %d %s", invalidation.code, invalidation.message)
	}

	// The edit removed what the paused rebuild wrote, so the next reader
	// builds it again, and the artifact ends whole.
	if answer := materializeNamed(root, command, artifacts, time.Minute); answer != nil {
		t.Errorf("the next reader was refused: %d %s", answer.code, answer.message)
	}
	whole, err := artifact.Whole(root)
	if err != nil {
		t.Fatalf("read the artifact: %v", err)
	}
	if !whole {
		t.Error("the artifact is not whole after the next reader named it")
	}
}

// TestALockHeldPastTheHookWaitIsReportedNotSilent holds the derived-artifact
// lock on a descriptor of its own, as another session's rebuild would, and
// calls both hook halves with no wait. The reader must be refused with code 2,
// told another session is rebuilding the artifact and to run the command
// again, and its rebuild must not run. The invalidation must answer code 2,
// because code 1 on a post-tool hook never reaches the agent that made the
// edit, with the artifact named stale and not removed, and the artifact must
// still be there. A hook the harness killed while it waited would say neither.
func TestALockHeldPastTheHookWaitIsReportedNotSilent(t *testing.T) {
	root := t.TempDir()
	writeHookFixture(t, root, "family/input.md", "# input\n")
	writeHookFixture(t, root, "family/members/one.md", "one\n")
	var rebuilds atomic.Int32
	artifact := derived.Artifact{
		Path:  "family/members",
		Feeds: func(_, path string) bool { return path == "family/input.md" },
		Rebuild: func(string) error {
			rebuilds.Add(1)
			return nil
		},
		Complete:     func(string) bool { return false },
		SessionStart: derived.SessionStartDefer,
	}
	artifacts := []derived.Artifact{artifact}
	holdDerivedLock(t, root)

	answer := materializeNamed(root, "grep -rn one family/members", artifacts, 0)
	if answer == nil {
		t.Fatal("the reader was allowed while another session held the lock past the wait")
	}
	if answer.code != 2 {
		t.Errorf("the reader's verdict is code %d, want 2: %s", answer.code, answer.message)
	}
	for _, words := range []string{"another session is rebuilding it now", "Run the command again"} {
		if !strings.Contains(answer.message, words) {
			t.Errorf("the reader's refusal does not say %q: %s", words, answer.message)
		}
	}
	if count := rebuilds.Load(); count != 0 {
		t.Errorf("the reader rebuilt %d time(s) without the lock", count)
	}

	invalidation := invalidateFed(root, "family/input.md", artifacts, 0)
	if invalidation == nil {
		t.Fatal("the invalidation said nothing while another session held the lock past the wait")
	}
	if invalidation.code != 2 {
		t.Errorf("the invalidation's verdict is code %d, want 2, the one post-tool code that reaches the editing agent: %s",
			invalidation.code, invalidation.message)
	}
	for _, words := range []string{"family/members is stale and was not removed", "another session"} {
		if !strings.Contains(invalidation.message, words) {
			t.Errorf("the invalidation's report does not say %q: %s", words, invalidation.message)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "family", "members", "one.md")); err != nil {
		t.Errorf("the invalidation removed the artifact without the lock: %v", err)
	}
}

// TestDerivedLockWaitsStayInsideTheHookTimeouts reads `.claude/settings.json`
// and checks that every entry running `le ai hooks pretool-bash` or `le ai
// hooks posttool-writeedit` carries the timeout bash.go copies into
// pretoolBashTimeout and posttoolWriteEditTimeout, and that each lock wait is
// below its hook's timeout. A drift lets the harness kill a hook while it
// waits, which reports nothing.
func TestDerivedLockWaitsStayInsideTheHookTimeouts(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "..", ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
				Timeout int    `json:"timeout"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(body, &settings); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		action  string
		timeout time.Duration
		wait    time.Duration
	}{
		{"pretool-bash", pretoolBashTimeout, materializeLockWait},
		{"posttool-writeedit", posttoolWriteEditTimeout, invalidateLockWait},
	}
	for _, one := range cases {
		if one.wait >= one.timeout {
			t.Errorf("%s waits %s for the lock, not below its timeout %s", one.action, one.wait, one.timeout)
		}
		found := 0
		for _, events := range settings.Hooks {
			for _, event := range events {
				for _, hook := range event.Hooks {
					if !strings.HasSuffix(hook.Command, " ai hooks "+one.action) {
						continue
					}
					found++
					if got := time.Duration(hook.Timeout) * time.Second; got != one.timeout {
						t.Errorf("settings give %q a timeout of %s, bash.go assumes %s", hook.Command, got, one.timeout)
					}
				}
			}
		}
		if found == 0 {
			t.Errorf("no hook entry in .claude/settings.json runs `le ai hooks %s`", one.action)
		}
	}
}
