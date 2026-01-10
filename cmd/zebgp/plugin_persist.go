package main

import (
	"os"

	"codeberg.org/thomas-mangin/zebgp/pkg/plugin/persist"
)

// cmdPluginPersist runs the route persistence plugin.
func cmdPluginPersist(_ []string) int {
	server := persist.NewPersister(os.Stdin, os.Stdout)
	return server.Run()
}
