// VALIDATES: the third evidence shape of {feature-declined}, the conditional.
// The quote is a whole sentence of the row's section, or of a section the
// payload names, whose obligation is guarded by a clause on the feature's use:
// a leading If, When, Where, Once or Unless clause, or a subject held by a
// restrictive relative clause ("a client that previously received ... MUST").
// It may be the row's own sentence, because that is where the condition lives.
// Its reason carries `ze never <condition>: <path>.go::<Symbol> <how>`, and the
// producer it names exists in the tree.
// PREVENTS: a declined feature leaving the count on an obligation the RFC never
// made conditional ("A receiver MUST NOT reorder ..."), or on a condition with
// no evidence that Ze never meets it, or on a producer nobody can open.
// METHOD: rows parsed by the real parser and judged by the exclusion evidence
// ratchet's gap, over the fixture RFC and over the real text of RFC 9915.

package rfc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// conditionalProducerFile is the producer every conditional fixture names, and
// conditionalNever the never clause naming it.
const (
	conditionalProducerFile = "internal/x/frame.go"
	conditionalNever        = "ze never compresses a frame: internal/x/frame.go::sendFrame sends every frame uncompressed"
	fixturePluralSentence   = "Receivers that expand a frame MUST keep its trailer."
	fixtureMixedSentence    = "A sender MAY pad a frame, and, if it compresses a frame, it MUST mark the frame."
)

// conditionalFixtureTree writes featureFixtureSource, with a plural-subject
// conditional and an optional sentence holding a conditional MUST added to
// section 4, and the producer file under a temporary checkout.
func conditionalFixtureTree(t *testing.T) string {
	t.Helper()
	source := strings.Replace(featureFixtureSource,
		fixtureUnmarkedSentence+"\n\n", fixtureUnmarkedSentence+"\n\n   "+fixturePluralSentence+"\n\n   "+fixtureMixedSentence+"\n\n", 1)
	tree := t.TempDir()
	writeFixtureFiles(t, tree, map[string]string{
		"rfc/full/rfc9999.txt":  source,
		conditionalProducerFile: "package x\n\nfunc sendFrame() {}\n",
	})
	return tree
}

// conditionalRow is a row of stem citing section, its own sentence rowText,
// annotated {feature-declined} with payload.
func conditionalRow(t *testing.T, stem, section, rowText, payload string) Requirement {
	t.Helper()
	parsed, err := parseAnnotation(payload, "fixture")
	if err != nil {
		t.Fatalf("the fixture annotation {%s} did not parse: %v", payload, err)
	}
	return Requirement{RFC: stem, RID: "RFC-" + section + "-1", Level: levelMust,
		Text: rowText + " (§" + section + ")", Section: section,
		Source: selftestSummaryRel, Line: 7, Annotation: parsed}
}

// conditionalCase is one conditional row: its section, the section its
// payload names, the quote, the row's own sentence (the quote when empty),
// the reason after the quote, and what a refusal says (accepted when empty).
type conditionalCase struct {
	name, cites, named, quote, rowText, reason string
	says                                       []string
}

func judgeConditional(t *testing.T, tree, stem string, cases []conditionalCase) {
	t.Helper()
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			payload := `feature-declined: "` + one.quote + `"; ` + one.reason
			if one.named != "" {
				payload = `feature-declined: §` + one.named + ` "` + one.quote + `"; ` + one.reason
			}
			rowText := one.rowText
			if rowText == "" {
				rowText = one.quote
			}
			row := conditionalRow(t, stem, one.cites, rowText, payload)
			missing, lacks := newEvidenceReader(tree).gap(&row)
			if len(one.says) == 0 {
				if lacks {
					t.Fatalf("%q under a row citing section %s was refused: %s", one.quote, one.cites, missing)
				}
				return
			}
			if !lacks {
				t.Fatalf("%q under a row citing section %s was accepted", one.quote, one.cites)
			}
			for _, want := range one.says {
				if !strings.Contains(missing, want) {
					t.Errorf("the refusal %q does not say %q", missing, want)
				}
			}
		})
	}
}

func TestAFeatureDeclinedConditionalNeedsItsNeverClause(t *testing.T) {
	const missingFile = "ze never compresses a frame: internal/x/gone.go::sendFrame sends every frame uncompressed"
	const missingSymbol = "ze never compresses a frame: internal/x/frame.go::compressFrame is never called"
	const otherSentence = "A receiver MUST NOT reorder the frames of one widget."
	const unrelatedNever = "ze never runs a widget: internal/x/frame.go::sendFrame sends every frame uncompressed"
	const roleOnlyNever = "ze never is a sender: internal/x/frame.go::sendFrame sends every frame uncompressed"
	cases := []conditionalCase{
		{name: "a relative-clause subject, the row's own sentence", cites: "4",
			quote: fixtureCompressedSentence, reason: conditionalNever},
		{name: "a relative-clause subject, under another row sentence", cites: "4",
			quote: fixtureCompressedSentence, rowText: fixtureRowObligation, reason: conditionalNever,
			says: []string{"the conditional must be the row's own sentence"}},
		{name: "a plural subject held by a relative clause", cites: "4",
			quote: fixturePluralSentence, reason: conditionalNever},
		{name: "a conditional from the section named", cites: "2", named: "4",
			quote: fixtureCompressedSentence, rowText: fixtureCompressedSentence, reason: conditionalNever,
			says: []string{"the conditional must be the row's own sentence", "section 4"}},
		{name: "an optional sentence with a conditional MUST, under another row sentence", cites: "4",
			quote: fixtureMixedSentence, rowText: fixtureRowObligation, reason: sectionedProducer,
			says: []string{"no never clause"}},
		{name: "an optional sentence with a conditional MUST and its never clause", cites: "4",
			quote: fixtureMixedSentence, rowText: fixtureRowObligation, reason: conditionalNever},
		{name: "a never clause on a condition the guard does not name", cites: "4",
			quote: fixtureCompressedSentence, reason: unrelatedNever,
			says: []string{"shares no word", "'compresses a frame'", "'runs a widget'"}},
		{name: "a never clause sharing only a role word and a stopword", cites: "4",
			quote: fixtureCompressedSentence, reason: roleOnlyNever,
			says: []string{"shares no word", "'compresses a frame'"}},
		{name: "a conditional with no never clause", cites: "4",
			quote: fixtureCompressedSentence, reason: sectionedProducer,
			says: []string{"no never clause", "ze never <condition>: <path>.go::<Symbol> <how>"}},
		{name: "a never clause naming a file the tree does not carry", cites: "4",
			quote: fixtureCompressedSentence, reason: missingFile,
			says: []string{"internal/x/gone.go::sendFrame", "does not carry"}},
		{name: "a never clause naming a function its file does not declare", cites: "4",
			quote: fixtureCompressedSentence, reason: missingSymbol,
			says: []string{"internal/x/frame.go::compressFrame", "declares no compressFrame"}},
		{name: "a never clause naming a test", cites: "4", quote: fixtureCompressedSentence,
			reason: "internal/x/frame.go::sendFrame sends frames; ze never compresses a frame: internal/x/frame_test.go::TestSendFrame checks it",
			says:   []string{"names the test internal/x/frame_test.go::TestSendFrame"}},
		{name: "a mandatory sentence with no conditional clause", cites: "2", named: "3",
			quote: otherSentence, rowText: fixtureRowObligation, reason: conditionalNever,
			says: []string{"only BCP 14 keywords are mandatory", "MUST NOT", featureDeclinedShapes}},
		{name: "a fragment of a conditional", cites: "4",
			quote: "that compresses a frame MUST mark it as compressed", reason: conditionalNever,
			says: []string{"not a whole sentence of section 4"}},
	}
	judgeConditional(t, conditionalFixtureTree(t), selftestStem, cases)
}

// VALIDATES: the two conditional sentences of RFC 9915 the owner's ruling
// names are accepted in their real text: §6.3's leading If clause and
// §18.2.10.5's "a client that previously received" subject, which follows a
// discourse phrase ("As a counterexample,").
// METHOD: rfc/full/rfc9915.txt copied into a fixture checkout beside a
// producer file, rows citing each section judged by gap.
func TestAFeatureDeclinedConditionalHoldsInRFC9915(t *testing.T) {
	const delegated = "If the client uses a delegated prefix to configure addresses on interfaces on itself or other nodes behind it, the preferred and valid lifetimes of those addresses MUST be no longer than the remaining preferred and valid lifetimes, respectively, for the delegated prefix at any time."
	const ntp = "As a counterexample, a client that previously received an NTP server address from the DHCP server and does not receive it anymore MUST stop using the configured NTP server address."
	const never = "ze never configures an address from a delegated prefix: internal/x/frame.go::sendFrame sends every frame uncompressed"
	text, err := os.ReadFile(filepath.Join("..", "..", "..", "rfc", "full", "rfc9915.txt"))
	if err != nil {
		t.Fatalf("read the RFC 9915 text: %v", err)
	}
	tree := t.TempDir()
	writeFixtureFiles(t, tree, map[string]string{
		"rfc/full/rfc9915.txt":  string(text),
		conditionalProducerFile: "package x\n\nfunc sendFrame() {}\n",
	})
	cases := []conditionalCase{
		{name: "§6.3 the leading If clause, the row's own sentence", cites: "6.3", quote: delegated, reason: never},
		{name: "§18.2.10.5 the client that previously received", cites: "18.2.10.5", quote: ntp, reason: never},
		{name: "§6.3 with no never clause", cites: "6.3", quote: delegated, reason: sectionedProducer,
			says: []string{"no never clause"}},
	}
	judgeConditional(t, tree, "rfc9915", cases)
}

// VALIDATES: the openings a conditional is read from, and the ones it is not.
// METHOD: conditionalGuard over sentences, without a tree.
func TestAConditionalOpeningIsALeadingClauseOrARelativeSubject(t *testing.T) {
	cases := []struct {
		quote string
		want  bool
	}{
		{"If the client uses a prefix, it MUST stop.", true},
		{"When a speaker sends a frame, it MUST mark it.", true},
		{"Where a node relays, the node MUST count.", true},
		{"Once a peer is up, it MUST NOT send it.", true},
		{"Unless a peer asks, a node MUST NOT send it.", true},
		{"In particular, if the prefix is advertised, the lifetimes MUST NOT exceed it.", true},
		{"A sender that compresses a frame MUST mark it.", true},
		{"The client that holds a lease MUST renew it.", true},
		{"Receivers that expand a frame MUST keep it.", true},
		{"A receiver MUST NOT reorder the frames of one widget.", false},
		{"The client MUST ensure that the lease is renewed.", false},
		{"Each frame MUST carry a trailer if it is compressed.", false},
		{"Clients MUST stop if the server says so.", false},
		{"A receiver of the frame that arrived MUST keep it.", false},
		{"A sender to the peer that compresses MUST mark it.", false},
		{"A 1 octet field that MUST be set to 0.", false},
		{"The rule is that a receiver MUST keep it.", false},
		{"The rules are that a receiver MUST keep it.", false},
		{"The text clarifies that a receiver MUST keep it.", false},
		{"The section defines that a receiver MUST keep it.", false},
		{"This is that a receiver MUST keep it.", false},
		{"A sender MAY pad a frame, and, if it does, it MUST mark it.", true},
		{"Notify the receiver that a frame MUST be kept.", false},
		{"This means that a receiver MUST NOT reorder frames.", false},
		{"The document requires that a receiver MUST keep it.", false},
		{"A receiver MUST NOT reorder frames that arrive.", false},
		{"No keyword guards this sentence if it is read.", false},
	}
	for _, one := range cases {
		if _, got := conditionalGuard(one.quote); got != one.want {
			t.Errorf("conditionalGuard(%q) answered %v, want %v", one.quote, got, one.want)
		}
	}
}

// VALIDATES: a never clause's condition meets the guard clause only through a
// content word, compared singular, never through a stopword, a determiner or
// a generic role word.
// METHOD: sharedContentWord over condition and guard pairs, without a tree.
func TestANeverClauseSharesAContentWordWithItsGuard(t *testing.T) {
	cases := []struct {
		condition, guard string
		want             bool
	}{
		{"compresses a frame", "compresses a frame", true},
		{"configures an address", "the client configure addresses on itself", true},
		{"runs a widget", "compresses a frame", false},
		{"is a sender", "a sender is up", false},
		{"has the receivers", "a receiver has it", false},
	}
	for _, one := range cases {
		if got := sharedContentWord(one.condition, one.guard); got != one.want {
			t.Errorf("sharedContentWord(%q, %q) answered %v, want %v", one.condition, one.guard, got, one.want)
		}
	}
}
