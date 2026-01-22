// Command ze-test provides test utilities for Ze BGP.
//
// Subcommands:
//
//	ze-test bgp [type] [flags]     Run BGP functional tests
//	ze-test syslog [flags]         Run syslog server for testing
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
	case "bgp":
		os.Exit(bgpCmd())
	case "syslog":
		os.Exit(syslogCmd())
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: ze-test <command> [options]

Commands:
  bgp       Run BGP functional tests (encoding, plugin, decoding, parsing)
  syslog    Run syslog server for testing

Run 'ze-test <command> --help' for command-specific help.
`)
}
