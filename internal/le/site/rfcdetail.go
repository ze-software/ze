// Design: website/AI.md -- one page for each RFC summary this repository carries
// Detail: rfcledger.go derives the snapshot; rfccompliance.go writes the index over them.
//
// The disclosure on these pages is FULL, by owner ruling of 2026-09-01. A gated
// MUST with no test, a weak, wrong or unimplemented audit verdict, a verdict
// that is no longer current, a tagged unit with no recorded break, and a record
// claiming no break exists are each named on the page under the requirement id
// they belong to. A count MAY accompany the list. It never stands in for it: a
// page that says "6 requirements carry a weak verdict" and does not name the
// six has reproduced the aggregate this family exists to replace.
package site

import (
	"html"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/rfc"
)

// The detail family's own addresses in the artifact.
const (
	// rfcDetailRoot reaches the site root from one stem's page, which sits one
	// level below the compliance index.
	rfcDetailRoot = "../../../"
	// rfcNoPublicRow is what the page says for a stem docs/features/rfc-status.md
	// carries no row for. 39 of the 190 stems are in that state, and a blank
	// cell would read as "no status" rather than as "no row".
	rfcNoPublicRow = "No row in the public ledger"
	// rfcUnproven is what a tagged unit with no recorded break reads as, in its
	// own column. It is not a proof and it is never counted as one, and the
	// sentence that says why is the legend above the table rather than a copy
	// on every row.
	rfcUnproven = "unproven"

	// rfcGatedBucketLabel names the whole that the coverage buckets partition.
	rfcGatedBucketLabel = "Gated MUST-level requirements"

	// rfcDetailMarker is what a page of THIS family carries and no other page
	// does. removeRetiredRFCPages keys its deletion on it.
	rfcDetailMarker = ` id="rfc-detail-title"`
	// rfcProofLegend is that sentence.
	rfcProofLegend = "A tagged unit reads " + rfcUnproven + " where no discrimination record " +
		"exists for it: nothing in this tree has been observed to break it, so the claim its " +
		"tag makes is unproven."
	// The two facts the index and one stem's page both label. Named once, so a
	// reader following a link from the index meets the same words on the page.
	rfcDeclaredGapsLabel = "Declared gaps"
	rfcTestTagsLabel     = "Test tags"
	// rfcEnrolmentLabel is the summary's own Meta label, spelled as the Meta
	// table spells it. The three surfaces that print it read it from here, so
	// none of them can drift into the American spelling the label does not use.
	rfcEnrolmentLabel = "Enrolment"
)

// writeRFCDetailPage publishes one summary's page and its mirror, and answers
// the route it wrote.
func writeRFCDetailPage(output string, entry *rfcLedgerStem, links pageLinks) (string, error) {
	route := rfcComplianceRoute + entry.Stem + "/"
	destination := rfcComplianceDirectory + "/" + entry.Stem + "/" + pageIndexFile
	shell := pageShell{
		Title:       entry.Display + " requirement ledger - Ze",
		Description: rfcDetailDescription(entry),
		Root:        rfcDetailRoot,
		Path:        destination,
		Sidebar:     pageSidebar(rfcDetailRoot, route[1:], links),
	}
	if err := writePublishedPage(output, destination,
		shell.render(rfcDetailBody(entry)), rfcDetailMirror(entry)); err != nil {
		return "", err
	}
	return route, nil
}

// rfcDetailSections declares this family's page ONCE: the heading of each
// section, the function that renders it as markup, and the function that states
// it in the mirror.
//
// Both renderings walk this list, so a section cannot reach the page and miss
// the mirror, and the two cannot disagree about what a heading is called
// (ai/rules/principles.md).
var rfcDetailSections = []struct {
	Heading string
	HTML    func(*rfcLedgerStem) string
	Mirror  func(*rfcLedgerStem) string
}{
	{"Overview", rfcOverviewHTML, rfcOverviewMirror},
	{"At a glance", rfcGlanceHTML, rfcGlanceMirror},
	{rfcEnrolmentLabel, rfcEnrolmentHTML, rfcEnrolmentMirror},
	{"What the public ledger says", rfcPublicLedgerHTML, rfcPublicLedgerMirror},
	{"Coverage", rfcCoverageHTML, rfcCoverageMirror},
	{"Requirements", rfcRequirementsHTML, rfcRequirementsMirror},
	{"Gaps and untested MUSTs", rfcGapsHTML, rfcGapsMirror},
	{"Proof state", rfcProofHTML, rfcProofMirror},
	{"Extraction sign-off", rfcExtractionHTML, rfcExtractionMirror},
	{"Superseded", rfcSupersededHTML, rfcSupersededMirror},
}

// rfcDetailDescription is the one-line summary a search result shows.
func rfcDetailDescription(entry *rfcLedgerStem) string {
	var out textbuf.Buffer
	out.Str(entry.Display)
	if entry.Title != "" {
		out.Str(": ").Str(entry.Title)
	}
	out.Str(". ").Int(int64(entry.Coverage.Gated)).
		Str(" gated MUST-level requirements, ").Int(int64(entry.Coverage.Gaps)).
		Str(" declared gaps, ").Int(int64(entry.Coverage.Missing)).Str(" with no test.")
	return out.String()
}

// rfcDetailBody renders one summary's page under <main>.
func rfcDetailBody(entry *rfcLedgerStem) string {
	var body textbuf.Buffer
	body.Str(`            <section aria-labeledby="rfc-detail-title" class="md-content reveal cat-observe">`).Byte('\n')
	body.Str(pageHero(html.EscapeString(rfcDetailHeading(entry)),
		html.EscapeString(rfcDetailLead(entry)), rfcDetailEyebrow(entry),
		rfcDetailMarker, heroClasses)).Byte('\n')
	body.Str(rfcComplianceStyle)
	for _, section := range rfcDetailSections {
		body.Str("<section><h2>").Str(html.EscapeString(section.Heading)).Str("</h2>\n").
			Str(section.HTML(entry)).Str("\n</section>\n")
	}
	body.Str("            </section>\n")
	return body.String()
}

// rfcDetailMirror renders one summary's Markdown sibling, which states the same
// facts as the page in the order the page states them.
func rfcDetailMirror(entry *rfcLedgerStem) string {
	var out textbuf.Buffer
	out.Str("# ").Str(rfcDetailHeading(entry)).Str("\n\n")
	out.Str(rfcDetailEyebrow(entry)).Str(". ").Str(rfcDetailLead(entry)).Byte('\n')
	for _, section := range rfcDetailSections {
		out.Str("\n## ").Str(section.Heading).Str("\n\n").Str(section.Mirror(entry))
	}
	return out.String()
}

// rfcDetailHeading is the page's own H1: the display name, and the RFC's title
// where the summary declares one.
func rfcDetailHeading(entry *rfcLedgerStem) string {
	if entry.Title == "" {
		return entry.Display
	}
	return entry.Display + " - " + entry.Title
}

// rfcDetailLead is the sentence under the heading.
func rfcDetailLead(entry *rfcLedgerStem) string {
	state := "not enrolled"
	if entry.Enrolled {
		state = "enrolled and gated by ./le rfc check"
	}
	return "Every requirement this repository extracted from " + entry.Display +
		", the tests bound to it, and what a reader has verified about them. " +
		"This summary is " + state + "."
}

// rfcDetailEyebrow is the label above the heading: the public status this RFC
// carries, or the fact that the public ledger carries no row for it.
func rfcDetailEyebrow(entry *rfcLedgerStem) string {
	if entry.PublicStatus == "" {
		return rfcNoPublicRow
	}
	return entry.PublicStatus
}

// rfcDetailCards answers one summary's headline numbers, each with the tone
// that names what its measure MEANS and the rule that chose it.
//
// SCALE first, then STANDING, as on the index and for the same reason: the
// population the gate holds, the part of it that does not bind Ze, then the
// shares that partition the whole of it. The shares are built by
// rfcStandingCards, so this page and the index cannot publish different
// partitions of one idea.
//
// The shares were over `gated - not-applicable` until 2026-09-02, and on a page
// about ONE RFC that subtraction was at its worst: a summary whose obligations
// are all annotated published four shares of nothing, and every other page
// quietly measured itself against a denominator its own scale card did not
// state. They are over the gated count now, with the not-applicable obligations
// as a named share (owner decision, 2026-09-02).
func rfcDetailCards(entry *rfcLedgerStem) []rfcCard {
	coverage := &entry.Coverage
	proof := rfcProofCountsOf(entry)
	unsoundWords := rfc.VerdictWeak + ", " + rfc.VerdictWrong + " or " + rfc.VerdictUnimplemented
	cards := []rfcCard{
		{Label: rfcGatedCardLabel(entry), Value: groupThousands(coverage.Gated),
			Count: "of " + groupThousands(coverage.Requirements) + " this summary declares",
			Note:  rfcGatedCardNote(entry),
			Tone:  rfcToneNeutral,
			Rule: "no color: a population is a scale, and a larger one is neither good news " +
				"nor bad. It is the accounting total"},
		{Label: "Out of scope", Value: groupThousands(coverage.NotApplicable),
			Count: "of " + groupThousands(coverage.Gated) + " gated MUSTs",
			Note: "a {not-applicable} annotation says the obligation does not bind Ze. Scope, " +
				"not coverage, and it is the Not applicable share below: it stays in the " +
				"denominator every share on this page is taken over",
			Tone: rfcToneNeutral,
			Rule: "no color: an obligation that never bound Ze is neither an achievement nor a " +
				"failure, and counting it either way would be a claim"},
	}
	cards = append(cards, rfcStandingCards(coverage.Gated, coverage.Bucket)...)
	return append(cards,
		rfcProofCard(coverage.Proven(), coverage.Units, coverage.Escapes, coverage.Stale, ""),
		rfcCard{Label: "Audit verdicts", Value: groupThousands(coverage.Audited),
			Count: "of " + groupThousands(coverage.Gated) + " gated MUSTs judged",
			Note: groupThousands(proof.Unsound) + " " + unsoundWords + ", " +
				groupThousands(proof.NotCurrent) + " no longer current. Each is named below " +
				"under its own requirement id",
			Tone: rfcAuditTone(entry, proof),
			Rule: "RED on the first " + unsoundWords + " verdict, amber while a verdict is no " +
				"longer current or a gated MUST is unjudged, green when every one is judged " +
				"sound and current"})
}

// rfcGatedCardLabel and rfcGatedCardNote say what the population card counts,
// which is NOT the same fact on an un-enrolled summary.
//
// `evaluate` skips every requirement of an RFC that is not enrolled
// (internal/le/rfc/check_core.go), so the gate holds none of them. The card
// said "MUST-level requirements the gate HOLDS" on all 190 pages, and on the
// un-enrolled ones that sentence was false (independent review, 2026-09-01).
func rfcGatedCardLabel(entry *rfcLedgerStem) string {
	if entry.Enrolled {
		return "Gated MUSTs"
	}
	return "MUSTs declared"
}

// rfcGatedCardNote is the sentence beneath it.
func rfcGatedCardNote(entry *rfcLedgerStem) string {
	if entry.Enrolled {
		return "MUST-level requirements the gate HOLDS. A population, not a result: the " +
			"shares beside it are what says how Ze stands"
	}
	return "MUST-level requirements this summary DECLARES. The gate holds none of them, " +
		"because this RFC is not enrolled (" + entry.Disposition.Kind + "), so every share " +
		"below reads what the summary records rather than what the gate enforces"
}

// rfcAuditTone says how the audit card reads: bad where a reader judged the
// tests unsound, and a warning where nobody has judged them or the judgement is
// no longer current.
func rfcAuditTone(entry *rfcLedgerStem, proof rfcProofCounts) string {
	if proof.Unsound > 0 {
		return rfcToneBad
	}
	if proof.NotCurrent > 0 || entry.Coverage.Audited < entry.Coverage.Gated {
		return rfcToneWarn
	}
	return rfcToneOK
}

// rfcOverviewHTML renders the card grid the index page carries, over one
// summary's own numbers.
func rfcOverviewHTML(entry *rfcLedgerStem) string {
	cards := rfcDetailCards(entry)
	return rfcCardsHTML(cards, entry.Coverage.Gated) + rfcToneLegendHTML(cards)
}

// rfcOverviewMirror states the same cards as a table.
func rfcOverviewMirror(entry *rfcLedgerStem) string {
	cards := rfcDetailCards(entry)
	return rfcCardsMirror(cards, entry.Coverage.Gated) + "\n" + rfcToneLegendMirror(cards)
}

// rfcGlanceFacts answers the at-a-glance rows, as (term, value) pairs already
// escaped for HTML.
//
// COUNTABLE facts and repository paths only. The enrolment reason, the public
// coverage sentence and the public remainder are prose hundreds of characters
// long, and a two-column table is not where a reader reads a paragraph (owner
// review, 2026-09-01). Each has its own section below.
//
// One list, read by the page and by the mirror, so the two cannot state
// different facts about one summary (AC-9).
func rfcGlanceFacts(entry *rfcLedgerStem) [][2]string {
	return [][2]string{
		{"Public status", html.EscapeString(rfcDetailEyebrow(entry))},
		{rfcEnrolmentLabel, html.EscapeString(rfcEnrolmentState(entry))},
		{"Requirements", strconv.Itoa(entry.Coverage.Requirements)},
		{"Gated MUST-level", strconv.Itoa(entry.Coverage.Gated)},
		{"Not applicable, so out of scope", strconv.Itoa(entry.Coverage.NotApplicable)},
		{rfcDeclaredGapsLabel, strconv.Itoa(entry.Coverage.Gaps)},
		{"Gated with no test", strconv.Itoa(entry.Coverage.Missing)},
		{"Nightly-only evidence", strconv.Itoa(entry.Coverage.NightlyOnly)},
		{rfcTestTagsLabel, strconv.Itoa(entry.Coverage.Tags)},
		{"Tagged units", strconv.Itoa(entry.Coverage.Units)},
		{"Recorded audit verdicts", strconv.Itoa(entry.Coverage.Audited)},
		{"Discrimination records", strconv.Itoa(entry.Coverage.Records)},
		{"Summary", "<code>" + html.EscapeString(entry.SummaryPath) + "</code>"},
		{"Requirement shard",
			rfcPathOrAbsent(entry.ShardPath, "no requirement declared, so no shard is generated")},
		{"RFC text",
			rfcPathOrAbsent(entry.SourcePath, "this checkout does not carry the RFC's own text")},
	}
}

// rfcPathOrAbsent answers a repository path as code, or says why there is none.
// An empty cell would read as a rendering fault rather than as a fact.
func rfcPathOrAbsent(path, absent string) string {
	if path == "" {
		return html.EscapeString(absent)
	}
	return "<code>" + html.EscapeString(path) + "</code>"
}

// rfcGlanceHTML renders the at-a-glance facts.
func rfcGlanceHTML(entry *rfcLedgerStem) string {
	var rows textbuf.Buffer
	for _, fact := range rfcGlanceFacts(entry) {
		rows.Str(rfcRowCells(html.EscapeString(fact[0]), fact[1]))
	}
	return rfcTableHTML(rfcHeadCells("Field", "Value"), rows.String())
}

// rfcGlanceMirror states the same facts.
func rfcGlanceMirror(entry *rfcLedgerStem) string {
	var out textbuf.Buffer
	out.Str(rfcMirrorHead("Field", "Value"))
	for _, fact := range rfcGlanceFacts(entry) {
		out.Str(rfcMirrorRow(fact[0], rfc.TableCell(rfcPlain(fact[1]))))
	}
	return out.String()
}

// rfcEnrolmentState is the short form the at-a-glance table carries: whether
// the gate holds this RFC, and the disposition kind where it does not.
func rfcEnrolmentState(entry *rfcLedgerStem) string {
	if entry.Enrolled {
		return "Enrolled"
	}
	return "Not enrolled (" + entry.Disposition.Kind + ")"
}

// rfcEnrolmentText says whether the gate holds this RFC, and the reason the
// summary states either way.
//
// No fallback on either side: ParseMeta refuses a summary with no
// `| Enrolment |` row and refuses a kind with no `| Enrolment reason |` beside
// it (readEnrolment, internal/le/rfc/meta.go), so every summary reaching this
// renderer carries a kind AND a reason. A branch for a state the parser refuses
// is dead code that reads like a real case (ai/rules/principles.md).
func rfcEnrolmentText(entry *rfcLedgerStem) string {
	if entry.Enrolled {
		return "Enrolled: " + entry.EnrolmentReason
	}
	return "Not enrolled (" + entry.Disposition.Kind + ", " +
		rfcDispositionMeaning(entry.Disposition.Kind) + "): " + entry.Disposition.Reason
}

// rfcDispositionMeaning answers what one un-enrolled kind says, in words a
// reader outside this project can act on.
//
// The sentence comes from internal/le/rfc, which holds the closed set the
// parser enforces. A kind spelled here would be a second declaration of that
// set, and the sixth kind landed on 2026-09-01 with the page still naming five
// (independent review). A kind the vocabulary does not know is NAMED rather
// than passed off as explained (ai/rules/principles.md).
func rfcDispositionMeaning(kind string) string {
	meaning, held := rfc.DispositionKindMeaning(kind)
	if !held {
		return "a disposition this page has no published meaning for"
	}
	return meaning
}

// rfcEnrolmentHTML renders the enrolment reason as the paragraph it is.
func rfcEnrolmentHTML(entry *rfcLedgerStem) string {
	return "<p>" + html.EscapeString(rfcEnrolmentText(entry)) + "</p>"
}

// rfcEnrolmentMirror states the same paragraph.
func rfcEnrolmentMirror(entry *rfcLedgerStem) string {
	return rfcEnrolmentText(entry) + "\n"
}

// rfcPublicLedgerHTML renders what docs/features/rfc-status.md says about this
// RFC: the status it publishes, what it says is covered, and what it says
// remains.
//
// Three labeled facts rather than one paragraph. The cell was reproduced
// verbatim and on RFC 4271 that is nine hundred words of editorial prose in
// front of a reader who wanted the status (owner review, 2026-09-01). Nothing
// is dropped: this is the disclosure, so the long halves are FOLDED behind a
// disclosure and a reader who opens one reads the whole cell.
func rfcPublicLedgerHTML(entry *rfcLedgerStem) string {
	if entry.PublicStatus == "" {
		return "<p>" + html.EscapeString(rfcNoPublicRow+rfcNoPublicRowWhy(entry)) + "</p>"
	}
	declared := rfcDeclaredIDs(entry)
	return "<p><strong>Status:</strong> " + html.EscapeString(entry.PublicStatus) + "</p>\n" +
		rfcClaimsHTML("What the ledger says is covered", rfcCoverageProse(entry), declared) +
		"\n" +
		rfcThemesHTML("What the ledger says remains", rfcRemainingText(entry), declared)
}

// rfcPublicLedgerMirror states the same three facts, and the same items.
func rfcPublicLedgerMirror(entry *rfcLedgerStem) string {
	if entry.PublicStatus == "" {
		return rfcNoPublicRow + rfcNoPublicRowWhy(entry) + "\n"
	}
	declared := rfcDeclaredIDs(entry)
	return "**Status:** " + entry.PublicStatus + "\n\n" +
		rfcClaimsMirror("What the ledger says is covered", rfcCoverageProse(entry), declared) +
		"\n" +
		rfcThemesMirror("What the ledger says remains", rfcRemainingText(entry), declared)
}

// rfcDeclaredIDs answers the requirement ids this summary declares, which is
// the set a mention in its prose can be linked to.
func rfcDeclaredIDs(entry *rfcLedgerStem) map[string]bool {
	declared := make(map[string]bool, len(entry.Requirements))
	for index := range entry.Requirements {
		declared[entry.Requirements[index].RID] = true
	}
	return declared
}

// rfcClaimsHTML renders a Coverage cell as the list of claims it is.
//
// The cell is a semicolon-chained list and was published as one paragraph of
// four thousand characters, which is unreadable and hides the structure the
// author wrote (owner review, 2026-09-01). A cell with no top-level semicolon
// answers one item and renders as the paragraph it is.
func rfcClaimsHTML(label, prose string, declared map[string]bool) string {
	items := rfcProseSplit(prose)
	if len(items) < 2 {
		return rfcFoldMarkupHTML(label, prose, rfcProseHTML(prose, declared))
	}
	var body textbuf.Buffer
	body.Str("<ul class=\"rfc-prose\">\n")
	for _, item := range items {
		// An empty item is the trailing semicolon the split keeps so that
		// rejoining reproduces the cell. It carries no claim, so it gets no
		// bullet.
		if strings.TrimSpace(item) == "" {
			continue
		}
		body.Str("<li>").Str(rfcProseHTML(strings.TrimSpace(item), declared)).Str("</li>\n")
	}
	body.Str("</ul>")
	return rfcFoldMarkupHTML(label, prose, body.String())
}

// rfcClaimsMirror states the same claims.
func rfcClaimsMirror(label, prose string, declared map[string]bool) string {
	items := rfcProseSplit(prose)
	if len(items) < 2 {
		return rfcFoldMarkupMirror(label, prose, rfcProseMirror(prose, declared))
	}
	var body textbuf.Buffer
	for _, item := range items {
		if strings.TrimSpace(item) == "" {
			continue
		}
		body.Str("- ").Str(rfcProseMirror(strings.TrimSpace(item), declared)).Byte('\n')
	}
	return rfcFoldMarkupMirror(label, prose, body.String())
}

// rfcThemesHTML renders a Remaining cell as its lead and its themed groups.
//
// The author wrote "Header error reporting: ...", "Timers: ..." and so on, and
// the renderer published them as one paragraph. A cell carrying no theme
// answers one part and renders whole rather than gaining headings nobody wrote.
func rfcThemesHTML(label, prose string, declared map[string]bool) string {
	themes := rfcProseThemes(prose)
	if len(themes) < 2 {
		return rfcFoldMarkupHTML(label, prose, rfcProseHTML(prose, declared))
	}
	var body textbuf.Buffer
	for _, theme := range themes {
		if theme.Label == "" {
			body.Str("<p>").Str(rfcProseHTML(strings.TrimSpace(theme.Body), declared)).
				Str("</p>\n")
			continue
		}
		body.Str("<p class=\"rfc-prose\"><strong>").Str(html.EscapeString(theme.Label)).
			Str(":</strong> ").Str(rfcProseHTML(strings.TrimSpace(theme.Body), declared)).Str("</p>\n")
	}
	return rfcFoldMarkupHTML(label, prose, strings.TrimSuffix(body.String(), "\n"))
}

// rfcThemesMirror states the same lead and the same themes.
func rfcThemesMirror(label, prose string, declared map[string]bool) string {
	themes := rfcProseThemes(prose)
	if len(themes) < 2 {
		return rfcFoldMarkupMirror(label, prose, rfcProseMirror(prose, declared))
	}
	var body textbuf.Buffer
	for _, theme := range themes {
		if theme.Label == "" {
			body.Str(rfcProseMirror(strings.TrimSpace(theme.Body), declared)).Str("\n\n")
			continue
		}
		body.Str("- **").Str(theme.Label).Str(":** ").
			Str(rfcProseMirror(strings.TrimSpace(theme.Body), declared)).Byte('\n')
	}
	return rfcFoldMarkupMirror(label, prose, strings.TrimSuffix(body.String(), "\n"))
}

// rfcNoPublicRowWhy says where the absence is DECLARED.
//
// The summary declares it. docs/features/rfc-status.md is generated from that
// declaration by `./le rfc index-update`, so the page names the authored fact
// and then the file that carries no row for it, in that order.
func rfcNoPublicRowWhy(entry *rfcLedgerStem) string {
	return ", so its summary declares `| Support | - |` and docs/features/rfc-status.md " +
		"carries no row for " + entry.Display + "."
}

// rfcCoverageProse answers the public ledger's Coverage cell, and says plainly
// when the row states none.
func rfcCoverageProse(entry *rfcLedgerStem) string {
	if strings.TrimSpace(entry.PublicCoverage) == "" {
		return "the public row declares no coverage prose"
	}
	return strings.TrimSpace(entry.PublicCoverage)
}

// rfcRemainingText answers the public ledger's Remaining cell, or says the page
// carries no row for this RFC.
func rfcRemainingText(entry *rfcLedgerStem) string {
	if entry.PublicStatus == "" {
		return rfcNoPublicRow
	}
	if strings.TrimSpace(entry.PublicRemaining) == "" {
		return "the public row declares no remainder"
	}
	return strings.TrimSpace(entry.PublicRemaining)
}

// rfcCoverageBucket is one polarity bucket: what it counts, how many, and the
// requirement ids that produced it.
//
// Ids is empty for a bucket that is not a weakness. Every bucket that IS one
// carries its ids, so no count on this page stands in for a list (AC-18).
type rfcCoverageBucket struct {
	Label string
	Count int
	IDs   []string
	// Partitions says this bucket is one PART of the gated population, so its
	// count belongs in the total. A bucket that is an overlay counts
	// requirements a partitioning bucket already counted, and adding it would
	// make the total exceed the population it is meant to equal.
	Partitions bool
}

// rfcCoverageBuckets answers the six per-RFC counters with the ids behind each
// weakness.
func rfcCoverageBuckets(entry *rfcLedgerStem) []rfcCoverageBucket {
	var both, one, missing, nightly, annotated []string
	for index := range entry.Requirements {
		requirement := &entry.Requirements[index]
		if !requirement.Gated {
			continue
		}
		if requirement.NightlyOnly {
			nightly = append(nightly, requirement.RID)
		}
		switch {
		case requirement.Annotation != nil:
			annotated = append(annotated, requirement.RID)
		case rfcHasBothPolarities(requirement):
			both = append(both, requirement.RID)
		case len(requirement.Covers) != 0:
			one = append(one, requirement.RID)
		default:
			missing = append(missing, requirement.RID)
		}
	}
	return []rfcCoverageBucket{
		{Label: "Positive and negative tests", Count: entry.Coverage.Both, IDs: both,
			Partitions: true},
		{Label: "Annotated instead of tested", Count: entry.Coverage.Annotated, IDs: annotated,
			Partitions: true},
		{Label: "One polarity only", Count: entry.Coverage.One, IDs: one, Partitions: true},
		{Label: "No test and no annotation", Count: entry.Coverage.Missing, IDs: missing,
			Partitions: true},
		{Label: "Evidence that runs nightly only", Count: entry.Coverage.NightlyOnly, IDs: nightly},
	}
}

// rfcCoverageTotal answers what the partitioning buckets add up to.
func rfcCoverageTotal(buckets []rfcCoverageBucket) int {
	total := 0
	for _, bucket := range buckets {
		if bucket.Partitions {
			total += bucket.Count
		}
	}
	return total
}

// rfcCoverageWalkNote says whether the two counts of one bucket agree.
//
// The Count comes from rfc.CoverageRows, which walks the shard. The IDs come
// from the walk rfcCoverageBuckets does over this page's own requirements. Two
// walks over one population can disagree, and a page that printed only one of
// them would publish shares that do not add up with every test green
// (independent review, 2026-09-01). The `Positive and negative tests` bucket
// names its members like the others, so no bucket escapes the comparison.
func rfcCoverageWalkNote(bucket rfcCoverageBucket) string {
	if len(bucket.IDs) == bucket.Count {
		return ""
	}
	return " The gate counts " + strconv.Itoa(bucket.Count) + " and this page names " +
		strconv.Itoa(len(bucket.IDs)) + ", so the two walks disagree."
}

// rfcCoverageAccountedNote says whether the parts add up to the whole.
func rfcCoverageAccountedNote(total, gated int) string {
	if total == gated {
		return "every gated MUST falls in exactly one bucket above"
	}
	return "the buckets account for " + strconv.Itoa(total) + " of " + strconv.Itoa(gated) +
		", so " + strconv.Itoa(gated-total) + " fall in none: the bucketing is incomplete"
}

// rfcCoverageRoleNote says what one bucket is: a part of the population, or an
// overlay counted again inside one of the parts.
func rfcCoverageRoleNote(bucket rfcCoverageBucket) string {
	if bucket.Partitions {
		return "one part of the gated population" + rfcCoverageWalkNote(bucket)
	}
	return "an overlay: each of these is also counted by the part it falls in" +
		rfcCoverageWalkNote(bucket)
}

// rfcHasBothPolarities answers whether a requirement carries a tag in each
// direction.
func rfcHasBothPolarities(requirement *rfcLedgerRequirement) bool {
	var positive, negative bool
	for _, cover := range requirement.Covers {
		if cover.Polarity == rfc.PolarityPositive {
			positive = true
		}
		if cover.Polarity == rfc.PolarityNegative {
			negative = true
		}
	}
	return positive && negative
}

// rfcCoverageHTML renders the polarity buckets as counts, and the membership of
// each weakness as its own labeled list under the table.
//
// The ids are NOT cells. A cell holding 25 of them is a paragraph inside a
// table, and it widens every other column to nothing (owner review,
// 2026-09-01). Each id links to the requirement's own row.
func rfcCoverageHTML(entry *rfcLedgerStem) string {
	if entry.Coverage.Gated == 0 {
		return "<p>" + html.EscapeString(entry.Display+
			" declares no MUST-level requirement, so the gate counts nothing here.") + "</p>"
	}
	buckets := rfcCoverageBuckets(entry)
	var rows textbuf.Buffer
	for _, bucket := range buckets {
		rows.Str(rfcRowCells(html.EscapeString(bucket.Label),
			"<strong>"+strconv.Itoa(bucket.Count)+"</strong>",
			html.EscapeString(rfcCoverageRoleNote(bucket))))
	}
	rows.Str(`<tr class="rfc-total"><td><strong>`).
		Str(html.EscapeString(rfcGatedBucketLabel)).Str(`</strong></td><td><strong>`).
		Int(int64(entry.Coverage.Gated)).Str("</strong></td><td>").
		Str(html.EscapeString(rfcCoverageAccountedNote(rfcCoverageTotal(buckets),
			entry.Coverage.Gated))).Str("</td></tr>\n")
	var out textbuf.Buffer
	out.Str(rfcTableHTML(rfcHeadCells("Bucket", "Count", "What it counts"),
		rows.String()))
	for _, bucket := range buckets {
		if len(bucket.IDs) == 0 {
			continue
		}
		out.Str("\n<p class=\"rfc-id-list\"><strong>").Str(html.EscapeString(bucket.Label)).
			Str(" (").Int(int64(len(bucket.IDs))).Str("):</strong> ").
			Str(rfcIDLinksHTML(bucket.IDs)).Str("</p>")
	}
	return out.String()
}

// rfcCoverageMirror states the same counts and the same membership.
func rfcCoverageMirror(entry *rfcLedgerStem) string {
	if entry.Coverage.Gated == 0 {
		return entry.Display + " declares no MUST-level requirement, so the gate counts " +
			"nothing here.\n"
	}
	buckets := rfcCoverageBuckets(entry)
	var out textbuf.Buffer
	out.Str(rfcMirrorHead("Bucket", "Count", "What it counts"))
	for _, bucket := range buckets {
		out.Str(rfcMirrorRow(bucket.Label, strconv.Itoa(bucket.Count),
			rfc.TableCell(rfcCoverageRoleNote(bucket))))
	}
	out.Str(rfcMirrorRow("**"+rfcGatedBucketLabel+"**",
		"**"+strconv.Itoa(entry.Coverage.Gated)+"**",
		rfc.TableCell(rfcCoverageAccountedNote(rfcCoverageTotal(buckets),
			entry.Coverage.Gated))))
	for _, bucket := range buckets {
		if len(bucket.IDs) == 0 {
			continue
		}
		out.Str("\n**").Str(bucket.Label).Str(" (").Int(int64(len(bucket.IDs))).Str("):** ").
			Str(rfcIDLinksMirror(bucket.IDs)).Byte('\n')
	}
	return out.String()
}

// rfcIDLinksHTML renders a list of requirement ids, each linked to its own row.
func rfcIDLinksHTML(ids []string) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, rfcRequirementRefHTML(id, ""))
	}
	return strings.Join(parts, ", ")
}

// rfcIDLinksMirror states the same list.
func rfcIDLinksMirror(ids []string) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, rfcRequirementRefMirror(id, ""))
	}
	return strings.Join(parts, ", ")
}

// rfcSectionNames maps a section id to the section's own name, where the
// extraction sign-off states one.
//
// A requirement row that says "Section 3" names a place a reader has to go and
// look up. The sign-off already carries "Constructing the Next Hop field", so
// the row carries it too (owner review, 2026-09-01).
func rfcSectionNames(entry *rfcLedgerStem) map[string]string {
	if entry.Extraction == nil {
		return nil
	}
	names := make(map[string]string, len(entry.Extraction.Sections))
	for _, section := range entry.Extraction.Sections {
		if section.Name == "" {
			continue
		}
		names[section.ID] = section.Name
	}
	return names
}

// rfcSectionText answers one requirement's section, with the section's own name
// where the sign-off states one.
func rfcSectionText(section string, names map[string]string) string {
	if names[section] == "" {
		return section
	}
	return section + " - " + names[section]
}

// rfcRequirementColumns are this table's columns, declared once.
//
// The colspan of the subject row is derived from this rather than written
// beside it: the table has changed shape three times, and a number repeated is
// a number that goes wrong the next time.
var rfcRequirementColumns = []string{"Requirement", "Level", "Section", "Tests"}

// rfcRequirementsHTML renders two rows per requirement: the SUBJECT, then its
// attributes.
//
// The requirement's own sentence is what the row is ABOUT and everything else
// is metadata on it, so it leads, always visible, spanning the table (owner
// review, 2026-09-01). It was behind a disclosure in the narrowest column
// before that, which is inside out: the subject hidden and its attributes on
// show.
//
// A TABLE rather than a block per requirement, and deliberately: the Proof
// state section below already renders one block per requirement, so blocks here
// would give a reader two lists of the same shape on one page. Level and
// section stay aligned down the column, which is what makes 228 rows scannable.
//
// There is no Note column. Its four marks each went where they explain
// something, and none was dropped -- see rfcRequirementTestsHTML.
func rfcRequirementsHTML(entry *rfcLedgerStem) string {
	if len(entry.Requirements) == 0 {
		return "<p>" + html.EscapeString(entry.Display+
			" declares no requirement, so this summary generates no shard.") + "</p>"
	}
	names := rfcSectionNames(entry)
	ambiguous := rfcAmbiguousNames(entry)
	var rows textbuf.Buffer
	for index := range entry.Requirements {
		requirement := &entry.Requirements[index]
		rows.Str(rfcSubjectRow(rfcRequirementColumns,
			`<code id="`+html.EscapeString(rfcAnchor(requirement.RID))+`">`+
				html.EscapeString(requirement.RID)+"</code>",
			rfcRequirementSubjectHTML(requirement)))
		// The metadata row continues the subject above it, so its identity cell
		// is empty rather than repeating the id a reader just read.
		rows.Str(rfcRowCells("",
			html.EscapeString(requirement.Level),
			html.EscapeString(rfcSectionText(requirement.Section, names)),
			rfcRequirementTestsHTML(requirement, ambiguous)))
	}
	return rfcTableHTML(rfcHeadCells(rfcRequirementColumns...), rows.String())
}

// rfcRequirementSubjectHTML renders the sentence that sits BESIDE the id.
//
// Beside rather than beneath: the id is its own narrow cell, so the eye reads
// id then sentence across, and every sentence on the page starts at the same
// offset instead of after an id of its own width (owner review, 2026-09-01).
func rfcRequirementSubjectHTML(requirement *rfcLedgerRequirement) string {
	if requirement.Text == "" {
		return ""
	}
	return `<span class="rfc-subject">` + html.EscapeString(requirement.Text) + "</span>"
}

// rfcRequirementTestsHTML renders one requirement's tests as a GRID: one row
// per citation, three fields each, plus a row for a polarity with no test.
//
// Divs rather than a nested table, by owner amendment of 2026-09-01. A table
// inside a cell inherits the outer table's width pressure, which is the problem
// the restructure exists to remove; a grid gets the alignment without it. The
// two short fields have fixed widths so the tier column lines up down the page
// and the linked name takes whatever is left.
//
// The TIER LEADS. `TestRFC4271PartialBitPreservedOnUnknownTransitive
// (unit/verify)` wrapped the qualifier away from the name it qualifies, and a
// short fixed-width token in front of a long variable-width one wraps cleanly.
//
// The roles make it data rather than a run of text. There is no header row: the
// three values say what they are -- a polarity word, a `kind/tier`, a test name
// -- and 228 hidden header rows on one page would be read out 228 times for
// nothing.
//
// The Note column is gone and this is where two of its four marks landed. The
// `{kind} reason` annotation sits under the rows, because a reader who has just
// read "no negative test" is already asking why; it was three cells to the
// right. The nightly-only mark sits with the tiers that make it nightly.
//
// The other two marks were duplicates and are not re-rendered: the audit
// verdict is named with its meaning and freshness under Proof state, and the
// superseded pointer under Superseded, both per requirement id. Measured over
// this corpus before the column went: all 4 audit marks and all 310 superseded
// marks have those homes, while 374 of 375 `{single-polarity}` reasons and 14
// of 851 `{not-applicable}` reasons had NO other home, which is why they are
// rendered here rather than assumed to be elsewhere.
func rfcRequirementTestsHTML(requirement *rfcLedgerRequirement,
	ambiguous map[string]bool,
) string {
	var out textbuf.Buffer
	out.Str(`<div class="rfc-tests" role="table" aria-label="`).
		Str(html.EscapeString("tests bound to " + requirement.RID)).Str(`">`).Byte('\n')
	for _, row := range rfcTestRows(requirement) {
		test := html.EscapeString(rfcNoPolarityText(row.Polarity))
		if row.Cover != nil {
			test = rfcCitationHTML(row.Cover, ambiguous)
		}
		out.Str(rfcTestsRowHTML(row.Polarity, row.Carrier, test))
	}
	out.Str("</div>")
	for _, mark := range rfcRequirementMarks(requirement) {
		out.Str(`<p class="rfc-mark"><strong>`).Str(html.EscapeString(mark[0])).
			Str(":</strong> ").Str(html.EscapeString(mark[1])).Str("</p>")
	}
	return out.String()
}

// rfcTestsRowHTML answers one row of that grid: the polarity, the kind and tier,
// and the test.
//
// A row stating an absent polarity carries no tier, and says so rather than
// leaving the cell blank: a blank reads as a rendering fault and an absent
// polarity is a disclosed fact (ai/rules/principles.md).
func rfcTestsRowHTML(polarity, carrier, test string) string {
	return `<div class="rfc-tests-row" role="row"><span role="cell">` +
		html.EscapeString(polarity) + `</span><span role="cell"><code>` +
		html.EscapeString(carrier) + `</code></span><span role="cell">` + test +
		"</span></div>\n"
}

// rfcRequirementMarks answers the marks that explain this requirement's tests:
// the annotation that says why a polarity is absent, and the nightly-only mark.
//
// One list, read by the page and by the mirror, so the two cannot state
// different reasons for one requirement.
func rfcRequirementMarks(requirement *rfcLedgerRequirement) [][2]string {
	var marks [][2]string
	if requirement.Annotation != nil {
		marks = append(marks, [2]string{"{" + requirement.Annotation.Kind + "}",
			requirement.Annotation.Reason})
	}
	if requirement.NightlyOnly {
		marks = append(marks, [2]string{"nightly-only",
			"every test bound to this requirement runs in the scheduled workflow alone, so " +
				"nothing here is proven on the merge path"})
	}
	return marks
}

// rfcRequirementTestsMirror states both polarities and the same marks. A
// Markdown cell holds no line break, so the lines are separated by a stop
// rather than by a newline.
func rfcRequirementTestsMirror(requirement *rfcLedgerRequirement,
	ambiguous map[string]bool,
) string {
	rows := rfcTestRows(requirement)
	parts := make([]string, 0, len(rows)+2)
	for index := range rows {
		row := &rows[index]
		if row.Cover == nil {
			parts = append(parts, "**"+row.Polarity+":** "+rfcNoPolarityText(row.Polarity))
			continue
		}
		parts = append(parts, "**"+row.Polarity+":** `"+row.Carrier+"` "+
			rfcCitationMirror(row.Cover, ambiguous))
	}
	for _, mark := range rfcRequirementMarks(requirement) {
		parts = append(parts, "**"+mark[0]+":** "+mark[1])
	}
	return strings.Join(parts, ". ")
}

// rfcHasPolarity answers whether any tagged unit proves this requirement in one
// direction.
func rfcHasPolarity(requirement *rfcLedgerRequirement, polarity string) bool {
	for index := range requirement.Covers {
		if requirement.Covers[index].Polarity == polarity {
			return true
		}
	}
	return false
}

// rfcNoPolarityText is what an absent polarity says.
func rfcNoPolarityText(polarity string) string { return "no " + polarity + " test" }

// rfcRequirementsMirror states the same rows.
func rfcRequirementsMirror(entry *rfcLedgerStem) string {
	if len(entry.Requirements) == 0 {
		return entry.Display + " declares no requirement, so this summary generates no shard.\n"
	}
	names := rfcSectionNames(entry)
	ambiguous := rfcAmbiguousNames(entry)
	var out textbuf.Buffer
	// The subject leads here too, and there is no Note column: its four marks
	// went where they explain something, exactly as on the page.
	out.Str(rfcMirrorHead("Requirement", "Text", "Level", "Section", "Tests"))
	for index := range entry.Requirements {
		requirement := &entry.Requirements[index]
		out.Str(rfcMirrorRow("`"+requirement.RID+"`",
			rfc.TableCell(requirement.Text), requirement.Level,
			rfc.TableCell(rfcSectionText(requirement.Section, names)),
			rfc.TableCell(rfcRequirementTestsMirror(requirement, ambiguous))))
	}
	return out.String()
}
