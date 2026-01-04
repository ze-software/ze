// Package main provides the ZeBGP daemon entry point.
package main

import (
	"fmt"
	"os"
	"strings"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	arg := os.Args[1]

	// Check for known commands first
	switch arg {
	case "server":
		os.Exit(cmdServer(os.Args[2:]))
	case "run":
		os.Exit(cmdRun(os.Args[2:]))
	case "cli":
		os.Exit(cmdCLI(os.Args[2:]))
	case "validate":
		os.Exit(cmdValidate(os.Args[2:]))
	case "decode":
		os.Exit(cmdDecode(os.Args[2:]))
	case "encode":
		os.Exit(cmdEncode(os.Args[2:]))
	case "config":
		os.Exit(cmdConfig(os.Args[2:]))
	case "api":
		os.Exit(cmdAPI(os.Args[2:]))
	case "config-dump":
		os.Exit(cmdConfigDump(os.Args[2:]))
	case "version":
		fmt.Printf("zebgp %s\n", version)
		os.Exit(0)
	case "help", "-h", "--help": //nolint:goconst // consistent pattern across cmd files
		usage()
		os.Exit(0)
	}

	// If arg looks like a config file, start daemon
	if looksLikeConfig(arg) {
		os.Exit(cmdServer(os.Args[1:]))
	}

	// Unknown command
	fmt.Fprintf(os.Stderr, "unknown command: %s\n", arg)
	usage()
	os.Exit(1)
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
	fmt.Fprintf(os.Stderr, `zebgp - BGP daemon

Usage:
  zebgp <config>              Start daemon with config file
  zebgp <command> [options]   Execute command

Commands:
  server <config>      Start the BGP daemon (same as zebgp <config>)
  run <command>        Execute API command on running daemon
  cli                  Interactive CLI with autocomplete
  validate <config>    Validate configuration file
  decode <hex>         Decode BGP message from hex to JSON
  encode <route>       Encode API route command to BGP hex
  config <subcommand>  Configuration management (edit, check, migrate, fmt)
  config-dump <config> Dump parsed config (debug tool)
  api <subcommand>     API plugins (rr for route server)
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
  zebgp /etc/zebgp/config.conf
  zebgp server /etc/zebgp/config.conf
  zebgp run peer list
  zebgp cli
  zebgp config edit /etc/zebgp/config.conf
  zebgp validate /etc/zebgp/config.conf
  zebgp config check config.conf
  zebgp config migrate old.conf -o new.conf
`)
}
