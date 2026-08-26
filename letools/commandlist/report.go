// Design: docs/architecture/api/commands.md -- the command verb taxonomy
//
// report.go holds what `le command-list` ANSWERS, apart from what produced it.
//
// The answer is a row per command, which is structured data every operator can
// act on: `| json` feeds a script, `| match show` keeps one verb's rows,
// `| count` says how many. It also renders ITSELF (Text), because the markdown
// table is what `make ze-command-list` has always printed and what a reader
// pastes into a document.

package commandlist

import "github.com/ze-software/ze/internal/core/textbuf"

// Command is one registered command, and it is one ROW of the answer.
type Command struct {
	// Verb is the first word of the CLI path when that word is one of the
	// taxonomy's verbs, and "-" when it is not.
	Verb string `json:"verb"`
	// Path is the CLI path the operator types, or the wire method when no
	// YANG command tree maps one.
	Path string `json:"path"`
	// WireMethod is the ze:command argument the handler registered, and it is
	// ABSENT for a command that has none: a streaming prefix and a TUI command
	// are reached by path alone.
	WireMethod string `json:"wire-method"`
	// Source says where the registration came from: builtin, streaming or cli.
	Source string `json:"source"`
}

// Commands is the whole answer of one run: every registered command, sorted by
// verb and then by path.
//
// It is a slice rather than a struct wrapping one, so `| json` answers the same
// array the `--json` flag of the script answered. The engine reads the rows
// straight out of it (internal/component/command/answer_shape.go, rowsIn).
type Commands []Command

// Text renders the inventory as the markdown table `make ze-command-list`
// prints. It ends in a newline.
//
// This is the Prose rendering leroot uses for the bare command, and every pipe
// operator bypasses it (letools/leroot, Prose).
func (c Commands) Text() string {
	var tb textbuf.Buffer
	tb.Str("# Command Inventory\n\n")
	tb.Str("| Verb | CLI Path | Wire Method | Source |\n")
	tb.Str("|------|----------|-------------|--------|\n")
	for _, entry := range c {
		tb.Str("| ").Str(entry.Verb).Str(" | ").Str(entry.Path).Str(" | ").
			Str(entry.WireMethod).Str(" | ").Str(entry.Source).Str(" |\n")
	}
	tb.Str("\nTotal: ").Int(int64(len(c))).Str(" commands\n")
	return tb.String()
}
