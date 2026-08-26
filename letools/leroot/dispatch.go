// Design: docs/architecture/core-design.md -- how a program dispatches le's commands
// Overview: leroot.go -- the registration adapter every le tool joins through
//
// The loop is the engine's: registry.LookupRoot answers which tool owns the
// first word, and the tool renders its own answer through the shared pipe
// operators. Nothing here parses a command, and nothing here holds a table of
// them.
//
// It lives beside the registration adapter instead of in cmd/le because le is
// not the only program that CAN run these commands. A ze binary with the ze_le
// tag carries them under the `ze le` root (letools/zele). A second loop would
// guarantee drift.
//
// One filter sits in front of the lookup, and it is the price of le linking the
// product. Tools that INTROSPECT ze -- the inventory, the command list -- must
// load ze's registry to read it, so the process's registry carries ze's root
// commands too. A program running le's commands runs only the names le
// registered (Owns), so `le interface` is an unknown command rather than ze's
// interface editor.

package leroot

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// isHelpArg reports whether the word asks for the command list rather than
// naming a command.
func isHelpArg(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

// Commands answers le's own root commands with the metadata the registry holds
// for each, sorted by name because ListRoot is. A product command that arrived
// with the registry le links is not one of le's, so it is not listed.
func Commands() []registry.RootCommand {
	all := registry.ListRoot()
	mine := make([]registry.RootCommand, 0, len(all))
	for _, rc := range all {
		if Owns(rc.Name) {
			mine = append(mine, rc)
		}
	}
	return mine
}

// Usage lists every registered le tool with the registry's groups. program
// supplies the page name: "le" for the binary and "ze le" for a ze_le build.
// Thus, every line that a reader copies is valid for the active program.
func Usage(program string) {
	roots := Commands()
	entries := make([]helpfmt.HelpEntry, 0, len(roots))
	for _, rc := range roots {
		entries = append(entries, helpfmt.HelpEntry{Name: rc.Name, Desc: rc.Meta.Description})
	}

	var tb textbuf.Buffer
	page := helpfmt.Page{
		Command: program,
		Summary: "the Ze repository and development entry point",
		Usage:   []string{tb.Str(program).Str(" <command> [options] [| json | yaml | table]").String()},
		Sections: []helpfmt.HelpSection{
			{Title: "Commands", Entries: entries},
		},
	}
	page.WriteErr()
}

// Dispatch resolves argv against le's commands and answers the tool's exit code.
// An unregistered first word is a caller error, so Dispatch returns 1 instead of
// guessing. It also refuses a word registered by ze because that command belongs
// to another program that happens to share this process.
//
// program supplies the page name for a refusal. It does not change the command
// set, which contains the commands that le registered.
func Dispatch(program string, args []string) int {
	if len(args) == 0 {
		Usage(program)
		return 1
	}
	if isHelpArg(args[0]) {
		Usage(program)
		return 0
	}

	if Owns(args[0]) {
		if handler := registry.LookupRoot(args[0]); handler != nil {
			return handler(&registry.RuntimeContext{}, args[1:])
		}
	}

	fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0]) //nolint:errcheck // CLI output
	Usage(program)
	return 1
}
