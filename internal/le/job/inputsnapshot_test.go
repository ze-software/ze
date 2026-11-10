// Related: inputsnapshot.go -- the scoped input-change question these tests drive
//
// VALIDATES: ChangedInputs names a changed path only when the caller's
// population reaches it, and judges content rather than the commit, so another
// session's edit or commit outside that population changes nothing.
// PREVENTS: a scoped lint or a scoped verify PASS refusing for an edit it never
// reads, and either accepting a state whose own inputs moved under it.

package job

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// reachesOnly answers a population holding exactly the named paths.
func reachesOnly(paths ...string) func(string) bool {
	return func(rel string) bool { return slices.Contains(paths, rel) }
}

func fixtureSnapshot(t *testing.T, root string) InputSnapshot {
	t.Helper()
	snapshot, err := SnapshotInputs(root, LintLabel)
	if err != nil {
		t.Fatalf("SnapshotInputs: %v", err)
	}
	return snapshot
}

func fixtureChanged(t *testing.T, root string, start InputSnapshot, reaches func(string) bool) []InputChange {
	t.Helper()
	changed, err := ChangedInputs(root, start, reaches)
	if err != nil {
		t.Fatalf("ChangedInputs: %v", err)
	}
	return changed
}

// TestChangedInputsJudgesOnlyTheReachedPaths edits one path the population
// reaches and three it does not: a tracked file, a new file and an ignored tree.
// Only the reached path is named.
func TestChangedInputsJudgesOnlyTheReachedPaths(t *testing.T) {
	root := fixtureRepo(t)
	start := fixtureSnapshot(t, root)

	write(t, root, "tracked.txt", "one, edited by this population")
	write(t, root, "changed.txt", "two, edited by another session")
	write(t, root, "elsewhere/new.go", "package elsewhere\n")
	write(t, root, "docs/page.md", "a page")

	got := fixtureChanged(t, root, start, reachesOnly("tracked.txt"))
	want := []InputChange{{Path: "tracked.txt", Kind: ChangedDirty}}
	if !slices.Equal(got, want) {
		t.Fatalf("changed = %v, want only the reached tracked.txt %v", got, want)
	}
	if got := fixtureChanged(t, root, start, reachesOnly("unrelated.go")); len(got) != 0 {
		t.Fatalf("an unreached population saw changes %v", got)
	}
}

// TestChangedInputsSeesEveryKindOfReachedChange covers each way a reached
// input moves, and the kind each one reports: a clean file edited, a dirty file
// edited again, a new file, a deletion, a clean file changed and committed so
// that it is clean again, and a non-ASCII name, which Git quotes unless asked
// for -z output.
func TestChangedInputsSeesEveryKindOfReachedChange(t *testing.T) {
	changes := []struct {
		name   string
		path   string
		kind   ChangeKind
		change func(*testing.T, string)
	}{
		{"clean-edited", "tracked.txt", ChangedDirty, func(t *testing.T, root string) {
			write(t, root, "tracked.txt", "one, edited")
		}},
		{"dirty-edited", "changed.txt", ChangedContent, func(t *testing.T, root string) {
			write(t, root, "changed.txt", "two, edited twice")
		}},
		{"created", "fresh.go", ChangedDirty, func(t *testing.T, root string) {
			write(t, root, "fresh.go", "package fresh\n")
		}},
		{"deleted", "tracked.txt", ChangedDirty, func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, "tracked.txt")); err != nil {
				t.Fatalf("delete: %v", err)
			}
		}},
		{"edited-and-committed", "tracked.txt", ChangedCommitted, func(t *testing.T, root string) {
			write(t, root, "tracked.txt", "one, committed")
			runGit(t, root, "add", "tracked.txt")
			runGit(t, root, "commit", "--quiet", "-m", "edit tracked")
		}},
		{"non-ascii-created", "naïve.go", ChangedDirty, func(t *testing.T, root string) {
			write(t, root, "naïve.go", "package naive\n")
		}},
		{"non-ascii-committed", "naïve.go", ChangedCommitted, func(t *testing.T, root string) {
			write(t, root, "naïve.go", "package naive\n")
			runGit(t, root, "add", "naïve.go")
			runGit(t, root, "commit", "--quiet", "-m", "add a non-ASCII name")
		}},
	}
	for _, change := range changes {
		t.Run(change.name, func(t *testing.T) {
			root := fixtureRepo(t)
			start := fixtureSnapshot(t, root)
			change.change(t, root)
			got := fixtureChanged(t, root, start, reachesOnly(change.path))
			want := []InputChange{{Path: change.path, Kind: change.kind}}
			if !slices.Equal(got, want) {
				t.Fatalf("changed = %v, want %v", got, want)
			}
		})
	}
}

// TestChangedInputsIgnoresACommitThatKeepsTheContent moves HEAD the way another
// session's commit does: once by committing a reached dirty file unchanged, and
// once by committing an unrelated file. The content the population reads is
// the same, so nothing changed; the whole-tree hash called both a change.
func TestChangedInputsIgnoresACommitThatKeepsTheContent(t *testing.T) {
	root := fixtureRepo(t)
	start := fixtureSnapshot(t, root)
	reaches := reachesOnly("changed.txt", "tracked.txt")

	runGit(t, root, "add", "changed.txt")
	runGit(t, root, "commit", "--quiet", "-m", "absorb the dirty reached file")
	write(t, root, "plan/spec.md", "a spec")
	write(t, root, "other.go", "package other\n")
	runGit(t, root, "add", "plan/spec.md", "other.go")
	runGit(t, root, "commit", "--quiet", "-m", "another session's commit")

	if got := fixtureChanged(t, root, start, reaches); len(got) != 0 {
		t.Fatalf("a commit that kept the reached content reported %v", got)
	}
}

// TestChangedInputsRefusesAnUnmeasuredStart proves a zero snapshot is not an
// empty measurement that matches any tree.
func TestChangedInputsRefusesAnUnmeasuredStart(t *testing.T) {
	root := fixtureRepo(t)
	if changed, err := ChangedInputs(root, InputSnapshot{}, reachesOnly("tracked.txt")); err == nil {
		t.Fatalf("an unmeasured start answered %v with no error", changed)
	}
}

// TestSnapshotInputsRefusesMeasurementErrors keeps the fail-closed contract
// InputHash has, for the population that reads the path: no repository is an
// error, and an unreadable input refuses a caller that reaches it. A caller
// that never reads that path is not refused by it.
func TestSnapshotInputsRefusesMeasurementErrors(t *testing.T) {
	if _, err := SnapshotInputs(t.TempDir(), LintLabel); err == nil {
		t.Fatal("a directory outside any repository produced a snapshot")
	}
	root := fixtureRepo(t)
	if err := os.Symlink("loop.go", filepath.Join(root, "loop.go")); err != nil {
		t.Fatalf("make unreadable input: %v", err)
	}
	start := fixtureSnapshot(t, root)
	_, err := ChangedInputs(root, start, reachesOnly("loop.go"))
	if err == nil {
		t.Fatal("an unreadable reached input produced an answer")
	}
	if !strings.Contains(err.Error(), "loop.go") {
		t.Fatalf("unreadable input error does not name the path: %v", err)
	}
	if got := fixtureChanged(t, root, start, reachesOnly("tracked.txt")); len(got) != 0 {
		t.Fatalf("an unreadable path nobody reads reported %v", got)
	}
}
