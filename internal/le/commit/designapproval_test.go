// VALIDATES: `./le commit create` refuses to close a spec whose Design
// documents table departs from a design page without Thomas's approval, or
// that the spec's change set never corrects; writes the Design-approved:
// trailer from his approval record once; and admits a changed decision row
// only with his words from his terminal
// (plan/release/validation/spec-design-needs-owner-approval.md, AC-11, AC-31,
// AC-32, AC-33).
// PREVENTS: a departure closing unapproved, or with the page still stating
// the decision it departed from; an approval that never reaches the other
// machine; and an agent rewriting a decision on its own.
package commit

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	docconsistency "github.com/ze-software/ze/internal/le/doc/consistency"
	"github.com/ze-software/ze/internal/le/le/human"
	"github.com/ze-software/ze/internal/le/le/human/humantest"
	specdesign "github.com/ze-software/ze/internal/le/spec/design"
	"github.com/ze-software/ze/internal/le/spec/design/designtest"
)

// The departure fixture: a spec that departs from one design page, and the
// one code file its landed work changed.
const (
	departureSpec = "plan/release/fixes/spec-departure.md"
	departurePage = "docs/architecture/widget.md"
	departureCode = "internal/departure.go"
)

// departureSpecText departs from the page's decision row.
const departureSpecText = "# Spec: departure\n\n| Field | Value |\n|-------|-------|\n| Status | in-progress |\n\n" +
	"## Task\n\nRender in the widget.\n\n## Design documents\n\n" +
	"| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |\n" +
	"|---|---|---|---|---|---|\n" +
	"| `" + departurePage + "` | Design decisions | Design header of `" + departureCode + "` | " +
	"\"The widget answers data and the surface renders it\" | departs | The widget renders its own text |\n"

// TestClosureRefusesUncorrectedDeparture is the wiring row "`./le commit
// create` closing a spec with an uncorrected `departs` page". Method: land the
// spec and its work, which never touches the page, record a clean review, and
// prepare the closure; it is refused naming the spec and the page.
//
// AC-32's precondition is an APPROVED departure, so Thomas's approval of the
// spec's text, listing its departs row, is recorded before the closure: the
// refusal judged is AC-32's and not AC-33's unapproved one.
func TestClosureRefusesUncorrectedDeparture(t *testing.T) {
	root := departureFixture(t)
	seedDepartureApproval(t, root)

	_, err := Create(root, &Options{Subject: "close the departure spec", Remove: []string{departureSpec},
		NoNews: "fixture: no operator-visible change"})
	if err == nil {
		t.Fatal("a closure whose change set never corrects the page it departs from was prepared")
	}
	want := "closure of spec-departure: it departs from " + departurePage + ", and the change set does not correct it"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("the closure was refused for another reason: %v\nwant %q", err, want)
	}
}

// departureFixture lands the departing spec and its page, then the spec's
// work, which never touches the page, and records a clean review of that
// work: a closure of it reaches the departure checks.
func departureFixture(t *testing.T) string {
	t.Helper()
	root := newCommitRepository(t)
	configureCommitAuthor(t, root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "departure-fixture")
	commitFixture(t, root, "the live spec", map[string]string{
		departureSpec: departureSpecText,
		departurePage: "# Widget\n\n## Design decisions\n\n| Decision | It forbids | Source | Date |\n|---|---|---|---|\n" +
			"| The widget answers data and the surface renders it | Rendering text in the widget | " + departureSpec + " | 2026-08-21 |\n",
	}, nil)
	commitFixture(t, root, "departure work\n\nSpec-work: departure",
		map[string]string{departureCode: "package internal\n\nconst Departure = 1\n"}, nil)
	recordFixtureReview(t, root, "spec", "departure", "verdict", "clean", "rounds", "1", "file", departureCode)
	return root
}

// seedDepartureApproval records Thomas's approval of the departing spec's
// text, which lists its one departs row.
func seedDepartureApproval(t *testing.T, root string) {
	t.Helper()
	if err := designtest.SeedApproval(root, departureSpec, departureSpecText, "2026-11-19", "the widget renders its own text"); err != nil {
		t.Fatal(err)
	}
}

// closeDeparture prepares the closure of the departing spec.
func closeDeparture(root string) (Prepared, error) {
	return Create(root, &Options{Subject: "close the departure spec", Remove: []string{departureSpec},
		NoNews: "fixture: no operator-visible change"})
}

// TestClosureRefusesUnapprovedDeparture is AC-33 and the survivors of AC-32
// and AC-33. Method: close the departing spec with no approval: refused
// naming the spec, the page and the approve command. Then record Thomas's
// approval, made during implementation, and land a correction of the page
// under the spec: the closure is prepared.
func TestClosureRefusesUnapprovedDeparture(t *testing.T) {
	root := departureFixture(t)
	_, err := closeDeparture(root)
	if err == nil {
		t.Fatal("a closure whose departure Thomas never approved was prepared")
	}
	for _, want := range []string{
		"spec-departure departs from " + departurePage + " without Thomas's approval",
		"./le spec design approve spec " + departureSpec,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the closure was refused without saying %q: %v", want, err)
		}
	}

	seedDepartureApproval(t, root)
	commitFixture(t, root, "the page states the new decision\n\nSpec-work: departure", map[string]string{
		departurePage: "# Widget\n\n## Design decisions\n\n| Decision | It forbids | Source | Date |\n|---|---|---|---|\n" +
			"| The widget renders its own text | Rendering in the surface | " + departureSpec + " | 2026-11-19 |\n",
	}, nil)
	if _, err := closeDeparture(root); err != nil {
		t.Fatalf("the closure of an approved departure whose page the work corrected was refused: %v", err)
	}
}

// TestDesignApprovedTrailerWrittenFromRecord is AC-31. Method: record
// Thomas's approval of a spec's text and prepare a commit carrying it: the
// message carries the Design-approved: line FormatApprovalTrailer spells.
// Land that line, change the spec outside the four sections, and prepare
// again: no second line. A hand-written body line starting with the key is
// refused, and prose naming the key inside a line is accepted.
func TestDesignApprovedTrailerWrittenFromRecord(t *testing.T) {
	// The carried spec is in-progress, so the commit also owes its Existing
	// Mechanisms analysis (specMechanismsRefusal); the section lies outside
	// the four the design digest covers.
	carried := departureSpecText + "\n## Existing Mechanisms\n\n| Mechanism | Where | What it does today | This spec |\n" +
		"|---|---|---|---|\n| The widget | `" + departureCode + "` | Answers data | Extends: renders text |\n"
	root := newCommitRepository(t)
	configureCommitAuthor(t, root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "design-approved-fixture")
	writeCommitFixture(t, root, departureSpec, carried)
	if err := designtest.SeedApproval(root, departureSpec, carried, "2026-11-19", "the widget renders its own text"); err != nil {
		t.Fatal(err)
	}
	want := specdesign.FormatApprovalTrailer(specdesign.Approval{Stem: "spec-departure",
		Digest: specdesign.DesignDigest(carried), Date: "2026-11-19", Words: "the widget renders its own text"})

	prepared, err := Create(root, &Options{Subject: "carry the approved spec", Files: []string{departureSpec}})
	if err != nil {
		t.Fatalf("the commit carrying an approved spec was refused: %v", err)
	}
	message, err := os.ReadFile(filepath.Join(root, prepared.Message))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(message), want+"\n") {
		t.Fatalf("the message does not carry %q:\n%s", want, message)
	}

	commitFixture(t, root, "carry the approved spec\n\n"+want, map[string]string{departureSpec: carried}, nil)
	writeCommitFixture(t, root, departureSpec, carried+"\n## Implementation Steps\n\n1. Render.\n")
	prepared, err = Create(root, &Options{Subject: "bookkeeping", Files: []string{departureSpec}})
	if err != nil {
		t.Fatalf("the second commit carrying the spec was refused: %v", err)
	}
	message, err = os.ReadFile(filepath.Join(root, prepared.Message))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(message), specdesign.DesignApprovedTrailer) {
		t.Fatalf("a second Design-approved: line was written after one landed:\n%s", message)
	}

	if _, err := Message("a subject", []string{want}, nil); err == nil {
		t.Error("a hand-written Design-approved: body line was accepted")
	}
	if _, err := Message("a subject", []string{"this commit records the " + specdesign.DesignApprovedTrailer + "line"}, nil); err != nil {
		t.Errorf("prose naming the key inside a line was refused: %v", err)
	}
}

// The decision-change fixture: a page whose one decision row a commit
// rewrites, the record the row cites, and Thomas's words.
const (
	decisionRecord = "plan/records/widget.md"
	decisionWords  = "Thomas: the widget renders its own text from now on"
)

// decisionPage is the fixture page holding one decision row.
func decisionPage(decision string) string {
	return "# Widget\n\n## Design decisions\n\n| Decision | It forbids | Source | Date |\n|---|---|---|---|\n" +
		"| " + decision + " | Rendering text in the widget | " + decisionRecord + " | 2026-08-21 |\n"
}

// TestDecisionChangeNeedsTheOwnerTerminal is AC-11. Method: commit a page with
// one decision row, rewrite the row, and prepare the commit three ways.
// Without decision-change the decision-row check refuses it. With
// decision-change and piped streams the owner terminal refuses it, naming the
// command for Thomas, and writes nothing. From a terminal the commit is
// prepared and carries `Decision-changed: <page>: <his words>`. A body line
// that starts with the key is refused as hand-written.
func TestDecisionChangeNeedsTheOwnerTerminal(t *testing.T) {
	prepare := func(t *testing.T, words string) (string, Prepared, error) {
		t.Helper()
		root := newCommitRepository(t)
		configureCommitAuthor(t, root)
		t.Setenv("CLAUDE_CODE_SESSION_ID", "decision-change-fixture")
		commitFixture(t, root, "the page and its record", map[string]string{
			decisionRecord: "# Widget record\n",
			departurePage:  decisionPage("The widget answers data and the surface renders it"),
		}, nil)
		writeCommitFixture(t, root, departurePage, decisionPage("The widget answers data and renders it too"))
		before := humantest.Snapshot(t, root)
		prepared, err := Create(root, &Options{Subject: "Thomas changes the widget decision",
			Files: []string{departurePage}, DecisionChange: words})
		if _, refused := errors.AsType[*human.Refusal](err); refused && !maps.Equal(before, humantest.Snapshot(t, root)) {
			t.Error("the refused owner keyword changed files or directories")
		}
		return root, prepared, err
	}

	t.Run("no keyword", func(t *testing.T) {
		humantest.Streams(t, "piped-both")
		_, _, err := prepare(t, "")
		if err == nil || !strings.Contains(err.Error(), "the decision of 2026-08-21 is changed") {
			t.Fatalf("a commit rewriting a decision row with no owner words answered %v", err)
		}
	})

	t.Run("piped", func(t *testing.T) {
		humantest.Streams(t, "piped-both")
		_, _, err := prepare(t, decisionWords)
		if _, refused := errors.AsType[*human.Refusal](err); !refused {
			t.Fatalf("decision-change without a terminal answered %v, want the owner terminal's refusal", err)
		}
		for _, want := range []string{"./le commit create", keywordDecisionChange, decisionWords} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the refusal does not name %q: %v", want, err)
			}
		}
		if code := exitCodeOf(err); code != 2 {
			t.Errorf("the refusal exits %d, want 2", code)
		}
	})

	t.Run("terminal", func(t *testing.T) {
		humantest.Streams(t, "terminal")
		root, prepared, err := prepare(t, decisionWords)
		if err != nil {
			t.Fatalf("decision-change from a terminal was refused: %v", err)
		}
		message, err := os.ReadFile(filepath.Join(root, prepared.Message))
		if err != nil {
			t.Fatal(err)
		}
		want := docconsistency.DecisionChangedTrailer + departurePage + ": " + decisionWords
		if !strings.Contains(string(message), want) {
			t.Fatalf("the message does not carry %q:\n%s", want, message)
		}
	})

	if _, err := Message("a subject", []string{docconsistency.DecisionChangedTrailer + departurePage + ": forged"}, nil); err == nil {
		t.Error("a hand-written Decision-changed: body line was accepted")
	}
}
