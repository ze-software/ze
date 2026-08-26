// Related: treehash.go -- the fingerprint these tests drive from its entry point
//
// VALIDATES: spec-le-is-a-ze-binary AC-11 for the tree hash of
// scripts/dev/ze-run.sh. A job records the tree that it judges. A second asker
// shares that job's verdict only when the two hashes match. Therefore, this
// implementation and the shell half must compute the same number for the same
// checkout. Otherwise, no job can attach across the migration.
// PREVENTS: a port that attaches with a different hash. Such a port would
// certify one session's commit with a run that never saw its code. The
// full_verify_coverage certificate in scripts/dev/commit_helper.py reads it.

package lejob

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestTreeHashMatchesTheShellOverAFixtureTree runs both implementations. The
// tree contains each hash input: a committed file, a changed committed file, an
// untracked file, and an ignored file.
func TestTreeHashMatchesTheShellOverAFixtureTree(t *testing.T) {
	repo := fixtureRepo(t)

	shell := shellTreeHash(t, repo)
	if got := TreeHash(repo); got != shell {
		t.Errorf("TreeHash = %s, the shell says %s", got, shell)
	}
}

// TestTreeHashSeesEveryChangeTheShellSees is the discrimination half: a hash
// that agreed with the shell by answering a constant would pass the test above.
func TestTreeHashSeesEveryChangeTheShellSees(t *testing.T) {
	repo := fixtureRepo(t)
	before := TreeHash(repo)

	steps := []struct {
		name string
		do   func()
	}{
		{"a tracked file changes", func() { write(t, repo, "tracked.txt", "moved") }},
		{"an untracked file appears", func() { write(t, repo, "fresh.txt", "new") }},
		{"an untracked file changes", func() { write(t, repo, "fresh.txt", "newer") }},
	}

	seen := map[string]bool{before: true}
	for _, step := range steps {
		step.do()
		got := TreeHash(repo)
		if seen[got] {
			t.Errorf("%s: the hash did not move", step.name)
		}
		seen[got] = true
		if shell := shellTreeHash(t, repo); got != shell {
			t.Errorf("%s: TreeHash = %s, the shell says %s", step.name, got, shell)
		}
	}
}

// TestTreeHashIgnoresWhatGitIgnores verifies the property required by the
// registry. The jobs directory is under tmp/, which is ignored. Therefore,
// writing an entry must not change the tree hash. If each admission changed
// the hash, two askers of one question would never match the same tree.
func TestTreeHashIgnoresWhatGitIgnores(t *testing.T) {
	repo := fixtureRepo(t)
	before := TreeHash(repo)

	if err := os.MkdirAll(filepath.Join(repo, JobsDir), 0o755); err != nil {
		t.Fatalf("make the jobs directory: %v", err)
	}
	write(t, repo, filepath.Join(JobsDir, "probe.job"), "LABEL=probe\n")

	if got := TreeHash(repo); got != before {
		t.Error("admitting a job moved the tree hash, so no two askers would ever attach")
	}
}

// TestTreeHashOutsideARepositoryIsStillAnAnswer pins what the shell does when
// git cannot answer: NO_HEAD stands in for the commit, and the hash is over
// that. A caller still gets a value, and it is the same value the shell gets.
func TestTreeHashOutsideARepositoryIsStillAnAnswer(t *testing.T) {
	dir := t.TempDir()
	if got := TreeHash(dir); got == "" || got == Unknown {
		t.Errorf("TreeHash outside a repository = %q, want a hash of the NO_HEAD stream", got)
	}
	if shell := shellTreeHash(t, dir); TreeHash(dir) != shell {
		t.Errorf("TreeHash = %s outside a repository, the shell says %s", TreeHash(dir), shell)
	}
}

// fixtureRepo builds a checkout holding one of each thing the hash is defined
// over, and answers its path.
func fixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	runGit(t, dir, "init", "--quiet")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "test")

	write(t, dir, ".gitignore", "tmp/\n")
	write(t, dir, "tracked.txt", "one")
	write(t, dir, "changed.txt", "two")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "--quiet", "-m", "fixture")

	write(t, dir, "changed.txt", "two, changed")
	write(t, dir, "untracked.txt", "three")
	write(t, dir, filepath.Join("tmp", "ignored.txt"), "four")
	return dir
}

// shellTreeHash answers what scripts/dev/verify-status.sh says about one tree.
// It is the other implementation, and the whole reason these tests exist.
func shellTreeHash(t *testing.T, dir string) string {
	t.Helper()
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "dev", "verify-status.sh"))
	if err != nil {
		t.Fatalf("locate verify-status.sh: %v", err)
	}
	cmd := exec.CommandContext(t.Context(), "bash", script, "tree_hash")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("verify-status.sh tree_hash in %s: %v", dir, err)
	}
	return string(trimNewline(out))
}

// git runs one git command in a fixture tree.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v: %s", args, dir, err, out)
	}
}

// write puts one file in a fixture tree, making its directory when it has to.
func write(t *testing.T, dir, rel, body string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("make %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
