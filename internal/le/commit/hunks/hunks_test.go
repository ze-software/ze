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
	content, err := Selection{"a.txt": {Entry: entry, Patch: one}}.Read(root, "a.txt")
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

// TestApplyAdmitsPathWithSpace proves a path holding a space is selected as
// one path: git lists the names the patch changed NUL-terminated, and a split
// on whitespace would read "a b.txt" as the two names "a" and "b.txt" and
// refuse the patch for changing a path it never touched.
//
// VALIDATES: Apply answers the entry of "a b.txt" and Read answers its blob;
// changedProblem reads one NUL-terminated name as one path.
// PREVENTS: a false "also changes" refusal for a path with a space.
func TestApplyAdmitsPathWithSpace(t *testing.T) {
	root := t.TempDir()
	gitIn(t, root, "init", "-q")
	const path = "dir one/a b.txt"
	writeFile(t, root, path, "a\n")
	gitIn(t, root, "add", "--", path)
	gitIn(t, root, "commit", "-q", "-m", "base")
	head := strings.TrimSpace(gitIn(t, root, "rev-parse", "HEAD"))
	writeFile(t, root, path, "a\nmore\n")
	patch := filepath.Join(t.TempDir(), "space.patch")
	writeFile(t, filepath.Dir(patch), "space.patch", gitIn(t, root, "diff", "--", path))

	entry, err := Apply(root, head, path, patch)
	if err != nil {
		t.Fatalf("a patch over %q: %v, want its entry", path, err)
	}
	content, err := Selection{path: {Entry: entry, Patch: patch}}.Read(root, path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "a\nmore\n" {
		t.Fatalf("selected content %q, want the patched blob", content)
	}
	if problem := changedProblem(path, patch, path+"\x00"); problem != "" {
		t.Fatalf("one NUL-terminated name read as more than one path: %s", problem)
	}
	if problem := changedProblem(path, patch, path+"\x00b.txt\x00"); !strings.Contains(problem, "also changes b.txt") {
		t.Fatalf("a second NUL-terminated name: %q, want the refusal naming b.txt", problem)
	}
}

// TestSelectionRefusesMalformedEntry proves a selected path whose entry is not
// a `git ls-files -s` line is an error, never the working-tree file: the
// working tree holds hunks the commit does not carry, so answering it would
// judge content the commit never lands.
//
// VALIDATES: Selected reports the path, Blob and Read both answer an error
// naming the path, while the working-tree file is present and readable.
// PREVENTS: a gate silently judging the working tree for a selected path.
func TestSelectionRefusesMalformedEntry(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "a.txt", "working tree\n")
	for _, entry := range []string{"", "100644\ta.txt", "100644 abc 0\tb.txt", "garbage"} {
		selection := Selection{"a.txt": {Entry: entry, Patch: "a.patch"}}
		if !selection.Selected("a.txt") {
			t.Fatalf("entry %q: a.txt is not reported selected", entry)
		}
		if blob, err := selection.Blob("a.txt"); err == nil || !strings.Contains(err.Error(), "hunks a.txt") {
			t.Fatalf("entry %q: Blob answered %q, %v, want an error naming a.txt", entry, blob, err)
		}
		if content, err := selection.Read(root, "a.txt"); err == nil {
			t.Fatalf("entry %q: Read answered %q, want an error, never the working-tree file", entry, content)
		}
	}
	if _, err := (Selection{}).Blob("a.txt"); err == nil {
		t.Fatal("Blob of a path that is not selected answered a blob")
	}
}
