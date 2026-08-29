// Design: docs/architecture/api/commands.md -- the ownership gate's answer
//
// report.go holds what `le command-ownership` ANSWERS, apart from what produced
// it.
//
// The answer IS the rows, so the payload is a slice rather than a struct
// wrapping one: `| json` renders the same array the script's --json rendered,
// `| count` says how many violations there are, and `| match root-not-allowlisted`
// keeps one kind. The slice also renders ITSELF (Text), because a violation
// list with a verdict under it is what a person reads here
// (internal/le/leroot, Prose).

package commandownership

import "github.com/ze-software/ze/internal/core/textbuf"

// Finding is one ownership violation, and it is one ROW of the answer. The keys
// are the script's, unchanged.
type Finding struct {
	Kind string `json:"kind"`
	File string `json:"file"`
	Msg  string `json:"message"`
}

// Findings is the whole answer of one run.
type Findings []Finding

// Text renders the findings for a person: one line per violation, then the
// verdict. It ends in a newline.
//
// This is the Prose rendering leroot uses for the bare command, and every pipe
// operator bypasses it.
func (f Findings) Text() string {
	var tb textbuf.Buffer
	if len(f) == 0 {
		return tb.Str("command-ownership: OK (owners cmd/ze-free, root handlers internal, no-owner roots allowlisted)\n").String()
	}

	for _, finding := range f {
		tb.Str("  [").Str(finding.Kind).Str("] ").Str(finding.File).Str(": ").Str(finding.Msg).Byte('\n')
	}
	tb.Str("\ncommand-ownership: FAILED, ").Int(int64(len(f))).Str(" problem(s)\n")
	return tb.String()
}
