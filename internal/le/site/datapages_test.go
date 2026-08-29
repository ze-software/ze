// Design: website/AI.md -- the data pages read one committed file and publish its own order
package site

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// dataPagePaths lays out one source tree carrying the named data fixtures, plus
// the repository's own page-links.json so a page takes the sidebar it publishes
// with.
//
// The data files are snapshots of what gh-pages 2fa8fa2ad published, so the
// parity tests below compare one fixed input against one fixed page. Reading
// the live website/data would make the golden move whenever a card is added.
func dataPagePaths(t *testing.T, fixtures map[string]string) Paths {
	t.Helper()
	root := repositoryRoot(t)
	source := t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, fixture := range fixtures {
		copyFixture(t, filepath.Join("testdata", fixture), filepath.Join(source, "data", name))
	}
	copyFixture(t, filepath.Join(root, "website", "data", "page-links.json"),
		filepath.Join(source, "data", "page-links.json"))
	return Paths{Repository: root, Source: source, Output: t.TempDir()}
}

// featuresPaths lays out the source the features page reads.
func featuresPaths(t *testing.T) Paths {
	t.Helper()
	return dataPagePaths(t, map[string]string{featuresDataFile: "published-features.json"})
}

// milestonesPaths lays out the source the timeline reads.
func milestonesPaths(t *testing.T) Paths {
	t.Helper()
	return dataPagePaths(t, map[string]string{milestonesDataFile: "published-milestones.json"})
}

// talksPaths lays out the source the talks listing reads.
func talksPaths(t *testing.T) Paths {
	t.Helper()
	return dataPagePaths(t, map[string]string{talksDataFile: "published-talks.json"})
}

// writeSourceData replaces one data file of a laid-out source tree, so a test
// can state the input it wants rather than the whole tree.
func writeSourceData(t *testing.T, paths Paths, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(paths.Source, "data", name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: the features page publishes the sections and the cards in the data
// file's own order, and never in an order a Go map or a sort produced.
//
// The method is positional: every section id and every card title is looked up
// in the rendered page and its offset must rise. A single misordered card moves
// one offset below the one before it.
func TestTheFeaturesPageKeepsTheDataFilesOwnOrder(t *testing.T) {
	paths := featuresPaths(t)

	if _, err := renderFeatures(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, featuresDest)

	var data featureData
	if err := readSourceJSON(paths.Source, featuresDataFile, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Sections) != 3 {
		t.Fatalf("the fixture holds %d sections, want the published 3", len(data.Sections))
	}

	previous := -1
	cards := 0
	for _, section := range data.Sections {
		at := strings.Index(page, `<section id="`+section.ID+`" aria-labelledby="`+section.ID+`-title"`)
		if at < 0 {
			t.Fatalf("the page carries no %q section", section.ID)
		}
		if at < previous {
			t.Fatalf("section %q sits above the section declared before it", section.ID)
		}
		previous = at
		for _, card := range section.Cards {
			cards++
			at := strings.Index(page, ">"+card.Title+"</a></h3>")
			if at < 0 {
				t.Fatalf("the page carries no card titled %q", card.Title)
			}
			if at < previous {
				t.Fatalf("card %q sits above the card declared before it", card.Title)
			}
			previous = at
		}
	}
	if cards != 56 {
		t.Fatalf("the fixture holds %d cards, want the published 56", cards)
	}
}

// VALIDATES: the category legend is written in the page's own declared order,
// never in the order the data happens to name the categories.
//
// The data's first card is "automate", which sits fourth in the legend. A
// legend derived from the data would put it first, and would drop any category
// no card carries.
func TestTheFeatureLegendFollowsItsOwnCategoryOrder(t *testing.T) {
	paths := featuresPaths(t)

	if _, err := renderFeatures(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, featuresDest)

	previous := -1
	for _, category := range legendCategories {
		at := strings.Index(page, `<button class="cat-`+category+`" data-cat="`+category+`"`)
		if at < 0 {
			t.Fatalf("the legend carries no button for %s", category)
		}
		if at < previous {
			t.Fatalf("the %s button sits above the category declared before it", category)
		}
		previous = at
	}
	for _, count := range []string{
		`aria-label="Filter features by Operate, 6 features"`,
		`aria-label="Filter features by Routing, 9 features"`,
		`aria-label="Filter features by Platform, 6 features"`,
	} {
		if !strings.Contains(page, count) {
			t.Errorf("the legend is missing %s", count)
		}
	}
}

// VALIDATES: the features page reads as the published page and carries the same
// chrome, and its mirror is the published mirror byte for byte.
func TestTheFeaturesPageReadsAsThePublishedPage(t *testing.T) {
	paths := featuresPaths(t)

	routes, err := renderFeatures(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 || routes[0] != "/features/" {
		t.Fatalf("the producer claimed %v, want [/features/]", routes)
	}

	page := readArtifact(t, paths.Output, featuresDest)
	for _, chrome := range []string{
		"<title>Features - Ze</title>",
		`<link rel="canonical" href="https://ze-software.net/features/" />`,
		`<link rel="stylesheet" href="../assets/site.css" />`,
		`<div id="site-header-mount" data-header-src="../assets/header.html"`,
		`<main id="top" class="has-page-sidebar" tabindex="-1">`,
		`<section aria-labelledby="features-title">`,
		`<h1 id="features-title">Every feature Ze ships.</h1>`,
		`<aside class="page-sidebar" aria-label="Related page links">`,
		"<footer>",
	} {
		if !strings.Contains(page, chrome) {
			t.Errorf("the features page is missing %q", chrome)
		}
	}

	got := visibleText(mainContent(t, page))
	want := visibleText(readFixture(t, "published-features-body.html"))
	if got != want {
		t.Errorf("the features page reads as\n  %q\nthe published page reads as\n  %q", got, want)
	}

	mirror := readArtifact(t, paths.Output, "features/"+pageMirrorFile)
	if mirror != readFixture(t, "published-features.md") {
		t.Errorf("the mirror is\n%q\nthe published mirror is\n%q",
			mirror, readFixture(t, "published-features.md"))
	}
}

// VALIDATES: every element the page labels by id keeps that link.
//
// visibleText cannot see an attribute, so the parity test above passes whether
// or not aria-labelledby names an element that exists. Phase 5 broke five such
// attributes with every parity test still green.
func TestEveryFeatureSectionIsLabelledByItsOwnHeading(t *testing.T) {
	paths := featuresPaths(t)

	if _, err := renderFeatures(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, featuresDest)

	for _, pair := range [][2]string{
		{`aria-labelledby="features-title"`, `id="features-title"`},
		{`aria-labelledby="core-title"`, `id="core-title"`},
		{`aria-labelledby="experimental-title"`, `id="experimental-title"`},
		{`aria-labelledby="roadmap-title"`, `id="roadmap-title"`},
	} {
		if !strings.Contains(page, pair[0]) {
			t.Errorf("the page carries no %s", pair[0])
		}
		if !strings.Contains(page, pair[1]) {
			t.Errorf("%s names %s, which the page does not carry", pair[0], pair[1])
		}
	}
}

// VALIDATES: a roadmap card links out of the site and opens in a new tab, while
// a shipped card links relative to the features page.
func TestAnExternalFeatureCardLeavesTheSite(t *testing.T) {
	paths := featuresPaths(t)

	if _, err := renderFeatures(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, featuresDest)

	external := `<h3><a href="https://github.com/ze-software/ze/blob/main/plan/spec-kernel-lockdown-hardening.md" target="_blank" rel="noopener">Kernel Lockdown</a></h3>`
	if !strings.Contains(page, external) {
		t.Errorf("the page is missing the external card link\n  %s", external)
	}
	internal := `<h3><a href="../features/ai-first/">AI Tool Interfaces</a></h3>`
	if !strings.Contains(page, internal) {
		t.Errorf("the page is missing the internal card link\n  %s", internal)
	}
}

// VALIDATES: a card whose category is not one of the seven is refused by name.
//
// An unknown category renders a cat- class no stylesheet defines, so the card
// loses its color and its filter button with nothing to notice. The retired
// build refused the same input.
func TestAFeatureCardWithAnUnknownCategoryIsRefused(t *testing.T) {
	paths := featuresPaths(t)
	writeSourceData(t, paths, featuresDataFile, `{"sections":[
		{"id":"core","heading":"Core","lead":"Lead","note":null,"cards":[
			{"category":"telepathy","status":null,"title":"Mind Reading","href":"features/","external":false,"chips":[],"bullets":[]}]},
		{"id":"experimental","heading":"Experimental","lead":"Lead","note":null,"cards":[]}]}`)

	_, err := renderFeatures(paths)
	if err == nil {
		t.Fatal("a card in an unknown category was published")
	}
	for _, named := range []string{"Mind Reading", "telepathy"} {
		if !strings.Contains(err.Error(), named) {
			t.Errorf("the refusal is %q, which does not name %q", err, named)
		}
	}
}

// VALIDATES: a data file with no core or no experimental section is refused,
// rather than publishing a feature count that means something else.
func TestFeaturesWithNoShippedSectionAreRefused(t *testing.T) {
	paths := featuresPaths(t)
	writeSourceData(t, paths, featuresDataFile, `{"sections":[
		{"id":"core","heading":"Core","lead":"Lead","note":null,"cards":[]}]}`)

	_, err := renderFeatures(paths)
	if err == nil {
		t.Fatal("a file with no experimental section was published")
	}
	if !strings.Contains(err.Error(), "experimental") {
		t.Errorf("the refusal is %q, which does not name the missing section", err)
	}
}

// VALIDATES: the timeline sorts newest first and groups a quarter's milestones
// into one run, so a quarter heading appears once.
//
// The method reads the rendered page: the dates on the spine must never rise,
// and each quarter heading must be unique.
func TestTheTimelineSortsNewestFirstAndGroupsByQuarter(t *testing.T) {
	paths := milestonesPaths(t)

	if _, err := renderMilestones(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, milestonesDest)

	dates := betweenAll(page, `<time datetime="`, `"`)
	if len(dates) != 26 {
		t.Fatalf("the page carries %d milestones, want the published 26", len(dates))
	}
	for index := 1; index < len(dates); index++ {
		if dates[index] > dates[index-1] {
			t.Errorf("milestone %d is dated %s, above the older %s", index, dates[index], dates[index-1])
		}
	}

	quarters := betweenAll(page, `<h2 class="tl-quarter-head">`, `</h2>`)
	seen := make(map[string]bool, len(quarters))
	for _, quarter := range quarters {
		if seen[quarter] {
			t.Errorf("%s heads two blocks, so its milestones are not one run", quarter)
		}
		seen[quarter] = true
	}
	if len(quarters) != 4 {
		t.Fatalf("the page carries %d quarters, want the published 4", len(quarters))
	}
	if quarters[0] != "Q3 2026" || quarters[len(quarters)-1] != "Q4 2025" {
		t.Errorf("the quarters run %v, want newest first", quarters)
	}
}

// VALIDATES: two milestones sharing one date keep the data file's own order, so
// a rebuild over an unchanged file publishes the same page.
func TestTwoMilestonesOnOneDateKeepTheFilesOrder(t *testing.T) {
	paths := milestonesPaths(t)
	writeSourceData(t, paths, milestonesDataFile, `{"intro":"Intro.","milestones":[
		{"date":"2026-01-05","title":"Older","category":"routing","blog":"","blurb":"Older."},
		{"date":"2026-03-02","title":"First of March","category":"operate","blog":"","blurb":"One."},
		{"date":"2026-03-02","title":"Second of March","category":"secure","blog":"","blurb":"Two."}]}`)

	if _, err := renderMilestones(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, milestonesDest)

	titles := betweenAll(page, `<h3 class="tl-title">`, `</h3>`)
	want := []string{"First of March", "Second of March", "Older"}
	if len(titles) != len(want) {
		t.Fatalf("the page lists %v, want %v", titles, want)
	}
	for index := range want {
		if titles[index] != want[index] {
			t.Fatalf("the page lists %v, want %v", titles, want)
		}
	}
}

// VALIDATES: the timeline reads as the published page and its mirror is the
// published mirror byte for byte.
func TestTheTimelineReadsAsThePublishedPage(t *testing.T) {
	paths := milestonesPaths(t)

	routes, err := renderMilestones(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 || routes[0] != "/project/milestones/" {
		t.Fatalf("the producer claimed %v, want [/project/milestones/]", routes)
	}

	page := readArtifact(t, paths.Output, milestonesDest)
	for _, chrome := range []string{
		"<title>Milestones - Ze</title>",
		`<link rel="canonical" href="https://ze-software.net/project/milestones/" />`,
		`<link rel="stylesheet" href="../../assets/site.css" />`,
		`<main id="top" class="has-page-sidebar" tabindex="-1">`,
		`<section aria-labelledby="milestones-title">`,
		`<h1 id="milestones-title">The road so far.</h1>`,
		`<section class="reveal" aria-label="Milestone timeline">`,
		".tl-quarter-head {",
		`<aside class="page-sidebar" aria-label="Related page links">`,
		"<footer>",
	} {
		if !strings.Contains(page, chrome) {
			t.Errorf("the timeline is missing %q", chrome)
		}
	}

	got := visibleText(mainContent(t, page))
	want := visibleText(readFixture(t, "published-milestones-body.html"))
	if got != want {
		t.Errorf("the timeline reads as\n  %q\nthe published page reads as\n  %q", got, want)
	}

	mirror := readArtifact(t, paths.Output, "project/milestones/"+pageMirrorFile)
	if mirror != readFixture(t, "published-milestones.md") {
		t.Errorf("the mirror is\n%q\nthe published mirror is\n%q",
			mirror, readFixture(t, "published-milestones.md"))
	}
}

// VALIDATES: the talks listing sorts newest first, and reads as the published
// listing with a byte-identical mirror.
func TestTheTalksListingReadsAsThePublishedListing(t *testing.T) {
	paths := talksPaths(t)

	routes, err := renderTalkIndex(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 || routes[0] != "/talks/" {
		t.Fatalf("the producer claimed %v, want [/talks/]", routes)
	}

	page := readArtifact(t, paths.Output, talksDest)
	for _, chrome := range []string{
		"<title>Talks - Ze</title>",
		`<link rel="canonical" href="https://ze-software.net/talks/" />`,
		`<link rel="stylesheet" href="../assets/site.css" />`,
		`<main id="top" tabindex="-1">`,
		`<section id="talks" aria-labelledby="talks-title">`,
		`<h1 id="talks-title">Talks and presentations.</h1>`,
		"<footer>",
	} {
		if !strings.Contains(page, chrome) {
			t.Errorf("the talks listing is missing %q", chrome)
		}
	}
	if strings.Contains(page, "page-sidebar") {
		t.Error("the talks listing carries a sidebar the published page has not")
	}

	got := visibleText(mainContent(t, page))
	want := visibleText(readFixture(t, "published-talks-body.html"))
	if got != want {
		t.Errorf("the talks listing reads as\n  %q\nthe published listing reads as\n  %q", got, want)
	}

	mirror := readArtifact(t, paths.Output, "talks/"+pageMirrorFile)
	if mirror != readFixture(t, "published-talks.md") {
		t.Errorf("the mirror is\n%q\nthe published mirror is\n%q",
			mirror, readFixture(t, "published-talks.md"))
	}
}

// VALIDATES: the talks listing is written newest first whatever order the data
// file states, and the file's order alone would publish the older talk first.
func TestTheTalksListingSortsNewestFirst(t *testing.T) {
	paths := talksPaths(t)
	writeSourceData(t, paths, talksDataFile, `{"talks":[
		{"slug":"oldest","venue":"First Venue","title":"Oldest","date":"2025-01-05"},
		{"slug":"newest","venue":"Third Venue","title":"Newest","date":"2026-09-01"},
		{"slug":"middle","venue":"Second Venue","title":"Middle","date":"2026-02-02"}]}`)

	if _, err := renderTalkIndex(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, talksDest)

	venues := betweenAll(page, "<h3>", "</h3>")
	want := []string{"Third Venue", "Second Venue", "First Venue"}
	if len(venues) != len(want) {
		t.Fatalf("the listing names %v, want %v", venues, want)
	}
	for index := range want {
		if venues[index] != want[index] {
			t.Fatalf("the listing names %v, want %v", venues, want)
		}
	}
}

// VALIDATES: the talks producer writes the listing page and its mirror, and
// nothing under a talk's own directory.
//
// The asymmetry is load-bearing: the retired build regenerated talks/index.html
// and froze every other talks/ path, so an authored deck survives a build
// untouched. refreshTalks owns the decks; this producer must not reach them.
func TestTheTalksProducerLeavesTheDecksAlone(t *testing.T) {
	paths := talksPaths(t)

	deck := filepath.Join(paths.Output, "talks", "linx-2026-06")
	if err := os.MkdirAll(deck, 0o755); err != nil {
		t.Fatal(err)
	}
	authored := "<html><body>the authored deck</body></html>\n"
	for _, name := range []string{"index.html", "index-inlined.html", pageMirrorFile} {
		if err := os.WriteFile(filepath.Join(deck, name), []byte(authored), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := renderTalkIndex(paths); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"index.html", "index-inlined.html", pageMirrorFile} {
		content := readArtifact(t, paths.Output, "talks/linx-2026-06/"+name)
		if content != authored {
			t.Errorf("the producer rewrote the deck's %s:\n%s", name, content)
		}
	}
	written := publishedFiles(t, filepath.Join(paths.Output, "talks"))
	want := []string{"index.html", "index.md", "linx-2026-06/index-inlined.html",
		"linx-2026-06/index.html", "linx-2026-06/index.md"}
	if strings.Join(written, " ") != strings.Join(want, " ") {
		t.Errorf("the talks directory holds %v, want %v", written, want)
	}
}

// VALIDATES: a talk with no date is refused by name rather than sorted to the
// end of the listing with an empty date line.
func TestATalkWithNoDateIsRefused(t *testing.T) {
	paths := talksPaths(t)
	writeSourceData(t, paths, talksDataFile,
		`{"talks":[{"slug":"undated","venue":"Somewhere","title":"Undated","date":""}]}`)

	_, err := renderTalkIndex(paths)
	if err == nil {
		t.Fatal("a talk with no date was published")
	}
	if !strings.Contains(err.Error(), "Undated") {
		t.Errorf("the refusal is %q, which does not name the talk", err)
	}
}

// VALIDATES: the four data pages together claim four routes, each of them one
// the site publishes, and no route twice.
//
// The claims are collected by RUNNING the producers into one artifact rather
// than from a list a test states, because a producer answers what it wrote. A
// route two of them wrote would appear twice here and the check below is what
// the coverage arithmetic would otherwise be the first to see, at phase 10.
func TestTheDataPagesClaimEachPublishedRouteOnce(t *testing.T) {
	published := make(map[string]bool)
	for _, route := range publishedArtifactRoutes(t) {
		published[route] = true
	}

	paths := dataPagePaths(t, map[string]string{
		featuresDataFile:     "published-features.json",
		milestonesDataFile:   "published-milestones.json",
		talksDataFile:        "published-talks.json",
		dependenciesDataFile: "published-dependencies.json",
	})
	repository := t.TempDir()
	copyFixture(t, filepath.Join("testdata", "published-go.mod"), filepath.Join(repository, "go.mod"))
	paths.Repository = repository

	writers := make(map[string][]string)
	for _, producer := range []Producer{
		{Name: featuresDirectory, Render: renderFeatures},
		{Name: "milestones", Render: renderMilestones},
		{Name: talksDirectory, Render: renderTalkIndex},
		{Name: dependenciesDirectory, Render: renderDependencies},
	} {
		routes, err := producer.Render(paths)
		if err != nil {
			t.Fatalf("producer %s: %v", producer.Name, err)
		}
		for _, route := range routes {
			writers[route] = append(writers[route], producer.Name)
		}
	}

	if len(writers) != 4 {
		t.Fatalf("the data pages claim %d routes, want 4: %v", len(writers), writers)
	}
	for route, names := range writers {
		if len(names) != 1 {
			t.Errorf("%s is written by %v, and exactly one producer may write a route", route, names)
		}
		if !published[route] {
			t.Errorf("the data pages claim %s, which the published site has not", route)
		}
	}
	t.Logf("the data pages claim %d of the %d published routes, leaving %d",
		len(writers), len(published), len(published)-len(writers))
}

// betweenAll answers every substring between one opening and closing marker, in
// the order they appear.
func betweenAll(text, opening, closing string) []string {
	var found []string
	rest := text
	for {
		start := strings.Index(rest, opening)
		if start < 0 {
			return found
		}
		rest = rest[start+len(opening):]
		end := strings.Index(rest, closing)
		if end < 0 {
			return found
		}
		found = append(found, rest[:end])
		rest = rest[end:]
	}
}

// publishedFiles answers every file under one directory, as slash-separated
// paths relative to it, sorted.
func publishedFiles(t *testing.T, directory string) []string {
	t.Helper()
	var found []string
	err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(directory, path)
		if err != nil {
			return err
		}
		found = append(found, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

// VALIDATES: every remaining refusal a data page carries, each named by the
// thing it refuses.
//
// A guard with no test is a guard nobody has run. Each case here states one
// malformed data file and asserts that the producer refuses it and says which
// input is wrong, because a build that stops without naming the file leaves an
// author reading the renderer.
func TestAMalformedDataFileIsRefusedByName(t *testing.T) {
	cases := []struct {
		name    string
		file    string
		content string
		render  func(Paths) ([]string, error)
		names   string
	}{
		{
			name:    "a section with no id",
			file:    featuresDataFile,
			content: `{"sections":[{"id":"","heading":"H","lead":"L","note":null,"cards":[]}]}`,
			render:  renderFeatures,
			names:   "no id",
		},
		{
			name: "two sections sharing an id",
			file: featuresDataFile,
			content: `{"sections":[
				{"id":"core","heading":"H","lead":"L","note":null,"cards":[]},
				{"id":"core","heading":"H","lead":"L","note":null,"cards":[]}]}`,
			render: renderFeatures,
			names:  `two "core" sections`,
		},
		{
			name:    "a card that links nowhere",
			file:    featuresDataFile,
			content: `{"sections":[{"id":"core","heading":"H","lead":"L","note":null,"cards":[{"category":"operate","status":null,"title":"Linkless","href":"","external":false,"chips":[],"bullets":[]}]}]}`,
			render:  renderFeatures,
			names:   "Linkless",
		},
		{
			name:    "a card with an unknown status",
			file:    featuresDataFile,
			content: `{"sections":[{"id":"core","heading":"H","lead":"L","note":null,"cards":[{"category":"operate","status":"almost","title":"Almost","href":"features/","external":false,"chips":[],"bullets":[]}]}]}`,
			render:  renderFeatures,
			names:   "almost",
		},
		{
			name:    "a timeline with no lead",
			file:    milestonesDataFile,
			content: `{"intro":"","milestones":[]}`,
			render:  renderMilestones,
			names:   "no intro",
		},
		{
			name:    "a milestone in an unknown category",
			file:    milestonesDataFile,
			content: `{"intro":"Intro.","milestones":[{"date":"2026-01-01","title":"Odd","category":"telepathy","blog":"","blurb":"B."}]}`,
			render:  renderMilestones,
			names:   "telepathy",
		},
		{
			name:    "a milestone with an unparseable date",
			file:    milestonesDataFile,
			content: `{"intro":"Intro.","milestones":[{"date":"the fifth","title":"Undated","category":"operate","blog":"","blurb":"B."}]}`,
			render:  renderMilestones,
			names:   "Undated",
		},
		{
			name:    "a talk with no slug",
			file:    talksDataFile,
			content: `{"talks":[{"slug":"","venue":"Venue","title":"Title","date":"2026-01-01"}]}`,
			render:  renderTalkIndex,
			names:   "slug",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			paths := dataPagePaths(t, map[string]string{
				featuresDataFile:   "published-features.json",
				milestonesDataFile: "published-milestones.json",
				talksDataFile:      "published-talks.json",
			})
			writeSourceData(t, paths, test.file, test.content)

			_, err := test.render(paths)
			if err == nil {
				t.Fatalf("%s was published", test.name)
			}
			if !strings.Contains(err.Error(), test.names) {
				t.Errorf("the refusal is %q, which does not name %q", err, test.names)
			}
		})
	}
}

// VALIDATES: a data file that is not JSON is refused with the decoder's own
// complaint, named against the file it read.
//
// The assertion is on the decoder's error type rather than on a phrase, because
// every later guard also names data/features.json: a test that only looked for
// the file name would pass with the decode error swallowed.
func TestADataFileThatIsNotJSONIsRefused(t *testing.T) {
	paths := featuresPaths(t)
	writeSourceData(t, paths, featuresDataFile, "not json at all\n")

	_, err := renderFeatures(paths)
	if err == nil {
		t.Fatal("a data file that is not JSON was published")
	}
	if _, decoded := errors.AsType[*json.SyntaxError](err); !decoded {
		t.Errorf("the refusal is %q, which does not carry the decoder's own complaint", err)
	}
	if !strings.Contains(err.Error(), "data/"+featuresDataFile) {
		t.Errorf("the refusal is %q, which does not name the file it read", err)
	}
}
