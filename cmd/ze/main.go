// Package main provides the ze command entry point.
package main

import (
	"fmt"
	"os"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/bgp"
	"codeberg.org/thomas-mangin/ze/cmd/ze/exabgp"
	"codeberg.org/thomas-mangin/ze/cmd/ze/hub"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	arg := os.Args[1]

	// Check for known commands first
	switch arg {
	case "bgp":
		os.Exit(bgp.Run(os.Args[2:]))
	case "exabgp":
		os.Exit(exabgp.Run(os.Args[2:]))
	case "version":
		fmt.Printf("ze %s\n", version)
		os.Exit(0)
	case "help", "-h", "--help":
		usage()
		os.Exit(0)
	}

	// If arg looks like a config file, start hub
	if looksLikeConfig(arg) {
		os.Exit(hub.Run(arg))
	}

	// Unknown command
	fmt.Fprintf(os.Stderr, "unknown command: %s\n", arg)
	usage()
	os.Exit(1)
}

// looksLikeConfig returns true if the argument looks like a config file path.
func looksLikeConfig(arg string) bool {
	// Check for common config extensions
	if strings.HasSuffix(arg, ".conf") ||
		strings.HasSuffix(arg, ".cfg") ||
		strings.HasSuffix(arg, ".yaml") ||
		strings.HasSuffix(arg, ".yml") ||
		strings.HasSuffix(arg, ".json") {
		return true
	}

	// Check if it's a path (contains / or starts with .)
	if strings.Contains(arg, "/") || strings.HasPrefix(arg, ".") {
		// Check if file exists
		if _, err := os.Stat(arg); err == nil {
			return true
		}
	}

	return false
}

func usage() {
	fmt.Fprintf(os.Stderr, `ze - Ze toolchain

Usage:
  ze <config>            Start hub with config file
  ze <command> [options]   Execute command

Commands:
  bgp      BGP daemon and tools
  exabgp   ExaBGP compatibility tools
  version  Show version
  help     Show this help

Examples:
  ze /etc/ze/config.conf   Start hub with config
  ze bgp help              Show BGP commands
`)
}
