// Design: docs/contributing/committing.md -- prepared push consumption and red-index independence.
package commit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/le/human/humantest"
)

// TestRunnerGoCreateIgnoresRecordedReds covers a Go carrier through callable
// Create with related, blind, unreached and malformed failure indexes. It
// retains the commit package's ordinary check composition; it does not clear
// registered checks or exempt Go from a composed producer's inventory floor.
func TestRunnerGoCreateIgnoresRecordedReds(t *testing.T) {
	for _, tc := range []struct{ name, index string }{
		{"related Go path", `{"stages":[{"stage":"doc wiring","exit-code":1,"groups":[{"group-id":"files:mine","kind":"files","related":["mine.go"],"summary":"a Design reference does not resolve","rerun":"le doc wiring"}]}]}`},
		{"blind red", `{"stages":[{"stage":"doc wiring","exit-code":1,"groups":[{"group-id":"subcheck:ci-sleep-ratchet","kind":"subcheck","related":[],"summary":"functional fixtures exceed the wait-call ceiling","rerun":"le doc wiring"}]}]}`},
		{"no related path and unreached stage", `{"stopped-at":"rfc check","stages":[{"stage":"arch tier/check","exit-code":1,"groups":[{"group-id":"tier:unknown","kind":"subcheck","related":[]}]},{"stage":"doc wiring","reached":false,"exit-code":0}]}`},
		{"unreached stage", `{"stopped-at":"rfc check","stages":[{"stage":"rfc check","reached":true,"exit-code":1,"groups":[]},{"stage":"doc wiring","reached":false,"exit-code":0}]}`},
		{"malformed index", `{"stages":[`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := newCommitRepository(t)
			t.Setenv("CLAUDE_CODE_SESSION_ID", "runner-go-red-index")
			commitOwed(t, root, "Go carrier baseline", map[string]string{
				"docs/architecture/fixture.md": "# Fixture\n",
				"mine.go":                      "// Design: docs/architecture/fixture.md -- fixture\npackage mine\n",
			}, nil)
			writeCommitFixture(t, root, "mine.go", "// Design: docs/architecture/fixture.md -- fixture\npackage mine\n\nfunc Mine() {}\n")
			if got := strings.TrimSpace(runCommitGitOutput(t, root, "diff", "--name-only", "--", "mine.go")); got != "mine.go" {
				t.Fatalf("Go carrier is not a modification of its tracked baseline: %q", got)
			}
			writeCommitFixture(t, root, "tmp/ze-verify-failures.json", tc.index)
			prepared, err := Create(root, &Options{Subject: "Go change over a recorded red", Files: []string{"mine.go"},
				NoTest: "mine.go is a scratch-repo fixture for the commit gate, not product code", DryRun: true})
			if err != nil {
				t.Fatalf("Create refused the Go carrier: %v", err)
			}
			if len(prepared.Added) != 1 {
				t.Fatalf("prepared paths %q, want only mine.go", prepared.Added)
			}
			if prepared.Added[0] != "mine.go" {
				t.Fatalf("prepared paths %q, want mine.go", prepared.Added)
			}
			for _, unwanted := range []string{"-red-ok:", "Broken-head-fix:"} {
				if strings.Contains(prepared.MessageText, unwanted) {
					t.Fatalf("message records admission %q:\n%s", unwanted, prepared.MessageText)
				}
			}
			if strings.Contains(prepared.Verify.Detail, "ze-verify-failures") {
				t.Fatalf("verify detail reads the failure index: %q", prepared.Verify.Detail)
			}
		})
	}
}

// TestRunnerPreparedPushPublishesOnlyPreparedCommit executes the generated
// script against a private bare origin after HEAD moves. Its subprocess runs
// the real Answer debt checker, recording the exact target it rechecks.
func TestRunnerPreparedPushPublishesOnlyPreparedCommit(t *testing.T) {
	humantest.Streams(t, "terminal")
	root := newOwedRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "runner-prepared-push")
	prepared := commitOwed(t, root, "prepared change", map[string]string{"a.txt": "prepared\n"}, nil)
	push, code, err := preparePush(root, pushTestAuthorisation)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("prepare push: code %d, %v", code, err)
	}
	later := commitOwed(t, root, "later local change", map[string]string{"b.txt": "later\n"}, nil)
	capture := installRunnerDebtChecker(t, root)
	runCommitScript(t, root, push.Script)
	assertRunnerDebtRecheck(t, capture, prepared)
	if got := strings.TrimSpace(runCommitGitOutput(t, root, "ls-remote", "origin", "refs/heads/main")); got != prepared+"\trefs/heads/main" {
		t.Fatalf("origin contains %q, want prepared commit %s", got, prepared)
	}
	if got := revParse(t, root, "HEAD"); got != later {
		t.Fatalf("script changed local HEAD to %s, want later commit %s", got, later)
	}
}

// TestRunnerPreparedPushRefusesMovedUpstream requires the real debt recheck
// to refuse before git push when a fetched private origin diverges from P.
func TestRunnerPreparedPushRefusesMovedUpstream(t *testing.T) {
	humantest.Streams(t, "terminal")
	root := newOwedRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "runner-diverged-push")
	prepared := commitOwed(t, root, "prepared change", map[string]string{"a.txt": "prepared\n"}, nil)
	push, code, err := preparePush(root, pushTestAuthorisation)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("prepare push: code %d, %v", code, err)
	}
	pushFromOtherMachine(t, root, "upstream change", "remote.txt")
	runCommitGit(t, root, "fetch", "-q", "origin")
	upstream := strings.TrimSpace(runCommitGitOutput(t, root, "ls-remote", "origin", "refs/heads/main"))
	capture := installRunnerDebtChecker(t, root)
	command := exec.CommandContext(t.Context(), "bash", filepath.Join(root, push.Script))
	command.Dir = root
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("diverged upstream admitted the prepared push:\n%s", output)
	}
	assertRunnerDebtRecheck(t, capture, prepared)
	for _, want := range []string{"the upstream moved", "refused by the re-check above", "nothing was pushed"} {
		if !strings.Contains(string(output), want) {
			t.Fatalf("refusal lacks %q:\n%s", want, output)
		}
	}
	if got := strings.TrimSpace(runCommitGitOutput(t, root, "ls-remote", "origin", "refs/heads/main")); got != upstream {
		t.Fatalf("refused script moved origin: %q, want %q", got, upstream)
	}
	if got := revParse(t, root, "HEAD"); got != prepared {
		t.Fatalf("refused script moved local HEAD: %s, want %s", got, prepared)
	}
}

// installRunnerDebtChecker records argv and delegates to the existing native
// Answer subprocess bridge. It never substitutes a verdict for the producer.
func installRunnerDebtChecker(t *testing.T, root string) string {
	t.Helper()
	installNativeReviewChecker(t, root)
	path := filepath.Join(root, "le")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	capture := filepath.Join(root, "tmp", "runner-debt-arguments")
	bridge := strings.Replace(string(body), "#!/bin/bash\n", "#!/bin/bash\nprintf '%s\\n' \"$@\" > "+shellQuote(capture)+"\n", 1)
	if err := os.WriteFile(path, []byte(bridge), 0o700); err != nil { //nolint:gosec // executable test-only native Answer bridge
		t.Fatal(err)
	}
	return capture
}

func assertRunnerDebtRecheck(t *testing.T, capture, target string) {
	t.Helper()
	arguments, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(arguments), "commit\ndebt-status\ncommit\n"+target+"\n"; got != want {
		t.Fatalf("debt recheck argv %q, want %q", got, want)
	}
}
