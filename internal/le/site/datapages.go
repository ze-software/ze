// Design: website/AI.md -- data pages and the monthly milestone overview
// Detail: shell.go wraps each body; dependencies.go is the fourth data page and reads go.mod too.
package site

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/feature"
)

// The three data pages register from here. A build discovers them through the
// registry rather than through a call it states by name.
func init() {
	registerProducer(Producer{Name: featuresDirectory, Render: renderFeatures})
	registerProducer(Producer{Name: "milestones", Render: renderMilestones})
	registerProducer(Producer{Name: talksDirectory, Render: renderTalkIndex})
}

// dataDateLayout is the date every data file writes, and displayDateLayout is
// how a reader sees one: "2026-06-11" becomes "11 June 2026".
const (
	dataDateLayout    = "2006-01-02"
	displayDateLayout = "2 January 2006"
)

// legendCategories are the seven topic hues in the order the category filter
// writes them, which is the Features page's own order rather than the order any
// data file happens to use.
//
// A page's legend is a fixed vocabulary: the same seven buttons appear whether
// or not the data has a card in each. Deriving the order from the data would
// reorder the buttons whenever a card moved, and would drop a category the data
// has nothing in yet.
var legendCategories = []string{
	categoryOperate, categoryRouting, categoryServices, categoryAutomate,
	categoryObserve, categorySecure, categoryPlatform,
}

// inlineMarkup converts the two markers a data file may carry into HTML.
//
// A data file holds text plus this vocabulary, never raw HTML, so the whole
// string is escaped first and only then are `code` and **bold** turned into
// tags. A "<" an author typed therefore reaches the reader as a "<".
func inlineMarkup(text string) string {
	escaped := html.EscapeString(text)
	return wrapInlineMarker(wrapInlineMarker(escaped, "`", "<code>", "</code>"), "**", "<strong>", "</strong>")
}

// wrapInlineMarker replaces each pair of marker runs with an opening and a
// closing tag. An unpaired marker is left as the reader typed it.
//
// The loop is bounded by the length of the text: each turn consumes at least
// the opening marker, and an unpaired marker returns.
func wrapInlineMarker(text, marker, opening, closing string) string {
	var out textbuf.Buffer
	rest := text
	for {
		start := strings.Index(rest, marker)
		if start < 0 {
			out.Str(rest)
			return out.String()
		}
		after := start + len(marker)
		end := strings.Index(rest[after:], marker)
		if end <= 0 {
			out.Str(rest)
			return out.String()
		}
		out.Str(rest[:start])
		out.Str(opening).Str(rest[after : after+end]).Str(closing)
		rest = rest[after+end+len(marker):]
	}
}

// readSourceJSON decodes one committed data file of the website source tree.
//
// An absent or malformed file is an error rather than an empty page: the file
// IS the page, so a build that cannot read it has nothing to publish and must
// say so by name.
func readSourceJSON(source, name string, target any) error {
	path := filepath.Join(source, "data", name)
	content, err := os.ReadFile(path) //nolint:gosec // a site build reads the checkout it was pointed at
	if err != nil {
		return fmt.Errorf("read data/%s: %w", name, err)
	}
	if err := json.Unmarshal(content, target); err != nil {
		return fmt.Errorf("read data/%s: %w", name, err)
	}
	return nil
}

// displayDate answers the date a reader sees, from the date a data file states.
func displayDate(date string) (string, error) {
	parsed, err := time.Parse(dataDateLayout, date)
	if err != nil {
		return "", fmt.Errorf("date %q: %w", date, err)
	}
	return parsed.Format(displayDateLayout), nil
}

// Where the features page reads and writes.
const (
	featuresDataFile = "features.json"
	// featuresDirectory is where the page publishes, and it is the producer's
	// own name: one page, one directory, one claim.
	featuresDirectory          = "features"
	featuresDest               = featuresDirectory + "/" + pageIndexFile
	featuresRoot               = "../"
	featuresRoute              = "/" + featuresDirectory + "/"
	featureSectionCore         = "core"
	featureSectionExperimental = "experimental"
)

// featureData is data/features.json: the two titled sections of the page, and
// one ordered list of cards. A card states which feature declarations it
// represents and never its own maturity: loadFeatureData derives from the
// declarations whether it is shipped, and so which section it sits in (D-9 of
// plan/pre-release/spec-feature-maturity-declared.md). The card order is the
// file's own within each section, so nothing here is sorted.
type featureData struct {
	Sections []featureSection `json:"sections"`
	Cards    []featureCard    `json:"cards"`
}

// featureSection is one titled block of the features page. Cards is derived by
// loadFeatureData and is never read from the file.
type featureSection struct {
	ID      string        `json:"id"`
	Heading string        `json:"heading"`
	Lead    string        `json:"lead"`
	Note    string        `json:"note"`
	Cards   []featureCard `json:"-"`
}

// featureCard is one feature. Features names the declarations under features/
// the card represents. External says whether Href leaves this site, which
// decides both the link target and whether the href is rewritten relative to
// the page. shipped is derived from the declarations, never read from the file.
type featureCard struct {
	Category string        `json:"category"`
	Features []string      `json:"features"`
	Title    string        `json:"title"`
	Href     string        `json:"href"`
	External bool          `json:"external"`
	Chips    []featureChip `json:"chips"`
	Bullets  []string      `json:"bullets"`
	shipped  bool
}

// featureChip is one badge on a card. Mode marks a chip that states where the
// feature runs rather than what it is, and it takes its own class.
type featureChip struct {
	Text string `json:"text"`
	Mode bool   `json:"mode"`
}

// validate refuses a card the page cannot render honestly.
//
// The retired renderer validated the same three fields and its build exited
// non-zero on a failure, so a card that fails one never reached a reader. Each
// is refused by name here for the same reason: an unknown category renders a
// cat- class no stylesheet defines, so the card loses its color and its filter
// button silently.
func (card *featureCard) validate(where string) error {
	if card.Title == "" {
		return fmt.Errorf("%s: a feature card states no title", where)
	}
	if err := validateSubjectCategory(card.Category); err != nil {
		return fmt.Errorf("%s: feature card %q: %w", where, card.Title, err)
	}
	if card.Href == "" {
		return fmt.Errorf("%s: feature card %q links nowhere", where, card.Title)
	}
	return nil
}

// status answers the card's maturity class: empty when shipped, the
// experimental level's name otherwise. The word comes from internal/le/feature,
// the one declaration of the maturity vocabulary.
func (card *featureCard) status() string {
	if card.shipped {
		return ""
	}
	return feature.CardClass()
}

// maturityLabel makes both states explicit without changing the declaration's
// definition of shipped. The card and its mirror publish this same label.
func (card *featureCard) maturityLabel() string {
	if card.shipped {
		return "Shipped in current code"
	}
	return feature.CardLabel(card.shipped)
}

// href answers what the card's title links to: an external address as the file
// states it, and an internal one relative to the features page.
func (card *featureCard) href() string {
	if card.External {
		return card.Href
	}
	return featuresRoot + card.Href
}

// mirrorHref answers what the card's title links to from the Markdown mirror,
// which a reader reaches from anywhere and so takes absolute site addresses.
func (card *featureCard) mirrorHref() string {
	if card.External {
		return card.Href
	}
	return siteBase + card.Href
}

// loadFeatureData reads data/features.json and derives each card's state from
// the declarations under the repository's features/.
//
// Every reader of the feature cards (the page, the facts, the homepage,
// llms.txt) goes through here, so the published count is the count of the cards
// this derivation kept. The file is decoded strictly: a card that still states a
// "status", or a section that still holds its own cards, is a hand-typed
// maturity this build no longer reads, and is refused rather than ignored.
func loadFeatureData(paths Paths) (featureData, error) {
	var data featureData
	where := "data/" + featuresDataFile
	content, err := os.ReadFile(filepath.Join(paths.Source, "data", featuresDataFile)) //nolint:gosec // a site build reads the checkout it was pointed at
	if err != nil {
		return data, fmt.Errorf("read %s: %w", where, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&data); err != nil {
		return data, fmt.Errorf("read %s: %w", where, err)
	}
	if err := data.checkSections(); err != nil {
		return data, err
	}
	declarations, err := feature.Declared(paths.Repository)
	if err != nil {
		return data, fmt.Errorf("%s: read the feature declarations: %w", where, err)
	}
	byID := feature.ByID(declarations)
	for index := range data.Cards {
		card := &data.Cards[index]
		if err := card.validate(where); err != nil {
			return data, err
		}
		shipped, err := feature.CardShipped(byID, card.Features)
		if err != nil {
			return data, fmt.Errorf("%s: feature card %q %w", where, card.Title, err)
		}
		card.shipped = shipped
	}
	if err := data.place(); err != nil {
		return data, err
	}
	return data, nil
}

// checkSections refuses a section with no id or a repeated one.
func (data *featureData) checkSections() error {
	seen := make(map[string]bool, len(data.Sections))
	for _, section := range data.Sections {
		if section.ID == "" {
			return fmt.Errorf("data/%s carries a section with no id", featuresDataFile)
		}
		if seen[section.ID] {
			return fmt.Errorf("data/%s declares two %q sections", featuresDataFile, section.ID)
		}
		seen[section.ID] = true
	}
	return nil
}

// place puts each card in the section its derived state names: core when
// shipped, experimental otherwise, each in the file's own card order.
//
// Both sections must exist, because the page's count is core plus
// experimental: a file missing either would publish a number that means
// something else. A file with no card is refused for the same reason, as a
// zero count.
func (data *featureData) place() error {
	core, err := data.section(featureSectionCore)
	if err != nil {
		return err
	}
	experimental, err := data.section(featureSectionExperimental)
	if err != nil {
		return err
	}
	for index := range data.Cards {
		if data.Cards[index].shipped {
			core.Cards = append(core.Cards, data.Cards[index])
			continue
		}
		experimental.Cards = append(experimental.Cards, data.Cards[index])
	}
	if len(data.Cards) == 0 {
		return fmt.Errorf("data/%s states no shipped or experimental feature, so the published feature count would be zero", featuresDataFile)
	}
	return nil
}

// section answers one section of the file by its id, for place to fill.
func (data *featureData) section(id string) (*featureSection, error) {
	for index := range data.Sections {
		if data.Sections[index].ID == id {
			return &data.Sections[index], nil
		}
	}
	return nil, fmt.Errorf("data/%s declares no %q section", featuresDataFile, id)
}

// renderFeatures publishes the features page and its mirror.
func renderFeatures(paths Paths) ([]string, error) {
	data, err := loadFeatureData(paths)
	if err != nil {
		return nil, err
	}
	shipped := data.Cards
	links, err := loadPageLinks(paths.Source)
	if err != nil {
		return nil, err
	}

	shell := pageShell{
		Title:       "Features - Ze",
		Description: "Shipped and experimental features, grouped by maturity and category.",
		Root:        featuresRoot,
		Path:        featuresDest,
		Sidebar:     pageSidebar(featuresRoot, featuresDest, links),
	}
	if err := writePublishedPage(paths.Output, featuresDest,
		shell.render(featuresBody(data, shipped)), featuresMirror(data, len(shipped))); err != nil {
		return nil, err
	}
	return []string{featuresRoute}, nil
}

// featuresBody renders the page under <main>: the hero and its filter legend
// first, then one block for each section in the file's own order.
func featuresBody(data featureData, shipped []featureCard) string {
	counts := make(map[string]int, len(legendCategories))
	for index := range shipped {
		counts[shipped[index].Category]++
	}

	var body textbuf.Buffer
	body.Str("            <section aria-labelledby=\"features-title\">\n")
	body.Str(pageHero("Every feature Ze ships.",
		strconv.Itoa(len(shipped))+" shipped or experimental feature cards.",
		"Project", ` id="features-title"`, heroClasses)).Byte('\n')
	body.Str("                <div class=\"section-note reveal\">\n")
	body.Str("                    <p>Each card&#39;s color is its category: how the feature fits ").
		Str("into the system. Solid cards are shipped and dashed cards are experimental. ").
		Str("Everything shipped runs in both daemon and appliance modes unless a card says otherwise. ").
		Str("Click a category to filter, click again to show everything.</p>\n")
	body.Str("                </div>\n")
	body.Str("            </section>\n")
	body.Str(`<div class="feature-catalog"><aside class="feature-filters" aria-label="Feature filters">`).Byte('\n')
	body.Str(`<div class="feature-filter-controls" hidden>`).Byte('\n')
	body.Str(`<div><label for="feature-search">Find a feature</label>`).
		Str(`<input id="feature-search" type="search" placeholder="Search capabilities…" />`).Byte('\n')
	body.Str(`<label for="feature-maturity">Maturity</label><select id="feature-maturity">`).
		Str(`<option value="all">All statuses</option><option value="supported">Shipped in current code</option>`).
		Str(`<option value="experimental">Experimental</option></select></div>`).Byte('\n')
	body.Str(`<div><button type="button" data-reset-features>Reset filters</button></div>`).Byte('\n')
	body.Str("</div>\n")
	body.Str("                <div class=\"legend reveal\" role=\"group\" aria-label=\"Filter features by category\">\n")
	for _, category := range legendCategories {
		label := html.EscapeString(categoryLabel(category))
		count := strconv.Itoa(counts[category])
		body.Str("                    <button class=\"cat-").Str(category).Str("\" data-cat=\"").Str(category).
			Str("\" aria-pressed=\"false\" aria-label=\"Filter features by ").Str(label).Str(", ").Str(count).
			Str(" features\">").Str(label).Str(" <span class=\"legend-count\" aria-hidden=\"true\">").Str(count).
			Str("</span></button>\n")
	}
	body.Str("                </div>\n")
	body.Str("</aside>\n<div class=\"feature-results\">\n")
	body.Str("                <p id=\"feature-filter-status\" class=\"feature-filter-status search-status\" aria-live=\"polite\"></p>\n")

	for _, section := range data.Sections {
		// A section no card's derived state places in is not published: its
		// heading and lead describe cards that are not there.
		if len(section.Cards) == 0 {
			continue
		}
		body.Str(featureSectionHTML(section))
		body.Byte('\n')
	}
	body.Str(`<div id="feature-empty" class="catalog-empty" hidden>`).
		Str(`<h2>No matching features</h2><p>Try another term, category or maturity.</p>`).
		Str(`<button type="button" data-reset-features>Reset filters</button></div>`).Byte('\n')
	body.Str("            <section id=\"roadmap\" aria-labelledby=\"roadmap-title\">\n").
		Str("                <h2 id=\"roadmap-title\">Release roadmap</h2>\n").
		Str("                <p>The <a href=\"../project/roadmap/\">release roadmap</a> lists remaining ").
		Str("work for the first release, later improvements, and proposals from the project's recorded plans. ").
		Str("A plan does not mean a feature is ready to use.</p>\n            </section>\n")
	body.Str("</div></div>\n")
	return body.String()
}

// featureSectionHTML renders one titled block: its heading, its lead, its
// optional note, and its cards in the file's own order.
func featureSectionHTML(section featureSection) string {
	var out textbuf.Buffer
	out.Str("            <section id=\"").Str(html.EscapeString(section.ID)).Str("\" aria-labelledby=\"").
		Str(html.EscapeString(section.ID)).Str("-title\" data-cards>\n")
	out.Str("                <div class=\"section-head reveal\">\n")
	out.Str("                    <h2 id=\"").Str(html.EscapeString(section.ID)).Str("-title\">").
		Str(html.EscapeString(section.Heading)).Str("</h2>\n")
	out.Str("                    <p>").Str(html.EscapeString(section.Lead)).Str("</p>\n")
	out.Str("                </div>\n")
	if section.Note != "" {
		out.Str("                <div class=\"section-note reveal\">\n")
		out.Str("                    <p>").Str(inlineMarkup(section.Note)).Str("</p>\n")
		out.Str("                </div>\n")
	}
	out.Str("                <div class=\"cards feature-grid reveal\">\n")
	for index := range section.Cards {
		out.Str(featureCardHTML(&section.Cards[index]))
	}
	out.Str("                </div>\n")
	out.Str("            </section>\n")
	return out.String()
}

// featureCardHTML renders one card: its category badge, its maturity badge, its
// linked title, its chips and its bullets.
func featureCardHTML(card *featureCard) string {
	classes := "card feature-card"
	if status := card.status(); status != "" {
		classes += " " + status
	}
	classes += " cat-" + card.Category

	var out textbuf.Buffer
	out.Str("                    <article class=\"").Str(html.EscapeString(classes)).Str("\" data-cat=\"").
		Str(html.EscapeString(card.Category)).Str(`" data-maturity="`)
	if card.shipped {
		out.Str("supported")
	} else {
		out.Str(feature.CardClass())
	}
	out.Str("\">\n")
	out.Str("                    <span class=\"cat\">").Str(html.EscapeString(categoryLabel(card.Category))).
		Str("</span>\n")
	out.Str(`                    <span class="status">`).
		Str(html.EscapeString(card.maturityLabel())).Str("</span>\n")
	target := ""
	if card.External {
		target = ` target="_blank" rel="noopener"`
	}
	out.Str("                    <h3><a href=\"").Str(html.EscapeString(card.href())).Byte('"').Str(target).Byte('>').
		Str(html.EscapeString(card.Title)).Str("</a></h3>\n")
	out.Str("                    <div class=\"chips\">\n")
	for _, chip := range card.Chips {
		class := compactLabelClass("chip", chip.Text)
		if chip.Mode {
			class = "chip mode"
		}
		out.Str("                    <span class=\"").Str(class).Str("\">").Str(html.EscapeString(chip.Text)).
			Str("</span>\n")
	}
	out.Str("                    </div>\n")
	out.Str("                    <ul>\n")
	for _, bullet := range card.Bullets {
		out.Str("                    <li>").Str(inlineMarkup(bullet)).Str("</li>\n")
	}
	out.Str("                    </ul>\n")
	out.Str("                    </article>\n")
	return out.String()
}

// featuresMirror renders the Markdown sibling from the same data the page uses,
// so the two cannot disagree about what the site ships.
func featuresMirror(data featureData, shipped int) string {
	var mirror textbuf.Buffer
	mirror.Str("# Every feature Ze ships.\n\n")
	mirror.Int(int64(shipped)).Str(" shipped or experimental feature cards. ").
		Str("Each card's category shows where the feature fits: operate, routing, services, automate, ").
		Str("observe, secure, or ").Str(strings.ToLower(categoryLabel(categoryPlatform))).
		Str(". Everything shipped runs in both daemon and appliance modes ").
		Str("unless a card says otherwise.\n\n")
	for _, section := range data.Sections {
		if len(section.Cards) == 0 {
			continue
		}
		mirror.Str("## ").Str(section.Heading).Str("\n\n")
		mirror.Str(section.Lead).Str("\n\n")
		if section.Note != "" {
			mirror.Str("> ").Str(section.Note).Str("\n\n")
		}
		for index := range section.Cards {
			card := &section.Cards[index]
			mirror.Str("### ").Str(card.Title).Str("\n\n")
			meta := strings.ToLower(categoryLabel(card.Category))
			meta += " / " + card.maturityLabel()
			line := "*" + meta + "*"
			if len(card.Chips) != 0 {
				chips := make([]string, 0, len(card.Chips))
				for _, chip := range card.Chips {
					chips = append(chips, "`"+chip.Text+"`")
				}
				line += " -- " + strings.Join(chips, " ")
			}
			mirror.Str(line).Str("\n\n")
			for _, bullet := range card.Bullets {
				mirror.Str("- ").Str(bullet).Byte('\n')
			}
			mirror.Str("\n[Learn more](").Str(card.mirrorHref()).Str(")\n\n")
		}
	}
	mirror.Str("## Release roadmap\n\nThe [release roadmap](").Str(siteBase).
		Str("project/roadmap/) lists remaining work for the first release, later improvements, and proposals from the project's recorded plans. ").
		Str("A plan does not mean a feature is ready to use.\n\n")
	return strings.TrimSpace(mirror.String()) + "\n"
}

// Where the milestones page reads and writes.
const (
	milestonesDataFile  = "milestones.json"
	milestonesDirectory = "project/milestones"
	milestonesDest      = milestonesDirectory + "/" + pageIndexFile
	milestonesRoot      = "../../"
	milestonesRoute     = "/" + milestonesDirectory + "/"
)

// milestoneData is data/milestones.json: the page's own lead and one entry for
// each landmark capability.
type milestoneData struct {
	Intro      string      `json:"intro"`
	Milestones []milestone `json:"milestones"`
}

// milestone is one landmark. Blog names the week that announced it, relative to
// the changes directory, and is empty for a milestone no week covers.
type milestone struct {
	Date     string `json:"date"`
	Title    string `json:"title"`
	Category string `json:"category"`
	Blog     string `json:"blog"`
	Blurb    string `json:"blurb"`
}

// milestoneMonth holds one month's key points and update reports. Date is the
// calendar month in YYYY-MM form, which sorts in chronological order.
type milestoneMonth struct {
	Date    string
	Label   string
	Items   []milestone
	Reports []changeWeek
}

// groupMilestones groups key points and reports by their starting month, newest
// first. Equal milestone dates keep the source order. A report spanning two
// months belongs only to the month it started in.
func groupMilestones(milestones []milestone, reports []changeWeek) ([]milestoneMonth, error) {
	months := make(map[string]*milestoneMonth)
	monthFor := func(date string) (*milestoneMonth, error) {
		parsed, err := time.Parse(dataDateLayout, date)
		if err != nil {
			return nil, fmt.Errorf("date %q: %w", date, err)
		}
		key := parsed.Format("2006-01")
		month, found := months[key]
		if !found {
			month = &milestoneMonth{Date: key, Label: parsed.Format("January 2006")}
			months[key] = month
		}
		return month, nil
	}
	sorted := append([]milestone{}, milestones...)
	sort.SliceStable(sorted, func(left, right int) bool {
		return sorted[left].Date > sorted[right].Date
	})
	for _, item := range sorted {
		month, err := monthFor(item.Date)
		if err != nil {
			return nil, fmt.Errorf("data/%s: milestone %q states date %q: %w",
				milestonesDataFile, item.Title, item.Date, err)
		}
		if err := validateSubjectCategory(item.Category); err != nil {
			return nil, fmt.Errorf("data/%s: milestone %q: %w", milestonesDataFile, item.Title, err)
		}
		month.Items = append(month.Items, item)
	}
	seen := make(map[string]bool, len(reports))
	for _, report := range reports {
		if seen[report.Slug] {
			continue
		}
		month, err := monthFor(report.CoverageStart)
		if err != nil {
			return nil, fmt.Errorf("update report %q: %w", report.Slug, err)
		}
		seen[report.Slug] = true
		month.Reports = append(month.Reports, report)
	}
	ordered := make([]milestoneMonth, 0, len(months))
	for _, month := range months {
		ordered = append(ordered, *month)
	}
	sort.SliceStable(ordered, func(left, right int) bool {
		return ordered[left].Date > ordered[right].Date
	})
	return ordered, nil
}

// milestoneReportLabel keeps footer links short; the full coverage range is
// carried by the link's accessible label and title.
func milestoneReportLabel(report changeWeek) (string, error) {
	parsed, err := time.Parse(dataDateLayout, report.CoverageStart)
	if err != nil {
		return "", fmt.Errorf("update report %q: %w", report.Slug, err)
	}
	return parsed.Format("2 Jan"), nil
}

// milestonesStyle is the timeline's own stylesheet. It is the one page whose
// layout no shared rule covers, so the rules travel with the page that needs
// them rather than growing site.css for a single reader.
const milestonesStyle = `        <style>
            main.has-page-sidebar:has(> .tl-overview) {
                display: block;
                width: min(72rem, calc(100% - 2rem));
                max-width: 72rem;
            }
            .tl-overview { margin: 0 0 1rem; }
            .tl-overview .journey-hero {
                padding: 1.25rem 0;
                margin-bottom: 0.8rem;
                min-height: 0;
            }
            .tl-overview .journey-hero h1 {
                margin: 0;
                font-size: clamp(1.8rem, 3.2vw, 2.5rem);
                color: var(--hero-text);
                background: none;
                -webkit-text-fill-color: currentColor;
            }
            .tl-overview .journey-hero p {
                margin-top: 0.45rem;
                max-width: 52rem;
                font-size: 1rem;
                line-height: 1.5;
            }
            .tl-toolbar {
                display: flex;
                flex-wrap: wrap;
                align-items: center;
                gap: 0.5rem 1rem;
                margin-bottom: 0.5rem;
                font-size: 0.82rem;
                color: var(--muted);
            }
            .tl-toolbar p { margin: 0; }
            .tl-overview .legend { margin: 0; gap: 0.4rem; }
            .tl-timeline {
                display: grid;
                grid-template-columns: repeat(2, minmax(0, 1fr));
                align-items: start;
                gap: 1rem;
            }
            .tl-card {
                min-width: 0;
                padding: 1rem 1.15rem;
                background: var(--bg-soft);
                border: 1px solid var(--line);
                border-radius: 12px;
            }
            .tl-month-title { margin: 0 0 0.8rem; font-size: 1.2rem; scroll-margin-top: 6rem; }
            .tl-section-title {
                margin: 0 0 0.5rem;
                font-size: 0.75rem;
                text-transform: uppercase;
                letter-spacing: 0.06em;
                color: var(--muted);
            }
            .tl-points {
                list-style: none;
                margin: 0;
                padding: 0;
                display: grid;
                gap: 0.5rem;
            }
            .tl-item {
                display: flex;
                flex-wrap: wrap;
                align-items: baseline;
                gap: 0.3rem 0.75rem;
                border-left: 3px solid var(--acc, var(--line-strong));
                padding-left: 0.75rem;
            }
            .tl-item.filtered-out { display: none; }
            .tl-title { font-weight: 600; }
            .tl-cat {
                font-size: 0.68rem;
                font-weight: 700;
                letter-spacing: 0.06em;
                text-transform: uppercase;
                color: var(--acc-deep, var(--muted));
            }
            .tl-no-landmark { margin: 0; color: var(--muted); }
            .tl-reports {
                margin-top: 1rem;
                padding-top: 0.8rem;
                border-top: 1px solid color-mix(in srgb, var(--line-strong) 60%, var(--text));
                display: flex;
                flex-wrap: wrap;
                align-items: baseline;
                gap: 0.5rem 1rem;
            }
            .tl-reports-label { font-size: 0.75rem; color: var(--muted); }
            .tl-link {
                font-size: 0.85rem;
                font-weight: 600;
                text-decoration: none;
            }
            .tl-link:hover { text-decoration: underline; }
            main.has-page-sidebar:has(> .tl-overview) > .page-sidebar {
                position: static;
                top: auto;
                width: auto;
                max-width: none;
                max-height: none;
                margin: 1.25rem 0 0;
                overflow: visible;
            }
            main.has-page-sidebar:has(> .tl-overview) .page-sidebar-nav {
                grid-template-columns: repeat(2, minmax(0, 1fr));
                align-items: start;
                gap: 1.5rem;
            }
            @media (max-width: 760px) {
                .tl-timeline { grid-template-columns: minmax(0, 1fr); }
                .tl-card { padding: 1rem; }
                main.has-page-sidebar:has(> .tl-overview) .page-sidebar-nav {
                    grid-template-columns: minmax(0, 1fr);
                }
            }
        </style>
`

// renderMilestones publishes the timeline page and its mirror.
func renderMilestones(paths Paths) ([]string, error) {
	var data milestoneData
	if err := readSourceJSON(paths.Source, milestonesDataFile, &data); err != nil {
		return nil, err
	}
	if data.Intro == "" {
		return nil, fmt.Errorf("data/%s states no intro", milestonesDataFile)
	}
	vocabulary, err := loadTopicVocabulary(paths.Source)
	if err != nil {
		return nil, err
	}
	reports, err := loadChangeWeeks(paths.Source, vocabulary)
	if err != nil {
		return nil, err
	}
	months, err := groupMilestones(data.Milestones, reports)
	if err != nil {
		return nil, err
	}
	links, err := loadPageLinks(paths.Source)
	if err != nil {
		return nil, err
	}

	description := "Landmark features in Ze's development, newest to oldest, with links to every update report."
	shell := pageShell{
		Title:       "Milestones - Ze",
		Description: description,
		Root:        milestonesRoot,
		Path:        milestonesDest,
		ExtraHead:   milestonesStyle,
		Sidebar:     pageSidebar(milestonesRoot, milestonesDest, links),
	}
	body, err := milestonesBody(data, months)
	if err != nil {
		return nil, err
	}
	mirror, err := milestonesMirror(data, months)
	if err != nil {
		return nil, err
	}
	if err := writePublishedPage(paths.Output, milestonesDest, shell.render(body), mirror); err != nil {
		return nil, err
	}
	return []string{milestonesRoute}, nil
}

// milestonesBody renders the page under <main>: the hero and its filter legend,
// then one card per month, newest first.
func milestonesBody(data milestoneData, months []milestoneMonth) (string, error) {
	var body textbuf.Buffer
	body.Str("            <section class=\"tl-overview\" aria-labelledby=\"milestones-title\">\n")
	body.Str("                <div class=\"section-head journey-hero reveal\">\n")
	body.Str("                    <h1 id=\"milestones-title\">Milestones</h1>\n")
	body.Str("                    <p>").Str(html.EscapeString(data.Intro)).Str("</p>\n")
	body.Str("                </div>\n")
	body.Str("                <div class=\"tl-toolbar\"><span>").Int(int64(len(data.Milestones))).
		Str(" milestones</span><p>Select again to clear.</p>\n")
	body.Str("                </div>\n")
	body.Str("                <div class=\"legend reveal\" role=\"group\" aria-label=\"Filter milestones by category\">\n")
	for _, category := range legendCategories {
		body.Str("                    <button class=\"cat-").Str(category).Str("\" data-cat=\"").Str(category).
			Str("\" aria-pressed=\"false\">").Str(html.EscapeString(categoryLabel(category))).Str("</button>\n")
	}
	body.Str("                </div>\n")
	body.Str("            </section>\n\n")

	body.Str("            <section class=\"tl-timeline reveal\" aria-label=\"Milestone timeline\">\n")
	for _, month := range months {
		card, err := milestoneMonthHTML(month)
		if err != nil {
			return "", err
		}
		body.Str(card)
	}
	body.Str("            </section>\n")
	return body.String(), nil
}

// milestoneMonthHTML renders the month's concise overview and report footer.
func milestoneMonthHTML(month milestoneMonth) (string, error) {
	var out textbuf.Buffer
	out.Str("                <article class=\"tl-card tl-month\" data-month=\"").Str(month.Date).
		Str("\" aria-labelledby=\"month-").Str(month.Date).Str("\">\n")
	out.Str("                    <h2 class=\"tl-month-title\" id=\"month-").Str(month.Date).
		Str("\"><time datetime=\"").Str(month.Date).Str("\">").Str(month.Label).Str("</time></h2>\n")
	out.Str("                    <h3 class=\"tl-section-title\">Key points</h3>\n")
	if len(month.Items) == 0 {
		out.Str("                    <p class=\"tl-no-landmark\">No milestone recorded for this month.</p>\n")
	}
	if len(month.Items) > 0 {
		out.Str("                    <ul class=\"tl-points\">\n")
		for _, item := range month.Items {
			category := html.EscapeString(item.Category)
			out.Str("                        <li class=\"tl-item cat-").Str(category).
				Str("\" data-cat=\"").Str(category).Str("\"><span class=\"tl-title\">").
				Str(html.EscapeString(item.Title)).Str("</span><span class=\"tl-cat\">").
				Str(html.EscapeString(strings.ToLower(categoryLabel(item.Category)))).Str("</span></li>\n")
		}
		out.Str("                    </ul>\n")
	}
	if len(month.Reports) > 0 {
		out.Str("                    <footer class=\"tl-reports\"><span class=\"tl-reports-label\">Update reports</span>\n")
		for _, report := range month.Reports {
			label, err := milestoneReportLabel(report)
			if err != nil {
				return "", err
			}
			out.Str("                        <a class=\"tl-link\" href=\"../changes/").Str(html.EscapeString(report.Slug)).
				Str("/\" title=\"").Str(html.EscapeString(report.label())).Str("\" aria-label=\"Update report ").
				Str(html.EscapeString(report.label())).Str("\">").Str(label).Str("</a>\n")
		}
		out.Str("                    </footer>\n")
	}
	out.Str("                </article>\n")
	return out.String(), nil
}

// milestonesMirror uses the same monthly groups and concise key points as HTML.
func milestonesMirror(data milestoneData, months []milestoneMonth) (string, error) {
	var mirror textbuf.Buffer
	mirror.Str("# Milestones\n\n")
	mirror.Str(data.Intro).Str("\n\n")
	for _, month := range months {
		mirror.Str("## ").Str(month.Label).Str("\n\n### Key points\n\n")
		if len(month.Items) == 0 {
			mirror.Str("No milestone recorded for this month.\n")
		}
		for _, item := range month.Items {
			mirror.Str("- ").Str(item.Title).Str(" (").Str(strings.ToLower(categoryLabel(item.Category))).Str(")\n")
		}
		if len(month.Reports) > 0 {
			mirror.Str("\n### Update reports\n\n")
			for _, report := range month.Reports {
				label, err := milestoneReportLabel(report)
				if err != nil {
					return "", err
				}
				mirror.Str("- [").Str(label).Str("](../changes/").Str(report.Slug).Str("/)\n")
			}
		}
		mirror.Byte('\n')
	}
	return strings.TrimSpace(mirror.String()) + "\n", nil
}

// Where the talks listing reads and writes.
//
// This producer owns the LISTING alone. Every talks/<slug>/ deck is authored,
// bundled by refreshTalks, and frozen: the retired build excluded any talks/
// path whose second segment is not index.html from every pass it ran, and the
// listing is the one page that exclusion lets through.
const (
	talksDataFile  = "talks.json"
	talksDirectory = "talks"
	talksDest      = talksDirectory + "/" + pageIndexFile
	talksRoot      = "../"
	talksRoute     = "/" + talksDirectory + "/"
)

// talkData is data/talks.json: one entry for each presentation, in whatever
// order the file states. The page sorts them.
type talkData struct {
	Talks []talkEntry `json:"talks"`
}

// talkEntry is one presentation. Slug names its deck directory, which this
// producer links and never writes.
type talkEntry struct {
	Slug  string `json:"slug"`
	Venue string `json:"venue"`
	Title string `json:"title"`
	Date  string `json:"date"`
}

// renderTalkIndex publishes the talks listing and its mirror.
func renderTalkIndex(paths Paths) ([]string, error) {
	var data talkData
	if err := readSourceJSON(paths.Source, talksDataFile, &data); err != nil {
		return nil, err
	}
	for _, talk := range data.Talks {
		if talk.Slug == "" || talk.Venue == "" || talk.Title == "" {
			return nil, fmt.Errorf("data/%s: a talk states no slug, venue or title", talksDataFile)
		}
		if _, err := time.Parse(dataDateLayout, talk.Date); err != nil {
			return nil, fmt.Errorf("data/%s: talk %q states date %q: %w",
				talksDataFile, talk.Title, talk.Date, err)
		}
	}

	// Newest first, and stable so two talks on one date keep the file's order.
	talks := append([]talkEntry{}, data.Talks...)
	sort.SliceStable(talks, func(left, right int) bool { return talks[left].Date > talks[right].Date })

	shell := pageShell{
		Title:       "Talks - Ze",
		Description: "Talks and presentations about Ze.",
		Root:        talksRoot,
		Path:        talksDest,
	}
	body, err := talksBody(talks)
	if err != nil {
		return nil, err
	}
	mirror, err := talksMirror(talks)
	if err != nil {
		return nil, err
	}
	if err := writePublishedPage(paths.Output, talksDest, shell.render(body), mirror); err != nil {
		return nil, err
	}
	return []string{talksRoute}, nil
}

// talksBody renders the listing under <main>: one card for each talk, newest
// first, each linking its deck and the standalone download beside it.
func talksBody(talks []talkEntry) (string, error) {
	var body textbuf.Buffer
	body.Str("            <section id=\"talks\" aria-labelledby=\"talks-title\">\n")
	body.Str(pageHero("Talks and presentations.", "Sharing Ze with the community.",
		journeyCommunity, ` id="talks-title"`, heroClasses)).Byte('\n')
	body.Str("                <div class=\"audience reveal\">\n")
	for _, talk := range talks {
		date, err := displayDate(talk.Date)
		if err != nil {
			return "", fmt.Errorf("talk %q: %w", talk.Title, err)
		}
		slug := html.EscapeString(talk.Slug)
		body.Str("                    <article class=\"audience-card\">\n")
		body.Str("                        <a href=\"").Str(slug).Str("/\" class=\"talk-link\">\n")
		body.Str("                            <h3>").Str(html.EscapeString(talk.Venue)).Str("</h3>\n")
		body.Str("                            <p>").Str(html.EscapeString(talk.Title)).Str("</p>\n")
		body.Str("                            <p class=\"talk-date\">").Str(date).Str("</p>\n")
		body.Str("                        </a>\n")
		body.Str("                        <p class=\"talk-alt\"><a href=\"").Str(slug).
			Str("/index-inlined.html\" download>Download standalone HTML deck</a></p>\n")
		body.Str("                    </article>\n")
	}
	body.Str("                </div>\n")
	body.Str("            </section>\n")
	return body.String(), nil
}

// talksMirror renders the Markdown sibling. Its links are absolute, because a
// reader reaches a mirror from anywhere.
func talksMirror(talks []talkEntry) (string, error) {
	var mirror textbuf.Buffer
	mirror.Str("# Talks and presentations.\n\nSharing Ze with the community.\n\n")
	for _, talk := range talks {
		date, err := displayDate(talk.Date)
		if err != nil {
			return "", fmt.Errorf("talk %q: %w", talk.Title, err)
		}
		mirror.Str("## ").Str(talk.Venue).Str("\n\n")
		mirror.Str(talk.Title).Str(" -- ").Str(date).Str("\n\n")
		mirror.Str("[Watch](").Str(siteBase).Str("talks/").Str(talk.Slug).Str("/)\n")
		mirror.Str("[Download standalone HTML deck](").Str(siteBase).Str("talks/").Str(talk.Slug).
			Str("/index-inlined.html)\n\n")
	}
	return strings.TrimSpace(mirror.String()) + "\n", nil
}
