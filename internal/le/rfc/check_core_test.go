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

	corrections := []correction{{RIDs: []string{"RFC9999-2-1"}, Quotes: []string{sentence}}}
	if !correctionAuthorizes("RFC9999-2-1", corrections, quoteFixtureSource) {
		t.Error("a correction quoting a sentence across a page break did not authorize")
	}
}
