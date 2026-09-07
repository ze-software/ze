// Design: docs/architecture/api/commands.md -- the declaration gate's answers
//
// report.go holds what `le plugin declarations` ANSWERS, apart from what
// produced it.
//
// The answer IS its rows, so the payload is a slice rather than a struct
// wrapping one: `| json` feeds a script, `| count` says how many, `| match`
// keeps one package's rows. It also renders ITSELF (Text), because a person
// reading a failure wants the plugin, the command and the repair under it.

package plugindeclarations

import "github.com/ze-software/ze/internal/core/textbuf"

// Finding is one command a plugin declares to Stage 1 and not to its
// registration, and it is one ROW of the check's answer.
type Finding struct {
	// Package is the plugin's package directory relative to the tree. It names
	// the plugin: a registration's Name field is usually a constant, and the
	// directory reads the same in a log and in an editor.
	Package string `json:"package"`
	// Command is the command name the runner declares. For a declaration the
	// gate could not read, it is the file and line that declaration sits at.
	Command string `json:"command"`
	// File and Line are where the runner's Commands field is written.
	File string `json:"file"`
	Line int    `json:"line"`
	// Reason states which of the two failures this row is: a command missing
	// from the registration, or a declaration the gate could not read.
	Reason string `json:"reason"`
}

// Findings is the whole answer of one check.
type Findings []Finding

// Text renders the findings for a person: the count, one line per command, and
// the repair. A run that found nothing renders the verdict. It ends in a
// newline.
func (f Findings) Text() string {
	var tb textbuf.Buffer
	if len(f) == 0 {
		return tb.Str("plugin-declarations: OK\n").String()
	}

	tb.Str("plugin-declarations: ").Int(int64(len(f))).
		Str(" command(s) declared to Stage 1 and absent from the registration:\n")
	for _, finding := range f {
		tb.Str("  ").Str(finding.Package).Str(": ").Str(finding.Command).
			Str("  (").Str(finding.File).Byte(':').Int(int64(finding.Line)).Str(")\n")
		tb.Str("    ").Str(finding.Reason).Byte('\n')
	}
	tb.Byte('\n')
	tb.Str("A plugin declares its commands once, in one commandDecls() function, and both\n")
	tb.Str("readers call it: registry.Registration.Commands in the plugin's init(), and the\n")
	tb.Str("sdk.Registration its runner passes to p.Run. Put the runner's slice behind that\n")
	tb.Str("function and name the function from both literals. A second copy is the drift\n")
	tb.Str("this gate exists to end, so do not repair a row by copying the list.\n")
	return tb.String()
}
