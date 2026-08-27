// Design: docs/architecture/iface/logical-name-resolution.md -- the guard's answer
//
// report.go holds what `le iface-resolution` ANSWERS, apart from what produced
// it.
//
// The answer IS the rows, so the payload is a slice rather than a struct
// wrapping one: `| json` then renders the same array the script's --json
// rendered, `| count` says how many sites there are, and `| match internal/`
// keeps one tree's. The slice also renders ITSELF (Text), because a violation
// list with the remedy under it is what a person needs here and the engine
// would render the rows as a table (internal/le/leroot, Prose).

package ifaceresolution

import "github.com/ze-software/ze/internal/core/textbuf"

// Finding is one direct kernel resolution site, and it is one ROW of the
// answer. The keys are the script's, unchanged.
type Finding struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Code string `json:"code"`
}

// Findings is the whole answer of one run: every site outside the allowlist,
// sorted by file and line.
type Findings []Finding

// Text renders the findings for a person: the count, one line per site, and the
// remedy. A run that found nothing renders the verdict the script printed. It
// ends in a newline.
//
// This is the Prose rendering leroot uses for the bare command, and every pipe
// operator bypasses it.
func (f Findings) Text() string {
	var tb textbuf.Buffer
	if len(f) == 0 {
		return tb.Str("iface-resolution: OK\n").String()
	}

	tb.Str("iface-resolution: ").Int(int64(len(f))).Str(" direct kernel resolution site(s) outside the allowlist:\n")
	for _, finding := range f {
		tb.Str("  ").Str(finding.File).Byte(':').Int(int64(finding.Line)).Str(": ").Str(finding.Code).Byte('\n')
	}
	tb.Byte('\n')
	tb.Str("Resolve logical interface names via iface.Resolve / iface.Addresses / iface.Subscribe\n")
	tb.Str("(or the iface dispatch ops), so os-name / mac-match selectors are honored. If a site\n")
	// The path is the SCRIPT's, and it stays so until the swap deletes it.
	// Both copies of the allowlist must agree while both exist, which is what
	// scripts/checks/parity_test.go compares, so a reader sent to either one
	// ends up editing both.
	tb.Str("must resolve directly, add it to the allowlist in scripts/checks/iface_resolution.go\n")
	tb.Str("with a reason.\n")
	return tb.String()
}
