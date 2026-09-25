// Design: docs/architecture/system-architecture.md -- le perf report subcommand

package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/perf"
	"github.com/ze-software/ze/internal/perf/report"
)

func cmdReport(args []string) int {
	fs := flag.NewFlagSet("le perf report", flag.ContinueOnError)

	_ = fs.Bool("md", true, "Markdown comparison table (default)")
	html := fs.Bool("html", false, "Self-contained HTML report")
	doc := fs.Bool("doc", false, "Full performance.md document with disclaimers and methodology")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage: le perf report [flags] <file> [file...]\n\nGenerate a comparison report from one or more result JSON files.\n\nExamples:\n  le perf report result-ze.json result-gobgp.json\n  le perf report --html result-ze.json result-gobgp.json > report.html\n  le perf report --doc result-*.json > docs/performance.md\n\nFlags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		// A help word is a question the flag set already answered by printing
		// its usage, so it is not a failure.
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	files := fs.Args()
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "error: at least one result file is required\n")
		return 1
	}

	var results []perf.Result

	for _, path := range files {
		data, err := cliio.ReadFile(path)
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
