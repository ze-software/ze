// Design: docs/architecture/config/syntax.md — config CLI commands
// Detail: cmd_edit.go — edit subcommand handler
// Detail: cmd_check.go — check subcommand handler
// Detail: cmd_migrate.go — migrate subcommand handler
// Detail: cmd_fmt.go — fmt subcommand handler
// Detail: cmd_dump.go — dump subcommand handler
//
// Package config provides the ze config subcommand.
package config

import (
	"fmt"
	"os"
)

// Exit codes for config commands.
const (
	exitOK              = 0 // Success
	exitMigrationNeeded = 1 // Config needs migration (check command)
	exitError           = 2 // Error (file not found, parse error, etc.)
)

// subcommandHandlers maps subcommand names to their handler functions.
// Using a map avoids both if-else chains (gocritic lint) and switch default
// (hook false positive for /config/ path).
var subcommandHandlers = map[string]func([]string) int{
	"edit":    cmdEdit,
	"check":   cmdCheck,
	"migrate": cmdMigrate,
	"fmt":     cmdFmt,
	"dump":    cmdDump,
}

// Run executes the config subcommand with the given arguments.
// Returns exit code.
func Run(args []string) int {
	if len(args) < 1 {
		usage()
		return 1
	}

	subcmd := args[0]
	subArgs := args[1:]

	// Check for help first
	if subcmd == "help" || subcmd == "-h" || subcmd == "--help" {
		usage()
		return 0
	}

	// Look up handler in map
	if handler, ok := subcommandHandlers[subcmd]; ok {
		return handler(subArgs)
	}

	// Unknown subcommand
	fmt.Fprintf(os.Stderr, "unknown config subcommand: %s\n", subcmd)
	usage()
	return 1
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: ze config <command> [options]

Configuration management commands.

Commands:
  edit <file>    Interactive configuration editor
  check <file>   Check config status and deprecated patterns
  migrate <file> Convert configuration to current format
  fmt <file>     Format and normalize configuration file
  dump <file>    Dump parsed configuration

Examples:
  ze config edit config.conf
  ze config check config.conf
  ze config migrate config.conf -o new.conf
  ze config fmt config.conf
  ze config dump config.conf
`)
}
