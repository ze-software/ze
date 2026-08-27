// Design: ai/rules/git-safety.md -- what the working-tree advisory answers
// Overview: workingtree.go -- the grouping that fills this in
//
// report.go holds what `le working-tree` ANSWERS, apart from what produced it.
//
// The payload is an object because the verdict needs three facts a reader must
// not have to count: how many paths are in flight, how they group, and whether
// a ceiling was asked for. One key holds rows, so the row operators act on the
// areas.

package workingtree

import "github.com/ze-software/ze/internal/core/textbuf"

// Area is one group of changed paths, and it is one ROW of the report.
type Area struct {
	Area  string   `json:"area"`
	Files []string `json:"files"`
}

// Report is the whole answer of one run.
type Report struct {
	Paths int    `json:"paths"`
	Areas []Area `json:"areas"`
	// MaxAreas is the ceiling the caller asked for, and 0 means the run is
	// advisory. It is in the payload because the verdict cannot be read
	// without it: three areas is a failure under a ceiling of two and a
	// remark under none.
	MaxAreas int `json:"max-areas"`
}

// The page names this many files of an area before it summarizes the rest, and
// pads the area name to this width so the counts line up.
const (
	namedFiles = 4
	areaWidth  = 16
	countWidth = 3
)

// Exceeded reports whether a ceiling was asked for and passed. It is the
// tool's verdict, so the command's exit code reads it rather than re-deriving
// it from the page.
func (r Report) Exceeded() bool {
	return r.MaxAreas > 0 && len(r.Areas) > r.MaxAreas
}

// Text renders the report for a person, in the words the advisory printed
// before it was a command. It ends in a newline.
//
// This is the Prose rendering leroot uses for the bare command, and every pipe
// operator bypasses it.
func (r Report) Text() string {
	var tb textbuf.Buffer

	if r.Paths == 0 {
		return tb.Str("working tree: clean\n").String()
	}

	tb.Str("working tree: ").Int(int64(r.Paths)).Str(" path(s) across ").
		Int(int64(len(r.Areas))).Str(" area(s)\n")
	for _, area := range r.Areas {
		tb.Str("  ").PadRight(area.Area, areaWidth).Byte(' ').
			PadLeft(countOf(len(area.Files)), countWidth).Str("  ")
		named := area.Files
		unnamed := 0
		if len(named) > namedFiles {
			unnamed = len(named) - namedFiles
			named = named[:namedFiles]
		}
		tb.Join(named, ", ")
		if unnamed > 0 {
			tb.Str(", ").Byte('+').Int(int64(unnamed)).Str(" more")
		}
		tb.Byte('\n')
	}

	if len(r.Areas) > 1 {
		tb.Byte('\n')
		tb.Str("More than one area is in flight. Land the chunks that are already\n")
		tb.Str("finished before starting the next piece (ai/rules/git-safety.md,\n")
		tb.Str("\"Commit Granularity\"). Areas are a hint, not a verdict: a feature\n")
		tb.Str("and its tests are one change even when they sit in two of them.\n")
	}

	if r.Exceeded() {
		tb.Byte('\n')
		tb.Str("working-tree-check: ").Int(int64(len(r.Areas))).
			Str(" areas exceeds max-areas ").Int(int64(r.MaxAreas)).Byte('\n')
	}
	return tb.String()
}

// countOf renders a file count for the padded column. PadLeft takes a string,
// and this is the only place a number is right-aligned.
func countOf(n int) string {
	var tb textbuf.Buffer
	return tb.Int(int64(n)).String()
}
