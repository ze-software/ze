// Design: docs/architecture/config/yang-config-design.md -- the leaf-mention report
//
// report.go holds what `le yang-leaf-mentions report` ANSWERS, apart from what
// produced it.
//
// The payload is an object rather than a slice, because the two counts are the
// point: a report of zero findings over zero modules and a report of zero
// findings over 60 modules say opposite things, and only the counts tell them
// apart. One key holds rows, so the row operators act on the findings.

package yangleafmentions

import "github.com/ze-software/ze/internal/core/textbuf"

// Finding is one config leaf the owning package never names, and it is one ROW
// of the report. The keys are the script's, unchanged.
type Finding struct {
	Module  string `json:"module"`
	Package string `json:"package"`
	Leaf    string `json:"leaf"`
	Path    string `json:"path"`
}

// Report is the whole answer of one scan.
type Report struct {
	Modules  int       `json:"modules"`
	Leaves   int       `json:"leaves"`
	Findings []Finding `json:"findings"`
}

// The two column widths the script's page used. They keep the module and the
// leaf path in fixed columns so a long list stays readable.
const (
	moduleWidth = 28
	pathWidth   = 46
)

// Text renders the report for a person: the counts, the advisory sentence a
// reader needs before acting on any row, and one line per finding. It ends in a
// newline.
func (r Report) Text() string {
	var tb textbuf.Buffer
	tb.Str("yang-leaf-mentions: ").Int(int64(r.Modules)).Str(" config modules, ").
		Int(int64(r.Leaves)).Str(" leaves, ").Int(int64(len(r.Findings))).
		Str(" never named by the owning package\n")
	tb.Str("ADVISORY. A finding is a candidate to read, not a defect: a key built at\n")
	tb.Str("run time, or read through a shared helper, is reported the same way.\n")
	for _, finding := range r.Findings {
		tb.Str("  ").PadRight(finding.Module, moduleWidth).Byte(' ').
			PadRight(finding.Path, pathWidth).Byte(' ').Str(finding.Package).Byte('\n')
	}
	return tb.String()
}
