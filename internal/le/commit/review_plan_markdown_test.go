package commit

import (
	"slices"
	"testing"
)

// TestReviewGatePassesAClosureWhoseOnlyPostReviewEditIsPlanMarkdown pins the
// exemption spec.IsReviewCode declares for the commit gate.
//
// VALIDATES: a closure commit whose reviewer pinned the spec beside the code
// stays clean when, after the review, only Markdown under plan/ changes (the
// spec's NOTE fixed, its closure sections filled, a learned summary added), and
// a post-review edit to a .go file in the same commit is still refused as stale.
// PREVENTS: a NOTE fix or the closure write-up re-opening a clean review, which
// planning.md says a NOTE never does, and the exemption widening into code.
func TestReviewGatePassesAClosureWhoseOnlyPostReviewEditIsPlanMarkdown(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CODE_SESSION_ID", "harness-session-id")
	stem := "native-port"
	code := "internal/a.go"
	specFile := "plan/post-release/engineering/spec-native-port.md"
	learned := "plan/learned/999-native-port.md"
	writeCommitFixture(t, root, code, "package internal\n")
	writeCommitFixture(t, root, specFile, "# Spec\n\nNOTE: unfixed\n")
	writeReviewArtifact(t, root, stem, code, specFile)

	writeCommitFixture(t, root, specFile, "# Spec\n\nNOTE: fixed\n\n## Review Gate\n\nclean\n")
	writeCommitFixture(t, root, learned, "# Learned\n")
	paths := []string{code, specFile, learned}
	clean := CheckReview(root, stem, paths)
	if !clean.Clean {
		t.Fatalf("a post-review edit to plan Markdown only was refused: %#v", clean)
	}
	if !slices.Equal(clean.CodeFiles, []string{code}) {
		t.Fatalf("code files = %v, want only %s", clean.CodeFiles, code)
	}

	writeCommitFixture(t, root, code, "package changed\n")
	stale := CheckReview(root, stem, paths)
	if stale.Clean {
		t.Fatalf("a post-review edit to a .go file passed the gate: %#v", stale)
	}
	if !slices.Equal(stale.Stale, []string{code}) {
		t.Fatalf("stale = %v, want only %s", stale.Stale, code)
	}
}
