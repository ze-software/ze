// VALIDATES: the section counts the home page and /quality/health/ publish are
// taken over the RFCs that are enrolled, render a public row and declare
// Implementation `ze` or `mixed`; Ze proven counts only what a verified
// discrimination record proves on a walked summary; a not-applicable row leaves
// the MUST rows Ze and the OS owe, counted not owed, only when its evidence
// passes the exclusion evidence check; an unwalked RFC is
// out of every section, counted in NotWalked.
// PREVENTS: a headline percentage raised by a tag that no recorded break
// stands behind (the tag-based count did that until 2026-10-30), by a row the
// summary itself calls unmet, or by leaving an RFC unwalked.

package rfc

import (
	"errors"
	"maps"
	"sync"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// sectionsOver answers ImplementedSectionsOf over figures built from
// requirements, with walked naming the stems carrying a sign-off and proven
// the requirement ids a verified record proves. The not-owed rows are judged
// by an evidence reader over a fixture tree whose rfc/full/rfc1.txt is
// featureFixtureSource (quoteFixtureSource and its optionality sections), so
// a row of rfc1 quoting it can carry valid evidence.
func sectionsOver(t *testing.T, metas map[string]Meta, requirements []Requirement,
	walked, proven map[string]bool,
) (ImplementedSections, error) {
	t.Helper()
	tree := t.TempDir()
	writeFixtureFiles(t, tree, map[string]string{"rfc/full/rfc1.txt": featureFixtureSource})
	return sectionsOverTree(t, tree, metas, requirements, walked, proven)
}

// sectionsOverTree is sectionsOver with the not-owed rows judged against the
// checkout tree, which a corpus test passes as this checkout's root so its
// rows are held to the RFC texts they quote.
func sectionsOverTree(t *testing.T, tree string, metas map[string]Meta, requirements []Requirement,
	walked, proven map[string]bool,
) (ImplementedSections, error) {
	t.Helper()
	stems := map[string]bool{}
	for stem := range metas {
		stems[stem] = true
	}
	return ImplementedSectionsOf(metas, figuresByStem(requirements, stems, walked, proven,
		figureEvidence{exclusions: newEvidenceReader(tree)}))
}

// TestImplementsPopulation checks AC-8: the population is enrolled, with a
// public row, Implementation `ze` or `mixed`, and nothing else decides it.
//
// The method is one case per way in and per way out. The enrolled RFC whose
// feature is still to come (rfc4739's shape) must be IN, which is the owner's
// 2026-10-30 change; a version still reading a status word would leave it out.
func TestImplementsPopulation(t *testing.T) {
	cases := []struct {
		name string
		meta Meta
		want bool
	}{
		{"enrolled ze with a row", Meta{Enrolment: enrolmentEnrolled, Implementation: implementationZe,
			Aggregate: "bgp-core", Row: true}, true},
		{"enrolled ze with a row and a feature still to come (rfc4739)", Meta{Enrolment: enrolmentEnrolled,
			Implementation: implementationZe, Aggregate: "ipsec", Row: true}, true},
		{"enrolled mixed with a row", Meta{Enrolment: enrolmentEnrolled, Implementation: implementationMixed,
			Aggregate: "ipsec", Row: true}, true},
		{"enrolled third-party with a row (rfc9582)", Meta{Enrolment: enrolmentEnrolled,
			Implementation: implementationThirdParty, Aggregate: "rpki", Row: true}, false},
		{"enrolled foundation with a row", Meta{Enrolment: enrolmentEnrolled,
			Implementation: implementationFoundation, Aggregate: "bgp-core", Row: true}, false},
		{"enrolled ze without a row", Meta{Enrolment: enrolmentEnrolled, Implementation: implementationZe}, false},
		{"out of scope ze with a row", Meta{Enrolment: dispositionOutOfScope, Implementation: implementationZe,
			Aggregate: "bgp-core", Row: true}, false},
	}
	for _, tc := range cases {
		meta := tc.meta
		if got := Implements(&meta); got != tc.want {
			t.Errorf("%s: Implements = %v, want %v", tc.name, got, tc.want)
		}
	}

	// The sections count the same population: rfc4739's obligations are in
	// them at their real figure, rfc9582's are in neither population.
	metas := map[string]Meta{
		"rfc4739": cases[1].meta,
		"rfc9582": cases[3].meta,
	}
	requirements := []Requirement{
		{RFC: "rfc4739", RID: "RFC4739-3-1", Level: levelMust},
		{RFC: "rfc4739", RID: "RFC4739-3-2", Level: levelMust},
		{RFC: "rfc9582", RID: "RFC9582-4-1", Level: levelMust},
	}
	walked := map[string]bool{"rfc4739": true, "rfc9582": true}
	got, err := sectionsOver(t, metas, requirements, walked, map[string]bool{"RFC4739-3-1": true, "RFC9582-4-1": true})
	if err != nil {
		t.Fatalf("ImplementedSectionsOf: %v", err)
	}
	if got.RFCs != 1 || got.Sections.Must.Kept != 2 || got.Sections.Must.ZeProven != 1 {
		t.Errorf("sections = %+v, want rfc4739 alone counted at 1 of 2", got)
	}
	if got.Inspected != 1 || got.MustInspected != 2 {
		t.Errorf("Inspected=%d MustInspected=%d, want 1 and 2: a third-party document is not enrolled",
			got.Inspected, got.MustInspected)
	}
}

// TestImplementedSectionsCountVerifiedRecordsOnly checks AC-6 for the
// published counts: a row counts as Ze proven only when a verified record
// proves it on a walked summary, and each other row lands in the section its
// annotation names.
//
// The method holds one walked RFC carrying every row shape beside one unwalked
// RFC. Tags are absent on purpose: the figures take no tags here, so a row both
// polarities cover and no record proves is simply a row the proven set omits.
func TestImplementedSectionsCountVerifiedRecordsOnly(t *testing.T) {
	metas := map[string]Meta{
		"rfc1": {Enrolment: enrolmentEnrolled, Implementation: implementationZe, Aggregate: "bgp-core", Row: true,
			ZeRoles: []string{"speaker"}},
		"rfc2": {Enrolment: enrolmentEnrolled, Implementation: implementationZe, Aggregate: "bgp-core", Row: true},
		"rfc3": {Enrolment: enrolmentEnrolled, Implementation: implementationZe},
	}
	requirements := []Requirement{
		// Proven by a verified record.
		{RFC: "rfc1", RID: "RFC1-1-1", Level: levelMust},
		// Single polarity WITH a record on its one side: proven.
		{RFC: "rfc1", RID: "RFC1-1-2", Level: levelMust,
			Annotation: &Annotation{Kind: AnnotationSinglePolarity, Polarity: PolarityPositive}},
		// Single polarity with NO record: the annotation alone proves nothing.
		{RFC: "rfc1", RID: "RFC1-1-3", Level: levelMust,
			Annotation: &Annotation{Kind: AnnotationSinglePolarity, Polarity: PolarityPositive}},
		// Not applicable with no evidence, with a record: the backlog shelters
		// it from the ratchet, so it stays owed, a Ze gap, never proven.
		{RFC: "rfc1", RID: "RFC1-1-4", Level: levelMust,
			Annotation: &Annotation{Kind: AnnotationNotApplicable}},
		// Gap, with a record: the summary says unmet, so never proven.
		{RFC: "rfc1", RID: "RFC1-1-5", Level: levelMust, Annotation: &Annotation{Kind: AnnotationGap}},
		// Tagged both ways, no record: not proven.
		{RFC: "rfc1", RID: "RFC1-1-6", Level: levelMust},
		evidencedNotApplicable(t),
		// An unwalked RFC with a verified record: out of every section,
		// counted in NotWalked.
		{RFC: "rfc2", RID: "RFC2-1-1", Level: levelMust},
		{RFC: "rfc2", RID: "RFC2-1-2", Level: levelMust},
		// No public row: out of the sections, in the inspected population.
		{RFC: "rfc3", RID: "RFC3-1-1", Level: levelMust},
	}
	proven := map[string]bool{"RFC1-1-1": true, "RFC1-1-2": true, "RFC1-1-4": true, "RFC1-1-5": true, "RFC1-1-7": true,
		"RFC2-1-1": true, "RFC3-1-1": true}

	got, err := sectionsOver(t, metas, requirements, map[string]bool{"rfc1": true, "rfc3": true}, proven)
	if err != nil {
		t.Fatalf("ImplementedSectionsOf: %v", err)
	}
	must := got.Sections.Must
	if must.ZeProven != 2 {
		t.Errorf("Ze proven = %d, want 2: RFC1-1-1 and RFC1-1-2 alone carry a verified record that proves", must.ZeProven)
	}
	if must.ZeGap != 4 {
		t.Errorf("Ze gap = %d, want 4: RFC1-1-3, the unevidenced RFC1-1-4, RFC1-1-5 and RFC1-1-6 are owed and unproven", must.ZeGap)
	}
	if must.NotOwed != 1 || must.NotOwedByKind[AnnotationNotApplicable] != 1 {
		t.Errorf("not owed = %d by kind %v, want RFC1-1-7 alone, the one not-applicable row whose evidence passes", must.NotOwed, must.NotOwedByKind)
	}
	if must.Kept != 6 || must.Total != 7 {
		t.Errorf("Kept=%d Total=%d, want 6 and 7: every MUST row of the walked rfc1 but RFC1-1-7, and none of the "+
			"unwalked rfc2, whose list is unbounded", must.Kept, must.Total)
	}
	if got.RFCs != 2 || got.NotWalked != 1 {
		t.Errorf("RFCs=%d NotWalked=%d, want 2 and 1", got.RFCs, got.NotWalked)
	}
	if got.Inspected != 3 || got.MustInspected != 10 {
		t.Errorf("Inspected=%d MustInspected=%d, want 3 and 10", got.Inspected, got.MustInspected)
	}
	if percent := got.ProvenPercent(); percent != "33.3" {
		t.Errorf("ProvenPercent() = %q, want \"33.3\": 2 Ze proven of the 6 MUST rows Ze owes", percent)
	}
}

// TestImplementedSectionsOfRefusesAnEmptyPopulation checks that a corpus in
// which no enrolled RFC renders a public row is an error rather than a zero.
//
// Sections of {0, 0} would render "0 of 0 (NaN%)" on the home page, and a
// caller cannot tell that from a ledger stating Ze proves nothing.
func TestImplementedSectionsOfRefusesAnEmptyPopulation(t *testing.T) {
	metas := map[string]Meta{
		"rfc1": {Enrolment: enrolmentEnrolled, Implementation: implementationZe},
	}
	requirements := []Requirement{{RFC: "rfc1", RID: "RFC1-1-1", Level: levelMust}}

	if _, err := sectionsOver(t, metas, requirements, map[string]bool{"rfc1": true}, nil); err == nil {
		t.Fatal("ImplementedSectionsOf answered over an empty population, want an error")
	}
}

// TestImplementedSectionsOfRefusesAnEnrolledRFCWithNoFigure checks that a
// Meta the figures do not cover is refused rather than silently left out.
func TestImplementedSectionsOfRefusesAnEnrolledRFCWithNoFigure(t *testing.T) {
	metas := map[string]Meta{
		"rfc1": {Enrolment: enrolmentEnrolled, Implementation: implementationZe, Aggregate: "bgp-core", Row: true},
	}
	if _, err := ImplementedSectionsOf(metas, map[string]StemFigures{}); err == nil {
		t.Fatal("ImplementedSectionsOf answered with an enrolled RFC missing from its figures, want an error")
	}
}

// corpusProof answers, for one checkout, the stems carrying a valid extraction
// sign-off and the requirement ids a verified discrimination record proves:
// the two sets the section counts read beside the requirements.
func corpusProof(t *testing.T, root string, collected Collected) (walked, proven map[string]bool) {
	t.Helper()
	walked, proven, err := corpusProofOf(root, collected)
	if err != nil {
		t.Fatal(err)
	}
	return walked, proven
}

func corpusProofOf(root string, collected Collected) (walked, proven map[string]bool, err error) {
	input, err := NewRenderInput(root, collected, nil, nil)
	if err != nil {
		return nil, nil, errors.New("render the corpus: " + err.Error())
	}
	signed, _, err := evaluateExtractions(input.Deriver, input.Requirements)
	if err != nil {
		return nil, nil, errors.New("evaluate the extraction sign-offs: " + err.Error())
	}
	walked = map[string]bool{}
	for stem := range signed {
		walked[stem] = true
	}
	return walked, provenRequirements(input.Discrimination), nil
}

// realPriorityCorpus is THIS checkout's corpus loaded once per test binary,
// through loadPriorityCorpus, the entry point `./le rfc tree` uses. Loading
// renders the corpus, which re-verifies every discrimination record against the
// tree: minutes under -race. The checkout does not change during a run, so each
// corpus test that rendered it on its own paid that again for the same answer,
// and three such renders ran the package past its timeout. Nothing reads it
// but to answer, so the tests that share it cannot reach one another.
var realPriorityCorpus = sync.OnceValues(func() (*priorityCorpus, error) {
	root, err := lepath.Root()
	if err != nil {
		return nil, err
	}
	return loadPriorityCorpus(root)
})

// realCorpusProofSets answers, for this checkout, the walked stems and the
// proven requirement ids of realPriorityCorpus, as fresh maps the caller owns.
func realCorpusProofSets(t *testing.T) (walked, proven map[string]bool) {
	t.Helper()
	corpus, err := realPriorityCorpus()
	if err != nil {
		t.Fatalf("load the real corpus: %v", err)
	}
	walked = map[string]bool{}
	for stem := range corpus.figures {
		if corpus.figures[stem].Walked {
			walked[stem] = true
		}
	}
	return walked, maps.Clone(corpus.proven)
}

// evidencedNotApplicable is a MUST NOT row of rfc1 whose {not-applicable}
// carries the evidence its kind owes: the row's own obligation sentence from
// section 2 of the fixture text sectionsOver writes, the route reflector it
// binds, and why Ze is not one. Its summary declares Ze a speaker.
func evidencedNotApplicable(t *testing.T) Requirement {
	t.Helper()
	row := evidenceRow(t, "RFC1-1-7", "2",
		`not-applicable: "`+fixtureReflectorSentence+`"; binds a route reflector; ze reflects no widget`)
	row.RFC = "rfc1"
	return row
}

// evidencedFeatureDeclined is a MUST row of rfc1 whose {feature-declined}
// carries the evidence its kind owes: an optionality sentence of section 4,
// named because the row cites section 2, as sectionedRow builds it.
func evidencedFeatureDeclined(t *testing.T, rid string) Requirement {
	t.Helper()
	row := sectionedRow(t, "2", "4", fixtureOptionalSentence)
	row.RFC, row.RID = "rfc1", rid
	return row
}

// acceptEveryExclusion is the exclusion judge of a unit test of the figure
// arithmetic alone: it accepts every not-owed row's evidence, so a bare
// annotation leaves the owed rows as its traits say. The tests of the evidence
// rule itself, and every corpus test, judge with (*evidenceReader).notOwedGap.
type acceptEveryExclusion struct{}

func (acceptEveryExclusion) notOwedGap(*Requirement) (string, bool) { return "", false }

// TestNoPublishedNotOwedRowLacksItsEvidence checks, on this checkout's corpus,
// that every MUST row the published sections count not owed carries the
// evidence its kind owes (owner ruling, 2026-10-29: a row leaves Ze's section
// only with checked evidence).
//
// The method takes the sections the way every surface publishes them, from
// the figures `./le rfc tree` loads (realPriorityCorpus, through
// StemFiguresOf), and counts the not-owed rows independently: a gated row of
// a walked RFC Ze implements whose kind leaves the owed rows on its traits
// (leavesOwedOnTraits), split by whether (*evidenceReader).notOwedGap accepts
// its evidence. The published not-owed count of each kind must equal the
// accepted rows alone, so no row the exclusion evidence backlog shelters, and
// no row whose evidence fails, reaches it.
func TestNoPublishedNotOwedRowLacksItsEvidence(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve checkout: %v", err)
	}
	collected, err := Collect(root)
	if err != nil {
		t.Fatalf("collect the corpus: %v", err)
	}
	corpus, err := realPriorityCorpus()
	if err != nil {
		t.Fatalf("load the real corpus: %v", err)
	}
	sections, err := ImplementedSectionsOf(collected.Metas, corpus.figures)
	if err != nil {
		t.Fatalf("the published sections: %v", err)
	}
	walked, _ := realCorpusProofSets(t)
	reader := newEvidenceReader(root)
	evidenced, lacking := map[string]int{}, map[string]int{}
	for i := range collected.Requirements {
		requirement := &collected.Requirements[i]
		meta := collected.Metas[requirement.RFC]
		if !leavesOwedOnTraits(requirement, &meta, walked) {
			continue
		}
		if _, lacks := reader.notOwedGap(requirement); lacks {
			lacking[requirement.Annotation.Kind]++
			continue
		}
		evidenced[requirement.Annotation.Kind]++
	}
	total := 0
	for _, kind := range AnnotationKinds() {
		total += evidenced[kind]
		if got := sections.Sections.Must.NotOwedByKind[kind]; got != evidenced[kind] {
			t.Errorf("{%s}: the published sections count %d MUST rows not owed, want the %d whose evidence passes "+
				"(%d more lack it and must stay owed)", kind, got, evidenced[kind], lacking[kind])
		}
		t.Logf("{%s}: %d MUST rows not owed with evidence, %d kept owed for want of it", kind, evidenced[kind], lacking[kind])
	}
	if sections.Sections.Must.NotOwed != total {
		t.Errorf("the published sections count %d MUST rows not owed, want %d", sections.Sections.Must.NotOwed, total)
	}
	t.Logf("headline: %d Ze proven of %d MUST rows owed (%s%%), %d not owed, %d MUST rows counted",
		sections.Sections.Must.ZeProven, sections.Sections.Must.Kept, sections.ProvenPercent(),
		sections.Sections.Must.NotOwed, sections.Sections.Must.Total)
}
