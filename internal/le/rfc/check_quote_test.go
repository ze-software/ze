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
// `./le verify worktree` does. The paraphrase answers exactly two violations: the row
// refusal, and the AC-8 ratchet over the stem's unquoted count, which the same row raised.
func TestCheckRefusesNewRowNotVerbatimInSection(t *testing.T) {
	quoted := commitFixtureTip(t, fixtureCorpus(), map[string]string{selftestSummaryRel: quoteFixtureSummary(
		"- [ ] [RFC9999-2-3] [SHOULD] A speaker MUST send the widget. (§2)")}, nil)
	gitFixture(t, quoted, []string{"checkout", "-q", "--detach"})
	if report, code := Check(quoted); code != 0 {
		t.Fatalf("a verbatim row answered %d:\n%s", code, report.Text())
	}

	const paraphrase = "Speakers ought to transmit widgets promptly"
	added := commitFixtureTip(t, fixtureCorpus(), map[string]string{selftestSummaryRel: quoteFixtureSummary(
		"- [ ] [RFC9999-2-3] [SHOULD] " + paraphrase + " (§2)")}, nil)
	gitFixture(t, added, []string{"checkout", "-q", "--detach"})
	report, code := Check(added)
	if code != 2 || len(report.Violations) != 2 {
		t.Fatalf("a paraphrased row the tip added answered %d with %d violation(s), want the row and the ratchet:\n%s",
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

// VALIDATES: AC-7 -- an unquoted row the commit under test did not touch is not refused,
// and it is counted in the stem's unquoted figure and the total `./le rfc check` prints.
// METHOD: the unquoted row is in the BASE commit; the tip commit changes an unrelated file.
func TestCheckCountsUnchangedUnquotedRow(t *testing.T) {
	base := fixtureCorpus()
	base[selftestSourceRel] = selftestRFCSource
	base[selftestSummaryRel] = quoteFixtureSummary(
		"- [ ] [RFC9999-2-3] [SHOULD] Speakers ought to transmit widgets promptly (§2)")
	root := commitFixtureTip(t, base, fixtureCorpusNudge(), nil)
	report, code := Check(root)
	if code != 0 {
		t.Fatalf("an unquoted row the tip did not touch answered %d:\n%s", code, report.Text())
	}
	if report.Unquoted[selftestStem] != 1 || report.UnquotedTotal != 1 {
		t.Fatalf("unquoted figures %v total %d, want %s 1 and total 1", report.Unquoted, report.UnquotedTotal, selftestStem)
	}
	text := report.Text()
	for _, want := range []string{"unquoted: 1 row(s) across 1 stem(s)", selftestStem + " 1"} {
		if !strings.Contains(text, want) {
			t.Errorf("the report omits %q:\n%s", want, text)
		}
	}
}

// VALIDATES: principles (no silent zero) -- a stem with rows and no RFC text is named as
// unjudged and counted in no figure, never reported as zero unquoted.
func TestCheckNamesStemWithoutRFCTextUnjudged(t *testing.T) {
	root := commitFixtureTip(t, fixtureCorpus(), fixtureCorpusNudge(), nil)
	figures, unjudged := unquotedFigures(root, []Requirement{*quoteRow(t, "A speaker MUST send the widget", "2"),
		{RFC: "rfc7777", RID: "RFC7777-2-1", Text: "A speaker MUST send the widget (§2)", Section: "2"}})
	if _, counted := figures["rfc7777"]; counted {
		t.Errorf("a stem with no RFC text was counted: %v", figures)
	}
	if len(unjudged) != 1 || !strings.Contains(unjudged[0], "rfc7777") {
		t.Errorf("the stem with no RFC text is not named unjudged: %v", unjudged)
	}
}

// VALIDATES: AC-8 through the entry point -- a tip commit that raises a stem's unquoted
// count over HEAD^ is refused, naming the stem and both counts.
func TestCheckRefusesUnquotedCountRise(t *testing.T) {
	root := commitFixtureTip(t, fixtureCorpus(), map[string]string{selftestSummaryRel: quoteFixtureSummary(
		"- [ ] [RFC9999-2-3] [SHOULD] Speakers ought to transmit widgets promptly (§2)")}, nil)
	report, code := Check(root)
	if code != 2 {
		t.Fatalf("an unquoted count rise answered %d:\n%s", code, report.Text())
	}
	for _, violation := range report.Violations {
		if strings.Contains(violation, "unquoted") && strings.Contains(violation, selftestStem) &&
			strings.Contains(violation, "0 -> 1") {
			return
		}
	}
	t.Errorf("no violation names %s's unquoted count moving 0 -> 1:\n%s", selftestStem, report.Text())
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

// VALIDATES: AC-8, the case the row check cannot see -- a tip commit that changes only the
// RFC text, so a row it left alone stops being verbatim, raises the stem's unquoted count and
// is refused by the ratchet, while the row check, which judges only rows the commit edited,
// says nothing about that row.
// METHOD: the base commit holds the two-sentence selftest RFC text and a verbatim SHOULD
// row quoting its second sentence; the tip rewrites the sentence it quotes
// in rfc/full/rfc9999.txt and touches no summary. Deleting the RFC-text paths from
// quoteChangedStems, or the ratchet itself, turns this red.
func TestCheckRefusesUnquotedCountRiseFromRFCTextChange(t *testing.T) {
	base := fixtureCorpus()
	base[selftestSourceRel] = selftestRFCSource
	base[selftestSummaryRel] = quoteFixtureSummary("- [ ] [RFC9999-2-3] [SHOULD] A receiver MUST NOT drop the widget. (§2)")
	changed := strings.Replace(selftestRFCSource, "MUST NOT drop the widget", "MUST NOT discard the widget", 1)
	if changed == selftestRFCSource {
		t.Fatal("the fixture RFC text no longer carries the sentence this test rewrites")
	}
	root := commitFixtureTip(t, base, map[string]string{selftestSourceRel: changed}, nil)
	report, code := Check(root)
	if code != 2 {
		t.Fatalf("an RFC text change that unquoted a row answered %d:\n%s", code, report.Text())
	}
	ratchet := false
	for _, violation := range report.Violations {
		if strings.Contains(violation, "RFC9999-2-3") {
			t.Errorf("the row check judged a row the tip commit did not edit: %s", violation)
		}
		if strings.Contains(violation, "unquoted rows 0 -> 1") && strings.Contains(violation, selftestStem) {
			ratchet = true
		}
	}
	if !ratchet {
		t.Errorf("no violation names %s's unquoted count moving 0 -> 1:\n%s", selftestStem, report.Text())
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
