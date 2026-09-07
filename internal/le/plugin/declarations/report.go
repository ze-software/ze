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

// Finding is one declaration a plugin's two Registration literals disagree
// about, and it is one ROW of the check's answer.
type Finding struct {
	// Package is the plugin's package directory relative to the tree. It names
	// the plugin: a registration's Name field is usually a constant, and the
	// directory reads the same in a log and in an editor.
	Package string `json:"package"`
	// Command is the declaration this row is about: a command name, or a pipe
	// alias as the command and name pair its registry keys it on. For a
	// declaration the gate could not read, it is the file and line that
	// declaration sits at, and for a package it cannot pair it is the package.
	Command string `json:"command"`
	// File and Line are where the field this row disagrees about is written.
	File string `json:"file"`
	Line int    `json:"line"`
	// Reason states which failure this row is: a declaration missing from the
	// registration, one missing from the runner, one the two spell differently,
	// one the gate could not read, or a package it cannot pair.
	Reason string `json:"reason"`
}

// Findings is the whole answer of one check.
type Findings []Finding

// Text renders the findings for a person: the count, one line per declaration,
// and the repair. A run that found nothing renders the verdict. It ends in a
// newline.
func (f Findings) Text() string {
	var tb textbuf.Buffer
	if len(f) == 0 {
		return tb.Str("plugin-declarations: OK\n").String()
	}

	tb.Str("plugin-declarations: ").Int(int64(len(f))).
		Str(" declaration(s) the runner and the registration disagree about:\n")
	for _, finding := range f {
		tb.Str("  ").Str(finding.Package).Str(": ").Str(finding.Command)
		// A row about the PACKAGE names no file: the gate refused to pair the
		// two literals rather than reading one of them, so there is no line to
		// send a reader to.
		if finding.File != "" {
			tb.Str("  (").Str(finding.File).Byte(':').Int(int64(finding.Line)).Str(")")
		}
		tb.Byte('\n')
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
