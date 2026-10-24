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
