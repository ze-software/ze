package rfc

import (
	"strings"
	"testing"
)

// quoteFixtureSource is an RFC text with a sentence crossing a page break in
// section 2, a subsection 2.1, and a section 3 that carries its own sentence.
const quoteFixtureSource = "Test RFC 9999\n\n1.  Introduction\n\n" +
	"   This document describes widgets and the frames that carry them.\n\n" +
	"2.  Widgets\n\n" +
	"   A speaker MUST send the widget before it sends\n\n\n\n" +
	"Author                       Standards Track                   [Page 3]\n" +
	"\f\n" +
	"RFC 9999                       Widgets                     September 2026\n\n\n" +
	"   any frame that depends on it.\n\n" +
	"2.1.  Widget Length\n\n" +
	"   The Length field (in octets) MUST be set to 32 for a widget.\n\n" +
	"3.  Frames\n\n" +
	"   A receiver MUST NOT reorder the frames of one widget.\n"

// quoteRow builds the requirement a summary line would parse to, citing section.
func quoteRow(t *testing.T, text, section string) *Requirement {
	t.Helper()

	line := "- [ ] [RFC9999-" + section + "-1] [MUST] " + text + " (§" + section + ")"
	if section == "" {
		line = "- [ ] [RFC9999-x-1] [MUST] " + text
	}
	req, err := parseChecklistLine(line, selftestStem, selftestSummaryRel, 7)
	if err != nil {
		t.Fatalf("parse %q: %v", line, err)
	}
	return req
}

// judgeQuote runs the row check's own judge over quoteFixtureSource.
func judgeQuote(t *testing.T, text, section string) (string, bool) {
	t.Helper()

	return rowQuoteRefusal(quoteRow(t, text, section), newQuoteSource(quoteFixtureSource))
}

// VALIDATES: AC-2 -- a row whose text is a verbatim span of its cited section passes even
// when the sentence crosses a page break.
// METHOD: the same sentence is first shown absent from the raw collapse (so the test can
// only pass through the page strip), then judged by the row check.
func TestCheckAcceptsQuoteAcrossPageBreak(t *testing.T) {
	const sentence = "A speaker MUST send the widget before it sends any frame that depends on it."
	if strings.Contains(squashWhitespace(quoteFixtureSource), sentence) {
		t.Fatal("the fixture sentence is in the raw collapse, so this test cannot prove the page strip")
	}
	if message, refused := judgeQuote(t, sentence, "2"); refused {
		t.Fatalf("a quote across a page break was refused: %s", message)
	}
}

// VALIDATES: AC-2 -- a row citing a section passes when the sentence is in a subsection.
func TestCheckAcceptsQuoteInSubsection(t *testing.T) {
	if message, refused := judgeQuote(t, "The Length field (in octets) MUST be set to 32", "2"); refused {
		t.Fatalf("a quote in subsection 2.1 of the cited section 2 was refused: %s", message)
	}
}

// VALIDATES: AC-3 -- a verbatim sentence cited under the wrong section is refused, and the
// refusal names the section the sentence is really in.
func TestCheckRefusesQuoteFoundInOtherSection(t *testing.T) {
	message, refused := judgeQuote(t, "A receiver MUST NOT reorder the frames of one widget.", "2")
	if !refused {
		t.Fatal("a sentence of section 3 cited under section 2 passed")
	}
	if !strings.Contains(message, "in section 3, not in section 2") {
		t.Errorf("the refusal does not name where the sentence is: %s", message)
	}
}

// VALIDATES: AC-4 -- a cited id that is not a heading ("3.b") resolves to its nearest
// heading ancestor ("3") and is matched there; an id under a subsection resolves to it.
func TestQuoteSectionResolvesToHeadingAncestor(t *testing.T) {
	source := newQuoteSource(quoteFixtureSource)
	for cited, want := range map[string]string{"3.b": "3", "2.1.4": "2.1", "2": "2"} {
		got, ok := source.resolve(cited)
		if !ok || got != want {
			t.Errorf("resolve(%q) = (%q, %v), want (%q, true)", cited, got, ok, want)
		}
	}
	if message, refused := judgeQuote(t, "A receiver MUST NOT reorder the frames of one widget.", "3.b"); refused {
		t.Fatalf("a quote cited under 3.b was refused: %s", message)
	}
}

// VALIDATES: AC-5 -- a cited section with no heading and no heading ancestor is refused as an
// unresolved anchor, and so is a row citing no section. Never a whole-document fallback: the
// quote below IS in the RFC, so a fallback would pass it.
func TestCheckRefusesUnresolvedSectionAnchor(t *testing.T) {
	const sentence = "A receiver MUST NOT reorder the frames of one widget."
	for _, section := range []string{"7", "7.3", ""} {
		message, refused := judgeQuote(t, sentence, section)
		if !refused {
			t.Errorf("section %q passed with no heading to resolve to", section)
			continue
		}
		if !strings.Contains(message, "unresolved anchor") {
			t.Errorf("section %q refused for the wrong reason: %s", section, message)
		}
	}
}

// VALIDATES: AC-6 and the quote-length boundary -- 23 characters is refused, 24 passes.
func TestCheckRefusesQuoteUnderMinimum(t *testing.T) {
	const span = "A receiver MUST NOT reorder" // 27 bytes, a verbatim span of section 3
	for length, wantRefused := range map[int]bool{minRowQuote - 1: true, minRowQuote: false} {
		quote := span[len(span)-length:]
		message, refused := judgeQuote(t, quote, "3")
		if refused != wantRefused {
			t.Errorf("a %d-character quote %q: refused=%v, want %v (%s)", length, quote, refused, wantRefused, message)
		}
	}
}

// VALIDATES: R-4 -- a quote carrying parentheses and a brace keeps the trailing section and
// the trailing marker, and the quote is everything before the section parenthetical.
func TestQuoteWithParenthesesKeepsTrailingSection(t *testing.T) {
	line := "- [ ] [RFC9999-2.1-1] [MUST] The Length field (in octets) MUST be {32} (§2.1) {gap: not built}"
	req, err := parseChecklistLine(line, selftestStem, selftestSummaryRel, 1)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if req.Section != "2.1" {
		t.Errorf("section is %q, want 2.1", req.Section)
	}
	if req.Annotation == nil || req.Annotation.Kind != "gap" {
		t.Errorf("the trailing {gap} marker was not peeled: %+v", req.Annotation)
	}
	if got, want := req.Quote(), "The Length field (in octets) MUST be {32}"; got != want {
		t.Errorf("quote is %q, want %q", got, want)
	}
}

// quoteFixtureSummary is the fixture summary with extra rows appended to its checklist.
func quoteFixtureSummary(rows ...string) string {
	summary := "# RFC 9999\n\n" + selftestMeta + "\n## Compliance Checklist\n\n" +
		"- [ ] [" + selftestRIDSend + "] [MUST] A speaker MUST send the widget (§2)\n"
	for _, row := range rows {
		summary += row + "\n"
	}
	return summary
}

// VALIDATES: AC-1 through the entry point -- a row the tip commit adds against HEAD^ whose
// text is not verbatim in its cited section reds `./le rfc check`, naming the stem, the id,
// the section and the text; the same commit adding a verbatim row stays green.
// METHOD: two fixture repositories whose tip commit adds one SHOULD row (SHOULD, so no
// coverage or extraction rule has anything to say about it), checked out detached as
// `./le verify worktree` does. The paraphrase answers exactly one violation: the row
// refusal. No second rule counts the same row.
func TestCheckRefusesNewRowNotVerbatimInSection(t *testing.T) {
	quoted := commitFixtureTip(t, fixtureCorpus(), map[string]string{selftestSummaryRel: quoteFixtureSummary(
		"- [ ] [RFC9999-2-3] [SHOULD] A speaker MUST send the widget. (§2)")}, nil)
	gitFixture(t, quoted, []string{"checkout", "-q", "--detach"})
	if report, code := Check(quoted, nil); code != 0 {
		t.Fatalf("a verbatim row answered %d:\n%s", code, report.Text())
	}

	const paraphrase = "Speakers ought to transmit widgets promptly"
	added := commitFixtureTip(t, fixtureCorpus(), map[string]string{selftestSummaryRel: quoteFixtureSummary(
		"- [ ] [RFC9999-2-3] [SHOULD] " + paraphrase + " (§2)")}, nil)
	gitFixture(t, added, []string{"checkout", "-q", "--detach"})
	report, code := Check(added, nil)
	if code != 2 || len(report.Violations) != 1 {
		t.Fatalf("a paraphrased row the tip added answered %d with %d violation(s), want the row:\n%s",
			code, len(report.Violations), report.Text())
	}
	row := ""
	for _, violation := range report.Violations {
		if strings.Contains(violation, "not a verbatim span of section") {
			row = violation
		}
	}
	for _, want := range []string{selftestStem, "RFC9999-2-3", "section 2", paraphrase} {
		if !strings.Contains(row, want) {
			t.Errorf("the row refusal omits %q: %q\n%s", want, row, report.Text())
		}
	}
}

// VALIDATES: AC-2 -- the row quote rule judges every row in the corpus: a row that is not
// verbatim in its cited section, and that the commit under test did not touch, is refused,
// naming the stem, the id, the section and the text, with the rule's reason.
// METHOD: the unquoted row is in the BASE commit, identical at HEAD^ and HEAD; the tip
// commit changes an unrelated file. Restoring a scope over the commit's added or edited
// rows turns this red.
func TestCheckRefusesUnchangedUnquotedRow(t *testing.T) {
	const paraphrase = "Speakers ought to transmit widgets promptly"
	base := fixtureCorpus()
	base[selftestSourceRel] = selftestRFCSource
	base[selftestSummaryRel] = quoteFixtureSummary("- [ ] [RFC9999-2-3] [SHOULD] " + paraphrase + " (§2)")
	root := commitFixtureTip(t, base, fixtureCorpusNudge(), nil)
	report, code := Check(root, nil)
	if code != 2 || len(report.Violations) != 1 {
		t.Fatalf("an unquoted row the tip did not touch answered %d with %d violation(s), want the row:\n%s",
			code, len(report.Violations), report.Text())
	}
	for _, want := range []string{selftestStem, "RFC9999-2-3", "section 2", paraphrase, "not a verbatim span of section 2"} {
		if !strings.Contains(report.Violations[0], want) {
			t.Errorf("the row refusal omits %q: %q", want, report.Violations[0])
		}
	}
}

// VALIDATES: principles (no silent zero) -- a stem with rows and no RFC text is refused row
// by row, each refusal naming the row and where to fetch the text, while a row of a stem
// whose text is present and verbatim is accepted. It is never left unjudged.
// METHOD: checkRowQuotes over the fixture tree, which holds rfc/full/rfc9999.txt and no
// text for rfc7777, with one row of each stem.
func TestCheckNamesStemWithoutRFCTextRefused(t *testing.T) {
	root := checkFixtureTree(t, nil)
	refusals := checkRowQuotes(root, []Requirement{*quoteRow(t, "A speaker MUST send the widget", "2"),
		{RFC: "rfc7777", RID: "RFC7777-2-1", Text: "A speaker MUST send the widget (§2)", Section: "2"},
		{RFC: "rfc7777", RID: "RFC7777-2-2", Text: "A receiver MUST NOT drop the widget (§2)", Section: "2"}})
	if len(refusals) != 2 {
		t.Fatalf("answered %d refusal(s), want one for each rfc7777 row: %q", len(refusals), refusals)
	}
	for i, rid := range []string{"RFC7777-2-1", "RFC7777-2-2"} {
		for _, want := range []string{rid, "the RFC's own text is not in this repository", "rfc/full/rfc7777.txt"} {
			if !strings.Contains(refusals[i], want) {
				t.Errorf("refusal %d omits %q: %q", i, want, refusals[i])
			}
		}
	}
}

// VALIDATES: the functional case of the row quote rule -- an author commits a MUST the RFC
// does not contain and the public check refuses it, while the same commit carrying the
// RFC's own sentence is accepted. Runs the selftest stage `./le rfc selftest` runs.
func TestRFCSelftestQuoteStageRefusesFabricatedRow(t *testing.T) {
	rows, err := runQuoteSelftest()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("the quote stage answered %d row(s), want 2", len(rows))
	}
	for _, row := range rows {
		if !row.Passed {
			t.Errorf("%s failed: %s", row.Case, row.Detail)
		}
	}
}

// VALIDATES: a tip commit that changes only the RFC text, so a row it left alone stops
// being verbatim, is refused, and the refusal names that row: the rule judges every row
// against the text in the tree, not only the rows the commit edited.
// METHOD: the base commit holds the two-sentence selftest RFC text and a verbatim SHOULD
// row quoting its second sentence; the tip rewrites the sentence it quotes
// in rfc/full/rfc9999.txt and touches no summary.
func TestCheckRefusesRowAnRFCTextChangeUnquoted(t *testing.T) {
	base := fixtureCorpus()
	base[selftestSourceRel] = selftestRFCSource
	base[selftestSummaryRel] = quoteFixtureSummary("- [ ] [RFC9999-2-3] [SHOULD] A receiver MUST NOT drop the widget. (§2)")
	changed := strings.Replace(selftestRFCSource, "MUST NOT drop the widget", "MUST NOT discard the widget", 1)
	if changed == selftestRFCSource {
		t.Fatal("the fixture RFC text no longer carries the sentence this test rewrites")
	}
	root := commitFixtureTip(t, base, map[string]string{selftestSourceRel: changed}, nil)
	report, code := Check(root, nil)
	if code != 2 {
		t.Fatalf("an RFC text change that unquoted a row answered %d:\n%s", code, report.Text())
	}
	// The extraction rules also refuse the changed text; the row refusal is the one this
	// test is about, and it must be there exactly once.
	rows := 0
	for _, violation := range report.Violations {
		if strings.Contains(violation, "RFC9999-2-3") && strings.Contains(violation, "not a verbatim span of section 2") {
			rows++
		}
	}
	if rows != 1 {
		t.Errorf("%d violation(s) refuse RFC9999-2-3 as not verbatim, want 1:\n%s", rows, report.Text())
	}
}

// VALIDATES: A-3 -- the match is case-sensitive, so a row that lowers the RFC's "MUST" to
// "must" is not the RFC's sentence and is refused, while the same row in the RFC's case passes.
// METHOD: one row judged in both cases against the selftest RFC text.
func TestCheckQuoteMatchIsCaseSensitive(t *testing.T) {
	source := newQuoteSource(selftestRFCSource)
	for text, wantRefused := range map[string]bool{
		"A receiver MUST NOT drop the widget.": false,
		"A receiver must not drop the widget.": true,
	} {
		req, err := parseChecklistLine("- [ ] [RFC9999-2-3] [MUST NOT] "+text+" (§2)", selftestStem, selftestSummaryRel, 1)
		if err != nil || req == nil {
			t.Fatalf("parse %q: %v", text, err)
		}
		if message, refused := rowQuoteRefusal(req, source); refused != wantRefused {
			t.Errorf("%q: refused=%v, want %v (%s)", text, refused, wantRefused, message)
		}
	}
}

// unnumberedFixtureSource is an RFC text with no numbered or lettered heading at all, at
// column 0 or at its margin: the shape of RFC 792, RFC 1997 and the TFTP options.
const unnumberedFixtureSource = "Test RFC 9999\n\nWidgets\n\n" +
	"   A speaker MUST send the widget. A receiver MUST NOT drop the widget.\n"

// wholeTextRows are the rows both D-1 tests add: one citing an unnumbered title, and one
// citing no section at all. Both quote a sentence of the text verbatim.
var wholeTextRows = []string{
	"- [ ] [RFC9999-Widgets-1] [SHOULD] A receiver MUST NOT drop the widget. (§Widgets)",
	"- [ ] [RFC9999-x-1] [SHOULD] A receiver MUST NOT drop the widget.",
}

// wholeTextFixtureBase is the base corpus over source, with an extraction sign-off that maps
// the first site of the front matter to the fixture's one MUST row. The shared fixture maps
// site "2:1", which a text without headings does not have, so without this the sign-off
// would be the only violation either D-1 test answers.
func wholeTextFixtureBase(t *testing.T, source string) map[string]string {
	t.Helper()

	scratch := checkFixtureTree(t, map[string]string{selftestSourceRel: source})
	inventory, err := NewDeriver(scratch).Inventory(selftestStem, 1)
	if err != nil || inventory == nil {
		t.Fatalf("derive the fixture inventory: %v", err)
	}
	document := extractionSelftestArtifact(inventory)
	sites, isSites := document[keySites].([]map[string]any)
	if !isSites {
		t.Fatalf("the selftest artifact holds %T sites", document[keySites])
	}
	for i, site := range sites {
		if site["id"] == frontSection+":1" {
			sites[i] = map[string]any{"id": site["id"], keyQuote: site[keyQuote],
				keyDisposition: DispositionMapped, "mapped-to": selftestRIDSend}
		}
	}
	artifact, err := marshalSelftestJSON(document)
	if err != nil {
		t.Fatalf("marshal the fixture artifact: %v", err)
	}
	base := fixtureCorpus()
	base[selftestSourceRel] = source
	base[checkFixtureExtractionRel] = artifact
	return base
}

// VALIDATES: AC-4 accept, owner decision D-1 -- in a stem whose RFC text has no numbered
// heading, the whole text is one citable section, so a verbatim row passes whatever section
// it cites, an unnumbered title or none.
// METHOD: the base commit holds the unnumbered RFC text; the tip adds the two rows, and the
// tree is checked out detached as `./le verify worktree` does.
func TestCheckAcceptsWholeTextQuoteInUnnumberedStem(t *testing.T) {
	base := wholeTextFixtureBase(t, unnumberedFixtureSource)
	root := commitFixtureTip(t, base, map[string]string{selftestSummaryRel: quoteFixtureSummary(wholeTextRows...)}, nil)
	gitFixture(t, root, []string{"checkout", "-q", "--detach"})
	if report, code := Check(root, nil); code != 0 {
		t.Fatalf("verbatim rows in a stem with no numbered heading answered %d:\n%s", code, report.Text())
	}
}

// VALIDATES: AC-4 refuse, owner decision D-1 -- a stem that has a numbered heading keeps
// refusing a citation that names none of its headings, even when the quote is verbatim in
// the front matter before the first heading.
// METHOD: the same rows as the accept test, over the same sentences followed by one numbered
// heading, so the only difference between the two trees is that heading.
func TestCheckRefusesFrontCitationInNumberedStem(t *testing.T) {
	base := wholeTextFixtureBase(t, unnumberedFixtureSource+"\n1.  Introduction\n\n   This document describes widgets.\n")
	root := commitFixtureTip(t, base, map[string]string{selftestSummaryRel: quoteFixtureSummary(wholeTextRows...)}, nil)
	gitFixture(t, root, []string{"checkout", "-q", "--detach"})
	report, code := Check(root, nil)
	if code != 2 {
		t.Fatalf("front-matter citations in a numbered stem answered %d:\n%s", code, report.Text())
	}
	for _, rid := range []string{"RFC9999-Widgets-1", "RFC9999-x-1"} {
		refused := false
		for _, violation := range report.Violations {
			if strings.Contains(violation, rid) && strings.Contains(violation, "unresolved anchor") {
				refused = true
			}
		}
		if !refused {
			t.Errorf("no unresolved-anchor refusal names %s:\n%s", rid, report.Text())
		}
	}
}

// indentedFixtureSource is the selftest RFC laid out as RFC 905 is: no heading at column 0,
// every heading indented to the body margin.
const indentedFixtureSource = "Test RFC 9999\n\n" +
	"     1  Introduction\n\n" +
	"     This document describes widgets.\n\n" +
	"     2  Widgets\n\n" +
	"     A speaker MUST send the widget. A receiver MUST NOT drop the widget.\n"

// VALIDATES: owner decision D-5 -- a stem whose headings are all indented has real sections,
// so D-1's whole-text reading does not apply to it: a verbatim row citing the section it is
// in passes, and a row citing a section the text does not have is refused as an unresolved
// anchor, whereas a whole-text stem would have accepted it.
// METHOD: the base commit holds the indented RFC text; the tip adds one row citing §2 and
// one citing §7, both quoting a sentence of section 2 verbatim, checked out detached.
func TestCheckRefusesMissingSectionInIndentedStem(t *testing.T) {
	base := wholeTextFixtureBase(t, indentedFixtureSource)
	root := commitFixtureTip(t, base, map[string]string{selftestSummaryRel: quoteFixtureSummary(
		"- [ ] [RFC9999-2-3] [SHOULD] A receiver MUST NOT drop the widget. (§2)",
		"- [ ] [RFC9999-7-1] [SHOULD] A receiver MUST NOT drop the widget. (§7)")}, nil)
	gitFixture(t, root, []string{"checkout", "-q", "--detach"})
	report, code := Check(root, nil)
	if code != 2 {
		t.Fatalf("a citation of a section the indented text lacks answered %d:\n%s", code, report.Text())
	}
	refused := false
	for _, violation := range report.Violations {
		if strings.Contains(violation, "RFC9999-2-3") {
			t.Errorf("the row citing the section it is in was refused: %s", violation)
		}
		if strings.Contains(violation, "RFC9999-7-1") && strings.Contains(violation, "unresolved anchor") {
			refused = true
		}
	}
	if !refused {
		t.Errorf("no unresolved-anchor refusal names RFC9999-7-1:\n%s", report.Text())
	}
}
