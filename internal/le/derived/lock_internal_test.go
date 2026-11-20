// Design: docs/architecture/core-design.md -- a derived artifact declares its inputs and its rebuild
// Related: lock.go -- the lock these tests hold against each other
//
// VALIDATES: the derived-artifact lock excludes a second holder that opened its
// own descriptor, and a removal never runs inside a batch a rebuild is
// publishing; a build a peer finished while Ensure waited is reused; a caller
// with a deadline is refused as busy rather than left waiting; a permission
// refusal names whether the lock file or its directory refused, with its fix.
// PREVENTS: the 2026-09-21 refusal (plan/journal/check-mode-mutates-the-tree.md):
// another session's invalidation removed rfc/requirements between a batch's
// first temporary and its last rename, and the reader whose grep asked for the
// build was refused.
//
// These run inside the package because writeTemporary is unexported. Every
// exclusion is checked with a SEPARATE open of the lock file, because flock
// binds an open file description: two descriptors of one process exclude each
// other exactly as two processes do, so the probe is not goroutine-local luck.
package derived

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// openLock opens its own descriptor to root's lock file, creating its
// directory, and leaves the closing to the caller.
func openLock(t *testing.T, root string) *os.File {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(LockPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create the lock directory: %v", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // a fixture path under t.TempDir()
	if err != nil {
		t.Fatalf("open the lock with a descriptor of its own: %v", err)
	}
	return file
}

// probeLock opens its own descriptor to root's lock and reports whether a
// non-blocking exclusive lock is refused, which is the answer "someone holds
// it". A lock it took is released before it answers.
func probeLock(t *testing.T, root string) bool {
	t.Helper()
	file := openLock(t, root)
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

// holdLock takes root's lock on a descriptor of its own, as another session
// would, and keeps it until the test ends.
func holdLock(t *testing.T, root string) {
	t.Helper()
	file := openLock(t, root)
	t.Cleanup(func() { file.Close() }) //nolint:errcheck // closing releases the lock the test held
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("hold the lock as another session: %v", err)
	}
}

// TestTheDerivedLockExcludesASecondHolderAndReleases takes the lock, probes it
// from a second descriptor, releases it and probes again.
func TestTheDerivedLockExcludesASecondHolderAndReleases(t *testing.T) {
	root := t.TempDir()
	if probeLock(t, root) {
		t.Fatal("the lock is held before anything took it, so the probe below proves nothing")
	}

	held, err := lock(root, time.Time{})
	if err != nil {
		t.Fatalf("take the lock: %v", err)
	}
	if !probeLock(t, root) {
		t.Error("a second descriptor took the lock while the first held it")
	}
	if err := held.Close(); err != nil {
		t.Fatalf("release the lock: %v", err)
	}
	if probeLock(t, root) {
		t.Error("the lock is still held after its holder closed it")
	}
}

// TestALockThatCannotBeTakenSaysSo puts a regular file where the lock's
// directory belongs, so the lock cannot be created, and asks for the typed
// refusal the hooks turn into their message, with the cause that selects the
// fix Operator names.
func TestALockThatCannotBeTakenSaysSo(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "tmp"), []byte("not a directory\n"), 0o600); err != nil {
		t.Fatalf("block tmp/: %v", err)
	}
	artifact := Artifact{Path: "page.md", Rebuild: func(string) error {
		t.Error("Rebuild ran without the lock")
		return nil
	}}
	var lockErr *LockError
	if err := artifact.Ensure(root, time.Time{}); !errors.As(err, &lockErr) {
		t.Errorf("Ensure without a lock answered %v, want a *LockError", err)
	} else if lockErr.Cause != LockCauseNotDirectory {
		t.Errorf("Ensure's refusal has cause %d, want LockCauseNotDirectory (%d)", lockErr.Cause, LockCauseNotDirectory)
	}
	lockErr = nil
	if err := artifact.Remove(root, time.Time{}); !errors.As(err, &lockErr) {
		t.Errorf("Remove without a lock answered %v, want a *LockError", err)
	} else if lockErr.Cause != LockCauseNotDirectory {
		t.Errorf("Remove's refusal has cause %d, want LockCauseNotDirectory (%d)", lockErr.Cause, LockCauseNotDirectory)
	}
}

// TestAPermissionRefusalNamesWhichSideRefused drives the two permission
// failures apart. A lock file that exists with mode 0000 refuses the open, and
// the fix is its owner or its mode, so the cause is LockCauseFileRefused and
// the error names the lock file and a chown and chmod of it. A tmp/ the user
// cannot write, with no lock file in it, refuses the create, so the cause is
// LockCausePermission and the fix is the directory. Each error carries its fix
// after the cause, so a command that prints only Error() still shows it.
func TestAPermissionRefusalNamesWhichSideRefused(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root opens a mode 0000 file and creates in a read-only directory, so neither refusal happens")
	}

	refusedRoot := t.TempDir()
	path := filepath.Join(refusedRoot, filepath.FromSlash(LockPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
	var lockErr *LockError
	if _, err := lock(refusedRoot, time.Time{}); !errors.As(err, &lockErr) {
		t.Errorf("a lock file of mode 0000 answered %v, want a *LockError", err)
	} else {
		if lockErr.Cause != LockCauseFileRefused {
			t.Errorf("a lock file of mode 0000 has cause %d, want LockCauseFileRefused (%d)", lockErr.Cause, LockCauseFileRefused)
		}
		if message := lockErr.Error(); !strings.Contains(message, "chmod u+rw "+path) {
			t.Errorf("the refusal does not carry the fix of the lock file's mode: %s", message)
		}
	}

	createRoot := t.TempDir()
	tmp := filepath.Join(createRoot, "tmp")
	if err := os.Mkdir(tmp, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(tmp, 0o750) })
	lockErr = nil
	if _, err := lock(createRoot, time.Time{}); !errors.As(err, &lockErr) {
		t.Errorf("a read-only tmp/ answered %v, want a *LockError", err)
	} else {
		if lockErr.Cause != LockCausePermission {
			t.Errorf("a read-only tmp/ has cause %d, want LockCausePermission (%d)", lockErr.Cause, LockCausePermission)
		}
		if message := lockErr.Error(); !strings.Contains(message, "chmod u+rwx "+tmp+"/") {
			t.Errorf("the refusal does not carry the fix of the directory: %s", message)
		}
	}
}

// TestALockHeldPastTheDeadlineIsRefusedAsBusy holds the lock on a descriptor
// of its own, as another session would, and calls Ensure and Remove with a
// deadline already passed. Each must answer a *LockError whose cause is
// LockCauseBusy, without running the rebuild or removing the artifact, rather
// than waiting for a holder that never lets go.
func TestALockHeldPastTheDeadlineIsRefusedAsBusy(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "page.md")
	if err := WriteAtomic(page, []byte("# page\n")); err != nil {
		t.Fatalf("write the page: %v", err)
	}
	artifact := Artifact{
		Path: "page.md",
		Rebuild: func(string) error {
			t.Error("Rebuild ran while another session held the lock")
			return nil
		},
		Complete: func(string) bool { return false },
	}
	holdLock(t, root)

	var lockErr *LockError
	if err := artifact.Ensure(root, time.Now()); !errors.As(err, &lockErr) {
		t.Errorf("Ensure past its deadline answered %v, want a *LockError", err)
	} else if lockErr.Cause != LockCauseBusy {
		t.Errorf("Ensure's refusal has cause %d, want LockCauseBusy (%d)", lockErr.Cause, LockCauseBusy)
	}
	lockErr = nil
	if err := artifact.Remove(root, time.Now()); !errors.As(err, &lockErr) {
		t.Errorf("Remove past its deadline answered %v, want a *LockError", err)
	} else if lockErr.Cause != LockCauseBusy {
		t.Errorf("Remove's refusal has cause %d, want LockCauseBusy (%d)", lockErr.Cause, LockCauseBusy)
	}
	if _, err := os.Stat(page); err != nil {
		t.Errorf("Remove deleted the page without the lock: %v", err)
	}
}

// TestARemovalNeverRunsInsideABatchInFlight pauses a directory rebuild between
// two batches and checks that the lock is held there, then lets a removal race
// the rest of the rebuild. The rebuild must finish without error whatever the
// order, and the removal must still run once it is done.
func TestARemovalNeverRunsInsideABatchInFlight(t *testing.T) {
	root := t.TempDir()
	paused := make(chan struct{})
	resume := make(chan struct{})
	members := filepath.Join(root, "family", "members")
	marker := filepath.Join(members, ".finished")
	artifact := Artifact{
		Path: "family/members",
		Rebuild: func(string) error {
			if err := os.MkdirAll(members, 0o750); err != nil {
				return err
			}
			if err := WriteAtomicAll([]File{{Path: filepath.Join(members, "one.md"), Content: []byte("one\n")}}); err != nil {
				return err
			}
			close(paused)
			<-resume
			return WriteAtomicAll([]File{
				{Path: filepath.Join(members, "two.md"), Content: []byte("two\n")},
				{Path: marker, Content: []byte("done\n")},
			})
		},
		Complete: func(string) bool {
			_, err := os.Stat(marker)
			return err == nil
		},
	}

	built := make(chan error, 1)
	go func() { built <- artifact.Ensure(root, time.Time{}) }()
	select {
	case <-paused:
	case err := <-built:
		t.Fatalf("the rebuild ended before its pause: %v", err)
	}

	if !probeLock(t, root) {
		t.Error("a rebuild between two batches holds no lock, so a removal can cut through it")
	}
	removed := make(chan error, 1)
	go func() { removed <- artifact.Remove(root, time.Time{}) }()
	close(resume)

	if err := <-built; err != nil {
		t.Errorf("the rebuild failed while a removal waited: %v", err)
	}
	if err := <-removed; err != nil {
		t.Errorf("the removal failed: %v", err)
	}
	if _, err := os.Stat(members); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the removal did not run once the rebuild finished: %v", err)
	}
}

// TestEnsureReusesABuildAPeerFinishedWhileItWaited holds the lock as a peer
// would, waits until Ensure reports through the lockWaiting seam that it found
// the lock held, then writes the artifact whole under it and releases. Ensure
// asks Whole only once it holds the lock, so it must find the peer's build and
// not run its own. The seam orders the peer's write after the wait began, so
// the test cannot pass by Ensure arriving after the release.
func TestEnsureReusesABuildAPeerFinishedWhileItWaited(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "page.md")
	var rebuilds atomic.Int32
	artifact := Artifact{Path: "page.md", Rebuild: func(string) error {
		rebuilds.Add(1)
		return WriteAtomic(page, []byte("# mine\n"))
	}}

	waiting := make(chan struct{})
	lockWaiting = func() { close(waiting) }
	t.Cleanup(func() { lockWaiting = nil })

	peer, err := lock(root, time.Time{})
	if err != nil {
		t.Fatalf("take the lock as the peer: %v", err)
	}
	defer peer.Close() //nolint:errcheck // a second close after the release below is harmless; this one covers a failure before it
	ensured := make(chan error, 1)
	go func() { ensured <- artifact.Ensure(root, time.Time{}) }()
	// Bounded: an Ensure that blocks on the peer's lock without calling the
	// seam would otherwise hang to the go test timeout. The deferred close
	// releases the peer's lock, so that Ensure ends after the failure.
	select {
	case <-waiting:
	case err := <-ensured:
		t.Fatalf("Ensure returned before it waited for the peer's lock: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("Ensure never reported through the lockWaiting seam that it found the peer's lock held")
	}
	if err := WriteAtomic(page, []byte("# peer\n")); err != nil {
		t.Fatalf("write the peer's build: %v", err)
	}
	if err := peer.Close(); err != nil {
		t.Fatalf("release the peer's lock: %v", err)
	}

	if err := <-ensured; err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if count := rebuilds.Load(); count != 0 {
		t.Errorf("Ensure rebuilt %d time(s) over a build the peer had just finished", count)
	}
	body, err := os.ReadFile(page) //nolint:gosec // a fixture path under t.TempDir()
	if err != nil {
		t.Fatalf("read the page: %v", err)
	}
	if string(body) != "# peer\n" {
		t.Errorf("the page is %q, want the peer's build", body)
	}
}

// TestWriteTemporaryLeavesAReadableFile asks for the mode writeTemporary sets
// through the open descriptor, on the temporary itself rather than on a
// published page, so a regression in the descriptor route is named here.
func TestWriteTemporaryLeavesAReadableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "page.md")
	temporaryPath, err := writeTemporary(path, []byte("# page\n"))
	if err != nil {
		t.Fatalf("writeTemporary: %v", err)
	}
	info, err := os.Stat(temporaryPath)
	if err != nil {
		t.Fatalf("stat the temporary: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o644 {
		t.Errorf("the temporary is %04o, want 0644", mode)
	}
}
