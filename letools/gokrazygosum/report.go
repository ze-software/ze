// Design: docs/architecture/appliance/gokrazy-image.md -- the gate's answer
// Overview: gokrazygosum.go -- the comparison that fills this in
//
// report.go holds what `le gokrazy-gosum` ANSWERS, apart from what produced it.
//
// The payload is an object rather than a slice because the two counts are the
// point. A report of zero conflicts over zero files and a report of zero
// conflicts over five files say opposite things, and only the counts tell them
// apart: the first checked nothing. One key holds rows, so the row operators
// act on the conflicts.

package gokrazygosum

import "github.com/ze-software/ze/internal/core/textbuf"

// Conflict is one (module, version) that the root go.sum and a builddir go.sum
// hash differently, and it is one ROW of the report.
type Conflict struct {
	Path         string `json:"path"`
	Module       string `json:"module"`
	Version      string `json:"version"`
	RootHash     string `json:"root-hash"`
	BuilddirHash string `json:"builddir-hash"`
}

// Report is the whole answer of one comparison: how many builddir files were
// read, how many of their entries the root module also holds, and every entry
// the two disagree about.
type Report struct {
	Files     int        `json:"files"`
	Shared    int        `json:"shared"`
	Conflicts []Conflict `json:"conflicts"`
}

// Text renders the report for a person, in the words the gate printed before it
// was a command. It ends in a newline.
//
// This is the Prose rendering leroot uses for the bare command, and every pipe
// operator bypasses it.
func (r Report) Text() string {
	var tb textbuf.Buffer

	if r.Files == 0 {
		return tb.Str("gokrazy-gosum: no tracked builddir go.sum files, nothing to check\n").String()
	}

	if len(r.Conflicts) == 0 {
		return tb.Str("gokrazy-gosum OK: ").Int(int64(r.Files)).Str(" builddir go.sum file(s), ").
			Int(int64(r.Shared)).Str(" entry/entries shared with ").Str(rootGosum).
			Str(", no hash conflict\n").String()
	}

	tb.Str("gokrazy-gosum: ").Int(int64(len(r.Conflicts))).
		Str(" hash conflict(s) between ").Str(rootGosum).
		Str(" and the tracked builddir go.sum files\n")
	for _, conflict := range r.Conflicts {
		tb.Str("  ").Str(conflict.Module).Byte(' ').Str(conflict.Version).Byte('\n')
		tb.Str("    ").Str(rootGosum).Str(": ").Str(conflict.RootHash).Byte('\n')
		tb.Str("    ").Str(conflict.Path).Str(": ").Str(conflict.BuilddirHash).Byte('\n')
	}
	tb.Byte('\n')
	tb.Str("One of the two records a module content the other would refuse. ")
	tb.Str("Re-resolve the builddir module, or the root, so both agree.\n")
	return tb.String()
}
