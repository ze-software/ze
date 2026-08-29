// Design: website/AI.md -- the editorial blog is one producer over blog/posts/*.md
package site

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// blogPaths lays out one artifact a blog render can write into, reading the
// real website/ sources of this checkout.
//
// The artifact is seeded with the facts snapshot the published pages were built
// against, cut to the five numbers the articles name. Two articles carry
// {{ze:...}} tokens, so without it their mirrors would publish the braces and
// the published page could not be compared.
func blogPaths(t *testing.T) Paths {
	t.Helper()
	root := repositoryRoot(t)
	output := t.TempDir()
	copyFixture(t, filepath.Join("testdata", "published-site-facts.json"),
		filepath.Join(output, "data", "site-facts.json"))
	return Paths{Repository: root, Source: filepath.Join(root, "website"), Output: output}
}

// blogPostsFixture writes one website source tree carrying only the articles
// given, keyed by file name. It answers the source root.
func blogPostsFixture(t *testing.T, posts map[string]string) string {
	t.Helper()
	source := t.TempDir()
	directory := filepath.Join(source, filepath.FromSlash(blogSourceDirectory))
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range posts {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return source
}

// VALIDATES: two articles sharing a date are published in file-name order, on
// every build of one unchanged tree.
//
// The retired renderer read the sources in file-name order and then sorted them
// by date with Python's stable sort, so the file name decided a tie. Go's
// sort.Slice is NOT stable, so a port that reaches for it publishes a different
// index page and a different feed on different runs of the same sources, and
// nothing in the artifact says which one a reader got.
func TestBlogPostsSharingADateOrderByFilename(t *testing.T) {
	source := blogPostsFixture(t, map[string]string{
		"zebra.md":  blogPostSource("Zebra", "2026-08-04"),
		"apple.md":  blogPostSource("Apple", "2026-08-04"),
		"mango.md":  blogPostSource("Mango", "2026-08-04"),
		"newest.md": blogPostSource("Newest", "2026-08-05"),
	})

	articles, err := loadBlogArticles(source)
	if err != nil {
		t.Fatal(err)
	}

	var order []string
	for _, article := range articles {
		order = append(order, article.Slug)
	}
	want := []string{"newest", "apple", "mango", "zebra"}
	if !slices.Equal(order, want) {
		t.Fatalf("the articles published as %v, want %v: three share a date, so the file name decides", order, want)
	}
}

// VALIDATES: an article with no date is published as a page, sorts below every
// dated article, and stays out of the feed rather than being given a date its
// author did not write.
func TestAnUndatedArticleSortsLastAndStaysOutOfTheFeed(t *testing.T) {
	source := blogPostsFixture(t, map[string]string{
		"dated.md":   blogPostSource("Dated", "2026-08-04"),
		"undated.md": blogPostSource("Undated", ""),
	})

	articles, err := loadBlogArticles(source)
	if err != nil {
		t.Fatal(err)
	}
	if articles[0].Slug != "dated" || articles[1].Slug != "undated" {
		t.Fatalf("the articles published as %s then %s, want the dated one first",
			articles[0].Slug, articles[1].Slug)
	}

	feed := blogFeed(articles)
	if strings.Contains(feed, "/undated/") {
		t.Errorf("the undated article reached the feed:\n%s", feed)
	}
	if !strings.Contains(feed, "/dated/") {
		t.Errorf("the dated article is missing from the feed:\n%s", feed)
	}
	if !strings.Contains(feed, "<lastBuildDate>Tue, 04 Aug 2026 00:00:00 +0000</lastBuildDate>") {
		t.Errorf("the feed states the wrong build date:\n%s", feed)
	}
}

// blogPostSource writes one minimal article source, with the date left out when
// it is empty.
func blogPostSource(title, date string) string {
	source := "---\ntitle: " + title + "\nauthor: Thomas Mangin\n"
	if date != "" {
		source += "date: " + date + "\n"
	}
	return source + "---\n\nBody of " + title + ".\n"
}

// VALIDATES: an article a page cannot be made from is refused by name rather
// than skipped.
//
// The retired renderer skipped a title-less article with a warning and warned
// about a missing author, and its build then exited non-zero on any warning, so
// neither was ever published. A refusal says the same thing at the file that
// carries the mistake.
func TestAnArticleAPageCannotBeMadeFromIsRefused(t *testing.T) {
	for _, refusal := range []struct {
		name, source, want string
	}{
		{"no title", "---\nauthor: Thomas Mangin\n---\n\nBody.\n", "no title"},
		{"no author", "---\ntitle: A\n---\n\nBody.\n", "no author"},
		{"bad date", "---\ntitle: A\nauthor: T\ndate: August 2026\n---\n\nBody.\n", "not YYYY-MM-DD"},
	} {
		t.Run(refusal.name, func(t *testing.T) {
			source := blogPostsFixture(t, map[string]string{"post.md": refusal.source})
			_, err := loadBlogArticles(source)
			if err == nil {
				t.Fatalf("an article with %s was accepted", refusal.name)
			}
			if !strings.Contains(err.Error(), refusal.want) ||
				!strings.Contains(err.Error(), "post.md") {
				t.Fatalf("the refusal reads %q, want it to name post.md and %q", err, refusal.want)
			}
		})
	}
}

// VALIDATES: one article page reads as the published one, carries the whole
// site shell, and its Markdown mirror matches the published mirror byte for
// byte.
//
// The published pair is reference-from-the-system at gh-pages HEAD 2fa8fa2ad:
// the one article carrying a deck, a themed illustration, key points, a
// contents list and five prose number tokens, so it exercises every block the
// page can hold.
func TestABlogArticleReadsAsThePublishedArticle(t *testing.T) {
	paths := blogPaths(t)

	routes, err := renderBlog(paths)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(routes, "/blog/reference-from-the-system/") {
		t.Fatalf("the producer claimed %v, want the article among them", routes)
	}

	page := readArtifact(t, paths.Output, "blog/reference-from-the-system/"+pageIndexFile)
	for _, chrome := range []string{
		"<title>Reference stays attached to code - Ze Blog</title>",
		`<link rel="canonical" href="https://ze-software.net/blog/reference-from-the-system/" />`,
		`<meta name="author" content="Thomas Mangin" />`,
		`<link rel="stylesheet" href="../../assets/site.css" />`,
		`<div id="site-header-mount" data-header-src="../../assets/header.html"`,
		`<main id="top" class="site-main-wide" tabindex="-1">`,
		"<footer>",
	} {
		if !strings.Contains(page, chrome) {
			t.Errorf("the article page is missing %q", chrome)
		}
	}

	got := visibleText(mainContent(t, page))
	want := visibleText(withCurrentRFCIndexCommand(readFixture(t, "published-blog-reference.html")))
	if got != want {
		t.Errorf("the article reads as\n  %q\nthe published article reads as\n  %q", got, want)
	}

	mirror := readArtifact(t, paths.Output, "blog/reference-from-the-system/"+pageMirrorFile)
	publishedMirror := withCurrentRFCIndexCommand(readFixture(t, "published-blog-reference.md"))
	if mirror != publishedMirror {
		t.Errorf("the mirror is\n%q\nthe published mirror is\n%q", mirror, publishedMirror)
	}
}

// withCurrentRFCIndexCommand corrects the one phrase where the published page
// disagrees with the article's source today.
//
// The SOURCE changed after the last Python-era publish: eae282592 retired make,
// so `make ze-rfc-index-update` became `./le rfc index-update`. The published
// page is frozen at the older wording, which is the staleness this spec exists
// to fix, so the comparison corrects the fixture rather than the render. Any
// OTHER difference is a rendering difference and fails the test.
func withCurrentRFCIndexCommand(published string) string {
	return strings.ReplaceAll(published, "make ze-rfc-index-update", "./le rfc index-update")
}

// VALIDATES: the published article carries the blocks a reader sees around its
// body, each with the class its stylesheet answers.
//
// visibleText above says the words are the same and says nothing about which
// element carries them, so the themed illustration, the key points and the
// contents list are asserted as markup here.
func TestAnArticlePageCarriesItsHeroIllustrationAndContents(t *testing.T) {
	paths := blogPaths(t)
	if _, err := renderBlog(paths); err != nil {
		t.Fatal(err)
	}

	page := readArtifact(t, paths.Output, "blog/reference-from-the-system/"+pageIndexFile)
	for _, want := range []string{
		`<section class="blog-article-shell has-visual" aria-labelledby="post-title">`,
		`<div class="journey-hero blog-article-hero reveal">`,
		`<span class="journey-eyebrow">Article</span>`,
		`<div class="blog-article-meta"><time datetime="2026-08-22">2026-08-22</time><span>by Thomas Mangin</span></div>`,
		`<figure class="blog-theme-image has-dark blog-article-visual reveal" role="img"`,
		`<img class="blog-theme-image-light" src="../../assets/blog/reference-from-the-system.svg"`,
		`<img class="blog-theme-image-dark" src="../../assets/blog/reference-from-the-system-dark.svg"`,
		`<aside class="blog-key-points reveal" aria-label="Key points">`,
		"<li>Facts stay with the owner</li>",
		`<nav class="blog-article-toc reveal" aria-label="Article sections">`,
		`<section class="md-content blog-article-content reveal" data-table-columns="off" data-code-copy="off">`,
		`<span data-ze-stat="cli_commands">402</span>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the article page is missing %q", want)
		}
	}
}

// VALIDATES: the blog index reads as the published index, and its mirror
// matches the published mirror byte for byte.
func TestTheBlogIndexReadsAsThePublishedIndex(t *testing.T) {
	paths := blogPaths(t)
	if _, err := renderBlog(paths); err != nil {
		t.Fatal(err)
	}

	page := readArtifact(t, paths.Output, blogIndexDest)
	for _, chrome := range []string{
		"<title>Blog - Ze</title>",
		`<link rel="canonical" href="https://ze-software.net/blog/" />`,
		`<link rel="alternate" type="application/rss+xml" title="Ze blog" href="feed.xml" />`,
		`<main id="top" class="site-main-wide" tabindex="-1">`,
		`<section class="blog-index" aria-labelledby="blog-title">`,
	} {
		if !strings.Contains(page, chrome) {
			t.Errorf("the blog index is missing %q", chrome)
		}
	}

	got := visibleText(mainContent(t, page))
	want := visibleText(readFixture(t, "published-blog-index.html"))
	if got != want {
		t.Errorf("the index reads as\n  %q\nthe published index reads as\n  %q", got, want)
	}

	mirror := readArtifact(t, paths.Output, blogDirectory+"/"+pageMirrorFile)
	if mirror != readFixture(t, "published-blog-index.md") {
		t.Errorf("the index mirror is\n%q\nthe published one is\n%q",
			mirror, readFixture(t, "published-blog-index.md"))
	}
}

// VALIDATES: each index card takes the presentation tone at its own position,
// and a card's color therefore says nothing about what the article is about.
//
// The tones are asserted against the published page, where the two articles
// sharing 2026-08-04 sit at positions five and six: an unstable sort would swap
// them and swap their colors with them.
func TestAnIndexCardTakesTheToneAtItsPosition(t *testing.T) {
	paths := blogPaths(t)
	if _, err := renderBlog(paths); err != nil {
		t.Fatal(err)
	}

	page := readArtifact(t, paths.Output, blogIndexDest)
	for _, want := range []string{
		`<article class="card card-post blog-card has-media tone-sky">`,
		`<article class="card card-post blog-card has-media tone-pink">`,
		`<h3><a href="how-ze-manages-memory/">How Ze keeps BGP traffic away from the garbage collector</a></h3>`,
		`<div class="blog-theme-image has-dark blog-card-media" role="img"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the blog index is missing %q", want)
		}
	}
	if strings.Index(page, "tone-pink") > strings.Index(page, "tone-gold") {
		t.Errorf("the fifth card took gold and the sixth pink, so the two articles sharing a date swapped")
	}
}

// VALIDATES: the feed carries one entry for each dated article, newest first,
// with the byline in dc:creator rather than in an author element RSS would want
// an email address for.
func TestTheBlogFeedCarriesEveryDatedArticle(t *testing.T) {
	paths := blogPaths(t)
	if _, err := renderBlog(paths); err != nil {
		t.Fatal(err)
	}

	feed := readArtifact(t, paths.Output, blogFeedDest)
	articles, err := loadBlogArticles(paths.Source)
	if err != nil {
		t.Fatal(err)
	}
	if items := strings.Count(feed, "<item>"); items != len(articles) {
		t.Errorf("the feed carries %d items over %d articles", items, len(articles))
	}
	for _, want := range []string{
		`<rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/">`,
		"<title>Ze blog</title>",
		"<link>https://ze-software.net/blog/</link>",
		"<lastBuildDate>Sat, 22 Aug 2026 00:00:00 +0000</lastBuildDate>",
		"<title>Reference stays attached to code</title>",
		`<guid isPermaLink="true">https://ze-software.net/blog/reference-from-the-system/</guid>`,
		"<pubDate>Sat, 22 Aug 2026 00:00:00 +0000</pubDate>",
		"<dc:creator>Thomas Mangin</dc:creator>",
	} {
		if !strings.Contains(feed, want) {
			t.Errorf("the feed is missing %q", want)
		}
	}
	if first, second := strings.Index(feed, "reference-from-the-system"),
		strings.Index(feed, "ai-slop-is-the-wrong-test"); first > second {
		t.Errorf("the feed is oldest first")
	}
}

// VALIDATES: an article this site no longer carries loses its page, so a
// withdrawn or renamed article stops being served rather than surviving from
// the previous artifact.
func TestARetiredArticleLosesItsPage(t *testing.T) {
	paths := blogPaths(t)
	retired := filepath.Join(paths.Output, blogDirectory, "an-article-that-was-withdrawn")
	if err := os.MkdirAll(retired, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(retired, pageIndexFile), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	staged := filepath.Join(paths.Output, filepath.FromSlash(blogSourceDirectory))
	if err := os.MkdirAll(staged, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := renderBlog(paths); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(retired); !os.IsNotExist(err) {
		t.Errorf("the withdrawn article kept its page: %v", err)
	}
	if _, err := os.Stat(staged); err != nil {
		t.Errorf("the staged sources were removed by the producer, before the build trims them: %v", err)
	}
}

// VALIDATES: every route the blog claims is a route the site publishes, and it
// claims all eight of them.
//
// AC-1 is arithmetic over the whole artifact, so a producer that claims a route
// the site does not publish would hide an unclaimed one somewhere else.
func TestTheBlogClaimsOnlyPublishedRoutes(t *testing.T) {
	paths := blogPaths(t)
	routes, err := renderBlog(paths)
	if err != nil {
		t.Fatal(err)
	}

	published := publishedArtifactRoutes(t)
	for _, route := range routes {
		if !slices.Contains(published, route) {
			t.Errorf("the blog claims %s, which the published site does not carry", route)
		}
	}
	var expected []string
	for _, route := range published {
		if strings.HasPrefix(route, "/blog/") || route == "/blog/" {
			expected = append(expected, route)
		}
	}
	if len(routes) != len(expected) {
		t.Fatalf("the blog claims %d routes, the published site carries %d under /blog/",
			len(routes), len(expected))
	}
}
