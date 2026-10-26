package rfc

// VALIDATES: `./le rfc next` over the priority fixture (AC-4, AC-9, and the
// level order). The fixture is priorityFixture in tree_test.go.
// PREVENTS: a gap named at the wrong level, a cycle named by half, a gap
// reached over a citation rather than declared, an optional member answered
// as the profile's gap.

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// TestNextActionNamesLowestUncoveredLevel: the gap is the lowest level of the
// required members holding an uncovered row, a `{updated-by}` row owed at its
// updater's level, a citation cycle named whole, and the optional register
// once the mandatory one is complete. Method: the fixture corpus with a
// growing proven set; each step names where the next gap moves.
func TestNextActionNamesLowestUncoveredLevel(t *testing.T) {
	// rfc1's only uncovered row carries {updated-by: RFC4-1-1}, so it is owed
	// at rfc4's level, 1, where rfc1 sorts first.
	corpus := priorityFixture(t, map[string]bool{"RFC1-1-1": true})
	report, err := corpus.next("edge")
	if err != nil {
		t.Fatalf("next edge: %v", err)
	}
	if report.Register != registerMandatory || report.Level == nil || *report.Level != 1 {
		t.Fatalf("next edge = %s at %v, want mandatory at level 1", report.Register, report.Level)
	}
	if len(report.Gaps) != 1 || report.Gaps[0].Stem != "rfc1" {
		t.Fatalf("gaps %+v, want rfc1 alone", report.Gaps)
	}
	gap := report.Gaps[0]
	if len(gap.Uncovered) != 1 || gap.Uncovered[0].ID != "RFC1-2-1" || gap.Uncovered[0].UpdatedBy != "RFC4-1-1" {
		t.Errorf("rfc1 uncovered %+v, want RFC1-2-1 updated by RFC4-1-1", gap.Uncovered)
	}
	if gap.Path != "edge: required by rfc/short/rfc1.md" {
		t.Errorf("rfc1 path %q", gap.Path)
	}

	// Across the release profiles (core, edge; lab is not one), the gap names
	// every profile requiring it: core requires rfc2 alone, so rfc1 serves edge.
	across, err := corpus.next("")
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if !slices.Equal(across.Profiles, []string{"core", "edge"}) {
		t.Errorf("profiles judged %v, want the release ones", across.Profiles)
	}
	if len(across.Gaps) != 1 || !slices.Equal(across.Gaps[0].Profiles, []string{"edge"}) {
		t.Errorf("gaps %+v, want rfc1 serving edge", across.Gaps)
	}

	// With levels 0 and 1 proven, the unwalked rfc5 is the gap; its cycle
	// partner rfc6 is no member, so the cycle is named and rfc6 owes nothing.
	cycle := priorityFixture(t, map[string]bool{"RFC1-1-1": true, "RFC1-2-1": true, "RFC2-1-1": true,
		"RFC2-1-2": true, "RFC4-1-1": true}, "rfc5")
	report, err = cycle.next("edge")
	if err != nil {
		t.Fatalf("next edge: %v", err)
	}
	if !slices.Equal(report.Component, []string{"rfc5", "rfc6"}) || len(report.Gaps) != 1 ||
		report.Gaps[0].Stem != "rfc5" || report.Gaps[0].Walked {
		t.Fatalf("component %v gaps %+v, want the unwalked rfc5 alone", report.Component, report.Gaps)
	}

	// lab's mandatory row is proven, so its next gap is optional.
	labCorpus := priorityFixture(t, map[string]bool{"RFC9-1-1": true})
	report, err = labCorpus.next("lab")
	if err != nil {
		t.Fatalf("next lab: %v", err)
	}
	if !report.MandatoryComplete || report.Register != registerOptional {
		t.Fatalf("next lab = %s, complete %v, want optional after a complete mandatory figure",
			report.Register, report.MandatoryComplete)
	}
	if len(report.Gaps) != 1 || report.Gaps[0].Uncovered[0].ID != "RFC9-1-2" {
		t.Errorf("gaps %+v, want RFC9-1-2", report.Gaps)
	}
	if !strings.Contains(report.Text(), "mandatory is complete") {
		t.Errorf("text does not say mandatory is complete:\n%s", report.Text())
	}
}

// TestNextIgnoresUndeclaredCitations: AC-4 for `next`. Core requires rfc2,
// which cites rfc1 and which rfc7 updates; both owe unproven rows and neither
// is listed for core. Proving rfc2's rows completes core, and neither rfc1
// nor rfc7 is ever a gap. Method: next core before and after rfc2 is proven.
func TestNextIgnoresUndeclaredCitations(t *testing.T) {
	report, err := priorityFixture(t, map[string]bool{}).next("core")
	if err != nil {
		t.Fatalf("next core: %v", err)
	}
	if len(report.Gaps) != 1 || report.Gaps[0].Stem != "rfc2" {
		t.Fatalf("gaps %+v, want rfc2 alone", report.Gaps)
	}
	report, err = priorityFixture(t, map[string]bool{"RFC2-1-1": true, "RFC2-1-2": true}).next("core")
	if err != nil {
		t.Fatalf("next core: %v", err)
	}
	if !report.Complete || report.Rollup.RFCs != 1 {
		t.Errorf("next core = %+v, want complete over rfc2 alone", report)
	}
}

// TestNextOptionalSectionSeparate: AC-9. The answered gap and the rollup come
// from required members only; an optional member owing rows is listed in its
// own "optional, not counted" section with its own rollup, under its own JSON
// key. Method: edge with every required row proven, rfc9 optional and unproven.
func TestNextOptionalSectionSeparate(t *testing.T) {
	proven := map[string]bool{"RFC1-1-1": true, "RFC1-2-1": true, "RFC2-1-1": true, "RFC2-1-2": true,
		"RFC3-1-1": true, "RFC4-1-1": true, "RFC5-1-1": true}
	report, err := priorityFixture(t, proven).next("edge")
	if err != nil {
		t.Fatalf("next edge: %v", err)
	}
	if !report.Complete || report.Rollup.RFCs != 5 || !report.Rollup.MandatoryComplete {
		t.Errorf("next edge complete %v rollup %+v, want complete over the five required RFCs",
			report.Complete, report.Rollup)
	}
	if report.Optional.Rollup.RFCs != 1 || len(report.Optional.Gaps) != 1 || report.Optional.Gaps[0].Stem != "rfc9" {
		t.Fatalf("optional section %+v, want rfc9 alone", report.Optional)
	}
	if got := report.Optional.Gaps[0].Uncovered; len(got) != 2 {
		t.Errorf("rfc9 uncovered %+v, want RFC9-1-1 and RFC9-1-2", got)
	}
	text := report.Text()
	for _, want := range []string{"optional, not counted", "rfc9", "edge: optional in rfc/short/rfc9.md"} {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q:\n%s", want, text)
		}
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"optional":{"rollup":`) {
		t.Errorf("json lacks the optional section: %s", encoded)
	}
}
