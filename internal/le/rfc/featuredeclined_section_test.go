// VALIDATES: a {feature-declined} payload may name the section its optionality
// quote comes from (`§<section> "<quote>"; <reason>`). The quote is then held
// against that section of rfc/full/<stem>.txt, matched as a row-section quote
// is, and a section that is the row's own, or one of its subsections, is
// refused as redundant, so each case has one spelling.
// PREVENTS: a row whose RFC states the feature optional in one section and the
// conditional obligation in another (RFC 9728 §2.2 and §3.3) having no valid
// evidence form, and a named section turning into a way round the section
// scope: the quote must still be verbatim in the section the payload names.
// METHOD: rows of the fixture RFC (quoteFixtureSource) parsed by the real
// parser and judged by the exclusion evidence ratchet's gap, the path that
// refuses every {feature-declined} row by id.

package rfc

import (
	"strings"
	"testing"
)

const sectionedProducer = "internal/x/frame.go::sendFrame sends one frame"

// sectionedRow is a row of the fixture RFC citing section, annotated with a
// {feature-declined} payload that names named as its quote's section.
func sectionedRow(t *testing.T, section, named, quote string) Requirement {
	t.Helper()
	return evidenceRow(t, "RFC9999-"+section+"-1", section,
		`feature-declined: §`+named+` "`+quote+`"; `+sectionedProducer)
}

func TestAFeatureDeclinedPayloadNamesItsQuotesSection(t *testing.T) {
	const quote = "A receiver MUST NOT reorder the frames of one widget."
	ann, err := parseAnnotation(`feature-declined: §3 "`+quote+`"; `+sectionedProducer, "fixture")
	if err != nil {
		t.Fatalf("a payload naming its quote's section was refused: %v", err)
	}
	if ann.QuoteSection != "3" {
		t.Errorf("quote section: %q, want %q", ann.QuoteSection, "3")
	}
	if ann.Quote != quote {
		t.Errorf("quote: %q, want %q", ann.Quote, quote)
	}
	if !strings.HasPrefix(ann.Reason, "§3 ") {
		t.Errorf("reason %q does not keep the whole body, section included", ann.Reason)
	}
	plain, err := parseAnnotation(`feature-declined: "`+quote+`"; `+sectionedProducer, "fixture")
	if err != nil {
		t.Fatalf("the form naming no section was refused: %v", err)
	}
	if plain.QuoteSection != "" {
		t.Errorf("a payload naming no section answered section %q", plain.QuoteSection)
	}
}

func TestAMalformedFeatureDeclinedSectionIsRefused(t *testing.T) {
	cases := []struct{ name, payload, says string }{
		{"a section mark with no section", `§ "A receiver MUST NOT reorder the frames"; ` + sectionedProducer,
			"needs the RFC's own sentence in double quotes"},
		{"a section glued to its quote", `§3"A receiver MUST NOT reorder the frames"; ` + sectionedProducer,
			"needs the RFC's own sentence in double quotes"},
		{"a named section keeps the length rule", `§3 "reorder"; ` + sectionedProducer,
			"identifies no sentence"},
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

func TestAFeatureDeclinedQuoteIsHeldToTheSectionItNames(t *testing.T) {
	tree := t.TempDir()
	writeFixtureFiles(t, tree, map[string]string{"rfc/full/rfc9999.txt": quoteFixtureSource})
	const frames = "A receiver MUST NOT reorder the frames of one widget."
	cases := []struct {
		name, cites, named, quote string
		says                      []string
	}{
		{name: "a quote verbatim in the section named", cites: "2", named: "3", quote: frames},
		{name: "a quote in a subsection of the section named", cites: "3", named: "2", quote: fixtureRelaySentence},
		{name: "a quote another section carries", cites: "2", named: "3", quote: fixtureRelaySentence,
			says: []string{"is not in section 3", fixtureRelaySentence, "it is in section 2.1"}},
		{name: "a quote no section carries", cites: "2", named: "3",
			quote: "no section of this RFC says this sentence",
			says:  []string{"section 3", "no section of this RFC says this sentence", "in no section of rfc/full/rfc9999.txt"}},
		{name: "a section the RFC has no heading for", cites: "2", named: "7", quote: frames,
			says: []string{"names section 7", "no heading of the RFC"}},
		{name: "the row's own section", cites: "3", named: "3", quote: frames,
			says: []string{"names section 3", "redundant", "drop the §3"}},
		{name: "a subsection of the row's own section", cites: "2", named: "2.1", quote: fixtureRelaySentence,
			says: []string{"names section 2.1", "redundant", "drop the §2.1"}},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			row := sectionedRow(t, one.cites, one.named, one.quote)
			missing, lacks := newEvidenceReader(tree).gap(&row)
			if len(one.says) == 0 {
				if lacks {
					t.Fatalf("a quote verbatim in the section named was refused: %s", missing)
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
