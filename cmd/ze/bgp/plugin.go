package bgp

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
	case "rib":
		return cmdPluginRib(args[1:])
	case "gr":
		return cmdPluginGR(args[1:])
	case "hostname":
		return cmdPluginHostname(args[1:])
	case "test":
		return cmdPluginTest(args[1:])
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
	fmt.Fprintf(os.Stderr, `Usage: ze bgp plugin <subcommand>

Plugin Subcommands:
  rr           Run as Route Server (IX route server plugin)
  rib          Run as RIB plugin (tracks Adj-RIB-In/Out, replays on reconnect)
  gr           Run as Graceful Restart capability plugin
  hostname     Run as Hostname (FQDN) capability plugin
  test         Test plugin YANG schema and config delivery (debugging)
  help         Show this help

The plugin subcommands run as API processes that communicate with ze
router via stdin/stdout. They are spawned by the router based
on plugin configuration.

Example config:
  plugin rr {
      run "ze bgp plugin rr";
      encoder json;
  }

  plugin rib {
      run "ze bgp plugin rib";
      encoder json;
  }

Testing:
  ze bgp plugin test --plugin ze.hostname --schema config.conf
  ze bgp plugin test --plugin ze.hostname --tree config.conf
  ze bgp plugin test --plugin ze.hostname --json config.conf
`)
}
