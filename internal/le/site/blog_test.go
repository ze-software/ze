// Design: website/AI.md -- the editorial blog is one producer over blog/posts/*.md
package site

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// blogPaths renders controlled editorial inputs, never a frozen copy of the
// public website. The undated article distinguishes page coverage from RSS.
func blogPaths(t *testing.T) Paths {
	t.Helper()
	source := blogPostsFixture(t, map[string]string{
		"reference-from-the-system.md": blogRenderingSource,
		"older.md":                     blogPostSource("Older article", "2026-08-04"),
		"undated.md":                   blogPostSource("Undated article", ""),
	})
	output := t.TempDir()
	copyFixture(t, filepath.Join("testdata", "published-site-facts.json"),
		filepath.Join(output, "data", "site-facts.json"))
	return Paths{Repository: repositoryRoot(t), Source: source, Output: output}
}

// blogRenderingSource exercises the page blocks with unmistakable test copy.
const blogRenderingSource = `---
title: A <renderer> & its sources
author: Fixture Writer
date: 2026-08-22
description: An index description, not the deck.
deck: A distinct deck & introduction.
image: assets/blog/probe.svg
image-dark: assets/blog/probe-dark.svg
image-alt: A light & dark illustration
---

## First section

Body with **emphasis** and [an external link](https://example.net/guide).

## Second section

- First item.
- Second item.

## Third section

A final paragraph.
`

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

// VALIDATES: a complete article preserves authored Markdown, escapes metadata,
// secures external links, and writes its shell and independent Markdown mirror.
func TestABlogArticleRendersItsAuthoredInputs(t *testing.T) {
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
		"<title>A &lt;renderer&gt; &amp; its sources - Ze Blog</title>",
		`<link rel="canonical" href="https://ze-software.net/blog/reference-from-the-system/" />`,
		`<meta name="author" content="Fixture Writer" />`,
		`<link rel="stylesheet" href="../../assets/site.css" />`,
		`<div id="site-header-mount" data-header-src="../../assets/header.html"`,
		`<main id="top" class="site-main-wide" tabindex="-1">`,
		"<footer>",
	} {
		if !strings.Contains(page, chrome) {
			t.Errorf("the article page is missing %q", chrome)
		}
	}

	for _, want := range []string{
		`<strong>emphasis</strong>`,
		`href="https://example.net/guide"`,
		`href="#first-section"`, `href="#second-section"`,
		`A distinct deck &amp; introduction.`,
		`<li>First item.</li>`, `<li>Second item.</li>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the article is missing %q", want)
		}
	}
	external := sliceBetween(t, page, `<a href="https://example.net/guide"`, ">")
	for _, want := range []string{`target="_blank"`, `rel="noopener"`} {
		if !strings.Contains(external, want) {
			t.Errorf("the external article link is missing %s", want)
		}
	}
	if strings.Contains(page, "<renderer>") {
		t.Error("the article title was emitted as markup")
	}
	mirror := readArtifact(t, paths.Output, "blog/reference-from-the-system/"+pageMirrorFile)
	want := "# A <renderer> & its sources\n\n*2026-08-22 by Fixture Writer*\n\n" +
		"A distinct deck & introduction.\n\n" +
		"![A light & dark illustration](../../assets/blog/probe.svg)\n\n" +
		"## First section\n\nBody with **emphasis** and [an external link](https://example.net/guide).\n\n" +
		"## Second section\n\n- First item.\n- Second item.\n\n" +
		"## Third section\n\nA final paragraph.\n"
	if mirror != want {
		t.Errorf("the mirror lost authored content:\ngot %q\nwant %q", mirror, want)
	}
}

// VALIDATES: a themed illustration and contents list retain their semantic
// markup and stylesheet hooks when rendered from authored inputs.
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
		`<div class="blog-article-meta"><time datetime="2026-08-22">2026-08-22</time><span>by Fixture Writer</span></div>`,
		`<figure class="blog-theme-image has-dark blog-article-visual reveal" role="img"`,
		`<img class="blog-theme-image-light" src="../../assets/blog/probe.svg"`,
		`<img class="blog-theme-image-dark" src="../../assets/blog/probe-dark.svg"`,
		`<nav class="blog-article-toc reveal" aria-label="Article sections">`,
		`<section class="md-content blog-article-content reveal" data-table-columns="off" data-code-copy="off">`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the article page is missing %q", want)
		}
	}
}

// VALIDATES: an article asking for key points gets the aside, and a prose
// number token is replaced by the span that lets a rebuild refresh the value.
//
// Both inputs are explicit so editorial changes cannot remove this coverage.
func TestAnArticleRendersItsKeyPointsAndProseNumbers(t *testing.T) {
	source := blogPostsFixture(t, map[string]string{"probe.md": strings.Join([]string{
		"---",
		"title: Counting the surface",
		"date: 2026-09-13",
		"author: Thomas Mangin",
		"description: One article carrying both a key-points list and a prose number.",
		"",
		"key-points: Facts stay with the owner | Pages publish checked views",
		"---",
		"",
		"Ze answers {{ze:cli-commands}} commands.",
		"",
	}, "\n")})

	output := t.TempDir()
	copyFixture(t, filepath.Join("testdata", "published-site-facts.json"),
		filepath.Join(output, "data", "site-facts.json"))
	paths := Paths{Repository: repositoryRoot(t), Source: source, Output: output}
	if _, err := renderBlog(paths); err != nil {
		t.Fatal(err)
	}

	page := readArtifact(t, paths.Output, "blog/probe/"+pageIndexFile)
	for _, want := range []string{
		`<aside class="blog-key-points reveal" aria-label="Key points">`,
		"<li>Facts stay with the owner</li>",
		"<li>Pages publish checked views</li>",
		`<span data-ze-stat="cli_commands">402</span>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the article page is missing %q", want)
		}
	}

	mirror := readArtifact(t, paths.Output, "blog/probe/"+pageMirrorFile)
	if !strings.Contains(mirror, "## Key points\n\n- Facts stay with the owner\n") {
		t.Errorf("the mirror carries no key points:\n%s", mirror)
	}
	if !strings.Contains(mirror, "Ze answers 402 commands.") {
		t.Errorf("the mirror did not resolve the prose number:\n%s", mirror)
	}
}

// VALIDATES: the index carries every authored title, date and description in
// source date order, with page links in HTML and mirror links in Markdown.
func TestTheBlogIndexRendersItsAuthoredInputs(t *testing.T) {
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
		`<article class="card card-post blog-card has-media tone-sky">`,
		`<div class="blog-theme-image has-dark blog-card-media" role="img"`,
		`<img class="blog-theme-image-light" src="../assets/blog/probe.svg"`,
		`<img class="blog-theme-image-dark" src="../assets/blog/probe-dark.svg"`,
	} {
		if !strings.Contains(page, chrome) {
			t.Errorf("the blog index is missing %q", chrome)
		}
	}

	mirror := readArtifact(t, paths.Output, blogDirectory+"/"+pageMirrorFile)
	for _, surface := range []struct {
		name    string
		content string
		wants   []string
	}{
		{"page", page, []string{
			`<h3><a href="reference-from-the-system/">A &lt;renderer&gt; &amp; its sources</a></h3>`,
			`<p>An index description, not the deck.</p>`,
			`<h3><a href="older/">Older article</a></h3>`,
			`<h3><a href="undated/">Undated article</a></h3>`,
		}},
		{"mirror", mirror, []string{
			"- [A <renderer> & its sources](reference-from-the-system/index.md) (2026-08-22): An index description, not the deck.",
			"- [Older article](older/index.md) (2026-08-04)",
			"- [Undated article](undated/index.md)",
		}},
	} {
		previous := -1
		for _, want := range surface.wants {
			at := strings.Index(surface.content, want)
			if at < 0 {
				t.Fatalf("%s is missing %q", surface.name, want)
			}
			if at <= previous {
				t.Errorf("%s is not in article order at %q", surface.name, want)
			}
			previous = at
		}
	}
	if strings.Contains(page, "A distinct deck") {
		t.Error("the index used the article deck instead of its description")
	}
}

// VALIDATES: presentation tones cycle by position rather than by topic or
// title. Eight controlled articles exercise the palette wraparound.
func TestAnIndexCardTakesTheToneAtItsPosition(t *testing.T) {
	articles := make([]blogArticle, len(presentationTones)+1)
	for index := range articles {
		articles[index] = blogArticle{Slug: "probe", Title: "Same title"}
	}
	page := blogIndexBody(articles)
	cards := strings.Split(page, `<article class="card card-post blog-card `)[1:]
	if len(cards) != len(articles) {
		t.Fatalf("rendered %d cards for %d articles", len(cards), len(articles))
	}
	for index, card := range cards {
		want := "tone-" + presentationTones[index%len(presentationTones)] + `">`
		if !strings.HasPrefix(card, want) {
			t.Errorf("card %d did not take its positional tone %q", index, want)
		}
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
	if items := strings.Count(feed, "<item>"); items != 2 {
		t.Errorf("the feed carries %d items, want the two dated articles", items)
	}
	if strings.Contains(feed, "/undated/") {
		t.Error("the feed assigned a publication date to an undated article")
	}
	for _, want := range []string{
		`<rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/">`,
		"<title>Ze blog</title>",
		"<link>https://ze-software.net/blog/</link>",
		"<lastBuildDate>Sat, 22 Aug 2026 00:00:00 +0000</lastBuildDate>",
		"<title>A &lt;renderer&gt; &amp; its sources</title>",
		`<guid isPermaLink="true">https://ze-software.net/blog/reference-from-the-system/</guid>`,
		"<pubDate>Sat, 22 Aug 2026 00:00:00 +0000</pubDate>",
		"<dc:creator>Fixture Writer</dc:creator>",
		"<title>Older article</title>",
		"<pubDate>Tue, 04 Aug 2026 00:00:00 +0000</pubDate>",
	} {
		if !strings.Contains(feed, want) {
			t.Errorf("the feed is missing %q", want)
		}
	}
	if first, second := strings.Index(feed, "reference-from-the-system"),
		strings.Index(feed, "/older/"); first > second {
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

// VALIDATES: claimed routes match both the controlled source population and
// the actual page/mirror artifacts, including an article excluded from RSS.
func TestTheBlogClaimsOnlyPublishedRoutes(t *testing.T) {
	paths := blogPaths(t)
	routes, err := renderBlog(paths)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"/blog/reference-from-the-system/", "/blog/older/", "/blog/undated/", "/blog/"}
	if !slices.Equal(routes, want) {
		t.Fatalf("the producer claims %v, want %v", routes, want)
	}
	for _, route := range routes {
		directory := strings.TrimPrefix(route, "/")
		if readArtifact(t, paths.Output, directory+pageIndexFile) == "" {
			t.Errorf("%s has no page", route)
		}
		if readArtifact(t, paths.Output, directory+pageMirrorFile) == "" {
			t.Errorf("%s has no mirror", route)
		}
	}
}
