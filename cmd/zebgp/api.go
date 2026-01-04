package main

import (
	"fmt"
	"os"
)

// cmdAPI dispatches to API subcommands.
func cmdAPI(args []string) int {
	if len(args) < 1 {
		apiUsage()
		return 1
	}

	switch args[0] {
	case "rr":
		return cmdAPIRR(args[1:])
	case "persist":
		return cmdAPIPersist(args[1:])
	case "help", "-h", "--help": //nolint:goconst // consistent with main.go, config.go
		apiUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown api subcommand: %s\n", args[0])
		apiUsage()
		return 1
	}
}

func apiUsage() {
	fmt.Fprintf(os.Stderr, `Usage: zebgp api <subcommand>

API Subcommands:
  rr           Run as Route Server (IX route server plugin)
  persist      Run as route persistence plugin (replays routes on reconnect)
  help         Show this help

The api subcommands run as API processes that communicate with the
ZeBGP router via stdin/stdout. They are spawned by the router based
on process configuration.

Example config:
  process rr {
      run "zebgp api rr";
      encoder json;
  }

  process persist {
      run "zebgp api persist";
      encoder json;
  }
`)
}
