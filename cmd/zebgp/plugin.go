package main

import (
	"fmt"
	"os"
)

// cmdPlugin dispatches to plugin subcommands.
func cmdPlugin(args []string) int {
	if len(args) < 1 {
		pluginUsage()
		return 1
	}

	switch args[0] {
	case "rr":
		return cmdPluginRR(args[1:])
	case "persist":
		return cmdPluginPersist(args[1:])
	case "help", "-h", "--help": //nolint:goconst // consistent with main.go, config.go
		pluginUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown plugin subcommand: %s\n", args[0])
		pluginUsage()
		return 1
	}
}

func pluginUsage() {
	fmt.Fprintf(os.Stderr, `Usage: zebgp plugin <subcommand>

Plugin Subcommands:
  rr           Run as Route Server (IX route server plugin)
  persist      Run as route persistence plugin (replays routes on reconnect)
  help         Show this help

The plugin subcommands run as API processes that communicate with the
ZeBGP router via stdin/stdout. They are spawned by the router based
on plugin configuration.

Example config:
  plugin rr {
      run "zebgp plugin rr";
      encoder json;
  }

  plugin persist {
      run "zebgp plugin persist";
      encoder json;
  }
`)
}
