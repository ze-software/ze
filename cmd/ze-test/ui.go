// Design: docs/architecture/testing/ci-format.md — UI test runner CLI

package main

import (
	"errors"
	"fmt"
	"os"
)

// ErrTestsFailed is returned when one or more tests fail.
var ErrTestsFailed = errors.New("tests failed")

var _ = register("ui", "Run UI functional tests (completion, CLI)", uiCmd)

func uiCmd() int {
	if err := runCISubcommand(ciRunnerConfig{
		Name:        "ui",
		TestSubdir:  "ui",
		Description: "UI",
		Detail:      "Run UI functional tests (.ci files in test/ui/).\nTests config completion, editor CLI, and other UI-facing features.",
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err) //nolint:errcheck // terminal output
		return 1
	}
	return 0
}
