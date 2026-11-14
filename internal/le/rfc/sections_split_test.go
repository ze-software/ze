package rfc

// VALIDATES: spec-rfc-figures-built-and-proven AC-1, AC-2, AC-6, AC-7, AC-8,
// AC-9 and "Which row lands where": every owed row that is neither proven nor
// excluded lands, at its level, in exactly one of proof pending, exclusion
// unchecked, not assessed and not built, counted row by row; a row is proof
// pending only when a non-gap tag covers it; the register is complete only
// when every open MUST section is empty; the intent counts stay in the text
// of `./le rfc tree` and `./le rfc next` and leave every published shape.
// PREVENTS: one "correctness gap" over rows that are not built and rows whose
// proof is pending, a not-owed claim with failing evidence drawn as either, an
// intent count reaching published JSON, and a row read back from JSON printing
// a false zero for an intent it never held.

import (
	"encoding/json"
	"maps"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// refuseExclusionsOf is an exclusion judge that refuses the evidence of the
// rows it names and accepts every other: a not-owed claim it names stays owed.
type refuseExclusionsOf map[string]bool

func (refused refuseExclusionsOf) notOwedGap(requirement *Requirement) (string, bool) {
	if refused[requirement.RID] {
		return "fixture: the evidence is refused", true
	}
	return "", false
}

// splitRow is one row of rfc1 at level, annotated with kind when it is set.
func splitRow(rid, level, kind string) Requirement {
	out := Requirement{RFC: "rfc1", RID: rid, Level: level}
	if kind != "" {
		out.Annotation = &Annotation{Kind: kind, Reason: "fixture"}
	}
	return out
}

// splitIntentRow is one optional row of rfc1 carrying an {intent} marker.
func splitIntentRow(rid string, state IntentState) Requirement {
	out := splitRow(rid, levelShould, "")
	out.Intent = &Intent{State: state}
	if state == IntentNotEver {
		out.Intent.Quote, out.Intent.Reason = "A speaker MAY log every widget.", "fixture"
	}
	return out
}

// taggedWith answers one positive `RFC requirement:` tag per id, the tags a
// figureEvidence reads to tell proof pending from not assessed (tested).
func taggedWith(rids ...string) map[string][]Tag {
	out := make(map[string][]Tag, len(rids))
	for _, rid := range rids {
		out[rid] = []Tag{{RID: rid, Polarity: PolarityPositive, File: "internal/fixture/split_test.go", Line: 1}}
	}
	return out
}

// splitFigures answers rfc1's figures and their checked sections, walked.
func splitFigures(t *testing.T, requirements []Requirement, proven map[string]bool,
	evidence figureEvidence,
) (StemFigures, Sections) {
	t.Helper()
	stems := map[string]bool{"rfc1": true}
	figures := figuresByStem(requirements, stems, stems, proven, evidence)["rfc1"]
	sections, err := SectionsOf("rfc1", &figures)
	if err != nil {
		t.Fatalf("SectionsOf refused the figures: %v", err)
	}
	return figures, sections
}

// TestMustGapSplitsIntoNotBuiltAndProofPending checks AC-1: the unproven owed
// MUST rows the summary or the audit declares unmet are not built, the tagged
// rest are proof pending, the untagged rest not assessed, and with Ze proven,
// OS verified and OS testing gap they sum to Kept, with Kept and not owed
// still summing to Total. The method holds one row of each shape; a {gap} row
// with a proving record is still not built.
func TestMustGapSplitsIntoNotBuiltAndProofPending(t *testing.T) {
	requirements := []Requirement{
		splitRow("RFC1-1-1", levelMust, ""),                       // proven
		splitRow("RFC1-1-2", levelMust, ""),                       // tag, no record: proof pending
		splitRow("RFC1-1-3", levelMust, AnnotationSinglePolarity), // tag, no record: proof pending
		splitRow("RFC1-1-4", levelMust, AnnotationGap),            // not built, its record proves nothing
		splitRow("RFC1-1-5", levelMust, AnnotationPartial),        // not built
		splitRow("RFC1-1-6", levelMust, ""),                       // audit unimplemented: not built
		splitRow("RFC1-1-7", levelMust, AnnotationLowerLayer),     // OS testing gap
		splitRow("RFC1-1-8", levelMust, AnnotationNotApplicable),  // not owed
		splitRow("RFC1-1-9", levelMust, ""),                       // no tag: not assessed
	}
	audits := map[string]Audit{"rfc1": {Verdicts: map[string]any{
		"RFC1-1-6": map[string]any{"verdict": VerdictUnimplemented}}}}
	proven := map[string]bool{"RFC1-1-1": true, "RFC1-1-4": true}
	_, sections := splitFigures(t, requirements, proven,
		figureEvidence{tags: taggedWith("RFC1-1-1", "RFC1-1-2", "RFC1-1-3"), audits: audits,
			exclusions: refuseExclusionsOf{}})

	want := MustSections{Total: 9, Kept: 8, ZeProven: 1, OSGap: 1, ProofPending: 2, NotAssessed: 1, NotBuilt: 3,
		NotOwed: 1, NotOwedByKind: map[string]int{AnnotationNotApplicable: 1}}
	if !reflect.DeepEqual(sections.Must, want) {
		t.Fatalf("MUST sections = %+v, want %+v", sections.Must, want)
	}
	must := &sections.Must
	sum := must.ZeProven + must.OSVerified + must.OSGap + must.ProofPending + must.ExclusionUnchecked +
		must.NotAssessed + must.NotBuilt
	if sum != must.Kept {
		t.Errorf("the MUST sections sum to %d, Kept is %d", sum, must.Kept)
	}
	if must.Kept+must.NotOwed != must.Total {
		t.Errorf("Kept %d plus not owed %d is not Total %d", must.Kept, must.NotOwed, must.Total)
	}
}

// TestNotBuiltCountsOnlyUnprovenOwedRows checks AC-2: a row the audit calls
// wrong that a record proves is Ze proven, a {linux-limit} row whose evidence
// passes is not owed, so neither is not built, and both stay in the unmet id
// list. A {linux-limit} row whose evidence fails is not built (row 3).
func TestNotBuiltCountsOnlyUnprovenOwedRows(t *testing.T) {
	requirements := []Requirement{
		splitRow("RFC1-1-1", levelMust, ""),                   // audit wrong, proven
		splitRow("RFC1-1-2", levelMust, AnnotationLinuxLimit), // evidence passes: not owed
		splitRow("RFC1-1-3", levelMust, AnnotationLinuxLimit), // evidence fails: not built
		splitRow("RFC1-1-4", levelMust, ""),                   // untested: not assessed
	}
	audits := map[string]Audit{"rfc1": {Verdicts: map[string]any{
		"RFC1-1-1": map[string]any{"verdict": VerdictWrong}}}}
	figures, sections := splitFigures(t, requirements, map[string]bool{"RFC1-1-1": true},
		figureEvidence{audits: audits, exclusions: refuseExclusionsOf{"RFC1-1-3": true}})

	want := MustSections{Total: 4, Kept: 3, ZeProven: 1, NotAssessed: 1, NotBuilt: 1,
		NotOwed: 1, NotOwedByKind: map[string]int{AnnotationLinuxLimit: 1}}
	if !reflect.DeepEqual(sections.Must, want) {
		t.Fatalf("MUST sections = %+v, want %+v", sections.Must, want)
	}
	if got := []string{"RFC1-1-1", "RFC1-1-2", "RFC1-1-3"}; !slices.Equal(figures.Unmet, got) {
		t.Errorf("Unmet = %v, want %v: the list is the summary's disclosure, not a section", figures.Unmet, got)
	}
}

// TestANotOwedClaimWithoutEvidenceIsExclusionUnchecked checks row 6 of "Which
// row lands where": a {not-applicable} or {feature-declined} row whose
// evidence fails is exclusion unchecked at its level, in neither of the other
// two open sections, and inside Kept; one whose evidence passes is not owed.
func TestANotOwedClaimWithoutEvidenceIsExclusionUnchecked(t *testing.T) {
	requirements := []Requirement{
		splitRow("RFC1-1-1", levelMust, AnnotationNotApplicable),
		splitRow("RFC1-1-2", levelMust, AnnotationFeatureDeclined),
		splitRow("RFC1-1-3", levelMust, AnnotationNotApplicable), // evidence passes
		splitRow("RFC1-2-1", levelShould, AnnotationNotApplicable),
		splitRow("RFC1-2-2", levelShould, AnnotationFeatureDeclined), // evidence passes
	}
	refused := refuseExclusionsOf{"RFC1-1-1": true, "RFC1-1-2": true, "RFC1-2-1": true}
	_, sections := splitFigures(t, requirements, nil, figureEvidence{exclusions: refused})

	wantMust := MustSections{Total: 3, Kept: 2, ExclusionUnchecked: 2,
		NotOwed: 1, NotOwedByKind: map[string]int{AnnotationNotApplicable: 1}}
	if !reflect.DeepEqual(sections.Must, wantMust) {
		t.Errorf("MUST sections = %+v, want %+v", sections.Must, wantMust)
	}
	wantShould := ShouldSections{Total: 2, ExclusionUnchecked: 1,
		NotOwed: 1, NotOwedByKind: map[string]int{AnnotationFeatureDeclined: 1}}
	if !reflect.DeepEqual(sections.Should, wantShould) {
		t.Errorf("SHOULD sections = %+v, want %+v", sections.Should, wantShould)
	}
}

// TestAnOpenRowWithNoTestIsNotAssessed checks the owner's rule "Keep the
// number real." (2026-11-13): an open MUST row and an open optional row, each
// with no tag and no annotation, are not assessed; the same rows with a tag
// are proof pending; a row whose only tag is a gap tag is not assessed, unless
// the row is `{gap}`, which is not built. The method splits the same five
// rows twice, without and with a test tagged to the first two.
func TestAnOpenRowWithNoTestIsNotAssessed(t *testing.T) {
	requirements := []Requirement{
		splitRow("RFC1-1-1", levelMust, ""),            // tagged in the second split only
		splitRow("RFC1-2-1", levelShould, ""),          // tagged in the second split only
		splitRow("RFC1-1-2", levelMust, ""),            // only a gap tag: not assessed
		splitRow("RFC1-1-3", levelMust, AnnotationGap), // a gap tag and {gap}: not built
		splitRow("RFC1-2-2", levelShould, ""),          // only a gap tag: not assessed
	}
	gapTag := func(rid string) []Tag {
		return []Tag{{RID: rid, Gap: true, Demonstration: "internal/fixture/split_test.go::TestGap",
			File: "internal/fixture/split_test.go", Line: 2}}
	}
	gapTags := map[string][]Tag{"RFC1-1-2": gapTag("RFC1-1-2"), "RFC1-1-3": gapTag("RFC1-1-3"),
		"RFC1-2-2": gapTag("RFC1-2-2")}

	_, untagged := splitFigures(t, requirements, nil, figureEvidence{tags: gapTags, exclusions: refuseExclusionsOf{}})
	wantMust := MustSections{Total: 3, Kept: 3, NotAssessed: 2, NotBuilt: 1, NotOwedByKind: map[string]int{}}
	if !reflect.DeepEqual(untagged.Must, wantMust) {
		t.Errorf("with no test tagged, MUST sections = %+v, want %+v", untagged.Must, wantMust)
	}
	wantShould := ShouldSections{Total: 2, NotAssessed: 2, NotOwedByKind: map[string]int{}}
	if !reflect.DeepEqual(untagged.Should, wantShould) {
		t.Errorf("with no test tagged, SHOULD sections = %+v, want %+v", untagged.Should, wantShould)
	}

	tagged := taggedWith("RFC1-1-1", "RFC1-2-1")
	maps.Copy(tagged, gapTags)
	_, withTests := splitFigures(t, requirements, nil, figureEvidence{tags: tagged, exclusions: refuseExclusionsOf{}})
	wantMust = MustSections{Total: 3, Kept: 3, ProofPending: 1, NotAssessed: 1, NotBuilt: 1,
		NotOwedByKind: map[string]int{}}
	if !reflect.DeepEqual(withTests.Must, wantMust) {
		t.Errorf("with a test tagged, MUST sections = %+v, want %+v", withTests.Must, wantMust)
	}
	wantShould = ShouldSections{Total: 2, ProofPending: 1, NotAssessed: 1, NotOwedByKind: map[string]int{}}
	if !reflect.DeepEqual(withTests.Should, wantShould) {
		t.Errorf("with a test tagged, SHOULD sections = %+v, want %+v", withTests.Should, wantShould)
	}
	if !strings.Contains(withTests.String(), ", exclusion unchecked 0, not assessed 1, not built 1") {
		t.Errorf("the sections text does not print not assessed between exclusion unchecked and not built: %s",
			withTests.String())
	}
}

// TestANotEverRowIsNotBuilt checks row 4 of "Which row lands where": an
// optional row the owner marked `{intent: not-ever}` is not built, a not-yet
// or unmarked open row is proof pending when a test is tagged to it and not
// assessed when none is, unless the summary says it is unmet, and the private
// intent counts still split the same rows by marker.
func TestANotEverRowIsNotBuilt(t *testing.T) {
	gapNotYet := splitIntentRow("RFC1-2-4", IntentNotYet)
	gapNotYet.Annotation = &Annotation{Kind: AnnotationGap, Reason: "fixture"}
	requirements := []Requirement{
		splitIntentRow("RFC1-2-1", IntentNotEver),
		splitIntentRow("RFC1-2-2", IntentNotYet),
		splitRow("RFC1-2-3", levelShould, ""),
		gapNotYet,
	}
	figures, sections := splitFigures(t, requirements, nil,
		figureEvidence{tags: taggedWith("RFC1-2-1", "RFC1-2-2", "RFC1-2-4"), exclusions: refuseExclusionsOf{}})

	should := &sections.Should
	if should.NotBuilt != 2 || should.ProofPending != 1 || should.NotAssessed != 1 || should.ExclusionUnchecked != 0 {
		t.Errorf("SHOULD sections = %+v, want not built 2 (not ever, gap), proof pending 1 (tagged not yet), "+
			"not assessed 1 (untagged, unmarked)", sections.Should)
	}
	if got := figures.intents(); got != (intentCounts{notEver: 1, notYet: 2, notDecided: 1}) {
		t.Errorf("intent counts = %+v, want not ever 1, not yet 2, not decided 1", got)
	}
}

// TestOpenOptionalRowsSplitIntoNotBuiltAndProofPending checks AC-7 and AC-9:
// the open optional rows the summary or the audit declares unmet are not
// built, the tagged rest proof pending, the untagged rest not assessed, and
// with proven, OS verified, OS testing gap and not owed they sum to the
// optional Total. The published shapes carry the new keys and none of the
// removed ones.
func TestOpenOptionalRowsSplitIntoNotBuiltAndProofPending(t *testing.T) {
	requirements := []Requirement{
		splitRow("RFC1-2-1", levelShould, ""),                       // proven
		splitRow("RFC1-2-2", levelShould, AnnotationGap),            // not built
		splitRow("RFC1-2-3", "MAY", AnnotationPartial),              // not built
		splitRow("RFC1-2-4", levelShould, ""),                       // audit wrong: not built
		splitRow("RFC1-2-5", "MAY", ""),                             // untested: not assessed
		splitRow("RFC1-2-6", levelShould, AnnotationSinglePolarity), // tag, no record: proof pending
		splitRow("RFC1-2-7", levelShould, AnnotationLowerLayer),     // OS testing gap
		splitRow("RFC1-2-8", levelShould, AnnotationNotApplicable),  // not owed
	}
	audits := map[string]Audit{"rfc1": {Verdicts: map[string]any{
		"RFC1-2-4": map[string]any{"verdict": VerdictWrong}}}}
	_, sections := splitFigures(t, requirements, map[string]bool{"RFC1-2-1": true},
		figureEvidence{tags: taggedWith("RFC1-2-6"), audits: audits, exclusions: refuseExclusionsOf{}})

	want := ShouldSections{Total: 8, Proven: 1, OSGap: 1, ProofPending: 1, NotAssessed: 1, NotBuilt: 3,
		NotOwed: 1, NotOwedByKind: map[string]int{AnnotationNotApplicable: 1}}
	if !reflect.DeepEqual(sections.Should, want) {
		t.Fatalf("SHOULD sections = %+v, want %+v", sections.Should, want)
	}
	should := &sections.Should
	sum := should.Proven + should.OSVerified + should.OSGap + should.NotOwed +
		should.ProofPending + should.ExclusionUnchecked + should.NotAssessed + should.NotBuilt
	if sum != should.Total {
		t.Errorf("the SHOULD sections sum to %d, Total is %d", sum, should.Total)
	}

	for label, value := range map[string]any{"must": sections.Must, "should": sections.Should} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		keys := map[string]any{}
		if err := json.Unmarshal(encoded, &keys); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"proof-pending", "exclusion-unchecked", "not-assessed", "not-built"} {
			if _, held := keys[key]; !held {
				t.Errorf("%s sections publish no %q: %s", label, key, encoded)
			}
		}
		for _, key := range []string{"ze-gap", "not-ever", "not-yet", "not-decided"} {
			if _, held := keys[key]; held {
				t.Errorf("%s sections still publish %q: %s", label, key, encoded)
			}
		}
	}
}

// TestRegisterCompleteNeedsNoOpenMustSection checks AC-6: the mandatory
// register is complete only when no member is unwalked and proof pending,
// exclusion unchecked, not assessed, not built and OS testing gap are all 0. The method is
// one walked RFC per bar shape, each open shape incomplete beside a proven one.
func TestRegisterCompleteNeedsNoOpenMustSection(t *testing.T) {
	open := Figure{Proven: 1, Total: 2, Attainable: 2}
	figures := map[string]StemFigures{
		"proven": {Stem: "proven", Walked: true, Mandatory: Figure{Proven: 2, Total: 2, Attainable: 2}},
		"stack": {Stem: "stack", Walked: true, OSVerified: 1,
			Mandatory: Figure{Proven: 1, Total: 3, Attainable: 1,
				Excluded: map[string]int{AnnotationLowerLayer: 1, AnnotationNotApplicable: 1}}},
		"pending":   {Stem: "pending", Walked: true, Mandatory: open, ProofPending: 1},
		"unchecked": {Stem: "unchecked", Walked: true, Mandatory: open, ExclusionUnchecked: 1},
		"untested":  {Stem: "untested", Walked: true, Mandatory: open, NotAssessed: 1},
		"not-built": {Stem: "not-built", Walked: true, Mandatory: open, NotBuilt: 1},
		"os-gap": {Stem: "os-gap", Walked: true, OSGap: 1,
			Mandatory: Figure{Proven: 1, Total: 2, Attainable: 1, Excluded: map[string]int{AnnotationLowerLayer: 1}}},
		"unwalked": {Stem: "unwalked"},
	}
	for _, one := range []struct {
		members  []string
		complete bool
	}{
		{[]string{"proven"}, true},
		{[]string{"proven", "stack"}, true},
		{[]string{"proven", "pending"}, false},
		{[]string{"proven", "unchecked"}, false},
		{[]string{"proven", "untested"}, false},
		{[]string{"proven", "not-built"}, false},
		{[]string{"proven", "os-gap"}, false},
		{[]string{"proven", "unwalked"}, false},
		{nil, false},
	} {
		rollup := rollupOf(figures, one.members)
		if got := rollup.mandatoryComplete(); got != one.complete {
			t.Errorf("%v: mandatoryComplete = %v, want %v (sections %+v)", one.members, got, one.complete,
				rollup.Sections.Must)
		}
	}
}

// intentTreeCorpus is the priority fixture's graph over rfc2 rows carrying
// every intent state, judged with every exclusion accepted.
func intentTreeCorpus(t *testing.T) *priorityCorpus {
	t.Helper()
	base := priorityFixture(t, nil)
	row := func(rid string, state IntentState) Requirement {
		out := splitIntentRow(rid, state)
		out.RFC = "rfc2"
		return out
	}
	requirements := []Requirement{
		{RFC: "rfc2", RID: "RFC2-1-1", Level: levelMust},
		row("RFC2-2-1", IntentNotEver),
		row("RFC2-2-2", IntentNotYet),
		{RFC: "rfc2", RID: "RFC2-2-3", Level: levelShould},
	}
	stems := map[string]bool{"rfc1": true, "rfc2": true}
	evidence := figureEvidence{exclusions: acceptEveryExclusion{}}
	figures := figuresByStem(requirements, stems, stems, nil, evidence)
	return newPriorityCorpus(base.graph, base.profiles, base.metas, requirements, figures, nil, evidence)
}

// intentClause is the clause intentTreeCorpus's rfc2 row prints.
const intentClause = "; intent: not ever 1, not yet 1, not decided 1"

// TestTreeJSONCarriesNoIntentCounts checks AC-8: a tree row's JSON carries the
// optional sections and no intent count, and a row read back from it holds no
// intent and prints no intent clause, while its sections print as the live
// row's do.
func TestTreeJSONCarriesNoIntentCounts(t *testing.T) {
	row := intentTreeCorpus(t).treeRow("rfc2", nil)
	encoded, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	for _, word := range []string{`"not-ever"`, `"not-yet"`, `"not-decided"`, "intent"} {
		if strings.Contains(string(encoded), word) {
			t.Errorf("the published row carries %s: %s", word, encoded)
		}
	}
	if !strings.Contains(string(encoded), `"not-built":1`) {
		t.Errorf("the published row lacks the optional not-built section: %s", encoded)
	}
	var decoded TreeRow
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.intent != nil {
		t.Errorf("a row read back from JSON holds intent counts %+v", *decoded.intent)
	}
	live, read := sectionsCell(row), sectionsCell(&decoded)
	if !strings.HasSuffix(live, intentClause) {
		t.Fatalf("the live cell %q does not end with %q", live, intentClause)
	}
	if read != strings.TrimSuffix(live, intentClause) {
		t.Errorf("the read-back cell = %q, want the live sections %q with no intent clause", read,
			strings.TrimSuffix(live, intentClause))
	}
}

// TestTreeTextKeepsTheIntentCounts checks AC-8: `./le rfc tree` prints a row's
// intent counts after its sections, in a clause neither section pattern of the
// le help wiring fixture reads, so the fixture still sees one SHOULD clause.
func TestTreeTextKeepsTheIntentCounts(t *testing.T) {
	row := intentTreeCorpus(t).treeRow("rfc2", nil)
	text := (&TreeReport{Rows: []*TreeRow{row}}).Text()
	at := strings.Index(text, intentClause)
	if at < 0 {
		t.Fatalf("tree text lacks %q: %s", intentClause, text)
	}
	if should := strings.Index(text, "; SHOULD "); should < 0 || should > at {
		t.Errorf("the intent clause is not after the SHOULD sections: %s", text)
	}
	if got := len(regexp.MustCompile(`SHOULD \d+: `).FindAllString(text, -1)); got != 1 {
		t.Errorf("tree text holds %d SHOULD clauses, want 1: %s", got, text)
	}
	if strings.Contains(text, "correctness gap") {
		t.Errorf("tree text still names the retired section: %s", text)
	}
}

// TestRollupLinesKeepTheIntentCounts checks AC-8 for rollups: the rollup line
// of `./le rfc tree` and of `./le rfc next` prints the members' summed intent
// counts, the rollup's JSON carries none, and a rollup read back from it
// prints its sections with no intent clause.
func TestRollupLinesKeepTheIntentCounts(t *testing.T) {
	figures := map[string]StemFigures{
		"rfc1": {Stem: "rfc1", Walked: true, Optional: Figure{Total: 2, Attainable: 2},
			OptionalProofPending: 2, NotYet: 1, NotDecided: 1},
		"rfc2": {Stem: "rfc2", Walked: true, Optional: Figure{Total: 2, Attainable: 2},
			OptionalProofPending: 1, OptionalNotBuilt: 1, NotEver: 1, NotYet: 1},
		"rfc3": {Stem: "rfc3", NotYet: 9},
	}
	clause := "; intent: not ever 1, not yet 2, not decided 1"
	scope := ScopeRollup{Rollup: rollupOf(figures, []string{"rfc1", "rfc2", "rfc3"})}
	live := scope.String()
	if !strings.Contains(live, clause) {
		t.Fatalf("rollup %q lacks %q: an unwalked member adds no intent", live, clause)
	}
	tree := (&TreeReport{Profile: "edge", Kind: "use-case", Rollup: &scope}).Text()
	if first, _, _ := strings.Cut(tree, "\n"); !strings.Contains(first, clause) {
		t.Errorf("the tree's rollup line lacks %q: %s", clause, first)
	}
	next := (&NextReport{Profiles: []string{"edge"}, Rollup: scope}).Text()
	if first, _, _ := strings.Cut(next, "\n"); !strings.Contains(first, clause) {
		t.Errorf("the next report's rollup line lacks %q: %s", clause, first)
	}
	encoded, err := json.Marshal(scope)
	if err != nil {
		t.Fatal(err)
	}
	for _, word := range []string{`"not-ever"`, `"not-yet"`, `"not-decided"`, "intent"} {
		if strings.Contains(string(encoded), word) {
			t.Errorf("the rollup's JSON carries %s: %s", word, encoded)
		}
	}
	var decoded ScopeRollup
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if got, want := decoded.String(), strings.Replace(live, clause, "", 1); got != want {
		t.Errorf("read-back rollup = %q, want %q", got, want)
	}
}

// TestNextOwesWhatTheMustSectionsCount checks R-6 at the MUST level: the MUST
// rows `./le rfc next` says owe Ze code (rowOwes) are exactly the rows the
// sections count proof pending, exclusion unchecked, not assessed or not
// built, and the rows
// it says owe an OS test are the OS testing gap. The method holds one MUST row
// of every shape on two walked RFCs and compares the two counts.
func TestNextOwesWhatTheMustSectionsCount(t *testing.T) {
	base := priorityFixture(t, nil)
	row := func(stem, rid, kind string) Requirement {
		out := splitRow(rid, levelMust, kind)
		out.RFC = stem
		return out
	}
	requirements := []Requirement{
		row("rfc1", "RFC1-1-1", ""), // proven
		row("rfc1", "RFC1-1-2", ""), // no tag: not assessed
		row("rfc1", "RFC1-1-3", AnnotationGap),
		row("rfc1", "RFC1-1-4", AnnotationNotApplicable), // refused: exclusion unchecked
		row("rfc1", "RFC1-1-5", AnnotationNotApplicable), // not owed
		row("rfc2", "RFC2-1-1", AnnotationLinuxLimit),    // refused: not built
		row("rfc2", "RFC2-1-2", AnnotationLowerLayer),    // OS testing gap
		row("rfc2", "RFC2-1-3", AnnotationRollup),        // counts nowhere
		row("rfc2", "RFC2-1-4", AnnotationSinglePolarity),
	}
	proven := map[string]bool{"RFC1-1-1": true}
	evidence := figureEvidence{exclusions: refuseExclusionsOf{"RFC1-1-4": true, "RFC2-1-1": true}}
	stems := map[string]bool{"rfc1": true, "rfc2": true}
	figures := figuresByStem(requirements, stems, stems, proven, evidence)
	corpus := newPriorityCorpus(base.graph, base.profiles, base.metas, requirements, figures, proven, evidence)

	zeCode, osTest := 0, 0
	for index := range requirements {
		owes, owed := corpus.rowOwes(&requirements[index])
		if !owed {
			continue
		}
		switch owes {
		case owesZeCode:
			zeCode++
		case owesOSTest:
			osTest++
		default:
			t.Fatalf("%s owes %q, which is neither kind", requirements[index].RID, owes)
		}
	}
	open, osGap := 0, 0
	for stem := range figures {
		one := figures[stem]
		sections, err := SectionsOf(stem, &one)
		if err != nil {
			t.Fatal(err)
		}
		open += sections.Must.ProofPending + sections.Must.ExclusionUnchecked + sections.Must.NotAssessed +
			sections.Must.NotBuilt
		osGap += sections.Must.OSGap
	}
	if zeCode != open || zeCode != 5 {
		t.Errorf("next owes Ze code on %d MUST rows, the open MUST sections count %d, want 5 and 5", zeCode, open)
	}
	if osTest != osGap || osTest != 1 {
		t.Errorf("next owes an OS test on %d MUST rows, the OS testing gap counts %d, want 1 and 1", osTest, osGap)
	}
}
