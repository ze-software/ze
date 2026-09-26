package rfc

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// backfillFixtureSource carries two MUST sentences in section 2, the two sites the selftest
// artifact maps to RFC9999-2-1 and RFC9999-2-2.
const backfillFixtureSource = "Test RFC 9999\n\n1.  Introduction\n\n    This document describes widgets.\n\n" +
	"2.  Widgets\n\n    A speaker MUST send the widget. A receiver MUST drop the gadget.\n"

// VALIDATES: AC-11 through the entry point -- `./le rfc quote-backfill stem <stem> apply`
// rewrites a row mapped to one site whose paraphrase shares the sentence's words, keeping its
// id, level and section, and lists a row whose paraphrase shares little with its sentence
// for review, unchanged.
func TestQuoteBackfillAppliesSingleSiteRowAndListsLowOverlap(t *testing.T) {
	const lowOverlap = "- [ ] [" + selftestRIDDrop + "] [MUST] Frames arriving late are never reordered (§2)"
	summary := "# RFC 9999\n\n" + selftestMeta + "\n## Compliance Checklist\n\n" +
		"- [ ] [" + selftestRIDSend + "] [MUST] The speaker MUST send a widget (§2)\n" +
		lowOverlap + "\n"
	tree := checkFixtureTree(t, map[string]string{
		"rfc/full/rfc9999.txt": backfillFixtureSource,
		selftestSummaryRel:     summary,
	})
	t.Setenv("ZE_REPO_ROOT", tree)
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	answer, code := Answer([]string{"quote-backfill", "stem", selftestStem, "apply"})
	if code != 0 {
		t.Fatalf("quote-backfill answered %d", code)
	}
	report, isReport := answer.(*quoteBackfillReport)
	if !isReport {
		t.Fatalf("quote-backfill answered %T", answer)
	}
	if len(report.Quoted) != 1 || report.Quoted[0].RID != selftestRIDSend {
		t.Errorf("quoted rows are %+v, want only %s", report.Quoted, selftestRIDSend)
	}
	if len(report.Review) != 1 || report.Review[0].RID != selftestRIDDrop {
		t.Errorf("review rows are %+v, want only %s", report.Review, selftestRIDDrop)
	}

	body, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(selftestSummaryRel)))
	if err != nil {
		t.Fatalf("read the summary back: %v", err)
	}
	written := string(body)
	if want := "- [ ] [" + selftestRIDSend + "] [MUST] A speaker MUST send the widget. (§2)\n"; !strings.Contains(written, want) {
		t.Errorf("the single-site row was not rewritten to its sentence, want %q in:\n%s", want, written)
	}
	if !strings.Contains(written, lowOverlap+"\n") {
		t.Errorf("the low-overlap row was changed:\n%s", written)
	}
}

// plantBackfillArtifact writes the fixture RFC's extraction artifact with the site mapping
// the test names, site id to requirement id. Every site the mapping leaves out is excluded,
// so the artifact says exactly what the test needs and nothing the selftest writer chose.
func plantBackfillArtifact(t *testing.T, root string, mapping map[string]string) {
	t.Helper()

	inventory, err := NewDeriver(root).Inventory(selftestStem, 1)
	if err != nil || inventory == nil {
		t.Fatalf("derive the fixture inventory: %v", err)
	}
	document := extractionSelftestArtifact(inventory)
	sites, isSites := document[keySites].([]map[string]any)
	if !isSites {
		t.Fatalf("the selftest artifact holds %T sites", document[keySites])
	}
	for i, site := range sites {
		id, _ := site["id"].(string)        //nolint:errcheck // the writer sets every id as a string
		quote, _ := site[keyQuote].(string) //nolint:errcheck // and every quote
		if rid, mapped := mapping[id]; mapped {
			sites[i] = map[string]any{"id": id, keyQuote: quote, keyDisposition: DispositionMapped, "mapped-to": rid}
			continue
		}
		sites[i] = map[string]any{"id": id, keyQuote: quote, keyDisposition: DispositionExcluded,
			"excluded-kind": "not-a-requirement", keyReason: "the test did not map this site"}
	}
	artifact, err := marshalSelftestJSON(document)
	if err != nil {
		t.Fatalf("marshal the fixture artifact: %v", err)
	}
	if err := writeSelftestFiles(root, map[string]string{checkFixtureExtractionRel: artifact}); err != nil {
		t.Fatalf("write the fixture artifact: %v", err)
	}
}

// backfillFixture builds a tree holding one summary and one RFC text, with the extraction
// artifact mapping the sites the test names.
func backfillFixture(t *testing.T, rows, source string, mapping map[string]string) string {
	t.Helper()

	summary := "# RFC 9999\n\n" + selftestMeta + "\n## Compliance Checklist\n\n" + rows
	tree := checkFixtureTree(t, map[string]string{
		"rfc/full/rfc9999.txt": source,
		selftestSummaryRel:     summary,
	})
	plantBackfillArtifact(t, tree, mapping)
	return tree
}

// backfillKinds answers each judged row's kind by id, whichever list it is on.
func backfillKinds(report *quoteBackfillReport) map[string]string {
	kinds := map[string]string{}
	for _, list := range [][]quoteBackfillRow{report.Quoted, report.Review, report.Human} {
		for _, row := range list {
			kinds[row.RID] = row.Kind
		}
	}
	return kinds
}

// VALIDATES: AC-10 through the entry point -- without apply, quote-backfill lists the row
// it would quote, with the sentence it would write, and leaves the summary byte-identical.
func TestQuoteBackfillDryRunWritesNothing(t *testing.T) {
	rows := "- [ ] [" + selftestRIDSend + "] [MUST] The speaker MUST send a widget (§2)\n"
	tree := backfillFixture(t, rows, backfillFixtureSource, map[string]string{"2:1": selftestRIDSend})
	summaryPath := filepath.Join(tree, filepath.FromSlash(selftestSummaryRel))
	before, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("read the summary: %v", err)
	}
	t.Setenv("ZE_REPO_ROOT", tree)
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	answer, code := Answer([]string{"quote-backfill", "stem", selftestStem})
	if code != 0 {
		t.Fatalf("quote-backfill answered %d", code)
	}
	report, isReport := answer.(*quoteBackfillReport)
	if !isReport {
		t.Fatalf("quote-backfill answered %T", answer)
	}
	if report.Applied {
		t.Error("a dry run reports it applied")
	}
	if len(report.Quoted) != 1 || report.Quoted[0].Quote != "A speaker MUST send the widget." {
		t.Errorf("quoted rows are %+v, want %s with its sentence", report.Quoted, selftestRIDSend)
	}
	if !strings.Contains(report.Text(), "dry run, nothing written") {
		t.Errorf("the rendering does not say nothing was written:\n%s", report.Text())
	}
	after, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("read the summary back: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Errorf("the dry run changed the summary:\n%s", after)
	}
}

// VALIDATES: AC-11 -- a row mapped by two sites, and a row mapped by a lead-in site ending
// in ":", both go to the review list with their reason, and neither line is rewritten. A
// row already verbatim is counted and left alone.
func TestQuoteBackfillSkipsLeadInAndMultiSite(t *testing.T) {
	const source = "Test RFC 9999\n\n1.  Introduction\n\n    This document describes widgets.\n\n" +
		"2.  Widgets\n\n    A speaker MUST send the widget. A speaker MUST send the gadget.\n\n" +
		"    A receiver MUST perform the following steps:\n\n    o  drop the frame.\n"
	const verbatim = "RFC9999-2-3"
	rows := "- [ ] [" + selftestRIDSend + "] [MUST] The speaker MUST send a widget (§2)\n" +
		"- [ ] [" + selftestRIDDrop + "] [MUST] The receiver MUST perform the following steps (§2)\n" +
		"- [ ] [" + verbatim + "] [MUST] A speaker MUST send the gadget. (§2) {gap: not built}\n"
	tree := backfillFixture(t, rows, source, map[string]string{
		"2:1": selftestRIDSend, "2:2": selftestRIDSend, "2:3": selftestRIDDrop,
	})

	report, err := quoteBackfill(tree, selftestStem, true)
	if err != nil {
		t.Fatalf("quote-backfill: %v", err)
	}
	kinds := backfillKinds(report)
	if kinds[selftestRIDSend] != backfillSeveralSites {
		t.Errorf("%s is %q, want %q", selftestRIDSend, kinds[selftestRIDSend], backfillSeveralSites)
	}
	if kinds[selftestRIDDrop] != backfillLeadIn {
		t.Errorf("%s is %q, want %q (report %+v)", selftestRIDDrop, kinds[selftestRIDDrop], backfillLeadIn, report)
	}
	if len(report.Quoted) != 0 || report.Applied {
		t.Errorf("rows were quoted: %+v", report.Quoted)
	}
	if report.Verbatim != 1 {
		t.Errorf("verbatim count is %d, want 1 (%s)", report.Verbatim, verbatim)
	}
	body, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(selftestSummaryRel)))
	if err != nil {
		t.Fatalf("read the summary back: %v", err)
	}
	if !strings.Contains(string(body), rows) {
		t.Errorf("the summary changed:\n%s", body)
	}
}

// VALIDATES: AC-12 on the RFC7432-10-1 shape -- a row whose paraphrase shares the
// sentence's words but states a number the sentence does not (a length of 0 where the RFC
// says 32 or 128) goes to review, and is never rewritten into a quote that hides it.
func TestQuoteBackfillReviewsInvertedNumber(t *testing.T) {
	const source = "Test RFC 9999\n\n1.  Introduction\n\n    This document describes widgets.\n\n" +
		"2.  Widgets\n\n    The Length field MUST be set to 32 for an IPv4 address or 128 for an IPv6 address.\n"
	rows := "- [ ] [" + selftestRIDSend + "] [MUST] The Length field MUST be set to 0, 32 or 128 for an IPv4 or IPv6 address (§2)\n"
	tree := backfillFixture(t, rows, source, map[string]string{"2:1": selftestRIDSend})

	report, err := quoteBackfill(tree, selftestStem, true)
	if err != nil {
		t.Fatalf("quote-backfill: %v", err)
	}
	if len(report.Review) != 1 || report.Review[0].Kind != backfillNumberAbsent ||
		!strings.Contains(report.Review[0].Reason, "number 0") {
		t.Errorf("review rows are %+v, want %s for the number 0", report.Review, backfillNumberAbsent)
	}
	if len(report.Quoted) != 0 {
		t.Errorf("the row was quoted: %+v", report.Quoted)
	}
}

// backfillWords answers n distinct content words that no stem folds together and that
// carry no digit, so an overlap count over them is exact.
func backfillWords(n int) []string {
	const letters = "bcdfghjklm"
	words := make([]string, 0, n)
	for i := range n {
		words = append(words, "word"+string(letters[i/len(letters)%len(letters)])+string(letters[i%len(letters)]))
	}
	return words
}

// VALIDATES: the A-6 overlap boundary -- a row sharing exactly half its content words with
// the sentence passes the pair tests, and one sharing 49 of 100 goes to review.
// PREVENTS: a threshold off by one in either direction.
func TestQuoteBackfillOverlapBoundary(t *testing.T) {
	words := backfillWords(100)
	row := "The speaker MUST " + strings.Join(words, " ")
	for _, tc := range []struct {
		shared int
		want   string
	}{
		{shared: 50, want: ""},
		{shared: 49, want: backfillLowOverlap},
	} {
		sentence := "A speaker MUST " + strings.Join(words[:tc.shared], " ") + "."
		source := newQuoteSource("Test RFC 9999\n\n2.  Widgets\n\n    " + sentence + "\n")
		req := &Requirement{RFC: selftestStem, RID: selftestRIDSend, Level: levelMust,
			Text: row + " (§2)", Section: "2"}
		if kind, reason := backfillPairRefusal(req, sentence, source); kind != tc.want {
			t.Errorf("%d of 100 shared: kind %q (%s), want %q", tc.shared, kind, reason, tc.want)
		}
	}
}

// VALIDATES: the other review kinds -- a [SHOULD] row mapped to a MUST sentence, a row
// whose negation the sentence lacks, and a sentence outside the cited section each go to
// review, so the citation, the level and the polarity are never rewritten by a quote.
func TestQuoteBackfillPairRefusalKinds(t *testing.T) {
	const sentence = "A speaker MUST send the widget to every peer."
	source := newQuoteSource("Test RFC 9999\n\n2.  Widgets\n\n    " + sentence + "\n\n3.  Gadgets\n\n    Nothing.\n")
	for _, tc := range []struct {
		name, level, text, section, want string
	}{
		{"level", "SHOULD", "A speaker SHOULD send the widget to every peer", "2", backfillLevelDiffers},
		{"polarity", levelMust, "A speaker MUST not send the widget to every peer", "2", backfillPolarity},
		{"section", levelMust, "A speaker MUST send the widget to every peer", "3", backfillOutsideSection},
		{"qualified", levelMust, "\"A speaker MUST send the widget to every peer.\" -- the widget half", "2", backfillQualified},
		{"quoted-term", levelMust, "A speaker MUST send the \"widget\" to every peer", "2", ""},
		{"accepted", levelMust, "A speaker MUST send widgets to all peers", "2", ""},
	} {
		req := &Requirement{RFC: selftestStem, RID: selftestRIDSend, Level: tc.level,
			Text: tc.text + " (§" + tc.section + ")", Section: tc.section}
		if kind, reason := backfillPairRefusal(req, sentence, source); kind != tc.want {
			t.Errorf("%s: kind %q (%s), want %q", tc.name, kind, reason, tc.want)
		}
	}
}

// backfillSampleRows are rows the backfill MUST send to review: first the ten the spec's
// measurement found wrongly worded, each mapped to a real sentence the paraphrase
// misstates (RFC7432-10-1 allows a length the RFC forbids, RFC4456-x-2 states a MUST NOT
// the RFC does not hold).
var backfillSampleRows = []string{
	"RFC7854-x-1", "RFC2866-5-1", "RFC2328-10.2-1", "RFC2328-10.1-1", "RFC4552-6-8",
	"RFC2328-D.3-3", "RFC4303-2.4-1", "RFC7432-10-1", "RFC5036-2.5.3-2", "RFC4456-x-2",
	// Two rows the A-6 hand-read found covering one half of a two-MUST sentence, which the
	// partial rule now sends to review.
	"RFC7911-5-1", "DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-1",
}

// backfillKnownMisses are rows the A-6 hand-read found the rule still quotes although the
// sentence changes what the row claims: a wrong value (RFC7432-6.3-3 says "normalized VID"
// where the RFC says the originating VID), a narrower scope (RFC9552-5.2-6 names modifying
// where the RFC names adding, removing or modifying), a different property (RFC5880-6.7.3-4).
// Word overlap cannot see these; a human reading the tagged tests can.
var backfillKnownMisses = []string{"RFC7432-6.3-3", "RFC9552-5.2-6", "RFC5880-6.7.3-4"}

// VALIDATES: AC-12 and A-6 over the real corpus -- a dry run over every summary judges
// every stem without error, and no sample row lands on the quote list. Run
// with -v, it logs the totals by kind, where each sample row landed, and fifteen quoted
// rows evenly spaced over the whole quote list, for a human to read against the RFC.
func TestQuoteBackfillRealCorpusReviewsSampleRows(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	stems, err := summaryStems(root)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	placed := map[string]string{}
	var quoted []quoteBackfillRow
	verbatim := 0
	for _, stem := range sortedSet(stems) {
		report, err := quoteBackfill(root, stem, false)
		if err != nil {
			t.Fatalf("%s: %v", stem, err)
		}
		verbatim += report.Verbatim
		quoted = append(quoted, report.Quoted...)
		for _, list := range [][]quoteBackfillRow{report.Quoted, report.Review, report.Human} {
			for _, row := range list {
				kinds[row.Kind]++
				placed[row.RID] = row.Kind + ": " + row.Reason
			}
		}
	}
	for _, rid := range backfillSampleRows {
		where, judged := placed[rid]
		if !judged {
			t.Errorf("%s: not judged (absent, or already verbatim)", rid)
			continue
		}
		if strings.HasPrefix(where, backfillQuote+":") {
			t.Errorf("%s would be quoted; a wrongly worded row MUST go to review", rid)
		}
		t.Logf("sample %s -> %s", rid, where)
	}
	for _, rid := range backfillKnownMisses {
		t.Logf("known miss %s -> %s", rid, placed[rid])
	}
	t.Logf("totals: verbatim %d, by kind %v", verbatim, kinds)
	for i := range 15 {
		if len(quoted) == 0 {
			break
		}
		row := quoted[i*len(quoted)/15]
		t.Logf("apply %s\n  row:   %s\n  quote: %s", row.RID, row.Text, row.Quote)
	}
}

// VALIDATES: a row stating one obligation of a sentence that states two goes to review,
// because the whole sentence would widen what the row and its tagged tests claim (R-7).
func TestQuoteBackfillReviewsPartialRow(t *testing.T) {
	const sentence = "A speaker MUST send the widget and MUST log the widget."
	source := newQuoteSource("Test RFC 9999\n\n2.  Widgets\n\n    " + sentence + "\n")
	req := &Requirement{RFC: selftestStem, RID: selftestRIDSend, Level: levelMust,
		Text: "A speaker MUST send the widget (§2)", Section: "2"}
	if kind, reason := backfillPairRefusal(req, sentence, source); kind != backfillPartial {
		t.Errorf("kind %q (%s), want %q", kind, reason, backfillPartial)
	}
}

// VALIDATES: quote-backfill writes the file its stem names, so a stem that is not a summary
// stem is refused before it becomes a path, and nothing outside rfc/short is read or written.
// METHOD: a tree holding a parseable summary one directory above rfc/short, reached by "../x".
func TestQuoteBackfillRefusesStemOutsideSummaries(t *testing.T) {
	tree := t.TempDir()
	outside := filepath.Join(tree, "rfc", "x.md")
	if err := os.MkdirAll(filepath.Dir(outside), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte(quoteFixtureSummary()), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, stem := range []string{"../x", "", "RFC7606", "a/b"} {
		if _, err := quoteBackfill(tree, stem, true); err == nil || !strings.Contains(err.Error(), "is not a summary stem") {
			t.Errorf("stem %q: err = %v, want a refusal naming the stem", stem, err)
		}
	}
}
