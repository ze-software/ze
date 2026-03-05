// Design: docs/architecture/testing/ci-format.md — test runner CLI
//
// Command ze-test provides test utilities for Ze BGP.
//
// Subcommands:
//
//	ze-test bgp [type] [flags]     Run BGP functional tests
//	ze-test ui [flags]             Run UI functional tests (completion, CLI)
//	ze-test editor [flags]         Run editor functional tests (.et files)
//	ze-test peer [flags]           BGP test peer (sink/echo/check modes)
//	ze-test syslog [flags]         Run syslog server for testing
//	ze-test text-plugin            Run minimal text-mode plugin (for .ci tests)
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
	if isHelpArg(cmd) {
		printUsage()
		return
	}

	// Shift args so subcommand sees itself as os.Args[0]
	os.Args = os.Args[1:]

	switch cmd {
	case "bgp":
		os.Exit(bgpCmd())
	case "editor":
		os.Exit(editorCmd())
	case "ui":
		os.Exit(uiCmd())
	case "peer":
		os.Exit(peerCmd())
	case "syslog":
		os.Exit(syslogCmd())
	case "text-plugin":
		os.Exit(textPluginCmd())
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

// isHelpArg checks if the argument is a help flag.
func isHelpArg(arg string) bool {
	return arg == "-h" || arg == "--help" || arg == "help"
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: ze-test <command> [options]

Commands:
  bgp          Run BGP functional tests (encoding, plugin, decoding, parsing)
  editor       Run editor functional tests (.et files)
  ui           Run UI functional tests (completion, CLI)
  peer         BGP test peer (sink/echo/check modes)
  syslog       Run syslog server for testing
  text-plugin  Run minimal text-mode plugin (for .ci tests)

Run 'ze-test <command> --help' for command-specific help.
`)
}
