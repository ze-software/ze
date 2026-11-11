// Design: website/AI.md -- the project overview shares canonical reports and publishing.
package site

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestBuildPublishesProjectOverview follows the registered Build entry point to
// the overview and its mirror, using distinctive report and milestone inputs.
// The project route has one publisher alongside the shared docs publisher.
func TestBuildPublishesProjectOverview(t *testing.T) {
	root, output := projectOverviewBuildFixture(t)
	report, err := Build(t.Context(), BuildOptions{Repository: root, Output: output})
	if err != nil {
		t.Fatal(err)
	}
	if report.Coverage.Red() {
		t.Fatalf("project publication has incomplete or duplicate ownership: %+v", report.Coverage)
	}
	claims, err := readProducerRecord(Paths{Repository: root, Output: output})
	if err != nil {
		t.Fatal(err)
	}
	claimed := false
	for _, claim := range claims {
		if claim.Route != "/project/" {
			continue
		}
		claimed = true
		if len(claim.Producers) != 1 {
			t.Errorf("project route needs one publisher, got %v", claim.Producers)
		}
	}
	if !claimed {
		t.Error("Build did not record a publisher for /project/")
	}
	page, err := extractMain(readArtifact(t, output, "project/index.html"))
	if err != nil {
		t.Fatal(err)
	}
	projectOverviewPublishedContent(t, page)
	mirror := readArtifact(t, output, "project/index.md")
	readableMirror, _, err := renderMarkdown([]byte(mirror))
	if err != nil {
		t.Fatal(err)
	}
	projectOverviewPublishedContent(t, readableMirror)
}

// projectOverviewBuildFixture keeps the registered project and docs publishers,
// reducing only unrelated page populations and live daemon inputs. The real
// authored overview is read from main; the synthetic repository is input data.
func projectOverviewBuildFixture(t *testing.T) (string, string) {
	t.Helper()
	var chosen []Producer
	for _, producer := range registeredProducers {
		if producer.Name == "docs" || producer.Name == "project" {
			chosen = append(chosen, producer)
		}
	}
	if len(chosen) == 0 {
		t.Fatal("no registered publisher can serve the project overview")
	}
	stubProducers(t, chosen...)
	previousDocs, previousHubs := docsManifest, hubPages
	previousUseCases, previousLabs := useCasePages, labDetailPages
	previousComparisons, previousQuality := comparePages, qualityPages
	previousOneShots := oneShotPages
	t.Cleanup(func() {
		docsManifest, hubPages = previousDocs, previousHubs
		useCasePages, labDetailPages = previousUseCases, previousLabs
		comparePages, qualityPages = previousComparisons, previousQuality
		oneShotPages = previousOneShots
	})
	docsManifest = nil
	hubPages = nil
	for _, page := range previousHubs {
		if page.Dest == "project/index.html" {
			hubPages = append(hubPages, page)
		}
	}
	useCasePages, labDetailPages = nil, nil
	comparePages, qualityPages, oneShotPages = nil, nil, nil
	stubLiveInputs(t, `[{"path":"show test","short-help":"Show rows","mode":"read-only"}]`)
	root, output := siteFixture(t)
	source := filepath.Join(root, "website")
	writeFixtureFile(t, filepath.Join(source, "project", "index.md"),
		string(projectOverviewAuthoredSource(t)))
	writeFixtureFile(t, filepath.Join(source, topicsVocabularyFile),
		`{"tags":{"BGP":"routing"}}`)
	writeFixtureFile(t, filepath.Join(source, "data", "page-links.json"), `{}`)
	writeFixtureFile(t, filepath.Join(source, changesSourceDirectory, "2026-08-17.md"),
		strings.Replace(weekSource("2026-08-17", "BGP"), "The intro.",
			"An older report covers the previous router changes.", 1))
	writeFixtureFile(t, filepath.Join(source, changesSourceDirectory, "2026-08-24.md"),
		strings.Replace(weekSource("2026-08-24", "BGP"), "The intro.",
			"The newest report proves the router applies configuration while running.", 1))
	writeFixtureFile(t, filepath.Join(source, "data", "milestones.json"), `{
  "intro": "Recorded project landmarks.",
  "milestones": [
    {"date":"2026-08-24","title":"A recorded configuration landmark","category":"operate","blog":"2026-08-24","blurb":"Configuration changes while running."},
    {"date":"2026-07-20","title":"A recorded routing landmark","category":"routing","blurb":"Routing behavior arrives."},
    {"date":"2026-06-01","title":"A recorded interface landmark","category":"operate","blurb":"The operator interface arrives."}
  ]
}`)
	return root, output
}

// projectOverviewAuthoredSource reads only the authored template. The project
// fixtures supply their own data and must not regenerate the real checkout's
// unrelated derived indexes to obtain these unchanged source bytes.
func projectOverviewAuthoredSource(t *testing.T) []byte {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join(root, projectSource))
	if err != nil {
		t.Fatal(err)
	}
	return source
}

// projectOverviewPublishedContent checks reader-visible text and resolved links,
// allowing markup and relative URL spelling to vary between HTML and Markdown.
func projectOverviewPublishedContent(t *testing.T, content string) {
	t.Helper()
	words, links := readablePage(t, content)
	text := strings.Join(words, " ")
	for _, required := range []string{
		"Where Ze stands", "Latest development", "Before the first release",
		"See Ze running", "How we got here", "How Ze is built", "Get involved",
		"The newest report proves the router applies configuration while running.",
		"2026-08-24", "2026-08-30", "A recorded configuration landmark",
		"A recorded routing landmark", "A recorded interface landmark",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("published overview lacks reader-visible %q", required)
		}
	}
	base, err := url.Parse("https://ze-software.net/project/")
	if err != nil {
		t.Fatal(err)
	}
	var destinations []string
	for _, link := range links {
		target, err := url.Parse(link.href)
		if err != nil {
			t.Errorf("published link %q is invalid: %v", link.href, err)
			continue
		}
		resolved := base.ResolveReference(target)
		destination := resolved.Path
		if resolved.Fragment != "" {
			destination += "#" + resolved.Fragment
		}
		destinations = append(destinations, destination)
	}
	for _, required := range []string{
		"/project/roadmap/", "/project/milestones/", "/project/changes/",
		"/project/activity/", "/blog/", "/talks/", "/project/history/",
		"/project/why-ze/", "/project/ai-development/", "/contribute/",
		"/project/changes/2026-08-24/", "/quality/", "/#product-terminal",
		"/demos/terminal/#continuous-live-configuration-showcase", "/guides/quickstart/",
	} {
		if !slices.Contains(destinations, required) {
			t.Errorf("published overview has no link to %s", required)
		}
	}
}

// TestProjectOverviewSelectsNewestReport reads unsorted report files through
// the canonical loader, then proves that adding a newer one changes the teaser.
func TestProjectOverviewSelectsNewestReport(t *testing.T) {
	source := t.TempDir()
	writeArtifactFile(t, source, changesSourceDirectory+"/2026-08-24.md",
		strings.Replace(weekSource("2026-08-24", "BGP"), "The intro.", "The current report.", 1))
	writeArtifactFile(t, source, changesSourceDirectory+"/2026-08-17.md",
		strings.Replace(weekSource("2026-08-17", "BGP"), "The intro.", "The older report.", 1))
	for _, selection := range []struct {
		date, summary string
	}{
		{"2026-08-24", "The current report."},
		{"2026-08-31", "The next report."},
	} {
		if selection.date == "2026-08-31" {
			writeArtifactFile(t, source, changesSourceDirectory+"/2026-08-31.md",
				strings.Replace(weekSource("2026-08-31", "BGP"), "The intro.", selection.summary, 1))
		}
		weeks, err := loadChangeWeeks(source, testVocabulary)
		if err != nil {
			t.Fatal(err)
		}
		teaser, err := projectLatestUpdate(weeks)
		if err != nil {
			t.Fatal(err)
		}
		words, links := readablePage(t, teaser)
		if !strings.Contains(strings.Join(words, " "), selection.summary) {
			t.Fatalf("newest report %s did not supply the teaser: %s", selection.date, teaser)
		}
		if len(links) != 1 {
			t.Fatalf("teaser needs one report destination, got %v", links)
		}
		if want := "../project/changes/" + selection.date + "/"; links[0].href != want {
			t.Fatalf("teaser points to %q, want %q", links[0].href, want)
		}
		if strings.Contains(teaser, "The older report.") {
			t.Fatal("an older report displaced the newest report")
		}
	}
}

// TestProjectOverviewSelectsLandmarks checks short histories and the selection
// boundary while preserving source order for landmarks on the same date.
func TestProjectOverviewSelectsLandmarks(t *testing.T) {
	landmarks := []milestone{
		{Date: "2026-07-01", Title: "Oldest landmark", Category: categoryRouting},
		{Date: "2026-08-24", Title: "First equal-date landmark", Category: categoryOperate},
		{Date: "2026-08-24", Title: "Second equal-date landmark", Category: categoryOperate},
		{Date: "2026-08-01", Title: "Middle landmark", Category: categoryServices},
	}
	for count := 0; count <= len(landmarks); count++ {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			markup, err := projectLandmarkList(landmarks[:count])
			if err != nil {
				t.Fatal(err)
			}
			if got, want := strings.Count(markup, "<li>"), min(count, 3); got != want {
				t.Fatalf("landmark count %d: got %d entries, want %d: %s", count, got, want, markup)
			}
			if count == 0 {
				if !strings.Contains(markup, "No project milestones have been recorded yet.") {
					t.Fatal("empty history was not explained")
				}
				return
			}
			if count >= 2 {
				if !strings.Contains(markup, "First equal-date landmark") {
					t.Fatal("newest landmark was omitted")
				}
			}
			if count >= 3 {
				first := strings.Index(markup, "First equal-date landmark")
				second := strings.Index(markup, "Second equal-date landmark")
				if first >= second {
					t.Fatal("equal-date landmarks lost their source order")
				}
			}
			if count == 4 {
				if strings.Contains(markup, "Oldest landmark") {
					t.Fatal("oldest landmark displaced a newer landmark")
				}
				if !strings.Contains(markup, "Middle landmark") {
					t.Fatal("third newest landmark was omitted")
				}
			}
		})
	}
	t.Run("four within one month", func(t *testing.T) {
		markup, err := projectLandmarkList([]milestone{
			{Date: "2026-08-01", Title: "Earliest August landmark", Category: categoryRouting},
			{Date: "2026-08-28", Title: "Newest August landmark", Category: categoryOperate},
			{Date: "2026-08-15", Title: "Third August landmark", Category: categoryServices},
			{Date: "2026-08-20", Title: "Second August landmark", Category: categoryRouting},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Count(markup, "<li>"); got != 3 {
			t.Fatalf("one-month history rendered %d entries, want 3: %s", got, markup)
		}
		if strings.Contains(markup, "Earliest August landmark") {
			t.Fatal("fourth landmark displaced a newer same-month landmark")
		}
		position := -1
		for _, title := range []string{"Newest August landmark", "Second August landmark", "Third August landmark"} {
			next := strings.Index(markup, title)
			if next <= position {
				t.Fatalf("same-month landmark %q is missing or out of order: %s", title, markup)
			}
			position = next
		}
	})
}

// TestProjectOverviewEscapesGeneratedContent checks the raw output and its
// parsed text so hostile-looking report and milestone text stays readable text.
func TestProjectOverviewEscapesGeneratedContent(t *testing.T) {
	const hostile = `<img src=x onerror="alert(1)"> & "quoted" {project_unknown}`
	week := changeWeek{
		Slug: "2026-08-24", CoverageStart: "2026-08-24", CoverageEnd: "2026-08-30",
		Duration: "1 week", Intro: hostile,
	}
	landmark := milestone{Date: "2026-08-24", Title: hostile, Blurb: hostile, Category: categoryRouting}
	markup, err := projectOverviewContent(projectLatestSlot+"\n"+projectLandmarksSlot,
		[]changeWeek{week}, []milestone{landmark})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(markup, "<img") {
		t.Fatal("generated prose became an image element")
	}
	words, _ := readablePage(t, markup)
	if got := strings.Count(strings.Join(words, " "), hostile); got != 3 {
		t.Fatalf("escaped report/title/blurb did not preserve text three times, got %d: %s", got, markup)
	}
}

// TestProjectOverviewRejectsUnfilledSlots proves incomplete authored layouts
// cannot become plausible published pages while a complete one still renders.
func TestProjectOverviewRejectsUnfilledSlots(t *testing.T) {
	week := changeWeek{Slug: "2026-08-24", Intro: "A report summary."}
	for _, source := range []string{
		projectLatestSlot,
		projectLandmarksSlot,
		projectLatestSlot + projectLatestSlot + projectLandmarksSlot,
		projectLatestSlot + projectLandmarksSlot + "{project_unknown}",
	} {
		if _, err := projectOverviewContent(source, []changeWeek{week}, nil); err == nil {
			t.Errorf("accepted incomplete or unknown layout %q", source)
		}
	}
	if _, err := projectOverviewContent(projectLatestSlot+projectLandmarksSlot, []changeWeek{week}, nil); err != nil {
		t.Fatalf("complete overview was refused: %v", err)
	}
}

// TestProjectOverviewRejectsInvalidSources preserves loader errors and explains
// why missing summaries or landmark titles cannot produce a useful overview.
func TestProjectOverviewRejectsInvalidSources(t *testing.T) {
	if _, err := projectLatestUpdate(nil); err == nil {
		t.Error("accepted a missing latest report")
	}
	if _, err := projectLatestUpdate([]changeWeek{{Slug: "2026-08-24"}}); err == nil {
		t.Error("accepted a report with no summary")
	}
	for _, landmark := range []milestone{
		{Date: "invalid", Title: "A title", Category: categoryRouting},
		{Date: "2026-08-24", Title: "A title", Category: "unknown"},
		{Date: "2026-08-24", Category: categoryRouting},
	} {
		if _, err := projectLandmarkList([]milestone{landmark}); err == nil {
			t.Errorf("accepted invalid landmark %+v", landmark)
		}
	}
	markup, err := projectLatestUpdate([]changeWeek{{
		Slug: "2026-08-24", Intro: "The full report.", homeSummary: "The authored overview.",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(markup, "The authored overview.") {
		t.Fatal("the report's authored homepage summary was not used")
	}
}

// TestProjectOverviewRefusesInvalidComposition checks that validation errors
// from the generated sections stop composition instead of publishing blanks.
func TestProjectOverviewRefusesInvalidComposition(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		weeks     []changeWeek
		landmarks []milestone
	}{
		{name: "missing report"},
		{name: "empty summary", weeks: []changeWeek{{Slug: "2026-08-24"}}},
		{
			name:      "invalid landmark",
			weeks:     []changeWeek{{Slug: "2026-08-24", Intro: "A useful summary."}},
			landmarks: []milestone{{Date: "invalid", Title: "A landmark", Category: categoryRouting}},
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			content, err := projectOverviewContent(projectLatestSlot+projectLandmarksSlot, scenario.weeks, scenario.landmarks)
			if err == nil {
				t.Fatal("accepted an invalid generated section")
			}
			if content != "" {
				t.Errorf("invalid composition supplied publishable content: %q", content)
			}
		})
	}
}

// TestProjectOverviewRefusesInvalidCanonicalInputs checks the registered
// publisher's input boundaries without regenerating unrelated checkout data.
func TestProjectOverviewRefusesInvalidCanonicalInputs(t *testing.T) {
	authored := string(projectOverviewAuthoredSource(t))
	for _, scenario := range []struct {
		name, changedFile, replacement, diagnostic string
	}{
		{"missing authored source", "", "", "project/index.md"},
		{"malformed vocabulary", topicsVocabularyFile, "{", "topics.json"},
		{"invalid report", changesSourceDirectory + "/2026-08-24.md", "No themed headers.", "no themed section"},
		{"malformed landmarks", "data/" + milestonesDataFile, "{", "milestones.json"},
		{"incomplete authored source", "project/index.md", projectLatestSlot, projectLandmarksSlot},
		{"unknown authored slot", "project/index.md", authored + "\n{project_unknown}", "{project_unknown}"},
		{"duplicate authored slot", "project/index.md", strings.Replace(authored, projectLatestSlot, projectLatestSlot+projectLatestSlot, 1), "exactly one " + projectLatestSlot},
		{"malformed shared renderer inputs", "data/page-links.json", "{", "page-links.json"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			root := t.TempDir()
			paths := Paths{Repository: root, Source: filepath.Join(root, "website"), Output: t.TempDir()}
			if scenario.name != "missing authored source" {
				writeFixtureFile(t, filepath.Join(root, projectSource), authored)
			}
			writeFixtureFile(t, filepath.Join(paths.Source, topicsVocabularyFile), `{"tags":{"BGP":"routing"}}`)
			writeFixtureFile(t, filepath.Join(paths.Source, changesSourceDirectory, "2026-08-24.md"), weekSource("2026-08-24", "BGP"))
			writeFixtureFile(t, filepath.Join(paths.Source, "data", milestonesDataFile), `{"milestones":[]}`)
			if scenario.changedFile != "" {
				writeFixtureFile(t, filepath.Join(paths.Source, filepath.FromSlash(scenario.changedFile)), scenario.replacement)
			}
			routes, err := renderProject(paths)
			if err == nil {
				t.Fatal("published a project overview from invalid canonical input")
			}
			if !strings.Contains(err.Error(), scenario.diagnostic) {
				t.Errorf("refusal %q did not identify %q", err, scenario.diagnostic)
			}
			if len(routes) != 0 {
				t.Errorf("failed publication claimed routes: %v", routes)
			}
		})
	}
}

// TestProjectOverviewPreservesLiteralTextInPublishedMirror publishes hostile-
// looking text through the real project renderer, then parses its Markdown
// mirror again to prove that literal markup and entities stay visible text.
func TestProjectOverviewPreservesLiteralTextInPublishedMirror(t *testing.T) {
	root, output := projectOverviewBuildFixture(t)
	source := filepath.Join(root, "website")
	const hostile = `Literal <img src=x onerror="alert(1)"> &lt;literal&gt; & ordinary text.`
	writeFixtureFile(t, filepath.Join(source, changesSourceDirectory, "2026-08-24.md"),
		strings.Replace(weekSource("2026-08-24", "BGP"), "The intro.", hostile, 1))
	landmarks, err := json.Marshal(milestoneData{Intro: "Recorded landmarks.", Milestones: []milestone{{
		Date: "2026-08-24", Title: hostile, Blurb: hostile, Category: categoryOperate,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(source, "data", milestonesDataFile), string(landmarks))
	const command = "show bgp | match <peer>"
	pageSource := string(projectOverviewAuthoredSource(t)) +
		"\n<p>A command stays literal: <code>show bgp | match &lt;peer&gt;</code>.</p>\n"
	writeFixtureFile(t, filepath.Join(root, projectSource), pageSource)
	if _, err := Build(t.Context(), BuildOptions{Repository: root, Output: output}); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, output, projectDestination)
	mirror := readArtifact(t, output, "project/index.md")
	reparsed, _, err := renderMarkdown([]byte(mirror))
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"HTML": page, "reparsed Markdown mirror": reparsed} {
		t.Run(name, func(t *testing.T) {
			words, _ := readablePage(t, content)
			text := strings.Join(words, " ")
			if got := strings.Count(text, hostile); got != 3 {
				t.Errorf("report, title and blurb lost literal text: got %d occurrences, want 3; rendered text: %s", got, text)
			}
			if !strings.Contains(text, command) {
				t.Errorf("inline command text changed: want %q in %q", command, text)
			}
			document, err := xhtml.Parse(strings.NewReader(content))
			if err != nil {
				t.Fatal(err)
			}
			if projectOverviewHasEventImage(document) {
				t.Error("literal image text became an event-bearing image element")
			}
		})
	}
}

// projectOverviewHasEventImage inspects parsed elements so safe text containing
// the same words cannot be confused with a live image event handler.
func projectOverviewHasEventImage(node *xhtml.Node) bool {
	if node.Type == xhtml.ElementNode && node.Data == "img" {
		for _, attribute := range node.Attr {
			if attribute.Key == "onerror" {
				return true
			}
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if projectOverviewHasEventImage(child) {
			return true
		}
	}
	return false
}
