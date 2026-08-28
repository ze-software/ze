package weakened

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestProspectiveCommitPreservesSharedIndexAndPairsRenameWithSpaces(t *testing.T) {
	root := t.TempDir()
	oldPath := "old tools/shared/a test_test.go"
	newPath := "new tools/shared/a test_test.go"
	baseline := "package shared\nfunc TestA(t *testing.T) {\n\trequire.NoError(t, err)\n\trequire.Equal(t, 1, got)\n}\n"
	writeProspectiveFile(t, root, oldPath, baseline)
	writeProspectiveFile(t, root, ContractPath, fixtureLedgerHeader)
	writeProspectiveFile(t, root, "foreign.txt", "baseline\n")
	runProspectiveGit(t, root, "init", "-q")
	runProspectiveGit(t, root, "add", "-A")
	runProspectiveGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "baseline")

	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, filepath.FromSlash(newPath))), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, filepath.FromSlash(oldPath)), filepath.Join(root, filepath.FromSlash(newPath))); err != nil {
		t.Fatal(err)
	}
	writeProspectiveFile(t, root, newPath, strings.ReplaceAll(baseline, "\trequire.NoError(t, err)\n", ""))
	writeProspectiveFile(t, root, "foreign.txt", "staged by another session\n")
	runProspectiveGit(t, root, "add", "--", "foreign.txt")

	prospective, problems := ProspectiveCommit(root, []string{newPath}, []string{oldPath})
	if len(problems) != 0 {
		t.Fatalf("ProspectiveCommit problems = %q", problems)
	}
	if len(prospective.RenamePairs) != 1 {
		t.Fatalf("RenamePairs = %#v, want one pair", prospective.RenamePairs)
	}
	wantPair := RenamePair{OldPath: oldPath, NewPath: newPath, Score: prospective.RenamePairs[0].Score}
	if prospective.RenamePairs[0] != wantPair {
		t.Fatalf("RenamePairs = %#v, want %q -> %q", prospective.RenamePairs, oldPath, newPath)
	}
	staged := runProspectiveGitOutput(t, root, "diff", "--cached", "--name-only")
	if strings.TrimSpace(staged) != "foreign.txt" {
		t.Fatalf("shared index changed by prospective audit: %q", staged)
	}

	result := CheckCommit(Request{Root: root, Paths: []string{newPath}, Removed: []string{oldPath}, RenamePairs: prospective.RenamePairs}, false)
	if result.ExitCode() != 1 || len(result.Findings) != 1 || result.Findings[0].Path != newPath || result.Findings[0].Name != "TestA" {
		t.Fatalf("rename weakening = code %d, findings %#v, problems %q", result.ExitCode(), result.Findings, result.Problems)
	}
}

func TestProspectiveCommitIgnoresUnrelatedWorktreeTests(t *testing.T) {
	root := t.TempDir()
	baseline := "package p\nfunc TestA(t *testing.T) { require.Equal(t, 1, got) }\n"
	writeProspectiveFile(t, root, "pkg/a_test.go", baseline)
	writeProspectiveFile(t, root, "pkg/unrelated_test.go", baseline)
	writeProspectiveFile(t, root, "docs/note.txt", "old\n")
	writeProspectiveFile(t, root, ContractPath, fixtureLedgerHeader)
	runProspectiveGit(t, root, "init", "-q")
	runProspectiveGit(t, root, "add", "-A")
	runProspectiveGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "baseline")
	writeProspectiveFile(t, root, "pkg/unrelated_test.go", "package p\nfunc TestA(t *testing.T) { t.Skip() }\n")
	writeProspectiveFile(t, root, "docs/note.txt", "new\n")

	prospective, problems := ProspectiveCommit(root, []string{"docs/note.txt"}, nil)
	if len(problems) != 0 || len(prospective.RenamePairs) != 0 {
		t.Fatalf("ProspectiveCommit = %#v, %q", prospective, problems)
	}
	result := CheckCommit(Request{Root: root, Paths: prospective.Paths}, false)
	if result.ExitCode() != 0 || len(result.Findings) != 0 {
		t.Fatalf("unrelated worktree test entered population: %#v", result)
	}
}

func TestLowSimilarityRenamePairingFailsClosedOnAmbiguity(t *testing.T) {
	t.Parallel()
	paths := []string{"new-one/shared/a_test.go", "new-two/shared/a_test.go"}
	removed := []string{"old-one/shared/a_test.go", "old-two/shared/a_test.go"}
	pairs := []RenamePair{
		{OldPath: removed[0], NewPath: paths[0], Score: 1},
		{OldPath: removed[1], NewPath: paths[1], Score: 1},
	}
	if accepted, problem := acceptedRenamePairs(paths, removed, pairs); problem == "" || len(accepted) != 0 {
		t.Fatalf("ambiguous low-score pairs = %#v, %q", accepted, problem)
	}
}

func TestUniqueSuffixPairingLeavesRectangularExtrasUnpaired(t *testing.T) {
	t.Parallel()
	oldPaths := []string{"former/docvalid/dispatch.go", "cmd/le/dispatch.go"}
	newPaths := []string{"internal/le/docvalid/dispatch.go", "internal/le/dispatch.go", "internal/other/dispatch.go"}
	pairs, unique := uniqueSuffixPairing(oldPaths, newPaths)
	want := [][2]string{
		{"cmd/le/dispatch.go", "internal/le/dispatch.go"},
		{"former/docvalid/dispatch.go", "internal/le/docvalid/dispatch.go"},
	}
	if !unique || !slices.Equal(pairs, want) {
		t.Fatalf("uniqueSuffixPairing = %#v, %v; want %#v, true", pairs, unique, want)
	}
}

func writeProspectiveFile(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func runProspectiveGit(t *testing.T, root string, args ...string) {
	t.Helper()
	_ = runProspectiveGitOutput(t, root, args...)
}

func runProspectiveGitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return string(output)
}
