package testweakened

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

// headBaselineCommitted is the committed test file every case starts from: a
// test and a helper that each make an assertion.
const headBaselineCommitted = "package pkg\n" +
	"func TestKept(t *testing.T) {\n\trequire.NoError(t, err)\n\trequire.Equal(t, 1, got)\n\tintentRow(t)\n}\n" +
	"func intentRow(t *testing.T) {\n\trequire.Equal(t, 2, got)\n}\n"

// commentedOut is the blocking shape every refusal case proposes: the
// assertion it replaces stays in the file as a comment (detect, "commenting
// out assertions"). Dropping the line alone only lowers a count, which the
// hook answers with a notice.
const commentedOut = "\t// require.NoError(t, err)\n"

// headBaselineRepository commits pkg/intent_test.go and answers the root.
func headBaselineRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeProposedFile(t, root, "pkg/intent_test.go", headBaselineCommitted)
	commitProposedFixture(t, root)
	return root
}

func headBaselineEdit(t *testing.T, root, path, oldString, newString string, replaceAll bool) ProposedReport {
	t.Helper()
	report, err := proposedFixture(root, ProposedRequest{
		Path: path, Tool: "Edit", ToolInput: ProposedToolInput{
			OldString: oldString, NewString: newString, ReplaceAll: replaceAll,
		},
	})
	if err != nil {
		t.Fatalf("Edit of %s: %v", path, err)
	}
	return report
}

// TestProposedSkipsUnitsHEADNeverHeld reproduces the two refusals of
// 2026-11-13 (plan/journal/guard-blocks-its-own-authors-repair.md): an
// untracked test file whose helper collided with a committed helper of the
// same name could not rename it, and could not drop a test or a helper.
// Method: a real Git fixture where pkg/intent_test.go is committed and
// pkg/split_test.go is new and untracked, declaring its own intentRow; the
// hook's whole-file Edit proposals are judged against it.
// VALIDATES: a unit no committed baseline records is never judged weakened,
// even when a committed file of the same directory declares its name.
// PREVENTS: the guard refusing its author's repair of test code no commit holds.
func TestProposedSkipsUnitsHEADNeverHeld(t *testing.T) {
	root := headBaselineRepository(t)
	untracked := "pkg/split_test.go"
	writeProposedFile(t, root, untracked, "package pkg\n"+
		"func TestSplitA(t *testing.T) {\n\tintentRow(t, 1)\n}\n"+
		"func TestSplitB(t *testing.T) {\n\tintentRow(t, 2)\n\trequire.Equal(t, 3, got)\n}\n"+
		"func intentRow(t *testing.T, want int) {\n\trequire.Equal(t, want, got)\n\trequire.NoError(t, err)\n}\n")

	renamed := headBaselineEdit(t, root, untracked, "intentRow(", "splitRow(", true)
	if renamed.ExitCode() != 0 || len(renamed.Weakened) != 0 || renamed.Notice {
		t.Fatalf("renaming a helper of an untracked file = %#v, want clean", renamed)
	}

	dropped := headBaselineEdit(t, root, untracked,
		"func TestSplitB(t *testing.T) {\n\tintentRow(t, 2)\n\trequire.Equal(t, 3, got)\n}\n", "", false)
	if dropped.ExitCode() != 0 || len(dropped.Weakened) != 0 || dropped.Notice {
		t.Fatalf("deleting a test of an untracked file = %#v, want clean", dropped)
	}

	helper := headBaselineEdit(t, root, untracked, "\trequire.NoError(t, err)\n", commentedOut, false)
	if helper.ExitCode() != 0 || len(helper.Weakened) != 0 || helper.Notice {
		t.Fatalf("an untracked helper losing an assertion = %#v, want clean", helper)
	}
}

// TestProposedSkipsAUnitAddedToATrackedFileSinceHEAD proves the same rule
// inside a tracked file: a helper the author added since HEAD is theirs to
// remove (the 2026-11-12 l2tp row), while the file's committed units stay
// guarded.
// VALIDATES: the baseline is HEAD's text of the file, not the working tree.
// PREVENTS: a tracked file's uncommitted helper being locked like a committed one.
func TestProposedSkipsAUnitAddedToATrackedFileSinceHEAD(t *testing.T) {
	root := headBaselineRepository(t)
	path := "pkg/intent_test.go"
	added := headBaselineCommitted + "func addedHelper(t *testing.T) {\n\trequire.Equal(t, 9, got)\n}\n"
	writeProposedFile(t, root, path, added)

	removed := headBaselineEdit(t, root, path, "func addedHelper(t *testing.T) {\n\trequire.Equal(t, 9, got)\n}\n", "", false)
	if removed.ExitCode() != 0 || len(removed.Weakened) != 0 {
		t.Fatalf("removing a helper HEAD never held = %#v, want clean", removed)
	}
}

// TestProposedStillRefusesCommittedLoss proves what the guard keeps: a
// committed test losing an assertion, and a committed helper removed, are
// each refused without an approval.
// VALIDATES: a unit HEAD records stays guarded exactly as before.
// PREVENTS: the HEAD baseline unlocking committed proof.
func TestProposedStillRefusesCommittedLoss(t *testing.T) {
	root := headBaselineRepository(t)
	path := "pkg/intent_test.go"

	test := headBaselineEdit(t, root, path, "\trequire.NoError(t, err)\n", commentedOut, false)
	if test.ExitCode() != 2 || len(test.Weakened) != 1 || test.Weakened[0].Name != "TestKept" {
		t.Fatalf("a committed test losing an assertion = %#v, want TestKept refused", test)
	}

	helper := headBaselineEdit(t, root, path, "func intentRow(t *testing.T) {\n\trequire.Equal(t, 2, got)\n}\n", "", false)
	if helper.ExitCode() != 2 || len(helper.Weakened) != 1 || helper.Weakened[0].Name != "intentRow" {
		t.Fatalf("a committed helper removed = %#v, want intentRow refused", helper)
	}
}

// TestProposedFollowsAMovedTestFile proves a move cannot launder a weakening:
// a file moved with `git mv`, and one deleted and written again under another
// name, are judged against the committed file the working tree no longer holds.
// VALIDATES: the baseline includes every committed test file of the same kind
// missing from its committed path.
// PREVENTS: a rename turning committed proof into untracked text the guard skips.
func TestProposedFollowsAMovedTestFile(t *testing.T) {
	for _, move := range []string{"git mv", "delete and recreate"} {
		t.Run(move, func(t *testing.T) {
			root := headBaselineRepository(t)
			moved := "pkg/moved_test.go"
			if move == "git mv" {
				if !runSelfTestGit(root, "mv", "pkg/intent_test.go", moved) {
					t.Fatal("git mv the committed test")
				}
			} else {
				if err := os.Remove(filepath.Join(root, "pkg", "intent_test.go")); err != nil {
					t.Fatal(err)
				}
				writeProposedFile(t, root, moved, headBaselineCommitted)
			}
			report := headBaselineEdit(t, root, moved, "\trequire.NoError(t, err)\n", commentedOut, false)
			if report.ExitCode() != 2 || len(report.Weakened) != 1 || report.Weakened[0].Name != "TestKept" {
				t.Fatalf("%s, then a lost assertion = %#v, want TestKept refused", move, report)
			}
			// The stub comparison names TestKept too, and the sibling's
			// finding is merged into it: its reason must still be shown.
			namesTheMissingSibling(t, move+", then a lost assertion", report, "pkg/intent_test.go")
		})
	}
}

// TestProposedSkipsACopyWhileTheOriginalStands records what the HEAD baseline
// does with an untracked copy of a committed test: the copy may lose
// assertions, because the committed original still proves what it proved.
// The first assertion is the proof: before the HEAD baseline, the copy's loss
// was refused. The second, that the original's own loss stays refused, is a
// guard that holds with or without the baseline; it shows the copy did not
// unlock the original, not that the fix works. Removing the original is a
// deletion the commit gate pairs with the copy by rename detection.
// VALIDATES: the copy is new text, not committed proof.
// PREVENTS: an over-broad baseline that locks every file sharing a test's name.
func TestProposedSkipsACopyWhileTheOriginalStands(t *testing.T) {
	root := headBaselineRepository(t)
	copied := "pkg/copy_test.go"
	writeProposedFile(t, root, copied, headBaselineCommitted)
	report := headBaselineEdit(t, root, copied, "\trequire.NoError(t, err)\n", commentedOut, false)
	if report.ExitCode() != 0 || len(report.Weakened) != 0 {
		t.Fatalf("an untracked copy losing an assertion = %#v, want clean", report)
	}
	original := headBaselineEdit(t, root, "pkg/intent_test.go", "\trequire.NoError(t, err)\n", commentedOut, false)
	if original.ExitCode() != 2 {
		t.Fatalf("the committed original losing an assertion = %#v, want refused", original)
	}
}

// TestApproveQualifiesAnAmbiguousUnitWithItsFile reproduces the approval that
// could not be recorded on 2026-11-13: a unit whose name two files of one
// directory declare is refused by name, the refusal names each file, and the
// file-qualified approval records the verdict for that file's unit alone.
// Method: both files committed, so the edit hook judges each; the qualified
// row admits the weakening in its own file and buys nothing in the other.
// VALIDATES: `approve unit <u> file <path>` resolves one carrier, the row
// keeps the file, and the hook matches the file.
// PREVENTS: a judge-approved change with no route to be recorded.
func TestApproveQualifiesAnAmbiguousUnitWithItsFile(t *testing.T) {
	root := t.TempDir()
	writeProposedFile(t, root, "pkg/intent_test.go", headBaselineCommitted)
	split := "pkg/split_test.go"
	writeProposedFile(t, root, split, "package pkg\nfunc intentRow(t *testing.T) {\n\trequire.Equal(t, 5, got)\n\trequire.NoError(t, err)\n}\n")
	commitProposedFixture(t, root)

	// The judge records its verdict BEFORE the edit, so both sides of the
	// unit are still the committed text when the row is written.
	_, err := Approve(root, fixtureSession, "pkg.intentRow", "the judge approved the split file's change")
	ambiguous, found := errors.AsType[*rfc.AmbiguousCarrierError](err)
	if !found || len(ambiguous.Paths) != 2 {
		t.Fatalf("unqualified approval of a shared name = %v, want an ambiguity naming both files", err)
	}
	fix := ambiguousUnitFix("pkg.intentRow", ambiguous)
	for _, want := range []string{"unit pkg.intentRow file pkg/intent_test.go reason", "unit pkg.intentRow file pkg/split_test.go reason"} {
		if !strings.Contains(fix, want) {
			t.Fatalf("ambiguity fix %q lacks %q", fix, want)
		}
	}

	if _, err := ApproveIn(root, fixtureSession, "pkg.intentRow", split, "the judge approved the split file's change"); err != nil {
		t.Fatalf("qualified approval: %v", err)
	}
	rows, problems := LoadApprovals(root, WeakeningApprovals(fixtureSession))
	if len(problems) != 0 || len(rows) != 1 || rows[0].File != split || rows[0].Stale != "" {
		t.Fatalf("qualified row = %#v, %v, want one live row keeping %s", rows, problems, split)
	}

	admitted := headBaselineEdit(t, root, split, "\trequire.NoError(t, err)\n", commentedOut, false)
	if admitted.ExitCode() != 0 || len(admitted.Weakened) != 1 {
		t.Fatalf("the approved change in %s = %#v, want admitted", split, admitted)
	}
	other := headBaselineEdit(t, root, "pkg/intent_test.go", "func intentRow(t *testing.T) {\n\trequire.Equal(t, 2, got)\n}\n", "", false)
	if other.ExitCode() != 2 {
		t.Fatalf("the same name in the other file = %#v, want refused", other)
	}

	if _, err := ApproveIn(root, fixtureSession, "pkg.intentRow", "../outside_test.go", "verdict"); err == nil {
		t.Fatal("a file qualifier outside the checkout was accepted")
	}
}

// refusesTestKept fails the test unless report refuses TestKept.
func refusesTestKept(t *testing.T, step string, report ProposedReport) {
	t.Helper()
	if report.ExitCode() != 2 || len(report.Weakened) != 1 || report.Weakened[0].Name != "TestKept" {
		t.Fatalf("%s = %#v, want TestKept refused", step, report)
	}
}

// namesTheMissingSibling fails the test unless the refusal names the missing
// committed sibling it was judged against and both ways out: the judge's
// command, and stopping to report the deleted sibling to the main thread. Only
// the stable parts are matched: the path, the command and the route words.
func namesTheMissingSibling(t *testing.T, step string, report ProposedReport, sibling string) {
	t.Helper()
	text := report.Text()
	for _, want := range []string{"committed " + sibling, "/ze-test-judge", ApproveCommand("pkg.TestKept"),
		sibling + " was deleted outside this edit", "report it to the main thread"} {
		if !strings.Contains(text, want) {
			t.Fatalf("%s: refusal %q lacks %q", step, text, want)
		}
	}
}

// TestProposedJudgesASymlinkAsItsTarget proves an Edit through a symlink to a
// committed test is judged as the committed test. The link sits in another
// directory, so the sibling arm of the baseline cannot see the target: only
// the resolution in checkoutAlias can.
// Method: pkg/intent_test.go is committed; alias/link_test.go is an untracked
// symlink to it; the Edit through the link comments out a committed assertion.
// VALIDATES: repositoryPath resolves an in-checkout symlink to its target.
// PREVENTS: a symlink turning committed proof into an untracked path with no
// baseline (before checkoutAlias, the link had none and the Edit passed).
func TestProposedJudgesASymlinkAsItsTarget(t *testing.T) {
	root := headBaselineRepository(t)
	if err := os.MkdirAll(filepath.Join(root, "alias"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "pkg", "intent_test.go"), filepath.Join(root, "alias", "link_test.go")); err != nil {
		t.Fatal(err)
	}
	report := headBaselineEdit(t, root, "alias/link_test.go", "\trequire.NoError(t, err)\n", commentedOut, false)
	refusesTestKept(t, "an Edit through a symlink to the committed test", report)
	if report.Path != "pkg/intent_test.go" {
		t.Fatalf("the symlink was judged as %s, want pkg/intent_test.go", report.Path)
	}
}

// TestProposedJudgesAHardLinkAsTheCommittedFile proves an Edit through a hard
// link to a committed test, in the committed file's directory, is judged
// against the committed file's HEAD units.
// Method: pkg/intent_test.go is committed; pkg/link_test.go is an untracked
// hard link to it; the Edit through the link comments out a committed assertion.
// VALIDATES: proposedBaselineSiblings adds a present committed sibling that is
// the same file as the edited path (os.SameFile).
// PREVENTS: a second name of the committed file passing with no baseline
// (before the same-file arm, the present sibling added nothing).
func TestProposedJudgesAHardLinkAsTheCommittedFile(t *testing.T) {
	root := headBaselineRepository(t)
	if err := os.Link(filepath.Join(root, "pkg", "intent_test.go"), filepath.Join(root, "pkg", "link_test.go")); err != nil {
		t.Fatal(err)
	}
	report := headBaselineEdit(t, root, "pkg/link_test.go", "\trequire.NoError(t, err)\n", commentedOut, false)
	refusesTestKept(t, "an Edit through a hard link to the committed test", report)
	// The committed file still stands, so the refusal names it as this file
	// under another name and does not call a present file deleted.
	text := report.Text()
	for _, want := range []string{"committed pkg/intent_test.go", ApproveCommand("pkg.TestKept")} {
		if !strings.Contains(text, want) {
			t.Fatalf("hard-link refusal %q lacks %q", text, want)
		}
	}
	if strings.Contains(text, "pkg/intent_test.go was deleted outside this edit") {
		t.Fatalf("hard-link refusal %q calls a present file deleted", text)
	}
}

// TestProposedJudgesAWriteUnderANewNameAfterAShellMove proves a Write that
// creates a Go test file HEAD never held is set beside the committed test
// files of its directory the working tree lacks, unit by unit name: a shell
// `mv` of the committed test, then one Write of a weaker copy under a new
// name, is refused at the edit. The legitimate cases stay clean: a new file
// that shares no unit name with the missing one, and a copy that keeps every
// assertion.
// Method: pkg/intent_test.go is committed and renamed on disk (os.Rename, as
// `mv` does, so the index still holds the old path); pkg/renamed_test.go is
// then written whole, through the Write route, never put on disk first.
// VALIDATES: proposedRenamedFindings judges a shared unit name against the
// missing sibling's HEAD text, and only a shared name.
// PREVENTS: the early return for a path HEAD lacks letting a rename-then-
// rewrite pass the hook (before the fix, every one of these Writes was clean).
func TestProposedJudgesAWriteUnderANewNameAfterAShellMove(t *testing.T) {
	root := headBaselineRepository(t)
	if err := os.Rename(filepath.Join(root, "pkg", "intent_test.go"), filepath.Join(root, "pkg", "moved_test.go")); err != nil {
		t.Fatal(err)
	}
	write := func(path, content string) ProposedReport {
		t.Helper()
		report, err := proposedFixture(root, ProposedRequest{Path: path, Tool: "Write", ToolInput: ProposedToolInput{Content: content}})
		if err != nil {
			t.Fatalf("Write of %s: %v", path, err)
		}
		return report
	}

	weaker := write("pkg/renamed_test.go", strings.Replace(headBaselineCommitted, "\trequire.NoError(t, err)\n", commentedOut, 1))
	refusesTestKept(t, "a weaker copy written under a new name after a shell move", weaker)
	namesTheMissingSibling(t, "a weaker copy written under a new name after a shell move", weaker, "pkg/intent_test.go")

	kept := write("pkg/renamed_test.go", headBaselineCommitted)
	if kept.ExitCode() != 0 || len(kept.Weakened) != 0 {
		t.Fatalf("a whole copy written under a new name = %#v, want clean", kept)
	}
	unrelated := write("pkg/unrelated_test.go", "package pkg\nfunc TestOther(t *testing.T) {\n\t// require.NoError(t, err)\n}\n")
	if unrelated.ExitCode() != 0 || len(unrelated.Weakened) != 0 {
		t.Fatalf("a new test sharing no name with the moved one = %#v, want clean", unrelated)
	}
}

// TestProposedJudgesAnEditOfAStubAfterAShellMove proves the two-step way
// round the rename judgement is closed: after a shell `mv` of a committed
// test, a stub written under a new name that shares no unit name with it is
// clean, and a later Edit that adds a weaker copy of a moved unit to the stub
// is refused, although the stub, not HEAD, is the file's old text. An Edit
// adding the unit as committed, or stronger, stays clean.
// Method: pkg/intent_test.go is committed and renamed on disk (os.Rename, as
// `mv` does); pkg/renamed_test.go is written as a stub through the Write
// route, put on disk, then takes a whole-file Edit that adds TestKept.
// VALIDATES: proposedRenamedFindings runs for every whole-file edit of a path
// HEAD does not hold, for the units the edit changes.
// PREVENTS: a stub becoming the baseline of a moved test (before the fix, the
// Edit set the weaker TestKept beside the stub, which lacks it, and was clean).
func TestProposedJudgesAnEditOfAStubAfterAShellMove(t *testing.T) {
	root := headBaselineRepository(t)
	if err := os.Rename(filepath.Join(root, "pkg", "intent_test.go"), filepath.Join(root, "pkg", "moved_test.go")); err != nil {
		t.Fatal(err)
	}
	path := "pkg/renamed_test.go"
	// The stub carries the commented-out assertion the weaker copy later
	// takes, so the file's own count of commented assertions never rises and
	// the whole-file verdict stays silent: only the unit set beside the moved
	// committed one can see the loss.
	stubTest := "func TestStub(t *testing.T) {\n\trequire.True(t, ok)\n" + commentedOut + "}\n"
	stub := "package pkg\n" + stubTest
	written, err := proposedFixture(root, ProposedRequest{Path: path, Tool: "Write", ToolInput: ProposedToolInput{Content: stub}})
	if err != nil {
		t.Fatalf("Write of the stub: %v", err)
	}
	if written.ExitCode() != 0 || len(written.Weakened) != 0 {
		t.Fatalf("a stub sharing no name with the moved test = %#v, want clean", written)
	}
	writeProposedFile(t, root, path, stub)

	committedKept := "func TestKept(t *testing.T) {\n\trequire.NoError(t, err)\n\trequire.Equal(t, 1, got)\n\tintentRow(t)\n}\n"
	add := func(unit string) ProposedReport {
		t.Helper()
		return headBaselineEdit(t, root, path, stubTest, unit+"func TestStub(t *testing.T) {\n\trequire.True(t, ok)\n}\n", false)
	}
	weaker := add(strings.Replace(committedKept, "\trequire.NoError(t, err)\n", commentedOut, 1))
	refusesTestKept(t, "an Edit adding a weaker copy of a moved unit to a stub", weaker)
	namesTheMissingSibling(t, "an Edit adding a weaker copy of a moved unit to a stub", weaker, "pkg/intent_test.go")

	equal := add(committedKept)
	if equal.ExitCode() != 0 || len(equal.Weakened) != 0 {
		t.Fatalf("an Edit adding the moved unit as committed = %#v, want clean", equal)
	}
	stronger := add(strings.Replace(committedKept, "\tintentRow(t)\n", "\tintentRow(t)\n\trequire.Len(t, rows, 3)\n", 1))
	if stronger.ExitCode() != 0 || len(stronger.Weakened) != 0 {
		t.Fatalf("an Edit adding a stronger copy of the moved unit = %#v, want clean", stronger)
	}
}

// TestProposedJudgesAnUntrackedFileWithNoMissingSiblingAsBefore proves the
// rename judgement reaches only a directory that lacks a committed test file:
// with every committed file of the directory in place, an untracked file may
// rename a helper, delete a test, and declare a unit under a committed name
// that proves less, as before (the refusals of 2026-11-13,
// plan/journal/guard-blocks-its-own-authors-repair.md).
// Method: pkg/intent_test.go is committed and stays; pkg/split_test.go is
// untracked and takes whole-file Edits.
// VALIDATES: proposedRenamedFindings judges nothing when the baseline has no
// committed sibling.
// PREVENTS: the edit-time rename arm locking an untracked file's units
// against a committed file that still stands.
func TestProposedJudgesAnUntrackedFileWithNoMissingSiblingAsBefore(t *testing.T) {
	root := headBaselineRepository(t)
	path := "pkg/split_test.go"
	writeProposedFile(t, root, path, "package pkg\n"+
		"func TestSplit(t *testing.T) {\n\tintentRow(t, 1)\n}\n"+
		"func TestGone(t *testing.T) {\n\trequire.Equal(t, 3, got)\n}\n"+
		"func intentRow(t *testing.T, want int) {\n\trequire.Equal(t, want, got)\n}\n")

	for _, edit := range []struct {
		step, oldString, newString string
		replaceAll                 bool
	}{
		{"renaming a helper", "intentRow(", "splitRow(", true},
		{"deleting a test", "func TestGone(t *testing.T) {\n\trequire.Equal(t, 3, got)\n}\n", "", false},
		{"adding a weaker unit under a committed name", "func TestSplit(t *testing.T) {\n",
			"func TestKept(t *testing.T) {\n" + commentedOut + "}\nfunc TestSplit(t *testing.T) {\n", false},
	} {
		report := headBaselineEdit(t, root, path, edit.oldString, edit.newString, edit.replaceAll)
		if report.ExitCode() != 0 || len(report.Weakened) != 0 || report.Notice {
			t.Fatalf("%s in an untracked file with no missing sibling = %#v, want clean", edit.step, report)
		}
	}
}

// TestProposedReadsDeletedSiblingsOfTheEditedDirectoryOnly proves the
// deleted-file arm of the baseline is one directory wide: a committed test
// file deleted in another directory is another package's proof and protects
// nothing in this edit, while one deleted in the edited file's own directory
// still guards its units against a delete-and-recreate under a new name.
// Method: alpha/gone_test.go is committed and deleted from the working tree;
// an untracked file holding the same units, in beta/ and then in alpha/,
// takes a whole-file Edit that comments out a committed assertion.
// VALIDATES: proposedCommittedUnits lists HEAD for the edited directory alone.
// PREVENTS: one file deleted anywhere locking every same-named unit of every
// edit, and a hook that lists every committed file on each edit.
func TestProposedReadsDeletedSiblingsOfTheEditedDirectoryOnly(t *testing.T) {
	root := t.TempDir()
	writeProposedFile(t, root, "alpha/gone_test.go", headBaselineCommitted)
	writeProposedFile(t, root, "beta/stay_test.go", "package beta\nfunc TestStay(t *testing.T) {\n\trequire.NoError(t, err)\n}\n")
	commitProposedFixture(t, root)
	if err := os.Remove(filepath.Join(root, "alpha", "gone_test.go")); err != nil {
		t.Fatal(err)
	}

	elsewhere := "beta/new_test.go"
	writeProposedFile(t, root, elsewhere, headBaselineCommitted)
	other := headBaselineEdit(t, root, elsewhere, "\trequire.NoError(t, err)\n", commentedOut, false)
	if other.ExitCode() != 0 || len(other.Weakened) != 0 {
		t.Fatalf("a file deleted in another directory protected %s: %#v, want clean", elsewhere, other)
	}

	sibling := "alpha/new_test.go"
	writeProposedFile(t, root, sibling, headBaselineCommitted)
	same := headBaselineEdit(t, root, sibling, "\trequire.NoError(t, err)\n", commentedOut, false)
	if same.ExitCode() != 2 || len(same.Weakened) != 1 || same.Weakened[0].Name != "TestKept" {
		t.Fatalf("a file deleted in the same directory, then a lost assertion in %s = %#v, want TestKept refused", sibling, same)
	}
}

// TestProposedJudgesAWriteThatRecreatesACommittedFile proves a Write that
// creates a path the working tree lacks is judged against HEAD's text of that
// path when HEAD holds it, so deleting a committed test file and writing it
// again with fewer assertions is refused at the edit, as the commit gate
// refuses it at the commit. A Write of a path HEAD never held stays clean.
// Method: pkg/intent_test.go is committed and deleted from the working tree,
// then written whole with an assertion commented out, written whole as it was
// committed, and a new path is written beside it.
// VALIDATES: Proposed reads proposedRecreated for a Write of a missing path.
// PREVENTS: a delete-then-Write laundering a weakening past the edit hook.
func TestProposedJudgesAWriteThatRecreatesACommittedFile(t *testing.T) {
	root := headBaselineRepository(t)
	path := "pkg/intent_test.go"
	if err := os.Remove(filepath.Join(root, "pkg", "intent_test.go")); err != nil {
		t.Fatal(err)
	}
	write := func(path, content string) ProposedReport {
		t.Helper()
		report, err := proposedFixture(root, ProposedRequest{Path: path, Tool: "Write", ToolInput: ProposedToolInput{Content: content}})
		if err != nil {
			t.Fatalf("Write of %s: %v", path, err)
		}
		return report
	}

	weakened := write(path, strings.Replace(headBaselineCommitted, "\trequire.NoError(t, err)\n", commentedOut, 1))
	if weakened.ExitCode() != 2 || len(weakened.Weakened) != 1 || weakened.Weakened[0].Name != "TestKept" {
		t.Fatalf("recreating the committed file with a lost assertion = %#v, want TestKept refused", weakened)
	}
	restored := write(path, headBaselineCommitted)
	if restored.ExitCode() != 0 || len(restored.Weakened) != 0 {
		t.Fatalf("recreating the committed file as committed = %#v, want clean", restored)
	}
	fresh := write("pkg/fresh_test.go", "package pkg\nfunc TestFresh(t *testing.T) {\n\t// require.NoError(t, err)\n}\n")
	if fresh.ExitCode() != 0 || len(fresh.Weakened) != 0 {
		t.Fatalf("a Write of a path HEAD never held = %#v, want clean", fresh)
	}
}

// TestProposedRefusesWhenTheBaselineCannotBeRead proves a baseline git cannot
// read refuses the edit rather than judging against an empty baseline, which
// would unlock every unit. An unborn HEAD (a repository with no commit) is
// the failure: git cannot resolve HEAD.
// Method: `git init` with no commit, a test file on disk, then a whole-file
// Edit and a Write of a new path. The Edit half fails at the tagged-carrier
// read of the edited path, before the directory listing runs, so it proves
// nothing about the listing's error branch:
// TestProposedRefusesWhenTheDirectoryListingFails covers that one.
// VALIDATES: Proposed answers errCommittedUnreadable when HEAD is unreadable.
// PREVENTS: a failed git read read as "nothing committed".
func TestProposedRefusesWhenTheBaselineCannotBeRead(t *testing.T) {
	root := t.TempDir()
	if !runSelfTestGit(root, "init", "-q") {
		t.Fatal("git init the fixture")
	}
	path := "pkg/intent_test.go"
	writeProposedFile(t, root, path, headBaselineCommitted)
	_, err := proposedFixture(root, ProposedRequest{
		Path: path, Tool: "Edit", ToolInput: ProposedToolInput{OldString: "\trequire.NoError(t, err)\n", NewString: commentedOut},
	})
	if !errors.Is(err, errCommittedUnreadable) {
		t.Fatalf("a whole-file Edit over an unborn HEAD = %v, want errCommittedUnreadable", err)
	}
	_, err = proposedFixture(root, ProposedRequest{
		Path: "pkg/new_test.go", Tool: "Write", ToolInput: ProposedToolInput{Content: headBaselineCommitted},
	})
	if !errors.Is(err, errCommittedUnreadable) {
		t.Fatalf("a Write of a new path over an unborn HEAD = %v, want errCommittedUnreadable", err)
	}
}

// TestApprovalRowsAreKeyedOnTheirFile proves two file-qualified approvals of
// one unit name are two rows: each loads, each admits the change in its own
// file alone, and re-approving one file replaces that row and keeps the other.
// Method: pkg/intent_test.go and pkg/split_test.go both commit intentRow; the
// unit is approved in the first file, then in the second, then in the first
// again with a new verdict, and the hook judges a weakening in each file.
// VALIDATES: parseLedger keys a row on target, unit and file, and rowNames
// replaces only the row of the same file.
// PREVENTS: the second file's approval refused as a duplicate, or replacing
// the first file's row.
func TestApprovalRowsAreKeyedOnTheirFile(t *testing.T) {
	root := t.TempDir()
	first, second := "pkg/intent_test.go", "pkg/split_test.go"
	writeProposedFile(t, root, first, headBaselineCommitted)
	writeProposedFile(t, root, second, "package pkg\nfunc intentRow(t *testing.T) {\n\trequire.Equal(t, 5, got)\n\trequire.NoError(t, err)\n}\n")
	commitProposedFixture(t, root)
	weakenFirst := func() ProposedReport {
		return headBaselineEdit(t, root, first, "func intentRow(t *testing.T) {\n\trequire.Equal(t, 2, got)\n}\n", "", false)
	}
	weakenSecond := func() ProposedReport {
		return headBaselineEdit(t, root, second, "\trequire.NoError(t, err)\n", commentedOut, false)
	}

	if _, err := ApproveIn(root, fixtureSession, "pkg.intentRow", first, "first verdict"); err != nil {
		t.Fatalf("approval in %s: %v", first, err)
	}
	if report := weakenFirst(); report.ExitCode() != 0 {
		t.Fatalf("the approved change in %s = %#v, want admitted", first, report)
	}
	if report := weakenSecond(); report.ExitCode() != 2 {
		t.Fatalf("the change in %s under the %s row = %#v, want refused", second, first, report)
	}

	if _, err := ApproveIn(root, fixtureSession, "pkg.intentRow", second, "second verdict"); err != nil {
		t.Fatalf("approval in %s: %v", second, err)
	}
	reasons := func() map[string]string {
		t.Helper()
		rows, problems := LoadApprovals(root, WeakeningApprovals(fixtureSession))
		if len(problems) != 0 || len(rows) != 2 {
			t.Fatalf("rows after two qualified approvals = %#v, %v, want two", rows, problems)
		}
		byFile := make(map[string]string, len(rows))
		for _, row := range rows {
			byFile[row.File] = row.Reason
		}
		return byFile
	}
	if got := reasons(); got[first] != "first verdict" || got[second] != "second verdict" {
		t.Fatalf("rows by file = %v, want one row per file", got)
	}
	if report := weakenSecond(); report.ExitCode() != 0 {
		t.Fatalf("the approved change in %s = %#v, want admitted", second, report)
	}

	replaced, err := ApproveIn(root, fixtureSession, "pkg.intentRow", first, "first verdict again")
	if err != nil || !replaced.Replaced {
		t.Fatalf("re-approval in %s = %#v, %v, want its row replaced", first, replaced, err)
	}
	if got := reasons(); got[first] != "first verdict again" || got[second] != "second verdict" {
		t.Fatalf("rows by file after re-approving %s = %v, want only its row replaced", first, got)
	}
}

// TestApproveRefusesAQualifierOnAnUnambiguousUnit proves a file qualifier is
// accepted only for a unit the bare name cannot resolve to one carrier: on a
// unit only one file declares, the qualified resolution would read that file
// alone and drop the unit's HEAD side under another path, so the staleness
// check would stop following the text the judge read.
// Method: pkg/intent_test.go alone declares TestKept; an approval qualified
// with that very file is recorded through ApproveIn.
// VALIDATES: resolveApproval answers errUnneededQualifier and writes nothing.
// PREVENTS: a qualified row that silently loses track of a moved unit.
func TestApproveRefusesAQualifierOnAnUnambiguousUnit(t *testing.T) {
	root := headBaselineRepository(t)
	_, err := ApproveIn(root, fixtureSession, "pkg.TestKept", "pkg/intent_test.go", "verdict")
	if !errors.Is(err, errUnneededQualifier) {
		t.Fatalf("a qualified approval of an unambiguous unit = %v, want errUnneededQualifier", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(fixtureApproval))); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("the refused approval wrote the approval file (%v)", statErr)
	}
}
