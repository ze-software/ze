// Design: website/AI.md -- a public URL that moved keeps answering at its old address
// Detail: docsmanifest.go states the destinations these routes moved from.
// Related: producer.go registers this as a derived producer, so it runs last.
package site

import (
	"encoding/json"
	"html"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The redirect producer is NOT registered, by owner decision on 2026-08-30:
// the site publishes no redirect stubs before the first release.
//
// The legacy-URL REWRITING still runs, and it matters more without the stubs
// rather than less. A stub catches a reader who follows an old address from
// outside; the rewrite fixes the old addresses inside our own pages, and with
// no stub behind them an unrewritten one reaches nothing at all.
//
// The code stays rather than being deleted, because the decision is about WHEN
// and not about whether: retiring a route without leaving a stub breaks every
// link to the old address, which is a cost worth paying only while nobody has
// linked one yet. Re-registering is the two lines below.
//
// When it comes back, it registers as a DERIVED producer, after every page
// producer: a stub replaces the index.html of a route and removes the Markdown
// mirror beside it, so a page producer running afterwards would write the page
// back.
//
//	func init() {
//		registerDerivedProducer(Producer{Name: "legacy-redirects", Render: renderLegacyRedirects})
//	}

// legacyRoute is one retired site-relative directory and the directory it moved
// to. Neither carries a leading or a trailing slash.
type legacyRoute struct {
	From string
	To   string
}

// legacyTable builds the redirect table in the order its entries were stated.
//
// The order is part of the contract: rewriteLegacyPublicURLs replaces each
// entry over the output of the entry before it, so a table that answered in a
// different order would rewrite a chained URL to a different target. A Go map
// iterates randomly, which is why the routes live in a slice and the map here
// only remembers where each one already sits.
//
// A repeated source keeps its first position and takes its last value, which is
// what assigning twice into a Python dict did.
type legacyTable struct {
	routes []legacyRoute
	at     map[string]int
}

// newLegacyTable answers an empty table.
func newLegacyTable() *legacyTable {
	return &legacyTable{at: make(map[string]int, 192)}
}

// set states that one retired route moved to another.
func (table *legacyTable) set(from, to string) {
	if index, stated := table.at[from]; stated {
		table.routes[index].To = to
		return
	}
	table.at[from] = len(table.routes)
	table.routes = append(table.routes, legacyRoute{From: from, To: to})
}

// legacyDocsDestinations are the five docs sources whose retired public URL was
// not docs/<stem>. Every other source moved from that address.
var legacyDocsDestinations = map[string]string{
	docsSourceContributeTesting: "contribute/testing",
	docsSourceDeprecatedOptions: "reference/deprecations",
	docsSourceGlossary:          "reference/glossary",
	docsSourceHistory:           "project/history",
	docsSourceRFCStatus:         "reference/rfcs",
}

// legacyDocsDestination answers the second address one docs source published at
// before the information architecture moved.
func legacyDocsDestination(docPath string) string {
	if destination, named := legacyDocsDestinations[docPath]; named {
		return destination
	}
	stem, _ := cutMarkdownSuffix(docPath)
	return "docs/" + stem
}

// useCasesDirectory is the public family the deployment examples publish under.
const useCasesDirectory = "use-cases"

// retiredUseCaseRoute answers the address one deployment example published at
// while the family was called usage.
func retiredUseCaseRoute(destination string) string {
	if destination == useCasesDirectory {
		return "usage"
	}
	return strings.Replace(destination, useCasesDirectory+"/", "usage/", 1)
}

// movedRoutes are the routes that moved for a reason no manifest states: a
// section that gained a parent, a page that changed family, a deck that left
// the presentations tree.
var movedRoutes = []legacyRoute{
	{From: "activity", To: "project/activity"},
	{From: changesProducer, To: changesDirectory},
	{From: "cli", To: "reference/cli"},
	{From: "command-equivalents", To: "reference/command-equivalents"},
	{From: "config-reference", To: "reference/configuration"},
	{From: dependenciesDirectory, To: "reference/" + dependenciesDirectory},
	{From: "docs/architecture/testing/l2tp-interop", To: "labs/l2tp-interop/architecture"},
	{From: "docs/architecture/testing/pppoe-interop", To: "labs/pppoe-interop/architecture"},
	{From: "docs/features/plugins", To: "reference/plugins"},
	{From: "docs/guide/exabgp-migration", To: "use-cases/exabgp-migration"},
	{From: "guides/configuration", To: "guides/configuration-model"},
	{From: "milestones", To: "project/milestones"},
	{From: "presentations/linx-2026-06", To: "talks/linx-2026-06"},
	{From: "presentations/netmcr-2026-04", To: "talks/netmcr-2026-04"},
	{From: "roadmap", To: "project/roadmap"},
	{From: "why-ze", To: "project/why-ze"},
}

// legacyFileRedirects are the two standalone decks that moved with their talk.
// They are files rather than directories, so each stub is written at the file's
// own address instead of at an index.html under it.
var legacyFileRedirects = []legacyRoute{
	{From: "presentations/linx-2026-06/index-inlined.html", To: "talks/linx-2026-06/index-inlined.html"},
	{From: "presentations/netmcr-2026-04/index-inlined.html", To: "talks/netmcr-2026-04/index-inlined.html"},
}

// legacyRoutes answers every retired directory this site still answers at, in
// the order the four sources of the table state them: the docs manifest first,
// in its own order, then the deployment examples, then one entry for each
// weekly changelog post, then the routes that moved on their own.
func legacyRoutes(source string) ([]legacyRoute, error) {
	table := newLegacyTable()
	for _, row := range docsManifest {
		destination, err := docsDestination(row.Source)
		if err != nil {
			return nil, err
		}
		stem, _ := cutMarkdownSuffix(row.Source)
		for _, retired := range []string{"docs/" + stem, legacyDocsDestination(row.Source)} {
			if retired != destination {
				table.set(retired, destination)
			}
		}
	}
	for _, page := range useCasePages {
		destination := strings.TrimSuffix(page.Dest, "/"+pageIndexFile)
		table.set(retiredUseCaseRoute(destination), destination)
	}
	slugs, err := changesPostSlugs(source)
	if err != nil {
		return nil, err
	}
	for _, slug := range slugs {
		table.set("changes/"+slug, changesDirectory+"/"+slug)
	}
	for _, moved := range movedRoutes {
		table.set(moved.From, moved.To)
	}
	return table.routes, nil
}

// changesPostSlugs answers the file stem of every weekly changelog post, in
// ascending order.
//
// The stem is the address the post published at while the changelog lived at
// the site root, so it is read from the file name rather than from the front
// matter: a post that later changed the week it covers still answers at the
// address a reader bookmarked.
func changesPostSlugs(source string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(source, filepath.FromSlash(changesSourceDirectory)))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	slugs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != markdownExtension {
			continue
		}
		slugs = append(slugs, strings.TrimSuffix(entry.Name(), markdownExtension))
	}
	sort.Strings(slugs)
	return slugs, nil
}

// renderLegacyRedirects publishes one stub for every retired address.
//
// A stub answers no route: pageRegistry drops a page carrying the noindex and
// refresh pair, so the coverage arithmetic never sees one. AC-12 of
// plan/spec-site-renderers-in-go.md is proven by redirect_test.go instead.
func renderLegacyRedirects(paths Paths) ([]string, error) { //nolint:unused,unparam // owner decision 2026-08-30, see the file header; the nil route slice is the Producer.Render signature answering "this producer publishes no route", stated three lines above
	routes, err := legacyRoutes(paths.Source)
	if err != nil {
		return nil, err
	}
	for _, route := range routes {
		stub := filepath.Join(paths.Output, filepath.FromSlash(route.From), pageIndexFile)
		if err := writeRedirectStub(stub, route.To); err != nil {
			return nil, err
		}
		// A retired route that once published a page left a Markdown mirror
		// beside it. The stub is not a page, so the mirror must go: the mirror
		// check counts a route by its index.html, and a stub with a mirror
		// would publish a body nothing links.
		if err := os.Remove(filepath.Join(filepath.Dir(stub), pageMirrorFile)); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	for _, route := range legacyFileRedirects {
		stub := filepath.Join(paths.Output, filepath.FromSlash(route.From))
		if err := writeRedirectStub(stub, route.To); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// writeRedirectStub writes one stub at an artifact path.
func writeRedirectStub(stub, target string) error { //nolint:unused // owner decision 2026-08-30, see the file header
	if err := os.MkdirAll(filepath.Dir(stub), 0o755); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return err
	}
	return os.WriteFile(stub, []byte(redirectStubHTML(target)), 0o644) //nolint:gosec // published web content: a web server, often another account, serves these bytes
}

// redirectStubHTML answers the page a retired address publishes.
//
// It says the same thing four ways, because each reader takes a different one:
// the robots tag stops a search engine indexing the stub, the refresh header
// moves a browser with no JavaScript, the canonical link tells a crawler which
// address counts, and the script moves a browser that has JavaScript while
// keeping the query string and the fragment the reader arrived with.
func redirectStubHTML(target string) string { //nolint:unused // owner decision 2026-08-30, see the file header
	trimmed := strings.Trim(target, "/")
	suffix := "/"
	if path.Ext(trimmed) != "" {
		suffix = ""
	}
	targetPath := "/" + trimmed + suffix
	targetURL := siteBase + trimmed + suffix
	escapedPath := html.EscapeString(targetPath)
	escapedURL := html.EscapeString(targetURL)
	// The script's argument is a JSON string, so a target holding a quote or a
	// backslash cannot end the literal early. json.Marshal answers one for any
	// string, and the error branch is unreachable for a string value.
	scriptTarget, err := json.Marshal(targetPath)
	if err != nil {
		panic("BUG: site.redirectStubHTML: a string has no JSON encoding: " + err.Error())
	}

	var stub textbuf.Buffer
	stub.Reset().Str("<!doctype html>\n<html lang=\"en\">\n<head>\n")
	stub.Str("    <meta charset=\"utf-8\">\n")
	stub.Str("    <meta name=\"robots\" content=\"noindex\">\n")
	stub.Str("    <meta http-equiv=\"refresh\" content=\"0; url=").Str(escapedPath).Str("\">\n")
	stub.Str("    <link rel=\"canonical\" href=\"").Str(escapedURL).Str("\">\n")
	stub.Str("    <title>Page moved - Ze</title>\n")
	stub.Str("    <script>location.replace(").Str(string(scriptTarget)).Str(" + location.search + location.hash);</script>\n")
	stub.Str("</head>\n<body>\n")
	stub.Str("    <p>This page moved to <a href=\"").Str(escapedURL).Str("\">").Str(escapedURL).Str("</a>.</p>\n")
	stub.Str("</body>\n</html>\n")
	return stub.String()
}

// rewriteLegacyPublicURLs answers one page's text with every retired absolute
// URL replaced by the address it moved to.
//
// The replacements run in the table's own order, each over the output of the
// one before it, so a route that moved twice reaches its current address in one
// pass. The order is therefore what decides the answer, and it is why the table
// is a slice.
func rewriteLegacyPublicURLs(text string, routes []legacyRoute) string {
	for _, route := range routes {
		text = strings.ReplaceAll(text, siteBase+route.From+"/", siteBase+route.To+"/")
	}
	for _, route := range legacyFileRedirects {
		text = strings.ReplaceAll(text, siteBase+route.From, siteBase+route.To)
	}
	return text
}

// rewriteArtifactLegacyURLs replaces every retired absolute URL the artifact
// carries with the address it moved to, and answers how many files changed.
//
// Five of them sit in docs/history.md today. A published page that links
// https://ze-software.net/changes/ sends its reader through a redirect stub to
// reach the page this same build wrote at project/changes/, and a machine
// reader following the Markdown mirror gets the retired address with no stub to
// follow at all. The retired build ran this pass over the finished artifact
// (website/tools/build.py, step_links) and the Go port left the function it
// needs unreached.
//
// It runs BETWEEN the page producers and the producers that read what they
// wrote, so the search index and llms-full.txt carry the current addresses
// rather than the retired ones. renderProducers states that order.
//
// A frozen talk deck is skipped, as it is by every other pass: a deck is
// published exactly as its author wrote it.
func rewriteArtifactLegacyURLs(paths Paths) (int, error) {
	routes, err := legacyRoutes(paths.Source)
	if err != nil {
		return 0, err
	}
	rewritten := 0
	err = filepath.WalkDir(paths.Output, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, relErr := filepath.Rel(paths.Output, path)
		if relErr != nil {
			return relErr
		}
		name := filepath.ToSlash(relative)
		if entry.IsDir() {
			if name == gitMetadataDir {
				return filepath.SkipDir
			}
			return nil
		}
		if !rewritableArtifactFile(entry.Name()) || isFrozenTalkPath(name) {
			return nil
		}
		content, readErr := os.ReadFile(path) //nolint:gosec // the artifact this build just wrote
		if readErr != nil {
			return readErr
		}
		updated := rewriteLegacyPublicURLs(string(content), routes)
		if updated == string(content) {
			return nil
		}
		if writeErr := os.WriteFile(path, []byte(updated), 0o644); writeErr != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
			return writeErr
		}
		rewritten++
		return nil
	})
	if err != nil {
		return 0, err
	}
	return rewritten, nil
}

// rewritableArtifactFile reports whether one artifact file carries links a
// reader follows: a published page, or the Markdown mirror beside it.
func rewritableArtifactFile(name string) bool {
	return strings.HasSuffix(name, ".html") || name == pageMirrorFile
}
