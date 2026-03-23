// Design: (none -- new tool, predates documentation)

// Command ze-perf is a BGP propagation latency benchmark tool.
//
// It measures route propagation timing through a device under test (DUT)
// by establishing sender and receiver BGP sessions, injecting routes,
// and timing their arrival.
//
// Usage:
//
//	ze-perf <command> [flags]
//	ze-perf run --dut-addr 172.31.0.2 --dut-asn 65000
//	ze-perf report result-ze.json result-gobgp.json
//	ze-perf track --check history.ndjson
package main

import (
	"fmt"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		printUsage(os.Stdout)
		return 0
	}

	switch args[0] {
	case "-h", "--help", "help":
		printUsage(os.Stdout)
		return 0
	case "run":
		return cmdRun(args[1:])
	case "report":
		return cmdReport(args[1:])
	case "track":
		return cmdTrack(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "error: unknown command %q\n\n", args[0])
		printUsage(os.Stderr)
		return 1
	}
}

func printUsage(w *os.File) {
	if _, err := fmt.Fprint(w, `ze-perf - BGP propagation latency benchmark tool

Usage: ze-perf <command> [flags]

Commands:
  run      Run benchmark against a BGP DUT
  report   Generate comparison report from result files
  track    Track performance history and detect regressions

Use "ze-perf <command> -h" for help on a specific command.
`); err != nil {
		return
	}
}
