// Design: docs/architecture/resolve.md -- what the iana-asn command answers
//
// report.go holds what the one action ANSWERS, apart from what produced it.

package ianaasn

import (
	"github.com/ze-software/ze/internal/core/textbuf"
)

// WriteReport is what `le iana-asn write` answers: the file it rewrote, and the
// two numbers that say what went into it.
//
// The numbers are published rather than implied. Records is what the five
// registries delegated between them and Ranges is what survived collapsing, so a
// reader can tell a run that read the whole world from one that read a fraction
// of it.
type WriteReport struct {
	// File is the generated table, relative to the tree.
	File string `json:"file"`
	// Ranges is how many collapsed ranges the table holds.
	Ranges int `json:"ranges"`
	// Records is how many delegation records the five files yielded.
	Records int `json:"records"`
}

// Text renders the verdict in the words the script printed. It ends in a
// newline.
func (r WriteReport) Text() string {
	var tb textbuf.Buffer

	return tb.Str("wrote ").Str(r.File).Str(" (").Int(int64(r.Ranges)).
		Str(" collapsed ranges from ").Int(int64(r.Records)).Str(" records)\n").String()
}
