package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// reviewGit runs git in root for a fixture and fails the test on error.
func reviewGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", append([]string{
		"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
	}, args...)...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return string(output)
}

// TestRecordReviewHunksPinsSelectedContent proves a review recorded with
// `hunks <path> <patch>` pins HEAD's blob with the patch applied, the content
// a hunks commit carries, and not the working-tree file holding a hunk the
// commit leaves out.
//
// VALIDATES: the recorded hash is the digest of the selected content.
// PREVENTS: a pin the commit gate can never match for a hunks code file.
func TestRecordReviewHunksPinsSelectedContent(t *testing.T) {
	root := t.TempDir()
	reviewGit(t, root, "init", "-q")
	const base = "package a\n\n// 1\n// 2\n// 3\n// 4\n// 5\n// 6\n// 7\n// 8\n// 9\n"
	mine := base + "// mine\n"
	both := "// theirs\n" + mine
	write := func(content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "pkg", "a.go"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(base)
	reviewGit(t, root, "add", "--", "pkg/a.go")
	reviewGit(t, root, "commit", "-q", "-m", "base")
	write(mine)
	patch := filepath.Join(t.TempDir(), "mine.patch")
	if err := os.WriteFile(patch, []byte(reviewGit(t, root, "diff", "--", "pkg/a.go")), 0o600); err != nil {
		t.Fatal(err)
	}
	write(both)

	artifact, err := recordReview(root, reviewRecord{
		Spec: "demo", Verdict: "clean", Rounds: 1, Model: "claude-opus-5", SessionID: "session",
		Hunks: []ReviewHunk{{Path: "pkg/a.go", Patch: patch}},
	})
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(mine))
	if len(artifact.Files) != 1 || artifact.Files[0].Path != "pkg/a.go" ||
		artifact.Files[0].Hash != hex.EncodeToString(digest[:]) {
		t.Fatalf("recorded %+v, want pkg/a.go pinned to the selected content", artifact.Files)
	}
}
