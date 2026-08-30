// Design: docs/features/ai-first.md — config fix plan command
// Overview: main.go — dispatch and exit codes

package cli

import (
	"flag"
	"os"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/helpfmt"
)

func cmdFix(args []string) int {
	fs := flag.NewFlagSet("config fix", flag.ExitOnError)
	plan := fs.Bool("plan", false, "plan-only mode (required)")

	fs.Usage = fixUsage

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if !*plan {
		helpfmt.WriteError(os.Stderr, false, "ze config fix requires --plan")
		fixUsage()
		return 1
	}

	if fs.NArg() == 0 {
		helpfmt.WriteError(os.Stderr, false, "missing config file")
		fixUsage()
		return 1
	}
	if fs.NArg() > 1 {
		helpfmt.WriteError(os.Stderr, false, "expected one config file, got %d", fs.NArg())
		return 1
	}

	payload, code := resolveFixPlan(fs.Arg(0))
	if payload == nil {
		return code
	}
	// The path is the one `show config fix` registers (register.go), so this
	// spelling of the command renders through the same column order and the
	// same default format the pipe layer gives the other one.
	return command.RenderLocalAnswer("show config fix", payload)
}

// resolveFixPlan reads a configuration, validates it, and answers the repair
// plan its diagnostics imply. It never edits the file.
//
// It is the payload half of cmdFix, lifted so `show config fix` answers with
// DATA (dataFix, config_data.go) and the two spellings cannot drift.
func resolveFixPlan(configPath string) (any, int) {
	data, err := cliio.ReadFile(configPath)
	if err != nil {
		helpfmt.WriteError(os.Stderr, false, "%v", err)
		return nil, exitError
	}

	result := runValidation(string(data), configPath)
	return diagnostic.NewFixPlan(configPath, result.Diagnostics), exitOK
}

func fixUsage() {
	p := helpfmt.Page{
		Command: "ze config fix",
		Summary: "Generate a plan-only repair plan for config diagnostics",
		Usage:   []string{"ze config fix --plan <config-file>"},
		Sections: []helpfmt.HelpSection{
			{Title: helpSectionOptions, Entries: []helpfmt.HelpEntry{
				{Name: "--plan", Desc: "Plan-only mode (required, never edits files)"},
			}},
		},
		Examples: []string{
			"ze config fix --plan config.conf",
			"ze cli -c \"show config fix config.conf | json\"",
		},
	}
	p.WriteErr()
}
