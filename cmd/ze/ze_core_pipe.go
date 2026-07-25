// Design: docs/architecture/api/commands.md -- ze pipe: offline pipe operators
//
// ze pipe applies the same pipe operators used in the online CLI
// (json, table, yaml, match, count, first, last, resolve) to stdin, so offline
// commands can be transformed the same way. `pipe` names the whole operator
// language (internal/component/command/pipe.go, knownPipeOps), not one clause of
// it -- `format` is only one operator kind within that language.

//go:build ze_core

package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func runPipe(args []string) int {
	if len(args) == 0 {
		pipeUsage()
		return 1
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" { //nolint:goconst // consistent pattern across cmd files
		pipeUsage()
		return 0
	}

	pipeStr := textbuf.Join(args, " ")
	var tb textbuf.Buffer
	input := tb.Str("_ | ").Str(pipeStr).String()

	_, format, errMsg := command.ProcessPipesChecked(input)
	if errMsg != "" {
		fmt.Fprintf(os.Stderr, "error: %s\n", errMsg)
		return 1
	}

	const maxStdin = 256 << 20 // 256 MB
	data, err := io.ReadAll(io.LimitReader(os.Stdin, maxStdin+1))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read stdin: %v\n", err)
		return 1
	}
	if len(data) > maxStdin {
		fmt.Fprintf(os.Stderr, "error: stdin exceeds 256 MB limit\n")
		return 1
	}

	result := format(string(data))
	os.Stdout.WriteString(result) //nolint:errcheck // CLI output
	if result != "" && !strings.HasSuffix(result, "\n") {
		os.Stdout.WriteString("\n") //nolint:errcheck // CLI output
	}
	return 0
}

func pipeUsage() {
	p := helpfmt.Page{
		Command: "ze pipe",
		Summary: "Apply pipe operators to stdin",
		Usage:   []string{"<command> | ze pipe <operator> [args]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Format Operators", Entries: []helpfmt.HelpEntry{
				{Name: "json [compact]", Desc: "Format as JSON (pretty or compact)"},
				{Name: "ndjson", Desc: "One compact JSON object per line"},
				{Name: "table", Desc: "Box-drawing table"},
				{Name: "text", Desc: "Space-aligned columns"},
				{Name: "yaml", Desc: "YAML output"},
			}},
			{Title: "Filter Operators", Entries: []helpfmt.HelpEntry{
				{Name: "match <pattern>", Desc: "Grep lines (case-insensitive)"},
				{Name: "count", Desc: "Count items (JSON-aware)"},
				{Name: "first <n>", Desc: "Take first N items"},
				{Name: "last <n>", Desc: "Take last N items"},
				{Name: "resolve", Desc: "Add reverse DNS names for IP values"},
			}},
		},
		Examples: []string{
			"ze debug show | ze pipe match reactor",
			"ze debug show | ze pipe count",
			"ze show bgp peer list | ze pipe json compact",
			"ze show bgp peer list | ze pipe yaml",
			"ze show bgp peer list | ze pipe first 5",
			"ze show bgp peer list | ze pipe resolve",
		},
	}
	p.WriteErr()
}
