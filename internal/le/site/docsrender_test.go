// Design: website/AI.md -- a page the docs producer publishes reads as the page it replaces
package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// renderOnePage publishes one page of the real checkout into a temporary
// artifact and answers the page and its Markdown mirror.
//
// The sources are the repository's own, not a fixture, because the parity
// target is the page the retired renderer made FROM THOSE SOURCES. The output
// is temporary, so nothing this test does can reach the published site.
func renderOnePage(t *testing.T, page sitePage) (html, mirror string) {
	t.Helper()
	root := repositoryRoot(t)
	paths := Paths{Repository: root, Source: filepath.Join(root, "website"), Output: t.TempDir()}
	renderer, err := newDocsRenderer(paths)
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	if err := renderer.render(page); err != nil {
		t.Fatalf("render %s: %v", page.Source, err)
	}
	published := filepath.Join(paths.Output, filepath.FromSlash(page.Dest))
	content, err := os.ReadFile(published)
	if err != nil {
		t.Fatalf("read the published page: %v", err)
	}
	sibling, err := os.ReadFile(filepath.Join(filepath.Dir(published), pageMirrorFile))
	if err != nil {
		t.Fatalf("read the published mirror: %v", err)
	}
	return string(content), string(sibling)
}

// anomalyPage is the manifest row the parity fixtures were taken from:
// docs/guide/anomaly.md, published at guides/anomaly/.
var anomalyPage = sitePage{
	Source:   "docs/guide/anomaly.md",
	Dest:     "guides/anomaly/" + pageIndexFile,
	Desc:     "Ze documentation: guide/anomaly.md",
	Category: categorySecure,
	DocRel:   "guide/anomaly.md",
}

// TestDocsProducerRendersAManifestRoute is the Wiring Test row for the docs
// pipeline, and the evidence for AC-4.
//
// It renders one manifest row end to end and compares the whole page against
// the page the retired renderer published from the same source: the words a
// reader sees, in order, and the target of every link. The acceptance target
// is the RENDERED page, so escaping, whitespace and anchor slugs are permitted
// to differ; a word or a link target is not.
func TestDocsProducerRendersAManifestRoute(t *testing.T) {
	page, mirror := renderOnePage(t, anomalyPage)
	published := readTestdata(t, "published-guides-anomaly.html")

	content, err := extractMain(page)
	if err != nil {
		t.Fatalf("extract main from the rendered page: %v", err)
	}
	reference, err := extractMain(published)
	if err != nil {
		t.Fatalf("extract main from the published page: %v", err)
	}
	wantWords, wantLinks := readablePage(t, reference)
	gotWords, gotLinks := readablePage(t, content)
	if strings.Join(wantWords, " ") != strings.Join(gotWords, " ") {
		t.Errorf("the page reads differently:\n%s", firstDifference(
			strings.Join(wantWords, "\n"), strings.Join(gotWords, "\n")))
	}
	if len(wantLinks) != len(gotLinks) {
		t.Fatalf("the page must carry %d links, not %d", len(wantLinks), len(gotLinks))
	}
	for index, want := range wantLinks {
		got := gotLinks[index]
		if want.label != got.label {
			t.Errorf("link %d must read %q, not %q", index, want.label, got.label)
		}
		if !sameLinkTarget(want.href, got.href) {
			t.Errorf("link %d must reach %q, not %q", index, want.href, got.href)
		}
	}

	// The chrome AC-3 names, on a page a producer wrote rather than on a
	// shell built by hand.
	for _, chrome := range []string{
		`<link rel="canonical" href="https://ze-software.net/guides/anomaly/"`,
		`Anomaly Detection - Ze</title>`,
		`<section class="md-content reveal cat-secure">`,
		`<span class="journey-eyebrow">Guide</span>`,
		`<nav class="doc-toc" aria-labelledby="doc-toc-title">`,
		`class="has-page-sidebar"`,
		`<aside class="page-sidebar"`,
	} {
		if !strings.Contains(page, chrome) {
			t.Errorf("the published page must carry %s", chrome)
		}
	}

	// AC-5: the mirror sits beside the page and reads as the published one.
	wantMirror := readTestdata(t, "published-guides-anomaly.md")
	if normalizeMirror(wantMirror) != normalizeMirror(mirror) {
		t.Errorf("the Markdown mirror reads differently:\n%s",
			firstDifference(normalizeMirror(wantMirror), normalizeMirror(mirror)))
	}
}

// sameLinkTarget reports whether two link targets reach one place.
//
// A fragment is compared by its presence rather than by its text: goldmark
// slugifies a heading differently from the retired renderer, and the owner
// accepted that on 2026-08-29 because the page's own contents list is built
// from the same slugifier and stays self-consistent.
func sameLinkTarget(want, got string) bool {
	wantPath, wantFragment, wantHas := strings.Cut(want, "#")
	gotPath, gotFragment, gotHas := strings.Cut(got, "#")
	if wantPath != gotPath || wantHas != gotHas {
		return false
	}
	return (wantFragment == "") == (gotFragment == "")
}

// normalizeMirror answers a mirror with the differences a reader cannot see
// removed: trailing spaces, and blank lines at either end.
func normalizeMirror(mirror string) string {
	lines := strings.Split(mirror, "\n")
	for index, line := range lines {
		lines[index] = strings.TrimRight(line, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// TestEveryPageTakesTheEyebrowThePublishedPageCarries covers the label
// derivation over the whole population rather than over one page.
//
// The eyebrow comes from four different rules -- the registry states it, the
// source family implies it, the published section implies it, or the page key
// is title-cased -- and a page landing in the wrong branch is a visible change
// no single-page test would find. The published pages are the answer key.
func TestEveryPageTakesTheEyebrowThePublishedPageCarries(t *testing.T) {
	labels := publishedEyebrows(t)
	pages, err := docsProducerPages()
	if err != nil {
		t.Fatalf("pages: %v", err)
	}
	for _, page := range pages {
		want, published := labels[page.Dest]
		if !published {
			t.Errorf("%s has no published eyebrow to check against", page.Dest)
			continue
		}
		metadata, _, err := parseFrontMatter(readSource(t, page.Source))
		if err != nil {
			t.Errorf("%s: front matter: %v", page.Source, err)
			continue
		}
		if got := journeyLabel(page, metadata); got != want {
			t.Errorf("%s takes the eyebrow %q, and the published page reads %q", page.Dest, got, want)
		}
	}
}

// readSource answers one page source of the checkout these tests read.
func readSource(t *testing.T, source string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(repositoryRoot(t), filepath.FromSlash(source)))
	if err != nil {
		t.Fatalf("read %s: %v", source, err)
	}
	return content
}

// publishedEyebrows answers the eyebrow each of this producer's 148 pages
// carried at gh-pages 2fa8fa2ad, the last commit the retired renderers made.
//
// It is committed rather than read from the sibling worktree so the answer key
// is the PUBLISHED page rather than whatever a later build left there.
func publishedEyebrows(t *testing.T) map[string]string {
	t.Helper()
	labels := map[string]string{}
	for line := range strings.SplitSeq(strings.TrimSpace(readTestdata(t, "published-eyebrows.txt")), "\n") {
		destination, label, separated := strings.Cut(line, "\t")
		if !separated || label == "" {
			t.Fatalf("the eyebrow fixture line %q must read `dest<TAB>label`", line)
		}
		labels[destination] = label
	}
	if len(labels) != 148 {
		t.Fatalf("the eyebrow fixture names %d pages, want the producer's 148", len(labels))
	}
	return labels
}

// TestACodeSpanHoldingAPipeStaysInOneTableCell is the source convention the
// renderer change made necessary, checked on the real page that carries it.
//
// GFM splits a table row on every unescaped pipe, inside a code span or not,
// so `rpf-check strict|loose|disable` written bare in a table breaks the cell
// in three and shows the reader the backticks. The retired renderer did not
// split, so the sources were written bare. The fix is `\|` in the SOURCE,
// which is one-time, rather than a renderer that stops following GFM, which
// would be permanent.
//
// The escape belongs ONLY inside a table. docs/features.md carries two HTML
// comments on lines of their own, which end the table, so every row after them
// is ordinary paragraph text where a backslash before a pipe is published to
// the reader. Both halves are checked here, because escaping too much is as
// visible as escaping too little.
func TestACodeSpanHoldingAPipeStaysInOneTableCell(t *testing.T) {
	page, _ := renderOnePage(t, sitePage{
		Source: "docs/features.md",
		Dest:   "reference/feature-status/" + pageIndexFile,
		Desc:   "Ze documentation: features.md",
		DocRel: "features.md",
	})
	for _, span := range []string{
		"<code>rpf-check strict|loose|disable</code>",
		"<code>| log | resolve</code>",
		"<code>| first N</code>",
		"<code>clear firewall irr asn|as-set</code>",
	} {
		if !strings.Contains(page, span) {
			t.Errorf("the published page must carry %s whole", span)
		}
	}
	if strings.Contains(page, `\|`) {
		t.Errorf("no published page shows a reader a backslash before a pipe")
	}
}
