// Design: docs/architecture/testing/ci-format.md -- Flow export functional test runner CLI

package main

import (
	"fmt"
	"os"
)

// flowExportCmd dispatches the "ze-test flow-export" subcommand. It runs the
// shared .ci runner over test/flow-export/*.ci files. Tests validate that ze
// boots with a flow-export config, opens the configured collector socket, and
// emits sFlow v5 / NetFlow v9 / IPFIX datagrams over UDP, plus the show
// handler and SIGHUP reload wiring.
var _ = register("flow-export", "Run flow-export functional tests (test/flow-export/*.ci)", flowExportCmd)

func flowExportCmd() int {
	if err := runCISubcommand(ciRunnerConfig{
		Name:        "flow-export",
		TestSubdir:  "flow-export",
		Description: "flow-export",
		Detail:      "Run flow-export functional tests (.ci files in test/flow-export/).\nCovers sFlow v5, NetFlow v9, and IPFIX counter export over UDP, the show flow-export handler, packet-sampling wiring, and reload-time reconfiguration.",
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err) //nolint:errcheck // terminal output
		return 1
	}
	return 0
}
