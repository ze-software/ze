// Design: docs/architecture/system-architecture.md -- ze-analyze subcommand dispatch

//go:build ze_analyze

package main

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/command/registry"
	zeversion "codeberg.org/thomas-mangin/ze/internal/core/version"
)

func init() {
	binarySetup = zeAnalyzeSetup
}

func zeAnalyzeSetup(args []string) ([]string, int) {
	registry.MustRegisterRootHandler("analyze", zeAnalyzeRootHandler, registry.Meta{
		Description: "BGP MRT analysis tools",
		Mode:        "offline",
		Section:     registry.SectionTest,
	})
	return args, 0
}

func zeAnalyzeRootHandler(_ *registry.RuntimeContext, args []string) int {
	if len(args) == 0 {
		zeAnalyzeUsage()
		return 1
	}
	if isHelpArg(args[0]) {
		zeAnalyzeUsage()
		return 0
	}

	switch args[0] {
	case "--version", "-V":
		fmt.Println(zeversion.Short())
		return 0
	case "download":
		return runDownload(args[1:])
	case "density":
		return runDensity(args[1:])
	case "attributes":
		return runAttributes(args[1:])
	case "communities":
		return runCommunities(args[1:])
	case "count-attrs":
		return runCountAttrs(args[1:])
	case "aspath":
		return runASPath(args[1:])
	case "mrt-dump":
		return runMRTDump(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		zeAnalyzeUsage()
		return 1
	}
}

func zeAnalyzeUsage() {
	fmt.Fprint(os.Stderr, `ze-analyze -- BGP MRT analysis tools

Usage:
  ze-analyze <command> [options] [files...]

Commands:
  download       Download MRT RIB dumps and BGP4MP updates from RIPE RIS / RouteViews
  density        Analyze NLRI density per UPDATE and burst distribution
  attributes     Analyze attribute repetition patterns for caching decisions
  communities    Generate per-ASN community defaults from MRT files
  count-attrs    Count attributes per route (distribution table)
  aspath         AS_PATH suffix sharing analysis (reversed trie compression)
  mrt-dump       Dump MRT records as BGP UPDATE hex (one per line)

Data sources:
  RIPE RIS:      https://data.ris.ripe.net/rrc00/
  RouteViews:    https://archive.routeviews.org/bgpdata/

Examples:
  ze-analyze download                              # fetch latest RIB + today's updates
  ze-analyze download 20260324 0000                # specific date/time
  ze-analyze density test/internet/ripe-updates.*.gz
  ze-analyze attributes test/internet/latest-bview.gz
  ze-analyze communities --threshold 0.95 test/internet/latest-bview.gz
`)
}
