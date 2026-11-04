// Design: docs/contributing/spec-workflow.md -- review pins the prospective commit tree.
package commit

import (
	"os"
	"path/filepath"
	"testing"
)

// TestClosureReviewIgnoresUnselectedConcurrentEdits records a real review then
// dirties historical Spec-work code that the closure does not select. Both a
// landed blob and a still-unpublished earlier block must come from the tree.
func TestClosureReviewIgnoresUnselectedConcurrentEdits(t *testing.T) {
	const code = "internal/reviewed.go"
	const spec = "plan/release/fixes/spec-prospective-review.md"
	for _, origin := range []string{"landed Spec-work", "earlier script block"} {
		t.Run(origin, func(t *testing.T) {
			root := newCommitRepository(t)
			configureCommitAuthor(t, root)
			t.Setenv("CLAUDE_CODE_SESSION_ID", "prospective-review-fixture")
			commitFixture(t, root, "the live spec", map[string]string{spec: "# Spec: prospective-review\n"}, nil)
			const reviewed = "package internal\n\nconst Reviewed = 1\n"
			if origin == "landed Spec-work" {
				commitFixture(t, root, "reviewed work\n\nSpec-work: prospective-review", map[string]string{code: reviewed}, nil)
			} else {
				writeCommitFixture(t, root, code, reviewed)
				if _, err := Create(root, &Options{Subject: "reviewed earlier block", Files: []string{code},
					NoTest: "fixture constant has no runtime behavior", SpecWork: "prospective-review"}); err != nil {
					t.Fatal(err)
				}
			}
			recordFixtureReview(t, root, "spec", "prospective-review", "verdict", "clean", "rounds", "1", "file", code)
			const concurrent = "package internal\n\nconst Reviewed = 1\nconst Concurrent = 2\n"
			writeCommitFixture(t, root, code, concurrent)
			prepared, err := Create(root, &Options{Subject: "close reviewed spec", Remove: []string{spec},
				Append: origin == "earlier script block"})
			if err != nil {
				t.Fatalf("unselected concurrent edit invalidated immutable %s: %v", origin, err)
			}
			if prepared.Review == nil {
				t.Fatal("closure skipped review instead of judging its tree")
			}
			if !prepared.Review.Clean {
				t.Fatalf("review over the selected tree was not clean: %#v", prepared.Review)
			}
			body, err := os.ReadFile(filepath.Join(root, code))
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != concurrent {
				t.Fatalf("preparation changed unselected concurrent bytes: %q", body)
			}
		})
	}
}

// TestClosureReviewRejectsAnUnreviewedModeChange proves that identical source
// bytes do not make a newly executable file the artifact's reviewed tree entry.
func TestClosureReviewRejectsAnUnreviewedModeChange(t *testing.T) {
	const code = "internal/reviewed.sh"
	const spec = "plan/release/fixes/spec-review-mode.md"
	root := newCommitRepository(t)
	configureCommitAuthor(t, root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "prospective-review-mode-fixture")
	commitFixture(t, root, "spec and source", map[string]string{
		code: "#!/bin/sh\nprintf 'reviewed bytes\\n'\n", spec: "# Spec: review-mode\n",
	}, nil)
	recordFixtureReview(t, root, "spec", "review-mode", "verdict", "clean", "rounds", "1", "file", code)
	if err := os.Chmod(filepath.Join(root, code), 0o700); err != nil { //nolint:gosec // the unreviewed executable mode is the defect under test
		t.Fatal(err)
	}
	if _, err := Create(root, &Options{Subject: "close spec with a mode change", Files: []string{code},
		Remove: []string{spec}, NoTest: "fixture only changes shell file mode"}); err == nil {
		t.Fatal("closure accepted executable mode the review artifact never pinned")
	}
}
