package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
)

// TestEveryDocsSourceRendersAndItsContentsResolve runs the goldmark pipeline
// over the whole real corpus, which one page fixture cannot do.
//
// It asserts two things a producer depends on and a single-page test cannot
// see: no source in docs/ fails to render, and every entry of a page's contents
// list points at an id that page's body carries. A contents entry with no
// target is a link a reader clicks and nothing happens, and it would appear on
// one page out of two hundred.
func TestEveryDocsSourceRendersAndItsContentsResolve(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	docs := filepath.Join(root, "docs")
	var pages, anchors int
	walkErr := filepath.WalkDir(docs, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			return walkErr
		}
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		relative, _ := filepath.Rel(root, path)
		_, body, frontErr := parseFrontMatter(source)
		if frontErr != nil {
			t.Errorf("%s: front matter: %v", relative, frontErr)
			return nil
		}
		rendered, headings, renderErr := renderMarkdown(body)
		if renderErr != nil {
			t.Errorf("%s: render: %v", relative, renderErr)
			return nil
		}
		pages++
		if strings.TrimSpace(rendered) == "" && strings.TrimSpace(string(body)) != "" {
			t.Errorf("%s: a source with text rendered an empty body", relative)
		}
		for _, heading := range headings {
			if heading.Level < 2 {
				continue
			}
			// The id is asserted before it is used. Without this line the
			// loop below is vacuous: a pipeline that stopped giving headings
			// an id would skip every one of them and the test would pass.
			if heading.ID == "" {
				t.Errorf("%s: the heading %q carries no id, so no contents entry can reach it", relative, heading.Label)
				continue
			}
			anchors++
			if !strings.Contains(rendered, `id="`+heading.ID+`"`) {
				t.Errorf("%s: the contents list links #%s, which the body does not carry", relative, heading.ID)
			}
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk %s: %v", docs, walkErr)
	}
	if pages < 100 {
		t.Fatalf("the corpus must hold the documentation pages; walked only %d", pages)
	}
	// The anchor floor is what makes the loop above mean something. A pipeline
	// that stopped giving headings an id would answer no heading at all, so
	// every assertion inside the loop would pass by never running.
	if anchors < 1000 {
		t.Fatalf("the corpus must yield the section anchors a contents list links; got %d over %d pages", anchors, pages)
	}
}
