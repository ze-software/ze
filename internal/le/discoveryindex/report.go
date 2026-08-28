// Design: docs/architecture/core-design.md -- what the package-map generator answers
// Overview: discoveryindex.go -- the walk that fills this in
//
// report.go holds what `le discovery-index` ANSWERS, apart from what produced
// it.
//
// The payload carries the map ITSELF, not its count. The index is the answer
// that a machine reader requests with `| json`. One key holds the rows so row
// operators act on the packages. Stale and Written are separate facts. A check
// reports a stale file without a write. An update reports the stale file that
// it rewrote.

package discoveryindex

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ErrNoAIDir says the tree holds no ai/ directory, so there is nowhere for the
// index to live. It is a property of the TREE, which a caller reads apart from
// an index that is out of date.
var ErrNoAIDir = errors.New("discoveryindex: the tree holds no ai/ directory")

// Report is the whole answer of one run.
type Report struct {
	File     string    `json:"file"`
	Packages []Package `json:"packages"`
	Todo     int       `json:"todo"`
	Stale    bool      `json:"stale"`
	Written  bool      `json:"written"`
}

// Text renders the verdict for a person, in the words the script prints. It
// ends in a newline.
//
// This is the Prose rendering leroot uses for the bare command, and every pipe
// operator bypasses it.
func (r Report) Text() string {
	var tb textbuf.Buffer
	switch {
	case r.Written:
		tb.Str("wrote ").Str(r.File).Str(" (").Int(int64(len(r.Packages))).Str(" packages)")
	case r.Stale:
		tb.Str("WARNING: ").Str(r.File).Str(" is stale -- run: ./le discovery-index update")
	default:
		tb.Str("checked ").Int(int64(len(r.Packages))).Str(" packages, ").Str(r.File).Str(" up to date")
	}
	return tb.Byte('\n').String()
}

// Check reads the tree and reports whether the committed index still matches
// it. It writes nothing.
func Check(root string) (Report, error) {
	report, content, err := survey(root)
	if err != nil {
		return Report{}, err
	}

	current, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(OutputRel))) //nolint:gosec // the index of the tree the caller named
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		// An index that is present and unreadable is not an index that is
		// missing. Reporting it says which of the two happened, where the
		// Python half read both as "no current content" and answered stale.
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

	packages, err := Build(root)
	if err != nil {
		return Report{}, "", err
	}

	todo := 0
	for _, pkg := range packages {
		if pkg.Responsibility == "TODO" {
			todo++
		}
	}
	return Report{File: OutputRel, Packages: packages, Todo: todo}, Render(packages), nil
}
