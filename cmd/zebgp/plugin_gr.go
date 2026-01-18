package main

import (
	"os"

	"codeberg.org/thomas-mangin/zebgp/pkg/plugin/gr"
)

// cmdPluginGR runs the Graceful Restart capability plugin.
// It receives per-peer restart-time config and registers GR capabilities.
func cmdPluginGR(_ []string) int {
	plugin := gr.NewGRPlugin(os.Stdin, os.Stdout)
	return plugin.Run()
}
