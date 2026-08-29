// Design: website/AI.md -- the weekly changelog is one producer over changes/posts/*.md
package site

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// changesPaths lays out one artifact a changelog render can write into, reading
// the real website/ sources of this checkout.
func changesPaths(t *testing.T) Paths {
	t.Helper()
	root := repositoryRoot(t)
	return Paths{Repository: root, Source: filepath.Join(root, "website"), Output: t.TempDir()}
}

// testVocabulary is the tag vocabulary the synthetic weeks below are written
// against: one family for each color a test needs.
var testVocabulary = map[string]string{
	"BGP":          categoryRouting,
	"CLI":          categoryOperate,
	"Presentation": categoryMeta,
}

// writeWeek writes one weekly post into a temporary source tree and reads it
// back, so a test states the source and asserts on what a build makes of it.
func writeWeek(t *testing.T, name, source string) (changeWeek, error) {
	t.Helper()
	directory := filepath.Join(t.TempDir(), filepath.FromSlash(changesSourceDirectory))
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	return readChangeWeek(path, testVocabulary)
}

// VALIDATES: a week whose body carries no themed header is refused by name
// rather than published as a page with a title and nothing under it.
//
// The retired renderer skipped such a week with a warning and its build then
// exited non-zero on any warning, so the week was never published either way. A
// refusal says the same thing at the file that carries the mistake.
func TestAWeekWithNoSectionHeaderIsRefused(t *testing.T) {
	_, err := writeWeek(t, "2026-08-17.md",
		"---\ncovers: 2026-08-17 .. 2026-08-23\ntags: BGP\n---\n\nA week of work, with no headers at all.\n")

	if err == nil {
		t.Fatal("a week with no themed section header was accepted")
	}
	if !strings.Contains(err.Error(), "no themed section header") {
		t.Fatalf("the refusal reads %q, want it to name the missing headers", err)
	}
}

// VALIDATES: a tag whose family the vocabulary does not list is refused by name.
//
// The retired renderer colored such a chip neutral and warned. Neutral is also
// what a family the vocabulary DELIBERATELY maps to meta takes, so an author's
// typo published as a deliberate classification and the two were
// indistinguishable on the page. The warning then failed the build, so the week
// never reached a reader; refusing states which tag is wrong.
func TestAnUnknownTagFamilyIsRefused(t *testing.T) {
	_, err := writeWeek(t, "2026-08-17.md", weekSource("2026-08-17", "BGP, Telemtry"))

	if err == nil {
		t.Fatal("a tag outside the vocabulary was accepted")
	}
	for _, want := range []string{"Telemtry", topicsVocabularyFile} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the refusal reads %q, want it to name %q", err, want)
		}
	}
}

// VALIDATES: a namespaced tag is classified and filtered by the family before
// the colon and shown in full, and a family the vocabulary maps to meta takes
// the neutral chip.
func TestANamespacedTagIsClassifiedByItsFamilyAndShownInFull(t *testing.T) {
	week, err := writeWeek(t, "2026-08-17.md", weekSource("2026-08-17", "BGP, Presentation: LINX 126"))
	if err != nil {
		t.Fatal(err)
	}

	want := []changeTopic{
		{Label: "BGP", Category: categoryRouting, Key: "BGP"},
		{Label: "Presentation: LINX 126", Category: categoryMeta, Key: "Presentation"},
	}
	if !slices.Equal(week.Topics, want) {
		t.Fatalf("the chips are %v, want %v", week.Topics, want)
	}
	if !strings.Contains(changesIndexRow(week), `<span class="ch-chip cat-meta">Presentation: LINX 126</span>`) {
		t.Errorf("the neutral chip is not on the row:\n%s", changesIndexRow(week))
	}
}

// VALIDATES: a draft week keeps its page, so a reviewer can read it at its own
// URL, and stays out of the feed, so a subscriber is not sent a week nobody has
// checked.
func TestADraftWeekIsPublishedAndKeptOutOfTheFeed(t *testing.T) {
	draft, err := writeWeek(t, "2026-08-17.md",
		"---\ncovers: 2026-08-17 .. 2026-08-23\ntags: BGP\nstatus: DRAFT pending review\n---\n\n"+
			"**Ze Weekly Update**\n\nThe intro.\n\n**BGP**\nSomething shipped.\n")
	if err != nil {
		t.Fatal(err)
	}
	live, err := writeWeek(t, "2026-08-10.md", weekSource("2026-08-10", "CLI"))
	if err != nil {
		t.Fatal(err)
	}
	if !draft.IsDraft || live.IsDraft {
		t.Fatalf("the status line was read as draft=%v and the week without one as draft=%v",
			draft.IsDraft, live.IsDraft)
	}

	feed := changesFeed([]changeWeek{draft, live})
	if strings.Contains(feed, "2026-08-17") {
		t.Errorf("the draft week reached the feed:\n%s", feed)
	}
	if !strings.Contains(feed, "<lastBuildDate>Mon, 10 Aug 2026 00:00:00 +0000</lastBuildDate>") {
		t.Errorf("the feed dates itself from the draft week:\n%s", feed)
	}

	body, err := changeWeekBody(draft, "The intro.")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, `<span class="tag">Draft -- pending review</span>`) {
		t.Errorf("the draft page carries no draft marker:\n%s", body)
	}
	if !strings.Contains(changesIndexRow(draft), `<span class="ch-draft">pending review</span>`) {
		t.Errorf("the index row carries no draft marker:\n%s", changesIndexRow(draft))
	}
	if !strings.Contains(changeWeekMirror(draft), "# Week of 2026-08-17 (Draft -- pending review)") {
		t.Errorf("the mirror carries no draft marker:\n%s", changeWeekMirror(draft))
	}
}

// weekSource writes one minimal weekly post covering the week that starts on
// the given day and carrying the tags given.
func weekSource(start, tags string) string {
	return "---\ncovers: " + start + " .. \ntags: " + tags + "\n---\n\n" +
		"**Ze Weekly Update**\n\nThe intro.\n\n**BGP**\nSomething shipped.\n"
}

// VALIDATES: one week's page reads as the published one, carries the whole site
// shell with its page sidebar, and its Markdown mirror matches the published
// mirror byte for byte.
//
// The published week is 2026-08-17 at gh-pages HEAD 2fa8fa2ad, the newest one,
// which carries five themed sections and the widest chip set of any week.
func TestAWeekReadsAsThePublishedWeek(t *testing.T) {
	paths := changesPaths(t)

	routes, err := renderChanges(paths)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(routes, "/project/changes/2026-08-17/") {
		t.Fatalf("the producer claimed %d routes, none of them the newest week", len(routes))
	}

	page := readArtifact(t, paths.Output, "project/changes/2026-08-17/"+pageIndexFile)
	for _, chrome := range []string{
		"<title>Week of 2026-08-17 - Ze</title>",
		`<link rel="canonical" href="https://ze-software.net/project/changes/2026-08-17/" />`,
		`<link rel="alternate" type="application/rss+xml" title="Ze weekly updates" href="../feed.xml" />`,
		`<link rel="stylesheet" href="../../../assets/site.css" />`,
		`<main id="top" class="has-page-sidebar" tabindex="-1">`,
		`<aside class="page-sidebar" aria-label="Related page links">`,
		`<section class="blog-post" aria-labelledby="post-title">`,
		"<div class=\"blog-block\" aria-label=\"🖥️ CLI and APIs\">",
		`<a class="page-sidebar-link" href="../../../project/milestones/">`,
		"<footer>",
	} {
		if !strings.Contains(page, chrome) {
			t.Errorf("the week page is missing %q", chrome)
		}
	}

	got := visibleText(mainContent(t, page))
	want := visibleText(readFixture(t, "published-changes-week.html"))
	if got != want {
		t.Errorf("the week reads as\n  %q\nthe published week reads as\n  %q", got, want)
	}

	mirror := readArtifact(t, paths.Output, "project/changes/2026-08-17/"+pageMirrorFile)
	if mirror != readFixture(t, "published-changes-week.md") {
		t.Errorf("the mirror is\n%q\nthe published mirror is\n%q",
			mirror, readFixture(t, "published-changes-week.md"))
	}
}

// VALIDATES: a bullet list written straight under its paragraph, as the weekly
// posts are for a chat client, reaches the mirror with the blank line Markdown
// needs between the two.
//
// goldmark parses either form the same way, so the page cannot show this. The
// mirror is read as Markdown by whatever opens it next, so it can.
func TestAListWrittenUnderItsParagraphGetsItsBlankLine(t *testing.T) {
	week, err := writeWeek(t, "2026-08-17.md",
		"---\ncovers: 2026-08-17 .. 2026-08-23\ntags: BGP\n---\n\n"+
			"**Ze Weekly Update**\n\nThe intro.\n\n**BGP**\nNew:\n- One thing.\n- Another thing.\n")
	if err != nil {
		t.Fatal(err)
	}

	mirror := changeWeekMirror(week)
	if !strings.Contains(mirror, "New:\n\n- One thing.\n- Another thing.") {
		t.Errorf("the mirror runs the list into its paragraph:\n%s", mirror)
	}
}

// VALIDATES: the index reads as the published index, and its mirror matches the
// published mirror byte for byte.
func TestTheChangesIndexReadsAsThePublishedIndex(t *testing.T) {
	paths := changesPaths(t)
	if _, err := renderChanges(paths); err != nil {
		t.Fatal(err)
	}

	page := readArtifact(t, paths.Output, changesIndexDest)
	for _, chrome := range []string{
		"<title>Changes - Ze</title>",
		`<link rel="canonical" href="https://ze-software.net/project/changes/" />`,
		`<link rel="alternate" type="application/rss+xml" title="Ze weekly updates" href="feed.xml" />`,
		".ch-chip.cat-meta {",
		`<section aria-labelledby="changes-title">`,
		`<main id="top" class="has-page-sidebar" tabindex="-1">`,
	} {
		if !strings.Contains(page, chrome) {
			t.Errorf("the changes index is missing %q", chrome)
		}
	}

	got := visibleText(mainContent(t, page))
	want := visibleText(readFixture(t, "published-changes-index.html"))
	if got != want {
		t.Errorf("the index reads as\n  %q\nthe published index reads as\n  %q", got, want)
	}

	mirror := readArtifact(t, paths.Output, changesDirectory+"/"+pageMirrorFile)
	if mirror != readFixture(t, "published-changes-index.md") {
		t.Errorf("the index mirror is\n%q\nthe published one is\n%q",
			mirror, readFixture(t, "published-changes-index.md"))
	}
}

// VALIDATES: the filter buttons and each week's category list are written in
// the legend's own order, so the buttons above the list and the classes on a
// row cannot disagree about where a category sits.
func TestTheCategoryLegendKeepsItsDeclaredOrder(t *testing.T) {
	paths := changesPaths(t)
	if _, err := renderChanges(paths); err != nil {
		t.Fatal(err)
	}

	page := readArtifact(t, paths.Output, changesIndexDest)
	previous := -1
	for _, category := range categoryFilterOrder {
		at := strings.Index(page, `<button class="cat-`+category+`" data-cat="`+category+`"`)
		if at < 0 {
			t.Fatalf("the legend carries no button for %s", category)
		}
		if at < previous {
			t.Fatalf("the %s button sits above the category declared before it", category)
		}
		previous = at
	}
	if !strings.Contains(page,
		`<a class="ch-week" data-cats="operate routing automate observe secure platform meta" href="2026-08-17/"`) {
		t.Errorf("the newest week's category list is not in the legend's order")
	}
}

// VALIDATES: data/changes.json states every week newest first, with the intro,
// the draft flag and the chips each week carries and nothing else.
//
// The file is a contract another producer reads, so the two newest weeks are
// compared against the published file field for field.
func TestTheChangesIndexFileIsNewestFirst(t *testing.T) {
	paths := changesPaths(t)
	if _, err := renderChanges(paths); err != nil {
		t.Fatal(err)
	}

	content := readArtifact(t, paths.Output, changesIndexFile)
	var weeks []changeWeek
	if err := json.Unmarshal([]byte(content), &weeks); err != nil {
		t.Fatalf("the published index does not parse: %v", err)
	}
	for index := 1; index < len(weeks); index++ {
		if weeks[index-1].Slug <= weeks[index].Slug {
			t.Fatalf("week %s is published above %s, so the file is not newest first",
				weeks[index-1].Slug, weeks[index].Slug)
		}
	}

	var published []changeWeek
	if err := json.Unmarshal([]byte(readFixture(t, "published-changes-head.json")), &published); err != nil {
		t.Fatal(err)
	}
	for index, want := range published {
		if weeks[index].Slug != want.Slug || weeks[index].Intro != want.Intro ||
			weeks[index].IsDraft != want.IsDraft || !slices.Equal(weeks[index].Topics, want.Topics) {
			t.Errorf("week %d is %+v, the published file states %+v", index, weeks[index], want)
		}
	}
	if !strings.HasPrefix(content, "[\n  {\n    \"slug\":") {
		t.Errorf("the file is not indented as the published one is:\n%s", content[:80])
	}
}

// VALIDATES: the feed is published at both of its addresses, with the same
// content at each.
//
// A feed client holds the URL it subscribed with and no redirect makes it
// follow, so the address the changelog had before it moved out of blog/ is
// still served.
func TestTheChangesFeedIsPublishedAtBothAddresses(t *testing.T) {
	paths := changesPaths(t)
	if _, err := renderChanges(paths); err != nil {
		t.Fatal(err)
	}

	feed := readArtifact(t, paths.Output, changesFeedDest)
	if legacy := readArtifact(t, paths.Output, changesLegacyFeedDest); legacy != feed {
		t.Errorf("the two feed addresses serve different content")
	}
	for _, want := range []string{
		`<rss version="2.0">`,
		"<title>Ze weekly updates</title>",
		"<link>https://ze-software.net/project/changes/</link>",
		"<lastBuildDate>Mon, 17 Aug 2026 00:00:00 +0000</lastBuildDate>",
		"<title>Week of 2026-08-17</title>",
		`<guid isPermaLink="true">https://ze-software.net/project/changes/2026-08-17/</guid>`,
		"<pubDate>Mon, 17 Aug 2026 00:00:00 +0000</pubDate>",
	} {
		if !strings.Contains(feed, want) {
			t.Errorf("the feed is missing %q", want)
		}
	}
	if first, second := strings.Index(feed, "2026-08-17/"), strings.Index(feed, "2025-12-15/"); first > second {
		t.Errorf("the feed is oldest first")
	}
}

// VALIDATES: a week this site no longer carries loses its page, and a directory
// that is not a week is left alone.
func TestARetiredWeekLosesItsPage(t *testing.T) {
	paths := changesPaths(t)
	retired := filepath.Join(paths.Output, filepath.FromSlash(changesDirectory), "2019-01-07")
	kept := filepath.Join(paths.Output, filepath.FromSlash(changesDirectory), "archive")
	for _, directory := range []string{retired, kept} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, pageIndexFile), []byte("<html></html>"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := renderChanges(paths); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(retired); !os.IsNotExist(err) {
		t.Errorf("the retired week kept its page: %v", err)
	}
	if _, err := os.Stat(kept); err != nil {
		t.Errorf("a directory that is not a week was removed: %v", err)
	}
}

// VALIDATES: every route the changelog claims is a route the site publishes,
// and it claims all thirty-eight of them.
func TestTheChangesClaimOnlyPublishedRoutes(t *testing.T) {
	paths := changesPaths(t)
	routes, err := renderChanges(paths)
	if err != nil {
		t.Fatal(err)
	}

	published := publishedArtifactRoutes(t)
	for _, route := range routes {
		if !slices.Contains(published, route) {
			t.Errorf("the changelog claims %s, which the published site does not carry", route)
		}
	}
	expected := 0
	for _, route := range published {
		if strings.HasPrefix(route, "/"+changesDirectory+"/") {
			expected++
		}
	}
	if len(routes) != expected {
		t.Fatalf("the changelog claims %d routes, the published site carries %d under /%s/",
			len(routes), expected, changesDirectory)
	}
}
