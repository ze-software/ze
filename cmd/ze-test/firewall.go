// Design: docs/architecture/testing/ci-format.md -- Firewall functional test runner CLI

package main

import (
	"fmt"
	"os"
)

// firewallCmd dispatches the "ze-test firewall" subcommand. It runs the shared
// .ci runner over test/firewall/*.ci files. Tests validate that ze boots with
// a firewall config, routes through the component reactor, and applies the
// backend (nft on Linux) without tripping a verify-time validator. Kernel-
// side `nft list tables` checks require CAP_NET_ADMIN and are tracked as a
// deferral on spec-fw-10.
func firewallCmd() int {
	if err := runCISubcommand(ciRunnerConfig{
		Name:        "firewall",
		TestSubdir:  "firewall",
		Description: "firewall",
		Detail:      "Run firewall functional tests (.ci files in test/firewall/).\nCovers component reactor wiring: boot-time parse -> validate -> Apply.",
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err) //nolint:errcheck // terminal output
		return 1
	}
	return 0
}
