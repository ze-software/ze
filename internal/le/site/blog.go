// Design: website/AI.md -- the editorial blog is one producer over blog/posts/*.md
// Detail: markdown.go renders each body, shell.go wraps it, mirror.go writes the sibling.
// Related: changes.go publishes the weekly changelog, which is a different section.
package site

import (
	"fmt"
	"html"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
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
	blogDirectory       = "blog"
	blogIndexDest       = blogDirectory + "/" + pageIndexFile
	blogIndexRoot       = "../"
	blogArticleRoot     = "../../"
	blogFeedDest        = blogDirectory + "/feed.xml"
	blogURL             = siteBase + blogDirectory + "/"
)

// blogIndexLead is the sentence the index hero and its mirror both open with.
// The link is the one difference between the two, so it is a parameter.
func blogIndexLead(changelogLink string) string {
	return "Design notes, deep dives, and engineering essays on how Ze is built. " +
		"For week-by-week shipping notes, read the " + changelogLink + "."
}

// The site's presentation palette. A tone is a color and never a topic
// category, so a card carrying one states nothing about what it says. Naming
// each one keeps the seven spellings in one place: blog.go cycles them for the
// index cards, plugins.go picks one per plugin, and home.go uses three of them.
const (
	toneSky   = "sky"
	toneLemon = "lemon"
	toneGrape = "grape"
	toneMint  = "mint"
	tonePink  = "pink"
	toneGold  = "gold"
	toneTeal  = "teal"
)

// presentationTones color the index cards. The order is the palette's own, and
// a card takes the tone at its position in the list, so inserting an article
// recolors the ones below it.
var presentationTones = []string{toneSky, toneLemon, toneGrape, toneMint, tonePink, toneGold, toneTeal}

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
	tokens, err := loadNumberTokens(paths.Output)
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
		shell.render(blogIndexBody(articles)), blogIndexMirror(articles)); err != nil {
		return nil, err
	}
	if err := writeNamedArtifact(paths.Output, blogFeedDest, blogFeed(articles)); err != nil {
		return nil, err
	}
	return append(routes, "/"+blogDirectory+"/"), nil
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
func renderBlogArticle(output string, article *blogArticle, tokens numberTokens) error {
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
	shell := pageShell{
		Title:       title,
		Description: description,
		Root:        blogArticleRoot,
		Path:        dest,
		ExtraHead:   metaTag("name", "author", article.Author),
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
	var page strings.Builder
	page.WriteString(`            <section class="` + shellClass + `" aria-labelledby="post-title">` + "\n")
	page.WriteString(pageHero(html.EscapeString(article.Title), html.EscapeString(article.lead()),
		"Article", ` id="post-title"`, "journey-hero blog-article-hero reveal") + "\n")
	page.WriteString(blogArticleMeta(article) + "\n")
	if visual := blogImage(article, blogArticleRoot, "eager", "figure", "blog-article-visual reveal"); visual != "" {
		page.WriteString(visual + "\n")
	}
	page.WriteString(`                <p class="post-back"><a href="../">&larr; All articles</a></p>` + "\n")
	page.WriteString("            </section>\n")
	page.WriteString(blogKeyPoints(article.KeyPoints))
	page.WriteString(blogArticleContents(headings))
	// An article is read top to bottom, so its tables take no column selector
	// and its code blocks take no copy button.
	page.WriteString(`            <section class="md-content blog-article-content reveal" ` +
		`data-table-columns="off" data-code-copy="off">` + "\n")
	page.WriteString(body)
	page.WriteString("            </section>\n")
	return page.String()
}

// blogArticleMeta renders the byline: when the article was written and who
// wrote it.
func blogArticleMeta(article *blogArticle) string {
	var meta strings.Builder
	meta.WriteString(`                <div class="blog-article-meta">`)
	if article.Date != "" {
		meta.WriteString(`<time datetime="` + html.EscapeString(article.Date) + `">` +
			html.EscapeString(article.Date) + "</time>")
	}
	meta.WriteString("<span>by " + html.EscapeString(article.Author) + "</span></div>")
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
			html.EscapeString(article.altText()) + `" loading="` + loading +
			`" decoding="async" /></` + tag + ">"
	}
	dark := html.EscapeString(articleAsset(root, article.ImageDark))
	return "<" + tag + ` class="blog-theme-image has-dark ` + classes + `" role="img" aria-label="` +
		html.EscapeString(article.altText()) + `">` +
		`<img class="blog-theme-image-light" src="` + light + `" alt="" loading="` + loading +
		`" decoding="async" />` +
		`<img class="blog-theme-image-dark" src="` + dark + `" alt="" loading="` + loading +
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
	var aside strings.Builder
	aside.WriteString(`            <aside class="blog-key-points reveal" aria-label="Key points">` + "\n")
	aside.WriteString(`                <p class="blog-key-points-label">Key points</p>` + "\n")
	aside.WriteString("                <ul>\n")
	for _, point := range points {
		aside.WriteString("                    <li>" + html.EscapeString(point) + "</li>\n")
	}
	aside.WriteString("                </ul>\n            </aside>\n")
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
	var contents strings.Builder
	contents.WriteString(`            <nav class="blog-article-toc reveal" aria-label="Article sections">` + "\n")
	contents.WriteString(`                <p class="blog-article-toc-label">In this article</p>` + "\n")
	contents.WriteString("                <ol>\n")
	for _, section := range sections {
		contents.WriteString(`                    <li><a href="#` + html.EscapeString(section.ID) + `">` +
			html.EscapeString(section.Label) + "</a></li>\n")
	}
	contents.WriteString("                </ol>\n            </nav>\n")
	return contents.String()
}

// blogArticleMirror renders the Markdown sibling of one article: the front
// matter an author wrote, then the body with its number tokens resolved.
func blogArticleMirror(article *blogArticle, body string) string {
	var mirror strings.Builder
	mirror.WriteString("# " + article.Title + "\n\n")
	byline := "by " + article.Author
	if article.Date != "" {
		byline = article.Date + " " + byline
	}
	mirror.WriteString("*" + byline + "*\n\n")
	if article.Deck != "" {
		mirror.WriteString(article.Deck + "\n\n")
	}
	if article.Image != "" {
		mirror.WriteString("![" + article.altText() + "](" +
			articleAsset(blogArticleRoot, article.Image) + ")\n\n")
	}
	if len(article.KeyPoints) != 0 {
		mirror.WriteString("## Key points\n\n")
		for _, point := range article.KeyPoints {
			mirror.WriteString("- " + point + "\n")
		}
		mirror.WriteString("\n")
	}
	mirror.WriteString(strings.TrimSpace(body))
	return strings.TrimSpace(mirror.String()) + "\n"
}

// blogIndexBody renders the index between <main> and </main>: one card for each
// article, newest first.
func blogIndexBody(articles []blogArticle) string {
	var page strings.Builder
	page.WriteString(`            <section class="blog-index" aria-labelledby="blog-title">` + "\n")
	page.WriteString(pageHero("The Ze blog.", blogIndexLead(`<a href="../project/changes/">changelog</a>`),
		"Blog", ` id="blog-title"`, "journey-hero blog-index-hero reveal") + "\n")
	page.WriteString(`                <div class="blog-list reveal">` + "\n")
	for index := range articles {
		page.WriteString(blogIndexCard(&articles[index], presentationTones[index%len(presentationTones)]))
	}
	page.WriteString("                </div>\n            </section>\n")
	return page.String()
}

// blogIndexCard renders one article's card on the index.
func blogIndexCard(article *blogArticle, tone string) string {
	classes := "card card-post blog-card"
	if article.Image != "" {
		classes += " has-media"
	}
	var card strings.Builder
	card.WriteString(`                    <article class="` + classes + " tone-" + tone + `">` + "\n")
	if article.Date != "" {
		card.WriteString(`                        <div class="blog-card-meta"><time datetime="` +
			html.EscapeString(article.Date) + `">` + html.EscapeString(article.Date) +
			"</time><span>Article</span></div>\n")
	}
	if media := blogImage(article, blogIndexRoot, "lazy", "div", "blog-card-media"); media != "" {
		card.WriteString(media + "\n")
	}
	card.WriteString(`                        <h3><a href="` + article.Slug + `/">` +
		html.EscapeString(article.Title) + "</a></h3>\n")
	if article.Description != "" {
		card.WriteString("                        <p>" + html.EscapeString(article.Description) + "</p>\n")
	}
	card.WriteString(`                        <span class="post-more">Read the article</span>` + "\n")
	card.WriteString("                    </article>\n")
	return card.String()
}

// blogIndexMirror renders the Markdown sibling of the index: one line for each
// article, linking the article's own mirror rather than its page.
func blogIndexMirror(articles []blogArticle) string {
	var mirror strings.Builder
	mirror.WriteString("# The Ze blog\n\n")
	mirror.WriteString(blogIndexLead("[changelog](../project/changes/)") + "\n\n")
	for index := range articles {
		article := &articles[index]
		mirror.WriteString("- [" + article.Title + "](" + article.Slug + "/" + pageMirrorFile + ")")
		if article.Date != "" {
			mirror.WriteString(" (" + article.Date + ")")
		}
		if article.Description != "" {
			mirror.WriteString(": " + article.Description)
		}
		mirror.WriteString("\n")
	}
	return strings.TrimSpace(mirror.String()) + "\n"
}

// blogFeed renders the RSS feed over the articles that carry a date.
//
// A dated article is what a feed reader can place in time, so an article with
// no date is published as a page and left out of the feed rather than given a
// date the author did not write.
func blogFeed(articles []blogArticle) string {
	var items strings.Builder
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
		items.WriteString("        <item>\n")
		items.WriteString("            <title>" + xmlText(article.Title) + "</title>\n")
		items.WriteString("            <link>" + link + "</link>\n")
		items.WriteString(`            <guid isPermaLink="true">` + link + "</guid>\n")
		items.WriteString("            <pubDate>" + feedDate(article.Date) + "</pubDate>\n")
		// RSS <author> wants an email address, so the byline goes in
		// dc:creator, which every reader understands and needs no address.
		items.WriteString("            <dc:creator>" + xmlText(article.Author) + "</dc:creator>\n")
		items.WriteString("            <description>" + xmlText(description) + "</description>\n")
		items.WriteString("        </item>\n")
	}
	var feed strings.Builder
	feed.WriteString(feedDeclaration)
	feed.WriteString(`<rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/">` + "\n")
	feed.WriteString("    <channel>\n")
	feed.WriteString("        <title>Ze blog</title>\n")
	feed.WriteString("        <link>" + blogURL + "</link>\n")
	feed.WriteString("        <description>Editorial articles on Ze.</description>\n")
	feed.WriteString("        <language>en</language>\n")
	feed.WriteString("        <lastBuildDate>" + feedDate(built) + "</lastBuildDate>\n")
	feed.WriteString(items.String())
	feed.WriteString("    </channel>\n</rss>\n")
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
