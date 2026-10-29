// VALIDATES: AC-7, AC-8 and AC-5's never-complete empty set. The
// mandatory figure is proven rows over gated rows minus {rollup}; a
// not-applicable row stays in the denominator; a tagged row without a verified
// proof, a stale proof, an escape and a {gap} row are all unproven; a
// {single-polarity} row is proven by its verified record and by nothing else; the
// optional figure counts the same way over the other levels. An unwalked RFC
// renders "not walked", and a rollup holding one, or holding no RFC, is never
// complete. No absent state exists.
// PREVENTS: a figure that counts tags rather than recorded breaks, an
// annotation that raises a figure, and a profile that reads complete while a
// member is unwalked or while it has no member.

package rfc

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// coverageFixture is one walked RFC carrying every row shape the figures
// distinguish, and one unwalked RFC.
func coverageFixture() ([]Requirement, []DiscriminationVerdict) {
	row := func(rid, level string, kind string) Requirement {
		out := Requirement{RFC: "rfc1", RID: rid, Level: level}
		if kind != "" {
			out.Annotation = &Annotation{Kind: kind, Reason: "fixture"}
		}
		return out
	}
	requirements := []Requirement{
		row("RFC1-1-1", levelMust, ""),                        // proven
		row("RFC1-1-2", levelMust, ""),                        // tagged, no record
		row("RFC1-1-3", levelMust, ""),                        // stale record
		row("RFC1-1-4", levelMust, ""),                        // no-break escape
		row("RFC1-1-5", levelMust, AnnotationNotApplicable),   // denominator only, its record proves nothing
		row("RFC1-1-6", levelMust, AnnotationRollup),          // out of both
		row("RFC1-1-7", levelMust, AnnotationGap),             // a record cannot prove a gap
		row("RFC1-1-8", levelMust, AnnotationLowerLayer),      // denominator only, its record proves nothing
		row("RFC1-1-9", levelMust, AnnotationSinglePolarity),  // proven by its verified record
		row("RFC1-1-10", levelMust, AnnotationSinglePolarity), // no record: the annotation alone proves nothing
		row("RFC1-2-1", levelShould, ""),                      // optional, proven
		row("RFC1-2-2", "MAY", ""),                            // optional, unproven
		{RFC: "rfc2", RID: "RFC2-1-1", Level: levelMust},
	}
	verdict := func(rid, route, state string) DiscriminationVerdict {
		return DiscriminationVerdict{Record: DiscriminationRecord{RID: rid, Route: route}, State: state}
	}
	verdicts := []DiscriminationVerdict{
		verdict("RFC1-1-1", RouteRevert, ProofVerified),
		verdict("RFC1-1-3", RouteRevert, "producer-changed"),
		verdict("RFC1-1-4", RouteNoBreak, ProofVerified),
		verdict("RFC1-1-5", RouteRevert, ProofVerified),
		verdict("RFC1-1-6", RouteRevert, ProofVerified),
		verdict("RFC1-1-7", RouteRevert, ProofVerified),
		verdict("RFC1-1-8", RouteRevert, ProofVerified),
		verdict("RFC1-1-9", RouteRevert, ProofVerified),
		verdict("RFC1-2-1", RouteMutant, ProofVerified),
		verdict("RFC2-1-1", RouteRevert, ProofVerified),
	}
	return requirements, verdicts
}

func TestCoverageMandatoryOptional(t *testing.T) {
	requirements, verdicts := coverageFixture()
	stems := map[string]bool{"rfc1": true, "rfc2": true, "rfc3": true}
	walked := map[string]bool{"rfc1": true, "rfc3": true}
	figures := figuresByStem(requirements, stems, walked, provenRequirements(verdicts))

	one := figures["rfc1"]
	if !reflect.DeepEqual(one.Mandatory, Figure{Proven: 2, Total: 9, Attainable: 7,
		Excluded: map[string]int{AnnotationNotApplicable: 1, AnnotationLowerLayer: 1}}) {
		t.Errorf("mandatory = %+v, want 2 proven of 9 owed", one.Mandatory)
	}
	if !reflect.DeepEqual(one.Optional, Figure{Proven: 1, Total: 2, Attainable: 2}) {
		t.Errorf("optional = %+v, want 1 proven of 2 owed", one.Optional)
	}
	if got := one.mandatoryCell(); got != "2 of 9 (22.2%)" {
		t.Errorf("mandatory cell = %q", got)
	}

	// A {single-polarity} row is owed either way, and proven only by its
	// verified record: the annotation alone proves nothing.
	proven := provenRequirements(verdicts)
	for index := range requirements {
		requirement := &requirements[index]
		if requirement.Annotation == nil || requirement.Annotation.Kind != AnnotationSinglePolarity {
			continue
		}
		owed, met := rowCounts(requirement, proven)
		want := requirement.RID == "RFC1-1-9"
		if !owed || met != want {
			t.Errorf("%s: owed=%v proven=%v, want owed and proven=%v", requirement.RID, owed, met, want)
		}
	}

	// AC-8: a proven row on an unwalked RFC still renders no figure.
	two := figures["rfc2"]
	if two.mandatoryCell() != notWalked || two.optionalCell() != notWalked {
		t.Errorf("unwalked cells = %q, %q, want %q", two.mandatoryCell(), two.optionalCell(), notWalked)
	}
	// A walked RFC with no row renders no percentage, never 0% or 100%.
	if got := figures["rfc3"].mandatoryCell(); got != "no rows" {
		t.Errorf("empty walked cell = %q, want no rows", got)
	}

	rollup := rollupOf(figures, []string{"rfc1", "rfc2", "rfc3"})
	want := Rollup{RFCs: 3, NotWalked: 1,
		Mandatory: Figure{Proven: 2, Total: 9, Attainable: 7,
			Excluded: map[string]int{AnnotationNotApplicable: 1, AnnotationLowerLayer: 1}},
		Optional: Figure{Proven: 1, Total: 2, Attainable: 2}}
	if !reflect.DeepEqual(rollup, want) {
		t.Errorf("rollup = %+v, want %+v", rollup, want)
	}
	for _, phrase := range []string{"3 RFCs", "1 not walked"} {
		if !strings.Contains(rollup.String(), phrase) {
			t.Errorf("rollup %q does not state %q", rollup.String(), phrase)
		}
	}
}

// TestRollupNeverCompleteWithUnwalkedOrEmpty: a set whose walked rows are all
// proven is complete only when no member is unwalked and the set has a member:
// 0 of 0 over no RFC is not a measurement.
func TestRollupNeverCompleteWithUnwalkedOrEmpty(t *testing.T) {
	figures := map[string]StemFigures{
		"rfc1": {Stem: "rfc1", Walked: true, Mandatory: Figure{Proven: 3, Total: 3}},
		"rfc2": {Stem: "rfc2"},
	}
	if !rollupOf(figures, []string{"rfc1"}).mandatoryComplete() {
		t.Error("a fully proven walked set is not complete")
	}
	if rollupOf(figures, []string{"rfc1", "rfc2"}).mandatoryComplete() {
		t.Error("a set with an unwalked RFC reads complete")
	}
	if rollupOf(figures, nil).mandatoryComplete() {
		t.Error("a set with no member reads complete")
	}
}

// TestRollupHasNoAbsentState: the absent state is deleted (owner, "all RFC
// should have a valid summary"). Method: a rollup and a tree row, encoded and
// rendered, carry no `absent` key and no "absent at 0%" text.
func TestRollupHasNoAbsentState(t *testing.T) {
	rollup := ScopeRollup{RFCs: 1, NotWalked: 1}
	row := &TreeRow{Stem: "rfc1", Summary: true}
	for label, value := range map[string]any{"rollup": rollup, "row": row} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), `"absent"`) {
			t.Errorf("%s JSON carries an absent key: %s", label, encoded)
		}
	}
	if text := rollup.String(); strings.Contains(text, "absent") {
		t.Errorf("rollup text %q still speaks of absent RFCs", text)
	}
}

// TestALinuxLimitLowersTheCeilingAndNotTheFigure reclassifies gated and optional
// gaps through the priority tree, preserving proof counts while lowering ceilings.
func TestALinuxLimitLowersTheCeilingAndNotTheFigure(t *testing.T) {
	base := priorityFixture(t, nil)
	requirements := []Requirement{
		{RFC: "rfc1", RID: "RFC1-1-1", Level: levelMust},
		{RFC: "rfc1", RID: "RFC1-1-2", Level: levelMust, Annotation: &Annotation{Kind: AnnotationGap}},
		{RFC: "rfc1", RID: "RFC1-2-1", Level: levelShould},
		{RFC: "rfc1", RID: "RFC1-2-2", Level: levelShould, Annotation: &Annotation{Kind: AnnotationGap}},
	}
	stems := map[string]bool{"rfc1": true}
	proven := map[string]bool{"RFC1-1-1": true, "RFC1-1-2": true, "RFC1-2-1": true, "RFC1-2-2": true}
	before := figuresByStem(requirements, stems, stems, proven)["rfc1"]
	requirements[1].Annotation = &Annotation{Kind: AnnotationLinuxLimit, LimitID: "LL-FIXTURE"}
	requirements[3].Annotation = &Annotation{Kind: AnnotationLinuxLimit, LimitID: "LL-FIXTURE"}
	figures := figuresByStem(requirements, stems, stems, proven)
	after := figures["rfc1"]
	for _, pair := range [][2]Figure{{before.Mandatory, after.Mandatory}, {before.Optional, after.Optional}} {
		if pair[0].Proven != 1 || pair[0].Total != 2 || pair[1].Proven != 1 || pair[1].Total != 2 {
			t.Fatalf("classification changed the proof figure: before %+v, after %+v", pair[0], pair[1])
		}
		if pair[0].Attainable != 2 || pair[1].Attainable != 1 || pair[1].Excluded[AnnotationLinuxLimit] != 1 {
			t.Fatalf("classification did not lower only the ceiling: before %+v, after %+v", pair[0], pair[1])
		}
	}
	corpus := newPriorityCorpus(base.graph, base.profiles, base.metas, requirements, figures, proven)
	report, err := corpus.tree("")
	if err != nil {
		t.Fatal(err)
	}
	var row *TreeRow
	for _, candidate := range report.Rows {
		if candidate.Stem == "rfc1" {
			row = candidate
		}
	}
	if row == nil {
		t.Fatal("the tree lost rfc1")
	}
	encoded, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"mandatory-proven":1`, `"mandatory-total":2`, `"mandatory-attainable":1`,
		`"optional-proven":1`, `"optional-total":2`, `"optional-attainable":1`,
		`"mandatory-excluded":{"linux-limit":1}`, `"optional-excluded":{"linux-limit":1}`} {
		if !strings.Contains(string(encoded), key) {
			t.Errorf("JSON lacks %s: %s", key, encoded)
		}
	}
	var decoded TreeRow
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	mandatory, optional := decoded.CeilingCells()
	if mandatory != "1 of 2 (50.0%)" || optional != mandatory {
		t.Errorf("decoded ceiling cells = %q, %q", mandatory, optional)
	}
	if !strings.Contains(report.Text(), "ceiling mandatory 1 of 2 (50.0%), optional 1 of 2 (50.0%)") {
		t.Errorf("tree text lacks its measured ceiling:\n%s", report.Text())
	}
}

// TestCeilingPartitionsEveryAnnotation holds the trait-driven arithmetic against
// explicit expected exclusions, with proofs present even on annotations.
func TestCeilingPartitionsEveryAnnotation(t *testing.T) {
	excluded := map[string]bool{AnnotationLinuxLimit: true, AnnotationLowerLayer: true,
		AnnotationFeatureDeclined: true, AnnotationNotApplicable: true}
	for _, kind := range append(AnnotationKinds(), "", "unknown-kind") {
		t.Run(kind, func(t *testing.T) {
			req := Requirement{RFC: "rfc1", RID: "RFC1-1-1", Level: levelMust}
			if kind != "" {
				req.Annotation = &Annotation{Kind: kind}
			}
			stems := map[string]bool{"rfc1": true}
			figure := figuresByStem([]Requirement{req}, stems, stems, map[string]bool{req.RID: true})["rfc1"].Mandatory
			total, attainable, proven := 1, 1, 0
			if kind == AnnotationRollup {
				total, attainable = 0, 0
			}
			if excluded[kind] {
				attainable = 0
				if figure.Excluded[kind] != 1 || len(figure.Excluded) != 1 {
					t.Errorf("exclusions = %v, want only %s:1", figure.Excluded, kind)
				}
			} else if len(figure.Excluded) != 0 {
				t.Errorf("attainable annotation excluded rows: %v", figure.Excluded)
			}
			if kind == "" || kind == AnnotationSinglePolarity {
				proven = 1
			}
			if figure.Total != total || figure.Attainable != attainable || figure.Proven != proven {
				t.Errorf("figure = %+v, want proven=%d total=%d attainable=%d", figure, proven, total, attainable)
			}
			if figure.Proven > figure.Attainable || figure.Attainable > figure.Total {
				t.Errorf("figure violates proven <= attainable <= total: %+v", figure)
			}
		})
	}
}

// TestTheRollupCeilingSumsWalkedMembersOnly checks unequal RFC sizes and all four
// exclusion reasons against both profile aggregation and the site's row view.
func TestTheRollupCeilingSumsWalkedMembersOnly(t *testing.T) {
	base := priorityFixture(t, nil)
	requirements := []Requirement{
		{RFC: "rfc1", RID: "RFC1-1-1", Level: levelMust, Annotation: &Annotation{Kind: AnnotationLinuxLimit}},
		{RFC: "rfc2", RID: "RFC2-1-1", Level: levelMust, Annotation: &Annotation{Kind: AnnotationLowerLayer}},
		{RFC: "rfc2", RID: "RFC2-1-2", Level: levelMust, Annotation: &Annotation{Kind: AnnotationFeatureDeclined}},
		{RFC: "rfc2", RID: "RFC2-1-3", Level: levelMust, Annotation: &Annotation{Kind: AnnotationNotApplicable}},
		{RFC: "rfc2", RID: "RFC2-1-4", Level: levelMust, Annotation: &Annotation{Kind: AnnotationGap}},
		{RFC: "rfc2", RID: "RFC2-1-5", Level: levelMust, Annotation: &Annotation{Kind: AnnotationPartial}},
		{RFC: "rfc2", RID: "RFC2-2-1", Level: levelShould},
		{RFC: "rfc2", RID: "RFC2-2-2", Level: levelShould, Annotation: &Annotation{Kind: AnnotationLinuxLimit}},
		{RFC: "rfc3", RID: "RFC3-1-1", Level: levelMust, Annotation: &Annotation{Kind: AnnotationLinuxLimit}},
		{RFC: "rfc3", RID: "RFC3-2-1", Level: levelShould, Annotation: &Annotation{Kind: AnnotationLowerLayer}},
	}
	stems := map[string]bool{"rfc1": true, "rfc2": true, "rfc3": true, "rfc4": true}
	walked := map[string]bool{"rfc1": true, "rfc2": true, "rfc4": true}
	figures := figuresByStem(requirements, stems, walked, nil)
	want := Rollup{RFCs: 4, NotWalked: 1,
		Mandatory: Figure{Total: 6, Attainable: 2, Excluded: map[string]int{
			AnnotationLinuxLimit: 1, AnnotationLowerLayer: 1, AnnotationFeatureDeclined: 1, AnnotationNotApplicable: 1}},
		Optional: Figure{Total: 2, Attainable: 1, Excluded: map[string]int{AnnotationLinuxLimit: 1}}}
	got := rollupOf(figures, []string{"rfc1", "rfc2", "rfc3", "rfc4"})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rollup = %+v, want %+v", got, want)
	}
	if text := got.Mandatory.CeilingString(); text != "2 of 6 (33.3%)" {
		t.Errorf("weighted ceiling = %q, want 33.3%%, not the 20%% average of 0/1 and 2/5", text)
	}
	corpus := newPriorityCorpus(base.graph, base.profiles, base.metas, requirements, figures, nil)
	rows := []*TreeRow{corpus.treeRow("rfc1", nil), corpus.treeRow("rfc2", nil),
		corpus.treeRow("rfc3", nil), corpus.treeRow("rfc4", nil)}
	summary := CeilingOf(rows)
	if !reflect.DeepEqual(summary.Rollup, want) {
		t.Errorf("view = %+v, want profile counts %+v", summary.Rollup, want)
	}
	if summary.LowestMandatory == nil || *summary.LowestMandatory != (StemCeiling{Stem: "rfc1", Total: 1}) {
		t.Errorf("lowest mandatory = %+v, want rfc1 at 0 of 1", summary.LowestMandatory)
	}
	if summary.LowestOptional == nil || *summary.LowestOptional != (StemCeiling{Stem: "rfc2", Total: 2, Attainable: 1}) {
		t.Errorf("lowest optional = %+v, want rfc2 at 1 of 2", summary.LowestOptional)
	}
	mandatory, optional := rows[2].CeilingCells()
	if mandatory != notWalked || optional != notWalked {
		t.Errorf("unwalked ceiling cells = %q, %q", mandatory, optional)
	}
	mandatory, optional = rows[3].CeilingCells()
	if mandatory != "no rows" || optional != "no rows" {
		t.Errorf("empty ceiling cells = %q, %q", mandatory, optional)
	}
	// Aggregating the same source again must not mutate per-RFC exclusions.
	if repeated := rollupOf(figures, []string{"rfc1", "rfc2", "rfc3", "rfc4"}); !reflect.DeepEqual(repeated, want) {
		t.Errorf("aggregation mutated source maps: %+v", repeated)
	}
}

// TestCeilingMinimumHasNoInventedMeasurement checks empty, unwalked and
// summary-less views, then tied fractions in reverse RFC order.
func TestCeilingMinimumHasNoInventedMeasurement(t *testing.T) {
	rows := []*TreeRow{
		{Stem: "rfc1", Summary: true, Walked: true},
		{Stem: "rfc2", Summary: true, MandatoryTotal: 9},
		{Stem: "rfc3", Walked: true, MandatoryTotal: 9},
	}
	summary := CeilingOf(rows)
	if summary.LowestMandatory != nil || summary.LowestOptional != nil || summary.Mandatory.Total != 0 {
		t.Fatalf("unmeasured view invented a ceiling: %+v", summary)
	}
	if summary.Mandatory.CeilingString() != "no rows" || (Figure{}).CeilingString() != "no rows" {
		t.Error("zero denominator invented a percentage")
	}
	mandatory, optional := rows[2].CeilingCells()
	if mandatory != noSummaryCell || optional != noSummaryCell {
		t.Errorf("summary-less cells = %q, %q", mandatory, optional)
	}
	tied := CeilingOf([]*TreeRow{
		{Stem: "rfc10", Summary: true, Walked: true, MandatoryTotal: 4, MandatoryAttainable: 2},
		{Stem: "rfc2", Summary: true, Walked: true, MandatoryTotal: 2, MandatoryAttainable: 1},
	})
	if tied.LowestMandatory == nil || tied.LowestMandatory.Stem != "rfc2" {
		t.Errorf("tie = %+v, want rfc2 in RFC-number order", tied.LowestMandatory)
	}
}

// TestCeilingRenderingBoundaries checks no rows, fully attainable, fully
// excluded and mixed figures without treating the ceiling as current proof.
func TestCeilingRenderingBoundaries(t *testing.T) {
	for _, one := range []struct {
		name    string
		figure  Figure
		ceiling string
		proof   string
	}{
		{"empty", Figure{}, "no rows", "no rows"},
		{"none excluded", Figure{Total: 4, Attainable: 4}, "4 of 4 (100.0%)", "0 of 4 (0.0%)"},
		{"all excluded", Figure{Total: 4}, "0 of 4 (0.0%)", "0 of 4 (0.0%)"},
		{"mixed", Figure{Proven: 1, Total: 4, Attainable: 3}, "3 of 4 (75.0%)", "1 of 4 (25.0%)"},
	} {
		t.Run(one.name, func(t *testing.T) {
			if got := one.figure.CeilingString(); got != one.ceiling {
				t.Errorf("ceiling = %q, want %q", got, one.ceiling)
			}
			if got := one.figure.String(); got != one.proof {
				t.Errorf("proof = %q, want %q", got, one.proof)
			}
		})
	}
}

// TestCeilingTextSeparatesExclusions renders both text paths and ensures an
// unwalked row's retained raw counts never escape as measurements.
func TestCeilingTextSeparatesExclusions(t *testing.T) {
	row := &TreeRow{Stem: "rfc1", Summary: true, Walked: true,
		MandatoryTotal: 2, MandatoryAttainable: 1, MandatoryExcluded: map[string]int{AnnotationLinuxLimit: 1},
		OptionalTotal: 3, OptionalAttainable: 1, OptionalExcluded: map[string]int{AnnotationLowerLayer: 2}}
	report := TreeReport{Rows: []*TreeRow{row}}
	summary := CeilingOf(report.Rows)
	for label, text := range map[string]string{"tree": report.Text(), "rollup": summary.Rollup.String()} {
		for _, phrase := range []string{"mandatory excluded: linux-limit 1", "optional excluded: lower-layer 2"} {
			if !strings.Contains(text, phrase) {
				t.Errorf("%s text lacks %q: %s", label, phrase, text)
			}
		}
	}
	row.Walked = false
	text := report.Text()
	if strings.Contains(text, "%") || strings.Contains(text, "excluded:") {
		t.Errorf("unwalked raw counts escaped into text: %s", text)
	}
	if !strings.Contains(text, "ceiling mandatory not walked, optional not walked") {
		t.Errorf("unwalked ceiling state missing: %s", text)
	}
}
