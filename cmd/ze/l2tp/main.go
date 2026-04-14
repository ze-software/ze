// Design: docs/architecture/core-design.md -- l2tp offline CLI
//
// Package l2tp provides the `ze l2tp` subcommand. Phase 1 scope is the
// offline `decode` verb: reads a hex-encoded L2TPv2 control message from
// stdin, prints a JSON representation to stdout.
package l2tp

import (
	"fmt"
	"os"
)

// Run executes the l2tp subcommand. Returns exit code.
func Run(args []string) int {
	if len(args) < 1 {
		usage()
		return 1
	}

	subcmd := args[0]
	subArgs := args[1:]

	if subcmd == "help" || subcmd == "-h" || subcmd == "--help" { //nolint:goconst // consistent pattern across cmd files
		usage()
		return 0
	}

	switch subcmd {
	case "decode":
		return cmdDecode(subArgs)
	default:
		fmt.Fprintf(os.Stderr, "error: unknown l2tp subcommand: %s\n", subcmd)
		usage()
		return 1
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: ze l2tp <subcommand> [flags]")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "subcommands:")
	fmt.Fprintln(os.Stderr, "  decode    Decode a hex L2TPv2 control message from stdin to JSON")
	fmt.Fprintln(os.Stderr, "  help      Show this message")
}
