// Package main provides the ze command entry point.
package main

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/cmd/ze/bgp"
	"codeberg.org/thomas-mangin/ze/cmd/ze/exabgp"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
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
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `ze - Ze toolchain

Usage:
  ze <command> [options]

Commands:
  bgp      BGP daemon and tools
  exabgp   ExaBGP compatibility tools
  version  Show version
  help     Show this help

Run 'ze bgp help' for BGP-specific commands.
Run 'ze exabgp help' for ExaBGP compatibility commands.
`)
}
