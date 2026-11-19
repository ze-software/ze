// Design: docs/contributing/gh-pages.md -- the fleet interface demonstration is written under tmp/ and never published
package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fleetDemoFiles are the files a fleet demonstration render leaves in a
// website output: the host page, the frame and the recording render writes
// (internal/le/site/fleetdemo, render), and one chunk file of the kind the
// tour writes beside the frame. Only their names and places reach the site's
// walks; their bytes are what must survive a build unchanged.
var fleetDemoFiles = map[string]string{
	"tmp/fleet-demo/index.html": `<!DOCTYPE html><html><head>` +
		`<meta name="robots" content="noindex, nofollow"><title>Fleet interface</title></head>` +
		`<body><div data-lg-replay-frame="frame.html"></div></body></html>`,
	"tmp/fleet-demo/frame.html":      `<!DOCTYPE html><html><body><main>hub-lon-01</main></body></html>`,
	"tmp/fleet-demo/recording.json":  `{"schema": 2}`,
	"tmp/fleet-demo/estate-hub.json": `{"exchanges": []}`,
}

// TestFleetDemoStaysOutOfThePublishedSite runs a site build over a scratch
// published checkout that holds a fleet demonstration, then asks each walk
// that decides what the site publishes.
//
// VALIDATES: AC-6 -- after a full Build into the published checkout (output
// beside the main checkout, so the build snapshots, seeds and prunes it as it
// does the owner's ../gh-pages), every file of tmp/fleet-demo/ is on disk with
// its bytes unchanged; the page registry (pageRegistry, the route list the
// build stamps and counts coverage from), the sitemap (sitemapURLs), the
// search index's page walk (pageSearchRecords), the publication receipt the
// build wrote (publicationProducerRecord, its provenance files) and git, under
// the ignore list the build wrote, name none of them. A published page of the
// same build is named by each, so no assertion passes on an empty walk.
// PREVENTS: the demonstration reaching the published site, and a build that
// deletes or rewrites it.
//
// The page producers are stubbed, as in TestBuildStampsEveryPublishedPage: a
// producer writes published pages, never under tmp/, and the sitemap and search
// producers are represented by the walks they call. The render itself is not
// run here, because package sitefleetdemo imports this one; that render writes
// only under tmp/fleet-demo/ is TestFleetDemoRenderWritesTheDemonstration's.
// The plain-figure walk is TestPlainFiguresSkipUnpublishedDirectories's.
func TestFleetDemoStaysOutOfThePublishedSite(t *testing.T) {
	stubLiveInputs(t, `[{"path":"show test","short-help":"Show rows","mode":"read-only"}]`)
	stubProducers(t)
	stubBuildClock(t, time.Date(2026, time.November, 19, 12, 0, 0, 0, time.UTC))
	parent := t.TempDir()
	root := filepath.Join(parent, "main")
	source := filepath.Join(root, "website")
	writeArtifactFile(t, source, "labs/index.html", authoredPage)
	writeArtifactFile(t, source, "labs/index.md", "# Labs\n\nThe labs page.\n")
	writeSourceAssets(t, source)
	if err := os.MkdirAll(filepath.Join(root, "ai"), 0o755); err != nil {
		t.Fatal(err)
	}
	roadmapGit(t, root, "init", "-q")
	commitSiteFixture(t, root)

	output := filepath.Join(parent, "gh-pages")
	writeArtifactFile(t, output, "assets/site.css", "\n")
	writeArtifactFile(t, output, "assets/site.js", "\n")
	for name, content := range fleetDemoFiles {
		writeArtifactFile(t, output, name, content)
	}

	if _, err := Build(t.Context(), BuildOptions{Repository: root, Output: output}); err != nil {
		t.Fatal(err)
	}

	for name, content := range fleetDemoFiles {
		kept, err := os.ReadFile(filepath.Join(output, filepath.FromSlash(name)))
		if err != nil {
			t.Fatalf("the build removed %s: %v", name, err)
		}
		if string(kept) != content {
			t.Errorf("the build rewrote %s:\n%s", name, kept)
		}
	}

	pages, err := pageRegistry(output)
	if err != nil {
		t.Fatal(err)
	}
	routes := make([]string, 0, len(pages))
	for _, page := range pages {
		routes = append(routes, page.Route)
	}
	assertNamesPublishedNotDemo(t, "the page registry", routes, "/labs/", "/tmp/")

	urls, err := sitemapURLs(output)
	if err != nil {
		t.Fatal(err)
	}
	assertNamesPublishedNotDemo(t, "the sitemap", urls, siteBase+"labs/", "/tmp/")

	records, err := pageSearchRecords(output)
	if err != nil {
		t.Fatal(err)
	}
	searched := make([]string, 0, len(records))
	for _, record := range records {
		searched = append(searched, record.URL)
	}
	assertNamesPublishedNotDemo(t, "the search index", searched, "labs/", "tmp/")

	paths, err := ResolvePaths(root, output)
	if err != nil {
		t.Fatal(err)
	}
	record, err := publicationProducerRecord(paths)
	if err != nil {
		t.Fatal(err)
	}
	if record.Provenance == nil {
		t.Fatal("the full build wrote no publication receipt")
	}
	received := make([]string, 0, len(record.Provenance.Files))
	for _, file := range record.Provenance.Files {
		received = append(received, file.Path)
	}
	assertNamesPublishedNotDemo(t, "the publication receipt", received, "labs/index.html", "tmp/")

	roadmapGit(t, output, "init", "-q")
	status := roadmapGit(t, output, "status", "--porcelain", "--untracked-files=all")
	assertNamesPublishedNotDemo(t, "git status", strings.Split(status, "\n"), "?? labs/index.html", "tmp/")
}

// assertNamesPublishedNotDemo fails unless names holds published exactly and
// no name holds unpublished. The first half proves the walk saw the build's
// published page, so the second cannot pass on a walk that saw nothing.
func assertNamesPublishedNotDemo(t *testing.T, walk string, names []string, published, unpublished string) {
	t.Helper()
	seen := false
	for _, name := range names {
		if name == published {
			seen = true
		}
		if strings.Contains(name, unpublished) {
			t.Errorf("%s names %q, a file of the unpublished demonstration", walk, name)
		}
	}
	if !seen {
		t.Errorf("%s does not name the published %q, so it proves nothing about tmp/: %v", walk, published, names)
	}
}
