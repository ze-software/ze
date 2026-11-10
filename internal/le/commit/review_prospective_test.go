// Design: docs/contributing/spec-workflow.md -- review pins the prospective commit tree.
package commit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/commit/hunks"
	"github.com/ze-software/ze/internal/le/spec"
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
				Append: origin == "earlier script block", NoNews: "fixture: no operator-visible change"})
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

// TestReviewIdentityExcludesUnselectedConcurrentBytes proves that bytes another
// session left in a file do not become part of what a review vouches for. The
// fixture records a real `le spec review record ... hunks <path> <patch>` while
// the working-tree file holds this session's hunk and a concurrent one, then
// reads the pin the artifact carries: it MUST be the digest of HEAD's blob with
// the selected patch applied, never the digest of the live file. The gate is
// then asked both ways: over the selection the commit carries it is clean, and
// over the whole working-tree file it refuses the path as changed after the
// review, because no review saw the concurrent hunk.
func TestReviewIdentityExcludesUnselectedConcurrentBytes(t *testing.T) {
	const path = "pkg/shared.go"
	const stem = "demo"
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "review-identity-fixture")
	configureCommitAuthor(t, root)
	commitFixture(t, root, "base", map[string]string{path: hunkBase}, nil)
	patch := writeHunkPatch(t, root, path, hunkMine, hunkBoth)
	recordFixtureReview(t, root, "spec", stem, "verdict", "clean", "rounds", "1", "hunks", path, patch)

	selected := reviewDigest([]byte(hunkMine), nil)
	live, err := spec.ReviewFile(root, path)
	if err != nil {
		t.Fatal(err)
	}
	if live.Hash == selected {
		t.Fatal("fixture: the working-tree file holds no concurrent bytes")
	}
	recorded := recordedReviewPin(t, root, stem, path)
	if recorded.Hash != selected {
		t.Fatalf("the review identity is not the selected snapshot: recorded %s, selected snapshot %s, "+
			"working-tree file %s", recorded.Hash, selected, live.Hash)
	}
	if recorded.Mode != "100644" {
		t.Fatalf("the review pinned mode %q, want the selected entry's 100644", recorded.Mode)
	}

	entry, err := hunks.Apply(root, headOf(t, root), path, patch)
	if err != nil {
		t.Fatal(err)
	}
	selection := hunks.Selection{path: {Entry: entry, Patch: patch}}
	if result := checkReviewSelected(root, stem, []string{path}, selection); !result.Clean {
		t.Fatalf("the review over the selected snapshot is not clean: %#v", result)
	}
	whole := CheckReview(root, stem, []string{path})
	if whole.Clean {
		t.Fatal("the review vouched for concurrent unselected bytes it never covered")
	}
	if len(whole.Stale) != 1 || whole.Stale[0] != path {
		t.Fatalf("the whole working-tree file is not refused as changed after the review: %#v", whole)
	}
	body, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != hunkBoth {
		t.Fatalf("recording the review changed the concurrent checkout bytes: %q", body)
	}
}

// TestReviewIdentityOfAnEarlierWholeSnapshotExcludesLaterEdits is the blob
// route's twin of TestReviewIdentityExcludesUnselectedConcurrentBytes. An
// earlier script block snapshots the whole file; another session then edits
// the working-tree copy before the review is recorded. The fixture records a
// real `le spec review record ... blob <path> <object> mode <path> 100644` over
// the earlier block's snapshot object and reads the pin the artifact carries:
// it MUST be the digest of the snapshot bytes, never the digest of the live
// file. The gate is then asked both ways: over the selection the earlier block
// carries it is clean, and over the whole working-tree file it refuses the path
// as changed after the review, because no review saw the later edit.
func TestReviewIdentityOfAnEarlierWholeSnapshotExcludesLaterEdits(t *testing.T) {
	const path = "internal/reviewed.go"
	const stem = "earlier-whole"
	root := newCommitRepository(t)
	configureCommitAuthor(t, root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "earlier-whole-identity-fixture")
	commitFixture(t, root, "the live spec", map[string]string{"plan/release/fixes/spec-earlier-whole.md": "# Spec: earlier-whole\n"}, nil)
	const selected = "package internal\n\nconst Reviewed = 1\n"
	const concurrent = selected + "const Concurrent = 2\n"
	writeCommitFixture(t, root, path, selected)
	earlier, err := Create(root, &Options{Subject: "reviewed earlier block", Files: []string{path},
		NoTest: "fixture constant has no runtime behavior", SpecWork: stem})
	if err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(filepath.Join(root, earlier.Script))
	if err != nil {
		t.Fatal(err)
	}
	blob, entry := "", ""
	if err := walkScriptBlocks(earlier.Script, string(script), func(line string) error {
		object, named, err := snapshotEntryFields(line)
		if named == path {
			blob, entry = object, line
		}
		return err
	}, func([]string) {}); err != nil {
		t.Fatal(err)
	}
	if blob == "" {
		t.Fatal("fixture: the earlier block carries no whole snapshot of the reviewed file")
	}
	writeCommitFixture(t, root, path, concurrent)
	recordFixtureReview(t, root, "spec", stem, "verdict", "clean", "rounds", "1",
		"blob", path, blob, "mode", path, "100644")

	snapshot := reviewDigest([]byte(selected), nil)
	live, err := spec.ReviewFile(root, path)
	if err != nil {
		t.Fatal(err)
	}
	if live.Hash == snapshot {
		t.Fatal("fixture: the working-tree file holds no later concurrent bytes")
	}
	recorded := recordedReviewPin(t, root, stem, path)
	if recorded.Hash != snapshot {
		t.Fatalf("the review identity is not the earlier block's snapshot: recorded %s, snapshot %s, "+
			"working-tree file %s", recorded.Hash, snapshot, live.Hash)
	}
	if recorded.Mode != "100644" {
		t.Fatalf("the review pinned mode %q, want the snapshot entry's 100644", recorded.Mode)
	}

	selection := hunks.Selection{path: {Entry: entry}}
	if result := checkReviewSelected(root, stem, []string{path}, selection); !result.Clean {
		t.Fatalf("the review over the earlier block's snapshot is not clean: %#v", result)
	}
	whole := CheckReview(root, stem, []string{path})
	if whole.Clean {
		t.Fatal("the review vouched for later concurrent bytes it never covered")
	}
	if len(whole.Stale) != 1 || whole.Stale[0] != path {
		t.Fatalf("the whole working-tree file is not refused as changed after the review: %#v", whole)
	}
	body, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != concurrent {
		t.Fatalf("recording the review changed the concurrent checkout bytes: %q", body)
	}
}

// recordedReviewPin answers the one pin the stem's review artifact records for
// path, parsed with the pattern the commit gate reads it through.
func recordedReviewPin(t *testing.T, root, stem, path string) spec.ReviewedFile {
	t.Helper()
	relative, err := spec.ReviewArtifactPath(root, stem)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatal(err)
	}
	var pins []spec.ReviewedFile
	for line := range strings.SplitSeq(string(content), "\n") {
		match := reviewFilePattern.FindStringSubmatch(line)
		if len(match) == 4 && match[3] == path {
			pins = append(pins, spec.ReviewedFile{Path: path, Hash: match[1], Mode: match[2]})
		}
	}
	if len(pins) != 1 {
		t.Fatalf("review artifact %s carries %d pin(s) for %s, want one:\n%s", relative, len(pins), path, content)
	}
	return pins[0]
}
