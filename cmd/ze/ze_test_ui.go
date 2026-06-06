// Design: docs/architecture/testing/ci-format.md — UI functional test runner

//go:build ze_test

package main

import (
	"fmt"
	"os"
)

func zeTestUICmd(args []string) int {
	if err := zeTestRunCISubcommand(zeTestCIRunnerConfig{
		Name:        "ui",
		TestSubdir:  "ui",
		Description: "UI",
		Detail:      "Run UI functional tests (.ci files in test/ui/).\nTests config completion, editor CLI, and other UI-facing features.",
	}, args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}
