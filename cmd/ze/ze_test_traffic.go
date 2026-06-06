// Design: docs/architecture/testing/ci-format.md — traffic functional test runner

//go:build ze_test

package main

import (
	"fmt"
	"os"
)

func zeTestTrafficCmd(args []string) int {
	if err := zeTestRunCISubcommand(zeTestCIRunnerConfig{
		Name:        "traffic",
		TestSubdir:  "traffic",
		Description: "traffic",
		Detail:      "Run traffic-control functional tests (.ci files in test/traffic/).\nCovers component reactor wiring: boot-time apply and reload-time reapply.",
	}, args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}
