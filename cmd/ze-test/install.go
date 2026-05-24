// Design: plan/spec-install-0-umbrella.md — install functional test runner

package main

import (
	"fmt"
	"os"
)

var _ = register("install", "Run install provisioning functional tests (test/install/*.ci)", installCmd)

func installCmd() int {
	if err := runCISubcommand(ciRunnerConfig{
		Name:        "install",
		TestSubdir:  "install",
		Description: "install",
		Detail:      "Run install provisioning functional tests (.ci files in test/install/).\nTests ze install CLI, config validation, and provisioning server setup.",
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err) //nolint:errcheck // terminal output
		return 1
	}
	return 0
}
