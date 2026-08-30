// Design: website/AI.md -- llms-full.txt is the whole site as one document
package site

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	sitewiki "github.com/ze-software/ze/internal/le/site/wiki"
)

// llmsFullPage is one page of the artifact a llms-full.txt test builds.
type llmsFullPage struct {
	route string
	title string
	body  string
}

// llmsFullArtifact lays out an artifact holding one page for each declared
// section, plus a frozen talk deck.
//
// It is small on purpose: the refusals below are about which section claims a
// page, and one page for each section exercises every one of them.
// TestEveryPublishedRouteBelongsToOneSection runs the same arithmetic over the
// whole published population.
var llmsFullArtifact = []llmsFullPage{
	{route: "/", title: "Ze", body: "Ze is a network operating system."},
	{route: "/guides/quickstart/", title: "Quickstart", body: "Install and peer in ten minutes."},
	{route: "/features/", title: "Features", body: "Every shipped feature, by category."},
	{route: "/docs/", title: "Documentation", body: "The documentation hub."},
	{route: "/use-cases/", title: "Use cases", body: "Deployments Ze is built for."},
	{route: "/reference/", title: "Reference", body: "The generated references."},
	{route: "/reference/cli/bgp/", title: "show bgp", body: "Show the BGP state."},
	{route: "/project/", title: "Project", body: "How Ze is built and by whom."},
	{route: "/license/", title: "License", body: "Ze is AGPLv3."},
}

// llmsFullPaths answers an artifact carrying llmsFullArtifact, with the
// repository's own nav.json as the source of section membership.
func llmsFullPaths(t *testing.T) Paths {
	t.Helper()
	root := repositoryRoot(t)
	output := t.TempDir()
	source := t.TempDir()
	copyFixture(t, filepath.Join(root, "website", "data", "nav.json"),
		filepath.Join(source, "data", "nav.json"))
	for _, page := range llmsFullArtifact {
		seedPublishedPage(t, output, page)
	}
	// A frozen talk deck: an index.html with no mirror beside it, which is what
	// every other mirror pass skips and what this one must skip too.
	writeArtifactFile(t, output, "talks/demo/index.html", "<!doctype html><html><body>deck</body></html>")
	writeArtifactFile(t, output, "talks/index.html", "<!doctype html><html><body>talks</body></html>")
	writeArtifactFile(t, output, "talks/index.md", "# Talks\n\nEvery deck.\n")
	return Paths{Repository: root, Source: source, Output: output}
}

// seedPublishedPage puts one page and its Markdown mirror into a test artifact.
func seedPublishedPage(t *testing.T, output string, page llmsFullPage) {
	t.Helper()
	directory := strings.Trim(page.route, "/")
	writeArtifactFile(t, output, filepath.ToSlash(filepath.Join(directory, pageIndexFile)),
		"<!doctype html><html><body>"+page.title+"</body></html>")
	writeArtifactFile(t, output, filepath.ToSlash(filepath.Join(directory, pageMirrorFile)),
		"# "+page.title+"\n\n"+page.body+"\n")
}

// VALIDATES: AC-15 -- llms-full.txt carries the full Markdown mirror of every
// published page, each preceded by its title and its canonical URL, with the
// frozen talk decks left out.
func TestLLMSFullCarriesEveryPublishedMirror(t *testing.T) {
	paths := llmsFullPaths(t)

	routes, err := renderLLMSFull(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Errorf("the llms-full producer claimed %v; llms-full.txt is a published file, not a route", routes)
	}
	content := readArtifact(t, paths.Output, llmsFullFile)
	for _, page := range llmsFullArtifact {
		want := llmsFullPagePrefix + page.title + "\n" + siteBase + strings.TrimPrefix(page.route, "/") + "\n"
		if !strings.Contains(content, want) {
			t.Errorf("%s is not preceded by its title and canonical URL:\nwant %q", page.route, want)
		}
		if !strings.Contains(content, page.body) {
			t.Errorf("the body of %s is missing", page.route)
		}
	}
	// The talks index is a page like any other and carries a mirror. The deck
	// under it is frozen, publishes no mirror, and must not be reached for one.
	if !strings.Contains(content, siteBase+"talks/\n") {
		t.Error("the talks index is missing; it is a page, not a deck")
	}
	if strings.Contains(content, siteBase+"talks/demo/") {
		t.Error("a frozen talk deck reached llms-full.txt")
	}
}

// VALIDATES: AC-15a and AC-15d -- the reading order puts what Ze is and why it
// is worth evaluating before how to use it.
//
// The method reads the section headings back out of the published file in the
// order they appear, and requires the declared order and the evaluation-first
// property of it. A file that concatenated the sections in route order, or in
// whatever order a map iterated, fails both.
func TestLLMSFullPutsEvaluationBeforeUsage(t *testing.T) {
	paths := llmsFullPaths(t)
	if _, err := renderLLMSFull(paths); err != nil {
		t.Fatal(err)
	}
	content := readArtifact(t, paths.Output, llmsFullFile)

	var headings []string
	for line := range strings.SplitSeq(content, "\n") {
		if heading, cut := strings.CutPrefix(line, llmsFullSectionPrefix); cut {
			headings = append(headings, heading)
		}
	}
	want := make([]string, 0, len(llmsFullReadingOrder)+1)
	for _, section := range llmsFullReadingOrder {
		want = append(want, section.title())
	}
	want = append(want, "Wiki")
	if len(headings) != len(want) {
		t.Fatalf("llms-full.txt carries %v and the reading order states %v", headings, want)
	}
	for position := range want {
		if headings[position] != want[position] {
			t.Errorf("section %d is %q and the reading order states %q", position, headings[position], want[position])
		}
	}

	usage := ""
	for _, section := range llmsFullReadingOrder {
		if section.Kind == readingUsage && usage == "" {
			usage = section.title()
		}
		if section.Kind == readingEvaluation && usage != "" {
			t.Errorf("the evaluation section %q comes after the usage section %q", section.title(), usage)
		}
	}
	if usage == "" {
		t.Error("no section is a usage section, so the order proves nothing")
	}
}

// VALIDATES: AC-15b -- a page that belongs to no section is refused by name,
// rather than appended at the end because nothing claimed it.
func TestLLMSFullRefusesAnUnsectionedPage(t *testing.T) {
	paths := llmsFullPaths(t)
	seedPublishedPage(t, paths.Output, llmsFullPage{
		route: "/unfiled/", title: "Unfiled", body: "A page no menu carries.",
	})

	_, err := renderLLMSFull(paths)
	if err == nil {
		t.Fatal("a page belonging to no section was accepted; it would land at the end of the file")
	}
	if !strings.Contains(err.Error(), "/unfiled/") {
		t.Errorf("the refusal does not name the page: %v", err)
	}
}

// VALIDATES: AC-15b's other half -- a page two sections claim is refused by
// name, rather than published twice.
//
// The method puts a nav entry for a route the reading order already names in
// "About this site". Both claims match the whole route, so neither is more
// specific and nothing in either file says which section should win.
func TestLLMSFullRefusesAPageTwoSectionsClaim(t *testing.T) {
	paths := llmsFullPaths(t)
	nav := loadNavFixture(t, paths.Source)
	for index := range nav.Dropdowns {
		if nav.Dropdowns[index].Label != "Docs" {
			continue
		}
		nav.Dropdowns[index].Columns[0] = append(nav.Dropdowns[index].Columns[0],
			navEntry{Href: "license/", Title: "License", Desc: "the license"})
	}
	writeNavFixture(t, paths.Source, nav)

	_, err := renderLLMSFull(paths)
	if err == nil {
		t.Fatal("a page two sections claim was accepted; it would be published twice")
	}
	if !strings.Contains(err.Error(), "/license/") ||
		!strings.Contains(err.Error(), "Docs") || !strings.Contains(err.Error(), "About this site") {
		t.Errorf("the refusal does not name the page and both sections: %v", err)
	}
}

// VALIDATES: AC-15c -- a section the reading order declares and no page fills is
// refused by name, so a section that silently empties is a red rather than a
// gap a reader meets.
func TestLLMSFullRefusesAnEmptySection(t *testing.T) {
	paths := llmsFullPaths(t)
	if err := os.RemoveAll(filepath.Join(paths.Output, "license")); err != nil {
		t.Fatal(err)
	}

	_, err := renderLLMSFull(paths)
	if err == nil {
		t.Fatal("a section no page fills was accepted; it would publish a heading over nothing")
	}
	if !strings.Contains(err.Error(), "About this site") {
		t.Errorf("the refusal does not name the section: %v", err)
	}
}

// VALIDATES: AC-15d -- nav.json reordered so a usage section precedes an
// evaluation section is refused, naming both sections.
//
// The reading order owns the document, so the reshuffle changes no byte of
// llms-full.txt. That is why it must be reported: the menu and the document
// would argue different orders with nothing to say so.
func TestLLMSFullRefusesANavOrderThatPutsUsageFirst(t *testing.T) {
	paths := llmsFullPaths(t)
	nav := loadNavFixture(t, paths.Source)
	// Docs is a usage section and Evaluate is an evaluation one. Moving Docs to
	// the front is the reshuffle a menu edit would make.
	reordered := []navDropdown{}
	for _, dropdown := range nav.Dropdowns {
		if dropdown.Label == "Docs" {
			reordered = append([]navDropdown{dropdown}, reordered...)
			continue
		}
		reordered = append(reordered, dropdown)
	}
	nav.Dropdowns = reordered
	writeNavFixture(t, paths.Source, nav)

	_, err := renderLLMSFull(paths)
	if err == nil {
		t.Fatal("a menu that runs usage before evaluation was accepted")
	}
	if !strings.Contains(err.Error(), "Docs") || !strings.Contains(err.Error(), "Start") {
		t.Errorf("the refusal does not name both sections: %v", err)
	}
}

// VALIDATES: AC-15d's other half -- a nav dropdown the reading order declares no
// section for is refused, because the check cannot place a dropdown with no kind.
func TestLLMSFullRefusesANavDropdownTheReadingOrderDoesNotDeclare(t *testing.T) {
	paths := llmsFullPaths(t)
	nav := loadNavFixture(t, paths.Source)
	nav.Dropdowns = append(nav.Dropdowns, navDropdown{Label: "Partners"})
	writeNavFixture(t, paths.Source, nav)

	_, err := renderLLMSFull(paths)
	if err == nil {
		t.Fatal("a dropdown the reading order does not declare was accepted")
	}
	if !strings.Contains(err.Error(), "Partners") {
		t.Errorf("the refusal does not name the dropdown: %v", err)
	}
}

// VALIDATES: AC-15b over the whole published population -- every one of the 712
// routes the site publishes belongs to exactly one section.
//
// The small artifact above proves the arithmetic. This proves the TABLE: the
// reading order plus the repository's own nav.json really do claim every
// published page once, so llms-full.txt is buildable against the real site and
// not only against a fixture.
func TestEveryPublishedRouteBelongsToOneSection(t *testing.T) {
	root := repositoryRoot(t)
	var nav siteNav
	if err := readSourceJSON(filepath.Join(root, "website"), navDataFile, &nav); err != nil {
		t.Fatal(err)
	}
	if err := checkNavReadingOrder(nav); err != nil {
		t.Fatal(err)
	}
	routes := publishedArtifactRoutes(t)
	pages := make([]Page, 0, len(routes))
	for _, route := range routes {
		directory := strings.Trim(route, "/")
		pages = append(pages, Page{
			Route:    route,
			HTML:     filepath.ToSlash(filepath.Join(directory, pageIndexFile)),
			Markdown: filepath.ToSlash(filepath.Join(directory, pageMirrorFile)),
		})
	}
	sections, err := assignPages(pages, nav)
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for position, section := range llmsFullReadingOrder {
		total += len(sections[position])
		t.Logf("%-16s %s %4d pages", section.title(), section.Kind, len(sections[position]))
	}
	frozen := 0
	for _, page := range pages {
		if isFrozenTalkPath(page.HTML) {
			frozen++
		}
	}
	if total+frozen != len(pages) {
		t.Errorf("the sections hold %d pages and %d are frozen decks; the site publishes %d",
			total, frozen, len(pages))
	}
}

// VALIDATES: AC-17 and AC-17b -- the wiki section references the wiki, in the
// wiki's own order, and republishes no page body.
func TestTheWikiSectionReferencesTheWikiRatherThanRepublishingIt(t *testing.T) {
	paths := llmsFullPaths(t)
	if _, err := renderLLMSFull(paths); err != nil {
		t.Fatal(err)
	}
	content := readArtifact(t, paths.Output, llmsFullFile)
	opens := strings.Index(content, "\n"+llmsFullSectionPrefix+"Wiki\n")
	if opens < 0 {
		t.Fatal("llms-full.txt carries no wiki section")
	}
	section := content[opens:]

	// AC-17b: the groups are the sidebar's own, in the sidebar's own order.
	want := []string{"About", "First Steps", "Configuration", "Operation"}
	position := 0
	for _, group := range want {
		found := strings.Index(section, "\n### "+group+"\n")
		if found < position {
			t.Errorf("the wiki group %q is missing or out of the sidebar's order", group)
			continue
		}
		position = found
	}
	// AC-17: each page is a title, a public URL and a summary, and no body.
	if !strings.Contains(section, "](https://") {
		t.Error("the wiki section carries no public URLs")
	}
	if strings.Contains(section, "\n# ") {
		t.Error("the wiki section carries a page body; it must reference the wiki, not republish it")
	}
}

// loadNavFixture reads the nav.json one test artifact was seeded with.
func loadNavFixture(t *testing.T, source string) siteNav {
	t.Helper()
	var nav siteNav
	if err := readSourceJSON(source, navDataFile, &nav); err != nil {
		t.Fatal(err)
	}
	return nav
}

// writeNavFixture puts an edited menu back where the producer reads it.
func writeNavFixture(t *testing.T, source string, nav siteNav) {
	t.Helper()
	content, err := json.MarshalIndent(nav, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "data", "nav.json"), content, 0o644); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: the reading-order table itself -- every section names a kind and
// exactly one source of membership.
//
// A section with no kind cannot be placed by the evaluation-before-usage check,
// and a section naming both a dropdown and a route list would claim pages twice.
func TestEveryDeclaredSectionNamesAKindAndOneMembershipSource(t *testing.T) {
	titles := make(map[string]bool, len(llmsFullReadingOrder))
	for _, section := range llmsFullReadingOrder {
		if section.Kind == readingUnspecified {
			t.Errorf("the section %q names no kind", section.title())
		}
		if (section.Nav == "") == (len(section.Routes) == 0) {
			t.Errorf("the section %q must name a nav dropdown or a route list, and exactly one", section.title())
		}
		if titles[section.title()] {
			t.Errorf("two sections are titled %q", section.title())
		}
		titles[section.title()] = true
		routes := append([]string(nil), section.Routes...)
		sort.Strings(routes)
		for position, route := range routes {
			if !strings.HasPrefix(route, "/") || !strings.HasSuffix(route, "/") {
				t.Errorf("the route %q is not spelled as pageRegistry spells one", route)
			}
			if position > 0 && routes[position-1] == route {
				t.Errorf("the section %q names %q twice", section.title(), route)
			}
		}
	}
}

// VALIDATES: AC-17a -- the build reads the committed website/data/wiki.json and
// never a wiki checkout, so a machine without the sibling directory writes the
// same artifact.
//
// The method points the build at a checkout that holds the committed index and
// nothing else, with no wiki beside it, and requires the wiki section to carry
// every group and every page the committed file states. A build that opened
// ../wiki would answer with an error or with a shorter section.
func TestTheBuildReadsTheCommittedWikiIndexAndNeverTheCheckout(t *testing.T) {
	source := repositoryRoot(t)
	committed, err := sitewiki.Read(source)
	if err != nil {
		t.Fatal(err)
	}
	standalone := filepath.Join(t.TempDir(), "main")
	copyFixture(t, filepath.Join(source, "website", "data", "wiki.json"),
		filepath.Join(standalone, "website", "data", "wiki.json"))
	if _, err := os.Stat(filepath.Join(filepath.Dir(standalone), "wiki")); !os.IsNotExist(err) {
		t.Fatalf("the fixture has a wiki checkout beside it, so it proves nothing: %v", err)
	}

	paths := llmsFullPaths(t)
	paths.Repository = standalone
	if _, err := renderLLMSFull(paths); err != nil {
		t.Fatal(err)
	}
	content := readArtifact(t, paths.Output, llmsFullFile)
	for _, group := range committed.Groups {
		if !strings.Contains(content, "\n### "+group.Title+"\n") {
			t.Errorf("the wiki group %q is missing from a build with no wiki checkout", group.Title)
		}
		for _, page := range group.Pages {
			if !strings.Contains(content, committed.URL(page.Slug)+")") {
				t.Errorf("the wiki page %q is missing from a build with no wiki checkout", page.Slug)
			}
		}
	}
}
