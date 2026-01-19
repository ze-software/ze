// Command zebgp-test provides test utilities for ZeBGP.
//
// Subcommands:
//
//	zebgp-test run [type] [flags]     Run functional tests
//	zebgp-test syslog [flags]         Run syslog server for testing
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	if cmd == "-h" || cmd == "--help" || cmd == "help" {
		printUsage()
		return
	}

	// Shift args so subcommand sees itself as os.Args[0]
	os.Args = os.Args[1:]

	switch cmd {
	case "run":
		os.Exit(runCmd())
	case "syslog":
		os.Exit(syslogCmd())
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: zebgp-test <command> [options]

Commands:
  run       Run functional tests (encoding, plugin, decoding, parsing)
  syslog    Run syslog server for testing

Run 'zebgp-test <command> --help' for command-specific help.
`)
}
