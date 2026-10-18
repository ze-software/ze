// Design: docs/architecture/cli/command-verbs.md -- everything is a selector the operator types
// Related: command.go -- Dispatcher.dispatchPlugin, the unmatched-input path that asks here first

package server

import (
	"errors"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
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

// missingSelectorError is the guard's refusal of a command that requires a
// selector and was given none, and every form of it names the line that
// supplies one, with the selector at the position Dispatch reads it from
// (writeSelectedPath) and what the selector leaf's ze:help says the value is: `*
// for every peer` on a clear that acts on a set, so the everything form is
// named rather than implied (ai/rules/cli.md).
//
// With one spare token or none, the operator left the selector out, and the
// refusal says so. With several, the operator typed values without their
// keywords, so no lone token could be adopted as the selector
// (adoptablePositional): saying only "requires a selector" would read as false
// to someone who typed one, so the refusal says why none was taken and names
// the keyword form, as `clear bgp rib out * family ipv4/unicast`.
func missingSelectorError(matchedCmd *Command, args []string) error {
	var tb textbuf.Buffer
	tb.Str(matchedCmd.Name).Str(" requires a selector")
	if len(args) < 2 {
		tb.Str(": ")
		writeSelectorForm(&tb, matchedCmd)
		return errors.New(tb.String())
	}
	keywords := make([]string, 0, len(matchedCmd.ArgDefs))
	for i := range matchedCmd.ArgDefs {
		if name := matchedCmd.ArgDefs[i].Name(); name != selectorLeaf {
			keywords = append(keywords, name)
		}
	}
	tb.Str(": ").Str(strings.Join(args, " ")).Str(" are ").Int(int64(len(args))).
		Str(" values typed without a keyword, so none is taken as the selector")
	if len(keywords) == 1 && len(args) == 2 {
		tb.Str(". Type the selector first and the other value after its keyword: ")
		writeSelectedPath(&tb, matchedCmd, args[0])
		tb.Str(" ").Str(keywords[0]).Str(" ").Str(args[1])
		return errors.New(tb.String())
	}
	tb.Str(". Type ")
	writeSelectorForm(&tb, matchedCmd)
	if len(keywords) > 0 {
		tb.Str(", then each other value after its keyword: ").Str(strings.Join(keywords, ", "))
	}
	return errors.New(tb.String())
}

// writeSelectorForm writes the line that gives matchedCmd its selector, then
// what the selector leaf's ze:help says the value is, when the model declares
// that leaf with a summary.
func writeSelectorForm(tb *textbuf.Buffer, matchedCmd *Command) {
	selector := writeSelectedPath(tb, matchedCmd, "<selector>")
	if selector == nil {
		return
	}
	if help := selector.ShortHelp(); help != "" {
		tb.Str(" (").Str(help).Str(")")
	}
}

// writeSelectedPath writes matchedCmd's path with value where Dispatch reads
// the selector: after the interior key token anchoredDef binds the selector
// leaf to, as `request peer <selector> teardown`, and after the whole path
// otherwise, as `clear bgp rib out <selector>`, where Dispatch adopts the lone
// positional value. It answers the selector leaf, nil when the command
// declares none.
func writeSelectedPath(tb *textbuf.Buffer, matchedCmd *Command, value string) *command.ArgDef {
	keyTokens := strings.Fields(matchedCmd.Name)
	var selector *command.ArgDef
	for i := range matchedCmd.ArgDefs {
		if matchedCmd.ArgDefs[i].Name() == selectorLeaf {
			selector = &matchedCmd.ArgDefs[i]
			break
		}
	}
	at := selectorAnchorIndex(keyTokens, matchedCmd.ArgDefs, selector)
	tb.Str(strings.Join(keyTokens[:at+1], " ")).Str(" ").Str(value)
	if rest := keyTokens[at+1:]; len(rest) > 0 {
		tb.Str(" ").Str(strings.Join(rest, " "))
	}
	return selector
}

// selectorAnchorIndex answers the index of the key token the selector value is
// typed after. matchCommandTokens asks anchoredDef at every key token but the
// last, from the first, and binds the value at the first one that answers the
// selector leaf, so this asks the same question in the same order. With no such
// token, the value follows the last key token.
func selectorAnchorIndex(keyTokens []string, defs []command.ArgDef, selector *command.ArgDef) int {
	last := len(keyTokens) - 1
	if selector == nil {
		return last
	}
	for i := range last {
		if anchoredDef(keyTokens[i], defs, nil) == selector {
			return i
		}
	}
	return last
}
