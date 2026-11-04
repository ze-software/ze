// Design: docs/contributing/running-commands.md -- a live owner excludes reclamation.
// Related: lifecycle.go -- ownership must be published before Git creates files.

package verify

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// TestReclaimPreservesWorktreeDuringAdd exercises the real cleanup producer
// while a fixture add owns an ordinary directory. The add boundary is controlled
// instead of creating or deleting a live Git worktree; it refuses immediately
// after observation, so no verification stage can run.
func TestReclaimPreservesWorktreeDuringAdd(t *testing.T) {
	root := newFixtureRepo(t).root
	deps := defaultDependencies()
	original := deps.git
	observed := false
	deps.git = func(ctx context.Context, budget time.Duration, directory string, args ...string) (commandResult, error) {
		if len(args) < 2 || args[0] != "worktree" || args[1] != "add" {
			return original(ctx, budget, directory, args...)
		}
		observed = true
		path := args[len(args)-2]
		if err := os.MkdirAll(path, 0o750); err != nil {
			t.Fatal(err)
		}
		marker, err := os.ReadFile(ownerMarker(path))
		if err != nil || string(marker) != strconv.Itoa(os.Getpid())+"\n" {
			t.Errorf("add has no published live owner: marker=%q err=%v", marker, err)
		}
		report := Reclaim(ctx, root)
		if report.Code != 0 {
			t.Errorf("reclamation during live add failed: %+v", report)
		}
		if _, err := os.Stat(path); err != nil {
			t.Errorf("reclamation deleted a live in-flight add: %v", err)
		}
		return commandResult{Code: 128, Output: "fixture stops after ownership observation"}, nil
	}
	report := run(t.Context(), root, Options{}, nil, deps)
	if !observed {
		t.Fatalf("fixture never reached add: %+v", report)
	}
	if report.Failure == nil || report.Failure.Kind != "worktree-add" {
		t.Fatalf("fixture did not stop at add refusal: %+v", report)
	}
	if report.Verify != nil {
		t.Error("ownership fixture started verification")
	}
	if report.Worktree != "" {
		if _, err := os.Stat(filepath.Dir(report.Worktree)); err != nil {
			t.Errorf("cleanup removed the artifact root: %v", err)
		}
	}
}
