// Design: docs/architecture/testing/tracked-build-gate.md -- the shared export materializes exactly what git holds.
// Goal: prove Commit writes the named commit's tree, byte for byte, and nothing
// the working tree or a later commit holds, and that it refuses what it cannot
// export.
// Method: build a two-commit repository with an uncommitted edit on top, export
// the first commit, and compare the exported tree with the files that commit
// recorded.
package export

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/repo/objects"
)

// gitIn runs git in repo with every user and system config shut out, so the
// fixture neither signs nor reads the developer's settings.
func gitIn(t *testing.T, repo string, args ...string) {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", append([]string{"-C", repo}, args...)...) //nolint:gosec // test-owned argv
	command.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid",
	)
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
}

// writeFiles writes each path to its content under root.
func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for path, content := range files {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// treeOf reads every regular file under root, keyed by its slash path.
func treeOf(t *testing.T, root string) map[string]string {
	t.Helper()
	found := map[string]string{}
	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		content, readErr := os.ReadFile(path) //nolint:gosec // the test's own export
		if readErr != nil {
			return readErr
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		found[filepath.ToSlash(relative)] = string(content)
		return nil
	})
	if walkErr != nil {
		t.Fatal(walkErr)
	}
	return found
}

func TestExportHelperMaterialisesCommit(t *testing.T) {
	repo := t.TempDir()
	gitIn(t, repo, "init", "-q")

	first := map[string]string{
		"go.mod":             "module example.invalid/fixture\n",
		"internal/a/a.go":    "package a\n",
		"vendor/modules.txt": "# vendored\n",
	}
	writeFiles(t, repo, first)
	gitIn(t, repo, "add", "-A")
	gitIn(t, repo, "commit", "-q", "-m", "first")

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	firstCommit, err := objects.Resolve(ctx, repo, "HEAD")
	if err != nil {
		t.Fatalf("resolve the first commit: %v", err)
	}

	// A later commit edits one file and deletes another, and the working tree
	// carries an uncommitted file: none of it may reach the export of the
	// first commit.
	writeFiles(t, repo, map[string]string{"internal/a/a.go": "package a // edited\n", "later.txt": "later\n"})
	gitIn(t, repo, "rm", "-q", "go.mod")
	gitIn(t, repo, "add", "-A")
	gitIn(t, repo, "commit", "-q", "-m", "second")
	writeFiles(t, repo, map[string]string{"uncommitted.txt": "working tree only\n"})

	dest := t.TempDir()
	if err := Commit(ctx, repo, firstCommit, dest); err != nil {
		t.Fatalf("export the first commit: %v", err)
	}
	got := treeOf(t, dest)
	if len(got) != len(first) {
		t.Errorf("the export holds %d files, want the %d the first commit recorded: %v", len(got), len(first), got)
	}
	for path, want := range first {
		if got[path] != want {
			t.Errorf("%s exported as %q, want %q", path, got[path], want)
		}
	}

	// The same directory now holds a tree, so a second export into it is
	// refused: tar would leave behind any file the second commit deleted.
	if err := Commit(ctx, repo, firstCommit, dest); err == nil {
		t.Error("an export into a non-empty directory was accepted")
	}
}

func TestExportHelperRefusesWhatItCannotExport(t *testing.T) {
	repo := t.TempDir()
	gitIn(t, repo, "init", "-q")
	writeFiles(t, repo, map[string]string{"a.txt": "a\n"})
	gitIn(t, repo, "add", "-A")
	gitIn(t, repo, "commit", "-q", "-m", "only")

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	cases := []struct {
		name     string
		revision string
	}{
		{"an unknown revision", "no-such-revision"},
		{"a revision git would read as an option", "--remote=elsewhere"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dest := t.TempDir()
			err := Commit(ctx, repo, tc.revision, dest)
			if err == nil {
				t.Fatalf("Commit(%q) exported something", tc.revision)
			}
			if !strings.Contains(err.Error(), tc.revision) {
				t.Errorf("the refusal does not name the revision %q: %v", tc.revision, err)
			}
			if left := treeOf(t, dest); len(left) != 0 {
				t.Errorf("a refused export left files behind: %v", left)
			}
		})
	}
}
