// Design: docs/architecture/testing/ci-format.md — firewall functional test runner

//go:build ze_test

package main

import (
	"fmt"
	"os"
)

func zeTestFirewallCmd(args []string) int {
	if err := zeTestRunCISubcommand(zeTestCIRunnerConfig{
		Name:        "firewall",
		TestSubdir:  "firewall",
		Description: "firewall",
		Detail:      "Run firewall functional tests (.ci files in test/firewall/).\nCovers component reactor wiring: boot-time parse -> validate -> Apply.",
	}, args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}
