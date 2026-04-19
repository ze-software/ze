// Design: docs/architecture/testing/ci-format.md -- Traffic functional test runner CLI

package main

import (
	"fmt"
	"os"
)

// trafficCmd dispatches the "ze-test traffic" subcommand. It runs the shared
// .ci runner over test/traffic/*.ci files. Tests validate that ze boots with
// a traffic-control config, applies the backend's Apply call, and reapplies
// on SIGHUP reload.
var _ = register("traffic", "Run traffic-control functional tests (test/traffic/*.ci)", trafficCmd)

func trafficCmd() int {
	if err := runCISubcommand(ciRunnerConfig{
		Name:        "traffic",
		TestSubdir:  "traffic",
		Description: "traffic",
		Detail:      "Run traffic-control functional tests (.ci files in test/traffic/).\nCovers component reactor wiring: boot-time apply and reload-time reapply.",
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err) //nolint:errcheck // terminal output
		return 1
	}
	return 0
}
