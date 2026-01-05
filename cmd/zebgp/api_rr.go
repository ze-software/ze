package main

import (
	"os"

	"codeberg.org/thomas-mangin/zebgp/pkg/api/rr"
)

// cmdAPIRR runs the Route Server API plugin.
func cmdAPIRR(_ []string) int {
	server := rr.NewRouteServer(os.Stdin, os.Stdout)
	return server.Run()
}
