// Design: docs/architecture/core-design.md -- the persistence guard's answer
//
// report.go holds what `le fs-persistence check` ANSWERS, apart from what
// produced it.
//
// The answer IS its rows, so the payload is a slice rather than a struct
// wrapping one: `| json` renders the array the script's --json rendered, and
// `| count` says how many. The slice also renders ITSELF (Text), because a
// violation list with the remedy under it is what a person reads here.

package fspersistence

import "github.com/ze-software/ze/internal/core/textbuf"

// Finding is one raw filesystem write that may persist runtime state, and it is
// one ROW of the check's answer. The keys are the script's, unchanged.
type Finding struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Pkg  string `json:"pkg"`
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
		return tb.Str("direct-fs-persistence: OK\n").String()
	}

	tb.Str("direct-fs-persistence: ").Int(int64(len(f))).
		Str(" persistence-layer bypass(es):\n")
	for _, finding := range f {
		tb.Str("  ").Str(finding.File).Byte(':').Int(int64(finding.Line)).
			Str(" (").Str(finding.Pkg).Byte('.').Str(finding.Fn).Str("): ").Str(finding.Code).Byte('\n')
	}
	tb.Byte('\n')
	tb.Str("Daemon runtime state must persist through internal/core/statestore under a\n")
	tb.Str("registered key. Live stores are opened only through config/storage; the\n")
	tb.Str("live-zefs-open rule also scans internal/core and accepts only named blob\n")
	tb.Str("artifact owners. Raw-write exceptions do not authorize direct blob opens.\n")
	tb.Str("Document genuine artifact exceptions in internal/le/fspersistence/fspersistence.go.\n")
	return tb.String()
}
