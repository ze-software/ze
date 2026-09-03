// VALIDATES: an abandoned worktree the sweep preserves is reported with its
// path, its age, the disk it holds, and the shape of its dirt, and that the
// size it reports excludes the shared build cache the extraction links in.
// PREVENTS: a machine filling with worktrees the gate preserved correctly and
// never named, which is what turns a right choice into 13.5 GB of tmp nobody
// can account for (plan/journal/full-disk-false-red.md).
package verify

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAPreservedWorktreeIsNamedWithItsAgeAndSize(t *testing.T) {
	repo := newFixtureRepo(t)
	path := filepath.Join(repo.root, "tmp", "verify-worktree", "preserved123")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	repo.git(t, "worktree", "add", "--detach", path, "HEAD")
	if err := os.WriteFile(ownerMarker(path), []byte("0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "untracked.txt"), []byte("keep me\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(path, "tracked.txt")); err != nil {
		t.Fatal(err)
	}

	report := Run(context.Background(), repo.root, Options{}, passingRunner)
	if report.Code != 0 {
		t.Fatalf("report = %#v", report)
	}
	if len(report.Preserved) != 1 {
		t.Fatalf("preserved = %#v, want the one worktree the sweep kept", report.Preserved)
	}
	kept := report.Preserved[0]
	if kept.Path != path || !kept.Measured {
		t.Fatalf("preserved = %#v, want %s measured", kept, path)
	}
	if kept.Untracked != 1 || kept.Deleted != 1 || kept.Modified != 0 {
		t.Fatalf("dirt = %d modified, %d untracked, %d deleted; want 0, 1, 1",
			kept.Modified, kept.Untracked, kept.Deleted)
	}
	if kept.SizeBytes == 0 || kept.SizeFloor {
		t.Fatalf("size = %d bytes (floor %v), want a complete non-zero measurement",
			kept.SizeBytes, kept.SizeFloor)
	}
	for _, want := range []string{
		"holds uncommitted changes, so it is left alone",
		path, " old, ", "0 modified, 1 untracked, 1 deleted",
	} {
		if !strings.Contains(report.Text(), want) {
			t.Fatalf("the report never named %q: %s", want, report.Text())
		}
	}
}

func TestAPreservedWorktreeSizeExcludesTheSharedCache(t *testing.T) {
	cacheHome := t.TempDir()
	shared := filepath.Join(cacheHome, "ze")
	if err := os.MkdirAll(shared, 0o750); err != nil {
		t.Fatal(err)
	}
	bulk := make([]byte, 1<<20)
	if err := os.WriteFile(filepath.Join(shared, "bulk"), bulk, 0o600); err != nil {
		t.Fatal(err)
	}
	worktree := t.TempDir()
	if err := os.WriteFile(filepath.Join(worktree, "own"), []byte("own\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(shared, filepath.Join(worktree, "cache")); err != nil {
		t.Fatal(err)
	}

	bytes, floor, err := directorySize(worktree)
	if err != nil {
		t.Fatal(err)
	}
	if floor {
		t.Fatal("the entry budget stopped a walk of two entries")
	}
	if bytes != 4 {
		t.Fatalf("size = %d bytes, want the 4 the worktree owns: the cache link is not followed", bytes)
	}
}

func TestAnUnmeasurableWorktreeSaysSoRatherThanReportingZero(t *testing.T) {
	kept := PreservedWorktree{Path: "/gone", Deleted: 3}
	line := preservedLine("gone", kept)
	if !strings.Contains(line, "age and size unknown") {
		t.Fatalf("line = %q, want it to say the measurement is absent", line)
	}
	if strings.Contains(line, "0s old") || strings.Contains(line, "0.0G") {
		t.Fatalf("line = %q, want no zero it never measured", line)
	}
}

func TestDirtIsCountedByShapeNotByLineCount(t *testing.T) {
	status := strings.Join([]string{
		"?? new.txt",
		" D gone.txt",
		"D  staged-delete.txt",
		" M edited.txt",
		"A  added.txt",
		"R  old.txt -> new-name.txt",
		"",
	}, "\n")
	var kept PreservedWorktree
	countDirt(&kept, status)
	if kept.Untracked != 1 || kept.Deleted != 2 || kept.Modified != 3 {
		t.Fatalf("dirt = %d modified, %d untracked, %d deleted; want 3, 1, 2",
			kept.Modified, kept.Untracked, kept.Deleted)
	}
}

func TestPreservedAgeRendersTheDurationTheSweepMeasured(t *testing.T) {
	kept := PreservedWorktree{Path: "/kept", Measured: true,
		AgeSeconds: int64((3*time.Hour + 12*time.Minute).Seconds()), SizeBytes: 8 << 30}
	line := preservedLine("kept", kept)
	if !strings.Contains(line, "3h12m0s old") {
		t.Fatalf("line = %q, want the age it measured", line)
	}
	if !strings.Contains(line, "8.0G") {
		t.Fatalf("line = %q, want the size it measured", line)
	}
}
