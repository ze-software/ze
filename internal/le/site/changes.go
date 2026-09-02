// Design: website/AI.md -- the weekly changelog is one producer over changes/posts/*.md
// Detail: markdown.go renders each section, shell.go wraps the page, feed.go the RSS.
// Related: blog.go publishes the editorial articles, which are a different section.
package site

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The weekly changelog is registered from here, so a build discovers it through
// the registry rather than through a call the build states by name.
func init() {
	registerProducer(Producer{Name: changesProducer, Render: renderChanges})
}

// Where the changelog reads, where it writes, and how far each page sits from
// the site root.
//
// changesLegacyFeedDest is the feed's first address, from before the changelog
// moved out of blog/. It is still published because a reader's client holds the
// URL it subscribed with, and no redirect makes a feed client follow.
// changesProducer is the changelog's own name: it names this producer in a
// coverage answer, the prose token that carries its count, and the route the
// section published at before it moved under project/.
const changesProducer = "changes"

const (
	changesSourceDirectory = "changes/posts"
	changesDirectory       = "project/changes"
	changesIndexDest       = changesDirectory + "/" + pageIndexFile
	changesIndexRoot       = "../../"
	changesWeekRoot        = "../../../"
	changesFeedDest        = changesDirectory + "/feed.xml"
	changesLegacyFeedDest  = "changes/feed.xml"
	changesIndexFile       = "data/changes.json"
	changesURL             = siteBase + changesDirectory + "/"
	changesFeedTitle       = "Ze weekly updates"
)

// changesIndexDescription is the meta description of the index, and the lead of
// its Markdown mirror is the same sentence in longer form below.
const changesIndexDescription = "What shipped in Ze, week by week: the weekly updates, newest first."

// topicsVocabularyFile is the controlled tag vocabulary: every tag family a
// week may carry, and the category that colors its chip.
const topicsVocabularyFile = "data/topics.json"

// categoryMeta is the eighth chip color, and the one no page takes. It is
// neutral, for work that belongs to no subsystem: a talk, an interoperability
// run, a standards pass.
const categoryMeta = "meta"

// categoryFilterOrder is the order the chips, the filter buttons and each
// week's category list are written in.
//
// It is stated here rather than read from website/data/topics.json because that
// file's own categories array is a set rather than a display order, and the
// order a reader sees is a decision about the page. The neutral bucket sorts
// last, after the seven subsystem hues.
var categoryFilterOrder = []string{
	categoryOperate, categoryRouting, categoryServices, categoryAutomate,
	categoryObserve, categorySecure, categoryPlatform, categoryMeta,
}

// weekHeader matches one themed section header of a weekly post, written as a
// whole line of bold text. The first one names the post and the rest open its
// sections.
var weekHeader = regexp.MustCompile(`(?m)^\*\*(.+?)\*\*[ \t]*$`)

// changeTopic is one chip of a week: what it says, what color it takes, and
// the vocabulary family that decided the color.
//
// The JSON names are the ones data/changes.json publishes, which the homepage
// and any other consumer of that file reads.
type changeTopic struct {
	Label    string `json:"label"`
	Category string `json:"category"`
	Key      string `json:"key"`
}

// changeWeek is one week of the changelog as data/changes.json states it. The
// sections are the page's own content and are not published in that file.
type changeWeek struct {
	Slug     string        `json:"slug"`
	Intro    string        `json:"intro"`
	IsDraft  bool          `json:"is_draft"`
	Topics   []changeTopic `json:"topics"`
	sections []weekSection
}

// weekSection is one themed part of a week: its header and its Markdown body.
type weekSection struct {
	Header string
	Body   string
}

// categories answers the distinct colors one week touched, in the order the
// legend states them.
func (week changeWeek) categories() []string {
	present := make(map[string]bool, len(week.Topics))
	for _, topic := range week.Topics {
		present[topic.Category] = true
	}
	var ordered []string
	for _, category := range categoryFilterOrder {
		if present[category] {
			ordered = append(ordered, category)
		}
	}
	return ordered
}

// dest answers this week's published page, relative to the artifact.
func (week changeWeek) dest() string {
	return week.route() + pageIndexFile
}

// route answers this week's published address, relative to the site root. The
// homepage teasers link a week by it, so the address has one spelling.
func (week changeWeek) route() string {
	return changesDirectory + "/" + week.Slug + "/"
}

// renderChanges publishes every week, the index over them, both feeds and the
// structured index other pages read.
func renderChanges(paths Paths) ([]string, error) {
	vocabulary, err := loadTopicVocabulary(paths.Source)
	if err != nil {
		return nil, err
	}
	weeks, err := loadChangeWeeks(paths.Source, vocabulary)
	if err != nil {
		return nil, err
	}
	links, err := loadPageLinks(paths.Source)
	if err != nil {
		return nil, err
	}

	routes := make([]string, 0, len(weeks)+1)
	for _, week := range weeks {
		if err := renderChangeWeek(paths.Output, week, links); err != nil {
			return nil, fmt.Errorf("week %s: %w", week.Slug, err)
		}
		routes = append(routes, "/"+changesDirectory+"/"+week.Slug+"/")
	}
	if err := removeRetiredWeeks(paths.Output, weeks); err != nil {
		return nil, err
	}

	index, err := changeIndexJSON(weeks)
	if err != nil {
		return nil, err
	}
	if err := writeNamedArtifact(paths.Output, changesIndexFile, index); err != nil {
		return nil, err
	}

	shell := pageShell{
		Title:       "Changes - Ze",
		Description: changesIndexDescription,
		Root:        changesIndexRoot,
		Path:        changesIndexDest,
		ExtraHead:   feedAlternateLink(changesFeedTitle, "feed.xml") + changesIndexStyle,
		Sidebar:     pageSidebar(changesIndexRoot, changesIndexDest, links),
	}
	if err := writePublishedPage(paths.Output, changesIndexDest,
		shell.render(changesIndexBody(weeks)), changesIndexMirror(weeks)); err != nil {
		return nil, err
	}

	feed := changesFeed(weeks)
	for _, dest := range []string{changesFeedDest, changesLegacyFeedDest} {
		if err := writeNamedArtifact(paths.Output, dest, feed); err != nil {
			return nil, err
		}
	}
	return append(routes, "/"+changesDirectory+"/"), nil
}

// loadTopicVocabulary reads the controlled tag vocabulary: a tag family, and
// the category its chip takes.
//
// A family mapped to a category the site has no color for is refused, because
// the chip would be published with a class no stylesheet answers.
func loadTopicVocabulary(source string) (map[string]string, error) {
	path := filepath.Join(source, filepath.FromSlash(topicsVocabularyFile))
	content, err := os.ReadFile(path) //nolint:gosec // a site build reads the checkout it was pointed at
	if err != nil {
		return nil, err
	}
	var vocabulary struct {
		Tags map[string]string `json:"tags"`
	}
	if err := json.Unmarshal(content, &vocabulary); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(vocabulary.Tags) == 0 {
		return nil, fmt.Errorf("%s states no tag", path)
	}
	known := make(map[string]bool, len(categoryFilterOrder))
	for _, category := range categoryFilterOrder {
		known[category] = true
	}
	for family, category := range vocabulary.Tags {
		if !known[category] {
			return nil, fmt.Errorf("%s maps the tag %s to the category %s, which the site has no color for",
				path, family, category)
		}
	}
	return vocabulary.Tags, nil
}

// loadChangeWeeks reads every weekly post, newest first.
//
// The sources are read in file-name order and then sorted by slug with a STABLE
// sort, so two posts whose covers line names one start date keep their file-name
// order rather than whichever order Go's unstable sort happened to leave.
func loadChangeWeeks(source string, vocabulary map[string]string) ([]changeWeek, error) {
	directory := filepath.Join(source, filepath.FromSlash(changesSourceDirectory))
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	var weeks []changeWeek
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), markdownExtension) {
			continue
		}
		week, err := readChangeWeek(filepath.Join(directory, entry.Name()), vocabulary)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path.Join(changesSourceDirectory, entry.Name()), err)
		}
		weeks = append(weeks, week)
	}
	if len(weeks) == 0 {
		return nil, fmt.Errorf("no weekly post in %s; the site publishes a changelog over them", directory)
	}
	sort.SliceStable(weeks, func(left, right int) bool { return weeks[left].Slug > weeks[right].Slug })
	return weeks, nil
}

// readChangeWeek reads one weekly post.
//
// A post whose body carries no themed header is refused by name. The retired
// renderer skipped it with a warning and its build then exited non-zero on any
// warning, so such a week was never published: refusing says the same thing
// where the mistake is, rather than publishing a week with no content.
func readChangeWeek(sourcePath string, vocabulary map[string]string) (changeWeek, error) {
	source, err := os.ReadFile(sourcePath) //nolint:gosec // a site build reads the checkout it was pointed at
	if err != nil {
		return changeWeek{}, err
	}
	metadata, body, err := parseFrontMatter(source)
	if err != nil {
		return changeWeek{}, err
	}
	covers := metadata["covers"]
	if covers == "" {
		covers = strings.ReplaceAll(strings.TrimSuffix(filepath.Base(sourcePath), markdownExtension), "..", " .. ")
	}
	slug := weekSlug(covers)
	if _, err := time.Parse(time.DateOnly, slug); err != nil {
		return changeWeek{}, fmt.Errorf("covers %q does not start with a YYYY-MM-DD date", covers)
	}
	intro, sections, found := splitWeekSections(string(body))
	if !found {
		return changeWeek{}, fmt.Errorf("no themed section header; a week is written as **Header** lines")
	}
	topics, err := weekTopics(metadata["tags"], vocabulary)
	if err != nil {
		return changeWeek{}, err
	}
	return changeWeek{
		Slug:     slug,
		Intro:    intro,
		IsDraft:  strings.HasPrefix(strings.ToUpper(metadata["status"]), "DRAFT"),
		Topics:   topics,
		sections: sections,
	}, nil
}

// weekSlug answers the date a week is published under: the first day its
// covers line names. A covers line may state a time, which the slug drops.
func weekSlug(covers string) string {
	start, _, _ := strings.Cut(covers, "..")
	day, _, _ := strings.Cut(strings.TrimSpace(start), " ")
	return day
}

// splitWeekSections divides one week's body into its intro and its themed
// sections.
//
// The FIRST bold line names the post rather than opening a section, so the text
// under it is the intro and every later bold line opens a section. A body with
// no bold line at all answers not-found, which is what makes a week with no
// content a refusal rather than an empty page.
func splitWeekSections(body string) (intro string, sections []weekSection, found bool) {
	headers := weekHeader.FindAllStringSubmatchIndex(body, -1)
	if len(headers) == 0 {
		return "", nil, false
	}
	for index, header := range headers {
		end := len(body)
		if index+1 < len(headers) {
			end = headers[index+1][0]
		}
		text := strings.TrimSpace(body[header[1]:end])
		if index == 0 {
			intro = text
			continue
		}
		sections = append(sections, weekSection{Header: body[header[2]:header[3]], Body: text})
	}
	return intro, sections, true
}

// weekTopics answers the chips one week carries, from its tags line.
//
// A namespaced tag such as "Presentation: LINX 126" is classified and filtered
// by the family before the colon and shown in full, so one vocabulary entry
// covers every talk. A family the vocabulary does not name is refused: its chip
// would take the neutral color, which is indistinguishable from a family the
// vocabulary deliberately maps there, so an author's typo would publish as a
// deliberate classification.
func weekTopics(tags string, vocabulary map[string]string) ([]changeTopic, error) {
	if strings.TrimSpace(tags) == "" {
		return nil, fmt.Errorf("no tags in the front matter; a week's chips come from them (vocabulary: %s)",
			topicsVocabularyFile)
	}
	topics := []changeTopic{}
	for field := range strings.SplitSeq(tags, ",") {
		tag := strings.TrimSpace(field)
		if tag == "" {
			continue
		}
		family, _, _ := strings.Cut(tag, ":")
		family = strings.TrimSpace(family)
		category, known := vocabulary[family]
		if !known {
			return nil, fmt.Errorf("the tag %s names the family %s, which %s does not list",
				tag, family, topicsVocabularyFile)
		}
		topics = append(topics, changeTopic{Label: tag, Category: category, Key: family})
	}
	return topics, nil
}

// weekDescriptionMax bounds the meta description of a week's page. A search
// result shows about this much, and the intro is written to be longer.
const weekDescriptionMax = 200

// renderChangeWeek publishes one week's page and its Markdown mirror.
func renderChangeWeek(output string, week changeWeek, links pageLinks) error {
	lead, err := paragraphText(week.Intro)
	if err != nil {
		return err
	}
	description := "Ze weekly update."
	if week.Intro != "" {
		description = truncateRunes(trimIntroWhitespace(week.Intro), weekDescriptionMax)
	}
	dest := week.dest()
	title := "Week of " + week.Slug + " - Ze"
	shell := pageShell{
		Title:       title,
		Description: description,
		Root:        changesWeekRoot,
		Path:        dest,
		ExtraHead:   feedAlternateLink(changesFeedTitle, "../feed.xml"),
		Sidebar:     pageSidebar(changesWeekRoot, dest, links),
	}
	body, err := changeWeekBody(week, lead)
	if err != nil {
		return err
	}
	return writePublishedPage(output, dest, shell.render(body), changeWeekMirror(week))
}

// changeWeekBody renders one week between <main> and </main>: the hero over its
// intro, and one block for each themed section.
func changeWeekBody(week changeWeek, lead string) (string, error) {
	var page textbuf.Buffer
	page.Reset().Str(`            <section class="blog-post" aria-labelledby="post-title">`).Byte('\n')
	if week.IsDraft {
		page.Str(`                <span class="tag">Draft -- pending review</span>`).Byte('\n')
	}
	page.Str(pageHero("Week of "+week.Slug, lead, "Weekly update", ` id="post-title"`, heroClasses)).Byte('\n')
	page.Str(`                <p class="post-back"><a href="../">&larr; All weekly updates</a></p>`).Byte('\n')
	page.Str("            </section>\n")
	page.Str(`            <section class="blog-post reveal">`).Byte('\n')
	page.Str(`                <div class="blog-grid">`).Byte('\n')
	for _, section := range week.sections {
		body, _, err := renderMarkdown([]byte(blankLineBeforeLists(section.Body)))
		if err != nil {
			return "", err
		}
		page.Str(`                    <div class="blog-block" aria-label="`).
			Str(html.EscapeString(section.Header)).Str(`">`).Byte('\n')
		page.Str(`                        <div class="md-content">`).Byte('\n')
		page.Str("                            <h2>").Str(html.EscapeString(section.Header)).Str("</h2>\n")
		page.Str("                            ").Str(body)
		page.Str("                        </div>\n                    </div>\n")
	}
	page.Str("                </div>\n            </section>\n")
	return page.String(), nil
}

// listItemOpening matches the first characters of a bullet list item.
var listItemOpening = regexp.MustCompile(`^[-*]\s`)

// blankLineBeforeLists puts the blank line a list needs between it and the
// paragraph above it.
//
// The weekly posts are written for a chat client, which needs no blank line, so
// most of them run "New:" straight into the bullets under it. The published
// Markdown mirror carries the blank line, which is the reason this pass is here
// rather than left to the renderer: goldmark parses either form the same way,
// and a mirror is read as Markdown by whatever opens it next.
func blankLineBeforeLists(body string) string {
	lines := strings.Split(body, "\n")
	spaced := make([]string, 0, len(lines))
	for _, line := range lines {
		opensItem := listItemOpening.MatchString(line)
		if opensItem && len(spaced) != 0 {
			previous := spaced[len(spaced)-1]
			if strings.TrimSpace(previous) != "" && !listItemOpening.MatchString(previous) {
				spaced = append(spaced, "")
			}
		}
		spaced = append(spaced, line)
	}
	return strings.Join(spaced, "\n")
}

// changeWeekMirror renders the Markdown sibling of one week.
func changeWeekMirror(week changeWeek) string {
	title := "Week of " + week.Slug
	if week.IsDraft {
		title += " (Draft -- pending review)"
	}
	var mirror textbuf.Buffer
	mirror.Reset().Str("# ").Str(title).Str("\n\n")
	if week.Intro != "" {
		mirror.Str(strings.TrimSpace(week.Intro)).Str("\n\n")
	}
	for _, section := range week.sections {
		mirror.Str("## ").Str(section.Header).Str("\n\n")
		mirror.Str(strings.TrimSpace(blankLineBeforeLists(section.Body))).Str("\n\n")
	}
	return strings.TrimSpace(mirror.String()) + "\n"
}

// changesIndexLead is the paragraph the index hero carries. It is markup rather
// than plain text, because it names the two neighboring pages as links.
const changesIndexLead = "What shipped in Ze, newest first: the weekly updates, mined from git " +
	"history and posted to Discord's <code>ze-news</code>. Each week's chips are the areas it " +
	"touched; click a category to show just the weeks that touched it, click again to show " +
	"everything, or click a week for the full write-up. Ze is pre-release, so the configuration " +
	`syntax can still change, and the <a href="../roadmap/">roadmap</a> tracks the path to a ` +
	`stable release. For the landmark features on a timeline, see <a href="../milestones/">Milestones</a>.`

// changesIndexBody renders the index between <main> and </main>: the filter
// buttons over the categories any week touched, then one row for each week.
func changesIndexBody(weeks []changeWeek) string {
	present := make(map[string]bool, len(categoryFilterOrder))
	for _, week := range weeks {
		for _, category := range week.categories() {
			present[category] = true
		}
	}

	var page textbuf.Buffer
	page.Reset().Str(`            <section aria-labelledby="changes-title">`).Byte('\n')
	page.Str(pageHero("Changes.", changesIndexLead, "Weekly updates", ` id="changes-title"`,
		"section-head "+heroClasses)).Byte('\n')
	page.Str(`                <div class="legend ch-filters reveal" role="group" `).
		Str(`aria-label="Filter weeks by category">`).Byte('\n')
	for _, category := range categoryFilterOrder {
		if !present[category] {
			continue
		}
		page.Str(`                    <button class="cat-`).Str(category).Str(`" data-cat="`).Str(category).
			Str(`" aria-pressed="false">`).Str(capitalizeWord(category)).Str("</button>\n")
	}
	page.Str("                </div>\n")
	page.Str(`                <div class="ch-list reveal">`).Byte('\n')
	for _, week := range weeks {
		page.Str(changesIndexRow(week))
	}
	page.Str(`                    <p class="ch-empty filtered-out">No weeks in that category yet.</p>`).Byte('\n')
	page.Str("                </div>\n            </section>\n")
	return page.String()
}

// changesIndexRow renders one week's row on the index. The whole row is the
// link, so a reader reaches the week from anywhere in it.
func changesIndexRow(week changeWeek) string {
	var row textbuf.Buffer
	row.Reset().Str(`                    <a class="ch-week" data-cats="`).
		Str(html.EscapeString(strings.Join(week.categories(), " "))).Str(`" href="`).Str(week.Slug).
		Str(`/" aria-label="Read Week of `).Str(week.Slug).Str(`">`).Byte('\n')
	row.Str(`                        <div class="ch-head">`).Byte('\n')
	draft := ""
	if week.IsDraft {
		draft = `<span class="ch-draft">pending review</span>`
	}
	row.Str(`                            <h2><span class="ch-week-title">Week of `).Str(week.Slug).
		Str("</span>").Str(draft).Str("</h2>\n")
	row.Str("                        </div>\n")
	if week.Intro != "" {
		row.Str(`                        <p class="ch-intro">`).
			Str(html.EscapeString(trimIntroWhitespace(week.Intro))).Str("</p>\n")
	}
	if len(week.Topics) != 0 {
		row.Str(`                        <div class="ch-chips">`).Byte('\n')
		for _, topic := range week.Topics {
			row.Str(`                            <span class="ch-chip cat-`).Str(topic.Category).Str(`">`).
				Str(html.EscapeString(topic.Label)).Str("</span>\n")
		}
		row.Str("                        </div>\n")
	}
	row.Str("                    </a>\n")
	return row.String()
}

// capitalizeWord answers one lower-case word with its first letter raised,
// which is how a category reads on a filter button.
func capitalizeWord(word string) string {
	if word == "" {
		return ""
	}
	return strings.ToUpper(word[:1]) + word[1:]
}

// changesIndexMirror renders the Markdown sibling of the index: one section for
// each week, linking the week's own mirror rather than its page.
func changesIndexMirror(weeks []changeWeek) string {
	var mirror textbuf.Buffer
	mirror.Reset().Str("# Changes\n\n")
	mirror.Str("What shipped in Ze, newest first: the weekly updates, mined from git history ").
		Str("and posted to Discord's `ze-news`. Each week lists the areas it touched; click a week for ").
		Str("the full write-up. Ze is pre-release, so the configuration syntax can still change, and the ").
		Str("[roadmap](../roadmap/) tracks the path to a stable release. For the landmark features on a ").
		Str("timeline, see [Milestones](../milestones/).\n\n")
	for _, week := range weeks {
		title := "Week of " + week.Slug
		if week.IsDraft {
			title += " (pending review)"
		}
		mirror.Str("## [").Str(title).Str("](").Str(week.Slug).Byte('/').Str(pageMirrorFile).Str(")\n\n")
		if week.Intro != "" {
			mirror.Str(trimIntroWhitespace(week.Intro)).Str("\n\n")
		}
		if len(week.Topics) != 0 {
			labels := make([]string, 0, len(week.Topics))
			for _, topic := range week.Topics {
				labels = append(labels, topic.Label)
			}
			mirror.Str("Areas: ").Str(strings.Join(labels, ", ")).Str("\n\n")
		}
	}
	return strings.TrimSpace(mirror.String()) + "\n"
}

// changesFeed renders the RSS feed over the weeks that are published.
//
// A draft week keeps its page, so a reviewer can read it at its own URL, and
// stays out of the feed, so a subscriber is not sent a week nobody has checked.
func changesFeed(weeks []changeWeek) string {
	var items textbuf.Buffer
	items.Reset()
	built := feedEpoch
	for _, week := range weeks {
		if week.IsDraft {
			continue
		}
		if built == feedEpoch {
			built = week.Slug
		}
		link := changesURL + week.Slug + "/"
		description := "Ze weekly update."
		if week.Intro != "" {
			description = trimIntroWhitespace(week.Intro)
		}
		items.Str("        <item>\n")
		items.Str("            <title>Week of ").Str(week.Slug).Str("</title>\n")
		items.Str("            <link>").Str(link).Str("</link>\n")
		items.Str(`            <guid isPermaLink="true">`).Str(link).Str("</guid>\n")
		items.Str("            <pubDate>").Str(feedDate(week.Slug)).Str("</pubDate>\n")
		items.Str("            <description>").Str(xmlText(description)).Str("</description>\n")
		items.Str("        </item>\n")
	}
	var feed textbuf.Buffer
	feed.Reset().Str(feedDeclaration)
	feed.Str(`<rss version="2.0">`).Byte('\n')
	feed.Str("    <channel>\n")
	feed.Str("        <title>").Str(changesFeedTitle).Str("</title>\n")
	feed.Str("        <link>").Str(changesURL).Str("</link>\n")
	feed.Str("        <description>What shipped in Ze each week, in Zeledon's voice, ").
		Str("mined from git history.</description>\n")
	feed.Str("        <language>en</language>\n")
	feed.Str("        <lastBuildDate>").Str(feedDate(built)).Str("</lastBuildDate>\n")
	feed.Str(items.String())
	feed.Str("    </channel>\n</rss>\n")
	return feed.String()
}

// changeIndexJSON renders the structured index: every week newest first, with
// its intro, whether it is a draft, and its chips.
//
// The encoder is built by hand rather than taken from json.MarshalIndent so
// HTML escaping can be turned off: the default rewrites "<", ">" and "&" into
// unicode escapes, and an intro that names a shell redirection would publish as
// a sequence a reader of the file has to decode.
func changeIndexJSON(weeks []changeWeek) (string, error) {
	var document bytes.Buffer
	encoder := json.NewEncoder(&document)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(weeks); err != nil {
		return "", fmt.Errorf("write %s: %w", changesIndexFile, err)
	}
	return document.String(), nil
}

// weekDirectory matches the name of a published week's directory, which is the
// date it is published under.
var weekDirectory = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// removeRetiredWeeks deletes the page of a week this site no longer carries, so
// a withdrawn or re-dated week stops being served.
//
// Only a date-named directory is considered, so the index and its siblings are
// left alone whatever else the section grows.
func removeRetiredWeeks(output string, weeks []changeWeek) error {
	live := make(map[string]bool, len(weeks))
	for _, week := range weeks {
		live[week.Slug] = true
	}
	root := filepath.Join(output, filepath.FromSlash(changesDirectory))
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() || live[entry.Name()] || !weekDirectory.MatchString(entry.Name()) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

// changesIndexStyle is the styling the weekly index carries in its own head.
//
// It sits in the page rather than in website/assets/css/site.css because it
// styles ONE page: the week rows, their category chips and the filter buttons
// above them. The site stylesheet is loaded by every page, so ninety lines
// nothing else uses would be paid for on all of them.
const changesIndexStyle = `        <style>
            .ch-list { display: grid; gap: 0.45rem; margin-top: 1.5rem; }
            .ch-week {
                display: block;
                padding: 0.95rem 1rem;
                border: 1px solid transparent;
                border-radius: 0.85rem;
                color: inherit;
                text-decoration: none;
                transition: background 160ms ease, border-color 160ms ease, box-shadow 160ms ease, transform 160ms ease;
            }
            .ch-week:hover,
            .ch-week:focus-visible {
                border-color: var(--line);
                background: rgba(255, 254, 254, 0.72);
                box-shadow: var(--clay);
                transform: translateY(-1px);
            }
            .ch-week:focus-visible {
                outline: 3px solid rgba(0, 159, 227, 0.22);
                outline-offset: 2px;
            }
            .ch-week.filtered-out { display: none; }
            .ch-filters { margin: 1.1rem 0 0.2rem; }
            .ch-empty { margin: 1.5rem 0; color: var(--muted); }
            .ch-empty.filtered-out { display: none; }
            .ch-head { display: flex; align-items: baseline; gap: 1rem; justify-content: flex-start; }
            .ch-head h2 { margin: 0; font-size: 1.12rem; letter-spacing: -0.01em; }
            .ch-week-title {
                color: var(--text);
                text-decoration: underline;
                text-decoration-color: var(--sky-chip);
                text-decoration-thickness: 0.16em;
                text-underline-offset: 0.18em;
            }
            .ch-week:hover .ch-week-title,
            .ch-week:focus-visible .ch-week-title { color: var(--sky-deep); text-decoration-color: var(--sky-base); }
            .ch-intro { margin: 0.3rem 0 0.65rem; color: var(--text); }
            .ch-chips { display: flex; flex-wrap: wrap; gap: 0.4rem; }
            .ch-chip {
                font-size: 0.72rem;
                font-weight: 600;
                line-height: 1.4;
                padding: 0.12rem 0.6rem;
                border-radius: 999px;
                background: var(--acc-tint, var(--bg-soft));
                color: var(--acc-deep, var(--muted));
                border: 1px solid var(--acc, var(--line));
                white-space: nowrap;
            }
            .ch-chip.cat-meta {
                background: var(--bg-soft);
                color: var(--muted);
                border-color: var(--line);
                border-style: dashed;
            }
            /* The meta bucket has no accent hue, so the shared .legend button
               style (white text over var(--acc)) renders invisible. Give the
               filter button a readable neutral treatment instead. */
            .ch-filters .cat-meta {
                background: var(--bg-soft);
                color: var(--muted);
                border-color: var(--line);
            }
            .ch-filters .cat-meta:hover { color: var(--text); }
            .ch-filters .cat-meta[aria-pressed="true"] {
                border-color: var(--muted);
                box-shadow: 0 0 0 2px var(--line);
            }
            .ch-draft { font-size: 0.7rem; font-weight: 700; text-transform: uppercase; letter-spacing: 0.05em; color: var(--acc-deep, var(--muted)); margin-left: 0.5rem; }
        </style>
`
