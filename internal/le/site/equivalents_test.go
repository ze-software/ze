// Design: website/AI.md -- one page per live command, mapping it onto the vendor CLIs
package site

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// VALIDATES: a command-equivalent detail page reads as the published one and
// carries the whole site shell.
//
// The published pair is `show bgp` at gh-pages HEAD 2fa8fa2ad: the command with
// the most curated vendor equivalents, pipe aliases and address fields, so the
// comparison exercises every card the page can carry. Commit 9f45348a7
// published a 481-byte fragment over each of these 396 pages.
func TestACommandEquivalentPageReadsAsThePublishedPage(t *testing.T) {
	paths := commandSurfacePaths(t)

	routes, err := renderCommandEquivalents(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 10 {
		t.Fatalf("the producer claimed %d routes, want one index and nine detail pages", len(routes))
	}
	page := readArtifact(t, paths.Output, equivalentsDirectory+"/show-bgp/"+pageIndexFile)
	for _, chrome := range []string{
		"<title>show bgp - Command Equivalents - Ze</title>",
		`<link rel="canonical" href="https://ze-software.net/reference/command-equivalents/show-bgp/" />`,
		`<link rel="stylesheet" href="../../../assets/site.css" />`,
		`<div id="site-header-mount" data-header-src="../../../assets/header.html"`,
		`<main id="top" class="has-page-sidebar" tabindex="-1">`,
		"<footer>",
	} {
		if !strings.Contains(page, chrome) {
			t.Errorf("the published detail page is missing %q", chrome)
		}
	}

	// One row is removed from the published page before comparing, and it is
	// the phase's one deliberate deviation on this surface. The published
	// Syntax row carried the retired `syntax` value, scraped out of the
	// description prose and cut at the first ". ". Its replacement is a Usage
	// row taken from the command model, which `show bgp` does not have in the
	// published catalog, so the row is absent rather than wrong.
	got := visibleText(mainContent(t, page))
	published := publishedSyntaxRow.ReplaceAllString(
		readFixture(t, "published-equivalent-show-bgp.html"), "")
	want := visibleText(mainContent(t, published))
	if got != want {
		t.Errorf("the detail page reads as\n  %q\nthe published page reads as\n  %q", got, want)
	}

	// The mirror carries the same deviation, plus one label correction: the
	// published page and its own mirror spelled one field "Pipes, on its rows"
	// and "Pipes, on rows". One concept takes one name, so both now read the
	// page's spelling.
	mirror := readArtifact(t, paths.Output, equivalentsDirectory+"/show-bgp/"+pageMirrorFile)
	wantMirror := strings.ReplaceAll(readFixture(t, "published-equivalent-show-bgp.md"),
		"- Pipes, on rows:", "- Pipes, on its rows:")
	wantMirror = strings.ReplaceAll(wantMirror, "- Syntax: `show bgp`\n", "")
	if mirror != wantMirror {
		t.Errorf("the mirror is\n%q\nthe published mirror is\n%q", mirror, wantMirror)
	}
}

// publishedSyntaxRow matches the detail-card row that carried the retired
// `syntax` value, so a comparison can state that one difference by name.
var publishedSyntaxRow = regexp.MustCompile(`(?s)<div><dt>Syntax</dt>.*?</div>`)

// mainOpen matches the element a published page opens its content with.
var mainOpen = regexp.MustCompile(`<main\b[^>]*>`)

// mainContent answers what one page holds between <main> and </main>, which is
// the part a producer writes. The chrome around it is the shared shell and is
// asserted separately.
func mainContent(t *testing.T, page string) string {
	t.Helper()
	opening := mainOpen.FindStringIndex(page)
	if opening == nil {
		t.Fatal("the page carries no <main>")
	}
	closing := strings.Index(page[opening[1]:], "</main>")
	if closing < 0 {
		t.Fatal("the page opens <main> and never closes it")
	}
	return page[opening[1] : opening[1]+closing]
}

// VALIDATES: the index row of each command names the command, its detail page
// and its vendor equivalents.
//
// The Ze cell carries exactly ONE code span holding the registry path, which is
// the identity `id="cmd-eq-<slug>"` already states. The published page put the
// scraped `syntax` value there instead, so 80 rows disagreed with their own
// anchor.
func TestTheEquivalentIndexRowNamesTheCommandAndItsDetailPage(t *testing.T) {
	paths := commandSurfacePaths(t)
	if _, err := renderCommandEquivalents(paths); err != nil {
		t.Fatal(err)
	}

	index := readArtifact(t, paths.Output, equivalentsIndexDest)
	for _, want := range []string{
		`<tr id="cmd-eq-show-bgp"`,
		`<td class="cmd-eq-ze"><a href="show-bgp/"><code>show bgp</code></a>`,
		`<td class="cmd-eq-detail-link"><a href="show-bgp/">details</a></td>`,
		"<code>show bgp summary</code>",
	} {
		if !strings.Contains(index, want) {
			t.Errorf("the index is missing %q", want)
		}
	}
	mirror := readArtifact(t, paths.Output, equivalentsDirectory+"/"+pageMirrorFile)
	if !strings.Contains(mirror, "| `show bgp` | Read-only |") {
		t.Errorf("the index mirror does not carry the show bgp row:\n%s", mirror)
	}
	if !strings.Contains(mirror, "[details](show-bgp/)") {
		t.Error("the index mirror does not link the detail page")
	}
}

// VALIDATES: a command the binary no longer has loses its detail page.
//
// Without this the producer stops writing the page, nothing else claims it, and
// the seed carries it into every later build: a route frozen at its last
// content with a fresh timestamp, which is the failure this whole spec exists
// to remove.
func TestARetiredCommandLosesItsDetailPage(t *testing.T) {
	paths := commandSurfacePaths(t)
	retired := filepath.Join(paths.Output, filepath.FromSlash(equivalentsDirectory), "show-retired")
	if err := os.MkdirAll(retired, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(retired, pageIndexFile), []byte("<html>gone</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := renderCommandEquivalents(paths); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(retired); !os.IsNotExist(err) {
		t.Errorf("the page of a retired command survived the build: %v", err)
	}
	if _, err := os.Stat(filepath.Join(paths.Output, filepath.FromSlash(equivalentsDirectory), "show-bgp")); err != nil {
		t.Errorf("a live command lost its page: %v", err)
	}
}

// VALIDATES: a curated intent naming a command the binary does not have stops
// the build.
//
// The whole point of the page is that a migrating operator can type the Ze
// command it names. An intent left behind by a rename would publish a command
// nobody can run, and nothing else in the build would notice.
func TestAStaleCuratedCommandStopsTheBuild(t *testing.T) {
	paths := commandSurfacePaths(t)
	path := filepath.Join(paths.Output, filepath.FromSlash(equivalentsFile))
	if err := os.WriteFile(path, []byte(`{"schema-version":1,
		"vendors":{"vyos":{"label":"VyOS","short-label":"VyOS"}},
		"entries":[{"id":"stale","category":"BGP","intent":"Show peers","ze":["show bgp gone"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := renderCommandEquivalents(paths)
	if err == nil {
		t.Fatal("a stale curated command was published rather than refused")
	}
	if !strings.Contains(err.Error(), "show bgp gone") || !strings.Contains(err.Error(), "stale") {
		t.Errorf("the refusal names neither the entry nor the command: %v", err)
	}
}

// VALIDATES: the routes these producers claim are exactly the published ones,
// and the arithmetic AC-1 counts lands where the phase says it does.
//
// The two inputs are both taken from gh-pages HEAD 2fa8fa2ad: the 712 published
// routes the coverage check counts, and the 395 command paths the catalog held
// at that commit. A slug rule that renamed one directory would move 395 routes
// at once, and the coverage check would report them as 395 unclaimed pages
// beside 395 new ones with nothing to say they are the same page.
func TestTheCommandSurfacesClaimOnlyPublishedRoutes(t *testing.T) {
	published := make(map[string]bool)
	for route := range strings.SplitSeq(strings.TrimSpace(readFixture(t, "published-routes.txt")), "\n") {
		published[route] = true
	}
	paths := strings.Split(strings.TrimSpace(readFixture(t, "published-command-paths.txt")), "\n")

	claimed := []string{"/reference/cli/", "/" + strings.TrimSuffix(equivalentsIndexDest, pageIndexFile)}
	for _, path := range paths {
		claimed = append(claimed, "/"+equivalentsDirectory+"/"+commandSlug(path)+"/")
	}
	for _, route := range claimed {
		if !published[route] {
			t.Errorf("%s is claimed but was never published", route)
		}
	}
	if len(claimed) != 397 {
		t.Fatalf("the command surfaces claim %d routes, want 397", len(claimed))
	}
	t.Logf("the command surfaces claim %d of the %d published routes, leaving %d for the phases after this one",
		len(claimed), len(published), len(published)-148-len(claimed))
}
