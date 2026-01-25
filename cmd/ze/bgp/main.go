// Package bgp provides the Ze BGP daemon subcommand.
package bgp

import (
	"fmt"
	"os"
	"strings"
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
		return cmdServer(args[1:])
	case "cli":
		return cmdCLI(args[1:])
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

	// If arg looks like a config file, start daemon
	if looksLikeConfig(arg) {
		return cmdServer(args)
	}

	// Unknown command
	fmt.Fprintf(os.Stderr, "unknown command: %s\n", arg)
	usage()
	return 1
}

// looksLikeConfig returns true if the argument looks like a config file path.
func looksLikeConfig(arg string) bool {
	// Check for common config extensions
	if strings.HasSuffix(arg, ".conf") ||
		strings.HasSuffix(arg, ".cfg") ||
		strings.HasSuffix(arg, ".yaml") ||
		strings.HasSuffix(arg, ".yml") ||
		strings.HasSuffix(arg, ".json") {
		return true
	}

	// Check if it's a path (contains / or starts with .)
	if strings.Contains(arg, "/") || strings.HasPrefix(arg, ".") {
		// Check if file exists
		if _, err := os.Stat(arg); err == nil {
			return true
		}
	}

	return false
}

func usage() {
	fmt.Fprintf(os.Stderr, `ze bgp - BGP daemon

Usage:
  ze bgp <config>              Start daemon with config file
  ze bgp <command> [options]   Execute command

Commands:
  server <config>      Start the BGP daemon (same as ze bgp <config>)
  cli                  Interactive CLI with autocomplete
  cli --run <command>  Execute API command on running daemon
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
  ze bgp /etc/ze/bgp/config.conf
  ze bgp server /etc/ze/bgp/config.conf
  ze bgp cli --run "peer list"
  ze bgp cli
  ze bgp config edit /etc/ze/bgp/config.conf
  ze bgp validate /etc/ze/bgp/config.conf
  ze bgp config check config.conf
  ze bgp config migrate old.conf -o new.conf

For ExaBGP compatibility tools, see 'ze exabgp help'.
`)
}
