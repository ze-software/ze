// Design: docs/architecture/core-design.md -- what the docs-to-code generator answers
// Overview: docstocode.go -- the walk that fills this in
//
// report.go holds what `le docs-to-code` ANSWERS, apart from what produced it,
// and the rendering of the file itself.
//
// The payload carries the index ITSELF, not its count. The index is the answer
// that a machine reader requests with `| json`. One key holds the rows so row
// operators act on references.
//
// Stale, Written, and Generated are separate.
// A check reports a stale file without a write. An update reports the file that
// it rewrote. A check generates a file that was absent.

package docstocode

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ErrNoAIDir says the tree holds no ai/ directory, so there is nowhere for the
// index to live. It is a property of the TREE, which a caller reads apart from
// an index that is out of date.
var ErrNoAIDir = errors.New("docstocode: the tree holds no ai/ directory")

// Report is the whole answer of one run.
type Report struct {
	File       string      `json:"file"`
	Docs       int         `json:"docs"`
	References []Reference `json:"references"`
	Stale      bool        `json:"stale"`
	Written    bool        `json:"written"`
	Generated  bool        `json:"generated"`
}

// Text renders the verdict for a person, in the words the script prints. It
// ends in a newline.
//
// This is the Prose rendering leroot uses for the bare command, and every pipe
// operator bypasses it.
func (r Report) Text() string {
	var tb textbuf.Buffer
	switch {
	case r.Generated:
		tb.Str("generated ").Str(r.File).Str(" (").Int(int64(r.Docs)).
			Str(" design docs); it is derived and not tracked")
	case r.Written:
		tb.Str("wrote ").Str(r.File).Str(" (").Int(int64(r.Docs)).Str(" design docs)")
	case r.Stale:
		tb.Str("WARNING: ").Str(r.File).Str(" is stale -- run: ./le docs-to-code update")
	default:
		tb.Str("checked ").Int(int64(r.Docs)).Str(" design docs, ").Str(r.File).Str(" up to date")
	}
	return tb.Byte('\n').String()
}

// Check reads the tree and reports whether the index still matches it.
//
// The index is generated and gitignored, so clean checkouts do not contain it.
// Verify worktrees, fresh clones, and from-scratch CI runs all start without
// it. A missing index is not STALE. A stale report for it failed the doc gate
// on clean checkouts but passed after a user ran the generator. Thus, an absent
// index is WRITTEN and reported as generated. Stale still means that an
// existing index differs.
func Check(root string) (Report, error) {
	report, content, err := survey(root)
	if err != nil {
		return Report{}, err
	}

	out := filepath.Join(root, filepath.FromSlash(OutputRel))
	current, err := os.ReadFile(out) //nolint:gosec // the index of the tree the caller named
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := os.WriteFile(out, []byte(content), 0o600); err != nil {
			return Report{}, err
		}
		report.Generated = true
		return report, nil
	case err != nil:
		// A present unreadable index is not missing. Reporting the error
		// distinguishes the two states. Treating both as "no current content"
		// would silently rewrite a file that this run cannot judge.
		return Report{}, err
	}
	report.Stale = string(current) != content
	return report, nil
}

// Update reads the tree and rewrites the index from it.
func Update(root string) (Report, error) {
	report, content, err := survey(root)
	if err != nil {
		return Report{}, err
	}

	out := filepath.Join(root, filepath.FromSlash(OutputRel))
	if err := os.WriteFile(out, []byte(content), 0o600); err != nil {
		return Report{}, err
	}
	report.Written = true
	return report, nil
}

// survey walks the tree once and answers the report both halves start from,
// plus the file content that report describes.
func survey(root string) (Report, string, error) {
	info, err := os.Stat(filepath.Join(root, "ai"))
	if err != nil || !info.IsDir() {
		return Report{}, "", ErrNoAIDir
	}

	refs, err := Build(root)
	if err != nil {
		return Report{}, "", err
	}
	return Report{File: OutputRel, Docs: countDocs(refs), References: refs}, Render(refs), nil
}

// countDocs answers how many distinct design documents the references name. It
// is the number both halves print, so it is derived from the same slice the
// rendering walks.
func countDocs(refs []Reference) int {
	docs := 0
	previous := ""
	for i, ref := range refs {
		if i == 0 || ref.Doc != previous {
			docs++
			previous = ref.Doc
		}
	}
	return docs
}

// Render answers the whole file the index holds, ending in a newline.
//
// A short reference list uses bullets, and a longer list uses a table. A column
// makes the longer list easier to scan than sentences. Only the table escapes a
// bar because a bar would otherwise end the cell.
func Render(refs []Reference) string {
	var tb textbuf.Buffer
	tb.Str("# Documentation to Code Index\n\n")
	tb.Str("<!-- GENERATED by ./le docs-to-code update -- do not edit -->\n")
	tb.Str("<!-- Regenerate: ./le docs-to-code update -->\n\n")
	tb.Str("Given a design doc, the `.go` files that cite it in their `// Design:`\n")
	tb.Str("header. The inverse of `ai/CODE-TO-DOCS.md` (which is built from doc-side\n")
	tb.Str("`<!-- source: -->` anchors). See `ai/rules/go-standards.md`.\n")

	for start := 0; start < len(refs); {
		end := start
		for end < len(refs) && refs[end].Doc == refs[start].Doc {
			end++
		}
		renderDoc(&tb, refs[start].Doc, refs[start:end])
		start = end
	}
	return tb.String()
}

// renderDoc writes one design document's section.
//
// The blank line between sections starts a section instead of ending the
// previous one. Thus, one newline follows the final reference, with no trailing
// blank line. The header omits that blank for the same reason.
func renderDoc(tb *textbuf.Buffer, doc string, refs []Reference) {
	tb.Str("\n## `").Str(doc).Str("`\n\n")

	if len(refs) <= InlineRefs {
		for _, ref := range refs {
			tb.Str("- `").Str(ref.File).Byte('`')
			if ref.Topic != "" {
				tb.Str(" -- ").Str(ref.Topic)
			}
			tb.Byte('\n')
		}
		return
	}

	tb.Str("| File | Topic |\n")
	tb.Str("|------|-------|\n")
	for _, ref := range refs {
		tb.Str("| `").Str(ref.File).Str("` | ").Str(escapePipes(ref.Topic)).Str(" |\n")
	}
}

// escapePipes protects a table cell from a topic holding a bar.
func escapePipes(text string) string {
	return strings.ReplaceAll(text, "|", `\|`)
}
