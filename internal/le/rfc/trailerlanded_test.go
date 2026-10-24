// Design: docs/contributing/rfc-implementation-guide.md -- owner approval for RFC-tagged test changes
// Related: approve.go -- TrailerLanded, the question whether an approval was already used.
package rfc

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestTrailerLandedAnswersOnlyWhatGitCanAnswer proves TrailerLanded answers
// "not landed" only where no commit can exist, and leaves the question open
// for every other git failure.
//
// The method asks one trailer of three roots: a directory no repository
// holds, a repository with no commit, and a directory whose `.git` file
// points at a gitdir that does not exist. Git refuses the last one with
// "fatal: not a git repository: /nonexistent", which shares its first words
// with the no-repository message and is not it.
//
// VALIDATES: the first two give no problem and landed=false; the third gives
// a problem naming the git failure and landed=false.
// PREVENTS: a broken repository read as one in which the approval never
// landed, which would let a spent approval admit a second change.
func TestTrailerLandedAnswersOnlyWhatGitCanAnswer(t *testing.T) {
	const trailer = "RFC-approved: pkg.TestX: Thomas approved the new count"

	outside := t.TempDir()
	// A temporary directory under the checkout would find the checkout, so
	// discovery stops at the directory's parent.
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(outside))

	empty := t.TempDir()
	initCommand := exec.CommandContext(context.Background(), "git", "init", "-q", empty)
	if output, err := initCommand.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}

	broken := t.TempDir()
	if err := os.WriteFile(filepath.Join(broken, ".git"), []byte("gitdir: /nonexistent\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, root := range []struct {
		name    string
		path    string
		problem bool
	}{
		{name: "no repository", path: outside, problem: false},
		{name: "no commit", path: empty, problem: false},
		{name: "gitdir missing", path: broken, problem: true},
	} {
		carrier, landed, problem := TrailerLanded(root.path, trailer)
		if landed || carrier != "" {
			t.Fatalf("%s: TrailerLanded = carrier %q, landed %v, want not landed", root.name, carrier, landed)
		}
		if root.problem && problem == "" {
			t.Fatalf("%s: TrailerLanded gave no problem, so a git failure read as not landed", root.name)
		}
		if !root.problem && problem != "" {
			t.Fatalf("%s: TrailerLanded gave problem %q, want none", root.name, problem)
		}
	}
}
