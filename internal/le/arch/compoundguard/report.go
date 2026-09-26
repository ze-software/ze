// Design: docs/contributing/ze-go-style.md -- the compound-guard gate's answer
//
// report.go holds what `le arch compound-guard check` ANSWERS, apart from what
// produced it.

package archcompoundguard

import (
	"github.com/ze-software/ze/internal/core/textbuf"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// Finding is one changed `||` guard that states more than one fact, and it is
// one ROW of the check's answer.
type Finding struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Fn   string `json:"fn"`
}

// Findings is the row set of one check.
type Findings []Finding

// CheckReport is the whole answer of one check: how many changed Go files were
// judged, and the guards among their changed lines that owe a split.
//
// The file count is part of the answer because the gate judges a change set,
// and an empty row set over zero files says something different from an empty
// row set over forty.
type CheckReport struct {
	// Base is the commit the change set was diffed against, and why that one.
	Base     repochanged.LineBase `json:"base"`
	Files    int                  `json:"files"`
	Findings Findings             `json:"findings"`
}

// exitCode is 1 on a finding and 0 on none.
func (r CheckReport) exitCode() int {
	if len(r.Findings) > 0 {
		return 1
	}
	return 0
}

// Text renders the scope line, one line per guard, and the fix. It ends in a
// newline.
func (r CheckReport) Text() string {
	var tb textbuf.Buffer
	tb.Str("compound-guard scope: the working tree against ").Str(r.Base.Commit)
	if r.Base.Reason != "" {
		tb.Str(" (").Str(r.Base.Reason).Byte(')')
	}
	tb.Str(", ").Int(int64(r.Files)).Str(" changed Go file(s), changed lines only\n")
	if len(r.Findings) == 0 {
		return tb.Str("compound-guard: OK\n").String()
	}

	tb.Str("compound-guard: ").Int(int64(len(r.Findings))).Str(" guard(s) state more than one fact:\n")
	for _, finding := range r.Findings {
		tb.Str("  ").Str(finding.File).Byte(':').Int(int64(finding.Line)).
			Str(" (").Str(finding.Fn).Str("): if a || b { leave }\n")
	}
	tb.Byte('\n')
	tb.Str("Split into one guard per fact: `if a { leave }` then `if b { leave }`.\n")
	tb.Str("|| stops at the first true operand and the body leaves, so the split keeps the meaning.\n")
	tb.Str("See docs/contributing/ze-go-style.md, \"Control flow a reader can simulate\".\n")
	return tb.String()
}
