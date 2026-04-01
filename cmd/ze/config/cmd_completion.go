// Design: docs/architecture/config/yang-config-design.md — config completion command
// Overview: main.go — dispatch and exit codes

package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

func cmdCompletion(args []string) int {
	fs := flag.NewFlagSet("config completion", flag.ExitOnError)
	context := fs.String("context", "", "context path with / separator (e.g., bgp/peer/1.1.1.1)")
	inputFlag := fs.String("input", "", "input text to complete (e.g., \"set \", \"set local\")")
	jsonOutput := fs.Bool("json", false, "output as JSON")
	ghost := fs.Bool("ghost", false, "show ghost text instead of completions")
	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze config completion",
			Summary: "Query the YANG-driven completion engine non-interactively",
			Usage:   []string{"ze config completion [options] <config-file>"},
			Sections: []helpfmt.HelpSection{
				{Title: "Options", Entries: []helpfmt.HelpEntry{
					{Name: "--context <path>", Desc: "Context path with / separator (e.g., bgp/peer/1.1.1.1)"},
					{Name: "--input <text>", Desc: "Input text to complete (e.g., \"set \", \"set local\")"},
					{Name: "--json", Desc: "Output as JSON"},
					{Name: "--ghost", Desc: "Show ghost text instead of completions"},
				}},
			},
			Examples: []string{
				"ze config completion --input set+ config.conf",
				"ze config completion --context bgp --input set+ config.conf",
				"ze config completion --context bgp --input set+local config.conf",
				"ze config completion --json --context bgp/peer/1.1.1.1 --input set+p config.conf",
				"ze config completion --ghost --context bgp --input set+router config.conf",
			},
		}
		p.Write()
		fmt.Fprintf(os.Stderr, "\nUseful for testing and debugging config editor completions.\n")
		fmt.Fprintf(os.Stderr, "Use - to read config from stdin.\n")
		fmt.Fprintf(os.Stderr, "\nInput uses + for spaces (unquoted), or regular spaces (quoted):\n")
		fmt.Fprintf(os.Stderr, "  --input set+           equivalent to \"set \"\n")
		fmt.Fprintf(os.Stderr, "  --input set+local      equivalent to \"set local\"\n")
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: missing config file (use - for stdin)\n")
		fs.Usage()
		return 1
	}

	configPath := fs.Arg(0)
	// Decode + as space in input (allows unquoted args in test runners)
	input := strings.ReplaceAll(*inputFlag, "+", " ")

	// Parse context path (/-separated to work in both shell and test runner)
	var contextPath []string
	if *context != "" {
		contextPath = strings.Split(*context, "/")
	}

	// Load and parse config
	data, err := loadConfigData(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading config: %v\n", err)
		return exitError
	}

	schema, err := config.YANGSchema()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	p := config.NewParser(schema)
	tree, err := p.Parse(string(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing config: %v\n", err)
		return exitError
	}

	// Create completer with config tree for data-aware completion
	completer := cli.NewCompleter()
	completer.SetTree(tree)

	if *ghost {
		return outputGhost(completer, input, contextPath, *jsonOutput)
	}

	return outputCompletions(completer, input, contextPath, *jsonOutput)
}

func outputGhost(c *cli.Completer, input string, contextPath []string, asJSON bool) int {
	text := c.GhostText(input, contextPath)

	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		if err := enc.Encode(map[string]string{"ghost": text}); err != nil {
			fmt.Fprintf(os.Stderr, "error encoding JSON: %v\n", err)
			return exitError
		}
		return exitOK
	}

	fmt.Println(text)
	return exitOK
}

func outputCompletions(c *cli.Completer, input string, contextPath []string, asJSON bool) int {
	completions := c.Complete(input, contextPath)

	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(completions); err != nil {
			fmt.Fprintf(os.Stderr, "error encoding JSON: %v\n", err)
			return exitError
		}
		return exitOK
	}

	if len(completions) == 0 {
		return exitOK
	}

	for _, comp := range completions {
		if comp.Description != "" {
			fmt.Printf("%-12s %-24s %s\n", comp.Type, comp.Text, comp.Description)
		} else {
			fmt.Printf("%-12s %s\n", comp.Type, comp.Text)
		}
	}

	return exitOK
}
