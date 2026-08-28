// Design: docs/architecture/core-design.md -- structured le answers
// Detail: tree.go -- the scan and explanation verdict that produces this answer.
//
// Issues is the one row set. The counts and package list are scalars, so every
// row operator unambiguously acts on findings. Text reproduces the producer for
// a person while JSON, YAML, and table rendering consume the fields directly.

package htmxupgrade

import "github.com/ze-software/ze/internal/core/textbuf"

type reportMode uint8

const (
	modeCheck reportMode = iota + 1
	modeReport
)

// Report is the answer of either htmx-upgrade action.
type Report struct {
	Issues   []Issue `json:"issues"`
	Files    int     `json:"files"`
	Packages string  `json:"packages"`

	stale          []staleRow
	explainedCount int
	mode           reportMode
}

// Text renders the native producer's lines in source order. It ends in a
// newline. Check carries only unexplained issues; report carries every issue.
func (r Report) Text() string {
	var tb textbuf.Buffer
	for _, issue := range r.Issues {
		tb.Str(issue.Path).Byte(':').Int(int64(issue.Line)).Str(": [").
			Str(issue.Category).Str("] ").Str(issue.Message).Byte('\n')
	}

	if r.mode == modeReport {
		tb.Byte('\n').Int(int64(uniqueIssueCount(r.Issues))).Str(" issue(s) over ").
			Int(int64(r.Files)).Str(" file(s) in ").Str(r.Packages).Byte('\n')
		return tb.String()
	}

	for _, row := range r.stale {
		tb.Str("STALE: ").Str(explained).Str(" explains ").Str(row.category).
			Str(" in ").Str(row.path).Str(", and the scan reports none there\n")
	}
	if len(r.Issues) > 0 || len(r.stale) > 0 {
		tb.Byte('\n').Int(int64(len(r.Issues))).Str(" unexplained issue(s) and ").
			Int(int64(len(r.stale))).Str(" stale row(s) over ").Int(int64(r.Files)).
			Str(" file(s) in ").Str(r.Packages).Byte('\n').
			Str("Fix the site, or add its row to ").Str(explained).Byte('\n')
		return tb.String()
	}

	return tb.Str("htmx-upgrade-check: ").Int(int64(r.Files)).Str(" file(s) in ").
		Str(r.Packages).Str(" carry no unexplained htmx 4 upgrade issue (").
		Int(int64(r.explainedCount)).Str(" explained)\n").String()
}

func uniqueIssueCount(issues []Issue) int {
	seen := make(map[Issue]bool, len(issues))
	for _, issue := range issues {
		seen[issue] = true
	}
	return len(seen)
}
