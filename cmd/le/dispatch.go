// Design: docs/architecture/core-design.md -- le's dispatch loop
// Overview: main.go -- the process boundary
//
// The loop is the engine's: registry.LookupRoot answers which tool owns the
// first word, and the tool renders its own answer through the shared pipe
// operators (letools/leroot). Nothing here parses a command, and nothing here
// holds a table of them.
//
// One filter sits in front of the lookup, and it is the price of le linking the
// product. Tools that INTROSPECT ze -- the inventory, the command list -- must
// load ze's registry to read it, so this process's registry carries ze's root
// commands too. le runs only the names le registered (letools/leroot, Owns),
// so `le interface` is an unknown command rather than ze's interface editor.

package main

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/letools/leroot"
)

// binaryName is what the usage page calls this program. It is a constant
// rather than os.Args[0] because le has one name; ze reads argv[0] because its
// personalities are one codebase under several names.
const binaryName = "le"

func isHelpArg(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

// leRoots answers le's own root commands with the metadata the registry holds
// for each, sorted by name because ListRoot is.
func leRoots() []registry.RootCommand {
	all := registry.ListRoot()
	mine := make([]registry.RootCommand, 0, len(all))
	for _, rc := range all {
		if leroot.Owns(rc.Name) {
			mine = append(mine, rc)
		}
	}
	return mine
}

// usage lists every tool le registered, grouped the way the registry groups
// them. A product command that arrived with the registry le links is not one of
// le's, so it is not offered here either.
func usage() {
	roots := leRoots()
	entries := make([]helpfmt.HelpEntry, 0, len(roots))
	for _, rc := range roots {
		entries = append(entries, helpfmt.HelpEntry{Name: rc.Name, Desc: rc.Meta.Description})
	}

	page := helpfmt.Page{
		Command: binaryName,
		Summary: "the Ze repository and development entry point",
		// A literal, as every other usage line in the tree is
		// (internal/plugins/skills, completion, exabgp all spell their own
		// program name). Concatenating the constant in would be the one
		// exception, and `performance.md` bans building strings that way.
		Usage: []string{"le <command> [options] [| json | yaml | table]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Commands", Entries: entries},
		},
	}
	page.WriteErr()
}

// dispatch resolves argv against le's own commands and answers the tool's exit
// code. A first word le did not register is the caller's error, so it is
// refused with 1 rather than guessed at -- including a word ze registered,
// which is a command of another program that happens to share this process.
func dispatch(args []string) int {
	if len(args) == 0 {
		usage()
		return 1
	}
	if isHelpArg(args[0]) {
		usage()
		return 0
	}

	if leroot.Owns(args[0]) {
		if handler := registry.LookupRoot(args[0]); handler != nil {
			return handler(&registry.RuntimeContext{}, args[1:])
		}
	}

	fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0]) //nolint:errcheck // CLI output
	usage()
	return 1
}
