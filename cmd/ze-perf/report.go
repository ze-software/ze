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

	_ = fs.Bool("md", true, "Markdown comparison table (default)")
	html := fs.Bool("html", false, "Self-contained HTML report")
	doc := fs.Bool("doc", false, "Full performance.md document with disclaimers and methodology")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: ze-perf report [flags] <file> [file...]

Generate a comparison report from one or more result JSON files.

Examples:
  ze-perf report result-ze.json result-gobgp.json
  ze-perf report --html result-ze.json result-gobgp.json > report.html
  ze-perf report --doc result-*.json > docs/performance.md

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

	// --doc and --html override --md.
	switch {
	case *doc:
		if err := report.PerformanceDoc(results, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "error: generating performance doc: %v\n", err)
			return 1
		}
	case *html:
		if err := report.HTML(results, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "error: generating HTML report: %v\n", err)
			return 1
		}
	default:
		if err := report.Markdown(results, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "error: generating markdown report: %v\n", err)
			return 1
		}
	}

	return 0
}
