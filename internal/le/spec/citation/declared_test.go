// VALIDATES: DeclaredDesignDocuments answers every page of every `// Design:`
// line of a header, and only the header's
// (spec-design-needs-owner-approval, Implementation
// Steps item 4).
// PREVENTS: a file that links two design pages owing a verdict on the first
// alone, which is what reading the first match only did.
package speccitation

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestDeclaredDesignDocumentsReadsEveryHeaderLine is the header rule. Method:
// a file with two `// Design:` lines, the second naming two pages, and one more
// line after the package clause; the answer holds the three header pages in
// order and not the fourth. A file the spec creates (absent) answers nothing
// with no error.
func TestDeclaredDesignDocumentsReadsEveryHeaderLine(t *testing.T) {
	root := t.TempDir()
	const file = "internal/component/widget/widget.go"
	header := "// Design: docs/architecture/widget.md -- the widget\n" +
		"// Design: docs/architecture/a.md, docs/architecture/b.md -- two more\n" +
		"// Related: other.go -- not a page\n\npackage widget\n\n" +
		"// Design: docs/architecture/late.md -- after the package clause\n"
	target := filepath.Join(root, filepath.FromSlash(file))
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(header), 0o600); err != nil {
		t.Fatal(err)
	}

	pages, err := DeclaredDesignDocuments(root, file)
	if err != nil {
		t.Fatalf("DeclaredDesignDocuments: %v", err)
	}
	want := []string{"docs/architecture/widget.md", "docs/architecture/a.md", "docs/architecture/b.md"}
	if !slices.Equal(pages, want) {
		t.Fatalf("pages = %q, want %q", pages, want)
	}

	absent, err := DeclaredDesignDocuments(root, "internal/component/widget/new.go")
	if err != nil {
		t.Fatalf("an absent file answered an error: %v", err)
	}
	if len(absent) != 0 {
		t.Fatalf("an absent file answered pages %q", absent)
	}
}
