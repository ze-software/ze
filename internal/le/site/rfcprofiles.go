// Design: docs/contributing/gh-pages.md -- the RFC profile pages, one per use case and per technology aggregate
// Related: rfcledger.go -- the ledger's profiles, the trees rfc.PriorityOf answers
// Related: rfccompliance.go -- the all-RFCs index these pages link back to
package site

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/rfc"
)

// Where the RFC profile pages are published: one index, and one page per
// profile file under it.
const (
	rfcProfilesProducerName = "rfc-profiles"
	rfcProfilesDirectory    = rfcComplianceDirectory + "/profiles"
	rfcProfilesRoute        = "/" + rfcProfilesDirectory + "/"
	rfcProfilesDest         = rfcProfilesDirectory + "/" + pageIndexFile
	rfcProfilesRoot         = "../../../"
	rfcProfileRoot          = "../../../../"
	// rfcProfileMarker is what a page of THIS family carries and no other
	// page does. removeRetiredRFCProfilePages keys its deletion on it.
	rfcProfileMarker = ` id="rfc-profile-title"`
	// rfcProfilesMarker is the index's own heading id.
	rfcProfilesMarker = ` id="rfc-profiles-title"`
	// rfcProfileMandatoryLabel and rfcProfileOptionalLabel name the two
	// coverage figures, on the cards and in every table column that carries
	// one, so a card and its column cannot be named apart.
	rfcProfileMandatoryLabel = "Mandatory"
	rfcProfileOptionalLabel  = "Optional"
)

func init() {
	registerProducer(Producer{Name: rfcProfilesProducerName, Render: renderRFCProfiles})
}

// renderRFCProfiles publishes the profile index and one page per profile from
// the published ledger, and answers every route it wrote.
//
// A ledger with no profile is refused rather than rendered: an index that
// links nothing passes every check the artifact carries, which is the silent
// hole the producer registry exists to make impossible.
func renderRFCProfiles(paths Paths) ([]string, error) {
	ledger, err := loadRequirementLedger(paths.Output)
	if err != nil {
		return nil, err
	}
	if len(ledger.Profiles) == 0 {
		return nil, fmt.Errorf("the published %s names no RFC profile, so no profile page can be "+
			"built; every file under rfc/profiles/ is one, and ./le rfc check names one that does not parse",
			rfcLedgerFile)
	}
	links, err := loadPageLinks(paths.Source)
	if err != nil {
		return nil, err
	}
	profiles := rfcProfilesOrdered(ledger.Profiles)
	display := make(map[string]string, len(ledger.Stems))
	for index := range ledger.Stems {
		display[ledger.Stems[index].Stem] = ledger.Stems[index].Display
	}

	shell := pageShell{
		Title:       "RFC coverage by profile - Ze",
		Description: "Mandatory and optional RFC requirement coverage of each use case Ze serves and each technology it implements, release profiles first.",
		Root:        rfcProfilesRoot,
		Path:        rfcProfilesDest,
		Sidebar:     pageSidebar(rfcProfilesRoot, rfcProfilesDest, links),
	}
	if err := writePublishedPage(paths.Output, rfcProfilesDest,
		shell.render(rfcProfilesIndexBody(profiles)), rfcProfilesIndexMirror(profiles)); err != nil {
		return nil, err
	}

	routes := make([]string, 0, len(profiles)+1)
	routes = append(routes, rfcProfilesRoute)
	written := make(map[string]bool, len(profiles))
	for _, report := range profiles {
		if report.Profile == "" {
			return nil, fmt.Errorf("the published %s carries a profile tree with no profile name", rfcLedgerFile)
		}
		route, err := writeRFCProfilePage(paths.Output, report, display, links)
		if err != nil {
			return nil, err
		}
		written[report.Profile] = true
		routes = append(routes, route)
	}
	// After the writing, so the live set is what this run PUT there.
	if err := removeRetiredRFCProfilePages(paths.Output, written); err != nil {
		return nil, err
	}
	return routes, nil
}

// rfcProfileGroup is one heading of the index: its label and the predicate
// that places a profile under it. Every profile meets exactly one.
type rfcProfileGroup struct {
	Label string
	Holds func(*rfc.TreeReport) bool
}

// rfcProfileGroups orders the index: the release profiles the owner rule
// judges come first, then the other use cases, then the aggregates.
var rfcProfileGroups = []rfcProfileGroup{
	{"Release profiles", func(r *rfc.TreeReport) bool { return !r.IsAggregate() && r.Release }},
	{"Other use-case profiles", func(r *rfc.TreeReport) bool { return !r.IsAggregate() && !r.Release }},
	{"Technology aggregates", func(r *rfc.TreeReport) bool { return r.IsAggregate() }},
}

// rfcProfilesOrdered answers the profiles in index order, each group in the
// order the ledger lists it (by name).
func rfcProfilesOrdered(profiles []*rfc.TreeReport) []*rfc.TreeReport {
	out := make([]*rfc.TreeReport, 0, len(profiles))
	for _, group := range rfcProfileGroups {
		for _, report := range profiles {
			if group.Holds(report) {
				out = append(out, report)
			}
		}
	}
	return out
}

// rfcProfileKindLabel is the eyebrow of one profile's page.
func rfcProfileKindLabel(report *rfc.TreeReport) string {
	if report.IsAggregate() {
		return "Technology aggregate"
	}
	if report.Release {
		return "Release profile"
	}
	return "Use-case profile"
}

// rfcProfileLead says what the profile's set of RFCs is and, for a release
// profile, what the owner rule asks of it.
func rfcProfileLead(report *rfc.TreeReport) string {
	if report.IsAggregate() {
		return report.Heading + ". The RFCs whose summary names the " + report.Profile +
			" aggregate, with their mandatory and optional coverage."
	}
	lead := "The root RFCs of this use case, every RFC they build on, and every RFC that updates one of them."
	if report.Release {
		lead += " Before release, every mandatory requirement of every RFC listed here must be " +
			"proven by a recorded break of its test."
	}
	return lead
}

// rfcProfileTitle is the page heading: the profile's Title, or its name for
// a file that states none.
func rfcProfileTitle(report *rfc.TreeReport) string {
	if report.Title == "" {
		return report.Profile
	}
	return report.Title
}

// writeRFCProfilePage publishes one profile's page and its mirror, and
// answers the route it wrote.
func writeRFCProfilePage(output string, report *rfc.TreeReport, display map[string]string,
	links pageLinks) (string, error) {
	route := rfcProfilesRoute + report.Profile + "/"
	destination := rfcProfilesDirectory + "/" + report.Profile + "/" + pageIndexFile
	shell := pageShell{
		Title:       rfcProfileTitle(report) + " RFC coverage - Ze",
		Description: rfcProfileKindLabel(report) + " " + report.Profile + ": " + report.Rollup.String() + ".",
		Root:        rfcProfileRoot,
		Path:        destination,
		Sidebar:     pageSidebar(rfcProfileRoot, route[1:], links),
	}
	if err := writePublishedPage(output, destination,
		shell.render(rfcProfileBody(report, display)), rfcProfileMirror(report, display)); err != nil {
		return "", err
	}
	return route, nil
}

// rfcProfileCard is one card of a profile's grid: its label, its value, the
// arithmetic under it, and the sentence saying what it measures.
type rfcProfileCard struct {
	Label, Value, Arithmetic, Meaning string
}

// rfcProfileCards answers the grid: both figures first, then the counts of
// members that add no rows, which say how far the figures can be trusted.
// The keywords each figure counts are the rfc package's own sets, so the
// card cannot name a keyword the figure does not count.
func rfcProfileCards(rollup *rfc.ScopeRollup) []rfcProfileCard {
	return []rfcProfileCard{
		{rfcProfileMandatoryLabel, rfcProfilePercent(rollup.Mandatory), rollup.Mandatory.String(),
			rfcKeywordSeries(rfc.MandatoryKeywords()) + " rows of the walked RFCs that a recorded test break proves."},
		{rfcProfileOptionalLabel, rfcProfilePercent(rollup.Optional), rollup.Optional.String(),
			rfcKeywordSeries(rfc.OptionalKeywords()) + " rows, counted the same way."},
		{"Not walked", strconv.Itoa(rollup.NotWalked), "of " + strconv.Itoa(rollup.RFCs) + " RFCs",
			"RFCs with no valid extraction sign-off or no summary: their rows count in neither figure."},
		{"Absent", strconv.Itoa(rollup.Absent), "of " + strconv.Itoa(rollup.RFCs) + " RFCs",
			"RFCs the profile needs that the corpus holds no summary of, each at 0%."},
		{"Mandatory register", rfcProfileComplete(rollup), "every row proven, none unwalked or absent",
			"A profile is complete only when nothing it reaches is unwalked or absent."},
	}
}

// rfcProfileComplete answers the mandatory register's state in words, the
// cell the card and the index both print.
func rfcProfileComplete(rollup *rfc.ScopeRollup) string {
	if rollup.MandatoryComplete {
		return "complete"
	}
	return "not complete"
}

// rfcKeywordSeries joins keywords as prose: `A, B and C`.
func rfcKeywordSeries(keywords []string) string {
	if len(keywords) < 2 {
		return strings.Join(keywords, "")
	}
	return strings.Join(keywords[:len(keywords)-1], ", ") + " and " + keywords[len(keywords)-1]
}

// rfcProfilePercent answers a figure's share, and says so when there are no rows:
// 0 of 0 is neither 0% nor 100%.
func rfcProfilePercent(figure rfc.Figure) string {
	if figure.Total == 0 {
		return "no rows"
	}
	return rfcPercentText(figure.Proven, figure.Total)
}

// rfcProfileBody renders one profile's page.
func rfcProfileBody(report *rfc.TreeReport, display map[string]string) string {
	var body textbuf.Buffer
	body.Str(`            <section aria-labeledby="rfc-profile-title" class="md-content reveal cat-observe">`).Byte('\n')
	body.Str(pageHero(html.EscapeString(rfcProfileTitle(report)), html.EscapeString(rfcProfileLead(report)),
		rfcProfileKindLabel(report), rfcProfileMarker, heroClasses)).Byte('\n')
	body.Str(rfcComplianceStyle).Str(rfcProfileStyle)
	body.Str(`<p><a href="../">All profiles</a> · <a href="../../">All RFCs</a></p>`).Byte('\n')
	body.Str(`<div class="rfc-card-grid">`).Byte('\n')
	for _, card := range rfcProfileCards(report.Rollup) {
		body.Str(`<div class="rfc-card rfc-neutral"><span>`).Str(html.EscapeString(card.Label)).
			Str("</span><strong>").Str(html.EscapeString(card.Value)).Str("</strong><b>").
			Str(html.EscapeString(card.Arithmetic)).Str("</b><p>").Str(html.EscapeString(card.Meaning)).
			Str("</p></div>\n")
	}
	body.Str("</div>\n")
	body.Str("<section><h2>Tree</h2>\n<p>").Str(html.EscapeString(rfcProfileTreeProse(report))).Str("</p>\n")
	for _, level := range rfcProfileLevels(report.Rows) {
		body.Str("<h3>").Str(html.EscapeString(level.Label)).Str("</h3>\n")
		var rows textbuf.Buffer
		for _, row := range level.Rows {
			mandatory, optional := row.Cells()
			cells := []string{rfcProfileStemHTML(row, display), html.EscapeString(row.Title),
				rfcProfileBarHTML(row, mandatory, row.MandatoryProven, row.MandatoryTotal),
				rfcProfileBarHTML(row, optional, row.OptionalProven, row.OptionalTotal)}
			if !report.IsAggregate() {
				cells = append(cells, html.EscapeString(row.Path))
			}
			rows.Str(rfcRowCells(cells...))
		}
		body.Str(rfcTableHTML(rfcHeadCells(rfcProfileColumns(report)...), rows.String())).Byte('\n')
	}
	body.Str("</section>\n            </section>\n")
	return body.String()
}

// rfcProfileMirror states the same page in Markdown, in the page's order.
func rfcProfileMirror(report *rfc.TreeReport, display map[string]string) string {
	var out textbuf.Buffer
	out.Str("# ").Str(rfcProfileTitle(report)).Str("\n\n")
	out.Str(rfcProfileKindLabel(report)).Str(". ").Str(rfcProfileLead(report)).Str("\n\n")
	out.Str("[All profiles](../) · [All RFCs](../../)\n\n")
	for _, card := range rfcProfileCards(report.Rollup) {
		out.Str("- **").Str(card.Label).Str("**: ").Str(card.Value).Str(" (").Str(card.Arithmetic).
			Str("). ").Str(card.Meaning).Byte('\n')
	}
	out.Str("\n## Tree\n\n").Str(rfcProfileTreeProse(report)).Byte('\n')
	for _, level := range rfcProfileLevels(report.Rows) {
		out.Str("\n### ").Str(level.Label).Str("\n\n").Str(rfcMirrorHead(rfcProfileColumns(report)...))
		for _, row := range level.Rows {
			mandatory, optional := row.Cells()
			cells := []string{rfcProfileStemMirror(row, display), rfcMirrorCell(row.Title), mandatory, optional}
			if !report.IsAggregate() {
				cells = append(cells, rfcMirrorCell(row.Path))
			}
			out.Str(rfcMirrorRow(cells...))
		}
	}
	return out.String()
}

// rfcProfileColumns answers the tree table's columns. An aggregate has no
// path column: its members are the RFCs naming it, reached over no edge.
func rfcProfileColumns(report *rfc.TreeReport) []string {
	columns := []string{"RFC", "Title", rfcProfileMandatoryLabel, rfcProfileOptionalLabel}
	if !report.IsAggregate() {
		columns = append(columns, "Why it is here")
	}
	return columns
}

// rfcProfileTreeProse says how the tree is ordered.
func rfcProfileTreeProse(report *rfc.TreeReport) string {
	prose := "Roots first: an RFC sits one level above the highest level of the RFCs it builds on, " +
		"and the RFCs of one citation cycle share a level. Each bar is the share of that figure's rows " +
		"a recorded break proves; an RFC that is not walked shows an empty bar and no figure."
	if report.IsAggregate() {
		return prose
	}
	return prose + " The last column is the chain of edges from a root to the RFC."
}

// rfcProfileLevel is one heading of the tree: the rows sharing a level.
type rfcProfileLevel struct {
	Label string
	Rows  []*rfc.TreeRow
}

// rfcProfileLevels groups rows, already ordered roots to leaves, by the
// heading each falls under: its level, no level for a summary with no text,
// or absent.
func rfcProfileLevels(rows []*rfc.TreeRow) []rfcProfileLevel {
	var levels []rfcProfileLevel
	for _, row := range rows {
		label := rfcProfileLevelLabel(row)
		if len(levels) == 0 || levels[len(levels)-1].Label != label {
			levels = append(levels, rfcProfileLevel{Label: label})
		}
		levels[len(levels)-1].Rows = append(levels[len(levels)-1].Rows, row)
	}
	return levels
}

// rfcProfileLevelLabel names the heading one row falls under. An absent RFC
// the closure reaches through its text keeps its level, so it sits among its
// level's rows rather than splitting that level's heading in two.
func rfcProfileLevelLabel(row *rfc.TreeRow) string {
	if row.Level != nil {
		return "Level " + strconv.Itoa(*row.Level)
	}
	if row.Absent {
		return "Absent from the corpus"
	}
	return "No level: the corpus holds no text of the RFC"
}

// rfcProfileStemHTML names one row's RFC, linked to its ledger page where it
// has a summary, with its cycle.
func rfcProfileStemHTML(row *rfc.TreeRow, display map[string]string) string {
	name := html.EscapeString(rfcProfileDisplay(row.Stem, display))
	if row.Summary {
		name = `<a href="../../` + html.EscapeString(row.Stem) + `/">` + name + "</a>"
	}
	if len(row.Component) != 0 {
		name += " (cycle with " + html.EscapeString(strings.Join(row.Component, ", ")) + ")"
	}
	return name
}

// rfcProfileStemMirror states the same name in the mirror.
func rfcProfileStemMirror(row *rfc.TreeRow, display map[string]string) string {
	name := rfcMirrorCell(rfcProfileDisplay(row.Stem, display))
	if row.Summary {
		name = "[" + name + "](../../" + row.Stem + "/)"
	}
	if len(row.Component) != 0 {
		name += " (cycle with " + strings.Join(row.Component, ", ") + ")"
	}
	return name
}

// rfcProfileDisplay answers the ledger's display name of a stem, the stem
// itself for a node with no summary.
func rfcProfileDisplay(stem string, display map[string]string) string {
	if name, held := display[stem]; held {
		return name
	}
	return stem
}

// rfcProfileBarHTML renders one figure as a bar and its cell. An absent RFC
// fills nothing at 0%; an RFC whose figure is not measured (not walked, no
// summary, no rows) carries an empty bar marked unmeasured, so every RFC has
// its bar and none reads as a measurement it is not.
func rfcProfileBarHTML(row *rfc.TreeRow, cell string, proven, total int) string {
	measured := row.Absent
	width := "0"
	if !row.Absent && row.Summary && row.Walked && total > 0 {
		measured = true
		width = strconv.FormatFloat(rfcPercent(proven, total), 'f', 1, 64)
	}
	class := "rfc-bar"
	if !measured {
		class += " rfc-bar-unmeasured"
	}
	escaped := html.EscapeString(cell)
	return `<div class="` + class + `" role="img" aria-label="` + escaped + `"><span style="width:` +
		width + `%"></span></div><span class="rfc-bar-note">` + escaped + "</span>"
}

// rfcMirrorCell escapes one plain value for a Markdown table cell.
func rfcMirrorCell(value string) string { return strings.ReplaceAll(value, "|", `\|`) }

// rfcProfileStyle is the family's own stylesheet: the bar. It is inline for
// the reason healthStyle is: these rules serve this family and nothing else.
const rfcProfileStyle = `<style>
.rfc-bar { display: block; width: 100%; min-width: 7rem; height: .7rem; margin: .2rem 0 .15rem; border-radius: 999px; background: var(--panel); border: 1px solid var(--line-strong); overflow: hidden; }
.rfc-bar span { display: block; height: 100%; background: var(--teal-chip); }
.rfc-bar-unmeasured { border-style: dashed; }
.rfc-bar-note { display: block; color: var(--muted); font-size: .85rem; white-space: nowrap; }
</style>
`

// rfcProfilesIndexBody renders the index: one table per group.
func rfcProfilesIndexBody(profiles []*rfc.TreeReport) string {
	var body textbuf.Buffer
	body.Str(`            <section aria-labeledby="rfc-profiles-title" class="md-content reveal cat-observe">`).Byte('\n')
	body.Str(pageHero("RFC coverage by profile", html.EscapeString(rfcProfilesLead),
		"Quality", rfcProfilesMarker, heroClasses)).Byte('\n')
	body.Str(rfcComplianceStyle).Str(rfcProfileStyle)
	body.Str(`<p><a href="../">All RFCs</a></p>`).Byte('\n')
	for _, group := range rfcProfileGroups {
		var rows textbuf.Buffer
		for _, report := range profiles {
			if !group.Holds(report) {
				continue
			}
			rows.Str(rfcRowCells(
				`<a href="`+html.EscapeString(report.Profile)+`/">`+html.EscapeString(rfcProfileTitle(report))+"</a>",
				rfcProfileRollupBarHTML(report.Rollup.Mandatory),
				rfcProfileRollupBarHTML(report.Rollup.Optional),
				strconv.Itoa(report.Rollup.RFCs), strconv.Itoa(report.Rollup.NotWalked),
				strconv.Itoa(report.Rollup.Absent), rfcProfileComplete(report.Rollup)))
		}
		if rows.Len() == 0 {
			continue
		}
		body.Str("<section><h2>").Str(html.EscapeString(group.Label)).Str("</h2>\n").
			Str(rfcTableHTML(rfcHeadCells(rfcProfilesIndexColumns...), rows.String())).Str("\n</section>\n")
	}
	body.Str("            </section>\n")
	return body.String()
}

// rfcProfilesIndexMirror states the index in Markdown.
func rfcProfilesIndexMirror(profiles []*rfc.TreeReport) string {
	var out textbuf.Buffer
	out.Str("# RFC coverage by profile\n\n").Str(rfcProfilesLead).Str("\n\n[All RFCs](../)\n")
	for _, group := range rfcProfileGroups {
		var rows textbuf.Buffer
		for _, report := range profiles {
			if !group.Holds(report) {
				continue
			}
			rows.Str(rfcMirrorRow("["+rfcMirrorCell(rfcProfileTitle(report))+"]("+report.Profile+"/)",
				report.Rollup.Mandatory.String(), report.Rollup.Optional.String(),
				strconv.Itoa(report.Rollup.RFCs), strconv.Itoa(report.Rollup.NotWalked),
				strconv.Itoa(report.Rollup.Absent), rfcProfileComplete(report.Rollup)))
		}
		if rows.Len() == 0 {
			continue
		}
		out.Str("\n## ").Str(group.Label).Str("\n\n").Str(rfcMirrorHead(rfcProfilesIndexColumns...)).Str(rows.String())
	}
	return out.String()
}

// rfcProfilesPointerHeading and the two renderings below are the section the
// all-RFCs compliance index carries, pointing at this family's index.
const rfcProfilesPointerHeading = "Coverage by profile"

// rfcProfilesPointerText is that section's sentence, before its link.
const rfcProfilesPointerText = "This page covers every RFC. Each use case Ze serves and each technology " +
	"aggregate has its own page, with its mandatory and optional figures and the RFCs it reaches ordered " +
	"roots to leaves: "

// rfcProfilesPointerHTML renders the section's body on the compliance index.
func rfcProfilesPointerHTML() string {
	return "<p>" + html.EscapeString(rfcProfilesPointerText) + `<a href="profiles/">RFC coverage by profile</a>.</p>`
}

// rfcProfilesPointerMirror states it in the index's mirror.
func rfcProfilesPointerMirror() string {
	return rfcProfilesPointerText + "[RFC coverage by profile](profiles/).\n"
}

// rfcProfilesLead is the index's lead sentence.
const rfcProfilesLead = "Each use case Ze serves and each technology it implements, with the share of " +
	"mandatory and optional RFC requirements a recorded test break proves. Release profiles come first: " +
	"before release, every mandatory requirement of every RFC they reach must be proven."

// rfcProfilesIndexColumns are the index tables' columns.
var rfcProfilesIndexColumns = []string{"Profile", rfcProfileMandatoryLabel, rfcProfileOptionalLabel, "RFCs", "Not walked", "Absent", "Mandatory register"}

// rfcProfileRollupBarHTML renders a rollup figure as a bar and its cell.
func rfcProfileRollupBarHTML(figure rfc.Figure) string {
	class, width := "rfc-bar rfc-bar-unmeasured", "0"
	if figure.Total > 0 {
		class = "rfc-bar"
		width = strconv.FormatFloat(rfcPercent(figure.Proven, figure.Total), 'f', 1, 64)
	}
	escaped := html.EscapeString(figure.String())
	return `<div class="` + class + `" role="img" aria-label="` + escaped + `"><span style="width:` +
		width + `%"></span></div><span class="rfc-bar-note">` + escaped + "</span>"
}

// removeRetiredRFCProfilePages deletes the page of a profile this build no
// longer publishes, keyed on the marker this family writes, never on a name
// absent from the live set: the output of a real build is the published
// checkout, so a directory another producer or an author put under this
// prefix is left alone (ai/rules/never-destroy-work.md).
func removeRetiredRFCProfilePages(output string, live map[string]bool) error {
	root := filepath.Join(output, filepath.FromSlash(rfcProfilesDirectory))
	directory, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range directory {
		if !entry.IsDir() {
			continue
		}
		if live[entry.Name()] {
			continue
		}
		page, err := os.ReadFile(filepath.Join(root, entry.Name(), pageIndexFile)) //nolint:gosec // a site build reads the artifact it was pointed at
		if err != nil {
			continue
		}
		if !strings.Contains(string(page), rfcProfileMarker) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}
