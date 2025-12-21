package main

import (
	"fmt"
	"os"
)

// cmdConfig handles "zebgp config <subcommand>" commands.
func cmdConfig(args []string) int {
	if len(args) < 1 {
		configUsage()
		return exitError
	}

	subcmd := args[0]
	subArgs := args[1:]

	switch subcmd {
	case "check":
		return cmdConfigCheckCLI(subArgs)
	case "migrate":
		return cmdConfigMigrateCLI(subArgs)
	case "fmt":
		return cmdConfigFmtCLI(subArgs)
	case "help", "-h", "--help":
		configUsage()
		return exitOK
	default:
		fmt.Fprintf(os.Stderr, "unknown config subcommand: %s\n", subcmd)
		configUsage()
		return exitError
	}
}

func configUsage() {
	fmt.Fprintf(os.Stderr, `Usage: zebgp config <command> [options]

Configuration management commands.

Commands:
  check <file>         Show config version and what needs migration
  migrate <file>       Convert config to current format
  fmt <file>           Format and normalize config

Examples:
  zebgp config check config.conf
  zebgp config migrate config.conf
  zebgp config migrate config.conf -o new.conf
  zebgp config migrate config.conf --in-place
  zebgp config fmt config.conf
  zebgp config fmt -w config.conf
`)
}
