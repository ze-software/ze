// Design: docs/features/ai-first.md — config fix plan command
// Overview: main.go — dispatch and exit codes

package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/helpfmt"
)

func cmdFix(args []string) int {
	fs := flag.NewFlagSet("config fix", flag.ExitOnError)
	plan := fs.Bool("plan", false, "plan-only mode (required)")
	jsonOut := fs.Bool("json", false, "JSON output (required)")

	fs.Usage = fixUsage

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if !*plan || !*jsonOut {
		fmt.Fprintf(os.Stderr, "error: ze config fix requires --plan --json\n")
		fixUsage()
		return 1
	}

	if fs.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "error: missing config file\n")
		fixUsage()
		return 1
	}
	if fs.NArg() > 1 {
		fmt.Fprintf(os.Stderr, "error: expected one config file, got %d\n", fs.NArg())
		return 1
	}
	configPath := fs.Arg(0)

	data, err := cliio.ReadFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}

	result := runValidation(string(data), configPath)

	fixResult := diagnostic.NewFixPlan(configPath, result.Diagnostics)

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(fixResult); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func fixUsage() {
	p := helpfmt.Page{
		Command: "ze config fix",
		Summary: "Generate a plan-only repair plan for config diagnostics",
		Usage:   []string{"ze config fix --plan --json <config-file>"},
		Sections: []helpfmt.HelpSection{
			{Title: "Options", Entries: []helpfmt.HelpEntry{
				{Name: "--plan", Desc: "Plan-only mode (required, never edits files)"},
				{Name: "--json", Desc: "JSON output (required)"},
			}},
		},
		Examples: []string{
			"ze config fix --plan --json config.conf",
		},
	}
	p.WriteErr()
}
