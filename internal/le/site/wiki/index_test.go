// Design: website/AI.md -- the wiki is referenced by the site, never republished by it
package sitewiki

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
)

// repositoryRoot answers the checkout these tests read the committed index from.
func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}

// fixtureWiki answers a copy of the committed wiki fixture, so a test that adds
// or removes a page writes into its own directory.
func fixtureWiki(t *testing.T) string {
	t.Helper()
	source := filepath.Join("testdata", "wiki")
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	for _, entry := range entries {
		content, readErr := os.ReadFile(filepath.Join(source, entry.Name()))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if writeErr := os.WriteFile(filepath.Join(target, entry.Name()), content, 0o644); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	return target
}

// VALIDATES: AC-17b -- the order and the grouping of the wiki section come from
// the wiki's own _Sidebar.md.
//
// The sidebar is the one curation this repository cannot derive. The method is
// to derive an index from a fixture whose sidebar states three groups in an
// order alphabetical sorting would not produce, and to read the answer back.
func TestTheWikiIndexTakesTheSidebarsOwnOrder(t *testing.T) {
	index, err := Derive(fixtureWiki(t), "https://example.test/wiki/")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Home", "About", "First Steps"}
	if len(index.Groups) != len(want) {
		t.Fatalf("the index has %d groups and the sidebar states %d", len(index.Groups), len(want))
	}
	for position, title := range want {
		if index.Groups[position].Title != title {
			t.Errorf("group %d is %q and the sidebar states %q", position, index.Groups[position].Title, title)
		}
	}
	// A bold sidebar line that carries a link is both the group heading and the
	// group's first page, which is how the wiki writes a section with a landing
	// page of its own.
	about := index.Groups[1]
	wantPages := []string{"about", "what-is-ze", "why-ze"}
	if len(about.Pages) != len(wantPages) {
		t.Fatalf("the About group holds %d pages and the sidebar lists %d", len(about.Pages), len(wantPages))
	}
	for position, slug := range wantPages {
		if about.Pages[position].Slug != slug {
			t.Errorf("About page %d is %q and the sidebar lists %q", position, about.Pages[position].Slug, slug)
		}
	}
	if index.URL("what-is-ze") != "https://example.test/wiki/what-is-ze" {
		t.Errorf("the page URL is %q", index.URL("what-is-ze"))
	}
}

// VALIDATES: AC-17 -- one page gets one entry, so the index states each title,
// URL and summary once.
//
// The live sidebar lists twelve pages under two groups each, and vpp under
// four, because a menu offers two ways to one page. The fixture repeats
// what-is-ze under First Steps for the same reason. The method is to require
// the repeat to stay at its first position and to leave the later group with
// the pages that are only its own.
func TestAPageTheSidebarListsTwiceGetsOneEntry(t *testing.T) {
	index, err := Derive(fixtureWiki(t), "")
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]int)
	for _, group := range index.Groups {
		for _, page := range group.Pages {
			seen[page.Slug]++
		}
	}
	for slug, count := range seen {
		if count != 1 {
			t.Errorf("%s has %d entries in the index", slug, count)
		}
	}
	firstSteps := index.Groups[2]
	wantPages := []string{"first-steps", "install"}
	if len(firstSteps.Pages) != len(wantPages) {
		t.Fatalf("First Steps holds %d pages and only %d are its own", len(firstSteps.Pages), len(wantPages))
	}
	for position, slug := range wantPages {
		if firstSteps.Pages[position].Slug != slug {
			t.Errorf("First Steps page %d is %q and the sidebar lists %q", position, firstSteps.Pages[position].Slug, slug)
		}
	}
}

// VALIDATES: AC-17 -- the section references the wiki rather than republishing
// it, so every page carries one summary and no body.
func TestEveryReferencedWikiPageCarriesASummary(t *testing.T) {
	index, err := Derive(fixtureWiki(t), "")
	if err != nil {
		t.Fatal(err)
	}
	if index.BaseURL != DefaultBaseURL {
		t.Errorf("an unnamed base URL answered %q and the default is %q", index.BaseURL, DefaultBaseURL)
	}
	for _, group := range index.Groups {
		for _, page := range group.Pages {
			if page.Summary == "" {
				t.Errorf("%s carries no summary", page.Slug)
			}
			if len([]rune(page.Summary)) > summaryLimit {
				t.Errorf("%s carries a %d character summary and the cap is %d",
					page.Slug, len([]rune(page.Summary)), summaryLimit)
			}
		}
	}
	// A heading, a quote and a table row are not prose, so the summary is the
	// first paragraph under them.
	about := index.Groups[1].Pages[0]
	if !strings.HasPrefix(about.Summary, "Background on the project") {
		t.Errorf("the About summary is %q; it must skip the heading and the quote and strip the bold marks", about.Summary)
	}
	why := index.Groups[1].Pages[2]
	if !strings.HasPrefix(why.Summary, "Existing daemons") {
		t.Errorf("the Why Ze summary is %q; it must skip the table", why.Summary)
	}
}

// VALIDATES: AC-17c -- a wiki page the sidebar does not list is refused by name.
//
// The method is to add a page the fixture's sidebar does not carry and that
// accountedUnlisted does not name, and to require both the refusal and the
// page's own name in the message. A page nobody has judged cannot become a
// silent omission in a committed artifact.
func TestTheWikiIndexRefusesAPageTheSidebarDoesNotList(t *testing.T) {
	wikiRoot := fixtureWiki(t)
	if err := os.WriteFile(filepath.Join(wikiRoot, "community-filters.md"),
		[]byte("# Community filters\n\nHow to publish a filter for other operators.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wikiRoot, "route-reflection.md"),
		[]byte("# Route reflection\n\nHow a route reflector is configured.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Derive(wikiRoot, "")
	if err == nil {
		t.Fatal("an unlisted wiki page was accepted; it would be a silent omission in a committed file")
	}
	if !strings.Contains(err.Error(), "route-reflection") {
		t.Errorf("the refusal does not name the page: %v", err)
	}
	// community-filters is a judged omission, so it is published as one rather
	// than refused. CLAUDE is judged too, and it is why the fixture holds it.
	if strings.Contains(err.Error(), "community-filters") || strings.Contains(err.Error(), "CLAUDE") {
		t.Errorf("the refusal names a page accountedUnlisted already judges: %v", err)
	}
}

// VALIDATES: AC-17c's other half -- a judged omission is PUBLISHED as one, with
// the reason it is out, so the committed file states what it leaves out.
func TestAJudgedOmissionIsPublishedWithItsReason(t *testing.T) {
	index, err := Derive(fixtureWiki(t), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(index.Unlisted) != 1 || index.Unlisted[0].Slug != "CLAUDE" {
		t.Fatalf("the index states %v as its omissions; the fixture holds one unlisted page, CLAUDE", index.Unlisted)
	}
	if index.Unlisted[0].Why == "" {
		t.Error("the omission carries no reason, so a reader cannot tell a decision from an oversight")
	}
}

// VALIDATES: a sidebar link that resolves to no page is refused by name, so the
// index never publishes a link a reader clicks into a 404.
func TestTheWikiIndexRefusesASidebarLinkWithNoPage(t *testing.T) {
	wikiRoot := fixtureWiki(t)
	if err := os.Remove(filepath.Join(wikiRoot, "install.md")); err != nil {
		t.Fatal(err)
	}
	_, err := Derive(wikiRoot, "")
	if err == nil {
		t.Fatal("a sidebar link with no page was accepted")
	}
	if !strings.Contains(err.Error(), "install") {
		t.Errorf("the refusal does not name the link: %v", err)
	}
}

// VALIDATES: AC-17a -- the committed index round-trips, so what `le site wiki
// update` writes is what the site build reads back.
func TestTheCommittedIndexRoundTrips(t *testing.T) {
	index, err := Derive(fixtureWiki(t), "https://example.test/wiki/")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "website", "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(root, index); err != nil {
		t.Fatal(err)
	}
	read, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if read.BaseURL != index.BaseURL || len(read.Groups) != len(index.Groups) ||
		read.PageCount() != index.PageCount() {
		t.Fatalf("the file read back states %d groups and %d pages; it was written with %d and %d",
			len(read.Groups), read.PageCount(), len(index.Groups), index.PageCount())
	}
}

// VALIDATES: AC-17a -- the index this repository commits is the one the wiki
// checkout beside it states.
//
// It is skipped where the wiki is not checked out, because the site build never
// needs it: that independence is the point of committing the file.
func TestTheCommittedIndexStatesTheLiveWiki(t *testing.T) {
	root := repositoryRoot(t)
	wikiRoot := filepath.Join(filepath.Dir(root), "wiki")
	if info, err := os.Stat(filepath.Join(wikiRoot, sidebarFile)); err != nil || !info.Mode().IsRegular() {
		t.Skip("no wiki checkout beside this one")
	}
	committed, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	derived, err := Derive(wikiRoot, committed.BaseURL)
	if err != nil {
		t.Fatal(err)
	}
	wanted, err := Marshal(derived)
	if err != nil {
		t.Fatal(err)
	}
	have, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(IndexFile)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(have, wanted) {
		t.Errorf("%s is stale; run `le site wiki update`", IndexFile)
	}
}
