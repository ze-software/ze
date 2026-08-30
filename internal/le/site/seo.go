// Design: website/AI.md -- a crawler is told what the site publishes, once, from the artifact
// Detail: redirect.go states which directories are stubs rather than pages.
package site

import (
	"html"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// The sitemap and the robots file are written after every page producer,
// because both are a walk over the finished artifact rather than over a list a
// producer states.
func init() {
	registerDerivedProducer(Producer{Name: "seo", Render: renderSEO})
}

// The two files a crawler reads before it reads a page.
const (
	sitemapFile = "sitemap.xml"
	robotsFile  = "robots.txt"
)

// sitemapSkipDirectories are the top-level directories that publish no page.
// Each holds files a crawler has no route to.
var sitemapSkipDirectories = map[string]bool{
	"assets": true, "data": true, "tmp": true, gitMetadataDir: true,
}

// renderSEO publishes the sitemap and the robots file. Neither is a route, so
// this producer answers none.
func renderSEO(paths Paths) ([]string, error) {
	routes, err := legacyRoutes(paths.Source)
	if err != nil {
		return nil, err
	}
	urls, err := sitemapURLs(paths.Output, routes)
	if err != nil {
		return nil, err
	}
	if err := writeNamedArtifact(paths.Output, sitemapFile, sitemapXML(urls)); err != nil {
		return nil, err
	}
	return nil, writeNamedArtifact(paths.Output, robotsFile, robotsTXT())
}

// sitemapURLs answers the absolute URL of every page a crawler should index, in
// ascending order and with no repeats.
//
// A redirect stub is left out. It answers at a retired address and its own
// canonical link names the page it moved to, so listing it would ask a crawler
// to index a page that says it is not the page.
func sitemapURLs(output string, routes []legacyRoute) ([]string, error) {
	retired := make(map[string]bool, len(routes))
	for _, route := range routes {
		retired[route.From] = true
	}
	seen := make(map[string]bool, 1024)
	var urls []string
	err := filepath.WalkDir(output, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, relErr := filepath.Rel(output, path)
		if relErr != nil {
			return relErr
		}
		name := filepath.ToSlash(relative)
		if entry.IsDir() {
			if sitemapSkipDirectories[name] {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() != pageIndexFile {
			return nil
		}
		directory := strings.TrimSuffix(strings.TrimSuffix(name, pageIndexFile), "/")
		if retired[directory] {
			return nil
		}
		url := siteBase
		if directory != "" {
			url += directory + "/"
		}
		if !seen[url] {
			seen[url] = true
			urls = append(urls, url)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(urls)
	return urls, nil
}

// sitemapXML answers the sitemap document for one set of URLs.
//
// Only the location is stated. A last-modified date would have to come from the
// build, which stamps every page on every run, so it would tell a crawler that
// every page changed whenever any page did.
func sitemapXML(urls []string) string {
	var sitemap strings.Builder
	sitemap.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	sitemap.WriteString("<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")
	for _, url := range urls {
		sitemap.WriteString("  <url>\n")
		sitemap.WriteString("    <loc>" + html.EscapeString(url) + "</loc>\n")
		sitemap.WriteString("  </url>\n")
	}
	sitemap.WriteString("</urlset>\n")
	return sitemap.String()
}

// robotsTXT answers the robots file: everything is public, and here is the map.
func robotsTXT() string {
	return "User-agent: *\nAllow: /\n\nSitemap: " + siteBase + sitemapFile + "\n"
}
