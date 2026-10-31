// VALIDATES: session directories are named by creation time, owned through
// .session-id, found again after the clock moves, kept apart on a same-second
// collision, and the earlier <date>-<id> shape is found after its backfill.
package sessiondir

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

var created = time.Date(2026, 10, 30, 14, 23, 5, 0, time.Local)

func readOwner(t *testing.T, dir string) string {
	t.Helper()
	owner, err := Owner(dir)
	if err != nil {
		t.Fatalf("read the owner of %s: %v", dir, err)
	}
	return owner
}

// A first resolution creates <date>-<HHMMSS> and records the full id; the raw
// file holds the id and a newline, and nothing else.
func TestResolveCreatesATimeNamedDirectoryOwnedThroughItsIDFile(t *testing.T) {
	sessionRoot := filepath.Join(t.TempDir(), "tmp", "session")
	id := "5f56b642-de2f-4884-a472-6af29586b920"

	name, err := Resolve(sessionRoot, id, created, 0o750)
	if err != nil {
		t.Fatal(err)
	}
	if name != "2026-10-30-142305" {
		t.Fatalf("name = %q, want 2026-10-30-142305", name)
	}
	raw, err := os.ReadFile(filepath.Join(sessionRoot, name, IDFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != id+"\n" {
		t.Fatalf("%s = %q, want %q", IDFile, raw, id+"\n")
	}
	entries, err := os.ReadDir(filepath.Join(sessionRoot, name))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("the new directory holds %v, want only %s (no temporary file left)", entries, IDFile)
	}
}

// The second resolution after midnight answers the directory made the day
// before, and creates no other: the name is looked up, never recomputed.
func TestResolveFindsTheSameDirectoryAfterMidnight(t *testing.T) {
	sessionRoot := t.TempDir()
	first, err := Resolve(sessionRoot, "sid-one", created, 0o750)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Resolve(sessionRoot, "sid-one", created.Add(12*time.Hour), 0o750)
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatalf("second resolution = %q, want the first %q", second, first)
	}
	entries, err := os.ReadDir(sessionRoot)
	if err != nil {
		t.Fatal(err)
	}
	dirs := 0
	for _, entry := range entries {
		if entry.IsDir() {
			dirs++
		}
	}
	if dirs != 1 {
		t.Errorf("session root holds %d directories, want 1: %v", dirs, entries)
	}
}

// Three sessions created in one second get the base name, then -2, then -3,
// each owned by its own id.
func TestResolveSuffixesASameSecondCollision(t *testing.T) {
	sessionRoot := t.TempDir()
	want := map[string]string{
		"sid-a": "2026-10-30-142305",
		"sid-b": "2026-10-30-142305-2",
		"sid-c": "2026-10-30-142305-3",
	}
	for _, id := range []string{"sid-a", "sid-b", "sid-c"} {
		name, err := Resolve(sessionRoot, id, created, 0o750)
		if err != nil {
			t.Fatal(err)
		}
		if name != want[id] {
			t.Errorf("%s got %q, want %q", id, name, want[id])
		}
		if owner := readOwner(t, filepath.Join(sessionRoot, name)); owner != id {
			t.Errorf("%s owner = %q, want %q", name, owner, id)
		}
	}
}

// A directory in the earlier <date>-<id> shape with no .session-id is found by
// its own id, gets the file, and is never claimed by another id; a dated
// regular file and a directory owned by someone else are not answers.
func TestFindBackfillsTheEarlierShapeForItsOwnIDOnly(t *testing.T) {
	sessionRoot := t.TempDir()
	legacy := filepath.Join(sessionRoot, "2026-10-29-sid-old")
	if err := os.Mkdir(legacy, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessionRoot, "2026-10-28-sid-old"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, found, err := Find(sessionRoot, "sid"); err != nil || found {
		t.Fatalf("Find(sid) = found %v, err %v; want no directory", found, err)
	}
	if _, err := os.Stat(filepath.Join(legacy, IDFile)); !os.IsNotExist(err) {
		t.Fatalf("a lookup for another id wrote %s: %v", IDFile, err)
	}

	name, found, err := Find(sessionRoot, "sid-old")
	if err != nil {
		t.Fatal(err)
	}
	if !found || name != "2026-10-29-sid-old" {
		t.Fatalf("Find(sid-old) = %q, %v; want 2026-10-29-sid-old", name, found)
	}
	if owner := readOwner(t, legacy); owner != "sid-old" {
		t.Fatalf("backfilled owner = %q, want sid-old", owner)
	}

	name, err = Resolve(sessionRoot, "sid-old", created, 0o750)
	if err != nil {
		t.Fatal(err)
	}
	if name != "2026-10-29-sid-old" {
		t.Fatalf("Resolve after backfill = %q, want the earlier directory", name)
	}
}

// Dated accepts both shapes on disk and refuses anything else.
func TestDatedAcceptsBothShapes(t *testing.T) {
	for name, want := range map[string]bool{
		"2026-10-30-142305":   true,
		"2026-10-30-142305-2": true,
		"2026-10-30-5f56b642": true,
		"2026-10-30-":         false,
		"2026-10-3a-142305":   false,
		".session-x":          false,
		"scratch":             false,
	} {
		if got := Dated(name); got != want {
			t.Errorf("Dated(%q) = %v, want %v", name, got, want)
		}
	}
}
