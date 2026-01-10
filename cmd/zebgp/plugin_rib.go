package main

import (
	"os"

	"codeberg.org/thomas-mangin/zebgp/pkg/plugin/rib"
)

// cmdPluginRib runs the RIB (Routing Information Base) plugin.
func cmdPluginRib(_ []string) int {
	manager := rib.NewRIBManager(os.Stdin, os.Stdout)
	return manager.Run()
}
