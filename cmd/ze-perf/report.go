// Design: (none -- new tool, predates documentation)
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/perf"
	"codeberg.org/thomas-mangin/ze/internal/perf/report"
)

func cmdReport(args []string) int {
	fs := flag.NewFlagSet("ze-perf report", flag.ContinueOnError)

	md := fs.Bool("md", true, "Markdown output (default)")
	html := fs.Bool("html", false, "HTML output")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: ze-perf report [flags] <file> [file...]

Generate a comparison report from one or more result JSON files.

Examples:
  ze-perf report result-ze.json result-gobgp.json
  ze-perf report --html result-ze.json result-gobgp.json > report.html

Flags:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	files := fs.Args()
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "error: at least one result file is required\n")
		return 1
	}

	var results []perf.Result

	for _, path := range files {
		data, err := os.ReadFile(path) //nolint:gosec // CLI tool reads user-provided file paths
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: reading %s: %v\n", path, err)
			return 1
		}

		var res perf.Result
		if err := json.Unmarshal(data, &res); err != nil {
			fmt.Fprintf(os.Stderr, "error: parsing %s: %v\n", path, err)
			return 1
		}

		results = append(results, res)
	}

	// --html overrides --md.
	if *html {
		*md = false
	}

	if *md {
		if err := report.Markdown(results, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "error: generating markdown report: %v\n", err)
			return 1
		}
	} else {
		if err := report.HTML(results, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "error: generating HTML report: %v\n", err)
			return 1
		}
	}

	return 0
}
