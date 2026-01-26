// Package config provides the ze config subcommand.
package config

import (
	"fmt"
	"os"
)

// Run executes the config subcommand with the given arguments.
// Returns exit code.
func Run(args []string) int {
	if len(args) < 1 {
		usage()
		return 1
	}

	subcmd := args[0]
	subArgs := args[1:]

	switch subcmd {
	case "edit":
		return cmdEdit(subArgs)
	case "help", "-h", "--help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown config subcommand: %s\n", subcmd)
		usage()
		return 1
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: ze config <command> [options]

Configuration management commands.

Commands:
  edit <file>    Interactive configuration editor

Examples:
  ze config edit config.conf
  ze config edit /etc/ze/config.conf
`)
}
