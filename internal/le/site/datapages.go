// Design: website/AI.md -- three pages whose whole content is one committed data file
// Detail: shell.go wraps each body; dependencies.go is the fourth data page and reads go.mod too.
package site

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
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
	var out strings.Builder
	rest := text
	for {
		start := strings.Index(rest, marker)
		if start < 0 {
			out.WriteString(rest)
			return out.String()
		}
		after := start + len(marker)
		end := strings.Index(rest[after:], marker)
		if end <= 0 {
			out.WriteString(rest)
			return out.String()
		}
		out.WriteString(rest[:start])
		out.WriteString(opening + rest[after:after+end] + closing)
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

// featureStatusLabels name the two maturities a card can carry. A card with no
// status is shipped, takes no badge, and counts as a feature.
var featureStatusLabels = map[string]string{
	"experimental": "Experimental",
	"aspiration":   "Spec'd",
}

// featureData is data/features.json: an ordered list of sections, each an
// ordered list of cards. Both orders are the file's own and are published
// verbatim, so nothing here is sorted.
type featureData struct {
	Sections []featureSection `json:"sections"`
}

// featureSection is one titled block of the features page.
type featureSection struct {
	ID      string        `json:"id"`
	Heading string        `json:"heading"`
	Lead    string        `json:"lead"`
	Note    string        `json:"note"`
	Cards   []featureCard `json:"cards"`
}

// featureCard is one feature. Status is empty for a shipped feature. External
// says whether Href leaves this site, which decides both the link target and
// whether the href is rewritten relative to the page.
type featureCard struct {
	Category string        `json:"category"`
	Status   string        `json:"status"`
	Title    string        `json:"title"`
	Href     string        `json:"href"`
	External bool          `json:"external"`
	Chips    []featureChip `json:"chips"`
	Bullets  []string      `json:"bullets"`
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
	if !pageCategories[card.Category] {
		return fmt.Errorf("%s: feature card %q states category %q, which is not one of the seven",
			where, card.Title, card.Category)
	}
	if card.Status != "" {
		if _, known := featureStatusLabels[card.Status]; !known {
			return fmt.Errorf("%s: feature card %q states status %q, which is neither experimental nor aspiration",
				where, card.Title, card.Status)
		}
	}
	if card.Href == "" {
		return fmt.Errorf("%s: feature card %q links nowhere", where, card.Title)
	}
	return nil
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

// section answers one section of the file by its id.
//
// The count in the page's own lead is core plus experimental, so a file missing
// either would publish a number that means something else. It is refused by
// name rather than counted as zero.
func (data featureData) section(id string) (featureSection, error) {
	for _, section := range data.Sections {
		if section.ID == id {
			return section, nil
		}
	}
	return featureSection{}, fmt.Errorf("data/%s declares no %q section", featuresDataFile, id)
}

// shippedCards answers the cards the page counts as features: the core and
// experimental sections, and never the roadmap, whose cards are specs.
func (data featureData) shippedCards() ([]featureCard, error) {
	core, err := data.section(featureSectionCore)
	if err != nil {
		return nil, err
	}
	experimental, err := data.section(featureSectionExperimental)
	if err != nil {
		return nil, err
	}
	return append(append([]featureCard{}, core.Cards...), experimental.Cards...), nil
}

// renderFeatures publishes the features page and its mirror.
func renderFeatures(paths Paths) ([]string, error) {
	var data featureData
	if err := readSourceJSON(paths.Source, featuresDataFile, &data); err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(data.Sections))
	for _, section := range data.Sections {
		if section.ID == "" {
			return nil, fmt.Errorf("data/%s carries a section with no id", featuresDataFile)
		}
		if seen[section.ID] {
			return nil, fmt.Errorf("data/%s declares two %q sections", featuresDataFile, section.ID)
		}
		seen[section.ID] = true
		for index := range section.Cards {
			if err := section.Cards[index].validate("data/" + featuresDataFile); err != nil {
				return nil, err
			}
		}
	}
	shipped, err := data.shippedCards()
	if err != nil {
		return nil, err
	}
	links, err := loadPageLinks(paths.Source)
	if err != nil {
		return nil, err
	}

	shell := pageShell{
		Title:       "Features - Ze",
		Description: "Every shipped feature and the planned roadmap, grouped by maturity and category.",
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

	var body strings.Builder
	body.WriteString("            <section aria-labelledby=\"features-title\">\n")
	body.WriteString(pageHero("Every feature Ze ships.",
		strconv.Itoa(len(shipped))+" shipped features plus the planned roadmap.",
		"Project", ` id="features-title"`, heroClasses) + "\n")
	body.WriteString("                <div class=\"section-note reveal\">\n")
	body.WriteString("                    <p>Each card&#39;s color is its category: how the feature fits " +
		"into the system. Solid cards are shipped; dashed cards are experimental; blueprint cards " +
		"at the bottom are specs, not code. Everything shipped runs in both daemon and appliance " +
		"modes unless a card says otherwise. Click a category to filter, click again to show " +
		"everything.</p>\n")
	body.WriteString("                </div>\n")
	body.WriteString("                <div class=\"legend reveal\" role=\"group\" aria-label=\"Filter features by category\">\n")
	for _, category := range legendCategories {
		label := capitalizeWord(category)
		count := strconv.Itoa(counts[category])
		body.WriteString("                    <button class=\"cat-" + category + "\" data-cat=\"" + category +
			"\" aria-pressed=\"false\" aria-label=\"Filter features by " + label + ", " + count +
			" features\">" + label + " <span class=\"legend-count\" aria-hidden=\"true\">" + count +
			"</span></button>\n")
	}
	body.WriteString("                </div>\n")
	body.WriteString("                <p id=\"feature-filter-status\" class=\"feature-filter-status search-status\" aria-live=\"polite\"></p>\n")
	body.WriteString("            </section>\n\n")

	for _, section := range data.Sections {
		body.WriteString(featureSectionHTML(section))
		body.WriteString("\n")
	}
	return body.String()
}

// featureSectionHTML renders one titled block: its heading, its lead, its
// optional note, and its cards in the file's own order.
func featureSectionHTML(section featureSection) string {
	var out strings.Builder
	out.WriteString("            <section id=\"" + html.EscapeString(section.ID) +
		"\" aria-labelledby=\"" + html.EscapeString(section.ID) + "-title\" data-cards>\n")
	out.WriteString("                <div class=\"section-head reveal\">\n")
	out.WriteString("                    <h2 id=\"" + html.EscapeString(section.ID) + "-title\">" +
		html.EscapeString(section.Heading) + "</h2>\n")
	out.WriteString("                    <p>" + html.EscapeString(section.Lead) + "</p>\n")
	out.WriteString("                </div>\n")
	if section.Note != "" {
		out.WriteString("                <div class=\"section-note reveal\">\n")
		out.WriteString("                    <p>" + inlineMarkup(section.Note) + "</p>\n")
		out.WriteString("                </div>\n")
	}
	out.WriteString("                <div class=\"cards feature-grid reveal\">\n")
	for index := range section.Cards {
		out.WriteString(featureCardHTML(&section.Cards[index]))
	}
	out.WriteString("                </div>\n")
	out.WriteString("            </section>\n")
	return out.String()
}

// featureCardHTML renders one card: its category badge, its maturity badge, its
// linked title, its chips and its bullets.
func featureCardHTML(card *featureCard) string {
	classes := "card feature-card"
	if card.Status != "" {
		classes += " " + card.Status
	}
	classes += " cat-" + card.Category

	var out strings.Builder
	out.WriteString("                    <article class=\"" + html.EscapeString(classes) +
		"\" data-cat=\"" + html.EscapeString(card.Category) + "\">\n")
	out.WriteString("                    <span class=\"cat\">" +
		html.EscapeString(capitalizeWord(card.Category)) + "</span>\n")
	if card.Status != "" {
		out.WriteString("                    <span class=\"status\">" +
			html.EscapeString(featureStatusLabels[card.Status]) + "</span>\n")
	}
	target := ""
	if card.External {
		target = ` target="_blank" rel="noopener"`
	}
	out.WriteString("                    <h3><a href=\"" + html.EscapeString(card.href()) + "\"" + target +
		">" + html.EscapeString(card.Title) + "</a></h3>\n")
	out.WriteString("                    <div class=\"chips\">\n")
	for _, chip := range card.Chips {
		class := "chip"
		if chip.Mode {
			class = "chip mode"
		}
		out.WriteString("                    <span class=\"" + class + "\">" +
			html.EscapeString(chip.Text) + "</span>\n")
	}
	out.WriteString("                    </div>\n")
	out.WriteString("                    <ul>\n")
	for _, bullet := range card.Bullets {
		out.WriteString("                    <li>" + inlineMarkup(bullet) + "</li>\n")
	}
	out.WriteString("                    </ul>\n")
	out.WriteString("                    </article>\n")
	return out.String()
}

// featuresMirror renders the Markdown sibling from the same data the page uses,
// so the two cannot disagree about what the site ships.
func featuresMirror(data featureData, shipped int) string {
	var mirror strings.Builder
	mirror.WriteString("# Every feature Ze ships.\n\n")
	mirror.WriteString(strconv.Itoa(shipped) + " shipped features plus the planned roadmap. " +
		"Each card's category shows where the feature fits: operate, routing, services, automate, " +
		"observe, secure, or platform. Everything shipped runs in both daemon and appliance modes " +
		"unless a card says otherwise.\n\n")
	for _, section := range data.Sections {
		mirror.WriteString("## " + section.Heading + "\n\n")
		mirror.WriteString(section.Lead + "\n\n")
		if section.Note != "" {
			mirror.WriteString("> " + section.Note + "\n\n")
		}
		for index := range section.Cards {
			card := &section.Cards[index]
			mirror.WriteString("### " + card.Title + "\n\n")
			meta := card.Category
			if card.Status != "" {
				meta += " / " + featureStatusLabels[card.Status]
			}
			line := "*" + meta + "*"
			if len(card.Chips) != 0 {
				chips := make([]string, 0, len(card.Chips))
				for _, chip := range card.Chips {
					chips = append(chips, "`"+chip.Text+"`")
				}
				line += " -- " + strings.Join(chips, " ")
			}
			mirror.WriteString(line + "\n\n")
			for _, bullet := range card.Bullets {
				mirror.WriteString("- " + bullet + "\n")
			}
			mirror.WriteString("\n[Learn more](" + card.mirrorHref() + ")\n\n")
		}
	}
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

// milestoneQuarter is one run of milestones that share a quarter, in the order
// the sorted list produced them.
type milestoneQuarter struct {
	Label string
	Items []milestone
}

// groupMilestones sorts newest first and then splits the sorted list into runs
// of one quarter each.
//
// The grouping is run-length over the SORTED list rather than a lookup keyed by
// quarter, which is what makes a quarter appear once: two milestones of one
// quarter are always adjacent after the sort. The sort is stable, so two
// milestones sharing a date stay in the file's own order.
func groupMilestones(milestones []milestone) ([]milestoneQuarter, error) {
	for _, item := range milestones {
		if _, err := time.Parse(dataDateLayout, item.Date); err != nil {
			return nil, fmt.Errorf("data/%s: milestone %q states date %q: %w",
				milestonesDataFile, item.Title, item.Date, err)
		}
		if !pageCategories[item.Category] {
			return nil, fmt.Errorf("data/%s: milestone %q states category %q, which is not one of the seven",
				milestonesDataFile, item.Title, item.Category)
		}
	}

	sorted := append([]milestone{}, milestones...)
	sort.SliceStable(sorted, func(left, right int) bool {
		return sorted[left].Date > sorted[right].Date
	})

	var quarters []milestoneQuarter
	previous := ""
	for _, item := range sorted {
		label, err := quarterLabel(item.Date)
		if err != nil {
			return nil, err
		}
		if label != previous {
			quarters = append(quarters, milestoneQuarter{Label: label})
			previous = label
		}
		last := len(quarters) - 1
		quarters[last].Items = append(quarters[last].Items, item)
	}
	return quarters, nil
}

// quarterLabel answers the heading one date sits under: "2026-04-13" is
// "Q2 2026".
func quarterLabel(date string) (string, error) {
	parsed, err := time.Parse(dataDateLayout, date)
	if err != nil {
		return "", fmt.Errorf("date %q: %w", date, err)
	}
	quarter := (int(parsed.Month())-1)/3 + 1
	return "Q" + strconv.Itoa(quarter) + " " + strconv.Itoa(parsed.Year()), nil
}

// monthLabel answers the date on the spine beside one milestone: "Jul 2026".
func monthLabel(date string) (string, error) {
	parsed, err := time.Parse(dataDateLayout, date)
	if err != nil {
		return "", fmt.Errorf("date %q: %w", date, err)
	}
	return parsed.Format("Jan 2006"), nil
}

// milestonesStyle is the timeline's own stylesheet. It is the one page whose
// layout no shared rule covers, so the rules travel with the page that needs
// them rather than growing site.css for a single reader.
const milestonesStyle = `        <style>
            .tl-quarter { margin: 0 0 0.5rem; }
            .tl-quarter-head {
                font-size: 0.82rem;
                font-weight: 700;
                letter-spacing: 0.12em;
                text-transform: uppercase;
                color: var(--muted);
                margin: 2.2rem 0 0.9rem;
                padding-left: 1.75rem;
            }
            .tl-list {
                list-style: none;
                margin: 0;
                padding: 0 0 0 1.75rem;
                position: relative;
            }
            /* The spine: one continuous line down the left of each quarter. */
            .tl-list::before {
                content: "";
                position: absolute;
                left: 6px;
                top: 6px;
                bottom: 6px;
                width: 2px;
                background: var(--line-strong);
                border-radius: 2px;
            }
            .tl-item {
                position: relative;
                padding: 0 0 1.4rem 0;
            }
            .tl-item.filtered-out { display: none; }
            .tl-node {
                position: absolute;
                left: -1.75rem;
                top: 0.35rem;
                width: 14px;
                height: 14px;
                border-radius: 50%;
                background: var(--acc, var(--muted));
                border: 3px solid var(--bg);
                box-shadow: 0 0 0 2px var(--acc, var(--muted));
            }
            .tl-date {
                font-size: 0.78rem;
                font-weight: 700;
                letter-spacing: 0.04em;
                color: var(--acc-deep, var(--muted));
                text-transform: uppercase;
            }
            .tl-card {
                margin-top: 0.25rem;
                padding: 0.9rem 1.1rem;
                background: var(--acc-tint, var(--bg-soft));
                border: 1px solid var(--line);
                border-left: 3px solid var(--acc, var(--line-strong));
                border-radius: 10px;
            }
            .tl-head {
                display: flex;
                flex-wrap: wrap;
                align-items: baseline;
                gap: 0.6rem;
                margin-bottom: 0.35rem;
            }
            .tl-title { margin: 0; font-size: 1.06rem; }
            .tl-cat {
                font-size: 0.68rem;
                font-weight: 700;
                letter-spacing: 0.06em;
                text-transform: uppercase;
                color: var(--acc-deep, var(--muted));
            }
            .tl-card p { margin: 0 0 0.5rem; color: var(--text); }
            .tl-link {
                font-size: 0.85rem;
                font-weight: 600;
                text-decoration: none;
                color: var(--acc-deep, var(--muted));
            }
            .tl-link:hover { text-decoration: underline; }
            @media (max-width: 640px) {
                .tl-quarter-head, .tl-list { padding-left: 1.4rem; }
                .tl-node { left: -1.4rem; }
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
	quarters, err := groupMilestones(data.Milestones)
	if err != nil {
		return nil, err
	}
	links, err := loadPageLinks(paths.Source)
	if err != nil {
		return nil, err
	}

	description := "The landmark features Ze has shipped, newest first: one node per " +
		"capability the first time it arrived, on a timeline."
	shell := pageShell{
		Title:       "Milestones - Ze",
		Description: description,
		Root:        milestonesRoot,
		Path:        milestonesDest,
		ExtraHead:   milestonesStyle,
		Sidebar:     pageSidebar(milestonesRoot, milestonesDest, links),
	}
	body, err := milestonesBody(data, quarters)
	if err != nil {
		return nil, err
	}
	mirror, err := milestonesMirror(data, quarters)
	if err != nil {
		return nil, err
	}
	if err := writePublishedPage(paths.Output, milestonesDest, shell.render(body), mirror); err != nil {
		return nil, err
	}
	return []string{milestonesRoute}, nil
}

// milestonesBody renders the page under <main>: the hero and its filter legend,
// then one block for each quarter, newest first.
func milestonesBody(data milestoneData, quarters []milestoneQuarter) (string, error) {
	var body strings.Builder
	body.WriteString("            <section aria-labelledby=\"milestones-title\">\n")
	body.WriteString("                <div class=\"section-head journey-hero reveal\">\n")
	body.WriteString("                    <span class=\"journey-eyebrow\">Timeline</span>\n")
	body.WriteString("                    <h1 id=\"milestones-title\">The road so far.</h1>\n")
	body.WriteString("                    <p>" + strconv.Itoa(len(data.Milestones)) +
		" milestones, newest first. " + html.EscapeString(data.Intro) + "</p>\n")
	body.WriteString("                </div>\n")
	body.WriteString("                <div class=\"section-note reveal\">\n")
	body.WriteString("                    <p>Each node&#39;s color is its category. This is the coarse " +
		"view: the <a href=\"../changes/\">Changes</a> log has every week, and " +
		"<a href=\"../../features/\">Features</a> lists what ships today. Click a category to " +
		"filter, click again to show everything.</p>\n")
	body.WriteString("                </div>\n")
	body.WriteString("                <div class=\"legend reveal\" role=\"group\" aria-label=\"Filter milestones by category\">\n")
	for _, category := range legendCategories {
		body.WriteString("                    <button class=\"cat-" + category + "\" data-cat=\"" +
			category + "\" aria-pressed=\"false\">" + capitalizeWord(category) + "</button>\n")
	}
	body.WriteString("                </div>\n")
	body.WriteString("            </section>\n\n")

	body.WriteString("            <section class=\"reveal\" aria-label=\"Milestone timeline\">\n")
	for _, quarter := range quarters {
		body.WriteString("                <div class=\"tl-quarter\" data-quarter>\n")
		body.WriteString("                    <h2 class=\"tl-quarter-head\">" +
			html.EscapeString(quarter.Label) + "</h2>\n")
		body.WriteString("                    <ol class=\"tl-list\">\n")
		for _, item := range quarter.Items {
			node, err := milestoneItemHTML(item)
			if err != nil {
				return "", err
			}
			body.WriteString(node)
		}
		body.WriteString("                    </ol>\n")
		body.WriteString("                </div>\n")
	}
	body.WriteString("            </section>\n")
	return body.String(), nil
}

// milestoneItemHTML renders one node of the spine.
func milestoneItemHTML(item milestone) (string, error) {
	month, err := monthLabel(item.Date)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	category := html.EscapeString(item.Category)
	out.WriteString("                    <li class=\"tl-item cat-" + category + "\" data-cat=\"" +
		category + "\">\n")
	out.WriteString("                        <span class=\"tl-node\" aria-hidden=\"true\"></span>\n")
	out.WriteString("                        <div class=\"tl-date\"><time datetime=\"" + item.Date + "\">" +
		month + "</time></div>\n")
	out.WriteString("                        <div class=\"tl-card\">\n")
	out.WriteString("                            <div class=\"tl-head\">\n")
	out.WriteString("                                <h3 class=\"tl-title\">" +
		html.EscapeString(item.Title) + "</h3>\n")
	out.WriteString("                                <span class=\"tl-cat\">" +
		html.EscapeString(item.Category) + "</span>\n")
	out.WriteString("                            </div>\n")
	out.WriteString("                            <p>" + inlineMarkup(item.Blurb) + "</p>\n")
	if item.Blog != "" {
		out.WriteString("                            <a class=\"tl-link\" href=\"../changes/" +
			html.EscapeString(item.Blog) + "/\">Read the week &rarr;</a>\n")
	}
	out.WriteString("                        </div>\n")
	out.WriteString("                    </li>\n")
	return out.String(), nil
}

// milestonesMirror renders the Markdown sibling from the same grouping the page
// uses, so the two cannot disagree about which quarter a landmark sits in.
func milestonesMirror(data milestoneData, quarters []milestoneQuarter) (string, error) {
	var mirror strings.Builder
	mirror.WriteString("# Milestones\n\n")
	mirror.WriteString(data.Intro + "\n\n")
	for _, quarter := range quarters {
		mirror.WriteString("## " + quarter.Label + "\n\n")
		for _, item := range quarter.Items {
			month, err := monthLabel(item.Date)
			if err != nil {
				return "", err
			}
			mirror.WriteString("### " + item.Title + " (" + month + ")\n\n")
			mirror.WriteString("*" + item.Category + "*\n\n")
			mirror.WriteString(item.Blurb + "\n")
			if item.Blog != "" {
				mirror.WriteString("\n[Read the week](../changes/" + item.Blog + "/)\n")
			}
			mirror.WriteString("\n")
		}
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
	var body strings.Builder
	body.WriteString("            <section id=\"talks\" aria-labelledby=\"talks-title\">\n")
	body.WriteString(pageHero("Talks and presentations.", "Sharing Ze with the community.",
		journeyCommunity, ` id="talks-title"`, heroClasses) + "\n")
	body.WriteString("                <div class=\"audience reveal\">\n")
	for _, talk := range talks {
		date, err := displayDate(talk.Date)
		if err != nil {
			return "", fmt.Errorf("talk %q: %w", talk.Title, err)
		}
		slug := html.EscapeString(talk.Slug)
		body.WriteString("                    <article class=\"audience-card\">\n")
		body.WriteString("                        <a href=\"" + slug + "/\" class=\"talk-link\">\n")
		body.WriteString("                            <h3>" + html.EscapeString(talk.Venue) + "</h3>\n")
		body.WriteString("                            <p>" + html.EscapeString(talk.Title) + "</p>\n")
		body.WriteString("                            <p class=\"talk-date\">" + date + "</p>\n")
		body.WriteString("                        </a>\n")
		body.WriteString("                        <p class=\"talk-alt\"><a href=\"" + slug +
			"/index-inlined.html\" download>Download standalone HTML deck</a></p>\n")
		body.WriteString("                    </article>\n")
	}
	body.WriteString("                </div>\n")
	body.WriteString("            </section>\n")
	return body.String(), nil
}

// talksMirror renders the Markdown sibling. Its links are absolute, because a
// reader reaches a mirror from anywhere.
func talksMirror(talks []talkEntry) (string, error) {
	var mirror strings.Builder
	mirror.WriteString("# Talks and presentations.\n\nSharing Ze with the community.\n\n")
	for _, talk := range talks {
		date, err := displayDate(talk.Date)
		if err != nil {
			return "", fmt.Errorf("talk %q: %w", talk.Title, err)
		}
		mirror.WriteString("## " + talk.Venue + "\n\n")
		mirror.WriteString(talk.Title + " -- " + date + "\n\n")
		mirror.WriteString("[Watch](" + siteBase + "talks/" + talk.Slug + "/)\n")
		mirror.WriteString("[Download standalone HTML deck](" + siteBase + "talks/" + talk.Slug +
			"/index-inlined.html)\n\n")
	}
	return strings.TrimSpace(mirror.String()) + "\n", nil
}
