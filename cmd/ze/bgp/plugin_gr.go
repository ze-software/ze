package bgp

import (
	"flag"
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/plugin/gr"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// cmdPluginGR runs the Graceful Restart capability plugin.
// It receives per-peer restart-time config and registers GR capabilities.
func cmdPluginGR(args []string) int {
	fs := flag.NewFlagSet("plugin gr", flag.ExitOnError)
	logLevel := fs.String("log-level", "disabled", "Log level (disabled, debug, info, warn, err)")
	showYang := fs.Bool("yang", false, "Output YANG schema and exit")
	_ = fs.Parse(args)

	// Output YANG schema if requested
	if *showYang {
		fmt.Print(grYANG)
		return 0
	}

	// Configure plugin logger (CLI flag takes precedence, then env var hierarchy)
	gr.SetLogger(slogutil.PluginLogger("gr", *logLevel))

	plugin := gr.NewGRPlugin(os.Stdin, os.Stdout)
	return plugin.Run()
}

// grYANG is the YANG schema for the GR plugin.
// TODO: Add actual YANG schema when plugin config schema is defined.
const grYANG = ""
