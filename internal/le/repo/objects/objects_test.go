// Design: docs/contributing/spec-workflow.md -- immutable committed object reads.
package objects

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureGit(t *testing.T, root, input string, args ...string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", append([]string{"-C", root}, args...)...)
	command.Stdin = strings.NewReader(input)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

// TestResolvedObjectsSurviveDirtyFilesAndHEADMovement pins a revision, advances
// HEAD and dirties the file, then reads the original blob twice in one batch.
func TestResolvedObjectsSurviveDirtyFilesAndHEADMovement(t *testing.T) {
	root := t.TempDir()
	fixtureGit(t, root, "", "init", "-q", "--initial-branch=main")
	blob := fixtureGit(t, root, "committed\x00bytes\n", "hash-object", "-w", "--stdin")
	tree := fixtureGit(t, root, "100644 blob "+blob+"\tsource.go\n", "mktree")
	commit := fixtureGit(t, root, "initial\n", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit-tree", tree)
	fixtureGit(t, root, "", "update-ref", "refs/heads/main", commit)
	resolved, err := Resolve(t.Context(), root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	other := fixtureGit(t, root, "changed\n", "hash-object", "-w", "--stdin")
	newTree := fixtureGit(t, root, "100644 blob "+other+"\tsource.go\n", "mktree")
	newCommit := fixtureGit(t, root, "later\n", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit-tree", newTree, "-p", commit)
	fixtureGit(t, root, "", "update-ref", "refs/heads/main", newCommit)
	if err := os.WriteFile(filepath.Join(root, "source.go"), []byte("dirty"), 0o600); err != nil {
		t.Fatal(err)
	}
	if resolved != commit {
		t.Fatalf("resolved %q, want pinned %q", resolved, commit)
	}
	blobs, err := Read(t.Context(), root, []string{blob, blob})
	if err != nil {
		t.Fatal(err)
	}
	if len(blobs) != 2 {
		t.Fatalf("batch returned %d blobs", len(blobs))
	}
	for _, data := range blobs {
		if string(data) != "committed\x00bytes\n" {
			t.Fatalf("mixed pinned blob with newer content: %q", data)
		}
	}
}

// TestObjectReaderRefusesMissingAndNonBlobObjects proves neither missing object
// nor a commit can be interpreted as an empty source blob.
func TestObjectReaderRefusesMissingAndNonBlobObjects(t *testing.T) {
	root := t.TempDir()
	fixtureGit(t, root, "", "init", "-q", "--initial-branch=main")
	tree := fixtureGit(t, root, "", "mktree")
	for _, object := range []string{strings.Repeat("1", 40), tree, "HEAD\nHEAD", "-options"} {
		if blobs, err := Read(t.Context(), root, []string{object}); err == nil {
			t.Fatalf("invalid source object %q answered %q", object, blobs)
		}
	}
	for _, revision := range []string{"missing-history", "-options"} {
		if resolved, err := Resolve(t.Context(), root, revision); err == nil {
			t.Fatalf("invalid revision %q resolved to %q", revision, resolved)
		}
	}
}
