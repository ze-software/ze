// Design: docs/architecture/api/commands.md — run convenience command
// Related: ../show/main.go — ze show (read-only commands)
// Related: ../cli/main.go — CLI client and BuildCommandTree
// Related: ../internal/cmdutil/cmdutil.go — shared command utilities
//
// Package run provides the ze run subcommand.
// It discovers available commands dynamically from the RPC registrations
// (same source as CLI autocomplete) and delegates execution to the CLI client.
// Unlike "ze show" which only exposes read-only commands, "ze run" exposes all commands.
package run

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/cmd/ze/cli"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdutil"
)

// tree is built once from all RPCs.
var tree = cli.BuildCommandTree(false)

// Run executes the run subcommand with the given arguments.
// Returns exit code.
func Run(args []string) int {
	if len(args) == 0 {
		usage()
		return 1
	}

	switch args[0] {
	case "help", "-h", "--help":
		usage()
		return 0
	default:
		code := cmdutil.RunCommand(args, false, "run")
		if code == -1 {
			usage()
			return 1
		}
		return code
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: ze run <command> [options]

Execute daemon commands. All registered commands are available.
For read-only commands only, use "ze show".

Options:
  --user <username>  Simulate authenticated user for authorization testing

Available commands:
`)

	cmdutil.PrintCommandList(tree)

	fmt.Fprintln(os.Stderr)
}
