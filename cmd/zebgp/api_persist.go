package main

import (
	"os"

	"github.com/exa-networks/zebgp/pkg/api/persist"
)

// cmdAPIPersist runs the route persistence API plugin.
func cmdAPIPersist(_ []string) int {
	server := persist.NewPersister(os.Stdin, os.Stdout)
	return server.Run()
}
