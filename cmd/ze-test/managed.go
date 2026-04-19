// Design: docs/architecture/testing/ci-format.md — managed config test runner CLI

package main

import (
	"fmt"
	"os"
)

var _ = register("managed", "Run managed config tests (hub, auth, fleet)", managedCmd)

func managedCmd() int {
	if err := runCISubcommand(ciRunnerConfig{
		Name:        "managed",
		TestSubdir:  "managed",
		Description: "managed",
		Detail:      "Run managed config functional tests (.ci files in test/managed/).\nTests fleet management: hub config, per-client auth, managed boot, config change.",
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err) //nolint:errcheck // terminal output
		return 1
	}
	return 0
}
