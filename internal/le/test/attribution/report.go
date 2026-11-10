// Design: docs/contributing/running-commands.md -- le test attribution report
// Overview: attribution.go -- the comparison that fills this in
//
// report.go holds what `le test attribution report` ANSWERS, apart from what
// produced it. One key holds rows, so the row operators act on the tests.

package testattribution

import (
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Row is one failing test of the log, and one ROW of the report.
type Row struct {
	Test   string `json:"test"`
	Class  Class  `json:"class"`
	Detail string `json:"detail"`
	OnDisk string `json:"on-disk"`
	AtHead string `json:"at-head"`
}

// Report is the whole answer of one attribution.
type Report struct {
	Log     string   `json:"log"`
	Package string   `json:"package"`
	Tags    string   `json:"tags"`
	Files   []string `json:"files"`
	// Compared states what the two runs of each test differed in, which bounds
	// what any row can claim.
	Compared string `json:"compared"`
	Rows     []Row  `json:"rows"`
}

// The exit codes. A red the session's files cause is the answer a caller
// acts on; a row with no verdict is not "not caused", so it is not 0 either.
const (
	exitNoneCaused = 0
	exitCaused     = 1
	exitNoVerdict  = 2
)

// ExitCode answers 1 when a row is caused or added (the named files bring the
// red), else 2 when a row reached no verdict (undetermined, not-run), else 0.
func (r Report) ExitCode() int {
	noVerdict := false
	for _, row := range r.Rows {
		switch row.Class {
		case ClassCaused, ClassAdded:
			return exitCaused
		case ClassUndetermined, ClassNotRun:
			noVerdict = true
		case ClassNotCaused, ClassRedAtHeadOnly, ClassNotReproduced:
		}
	}
	if noVerdict {
		return exitNoVerdict
	}
	return exitNoneCaused
}

// Text renders the report for a person. It ends in a newline.
//
// This is the prose rendering leroot uses for the bare command, and every pipe
// operator bypasses it.
func (r Report) Text() string {
	var tb textbuf.Buffer
	tb.Str("test attribution: ").Int(int64(len(r.Rows))).Str(" failing test(s) of ").Str(r.Package).
		Str(" from ").Str(r.Log).Byte('\n')
	for _, row := range r.Rows {
		tb.Str("  ").PadRight(string(row.Class), 17).Str(row.Test).Byte('\n')
		tb.Str("      ").Str(row.Detail).Byte('\n')
	}
	tb.Str("compared: ").Str(r.Compared).Byte('\n')
	switch r.ExitCode() {
	case exitCaused:
		tb.Str("verdict: the named files cause at least one red; fix it before you commit\n")
	case exitNoVerdict:
		tb.Str("verdict: no row is caused, but at least one reached no verdict; read its detail\n")
	default:
		tb.Str("verdict: the named files cause none of these reds\n")
	}
	return tb.String()
}
