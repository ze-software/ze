// Package bgp provides the Ze BGP daemon subcommand.
package bgp

import (
	"fmt"
	"os"
)

const version = "0.1.0"

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
	case "server":
		// Server command removed - config parsing happens at root level
		fmt.Fprintf(os.Stderr, "error: 'ze bgp server' is deprecated\n")
		fmt.Fprintf(os.Stderr, "use 'ze config.conf' to start the BGP daemon\n")
		return 1
	case "validate":
		return cmdValidate(args[1:])
	case "decode":
		return cmdDecode(args[1:])
	case "encode":
		return cmdEncode(args[1:])
	case "config":
		return cmdConfig(args[1:])
	case "plugin":
		return cmdPlugin(args[1:])
	case "config-dump":
		return cmdConfigDump(args[1:])
	case "schema":
		return cmdSchema(args[1:])
	case "version":
		fmt.Printf("ze bgp %s\n", version)
		return 0
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
	fmt.Fprintf(os.Stderr, `ze bgp - BGP tools

Usage:
  ze bgp <command> [options]   Execute command

To start the BGP daemon, use: ze <config>

Commands:
  validate <config>    Validate configuration file
  decode <hex>         Decode BGP message from hex to JSON
  encode <route>       Encode API route command to BGP hex
  config <subcommand>  Configuration management (edit, check, migrate, fmt)
  config-dump <config> Dump parsed config (debug tool)
  plugin <subcommand>  Plugin system (rr for route server)
  schema <subcommand>  Schema discovery (list, show, handlers, protocol)
  version              Show version
  help                 Show this help

Config Subcommands:
  config edit <file>          Interactive configuration editor
  config check <file>         Show version and what needs migration
  config migrate <file>       Convert config to current format
  config migrate <file> -o <output>    Write to file
  config migrate <file> --in-place     Convert in place (with backup)
  config fmt <file>           Format and normalize config

Examples:
  ze config.conf                          # Start BGP daemon
  ze cli --run "peer list"                # Use ze cli (not ze bgp cli)
  ze bgp validate /etc/ze/bgp/config.conf
  ze bgp config edit /etc/ze/bgp/config.conf
  ze bgp config check config.conf
  ze bgp config migrate old.conf -o new.conf
`)
}
