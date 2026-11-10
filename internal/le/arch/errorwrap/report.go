// Design: docs/contributing/ze-go-style.md -- the error-wrap gate's answer
//
// report.go holds what `le arch error-wrap check` ANSWERS, apart from what
// produced it.

package archerrorwrap

import (
	"github.com/ze-software/ze/internal/core/textbuf"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// Finding is one wrap that adds a segment its wrapped error already carries,
// and it is one ROW of the check's answer. It names both sites: the wrap, and
// the constructor that first wrote the segment.
type Finding struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Fn         string `json:"fn"`
	Segment    string `json:"segment"`
	OriginFile string `json:"origin-file"`
	OriginLine int    `json:"origin-line"`
	OriginFn   string `json:"origin-fn"`
}

// Findings is the row set of one check.
type Findings []Finding

// CheckReport is the whole answer of one run: what was judged, and the wraps
// among it that repeat a segment.
//
// Untraced is part of the answer because the trace is syntactic: a wrapped
// error that came through a function value, an interface method or a
// parameter has a producer this check cannot name, and a wrap it could not
// follow is counted rather than passed in silence.
type CheckReport struct {
	// Base is the commit the change set was diffed against, and why that one.
	// It is empty for `report`, which names its Scope instead.
	Base     repochanged.LineBase `json:"base"`
	Scope    string               `json:"scope"`
	Files    int                  `json:"files"`
	Wraps    int                  `json:"wraps"`
	Untraced int                  `json:"untraced"`
	Findings Findings             `json:"findings"`
}

// exitCode is 1 on a finding and 0 on none.
func (r CheckReport) exitCode() int {
	if len(r.Findings) > 0 {
		return 1
	}
	return 0
}

// Text renders the scope line, one entry per finding naming both sites, and
// the fix. It ends in a newline.
func (r CheckReport) Text() string {
	var tb textbuf.Buffer
	tb.Str("error-wrap scope: ")
	r.writeScope(&tb)
	tb.Str(", ").Int(int64(r.Files)).Str(" Go file(s), ").
		Int(int64(r.Wraps)).Str(" wrap(s) judged, ").
		Int(int64(r.Untraced)).Str(" whose wrapped error could not be traced to its producer\n")
	if len(r.Findings) == 0 {
		return tb.Str("error-wrap: OK\n").String()
	}

	tb.Str("error-wrap: ").Int(int64(len(r.Findings))).
		Str(" wrap(s) add a segment the wrapped error already carries:\n")
	for _, finding := range r.Findings {
		tb.Str("  ").Str(finding.File).Byte(':').Int(int64(finding.Line)).
			Str(" (").Str(finding.Fn).Str(") adds \"").Str(finding.Segment).Str("\"\n")
		tb.Str("    already written by ").Str(finding.OriginFn).Str(" at ").
			Str(finding.OriginFile).Byte(':').Int(int64(finding.OriginLine)).Byte('\n')
	}
	tb.Byte('\n')
	tb.Str("The operator reads the segment twice in one message (reload: parse config: parse config: ...).\n")
	tb.Str("Fix: drop the segment from the outer format string, or remove it at the producer\n")
	tb.Str("so exactly one layer adds it and the others pass the error through.\n")
	tb.Str("See docs/contributing/ze-go-style.md, \"Every error is handled\".\n")
	return tb.String()
}

// writeScope names what was judged: the named scope of a report, or the base
// of a change-set check.
func (r CheckReport) writeScope(tb *textbuf.Buffer) {
	if r.Scope == "" {
		tb.Str("the working tree against ").Str(r.Base.Commit)
		if r.Base.Reason != "" {
			tb.Str(" (").Str(r.Base.Reason).Byte(')')
		}
		tb.Str(", changed lines only")
		return
	}
	tb.Str(r.Scope)
}
