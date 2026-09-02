// VALIDATES: spec-le-is-a-ze-binary AC-5 and AC-7 -- the summary reader is
// called as a function, and its answer is structured data.
// PREVENTS: a parser that skips what it cannot read. A silently dropped
// checklist line is a MUST nobody enforces and a gate that still says OK, which
// is the failure this whole module exists to make impossible.

package rfc

import (
	"strings"
	"testing"
)

// oneLine parses a single checklist line and answers what it produced.
func oneLine(t *testing.T, line string) (*Requirement, error) {
	t.Helper()
	return parseChecklistLine(line, "rfc9999", "rfc/short/rfc9999.md", 7)
}

func TestAChecklistLineParsesIntoItsFields(t *testing.T) {
	req, err := oneLine(t, "- [x] [RFC9999-5.3-4] [MUST NOT] A speaker MUST NOT send it (§5.3)")
	if err != nil {
		t.Fatalf("parsing a well-formed line: %v", err)
	}
	if req == nil {
		t.Fatal("a well-formed compliance line parsed as prose")
	}
	if req.RID != "RFC9999-5.3-4" || req.Level != "MUST NOT" || req.Section != "5.3" {
		t.Errorf("fields: rid=%q level=%q section=%q", req.RID, req.Level, req.Section)
	}
	if !req.Ticked {
		t.Error("a hand-ticked box was not recorded")
	}
	if !req.Gated() {
		t.Error("MUST NOT is a gated level and this row says it is not")
	}
}

func TestAnAdHocCategoryLineIsProseRatherThanAnError(t *testing.T) {
	req, err := oneLine(t, "- [ ] [FORMAT] the header is four octets")
	if err != nil {
		t.Fatalf("an implementation-task line is not a compliance line: %v", err)
	}
	if req != nil {
		t.Errorf("an ad-hoc category line parsed as a requirement: %+v", req)
	}
}

func TestALineCarryingAKeywordBracketIsRefusedRatherThanSkipped(t *testing.T) {
	// The fail-open this refusal closes: the retired counter form has an
	// unrecognized FIRST bracket, so a reader deciding from that bracket alone
	// dismissed it as an ad-hoc line and took a live MUST out of the ledger.
	_, err := oneLine(t, "- [ ] [RFC9999-R012] [MUST] A speaker MUST send it (§2)")
	if err == nil {
		t.Fatal("the retired counter form was silently skipped")
	}
	if !strings.Contains(err.Error(), "carries an RFC 2119 keyword but does not parse") {
		t.Errorf("the refusal does not say why: %v", err)
	}
}

func TestAnIdMustAgreeWithTheSectionItsLineCites(t *testing.T) {
	_, err := oneLine(t, "- [ ] [RFC9999-5.3-1] [MUST] A speaker MUST send it (§2)")
	if err == nil {
		t.Fatal("an id claiming §5.3 on a line citing §2 was accepted")
	}
	if !strings.Contains(err.Error(), "disagrees with its section") {
		t.Errorf("the refusal does not name the contradiction: %v", err)
	}
}

func TestAnOrdinalStartsAtOne(t *testing.T) {
	_, err := oneLine(t, "- [ ] [RFC9999-2-0] [MUST] A speaker MUST send it (§2)")
	if err == nil || !strings.Contains(err.Error(), "ordinal starts at 1") {
		t.Errorf("a zero ordinal was accepted: %v", err)
	}
}

func TestTheSectionIsTheTrailingCitationAndNotTheFirstMention(t *testing.T) {
	cases := []struct {
		name, text, want string
	}{
		{"a section mark", "text (§5.3)", "5.3"},
		{"the word Section", "text (Section 6)", "6"},
		{"the bare S form", "text (S4.1)", "4.1"},
		{"a lettered section", "text (§3.b)", "3.b"},
		{"no citation at all", "text with none", noSection},
		{
			// A requirement whose prose mentions another section first must
			// anchor to the section it is FROM, not the one it refers TO.
			name: "a mention before the citation",
			text: "routes via §3.j to session reset (§5.3)",
			want: "5.3",
		},
		{
			// Anchoring an RFC 1071 requirement to RFC 2328's §A.3.1 would
			// point at another document's numbering.
			name: "a citation naming another document",
			text: "as RFC 2328 §A.3.1 says (§7.1)",
			want: "7.1",
		},
		{"a cross-document citation and nothing else", "as RFC 2328 §A.3.1 says", noSection},
		{"the first of several", "merged (§2, §3.h)", "2"},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			if got := extractSection(one.text); got != one.want {
				t.Errorf("ExtractSection(%q) = %q, want %q", one.text, got, one.want)
			}
		})
	}
}

func TestSHOULDAndSHALLAreNotReadAsTheBareSForm(t *testing.T) {
	// The bare-S form demands a digit right after the S. Without that, every
	// SHOULD in the corpus would anchor a requirement to section "HOULD".
	for _, text := range []string{"a speaker SHOULD do it", "a speaker SHALL do it", "AS4 is a number"} {
		if got := extractSection(text); got != noSection {
			t.Errorf("ExtractSection(%q) = %q, want %q", text, got, noSection)
		}
	}
}

func TestACoverageAnnotationAndASupersededMarkerCompose(t *testing.T) {
	// The two are different registers and one must never cost the reader the
	// other: a requirement can be both un-testable here and restated over
	// there.
	req, err := oneLine(t, "- [ ] [RFC9999-2-1] [MUST] A speaker MUST send it (§2) "+
		"{gap: no producer yet} {superseded: restated RFC9568-5.2.3-2; the successor states it}")
	if err != nil {
		t.Fatalf("a line carrying both markers: %v", err)
	}
	if req.Annotation == nil || req.Annotation.Kind != "gap" {
		t.Fatalf("the coverage annotation was lost: %+v", req.Annotation)
	}
	if req.Superseded == nil || req.Superseded.Target != "RFC9568-5.2.3-2" {
		t.Fatalf("the forward pointer was lost: %+v", req.Superseded)
	}
	if strings.Contains(req.Text, "{") {
		t.Errorf("a marker was left inside the requirement text: %q", req.Text)
	}
	if req.Section != "2" {
		t.Errorf("a marker left in the text moved the anchor: %q", req.Section)
	}
}

func TestASinglePolarityAnnotationCarriesItsPolarityAndItsReason(t *testing.T) {
	req, err := oneLine(t, "- [ ] [RFC9999-2-1] [MUST] A speaker MUST send it (§2) "+
		"{single-polarity: negative; the positive case needs hardware}")
	if err != nil {
		t.Fatalf("a single-polarity annotation: %v", err)
	}
	if req.Annotation.Polarity != "negative" {
		t.Errorf("polarity: %q", req.Annotation.Polarity)
	}
	if req.Annotation.Reason != "the positive case needs hardware" {
		t.Errorf("reason: %q", req.Annotation.Reason)
	}
}

func TestEverySupersededDispositionStatesWhatItNames(t *testing.T) {
	cases := []struct {
		name, body, target string
		refused            string
	}{
		{name: "restated", body: "{superseded: restated RFC9568-5.2.3-2; why}", target: "RFC9568-5.2.3-2"},
		{name: "dropped", body: "{superseded: dropped; the successor states nothing}"},
		{name: "unextracted", body: "{superseded: unextracted §8.2.3; debt}", target: "§8.2.3"},
		{name: "unresolved", body: "{superseded: unresolved; the text is not here}"},
		{
			name: "a disposition nobody knows", body: "{superseded: moved; why}",
			refused: "unknown {superseded} disposition",
		},
		{
			name: "dropped naming a target", body: "{superseded: dropped RFC1-1-1; why}",
			refused: "names nothing",
		},
		{
			name: "restated naming nothing", body: "{superseded: restated; why}",
			refused: "needs exactly one successor requirement id",
		},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			req, err := oneLine(t, "- [ ] [RFC9999-2-1] [MUST] A speaker MUST send it (§2) "+one.body)
			if one.refused != "" {
				if err == nil || !strings.Contains(err.Error(), one.refused) {
					t.Fatalf("want a refusal naming %q, got %v", one.refused, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("%s: %v", one.name, err)
			}
			if req.Superseded.Disposition != one.name || req.Superseded.Target != one.target {
				t.Errorf("%s: %+v", one.name, req.Superseded)
			}
		})
	}
}

func TestADuplicateIdIsRefused(t *testing.T) {
	text := "- [ ] [RFC9999-2-1] [MUST] first (§2)\n- [ ] [RFC9999-2-1] [MUST] second (§2)\n"
	_, err := parseSummaryText(text, "rfc9999", "rfc/short/rfc9999.md")
	if err == nil || !strings.Contains(err.Error(), "duplicate requirement id") {
		t.Errorf("two rows sharing one id were accepted: %v", err)
	}
}

// VALIDATES: AC-1 and AC-3 -- enrolment is read from the summary's own Meta
// table, and an absent or unrecognized value is REFUSED rather than defaulted.
// PREVENTS: the largest recorded defect class in this repository. A field that
// defaulted to "not enrolled" would take an RFC out of the gated population with
// no author intending it and no gate saying so
// (plan/journal/gate-excludes-part-of-its-population.md).
func TestEnrolmentIsReadFromTheSummaryMetaTable(t *testing.T) {
	const head = "# RFC 9999\n\n## Meta\n\n| Field | Value |\n|-------|-------|\n| Title | Widgets |\n"
	const where = "rfc/short/rfc9999.md"

	meta, err := ParseMeta(head+
		"| Enrolment | enrolled |\n| Enrolment reason | treat-as-withdraw is proven both ways |\n"+
		"| Support | - |\n", "rfc9999", where)
	if err != nil {
		t.Fatalf("a summary declaring its enrolment did not parse: %v", err)
	}
	if !meta.Enrolled() || meta.EnrolmentReason != "treat-as-withdraw is proven both ways" {
		t.Errorf("the parse read %q / %q", meta.Enrolment, meta.EnrolmentReason)
	}

	for _, one := range []struct {
		name string
		rows string
		want string
	}{
		{"absent", "| Support | - |\n", "no `Enrolment` row"},
		{"unknown value", "| Enrolment | maybe |\n| Enrolment reason | x |\n| Support | - |\n",
			"not one of"},
		{"no reason", "| Enrolment | backlog |\n| Support | - |\n", "no `Enrolment reason` row"},
		{"near-miss label", "| Enrolled | enrolled |\n| Enrolment | enrolled |\n" +
			"| Enrolment reason | x |\n| Support | - |\n", "spelling nothing reads"},
	} {
		t.Run(one.name, func(t *testing.T) {
			_, err := ParseMeta(head+one.rows, "rfc9999", where)
			if err == nil {
				t.Fatalf("%s was accepted", one.name)
			}
			if !strings.Contains(err.Error(), one.want) {
				t.Errorf("the refusal does not say %q:\n%s", one.want, err)
			}
		})
	}
}

// VALIDATES: the Meta scan stops at the end of the Meta TABLE.
// PREVENTS: rfc/short/rfc8277.md refusing for a duplicate that is not one. It
// states its AFI/SAFI scope as a second table under the same heading, whose
// first column repeats `1`, and a scan bounded by the next `##` heading read
// those as Meta field names.
func TestTheMetaScanStopsAtItsOwnTable(t *testing.T) {
	const summary = "# RFC 9999\n\n## Meta\n\n| Field | Value |\n|-------|-------|\n" +
		"| Title | Widgets |\n| Enrolment | enrolled |\n| Enrolment reason | gated |\n" +
		"| Support | - |\n\n**Scope:**\n| AFI | SAFI | Description |\n|---|---|---|\n" +
		"| 1 | 4 | Labeled IPv4 Unicast |\n| 1 | 128 | VPN-IPv4 |\n"
	meta, err := ParseMeta(summary, "rfc9999", "rfc/short/rfc9999.md")
	if err != nil {
		t.Fatalf("a second table under the same heading was read as Meta rows: %v", err)
	}
	if meta.Title != "Widgets" {
		t.Errorf("the title is %q", meta.Title)
	}
}

func TestGatedCountsCountOnlyMustLevelRows(t *testing.T) {
	reqs := []Requirement{
		{RFC: "rfc1", Level: "MUST"}, {RFC: "rfc1", Level: "SHOULD"},
		{RFC: "rfc1", Level: "REQUIRED"}, {RFC: "rfc2", Level: "MAY"},
	}
	got := gatedCounts(reqs)
	if got["rfc1"] != 2 {
		t.Errorf("rfc1 gated count = %d, want 2", got["rfc1"])
	}
	if _, held := got["rfc2"]; held {
		t.Errorf("an RFC declaring only advisory rows is counted: %v", got)
	}
}

// VALIDATES: the title comes from the labeled Meta row, and a summary carrying
// no such row answers empty rather than guessing at the H1.
//
// The H1 separator is an em dash in one summary, a double hyphen in another and
// a colon in a third, and one H1 carries a "(short)" suffix, so a fallback
// parser would have to guess which half of the heading is the title. A wrong
// title on a published page states a fact about a standards document that the
// document does not state.
func TestASummaryTitleComesFromTheMetaRow(t *testing.T) {
	const summary = "# RFC 9999 -- Widgets, or so it says\n\n## Meta\n\n" +
		"| Field | Value |\n|-------|-------|\n| RFC | 9999 |\n" +
		"| Title | The Widget Protocol |\n| Enrolment | enrolled |\n" +
		"| Enrolment reason | gated |\n| Support | - |\n"
	meta, err := ParseMeta(summary, "rfc9999", "rfc/short/rfc9999.md")
	if err != nil {
		t.Fatalf("the fixture summary did not parse: %v", err)
	}
	if meta.Title != "The Widget Protocol" {
		t.Errorf("the title is %q, want the Meta row's own value", meta.Title)
	}
	bare, err := ParseMeta("# RFC 9999 -- Widgets\n\n## Meta\n\n| Field | Value |\n|--|--|\n"+
		"| Enrolment | enrolled |\n| Enrolment reason | gated |\n| Support | - |\n",
		"rfc9999", "rfc/short/rfc9999.md")
	if err != nil {
		t.Fatalf("a summary with no Title row did not parse: %v", err)
	}
	if bare.Title != "" {
		t.Errorf("a summary with no Title row answered %q, want the empty string", bare.Title)
	}
}

// VALIDATES: every summary of this corpus declares its title, so the empty
// answer above is unreachable on the published pages.
//
// The method is the real tree rather than a fixture: the backfill of 2026-09-01
// is what makes this true, and only the corpus can say whether it stayed true.
func TestEverySummaryCarriesATitleRow(t *testing.T) {
	root := checkoutRoot(t)
	stems, err := summaryStems(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(stems) == 0 {
		t.Fatal("this checkout carries no summary, so this proves nothing")
	}
	metas, metaProblems, err := summaryMetas(root, stems)
	if err != nil {
		t.Fatal(err)
	}
	if len(metaProblems) > 0 {
		t.Fatalf("the corpus holds %d unparsable Meta table(s): %v", len(metaProblems), metaProblems)
	}
	titles := titlesFrom(metas)
	var missing []string
	for _, stem := range sortedSet(stems) {
		if titles[stem] == "" {
			missing = append(missing, stem)
		}
	}
	if len(missing) != 0 {
		t.Errorf("%d summary/summaries declare no Meta | Title | row: %s",
			len(missing), strings.Join(missing, ", "))
	}
}

// VALIDATES: an unescaped pipe inside a Meta value is REFUSED rather than
// truncating the value.
// PREVENTS: the RFC 1994 defect one level up. The authored public page wrote
// `MD5(id||secret||challenge)` in a coverage cell, so its own parser cut that
// row into nine cells, truncated the coverage and discarded the remainder
// entirely. A Meta table that read the same shape the same way would reproduce
// the loss on the surface that now feeds the page, and the freshness check
// would call the result fresh, because it compares generation with generation.
func TestAnUnescapedPipeInAMetaValueIsRefused(t *testing.T) {
	const head = "# RFC 9999\n\n## Meta\n\n| Field | Value |\n|-------|-------|\n" +
		"| Enrolment | enrolled |\n| Enrolment reason | gated |\n| Support | - |\n"

	_, err := ParseMeta(head+"| Title | MD5(id|secret) |\n", "rfc9999", "rfc/short/rfc9999.md")
	if err == nil {
		t.Fatal("an unescaped pipe was accepted, so the value was truncated in silence")
	}
	if !strings.Contains(err.Error(), "unescaped") {
		t.Errorf("the refusal does not name the cause:\n%s", err)
	}

	// Escaped, it survives whole, which is what lets the row round-trip.
	meta, err := ParseMeta(head+`| Title | MD5(id\|secret) |`+"\n", "rfc9999", "rfc/short/rfc9999.md")
	if err != nil {
		t.Fatalf("an escaped pipe was refused: %v", err)
	}
	if meta.Title != `MD5(id\|secret)` {
		t.Errorf("the escaped value came back as %q", meta.Title)
	}
}

// VALIDATES: the Meta scan stops at PROSE between the heading and its table,
// rather than adopting the next table it finds.
// PREVENTS: the silent version of a missing Meta table. A summary that lost its
// table would otherwise read a later section's -- a wire-format field column, an
// AFI/SAFI column -- as its Meta rows, and declare an enrolment nobody wrote.
// The refusal is the loud outcome, and nothing pinned it until an independent
// review noticed a revert to `continue` would be invisible.
func TestProseBetweenTheHeadingAndItsTableEndsTheMetaScan(t *testing.T) {
	const summary = "# RFC 9999\n\n## Meta\n\nThe table for this document is still to be written.\n\n" +
		"| AFI | SAFI | Description |\n|---|---|---|\n| 1 | 4 | Labeled IPv4 Unicast |\n"

	_, err := ParseMeta(summary, "rfc9999", "rfc/short/rfc9999.md")
	if err == nil {
		t.Fatal("a later table was adopted as the Meta table, so the summary declared what nobody wrote")
	}
	if !strings.Contains(err.Error(), "carries no table") {
		t.Errorf("the refusal does not name the cause:\n%s", err)
	}
}
