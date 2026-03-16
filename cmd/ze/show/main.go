// Design: docs/architecture/api/commands.md — show convenience command
// Related: ../run/main.go — ze run (all commands)
// Related: ../cli/main.go — CLI client and BuildCommandTree
// Related: ../internal/cmdutil/cmdutil.go — shared command utilities
//
// Package show provides the ze show subcommand.
// It discovers available read-only commands dynamically from the RPC registrations
// (same source as CLI autocomplete) and delegates execution to the CLI client.
// Only commands marked ReadOnly are exposed — for all commands use "ze run".
package show

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/cmd/ze/cli"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdutil"
)

// tree is built once from read-only RPCs.
var tree = cli.BuildCommandTree(true)

// Run executes the show subcommand with the given arguments.
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
		code := cmdutil.RunCommand(args, true, "show")
		if code == -1 {
			usage()
			return 1
		}
		return code
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: ze show <command> [options]

Show daemon state (read-only commands only).
For all commands including destructive operations, use "ze run".

Available commands:
`)

	cmdutil.PrintCommandList(tree)

	fmt.Fprintln(os.Stderr)
}
