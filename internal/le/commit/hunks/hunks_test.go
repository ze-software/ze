package hunks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitIn runs git in root for a fixture and fails the test on error.
func gitIn(t *testing.T, root string, args ...string) string {
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

func writeFile(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestApplyRefusesAnotherPath proves AC-3: a patch named for one path that
// also changes another is refused, naming the other path, because the commit
// would carry content its keyword never named.
//
// VALIDATES: the refusal names the extra path; a single-path patch answers the
// entry line of that path's new blob, and Read answers that blob.
func TestApplyRefusesAnotherPath(t *testing.T) {
	root := t.TempDir()
	gitIn(t, root, "init", "-q")
	writeFile(t, root, "a.txt", "a\n")
	writeFile(t, root, "b.txt", "b\n")
	gitIn(t, root, "add", "--", "a.txt", "b.txt")
	gitIn(t, root, "commit", "-q", "-m", "base")
	head := strings.TrimSpace(gitIn(t, root, "rev-parse", "HEAD"))
	writeFile(t, root, "a.txt", "a\nmore\n")
	writeFile(t, root, "b.txt", "b\nmore\n")
	both := filepath.Join(t.TempDir(), "both.patch")
	writeFile(t, filepath.Dir(both), "both.patch", gitIn(t, root, "diff"))
	one := filepath.Join(t.TempDir(), "one.patch")
	writeFile(t, filepath.Dir(one), "one.patch", gitIn(t, root, "diff", "--", "a.txt"))

	_, err := Apply(root, head, "a.txt", both)
	if err == nil || !strings.Contains(err.Error(), "also changes b.txt") {
		t.Fatalf("a patch over two paths: %v, want a refusal naming b.txt", err)
	}
	entry, err := Apply(root, head, "a.txt", one)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(entry, "\ta.txt") {
		t.Fatalf("entry %q does not name a.txt", entry)
	}
	content, err := Selection{"a.txt": entry}.Read(root, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "a\nmore\n" {
		t.Fatalf("selected content %q, want the patched blob", content)
	}
	if staged := gitIn(t, root, "diff", "--cached", "--name-only"); staged != "" {
		t.Fatalf("Apply wrote the shared index: %q", staged)
	}
}

// applyRepository commits a.txt and a.bin and answers the root and HEAD.
func applyRepository(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	gitIn(t, root, "init", "-q")
	writeFile(t, root, "a.txt", "a\n")
	writeFile(t, root, "a.bin", "\x00\x01\x02")
	gitIn(t, root, "add", "--", "a.txt", "a.bin")
	gitIn(t, root, "commit", "-q", "-m", "base")
	return root, strings.TrimSpace(gitIn(t, root, "rev-parse", "HEAD"))
}

// TestApplyRefusals proves the refusals Apply owes before a blob is built:
// an empty patch, a patch deleting the path, a binary change and a path git
// must quote. Each refusal names the path and the step that fixes it.
//
// VALIDATES: the refusal text of each case.
// PREVENTS: an empty or deleting patch reaching the script, a binary path
// committed as hunks, and a quoted-path refusal with no fix.
func TestApplyRefusals(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		root, head := applyRepository(t)
		patch := filepath.Join(t.TempDir(), "empty.patch")
		writeFile(t, filepath.Dir(patch), "empty.patch", "")
		_, err := Apply(root, head, "a.txt", patch)
		if err == nil || !strings.Contains(err.Error(), "is empty") || !strings.Contains(err.Error(), "git diff -- a.txt") {
			t.Fatalf("an empty patch: %v, want the empty refusal naming the command that writes one", err)
		}
	})
	t.Run("deletion", func(t *testing.T) {
		root, head := applyRepository(t)
		if err := os.Remove(filepath.Join(root, "a.txt")); err != nil {
			t.Fatal(err)
		}
		patch := filepath.Join(t.TempDir(), "delete.patch")
		writeFile(t, filepath.Dir(patch), "delete.patch", gitIn(t, root, "diff", "--", "a.txt"))
		_, err := Apply(root, head, "a.txt", patch)
		if err == nil || !strings.Contains(err.Error(), "deletes a.txt") || !strings.Contains(err.Error(), "remove a.txt") {
			t.Fatalf("a deleting patch: %v, want the deletion refusal pointing at remove", err)
		}
	})
	t.Run("binary", func(t *testing.T) {
		root, head := applyRepository(t)
		writeFile(t, root, "a.bin", "\x00\x01\x03")
		patch := filepath.Join(t.TempDir(), "binary.patch")
		writeFile(t, filepath.Dir(patch), "binary.patch", gitIn(t, root, "diff", "--binary", "--", "a.bin"))
		_, err := Apply(root, head, "a.bin", patch)
		if err == nil || !strings.Contains(err.Error(), "binary") || !strings.Contains(err.Error(), "file a.bin") {
			t.Fatalf("a binary patch: %v, want the binary refusal pointing at file", err)
		}
	})
	t.Run("quoted", func(t *testing.T) {
		root, _ := applyRepository(t)
		const path = `a"b.txt`
		writeFile(t, root, path, "q\n")
		gitIn(t, root, "add", "--", path)
		gitIn(t, root, "commit", "-q", "-m", "quoted")
		head := strings.TrimSpace(gitIn(t, root, "rev-parse", "HEAD"))
		writeFile(t, root, path, "q\nmore\n")
		patch := filepath.Join(t.TempDir(), "quoted.patch")
		writeFile(t, filepath.Dir(patch), "quoted.patch", gitIn(t, root, "diff", "--", path))
		_, err := Apply(root, head, path, patch)
		if err == nil || !strings.Contains(err.Error(), "git quotes this path") || !strings.Contains(err.Error(), "Rename the file") {
			t.Fatalf("a path git quotes: %v, want the quoted-path refusal with its fix", err)
		}
	})
}
