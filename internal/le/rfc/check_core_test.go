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
	if errs := featureDeclinedQuote("fixture", req, map[string]*quoteSource{}, tree); len(errs) != 0 {
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

// evidenceRow is one requirement of the fixture RFC (quoteFixtureSource),
// citing section, annotated by a checklist reason the real parser reads.
func evidenceRow(t *testing.T, rid, section, annotation string) Requirement {
	t.Helper()
	parsed, err := parseAnnotation(annotation, "fixture")
	if err != nil {
		t.Fatalf("the fixture annotation {%s} did not parse: %v", annotation, err)
	}
	// The row states the quoted sentence, as a summary row states its RFC's
	// obligation: a quoted obligation must be the row's own (rowOwnSentence).
	return Requirement{RFC: selftestStem, RID: rid, Level: levelMust, Text: parsed.Quote, Section: section,
		Source: selftestSummaryRel, Line: 7, Annotation: parsed}
}

// VALIDATES: AC-1 -- a {not-applicable} row carries a role-condition quote from
// the section it cites and the specific role that sentence binds, or the
// evidence ratchet refuses it by id, naming the missing quote.
// METHOD: four rows of one fixture RFC through the parser and
// (*evidenceReader).gap: a bare reason, a quote from another section, a quote
// from the cited section, and a quote from a subsection of it. The parser
// refuses a quoted payload naming no bound role, a bound role the quote does
// not carry, a bound role made only of words for any party (singular or
// plural), or no reason. The roles ze fills are the Meta table's
// (TestNotApplicableRolesAreWholeWordsAndDistinct).
func TestNotApplicableNeedsSectionRoleQuote(t *testing.T) {
	tree := t.TempDir()
	writeFixtureFiles(t, tree, map[string]string{"rfc/full/rfc9999.txt": quoteFixtureSource,
		evidenceBacklogRel: "lower-layer 0\nnot-applicable 1\n"})
	const inTwo = "A speaker MUST send the widget before it sends any frame that depends on it."
	const inThree = "A receiver MUST NOT reorder the frames of one widget."
	const inTwoOne = fixtureRelaySentence
	cases := []struct {
		name, annotation, missing string
	}{
		{"a bare reason", "not-applicable: ze sends no widget", "carries no role-condition quote from section 2"},
		{"a quote from another section", `not-applicable: "` + fixtureScopingSentence + `"; binds relay agents; it relays nothing`,
			"its role-condition quote is in section 3, and the row cites section 2"},
		{"a quote from the cited section", `not-applicable: "` + fixtureReflectorSentence + `"; binds a route reflector; it reflects no widget`, ""},
		{"a quote from a subsection", `not-applicable: "` + inTwoOne + `"; binds a relay agent; it relays no widget`, ""},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			req := evidenceRow(t, "RFC9999-2-1", "2", one.annotation)
			missing, lacks := newEvidenceReader(tree).gap(&req)
			if one.missing == "" {
				if lacks {
					t.Fatalf("a role-condition quote from the cited section was refused: %s", missing)
				}
				return
			}
			if !lacks {
				t.Fatal("a row without a role-condition quote from its section was accepted")
			}
			if !strings.Contains(missing, one.missing) {
				t.Errorf("the refusal %q does not say %q", missing, one.missing)
			}
			errs, _ := checkExclusionEvidence(tree, []Requirement{req}, evidenceBaseline{state: priorRead, prints: map[string]string{}})
			if len(errs) == 0 || !strings.Contains(errs[0], "RFC9999-2-1") || !strings.Contains(errs[0], notApplicableFormat) {
				t.Errorf("a new row lacking its evidence was not refused by id with the format: %q", errs)
			}
		})
	}
	req := evidenceRow(t, "RFC9999-2-1", "2", `not-applicable: "`+fixtureReflectorSentence+`"; binds a route reflector; it reflects no widget`)
	if req.Annotation.Quote != fixtureReflectorSentence || req.Annotation.BoundRole != "a route reflector" {
		t.Errorf("the evidence form parsed to quote %q, bound role %q", req.Annotation.Quote, req.Annotation.BoundRole)
	}
	refusals := []struct{ payload, says string }{
		{`"` + inTwo + `"; it sends no widget`, "names no role the quoted sentence binds"},
		{`"` + fixtureScopingSentence + `"; binds a gateway; it sends no widget`, "is not in the quoted sentence"},
		{`"` + inTwo + `"; binds the widget; it sends no widget`, "is not the subject of the quoted obligation"},
		{`"` + inTwo + `"; binds a gateway; it sends no widget`, "is not the subject of the quoted obligation"},
		{`"` + inThree + `"; binds frames; it reorders nothing`, "is not the subject of the quoted obligation"},
		{`"It MUST NOT reflect the widget to its own clients."; binds a route reflector; it reflects nothing`, "subject is the pronoun 'it'"},
		{`"They all MUST reflect the widget to every client."; binds a route reflector; it reflects nothing`, "subject is the pronoun 'they'"},
		{`"Route reflectors each MUST reflect the widget to every client."; binds a route reflector; it reflects no widget`, ""},
		{`"` + inTwo + `"; binds a speaker; it sends no widget`, "a word for any party to a protocol"},
		{`"` + inTwo + `"; binds The Speakers; it sends no widget`, "a word for any party to a protocol"},
		{`"` + fixtureReflectorSentence + `"; binds a route reflector`, "gives no reason"},
		{`"short"; binds a widget; why`, "under 24 characters"},
	}
	for _, one := range refusals {
		_, err := parseAnnotation("not-applicable: "+one.payload, "fixture")
		if one.says == "" {
			// A quantifier between the subject and its keyword is allowed.
			if err != nil {
				t.Errorf("{not-applicable: %s} was refused: %v", one.payload, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("{not-applicable: %s} parsed; a quoted payload owes a quote, a specific bound role the quote names and a reason", one.payload)
			continue
		}
		if !strings.Contains(err.Error(), one.says) || !strings.Contains(err.Error(), notApplicableFormat) {
			t.Errorf("{not-applicable: %s} was refused with %q, want it to say %q and end with the format", one.payload, err, one.says)
		}
	}
}

// VALIDATES: AC-2 -- a {feature-declined} quote is matched within the section
// the row cites: one the RFC carries in another section is refused, naming the
// section it is in and the section searched.
// METHOD: the same quote under a row citing its own section and a row citing
// another. checkFeatureDeclined keeps refusing a quote the RFC does not carry
// at all, and leaves the section to the ratchet.
func TestFeatureDeclinedQuoteIsSectionScoped(t *testing.T) {
	tree := t.TempDir()
	writeFixtureFiles(t, tree, map[string]string{"rfc/full/rfc9999.txt": quoteFixtureSource})
	const quote = "A receiver MUST NOT reorder the frames of one widget."
	const annotation = `feature-declined: "` + quote + `"; internal/x/frame.go::sendFrame sends one frame`
	own := evidenceRow(t, "RFC9999-3-1", "3", annotation)
	if missing, lacks := newEvidenceReader(tree).gap(&own); lacks {
		t.Fatalf("a quote from the row's own section was refused: %s", missing)
	}
	other := evidenceRow(t, "RFC9999-2-1", "2", annotation)
	missing, lacks := newEvidenceReader(tree).gap(&other)
	if !lacks {
		t.Fatal("a quote from another section was accepted for a row citing section 2")
	}
	for _, want := range []string{"optionality quote is in section 3", "cites section 2"} {
		if !strings.Contains(missing, want) {
			t.Errorf("the refusal %q does not say %q", missing, want)
		}
	}
	reader := newTextReader(map[string]string{"internal/x/frame.go": "package x\n\nfunc sendFrame() {}\n"})
	if errs := checkFeatureDeclined(tree, reader, []Requirement{other}); len(errs) != 0 {
		t.Errorf("checkFeatureDeclined refused a quote the RFC carries, which is the ratchet's to judge: %v", errs)
	}
	absent := evidenceRow(t, "RFC9999-2-2", "2", `feature-declined: "no section of this RFC says this sentence"; internal/x/frame.go::sendFrame sends one frame`)
	if errs := checkFeatureDeclined(tree, reader, []Requirement{absent}); len(errs) != 1 || !strings.Contains(errs[0], "is not in rfc/full/rfc9999.txt") {
		t.Errorf("a quote the RFC does not carry was not refused: %v", errs)
	}
}
