package rfc

import (
	"strings"
	"testing"
)

// VALIDATES: AC-9 -- featureDeclinedQuote and correctionAuthorizes match against the same
// page-stripped haystack as the row check, so a sentence crossing a page break is found by
// all three quote paths.
// METHOD: the sentence is shown absent from the raw collapse first, so each assertion can
// only pass through quoteHaystack.
func TestFeatureDeclinedQuoteAcrossPageBreak(t *testing.T) {
	const sentence = "A speaker MUST send the widget before it sends any frame that depends on it."
	if strings.Contains(squashWhitespace(quoteFixtureSource), sentence) {
		t.Fatal("the fixture sentence is in the raw collapse, so this test cannot prove the page strip")
	}
	tree := t.TempDir()
	writeFixtureFiles(t, tree, map[string]string{"rfc/full/rfc9999.txt": quoteFixtureSource})

	req := Requirement{RFC: selftestStem, RID: "RFC9999-2-1", Source: selftestSummaryRel, Line: 1,
		Annotation: &Annotation{Kind: AnnotationFeatureDeclined, Quote: sentence}}
	if errs := featureDeclinedQuote("fixture", req, map[string]string{}, tree); len(errs) != 0 {
		t.Errorf("a feature-declined quote across a page break was refused: %v", errs)
	}

	corrections := []correction{{Kind: correctionLevel, RIDs: []string{"RFC9999-2-1"}, Quotes: []string{sentence}}}
	if !correctionAuthorizes("RFC9999-2-1", corrections, quoteFixtureSource) {
		t.Error("a correction quoting a sentence across a page break did not authorize")
	}
}

// TestUpdatedByMarkerRequiresUpdaterListed: AC-6. A member carrying a
// `{updated-by}` marker requires its updater listed for the same profile in a
// tier no lower than its own: an updater listed in neither row is refused, a
// required member's updater listed only as optional is refused, and both
// matching shapes pass. Method: checkUpdatedByMembership over one marker on
// rfc1 naming rfc4, with rfc4's rows varied.
func TestUpdatedByMarkerRequiresUpdaterListed(t *testing.T) {
	requirements := []Requirement{{RFC: "rfc1", RID: "RFC1-2-1", Level: "MUST", Line: 7,
		UpdatedBy: &UpdatedBy{Target: "RFC4-1-1", Reason: "rewritten"}}}
	cases := map[string]struct {
		member, updater Meta
		want            string
	}{
		"updater listed nowhere": {Meta{UseCases: []string{"edge"}}, Meta{}, "| Use cases | edge |"},
		"updater only optional": {Meta{UseCases: []string{"edge"}}, Meta{OptionalUseCases: []string{"edge"}},
			"| Use cases | edge |"},
		"optional member, updater nowhere": {Meta{OptionalUseCases: []string{"edge"}}, Meta{},
			"| Optional use cases | edge |"},
		"both required":                     {Meta{UseCases: []string{"edge"}}, Meta{UseCases: []string{"edge"}}, ""},
		"optional member, updater required": {Meta{OptionalUseCases: []string{"edge"}}, Meta{UseCases: []string{"edge"}}, ""},
		"both optional":                     {Meta{OptionalUseCases: []string{"edge"}}, Meta{OptionalUseCases: []string{"edge"}}, ""},
	}
	for label, one := range cases {
		t.Run(label, func(t *testing.T) {
			refusals := checkUpdatedByMembership(requirements, map[string]Meta{"rfc1": one.member, "rfc4": one.updater})
			if one.want == "" {
				if len(refusals) != 0 {
					t.Errorf("refused %q", refusals)
				}
				return
			}
			if len(refusals) != 1 {
				t.Fatalf("want one refusal, got %q", refusals)
			}
			for _, want := range []string{"rfc/short/rfc1.md", "RFC1-2-1", "RFC4-1-1", "rfc/short/rfc4.md", one.want} {
				if !strings.Contains(refusals[0], want) {
					t.Errorf("refusal %q does not name %q", refusals[0], want)
				}
			}
		})
	}
}
