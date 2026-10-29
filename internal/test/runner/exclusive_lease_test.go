// Design: docs/architecture/testing/runner-architecture.md — stable exclusive-group lease inodes.

package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// TestExclusiveGroupLeaseStableInodeAndCancellation retains an independent open
// descriptor while real runner processes hold, cancel a waiter, release, and
// reacquire the same group. The original inode MUST remain the lock authority.
func TestExclusiveGroupLeaseStableInodeAndCancellation(t *testing.T) {
	group := t.TempDir()
	first := startExclusiveProcess(t, group, "hold")
	first.requireEntered(t)
	path := exclusiveGroupTestPath(portLockDirectory(), group)
	witness, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := witness.Close(); err != nil {
			t.Error(err)
		}
	})
	original, err := witness.Stat()
	if err != nil {
		t.Fatal(err)
	}
	assertHeld := func() {
		t.Helper()
		current, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(original, current) {
			t.Fatal("group lease replaced its stable lock inode")
		}
		if err := syscall.Flock(int(witness.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !isWouldBlock(err) {
			t.Fatalf("original lock inode admitted a second holder: %v", err)
		}
	}
	assertHeld()
	waiter := startExclusiveProcess(t, group, "contend")
	if event := waiter.next(t); event.Entered || !event.Finished || !event.Canceled {
		t.Fatalf("waiter did not cancel without entering: %+v", event)
	}
	waiter.stop(t)
	assertHeld()
	first.stop(t)

	second := startExclusiveProcess(t, group, "hold")
	second.requireEntered(t)
	assertHeld()
	second.stop(t)
	current, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(original, current) {
		t.Fatal("release unlinked or replaced the group lock inode")
	}
	if err := syscall.Flock(int(witness.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("released group remains locked: %v", err)
	}
}

func exclusiveGroupTestPath(dir, group string) string {
	digest := sha256.Sum256([]byte(group))
	return filepath.Join(dir, "group-"+hex.EncodeToString(digest[:])+".lock")
}

// TestExclusiveGroupLeaseRefusesUnsafePaths exercises real filesystem objects,
// including the existing directory trust check and final-component symlinks.
func TestExclusiveGroupLeaseRefusesUnsafePaths(t *testing.T) {
	for _, kind := range []string{"directory-symlink", "directory-writable", "file-symlink", "file-writable", "file-directory", "file-fifo"} {
		t.Run(kind, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "locks")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			path := exclusiveGroupTestPath(dir, "test-group")
			switch kind {
			case "directory-symlink":
				link := filepath.Join(t.TempDir(), "link")
				if err := os.Symlink(dir, link); err != nil {
					t.Fatal(err)
				}
				dir = link
			case "directory-writable":
				if err := os.Chmod(dir, 0o777); err != nil {
					t.Fatal(err)
				}
			case "file-symlink":
				if err := os.Symlink(filepath.Join(t.TempDir(), "target"), path); err != nil {
					t.Fatal(err)
				}
			case "file-writable":
				if err := os.WriteFile(path, nil, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(path, 0o666); err != nil {
					t.Fatal(err)
				}
			case "file-directory":
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			case "file-fifo":
				if err := syscall.Mkfifo(path, 0o600); err != nil {
					t.Fatal(err)
				}
			default:
				t.Fatalf("unknown test object %q", kind)
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			lease, err := leaseExclusiveGroupIn(ctx, dir, "test-group")
			if lease != nil {
				if err := lease.Close(); err != nil {
					t.Error(err)
				}
			}
			if err == nil {
				t.Fatal("unsafe lock path was accepted")
			}
			if !strings.Contains(err.Error(), dir) {
				t.Fatalf("lock refusal does not identify its path: %v", err)
			}
		})
	}
}

// TestParallelRunnerExclusiveAcquisitionFailureIsVisible plants an unusable
// object at this test's unique group path. Run MUST fail without its callback,
// retain the error on the record, and hand that same error to the failure hook.
func TestParallelRunnerExclusiveAcquisitionFailureIsVisible(t *testing.T) {
	group := t.TempDir()
	dir := portLockDirectory()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := checkPortLockDir(dir); err != nil {
		t.Fatal(err)
	}
	path := exclusiveGroupTestPath(dir, group)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// This is the directory planted above, never a held lock inode.
		if err := os.Remove(path); err != nil {
			t.Error(err)
		}
	})
	r := NewParallelRunner[string](NewColorsWithOverride(false))
	r.SetQuiet(true)
	r.SetVerbose(true)
	r.setNoSummary(true)
	called := false
	record := r.addTestWithoutNick("unusable-group", "", func(context.Context, string) (bool, error) {
		called = true
		return true, nil
	})
	record.ExclusiveGroup = group
	r.display.SetOutput(io.Discard)
	var reported error
	r.SetOnFail(func(_ string, err error) { reported = err })
	if r.Run(t.Context()) {
		t.Fatal("runner accepted an unusable group lease")
	}
	if called {
		t.Fatal("runner executed without its requested group lease")
	}
	if record.State != StateFail || record.Error == nil || reported == nil {
		t.Fatalf("missing visible failure: state=%v record=%v reported=%v", record.State, record.Error, reported)
	}
	if !strings.Contains(reported.Error(), group) || !errors.Is(reported, record.Error) {
		t.Fatalf("failure hook lost the group's acquisition error: %v", reported)
	}
}

// TestParallelRunnerExclusiveCancellationWhileWaitingForSlot holds the only
// concurrency slot, then observes a grouped test acquiring its lease before
// waiting for that slot. Cancellation MUST release the group while the first
// callback still owns the slot, and MUST NOT invoke the canceled callback.
func TestParallelRunnerExclusiveCancellationWhileWaitingForSlot(t *testing.T) {
	group := t.TempDir()
	initial, err := leaseExclusiveGroup(t.Context(), group)
	if err != nil {
		t.Fatal(err)
	}
	initialOpen := true
	defer func() {
		if initialOpen {
			if err := initial.Close(); err != nil {
				t.Error(err)
			}
		}
	}()
	witness, err := os.OpenFile(exclusiveGroupTestPath(portLockDirectory(), group), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := witness.Close(); err != nil {
			t.Error(err)
		}
	}()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	r := NewParallelRunner[string](NewColorsWithOverride(false))
	r.SetQuiet(true)
	r.setNoSummary(true)
	r.SetConcurrency(1)
	entered := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	r.addTestWithoutNick("slot-holder", "", func(context.Context, string) (bool, error) {
		close(entered)
		<-release
		return true, nil
	})
	called := false
	record := r.addTestWithoutNick("slot-waiter", "", func(context.Context, string) (bool, error) {
		called = true
		return true, nil
	})
	record.ExclusiveGroup = group
	r.display.SetOutput(io.Discard)
	finished := make(chan bool, 1)
	done := make(chan struct{})
	go func() {
		finished <- r.Run(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		releaseOnce.Do(func() { close(release) })
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("runner did not stop during cleanup")
		}
	})
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("ungrouped test could not enter while a group lease was held")
	}
	if err := initial.Close(); err != nil {
		t.Fatal(err)
	}
	initialOpen = false

	// The observed flock is the barrier: no scheduler sleep guesses whether the
	// grouped test has reached its semaphore wait.
	awaitExclusiveGroupLock(t, witness, true)
	cancel()
	awaitExclusiveGroupLock(t, witness, false)
	releaseOnce.Do(func() { close(release) })
	select {
	case passed := <-finished:
		if passed || called || !errors.Is(record.Error, context.Canceled) {
			t.Fatalf("canceled slot waiter: passed=%t called=%t error=%v", passed, called, record.Error)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runner did not finish after cancellation and slot release")
	}
}

func awaitExclusiveGroupLock(t *testing.T, witness *os.File, held bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		err := syscall.Flock(int(witness.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		switch {
		case err == nil:
			if err := syscall.Flock(int(witness.Fd()), syscall.LOCK_UN); err != nil {
				t.Fatal(err)
			}
			if !held {
				return
			}
		case isWouldBlock(err):
			if held {
				return
			}
		default:
			t.Fatal(err)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("group lock did not become held=%t: %v", held, ctx.Err())
		case <-ticker.C:
		}
	}
}
