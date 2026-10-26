package rfc

// VALIDATES: `./le rfc next` over the priority fixture (AC-10, AC-11, AC-12,
// A-4, R-7). The fixture is priorityFixture in tree_test.go.
// PREVENTS: a gap named at the wrong level, a cycle named by half, a
// mandatory figure read complete while an RFC is absent.

import (
	"slices"
	"strings"
	"testing"
)

// TestNextActionNamesLowestUncoveredLevel holds AC-10, AC-11, AC-12 and A-4.
// Method: the fixture corpus with a growing proven set; each step names
// where the next gap moves.
func TestNextActionNamesLowestUncoveredLevel(t *testing.T) {
	// AC-10: rfc1's only uncovered row carries {updated-by: RFC4-1-1}, so it
	// is owed at rfc4's level, 1, where rfc1 sorts first.
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
	if gap.Path != "edge: rfc3 builds-on rfc2 builds-on rfc1" {
		t.Errorf("rfc1 path %q", gap.Path)
	}

	// AC-11: across the release profiles (core, edge; lab is not one), the
	// gap names every profile it serves.
	across, err := corpus.next("")
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if !slices.Equal(across.Profiles, []string{"core", "edge"}) {
		t.Errorf("profiles judged %v, want the release ones", across.Profiles)
	}
	if len(across.Gaps) != 1 || !slices.Equal(across.Gaps[0].Profiles, []string{"core", "edge"}) {
		t.Errorf("gaps %+v, want rfc1 serving core and edge", across.Gaps)
	}
	if !slices.Equal(across.Absent, []string{"rfc7948"}) {
		t.Errorf("absent %v, want rfc7948", across.Absent)
	}

	// A-4: with levels 0 and 1 proven, the cycle rfc5 <-> rfc6 is named whole.
	cycle := priorityFixture(t, map[string]bool{"RFC1-1-1": true, "RFC1-2-1": true, "RFC2-1-1": true,
		"RFC2-1-2": true, "RFC4-1-1": true}, "rfc5")
	report, err = cycle.next("edge")
	if err != nil {
		t.Fatalf("next edge: %v", err)
	}
	if !slices.Equal(report.Component, []string{"rfc5", "rfc6"}) || len(report.Gaps) != 2 {
		t.Fatalf("component %v gaps %+v, want rfc5 and rfc6", report.Component, report.Gaps)
	}
	if report.Gaps[0].Walked || report.Gaps[1].Summary {
		t.Errorf("rfc5 walked %v, rfc6 summary %v, want both owed", report.Gaps[0].Walked, report.Gaps[1].Summary)
	}

	// AC-12: lab's mandatory row is proven, so its next gap is optional.
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

	// R-7: an absent RFC keeps the mandatory figure incomplete and is named
	// once nothing in the corpus owes a mandatory row.
	report, err = labCorpus.next("far")
	if err != nil {
		t.Fatalf("next far: %v", err)
	}
	if report.MandatoryComplete || len(report.Gaps) != 1 || !report.Gaps[0].Absent || report.Gaps[0].Stem != "rfc8212" {
		t.Errorf("next far = %+v, want the absent rfc8212 as the mandatory gap", report)
	}
}
