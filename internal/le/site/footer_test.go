// Design: website/AI.md -- the footer carries the page publication stamp
package site

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// authoredPage is one hand-authored page carrying the footer a build replaces.
// The indentation and the stale stamp are both deliberate: a real authored page
// carries the footer of whichever build last wrote it.
const authoredPage = `<!doctype html>
<html lang="en">
    <body>
        <main id="top"><h1>Lab</h1></main>
        <footer>
            <div class="footer-inner">
                <div class="footer-bottom">
                    <a href="../../license/">Ze is AGPLv3 open source.</a>
                </div>
            </div>
        </footer>
    </body>
</html>
`

// stubBuildClock makes a build publish at the given time and restores the wall
// clock when the test ends.
func stubBuildClock(t *testing.T, published time.Time) {
	t.Helper()
	previous := buildClock
	t.Cleanup(func() { buildClock = previous })
	buildClock = func() time.Time { return published }
}

// VALIDATES: the footer carries the license line and the publication stamp, and
// the stamp reads in UTC whatever zone the build machine keeps.
func TestFooterCarriesTheLicenseLineAndTheStamp(t *testing.T) {
	published := time.Date(2026, time.August, 17, 15, 32, 5, 0, time.FixedZone("BST", 3600))
	footer := footerHTML("../../", published)
	if !strings.Contains(footer, `<a href="../../license/">Ze is AGPLv3 open source.</a>`) {
		t.Fatalf("footer lost the license line: %s", footer)
	}
	if !strings.Contains(footer, `<span class="footer-published">Published 17 August 2026 14:32 UTC</span>`) {
		t.Fatalf("footer lost the publication stamp: %s", footer)
	}
}

// VALIDATES: the root page links the license without a directory prefix, and a
// nested page climbs one level for each directory it sits under.
func TestPageRootClimbsOneLevelPerDirectory(t *testing.T) {
	for _, testCase := range []struct{ page, want string }{
		{"index.html", ""},
		{"labs/index.html", "../"},
		{"labs/appliance-install/index.html", "../../"},
		{"docs/guide/ipsec/index.html", "../../../"},
	} {
		if got := pageRoot(testCase.page); got != testCase.want {
			t.Errorf("pageRoot(%q) = %q, want %q", testCase.page, got, testCase.want)
		}
	}
}

// VALIDATES: a build replaces the whole footer of an already authored page and
// leaves every other byte of that page alone.
func TestPatchFooterStampsAnAlreadyAuthoredPage(t *testing.T) {
	published := time.Date(2026, time.August, 17, 14, 32, 5, 0, time.UTC)
	patched, found := patchFooter([]byte(authoredPage), "../../", published)
	if !found {
		t.Fatal("an authored footer was not found")
	}
	page := string(patched)
	if !strings.Contains(page, `<span class="footer-published">Published 17 August 2026 14:32 UTC</span>`) {
		t.Fatalf("page carries no stamp: %s", page)
	}
	if strings.Count(page, "<footer>") != 1 || strings.Count(page, "</footer>") != 1 {
		t.Fatalf("footer was duplicated rather than replaced: %s", page)
	}
	if !strings.Contains(page, `<main id="top"><h1>Lab</h1></main>`) {
		t.Fatalf("page body was rewritten: %s", page)
	}
}

// VALIDATES: a page with no footer is left exactly as it is, so a build cannot
// invent chrome for a page that carries none.
func TestPatchFooterLeavesAPageWithNoFooter(t *testing.T) {
	page := []byte("<html><body>no chrome</body></html>\n")
	patched, found := patchFooter(page, "", time.Now())
	if found {
		t.Fatal("a footer was reported on a page that carries none")
	}
	if !bytes.Equal(patched, page) {
		t.Fatalf("page was rewritten: %q", patched)
	}
}

// VALIDATES: a page this build did not otherwise change keeps the stamp it was
// published with, and a page the build did change carries the new one.
// PREVENTS: a build that changes three pages rewriting every page on the site,
// which buries the three real changes in a timestamp diff.
func TestCarryPublicationStampsKeepsTheStampOfAnUnchangedPage(t *testing.T) {
	previous := t.TempDir()
	next := t.TempDir()
	old := `<span class="footer-published">Published 17 August 2026 14:32 UTC</span>`
	fresh := `<span class="footer-published">Published 29 August 2026 09:00 UTC</span>`
	write := func(root, name, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(previous, "quiet/index.html", "<p>same words</p>"+old)
	write(next, "quiet/index.html", "<p>same words</p>"+fresh)
	write(previous, "edited/index.html", "<p>old words</p>"+old)
	write(next, "edited/index.html", "<p>new words</p>"+fresh)
	write(next, "added/index.html", "<p>brand new</p>"+fresh)

	carried, err := carryPublicationStamps(previous, next)
	if err != nil {
		t.Fatal(err)
	}
	if carried != 1 {
		t.Fatalf("carried %d stamps, want 1", carried)
	}
	for _, testCase := range []struct{ page, want string }{
		{"quiet/index.html", old},
		{"edited/index.html", fresh},
		{"added/index.html", fresh},
	} {
		content, readErr := os.ReadFile(filepath.Join(next, testCase.page))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !strings.Contains(string(content), testCase.want) {
			t.Errorf("%s carries the wrong stamp: %s", testCase.page, content)
		}
	}
}

// VALIDATES: a full build stamps the publication time into the footer of every
// published page, including a hand-authored page that carries no stamp of its
// own.
// PREVENTS: the state of 2026-08-29, where the native build staged authored
// pages verbatim and the published site lost the stamp on 409 pages, because
// the retired Python build was the only thing that had ever written one.
func TestBuildStampsEveryPublishedPage(t *testing.T) {
	stubLiveCommandCatalog(t, `[{"path":"show test","description":"Show rows","mode":"read-only"}]`)
	// This build is about staging, so the page producers are stubbed out: a
	// synthetic checkout carries no docs/ tree for them to publish, and
	// TestBuildRunsEveryRegisteredProducer already pins the registry itself.
	stubProducers(t)
	stubBuildClock(t, time.Date(2026, time.August, 29, 11, 51, 0, 0, time.UTC))
	parent := t.TempDir()
	root := filepath.Join(parent, "main")
	source := filepath.Join(root, "website")
	if err := os.MkdirAll(filepath.Join(source, "labs", "appliance-install"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "labs", "appliance-install", "index.html"), []byte(authoredPage), 0o644); err != nil {
		t.Fatal(err)
	}
	writeSourceAssets(t, source)
	seedAssets := filepath.Join(parent, "gh-pages", "assets")
	if err := os.MkdirAll(seedAssets, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, asset := range []string{"site.css", "site.js"} {
		if err := os.WriteFile(filepath.Join(seedAssets, asset), []byte("\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{{"init"}, {"add", "."}} {
		command := exec.CommandContext(t.Context(), "git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", arguments, err, output)
		}
	}
	report, err := Build(BuildOptions{Repository: root, Output: filepath.Join(parent, "artifact")})
	if err != nil {
		t.Fatal(err)
	}
	if report.Published != "29 August 2026 11:51 UTC" {
		t.Fatalf("build reported %q as its publication time", report.Published)
	}
	page, err := os.ReadFile(filepath.Join(parent, "artifact", "labs", "appliance-install", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), `<span class="footer-published">Published 29 August 2026 11:51 UTC</span>`) {
		t.Fatalf("the published page carries no stamp: %s", page)
	}
	if !strings.Contains(string(page), `<a href="../../license/">Ze is AGPLv3 open source.</a>`) {
		t.Fatalf("the published page lost its license line: %s", page)
	}
}
