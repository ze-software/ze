// Design: docs/contributing/spec-workflow.md -- authorization precedes owner-session writes.
package spec

import (
	"maps"
	"strconv"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/le/human/humantest"
)

// TestOwnerReviewCommandRefusesBeforeNamespaceWrites reaches the registered
// review record entry below and above the cap. A terminal succeeds; every pipe
// or unanswered detector leaves the entire fixture unchanged, including session
// directories that resolving the owner would otherwise create before recording.
func TestOwnerReviewCommandRefusesBeforeNamespaceWrites(t *testing.T) {
	for _, mode := range []string{"terminal", "piped-stdin", "piped-stdout", "piped-both", "closed-stdin", "closed-stdout"} {
		t.Run(mode, func(t *testing.T) {
			humantest.Streams(t, mode)
			for _, rounds := range []int{1, 6} {
				t.Run(strconv.Itoa(rounds), func(t *testing.T) {
					root := reviewFixture(t)
					t.Setenv("ZE_REPO_ROOT", root)
					t.Setenv("CLAUDE_CODE_SESSION_ID", "review-terminal-entry-fixture")
					env.ResetCache()
					t.Cleanup(env.ResetCache)
					before := humantest.Snapshot(t, root)
					_, code := Review([]string{"record", "spec", "demo", "verdict", "clean", "rounds", strconv.Itoa(rounds),
						"file", "pkg/a.go", "rounds-reason", "an extra pass found a product defect",
						keywordOwnerAuthorised, "Thomas asked for this pass"})
					if mode == "terminal" {
						if code != 0 {
							t.Fatalf("terminal owner review exit = %d, want 0", code)
						}
						check, err := CheckReview(root, "demo", "review-terminal-entry-fixture", []string{"pkg/a.go"})
						if err != nil || check.Blocked {
							t.Fatalf("recorded terminal review: %+v, %v", check, err)
						}
						return
					}
					if code != 2 {
						t.Errorf("owner-review command exit = %d, want 2", code)
					}
					if !maps.Equal(before, humantest.Snapshot(t, root)) {
						t.Error("refused review command created an owner-session namespace, directory or artifact")
					}
				})
			}
		})
	}
}

// TestOrdinaryReviewCommandRemainsNoninteractive protects the registered entry
// without owner authorization, not just the lower recorder, from a blanket gate.
func TestOrdinaryReviewCommandRemainsNoninteractive(t *testing.T) {
	humantest.Streams(t, "piped-both")
	root := reviewFixture(t)
	t.Setenv("ZE_REPO_ROOT", root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "ordinary-review-entry-fixture")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	_, code := Review([]string{"record", "spec", "demo", "verdict", "clean", "rounds", "1", "file", "pkg/a.go"})
	if code != 0 {
		t.Fatalf("ordinary review command under pipes exit = %d, want 0", code)
	}
	check, err := CheckReview(root, "demo", "ordinary-review-entry-fixture", []string{"pkg/a.go"})
	if err != nil || check.Blocked {
		t.Fatalf("ordinary registered review: %+v, %v", check, err)
	}
}
