// Design: website/AI.md -- a crawler is told what the site publishes, once, from the artifact
package site

import (
	"encoding/xml"
	"slices"
	"testing"
)

// seoArtifact is the artifact the SEO tests walk: three published pages, one
// retired address answering with a stub, and one page under each directory a
// crawler has no route to.
var seoArtifact = []string{
	pageIndexFile,
	"docs/" + pageIndexFile,
	"guides/quickstart/" + pageIndexFile,
	"assets/" + pageIndexFile,
	"data/" + pageIndexFile,
	"tmp/" + pageIndexFile,
}

// seoPaths lays out an artifact carrying seoArtifact plus the stub at the one
// retired address the sitemap must leave out.
func seoPaths(t *testing.T) Paths {
	t.Helper()
	output := t.TempDir()
	for _, name := range seoArtifact {
		writeArtifactFile(t, output, name, "<!doctype html><html><body>page</body></html>\n")
	}
	// A stub written out here rather than rendered: no test drives the redirect
	// renderer, and the sitemap decides by the address rather than by the bytes.
	writeArtifactFile(t, output, "roadmap/"+pageIndexFile,
		"<!doctype html><html><head><meta name=\"robots\" content=\"noindex\">"+
			"<meta http-equiv=\"refresh\" content=\"0; url=/project/roadmap/\"></head></html>\n")
	// A file that is not a page: a crawler reaches it through no route.
	writeArtifactFile(t, output, "docs/"+pageMirrorFile, "# Documentation\n")
	return Paths{Repository: t.TempDir(), Source: t.TempDir(), Output: output}
}

// publishedSitemapURLs answers the locations one published sitemap states, in
// the order the document states them.
func publishedSitemapURLs(t *testing.T, output string) []string {
	t.Helper()
	var document struct {
		XMLName xml.Name `xml:"urlset"`
		URLs    []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	published := readArtifact(t, output, sitemapFile)
	if err := xml.Unmarshal([]byte(published), &document); err != nil {
		t.Fatalf("the published sitemap is not XML: %v\n%s", err, published)
	}
	locations := make([]string, 0, len(document.URLs))
	for _, url := range document.URLs {
		locations = append(locations, url.Loc)
	}
	return locations
}

// VALIDATES: AC-13 -- the sitemap is a walk over the ARTIFACT: every published
// page, in ascending order, with no repeat, no directory a crawler has no route
// to, and no retired address.
//
// A stub answers at an address that moved and its own canonical link names the
// page it moved to, so listing one would ask a crawler to index a page that
// says it is not the page.
func TestTheSitemapListsEveryPublishedPageAndNoRetiredAddress(t *testing.T) {
	paths := seoPaths(t)

	routes, err := renderSEO(paths)
	if err != nil {
		t.Fatal(err)
	}

	if len(routes) != 0 {
		t.Errorf("the SEO producer claimed %v; neither file it writes is a route", routes)
	}
	locations := publishedSitemapURLs(t, paths.Output)
	want := []string{siteBase, siteBase + "docs/", siteBase + "guides/quickstart/"}
	if !slices.Equal(locations, want) {
		t.Fatalf("the sitemap states %v, want %v", locations, want)
	}
	if !slices.IsSorted(locations) {
		t.Errorf("the sitemap states %v out of order", locations)
	}
}

// VALIDATES: AC-13 -- the sitemap is regenerated from the artifact rather than
// carried forward, so a page the artifact does not hold is not listed and a
// page it gains is.
//
// The method renders twice over one artifact and adds a page between the runs.
// A sitemap seeded from the previous build, or from a list a producer states,
// would answer the same both times.
func TestAPageAbsentFromTheArtifactIsAbsentFromTheSitemap(t *testing.T) {
	paths := seoPaths(t)
	if _, err := renderSEO(paths); err != nil {
		t.Fatal(err)
	}
	before := publishedSitemapURLs(t, paths.Output)

	writeArtifactFile(t, paths.Output, "labs/"+pageIndexFile, "<!doctype html><html><body>labs</body></html>\n")
	if _, err := renderSEO(paths); err != nil {
		t.Fatal(err)
	}
	after := publishedSitemapURLs(t, paths.Output)

	labs := siteBase + "labs/"
	if slices.Contains(before, labs) {
		t.Errorf("the sitemap listed %s before the artifact held it: %v", labs, before)
	}
	if !slices.Contains(after, labs) {
		t.Errorf("the sitemap does not list %s after the artifact gained it: %v", labs, after)
	}
	if len(after) != len(before)+1 {
		t.Errorf("the sitemap went from %v to %v; one page was added", before, after)
	}
}

// VALIDATES: AC-13 -- the robots file states that everything is public and
// names the map, at the one absolute address a crawler reads it from.
func TestTheRobotsFileSendsACrawlerToTheSitemap(t *testing.T) {
	paths := seoPaths(t)

	if _, err := renderSEO(paths); err != nil {
		t.Fatal(err)
	}

	want := "User-agent: *\nAllow: /\n\nSitemap: " + siteBase + sitemapFile + "\n"
	if got := readArtifact(t, paths.Output, robotsFile); got != want {
		t.Errorf("the robots file states\n%q\nwant\n%q", got, want)
	}
}
