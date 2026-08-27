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
		Str(" raw filesystem write(s) that may persist runtime state:\n")
	for _, finding := range f {
		tb.Str("  ").Str(finding.File).Byte(':').Int(int64(finding.Line)).
			Str(" (").Str(finding.Pkg).Byte('.').Str(finding.Fn).Str("): ").Str(finding.Code).Byte('\n')
	}
	tb.Byte('\n')
	tb.Str("Daemon runtime state must persist through the managed zefs store, not loose\n")
	tb.Str("files: use internal/core/statestore (Put/Get under a registered pkg/zefs key)\n")
	tb.Str("so appliance state lives inside database.zefs. If this write is a genuine\n")
	tb.Str("non-state file (kernel knob, ephemeral scratch, external artifact, storage\n")
	tb.Str("layer), add an allowlist entry with a reason in scripts/checks/direct_fs_persistence.go.\n")
	return tb.String()
}
