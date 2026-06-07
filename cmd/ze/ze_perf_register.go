// Design: docs/architecture/system-architecture.md -- ze-perf subcommand dispatch

//go:build ze_perf

package main

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/command/registry"
	zeversion "codeberg.org/thomas-mangin/ze/internal/core/version"
)

func init() {
	binarySetup = zePerfSetup
}

func zePerfSetup(args []string) ([]string, int) {
	registry.MustRegisterRootHandler("perf", zePerfRootHandler, registry.Meta{
		Description: "BGP propagation latency benchmark tool",
		Mode:        "offline",
		Section:     registry.SectionTest,
	})
	return args, 0
}

func zePerfRootHandler(_ *registry.RuntimeContext, args []string) int {
	if len(args) == 0 {
		zePerfUsage()
		return 0
	}

	switch args[0] {
	case "-h", "--help", "help": //nolint:goconst // consistent pattern across cmd files
		zePerfUsage()
		return 0
	case "--version", "-V":
		fmt.Println(zeversion.Short())
		return 0
	case "run":
		return cmdPerfRun(args[1:])
	case "report":
		return cmdPerfReport(args[1:])
	case "track":
		return cmdPerfTrack(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "error: unknown command %q\n\n", args[0])
		zePerfUsage()
		return 1
	}
}

func zePerfUsage() {
	fmt.Fprint(os.Stderr, `ze-perf - BGP propagation latency benchmark tool

Usage: ze-perf <command> [flags]

Commands:
  run      Run benchmark against a BGP DUT
  report   Generate comparison report from result files
  track    Track performance history and detect regressions

Use "ze-perf <command> -h" for help on a specific command.
`)
}
