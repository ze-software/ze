// Design: docs/architecture/testing/ci-format.md -- L2TP wire functional test runner CLI

package main

import (
	"fmt"
	"os"
)

// l2tpWireCmd dispatches the "ze-test l2tp-wire" subcommand. It runs the shared
// .ci runner over test/l2tp-wire/*.ci files. Tests validate L2TP wire-level
// decoding of control messages (SCCRQ, truncated packets).
var _ = register("l2tp-wire", "Run L2TP wire-level functional tests (test/l2tp-wire/*.ci)", l2tpWireCmd)

func l2tpWireCmd() int {
	if err := runCISubcommand(ciRunnerConfig{
		Name:        "l2tp-wire",
		TestSubdir:  "l2tp-wire",
		Description: "l2tp-wire",
		Detail:      "Run L2TP wire-level functional tests (.ci files in test/l2tp-wire/).\nCovers control message decode (SCCRQ) and truncated packet handling.",
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err) //nolint:errcheck // terminal output
		return 1
	}
	return 0
}
