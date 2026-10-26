package spec

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestReviewCheckIgnoresPlanMarkdownEditedAfterTheReview pins the exemption
// IsReviewCode declares, through the recorder and `le spec review check`.
//
// VALIDATES: a review recorded over the code and the spec stays clean when only
// the spec under plan/ changes afterwards, and a later edit to the .go file is
// still refused as stale.
// PREVENTS: fixing a review NOTE or filling the closure sections forcing a fresh
// round over unchanged code, and the exemption reaching code.
func TestReviewCheckIgnoresPlanMarkdownEditedAfterTheReview(t *testing.T) {
	root := reviewFixture(t)
	specFile := "plan/post-release/engineering/spec-demo.md"
	writeReviewFixtureFile(t, root, specFile, "# Spec\n\nNOTE: unfixed\n")
	request := reviewRecord{
		Spec: specFile, Verdict: "clean", Rounds: 1,
		Files: []string{"pkg/a.go", specFile}, Model: "claude-opus-5", SessionID: "session-a",
		Now: time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC),
	}
	if _, err := recordReview(root, request); err != nil {
		t.Fatalf("recordReview: %v", err)
	}

	writeReviewFixtureFile(t, root, specFile, "# Spec\n\nNOTE: fixed\n\n## Review Gate\n\nclean\n")
	files := []string{"pkg/a.go", specFile}
	clean, err := CheckReview(root, "demo", "session-a", files)
	if err != nil {
		t.Fatalf("check after the spec edit: %v", err)
	}
	if clean.Blocked {
		t.Fatalf("a post-review edit to plan Markdown only was refused: %#v", clean)
	}

	writeReviewFixtureFile(t, root, "pkg/a.go", "package a\nfunc B() {}\n")
	stale, err := CheckReview(root, "demo", "session-a", files)
	if err != nil {
		t.Fatalf("check after the code edit: %v", err)
	}
	if !stale.Blocked {
		t.Fatalf("a post-review edit to a .go file passed: %#v", stale)
	}
	if stale.Reason != "stale" {
		t.Fatalf("code edit check = %#v, want stale", stale)
	}
}

func writeReviewFixtureFile(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
