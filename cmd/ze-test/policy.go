// Design: docs/architecture/testing/ci-format.md -- Policy routing functional test runner CLI

package main

import (
	"fmt"
	"os"
)

var _ = register("policy", "Run policy routing functional tests (test/policy/*.ci)", policyCmd)

func policyCmd() int {
	if err := runCISubcommand(ciRunnerConfig{
		Name:        "policy",
		TestSubdir:  "policy",
		Description: "policy routing",
		Detail:      "Run policy routing functional tests (.ci files in test/policy/).\nCovers boot-time apply, table/next-hop actions, tcp-flags, tcp-mss, and reload.",
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err) //nolint:errcheck // terminal output
		return 1
	}
	return 0
}
