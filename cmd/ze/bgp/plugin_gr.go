package bgp

import (
	"flag"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/plugin/gr"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// cmdPluginGR runs the Graceful Restart capability plugin.
// It receives per-peer restart-time config and registers GR capabilities.
func cmdPluginGR(args []string) int {
	fs := flag.NewFlagSet("plugin gr", flag.ExitOnError)
	logLevel := fs.String("log-level", "disabled", "Log level (disabled, debug, info, warn, err)")
	_ = fs.Parse(args)

	// Configure plugin logger from CLI flag
	gr.SetLogger(slogutil.LoggerWithLevel("gr", *logLevel))

	plugin := gr.NewGRPlugin(os.Stdin, os.Stdout)
	return plugin.Run()
}
