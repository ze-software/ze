// VALIDATES: the {feature-declined} annotation kind -- its format, the RFC
// sentence and the producer its reason must name, its place inside the gated
// denominator and outside the proven numerator, and its inability to take a
// {gap}'s slot.
// PREVENTS: a second {not-applicable}. That kind says "this never bound Ze" and
// nothing in the tree can contradict it; this one says "the RFC makes this
// feature optional, in THESE words, and THIS function does the narrower thing
// ze chose", and the gate goes and checks both. Drop either demand and the
// difference is gone.

package rfc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fixture RFC text, the checklist line that quotes it, and the producer
// that line names. One quote in one place, so a case that edits the text and a
// case that edits the line cannot drift apart.
const (
	featureRFCSource = "2.5.1.  Extended (64-bit) Sequence Number\n\n" +
		"   To support high-speed IPsec implementations, a new option for\n" +
		"   sequence numbers SHOULD be offered, as an extension to the current,\n" +
		"   32-bit sequence number field.  Use of an Extended Sequence Number\n" +
		"   (ESN) MUST be negotiated by an SA management protocol.\n"
	featureQuote    = "a new option for sequence numbers SHOULD be offered"
	featureProducer = "internal/plugins/ospf/ipsec_install.go::buildIPsecSA"
	featureReason   = featureProducer + " installs one manually keyed SA per interface, and " +
		"ze negotiates no AH SA at all, so no ESN is ever used"
	featureLine = "- [ ] [RFC9999-2.5.1-1] [MUST] Use of an ESN MUST be negotiated by an SA " +
		"management protocol (§2.5.1) {feature-declined: \"" + featureQuote + "\"; " +
		featureReason + "}"
)

// featureTree writes one RFC text under a temporary checkout and answers its
// root, so the check reads the source through SourcePath like every other
// reader of it.
func featureTree(t *testing.T, stem, source string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, fullRel), 0o750); err != nil {
		t.Fatalf("make the fixture source directory: %v", err)
	}
	if source == "" {
		return root
	}
	if err := os.WriteFile(filepath.Join(root, fullRel, stem+".txt"), []byte(source), 0o600); err != nil {
		t.Fatalf("write the fixture source: %v", err)
	}
	return root
}

func TestAFeatureDeclinedAnnotationCarriesItsQuoteAndItsProducer(t *testing.T) {
	req, err := oneLine(t, featureLine)
	if err != nil {
		t.Fatalf("a well-formed feature-declined annotation: %v", err)
	}
	if req.Annotation == nil || req.Annotation.Kind != AnnotationFeatureDeclined {
		t.Fatalf("the annotation was lost: %+v", req.Annotation)
	}
	if req.Annotation.Quote != featureQuote {
		t.Errorf("quote: %q, want %q", req.Annotation.Quote, featureQuote)
	}
	if req.Annotation.Producer != featureProducer {
		t.Errorf("producer: %q, want %q", req.Annotation.Producer, featureProducer)
	}
	// Reason keeps the whole body, so a renderer that knows only `{kind} reason`
	// still publishes the quote and the producer.
	if !strings.Contains(req.Annotation.Reason, featureQuote) ||
		!strings.Contains(req.Annotation.Reason, featureProducer) {
		t.Errorf("reason: %q, want the quote and the producer", req.Annotation.Reason)
	}
	if strings.Contains(req.Text, "{") {
		t.Errorf("the marker was left inside the requirement text: %q", req.Text)
	}
	if req.Section != "2.5.1" {
		t.Errorf("the marker moved the anchor: %q", req.Section)
	}
}

// TestAFeatureDeclinedReasonIsRefusedWithoutBothFacts drives the two halves of
// the format apart, because a reason with no quote and a reason with no
// producer are each the assertable claim this kind may not become.
func TestAFeatureDeclinedReasonIsRefusedWithoutBothFacts(t *testing.T) {
	cases := []struct {
		name, body, refused string
	}{
		{
			name:    "prose where the quote should be",
			body:    "{feature-declined: ESN is optional; " + featureReason + "}",
			refused: "needs the RFC's own sentence in double quotes",
		},
		{
			name:    "a quote with nothing after it",
			body:    "{feature-declined: \"" + featureQuote + "\";}",
			refused: "needs the RFC's own sentence in double quotes",
		},
		{
			name:    "a quote too short to identify a sentence",
			body:    "{feature-declined: \"is optional\"; " + featureReason + "}",
			refused: "identifies no sentence",
		},
		{
			name: "a quote and prose, but no producer",
			body: "{feature-declined: \"" + featureQuote + "\"; ze negotiates no AH SA, " +
				"so no ESN is ever used}",
			refused: "names no producer",
		},
		{
			name: "a test named as the producer",
			body: "{feature-declined: \"" + featureQuote + "\"; " +
				"internal/plugins/ospf/ipsec_install_test.go::TestBuildIPsecSA covers it}",
			refused: "decides nothing about what ze offers",
		},
		{
			name: "a producer naming no symbol",
			body: "{feature-declined: \"" + featureQuote + "\"; " +
				"internal/plugins/ospf/ipsec_install.go installs the manual SA}",
			refused: "names no producer",
		},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			_, err := oneLine(t, "- [ ] [RFC9999-2-1] [MUST] A speaker MUST send it (§2) "+one.body)
			if err == nil {
				t.Fatalf("%s parsed, want a refusal", one.body)
			}
			if !strings.Contains(err.Error(), one.refused) {
				t.Errorf("refusal %q does not say %q", err.Error(), one.refused)
			}
			if !strings.Contains(err.Error(), "Format: {feature-declined:") {
				t.Errorf("refusal %q shows the author no format", err.Error())
			}
		})
	}
}

// TestAFeatureDeclinedAnnotationCannotDisplaceAGap holds the property the note
// above SupersededKind records: a way OUT of the gated population must not be
// creatable by adding a marker beside the one already there.
func TestAFeatureDeclinedAnnotationCannotDisplaceAGap(t *testing.T) {
	const declined = "{feature-declined: \"" + featureQuote + "\"; " + featureReason + "}"
	for _, line := range []string{
		"- [ ] [RFC9999-2-1] [MUST] A speaker MUST send it (§2) {gap: no producer yet} " + declined,
		"- [ ] [RFC9999-2-1] [MUST] A speaker MUST send it (§2) " + declined + " {gap: no producer yet}",
	} {
		req, err := oneLine(t, line)
		if err == nil {
			t.Fatalf("a line carrying both annotations parsed as %+v, want a refusal", req.Annotation)
		}
		if !strings.Contains(err.Error(), "two coverage annotations on one line") {
			t.Errorf("refusal %q does not name the collision", err.Error())
		}
	}
}

// TestAFeatureDeclinedRequirementIsAnnotatedAndNotProven runs the same corpus
// twice, once with the annotation and once without it, and demands the
// published share be identical.
//
// The obligation's condition is false, so nothing is owed and nothing is
// proven: it belongs in the denominator both times and in the numerator neither
// time. Annotating a row may not move the published percentage by a point,
// which is what would happen if the kind reached CoverageRow.Both or the
// single-polarity arm of ProvenShareOf.
func TestAFeatureDeclinedRequirementIsAnnotatedAndNotProven(t *testing.T) {
	metas := map[string]Meta{"rfc1": {Enrolment: enrolmentEnrolled, Support: "core", Status: "Supported"}}
	tags := []Tag{
		{RID: "RFC1-1-1", Polarity: PolarityPositive, File: "a_test.go"},
		{RID: "RFC1-1-1", Polarity: PolarityNegative, File: "a_test.go"},
	}
	bare := []Requirement{
		{RFC: "rfc1", RID: "RFC1-1-1", Level: levelMust},
		{RFC: "rfc1", RID: "RFC1-1-2", Level: levelMust},
	}
	annotated := []Requirement{
		{RFC: "rfc1", RID: "RFC1-1-1", Level: levelMust},
		{RFC: "rfc1", RID: "RFC1-1-2", Level: levelMust, Annotation: &Annotation{
			Kind: AnnotationFeatureDeclined, Quote: featureQuote,
			Producer: featureProducer, Reason: featureReason,
		}},
	}

	rows := CoverageRows(annotated, tags, nil)
	if len(rows) != 1 {
		t.Fatalf("CoverageRows answered %d rows, want 1", len(rows))
	}
	row := rows[0]
	if row.Annotated != 1 {
		t.Errorf("Annotated = %d, want 1: a feature-declined requirement is annotated", row.Annotated)
	}
	if row.Both != 1 || row.One != 0 || row.Missing != 0 {
		t.Errorf("both=%d one=%d missing=%d, want the annotated requirement in none of them",
			row.Both, row.One, row.Missing)
	}
	if row.Gated != 2 {
		t.Errorf("Gated = %d, want 2: the annotation does not leave the population", row.Gated)
	}

	before, err := ProvenShareOf(metas, bare, tags, nil)
	if err != nil {
		t.Fatalf("ProvenShareOf over the bare corpus: %v", err)
	}
	after, err := ProvenShareOf(metas, annotated, tags, nil)
	if err != nil {
		t.Fatalf("ProvenShareOf over the annotated corpus: %v", err)
	}
	if after != before {
		t.Errorf("the published share moved when a requirement gained {feature-declined}: %+v, was %+v",
			after, before)
	}
	if after.Proven != 1 || after.Gated != 2 {
		t.Errorf("Proven=%d Gated=%d, want 1 of 2: a declined feature is not proven by ze",
			after.Proven, after.Gated)
	}
}

// TestAFeatureDeclinedAnnotationIsStaleBesideATaggedTest checks the arm of
// evaluate that {feature-declined} joined. A requirement Ze CAN prove is a
// requirement whose feature Ze offers, and the tag is the evidence that it
// does.
func TestAFeatureDeclinedAnnotationIsStaleBesideATaggedTest(t *testing.T) {
	requirements := []Requirement{{RFC: "rfc1", RID: "RFC1-1-1", Level: levelMust,
		Source: "rfc/short/rfc1.md", Line: 7, Annotation: &Annotation{
			Kind: AnnotationFeatureDeclined, Quote: featureQuote,
			Producer: featureProducer, Reason: featureReason,
		}}}
	tags := []Tag{{RID: "RFC1-1-1", Polarity: PolarityPositive, File: "a_test.go", Line: 3}}

	findings := evaluate(requirements, tags, map[string]bool{"rfc1": true})
	if len(findings) != 1 {
		t.Fatalf("evaluate answered %d findings, want 1: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "the annotation is stale") {
		t.Errorf("finding %q does not say the annotation is stale", findings[0].Message)
	}
}

// TestCheckFeatureDeclinedReadsTheRFCAndTheTree is the check that makes the
// kind checkable rather than assertable. Both facts are held against something
// a reader can open, and each way either one can rot is a case here.
func TestCheckFeatureDeclinedReadsTheRFCAndTheTree(t *testing.T) {
	const file = "internal/plugins/ospf/ipsec_install.go"
	source := "package ospf\n\nfunc buildIPsecSA(ifindex int) int {\n\treturn ifindex\n}\n"

	cases := []struct {
		name, quote, producer, rfcText, refused string
	}{
		{
			name:  "a quote and a producer this checkout can show",
			quote: featureQuote, producer: featureProducer, rfcText: featureRFCSource,
		},
		{
			// Line breaks are what the RFC's own text puts inside a sentence,
			// so a quote that reads as one sentence has to match across them.
			name: "a quote the RFC wraps over three lines",
			quote: "a new option for sequence numbers SHOULD be offered, as an extension to " +
				"the current, 32-bit sequence number field",
			producer: featureProducer, rfcText: featureRFCSource,
		},
		{
			name:     "a sentence the RFC does not contain",
			quote:    "an Extended Sequence Number is an optional feature nobody has to offer",
			producer: featureProducer, rfcText: featureRFCSource,
			refused: "is not in rfc/full/rfc1.txt",
		},
		{
			name:  "an RFC whose text this repository does not hold",
			quote: featureQuote, producer: featureProducer, rfcText: "",
			refused: "the RFC's own text is not in this repository",
		},
		{
			name:  "a symbol the file no longer declares",
			quote: featureQuote, producer: file + "::buildIPsecSAOld", rfcText: featureRFCSource,
			refused: "declares no buildIPsecSAOld",
		},
		{
			name:  "a file this checkout does not carry",
			quote: featureQuote, producer: "internal/gone/away.go::buildIPsecSA",
			rfcText: featureRFCSource,
			refused: "this checkout does not carry",
		},
		{
			// Unreachable through the parser, which refuses the line first. The
			// check still refuses it, because a guard that skips the one state
			// it exists to catch is not a guard.
			name:  "an annotation carrying no producer at all",
			quote: featureQuote, producer: "", rfcText: featureRFCSource,
			refused: "names no producer",
		},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			requirements := []Requirement{{RFC: "rfc1", RID: "RFC1-2.5.1-1", Level: levelMust,
				Source: "rfc/short/rfc1.md", Line: 7, Annotation: &Annotation{
					Kind: AnnotationFeatureDeclined, Quote: one.quote,
					Producer: one.producer, Reason: one.producer + " installs the manual SA",
				}}}
			errs := checkFeatureDeclined(featureTree(t, "rfc1", one.rfcText),
				newTextReader(map[string]string{file: source}), requirements)
			if one.refused == "" {
				if len(errs) != 0 {
					t.Fatalf("a quote and a producer the tree holds were refused: %v", errs)
				}
				return
			}
			if len(errs) != 1 {
				t.Fatalf("checkFeatureDeclined answered %d errors, want 1: %v", len(errs), errs)
			}
			if !strings.Contains(errs[0], one.refused) {
				t.Errorf("refusal %q does not say %q", errs[0], one.refused)
			}
		})
	}
}
