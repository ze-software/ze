// Design: docs/architecture/testing/verify-freshness-scope.md -- abandoned artifact ownership.
// Related: cleanup.go -- the sweep must handle a marker without its directory.

package verify

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
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
