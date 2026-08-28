// Related: treehash.go -- the fingerprint these tests drive from its entry point
//
// VALIDATES: the native tree hash includes the commit, tracked diff, and sorted
// untracked-file fingerprints required by job admission and verify certificates.
// PREVENTS: attachment to a run that never saw the asker's tree.

package job

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestTreeHashMatchesTheFixtureTranscript builds the specified byte stream
// independently from TreeHash and compares the resulting digest.
func TestTreeHashMatchesTheFixtureTranscript(t *testing.T) {
	repo := fixtureRepo(t)

	want := fixtureTreeHash(t, repo)
	if got := TreeHash(repo); got != want {
		t.Errorf("TreeHash = %s, fixture transcript says %s", got, want)
	}
}

// TestTreeHashSeesEveryFixtureChange is the discrimination half: a hash that
// answered a constant would pass no matter how its transcript was assembled.
func TestTreeHashSeesEveryFixtureChange(t *testing.T) {
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
		if want := fixtureTreeHash(t, repo); got != want {
			t.Errorf("%s: TreeHash = %s, fixture transcript says %s", step.name, got, want)
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

// TestTreeHashOutsideARepositoryIsStillAnAnswer pins the NO_HEAD stand-in.
func TestTreeHashOutsideARepositoryIsStillAnAnswer(t *testing.T) {
	dir := t.TempDir()
	wantBytes := sha256.Sum256([]byte("NO_HEAD\n"))
	want := hex.EncodeToString(wantBytes[:])
	if got := TreeHash(dir); got != want {
		t.Errorf("TreeHash outside a repository = %q, want %q", got, want)
	}
}

// TestDirtyManifestMatchesTheStateFileFixture pins the per-path half of the
// verification certificate. It covers changed, untracked, ignored, and deleted
// paths.
func TestDirtyManifestMatchesTheStateFileFixture(t *testing.T) {
	repo := fixtureRepo(t)
	manifest := DirtyManifest(repo)

	for _, rel := range []string{"changed.txt", "untracked.txt"} {
		want, err := fileHash(filepath.Join(repo, rel))
		if err != nil {
			t.Fatalf("hash fixture %s: %v", rel, err)
		}
		if got := manifest[rel]; got != want {
			t.Errorf("manifest[%q] = %q, want %q", rel, got, want)
		}
	}
	if _, exists := manifest[filepath.Join("tmp", "ignored.txt")]; exists {
		t.Error("the dirty manifest included an ignored path")
	}

	if err := os.Remove(filepath.Join(repo, "changed.txt")); err != nil {
		t.Fatal(err)
	}
	if got := DirtyManifest(repo)["changed.txt"]; got != "MISSING" {
		t.Errorf("deleted path fingerprint = %q, want MISSING", got)
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

// fixtureTreeHash assembles the specified hash transcript independently of the
// production helpers. Every Git command succeeds in fixtureRepo.
func fixtureTreeHash(t *testing.T, dir string) string {
	t.Helper()
	sum := sha256.New()
	for _, args := range [][]string{
		{"rev-parse", "HEAD"},
		{"diff", "HEAD"},
	} {
		if _, err := sum.Write(runGitOutput(t, dir, args...)); err != nil {
			t.Fatal(err)
		}
	}
	untracked := strings.Split(strings.TrimSuffix(string(runGitOutput(t, dir, "ls-files", "-o", "--exclude-standard")), "\n"), "\n")
	sort.Strings(untracked)
	for _, rel := range untracked {
		if rel == "" {
			continue
		}
		if _, err := sum.Write([]byte(rel + "\n")); err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		fingerprint := "MISSING"
		if err == nil {
			digest := sha256.Sum256(content)
			fingerprint = hex.EncodeToString(digest[:])
		}
		if _, err := sum.Write([]byte(fingerprint + "\n")); err != nil {
			t.Fatal(err)
		}
	}
	return hex.EncodeToString(sum.Sum(nil))
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

func runGitOutput(t *testing.T, dir string, args ...string) []byte {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v in %s: %v", args, dir, err)
	}
	return out
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
