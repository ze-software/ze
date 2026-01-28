package bgp

import (
	"flag"
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/plugin/rib"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// cmdPluginRib runs the RIB (Routing Information Base) plugin.
func cmdPluginRib(args []string) int {
	fs := flag.NewFlagSet("plugin rib", flag.ExitOnError)
	logLevel := fs.String("log-level", "disabled", "Log level (disabled, debug, info, warn, err)")
	showYang := fs.Bool("yang", false, "Output YANG schema and exit")
	_ = fs.Parse(args)

	// Output YANG schema if requested
	if *showYang {
		fmt.Print(ribYANG)
		return 0
	}

	// Configure plugin logger (CLI flag takes precedence, then env var hierarchy)
	rib.SetLogger(slogutil.PluginLogger("rib", *logLevel))

	manager := rib.NewRIBManager(os.Stdin, os.Stdout)
	return manager.Run()
}

// ribYANG is the YANG schema for the RIB plugin.
// TODO: Add actual YANG schema when plugin config schema is defined.
const ribYANG = ""
