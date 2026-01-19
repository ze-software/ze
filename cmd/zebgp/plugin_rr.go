package main

import (
	"os"

	"codeberg.org/thomas-mangin/zebgp/internal/plugin/rr"
)

// cmdPluginRR runs the Route Server plugin.
func cmdPluginRR(_ []string) int {
	server := rr.NewRouteServer(os.Stdin, os.Stdout)
	return server.Run()
}
