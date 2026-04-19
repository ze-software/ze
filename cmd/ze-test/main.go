// Design: docs/architecture/testing/ci-format.md -- test runner CLI
//
// Command ze-test provides test utilities for Ze BGP.
//
// Subcommands register themselves via `var _ = register(...)` in their
// own file; run `ze-test --help` for the live list.
package main

import (
	"fmt"
	"os"
	"sort"
)

// subcommand describes one top-level `ze-test <name>` entry point.
// Handlers take no parameters because main shifts os.Args before
// dispatching, so the subcommand sees itself as os.Args[0].
type subcommand struct {
	desc    string
	handler func() int
}

// subcommands holds the registry populated from each command file's
// package-level register() call. Keyed by the command name the user
// types on the command line (e.g. "firewall", "rtr-mock").
var subcommands = map[string]subcommand{}

// register adds a subcommand to the dispatch table. Called from a
// package-level `var _ = register(...)` in each subcommand file so
// main.go stays agnostic of which commands exist. Returns the name
// to fit the `var _ =` registration idiom used across the codebase
// (see env.MustRegister).
func register(name, desc string, handler func() int) string { //nolint:unparam // return enables `var _ = register(...)` package-level registration
	if _, exists := subcommands[name]; exists {
		panic("BUG: ze-test: duplicate subcommand " + name)
	}
	subcommands[name] = subcommand{desc: desc, handler: handler}
	return name
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	if isHelpArg(cmd) {
		printUsage()
		return
	}

	// Shift args so subcommand sees itself as os.Args[0]
	os.Args = os.Args[1:]

	sc, ok := subcommands[cmd]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
	os.Exit(sc.handler())
}

// isHelpArg checks if the argument is a help flag.
func isHelpArg(arg string) bool {
	return arg == "-h" || arg == "--help" || arg == "help"
}

func printUsage() {
	names := make([]string, 0, len(subcommands))
	width := 0
	for name := range subcommands {
		names = append(names, name)
		if len(name) > width {
			width = len(name)
		}
	}
	sort.Strings(names)

	fmt.Fprintf(os.Stderr, "Usage: ze-test <command> [options]\n\nCommands:\n")
	for _, name := range names {
		fmt.Fprintf(os.Stderr, "  %-*s  %s\n", width, name, subcommands[name].desc)
	}
	fmt.Fprintf(os.Stderr, "\nRun 'ze-test <command> --help' for command-specific help.\n")
}
