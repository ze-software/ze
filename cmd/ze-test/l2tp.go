// Design: docs/architecture/testing/ci-format.md -- L2TP functional test runner CLI

package main

import (
	"fmt"
	"os"
)

// l2tpCmd dispatches the "ze-test l2tp" subcommand. It runs the shared .ci
// runner over test/l2tp/*.ci files.
var _ = register("l2tp", "Run L2TPv2 functional tests (listener, tunnel FSM, handshake)", l2tpCmd)

func l2tpCmd() int {
	if err := runCISubcommand(ciRunnerConfig{
		Name:        "l2tp",
		TestSubdir:  "l2tp",
		Description: "l2tp",
		Detail:      "Run L2TPv2 functional tests (.ci files in test/l2tp/).\nCovers listener binding, control-connection handshake, challenge/response, hello keepalive, tie-breaker, and teardown.",
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err) //nolint:errcheck // terminal output
		return 1
	}
	return 0
}
