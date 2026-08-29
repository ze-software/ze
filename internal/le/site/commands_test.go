// Design: website/AI.md -- the CLI reference is one published page of the live catalog
package site

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"
)

// commandSurfacePaths lays out one artifact carrying the two data files every
// command surface reads.
//
// Both are cut from what gh-pages HEAD 2fa8fa2ad published, and cut together:
// nine commands that between them exercise every branch a row can take, and the
// curated intents whose Ze commands are all inside those nine. Cutting one
// without the other would make the fixture state a curated command the catalog
// does not have, which the renderer refuses by design.
func commandSurfacePaths(t *testing.T) Paths {
	t.Helper()
	root := repositoryRoot(t)
	output := t.TempDir()
	if err := os.MkdirAll(filepath.Join(output, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	copyFixture(t, filepath.Join("testdata", "published-cli-commands.json"),
		filepath.Join(output, filepath.FromSlash(catalogFile)))
	copyFixture(t, filepath.Join("testdata", "published-command-equivalents.json"),
		filepath.Join(output, filepath.FromSlash(equivalentsFile)))
	return Paths{Repository: root, Source: filepath.Join(root, "website"), Output: output}
}

// copyFixture puts one file where a renderer expects to read it.
func copyFixture(t *testing.T, source, target string) {
	t.Helper()
	content, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

// visibleText answers the words a reader sees in one HTML fragment, with the
// runs of whitespace collapsed. It is what "reads the same" means: two pages
// that differ in escaping, indentation or attribute quoting answer one string.
func visibleText(fragment string) string {
	var text strings.Builder
	tokenizer := xhtml.NewTokenizer(strings.NewReader(fragment))
	skip := 0
	for {
		switch tokenizer.Next() {
		case xhtml.ErrorToken:
			return strings.Join(strings.Fields(text.String()), " ")
		case xhtml.StartTagToken:
			name, _ := tokenizer.TagName()
			if string(name) == "script" || string(name) == "style" {
				skip++
			}
		case xhtml.EndTagToken:
			name, _ := tokenizer.TagName()
			if string(name) == "script" || string(name) == "style" {
				skip--
			}
		case xhtml.TextToken:
			if skip == 0 {
				text.Write(tokenizer.Text())
			}
		default:
			// A self-closing tag, a comment and a doctype each carry no text a
			// reader sees, so they change nothing here.
		}
	}
}

// VALIDATES: the published CLI reference carries the whole site shell.
//
// Commit 9f45348a7 published docvalid's contract fixture over this page: a bare
// "<!doctype html><html><body>" with no head, no title, no stylesheet, no
// header, no sidebar and no footer, cutting it from about 10KB to 481 bytes.
// The method is the one a reader would use: render the page and look for each
// piece of chrome the published page carries.
func TestTheCLIReferencePageCarriesTheFullSiteShell(t *testing.T) {
	paths := commandSurfacePaths(t)

	routes, err := renderCLIReference(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 || routes[0] != "/reference/cli/" {
		t.Fatalf("the producer claimed %v, want [/reference/cli/]", routes)
	}
	page := readArtifact(t, paths.Output, cliReferenceDest)
	for _, chrome := range []string{
		`<html lang="en">`,
		"<title>CLI Reference - Ze</title>",
		`<meta name="description" content="Every ze command`,
		`<link rel="canonical" href="https://ze-software.net/reference/cli/" />`,
		`<link rel="stylesheet" href="../../assets/site.css" />`,
		`<div id="site-header-mount" data-header-src="../../assets/header.html"`,
		`<main id="top" class="has-page-sidebar" tabindex="-1">`,
		`<aside class="page-sidebar"`,
		`<script src="../../assets/site.js" defer></script>`,
		"<footer>",
	} {
		if !strings.Contains(page, chrome) {
			t.Errorf("the published CLI reference is missing %q", chrome)
		}
	}
	if _, err := os.Stat(filepath.Join(paths.Output, "reference", "cli", pageMirrorFile)); err != nil {
		t.Errorf("the page carries no Markdown mirror: %v", err)
	}
}

// publishedRow matches one command row of the published page, and publishedCell
// one of its four cells, so a test can pair a rendered row with the published
// one cell by cell.
var (
	publishedRow  = regexp.MustCompile(`(?s)<tr id="cmd-([a-z0-9-]+)">.*?</tr>`)
	publishedCell = regexp.MustCompile(`(?s)<td[^>]*>(.*?)</td>`)
)

// rowCells answers the visible text of each cell of one command row.
func rowCells(row string) []string {
	matches := publishedCell.FindAllStringSubmatch(row, -1)
	cells := make([]string, 0, len(matches))
	for _, match := range matches {
		cells = append(cells, visibleText(match[1]))
	}
	return cells
}

// VALIDATES: each command row reads as the row the published page carries.
//
// The comparison is on VISIBLE TEXT, which is what the owner set as the parity
// target on 2026-08-29: character-reference spelling, whitespace and attribute
// quoting may differ, what a reader sees may not. The fixture pairs the
// published catalog with the published rows, both taken from gh-pages HEAD
// 2fa8fa2ad, so neither side of the comparison is authored here.
//
// The FIRST cell is compared against the command path rather than against the
// published cell, and that is the one deliberate difference on this page. The
// published cell held the retired `syntax` value where the scraper had produced
// one, so 80 rows disagreed with the `id="cmd-<slug-of-path>"` they were
// anchored on. The row now opens with the path it is anchored on, and the
// invocation form is published beside the description from the command model's
// own `usage`. The other three cells must match exactly.
func TestACommandRowReadsAsThePublishedRow(t *testing.T) {
	paths := commandSurfacePaths(t)
	if _, err := renderCLIReference(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, cliReferenceDest)
	published := readFixture(t, "published-cli-rows.html")

	rendered := make(map[string][]string)
	for _, match := range publishedRow.FindAllStringSubmatch(page, -1) {
		rendered[match[1]] = rowCells(match[0])
	}
	rows := publishedRow.FindAllStringSubmatch(published, -1)
	if len(rows) == 0 {
		t.Fatal("the published fixture holds no command row")
	}
	for _, match := range rows {
		slug := match[1]
		want := rowCells(match[0])
		got, found := rendered[slug]
		if !found {
			t.Errorf("cmd-%s has no row on the rendered page", slug)
			continue
		}
		if len(got) != 4 || len(want) != 4 {
			t.Errorf("cmd-%s has %d rendered cells and %d published ones, want 4 of each",
				slug, len(got), len(want))
			continue
		}
		if commandSlug(got[0]) != slug {
			t.Errorf("cmd-%s opens with %q, want the registry path it is anchored on", slug, got[0])
		}
		for cell := 1; cell < 4; cell++ {
			if got[cell] != want[cell] {
				t.Errorf("cmd-%s cell %d reads as\n  %q\nthe published cell reads as\n  %q",
					slug, cell+1, got[cell], want[cell])
			}
		}
	}
}

// VALIDATES: the shared operator table reads as the published one.
//
// The table is the page's contract for the pipe operators: its Available column
// is the union across every command, so `save` reads "Always, Local process
// only" once rather than contradicting itself between rows.
func TestTheOperatorGuideReadsAsThePublishedGuide(t *testing.T) {
	paths := commandSurfacePaths(t)
	commands, err := loadCommandCatalog(paths.Output)
	if err != nil {
		t.Fatal(err)
	}

	got := visibleText(renderOperatorGuide(commands))
	want := visibleText(readFixture(t, "published-cli-operator-guide.html"))
	if got != want {
		t.Errorf("the operator guide reads as\n  %q\nthe published guide reads as\n  %q", got, want)
	}
}

// VALIDATES: a command's usage line is published, and the retired `syntax`
// field is not.
//
// `syntax` was scraped out of each description's "Usage:" sentence and cut at
// the first ". ", so several published values were unbalanced, such as
// `show metrics name <name> [label=value`. The command model states `usage`
// instead, and this test pins that the renderer reads the second and not the
// first.
func TestTheCatalogPublishesUsageAndNotTheScrapedSyntax(t *testing.T) {
	paths := commandSurfacePaths(t)
	writeCatalog(t, paths.Output, `[{"path":"show test","mode":"read-only",
		"description":"Show the rows of the test table.",
		"usage":"show test [name <name>]","syntax":"show test [name <name>"}]`)

	if _, err := renderCLIReference(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, cliReferenceDest)
	if !strings.Contains(page, "<code>show test [name &lt;name&gt;]</code>") {
		t.Error("the page does not publish the command model's own usage line")
	}
	if strings.Contains(page, "show test [name &lt;name&gt;<") {
		t.Error("the page published the retired scraped syntax value, cut before its bracket closed")
	}
	if !strings.Contains(page, `<tr id="cmd-show-test"><td><code>show test</code></td>`) {
		t.Error("the row does not open with the registry path it is anchored on")
	}
}

// VALIDATES: an operator whose availability nothing names stops the build.
//
// The qualifier is part of the published contract: a reader who cannot see that
// `match` needs rows will pipe into it and get an error. A catalog that states
// an availability this site has no word for would publish the operator with no
// qualifier at all, which reads as "always".
func TestAnUnknownOperatorAvailabilityStopsTheBuild(t *testing.T) {
	paths := commandSurfacePaths(t)
	writeCatalog(t, paths.Output, `[{"path":"show test","mode":"read-only","operators":
		[{"name":"json","class":"global","available":"sometimes","description":"JSON"}]}]`)

	_, err := renderCLIReference(paths)
	if err == nil {
		t.Fatal("an unknown operator availability was published rather than refused")
	}
	if !strings.Contains(err.Error(), "sometimes") || !strings.Contains(err.Error(), "json") {
		t.Errorf("the refusal names neither the operator nor its availability: %v", err)
	}
}

// VALIDATES: a verb with more commands than one table can hold is split by
// subject, and its long tail shares one catch-all.
//
// `show` alone holds 254 commands across 67 subjects. Splitting every subject
// gives dozens of one-row tables and splitting none gives a table nobody can
// scan, so the rule is a threshold in each direction.
func TestAnOversizedVerbSplitsBySubjectWithACatchAll(t *testing.T) {
	commands := make([]catalogCommand, 0, 32)
	for _, path := range []string{"show bgp one", "show bgp two", "show bgp three", "show bgp four"} {
		commands = append(commands, catalogCommand{Path: path, Mode: "read-only"})
	}
	for index := range commandGroupMax {
		commands = append(commands, catalogCommand{Path: "show tail" + string(rune('a'+index)), Mode: "read-only"})
	}

	groups := groupCommands(commands)
	labels := make([]string, 0, len(groups))
	for _, group := range groups {
		labels = append(labels, group.Label)
	}
	if len(groups) != 2 || labels[0] != "show bgp" || labels[1] != "show (other)" {
		t.Fatalf("groups are %v, want [show bgp show (other)]", labels)
	}
	if len(groups[0].Commands) != 4 || len(groups[0].Commands) >= len(groups[1].Commands) {
		t.Errorf("the frequent subject holds %d commands and the tail %d",
			len(groups[0].Commands), len(groups[1].Commands))
	}
}

// writeCatalog replaces the published command catalog of one artifact.
func writeCatalog(t *testing.T, output, catalog string) {
	t.Helper()
	path := filepath.Join(output, filepath.FromSlash(catalogFile))
	if err := os.WriteFile(path, []byte(catalog), 0o644); err != nil {
		t.Fatal(err)
	}
}

// readFixture answers one golden file taken from the published site.
func readFixture(t *testing.T, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
