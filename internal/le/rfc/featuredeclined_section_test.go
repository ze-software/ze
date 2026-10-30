// VALIDATES: a {feature-declined} payload may name the section its optionality
// quote comes from (`§<section> "<quote>"; <reason>`). The named section is a
// section id at parse time and an exact heading of rfc/full/<stem>.txt, never
// an ancestor reached by fallback. A named section is refused as redundant
// whenever the quote lies in the row's own section, so each case has one
// spelling. With or without a section named, the quote is a whole sentence of
// the section it is held to, it carries an optionality marker, its BCP 14
// keywords are not only mandatory ones, and it is not the row's own sentence.
// A mandatory sentence guarded by a condition is the conditional shape, judged
// in featuredeclined_conditional_test.go.
// PREVENTS: a row whose RFC states the feature optional in one section and the
// conditional obligation in another (RFC 9728 §2.2 and §3.3) having no valid
// evidence form, a named section turning into a way round the section scope
// (`§2.99` read as §2), and any 24-character fragment, or the obligation
// itself, passing as the RFC's statement that the feature is optional.
// METHOD: rows of the fixture RFC (featureFixtureSource) parsed by the real
// parser and judged by the exclusion evidence ratchet's gap, the path that
// refuses every {feature-declined} row by id.

package rfc

import (
	"encoding/json"
	"strings"
	"testing"
)

const sectionedProducer = "internal/x/frame.go::sendFrame sends one frame"

// The optionality sentences of featureFixtureSource, and the obligation a row
// citing section 4 states. fixturePaddingSentence sits in section 2.1,
// fixtureOptionalSentence, fixtureOptionallySentence, fixtureSupportsSentence
// and fixtureUnmarkedSentence in section 4, and fixtureAppendixSentence in
// appendix A.1.
const (
	fixturePaddingSentence    = "A sender MAY pad the Length field to 64 octets."
	fixtureOptionalSentence   = "Compression of the frames of one widget is OPTIONAL."
	fixtureOptionallySentence = "A widget optionally carries a trailer."
	fixtureSupportsSentence   = "If a receiver supports compression, it says so in its first frame."
	fixtureUnmarkedSentence   = "Each compressed frame carries a trailer."
	fixtureCompressedSentence = "A sender that compresses a frame MUST mark it as compressed."
	fixtureAppendixSentence   = "A receiver MAY ignore a widget extension."
	fixtureRowObligation      = "A receiver MUST expand each compressed frame."
)

// featureFixtureSource is quoteFixtureSource with an optional sentence in
// section 2.1, a section 4 that makes compression optional and states the
// obligations conditional on it, and an appendix A with a subsection A.1.
var featureFixtureSource = strings.Replace(quoteFixtureSource,
	fixtureRelaySentence+"\n\n", fixtureRelaySentence+"\n\n   "+fixturePaddingSentence+"\n\n", 1) +
	"\n4.  Compression\n\n" +
	"   " + fixtureOptionalSentence + "\n\n" +
	"   " + fixtureCompressedSentence + "\n\n" +
	"   " + fixtureRowObligation + "\n\n" +
	"   " + fixtureOptionallySentence + "\n\n" +
	"   " + fixtureSupportsSentence + "\n\n" +
	"   " + fixtureUnmarkedSentence + "\n\n" +
	"Appendix A.  Extensions\n\n" +
	"   This appendix describes widget extensions.\n\n" +
	"A.1.  Ignoring Extensions\n\n" +
	"   " + fixtureAppendixSentence + "\n"

// sectionedRow is a row of the fixture RFC citing section, annotated with a
// {feature-declined} payload quoting quote and naming named as its section
// (no section when named is empty). Its own sentence is fixtureRowObligation
// rather than the quote, as a real row's is: a row declares the conditional
// obligation, and the quote the optionality.
func sectionedRow(t *testing.T, section, named, quote string) Requirement {
	t.Helper()
	payload := `feature-declined: "` + quote + `"; ` + sectionedProducer
	if named != "" {
		payload = `feature-declined: §` + named + ` "` + quote + `"; ` + sectionedProducer
	}
	row := evidenceRow(t, "RFC9999-"+section+"-1", section, payload)
	row.Level = levelMust
	row.Text = fixtureRowObligation + " (§" + section + ")"
	return row
}

// featureFixtureTree writes featureFixtureSource under a temporary checkout.
func featureFixtureTree(t *testing.T) string {
	t.Helper()
	tree := t.TempDir()
	writeFixtureFiles(t, tree, map[string]string{"rfc/full/rfc9999.txt": featureFixtureSource})
	return tree
}

func TestAFeatureDeclinedPayloadNamesItsQuotesSection(t *testing.T) {
	const quote = fixtureOptionalSentence
	ann, err := parseAnnotation(`feature-declined: §4 "`+quote+`"; `+sectionedProducer, "fixture")
	if err != nil {
		t.Fatalf("a payload naming its quote's section was refused: %v", err)
	}
	if ann.QuoteSection != "4" {
		t.Errorf("quote section: %q, want %q", ann.QuoteSection, "4")
	}
	if ann.Quote != quote {
		t.Errorf("quote: %q, want %q", ann.Quote, quote)
	}
	if !strings.HasPrefix(ann.Reason, "§4 ") {
		t.Errorf("reason %q does not keep the whole body, section included", ann.Reason)
	}
	appendix, err := parseAnnotation(`feature-declined: §A.1 "`+fixtureAppendixSentence+`"; `+sectionedProducer, "fixture")
	if err != nil {
		t.Fatalf("a payload naming an appendix section was refused: %v", err)
	}
	if appendix.QuoteSection != "A.1" {
		t.Errorf("appendix quote section: %q, want %q", appendix.QuoteSection, "A.1")
	}
	plain, err := parseAnnotation(`feature-declined: "`+quote+`"; `+sectionedProducer, "fixture")
	if err != nil {
		t.Fatalf("the form naming no section was refused: %v", err)
	}
	if plain.QuoteSection != "" {
		t.Errorf("a payload naming no section answered section %q", plain.QuoteSection)
	}
}

// VALIDATES: the quote section survives the stored JSON form of an
// annotation, under its kebab-case key, so a reader of the stored ledger
// holds the quote to the same section the parser read.
// METHOD: one parsed annotation marshaled and read back.
func TestAFeatureDeclinedQuoteSectionRoundTripsThroughJSON(t *testing.T) {
	ann, err := parseAnnotation(`feature-declined: §A.1 "`+fixtureAppendixSentence+`"; `+sectionedProducer, "fixture")
	if err != nil {
		t.Fatalf("the fixture annotation did not parse: %v", err)
	}
	stored, err := json.Marshal(ann)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(stored), `"quote-section":"A.1"`) {
		t.Errorf("the stored form %s does not carry \"quote-section\":\"A.1\"", stored)
	}
	var read Annotation
	if err := json.Unmarshal(stored, &read); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if read.QuoteSection != "A.1" || read.Quote != fixtureAppendixSentence {
		t.Errorf("read back section %q quote %q, want A.1 and %q", read.QuoteSection, read.Quote, fixtureAppendixSentence)
	}
}

func TestAMalformedFeatureDeclinedSectionIsRefused(t *testing.T) {
	const sentence = `"` + fixtureOptionalSentence + `"; ` + sectionedProducer
	cases := []struct{ name, payload, says string }{
		{"a section mark with no section", `§ ` + sentence,
			"needs the RFC's own sentence in double quotes"},
		{"a section glued to its quote", `§3` + sentence,
			"needs the RFC's own sentence in double quotes"},
		{"a named section keeps the length rule", `§3 "reorder"; ` + sectionedProducer,
			"identifies no sentence"},
		{"a section carrying punctuation", `§2.2; ` + sentence, "is not a section id"},
		{"a section carrying a word", `§2.foo ` + sentence, "is not a section id"},
		{"a section carrying a trailing dot", `§2. ` + sentence, "is not a section id"},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			_, err := parseAnnotation("feature-declined: "+one.payload, "fixture")
			if err == nil {
				t.Fatalf("{feature-declined: %s} parsed, want a refusal", one.payload)
			}
			if !strings.Contains(err.Error(), one.says) || !strings.Contains(err.Error(), featureDeclinedFormat) {
				t.Errorf("refusal %q does not say %q and end with the format", err, one.says)
			}
		})
	}
}

// featureCase is one {feature-declined} row: the section it cites, the
// section its payload names (empty for none), its quote, the row's own
// sentence when it is not fixtureRowObligation, and what a refusal says.
type featureCase struct {
	name, cites, named, quote, rowText string
	says                               []string
}

func TestAFeatureDeclinedQuoteIsHeldToTheSectionItNames(t *testing.T) {
	cases := []featureCase{
		{name: "a quote verbatim in the section named", cites: "2", named: "4", quote: fixtureOptionalSentence},
		{name: "a quote in an appendix subsection named", cites: "3", named: "A.1", quote: fixtureAppendixSentence},
		{name: "a quote in a subsection of the section named", cites: "3", named: "A", quote: fixtureAppendixSentence},
		{name: "a quote another section carries", cites: "2", named: "3", quote: fixtureOptionalSentence,
			says: []string{"is not in section 3", fixtureOptionalSentence, "it is in section 4"}},
		{name: "a quote no section carries", cites: "2", named: "3",
			quote: "no section of this RFC says this sentence",
			says:  []string{"section 3", "no section of this RFC says this sentence", "in no section of rfc/full/rfc9999.txt"}},
		{name: "a section the RFC has no heading for", cites: "2", named: "7", quote: fixtureOptionalSentence,
			says: []string{"names section 7", "no heading of the RFC"}},
		{name: "a subsection the RFC has no heading for, under a heading holding the quote", cites: "3", named: "2.99",
			quote: fixturePaddingSentence, says: []string{"names section 2.99", "no heading of the RFC"}},
		{name: "the row's own section", cites: "4", named: "4", quote: fixtureOptionalSentence,
			says: []string{"names section 4", "redundant", "drop the §4"}},
		{name: "a subsection of the row's own section", cites: "2", named: "2.1", quote: fixturePaddingSentence,
			says: []string{"names section 2.1", "redundant", "drop the §2.1"}},
		{name: "an ancestor of the row's section, for a quote in the row's section", cites: "2.1", named: "2",
			quote: fixturePaddingSentence, says: []string{"names section 2", "redundant", "drop the §2"}},
	}
	tree := featureFixtureTree(t)
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			row := sectionedRow(t, one.cites, one.named, one.quote)
			if one.rowText != "" {
				row.Text = one.rowText + " (§" + one.cites + ")"
			}
			missing, lacks := newEvidenceReader(tree).gap(&row)
			if len(one.says) == 0 {
				if lacks {
					t.Fatalf("§%s %q under a row citing section %s was refused: %s", one.named, one.quote, one.cites, missing)
				}
				return
			}
			if !lacks {
				t.Fatalf("§%s %q under a row citing section %s was accepted", one.named, one.quote, one.cites)
			}
			for _, want := range one.says {
				if !strings.Contains(missing, want) {
					t.Errorf("the refusal %q does not say %q", missing, want)
				}
			}
		})
	}
}

func TestAFeatureDeclinedQuoteIsAnOptionalitySentence(t *testing.T) {
	cases := []featureCase{
		{name: "an upper-case OPTIONAL sentence of the row's section", cites: "4", quote: fixtureOptionalSentence},
		{name: "a MAY sentence of a subsection of the row's section", cites: "2", quote: fixturePaddingSentence},
		{name: "a lower-case optionally sentence", cites: "4", quote: fixtureOptionallySentence},
		{name: "an if-supports conditional", cites: "4", quote: fixtureSupportsSentence},
		{name: "a fragment of an optionality sentence", cites: "4", quote: "frames of one widget is OPTIONAL",
			says: []string{"not a whole sentence of section 4", "frames of one widget is OPTIONAL"}},
		{name: "a fragment, from the section named", cites: "2", named: "4", quote: "frames of one widget is OPTIONAL",
			says: []string{"not a whole sentence of section 4"}},
		{name: "a sentence whose only keyword is MUST, guarded by no condition", cites: "4", quote: fixtureRowObligation,
			says: []string{"only BCP 14 keywords are mandatory", "MUST", featureDeclinedShapes}},
		{name: "a conditional MUST sentence with no never clause", cites: "4", quote: fixtureCompressedSentence,
			rowText: fixtureCompressedSentence, says: []string{"conditional obligation", "no never clause"}},
		{name: "a MUST sentence, from the section named", cites: "2", named: "3",
			quote: "A receiver MUST NOT reorder the frames of one widget.",
			says:  []string{"only BCP 14 keywords are mandatory", "MUST NOT"}},
		{name: "a sentence with no optionality marker", cites: "4", quote: fixtureUnmarkedSentence,
			says: []string{"no optionality marker", "OPTIONAL", "optionally", featureDeclinedShapes}},
		{name: "the row's own sentence", cites: "4", quote: fixtureOptionalSentence, rowText: fixtureOptionalSentence,
			says: []string{"holds the row's own sentence"}},
	}
	tree := featureFixtureTree(t)
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			row := sectionedRow(t, one.cites, one.named, one.quote)
			if one.rowText != "" {
				row.Text = one.rowText + " (§" + one.cites + ")"
			}
			missing, lacks := newEvidenceReader(tree).gap(&row)
			if len(one.says) == 0 {
				if lacks {
					t.Fatalf("§%s %q under a row citing section %s was refused: %s", one.named, one.quote, one.cites, missing)
				}
				return
			}
			if !lacks {
				t.Fatalf("§%s %q under a row citing section %s was accepted", one.named, one.quote, one.cites)
			}
			for _, want := range one.says {
				if !strings.Contains(missing, want) {
					t.Errorf("the refusal %q does not say %q", missing, want)
				}
			}
		})
	}
}
