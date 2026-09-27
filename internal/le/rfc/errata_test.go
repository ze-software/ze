package rfc

import (
	"strings"
	"testing"
)

// erratumFixture is a verified erratum of the quote fixture RFC that corrects
// the one sentence of its section 3.
const erratumFixture = "Errata ID: 1234\nRFC: 9999\nStatus: Verified\nType: Technical\n" +
	"Location: Section 3\nDate Verified: 2026-09-27\nSource: https://www.rfc-editor.org/errata/eid1234\n\n" +
	"Original Text:\n   A receiver MUST NOT reorder the frames of one widget.\n\n" +
	"Corrected Text:\n   A receiver MUST NOT reorder or drop the frames of one widget.\n\n" +
	"   A receiver SHOULD count the frames it drops.\n"

// errataStore answers the erratum texts a test hands it, by number, the way the
// tree reader answers the files under rfc/errata/<stem>/.
func errataStore(texts map[string]string) func(string) (string, bool) {
	return func(number string) (string, bool) {
		text, held := texts[number]
		return text, held
	}
}

// judgeErratumQuote runs the row check over source with the given erratum store,
// for a row citing section 3 and whatever the cite adds to its parenthetical.
func judgeErratumQuote(t *testing.T, source, text, cite string, store map[string]string) (string, bool) {
	t.Helper()

	line := "- [ ] [RFC9999-3-1] [MUST] " + text + " (§3" + cite + ")"
	req, err := parseChecklistLine(line, selftestStem, selftestSummaryRel, 7)
	if err != nil {
		t.Fatalf("parse %q: %v", line, err)
	}
	return rowQuoteRefusal(req, newQuoteSource(source).withErrata(errataStore(store)))
}

// VALIDATES: D-11 -- a row that cites an erratum and quotes the erratum's corrected text
// passes, and the same quote with no erratum cited is refused: the corrected text is not in
// the RFC as published.
func TestCheckAcceptsQuoteFromCitedErratum(t *testing.T) {
	store := map[string]string{"1234": erratumFixture}
	for _, sentence := range []string{
		"A receiver MUST NOT reorder or drop the frames of one widget.",
		"A receiver SHOULD count the frames it drops.",
	} {
		if message, refused := judgeErratumQuote(t, quoteFixtureSource, sentence, ", erratum 1234", store); refused {
			t.Errorf("a quote of the cited erratum's corrected text was refused: %s", message)
		}
		if _, refused := judgeErratumQuote(t, quoteFixtureSource, sentence, "", store); !refused {
			t.Errorf("the corrected text %q passed with no erratum cited", sentence)
		}
	}
}

// VALIDATES: D-11 -- a row citing an erratum the repository does not hold is refused, and the
// refusal names the file to store and where the verified text is published.
func TestCheckRefusesRowCitingMissingErratum(t *testing.T) {
	message, refused := judgeErratumQuote(t, quoteFixtureSource,
		"A receiver MUST NOT reorder or drop the frames of one widget.", ", erratum 1234", map[string]string{})
	if !refused {
		t.Fatal("a row citing an erratum nobody stored passed")
	}
	for _, want := range []string{"rfc/errata/" + selftestStem + "/1234.txt", "https://www.rfc-editor.org/errata/eid1234"} {
		if !strings.Contains(message, want) {
			t.Errorf("the refusal omits %q: %s", want, message)
		}
	}
}

// VALIDATES: D-11 -- only a verified erratum is evidence, and a stored file that names another
// erratum or carries no original or corrected text is refused, naming the file.
func TestCheckRefusesUnverifiedOrMalformedErratum(t *testing.T) {
	withoutCorrected, _, _ := strings.Cut(erratumFixture, "Corrected Text:")
	for name, text := range map[string]string{
		"reported":     strings.Replace(erratumFixture, "Status: Verified", "Status: Reported", 1),
		"other id":     strings.Replace(erratumFixture, "Errata ID: 1234", "Errata ID: 1235", 1),
		"no corrected": withoutCorrected,
		"no original":  strings.Replace(erratumFixture, "Original Text:", "Before:", 1),
	} {
		message, refused := judgeErratumQuote(t, quoteFixtureSource,
			"A receiver MUST NOT reorder or drop the frames of one widget.", ", erratum 1234", map[string]string{"1234": text})
		if !refused {
			t.Errorf("%s: the erratum was accepted as evidence", name)
			continue
		}
		if !strings.Contains(message, "rfc/errata/"+selftestStem+"/1234.txt") {
			t.Errorf("%s: the refusal does not name the file: %s", name, message)
		}
	}
}

// VALIDATES: D-11 -- a row citing an erratum is judged against the RFC as that erratum
// corrects it, so the published sentence the erratum replaced is refused, naming the erratum,
// while a sentence of the cited section the erratum left alone still passes.
func TestCheckRefusesPublishedTextErratumReplaced(t *testing.T) {
	store := map[string]string{"1234": erratumFixture}
	message, refused := judgeErratumQuote(t, quoteFixtureSource,
		"A receiver MUST NOT reorder the frames of one widget.", ", erratum 1234", store)
	if !refused {
		t.Fatal("the published sentence the cited erratum replaced passed")
	}
	if !strings.Contains(message, "erratum 1234 replaced") {
		t.Errorf("the refusal does not name the erratum that replaced the sentence: %s", message)
	}
	extended := strings.Replace(quoteFixtureSource, "one widget.\n",
		"one widget.\n\n   A sender MUST pace the frames of one widget.\n", 1)
	if message, refused := judgeErratumQuote(t, extended,
		"A sender MUST pace the frames of one widget.", ", erratum 1234", store); refused {
		t.Errorf("a sentence the erratum left alone was refused: %s", message)
	}
}

// VALIDATES: D-11 -- when the erratum's original text is not verbatim in the section it names,
// the check cannot tell which published sentence it replaced, so the published text is refused
// and only the corrected text passes. A silent pass would accept superseded text.
func TestCheckRefusesPublishedTextWhenErratumUnplaced(t *testing.T) {
	unplaced := strings.Replace(erratumFixture, "Location: Section 3", "Location: Section 2", 1)
	store := map[string]string{"1234": unplaced}
	message, refused := judgeErratumQuote(t, quoteFixtureSource,
		"A receiver MUST NOT reorder the frames of one widget.", ", erratum 1234", store)
	if !refused {
		t.Fatal("published text passed under an erratum the check could not place")
	}
	if !strings.Contains(message, "not verbatim in section 2") {
		t.Errorf("the refusal does not say the erratum could not be placed: %s", message)
	}
	if message, refused := judgeErratumQuote(t, quoteFixtureSource,
		"A receiver SHOULD count the frames it drops.", ", erratum 1234", store); refused {
		t.Errorf("the corrected text of an unplaced erratum was refused: %s", message)
	}
}

// VALIDATES: the erratum citation forms the corpus writes, read from the trailing
// parenthetical only.
func TestCitedErrataReadsTrailingParenthetical(t *testing.T) {
	for text, want := range map[string]string{
		"A row (§7.1, erratum 8301)":             "8301",
		"A row (§5.2.9; erratum 8300)":           "8300",
		"A row (§5.4, errata 543)":               "543",
		"A row (§6, erratum 7 and erratum 9)":    "7 9",
		"A row about erratum 12 prose (§6)":      "",
		"A row (§6, Errata ID 7840)":             "7840",
		"A row (§6, erratum 8301, erratum 8301)": "8301",
	} {
		if got := strings.Join(citedErrata(text), " "); got != want {
			t.Errorf("citedErrata(%q) = %q, want %q", text, got, want)
		}
	}
}

// VALIDATES: D-11 through the entry point -- a tip commit adding a row that cites an erratum
// is refused while the erratum's file is absent, naming the file, and is green once the
// commit also carries the verified text. A later commit that edits only the erratum file so
// the row stops being verbatim has that row refused, so the erratum store is part of what
// every row is judged on.
// METHOD: fixture repositories checked by Check, the `./le rfc check` entry point.
func TestCheckJudgesErratumRowThroughCommits(t *testing.T) {
	const row = "- [ ] [RFC9999-2-3] [SHOULD] A receiver SHOULD count the frames it drops. (§2, erratum 1234)"
	erratumRel := stemPath(errataRel+"/"+selftestStem, "1234", ".txt")
	placed := strings.Replace(strings.Replace(erratumFixture, "Location: Section 3", "Location: Section 2", 1),
		"   A receiver MUST NOT reorder the frames of one widget.", "   A receiver MUST NOT drop the widget.", 1)
	withSource := func() map[string]string {
		base := fixtureCorpus()
		base[selftestSourceRel] = selftestRFCSource
		return base
	}

	missing := commitFixtureTip(t, withSource(), map[string]string{selftestSummaryRel: quoteFixtureSummary(row)}, nil)
	report, code := Check(missing)
	if code != 2 || !strings.Contains(strings.Join(report.Violations, "\n"), erratumRel) {
		t.Fatalf("a row citing an unstored erratum answered %d without naming %s:\n%s", code, erratumRel, report.Text())
	}

	stored := commitFixtureTip(t, withSource(), map[string]string{selftestSummaryRel: quoteFixtureSummary(row),
		erratumRel: placed}, nil)
	if report, code := Check(stored); code != 0 {
		t.Fatalf("a row quoting its stored erratum answered %d:\n%s", code, report.Text())
	}

	base := withSource()
	base[selftestSummaryRel] = quoteFixtureSummary(row)
	base[erratumRel] = placed
	edited := commitFixtureTip(t, base, map[string]string{erratumRel: strings.Replace(placed, "count the frames", "log the frames", 1)}, nil)
	report, code = Check(edited)
	if code != 2 || len(report.Violations) != 1 || !strings.Contains(report.Violations[0], "RFC9999-2-3") {
		t.Fatalf("an erratum edit that unquoted a row answered %d without refusing the row:\n%s", code, report.Text())
	}
}
