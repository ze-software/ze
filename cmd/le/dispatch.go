// Design: docs/architecture/core-design.md -- le's dispatch loop
// Overview: main.go -- the process boundary
//
// The loop is the engine's: registry.LookupRoot answers which tool owns the
// first word, and the tool renders its own answer through the shared pipe
// operators (letools/leroot). Nothing here parses a command, and nothing here
// holds a table of them.

package main

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/helpfmt"
)

// binaryName is what the usage page calls this program. It is a constant
// rather than os.Args[0] because le has one name; ze reads argv[0] because its
// personalities are one codebase under several names.
const binaryName = "le"

func isHelpArg(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

// usage lists every registered tool, grouped the way the registry groups them.
func usage() {
	roots := registry.ListRoot()
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

// dispatch resolves argv against the registry and answers the tool's exit
// code. An unknown first word is the caller's error, so it is refused with 1
// rather than guessed at.
func dispatch(args []string) int {
	if len(args) == 0 {
		usage()
		return 1
	}
	if isHelpArg(args[0]) {
		usage()
		return 0
	}

	if handler := registry.LookupRoot(args[0]); handler != nil {
		return handler(&registry.RuntimeContext{}, args[1:])
	}

	fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0]) //nolint:errcheck // CLI output
	usage()
	return 1
}
