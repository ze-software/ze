// Design: docs/architecture/testing/ci-format.md — L2TP wire functional test runner

//go:build ze_test

package main

import (
	"fmt"
	"os"
)

func zeTestL2tpWireCmd(args []string) int {
	if err := zeTestRunCISubcommand(zeTestCIRunnerConfig{
		Name:        "l2tp-wire",
		TestSubdir:  "l2tp-wire",
		Description: "l2tp-wire",
		Detail:      "Run L2TP wire-level functional tests (.ci files in test/l2tp-wire/).\nCovers control message decode (SCCRQ) and truncated packet handling.",
	}, args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}
