package gitenv

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// VALIDATES: under ReadOnly a `git status` over a stat-dirty file, and under
// ReadOnlyPatch the `git diff HEAD` golangci-lint's revgrep runs, leave
// .git/index byte-identical, while the same queries with the plain environment
// rewrite it.
// PREVENTS: the orphaned .git/index.lock. A query that rewrites the index holds
// its lock to do so, and a deadline that SIGKILLs it there leaves the lock
// behind, blocking every session's git writes. The plain-environment half
// proves the fixture is stat-dirty, so a green here is not a clean index.
func TestReadOnlyQueriesLeaveTheIndexUnwritten(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	for _, row := range []struct {
		name      string
		args      []string
		guarded   func([]string) []string
		unguarded bool
	}{
		{name: "status", args: []string{"status", "--porcelain"}, guarded: ReadOnly},
		{name: "diff HEAD", args: []string{"diff", "--color=never", "--no-ext-diff", "--relative", "HEAD", "--"}, guarded: ReadOnlyPatch},
		{name: "status, plain", args: []string{"status", "--porcelain"}, unguarded: true},
		{name: "diff HEAD, plain", args: []string{"diff", "--color=never", "--no-ext-diff", "--relative", "HEAD", "--"}, unguarded: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			tree := statDirtyTree(t)
			before := readIndex(t, tree)
			env := isolatedEnvironment(t)
			if !row.unguarded {
				env = row.guarded(env)
			}
			runGit(t, tree, env, row.args...)
			written := !bytes.Equal(before, readIndex(t, tree))
			if written != row.unguarded {
				t.Fatalf("git %v rewrote the index: %v, want %v", row.args, written, row.unguarded)
			}
		})
	}
}

// VALIDATES: ReadOnlyPatch numbers its config pair after any the caller already
// declared, and leaves a count git cannot parse for git to refuse.
// PREVENTS: overwriting a caller's GIT_CONFIG_KEY_0, or replacing its count so
// git silently ignores the caller's own settings.
func TestReadOnlyPatchKeepsTheCallersConfigPairs(t *testing.T) {
	got := ReadOnlyPatch([]string{"GIT_CONFIG_COUNT=2", "GIT_CONFIG_KEY_0=a.b", "GIT_CONFIG_VALUE_0=1"})
	for _, want := range []string{optionalLocksOff, "GIT_CONFIG_KEY_2=diff.autoRefreshIndex", "GIT_CONFIG_VALUE_2=false", "GIT_CONFIG_COUNT=3", "GIT_CONFIG_KEY_0=a.b"} {
		if !slices.Contains(got, want) {
			t.Errorf("ReadOnlyPatch lacks %q: %v", want, got)
		}
	}
	if last := got[len(got)-1]; last != "GIT_CONFIG_COUNT=3" {
		t.Errorf("the new count is not the last GIT_CONFIG_COUNT, so exec keeps the old one: %v", got)
	}
	bogus := ReadOnlyPatch([]string{"GIT_CONFIG_COUNT=many"})
	if !slices.Equal(bogus, []string{"GIT_CONFIG_COUNT=many", optionalLocksOff}) {
		t.Errorf("a count git cannot parse was rewritten: %v", bogus)
	}
	base := []string{"A=1"}
	_ = ReadOnly(base)
	if !slices.Equal(base, []string{"A=1"}) {
		t.Errorf("ReadOnly modified its input: %v", base)
	}
}

// statDirtyTree commits one file, then moves its mtime so the index's cached
// stat data no longer matches while the content still does.
func statDirtyTree(t *testing.T) string {
	t.Helper()
	tree := t.TempDir()
	env := isolatedEnvironment(t)
	runGit(t, tree, env, "init", "-q")
	file := filepath.Join(tree, "f")
	if err := os.WriteFile(file, []byte("a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, tree, env, "add", "f")
	runGit(t, tree, env, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "x")
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(file, later, later); err != nil {
		t.Fatal(err)
	}
	return tree
}

// isolatedEnvironment keeps the user's and the system's git config out of the
// fixture, so a setting on the developer's machine cannot decide the verdict.
func isolatedEnvironment(t *testing.T) []string {
	t.Helper()
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + t.TempDir(),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=" + os.DevNull,
	}
}

func readIndex(t *testing.T, tree string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(tree, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func runGit(t *testing.T, tree string, env []string, args ...string) {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", args...)
	command.Dir = tree
	command.Env = env
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
