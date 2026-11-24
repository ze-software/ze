// VALIDATES: `le spec design carry` carries Thomas's approval of a design to
// a later text only when the change writes in his own decisions: a row added
// for a row of a page's Design decisions section, or a quote that follows a
// rename on its page, with the judge's approval of the later text. Every other
// change is refused by name and writes no record. The carried approval is the
// one the lookup, the admission and the owed trailer answer, in the checkout
// that carried it and, through the trailer, in any other.
// PREVENTS: a design Thomas did not see starting under his earlier approval,
// and his own rulings being put back to him for approval.
package specdesign_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/le/human/humantest"
	specdesign "github.com/ze-software/ze/internal/le/spec/design"
	"github.com/ze-software/ze/internal/le/spec/design/designtest"
)

// The carry fixture: the widget spec Thomas approved, with one row the files
// do not owe (so a test can remove or change it and still pass the check), a
// second decision the page gains after the approval, and the page's first
// decision as a rename leaves it.
const (
	carriedRecord   = "tmp/spec-design/carried/spec-widget.md"
	carryWords      = "approved, C-11 (a)"
	carryDate       = "2026-11-19"
	wireRow         = "| `" + widgetPage + "` | Wire format | Design header of `" + widgetCode + "` | \"The answer is JSON.\" | agrees | JSON stays |\n"
	doorRowStart    = "| `" + widgetDoor + "` |"
	secondDecision  = "The widget keeps no cache"
	secondRow       = "| `" + widgetPage + "` | Design decisions | Decision of 2026-11-24 | \"" + secondDecision + "\" | agrees | No cache is added |\n"
	renamedDecision = "The gadget answers data and the surface renders it"
)

// carryBaseText is the text Thomas approved.
var carryBaseText = withRow(completeSpecText, wireRow)

// withRow answers text with row added to its Design documents table, before
// the door row.
func withRow(text, row string) string {
	return strings.Replace(text, doorRowStart, row+doorRowStart, 1)
}

// pageWithSecondDecision is the widget page once Thomas ruled a second time.
var pageWithSecondDecision = strings.Replace(widgetPageText, "|\n\n## Wire format",
	"|\n| "+secondDecision+" | Caching in the widget | "+widgetRecord+" | 2026-11-24 |\n\n## Wire format", 1)

// pageRenamed is the widget page once a rename reached its decision row.
var pageRenamed = strings.Replace(widgetPageText, widgetDecision, renamedDecision, 1)

// commitApproval commits the widget spec as text with the Design-approved:
// line of Thomas's approval of it, as ./le commit create writes it.
func commitApproval(t *testing.T, root, text string) {
	t.Helper()
	writeTree(t, root, map[string]string{widgetSpec: text})
	trailer := specdesign.FormatApprovalTrailer(specdesign.Approval{Stem: "spec-widget",
		Digest: specdesign.DesignDigest(text), Date: carryDate, Words: carryWords})
	commitSpec(t, root, trailer)
}

// commitSpec commits the widget spec as the checkout holds it, with trailer.
func commitSpec(t *testing.T, root, trailer string) {
	t.Helper()
	for _, args := range [][]string{{"add", "--", widgetSpec}, {"commit", "-q", "-m", "the spec\n\n" + trailer}} {
		if err := designtest.Git(root, args...); err != nil {
			t.Fatal(err)
		}
	}
}

// runCarry writes the widget spec as text and runs the carry action over it.
func runCarry(t *testing.T, root, text string) (string, int) {
	t.Helper()
	writeTree(t, root, map[string]string{widgetSpec: text})
	var answer any
	var code int
	stderr := captureStderr(t, func() {
		answer, code = specdesign.Answer([]string{"carry", "spec", widgetSpec})
	})
	if answer == nil {
		return stderr, code
	}
	return answerText(t, answer) + stderr, code
}

// approvedRoot is a widget checkout whose history carries Thomas's approval
// of carryBaseText.
func approvedRoot(t *testing.T) string {
	t.Helper()
	root := widgetRoot(t)
	gitInit(t, root)
	commitApproval(t, root, carryBaseText)
	return root
}

// TestCarryAllowsOnlyHisOwnDecisions is the carry's whole judgment, through
// the command. Method: each case starts from a history that carries Thomas's
// approval of carryBaseText, changes the page and the spec, records the
// judge's verdict as the case says, and runs `carry`. The two allowed changes
// exit 0, write the carried record, and the lookup then answers his approval
// for the new text. Every other change exits 2 naming what goes back to him,
// writes no record, and leaves the new text unapproved.
func TestCarryAllowsOnlyHisOwnDecisions(t *testing.T) {
	const judgedThisText = "this text"
	departing := strings.Replace(strings.Replace(withRow(carryBaseText, secondRow),
		"| agrees | No cache is added |", "| departs | A cache is added |", 1),
		"### Documents departed from\n\nnone\n",
		"### Documents departed from\n\n`"+widgetPage+"`: before, no cache; after, a cache.\n", 1)
	cases := []struct {
		name string
		page string
		text string
		// judged is the text the judge's approve record names: judgedThisText
		// for the case's own text, "" for no record.
		judged  string
		verdict string
		code    int
		wants   []string
	}{
		{"a row added for a decision of his", pageWithSecondDecision, withRow(carryBaseText, secondRow),
			judgedThisText, "approve", 0, []string{`"status":"carried"`, `"rows-added":1`, `"quotes-renamed":0`}},
		{"a quote that follows a rename on its page", pageRenamed, strings.Replace(carryBaseText, widgetDecision, renamedDecision, 1),
			judgedThisText, "approve", 0, []string{`"status":"carried"`, `"rows-added":0`, `"quotes-renamed":1`}},
		{"the Task changed", widgetPageText, strings.Replace(carryBaseText, "Render the widget in the surface.", "Render it twice.", 1),
			judgedThisText, "approve", 2, []string{"## Task changed since the approval", "these changes go back to Thomas", "./le spec design approve spec " + widgetSpec}},
		{"a criterion changed", widgetPageText, strings.Replace(carryBaseText, "the surface renders it |", "the surface renders it twice |", 1),
			judgedThisText, "approve", 2, []string{"## Acceptance Criteria changed since the approval"}},
		{"the presentation changed", widgetPageText, strings.Replace(carryBaseText, "The surface renders the widget.", "The surface renders two widgets.", 1),
			judgedThisText, "approve", 2, []string{"## Design presentation changed since the approval"}},
		{"a row removed", widgetPageText, completeSpecText,
			judgedThisText, "approve", 2, []string{"Wire format) was removed, or changed in more than its quoted statement"}},
		{"a departs row added", pageWithSecondDecision, departing,
			judgedThisText, "approve", 2, []string{`gives the verdict "departs"`, "## Design presentation changed since the approval"}},
		{"a verdict changed", widgetPageText, strings.Replace(carryBaseText, "| agrees | JSON stays |", "| silent | JSON stays |", 1),
			judgedThisText, "approve", 2, []string{"Wire format) was removed, or changed in more than its quoted statement"}},
		{"how the design stands changed", widgetPageText, strings.Replace(carryBaseText, "| agrees | JSON stays |", "| agrees | JSON goes |", 1),
			judgedThisText, "approve", 2, []string{"Wire format) was removed, or changed in more than its quoted statement"}},
		{"a quote changed with no rename on the page", widgetPageText, strings.Replace(carryBaseText, "\"The answer is JSON.\"", "\"The answer is\"", 1),
			judgedThisText, "approve", 2, []string{"still holds the approved one, so the change is no rename on the page"}},
		{"a quote the page does not hold", widgetPageText, strings.Replace(carryBaseText, "\"The answer is JSON.\"", "\"The answer is YAML.\"", 1),
			judgedThisText, "approve", 2, []string{"is not ready to carry an approval", "not a verbatim span"}},
		{"a row added that is no decision of his", widgetPageText, withRow(carryBaseText, strings.Replace(wireRow, "The answer is JSON.", "The answer is", 1)),
			judgedThisText, "approve", 2, []string{"is not about a row of a page's ## Design decisions section"}},
		{"no judge record of the new text", pageWithSecondDecision, withRow(carryBaseText, secondRow),
			"", "", 2, []string{"is not ready to carry an approval", "no judge has judged this design"}},
		{"the judge judged another text", pageWithSecondDecision, withRow(carryBaseText, secondRow),
			carryBaseText, "approve", 2, []string{"is not ready to carry an approval", "the judge judged another text"}},
		{"the judge refused the new text", pageWithSecondDecision, withRow(carryBaseText, secondRow),
			judgedThisText, "refuse", 2, []string{"is not ready to carry an approval", "the judge refused this design"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := approvedRoot(t)
			writeTree(t, root, map[string]string{widgetPage: tc.page})
			judged := tc.judged
			if judged == judgedThisText {
				judged = tc.text
			}
			if judged != "" {
				writeJudgeRecord(t, root, judged, tc.verdict)
			}
			output, code := runCarry(t, root, tc.text)
			if code != tc.code {
				t.Fatalf("carry answered %d, want %d:\n%s", code, tc.code, output)
			}
			requireContains(t, "the carry output", output, tc.wants...)
			approval, found, err := specdesign.Approved(root, "spec-widget", tc.text)
			if err != nil {
				t.Fatal(err)
			}
			if tc.code != 0 {
				requireAbsent(t, root, carriedRecord)
				if found {
					t.Errorf("a refused carry left the text approved: %+v", approval)
				}
				return
			}
			if !found {
				t.Fatal("the carried text is not approved")
			}
			if approval.CarriedFrom != specdesign.DesignDigest(carryBaseText) || approval.Words != carryWords || approval.Date != carryDate {
				t.Errorf("the lookup answers %+v, want Thomas's approval of %s carried from the approved digest", approval, carryDate)
			}
		})
	}
}

// TestCarryReadsTheCommitWhenTheRecordKeptNoText is the carry of an approval
// whose local record has no copy of the approved text beside it, as every
// record written before the copy existed. Method: the history carries the
// approval of carryBaseText and the checkout holds the record of the same
// digest with no text; the carry reads the text from the commit and passes.
func TestCarryReadsTheCommitWhenTheRecordKeptNoText(t *testing.T) {
	root := approvedRoot(t)
	if err := designtest.SeedApproval(root, widgetSpec, carryBaseText, carryDate, carryWords); err != nil {
		t.Fatal(err)
	}
	text := withRow(carryBaseText, secondRow)
	writeTree(t, root, map[string]string{widgetPage: pageWithSecondDecision})
	writeJudgeRecord(t, root, text, "approve")
	output, code := runCarry(t, root, text)
	if code != 0 {
		t.Fatalf("carry answered %d, want 0:\n%s", code, output)
	}
	requireContains(t, "the carry answer", output, `"status":"carried"`, `"rows-added":1`)
}

// TestCarryRefusesWithNoApprovalOfHis is the refusal of a carry with nothing
// to carry. Method: a judged, complete spec in a history with no approval
// exits 2 naming that Thomas approved no text, and writes no record.
func TestCarryRefusesWithNoApprovalOfHis(t *testing.T) {
	root := widgetRoot(t)
	gitInit(t, root)
	writeJudgeRecord(t, root, completeSpecText, "approve")
	output, code := runCarry(t, root, completeSpecText)
	if code != 2 {
		t.Fatalf("carry answered %d, want 2:\n%s", code, output)
	}
	requireContains(t, "the carry refusal", output, "Thomas approved no text of spec-widget.md", "./le spec design approve spec "+widgetSpec)
	requireAbsent(t, root, carriedRecord)
}

// TestCarryReachesEveryGate is the end-to-end path. Method: the history
// carries Thomas's approval of carryBaseText; the page gains a decision and
// the spec its row. Before the carry the admission refuses and names the
// carry command. After the judge's approval and `carry`, the admission admits
// and reports the carry, and the commit owes a Design-approved: line that
// names the approved digest, the judge's sentence and his words. Once a
// commit carries that line the commit owes nothing more, and with the local
// record gone (another checkout) the lookup answers the same carried approval
// from the trailer. A second carry then starts from the carried one and keeps
// his words.
func TestCarryReachesEveryGate(t *testing.T) {
	root := approvedRoot(t)
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	baseDigest := specdesign.DesignDigest(carryBaseText)
	text := withRow(carryBaseText, secondRow)
	writeTree(t, root, map[string]string{widgetPage: pageWithSecondDecision, widgetSpec: text})

	admission, err := specdesign.Admit(root, widgetSpec, text, now)
	if err != nil {
		t.Fatal(err)
	}
	if admission.Admitted {
		t.Fatal("the changed text is admitted before any carry")
	}
	requireContains(t, "the admission refusal", strings.Join(admission.Refusal(widgetSpec), "\n"),
		"./le spec design carry spec "+widgetSpec, "./le spec design approve spec "+widgetSpec)

	writeJudgeRecord(t, root, text, "approve")
	if output, code := runCarry(t, root, text); code != 0 {
		t.Fatalf("carry answered %d, want 0:\n%s", code, output)
	}
	admission, err = specdesign.Admit(root, widgetSpec, text, now)
	if err != nil {
		t.Fatal(err)
	}
	if !admission.Admitted {
		t.Fatal("the carried text is not admitted")
	}
	requireContains(t, "the admission report", admission.Report(), carryDate, carryWords, "carried on", "every verdict holds")

	owed, err := specdesign.OwedApprovalTrailer(root, widgetSpec, text, now)
	if err != nil {
		t.Fatal(err)
	}
	requireContains(t, "the owed trailer", owed,
		"Design-approved: spec-widget "+specdesign.DesignDigest(text)+": carried "+baseDigest+" ",
		` judge "every verdict holds" approved `+carryDate+" "+carryWords)
	commitSpec(t, root, owed)
	owed, err = specdesign.OwedApprovalTrailer(root, widgetSpec, text, now)
	if err != nil {
		t.Fatal(err)
	}
	if owed != "" {
		t.Fatalf("a second line is owed after the carried one landed: %q", owed)
	}

	if err := os.RemoveAll(filepath.Join(root, "tmp", "spec-design", "carried")); err != nil {
		t.Fatal(err)
	}
	approval, found, err := specdesign.Approved(root, "spec-widget", text)
	if err != nil || !found {
		t.Fatalf("the carried trailer was not found: found %v, err %v", found, err)
	}
	if approval.CarriedFrom != baseDigest || approval.Words != carryWords || approval.Date != carryDate ||
		approval.JudgeSentence != "every verdict holds" || approval.Commit == "" {
		t.Fatalf("the lookup answers %+v, want the carried approval from the trailer", approval)
	}

	renamed := strings.Replace(text, widgetDecision, renamedDecision, 1)
	writeTree(t, root, map[string]string{widgetPage: strings.Replace(pageWithSecondDecision, widgetDecision, renamedDecision, 1)})
	writeJudgeRecord(t, root, renamed, "approve")
	output, code := runCarry(t, root, renamed)
	if code != 0 {
		t.Fatalf("the second carry answered %d, want 0:\n%s", code, output)
	}
	requireContains(t, "the second carry", output, `"carried-from":"`+specdesign.DesignDigest(text)+`"`,
		`"owner-words":"`+carryWords+`"`, `"quotes-renamed":1`)
}

// TestCarryStartsFromAnApprovalNoCommitHolds is the carry of an approval that
// exists only as Thomas's local record. Method: from a real terminal he
// approves carryBaseText, which keeps the approved text beside the record;
// with no commit made, the page gains a decision and the spec its row, and
// the carry passes from that copy. With the copy removed, nothing holds the
// text he approved, so the carry refuses and names that.
func TestCarryStartsFromAnApprovalNoCommitHolds(t *testing.T) {
	root := widgetRoot(t)
	gitInit(t, root)
	humantest.Streams(t, "terminal")
	writeJudgeRecord(t, root, carryBaseText, "approve")
	if output, code := runApprove(t, root, carryBaseText, carryWords); code != 0 {
		t.Fatalf("approve answered %d, want 0:\n%s", code, output)
	}
	text := withRow(carryBaseText, secondRow)
	writeTree(t, root, map[string]string{widgetPage: pageWithSecondDecision})
	writeJudgeRecord(t, root, text, "approve")

	kept := filepath.Join(root, "tmp", "spec-design", "approved", "spec-widget.text")
	body, err := os.ReadFile(kept)
	if err != nil {
		t.Fatalf("the approval kept no copy of the approved text: %v", err)
	}
	if err := os.Remove(kept); err != nil {
		t.Fatal(err)
	}
	output, code := runCarry(t, root, text)
	if code != 2 {
		t.Fatalf("carry with no copy of the approved text answered %d, want 2:\n%s", code, output)
	}
	requireContains(t, "the carry refusal", output, "the text he approved is not kept", "./le spec design approve spec "+widgetSpec)
	requireAbsent(t, root, carriedRecord)

	if err := os.WriteFile(kept, body, 0o600); err != nil {
		t.Fatal(err)
	}
	output, code = runCarry(t, root, text)
	if code != 0 {
		t.Fatalf("carry from the local approval answered %d, want 0:\n%s", code, output)
	}
	requireContains(t, "the carry answer", output, `"status":"carried"`, `"owner-words":"`+carryWords+`"`, `"rows-added":1`)
	if _, found, err := specdesign.Approved(root, "spec-widget", text); err != nil || !found {
		t.Fatalf("the carried text is not approved: found %v, err %v", found, err)
	}
}
