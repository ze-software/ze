// Design: docs/architecture/core-design.md -- what the package-map generator answers
// Overview: discoveryindex.go -- the walk that fills this in
//
// report.go holds what `le discovery-index` ANSWERS, apart from what produced
// it.
//
// The payload carries the map ITSELF, not its count. The index is the answer
// that a machine reader requests with `| json`. One key holds the rows so row
// operators act on the packages.

package discoveryindex

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/derived"
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
	Written  bool      `json:"written"`
}

// Text renders the verdict for a person, in the words the script prints. It
// ends in a newline.
//
// This is the Prose rendering leroot uses for the bare command, and every pipe
// operator bypasses it. Update is the only writer, so Written is the only
// state: the map is DERIVED, and nothing compares it against a stored copy.
func (r Report) Text() string {
	var tb textbuf.Buffer
	tb.Str("wrote ").Str(r.File).Str(" (").Int(int64(len(r.Packages))).Str(" packages)")
	return tb.Byte('\n').String()
}

// Update reads the tree and rewrites the index from it.
func Update(root string) (Report, error) {
	report, content, err := survey(root)
	if err != nil {
		return Report{}, err
	}

	out := filepath.Join(root, filepath.FromSlash(OutputRel))
	if err := derived.WriteAtomic(out, []byte(content)); err != nil {
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
