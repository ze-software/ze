// Design: docs/architecture/zefs-format.md -- a tree walk beside a running writer.
// Related: check_tree_unix.go -- walkFrameDirectory, the walk under test.

//go:build linux || darwin

package zefs

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"golang.org/x/sys/unix"
)

// vanishFixture writes a clean frame at meta/a/one and meta/a/two and a frame
// whose CRC fails at file/x, under a new tree root it returns.
func vanishFixture(t *testing.T) string {
	t.Helper()
	frame, err := EncodeNetcapstring([]byte("secret"), 8)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := bytes.Clone(frame)
	corrupt[bytes.IndexByte(corrupt, '\n')+1] ^= 1
	root := filepath.Join(t.TempDir(), "source")
	writeFrameFixture(t, root, "meta/a/one", frame)
	writeFrameFixture(t, root, "meta/a/two", frame)
	writeFrameFixture(t, root, "file/x", corrupt)
	return root
}

// removedFrame is the frame of the fixture tree (meta/a/two) every test here
// removes under the walk.
const removedFrame = "two"

// removeOnOpen is the seam a removal by a writer in another process falls in:
// between the walk's listing of removedFrame and its open, it unlinks that
// frame, as the store's Remove does, once.
func removeOnOpen(t *testing.T) func(*os.File, string) {
	t.Helper()
	done := false
	return func(directory *os.File, listed string) {
		if done {
			return
		}
		if listed != removedFrame {
			return
		}
		done = true
		if err := unix.Unlinkat(int(directory.Fd()), listed, 0); err != nil {
			t.Fatal(err)
		}
	}
}

func entryStatus(report *CheckReport, key string) (EntryStatus, bool) {
	for _, entry := range report.Entries {
		if entry.Key == key {
			return entry, true
		}
	}
	return EntryStatus{}, false
}

// TestCheckPathReportsAKeyRemovedDuringTheWalk proves a key a running writer
// removes after the walk listed it neither fails the check nor reads as clean:
// the walk completes, names the key in Vanished, and still judges every frame
// it read, the corrupt one included. Method: the seam unlinks meta/a/two
// between its listing and its open. Red against the walk before the fix, which
// returned "zefs: open ...: no such file or directory" and no report.
func TestCheckPathReportsAKeyRemovedDuringTheWalk(t *testing.T) {
	root := vanishFixture(t)
	report, err := checkFrameTree(root, removeOnOpen(t))
	if err != nil {
		t.Fatalf("check failed on a removed key: %v", err)
	}
	if !slices.Equal(report.Vanished, []string{"meta/a/two"}) {
		t.Fatalf("vanished %q, want [meta/a/two]", report.Vanished)
	}
	if _, ok := entryStatus(report, "meta/a/two"); ok {
		t.Fatal("a removed key was reported as read")
	}
	one, ok := entryStatus(report, "meta/a/one")
	if !ok || one.Status != "ok" {
		t.Fatalf("meta/a/one = %+v, %v; want ok", one, ok)
	}
	if report.TotalEntries != 1 {
		t.Fatalf("clean entries %d, want 1", report.TotalEntries)
	}
}

// TestCheckPathCorruptFrameStaysCorruptBesideARemoval proves tolerating a
// removed key weakens nothing that counts as corruption: a frame that opens
// and fails its CRC is still reported corrupt in the same walk. Method: the
// fixture's file/x carries a flipped payload byte while meta/a/two vanishes.
func TestCheckPathCorruptFrameStaysCorruptBesideARemoval(t *testing.T) {
	root := vanishFixture(t)
	report, err := checkFrameTree(root, removeOnOpen(t))
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := entryStatus(report, "file/x")
	if !ok {
		t.Fatalf("file/x not reported: %+v", report.Entries)
	}
	if entry.Status != "crc-mismatch" {
		t.Fatalf("file/x status %q, want crc-mismatch: %s", entry.Status, entry.Error)
	}
	if report.CorruptEntries != 1 {
		t.Fatalf("corrupt entries %d, want 1", report.CorruptEntries)
	}
}

// TestCheckPathReportsADirectoryPrunedBeforeItsOpen proves a directory the
// writer pruned after its last key went, listed but gone at its open, is named
// with a trailing "/" rather than failing the walk. Method: the seam removes
// meta/gone/k and then meta/gone when the walk lists gone in meta.
func TestCheckPathReportsADirectoryPrunedBeforeItsOpen(t *testing.T) {
	root := vanishFixture(t)
	frame, err := EncodeNetcapstring([]byte("value"), 5)
	if err != nil {
		t.Fatal(err)
	}
	writeFrameFixture(t, root, "meta/gone/k", frame)
	done := false
	prune := func(directory *os.File, listed string) {
		if done {
			return
		}
		if listed != "gone" {
			return
		}
		done = true
		if err := os.Remove(filepath.Join(directory.Name(), "gone", "k")); err != nil {
			t.Fatal(err)
		}
		if err := unix.Unlinkat(int(directory.Fd()), "gone", unix.AT_REMOVEDIR); err != nil {
			t.Fatal(err)
		}
	}
	report, err := checkFrameTree(root, prune)
	if err != nil {
		t.Fatalf("check failed on a pruned directory: %v", err)
	}
	if !slices.Equal(report.Vanished, []string{"meta/gone/"}) {
		t.Fatalf("vanished %q, want [meta/gone/]", report.Vanished)
	}
}

// TestCheckPathReportsADirectoryPrunedWhileRead proves the walk survives a
// directory pruned while it holds it open. Method: when the walk lists k in
// meta/gone, the seam removes k and then meta/gone, as Remove does with an
// emptied ancestor. k is named; on Linux the next read of the dead directory
// answers ENOENT and the directory is named too. Red on Linux before the fix,
// which failed with "zefs: open" for k (and "zefs: read directory" after it).
func TestCheckPathReportsADirectoryPrunedWhileRead(t *testing.T) {
	root := vanishFixture(t)
	frame, err := EncodeNetcapstring([]byte("value"), 5)
	if err != nil {
		t.Fatal(err)
	}
	writeFrameFixture(t, root, "meta/gone/k", frame)
	done := false
	prune := func(directory *os.File, listed string) {
		if done {
			return
		}
		if filepath.Base(directory.Name()) != "gone" {
			return
		}
		done = true
		if err := unix.Unlinkat(int(directory.Fd()), listed, 0); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(directory.Name()); err != nil {
			t.Fatal(err)
		}
	}
	report, err := checkFrameTree(root, prune)
	if err != nil {
		t.Fatalf("check failed on a directory pruned while read: %v", err)
	}
	want := []string{"meta/gone/k"}
	if runtime.GOOS == "linux" {
		want = append(want, "meta/gone/")
	}
	if !slices.Equal(report.Vanished, want) {
		t.Fatalf("vanished %q, want %q", report.Vanished, want)
	}
}

// TestCheckPathFailsOnAnOpenErrorThatIsNotARemoval proves only not-exist is
// read as a removal: a listed entry replaced by a symlink fails its no-follow
// open with ELOOP, and the check fails with no report, as before.
func TestCheckPathFailsOnAnOpenErrorThatIsNotARemoval(t *testing.T) {
	root := vanishFixture(t)
	done := false
	replace := func(directory *os.File, listed string) {
		if done {
			return
		}
		if listed != "two" {
			return
		}
		done = true
		if err := unix.Unlinkat(int(directory.Fd()), listed, 0); err != nil {
			t.Fatal(err)
		}
		if err := unix.Symlinkat("one", int(directory.Fd()), listed); err != nil {
			t.Fatal(err)
		}
	}
	report, err := checkFrameTree(root, replace)
	if report != nil {
		t.Fatalf("report %+v beside an unsafe node", report)
	}
	if !errors.Is(err, unix.ELOOP) {
		t.Fatalf("error %v, want ELOOP", err)
	}
}

// TestFrameWalkWithNoVanishedReceiverFails proves no caller skips a removed key
// in silence: a walk that names no vanished receiver fails on it with the
// not-exist error, as every walk did before the receiver existed.
func TestFrameWalkWithNoVanishedReceiverFails(t *testing.T) {
	root := vanishFixture(t)
	walk := frameWalk{
		visit:      func(string, []byte) error { return nil },
		beforeOpen: removeOnOpen(t),
	}
	if err := walkFrameTree(root, walk); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("walk = %v, want a not-exist failure", err)
	}
}

// TestRepairPathReportsAKeyRemovedDuringTheSalvage proves a salvage beside a
// running writer completes, names the removed key in Vanished, recovers every
// clean frame it read, still skips the corrupt one, and publishes an output
// without the removed key. Red against the walk before the fix, which failed
// with "zefs: open ...: no such file or directory" and published nothing.
func TestRepairPathReportsAKeyRemovedDuringTheSalvage(t *testing.T) {
	root := vanishFixture(t)
	output := filepath.Join(t.TempDir(), "repaired")
	report, err := repairFrameTree(root, output, removeOnOpen(t))
	if err != nil {
		t.Fatalf("repair failed on a removed key: %v", err)
	}
	if !slices.Equal(report.Vanished, []string{"meta/a/two"}) {
		t.Fatalf("vanished %q, want [meta/a/two]", report.Vanished)
	}
	if !slices.Equal(report.Recovered, []string{"meta/a/one"}) {
		t.Fatalf("recovered %q, want [meta/a/one]", report.Recovered)
	}
	if report.SkippedCount != 1 {
		t.Fatalf("skipped %+v, want file/x", report.Skipped)
	}
	if report.Skipped[0].Key != "file/x" {
		t.Fatalf("skipped %+v, want file/x", report.Skipped)
	}
	if _, err := os.Lstat(filepath.Join(output, "meta", "a", "two")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("removed key in the output: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(output, "meta", "a", "one")); err != nil {
		t.Fatalf("recovered key missing from the output: %v", err)
	}
}
