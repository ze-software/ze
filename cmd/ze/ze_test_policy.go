// Design: docs/architecture/testing/ci-format.md — policy routing functional test runner

//go:build ze_test

package main

import (
	"fmt"
	"os"
)

func zeTestPolicyCmd(args []string) int {
	if err := zeTestRunCISubcommand(zeTestCIRunnerConfig{
		Name:        "policy",
		TestSubdir:  "policy",
		Description: "policy routing",
		Detail:      "Run policy routing functional tests (.ci files in test/policy/).\nCovers boot-time apply, table/next-hop actions, tcp-flags, tcp-mss, and reload.",
	}, args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}
