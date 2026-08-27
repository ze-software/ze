// VALIDATES: the native lifecycle keeps the Python producer's paths and
// operator diagnostics without executing scripts/dev/verify_worktree.py or its
// verification body.
// PREVENTS: a port that is behaviorally native but changes the evidence paths
// or the diagnostics developers already use.
package verifyworktree

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/verify"
)

func TestPythonPathAndOwnerMarkerParity(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	sha := "d3bd1d88b88c0000000000000000000000000000"
	stamp := "20260822T130818Z"
	path := worktreePath(root, sha, stamp)
	want := filepath.Join(root, "tmp", "verify-worktree", "20260822T130818Z-d3bd1d88b88c")
	if path != want {
		t.Fatalf("worktree path = %q, want Python producer path %q", path, want)
	}
	if marker := ownerMarker(path); marker != path+".owner" {
		t.Fatalf("owner marker = %q, want %q", marker, path+".owner")
	}
}

func TestPythonInvalidCommitDiagnosticParityWithoutVerifyBody(t *testing.T) {
	repo := newFixtureRepo(t)
	called := false
	runner := func(context.Context, string, verify.Identity) verify.GateResult {
		called = true
		return verify.GateResult{}
	}

	report := Run(context.Background(), repo.root, Options{Commit: "not-a-commit"}, runner)
	if called {
		t.Fatal("invalid commit invoked the verification body")
	}
	want := "verify-worktree: not-a-commit does not name a commit\n"
	if got := report.Text(); got != want {
		t.Fatalf("diagnostic = %q, want Python lifecycle diagnostic %q", got, want)
	}
}

func TestPythonAddFailureDiagnosticNamesShortCommitWithoutVerifyBody(t *testing.T) {
	root := t.TempDir()
	sha := strings.Repeat("b", 40)
	fakeGit := func(_ context.Context, _ string, args ...string) (commandResult, error) {
		switch args[0] {
		case "rev-parse":
			return commandResult{Output: sha + "\n"}, nil
		case "worktree":
			if args[1] == "add" {
				return commandResult{Code: 128, Output: "fixture add refusal\n"}, nil
			}
			return commandResult{}, nil
		default:
			return commandResult{}, nil
		}
	}
	deps := dependencies{
		git: fakeGit,
		now: func() time.Time { return time.Date(2026, 8, 22, 13, 8, 18, 0, time.UTC) },
		pid: func() int { return 1 }, alive: func(int) bool { return true },
	}
	called := false
	runner := func(context.Context, string, verify.Identity) verify.GateResult {
		called = true
		return verify.GateResult{}
	}

	report := run(context.Background(), root, Options{}, runner, deps)
	if called {
		t.Fatal("worktree add failure invoked the verification body")
	}
	want := "verify-worktree: git worktree add failed for bbbbbbbbbbbb: fixture add refusal"
	if !strings.Contains(report.Text(), want) {
		t.Fatalf("diagnostic = %q, want it to contain %q", report.Text(), want)
	}
}
