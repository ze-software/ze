// Design: docs/architecture/testing/ci-format.md -- Static route functional test runner CLI

package main

import (
	"fmt"
	"os"
)

// staticCmd dispatches the "ze-test static" subcommand. It runs the shared
// .ci runner over test/static/*.ci files. Tests validate that ze boots with
// a static route config, applies routes, and handles reload add/remove.
var _ = register("static", "Run static route functional tests (test/static/*.ci)", staticCmd)

func staticCmd() int {
	if err := runCISubcommand(ciRunnerConfig{
		Name:        "static",
		TestSubdir:  "static",
		Description: "static",
		Detail:      "Run static route functional tests (.ci files in test/static/).\nCovers boot-time apply, reload add/remove, and show output.",
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err) //nolint:errcheck // terminal output
		return 1
	}
	return 0
}
