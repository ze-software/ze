// Design: docs/contributing/spec-workflow.md -- retries preserve authorization and complete review scope.
package spec

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/le/human/humantest"
)

// TestOwnerReviewRefusalPreservesRetryOperation shell-decodes the printed retry
// and replays the registered writer under a PTY from a different operator
// context. The above-cap reason, all files and owner words must survive intact,
// and the artifact must still belong to the originating harness session.
func TestOwnerReviewRefusalPreservesRetryOperation(t *testing.T) {
	humantest.Streams(t, "piped-both")
	root := reviewFixture(t)
	if err := os.WriteFile(filepath.Join(root, "pkg", "b.go"), []byte("package a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "findings.md"), []byte("pkg/a.go: independently reviewed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZE_REPO_ROOT", root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "review-retry-origin")
	t.Setenv("ZE_AGENT_ID", "origin-agent")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	request := reviewRecord{Spec: "demo", Verdict: "clean", Rounds: 6, Files: []string{"pkg/a.go", "pkg/b.go"},
		Reviewers: "owner's \"$reviewer\"", FindingsFile: "findings.md", SessionID: "review-retry-origin",
		RoundsReason:    "pass found Thomas's \"$widget\" and `printf marker`",
		OwnerAuthorised: "Thomas's \"$widget\" and `printf marker` may be reviewed"}
	_, err := recordReview(root, request)
	t.Setenv("ZE_REPO_ROOT", t.TempDir())
	t.Setenv("CLAUDE_CODE_SESSION_ID", "operator-fixture")
	t.Setenv("ZE_AGENT_ID", "operator-agent")
	env.ResetCache()
	arguments := humantest.RetryArguments(t, err)
	if !slices.Equal(arguments[:4], []string{"./le", "spec", "review", "record"}) {
		t.Fatalf("review retry route = %q", arguments)
	}
	parsed, problem := parseReviewRecordLine(arguments[4:])
	if problem != nil {
		t.Fatal(problem)
	}
	parsed.SessionID = request.SessionID
	if !reflect.DeepEqual(parsed, request) {
		t.Errorf("retry record = %+v, want %+v", parsed, request)
	}
	t.Run("retry", func(t *testing.T) {
		humantest.Streams(t, "terminal")
		payload, code := Review(arguments[3:])
		if code != 0 {
			t.Fatalf("printed review retry exit = %d, want 0", code)
		}
		artifact, ok := payload.(reviewArtifact)
		if !ok {
			t.Fatalf("review answer = %T", payload)
		}
		path := reviewArtifactPath(root, request.Spec, request.SessionID)
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{" rounds=" + strconv.Itoa(request.Rounds) + " ",
			"\nrounds-reason: " + request.RoundsReason + "\n", "\nowner-authorised: " + request.OwnerAuthorised + "\n"} {
			if !strings.Contains(string(body), field) {
				t.Errorf("persisted review omitted %q: %s", field, body)
			}
		}
		recorded, err := parseReviewArtifact(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(recorded.Files) != len(request.Files) {
			t.Errorf("persisted review file count = %d, want %d", len(recorded.Files), len(request.Files))
		}
		if artifact.Rounds != request.Rounds || artifact.OwnerAuthorised != request.OwnerAuthorised || artifact.RoundsReason != request.RoundsReason || len(artifact.Files) != len(request.Files) {
			t.Fatalf("replayed review lost authorization or scope: %+v", artifact)
		}
		check, err := CheckReview(root, request.Spec, request.SessionID, request.Files)
		if err != nil || check.Blocked {
			t.Fatalf("replayed review is not consumable for the original scope: %+v, %v", check, err)
		}
	})
}
