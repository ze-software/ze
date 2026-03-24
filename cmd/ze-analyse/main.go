// Design: (none -- research/analysis tool)
//
// ze-analyse consolidates MRT analysis tools for BGP research.
// Each subcommand analyzes a different aspect of real-world BGP data.
//
// Usage: ze-analyse <command> [options] [files...]
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "download":
		os.Exit(runDownload(args))
	case "density":
		os.Exit(runDensity(args))
	case "attributes":
		os.Exit(runAttributes(args))
	case "communities":
		os.Exit(runCommunities(args))
	case "count-attrs":
		os.Exit(runCountAttrs(args))
	case "mrt-dump":
		os.Exit(runMRTDump(args))
	case "help", "-h", "--help":
		usage()
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `ze-analyse -- BGP MRT analysis tools

Usage:
  ze-analyse <command> [options] [files...]

Commands:
  download       Download MRT RIB dumps and BGP4MP updates from RIPE RIS / RouteViews
  density        Analyze NLRI density per UPDATE and burst distribution
  attributes     Analyze attribute repetition patterns for caching decisions
  communities    Generate per-ASN community defaults from MRT files
  count-attrs    Count attributes per route (distribution table)
  mrt-dump       Dump MRT records as BGP UPDATE hex (one per line)

Data sources:
  RIPE RIS:      https://data.ris.ripe.net/rrc00/
  RouteViews:    https://archive.routeviews.org/bgpdata/

Examples:
  ze-analyse download                              # fetch latest RIB + today's updates
  ze-analyse download 20260324 0000                # specific date/time
  ze-analyse density test/internet/ripe-updates.*.gz
  ze-analyse attributes test/internet/latest-bview.gz
  ze-analyse communities --threshold 0.95 test/internet/latest-bview.gz
`)
}
