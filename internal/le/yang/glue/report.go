// Design: docs/architecture/command-ownership.md -- what the yang-glue commands answer
//
// report.go holds what the two actions ANSWER, apart from what produced it.
//
// Each answer carries one row set, so `| json` feeds a script, `| match embed`
// keeps one half of the pair and `| count` says how many. Each also renders
// ITSELF, because the script printed a verdict rather than a table and the
// verdict is what a person reads (internal/le/leroot, Prose).
//
// The rendering is the script's, word for word, with one deliberate change: a
// stale file is named RELATIVE to the tree rather than by its absolute path.
// The absolute form names this machine's checkout as much as it names the file,
// and it would leave `| json` and the default rendering disagreeing about what
// the value is (ai/rules/cli.md). internal/le/parity/parity_test.go normalizes it.

package yangglue

import (
	"github.com/ze-software/ze/internal/core/textbuf"
)

// CheckReport is what `le yang-glue check` answers: how many schema packages
// were read, and every generated file that disagrees with the .yang files
// beside it.
type CheckReport struct {
	// Dirs is how many yang/ directories the walk read. A run that read none
	// has proven nothing, which is why the verdict names the number.
	Dirs int `json:"dirs"`
	// Stale names every generated file that must be rewritten, relative to the
	// tree, in walk order. It is the answer's only row set.
	Stale []string `json:"stale"`
}

// Text renders the native check verdict. It ends in a newline.
func (r CheckReport) Text() string {
	var tb textbuf.Buffer

	if r.Dirs == 0 {
		return "./le yang-glue check: no yang/ directories with .yang files found\n"
	}

	if len(r.Stale) == 0 {
		return tb.Str("./le yang-glue check: ").Int(int64(r.Dirs)).Str(" yang/ directories are current\n").String()
	}

	for _, file := range r.Stale {
		tb.Str("stale: ").Str(file).Byte('\n')
	}
	tb.Str("./le yang-glue check: ").Int(int64(len(r.Stale))).Str(" generated files are stale; run ./le yang-glue write\n")

	return tb.String()
}

// WriteReport is what `le yang-glue write` answers: how many schema packages
// were read, and every generated file whose bytes changed.
type WriteReport struct {
	// Dirs is how many yang/ directories the walk read.
	Dirs int `json:"dirs"`
	// Written names every file this run rewrote, relative to the tree. A file
	// whose bytes already agreed is absent, because it was not written.
	Written []string `json:"written"`
}

// Text renders the native write verdict. It ends in a newline.
func (r WriteReport) Text() string {
	var tb textbuf.Buffer

	if r.Dirs == 0 {
		return "./le yang-glue write: no yang/ directories with .yang files found\n"
	}

	return tb.Str("./le yang-glue write: generated glue for ").Int(int64(r.Dirs)).Str(" yang/ directories\n").String()
}
