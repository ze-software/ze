package bgp

import (
	"fmt"
	"os"
)

// cmdConfig handles "ze bgp config <subcommand>" commands.
func cmdConfig(args []string) int {
	if len(args) < 1 {
		configUsage()
		return exitError
	}

	subcmd := args[0]
	subArgs := args[1:]

	switch subcmd {
	case "edit":
		return cmdEdit(subArgs)
	case "check":
		return cmdConfigCheckCLI(subArgs)
	case "migrate":
		return cmdConfigMigrateCLI(subArgs)
	case "fmt":
		return cmdConfigFmtCLI(subArgs)
	case "help", "-h", "--help": //nolint:goconst // consistent pattern across cmd files
		configUsage()
		return exitOK
	default:
		fmt.Fprintf(os.Stderr, "unknown config subcommand: %s\n", subcmd)
		configUsage()
		return exitError
	}
}

func configUsage() {
	fmt.Fprintf(os.Stderr, `Usage: ze bgp config <command> [options]

Configuration management commands.

Commands:
  edit <file>          Interactive configuration editor
  check <file>         Show config version and what needs migration
  migrate <file>       Convert config to current format
  fmt <file>           Format and normalize config

Examples:
  ze bgp config edit config.conf
  ze bgp config check config.conf
  ze bgp config migrate config.conf
  ze bgp config migrate config.conf -o new.conf
  ze bgp config migrate config.conf --in-place
  ze bgp config fmt config.conf
  ze bgp config fmt -w config.conf
`)
}
