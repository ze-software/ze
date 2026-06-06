// Design: docs/architecture/testing/ci-format.md — install functional test runner

//go:build ze_test

package main

import (
	"fmt"
	"os"
)

func zeTestInstallCmd(args []string) int {
	if err := zeTestRunCISubcommand(zeTestCIRunnerConfig{
		Name:        "install",
		TestSubdir:  "install",
		Description: "install",
		Detail:      "Run install provisioning functional tests (.ci files in test/install/).\nTests ze install CLI, config validation, and provisioning server setup.",
	}, args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}
