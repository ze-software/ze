// Design: website/AI.md -- the homepage is authored copy around six data slots
// Detail: homebody.go holds the template; facts.go holds the numbers it shows.
package site

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
)

// The homepage registers from here.
func init() {
	registerProducer(Producer{Name: homeProducerName, Render: renderHome})
}

// Where the homepage is published, and what its hero replays.
const (
	homeProducerName = "home"
	homeRoute        = "/"
	homeDest         = pageIndexFile
	// homeRoot is empty because the homepage IS the site root, so every link
	// it carries is already relative to itself.
	homeRoot      = ""
	homeHeroDemo  = "cli-dashboard"
	homeHeroLabel = "Operate BGP from the live dashboard demonstration"
)

// The homepage's two data files, under website/data/.
const (
	audienceDataFile = "audience.json"
	whatsNewDataFile = "whats-new.json"
)

// homeTeaserTones are the presentation colors the three weekly-update teasers
// cycle through, by position rather than by content: a tone says nothing about
// what a week contains, which is why it is not a category.
var homeTeaserTones = []string{toneSky, tonePink, toneGrape}

// homeTeasers and homeUpdateTags bound the two lists the momentum band shows.
// Both bounds are the page's, not the data's: a fourth teaser pushes the proof
// strip off a laptop screen, and a fifth tag wraps the card.
const (
	homeTeasers    = 3
	homeUpdateTags = 4
)

// homeSummaryRunes is how much of a summary the "Latest news" band shows. The
// band sits above the fold and the proof cards under it have to stay visible,
// so one line each is the whole budget.
const homeSummaryRunes = 108

// audienceData is website/data/audience.json: the two card grids the homepage
// shows, each in the file's own order.
//
// Run is the "Run paths" grid and Who is the "Safe ways to try Ze" grid. The
// order is the author's, so nothing here sorts.
type audienceData struct {
	Run []audienceCard `json:"run"`
	Who []audienceCard `json:"who"`
}

// audienceCard is one card of either grid.
//
// Category colors the card and Tone overrides that color without claiming the
// card belongs to a category. Link is optional: a card with none shows no call
// to action.
type audienceCard struct {
	Title    string        `json:"title"`
	Label    string        `json:"label"`
	Category string        `json:"category"`
	Tone     string        `json:"tone"`
	Chips    []string      `json:"chips"`
	Body     string        `json:"body"`
	Link     *audienceLink `json:"link"`
}

// audienceLink is a card's call to action: where it goes, what it says, and the
// smaller line beside it.
type audienceLink struct {
	Href     string `json:"href"`
	Label    string `json:"label"`
	Sublabel string `json:"sublabel"`
}

// whatsNewData is website/data/whats-new.json: the band's heading, its "all
// updates" link, and the one freeform note an author writes by hand.
//
// The other two slots are generated from the newest article and the newest
// week, so this file carries only what nothing else can answer.
type whatsNewData struct {
	Title string        `json:"title"`
	Link  *audienceLink `json:"link"`
	Note  *whatsNewNote `json:"note"`
}

// whatsNewNote is the hand-written slot of the band. Dropping the whole object
// renders the band with its two generated slots.
type whatsNewNote struct {
	Label string        `json:"label"`
	Tone  string        `json:"tone"`
	Title string        `json:"title"`
	Body  string        `json:"body"`
	Link  *audienceLink `json:"link"`
}

// renderHome publishes the homepage and its Markdown mirror.
//
// It reads the facts snapshot from the ARTIFACT rather than deriving numbers of
// its own, because refreshNativeSurfaces writes that snapshot before any
// producer runs. One derivation, so the homepage and every page carrying a
// {{ze:...}} token state one number.
func renderHome(paths Paths) ([]string, error) {
	var audience audienceData
	if err := readSourceJSON(paths.Source, audienceDataFile, &audience); err != nil {
		return nil, err
	}
	if len(audience.Run) == 0 {
		return nil, fmt.Errorf("data/%s states no run path, so the homepage would show an empty grid", audienceDataFile)
	}
	if len(audience.Who) == 0 {
		return nil, fmt.Errorf("data/%s states no use case, so the homepage would show an empty grid", audienceDataFile)
	}
	var whatsNew whatsNewData
	if err := readSourceJSON(paths.Source, whatsNewDataFile, &whatsNew); err != nil {
		return nil, err
	}
	if whatsNew.Title == "" {
		return nil, fmt.Errorf("data/%s states no title, so the Latest news band would have no heading", whatsNewDataFile)
	}
	if whatsNew.Link == nil {
		return nil, fmt.Errorf("data/%s states no link, so the Latest news heading would lead nowhere", whatsNewDataFile)
	}
	var features featureData
	if err := readSourceJSON(paths.Source, featuresDataFile, &features); err != nil {
		return nil, err
	}
	var facts siteFacts
	if err := readArtifactJSON(paths.Output, factsFile, &facts); err != nil {
		return nil, err
	}
	articles, err := loadBlogArticles(paths.Source)
	if err != nil {
		return nil, err
	}
	vocabulary, err := loadTopicVocabulary(paths.Source)
	if err != nil {
		return nil, err
	}
	weeks, err := loadChangeWeeks(paths.Source, vocabulary)
	if err != nil {
		return nil, err
	}
	hero, err := newDemoCatalog(paths).heroMount(homeHeroDemo, homeRoot, homeHeroLabel)
	if err != nil {
		return nil, err
	}
	stats, err := homeProofStats(&facts)
	if err != nil {
		return nil, err
	}

	body, err := homeBody(&audience, &whatsNew, features, articles, weeks, hero, stats)
	if err != nil {
		return nil, err
	}
	shell := pageShell{
		Title: "Ze - Configuration and Protocol Engine for Internet Infrastructure",
		Description: "Ze is an open-source configuration and protocol engine. The network " +
			"operating system built on it speaks BGP, manages Linux interfaces, " +
			"programs the FIB, and serves config over SSH, web, API, and MCP.",
		SocialDescription: "Open-source configuration and protocol engine with a " +
			"protocol-agnostic core, YANG-modeled subsystems, operator " +
			"interfaces, runnable labs, and an ExaBGP migration path.",
		Root:      homeRoot,
		Path:      homeDest,
		ExtraHead: demoPlayerHead(homeRoot),
	}
	page := shell.render(body)
	mirror, err := homeMirror(page)
	if err != nil {
		return nil, err
	}
	if err := writePublishedPage(paths.Output, homeDest, page, mirror); err != nil {
		return nil, err
	}
	return []string{homeRoute}, nil
}

// homeMirror answers the homepage's index.md.
//
// It is converted back from the rendered page rather than written a second
// time, because the homepage has no Markdown source: the copy lives in
// homeTemplate and the numbers come from the snapshot. A hand-written mirror
// would be a second statement of both, and the two would drift.
//
// The retired build published no mirror here at all, and `./le site check`
// refuses a published route that carries none, so the homepage was the one
// route the mirror contract never covered.
func homeMirror(page string) (string, error) {
	main, err := extractMain(page)
	if err != nil {
		return "", err
	}
	return htmlToMarkdown(main, pageCanonicalURL(homeDest))
}

// homeBody fills the template's eleven slots.
func homeBody(audience *audienceData, whatsNew *whatsNewData, features featureData,
	articles []blogArticle, weeks []changeWeek, hero string, stats map[string]string,
) (string, error) {
	runCards, err := audienceCards(audience.Run)
	if err != nil {
		return "", err
	}
	whoCards, err := audienceCards(audience.Who)
	if err != nil {
		return "", err
	}
	slots := []string{
		"{hero_demo}", hero,
		"{whats_new}", whatsNewBand(whatsNew, articles, weeks),
		"{run_cards}", runCards,
		"{who_cards}", whoCards,
		"{category_links}", featureCategoryLinks(features),
		"{blog_teaser_cards}", blogTeaserCards(weeks),
	}
	for name, span := range stats {
		slots = append(slots, name, span)
	}
	if err := everySlotFilled(slots); err != nil {
		return "", err
	}
	// One pass, so a slot's own content can never be substituted again: a card
	// body spelling {run_cards} reaches the reader as those characters.
	return strings.NewReplacer(slots...).Replace(homeTemplate), nil
}

// homeSlotPattern matches one slot of the homepage template. The digit is
// load-bearing: {e2e_tests} is a slot, and a pattern of letters alone reads
// past it and leaves that one span unguarded.
var homeSlotPattern = regexp.MustCompile(`\{[a-z0-9_]+\}`)

// everySlotFilled refuses a template with a slot the producer does not fill, or
// a producer filling a slot the template does not have.
//
// It reads the TEMPLATE rather than the filled page, because a card body or a
// week's intro may spell braces of its own and that is a reader's text rather
// than a hole in the page.
func everySlotFilled(slots []string) error {
	filled := make(map[string]bool, len(slots)/2)
	for index := 0; index < len(slots); index += 2 {
		filled[slots[index]] = true
	}
	declared := map[string]bool{}
	for _, slot := range homeSlotPattern.FindAllString(homeTemplate, -1) {
		declared[slot] = true
		if !filled[slot] {
			return fmt.Errorf("the homepage template has the slot %s and nothing fills it", slot)
		}
	}
	for slot := range filled {
		if !declared[slot] {
			return fmt.Errorf("the homepage fills the slot %s, which its template does not have", slot)
		}
	}
	return nil
}

// homeProofStats answers the six spans the proof strip carries.
//
// Each one states the fact's own key in data-ze-stat, so the page says which
// number in the snapshot it is showing and a reader can check the two against
// each other. A span with no value is refused: a blank where a number belongs
// reads as a measurement of nothing.
func homeProofStats(facts *siteFacts) (map[string]string, error) {
	values := map[string]struct{ key, value string }{
		"{unit_tests}":      {"tests.unit_display", facts.Tests.UnitDisplay},
		"{e2e_tests}":       {"tests.e2e_display", facts.Tests.E2EDisplay},
		"{fuzz_targets}":    {"tests.fuzz_display", facts.Tests.FuzzDisplay},
		"{interop_targets}": {"interop.target_display", facts.Interop.TargetDisplay},
		"{rfc_must_checks}": {"rfc.gated_must_display", facts.RFC.GatedMustDisplay},
		"{rfc_enrolled}":    {"rfc.enrolled_display", facts.RFC.EnrolledDisplay},
	}
	spans := make(map[string]string, len(values))
	for slot, stat := range values {
		if stat.value == "" {
			return nil, fmt.Errorf("the facts snapshot states no %s, so the homepage proof strip would show a blank", stat.key)
		}
		spans[slot] = `<span data-ze-stat="` + html.EscapeString(stat.key) + `">` + html.EscapeString(stat.value) + `</span>`
	}
	return spans, nil
}

// audienceCards renders one grid of the homepage, in the data file's own order.
func audienceCards(cards []audienceCard) (string, error) {
	rendered := make([]string, 0, len(cards))
	for index := range cards {
		card, err := audienceCardHTML(&cards[index])
		if err != nil {
			return "", err
		}
		rendered = append(rendered, card)
	}
	return strings.Join(rendered, "\n"), nil
}

// audienceCardHTML renders one card of a homepage grid.
//
// A card with no title or no body is refused by name. The retired renderer
// raised a KeyError on the first and published an empty paragraph for the
// second, and an empty card on the homepage is a hole a reader meets before
// anything else on the site.
func audienceCardHTML(card *audienceCard) (string, error) {
	if card.Title == "" {
		return "", fmt.Errorf("an audience card in data/%s states no title", audienceDataFile)
	}
	if card.Body == "" {
		return "", fmt.Errorf("the audience card %q states no body", card.Title)
	}
	category := card.Category
	if category == "" {
		category = categoryPlatform
	}
	label := card.Label
	if label == "" {
		label = capitalizeWord(category)
	}
	tone := ""
	if card.Tone != "" {
		tone = " tone-" + html.EscapeString(card.Tone)
	}
	title := html.EscapeString(card.Title)
	action := ""
	if card.Link != nil {
		if card.Link.Href == "" {
			return "", fmt.Errorf("the audience card %q states a link with no href", card.Title)
		}
		if card.Link.Label == "" {
			return "", fmt.Errorf("the audience card %q states a link with no label", card.Title)
		}
		title = `<a href="` + html.EscapeString(card.Link.Href) + `">` + title + `</a>`
		action = "                        <span class=\"audience-card-cta\">" + html.EscapeString(card.Link.Label) +
			" <small>" + html.EscapeString(card.Link.Sublabel) + "</small></span>\n"
	}

	var out strings.Builder
	out.WriteString("                    <article class=\"card audience-card cat-" + html.EscapeString(category) + tone + "\">\n")
	out.WriteString("                        <span class=\"cat\">" + html.EscapeString(label) + "</span>\n")
	out.WriteString("                        <h3>" + title + "</h3>\n")
	out.WriteString("                        <p>\n")
	out.WriteString("                            " + html.EscapeString(card.Body) + "\n")
	out.WriteString("                        </p>\n")
	if len(card.Chips) != 0 {
		out.WriteString("                        <div class=\"chips\">\n")
		for _, chip := range card.Chips {
			out.WriteString("                            <span class=\"chip\">" + html.EscapeString(chip) + "</span>\n")
		}
		out.WriteString("                        </div>\n")
	}
	out.WriteString(action)
	out.WriteString("                    </article>")
	return out.String(), nil
}

// featureCategoryLinks renders the seven category buttons under the features
// summary, each with the number of shipped and experimental cards it holds.
//
// The order is the page's own (legendCategories), never the data's: a list
// derived from the cards reorders its buttons whenever a card moves, and drops
// a category no card carries.
func featureCategoryLinks(features featureData) string {
	counts := map[string]int{}
	for _, section := range features.Sections {
		if section.ID != featureSectionCore && section.ID != featureSectionExperimental {
			continue
		}
		for _, card := range section.Cards {
			counts[card.Category]++
		}
	}
	links := make([]string, 0, len(legendCategories))
	for _, category := range legendCategories {
		links = append(links, "                    <a class=\"cat-"+category+"\" href=\""+
			sectionFeatures+"#"+category+"\">"+capitalizeWord(category)+" ("+
			strconv.Itoa(counts[category])+")</a>")
	}
	return strings.Join(links, "\n")
}

// whatsNewBand renders the three slots above the proof strip: the newest
// article, the newest weekly update, and the note an author wrote.
//
// A slot with nothing to show is dropped rather than rendered empty, and a band
// with no slot at all renders nothing, so a checkout with no article still
// publishes a homepage.
func whatsNewBand(data *whatsNewData, articles []blogArticle, weeks []changeWeek) string {
	var items []string
	if len(articles) != 0 {
		newest := articles[0]
		items = append(items, whatsNewItem("Engineering note", toneGrape,
			blogDirectory+"/"+newest.Slug+"/", newest.Title, clipSummary(newest.Description)))
	}
	if len(weeks) != 0 {
		newest := weeks[0]
		intro := newest.Intro
		if intro == "" {
			intro = "What shipped that week."
		}
		items = append(items, whatsNewItem("Recently shipped", toneSky,
			changeWeek{Slug: newest.Slug}.route(), "Week of "+newest.Slug, clipSummary(intro)))
	}
	if note := data.Note; note != nil {
		href := data.Link.Href
		if note.Link != nil {
			href = note.Link.Href
		}
		tone := note.Tone
		if tone == "" {
			tone = toneSky
		}
		items = append(items, whatsNewItem(note.Label, tone, href, note.Title, clipSummary(note.Body)))
	}
	if len(items) == 0 {
		return ""
	}
	return "            <section class=\"whats-new reveal\" aria-labelledby=\"whats-new-title\">\n" +
		"                <div class=\"whats-new-head\">\n" +
		"                    <h2 id=\"whats-new-title\">" + html.EscapeString(data.Title) + "</h2>\n" +
		"                    <a href=\"" + html.EscapeString(data.Link.Href) + "\">" +
		html.EscapeString(data.Link.Label) + "</a>\n" +
		"                </div>\n" + strings.Join(items, "\n") + "\n            </section>\n"
}

// whatsNewItem renders one slot of the Latest news band.
func whatsNewItem(label, tone, href, title, summary string) string {
	return "                <article class=\"whats-new-item tone-" + html.EscapeString(tone) + "\">\n" +
		"                    <span class=\"whats-new-label\">" + html.EscapeString(label) + "</span>\n" +
		"                    <h3><a href=\"" + html.EscapeString(href) + "\">" + html.EscapeString(title) + "</a></h3>\n" +
		"                    <p>" + html.EscapeString(summary) + "</p>\n" +
		"                </article>"
}

// blogTeaserCards renders the three newest weekly updates as cards.
//
// The weeks arrive newest first from loadChangeWeeks, which sorts them with a
// stable sort, so two weeks covering one start date keep their file-name order
// here as they do on the changelog index.
func blogTeaserCards(weeks []changeWeek) string {
	shown := min(homeTeasers, len(weeks))
	cards := make([]string, 0, shown)
	for index := range shown {
		week := weeks[index]
		intro := week.Intro
		if intro == "" {
			intro = "Weekly update"
		}
		var out strings.Builder
		out.WriteString("                    <article class=\"card card-post home-update-card tone-" +
			homeTeaserTones[index%len(homeTeaserTones)] + "\">\n")
		out.WriteString("                        <div class=\"home-update-head\">\n")
		out.WriteString("                            <span class=\"cat\">Update</span>\n")
		out.WriteString("                            <span class=\"home-update-number\">" +
			twoDigits(index+1) + "</span>\n")
		out.WriteString("                        </div>\n")
		out.WriteString("                        <p class=\"home-update-date\">Week of " +
			html.EscapeString(week.Slug) + "</p>\n")
		out.WriteString("                        <h3><a href=\"" + html.EscapeString(week.route()) +
			"\">" + html.EscapeString(intro) + "</a></h3>\n")
		out.WriteString(homeUpdateTagRow(week.Topics))
		out.WriteString("                    </article>")
		cards = append(cards, out.String())
	}
	return strings.Join(cards, "\n")
}

// homeUpdateTagRow renders a teaser's tags: up to four, one per category first
// so a card shows the breadth of the week rather than four shades of one
// subject, then the rest of the week's tags in their own order.
func homeUpdateTagRow(topics []changeTopic) string {
	picked := pickHomeUpdateTopics(topics)
	if len(picked) == 0 {
		return ""
	}
	var out strings.Builder
	out.WriteString("                        <div class=\"home-update-tags\">\n")
	for _, topic := range picked {
		out.WriteString("                            <span class=\"home-update-tag cat-" +
			html.EscapeString(topic.Category) + "\">" + html.EscapeString(topic.Label) + "</span>\n")
	}
	out.WriteString("                        </div>\n")
	return out.String()
}

// pickHomeUpdateTopics answers the tags one teaser shows: one per distinct
// category in the week's own order, then whatever is left, up to the card's
// bound.
func pickHomeUpdateTopics(topics []changeTopic) []changeTopic {
	picked := make([]changeTopic, 0, homeUpdateTags)
	seen := map[string]bool{}
	for _, topic := range topics {
		if seen[topic.Category] {
			continue
		}
		seen[topic.Category] = true
		picked = append(picked, topic)
		if len(picked) == homeUpdateTags {
			return picked
		}
	}
	for _, topic := range topics {
		if seen[topic.Category] && !heldTopic(picked, topic) {
			picked = append(picked, topic)
		}
		if len(picked) == homeUpdateTags {
			return picked
		}
	}
	return picked
}

// heldTopic reports whether one topic is already among those picked.
func heldTopic(picked []changeTopic, topic changeTopic) bool {
	for _, held := range picked {
		if held.Key == topic.Key && held.Label == topic.Label {
			return true
		}
	}
	return false
}

// clipSummary answers one line of summary, cut on a word boundary.
//
// The count is in runes rather than bytes, so a summary carrying an accent is
// cut where a reader sees the boundary. A summary already short enough is
// returned with its whitespace collapsed and nothing else changed.
func clipSummary(text string) string {
	collapsed := strings.Join(strings.Fields(text), " ")
	runes := []rune(collapsed)
	if len(runes) <= homeSummaryRunes {
		return collapsed
	}
	cut := string(runes[:homeSummaryRunes])
	if space := strings.LastIndex(cut, " "); space >= 0 {
		cut = cut[:space]
	}
	return strings.TrimRight(cut, ",.;:") + "…"
}

// twoDigits answers a number the teaser cards print with a leading zero, which
// is a position rather than a count and so is never above ninety-nine.
func twoDigits(number int) string {
	if number < 10 {
		return "0" + strconv.Itoa(number)
	}
	return strconv.Itoa(number)
}
