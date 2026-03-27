// Design: docs/architecture/config/environment.md — ze env CLI command
//
// Package environ provides the "ze env" subcommand to list and inspect
// Ze environment variables with their defaults and current values.
package environ

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

// Run executes the env subcommand with the given arguments.
// Returns exit code.
func Run(args []string) int {
	if len(args) == 0 {
		return showAll(false)
	}

	switch args[0] {
	case "help", "-h", "--help":
		usage()
		return 0
	case "list":
		return cmdList(args[1:])
	case "get":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "error: ze env get requires a key\n")
			return 1
		}
		return showOne(args[1])
	}

	// Default: treat as a key to look up
	return showOne(args[0])
}

// cmdList parses flags for "ze env list" and displays the table.
func cmdList(args []string) int {
	fs := flag.NewFlagSet("ze env list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	verbose := fs.Bool("v", false, "show current effective values")
	fs.BoolVar(verbose, "verbose", false, "show current effective values")

	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "error: unexpected argument %q\n", fs.Arg(0))
		return 1
	}
	return showAll(*verbose)
}

// showAll displays all known env vars in a table.
// If verbose, also shows the current effective value.
func showAll(verbose bool) int {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)

	if verbose {
		printRow(w, "KEY", "TYPE", "DEFAULT", "CURRENT", "DESCRIPTION")
		printRow(w, "---", "----", "-------", "-------", "-----------")
	} else {
		printRow(w, "KEY", "TYPE", "DEFAULT", "DESCRIPTION")
		printRow(w, "---", "----", "-------", "-----------")
	}

	for _, e := range env.Entries() {
		dflt := valueOrDash(e.Default)
		if verbose {
			current := currentValue(e.Key)
			printRow(w, e.Key, e.Type, dflt, current, e.Description)
		} else {
			printRow(w, e.Key, e.Type, dflt, e.Description)
		}
	}

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

// printRow writes a tab-separated row to w.
func printRow(w *tabwriter.Writer, cols ...string) {
	if _, err := fmt.Fprintln(w, strings.Join(cols, "\t")); err != nil {
		return
	}
}

// currentValue returns the effective value for an env var key.
// Skips pattern keys like "ze.log.<subsystem>" that contain angle brackets.
func currentValue(key string) string {
	if strings.Contains(key, "<") {
		return "-"
	}
	return valueOrDash(env.Get(key))
}

// showOne displays a single env var with its current value and metadata.
func showOne(key string) int {
	// Normalize key to dot notation for matching
	normalized := strings.ReplaceAll(strings.ToLower(key), "_", ".")

	for _, e := range env.AllEntries() {
		if e.Key != normalized && e.Key != key {
			continue
		}

		current := currentValue(e.Key)
		fmt.Printf("Key:         %s\n", e.Key)
		fmt.Printf("Type:        %s\n", e.Type)
		fmt.Printf("Default:     %s\n", valueOrDash(e.Default))
		fmt.Printf("Current:     %s\n", valueOrDash(current))
		fmt.Printf("Description: %s\n", e.Description)
		if e.Private {
			fmt.Printf("Private:     yes\n")
		}

		// Show all notation forms
		under := strings.ReplaceAll(e.Key, ".", "_")
		fmt.Printf("\nAccepted forms:\n")
		fmt.Printf("  %s\n", e.Key)
		fmt.Printf("  %s\n", under)
		fmt.Printf("  %s\n", strings.ToUpper(under))
		return 0
	}

	fmt.Fprintf(os.Stderr, "error: unknown env var %q\n", key)
	return 1
}

func valueOrDash(v string) string {
	if v == "" {
		return "-"
	}
	return v
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: ze env [command]

Commands:
  list [-v]     List all Ze environment variables (default)
  get <key>     Show details for a specific env var
  help          Show this help

The -v flag shows the current effective value alongside defaults.

All Ze env vars accept three notation forms:
  ze.foo.bar         (dot notation, highest priority)
  ze_foo_bar         (lowercase underscore)
  ZE_FOO_BAR         (uppercase underscore, shell convention)

Examples:
  ze env                        # List all env vars
  ze env list -v                # List with current values
  ze env get ze.log             # Show details for ze.log
  ze env ZE_PLUGIN_HUB_HOST    # Look up by any notation
`)
}
