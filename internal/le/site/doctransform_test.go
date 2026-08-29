// Design: website/AI.md -- each pass over a rendered body, on its own
package site

import (
	"strings"
	"testing"
)

// TestEvidenceCellsLiftCitationsOntoTheirOwnLines covers the comparison
// tables' citation layout.
//
// A comparison table states a claim and the files that prove it in one cell,
// which a reader has to disentangle. The pass lifts the citations out, keeps
// them as code elements so the page's own script still resolves each one, and
// leaves a cell that cites nothing exactly as it was.
func TestEvidenceCellsLiftCitationsOntoTheirOwnLines(t *testing.T) {
	cells := []struct {
		name string
		cell string
		want string
	}{
		{
			name: "one citation moves and the prose closes up",
			cell: "<td>Ze has <code>go.mod</code>; VyOS has Debian/Python package metadata.</td>",
			want: `<td>Ze has; VyOS has Debian/Python package metadata. <span class="ev-src">` +
				`<span class="ev-ref"><code>go.mod</code></span></span></td>`,
		},
		{
			name: "a line range joins the citation it continues",
			cell: "<td>Static routes <code>ze/internal/plugins/static/routes.go:1-15</code>, <code>:24-43</code> program the FIB.</td>",
			want: `<td>Static routes program the FIB. <span class="ev-src"><span class="ev-ref">` +
				`<code>internal/plugins/static/routes.go:1-15</code>, <code>:24-43</code></span></span></td>`,
		},
		{
			name: "a cell that cites nothing is left alone",
			cell: "<td>Run <code>ze show bgp summary</code> to see the peers.</td>",
			want: "<td>Run <code>ze show bgp summary</code> to see the peers.</td>",
		},
	}
	for _, cell := range cells {
		t.Run(cell.name, func(t *testing.T) {
			if got := relayoutEvidenceCells(cell.cell); got != cell.want {
				t.Errorf("relayout gave\n  %s\nwant\n  %s", got, cell.want)
			}
		})
	}
}

// TestEvidenceCleanupKeepsACharacterReferenceWhole is the regression this pass
// acquired when the renderer changed.
//
// The prose cleanups treat a semicolon as punctuation a lifted citation left
// behind. goldmark writes a quotation mark inside a code span as &quot;, whose
// closing semicolon is not that, and rewriting it publishes the literal text
// &quot to a reader. The retired renderer left the quotation mark alone and so
// never met the case.
func TestEvidenceCleanupKeepsACharacterReferenceWhole(t *testing.T) {
	const cell = `<td>traceroute builds ICMP through <code>net.ListenPacket(&quot;ip4:icmp&quot;, ...)</code>, ` +
		`which the kernel refuses. <code>internal/test/runner/caps_declaration_lint_test.go</code></td>`
	got := relayoutEvidenceCells(cell)
	if strings.Contains(got, "&quot,") || strings.Contains(got, "&quot.") {
		t.Errorf("the cleanup broke a character reference:\n%s", got)
	}
	if strings.Count(got, "&quot;") != 2 {
		t.Errorf("both quotation marks must survive the cleanup:\n%s", got)
	}
	if !strings.Contains(got, `<span class="ev-src">`) {
		t.Errorf("the citation must still move onto its own line:\n%s", got)
	}
}

// TestAVerdictCellTakesItsSymbol covers the Yes, No, Partial and N/A cells a
// comparison table scans by color.
//
// The color and the symbol carry the verdict, so the word only adds width.
// N/A keeps its text because it has no symbol, and a cell a renderer already
// classed decided its own colors and is left alone.
func TestAVerdictCellTakesItsSymbol(t *testing.T) {
	cells := map[string]string{
		"<td>Yes</td>":                  `<td class="cell-yes">✓</td>`,
		"<td>No, not on Linux</td>":     `<td class="cell-no">✕</td>`,
		"<td>Partial</td>":              `<td class="cell-partial">∿</td>`,
		"<td>N/A</td>":                  `<td class="cell-na">N/A</td>`,
		`<td class="own">Yes</td>`:      `<td class="own">Yes</td>`,
		"<td>Yesterday's counters</td>": "<td>Yesterday's counters</td>",
		"<td>Nothing to report</td>":    "<td>Nothing to report</td>",
	}
	for cell, want := range cells {
		if got := colorCodeCells(cell); got != want {
			t.Errorf("%s became %s, want %s", cell, got, want)
		}
	}
}

// TestALinkThatLeavesTheSiteOpensInANewTab covers the external-link pass. The
// new page must not reach back into this one through window.opener, so every
// outward link states rel="noopener", and a rel the author already wrote
// survives.
func TestALinkThatLeavesTheSiteOpensInANewTab(t *testing.T) {
	links := map[string]string{
		`<a href="https://github.com/ze-software/ze">code</a>`: `<a href="https://github.com/ze-software/ze" target="_blank" rel="noopener">code</a>`,
		`<a href="../guides/rpki/">RPKI</a>`:                   `<a href="../guides/rpki/">RPKI</a>`,
		`<a href="https://ze-software.net/faq/">FAQ</a>`:       `<a href="https://ze-software.net/faq/">FAQ</a>`,
		`<a rel="nofollow" href="https://example.test/">x</a>`: `<a rel="nofollow noopener" href="https://example.test/" target="_blank">x</a>`,
		`<a href="mailto:ze@example.test">mail</a>`:            `<a href="mailto:ze@example.test">mail</a>`,
	}
	for link, want := range links {
		if got := patchExternalLinkTargets(link); got != want {
			t.Errorf("%s became\n  %s\nwant\n  %s", link, got, want)
		}
	}
}

// TestACrossDocumentLinkResolvesToThePublishedPage covers both halves of the
// link rewriting: the rendered page and its Markdown mirror.
//
// A link to another page the manifest publishes must keep the reader on the
// site. Every other relative link must reach the source on the code host,
// because this site publishes no page for it.
func TestACrossDocumentLinkResolvesToThePublishedPage(t *testing.T) {
	manifest := map[string]string{
		"guide/rpki.md":    "guides/rpki",
		"features/srv6.md": "features/srv6",
		"guide/anomaly.md": "guides/anomaly",
	}
	const from = "guides/anomaly"
	const docRel = "guide/anomaly.md"

	body := map[string]string{
		`<a href="rpki.md">RPKI</a>`:              `<a href="../rpki/">RPKI</a>`,
		`<a href="rpki.md#caches">caches</a>`:     `<a href="../rpki/#caches">caches</a>`,
		`<a href="../features/srv6.md">SRv6</a>`:  `<a href="../../features/srv6/">SRv6</a>`,
		`<a href="unpublished.md">gone</a>`:       `<a href="` + codeHostBlob + `guide/unpublished.md" target="_blank" rel="noopener">gone</a>`,
		`<a href="mcp/">MCP</a>`:                  `<a href="` + codeHostTree + `guide/mcp" target="_blank" rel="noopener">MCP</a>`,
		`<a href="https://example.test/">out</a>`: `<a href="https://example.test/">out</a>`,
		`<a href="#section">here</a>`:             `<a href="#section">here</a>`,
	}
	for link, want := range body {
		if got := rewriteDocLinks(link, docRel, manifest, from); got != want {
			t.Errorf("%s became\n  %s\nwant\n  %s", link, got, want)
		}
	}

	mirror := map[string]string{
		"[RPKI](rpki.md)":              "[RPKI](../rpki/index.md)",
		"[caches](rpki.md#caches)":     "[caches](../rpki/index.md#caches)",
		"[gone](unpublished.md)":       "[gone](" + codeHostBlob + "guide/unpublished.md)",
		"[MCP](mcp/)":                  "[MCP](" + codeHostTree + "guide/mcp)",
		"[out](https://example.test/)": "[out](https://example.test/)",
		"[picture](img/graph.png)":     "[picture](img/graph.png)",
	}
	for link, want := range mirror {
		if got := rewriteDocLinksMarkdown(link, docRel, manifest, from); got != want {
			t.Errorf("%s became\n  %s\nwant\n  %s", link, got, want)
		}
	}
}

// TestAJourneyHeroWrapsTheTitleAndItsLead covers the hero block a page opens
// with, including the two shapes the retired renderer recognized and the one
// it left alone.
func TestAJourneyHeroWrapsTheTitleAndItsLead(t *testing.T) {
	withLead := wrapJourneyHero("<h1 id=\"t\">Title</h1>\n<p>The lead.</p>\n<p>Body.</p>", "Guide")
	want := "<div class=\"journey-hero reveal\">\n    <span class=\"journey-eyebrow\">Guide</span>\n" +
		"    <h1 id=\"t\">Title</h1>\n    <p>The lead.</p>\n</div>\n<p>Body.</p>"
	if withLead != want {
		t.Errorf("the hero must carry the title and its lead:\n%s\nwant\n%s", withLead, want)
	}

	titleOnly := wrapJourneyHero("<h1>Title</h1>\n<ul><li>a</li></ul>", "Guide")
	if strings.Contains(titleOnly, "<p>") || !strings.Contains(titleOnly, "<ul>") {
		t.Errorf("a page with no lead paragraph must take a hero with no paragraph:\n%s", titleOnly)
	}

	const noHeading = "<p>This page opens with prose.</p>"
	if got := wrapJourneyHero(noHeading, "Guide"); got != noHeading {
		t.Errorf("a page that opens with no heading has no title to present: %s", got)
	}
}

// TestANumberTokenResolvesFromTheFactsSnapshot covers the {{ze:...}} prose
// tokens: the plain value for a mirror, the marked span for the HTML, the
// refusal of a token this site has no key for, and the build that has no
// snapshot yet.
func TestANumberTokenResolvesFromTheFactsSnapshot(t *testing.T) {
	tokens := numberTokens{"unit-tests": "23,700+"}
	plain, err := tokens.substitute("Ze runs {{ze:unit-tests}} unit tests.", false)
	if err != nil || plain != "Ze runs 23,700+ unit tests." {
		t.Errorf("the mirror takes the plain number: %q %v", plain, err)
	}
	marked, err := tokens.substitute("Ze runs {{ze:unit-tests}} unit tests.", true)
	if err != nil || marked != `Ze runs <span data-ze-stat="tests.unit_display">23,700+</span> unit tests.` {
		t.Errorf("the page takes the marked number: %q %v", marked, err)
	}
	if _, err := tokens.substitute("Ze has {{ze:invented-count}} of them.", false); err == nil {
		t.Error("a token this site has no key for can never resolve and must be refused")
	}
	nothing, err := numberTokens(nil).substitute("Ze runs {{ze:unit-tests}} tests.", false)
	if err != nil || nothing != "Ze runs {{ze:unit-tests}} tests." {
		t.Errorf("a build with no snapshot leaves the token alone: %q %v", nothing, err)
	}
}

// TestPageMetadataIsRefusedRatherThanGuessed covers the two front matter
// fields a source can get wrong, where guessing publishes the wrong page.
func TestPageMetadataIsRefusedRatherThanGuessed(t *testing.T) {
	if _, err := pageCategory(sitePage{}, map[string]string{"category": "networking"}); err == nil {
		t.Error("a category the site has no color for must be refused")
	}
	category, err := pageCategory(sitePage{Category: categorySecure}, map[string]string{"category": categoryRouting})
	if err != nil || category != categorySecure {
		t.Errorf("the registry's category wins over the source's: %q %v", category, err)
	}
	if _, err := tableColumnsEnabled(map[string]string{tableColumnsKey: "nope"}); err == nil {
		t.Error("a flag value the vocabulary does not name must be refused, not read as false")
	}
	off, err := tableColumnsEnabled(map[string]string{tableColumnsKey: "off"})
	if err != nil || off {
		t.Errorf("`off` turns the flag off: %v %v", off, err)
	}
	fallback, err := tableColumnsEnabled(map[string]string{})
	if err != nil || !fallback {
		t.Errorf("a source that states nothing keeps the column controls: %v %v", fallback, err)
	}
}
