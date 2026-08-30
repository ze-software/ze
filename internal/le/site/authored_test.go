// Design: website/AI.md -- a hand-authored page is published through the shell every other page uses
package site

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// authoredFixture lays out one website source holding a hand-authored page, a
// frozen talk deck, and the sidebar data the page's own sidebar comes from. It
// answers the source and the artifact of one build.
//
// The authored page carries the chrome such a page was written with: a head, a
// header mount naming the retired route names, a page sidebar of its own, and a
// footer with no publication stamp. What the producer does with each is what
// these tests are about.
func authoredFixture(t *testing.T) (source, output string) {
	t.Helper()
	parent := t.TempDir()
	source = filepath.Join(parent, "website")
	output = filepath.Join(parent, "artifact")
	page := strings.Join([]string{
		`<!doctype html>`,
		`<html lang="en">`,
		`    <head>`,
		`        <title>VLAN QoS Wire-Level Proof - Ze</title>`,
		`        <meta`,
		`            name="description"`,
		`            content="Proving 802.1p tagging on the wire, not in Ze&#x27;s kernel state."`,
		`        />`,
		`    </head>`,
		`    <body>`,
		`        <div id="site-header-mount" data-header-src="../../assets/header.html">`,
		`            <noscript><a href="../../changes/">Changes</a></noscript>`,
		`        </div>`,
		``,
		`        <main id="top" class="has-page-sidebar">`,
		`            <section aria-labelledby="lab-title">`,
		`                <h1 id="lab-title">VLAN QoS Wire-Level Proof</h1>`,
		`                <p>Egress maps stamp the <code>PCP</code> bits.</p>`,
		`            </section>`,
		`            <aside class="page-sidebar" aria-label="Related page links">`,
		`                <nav class="page-sidebar-nav" aria-label="Related links">`,
		`                    <a class="page-sidebar-link" href="../../stale/">`,
		`                        <span class="page-sidebar-link-label">A retired link</span>`,
		`                    </a>`,
		`                </nav>`,
		`            </aside>`,
		`        </main>`,
		``,
		`        <footer>`,
		`            <div class="footer-inner">A footer the page was written with</div>`,
		`        </footer>`,
		`    </body>`,
		`</html>`,
		``,
	}, "\n")
	links := `{"pages":{"labs/vlan-qos/":{"eyebrow":"Lab evidence","groups":[` +
		`{"title":"More labs","links":[{"href":"labs/","label":"All labs"}]}]}}}`
	writeFixtureFile(t, filepath.Join(source, "labs", "vlan-qos", pageIndexFile), page)
	writeFixtureFile(t, filepath.Join(source, "data", "page-links.json"), links)
	writeFixtureFile(t, filepath.Join(source, "talks", "linx-2026-06", pageIndexFile), frozenDeckHTML)
	writeFixtureFile(t, filepath.Join(source, "talks", "linx-2026-06", "slides.md"), "# Slides\n")
	return source, output
}

// frozenDeckHTML is a talk deck: its own document, with none of the site's own
// chrome and no <main> for a shell to splice a body into.
const frozenDeckHTML = "<!doctype html>\n<html><head><title>LINX 2026</title></head>\n" +
	"<body class=\"reveal\"><div class=\"slides\"><section>Ze at LINX</section></div></body></html>\n"

// renderAuthoredFixture runs the producer over one fixture and answers the
// routes it claimed and the artifact it wrote into.
func renderAuthoredFixture(t *testing.T) (routes []string, output string) {
	t.Helper()
	source, output := authoredFixture(t)
	routes, err := renderAuthoredPages(Paths{Repository: filepath.Dir(source), Source: source, Output: output})
	if err != nil {
		t.Fatalf("render the authored pages: %v", err)
	}
	return routes, output
}

// VALIDATES: AC-3, for the hand-authored pages. A page written by hand under
// website/ is published through the same shell as a generated one.
//
// The method is to render a page whose own chrome is stale, then ask for each
// piece of the shell. The stale chrome is what makes the test discriminate: a
// producer that staged the file would answer the retired link, the missing
// canonical and the unstamped footer.
func TestAnAuthoredPageIsPublishedThroughTheSharedShell(t *testing.T) {
	_, output := renderAuthoredFixture(t)
	page := readArtifact(t, output, "labs/vlan-qos/index.html")

	for _, element := range []string{
		`<link rel="canonical" href="https://ze-software.net/labs/vlan-qos/" />`,
		`<meta property="og:url" content="https://ze-software.net/labs/vlan-qos/" />`,
		`<title>VLAN QoS Wire-Level Proof - Ze</title>`,
		`<meta name="description" content="Proving 802.1p tagging on the wire, not in Ze&#39;s kernel state." />`,
		`<script id="theme-bootstrap">`,
		`<link rel="stylesheet" href="../../assets/site.css" />`,
		`<div id="site-header-mount" data-header-src="../../assets/header.html" data-site-root="../../">`,
		`<a href="../../project/changes/">Changes</a>`,
		`<script src="../../assets/site.js" defer></script>`,
		`<span class="footer-published">Published `,
	} {
		if !strings.Contains(page, element) {
			t.Errorf("the published page carries no %s", element)
		}
	}
	if strings.Contains(page, `<a href="../../changes/">`) {
		t.Errorf("the published page kept the header the author wrote, not the shared mount")
	}
	if strings.Contains(page, "A footer the page was written with") {
		t.Errorf("the published page kept the footer the author wrote, so it carries no stamp")
	}
}

// VALIDATES: AC-3, the part of the shell a visible-text comparison cannot see.
//
// The section's aria-labelledby names the heading that titles it, so a reader
// on a screen reader hears the section named. It lives in the authored body and
// nothing in the shell writes it, which is exactly why a producer can drop it
// without a visible-text test noticing.
func TestAnAuthoredPageKeepsTheAccessibleNameOfItsSection(t *testing.T) {
	_, output := renderAuthoredFixture(t)
	page := readArtifact(t, output, "labs/vlan-qos/index.html")

	if !strings.Contains(page, `<section aria-labelledby="lab-title">`) {
		t.Errorf("the published page states no aria-labelledby on its section")
	}
	if !strings.Contains(page, `<h1 id="lab-title">VLAN QoS Wire-Level Proof</h1>`) {
		t.Errorf("the published page states no heading for aria-labelledby to name")
	}
}

// VALIDATES: AC-3, the sidebar rule. The published sidebar is the one
// website/data/page-links.json declares, and the class on <main> follows it.
//
// A page carries its own sidebar in the source, so a producer that spliced the
// authored body unchanged would publish a link the site retired. One source of
// truth is what this asserts.
func TestAnAuthoredPageTakesItsSidebarFromTheSiteData(t *testing.T) {
	_, output := renderAuthoredFixture(t)
	page := readArtifact(t, output, "labs/vlan-qos/index.html")

	if !strings.Contains(page, `<main id="top" class="has-page-sidebar" tabindex="-1">`) {
		t.Errorf("the published page does not open <main> as a page with a sidebar")
	}
	if !strings.Contains(page, `<span class="page-sidebar-link-label">All labs</span>`) {
		t.Errorf("the published sidebar does not carry the link page-links.json declares")
	}
	if strings.Contains(page, "A retired link") {
		t.Errorf("the published sidebar is the one the author wrote, not the one the site declares")
	}
	if count := strings.Count(page, `<aside class="page-sidebar"`); count != 1 {
		t.Errorf("the published page carries %d page sidebars, want 1", count)
	}
}

// VALIDATES: AC-5, for a page whose only source is markup. Its mirror is
// converted back from the body this build published.
//
// The method is to read the mirror and ask for the page rather than for the
// markup. A mirror copied from the source would open with "<!doctype html>",
// and one converted from the WHOLE page would carry the sidebar's links, which
// the mirror's own skip list drops.
func TestAnAuthoredPageMirrorIsConvertedFromThePublishedBody(t *testing.T) {
	_, output := renderAuthoredFixture(t)
	mirror := readArtifact(t, output, "labs/vlan-qos/index.md")

	want := "# VLAN QoS Wire-Level Proof\n\nEgress maps stamp the `PCP` bits.\n"
	if mirror != want {
		t.Errorf("the mirror reads\n%q\nwant\n%q", mirror, want)
	}
}

// VALIDATES: the frozen talk rule. A deck is published exactly as its author
// wrote it, while the pages beside it are rendered.
//
// The predicate is the retired build's: a talks/ path whose second segment is
// not index.html is frozen. A deck put through the shell would lose its own
// document and gain a mirror that says nothing about it.
func TestAFrozenTalkDeckIsPublishedAsItWasAuthored(t *testing.T) {
	routes, output := renderAuthoredFixture(t)

	if deck := readArtifact(t, output, "talks/linx-2026-06/index.html"); deck != frozenDeckHTML {
		t.Errorf("the published deck is\n%q\nwant the authored bytes\n%q", deck, frozenDeckHTML)
	}
	if _, err := os.Stat(filepath.Join(output, "talks", "linx-2026-06", pageMirrorFile)); !os.IsNotExist(err) {
		t.Errorf("a frozen deck got a Markdown mirror: %v", err)
	}
	if !slices.Contains(routes, "/talks/linx-2026-06/") {
		t.Errorf("the producer claims %v, and a published deck is a route it must claim", routes)
	}
	// The page beside it went through the shell, so "frozen" is a property of
	// the deck rather than of this producer.
	if page := readArtifact(t, output, "labs/vlan-qos/index.html"); !strings.Contains(page, "<link rel=\"canonical\"") {
		t.Errorf("the authored page was left alone as well, so nothing was rendered")
	}
}

// VALIDATES: AC-1's mirror half for a frozen deck. `./le site check` refuses a
// published route with no index.md, and a deck is the one route that carries
// none.
//
// The retired build's own mirror check skipped the same paths. Without the
// exemption the coverage arithmetic goes green and the mirror check stays red,
// which is a red no producer can clear.
func TestTheMirrorCheckExemptsAFrozenTalkDeck(t *testing.T) {
	_, output := renderAuthoredFixture(t)
	writeProducerPage(t, output, "guides/anomaly")

	missing, err := checkPageMirrors(output)
	if err != nil {
		t.Fatalf("check the mirrors: %v", err)
	}
	if len(missing) != 1 || !strings.Contains(missing[0], "/guides/anomaly/") {
		t.Errorf("the mirror check reports %v, want the one page that owes a mirror", missing)
	}
}

// VALIDATES: AC-4's parity target, for a hand-authored page with no sidebar.
//
// The page reads the same as the one published at gh-pages 2fa8fa2ad: the same
// visible text and the same link targets. The comparison is over <main>,
// because the shell around it is what AC-3 changes deliberately.
func TestTheZeledonPageReadsAsThePublishedPage(t *testing.T) {
	assertAuthoredPageParity(t, "zeledon/index.html", "published-zeledon.html", "published-zeledon.md")
}

// VALIDATES: AC-4's parity target, for a hand-authored page WITH a sidebar.
//
// The labs index is the sidebar case: page-links.json states its groups, so a
// sidebar rendered from stale data or dropped altogether shows up as a
// difference in the link targets.
func TestTheLabsIndexPageReadsAsThePublishedPage(t *testing.T) {
	assertAuthoredPageParity(t, "labs/index.html", "published-labs-index.html", "published-labs-index.md")
}

// assertAuthoredPageParity renders one authored page of the real checkout and
// compares it against the page and the mirror published at gh-pages 2fa8fa2ad.
func assertAuthoredPageParity(t *testing.T, name, pageFixture, mirrorFixture string) {
	t.Helper()
	root := repositoryRoot(t)
	source := filepath.Join(root, "website")
	links, err := loadPageLinks(source)
	if err != nil {
		t.Fatalf("read the sidebar data: %v", err)
	}
	authored, err := os.ReadFile(filepath.Join(source, filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("read the authored page: %v", err)
	}
	page, err := authoredShellPage(string(authored), name, links)
	if err != nil {
		t.Fatalf("render %s: %v", name, err)
	}

	body, err := extractMain(page)
	if err != nil {
		t.Fatalf("read the rendered body: %v", err)
	}
	published, err := extractMain(readTestdata(t, pageFixture))
	if err != nil {
		t.Fatalf("read the published body: %v", err)
	}
	if got, want := visibleText(body), visibleText(published); got != want {
		gotAt, wantAt := firstMismatch(got, want)
		t.Errorf("%s reads differently from the published page.\ngot:  %s\nwant: %s", name, gotAt, wantAt)
	}
	if got, want := linkTargets(body), linkTargets(published); !slices.Equal(got, want) {
		t.Errorf("%s links %v, want %v", name, got, want)
	}

	mirror, err := htmlToMarkdown(body, pageCanonicalURL(name))
	if err != nil {
		t.Fatalf("convert the mirror: %v", err)
	}
	if want := readTestdata(t, mirrorFixture); mirror != want {
		t.Errorf("the mirror of %s differs from the published one:\n%s", name, firstDifference(mirror, want))
	}
}

// VALIDATES: AC-1, over the real checkout. Every hand-authored page of
// website/ is claimed by this producer exactly once, and every route it claims
// is one the site publishes.
//
// This is the measurement phase 10 owes: thirteen labs, performance,
// style-guide and zeledon pages plus two frozen decks were published by staging
// and claimed by nobody, so `./le site check` counted each of them unclaimed.
func TestEveryAuthoredPageIsClaimedExactlyOnce(t *testing.T) {
	root := repositoryRoot(t)
	source := filepath.Join(root, "website")
	output := t.TempDir()

	routes, err := renderAuthoredPages(Paths{Repository: root, Source: source, Output: output})
	if err != nil {
		t.Fatalf("render the authored pages: %v", err)
	}

	published := publishedArtifactRoutes(t)
	seen := map[string]int{}
	for _, route := range routes {
		seen[route]++
		if seen[route] != 1 {
			t.Errorf("%s is claimed %d times", route, seen[route])
		}
		if !slices.Contains(published, route) {
			t.Errorf("%s is claimed and the site does not publish it", route)
		}
	}
	for _, route := range []string{
		"/labs/", "/labs/appliance-install/", "/labs/bgp-interop/", "/labs/ipsec-interop/",
		"/labs/l2tp-interop/", "/labs/looking-glass-graph/", "/labs/pppoe-interop/",
		"/labs/vlan-qos/", "/labs/vpp-dataplane/", "/performance/", "/style-guide/",
		"/talks/linx-2026-06/", "/talks/netmcr-2026-04/", "/zeledon/",
	} {
		if !slices.Contains(routes, route) {
			t.Errorf("%s is published by nobody: the producer claims %v", route, routes)
		}
	}
}
