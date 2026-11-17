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
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// cmdGraph is `ze config graph`: the answer of `show config graph`, printed.
func cmdGraph(args []string) int {
	return command.PrintLocalAnswer("show config graph", graphAnswer(command.RouteLocal, args))
}

// writeGraphUsage writes the help page of `ze config graph`, colored when
// stderr, where the local routes print it, is a terminal.
func writeGraphUsage(diag *textbuf.Buffer) {
	p := helpfmt.Page{
		Command:   "ze config graph",
		ShortHelp: "Show config dependency graph as JSON",
		Usage:     []string{"ze config graph [" + accountOptionUsage + "] <config-file>"},
		Sections: []helpfmt.HelpSection{
			{Title: helpSectionOptions, Entries: []helpfmt.HelpEntry{
				accountOptionHelp(),
			}},
		},
		Examples: []string{
			"ze config graph config.conf",
		},
	}
	p.WriteTo(diag, slogutil.UseColor(os.Stderr))
}

// graphAnswer answers `show config graph`: the dependency graph as the
// indented JSON text the command has always printed, as a Blob, or the
// refusal in Diagnostic. It writes nothing itself (command.LocalDataHandler).
func graphAnswer(route command.Route, args []string) command.LocalAnswer {
	var diag textbuf.Buffer
	fs := flag.NewFlagSet("config graph", flag.ContinueOnError)
	fs.SetOutput(&diag)
	user := fs.String("user", "", "account that asks the running ze about a config that declares an external plugin")
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

	readPath, err := command.CallerPath(route, "show config graph", args, fs.Arg(0))
	if err != nil {
		return command.LocalRefusal(exitError, command.LocalErrorLine(err))
	}
	data, err := cliio.ReadFile(readPath)
	if err != nil {
		return command.LocalRefusal(exitError, command.LocalErrorLine(err))
	}
	// The operator's own process asks the running ze for the graph of a
	// config that declares an external plugin (graphAsking). The daemon is the
	// process that answers on its own route, and asks nobody.
	if route.CallerLocal() {
		return graphAsking(fs.Arg(0), data, *user, routeAsk)
	}
	return graphBytesAnswer(data)
}

// graphBytesAnswer builds the same inactive-pruned graph for a caller file
// and supplied bytes, without running the configuration loader.
func graphBytesAnswer(data []byte) command.LocalAnswer {
	schema, declared, err := config.SchemaWithDeclaredPlugins(config.ConfigSource{Text: string(data)})
	if err != nil {
		return command.LocalRefusal(exitError, command.LocalErrorLine(opererr.Wrap(err,
			"The configuration schema could not be loaded.",
			"The configuration could not be interpreted; no dependency graph was produced.",
			"ze show doctor")))
	}

	p := config.NewParser(schema)
	tree, err := p.Parse(string(data))
	if err != nil {
		if refusal := declaredParseExplanation(declared, err); refusal != nil {
			return command.LocalRefusal(exitError, command.LocalErrorLine(refusal))
		}
		return command.LocalRefusal(exitError, command.LocalErrorLine(opererr.Wrap(err,
			"The configuration dependency graph could not be built.",
			"The supplied configuration could not be parsed; no graph was produced.",
			"ze config validate -")))
	}

	config.PruneInactive(tree, schema)

	graph := config.BuildGraph(tree, schema)

	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(graph); err != nil {
		return command.LocalRefusal(exitError, command.LocalErrorLine(opererr.Wrap(err,
			"The configuration dependency graph could not be encoded.",
			"No complete JSON graph was produced.",
			"ze show doctor")))
	}

	return command.LocalAnswer{Payload: command.Blob(out.Bytes()), Code: exitOK}
}

// declaredParseExplanation answers only a new operator explanation added by
// the declared-schema step. Nil means the cause was unchanged: callers MUST
// keep their existing ordinary parse/refusal wrapper. Only operator-error
// pointers are compared, never arbitrary parser errors.
func declaredParseExplanation(declared *config.DeclaredSchemas, cause error) *opererr.Error {
	explained, isOperator := errors.AsType[*opererr.Error](declared.Explain(cause))
	if !isOperator {
		return nil
	}
	original, wasOperator := errors.AsType[*opererr.Error](cause)
	if wasOperator {
		if explained == original {
			return nil
		}
	}
	return explained
}
