// Design: docs/architecture/testing/ci-format.md — flow-export functional test runner

//go:build ze_test

package main

import (
	"fmt"
	"os"
)

func zeTestFlowExportCmd(args []string) int {
	if err := zeTestRunCISubcommand(zeTestCIRunnerConfig{
		Name:        "flow-export",
		TestSubdir:  "flow-export",
		Description: "flow-export",
		Detail:      "Run flow-export functional tests (.ci files in test/flow-export/).\nCovers sFlow v5, NetFlow v9, and IPFIX counter export over UDP, the show flow-export handler, packet-sampling wiring, and reload-time reconfiguration.",
	}, args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}
