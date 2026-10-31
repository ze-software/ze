// VALIDATES: a {feature-declined} heading form is tied to an owner decision
// through the summary's `## Meta` row `Declined features`, each entry
// `<feature words> (<owner-decision source>)`: the heading's words equal or
// hold an entry's words as whole words, a two-letter feature word passes only
// as an entry's whole words, and every entry's words are in the RFC text.
// PREVENTS: a heading form that declines a feature nobody decided to decline,
// and a declared entry that names nothing the RFC says.
// METHOD: ParseSummaryText over the fixture summary with its Meta table
// varied, and checkDeclinedFeaturesInText over the real text of RFC 8907.

package rfc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureWithDeclined is the fixture summary holding rows, with a `Declined
// features` row of value after its `Ze roles` row, or none when value is
// empty.
func fixtureWithDeclined(t *testing.T, value string, rows ...string) string {
	t.Helper()
	summary := quoteFixtureSummary(rows...)
	if !strings.Contains(summary, selftestZeRolesRow) {
		t.Fatal("the fixture summary holds no Ze roles row to follow")
	}
	if value == "" {
		return summary
	}
	return strings.Replace(summary, selftestZeRolesRow, selftestZeRolesRow+"| Declined features | "+value+" |\n", 1)
}

// headingFixtureRow is one heading-form checklist row of the fixture summary.
func headingFixtureRow(words string) string {
	return `- [ ] [RFC9999-2-3] [MUST] A speaker MUST send the widget. (§2) {feature-declined: heading "` + words +
		`"; ze never uses ` + words + `: internal/x/frame.go::sendFrame sends PAP only}`
}

func TestTheDeclinedFeaturesRowTiesAHeadingToAnOwnerDecision(t *testing.T) {
	const chap = "CHAP (owner 2026-09-21 tacacs_methods)"
	cases := []struct{ name, declared, words, refused string }{
		{"the entry itself", chap, "CHAP", ""},
		{"a heading holding the entry", chap, "CHAP Login", ""},
		{"one entry of several", "PAP (owner a), CHAP (owner 2026-09-21 tacacs_methods, PAP kept)", "CHAP", ""},
		{"an owner-named two-letter feature", "PE (owner 2026-09-21 ospf_pe)", "PE Routers", ""},
		{"no row", "", "CHAP", "rfc9999 declares no declined feature"},
		{"no row names the row to add", "", "CHAP", "Add the row `| Declined features | CHAP (<owner-decision source>) |` to the `## Meta` table of rfc/short/rfc9999.md"},
		{"no row names the row", "", "CHAP", "rfc/short/rfc9999.md:"},
		{"no matching entry", "PAP (owner a)", "CHAP", "holds no entry the heading 'CHAP' names"},
		{"no matching entry names the row to write", "PAP (owner a)", "CHAP", "`| Declined features | PAP (owner a), CHAP (<owner-decision source>) |`"},
		{"an entry inside a hyphenated word", "MS-CHAP (owner a)", "CHAP", "holds no entry the heading 'CHAP' names"},
		{"a two-letter word not an entry's whole words", "PE Routers (owner a)", "PE Routers", "under 3 characters"},
		{"a two-letter word with a generic word", "ID Field (owner a)", "ID Field", "under 3 characters"},
		{"an entry with no source", "CHAP", "CHAP", "names no owner decision"},
		{"an empty source", "CHAP ()", "CHAP", "names no owner decision"},
		{"a placeholder source", "CHAP (none)", "CHAP", "names no owner decision"},
		{"an empty entry", chap + ", ", "CHAP", "holds an empty entry"},
		{"an entry with no words", "(owner a)", "CHAP", "names no feature"},
		{"an unclosed source", "CHAP (owner a", "CHAP", "unbalanced"},
		{"two sources", "CHAP (owner a) (owner b)", "CHAP", "with the source last"},
		{"words after the source", "CHAP (owner a) Login", "CHAP", "with the source last"},
		{"a nested source", "CHAP (owner (a))", "CHAP", "unbalanced"},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			_, err := ParseSummaryText(fixtureWithDeclined(t, one.declared, headingFixtureRow(one.words)),
				selftestStem, selftestSummaryRel)
			if one.refused == "" {
				if err != nil {
					t.Fatalf("heading %q with declined features %q was refused: %v", one.words, one.declared, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), one.refused) {
				t.Fatalf("heading %q with declined features %q answered %v, want a refusal saying %q",
					one.words, one.declared, err, one.refused)
			}
		})
	}
	// A summary with no heading form owes no row.
	plain := `- [ ] [RFC9999-2-3] [MUST] A speaker MUST send the widget. (§2)`
	if _, err := ParseSummaryText(fixtureWithDeclined(t, "", plain), selftestStem, selftestSummaryRel); err != nil {
		t.Errorf("a summary with no heading form was refused: %v", err)
	}
}

func TestTheDeclinedFeaturesAreInTheRFCText(t *testing.T) {
	tree := headingFixtureTree(t)
	row := conditionalRow(t, "rfc8907", "5.4.2.3", headingChapRow, `feature-declined: heading "CHAP"; `+headingNever)
	absent := conditionalRow(t, "rfc8906", "5.4.2.3", headingChapRow, `feature-declined: heading "CHAP"; `+headingNever)
	metas := map[string]Meta{
		"rfc8907": {DeclinedFeatures: []DeclinedFeature{{Words: "CHAP", Source: "owner a"}, {Words: "Widgetry", Source: "owner b"}}},
		"rfc8906": {DeclinedFeatures: []DeclinedFeature{{Words: "CHAP", Source: "owner a"}}},
	}
	errs := checkDeclinedFeaturesInText(tree, metas, []Requirement{row, absent})
	if len(errs) != 2 {
		t.Fatalf("want two refusals, Widgetry and the absent text, got %d: %q", len(errs), errs)
	}
	if !strings.Contains(errs[0], "'Widgetry', which rfc/full/rfc8907.txt never names") {
		t.Errorf("the refusal %q does not name Widgetry", errs[0])
	}
	if !strings.Contains(errs[1], "rfc/full/rfc8906.txt is absent") {
		t.Errorf("the refusal %q does not name the absent text", errs[1])
	}
}

// VALIDATES: the RFC's own title is read from the front matter of the five
// corpus texts that open with no Abstract: a "DARPA INTERNET PROGRAM" line
// follows the title of RFC 791 and 792, RFC 905 writes "Status of this
// Memo:", and RFC 1034 and 1035 number their status heading, so their title
// ends the front matter.
func TestTheTitleOfAnRFCWithNoAbstract(t *testing.T) {
	want := map[string]string{
		"rfc791":  "INTERNET PROTOCOL",
		"rfc792":  "INTERNET CONTROL MESSAGE PROTOCOL",
		"rfc905":  "ISO Transport Protocol Specification ISO DP 8073",
		"rfc1034": "DOMAIN NAMES - CONCEPTS AND FACILITIES",
		"rfc1035": "DOMAIN NAMES - IMPLEMENTATION AND SPECIFICATION",
	}
	for stem, title := range want {
		text, err := os.ReadFile(filepath.Join("..", "..", "..", "rfc", "full", stem+".txt"))
		if err != nil {
			t.Fatalf("read the %s text: %v", stem, err)
		}
		if got := newQuoteSource(string(text)).documentTitle; got != title {
			t.Errorf("the title of %s is %q, want %q", stem, got, title)
		}
	}
}

// VALIDATES: every word genericHeadingWords was found missing titles a section
// of nearly any RFC, so a heading of it alone names no feature.
func TestTheGenericHeadingWordsHoldTheCommonTitles(t *testing.T) {
	for word := range strings.FieldsSeq("timer timers behaviour address addressing attribute capability " + //nolint:misspell // RFCs written in UK English title sections "Behaviour".
		"negotiation notification request response reply data length flag code configuration management " +
		"compatibility transport connection algorithm registration change encoding session author") {
		if !headingWordsGeneric(word) {
			t.Errorf("the heading %q names a feature, want a generic heading word", word)
		}
	}
}

// VALIDATES: the never clause is read only after the quoted segment of the
// payload, so a never clause inside the quotation marks proves nothing.
// METHOD: neverClauseGap over a hand-built heading whose words hold a never
// clause and are written with blanks inside the quotation marks, which the
// parser trims, so the words alone do not find the segment's end.
func TestTheNeverClauseIsReadAfterTheQuotedSegment(t *testing.T) {
	tree := headingFixtureTree(t)
	inside := "CHAP; ze never uses CHAP: internal/x/frame.go::sendFrame sends PAP"
	req := conditionalRow(t, "rfc8907", "5.4.2.3", headingChapRow, `feature-declined: heading "CHAP"; `+headingNever)
	req.Annotation.Heading = inside
	req.Annotation.Reason = `heading " ` + inside + ` "; internal/x/frame.go::sendFrame sends PAP only`
	missing, lacks := neverClauseGap(&req, newSourceReader(tree), neverScope{opening: "its heading form",
		evidence: inside, against: "the heading's feature words"})
	if !lacks || !strings.Contains(missing, "no never clause") {
		t.Fatalf("a never clause inside the quotation marks was read: %q, %v", missing, lacks)
	}
}
