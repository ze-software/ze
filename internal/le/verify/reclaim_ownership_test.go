// Design: docs/architecture/testing/verify-freshness-scope.md -- abandoned artifact ownership.
// Related: cleanup.go -- the sweep must handle a marker without its directory.

package verify

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// TestReclaimDirectorylessOwnerAndInterruptedRemoval uses a fixture Git repository
// and ordinary owned directories, never a live worktree. The orphan has already
// lost its Git registration, while the marker lost its directory first.
func TestReclaimDirectorylessOwnerAndInterruptedRemoval(t *testing.T) {
	root := newFixtureRepo(t).root
	base := filepath.Join(root, "tmp", "verify-worktree")
	orphan := filepath.Join(base, "interrupted")
	gone := filepath.Join(base, "directoryless")
	if err := os.MkdirAll(orphan, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{orphan, gone} {
		if err := os.WriteFile(ownerMarker(path), []byte("0\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(orphan, "leftover"), []byte("interrupted removal\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := sweepAbandoned(t.Context(), root, base, defaultDependencies())
	if len(got.Failures) != 0 {
		t.Fatalf("reclaim failed: %+v", got.Failures)
	}
	for _, path := range []string{orphan, ownerMarker(orphan), ownerMarker(gone)} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("owned abandoned artifact remains: %s: %v", path, err)
		}
	}
}

// TestReclaimRefusesUnknownOwnerAndPreservesLiveOwner exercises the real sweep
// with malformed and unreadable identity, and an owner that is this live test.
func TestReclaimRefusesUnknownOwnerAndPreservesLiveOwner(t *testing.T) {
	for _, owner := range []string{"foreign identity\n", "unreadable", strconv.Itoa(os.Getpid()) + "\n"} {
		t.Run(owner, func(t *testing.T) {
			root := newFixtureRepo(t).root
			path := filepath.Join(root, "tmp", "verify-worktree", "preserved")
			if err := os.MkdirAll(path, 0o750); err != nil {
				t.Fatal(err)
			}
			marker := ownerMarker(path)
			if owner == "unreadable" {
				if err := os.Mkdir(marker, 0o700); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(marker, []byte(owner), 0o600); err != nil {
				t.Fatal(err)
			}
			got := sweepAbandoned(context.Background(), root, filepath.Dir(path), defaultDependencies())
			if len(got.Removed) != 0 {
				t.Errorf("reclaimed unowned or live resource: %+v", got)
			}
			for _, keep := range []string{path, marker} {
				if _, err := os.Lstat(keep); err != nil {
					t.Errorf("resource did not survive: %s: %v", keep, err)
				}
			}
			if owner != strconv.Itoa(os.Getpid())+"\n" && len(got.Failures) == 0 {
				t.Errorf("unknown owner was not explicitly refused: %+v", got)
			}
		})
	}
}

// TestReclaimFinishesInterruptedRegisteredRemoval builds two real detached
// worktrees of a fixture repository with a dead owner marker, deletes every
// tracked file in both, and sweeps. The first is what a killed
// `git worktree remove` leaves: still registered, only ` D` and `D ` lines. The
// sweep must finish its removal, directory, registration and marker. The second
// also holds one untracked file, content no commit carries, so it must stay.
//
// MUTATION: delete the `ownerDead && onlyDeletions(...)` branch in
// sweepAbandoned and the first worktree is preserved as "holds uncommitted
// changes", so every assertion on it goes red. Let onlyDeletions accept `??`
// and the second worktree is removed, so its survival assertion goes red.
func TestReclaimFinishesInterruptedRegisteredRemoval(t *testing.T) {
	repo := newFixtureRepo(t)
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(repo.root, name), []byte(name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	repo.git(t, "add", "-A")
	repo.git(t, "commit", "-q", "-m", "more")
	base := filepath.Join(repo.root, "tmp", "verify-worktree")
	if err := os.MkdirAll(base, 0o750); err != nil {
		t.Fatal(err)
	}
	interrupted := filepath.Join(base, "interrupted")
	untracked := filepath.Join(base, "untracked")
	for _, path := range []string{interrupted, untracked} {
		repo.git(t, "worktree", "add", "--detach", path, "HEAD")
		for _, name := range []string{"tracked.txt", "a.txt", "b.txt"} {
			if err := os.Remove(filepath.Join(path, name)); err != nil {
				t.Fatal(err)
			}
		}
		// The index drops one path too, so the status carries a `D ` line
		// beside the ` D` lines.
		repo.git(t, "-C", path, "update-index", "--force-remove", "b.txt")
		if err := os.WriteFile(ownerMarker(path), []byte("0\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(untracked, "keep.txt"), []byte("only here\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := sweepAbandoned(t.Context(), repo.root, base, defaultDependencies())
	if len(got.Failures) != 0 {
		t.Fatalf("sweep failed: %+v", got.Failures)
	}
	for _, path := range []string{interrupted, ownerMarker(interrupted)} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("interrupted removal left %s behind: %v", path, err)
		}
	}
	registrations := repo.git(t, "worktree", "list", "--porcelain")
	if strings.Contains(registrations, interrupted) {
		t.Errorf("interrupted removal is still registered:\n%s", registrations)
	}
	if !slices.Contains(got.Removed, interrupted) {
		t.Errorf("removed = %v, want %s", got.Removed, interrupted)
	}
	if !strings.Contains(strings.Join(got.Diagnostics, "\n"), "finished interrupted removal of interrupted") {
		t.Errorf("no diagnostic names the finished removal: %v", got.Diagnostics)
	}
	for _, path := range []string{untracked, ownerMarker(untracked), filepath.Join(untracked, "keep.txt")} {
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("worktree holding an untracked file was not preserved: %s: %v", path, err)
		}
	}
	if !strings.Contains(registrations, untracked) {
		t.Errorf("preserved worktree lost its registration:\n%s", registrations)
	}
	if len(got.Preserved) != 1 || got.Preserved[0].Path != untracked || got.Preserved[0].Untracked != 1 {
		t.Errorf("preserved = %+v, want only %s with one untracked file", got.Preserved, untracked)
	}
}
