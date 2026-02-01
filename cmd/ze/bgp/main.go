// Package bgp provides the Ze BGP daemon subcommand.
package bgp

import (
	"fmt"
	"os"
)

// Run executes the bgp subcommand with the given arguments.
// Returns exit code.
func Run(args []string) int {
	// Check for child mode first (hub integration)
	if isChildMode(args) {
		return runChildModeWithArgs(args)
	}

	if len(args) < 1 {
		usage()
		return 1
	}

	arg := args[0]

	// Check for known commands first
	switch arg {
	case "decode":
		return cmdDecode(args[1:])
	case "encode":
		return cmdEncode(args[1:])
	case "plugin":
		return cmdPlugin(args[1:])
	case "help", "-h", "--help": //nolint:goconst // consistent pattern across cmd files
		usage()
		return 0
	}

	// Unknown command
	fmt.Fprintf(os.Stderr, "unknown command: %s\n", arg)
	usage()
	return 1
}

func usage() {
	fmt.Fprintf(os.Stderr, `ze bgp - BGP protocol tools

Usage:
  ze bgp <command> [options]

Commands:
  decode <hex>         Decode BGP message from hex to JSON
  encode <route>       Encode API route command to BGP hex
  plugin <subcommand>  Plugin system (rr, rib, gr, etc.)
  help                 Show this help

See also:
  ze validate          Validate configuration
  ze config            Configuration management
  ze schema            Schema discovery
  ze version           Show version

Examples:
  ze bgp decode update <hex>
  ze bgp encode "nlri ipv4/unicast add 10.0.0.0/24"
  ze bgp plugin rr --help
`)
}
