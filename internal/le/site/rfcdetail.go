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
	var out strings.Builder
	out.WriteString(entry.Display)
	if entry.Title != "" {
		out.WriteString(": " + entry.Title)
	}
	out.WriteString(". " + strconv.Itoa(entry.Coverage.Gated) +
		" gated MUST-level requirements, " + strconv.Itoa(entry.Coverage.Gaps) +
		" declared gaps, " + strconv.Itoa(entry.Coverage.Missing) + " with no test.")
	return out.String()
}

// rfcDetailBody renders one summary's page under <main>.
func rfcDetailBody(entry *rfcLedgerStem) string {
	var body strings.Builder
	body.WriteString(`            <section aria-labeledby="rfc-detail-title" class="md-content reveal cat-observe">` + "\n")
	body.WriteString(pageHero(html.EscapeString(rfcDetailHeading(entry)),
		html.EscapeString(rfcDetailLead(entry)), rfcDetailEyebrow(entry),
		` id="rfc-detail-title"`, heroClasses) + "\n")
	body.WriteString(rfcComplianceStyle)
	for _, section := range rfcDetailSections {
		body.WriteString("<section><h2>" + html.EscapeString(section.Heading) + "</h2>\n" +
			section.HTML(entry) + "\n</section>\n")
	}
	body.WriteString("            </section>\n")
	return body.String()
}

// rfcDetailMirror renders one summary's Markdown sibling, which states the same
// facts as the page in the order the page states them.
func rfcDetailMirror(entry *rfcLedgerStem) string {
	var out strings.Builder
	out.WriteString("# " + rfcDetailHeading(entry) + "\n\n")
	out.WriteString(rfcDetailEyebrow(entry) + ". " + rfcDetailLead(entry) + "\n")
	for _, section := range rfcDetailSections {
		out.WriteString("\n## " + section.Heading + "\n\n" + section.Mirror(entry))
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
// shares that partition what does. The shares are built by rfcStandingCards, so
// this page and the index cannot publish different partitions of one idea.
func rfcDetailCards(entry *rfcLedgerStem) []rfcCard {
	coverage := &entry.Coverage
	proof := rfcProofCountsOf(entry)
	unsoundWords := rfc.VerdictWeak + ", " + rfc.VerdictWrong + " or " + rfc.VerdictUnimplemented
	cards := []rfcCard{
		{Label: "Gated MUSTs", Value: groupThousands(coverage.Gated),
			Count: "of " + groupThousands(coverage.Requirements) + " this summary declares",
			Note: "MUST-level requirements the gate HOLDS. A population, not a result: the " +
				"shares beside it are what says how Ze stands",
			Tone: rfcToneNeutral,
			Rule: "no color: a population is a scale, and a larger one is neither good news " +
				"nor bad. It is the accounting total"},
		{Label: "Out of scope", Value: groupThousands(coverage.NotApplicable),
			Count: "of " + groupThousands(coverage.Gated) + " gated MUSTs",
			Note: "a {not-applicable} annotation says the obligation does not bind Ze. Scope, " +
				"not coverage: it is in no share below",
			Tone: rfcToneNeutral,
			Rule: "no color: an obligation that never bound Ze is neither an achievement nor a " +
				"failure, and counting it either way would be a claim"},
	}
	cards = append(cards, rfcStandingCards(coverage.Binding(), coverage.Bucket)...)
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
	return rfcCardsHTML(cards, entry.Coverage.Binding()) + rfcToneLegendHTML(cards)
}

// rfcOverviewMirror states the same cards as a table.
func rfcOverviewMirror(entry *rfcLedgerStem) string {
	cards := rfcDetailCards(entry)
	return rfcCardsMirror(cards, entry.Coverage.Binding()) + "\n" + rfcToneLegendMirror(cards)
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
		{"Obligations that bind Ze", strconv.Itoa(entry.Coverage.Binding())},
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
	var rows strings.Builder
	for _, fact := range rfcGlanceFacts(entry) {
		rows.WriteString(rfcRowCells(html.EscapeString(fact[0]), fact[1]))
	}
	return rfcTableHTML(rfcHeadCells("Field", "Value"), rows.String())
}

// rfcGlanceMirror states the same facts.
func rfcGlanceMirror(entry *rfcLedgerStem) string {
	var out strings.Builder
	out.WriteString(rfcMirrorHead("Field", "Value"))
	for _, fact := range rfcGlanceFacts(entry) {
		out.WriteString(rfcMirrorRow(fact[0], rfc.TableCell(rfcPlain(fact[1]))))
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
	return "Not enrolled (" + entry.Disposition.Kind + "): " + entry.Disposition.Reason
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
	var body strings.Builder
	body.WriteString("<ul class=\"rfc-prose\">\n")
	for _, item := range items {
		body.WriteString("<li>" + rfcProseHTML(strings.TrimSpace(item), declared) + "</li>\n")
	}
	body.WriteString("</ul>")
	return rfcFoldMarkupHTML(label, prose, body.String())
}

// rfcClaimsMirror states the same claims.
func rfcClaimsMirror(label, prose string, declared map[string]bool) string {
	items := rfcProseSplit(prose)
	if len(items) < 2 {
		return rfcFoldMarkupMirror(label, prose, rfcProseMirror(prose, declared))
	}
	var body strings.Builder
	for _, item := range items {
		body.WriteString("- " + rfcProseMirror(strings.TrimSpace(item), declared) + "\n")
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
	var body strings.Builder
	for _, theme := range themes {
		if theme.Label == "" {
			body.WriteString("<p>" + rfcProseHTML(strings.TrimSpace(theme.Body), declared) +
				"</p>\n")
			continue
		}
		body.WriteString("<p class=\"rfc-prose\"><strong>" + html.EscapeString(theme.Label) +
			":</strong> " + rfcProseHTML(strings.TrimSpace(theme.Body), declared) + "</p>\n")
	}
	return rfcFoldMarkupHTML(label, prose, strings.TrimSuffix(body.String(), "\n"))
}

// rfcThemesMirror states the same lead and the same themes.
func rfcThemesMirror(label, prose string, declared map[string]bool) string {
	themes := rfcProseThemes(prose)
	if len(themes) < 2 {
		return rfcFoldMarkupMirror(label, prose, rfcProseMirror(prose, declared))
	}
	var body strings.Builder
	for _, theme := range themes {
		if theme.Label == "" {
			body.WriteString(rfcProseMirror(strings.TrimSpace(theme.Body), declared) + "\n\n")
			continue
		}
		body.WriteString("- **" + theme.Label + ":** " +
			rfcProseMirror(strings.TrimSpace(theme.Body), declared) + "\n")
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
}

// rfcCoverageBuckets answers the six per-RFC counters with the ids behind each
// weakness.
func rfcCoverageBuckets(entry *rfcLedgerStem) []rfcCoverageBucket {
	var one, missing, nightly, annotated []string
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
		case len(requirement.Covers) != 0:
			one = append(one, requirement.RID)
		default:
			missing = append(missing, requirement.RID)
		}
	}
	return []rfcCoverageBucket{
		{Label: "Positive and negative tests", Count: entry.Coverage.Both},
		{Label: "Annotated instead of tested", Count: entry.Coverage.Annotated, IDs: annotated},
		{Label: "One polarity only", Count: entry.Coverage.One, IDs: one},
		{Label: "No test and no annotation", Count: entry.Coverage.Missing, IDs: missing},
		{Label: "Evidence that runs nightly only", Count: entry.Coverage.NightlyOnly, IDs: nightly},
	}
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
	var rows strings.Builder
	for _, bucket := range buckets {
		rows.WriteString(rfcRowCells(html.EscapeString(bucket.Label),
			"<strong>"+strconv.Itoa(bucket.Count)+"</strong>"))
	}
	var out strings.Builder
	out.WriteString(rfcTableHTML(rfcHeadCells("Bucket", "Count"), rows.String()))
	for _, bucket := range buckets {
		if len(bucket.IDs) == 0 {
			continue
		}
		out.WriteString("\n<p class=\"rfc-id-list\"><strong>" + html.EscapeString(bucket.Label) +
			" (" + strconv.Itoa(len(bucket.IDs)) + "):</strong> " +
			rfcIDLinksHTML(bucket.IDs) + "</p>")
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
	var out strings.Builder
	out.WriteString(rfcMirrorHead("Bucket", "Count"))
	for _, bucket := range buckets {
		out.WriteString(rfcMirrorRow(bucket.Label, strconv.Itoa(bucket.Count)))
	}
	for _, bucket := range buckets {
		if len(bucket.IDs) == 0 {
			continue
		}
		out.WriteString("\n**" + bucket.Label + " (" + strconv.Itoa(len(bucket.IDs)) + "):** " +
			rfcIDLinksMirror(bucket.IDs) + "\n")
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
// The colspan of the text row is derived from this rather than written beside
// it: the table gained a Tests column and lost two on 2026-09-01, and a number
// repeated is a number that goes wrong the next time.
var rfcRequirementColumns = []string{"Requirement", "Level", "Section", "Tests", "Note"}

// rfcRequirementsHTML renders one row per requirement, carrying the cells
// rfc/requirements/<stem>.md carries.
//
// Two things about width, both from the owner's review of 2026-09-01.
//
// The two polarities share ONE column, a labeled line each, so each list has
// the whole column rather than half of it. A polarity with no test SAYS so: "no
// negative test" is a disclosed fact under the disclosure ruling, and an empty
// cell is what a reader skims past.
//
// The requirement's own TEXT is a SECOND ROW spanning the table, not a
// disclosure inside the id cell. Inside the cell it expanded within the
// narrowest column on the page, which is where the sentence was unreadable. It
// is still a `details`, so it is reachable by keyboard and by touch, and it is
// still in the markup for a reader searching the page.
func rfcRequirementsHTML(entry *rfcLedgerStem) string {
	if len(entry.Requirements) == 0 {
		return "<p>" + html.EscapeString(entry.Display+
			" declares no requirement, so this summary generates no shard.") + "</p>"
	}
	names := rfcSectionNames(entry)
	ambiguous := rfcAmbiguousNames(entry)
	var rows strings.Builder
	for index := range entry.Requirements {
		requirement := &entry.Requirements[index]
		rows.WriteString(rfcRowCells(
			`<code id="`+html.EscapeString(rfcAnchor(requirement.RID))+`">`+
				html.EscapeString(requirement.RID)+"</code>",
			html.EscapeString(requirement.Level),
			html.EscapeString(rfcSectionText(requirement.Section, names)),
			rfcRequirementTestsHTML(requirement, ambiguous),
			rfcInlineHTML(requirement.Note)))
		if requirement.Text == "" {
			continue
		}
		rows.WriteString(rfcSpanRow(rfcRequirementColumns,
			rfcRequirementTextHTML(requirement.Text)))
	}
	return rfcTableHTML(rfcHeadCells(rfcRequirementColumns...), rows.String())
}

// rfcRequirementTestsHTML renders both polarities of one requirement, a
// labeled line each.
//
// The absence of one polarity is STATED. A requirement tested one way and not
// the other is exactly what the gate exists to surface, and a blank half-cell
// says it in a way nobody reads.
func rfcRequirementTestsHTML(requirement *rfcLedgerRequirement,
	ambiguous map[string]bool,
) string {
	var out strings.Builder
	for _, polarity := range []string{rfc.PolarityPositive, rfc.PolarityNegative} {
		if out.Len() > 0 {
			out.WriteString("<br />")
		}
		out.WriteString(`<span class="rfc-polarity">` + html.EscapeString(polarity) +
			":</span> ")
		if !rfcHasPolarity(requirement, polarity) {
			out.WriteString(html.EscapeString(rfcNoPolarityText(polarity)))
			continue
		}
		out.WriteString(rfcCitationsHTML(requirement, polarity, ambiguous))
	}
	return out.String()
}

// rfcRequirementTestsMirror states both polarities, labeled, in one cell. A
// Markdown cell holds no line break, so the two are separated by a marker
// rather than by a newline.
func rfcRequirementTestsMirror(requirement *rfcLedgerRequirement,
	ambiguous map[string]bool,
) string {
	parts := make([]string, 0, 2)
	for _, polarity := range []string{rfc.PolarityPositive, rfc.PolarityNegative} {
		if !rfcHasPolarity(requirement, polarity) {
			parts = append(parts, "**"+polarity+":** "+rfcNoPolarityText(polarity))
			continue
		}
		parts = append(parts, "**"+polarity+":** "+
			rfcCitationsMirror(requirement, polarity, ambiguous))
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

// rfcRequirementTextHTML puts one requirement's sentence behind a disclosure on
// a row of its own.
//
// The disclosure sits INSIDE the spanning cell rather than in the row above it.
// A `details` cannot contain a `tr`, and driving a row from a control in the
// previous row needs either script or a checkbox hack; both cost more
// accessibility than they buy. What the owner asked for is that the sentence
// span the table when it opens, and it does: the row is the full width, so both
// the control and the text it reveals are.
func rfcRequirementTextHTML(text string) string {
	if text == "" {
		return ""
	}
	return "<details class=\"rfc-text\"><summary>requirement text</summary>\n<p>" +
		html.EscapeString(text) + "</p>\n</details>"
}

// rfcRequirementsMirror states the same rows.
func rfcRequirementsMirror(entry *rfcLedgerStem) string {
	if len(entry.Requirements) == 0 {
		return entry.Display + " declares no requirement, so this summary generates no shard.\n"
	}
	names := rfcSectionNames(entry)
	ambiguous := rfcAmbiguousNames(entry)
	var out strings.Builder
	out.WriteString(rfcMirrorHead("Requirement", "Level", "Section", "Text", "Tests", "Note"))
	for index := range entry.Requirements {
		requirement := &entry.Requirements[index]
		out.WriteString(rfcMirrorRow("`"+requirement.RID+"`", requirement.Level,
			rfc.TableCell(rfcSectionText(requirement.Section, names)),
			rfc.TableCell(requirement.Text),
			rfc.TableCell(rfcRequirementTestsMirror(requirement, ambiguous)),
			rfc.TableCell(requirement.Note)))
	}
	return out.String()
}
