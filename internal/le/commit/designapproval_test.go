// VALIDATES: `./le commit create` refuses to close a spec whose Design
// documents table departs from a design page that the spec's change set never
// corrects (plan/release/validation/spec-design-needs-owner-approval.md,
// AC-32).
// PREVENTS: a departure Thomas approved closing with the page still stating
// the decision it departed from.
package commit

import (
	"strings"
	"testing"
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
// AC-32's precondition is an APPROVED departure. The approval record's writer
// is built in phase 5 of the spec; phase 6, which builds this closure check,
// seeds that record here before the closure, so the refusal judged is AC-32's
// and not AC-33's unapproved one.
func TestClosureRefusesUncorrectedDeparture(t *testing.T) {
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
