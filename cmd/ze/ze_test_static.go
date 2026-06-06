// Design: docs/architecture/testing/ci-format.md — static route functional test runner

//go:build ze_test

package main

import (
	"fmt"
	"os"
)

func zeTestStaticCmd(args []string) int {
	if err := zeTestRunCISubcommand(zeTestCIRunnerConfig{
		Name:        "static",
		TestSubdir:  "static",
		Description: "static",
		Detail:      "Run static route functional tests (.ci files in test/static/).\nCovers boot-time apply, reload add/remove, and show output.",
	}, args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}
