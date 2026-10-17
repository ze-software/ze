// Design: docs/architecture/cli/command-verbs.md -- everything is a selector the operator types
// Related: command.go -- Dispatcher.dispatchPlugin, the unmatched-input path that asks here first

package server

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// IncompleteCommandError refuses input that stops at a prefix of registered
// commands. The prefix names no target, and a command that acts on a set
// never reads a missing selector as everything (ai/rules/cli.md), so the
// refusal lists the whole commands the operator can type instead.
type IncompleteCommandError struct {
	Prefix string   // the input as typed, lowercased and trimmed
	Forms  []string // every registered command below Prefix, sorted
}

// Error names the problem, why it is refused, and the commands that fix it,
// one per line.
func (e *IncompleteCommandError) Error() string {
	var tb textbuf.Buffer
	tb.Str(`"`).Str(e.Prefix).Str(`" is not a whole command: it does not say what to act on, and Ze never acts on everything unless you type it. Type one of:`)
	for _, form := range e.Forms {
		tb.Str("\n  ").Str(form)
	}
	return tb.String()
}

// incompleteCommand answers the refusal for input that is a strict prefix of
// at least one visible registered command, builtin or plugin, and nil for
// input that prefixes nothing visible, which stays an unknown command. The
// forms are read from the registries, so a new command appears here with no
// edit to this file.
func (d *Dispatcher) incompleteCommand(lowerInput string) error {
	prefix := strings.TrimSpace(lowerInput)
	if prefix == "" {
		return nil
	}
	below := prefix + " "
	var forms []string
	for key := range d.commands {
		if strings.HasPrefix(key, below) {
			forms = append(forms, key)
		}
	}
	for _, registered := range d.registry.All() {
		// A hidden command is kept out of completion and help, so the refusal
		// does not reveal it either: a prefix whose only completions are
		// hidden stays an unknown command.
		if registered.Hidden {
			continue
		}
		if strings.HasPrefix(registered.LowerName, below) {
			forms = append(forms, registered.LowerName)
		}
	}
	if len(forms) == 0 {
		return nil
	}
	slices.Sort(forms)
	return &IncompleteCommandError{Prefix: prefix, Forms: slices.Compact(forms)}
}
