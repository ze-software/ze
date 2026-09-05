package rfc

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fixture RFC this file classifies. It carries one OPTIONAL feature, so a
// feature-out-of-scope reason has a real sentence to quote, and its two
// obligations give one mapping and one exclusion.
const classifySource = `Network Working Group                                          A. Tester
Request for Comments: 9998                                  October 2026


1.  Introduction

   The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT",
   "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this
   document are to be interpreted as described in RFC 2119.

2.  Requirements

   A speaker MUST do the first thing.  Support for widget mirroring is
   optional.  A speaker that mirrors widgets MUST number every mirror.

3.  References

   Nothing normative lives here.
`

func classifyTree(t *testing.T) string {
	t.Helper()
	return fixtureTree(t, map[string]string{
		"feature-gates.txt":    "",
		"go.mod":               "module fixture\n",
		"rfc/full/rfc9998.txt": classifySource,
		"rfc/short/rfc9998.md": "# RFC 9998\n\n## Compliance Checklist\n\n" +
			"- [ ] [RFC9998-2-1] [MUST] A speaker MUST do the first thing (§2)\n",
	})
}

// writeDecisions stores one decisions file and answers its path.
func writeDecisions(t *testing.T, tree string, decisions classifyDecisions) string {
	t.Helper()
	body, err := json.MarshalIndent(decisions, "", "  ")
	if err != nil {
		t.Fatalf("encode decisions: %v", err)
	}
	path := filepath.Join(tree, "decisions.json")
	if err := os.WriteFile(path, append(body, '\n'), 0o600); err != nil {
		t.Fatalf("write decisions: %v", err)
	}
	return path
}

// walkedSections decides every derived section of the fixture, so a test about
// sites is not also a test about an unclassified section.
func walkedSections() []classifySectionDecision {
	return []classifySectionDecision{
		{ID: "front", Disposition: dispositionSkipped, SkipKind: "front-matter",
			Reason: "Title and header block."},
		{ID: "1", Disposition: dispositionWalked},
		{ID: "2", Disposition: dispositionWalked},
		{ID: "3", Disposition: dispositionSkipped, SkipKind: "references",
			Reason: "Reference list."},
	}
}

// TestExtractionClassifyAppliesAWholeWalkAndPlacesIt is the positive polarity:
// a decisions file that classifies every site and section reaches the corpus,
// carrying each authored field into the artifact the production parser reads.
func TestExtractionClassifyAppliesAWholeWalkAndPlacesIt(t *testing.T) {
	tree := classifyTree(t)
	path := writeDecisions(t, tree, classifyDecisions{
		Stem: "rfc9998", SignedOff: "2026-09-05", Reviewer: "tester",
		Sections: walkedSections(),
		Sites: []classifySiteDecision{
			{ID: "2:1", Disposition: DispositionMapped, MappedTo: "RFC9998-2-1"},
			{ID: "2:2", Disposition: DispositionExcluded, ExcludedKind: featureOutOfScope,
				Reason: "Conditional on widget mirroring, which Ze does not offer. " +
					"RFC 9998 Section 2 states the option in its own words: " +
					"'Support for widget mirroring is optional.'"},
		},
	})

	report, err := classifyExtraction(tree, path)
	if err != nil {
		t.Fatalf("classifyExtraction: %v", err)
	}
	if !report.Placed || report.Path != "rfc/extraction/rfc9998.json" {
		t.Fatalf("a fully classified walk did not reach the corpus: %+v", report)
	}
	if report.Mapped != 1 || report.Excluded != 1 || report.BindsAnotherRole != 0 {
		t.Errorf("the census is %+v", report)
	}
	if len(report.Unclassified) != 0 || len(report.UnclassifiedSections) != 0 {
		t.Errorf("a whole walk reported a remainder: %+v", report)
	}

	artifact, err := ParseExtractionArtifact(tree, filepath.Join(tree, "rfc", "extraction", "rfc9998.json"))
	if err != nil {
		t.Fatalf("the production parser refused the applied walk: %v", err)
	}
	if artifact.SignedOff != "2026-09-05" || artifact.Reviewer != "tester" {
		t.Errorf("the sign-off fields are %q by %q", artifact.SignedOff, artifact.Reviewer)
	}
	if artifact.Sites[0].MappedTo != "RFC9998-2-1" || artifact.Sites[1].ExcludedKind != featureOutOfScope {
		t.Errorf("the decisions did not land: %+v", artifact.Sites)
	}
	if sites, sections := artifact.Unclassified(); sites+sections != 0 {
		t.Errorf("%d site(s) and %d section(s) stayed unclassified", sites, sections)
	}
}

// TestExtractionClassifyReportsAnUnclassifiedRemainderRatherThanDefaultingIt
// is the property the two scripts this verb replaces had: a site with no
// honest disposition is left null on purpose, named in the report, and kept out
// of the corpus. A default disposition would annotate an obligation away.
func TestExtractionClassifyReportsAnUnclassifiedRemainderRatherThanDefaultingIt(t *testing.T) {
	tree := classifyTree(t)
	path := writeDecisions(t, tree, classifyDecisions{
		Stem: "rfc9998", Sections: walkedSections(),
		Sites: []classifySiteDecision{
			{ID: "2:1", Disposition: DispositionMapped, MappedTo: "RFC9998-2-1"},
			{ID: "2:2", Residual: "MET, but no checklist row declares it yet"},
		},
	})

	report, err := classifyExtraction(tree, path)
	if err != nil {
		t.Fatalf("classifyExtraction: %v", err)
	}
	if report.Placed {
		t.Fatalf("an unclassified walk reached the corpus: %+v", report)
	}
	if !strings.Contains(report.Path, filepath.ToSlash(filepath.Join("tmp", "session"))) {
		t.Errorf("the unclassified walk went to %q, want this session's scratch", report.Path)
	}
	if len(report.Unclassified) != 1 || report.Unclassified[0].ID != "2:2" ||
		report.Unclassified[0].Note != "MET, but no checklist row declares it yet" {
		t.Fatalf("the remainder is %+v", report.Unclassified)
	}
	if !strings.Contains(report.Text(), "UNCLASSIFIED 2:2: MET, but no checklist row declares it yet") {
		t.Errorf("the report does not name the remainder:\n%s", report.Text())
	}

	corpus := filepath.Join(tree, "rfc", "extraction", "rfc9998.json")
	if _, err := os.Stat(corpus); !os.IsNotExist(err) {
		t.Errorf("the corpus holds a file it must not: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(report.Path)))
	if err != nil {
		t.Fatalf("read the scratch walk: %v", err)
	}
	if !strings.Contains(string(body), `"id": "2:2",`) ||
		!strings.Contains(string(body), `"disposition": null`) {
		t.Errorf("the unclassified site was not written as null:\n%s", body)
	}
	if strings.Contains(string(body), "no checklist row declares it") {
		t.Errorf("the residual note was written into the artifact:\n%s", body)
	}
}

// TestExtractionClassifyRoundTripLeavesAClassifiedArtifactByteIdentical proves
// the writer transcribes. Re-applying the decisions a landed sign-off already
// holds must change nothing, or a re-run of a walk would show as a diff nobody
// authored.
func TestExtractionClassifyRoundTripLeavesAClassifiedArtifactByteIdentical(t *testing.T) {
	tree := classifyTree(t)
	decisions := classifyDecisions{
		Stem: "rfc9998", SignedOff: "2026-09-05", Reviewer: "tester",
		Sections: walkedSections(),
		Sites: []classifySiteDecision{
			{ID: "2:1", Disposition: DispositionMapped, MappedTo: "RFC9998-2-1"},
			{ID: "2:2", Disposition: DispositionExcluded, ExcludedKind: bindsAnotherRole,
				Producer: "rfc/full/rfc9998.txt",
				Reason: "Binds the widget mirror numbering authority, a role Ze never acts as. " +
					"The producer that would act as it if Ze did is rfc/full/rfc9998.txt."},
		},
	}
	path := writeDecisions(t, tree, decisions)
	if _, err := classifyExtraction(tree, path); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	corpus := filepath.Join(tree, "rfc", "extraction", "rfc9998.json")
	first, err := os.ReadFile(corpus)
	if err != nil {
		t.Fatalf("read the landed walk: %v", err)
	}

	report, err := classifyExtraction(tree, path)
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if !report.Placed || report.BindsAnotherRole != 1 {
		t.Fatalf("the re-apply is %+v", report)
	}
	second, err := os.ReadFile(corpus)
	if err != nil {
		t.Fatalf("re-read the landed walk: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Errorf("re-applying the same decisions rewrote the artifact:\nfirst:\n%s\nsecond:\n%s",
			first, second)
	}
}

// TestExtractionClassifyRefusals is the negative polarity, one case per refusal
// the writer owns. Each one asserts that nothing was written, because a
// half-applied walk on disk is worse than a refused one.
func TestExtractionClassifyRefusals(t *testing.T) {
	role := "Binds the widget mirror numbering authority, a role Ze never acts as. " +
		"The producer that would act as it if Ze did is rfc/full/rfc9998.txt."

	for _, test := range []struct {
		name      string
		decisions classifyDecisions
		want      string
	}{{
		name: "a disposition outside the closed set",
		decisions: classifyDecisions{Stem: "rfc9998", Sites: []classifySiteDecision{
			{ID: "2:1", Disposition: "declined", MappedTo: "RFC9998-2-1"},
		}},
		want: "is not one of ['excluded', 'mapped']",
	}, {
		name: "an exclusion kind outside the closed vocabulary",
		decisions: classifyDecisions{Stem: "rfc9998", Sites: []classifySiteDecision{
			{ID: "2:1", Disposition: DispositionExcluded, ExcludedKind: "owner-approved",
				Reason: "the owner said so"},
		}},
		want: "excluded needs an 'excluded-kind' from",
	}, {
		name: "binds-another-role with no producer named",
		decisions: classifyDecisions{Stem: "rfc9998", Sites: []classifySiteDecision{
			{ID: "2:2", Disposition: DispositionExcluded, ExcludedKind: bindsAnotherRole,
				Reason: "Binds the widget mirror numbering authority, which Ze is not."},
		}},
		want: "needs a 'producer'",
	}, {
		name: "binds-another-role whose producer the reason never carries",
		decisions: classifyDecisions{Stem: "rfc9998", Sites: []classifySiteDecision{
			{ID: "2:2", Disposition: DispositionExcluded, ExcludedKind: bindsAnotherRole,
				Producer: "rfc/full/rfc9998.txt",
				Reason:   "Binds the widget mirror numbering authority, which Ze is not."},
		}},
		want: "the 'reason' does not carry the producer",
	}, {
		name: "binds-another-role whose producer is not in the tree",
		decisions: classifyDecisions{Stem: "rfc9998", Sites: []classifySiteDecision{
			{ID: "2:2", Disposition: DispositionExcluded, ExcludedKind: bindsAnotherRole,
				Producer: "internal/component/widget/mirror.go",
				Reason: "Binds the widget mirror numbering authority. The producer that would " +
					"act as it if Ze did is internal/component/widget/mirror.go."},
		}},
		want: "is not in this tree",
	}, {
		name: "feature-out-of-scope with no quoted sentence",
		decisions: classifyDecisions{Stem: "rfc9998", Sites: []classifySiteDecision{
			{ID: "2:2", Disposition: DispositionExcluded, ExcludedKind: featureOutOfScope,
				Reason: "Conditional on widget mirroring, which the RFC makes optional."},
		}},
		want: "needs the reason to QUOTE the sentence",
	}, {
		name: "feature-out-of-scope quoting a sentence no source holds",
		decisions: classifyDecisions{Stem: "rfc9998", Sites: []classifySiteDecision{
			{ID: "2:2", Disposition: DispositionExcluded, ExcludedKind: featureOutOfScope,
				Reason: "Conditional on widget mirroring. RFC 9998 Section 2 says " +
					"'Support for widget mirroring is entirely at the operator's option.'"},
		}},
		want: "no quoted sentence in the reason appears in",
	}, {
		name: "an exclusion with no reason",
		decisions: classifyDecisions{Stem: "rfc9998", Sites: []classifySiteDecision{
			{ID: "2:1", Disposition: DispositionExcluded, ExcludedKind: "not-a-requirement"},
		}},
		want: "excluded needs a non-empty 'reason'",
	}, {
		name: "a mapping that names no requirement id",
		decisions: classifyDecisions{Stem: "rfc9998", Sites: []classifySiteDecision{
			{ID: "2:1", Disposition: DispositionMapped},
		}},
		want: "mapped needs a 'mapped-to'",
	}, {
		name: "a decision for a site the source does not derive",
		decisions: classifyDecisions{Stem: "rfc9998", Sites: []classifySiteDecision{
			{ID: "9:1", Disposition: DispositionMapped, MappedTo: "RFC9998-9-1"},
		}},
		want: "is not a site this source derives",
	}, {
		name: "one site decided twice",
		decisions: classifyDecisions{Stem: "rfc9998", Sites: []classifySiteDecision{
			{ID: "2:1", Disposition: DispositionMapped, MappedTo: "RFC9998-2-1"},
			{ID: "2:1", Disposition: DispositionExcluded, ExcludedKind: bindsAnotherRole,
				Producer: "rfc/full/rfc9998.txt", Reason: role},
		}},
		want: "is decided twice",
	}, {
		name: "a residual carrying decision fields",
		decisions: classifyDecisions{Stem: "rfc9998", Sites: []classifySiteDecision{
			{ID: "2:1", Residual: "cannot classify", MappedTo: "RFC9998-2-1"},
		}},
		want: "carries decision fields with no 'disposition'",
	}, {
		name: "a disposition and a residual at once",
		decisions: classifyDecisions{Stem: "rfc9998", Sites: []classifySiteDecision{
			{ID: "2:1", Disposition: DispositionMapped, MappedTo: "RFC9998-2-1",
				Residual: "cannot classify"},
		}},
		want: "carries both a disposition and a 'residual'",
	}, {
		name: "a relocation naming no spec",
		decisions: classifyDecisions{Stem: "rfc9998", Sites: []classifySiteDecision{
			{ID: "2:2", Disposition: DispositionExcluded, ExcludedKind: relocatedToSpec,
				Reason: "owed by the mirroring spec", RelocatedTo: "plan/known-failures/mirror.md",
				ReservedID: "RFC9998-2-2"},
		}},
		want: "needs a 'relocated-to' naming the spec",
	}, {
		name: "a section skipped with no kind",
		decisions: classifyDecisions{Stem: "rfc9998", Sections: []classifySectionDecision{
			{ID: "3", Disposition: dispositionSkipped, Reason: "Reference list."},
		}},
		want: "skipped needs a 'skip-kind' from",
	}, {
		name: "a section skipped with no reason",
		decisions: classifyDecisions{Stem: "rfc9998", Sections: []classifySectionDecision{
			{ID: "3", Disposition: dispositionSkipped, SkipKind: "references"},
		}},
		want: "skipped needs a non-empty 'reason'",
	}, {
		name: "a section disposition outside the closed set",
		decisions: classifyDecisions{Stem: "rfc9998", Sections: []classifySectionDecision{
			{ID: "3", Disposition: "ignored"},
		}},
		want: "is not one of ['skipped', 'walked']",
	}, {
		name:      "a decisions file that decides nothing",
		decisions: classifyDecisions{Stem: "rfc9998", Reviewer: "tester", SignedOff: "2026-09-05"},
		want:      "names no site and no section decision",
	}, {
		name: "a decisions file whose sites belong to another RFC",
		decisions: classifyDecisions{Stem: "rfc9997", Sites: []classifySiteDecision{
			{ID: "2:1", Disposition: DispositionMapped, MappedTo: "RFC9997-2-1"},
		}},
		want: "has no source text at rfc/full/rfc9997.txt",
	}} {
		t.Run(test.name, func(t *testing.T) {
			tree := classifyTree(t)
			path := writeDecisions(t, tree, test.decisions)
			report, err := classifyExtraction(tree, path)
			if err == nil {
				t.Fatalf("the decisions were applied: %+v", report)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("the refusal is %q, want it to carry %q", err, test.want)
			}
			if _, statErr := os.Stat(filepath.Join(tree, "rfc", "extraction", "rfc9998.json")); !os.IsNotExist(statErr) {
				t.Errorf("a refused walk left an artifact behind: %v", statErr)
			}
		})
	}
}

// TestExtractionClassifyRefusesAResidualOverALandedDecision keeps a walk from
// withdrawing a decision by silence: a site the corpus already classified needs
// a re-classification, and a note saying "could not classify" contradicts the
// artifact rather than replacing it.
func TestExtractionClassifyRefusesAResidualOverALandedDecision(t *testing.T) {
	tree := classifyTree(t)
	whole := classifyDecisions{
		Stem: "rfc9998", SignedOff: "2026-09-05", Reviewer: "tester",
		Sections: walkedSections(),
		Sites: []classifySiteDecision{
			{ID: "2:1", Disposition: DispositionMapped, MappedTo: "RFC9998-2-1"},
			{ID: "2:2", Disposition: DispositionExcluded, ExcludedKind: "not-a-requirement",
				Reason: "The sentence states the option itself and directs no implementation."},
		},
	}
	if _, err := classifyExtraction(tree, writeDecisions(t, tree, whole)); err != nil {
		t.Fatalf("land the first walk: %v", err)
	}

	withdraw := classifyDecisions{Stem: "rfc9998", Sites: []classifySiteDecision{
		{ID: "2:2", Residual: "on second reading nothing fits"},
	}}
	before, err := os.ReadFile(filepath.Join(tree, "rfc", "extraction", "rfc9998.json"))
	if err != nil {
		t.Fatalf("read the landed walk: %v", err)
	}
	_, err = classifyExtraction(tree, writeDecisions(t, tree, withdraw))
	if err == nil {
		t.Fatal("a residual withdrew a landed decision")
	}
	if !strings.Contains(err.Error(), "cannot withdraw") {
		t.Errorf("the refusal is %q", err)
	}
	after, err := os.ReadFile(filepath.Join(tree, "rfc", "extraction", "rfc9998.json"))
	if err != nil {
		t.Fatalf("re-read the landed walk: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("the refusal changed the artifact:\n%s", after)
	}
}

// TestExtractionClassifyReplacesEveryFieldOfAReclassifiedSite proves a decision
// rebuilds its site rather than editing it: an exclusion's kind and reason must
// not survive the mapping that replaces it, where they would be read by the
// site publisher and by nothing else.
func TestExtractionClassifyReplacesEveryFieldOfAReclassifiedSite(t *testing.T) {
	tree := classifyTree(t)
	excluded := classifyDecisions{
		Stem: "rfc9998", SignedOff: "2026-09-05", Reviewer: "tester",
		Sections: walkedSections(),
		Sites: []classifySiteDecision{
			{ID: "2:1", Disposition: DispositionExcluded, ExcludedKind: "not-a-requirement",
				Reason: "Read on the first pass as a description of another system."},
			{ID: "2:2", Disposition: DispositionExcluded, ExcludedKind: "advisory-in-context",
				Reason: "The keyword sits inside the optional construction above it."},
		},
	}
	if _, err := classifyExtraction(tree, writeDecisions(t, tree, excluded)); err != nil {
		t.Fatalf("land the first walk: %v", err)
	}

	corrected := excluded
	corrected.Sites = []classifySiteDecision{
		{ID: "2:1", Disposition: DispositionMapped, MappedTo: "RFC9998-2-1"},
	}
	if _, err := classifyExtraction(tree, writeDecisions(t, tree, corrected)); err != nil {
		t.Fatalf("apply the correction: %v", err)
	}

	artifact, err := ParseExtractionArtifact(tree, filepath.Join(tree, "rfc", "extraction", "rfc9998.json"))
	if err != nil {
		t.Fatalf("parse the corrected walk: %v", err)
	}
	site := artifact.Sites[0]
	if site.Disposition != DispositionMapped || site.MappedTo != "RFC9998-2-1" {
		t.Fatalf("the correction did not land: %+v", site)
	}
	if site.ExcludedKind != "" || site.Reason != "" {
		t.Errorf("the replaced exclusion left fields behind: %+v", site)
	}
	if artifact.Sites[1].ExcludedKind != "advisory-in-context" {
		t.Errorf("a site the correction did not name lost its decision: %+v", artifact.Sites[1])
	}
}
