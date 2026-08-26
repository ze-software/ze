// Design: docs/architecture/core-design.md -- what a move answers
//
// The answer is a RECORD of the move: which declarations left, where they came
// from, and where they went. The script this replaced computed all of it and
// then threw it away, printing one summary line and discarding the name of
// every declaration it had matched.
//
// That line is still what a person sees, because it is what a developer reads
// after a split, and Text renders it byte for byte. The record underneath it is
// what `| json` carries, so a caller that splits a file in a script can read
// what moved instead of parsing a sentence.

package goextract

import "github.com/ze-software/ze/internal/core/textbuf"

// Moved is one declaration that left the source, and where it was.
//
// The lines are the extent in the SOURCE file before the move, doc comment
// included. They are the answer to "what did this take out of my file", which
// no other record holds once the file is rewritten.
type Moved struct {
	Symbol    string `json:"symbol"`
	FirstLine int    `json:"first-line"`
	LastLine  int    `json:"last-line"`
}

// Report is what one move did. Symbols is the one row set, so the row
// operators act on the declarations that moved.
type Report struct {
	Source  string  `json:"source"`
	Dest    string  `json:"dest"`
	Lines   int     `json:"lines"`
	Symbols []Moved `json:"symbols"`
}

// Text renders the summary line the script printed, unchanged: the count of
// declarations, the count of lines they occupy in the destination, and the two
// files.
func (r Report) Text() string {
	var tb textbuf.Buffer
	return tb.Str("extracted ").Int(int64(len(r.Symbols))).Str(" symbols (").
		Int(int64(r.Lines)).Str(" lines) from ").Str(r.Source).Str(" → ").Str(r.Dest).
		Byte('\n').String()
}
