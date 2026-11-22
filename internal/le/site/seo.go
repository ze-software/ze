// Design: website/AI.md -- a crawler is told what the site publishes, once, from the artifact
package site

import (
	"html"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The sitemap and the robots file are written after every page producer,
// because both are a walk over the finished artifact rather than over a list a
// producer states.
func init() {
	registerDerivedProducer(Producer{Name: "seo", Render: renderSEO})
}

// The files a crawler or visitor reaches at the site root.
const (
	sitemapFile  = "sitemap.xml"
	robotsFile   = "robots.txt"
	notFoundFile = "404.html"
)

// renderSEO publishes the sitemap, robots file, and custom 404 page. None is a
// content route, so this producer answers none.
func renderSEO(paths Paths) ([]string, error) {
	urls, err := sitemapURLs(paths.Output)
	if err != nil {
		return nil, err
	}
	if err := writeNamedArtifact(paths.Output, sitemapFile, sitemapXML(urls)); err != nil {
		return nil, err
	}
	if err := writeNamedArtifact(paths.Output, notFoundFile, notFoundHTML()); err != nil {
		return nil, err
	}
	return nil, writeNamedArtifact(paths.Output, robotsFile, robotsTXT())
}

// sitemapURLs answers the absolute URL of every page a crawler should index, in
// ascending order and with no repeats.
//
// No address is left out for having moved: the site publishes no redirect
// stub, so every index.html in the artifact is a page a reader reaches.
func sitemapURLs(output string) ([]string, error) {
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
			if unpublishedDirectories[name] {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() != pageIndexFile {
			return nil
		}
		directory := strings.TrimSuffix(strings.TrimSuffix(name, pageIndexFile), "/")
		if directory == searchDirectory || directory == "style-guide" {
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
	slices.Sort(urls)
	return urls, nil
}

// sitemapXML answers the sitemap document for one set of URLs.
//
// Only the location is stated. A last-modified date would have to come from the
// build, which stamps every page on every run, so it would tell a crawler that
// every page changed whenever any page did.
func sitemapXML(urls []string) string {
	var sitemap textbuf.Buffer
	sitemap.Reset().Str("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	sitemap.Str("<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")
	for _, url := range urls {
		sitemap.Str("  <url>\n")
		sitemap.Str("    <loc>").Str(html.EscapeString(url)).Str("</loc>\n")
		sitemap.Str("  </url>\n")
	}
	sitemap.Str("</urlset>\n")
	return sitemap.String()
}

// robotsTXT answers the robots file: everything is public, and here is the map.
func robotsTXT() string {
	return "User-agent: *\nAllow: /\n\nSitemap: " + siteBase + sitemapFile + "\n"
}

// notFoundHTML answers the branded 404 page GitHub Pages serves for missing
// URLs, using root-relative links so it works from any path depth.
func notFoundHTML() string {
	const body = `            <section class="md-content reveal" aria-labelledby="not-found-title">
                <div class="journey-hero reveal">
                    <span class="journey-eyebrow">404</span>
                    <h1 id="not-found-title">Page not found.</h1>
                    <p>The page you asked for does not exist or has moved. Use site search or start from one of the hubs below.</p>
                </div>
                <div class="link-list">
                    <a href="/">Home <span>overview, demos, and quickstart</span></a>
                    <a href="/search/">Search <span>find guides, commands, and configuration options</span></a>
                    <a href="/docs/">Documentation <span>guides organized by task</span></a>
                    <a href="/features/">Features <span>capabilities and maturity status</span></a>
                    <a href="/reference/">Reference <span>CLI, YANG tree, plugins, and RFC status</span></a>
                    <a href="/project/changes/">Changes <span>update reports and changelog</span></a>
                    <a href="/project/roadmap/">Roadmap <span>planned release work</span></a>
                </div>
            </section>
`
	shell := pageShell{
		Title:       "Page Not Found - Ze",
		Description: "The requested page could not be found on ze-software.net.",
		Root:        "/",
		Path:        notFoundFile,
	}
	return shell.render(body)
}
