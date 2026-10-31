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

	"github.com/ze-software/ze/internal/le/rfc/graph"
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

// nextCorpusOf answers a corpus whose one release profile, core, requires
// rfc1 alone, holding rows, with the proof set and the OS proofs verdicts
// give, and exclusions judging every not-owed row: the same derivations
// StemFiguresOf and priorityCorpusOf make, with one figureEvidence read by the
// figures and by rowOwes alike.
func nextCorpusOf(rows []Requirement, verdicts []DiscriminationVerdict, exclusions exclusionJudge) *priorityCorpus {
	stems := map[string]bool{"rfc1": true}
	metas := map[string]Meta{"rfc1": {Title: "Title of rfc1", Aggregate: "base", UseCases: []string{"core"}}}
	profiles := Profiles{
		UseCases:   map[string]UseCase{"core": {Name: "core", Title: "Core", Release: true}},
		Aggregates: map[string]Aggregate{"base": {Name: "base"}},
	}
	proven := provenRequirements(verdicts)
	evidence := figureEvidence{osProofs: osProofsOf(verdicts), exclusions: exclusions}
	figures := figuresByStem(rows, stems, stems, proven, evidence)
	return newPriorityCorpus(graph.Build(map[string]graph.Text{"rfc1": {}}), profiles, metas, rows, figures,
		proven, evidence)
}

// nextVerdict answers a verified, proving record for rid that broke producer.
func nextVerdict(rid, producer string) DiscriminationVerdict {
	return DiscriminationVerdict{Record: DiscriminationRecord{RID: rid, Route: RouteRevert, Producer: producer},
		State: ProofVerified}
}

// nextOwedRows answers the rows of rfc1's report: every uncovered row, each
// as id=owes.
func nextOwedRows(t *testing.T, report *NextReport) []string {
	t.Helper()
	out := []string{}
	for index := range report.Gaps {
		for _, row := range report.Gaps[index].Uncovered {
			out = append(out, row.ID+"="+row.Owes)
		}
	}
	return out
}

// TestNextNamesAnOSTestingGapAsOwed: AC-9 for `next`. Goal: a set whose only
// MUST gap is an OS testing gap is not complete, and next names that row as
// owing a test of the OS behavior, never Ze code, in text and JSON alike; a
// row an annotation leaves not owed blocks nothing. Method: rfc1 with its Ze
// row proven, one {feature-declined} row, and one {lower-layer} row with no
// proving record.
func TestNextNamesAnOSTestingGapAsOwed(t *testing.T) {
	declined := Requirement{RFC: "rfc1", RID: "RFC1-1-3", Level: levelMust,
		Annotation: &Annotation{Kind: AnnotationFeatureDeclined, Reason: "the feature is not offered"}}
	rows := []Requirement{{RFC: "rfc1", RID: "RFC1-1-1", Level: levelMust}, lowerLayerRow("RFC1-1-2"), declined}
	report, err := nextCorpusOf(rows, []DiscriminationVerdict{nextVerdict("RFC1-1-1", "x.go::f")}, acceptEveryExclusion{}).next("core")
	if err != nil {
		t.Fatalf("next core: %v", err)
	}
	if report.Complete || report.MandatoryComplete || report.Register != registerMandatory {
		t.Fatalf("next core complete %v mandatory complete %v register %s, want an incomplete mandatory register",
			report.Complete, report.MandatoryComplete, report.Register)
	}
	if got, want := nextOwedRows(t, report), []string{"RFC1-1-2=" + owesOSTest}; !slices.Equal(got, want) {
		t.Errorf("owed rows %v, want %v", got, want)
	}
	text := report.Text()
	if strings.Contains(text, "nothing is owed") {
		t.Errorf("text reports nothing owed over an OS testing gap:\n%s", text)
	}
	if !strings.Contains(text, "1 OS testing gap, a test of the OS behavior on the state Ze installs is owed: RFC1-1-2") {
		t.Errorf("text does not name the OS testing gap:\n%s", text)
	}
	if strings.Contains(text, " uncovered: ") {
		t.Errorf("text names an OS testing gap as Ze code owed:\n%s", text)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"id":"RFC1-1-2","level":"MUST","owes":"os-test"`) {
		t.Errorf("json does not name the OS testing gap: %s", encoded)
	}
	if !strings.Contains(string(encoded), `"mandatory-complete":false,"complete":false`) {
		t.Errorf("json reports complete: %s", encoded)
	}
}

// TestNextCompleteOnlyWhenEveryOwedRowIsProven: Goal: Complete holds once the
// Ze row is Ze proven and the {lower-layer} row OS verified, and a not-owed
// row does not hold it back. Method: the rows of the OS-gap case, with a
// proving record that broke the producer the {lower-layer} row names.
func TestNextCompleteOnlyWhenEveryOwedRowIsProven(t *testing.T) {
	declined := Requirement{RFC: "rfc1", RID: "RFC1-1-3", Level: levelMust,
		Annotation: &Annotation{Kind: AnnotationFeatureDeclined, Reason: "the feature is not offered"}}
	rows := []Requirement{{RFC: "rfc1", RID: "RFC1-1-1", Level: levelMust}, lowerLayerRow("RFC1-1-2"), declined}
	verdicts := []DiscriminationVerdict{nextVerdict("RFC1-1-1", "x.go::f"), nextVerdict("RFC1-1-2", lowerLayerProducer)}
	report, err := nextCorpusOf(rows, verdicts, acceptEveryExclusion{}).next("core")
	if err != nil {
		t.Fatalf("next core: %v", err)
	}
	if !report.Complete || !report.MandatoryComplete {
		t.Fatalf("next core complete %v mandatory complete %v, want complete; gaps %+v",
			report.Complete, report.MandatoryComplete, report.Gaps)
	}
	if !strings.Contains(report.Text(), "nothing is owed") {
		t.Errorf("text does not say nothing is owed:\n%s", report.Text())
	}
}

// TestNextNeverNamesANotEverRow: Goal: a SHOULD row the owner marked
// `{intent: not-ever}` is not owed work, so `./le rfc next` never names it,
// while a `{intent: not-yet}` row beside it stays owed Ze code. Method: rfc1
// with its MUST row proven and two unproven SHOULD rows, one of each intent;
// the optional register must name the not-yet row alone.
func TestNextNeverNamesANotEverRow(t *testing.T) {
	notEver := Requirement{RFC: "rfc1", RID: "RFC1-1-2", Level: levelShould,
		Intent: &Intent{State: IntentNotEver, Quote: "x", Reason: "y"}}
	notYet := Requirement{RFC: "rfc1", RID: "RFC1-1-3", Level: levelShould, Intent: &Intent{State: IntentNotYet}}
	rows := []Requirement{{RFC: "rfc1", RID: "RFC1-1-1", Level: levelMust}, notEver, notYet}
	report, err := nextCorpusOf(rows, []DiscriminationVerdict{nextVerdict("RFC1-1-1", "x.go::f")}, acceptEveryExclusion{}).next("core")
	if err != nil {
		t.Fatalf("next core: %v", err)
	}
	if report.Complete || report.Register != registerOptional {
		t.Fatalf("next core complete %v register %s, want an incomplete optional register",
			report.Complete, report.Register)
	}
	if got, want := nextOwedRows(t, report), []string{"RFC1-1-3=" + owesZeCode}; !slices.Equal(got, want) {
		t.Errorf("owed rows %v, want %v", got, want)
	}
}

// TestNextNamesAZeGapAsZeCode: Goal: an unproven unannotated MUST row is
// named as Ze code owed, under "uncovered". Method: rfc1 with the
// {lower-layer} row OS verified and its Ze row unproven.
func TestNextNamesAZeGapAsZeCode(t *testing.T) {
	rows := []Requirement{{RFC: "rfc1", RID: "RFC1-1-1", Level: levelMust}, lowerLayerRow("RFC1-1-2")}
	report, err := nextCorpusOf(rows, []DiscriminationVerdict{nextVerdict("RFC1-1-2", lowerLayerProducer)}, acceptEveryExclusion{}).next("core")
	if err != nil {
		t.Fatalf("next core: %v", err)
	}
	if report.Complete || report.Register != registerMandatory {
		t.Fatalf("next core complete %v register %s, want an incomplete mandatory register",
			report.Complete, report.Register)
	}
	if got, want := nextOwedRows(t, report), []string{"RFC1-1-1=" + owesZeCode}; !slices.Equal(got, want) {
		t.Errorf("owed rows %v, want %v", got, want)
	}
	if text := report.Text(); !strings.Contains(text, "1 uncovered: RFC1-1-1\n") {
		t.Errorf("text does not name the Ze gap:\n%s", text)
	}
}

// TestNextNamesABareNotApplicableRowAsZeCode: Goal: `./le rfc next` and the
// figures judge a not-owed row by one rule, so a {not-applicable} row with no
// role-condition quote stays owed Ze code in both, and the same row leaves
// both once its evidence passes. Method: rfc1 with a proven MUST row and a
// bare {not-applicable} MUST row, judged first by the tree's own evidence
// reader ((*evidenceReader).notOwedGap refuses a row with no quote before it
// reads a file), then by a judge that accepts every row.
func TestNextNamesABareNotApplicableRowAsZeCode(t *testing.T) {
	bare := Requirement{RFC: "rfc1", RID: "RFC1-1-2", Level: levelMust,
		Annotation: &Annotation{Kind: AnnotationNotApplicable, Reason: "Ze is not that role"}}
	rows := []Requirement{{RFC: "rfc1", RID: "RFC1-1-1", Level: levelMust}, bare}
	verdicts := []DiscriminationVerdict{nextVerdict("RFC1-1-1", "x.go::f")}

	corpus := nextCorpusOf(rows, verdicts, newEvidenceReader(t.TempDir()))
	report, err := corpus.next("core")
	if err != nil {
		t.Fatalf("next core: %v", err)
	}
	if report.Complete || report.MandatoryComplete {
		t.Fatalf("next core complete %v mandatory complete %v, want the bare row owed",
			report.Complete, report.MandatoryComplete)
	}
	if got, want := nextOwedRows(t, report), []string{"RFC1-1-2=" + owesZeCode}; !slices.Equal(got, want) {
		t.Errorf("owed rows %v, want %v", got, want)
	}
	if figure := corpus.figures["rfc1"].Mandatory; figure.Attainable != 2 || figure.Excluded[AnnotationNotApplicable] != 0 {
		t.Errorf("figure %+v, want the bare row among the two rows Ze owes, as next lists it", figure)
	}

	accepted := nextCorpusOf(rows, verdicts, acceptEveryExclusion{})
	report, err = accepted.next("core")
	if err != nil {
		t.Fatalf("next core, every exclusion accepted: %v", err)
	}
	if !report.Complete {
		t.Errorf("next core lists %v once the row's evidence passes, want nothing owed", nextOwedRows(t, report))
	}
	if figure := accepted.figures["rfc1"].Mandatory; figure.Attainable != 1 || figure.Excluded[AnnotationNotApplicable] != 1 {
		t.Errorf("figure %+v, want the row out of the rows Ze owes, as next leaves it", figure)
	}
}
