// VALIDATES: debt clearing by piece writes `cleared` only once every piece of
// one cut has exited 0 over ONE commit.
// PREVENTS: the ledger's whole purpose failing under the chunking that made it
// runnable -- a row cleared by a run that judged part of the population, or by
// pieces that judged two different trees.
package commit

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// greenDebtRunner answers exit 0 for every stage, which is the verdict this
// package's decision is tested against: the question here is what the CLEARING
// does with a green piece, never whether the stages are green.
func greenDebtRunner(_ context.Context, _ string, identity verifyengine.Identity) verifyengine.ActionResult {
	return verifyengine.ActionResult{Identity: identity, Registered: true, Completed: true}
}

// debtPartProgress reads the durable record back.
func debtPartProgress(t *testing.T, root string) debtProgress {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(debtPartsPath)))
	if err != nil {
		t.Fatalf("read the progress record: %v", err)
	}
	var progress debtProgress
	if err := json.Unmarshal(content, &progress); err != nil {
		t.Fatalf("parse the progress record: %v", err)
	}
	return progress
}

// TestOnePieceOfTheVerificationClearsNoRow is the rule the ledger exists for,
// under the cut that makes the run survivable: a piece proves its own stages
// and nothing else, so the row stays open until the last piece lands.
func TestOnePieceOfTheVerificationClearsNoRow(t *testing.T) {
	root := debtClearFixture(t)

	first, code := clearDebtWith(root, debtPart{Index: 1, Of: 3}, greenDebtRunner)
	if code != 0 {
		t.Fatalf("a green piece answered %d: %#v", code, first)
	}
	if first.Cleared != 0 || first.Remaining != first.Open {
		t.Errorf("part 1 of 3 cleared %d of %d rows, want none", first.Cleared, first.Open)
	}
	if !slices.Equal(first.Proven, []int{1}) {
		t.Errorf("part 1 of 3 reports %v proven, want [1]", first.Proven)
	}
	if status := debtClearStatuses(t, root)[debtGates[0].Name]; status != "open" {
		t.Errorf("the runnable row is %q after one piece of three, want open", status)
	}

	second, _ := clearDebtWith(root, debtPart{Index: 2, Of: 3}, greenDebtRunner)
	if second.Cleared != 0 || !slices.Equal(second.Proven, []int{1, 2}) {
		t.Errorf("part 2 of 3 cleared %d rows and reports %v proven, want none and [1 2]",
			second.Cleared, second.Proven)
	}

	third, code := clearDebtWith(root, debtPart{Index: 3, Of: 3}, greenDebtRunner)
	if code != 0 {
		t.Fatalf("the last piece answered %d: %#v", code, third)
	}
	if third.Cleared != 1 || third.Remaining != 1 {
		t.Errorf("the last piece cleared %d of %d rows, want 1 with the unrunnable row remaining",
			third.Cleared, third.Open)
	}
	if status := debtClearStatuses(t, root)[debtGates[0].Name]; status != "cleared" {
		t.Errorf("the runnable row is %q after every piece passed, want cleared", status)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(debtPartsPath))); !os.IsNotExist(err) {
		t.Errorf("the progress record survived the clearing (%v), so a row written later at this HEAD would clear on it", err)
	}
}

// TestAHeadMoveDiscardsThePiecesProvenBeforeIt holds the pin. A piece's verdict
// is evidence about the tree it ran on, so pieces from two commits are not
// pieces of one verification and MUST NOT add up to a cleared row.
func TestAHeadMoveDiscardsThePiecesProvenBeforeIt(t *testing.T) {
	root := debtClearFixture(t)

	if _, code := clearDebtWith(root, debtPart{Index: 1, Of: 2}, greenDebtRunner); code != 0 {
		t.Fatalf("the first piece answered %d", code)
	}
	before := debtPartProgress(t, root)

	commitDebtFixture(t, root, "second commit")

	second, code := clearDebtWith(root, debtPart{Index: 2, Of: 2}, greenDebtRunner)
	if code != 0 {
		t.Fatalf("the piece after the HEAD move answered %d: %#v", code, second)
	}
	if second.Cleared != 0 {
		t.Errorf("pieces of two commits cleared %d row(s), want none", second.Cleared)
	}
	if !slices.Equal(second.Proven, []int{2}) {
		t.Errorf("after the HEAD move the record holds %v, want only [2]", second.Proven)
	}
	after := debtPartProgress(t, root)
	if after.Commit == before.Commit {
		t.Fatalf("the record still names %s, so the fixture never moved HEAD", after.Commit)
	}
	if status := debtClearStatuses(t, root)[debtGates[0].Name]; status != "open" {
		t.Errorf("the runnable row is %q, want open: no commit had every piece pass over it", status)
	}
}

// TestADifferentCutStartsTheRecordAgain holds the second half of the pin. Two
// cuts deal the stages differently, so a piece of one and a piece of the other
// are not a population however many of them there are.
func TestADifferentCutStartsTheRecordAgain(t *testing.T) {
	root := debtClearFixture(t)
	if _, code := clearDebtWith(root, debtPart{Index: 1, Of: 4}, greenDebtRunner); code != 0 {
		t.Fatalf("the first piece answered %d", code)
	}
	result, _ := clearDebtWith(root, debtPart{Index: 2, Of: 3}, greenDebtRunner)
	if !slices.Equal(result.Proven, []int{2}) {
		t.Errorf("a piece of a 3-way cut reports %v proven, want only [2]: the 4-way record judged another population",
			result.Proven)
	}
	if got := debtPartProgress(t, root).Of; got != 3 {
		t.Errorf("the record holds a cut of %d, want the 3 the last piece ran", got)
	}
}

// TestThePartKeywordsTakeBothOrNeither holds the grammar. "part 3" alone cannot
// say how many pieces the stages were dealt into.
func TestThePartKeywordsTakeBothOrNeither(t *testing.T) {
	uncut, err := debtPartFrom(keywordValues{})
	if err != nil || uncut != uncutDebtPart() {
		t.Errorf("no keywords answered (%#v, %v), want the uncut pass", uncut, err)
	}
	cut, err := debtPartFrom(keywordValues{"part": {"3"}, "of": {"6"}})
	if err != nil || cut != (debtPart{Index: 3, Of: 6}) {
		t.Errorf("`part 3 of 6` answered (%#v, %v)", cut, err)
	}
	for name, values := range map[string]keywordValues{
		"an index with no count": {"part": {"3"}},
		"a count with no index":  {"of": {"6"}},
		"an index past the cut":  {"part": {"7"}, "of": {"6"}},
		"a zero index":           {"part": {"0"}, "of": {"6"}},
		"a word for a number":    {"part": {"one"}, "of": {"6"}},
	} {
		if _, err := debtPartFrom(values); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

// commitDebtFixture moves the fixture checkout's HEAD.
func commitDebtFixture(t *testing.T, root, subject string) {
	t.Helper()
	for _, args := range [][]string{
		{"commit", "-q", "--allow-empty", "-m", subject},
	} {
		command := exec.CommandContext(t.Context(), "git", args...) //nolint:gosec // the argument list is the literal above
		command.Dir = root
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %s in the fixture checkout: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
}
