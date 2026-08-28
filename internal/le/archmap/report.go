// Design: docs/architecture/core-design.md -- what the architecture-list generator answers
// Overview: archmap.go -- the run that fills this in
//
// report.go holds what `le arch-map` ANSWERS, apart from what produced it.
//
// The payload is an object because a reader needs three facts the page states
// in one sentence: which file was judged, what each generated block now holds,
// and whether the file was stale and whether it was rewritten. One key holds
// rows, so the row operators act on the blocks.

package archmap

import "github.com/ze-software/ze/internal/core/textbuf"

// Block is one generated list, and it is one ROW of the report.
type Block struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Directories int    `json:"directories"`
}

// Report is the whole answer of one run. Stale and Written are separate facts:
// check answers a stale file and writes nothing, and update answers a stale
// file it has just rewritten.
type Report struct {
	File    string  `json:"file"`
	Blocks  []Block `json:"blocks"`
	Stale   bool    `json:"stale"`
	Written bool    `json:"written"`
}

// Text renders the verdict for a person, in the words the script printed. It
// ends in a newline.
//
// This is the Prose rendering leroot uses for the bare command, and every pipe
// operator bypasses it.
func (r Report) Text() string {
	var tb textbuf.Buffer
	tb.Str(r.File).Str(": architecture lists ")

	switch {
	case r.Written:
		tb.Str("regenerated")
	case r.Stale:
		tb.Str("are stale -- run: ./le arch-map update")
	default:
		tb.Str("up to date")
	}
	return tb.Byte('\n').String()
}
