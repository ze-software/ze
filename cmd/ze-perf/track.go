// Design: (none -- new tool, predates documentation)
package main

import (
	"flag"
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/perf"
	"codeberg.org/thomas-mangin/ze/internal/perf/report"
)

func cmdTrack(args []string) int {
	fs := flag.NewFlagSet("ze-perf track", flag.ContinueOnError)

	md := fs.Bool("md", true, "Markdown output (default)")
	html := fs.Bool("html", false, "HTML output")
	check := fs.Bool("check", false, "Check for regressions (exit non-zero on regression)")
	last := fs.Int("last", 0, "Only consider last N entries (0 = all)")
	thresholdConvergence := fs.Int("threshold-convergence", 20, "Convergence regression threshold (%)")
	thresholdThroughput := fs.Int("threshold-throughput", 20, "Throughput regression threshold (%)")
	thresholdP99 := fs.Int("threshold-p99", 30, "P99 latency regression threshold (%)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: ze-perf track [flags] <history.ndjson>

Track performance history and detect regressions from an NDJSON file.

Examples:
  ze-perf track history.ndjson
  ze-perf track --check history.ndjson
  ze-perf track --html history.ndjson > trend.html
  ze-perf track --check --threshold-convergence 15 history.ndjson

Flags:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	files := fs.Args()
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "error: NDJSON history file is required\n")
		return 1
	}

	if len(files) > 1 {
		fmt.Fprintf(os.Stderr, "error: only one history file is supported\n")
		return 1
	}

	f, err := os.Open(files[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: opening %s: %v\n", files[0], err)
		return 1
	}
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: closing %s: %v\n", files[0], err)
		}
	}()

	results, err := perf.ReadNDJSON(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: reading %s: %v\n", files[0], err)
		return 1
	}

	if len(results) == 0 {
		fmt.Fprintf(os.Stderr, "error: no results found in %s\n", files[0])
		return 1
	}

	// Apply --last filter.
	if *last > 0 && *last < len(results) {
		results = results[len(results)-*last:]
	}

	if *check {
		thresholds := perf.Thresholds{
			ConvergencePct: *thresholdConvergence,
			ThroughputPct:  *thresholdThroughput,
			P99Pct:         *thresholdP99,
		}

		regressions := perf.CheckHistory(results, thresholds, *last)
		if len(regressions) > 0 {
			for _, r := range regressions {
				fmt.Fprintf(os.Stderr, "regression: %s\n", r.Message)
			}

			return 1
		}

		fmt.Fprintf(os.Stderr, "no regressions detected\n")

		return 0
	}

	// --html overrides --md.
	format := "md"
	if *html {
		*md = false
		format = "html"
	}

	_ = *md // Consumed via format selection above.

	if err := report.Trend(results, os.Stdout, format); err != nil {
		fmt.Fprintf(os.Stderr, "error: generating trend report: %v\n", err)
		return 1
	}

	return 0
}
