package site

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/lepath"
)

// testShell answers a shell with every caller-supplied field set, so a test
// asserting on the chrome states only what it changes.
func testShell() pageShell {
	return pageShell{
		Title:             "Command Ownership - Ze",
		Description:       "Ze documentation: architecture/command-ownership.md",
		SocialTitle:       "Command Ownership - Ze",
		SocialDescription: "Ze documentation: architecture/command-ownership.md",
		Root:              "../../",
		Path:              "architecture/command-ownership/index.html",
	}
}

// TestShellCarriesEveryChromeElement checks AC-3 the way a reader meets the
// page: every element the published page shows is present once, in a page the
// shell rendered from nothing but its own fields.
//
// The published page at gh-pages HEAD is the reference. Each want below is a
// fragment of that page, so a shell that drops one publishes a page missing a
// title, a stylesheet, a navigation fallback or a publication stamp.
func TestShellCarriesEveryChromeElement(t *testing.T) {
	buildClock = func() time.Time { return time.Date(2026, 8, 25, 3, 17, 0, 0, time.UTC) }
	t.Cleanup(func() { buildClock = time.Now })

	page := testShell().render("<p>body</p>")

	wants := []struct {
		element  string
		fragment string
	}{
		{"the doctype", "<!doctype html>\n<html lang=\"en\">"},
		{"the theme bootstrap", `<script id="theme-bootstrap">`},
		{"the title", "<title>Command Ownership - Ze</title>"},
		{"the description", `<meta name="description" content="Ze documentation: architecture/command-ownership.md" />`},
		{"the social title", `<meta property="og:title" content="Command Ownership - Ze" />`},
		{"the social image", `<meta property="og:image" content="https://ze-software.net/assets/social-card.png" />`},
		{"the twitter card", `<meta name="twitter:card" content="summary_large_image" />`},
		{"the icon", `<link rel="icon" href="../../assets/ze.svg" type="image/svg+xml" />`},
		{"the font stylesheet", `<link rel="stylesheet" href="../../assets/vendor/fonts/fonts.css" />`},
		{"the canonical link", `<link rel="canonical" href="https://ze-software.net/architecture/command-ownership/" />`},
		{"the social url", `<meta property="og:url" content="https://ze-software.net/architecture/command-ownership/" />`},
		{"the stylesheet", `<link rel="stylesheet" href="../../assets/site.css" />`},
		{"the structured data", `<script type="application/ld+json">{"@context":"https://schema.org"`},
		{"the skip link", `<a class="skip-link" href="#top">Skip to main content</a>`},
		{"the header mount", `<div id="site-header-mount" data-header-src="../../assets/header.html" data-site-root="../../">`},
		{"the noscript fallback", `<nav class="site-header-fallback" aria-label="Site navigation">`},
		{"a fallback hub link", `<a href="../../contribute/">Contribute</a>`},
		{"the main element", `<main id="top" tabindex="-1">`},
		{"the body", "<p>body</p>"},
		{"the deferred script", `<script src="../../assets/site.js" defer></script>`},
		{"the footer stamp", `<span class="footer-published">Published 25 August 2026 03:17 UTC</span>`},
		{"the close", "</body>\n</html>"},
	}
	for _, want := range wants {
		if strings.Count(page, want.fragment) != 1 {
			t.Errorf("the page must carry %s exactly once: %q\npage:\n%s", want.element, want.fragment, page)
		}
	}
}

// TestShellPutsCanonicalBeforeTheStylesheet pins the ONE ordering constraint the
// head carries. The retired renderer emitted no canonical link and no og:url; a
// later pass inserted both immediately before the site.css link, and every
// published page shows them there. A shell that emits them where the rest of
// the social meta lives would move them.
func TestShellPutsCanonicalBeforeTheStylesheet(t *testing.T) {
	page := testShell().render("<p>body</p>")

	canonical := strings.Index(page, `<link rel="canonical"`)
	social := strings.Index(page, `<meta property="og:url"`)
	stylesheet := strings.Index(page, `assets/site.css`)
	fonts := strings.Index(page, `assets/vendor/fonts/fonts.css`)
	if canonical < 0 || social < 0 || stylesheet < 0 || fonts < 0 {
		t.Fatalf("the head must carry the fonts link, the canonical link, og:url and the stylesheet:\n%s", page)
	}
	if fonts >= canonical || canonical >= social || social >= stylesheet {
		t.Errorf("the head must read fonts, canonical, og:url, stylesheet; got fonts=%d canonical=%d og:url=%d stylesheet=%d",
			fonts, canonical, social, stylesheet)
	}
}

// TestSidebarClassFollowsAnEmptySidebar checks that the class on <main> and the
// sidebar under it are decided from ONE value.
//
// The retired renderer computed the sidebar in the head call and read it back
// in the foot call through a module global. The three cases below are the ones
// that global made possible to get wrong: a sidebar wins over a wide request, a
// wide request applies only when there is no sidebar, and a page with neither
// takes no class at all.
func TestSidebarClassFollowsAnEmptySidebar(t *testing.T) {
	sidebar := "            <aside class=\"page-sidebar\" aria-label=\"Related page links\">\n            </aside>\n"
	cases := []struct {
		name    string
		sidebar string
		wide    bool
		want    string
	}{
		{"a sidebar wins over a wide request", sidebar, true, `<main id="top" class="has-page-sidebar" tabindex="-1">`},
		{"a sidebar with no wide request", sidebar, false, `<main id="top" class="has-page-sidebar" tabindex="-1">`},
		{"a wide request with no sidebar", "", true, `<main id="top" class="site-main-wide" tabindex="-1">`},
		{"neither", "", false, `<main id="top" tabindex="-1">`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			shell := testShell()
			shell.Sidebar = testCase.sidebar
			shell.Wide = testCase.wide
			page := shell.render("<p>body</p>")
			if !strings.Contains(page, testCase.want) {
				t.Errorf("main must open as %q\npage:\n%s", testCase.want, page)
			}
			if strings.Contains(page, "page-sidebar") != (testCase.sidebar != "") {
				t.Errorf("the sidebar markup and the class must agree; sidebar=%q page:\n%s", testCase.sidebar, page)
			}
		})
	}
}

// TestPageSidebarDropsAGroupThatResolvesToThisPage checks the rule that makes an
// empty sidebar possible: a link to the page a reader is already on is dropped,
// and a group left with no link is dropped with it.
func TestPageSidebarDropsAGroupThatResolvesToThisPage(t *testing.T) {
	links := pageLinks{
		Pages: map[string]pageLinkSpec{
			"compare/bgp/": {
				Eyebrow: "Comparison",
				Groups: []pageLinkGroup{
					{Title: "Only this page", Links: []pageLink{{Href: "compare/bgp/", Label: "BGP"}}},
					{Title: "Elsewhere", Links: []pageLink{{Href: "compare/nos/", Label: "NOS"}}},
				},
			},
		},
	}
	sidebar := pageSidebar("../../", "compare/bgp/index.html", links)
	if strings.Contains(sidebar, "Only this page") {
		t.Errorf("a group whose every link resolves to this page must be dropped:\n%s", sidebar)
	}
	if !strings.Contains(sidebar, `href="../../compare/nos/"`) {
		t.Errorf("a group with a link elsewhere must survive, rooted at the page:\n%s", sidebar)
	}

	only := pageLinks{Pages: map[string]pageLinkSpec{
		"compare/bgp/": {Groups: []pageLinkGroup{
			{Title: "Only this page", Links: []pageLink{{Href: "compare/bgp/", Label: "BGP"}}},
		}},
	}}
	if empty := pageSidebar("../../", "compare/bgp/index.html", only); empty != "" {
		t.Errorf("a sidebar whose every group emptied must answer nothing, so <main> loses its class; got:\n%s", empty)
	}
}

// TestPageLinksLoadFromTheRepositorysOwnData checks that the sidebar data this
// repository publishes parses through the loader a producer will use, and that
// a checkout with no such file answers an empty set rather than an error.
func TestPageLinksLoadFromTheRepositorysOwnData(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	links, err := loadPageLinks(filepath.Join(root, "website"))
	if err != nil {
		t.Fatalf("load website/data/page-links.json: %v", err)
	}
	if len(links.Pages) == 0 || len(links.Patterns) == 0 || len(links.External) == 0 {
		t.Errorf("the published sidebar data must carry pages, patterns and external links; got %d/%d/%d",
			len(links.Pages), len(links.Patterns), len(links.External))
	}

	empty, err := loadPageLinks(t.TempDir())
	if err != nil {
		t.Errorf("a source with no sidebar data must answer an empty set, not an error: %v", err)
	}
	if len(empty.Pages) != 0 {
		t.Errorf("a source with no sidebar data must answer nothing: %v", empty)
	}
}
