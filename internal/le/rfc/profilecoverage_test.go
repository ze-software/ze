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
	"slices"
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
	figures := figuresByStem(requirements, stems, walked, provenRequirements(verdicts), figureEvidence{exclusions: acceptEveryExclusion{}})

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
		Optional: Figure{Proven: 1, Total: 2, Attainable: 2},
		Sections: Sections{
			Must: MustSections{Total: 9, Kept: 8, ZeProven: 2, OSGap: 1, ZeGap: 5, NotOwed: 1,
				NotOwedByKind: map[string]int{AnnotationNotApplicable: 1}},
			Should: ShouldSections{Total: 2, Proven: 1, NotDecided: 1, NotOwedByKind: map[string]int{}}}}
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
		"rfc1": {Stem: "rfc1", Walked: true, Mandatory: Figure{Proven: 3, Total: 3, Attainable: 3}},
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

// TestALinuxLimitMovesARowToNotOwedAndNotTheFigure reclassifies gated and
// optional gaps through the priority tree, preserving proof counts while
// moving the rows from the Ze correctness gap to not owed, in the figures, the
// tree JSON and the tree text.
func TestALinuxLimitMovesARowToNotOwedAndNotTheFigure(t *testing.T) {
	base := priorityFixture(t, nil)
	requirements := []Requirement{
		{RFC: "rfc1", RID: "RFC1-1-1", Level: levelMust},
		{RFC: "rfc1", RID: "RFC1-1-2", Level: levelMust, Annotation: &Annotation{Kind: AnnotationGap}},
		{RFC: "rfc1", RID: "RFC1-2-1", Level: levelShould},
		{RFC: "rfc1", RID: "RFC1-2-2", Level: levelShould, Annotation: &Annotation{Kind: AnnotationGap}},
	}
	stems := map[string]bool{"rfc1": true}
	proven := map[string]bool{"RFC1-1-1": true, "RFC1-1-2": true, "RFC1-2-1": true, "RFC1-2-2": true}
	before := figuresByStem(requirements, stems, stems, proven, figureEvidence{exclusions: acceptEveryExclusion{}})["rfc1"]
	requirements[1].Annotation = &Annotation{Kind: AnnotationLinuxLimit, LimitID: "LL-FIXTURE"}
	requirements[3].Annotation = &Annotation{Kind: AnnotationLinuxLimit, LimitID: "LL-FIXTURE"}
	figures := figuresByStem(requirements, stems, stems, proven, figureEvidence{exclusions: acceptEveryExclusion{}})
	after := figures["rfc1"]
	for _, pair := range [][2]Figure{{before.Mandatory, after.Mandatory}, {before.Optional, after.Optional}} {
		if pair[0].Proven != 1 || pair[0].Total != 2 || pair[1].Proven != 1 || pair[1].Total != 2 {
			t.Fatalf("classification changed the proof figure: before %+v, after %+v", pair[0], pair[1])
		}
		if pair[0].Attainable != 2 || pair[1].Attainable != 1 || pair[1].Excluded[AnnotationLinuxLimit] != 1 {
			t.Fatalf("classification did not move only the owed rows: before %+v, after %+v", pair[0], pair[1])
		}
	}
	corpus := newPriorityCorpus(base.graph, base.profiles, base.metas, requirements, figures, proven,
		figureEvidence{exclusions: acceptEveryExclusion{}})
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
	for _, key := range []string{`"mandatory-proven":1`, `"mandatory-total":2`,
		`"optional-proven":1`, `"optional-total":2`,
		`"must":{"total":2,"kept":1,"ze-proven":1,"os-verified":0,"os-gap":0,"ze-gap":0,"not-owed":1,"not-owed-by-kind":{"linux-limit":1}}`,
		`"should":{"total":2,"proven":1,"os-verified":0,"os-gap":0,"not-owed":1,"not-ever":0,"not-yet":0,"not-decided":0,"not-owed-by-kind":{"linux-limit":1}}`} {
		if !strings.Contains(string(encoded), key) {
			t.Errorf("JSON lacks %s: %s", key, encoded)
		}
	}
	var decoded TreeRow
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Sections == nil || !reflect.DeepEqual(*decoded.Sections, *row.Sections) {
		t.Errorf("decoded sections = %+v, want %+v", decoded.Sections, row.Sections)
	}
	want := "MUST 1 kept: Ze proven 1, OS verified 0, OS testing gap 0, Ze correctness gap 0; MUST not owed 1, linux-limit 1"
	if !strings.Contains(report.Text(), want) {
		t.Errorf("tree text lacks %q:\n%s", want, report.Text())
	}
	if strings.Contains(report.Text(), "ceiling") || strings.Contains(report.Text(), "%") {
		t.Errorf("tree text still states a ceiling or a percentage:\n%s", report.Text())
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
			figure := figuresByStem([]Requirement{req}, stems, stems, map[string]bool{req.RID: true}, figureEvidence{exclusions: acceptEveryExclusion{}})["rfc1"].Mandatory
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

// TestTheRollupSectionsSumWalkedMembersOnly checks unequal RFC sizes and all
// four exclusion reasons: a rollup sums the walked members' figures and
// sections as counts, an unwalked member adds nothing, and the row view
// publishes no sections for it.
func TestTheRollupSectionsSumWalkedMembersOnly(t *testing.T) {
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
	figures := figuresByStem(requirements, stems, walked, nil, figureEvidence{exclusions: acceptEveryExclusion{}})
	want := Rollup{RFCs: 4, NotWalked: 1,
		Mandatory: Figure{Total: 6, Attainable: 2, Excluded: map[string]int{
			AnnotationLinuxLimit: 1, AnnotationLowerLayer: 1, AnnotationFeatureDeclined: 1, AnnotationNotApplicable: 1}},
		Optional: Figure{Total: 2, Attainable: 1, Excluded: map[string]int{AnnotationLinuxLimit: 1}},
		Sections: Sections{
			Must: MustSections{Total: 6, Kept: 3, OSGap: 1, ZeGap: 2, NotOwed: 3, NotOwedByKind: map[string]int{
				AnnotationLinuxLimit: 1, AnnotationFeatureDeclined: 1, AnnotationNotApplicable: 1}},
			Should: ShouldSections{Total: 2, NotOwed: 1, NotDecided: 1,
				NotOwedByKind: map[string]int{AnnotationLinuxLimit: 1}}}}
	got := rollupOf(figures, []string{"rfc1", "rfc2", "rfc3", "rfc4"})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rollup = %+v, want %+v", got, want)
	}
	corpus := newPriorityCorpus(base.graph, base.profiles, base.metas, requirements, figures, nil,
		figureEvidence{exclusions: acceptEveryExclusion{}})
	if row := corpus.treeRow("rfc3", nil); row.Sections != nil || sectionsCell(row) != notWalked {
		t.Errorf("unwalked row published sections %+v, cell %q", row.Sections, sectionsCell(row))
	}
	if row := corpus.treeRow("rfc2", nil); row.Sections == nil || row.Sections.Must.Kept != 3 {
		t.Errorf("walked row sections = %+v, want rfc2's 3 kept MUST rows", row.Sections)
	}
	// Aggregating the same source again must not mutate per-RFC exclusions.
	if repeated := rollupOf(figures, []string{"rfc1", "rfc2", "rfc3", "rfc4"}); !reflect.DeepEqual(repeated, want) {
		t.Errorf("aggregation mutated source maps: %+v", repeated)
	}
}

// TestRegisterCompleteNeedsNoZeOrOSGap checks AC-9: the mandatory register is
// complete only when the MUST bar holds no Ze correctness gap and no OS testing
// gap. The method is one walked RFC per bar shape: Ze proven alone, Ze proven
// beside an OS verified row and a row not owed (complete), one Ze gap, and one
// OS gap (each incomplete, though neither is Ze proven).
func TestRegisterCompleteNeedsNoZeOrOSGap(t *testing.T) {
	lowerLayer := map[string]int{AnnotationLowerLayer: 1}
	figures := map[string]StemFigures{
		"proven": {Stem: "proven", Walked: true, Mandatory: Figure{Proven: 2, Total: 2, Attainable: 2}},
		"stack": {Stem: "stack", Walked: true, OSVerified: 1,
			Mandatory: Figure{Proven: 1, Total: 3, Attainable: 1,
				Excluded: map[string]int{AnnotationLowerLayer: 1, AnnotationNotApplicable: 1}}},
		"ze-gap": {Stem: "ze-gap", Walked: true, Mandatory: Figure{Proven: 1, Total: 2, Attainable: 2}},
		"os-gap": {Stem: "os-gap", Walked: true, OSGap: 1,
			Mandatory: Figure{Proven: 1, Total: 2, Attainable: 1, Excluded: lowerLayer}},
	}
	for _, one := range []struct {
		members  []string
		complete bool
	}{
		{[]string{"proven"}, true},
		{[]string{"proven", "stack"}, true},
		{[]string{"proven", "ze-gap"}, false},
		{[]string{"proven", "os-gap"}, false},
	} {
		rollup := rollupOf(figures, one.members)
		if got := rollup.mandatoryComplete(); got != one.complete {
			t.Errorf("%v: mandatoryComplete = %v, want %v (sections %+v)", one.members, got, one.complete,
				rollup.Sections.Must)
		}
	}
}

// TestSectionsCellHasNoInventedMeasurement checks that a summary-less row, an
// unwalked row, a walked row with no rows and a rollup with no rows name their
// state rather than print a
// bar of zeros.
func TestSectionsCellHasNoInventedMeasurement(t *testing.T) {
	if cell := sectionsCell(&TreeRow{Stem: "rfc3", Walked: true, Mandatory: Figure{Total: 9}}); cell != noSummaryCell {
		t.Errorf("summary-less cell = %q, want %q", cell, noSummaryCell)
	}
	if cell := sectionsCell(&TreeRow{Stem: "rfc2", Summary: true, Mandatory: Figure{Total: 9}}); cell != notWalked {
		t.Errorf("unwalked cell = %q, want %q", cell, notWalked)
	}
	if cell := sectionsCell(&TreeRow{Stem: "rfc4", Summary: true, Walked: true, Sections: &Sections{}}); cell != noRowsCell {
		t.Errorf("walked cell with no rows = %q, want %q", cell, noRowsCell)
	}
	if text := (Rollup{RFCs: 1, NotWalked: 1}).String(); text != "no rows; 1 RFCs, 1 not walked" {
		t.Errorf("empty rollup = %q, want no rows and its member counts", text)
	}
}

// TestFigureRenderingBoundaries checks no rows, none proven and mixed proof
// figures: a zero total renders no percentage.
func TestFigureRenderingBoundaries(t *testing.T) {
	for _, one := range []struct {
		name   string
		figure Figure
		proof  string
	}{
		{"empty", Figure{}, "no rows"},
		{"none proven", Figure{Total: 4, Attainable: 4}, "0 of 4 (0.0%)"},
		{"mixed", Figure{Proven: 1, Total: 4, Attainable: 3}, "1 of 4 (25.0%)"},
	} {
		t.Run(one.name, func(t *testing.T) {
			if got := one.figure.String(); got != one.proof {
				t.Errorf("proof = %q, want %q", got, one.proof)
			}
		})
	}
}

// TestSectionsTextSeparatesNotOwedKinds renders both text paths and ensures
// each not-owed kind and each OS row is named apart, and that an unwalked
// row's retained raw counts never escape as measurements.
func TestSectionsTextSeparatesNotOwedKinds(t *testing.T) {
	figures := StemFigures{Stem: "rfc1", Walked: true,
		Mandatory:     Figure{Total: 2, Attainable: 1, Excluded: map[string]int{AnnotationLinuxLimit: 1}},
		Optional:      Figure{Total: 3, Attainable: 1, Excluded: map[string]int{AnnotationLowerLayer: 2}},
		OptionalOSGap: 2, NotDecided: 1}
	sections, err := SectionsOf("rfc1", &figures)
	if err != nil {
		t.Fatal(err)
	}
	row := &TreeRow{Stem: "rfc1", Summary: true, StemFigures: figures, Sections: &sections}
	report := TreeReport{Rows: []*TreeRow{row}}
	rollup := Rollup{RFCs: 1, Sections: sections}
	for label, text := range map[string]string{"tree": report.Text(), "rollup": rollup.String()} {
		for _, phrase := range []string{"MUST not owed 1, linux-limit 1", "Ze correctness gap 1",
			"SHOULD 3: proven 0, OS verified 0, OS testing gap 2", "not decided 1", "SHOULD not owed 0"} {
			if !strings.Contains(text, phrase) {
				t.Errorf("%s text lacks %q: %s", label, phrase, text)
			}
		}
	}
	row.Walked, row.Sections = false, nil
	text := report.Text()
	if strings.Contains(text, "%") || strings.Contains(text, "not owed") {
		t.Errorf("unwalked raw counts escaped into text: %s", text)
	}
	if !strings.Contains(text, "rfc1  not walked") {
		t.Errorf("unwalked state missing: %s", text)
	}
}

// TestStemFiguresCountProvenByEvidenceKind checks that each proven mandatory
// row lands under the STRONGEST kind of the tags covering it (interop >
// functional > editor > unit), that an unproven row lands nowhere, and that
// the kinds sum to the proven count.
//
// The method gives one row a unit tag and an interop tag, listed unit first,
// so a version that took the first tag, or ranked by the reading order
// carrierKindOrder declares, answers unit. A proven row whose only tag no
// carrier claims must still be counted, under kindUnknown, or the sum falls
// short in silence.
func TestStemFiguresCountProvenByEvidenceKind(t *testing.T) {
	carriers := []Carrier{
		{Name: "unit", Kind: kindUnit, Tier: tierVerify, Prefix: "internal/component/", Suffix: "_test.go"},
		{Name: "functional", Kind: kindFunctional, Tier: tierVerify, Prefix: "test/plugin/", Suffix: ciSuffix},
		{Name: "editor", Kind: kindEditor, Tier: tierVerify, Prefix: "test/editor/", Suffix: etSuffix},
		{Name: "interop", Kind: kindInterop, Tier: tierNightly, Prefix: "test/interop/", Suffix: ciSuffix},
	}
	requirements := []Requirement{
		{RFC: "rfc1", RID: "RFC1-1-1", Level: levelMust},   // unit and interop: interop
		{RFC: "rfc1", RID: "RFC1-1-2", Level: levelMust},   // editor and functional: functional
		{RFC: "rfc1", RID: "RFC1-1-3", Level: levelMust},   // editor alone
		{RFC: "rfc1", RID: "RFC1-1-4", Level: levelMust},   // unit alone
		{RFC: "rfc1", RID: "RFC1-1-5", Level: levelMust},   // no carrier claims its tag
		{RFC: "rfc1", RID: "RFC1-1-6", Level: levelMust},   // interop tag, no record: not proven
		{RFC: "rfc1", RID: "RFC1-1-7", Level: levelShould}, // optional: never in the mandatory kinds
	}
	tags := []Tag{
		{RID: "RFC1-1-1", File: "internal/component/a/a_test.go"},
		{RID: "RFC1-1-1", File: "test/interop/a.ci"},
		{RID: "RFC1-1-2", File: "test/editor/b.et"},
		{RID: "RFC1-1-2", File: "test/plugin/b.ci"},
		{RID: "RFC1-1-3", File: "test/editor/c.et"},
		{RID: "RFC1-1-4", File: "internal/component/d/d_test.go"},
		{RID: "RFC1-1-4", File: "test/interop/d.ci", Gap: true},
		{RID: "RFC1-1-5", File: "elsewhere/e.txt"},
		{RID: "RFC1-1-6", File: "test/interop/f.ci"},
		{RID: "RFC1-1-7", File: "test/interop/g.ci"},
	}
	proven := map[string]bool{"RFC1-1-1": true, "RFC1-1-2": true, "RFC1-1-3": true, "RFC1-1-4": true,
		"RFC1-1-5": true, "RFC1-1-7": true}
	stems := map[string]bool{"rfc1": true}
	evidence := figureEvidence{tags: tagsByRID(tags), carriers: carriers}

	figures := figuresByStem(requirements, stems, stems, proven, evidence)["rfc1"]

	want := map[string]int{kindInterop: 1, kindFunctional: 1, kindEditor: 1, kindUnit: 1, kindUnknown: 1}
	if len(figures.MandatoryByKind) != len(want) {
		t.Errorf("MandatoryByKind = %v, want %v", figures.MandatoryByKind, want)
	}
	sum := 0
	for kind, count := range figures.MandatoryByKind {
		sum += count
		if want[kind] != count {
			t.Errorf("MandatoryByKind[%s] = %d, want %d", kind, count, want[kind])
		}
	}
	if sum != figures.Mandatory.Proven || figures.Mandatory.Proven != 5 {
		t.Errorf("kinds sum to %d over %d proven mandatory rows, want 5 and 5", sum, figures.Mandatory.Proven)
	}
}

// TestStemFiguresListUnmetIDs checks that the unmet ids are every owed row
// whose annotation's declared traits say Unmet, plus every row whose recorded
// audit verdict says the requirement is not met, sorted, once each, and
// nothing else.
//
// The method holds one row of each unmet annotation, one of each unmet audit
// verdict, and a control for each that must stay off the list: an annotation
// that is not unmet, an audit verdict that is not unmet, a bare row, and a
// rollup, which owes nothing. An unwalked stem still lists its unmet rows: they
// are the summary's own disclosure.
func TestStemFiguresListUnmetIDs(t *testing.T) {
	requirements := []Requirement{
		{RFC: "rfc1", RID: "RFC1-1-9", Level: levelMust, Annotation: &Annotation{Kind: AnnotationGap}},
		{RFC: "rfc1", RID: "RFC1-1-2", Level: levelMust, Annotation: &Annotation{Kind: AnnotationPartial}},
		{RFC: "rfc1", RID: "RFC1-1-3", Level: levelShould, Annotation: &Annotation{Kind: AnnotationLinuxLimit}},
		{RFC: "rfc1", RID: "RFC1-1-4", Level: levelMust},
		{RFC: "rfc1", RID: "RFC1-1-5", Level: levelMust},
		{RFC: "rfc1", RID: "RFC1-1-6", Level: levelMust},
		{RFC: "rfc1", RID: "RFC1-1-7", Level: levelMust, Annotation: &Annotation{Kind: AnnotationNotApplicable}},
		{RFC: "rfc1", RID: "RFC1-1-8", Level: levelMust},
		{RFC: "rfc1", RID: "RFC1-1-1", Level: levelMust},
		{RFC: "rfc1", RID: "RFC1-1-10", Level: levelMust,
			Annotation: &Annotation{Kind: AnnotationRollup, Targets: []string{"RFC1-1-9"}}},
		{RFC: "rfc2", RID: "RFC2-1-1", Level: levelMust, Annotation: &Annotation{Kind: AnnotationGap}},
	}
	audits := map[string]Audit{"rfc1": {Verdicts: map[string]any{
		"RFC1-1-4":  map[string]any{"verdict": VerdictWrong},
		"RFC1-1-5":  map[string]any{"verdict": VerdictUnimplemented},
		"RFC1-1-6":  map[string]any{"verdict": VerdictPartial},
		"RFC1-1-8":  map[string]any{"verdict": VerdictEnforced},
		"RFC1-1-1":  map[string]any{"verdict": VerdictWeak},
		"RFC1-1-10": map[string]any{"verdict": VerdictWrong},
	}}}
	stems := map[string]bool{"rfc1": true, "rfc2": true}
	walked := map[string]bool{"rfc1": true}

	figures := figuresByStem(requirements, stems, walked, nil, figureEvidence{audits: audits, exclusions: acceptEveryExclusion{}})

	want := []string{"RFC1-1-2", "RFC1-1-3", "RFC1-1-4", "RFC1-1-5", "RFC1-1-6", "RFC1-1-9"}
	if !slices.Equal(figures["rfc1"].Unmet, want) {
		t.Errorf("rfc1 Unmet = %v, want %v", figures["rfc1"].Unmet, want)
	}
	if got := figures["rfc2"].Unmet; !slices.Equal(got, []string{"RFC2-1-1"}) {
		t.Errorf("unwalked rfc2 Unmet = %v, want [RFC2-1-1]: a disclosure is listed walked or not", got)
	}
}

// lowerLayerProducer is the function lowerLayerRow names as installing the
// state the layer below acts on.
const lowerLayerProducer = "internal/plugins/ospf/ipsec_install.go::buildIPsecSA"

// lowerLayerRow is one gated rfc1 row a layer under Ze performs.
func lowerLayerRow(rid string) Requirement {
	return Requirement{RFC: "rfc1", RID: rid, Level: levelMust, Annotation: &Annotation{
		Kind: AnnotationLowerLayer, Layer: "Linux XFRM",
		Producer: lowerLayerProducer,
		Reason:   "the kernel performs it",
	}}
}

// TestLowerLayerWithRecordIsOSVerified checks AC-3 on the figures. Goal: a
// {lower-layer} row whose tagged unit carries a record that still verifies and
// proves counts OS verified. Method: one such row through figuresByStem, with
// the proof set provenRequirements derives from the verdicts, so the predicate
// is the one that proves a Ze row (R-4). The row stays out of Ze's proven
// count: a layer under Ze performs it.
func TestLowerLayerWithRecordIsOSVerified(t *testing.T) {
	requirements := []Requirement{lowerLayerRow("RFC1-1-1")}
	verdicts := []DiscriminationVerdict{
		{Record: DiscriminationRecord{RID: "RFC1-1-1", Route: RouteRevert, Producer: lowerLayerProducer}, State: ProofVerified},
	}
	stems := map[string]bool{"rfc1": true}
	proofs := osProofsOf(verdicts)
	figures := figuresByStem(requirements, stems, stems, provenRequirements(verdicts), figureEvidence{osProofs: proofs})["rfc1"]
	if figures.OSVerified != 1 || figures.OSGap != 0 {
		t.Errorf("OSVerified=%d OSGap=%d, want 1 and 0", figures.OSVerified, figures.OSGap)
	}
	if figures.Mandatory.Proven != 0 || figures.Mandatory.Total != 1 {
		t.Errorf("mandatory = %+v, want 0 proven of 1: OS verified is not proven by Ze", figures.Mandatory)
	}
	if !osVerified(&requirements[0], proofs) {
		t.Error("osVerified refused a lower-layer row with a proving record")
	}
}

// TestLowerLayerWithoutRecordIsOSGap checks AC-4. Goal: a {lower-layer} row
// with no verifying record counts OS testing gap, whether or not a test is
// tagged. Method: an untagged row with no verdict, and a row whose record
// proves an UNANNOTATED row so the proof set is not empty.
func TestLowerLayerWithoutRecordIsOSGap(t *testing.T) {
	requirements := []Requirement{
		lowerLayerRow("RFC1-1-1"),
		{RFC: "rfc1", RID: "RFC1-1-2", Level: levelMust},
	}
	verdicts := []DiscriminationVerdict{
		{Record: DiscriminationRecord{RID: "RFC1-1-2", Route: RouteRevert, Producer: lowerLayerProducer}, State: ProofVerified},
	}
	stems := map[string]bool{"rfc1": true}
	proofs := osProofsOf(verdicts)
	figures := figuresByStem(requirements, stems, stems, provenRequirements(verdicts), figureEvidence{osProofs: proofs})["rfc1"]
	if figures.OSVerified != 0 || figures.OSGap != 1 {
		t.Errorf("OSVerified=%d OSGap=%d, want 0 and 1", figures.OSVerified, figures.OSGap)
	}
	if osVerified(&requirements[1], proofs) {
		t.Error("osVerified answered true for a row no lower layer performs")
	}
}

// TestLowerLayerWithANonVerifyingRecordIsNotOSVerified checks R-4. Goal: a
// tagged {lower-layer} row is OS verified only through DiscriminationVerdict.Proven,
// so a stale record, one whose unit is gone, an unfounded escape and the
// verified no-break escape each leave it an OS testing gap. Method: one row per record shape through figuresByStem.
func TestLowerLayerWithANonVerifyingRecordIsNotOSVerified(t *testing.T) {
	for _, one := range []struct {
		name  string
		route string
		state string
	}{
		{"stale producer", RouteRevert, ProofProducerChanged},
		{"unit gone", RouteRevert, ProofUnitGone},
		{"unfounded escape", RouteNoBreak, ProofEscapeUnfounded},
		{"no-break escape", RouteNoBreak, ProofVerified},
	} {
		t.Run(one.name, func(t *testing.T) {
			requirements := []Requirement{lowerLayerRow("RFC1-1-1")}
			verdicts := []DiscriminationVerdict{
				{Record: DiscriminationRecord{RID: "RFC1-1-1", Route: one.route, Producer: lowerLayerProducer}, State: one.state},
			}
			stems := map[string]bool{"rfc1": true}
			figures := figuresByStem(requirements, stems, stems, provenRequirements(verdicts),
				figureEvidence{osProofs: osProofsOf(verdicts)})["rfc1"]
			if figures.OSVerified != 0 || figures.OSGap != 1 {
				t.Errorf("OSVerified=%d OSGap=%d, want 0 and 1", figures.OSVerified, figures.OSGap)
			}
		})
	}
}

// TestLowerLayerRecordBreakingAnotherProducerIsOSGap checks OS verified is
// about the state Ze installs. Goal: a {lower-layer} row counts OS verified
// only when the proving record broke the producer its annotation names, so a
// record that broke some other function, even one in the same file, proves
// nothing about that state and leaves an OS testing gap. A paired break of the
// named producer counts. Method: one row per record through figuresByStem.
func TestLowerLayerRecordBreakingAnotherProducerIsOSGap(t *testing.T) {
	for _, one := range []struct {
		name     string
		record   DiscriminationRecord
		verified int
	}{
		{"another file", DiscriminationRecord{Producer: "internal/plugins/ospf/neighbor.go::sendHello"}, 0},
		{"another function of the same file", DiscriminationRecord{Producer: "internal/plugins/ospf/ipsec_install.go::removeIPsecSA"}, 0},
		{"the named producer, paired", DiscriminationRecord{Producer: "internal/plugins/ospf/neighbor.go::sendHello",
			Paired: []PairedBreak{{Producer: lowerLayerProducer, Break: "x"}}}, 1},
	} {
		t.Run(one.name, func(t *testing.T) {
			requirements := []Requirement{lowerLayerRow("RFC1-1-1")}
			record := one.record
			record.RID, record.Route = "RFC1-1-1", RouteRevert
			verdicts := []DiscriminationVerdict{{Record: record, State: ProofVerified}}
			stems := map[string]bool{"rfc1": true}
			figures := figuresByStem(requirements, stems, stems, provenRequirements(verdicts),
				figureEvidence{osProofs: osProofsOf(verdicts)})["rfc1"]
			if figures.OSVerified != one.verified || figures.OSGap != 1-one.verified {
				t.Errorf("OSVerified=%d OSGap=%d, want %d and %d", figures.OSVerified, figures.OSGap, one.verified, 1-one.verified)
			}
		})
	}
}
