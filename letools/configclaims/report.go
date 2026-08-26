// Design: docs/architecture/config/yang-config-design.md -- the claim gate's answer
//
// report.go holds what `le config-claims` ANSWERS, apart from what produced it.
//
// The answer is the inventory the gate read and the findings it drew, which is
// structured data an operator can act on: `| json` feeds a script and `| yaml`
// reads it. The report also renders ITSELF (Text), because the gate's page is
// what a person reads here and the engine would render it as a table
// (letools/leroot, Prose).

package configclaims

import "github.com/ze-software/ze/internal/core/textbuf"

// Report is the whole answer of one run, and its four fields are the four the
// script's --json emitted, under the same keys. Roots and Claims are the
// inventory the gate enumerated, which is what tells a clean tree from a walk
// that read nothing.
type Report struct {
	Roots       int      `json:"roots"`
	Claims      int      `json:"claims"`
	Allowlisted []string `json:"allowlisted"`
	Findings    []string `json:"findings"`
}

// Text renders the report for a person, in the page the script printed: the
// heading, the inventory, the recorded exceptions, then the findings. It ends
// in a newline.
//
// This is the Prose rendering leroot uses for the bare command, and every pipe
// operator bypasses it.
//
// The page carries no color. The script printed none, and this page is pasted
// into a terminal transcript when a gate fails rather than read at a prompt.
func (r Report) Text() string {
	var tb textbuf.Buffer
	tb.Str("# Config Claim Completeness Gate\n\n")
	tb.Str("Top-level config roots: ").Int(int64(r.Roots)).Byte('\n')
	tb.Str("Claims: ").Int(int64(r.Claims)).Byte('\n')
	tb.Str("Allowlisted: ").Int(int64(len(r.Allowlisted))).Str("\n\n")

	for _, path := range r.Allowlisted {
		tb.Str("  allowlisted ").Str(path).Byte('\n')
	}

	if len(r.Findings) > 0 {
		tb.Str("\n## Findings (").Int(int64(len(r.Findings))).Str(")\n\n")
		for _, finding := range r.Findings {
			tb.Str("  ").Str(finding).Byte('\n')
		}
		return tb.String()
	}

	// The verdict line belongs to the page rather than to the data: the script
	// printed it on stdout for a reader and left it out of the JSON, so a
	// caller reading the payload sees the empty findings list instead.
	tb.Str("config-claims: OK\n")
	return tb.String()
}
