// Design: docs/architecture/config/syntax.md — config dependency graph CLI command
// Overview: main.go — dispatch and exit codes

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"os"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// cmdGraph is `ze config graph`: the answer of `show config graph`, printed.
func cmdGraph(args []string) int {
	return command.PrintLocalAnswer("show config graph", graphAnswer(args))
}

// writeGraphUsage writes the help page of `ze config graph`, colored when
// stderr, where the local routes print it, is a terminal.
func writeGraphUsage(diag *textbuf.Buffer) {
	p := helpfmt.Page{
		Command:   "ze config graph",
		ShortHelp: "Show config dependency graph as JSON",
		Usage:     []string{"ze config graph <config-file>"},
		Examples: []string{
			"ze config graph config.conf",
		},
	}
	p.WriteTo(diag, slogutil.UseColor(os.Stderr))
}

// graphAnswer answers `show config graph`: the dependency graph as the
// indented JSON text the command has always printed, as a Blob, or the
// refusal in Diagnostic. It writes nothing itself (command.LocalDataHandler).
func graphAnswer(args []string) command.LocalAnswer {
	var diag textbuf.Buffer
	fs := flag.NewFlagSet("config graph", flag.ContinueOnError)
	fs.SetOutput(&diag)
	fs.Usage = func() { writeGraphUsage(&diag) }

	// The flag package wrote the help page, or the flag error and the page,
	// into diag. `-h` asked for the page, so it succeeds as it always did.
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return command.LocalRefusal(exitOK, diag.String())
		}
		return command.LocalRefusal(exitError, diag.String())
	}

	if fs.NArg() < 1 {
		diag.Str("error: missing config file (use - for stdin)\n")
		writeGraphUsage(&diag)
		return command.LocalRefusal(exitError, diag.String())
	}

	data, err := cliio.ReadFile(fs.Arg(0))
	if err != nil {
		return command.LocalRefusal(exitError, command.LocalErrorLine(err))
	}

	schema, err := config.YANGSchema()
	if err != nil {
		return command.LocalRefusal(exitError, command.LocalErrorLine(err))
	}

	p := config.NewParser(schema)
	tree, err := p.Parse(string(data))
	if err != nil {
		return command.LocalRefusal(exitError, command.LocalErrorLine(err))
	}

	config.PruneInactive(tree, schema)

	graph := config.BuildGraph(tree, schema)

	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(graph); err != nil {
		return command.LocalRefusal(exitError, command.LocalErrorLine(err))
	}

	return command.LocalAnswer{Payload: command.Blob(out.Bytes()), Code: exitOK}
}
