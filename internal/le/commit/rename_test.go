// Related: review.go — closureStem, closedSpecStem, renamedSpecs
//
// VALIDATES: a spec whose stem is renamed in one commit, with its content
// carried over and only the old stem replaced by the new one, is a rename and
// does not count as a closure, both when the commit is prepared and when a
// discharge re-derives it from the commit's own blobs.
// PREVENTS: the owner's rename of a spec being refused with "no
// independent-review artifact", and the opposite failure: a real closure
// passing as a rename because some other spec was added beside it.
package commit

import (
	"strings"
	"testing"
)

// renameWork is the stem-free body each fixture spec carries. Two specs with
// the same body would match each other under stem replacement, which is the
// one-to-one case TestOneAddedSpecCannotExcuseTwoRemovals pins on purpose.
var renameWork = map[string]string{
	"old-name": "Carry the lab cleanup.",
	"new-name": "Carry the lab cleanup.",
	"finished": "Land the finished work.",
	"moving":   "Move between buckets.",
}

// renameFixtureSpec is a spec body that names its own stem twice, in the title
// and in a self-reference, the two places a rename has to rewrite.
func renameFixtureSpec(stem string) string {
	return "# Spec: " + stem + "\n\n| Status | in-progress |\n\n" + renameWork[stem] + "\n" +
		"See plan/spec-" + stem + ".md for the acceptance criteria.\n"
}

// commitRenameBaseline commits each spec at HEAD with its fixture body, so the
// removed side of a rename has a committed blob to compare against.
func commitRenameBaseline(t *testing.T, stems ...string) string {
	t.Helper()
	root := newCommitRepository(t)
	for _, stem := range stems {
		path := "plan/spec-" + stem + ".md"
		writeCommitFixture(t, root, path, renameFixtureSpec(stem))
		runCommitGit(t, root, "add", "--", path)
	}
	runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t",
		"-c", "commit.gpgsign=false", "commit", "-q", "-m", "spec baseline")
	return root
}

// TestAStemRenameWithCarriedContentIsNotAClosure is case (a): the old spec is
// removed, the new one carries the same text with the stem replaced, and the
// commit closes nothing.
func TestAStemRenameWithCarriedContentIsNotAClosure(t *testing.T) {
	root := commitRenameBaseline(t, "old-name")
	writeCommitFixture(t, root, "plan/spec-new-name.md", renameFixtureSpec("new-name"))

	stem, err := closureStem(root,
		[]string{"plan/spec-new-name.md", "plan/spec-old-name.md"},
		[]string{"plan/spec-old-name.md"})
	if err != nil {
		t.Fatalf("a stem rename was refused: %v", err)
	}
	if stem != "" {
		t.Fatalf("closureStem = %q, want empty: a rename closes nothing", stem)
	}
}

// TestARenameThatAlsoChangesContentIsAClosure is case (b). Any change beyond
// the stem means the new file is not the old spec under another name, so the
// removal is a closure and the commit is refused without a review.
func TestARenameThatAlsoChangesContentIsAClosure(t *testing.T) {
	root := commitRenameBaseline(t, "old-name")
	writeCommitFixture(t, root, "plan/spec-new-name.md",
		renameFixtureSpec("new-name")+"\nOne more acceptance criterion.\n")

	stem, err := closureStem(root,
		[]string{"plan/spec-new-name.md", "plan/spec-old-name.md"},
		[]string{"plan/spec-old-name.md"})
	if err != nil {
		t.Fatalf("closureStem: %v", err)
	}
	if stem != "old-name" {
		t.Fatalf("closureStem = %q, want \"old-name\": the new spec's content changed", stem)
	}

	_, err = Create(root, &Options{
		Subject: "plan: rename and edit a spec",
		Files:   []string{"plan/spec-new-name.md"},
		Remove:  []string{"plan/spec-old-name.md"},
	})
	if err == nil || !strings.Contains(err.Error(), "no independent-review artifact") {
		t.Fatalf("Create = %v, want the closure refused for want of a review", err)
	}
}

// TestCreateAcceptsAPureStemRename runs case (a) through the real entry point,
// where the refusal was observed.
func TestCreateAcceptsAPureStemRename(t *testing.T) {
	root := commitRenameBaseline(t, "old-name")
	writeCommitFixture(t, root, "plan/spec-new-name.md", renameFixtureSpec("new-name"))

	if _, err := Create(root, &Options{
		Subject: "plan: rename a spec",
		Files:   []string{"plan/spec-new-name.md"},
		Remove:  []string{"plan/spec-old-name.md"},
	}); err != nil {
		t.Fatalf("Create refused a pure stem rename: %v", err)
	}
}

// TestOneRenameBesideARemovalLeavesTheRemovalAClosure is case (c): of two
// removed specs, the one renamed is excused and the other still closes.
func TestOneRenameBesideARemovalLeavesTheRemovalAClosure(t *testing.T) {
	root := commitRenameBaseline(t, "old-name", "finished")
	writeCommitFixture(t, root, "plan/spec-new-name.md", renameFixtureSpec("new-name"))

	stem, err := closureStem(root,
		[]string{"plan/spec-new-name.md", "plan/spec-old-name.md", "plan/spec-finished.md"},
		[]string{"plan/spec-old-name.md", "plan/spec-finished.md"})
	if err != nil {
		t.Fatalf("closureStem: %v", err)
	}
	if stem != "finished" {
		t.Fatalf("closureStem = %q, want \"finished\": only old-name was renamed", stem)
	}
}

// TestOneAddedSpecCannotExcuseTwoRemovals keeps the pairing one to one. A spec
// that matches two removed ones under their own stems excuses neither.
func TestOneAddedSpecCannotExcuseTwoRemovals(t *testing.T) {
	root := newCommitRepository(t)
	body := "# Spec\n\nNo stem appears here.\n"
	for _, path := range []string{"plan/spec-one.md", "plan/spec-two.md"} {
		writeCommitFixture(t, root, path, body)
		runCommitGit(t, root, "add", "--", path)
	}
	runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t",
		"-c", "commit.gpgsign=false", "commit", "-q", "-m", "spec baseline")
	writeCommitFixture(t, root, "plan/spec-three.md", body)

	if _, err := closureStem(root,
		[]string{"plan/spec-three.md"},
		[]string{"plan/spec-one.md", "plan/spec-two.md"}); err == nil {
		t.Fatal("one added spec excused two removals")
	}
}

// TestAnExistingSpecIsNoRenameTarget keeps "adds a new spec" literal. A spec
// already at HEAD that this commit edits is not the renamed file, whatever it
// now holds.
func TestAnExistingSpecIsNoRenameTarget(t *testing.T) {
	root := commitRenameBaseline(t, "old-name", "new-name")
	writeCommitFixture(t, root, "plan/spec-new-name.md", renameFixtureSpec("new-name"))

	stem, err := closureStem(root,
		[]string{"plan/spec-new-name.md", "plan/spec-old-name.md"},
		[]string{"plan/spec-old-name.md"})
	if err != nil {
		t.Fatalf("closureStem: %v", err)
	}
	if stem != "old-name" {
		t.Fatalf("closureStem = %q, want \"old-name\": spec-new-name existed already", stem)
	}
}

// TestABucketMoveIsStillARelocation is case (d), over a real repository so the
// content read runs: the same name in another bucket stays a relocation.
func TestABucketMoveIsStillARelocation(t *testing.T) {
	root := commitRenameBaseline(t, "moving")
	writeCommitFixture(t, root, "plan/immediate/spec-moving.md", renameFixtureSpec("moving")+"edited\n")

	stem, err := closureStem(root,
		[]string{"plan/immediate/spec-moving.md", "plan/spec-moving.md"},
		[]string{"plan/spec-moving.md"})
	if err != nil {
		t.Fatalf("closureStem: %v", err)
	}
	if stem != "" {
		t.Fatalf("closureStem = %q, want empty: a bucket move closes nothing", stem)
	}
}

// TestADischargeReadsARenameFromTheCommitBlobs pins the second caller. A
// discharge re-derives the closure months later from the commit alone, so the
// rename has to be recognized from the parent's blob and the commit's blob,
// with nothing left in the working tree.
func TestADischargeReadsARenameFromTheCommitBlobs(t *testing.T) {
	root := commitRenameBaseline(t, "old-name")
	parent := strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "HEAD"))
	writeCommitFixture(t, root, "plan/spec-new-name.md", renameFixtureSpec("new-name"))
	runCommitGit(t, root, "add", "--", "plan/spec-new-name.md")
	runCommitGit(t, root, "rm", "-q", "--", "plan/spec-old-name.md")
	runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t",
		"-c", "commit.gpgsign=false", "commit", "-q", "-m", "rename")
	sha := strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "HEAD"))
	runCommitGit(t, root, "rm", "-q", "--", "plan/spec-new-name.md")

	facts := commitFacts{
		SHA: sha, Parent: parent, Subject: "rename",
		Paths:   []string{"plan/spec-new-name.md", "plan/spec-old-name.md"},
		Removed: []string{"plan/spec-old-name.md"},
	}
	if err := verifyKindNotApplicable(root, gateReviewOverride, facts); err != nil {
		t.Fatalf("a discharge read the committed rename as a closure: %v", err)
	}
	path, err := removedSpecPath(root, facts)
	if err != nil || path != "" {
		t.Fatalf("removedSpecPath = %q, %v; want no closure", path, err)
	}
}
