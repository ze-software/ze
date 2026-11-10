// Design: docs/contributing/spec-workflow.md -- native generated publication checks immutable review evidence.
package commit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// TestGeneratedReviewNativeHelper serves the real registered commit checker in
// a subprocess, so a generated script never substitutes an echo for its gate.
//
// test-asserts-nothing: subprocess entry point; the tests that install it with
// installNativeReviewChecker assert its exit status and output.
func TestGeneratedReviewNativeHelper(t *testing.T) {
	if os.Getenv("ZE_TEST_NATIVE_REVIEW") != "1" {
		return
	}
	for index, word := range os.Args {
		if word != "--" {
			continue
		}
		env.ResetCache()
		answer, code := Answer(os.Args[index+1:])
		if rendered, ok := answer.(interface{ Text() string }); ok {
			_, _ = os.Stdout.WriteString(rendered.Text()) //nolint:errcheck // subprocess diagnostic output
		}
		os.Exit(code)
	}
	os.Exit(2)
}

func installNativeReviewChecker(t *testing.T, root string) {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	writeCommitFixture(t, root, "le", "#!/bin/bash\nexec "+shellQuote(binary)+" -test.run '^TestGeneratedReviewNativeHelper$' -- \"${@:2}\"\n")
	if err := os.Chmod(filepath.Join(root, "le"), 0o700); err != nil { //nolint:gosec // executable native test bridge
		t.Fatal(err)
	}
	t.Setenv("ZE_TEST_NATIVE_REVIEW", "1")
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

func runtimeReviewFixture(t *testing.T) (string, string, string) {
	t.Helper()
	root := newCommitRepository(t)
	configureCommitAuthor(t, root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "runtime-review-origin")
	const code = "internal/reviewed.sh"
	const spec = "plan/release/fixes/spec-runtime-review.md"
	commitFixture(t, root, "reviewed work\n\nSpec-work: runtime-review", map[string]string{
		code: "#!/bin/sh\nprintf 'reviewed\\n'\n", spec: "# Spec: runtime-review\n",
	}, nil)
	recordFixtureReview(t, root, "spec", "runtime-review", "verdict", "clean", "rounds", "1", "file", code)
	installNativeReviewChecker(t, root)
	return root, code, spec
}

// TestGeneratedReviewUsesOriginArtifact refuses when the originating artifact
// goes stale even if a different ambient session has a clean substitute.
func TestGeneratedReviewUsesOriginArtifact(t *testing.T) {
	root, code, spec := runtimeReviewFixture(t)
	prepared, err := Create(root, &Options{Subject: "close origin-reviewed spec", Remove: []string{spec}, NoNews: "fixture: no operator-visible change"})
	if err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "HEAD"))
	t.Setenv("CLAUDE_CODE_SESSION_ID", "runtime-review-substitute")
	recordFixtureReview(t, root, "spec", "runtime-review", "verdict", "clean", "rounds", "1", "file", code)
	origin := filepath.Join(root, "tmp", "review", "runtime-review-runtime-review-origin.md")
	if err := os.WriteFile(origin, []byte("<!-- ze-review spec=runtime-review verdict=findings rounds=1 -->\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), "bash", filepath.Join(root, prepared.Script))
	command.Dir = root
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("ambient clean artifact substituted for refused originating review:\n%s", output)
	}
	if got := strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "HEAD")); got != head {
		t.Fatalf("review refusal moved HEAD: %s", output)
	}
}

// TestGeneratedReviewRechecksEveryCASAttempt lands a peer commit only at the
// first update-ref. The next prospective tree contains unreviewed historical
// bytes, which the real checker must refuse before a second publication.
func TestGeneratedReviewRechecksEveryCASAttempt(t *testing.T) {
	root, code, spec := runtimeReviewFixture(t)
	prepared, err := Create(root, &Options{Subject: "close CAS-reviewed spec", Remove: []string{spec}, NoNews: "fixture: no operator-visible change"})
	if err != nil {
		t.Fatal(err)
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	wrapper := "#!/bin/bash\nif [ \"${1:-}\" = update-ref ] && [ ! -e \"$ZE_TEST_CAS_DONE\" ]; then\n  : > \"$ZE_TEST_CAS_DONE\"\n  ( unset GIT_INDEX_FILE; \"$ZE_TEST_REAL_GIT\" read-tree HEAD; printf '%s' 'peer unreviewed bytes' > " + shellQuote(code) + "; \"$ZE_TEST_REAL_GIT\" add -- " + shellQuote(code) + "; \"$ZE_TEST_REAL_GIT\" -c commit.gpgsign=false commit -q -m 'peer review change' )\nfi\nexec \"$ZE_TEST_REAL_GIT\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0o700); err != nil { //nolint:gosec // executable Git forwarding fixture
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), "bash", filepath.Join(root, prepared.Script))
	command.Dir = root
	marker := filepath.Join(bin, "attempted")
	command.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "ZE_TEST_REAL_GIT="+realGit, "ZE_TEST_CAS_DONE="+marker)
	output, err := command.CombinedOutput()
	if _, reached := os.Stat(marker); reached != nil {
		t.Fatalf("first CAS never reached, so retry proof is absent: %v\n%s", reached, output)
	}
	if err == nil {
		t.Fatalf("retry published an unreviewed prospective tree:\n%s", output)
	}
	if got := strings.TrimSpace(runCommitGitOutput(t, root, "log", "-1", "--format=%s")); got != "peer review change" {
		t.Fatalf("review refusal did not leave the peer's HEAD: %q\n%s", got, output)
	}
}

// TestExplicitReviewBlobRemovalAndMissingObject uses recorder sources whose
// bytes differ from the checkout, publishes their deletion, and refuses a lost
// selected blob without moving HEAD.
func TestExplicitReviewBlobRemovalAndMissingObject(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(map[bool]string{false: "removal", true: "missing blob"}[missing], func(t *testing.T) {
			root, code, spec := runtimeReviewFixture(t)
			options := &Options{Subject: "close explicit-source review", Remove: []string{spec, code}, NoNews: "fixture: no operator-visible change"}
			args := []string{"spec", "runtime-review", "verdict", "clean", "rounds", "1", "remove", code}
			blob := ""
			if missing {
				writeCommitFixture(t, root, code, "#!/bin/sh\nprintf 'new selected bytes\\n'\n")
				blob = strings.TrimSpace(runCommitGitOutput(t, root, "hash-object", "-w", "--", code))
				args = []string{"spec", "runtime-review", "verdict", "clean", "rounds", "1", "blob", code, blob, "mode", code, "100644"}
				options.Files = []string{code}
				options.Remove = []string{spec}
			}
			recordFixtureReview(t, root, args...)
			prepared, err := Create(root, options)
			if err != nil {
				t.Fatal(err)
			}
			head := strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "HEAD"))
			if missing {
				if err := os.Remove(filepath.Join(root, ".git", "objects", blob[:2], blob[2:])); err != nil {
					t.Fatal(err)
				}
			}
			command := exec.CommandContext(t.Context(), "bash", filepath.Join(root, prepared.Script))
			command.Dir = root
			output, err := command.CombinedOutput()
			if missing {
				if err == nil {
					t.Fatalf("missing selected object published:\n%s", output)
				}
				if got := strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "HEAD")); got != head {
					t.Fatalf("missing-object refusal moved HEAD: %s", output)
				}
				return
			}
			if err != nil {
				t.Fatalf("reviewed deletion failed: %v\n%s", err, output)
			}
			if status, _ := gitExit(root, "cat-file", "-e", "HEAD:"+code); status == 0 {
				t.Fatal("explicitly reviewed removal retained the code entry")
			}
		})
	}
}
