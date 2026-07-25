// Design: docs/architecture/config/syntax.md — config dependency graph CLI command
// Overview: main.go — dispatch and exit codes

package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/helpfmt"
)

func cmdGraph(args []string) int {
	fs := flag.NewFlagSet("config graph", flag.ExitOnError)

	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze config graph",
			Summary: "Show config dependency graph as JSON",
			Usage:   []string{"ze config graph <config-file>"},
			Examples: []string{
				"ze config graph config.conf",
			},
		}
		p.WriteErr()
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: missing config file (use - for stdin)\n")
		fs.Usage()
		return exitError
	}

	data, err := cliio.ReadFile(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
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
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	config.PruneInactive(tree, schema)

	graph := config.BuildGraph(tree, schema)

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(graph); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	return exitOK
}
