package main

import (
	"os"

	"github.com/exa-networks/zebgp/pkg/api/rr"
)

// cmdAPIRR runs the Route Server API plugin.
func cmdAPIRR(_ []string) int {
	server := rr.NewRouteServer(os.Stdin, os.Stdout)
	return server.Run()
}
