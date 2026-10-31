// Design: docs/contributing/gh-pages.md -- the RFC profile pages, one per use case and per technology aggregate
// Related: rfcledger.go -- the ledger's profiles, the trees rfc.PriorityOf answers
// Related: rfccompliance.go -- the all-RFCs index these pages link back to
package site

import (
	"errors"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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
	// coverage figures in the columns of the filtered RFC list (rfcfilter.go)
	// that still prints them.
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
	useCases, err := rfcProfileUseCases(paths.Source)
	if err != nil {
		return nil, err
	}
	profiles := rfcProfilesOrdered(ledger.Profiles)
	if err := rfcProfileUseCasesKnown(useCases, profiles); err != nil {
		return nil, err
	}
	display := make(map[string]string, len(ledger.Stems))
	for index := range ledger.Stems {
		display[ledger.Stems[index].Stem] = ledger.Stems[index].Display
	}
	figures, err := loadRFCFigureSet(paths.Output)
	if err != nil {
		return nil, err
	}
	for _, report := range profiles {
		if err := rfcProfileTiersAgree(report, figures); err != nil {
			return nil, err
		}
	}

	shell := pageShell{
		Title:       "RFC coverage by profile - Ze",
		Description: "Mandatory and optional RFC requirement coverage of each use case Ze serves and each technology it implements, release profiles first.",
		Root:        rfcProfilesRoot,
		Path:        rfcProfilesDest,
		Sidebar:     pageSidebar(rfcProfilesRoot, rfcProfilesDest, links),
	}
	if err := writeRFCFigurePage(paths.Output, rfcProfilesDest, figures,
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
		route, err := writeRFCProfilePage(paths.Output, report, figures, display, links, useCases[report.Profile])
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

// rfcProfileTiersAgree refuses a profile whose tree rows and published tier
// disagree: the tables list the tree's rows while the bars sum the tier the
// profile's data file names, so a difference would print a bar over RFCs
// the table does not show. Both come from rfc.ProfileTiersOf's membership.
func rfcProfileTiersAgree(report *rfc.TreeReport, figures *rfcFigureSet) error {
	profile := figures.profiles[report.Profile]
	if profile == nil {
		var tb textbuf.Buffer
		return errors.New(tb.Str("data/rfc/index.json lists no profile named ").Str(report.Profile).
			Str(", which the published ").Str(rfcLedgerFile).Str(" holds, so its page has no figure; ").
			Str(rfcPublishMissing).String())
	}
	optional := []*rfc.TreeRow{}
	if report.Optional != nil {
		optional = report.Optional.Rows
	}
	for _, tier := range []struct {
		name    string
		rows    []*rfc.TreeRow
		members []string
	}{{"required", report.Rows, profile.Required}, {"optional", optional, profile.Optional}} {
		stems := make([]string, 0, len(tier.rows))
		for _, row := range tier.rows {
			stems = append(stems, row.Stem)
		}
		slices.Sort(stems)
		members := slices.Sorted(slices.Values(tier.members))
		if slices.Equal(stems, members) {
			continue
		}
		var tb textbuf.Buffer
		return errors.New(tb.Str("profile ").Str(report.Profile).Str(": the ").Str(tier.name).
			Str(" tier of the published ").Str(rfcLedgerFile).Str(" lists ").Str(strings.Join(stems, ", ")).
			Str(" but data/rfc/profiles/").Str(report.Profile).Str(".json lists ").Str(strings.Join(members, ", ")).
			Str(", so the page would draw its bars over other RFCs than its table shows; ").
			Str("both are written by one build, so ").Str(rfcPublishMissing).String())
	}
	return nil
}

// writeRFCFigurePage publishes a page and its mirror with the last build's
// value inside every marked element.
func writeRFCFigurePage(output, destination string, figures *rfcFigureSet, page, mirror string) error {
	page, err := figures.fillHTML(page)
	if err != nil {
		return err
	}
	mirror, err = figures.fillMirror(mirror)
	if err != nil {
		return err
	}
	return writePublishedPage(output, destination, page, mirror)
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
			" aggregate, with the MUST bar and the overview bar of the set and of each RFC."
	}
	lead := "The RFCs this use case requires, each declared by the Use cases row of its own summary. " +
		"RFCs it lists as optional follow in their own table and never enter its figures."
	if report.Release {
		lead += " Before release, every mandatory requirement of every required RFC must be " +
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
func writeRFCProfilePage(output string, report *rfc.TreeReport, figures *rfcFigureSet,
	display map[string]string, links pageLinks, useCases []rfcProfileUseCase) (string, error) {
	route := rfcProfilesRoute + report.Profile + "/"
	destination := rfcProfilesDirectory + "/" + report.Profile + "/" + pageIndexFile
	shell := pageShell{
		Title:       rfcProfileTitle(report) + " RFC coverage - Ze",
		Description: rfcProfileKindLabel(report) + " " + report.Profile + ": the MUST and overview coverage of its RFCs.",
		Root:        rfcProfileRoot,
		Path:        destination,
		Sidebar:     pageSidebar(rfcProfileRoot, route[1:], links),
	}
	if err := writeRFCFigurePage(output, destination, figures,
		shell.render(rfcProfileBody(report, display, useCases)),
		rfcProfileMirror(report, display, useCases)); err != nil {
		return "", err
	}
	return route, nil
}

// rfcProfileUseCase is one use-case page that links to a profile's coverage
// page: its route under the site root and its heading.
type rfcProfileUseCase struct {
	Route, Title string
}

// rfcProfileUseCaseLink finds the link a use-case page makes to a profile's
// coverage page, and captures the profile name.
var rfcProfileUseCaseLink = regexp.MustCompile(`quality/rfc-compliance/profiles/([a-z0-9-]+)/`)

// rfcProfileUseCases answers, for each profile, the use-case pages that link to
// its coverage page. The pairing is declared once, by the link the use-case
// page carries, and the profile page derives its link back from it, so the two
// directions cannot disagree. A use-case page with no heading is refused,
// because the link back would have nothing to name.
func rfcProfileUseCases(source string) (map[string][]rfcProfileUseCase, error) {
	pages, err := filepath.Glob(filepath.Join(source, "use-cases", "*", "index.md"))
	if err != nil {
		return nil, err
	}
	useCases := map[string][]rfcProfileUseCase{}
	for _, page := range pages {
		content, err := os.ReadFile(page) //nolint:gosec // a site build reads the checkout it was pointed at
		if err != nil {
			return nil, err
		}
		matches := rfcProfileUseCaseLink.FindAllSubmatch(content, -1)
		if len(matches) == 0 {
			continue
		}
		heading, found := strings.CutPrefix(string(content), "# ")
		if !found {
			return nil, fmt.Errorf("%s links to an RFC profile page but does not open with a `# ` heading, "+
				"so the profile page cannot name it; add the heading as its first line", page)
		}
		heading, _, _ = strings.Cut(heading, "\n")
		useCase := rfcProfileUseCase{
			Route: "use-cases/" + filepath.Base(filepath.Dir(page)) + "/",
			Title: strings.TrimSpace(heading),
		}
		named := map[string]bool{}
		for _, match := range matches {
			profile := string(match[1])
			if named[profile] {
				continue
			}
			named[profile] = true
			useCases[profile] = append(useCases[profile], useCase)
		}
	}
	return useCases, nil
}

// rfcProfileUseCasesKnown refuses a use-case page that links to a profile no
// file under rfc/profiles/ declares: its link would publish as a dead page.
func rfcProfileUseCasesKnown(useCases map[string][]rfcProfileUseCase, profiles []*rfc.TreeReport) error {
	known := make(map[string]bool, len(profiles))
	for _, report := range profiles {
		known[report.Profile] = true
	}
	for profile, pages := range useCases {
		if known[profile] {
			continue
		}
		return fmt.Errorf("website/%sindex.md links to the RFC profile %q, which no file under rfc/profiles/ "+
			"declares; link one of the profiles listed at quality/rfc-compliance/profiles/", pages[0].Route, profile)
	}
	return nil
}

// rfcProfileCard is one card of a profile's grid: its label, its value, the
// arithmetic under it, and the sentence saying what it measures.
type rfcProfileCard struct {
	Label, Value, Arithmetic, Meaning string
}

// rfcProfileCards answers the grid the profile pages printed before they
// read data/rfc/. No page renders it: it stays only while
// TestCeilingPublicationsAgree (rfcfilter_test.go) still calls it, and that
// test belongs to the in-flight phase of spec-rfc-figures-sectioned-presentation
// that retires the ceilings, which deletes both.
func rfcProfileCards(rollup *rfc.ScopeRollup) []rfcProfileCard {
	return []rfcProfileCard{
		{rfcProfileMandatoryLabel, rfcProfilePercent(rollup.Mandatory), rollup.Mandatory.String(),
			rfcKeywordSeries(rfc.MandatoryKeywords()) + " rows of the walked RFCs that a recorded test break proves."},
		{rfcProfileOptionalLabel, rfcProfilePercent(rollup.Optional), rollup.Optional.String(),
			rfcKeywordSeries(rfc.OptionalKeywords()) + " rows, counted the same way."},
		{"Mandatory ceiling", rfcCeilingPercent(rollup.Mandatory),
			rfcCeilingFigure(rollup.Mandatory.Attainable, rollup.Mandatory.Total, rollup.Mandatory.Excluded),
			"Attainable mandatory rows under current classifications, over the unchanged total."},
		{"Optional ceiling", rfcCeilingPercent(rollup.Optional),
			rfcCeilingFigure(rollup.Optional.Attainable, rollup.Optional.Total, rollup.Optional.Excluded),
			"Attainable optional rows under current classifications, over the unchanged total."},
		{"Not walked", strconv.Itoa(rollup.NotWalked), "of " + strconv.Itoa(rollup.RFCs) + " RFCs",
			"RFCs with no valid extraction sign-off: their rows count in neither figure."},
		{"Mandatory register", rfcProfileComplete(rollup), "every row proven, none unwalked",
			"A profile is complete only when it requires at least one RFC and none of them is unwalked."},
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

// rfcCeilingPercent never publishes a percentage for an empty population.
func rfcCeilingPercent(figure rfc.Figure) string {
	if figure.Total == 0 {
		return "no rows"
	}
	return rfcPercentText(figure.Attainable, figure.Total)
}

// rfcProfileSubject names one tier of a profile as the RFC data's markers do.
func rfcProfileSubject(report *rfc.TreeReport, tier string) string {
	return "profile/" + report.Profile + "/" + tier
}

// rfcProfileFigure is one line of a tier's figure table: its words and the
// field of the RFC data it shows.
type rfcProfileFigure struct {
	Label, Field string
}

// rfcProfileFigures answers a tier's figure table. The MUST bar's segments
// are read from rfcBars, the one declaration of their words, so the table and
// the bar cannot name a segment apart; the not-owed kinds are the annotation
// kinds a row can leave the MUST bar by, read from the rfc package.
func rfcProfileFigures() []rfcProfileFigure {
	figures := []rfcProfileFigure{
		{"RFCs", "rfcs"},
		{"Not walked", "not-walked"},
		{"Mandatory register", "register"},
		{"MUST rows", "must.total"},
		{"MUST rows kept (the MUST bar)", "must.kept"},
	}
	for _, segment := range rfcBars.Must {
		if len(segment.Fields) != 1 {
			panic("BUG: a MUST bar segment counts one field, " + segment.Segment + " counts " + strconv.Itoa(len(segment.Fields)))
		}
		figures = append(figures, rfcProfileFigure{segment.Label, segment.Fields[0]})
	}
	figures = append(figures, rfcProfileFigure{"MUST rows not owed", "must.not-owed"})
	kinds := rfcProfileNotOwedKinds()
	for _, kind := range kinds {
		figures = append(figures, rfcProfileFigure{"MUST not owed: " + kind, "must.not-owed-by-kind." + kind})
	}
	figures = append(figures,
		rfcProfileFigure{"SHOULD / MAY rows", "should.total"},
		rfcProfileFigure{"SHOULD proven", "should.proven"},
		rfcProfileFigure{"SHOULD OS verified", "should.os-verified"},
		rfcProfileFigure{"SHOULD OS testing gap", "should.os-gap"},
		rfcProfileFigure{"SHOULD not owed", "should.not-owed"})
	for _, kind := range kinds {
		figures = append(figures, rfcProfileFigure{"SHOULD not owed: " + kind, "should.not-owed-by-kind." + kind})
	}
	return append(figures,
		rfcProfileFigure{"SHOULD not ever", "should.not-ever"},
		rfcProfileFigure{"SHOULD not yet", "should.not-yet"},
		rfcProfileFigure{"SHOULD not decided", "should.not-decided"})
}

// rfcProfileNotOwedKinds answers the annotation kinds that take a row out of
// the bars' owed rows without a lower layer performing it (not applicable,
// declined, OS blocked): the keys rfcStemDataOf files under not-owed-by-kind.
func rfcProfileNotOwedKinds() []string {
	var kinds []string
	for _, kind := range rfc.AnnotationKinds() {
		traits, known := rfc.AnnotationTraitsOf(kind)
		if !known {
			panic("BUG: a registered annotation kind has no traits: " + kind)
		}
		if traits.CeilingExcluded && !traits.OSMet {
			kinds = append(kinds, kind)
		}
	}
	return kinds
}

// rfcProfileFiguresProse says what the bars and the figure table count.
func rfcProfileFiguresProse() string {
	return "The MUST bar counts the " + rfcKeywordSeries(rfc.MandatoryKeywords()) + " rows of the walked RFCs " +
		"that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze " +
		"performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap " +
		"until one does. The overview bar adds the " + rfcKeywordSeries(rfc.OptionalKeywords()) + " rows and " +
		"the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, " +
		"so its rows count in no figure. Every count is read from the RFC data files when the page loads."
}

// rfcProfileTierHTML renders one tier's two bars and figure table.
func rfcProfileTierHTML(subject string) string {
	var out textbuf.Buffer
	out.Str("<p><strong>MUST bar</strong></p>\n").Str(rfcBarMarkHTML(subject, "must")).Byte('\n')
	out.Str("<p><strong>Overview bar</strong></p>\n").Str(rfcBarMarkHTML(subject, "overview")).Byte('\n')
	var rows textbuf.Buffer
	for _, figure := range rfcProfileFigures() {
		rows.Str(rfcRowCells(html.EscapeString(figure.Label), rfcValueHTML(subject, figure.Field)))
	}
	out.Str(rfcTableHTML(rfcHeadCells("Figure", "Rows"), rows.String())).Byte('\n')
	out.Str("<p>").Str(html.EscapeString(rfcProfileFiguresProse())).Str("</p>\n")
	return out.String()
}

// rfcProfileTierMirror states the same tier in the mirror.
func rfcProfileTierMirror(subject string) string {
	var out textbuf.Buffer
	out.Str("- **MUST bar**: ").Str(rfcBarMirror(subject, "must")).Byte('\n')
	out.Str("- **Overview bar**: ").Str(rfcBarMirror(subject, "overview")).Str("\n\n")
	out.Str(rfcMirrorHead("Figure", "Rows"))
	for _, figure := range rfcProfileFigures() {
		out.Str(rfcMirrorRow(rfcMirrorCell(figure.Label), rfcValueMirror(subject, figure.Field)))
	}
	return out.Str("\n").Str(rfcProfileFiguresProse()).Byte('\n').String()
}

// rfcProfileRowsHTML renders a tier's table rows, each RFC with its bars.
func rfcProfileRowsHTML(report *rfc.TreeReport, rows []*rfc.TreeRow, display map[string]string) string {
	var out textbuf.Buffer
	for _, row := range rows {
		cells := []string{rfcProfileStemHTML(row, display), html.EscapeString(row.Title),
			rfcBarMarkHTML(row.Stem, "must"), rfcBarMarkHTML(row.Stem, "overview")}
		if !report.IsAggregate() {
			cells = append(cells, html.EscapeString(row.Path))
		}
		out.Str(rfcRowCells(cells...))
	}
	return out.String()
}

// rfcProfileRowsMirror states the same rows in the mirror.
func rfcProfileRowsMirror(report *rfc.TreeReport, rows []*rfc.TreeRow, display map[string]string) string {
	var out textbuf.Buffer
	for _, row := range rows {
		cells := []string{rfcProfileStemMirror(row, display), rfcMirrorCell(row.Title),
			rfcBarMirror(row.Stem, "must"), rfcBarMirror(row.Stem, "overview")}
		if !report.IsAggregate() {
			cells = append(cells, rfcMirrorCell(row.Path))
		}
		out.Str(rfcMirrorRow(cells...))
	}
	return out.String()
}

// rfcFiguresScriptHTML loads the script that fills the marked elements.
func rfcFiguresScriptHTML(root string) string {
	return `<script src="` + root + `assets/rfc-figures.js" defer></script>` + "\n"
}

// rfcProfileBody renders one profile's page. Every figure is a marked
// element writeRFCFigurePage fills with the last build's value and
// rfc-figures.js replaces from data/rfc/.
func rfcProfileBody(report *rfc.TreeReport, display map[string]string, useCases []rfcProfileUseCase) string {
	var body textbuf.Buffer
	body.Str(`            <section aria-labeledby="rfc-profile-title" class="md-content reveal cat-observe">`).Byte('\n')
	body.Str(pageHero(html.EscapeString(rfcProfileTitle(report)), html.EscapeString(rfcProfileLead(report)),
		rfcProfileKindLabel(report), rfcProfileMarker, heroClasses)).Byte('\n')
	body.Str(rfcComplianceStyle)
	body.Str(`<p><a href="../">All profiles</a> · <a href="../../">All RFCs</a></p>`).Byte('\n')
	for _, useCase := range useCases {
		body.Str(`<p>Use case: <a href="`).Str(rfcProfileRoot).Str(useCase.Route).Str(`">`).
			Str(html.EscapeString(useCase.Title)).Str("</a></p>\n")
	}
	body.Str("<section><h2>").Str(rfcProfileFiguresHeading).Str("</h2>\n").
		Str(rfcProfileTierHTML(rfcProfileSubject(report, "required"))).Str("</section>\n")
	body.Str("<section><h2>Tree</h2>\n<p>").Str(html.EscapeString(rfcProfileTreeProse(report))).Str("</p>\n")
	for _, level := range rfcProfileLevels(report.Rows) {
		body.Str("<h3>").Str(html.EscapeString(level.Label)).Str("</h3>\n")
		body.Str(rfcTableHTML(rfcHeadCells(rfcProfileColumns(report)...), rfcProfileRowsHTML(report, level.Rows, display))).Byte('\n')
	}
	body.Str("</section>\n")
	if report.Optional != nil && len(report.Optional.Rows) != 0 {
		body.Str("<section><h2>").Str(rfcProfileOptionalHeading).Str("</h2>\n<p>").
			Str(html.EscapeString(rfcProfileOptionalProse(report))).Str("</p>\n")
		body.Str(rfcProfileTierHTML(rfcProfileSubject(report, "optional")))
		body.Str(rfcTableHTML(rfcHeadCells(rfcProfileColumns(report)...),
			rfcProfileRowsHTML(report, report.Optional.Rows, display))).Str("\n</section>\n")
	}
	body.Str(rfcFiguresScriptHTML(rfcProfileRoot))
	body.Str("            </section>\n")
	return body.String()
}

// rfcProfileFiguresHeading heads a profile's required-tier figures.
const rfcProfileFiguresHeading = "Figures"

// rfcProfileOptionalHeading heads a profile's optional tier.
const rfcProfileOptionalHeading = "Optional (not counted)"

// rfcProfileOptionalProse says what report's optional tier is and which
// summary row declares it; the tier's own bars and figures follow it.
func rfcProfileOptionalProse(report *rfc.TreeReport) string {
	figures := " Their own bars and figures follow."
	if report.IsAggregate() {
		return "RFCs of this aggregate listed as optional, each declared by the Aggregate tier row of its " +
			"own summary. They count in none of the figures above until the owner brings one into scope " +
			"by deleting that row." + figures
	}
	return "RFCs this use case lists as optional, each declared by the Optional use cases row of its " +
		"own summary. They count in none of the figures above until the owner brings one into scope " +
		"by moving it to the Use cases row." + figures
}

// rfcProfileDataLinks names, in a mirror, the files its figures were read
// from, relative to the mirror at root.
func rfcProfileDataLinks(root, file string) string {
	return "Figures as of the last build, read from [" + file + "](" + root + file + ") and [" +
		rfcDataIndexFile + "](" + root + rfcDataIndexFile + ").\n\n"
}

// rfcProfileMirror states the same page in Markdown, in the page's order.
func rfcProfileMirror(report *rfc.TreeReport, display map[string]string, useCases []rfcProfileUseCase) string {
	var out textbuf.Buffer
	out.Str("# ").Str(rfcProfileTitle(report)).Str("\n\n")
	out.Str(rfcProfileKindLabel(report)).Str(". ").Str(rfcProfileLead(report)).Str("\n\n")
	out.Str("[All profiles](../) · [All RFCs](../../)\n\n")
	for _, useCase := range useCases {
		out.Str("Use case: [").Str(useCase.Title).Str("](").Str(rfcProfileRoot).Str(useCase.Route).Str(")\n\n")
	}
	out.Str(rfcProfileDataLinks(rfcProfileRoot, rfcProfileDataFile(report.Profile)))
	out.Str("## ").Str(rfcProfileFiguresHeading).Str("\n\n").Str(rfcProfileTierMirror(rfcProfileSubject(report, "required")))
	out.Str("\n## Tree\n\n").Str(rfcProfileTreeProse(report)).Byte('\n')
	for _, level := range rfcProfileLevels(report.Rows) {
		out.Str("\n### ").Str(level.Label).Str("\n\n").Str(rfcMirrorHead(rfcProfileColumns(report)...)).
			Str(rfcProfileRowsMirror(report, level.Rows, display))
	}
	if report.Optional != nil && len(report.Optional.Rows) != 0 {
		out.Str("\n## ").Str(rfcProfileOptionalHeading).Str("\n\n").
			Str(rfcProfileOptionalProse(report)).Str("\n\n").
			Str(rfcProfileTierMirror(rfcProfileSubject(report, "optional"))).Byte('\n').
			Str(rfcMirrorHead(rfcProfileColumns(report)...)).
			Str(rfcProfileRowsMirror(report, report.Optional.Rows, display))
	}
	return out.String()
}

// The tree table's bar columns.
const (
	rfcProfileMustColumn     = "MUST bar"
	rfcProfileOverviewColumn = "Overview bar"
)

// rfcProfileColumns answers the tree table's columns. An aggregate has no
// path column: its members are the RFCs naming it in their aggregate cell.
func rfcProfileColumns(report *rfc.TreeReport) []string {
	columns := []string{"RFC", "Title", rfcProfileMustColumn, rfcProfileOverviewColumn}
	if !report.IsAggregate() {
		columns = append(columns, "Why it is here")
	}
	return columns
}

// rfcProfileTreeProse says how the tree is ordered.
func rfcProfileTreeProse(report *rfc.TreeReport) string {
	prose := "Roots first: an RFC sits one level above the highest level of the RFCs it builds on, " +
		"and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the " +
		"set above, from its own data file; an RFC that is not walked shows \"not walked\" and no bar."
	if report.IsAggregate() {
		return prose
	}
	return prose + " The last column names the summary whose Use cases row declares the RFC: a " +
		"normative citation never adds one."
}

// rfcProfileLevel is one heading of the tree: the rows sharing a level.
type rfcProfileLevel struct {
	Label string
	Rows  []*rfc.TreeRow
}

// rfcProfileLevels groups rows, already ordered roots to leaves, by the
// heading each falls under: its level, or no level for a summary with no text.
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

// rfcProfileLevelLabel names the heading one row falls under.
func rfcProfileLevelLabel(row *rfc.TreeRow) string {
	if row.Level != nil {
		return "Level " + strconv.Itoa(*row.Level)
	}
	return "No level: the corpus holds no text of the RFC"
}

// rfcProfileStemHTML names one row's RFC, linked to its ledger page, with its
// cycle. Every row of a profile page is a declared member, and every member
// has a summary, so every row has a page to link.
func rfcProfileStemHTML(row *rfc.TreeRow, display map[string]string) string {
	name := `<a href="../../` + html.EscapeString(row.Stem) + `/">` +
		html.EscapeString(rfcProfileDisplay(row.Stem, display)) + "</a>"
	if len(row.Component) != 0 {
		name += " (cycle with " + html.EscapeString(strings.Join(row.Component, ", ")) + ")"
	}
	return name
}

// rfcProfileStemMirror states the same name in the mirror.
func rfcProfileStemMirror(row *rfc.TreeRow, display map[string]string) string {
	name := "[" + rfcMirrorCell(rfcProfileDisplay(row.Stem, display)) + "](../../" + row.Stem + "/)"
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

// rfcProfileBarHTML renders one figure as a bar and its cell. An RFC whose
// figure is not measured (not walked, no rows) carries an empty
// bar marked unmeasured, so every RFC has its bar and none reads as a
// measurement it is not.
func rfcProfileBarHTML(row *rfc.TreeRow, cell string, proven, total int) string {
	measured := false
	width := "0"
	if row.Walked && total > 0 {
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
.rfc-bar span { display: block; height: 100%; background: var(--teal-base); }
.rfc-bar-unmeasured { border-style: dashed; }
.rfc-bar-note { display: block; color: var(--muted); font-size: .85rem; white-space: nowrap; }
.rfc-ceiling { display: block; color: var(--muted); font-size: .8rem; white-space: normal; }
</style>
`

// rfcProfilesIndexBody renders the index: one table per group, each profile
// with its required tier's two bars and counts as marked elements.
func rfcProfilesIndexBody(profiles []*rfc.TreeReport) string {
	var body textbuf.Buffer
	body.Str(`            <section aria-labeledby="rfc-profiles-title" class="md-content reveal cat-observe">`).Byte('\n')
	body.Str(pageHero("RFC coverage by profile", html.EscapeString(rfcProfilesLead),
		"Quality", rfcProfilesMarker, heroClasses)).Byte('\n')
	body.Str(rfcComplianceStyle)
	body.Str(`<p><a href="../">All RFCs</a></p>`).Byte('\n')
	for _, group := range rfcProfileGroups {
		var rows textbuf.Buffer
		for _, report := range profiles {
			if !group.Holds(report) {
				continue
			}
			subject := rfcProfileSubject(report, "required")
			rows.Str(rfcRowCells(
				`<a href="`+html.EscapeString(report.Profile)+`/">`+html.EscapeString(rfcProfileTitle(report))+"</a>",
				rfcBarMarkHTML(subject, "must"), rfcBarMarkHTML(subject, "overview"),
				`<a href="`+html.EscapeString(rfcProfileListHref(report))+`">`+rfcValueHTML(subject, "rfcs")+"</a>",
				rfcValueHTML(subject, "not-walked"), rfcValueHTML(subject, "register")))
		}
		if rows.Len() == 0 {
			continue
		}
		body.Str("<section><h2>").Str(html.EscapeString(group.Label)).Str("</h2>\n").
			Str(rfcTableHTML(rfcHeadCells(rfcProfilesIndexColumns...), rows.String())).Str("\n</section>\n")
	}
	body.Str(rfcFiguresScriptHTML(rfcProfilesRoot))
	body.Str("            </section>\n")
	return body.String()
}

// rfcProfilesIndexMirror states the index in Markdown.
func rfcProfilesIndexMirror(profiles []*rfc.TreeReport) string {
	var out textbuf.Buffer
	out.Str("# RFC coverage by profile\n\n").Str(rfcProfilesLead).Str("\n\n[All RFCs](../)\n\n").
		Str("Figures as of the last build, read from [").Str(rfcDataIndexFile).Str("](").Str(rfcProfilesRoot).
		Str(rfcDataIndexFile).Str(") and the profile files it names.\n")
	for _, group := range rfcProfileGroups {
		var rows textbuf.Buffer
		for _, report := range profiles {
			if !group.Holds(report) {
				continue
			}
			subject := rfcProfileSubject(report, "required")
			rows.Str(rfcMirrorRow("["+rfcMirrorCell(rfcProfileTitle(report))+"]("+report.Profile+"/)",
				rfcBarMirror(subject, "must"), rfcBarMirror(subject, "overview"),
				"["+rfcValueMirror(subject, "rfcs")+"]("+rfcProfileListHref(report)+")",
				rfcValueMirror(subject, "not-walked"), rfcValueMirror(subject, "register")))
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
	"aggregate has its own page, with its MUST bar and overview bar and the RFCs it lists ordered " +
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
const rfcProfilesLead = "Each use case Ze serves and each technology it implements, with the MUST bar and " +
	"the overview bar of the RFCs it requires. Release profiles come first: " +
	"before release, every mandatory requirement of every RFC they require must be proven. Optional " +
	"RFCs a use case lists are not counted."

// rfcProfilesIndexColumns are the index tables' columns.
var rfcProfilesIndexColumns = []string{"Profile", rfcProfileMustColumn, rfcProfileOverviewColumn, "RFCs", "Not walked", "Mandatory register"}

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
