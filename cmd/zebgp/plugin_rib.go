package main

import (
	"flag"
	"os"

	"codeberg.org/thomas-mangin/zebgp/pkg/plugin/rib"
	"codeberg.org/thomas-mangin/zebgp/pkg/slogutil"
)

// cmdPluginRib runs the RIB (Routing Information Base) plugin.
func cmdPluginRib(args []string) int {
	fs := flag.NewFlagSet("plugin rib", flag.ExitOnError)
	logLevel := fs.String("log-level", "disabled", "Log level (disabled, debug, info, warn, err)")
	_ = fs.Parse(args)

	// Configure plugin logger from CLI flag
	rib.SetLogger(slogutil.LoggerWithLevel("rib", *logLevel))

	manager := rib.NewRIBManager(os.Stdin, os.Stdout)
	return manager.Run()
}
