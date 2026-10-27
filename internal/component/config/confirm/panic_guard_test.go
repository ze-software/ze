package confirm

import (
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/crashlog"
)

// panickingCommit is a commit whose apply writes next to d's running config,
// as a part-applied commit would, and then meets a Ze defect.
func panickingCommit(d *fakeDaemon, next string) Commit {
	commit := d.commit(next)
	apply := commit.Apply
	commit.Apply = func() error {
		if err := apply(); err != nil {
			return err
		}
		panic("boom: apply defect")
	}
	return commit
}

// TestConfirmedApplyPanicArmsWindow proves a confirmed commit whose apply
// panics keeps a revert behind what it applied. Method: on the real worker,
// open a window (and, in the second case, force a new countdown into an open
// one) with an apply that changes the running config and then panics. The
// caller must get the defect error, the window must be open with the recorded
// deadline, and the worker must revert to the first rollback at that deadline
// and clear the record, so no stale record is left for the next start.
func TestConfirmedApplyPanicArmsWindow(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, w *Window, d *fakeDaemon)
		force bool
	}{
		{name: "open", setup: func(*testing.T, *Window, *fakeDaemon) {}},
		{
			name: "force",
			setup: func(t *testing.T, w *Window, d *fakeDaemon) {
				if err := w.Confirmed("alice", time.Hour, false, d.commit("v2")); err != nil {
					t.Fatalf("opening the first window: %v", err)
				}
			},
			force: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))
			d := newFakeDaemon("v1")
			w := newTestWindow(t, d)
			tc.setup(t, w, d)

			err := w.Confirmed("alice", 100*time.Millisecond, tc.force, panickingCommit(d, "v3"))
			if err == nil {
				t.Fatal("Confirmed answered nil for an apply that panicked")
			}
			if !strings.Contains(err.Error(), "internal Ze defect ended this config write") {
				t.Fatalf("Confirmed error %q does not say an internal defect ended the write", err)
			}
			status, open := w.Status()
			if !open {
				t.Fatal("no window is open after the panicked apply: what it applied has no revert behind it")
			}
			if left := status.Left(); left > time.Minute {
				t.Fatalf("the window's deadline is %v away, want the new countdown", left)
			}
			if got := waitRevert(t, d); got != "v1" {
				t.Fatalf("the deadline reverted to %q, want the first rollback v1", got)
			}
			// A write after the revert waits for the worker to finish it, and
			// is refused while the window would still be open.
			if err := WriteOutside(w, func() error { return nil }); err != nil {
				t.Fatalf("the write after the deadline revert failed: %v", err)
			}
			if got := d.config(); got != "v1" {
				t.Fatalf("running config after the revert is %q, want v1", got)
			}
			if d.saved() != nil {
				t.Fatal("the record is still on disk after the revert: the next start would revert again")
			}
		})
	}
}

// TestConfirmWritePanicContained proves a Ze defect met by one config write on
// the window's worker ends that write only. Method: WriteOutside, the route a
// gNMI Set, an API config session and a managed push take, runs an apply that
// panics on the real worker; the caller must get a plain error without the
// panic value, the guard must count and crash-record it, and the same worker
// must then run the next write.
func TestConfirmWritePanicContained(t *testing.T) {
	before, declared := writeGuard.Panics(taskWrite)
	if !declared {
		t.Fatalf("task %q is not declared on the config-confirm guard", taskWrite)
	}
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))

	d := newFakeDaemon("v1")
	w := NewWindow(d.Revert, d)
	t.Cleanup(w.Stop)

	err := WriteOutside(w, func() error { panic("boom: apply defect") })
	if err == nil {
		t.Fatal("WriteOutside answered nil for a write that panicked")
	}
	if !strings.Contains(err.Error(), "internal Ze defect ended this config write") {
		t.Fatalf("WriteOutside error %q does not say an internal defect ended the write", err)
	}
	if strings.Contains(err.Error(), "boom") {
		t.Fatalf("WriteOutside error %q carries the panic value to the caller", err)
	}

	applied := false
	if err := WriteOutside(w, func() error { applied = true; return nil }); err != nil {
		t.Fatalf("the write after the contained panic failed: %v", err)
	}
	if !applied {
		t.Fatal("the worker did not run the write after the contained panic")
	}

	if count, _ := writeGuard.Panics(taskWrite); count != before+1 {
		t.Fatalf("Panics(%q) = %d, want %d: the panic was not counted", taskWrite, count, before+1)
	}
	crashes := crashlog.ListCrashes()
	if len(crashes) != 1 {
		t.Fatalf("ListCrashes() = %d records, want 1", len(crashes))
	}
	if crashes[0].Kind != crashlog.KindRecovered {
		t.Fatalf("crash record %+v is not %q", crashes[0], crashlog.KindRecovered)
	}
	record := crashlog.ReadCrash(crashes[0].Name)
	if !strings.Contains(record, "config-confirm "+taskWrite+", unit config write") {
		t.Fatalf("crash record does not name the task and unit:\n%s", record)
	}
	if !strings.Contains(err.Error(), crashes[0].Name) {
		t.Fatalf("WriteOutside error %q does not name the crash record %q", err, crashes[0].Name)
	}
}
