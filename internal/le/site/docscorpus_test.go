// Design: website/AI.md -- every page of the docs population publishes
package site

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestDocsProducerPublishesEveryPage renders the whole population, which one
// page fixture cannot do.
//
// It asserts what a producer owes for every page it claims: the page is
// written, it carries the shared chrome, and its Markdown mirror sits beside
// it. A defect that reaches one source out of a hundred and fifty is invisible
// to a fixture test and visible here.
//
// One failure is permitted and only one: a page that shows a recorded
// demonstration cannot publish in a checkout that has not rendered its media,
// because the media is generated and is not tracked. Any other error, and any
// failure on a page that shows no demonstration, is a defect.
func TestDocsProducerPublishesEveryPage(t *testing.T) {
	root := repositoryRoot(t)
	output := t.TempDir()
	paths := Paths{Repository: root, Source: filepath.Join(root, "website"), Output: output}
	renderer, err := newDocsRenderer(paths)
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	pages, err := docsProducerPages()
	if err != nil {
		t.Fatalf("pages: %v", err)
	}
	published, waiting := 0, 0
	for _, page := range pages {
		showsDemo := demoMarker.Match(readSource(t, page.Source))
		err := renderer.render(page)
		if errors.Is(err, errDemoMediaAbsent) {
			if !showsDemo {
				t.Errorf("%s shows no recorded demonstration and must not wait for one", page.Source)
			}
			waiting++
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", page.Source, err)
			continue
		}
		published++
		assertPagePublished(t, output, page)
	}
	if published+waiting != len(pages) {
		t.Fatalf("%d pages published and %d waited for media, and the producer names %d",
			published, waiting, len(pages))
	}
	// The floor keeps the loop above from passing by never running. It is a
	// floor rather than the exact count because a source that stops showing a
	// demonstration should not turn this test red.
	if published < 100 {
		t.Fatalf("only %d pages published; %d wait for terminal demonstration media", published, waiting)
	}
	t.Logf("%d pages published, %d waiting for terminal demonstration media", published, waiting)
}

// headingAnchor matches one heading id the rendered body carries.
var headingAnchor = regexp.MustCompile(`href="#([^"]+)"`)

// assertPagePublished checks what every published page owes: the chrome the
// shell writes, a body, and the Markdown mirror beside it.
func assertPagePublished(t *testing.T, output string, page sitePage) {
	t.Helper()
	path := filepath.Join(output, filepath.FromSlash(page.Dest))
	content, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("%s: %v", page.Dest, err)
		return
	}
	rendered := string(content)
	for _, chrome := range []string{
		"<!doctype html>",
		`<link rel="canonical" href="` + pageCanonicalURL(page.Dest) + `"`,
		`<section class="md-content reveal`,
		`<div class="journey-hero reveal">`,
		"</main>",
	} {
		if !strings.Contains(rendered, chrome) {
			t.Errorf("%s must carry %s", page.Dest, chrome)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), pageMirrorFile)); err != nil {
		t.Errorf("%s has no Markdown mirror beside it: %v", page.Dest, err)
	}

	// A backslash before a pipe is a table-cell escape, and GFM removes it
	// only inside a table. Written anywhere else it reaches the reader, which
	// is what happened on reference/feature-status: two HTML comments on lines
	// of their own end the features table, and the eight escaped pipes below
	// them were published with their backslashes.
	if strings.Contains(rendered, `\|`) {
		t.Errorf("%s shows a reader a backslash before a pipe, which only a table cell removes", page.Dest)
	}

	// A contents entry pointing at an id the body does not carry is a link a
	// reader clicks and nothing happens.
	body, err := extractMain(rendered)
	if err != nil {
		t.Errorf("%s: extract main: %v", page.Dest, err)
		return
	}
	contents, _, found := strings.Cut(body, "</nav>")
	if !found || !strings.Contains(contents, `class="doc-toc"`) {
		return
	}
	for _, anchor := range headingAnchor.FindAllStringSubmatch(contents, -1) {
		if anchor[1] == "doc-toc-title" {
			continue
		}
		if !strings.Contains(body, `id="`+anchor[1]+`"`) {
			t.Errorf("%s: the contents list links #%s, which the page does not carry", page.Dest, anchor[1])
		}
	}
}
