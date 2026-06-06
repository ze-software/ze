// Design: docs/architecture/testing/ci-format.md — managed config test runner

//go:build ze_test

package main

import (
	"fmt"
	"os"
)

func zeTestManagedCmd(args []string) int {
	if err := zeTestRunCISubcommand(zeTestCIRunnerConfig{
		Name:            "managed",
		TestSubdir:      "managed",
		Description:     "managed",
		Detail:          "Run managed config functional tests (.ci files in test/managed/).\nTests fleet management: hub config, per-client auth, managed boot, config change.",
		DefaultParallel: 1,
	}, args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}
