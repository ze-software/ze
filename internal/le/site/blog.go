// Design: website/AI.md -- the editorial blog is one producer over blog/posts/*.md
// Detail: markdown.go renders each body, shell.go wraps it, mirror.go writes the sibling.
// Related: changes.go publishes the weekly changelog, which is a different section.
package site

import (
	"errors"
	"fmt"
	"html"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The blog is registered from here, so a build discovers it through the
// registry rather than through a call the build states by name.
func init() {
	registerProducer(Producer{Name: "blog", Render: renderBlog})
}

// Where the blog reads, where it writes, and how far each page sits from the
// site root.
const (
	blogSourceDirectory = "blog/posts"
	blogTopicsFile      = "blog-topics.json"
	blogDirectory       = "blog"
	blogIndexDest       = blogDirectory + "/" + pageIndexFile
	blogIndexRoot       = "../"
	blogArticleRoot     = "../../"
	blogFeedDest        = blogDirectory + "/feed.xml"
	blogURL             = siteBase + blogDirectory + "/"
)

// blogTopicsDocument is the JSON file declaring the ordered blog topics.
type blogTopicsDocument struct {
	Topics []blogTopic `json:"topics"`
}

// blogTopic is one curated topic section on the blog index.
type blogTopic struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

// blogTopicGroup pairs a declared topic with the articles assigned to it.
type blogTopicGroup struct {
	Topic    blogTopic
	Articles []blogArticle
}

// blogIndexLead is the sentence the index hero and its mirror both open with.
// The link is the one difference between the two, so it is a parameter.
func blogIndexLead(changelogLink string) string {
	return "Design notes, deep dives, and engineering essays on how Ze is built. " +
		"For week-by-week shipping notes, read the " + changelogLink + "."
}

// blogArticle is one editorial article: its front matter and its Markdown body.
//
// Title and Author are required, so neither is ever empty here. Date is not:
// an article with no date renders without its time element, stays out of the
// feed, and sorts below every dated article.
type blogArticle struct {
	Slug        string
	Title       string
	Date        string
	Author      string
	Description string
	Category    string
	Topic       string
	TopicTitle  string
	// Deck is the hero lead, and it wins over Description, which is written
	// for the index card and the feed entry rather than for the page.
	Deck string
	// Image is the article's illustration. ImageDark switches the page to the
	// two-element themed form, and ImageAlt describes both.
	Image     string
	ImageDark string
	ImageAlt  string
	KeyPoints []string
	Body      string
}

// cardCategory also gives a zero-value article neutral presentation. Sources
// with a stated category were validated by readBlogArticle before rendering.
func (article *blogArticle) cardCategory() string {
	if article.Category == "" {
		return categoryMeta
	}
	return article.Category
}

// lead answers the sentence under the article title: the deck an author wrote
// for the page, and the index description when there is none.
func (article *blogArticle) lead() string {
	if article.Deck != "" {
		return article.Deck
	}
	return article.Description
}

// dest answers this article's published page, relative to the artifact.
func (article *blogArticle) dest() string {
	return blogDirectory + "/" + article.Slug + "/" + pageIndexFile
}

// altText answers what a reader who cannot see the illustration is told. An
// author who states no alternative text gets the title, which says less but is
// never empty.
func (article *blogArticle) altText() string {
	if article.ImageAlt != "" {
		return article.ImageAlt
	}
	return article.Title
}

// keyPointSeparator splits the one front-matter list this site has. The front
// matter is scalar-only by design, so a pipe list keeps an article's key points
// editable without a YAML parser, and a comma stays available inside a point.
const keyPointSeparator = "|"

// renderBlog publishes every article, the index over them, and the feed.
func renderBlog(paths Paths) ([]string, error) {
	articles, err := loadBlogArticles(paths.Source)
	if err != nil {
		return nil, err
	}
	topics, err := loadBlogTopics(paths.Source)
	if err != nil {
		return nil, err
	}
	groups, err := groupBlogArticles(articles, topics)
	if err != nil {
		return nil, err
	}
	tokens, err := loadSiteFactSnapshot(paths.Output)
	if err != nil {
		return nil, err
	}

	routes := make([]string, 0, len(articles)+1)
	for index := range articles {
		if err := renderBlogArticle(paths.Output, &articles[index], tokens); err != nil {
			return nil, fmt.Errorf("blog article %s: %w", articles[index].Slug, err)
		}
		routes = append(routes, "/"+blogDirectory+"/"+articles[index].Slug+"/")
	}
	if err := removeRetiredArticles(paths.Output, articles); err != nil {
		return nil, err
	}

	indexDescription := "Editorial articles on Ze: design notes, deep dives, and talk write-ups."
	shell := pageShell{
		Title:       "Blog - Ze",
		Description: indexDescription,
		Root:        blogIndexRoot,
		Path:        blogIndexDest,
		ExtraHead:   feedAlternateLink("Ze blog", "feed.xml"),
		// The blog carries no page sidebar: website/data/page-links.json names
		// no blog key, and the retired renderer passed none either.
		Wide: true,
	}
	if err := writePublishedPage(paths.Output, blogIndexDest,
		shell.render(blogIndexBodyGrouped(articles, groups)),
		blogIndexMirrorGrouped(articles, groups)); err != nil {
		return nil, err
	}
	if err := writeNamedArtifact(paths.Output, blogFeedDest, blogFeed(articles)); err != nil {
		return nil, err
	}
	return append(routes, "/"+blogDirectory+"/"), nil
}

// loadBlogTopics reads the declared blog topics when the source tree provides
// data/blog-topics.json. A tree without that file returns nil so fixtures that
// test article rendering alone stay independent of topic curation.
func loadBlogTopics(source string) ([]blogTopic, error) {
	var doc blogTopicsDocument
	err := readSourceJSON(source, blogTopicsFile, &doc)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	seen := make(map[string]bool, len(doc.Topics))
	for _, topic := range doc.Topics {
		if topic.ID == "" {
			return nil, fmt.Errorf("%s: topic entry has an empty id", blogTopicsFile)
		}
		if topic.Title == "" {
			return nil, fmt.Errorf("%s: topic %q has an empty title", blogTopicsFile, topic.ID)
		}
		if seen[topic.ID] {
			return nil, fmt.Errorf("%s: duplicate topic id %q", blogTopicsFile, topic.ID)
		}
		seen[topic.ID] = true
	}
	return doc.Topics, nil
}

// groupBlogArticles validates each article's topic against the declared set and
// returns non-empty topic sections in declared order, with articles newest
// first inside each section. It also populates TopicTitle on each article.
func groupBlogArticles(articles []blogArticle, topics []blogTopic) ([]blogTopicGroup, error) {
	if len(topics) == 0 {
		for index := range articles {
			if articles[index].Topic != "" {
				return nil, fmt.Errorf("blog article %s declares topic %q, but %s is absent",
					articles[index].Slug, articles[index].Topic, blogTopicsFile)
			}
		}
		return nil, nil
	}

	byID := make(map[string]int, len(topics))
	validIDs := make([]string, len(topics))
	buckets := make([]blogTopicGroup, len(topics))
	for index, topic := range topics {
		byID[topic.ID] = index
		validIDs[index] = topic.ID
		buckets[index].Topic = topic
	}

	for index := range articles {
		topicID := articles[index].Topic
		if topicID == "" {
			return nil, fmt.Errorf("blog article %s has no topic in front matter; expected one of: %s",
				articles[index].Slug, strings.Join(validIDs, ", "))
		}
		bucketIndex, ok := byID[topicID]
		if !ok {
			return nil, fmt.Errorf("blog article %s has unknown topic %q; expected one of: %s",
				articles[index].Slug, topicID, strings.Join(validIDs, ", "))
		}
		articles[index].TopicTitle = topics[bucketIndex].Title
		buckets[bucketIndex].Articles = append(buckets[bucketIndex].Articles, articles[index])
	}

	groups := make([]blogTopicGroup, 0, len(buckets))
	for _, bucket := range buckets {
		if len(bucket.Articles) > 0 {
			groups = append(groups, bucket)
		}
	}
	return groups, nil
}

// loadBlogArticles reads every article, newest first.
//
// The order is the one the retired renderer published and it is TWO keys, not
// one: the sources are read in file-name order and then sorted by date with a
// STABLE sort, so two articles sharing a date stay in file-name order. Go's
// sort.Slice is not stable, and using it here would give a different index page
// and a different feed on different runs of one unchanged tree.
func loadBlogArticles(source string) ([]blogArticle, error) {
	directory := filepath.Join(source, filepath.FromSlash(blogSourceDirectory))
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	var articles []blogArticle
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), markdownExtension) {
			continue
		}
		article, err := readBlogArticle(filepath.Join(directory, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path.Join(blogSourceDirectory, entry.Name()), err)
		}
		articles = append(articles, article)
	}
	if len(articles) == 0 {
		return nil, fmt.Errorf("no blog article in %s; the site publishes a blog index over them", directory)
	}
	sort.SliceStable(articles, func(left, right int) bool {
		return articles[left].Date > articles[right].Date
	})
	return articles, nil
}

// readBlogArticle reads one article source.
//
// A title and an author are required. The retired renderer skipped a
// title-less article and warned about a missing author, and its build then
// exited non-zero on any warning, so neither article was ever published:
// refusing by name says the same thing where the mistake is.
func readBlogArticle(sourcePath string) (blogArticle, error) {
	source, err := os.ReadFile(sourcePath) //nolint:gosec // a site build reads the checkout it was pointed at
	if err != nil {
		return blogArticle{}, err
	}
	metadata, body, err := parseFrontMatter(source)
	if err != nil {
		return blogArticle{}, err
	}
	if metadata["title"] == "" {
		return blogArticle{}, fmt.Errorf("no title in the front matter")
	}
	if metadata["author"] == "" {
		return blogArticle{}, fmt.Errorf("no author in the front matter; every article carries a byline")
	}
	if date := metadata["date"]; date != "" {
		if _, err := time.Parse(time.DateOnly, date); err != nil {
			return blogArticle{}, fmt.Errorf("date %q is not YYYY-MM-DD", date)
		}
	}
	category, err := cardCategory(metadata["category"])
	if err != nil {
		return blogArticle{}, err
	}
	slug := metadata["slug"]
	if slug == "" {
		slug = strings.TrimSuffix(filepath.Base(sourcePath), markdownExtension)
	}
	return blogArticle{
		Slug:        slug,
		Title:       metadata["title"],
		Date:        metadata["date"],
		Author:      metadata["author"],
		Description: metadata["description"],
		Category:    category,
		Topic:       metadata["topic"],
		Deck:        metadata["deck"],
		Image:       metadata["image"],
		ImageDark:   metadata["image-dark"],
		ImageAlt:    metadata["image-alt"],
		KeyPoints:   splitFrontMatterList(metadata["key-points"]),
		Body:        strings.TrimSpace(string(body)),
	}, nil
}

// splitFrontMatterList answers the parts of one pipe-separated front-matter
// value, with the empty parts dropped.
func splitFrontMatterList(value string) []string {
	var parts []string
	for part := range strings.SplitSeq(value, keyPointSeparator) {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

// renderBlogArticle publishes one article page and its Markdown mirror.
func renderBlogArticle(output string, article *blogArticle, tokens siteFactSnapshot) error {
	// The prose numbers resolve twice: as data-ze-stat spans for the page, so a
	// build can check a published number against the snapshot it came from, and
	// plain for the mirror.
	marked, err := tokens.substitute(article.Body, true)
	if err != nil {
		return err
	}
	plain, err := tokens.substitute(article.Body, false)
	if err != nil {
		return err
	}
	body, headings, err := renderMarkdown([]byte(marked))
	if err != nil {
		return err
	}
	// An article links out more than any other page, and a link that leaves
	// the site owes the same target and rel here as it does on a docs page
	// (docs.go). Only docs.go called this, so every external link in an
	// article opened in place and reached back through window.opener.
	body = patchExternalLinkTargets(body)

	dest := article.dest()
	title := article.Title + " - Ze Blog"
	description := article.Description
	if description == "" {
		description = article.Title
	}
	extraHead := metaTag("name", "author", article.Author)
	if article.Date != "" {
		extraHead += metaTag("property", "article:published_time", article.Date)
	}
	shell := pageShell{
		Title:       title,
		Description: description,
		OGType:      "article",
		Root:        blogArticleRoot,
		Path:        dest,
		ExtraHead:   extraHead,
		Wide:        true,
	}
	page := shell.render(blogArticleBody(article, body, headings))
	return writePublishedPage(output, dest, page, blogArticleMirror(article, plain))
}

// blogArticleBody renders one article between <main> and </main>: the hero and
// its byline, the illustration, the key points, the contents list, and the
// article itself.
func blogArticleBody(article *blogArticle, body string, headings []docHeading) string {
	shellClass := "blog-article-shell"
	if article.Image != "" {
		shellClass += " has-visual"
	}
	var page textbuf.Buffer
	page.Str(`            <section class="`).Str(shellClass).Str(`" aria-labelledby="post-title">`).Byte('\n')
	page.Str(`                <div class="blog-article-heading">`).Byte('\n')
	page.Str(pageHero(html.EscapeString(article.Title), html.EscapeString(article.lead()),
		"Article", ` id="post-title"`, "journey-hero blog-article-hero reveal")).Byte('\n')
	page.Str(blogArticleMeta(article)).Byte('\n')
	page.Str("                </div>\n")
	if visual := blogImage(article, blogArticleRoot, "eager", "figure", "blog-article-visual reveal"); visual != "" {
		page.Str(visual).Byte('\n')
	}
	page.Str("            </section>\n")
	page.Str(`            <div class="blog-reading-layout">`).Byte('\n')
	page.Str(`                <aside class="blog-reading-rail" aria-label="Article navigation">`).Byte('\n')
	page.Str(blogArticleContents(headings))
	page.Str(`                    <nav class="blog-related-links" aria-label="Related links">`).Byte('\n')
	page.Str(`                        <p class="post-back"><a href="../">&larr; All articles</a></p>`).Byte('\n')
	page.Str(`                        <a href="../feed.xml">Subscribe to the blog</a>`).Byte('\n')
	page.Str(`                        <a href="../../docs/">Explore the documentation</a>`).Byte('\n')
	page.Str("                    </nav>\n                </aside>\n")
	page.Str(`                <div class="blog-reading-body">`).Byte('\n')
	page.Str(blogKeyPoints(article.KeyPoints))
	// An article is read top to bottom, so its tables take no column selector
	// and its code blocks take no copy button.
	page.Str(`            <section class="md-content blog-article-content reveal" `).
		Str(`data-table-columns="off" data-code-copy="off">`).Byte('\n')
	page.Str(body)
	page.Str("            </section>\n")
	page.Str("                </div>\n            </div>\n")
	return page.String()
}

// blogArticleMeta renders the byline: when the article was written and who
// wrote it.
func blogArticleMeta(article *blogArticle) string {
	var meta textbuf.Buffer
	meta.Str(`                <div class="blog-article-meta">`)
	if article.Date != "" {
		meta.Str(`<time datetime="`).Str(html.EscapeString(article.Date)).Str(`">`).
			Str(html.EscapeString(article.Date)).Str("</time>")
	}
	meta.Str("<span>by ").Str(html.EscapeString(article.Author)).Str("</span>")
	if article.Topic != "" && article.TopicTitle != "" {
		meta.Str(`<a class="blog-article-topic" href="../#topic-`).Str(html.EscapeString(article.Topic)).
			Str(`">`).Str(html.EscapeString(article.TopicTitle)).Str("</a>")
	}
	meta.Str("</div>")
	return meta.String()
}

// blogImage renders one article illustration, or the empty string for an
// article that shows none.
//
// An article that states a dark illustration takes the themed form: two images
// inside one labeled container, with the stylesheet choosing between them, so
// the alternative text is on the container rather than repeated on both.
func blogImage(article *blogArticle, root, loading, tag, classes string) string {
	if article.Image == "" {
		return ""
	}
	light := html.EscapeString(articleAsset(root, article.Image))
	if article.ImageDark == "" {
		return "<" + tag + ` class="` + classes + `"><img src="` + light + `" alt="` +
			html.EscapeString(article.altText()) + `" width="1200" height="675" loading="` + loading +
			`" decoding="async" /></` + tag + ">"
	}
	dark := html.EscapeString(articleAsset(root, article.ImageDark))
	return "<" + tag + ` class="blog-theme-image has-dark ` + classes + `" role="img" aria-label="` +
		html.EscapeString(article.altText()) + `">` +
		`<img class="blog-theme-image-light" src="` + light + `" alt="" width="1200" height="675" loading="` + loading +
		`" decoding="async" />` +
		`<img class="blog-theme-image-dark" src="` + dark + `" alt="" width="1200" height="675" loading="` + loading +
		`" decoding="async" /></` + tag + ">"
}

// articleAsset answers where one article asset is reached from a page. An
// absolute URL and a rooted path already say where they are; anything else is
// relative to the site root, so it takes the page's own prefix.
func articleAsset(root, reference string) string {
	if reference == "" {
		return ""
	}
	if strings.HasPrefix(reference, "http://") || strings.HasPrefix(reference, "https://") ||
		strings.HasPrefix(reference, "/") {
		return reference
	}
	return root + strings.TrimPrefix(reference, "/")
}

// blogKeyPoints renders the summary an author wrote for a reader deciding
// whether to read the article.
func blogKeyPoints(points []string) string {
	if len(points) == 0 {
		return ""
	}
	var aside textbuf.Buffer
	aside.Str(`            <aside class="blog-key-points reveal" aria-label="Key points">`).Byte('\n')
	aside.Str(`                <p class="blog-key-points-label">Key points</p>`).Byte('\n')
	aside.Str("                <ul>\n")
	for _, point := range points {
		aside.Str("                    <li>").Str(html.EscapeString(point)).Str("</li>\n")
	}
	aside.Str("                </ul>\n            </aside>\n")
	return aside.String()
}

// blogArticleContentsFloor is the number of sections below which an article
// gets no contents list. Two entries name what the reader can already see in
// one screen, so the list costs a reader more than it saves.
const blogArticleContentsFloor = 3

// blogArticleContents renders the in-article navigation over its level 2
// headings. The ids are goldmark's own, and the same ids are on the headings
// this list was built from, so the list stays self-consistent.
func blogArticleContents(headings []docHeading) string {
	var sections []docHeading
	for _, heading := range headings {
		if heading.Level == 2 && heading.ID != "" && heading.Label != "" {
			sections = append(sections, heading)
		}
	}
	if len(sections) < blogArticleContentsFloor {
		return ""
	}
	var contents textbuf.Buffer
	contents.Str(`            <nav class="blog-article-toc reveal" aria-label="Article sections">`).Byte('\n')
	contents.Str(`                <details class="reading-outline" open>`).Byte('\n')
	contents.Str(`                <summary class="blog-article-toc-label">In this article</summary>`).Byte('\n')
	contents.Str("                <ol>\n")
	for _, section := range sections {
		contents.Str(`                    <li><a href="#`).Str(html.EscapeString(section.ID)).Str(`">`).
			Str(html.EscapeString(section.Label)).Str("</a></li>\n")
	}
	contents.Str("                </ol>\n                </details>\n            </nav>\n")
	return contents.String()
}

// blogArticleMirror renders the Markdown sibling of one article: the front
// matter an author wrote, then the body with its number tokens resolved.
func blogArticleMirror(article *blogArticle, body string) string {
	var mirror textbuf.Buffer
	mirror.Str("# ").Str(article.Title).Str("\n\n")
	byline := "by " + article.Author
	if article.Date != "" {
		byline = article.Date + " " + byline
	}
	mirror.Byte('*').Str(byline).Str("*\n\n")
	if article.Deck != "" {
		mirror.Str(article.Deck).Str("\n\n")
	}
	if article.Image != "" {
		mirror.Str("![").Str(article.altText()).Str("](").Str(articleAsset(blogArticleRoot, article.Image)).
			Str(")\n\n")
	}
	if len(article.KeyPoints) != 0 {
		mirror.Str("## Key points\n\n")
		for _, point := range article.KeyPoints {
			mirror.Str("- ").Str(point).Byte('\n')
		}
		mirror.Byte('\n')
	}
	mirror.Str(strings.TrimSpace(body))
	return strings.TrimSpace(mirror.String()) + "\n"
}

// blogIndexBody renders the index between <main> and </main>: one card for each
// article, newest first.
func blogIndexBody(articles []blogArticle) string {
	return blogIndexBodyGrouped(articles, nil)
}

// blogIndexBodyGrouped renders the index either grouped into declared topic
// sections with jump links at the top, or as one flat list when no topics are
// configured.
func blogIndexBodyGrouped(articles []blogArticle, groups []blogTopicGroup) string {
	var page textbuf.Buffer
	page.Str(`            <section class="blog-index" aria-labelledby="blog-title">`).Byte('\n')
	page.Str(pageHero("The Ze blog.", blogIndexLead(`<a href="../project/changes/">changelog</a>`),
		"Blog", ` id="blog-title"`, "journey-hero blog-index-hero reveal")).Byte('\n')
	if len(groups) == 0 {
		page.Str("                <h2 class=\"sr-only\">Articles</h2>\n")
		page.Str(`                <div class="blog-list reveal">`).Byte('\n')
		for index := range articles {
			page.Str(blogIndexCard(&articles[index]))
		}
		page.Str("                </div>\n            </section>\n")
		return page.String()
	}

	page.Str(`                <nav class="blog-topic-nav reveal" aria-label="Blog topics">`).Byte('\n')
	for _, group := range groups {
		page.Str(`                    <a href="#topic-`).Str(html.EscapeString(group.Topic.ID)).Str(`">`).
			Str(html.EscapeString(group.Topic.Title)).
			Str(` <span class="blog-topic-count">`).Int(int64(len(group.Articles))).Str("</span></a>\n")
	}
	page.Str("                </nav>\n")

	for _, group := range groups {
		id := html.EscapeString(group.Topic.ID)
		page.Str(`                <section class="blog-topic reveal" id="topic-`).Str(id).
			Str(`" aria-labelledby="topic-`).Str(id).Str(`-title">`).Byte('\n')
		page.Str(`                    <div class="blog-topic-head">`).Byte('\n')
		page.Str(`                        <h2 id="topic-`).Str(id).Str(`-title">`).
			Str(html.EscapeString(group.Topic.Title)).Str("</h2>\n")
		if group.Topic.Summary != "" {
			page.Str("                        <p>").Str(html.EscapeString(group.Topic.Summary)).Str("</p>\n")
		}
		page.Str("                    </div>\n")
		page.Str(`                    <div class="blog-list">`).Byte('\n')
		for index := range group.Articles {
			page.Str(blogIndexCard(&group.Articles[index]))
		}
		page.Str("                    </div>\n                </section>\n")
	}
	page.Str("            </section>\n")
	return page.String()
}

// blogIndexCard renders one article's card on the index.
func blogIndexCard(article *blogArticle) string {
	classes := "card card-post blog-card"
	if article.Image != "" {
		classes += " has-media"
	}
	var card textbuf.Buffer
	card.Str(`                    <article class="`).Str(classes).Str(" cat-").Str(article.cardCategory()).Str(`">`).Byte('\n')
	if article.Date != "" {
		card.Str(`                        <div class="blog-card-meta"><time datetime="`).
			Str(html.EscapeString(article.Date)).Str(`">`).Str(html.EscapeString(article.Date)).
			Str("</time><span>Article</span></div>\n")
	}
	if media := blogImage(article, blogIndexRoot, "lazy", "div", "blog-card-media"); media != "" {
		card.Str(media).Byte('\n')
	}
	card.Str(`                        <h3><a href="`).Str(article.Slug).Str(`/">`).
		Str(html.EscapeString(article.Title)).Str("</a></h3>\n")
	if article.Description != "" {
		card.Str("                        <p>").Str(html.EscapeString(article.Description)).Str("</p>\n")
	}
	card.Str(`                        <span class="post-more">Read the article</span>`).Byte('\n')
	card.Str("                    </article>\n")
	return card.String()
}

// blogIndexMirror renders the Markdown sibling of the index: one line for each
// article, linking the article's own mirror rather than its page.
func blogIndexMirror(articles []blogArticle) string {
	return blogIndexMirrorGrouped(articles, nil)
}

// blogIndexMirrorGrouped renders the Markdown sibling grouped by topic when
// topic sections are active, or as one flat list otherwise.
func blogIndexMirrorGrouped(articles []blogArticle, groups []blogTopicGroup) string {
	var mirror textbuf.Buffer
	mirror.Str("# The Ze blog\n\n")
	mirror.Str(blogIndexLead("[changelog](../project/changes/)")).Str("\n\n")
	if len(groups) == 0 {
		writeBlogIndexMirrorList(&mirror, articles)
		return strings.TrimSpace(mirror.String()) + "\n"
	}
	for _, group := range groups {
		mirror.Str("## ").Str(group.Topic.Title).Str("\n\n")
		if group.Topic.Summary != "" {
			mirror.Str(group.Topic.Summary).Str("\n\n")
		}
		writeBlogIndexMirrorList(&mirror, group.Articles)
		mirror.Byte('\n')
	}
	return strings.TrimSpace(mirror.String()) + "\n"
}

func writeBlogIndexMirrorList(mirror *textbuf.Buffer, articles []blogArticle) {
	for index := range articles {
		article := &articles[index]
		mirror.Str("- [").Str(article.Title).Str("](").Str(article.Slug).Byte('/').Str(pageMirrorFile).Byte(')')
		if article.Date != "" {
			mirror.Str(" (").Str(article.Date).Byte(')')
		}
		if article.Description != "" {
			mirror.Str(": ").Str(article.Description)
		}
		mirror.Byte('\n')
	}
}

// blogFeed renders the RSS feed over the articles that carry a date.
//
// A dated article is what a feed reader can place in time, so an article with
// no date is published as a page and left out of the feed rather than given a
// date the author did not write.
func blogFeed(articles []blogArticle) string {
	var items textbuf.Buffer
	built := feedEpoch
	for index := range articles {
		article := &articles[index]
		if article.Date == "" {
			continue
		}
		if built == feedEpoch {
			built = article.Date
		}
		link := blogURL + article.Slug + "/"
		description := article.Description
		if description == "" {
			description = article.Title
		}
		items.Str("        <item>\n")
		items.Str("            <title>").Str(xmlText(article.Title)).Str("</title>\n")
		items.Str("            <link>").Str(link).Str("</link>\n")
		items.Str(`            <guid isPermaLink="true">`).Str(link).Str("</guid>\n")
		items.Str("            <pubDate>").Str(feedDate(article.Date)).Str("</pubDate>\n")
		// RSS <author> wants an email address, so the byline goes in
		// dc:creator, which every reader understands and needs no address.
		items.Str("            <dc:creator>").Str(xmlText(article.Author)).Str("</dc:creator>\n")
		items.Str("            <description>").Str(xmlText(description)).Str("</description>\n")
		items.Str("        </item>\n")
	}
	var feed textbuf.Buffer
	feed.Str(feedDeclaration)
	feed.Str(`<rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/">`).Byte('\n')
	feed.Str("    <channel>\n")
	feed.Str("        <title>Ze blog</title>\n")
	feed.Str("        <link>").Str(blogURL).Str("</link>\n")
	feed.Str("        <description>Editorial articles on Ze.</description>\n")
	feed.Str("        <language>en</language>\n")
	feed.Str("        <lastBuildDate>").Str(feedDate(built)).Str("</lastBuildDate>\n")
	feed.Str(items.String())
	feed.Str("    </channel>\n</rss>\n")
	return feed.String()
}

// removeRetiredArticles deletes the page of an article this site no longer
// carries, so a renamed or withdrawn article stops being served.
//
// The sources sit under blog/posts in the staged artifact until the build trims
// them, which happens after every producer has run, so that directory is not a
// retired article.
func removeRetiredArticles(output string, articles []blogArticle) error {
	live := make(map[string]bool, len(articles))
	for index := range articles {
		live[articles[index].Slug] = true
	}
	root := filepath.Join(output, blogDirectory)
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() || live[entry.Name()] || blogDirectory+"/"+entry.Name() == blogSourceDirectory {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}
