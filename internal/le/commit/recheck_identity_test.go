// Design: docs/contributing/committing.md -- the generated script's re-checks carry their identity in argv.
// Related: review_publication_runtime_test.go -- the native checker bridge, and the refusing half of this proof.
package commit

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// recheckWrongHelperExit is the exit code of the `le` planted where the
// generated script must not look, so a run that reached it cannot pass.
const recheckWrongHelperExit = 97

// TestGeneratedRecheckKeepsThePreparingIdentity is the commit-script half of
// AC-4 / P-4 of spec-tooling-one-off-defects: the review re-check the
// generated script runs at publication keeps the preparing session's identity,
// reaches the checkout's own `./le`, and receives its arguments with their
// boundaries intact.
//
// Method: prepare a closure under one session, its reviewed code at a path
// holding a space. Then switch the ambient session to another whose own review
// artifact for the same spec says findings, and run the script from a
// directory whose `./le`, and a `le` first on PATH, each record that they ran
// and fail. The checkout's `le` records the argv it receives, NUL-separated,
// and serves the real registered checker. The run must publish, so the
// checker judged the preparing session's clean artifact and not the ambient
// session's findings; the refusing half, a stale originating artifact beside
// a clean ambient one, is TestGeneratedReviewUsesOriginArtifact. Neither
// planted helper may run, and the recorded argv must hold the spaced path as
// one word, the originating artifact and the published tree.
//
// The build name the launcher selects (`./le --name`, ZE_LE_BUILD_NAME) is
// the launcher's half: TestLeLauncherCarriesTheBuildNameToNestedCalls and
// TestRefuseWrongBuildNameGuardsANamedSession in cmd/ze carry it.
//
// MUTATION: (prepare.go, `return reviewCheckCommand(stem, covered) + " " + keywordArtifact + " " +`, `return reviewCheckCommand(stem, covered) + " #" + keywordArtifact + " " +`): the artifact and tree become a shell comment, so the checker reads the ambient session's findings artifact and refuses the publication.
// MUTATION: (review.go, `words := []string{"./le", area, actionReviewCheck, "spec", stem}`, `words := []string{"le", area, actionReviewCheck, "spec", stem}`): the script runs the `le` first on PATH, the planted wrong helper, which fails the run and leaves its sentinel.
// MUTATION: (review.go, `return quotePaths(words)`, `return strings.Join(words, " ")`): the spaced code path splits into two arguments, so the recorded argv no longer holds it as one word.
func TestGeneratedRecheckKeepsThePreparingIdentity(t *testing.T) {
	root := newCommitRepository(t)
	configureCommitAuthor(t, root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "recheck-identity-origin")
	const code = "internal/reviewed file.sh"
	const specPath = "plan/release/fixes/spec-runtime-review.md"
	commitFixture(t, root, "reviewed work\n\nSpec-work: runtime-review", map[string]string{
		code: "#!/bin/sh\nprintf 'reviewed\\n'\n", specPath: "# Spec: runtime-review\n",
	}, nil)
	recordFixtureReview(t, root, "spec", "runtime-review", "verdict", "clean", "rounds", "1", "file", code)
	installNativeReviewChecker(t, root)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(t.TempDir(), "argv.record")
	writeCommitFixture(t, root, "le", "#!/bin/bash\nprintf '%s\\0' \"$@\" >> \"$ZE_TEST_RECHECK_ARGV\"\n"+
		"exec "+shellQuote(binary)+" -test.run '^TestGeneratedReviewNativeHelper$' -- \"${@:2}\"\n")
	if err := os.Chmod(filepath.Join(root, "le"), 0o700); err != nil { //nolint:gosec // executable native test bridge
		t.Fatal(err)
	}
	prepared, err := Create(root, &Options{Subject: "close recheck-identity spec", Remove: []string{specPath}})
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("CLAUDE_CODE_SESSION_ID", "recheck-identity-ambient")
	ambient := filepath.Join(root, "tmp/review/runtime-review-recheck-identity-ambient.md")
	if err := os.WriteFile(ambient, []byte("<!-- ze-review spec=runtime-review verdict=findings rounds=1 -->\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	elsewhere := t.TempDir()
	sentinel := filepath.Join(elsewhere, "wrong-helper-ran")
	wrong := "#!/bin/bash\n: > " + shellQuote(sentinel) + "\nexit " + strconv.Itoa(recheckWrongHelperExit) + "\n"
	pathDir := filepath.Join(elsewhere, "bin")
	for _, helper := range []string{filepath.Join(elsewhere, "le"), filepath.Join(pathDir, "le")} {
		if err := os.MkdirAll(filepath.Dir(helper), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(helper, []byte(wrong), 0o700); err != nil { //nolint:gosec // executable wrong-helper fixture
			t.Fatal(err)
		}
	}
	command := exec.CommandContext(t.Context(), "bash", filepath.Join(root, filepath.FromSlash(prepared.Script)))
	command.Dir = elsewhere
	command.Env = append(os.Environ(),
		"PATH="+pathDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ZE_TEST_RECHECK_ARGV="+record)
	output, err := command.CombinedOutput()
	if _, reached := os.Stat(sentinel); reached == nil {
		t.Fatalf("the script ran a le other than the checkout's own:\n%s", output)
	}
	if err != nil {
		t.Fatalf("the re-check refused a publication the preparing session reviewed clean: %v\n%s", err, output)
	}
	if status, _ := gitExit(root, "cat-file", "-e", "HEAD:"+specPath); status == 0 {
		t.Fatalf("the closure did not publish:\n%s", output)
	}
	recorded, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("the checkout's le never ran, so no re-check judged the publication: %v\n%s", err, output)
	}
	words := strings.Split(strings.TrimSuffix(string(recorded), "\x00"), "\x00")
	if len(words) < 4 || !slices.Equal(words[:4], []string{area, actionReviewCheck, keywordSpec, "runtime-review"}) {
		t.Fatalf("re-check argv = %q, want it to open with %s %s %s runtime-review", words, area, actionReviewCheck, keywordSpec)
	}
	if got := valueAfter(words, keywordFile); got != code {
		t.Fatalf("re-check file = %q, want %q as one argument (argv %q)", got, code, words)
	}
	if got := valueAfter(words, keywordArtifact); filepath.Base(got) != "runtime-review-recheck-identity-origin.md" {
		t.Fatalf("re-check artifact = %q, want the preparing session's (argv %q)", got, words)
	}
	tree := strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "HEAD^{tree}"))
	if got := valueAfter(words, keywordTree); got != tree {
		t.Fatalf("re-check tree = %q, want the published tree %s (argv %q)", got, tree, words)
	}
	if strings.Count(string(recorded), "\x00"+keywordSpec+"\x00") != 1 {
		t.Fatalf("the re-check ran more than once on one publication attempt: %q", words)
	}
}

// valueAfter answers the word after the first occurrence of keyword in words,
// and "" when keyword is absent or last, which no assertion above accepts.
func valueAfter(words []string, keyword string) string {
	index := slices.Index(words, keyword)
	if index < 0 || index+1 >= len(words) {
		return ""
	}
	return words[index+1]
}
