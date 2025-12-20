// Package main provides the ZeBGP CLI client entry point.
//
// This tool is deprecated. Use "zebgp run" instead.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "zebgp-cli is deprecated. Use 'zebgp run <command>' instead.")
	fmt.Fprintln(os.Stderr, "Examples:")
	fmt.Fprintln(os.Stderr, "  zebgp run peer list")
	fmt.Fprintln(os.Stderr, "  zebgp run -i          # interactive mode")
	os.Exit(1)
}
