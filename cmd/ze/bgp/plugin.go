package bgp

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/plugin/registry"
)

// cmdPlugin dispatches to plugin subcommands via the registry.
func cmdPlugin(args []string) int {
	if len(args) < 1 {
		pluginUsage()
		return 1
	}

	switch args[0] {
	case "test":
		// Test is a debugging tool, not a real plugin.
		return cmdPluginTest(args[1:])
	case "help", "-h", "--help": //nolint:goconst // consistent with main.go, config.go
		pluginUsage()
		return 0
	}

	// Look up in registry.
	reg := registry.Lookup(args[0])
	if reg == nil {
		fmt.Fprintf(os.Stderr, "unknown plugin subcommand: %s\n", args[0])
		pluginUsage()
		return 1
	}
	return reg.CLIHandler(args[1:])
}

func pluginUsage() {
	fmt.Fprintf(os.Stderr, `Usage: ze bgp plugin <subcommand>

Plugin Subcommands:
`)
	if err := registry.WriteUsage(os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "  (error listing plugins: %v)\n", err)
	}
	fmt.Fprintf(os.Stderr, `  test         Test plugin YANG schema and config delivery (debugging)
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
