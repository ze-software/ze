// Design: docs/architecture/cli/command-namespacing.md -- the dash-stdio guard's answer
//
// report.go holds what `le dash-stdio check` ANSWERS, apart from what produced
// it.
//
// The answer IS its rows, so the payload is a slice rather than a struct
// wrapping one: `| json` renders the array the script's --json rendered, and
// `| count` says how many. The slice also renders ITSELF (Text), because a
// violation list with the remedy under it is what a person reads here.

package dashstdio

import "github.com/ze-software/ze/internal/core/textbuf"

// Finding is one raw os path call on a user-supplied path, and it is one ROW of
// the check's answer. The keys are the script's, unchanged.
type Finding struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Fn   string `json:"fn"`
	Code string `json:"code"`
}

// Findings is the whole answer of one check.
type Findings []Finding

// Text renders the findings for a person: the count, one line per site, and the
// remedy. A run that found nothing renders the verdict the script printed. It
// ends in a newline.
func (f Findings) Text() string {
	var tb textbuf.Buffer
	if len(f) == 0 {
		return tb.Str("cli-dash-stdio: OK\n").String()
	}

	tb.Str("cli-dash-stdio: ").Int(int64(len(f))).
		Str(" raw os call(s) on a user-supplied path (must use internal/core/cliio so \"-\" means stdin/stdout):\n")
	for _, finding := range f {
		tb.Str("  ").Str(finding.File).Byte(':').Int(int64(finding.Line)).
			Str(" (os.").Str(finding.Fn).Str("): ").Str(finding.Code).Byte('\n')
	}
	tb.Byte('\n')
	tb.Str("A filename-accepting command must read/write through internal/core/cliio\n")
	tb.Str("(ReadFile/OpenReader/Create/WriteFile) so \"-\" resolves to stdin/stdout. If this\n")
	tb.Str("path can never be \"-\" (a device node, an internally-derived name), add an\n")
	tb.Str("allowlist entry with a reason in scripts/checks/cli_dash_stdio.go.\n")
	return tb.String()
}
