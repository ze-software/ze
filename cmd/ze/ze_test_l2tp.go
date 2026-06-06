// Design: docs/architecture/testing/ci-format.md — L2TP functional test runner

//go:build ze_test

package main

import (
	"fmt"
	"os"
)

func zeTestL2tpCmd(args []string) int {
	if err := zeTestRunCISubcommand(zeTestCIRunnerConfig{
		Name:        "l2tp",
		TestSubdir:  "l2tp",
		Description: "l2tp",
		Detail:      "Run L2TPv2 functional tests (.ci files in test/l2tp/).\nCovers listener binding, control-connection handshake, challenge/response, hello keepalive, tie-breaker, and teardown.",
	}, args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}
