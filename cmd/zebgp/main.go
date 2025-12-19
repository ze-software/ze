// Package main provides the ZeBGP daemon entry point.
package main

import (
	"fmt"
	"os"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "validate":
		os.Exit(cmdValidate(os.Args[2:]))
	case "run":
		os.Exit(cmdRun(os.Args[2:]))
	case "version":
		fmt.Printf("zebgp %s\n", version)
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
	fmt.Fprintf(os.Stderr, `zebgp - BGP daemon

Usage:
  zebgp <command> [options]

Commands:
  validate <config>   Validate configuration file
  run <config>        Run the BGP daemon
  version             Show version
  help                Show this help

Examples:
  zebgp validate /etc/zebgp/config.conf
  zebgp run /etc/zebgp/config.conf
`)
}
