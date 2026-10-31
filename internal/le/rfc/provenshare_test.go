// VALIDATES: the published proof share is taken over the RFCs that are
// enrolled, render a public row and declare Implementation `ze` or `mixed`, and
// counts as proven only what a verified discrimination record proves on a
// walked summary, keeping a not-applicable requirement in its denominator and
// an unwalked RFC out of both sides, counted in NotWalked.
// PREVENTS: a headline percentage raised by annotating requirements away
// (excluding {not-applicable} did that until 2026-09-02), by a tag that no
// recorded break stands behind (the tag-based count did that until
// 2026-10-30), or by leaving an RFC unwalked.

package rfc

import (
	"errors"
	"maps"
	"sync"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// shareOver answers ProvenShareOf over figures built from requirements, with
// walked naming the stems carrying a sign-off and proven the requirement ids a
// verified record proves.
func shareOver(t *testing.T, metas map[string]Meta, requirements []Requirement,
	walked, proven map[string]bool,
) (ProvenShare, error) {
	t.Helper()
	stems := map[string]bool{}
	for stem := range metas {
		stems[stem] = true
	}
	return ProvenShareOf(metas, figuresByStem(requirements, stems, walked, proven, figureEvidence{}))
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

	// The share counts the same population: rfc4739's obligations are in the
	// denominator at their real figure, rfc9582's are in neither population.
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
	share, err := shareOver(t, metas, requirements, walked, map[string]bool{"RFC4739-3-1": true, "RFC9582-4-1": true})
	if err != nil {
		t.Fatalf("ProvenShareOf: %v", err)
	}
	if share.RFCs != 1 || share.Gated != 2 || share.Proven != 1 {
		t.Errorf("share = %+v, want rfc4739 alone counted at 1 of 2", share)
	}
	if share.Inspected != 1 || share.GatedInspected != 2 {
		t.Errorf("Inspected=%d GatedInspected=%d, want 1 and 2: a third-party document is not enrolled",
			share.Inspected, share.GatedInspected)
	}
}

// TestProvenShareCountsVerifiedRecordsOnly checks AC-6 for the share: a row
// counts as proven only when a verified record proves it on a walked summary.
//
// The method holds one walked RFC carrying every row shape beside one unwalked
// RFC. Tags are absent on purpose: the share takes no tags, so a row both
// polarities cover and no record proves is simply a row the proven set omits.
func TestProvenShareCountsVerifiedRecordsOnly(t *testing.T) {
	metas := map[string]Meta{
		"rfc1": {Enrolment: enrolmentEnrolled, Implementation: implementationZe, Aggregate: "bgp-core", Row: true},
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
		// Not applicable, with a record: stays in the denominator, never proven.
		{RFC: "rfc1", RID: "RFC1-1-4", Level: levelMust,
			Annotation: &Annotation{Kind: AnnotationNotApplicable}},
		// Gap, with a record: the summary says unmet, so never proven.
		{RFC: "rfc1", RID: "RFC1-1-5", Level: levelMust, Annotation: &Annotation{Kind: AnnotationGap}},
		// Tagged both ways, no record: not proven.
		{RFC: "rfc1", RID: "RFC1-1-6", Level: levelMust},
		// An unwalked RFC with a verified record: out of both sides, counted
		// in NotWalked.
		{RFC: "rfc2", RID: "RFC2-1-1", Level: levelMust},
		{RFC: "rfc2", RID: "RFC2-1-2", Level: levelMust},
		// No public row: out of the share, in the inspected population.
		{RFC: "rfc3", RID: "RFC3-1-1", Level: levelMust},
	}
	proven := map[string]bool{"RFC1-1-1": true, "RFC1-1-2": true, "RFC1-1-4": true, "RFC1-1-5": true,
		"RFC2-1-1": true, "RFC3-1-1": true}

	share, err := shareOver(t, metas, requirements, map[string]bool{"rfc1": true, "rfc3": true}, proven)
	if err != nil {
		t.Fatalf("ProvenShareOf: %v", err)
	}
	if share.Proven != 2 {
		t.Errorf("Proven = %d, want 2: RFC1-1-1 and RFC1-1-2 alone carry a verified record that proves", share.Proven)
	}
	if share.Gated != 6 {
		t.Errorf("Gated = %d, want 6: every gated row of the walked rfc1, and none of the "+
			"unwalked rfc2, whose list is unbounded", share.Gated)
	}
	if share.RFCs != 2 || share.NotWalked != 1 {
		t.Errorf("RFCs=%d NotWalked=%d, want 2 and 1", share.RFCs, share.NotWalked)
	}
	if share.Inspected != 3 || share.GatedInspected != 9 {
		t.Errorf("Inspected=%d GatedInspected=%d, want 3 and 9", share.Inspected, share.GatedInspected)
	}
	if got := share.Percent(); got != "33.3" {
		t.Errorf("Percent() = %q, want \"33.3\"", got)
	}
}

// TestProvenShareOfRefusesAnEmptyPopulation checks that a corpus in which no
// enrolled RFC renders a public row is an error rather than a zero.
//
// A ProvenShare of {0, 0} would render "0 of 0 (NaN%)" on the home page, and a
// caller cannot tell that from a ledger stating Ze proves nothing.
func TestProvenShareOfRefusesAnEmptyPopulation(t *testing.T) {
	metas := map[string]Meta{
		"rfc1": {Enrolment: enrolmentEnrolled, Implementation: implementationZe},
	}
	requirements := []Requirement{{RFC: "rfc1", RID: "RFC1-1-1", Level: levelMust}}

	if _, err := shareOver(t, metas, requirements, map[string]bool{"rfc1": true}, nil); err == nil {
		t.Fatal("ProvenShareOf answered a share over an empty population, want an error")
	}
}

// TestProvenShareOfRefusesAnEnrolledRFCWithNoFigure checks that a Meta the
// figures do not cover is refused rather than silently left out of the share.
func TestProvenShareOfRefusesAnEnrolledRFCWithNoFigure(t *testing.T) {
	metas := map[string]Meta{
		"rfc1": {Enrolment: enrolmentEnrolled, Implementation: implementationZe, Aggregate: "bgp-core", Row: true},
	}
	if _, err := ProvenShareOf(metas, map[string]StemFigures{}); err == nil {
		t.Fatal("ProvenShareOf answered with an enrolled RFC missing from its figures, want an error")
	}
}

// corpusProof answers, for one checkout, the stems carrying a valid extraction
// sign-off and the requirement ids a verified discrimination record proves:
// the two sets the share's figures read beside the requirements.
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
	for stem, figures := range corpus.figures {
		if figures.Walked {
			walked[stem] = true
		}
	}
	return walked, maps.Clone(corpus.proven)
}
