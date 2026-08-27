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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
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
		if slices.Contains(args, "--json") {
			return printPipeCatalogJSON(os.Stdout)
		}
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
	// A chain can be refused AFTER the command's answer is in hand — an
	// operator the answer's shape cannot support is the usual reason — and the
	// refusal arrives as the formatted string. Printing it to stdout and
	// exiting 0 would leave a script parsing the refusal as data.
	if command.IsPipeError(result) {
		fmt.Fprintln(os.Stderr, result)
		return 1
	}
	os.Stdout.WriteString(result) //nolint:errcheck // CLI output
	if result != "" && !strings.HasSuffix(result, "\n") {
		os.Stdout.WriteString("\n") //nolint:errcheck // CLI output
	}
	return 0
}

// pipeOperatorJSON is the machine-readable form of one operator's contract.
// Before it, the only surface publishing all sixteen names to a machine was an
// authenticated web completion endpoint, and every CLI surface carried a
// hand-typed subset. A tool author reads this.
type pipeOperatorJSON struct {
	Name              string   `json:"name"`
	Class             string   `json:"class"`
	Arg               string   `json:"arg"`
	ArgHint           string   `json:"arg-hint,omitempty"`
	Repeat            string   `json:"repeat"`
	Shapes            []string `json:"shapes"`
	Description       string   `json:"description"`
	NeedsAddressField bool     `json:"needs-address-field,omitempty"`
	LocalOnly         bool     `json:"local-only,omitempty"`
}

// printPipeCatalogJSON writes the operator catalog for `ze pipe help --json`.
func printPipeCatalogJSON(w io.Writer) int {
	ops := command.PipeOperatorCatalog()
	out := make([]pipeOperatorJSON, 0, len(ops))
	for _, op := range ops {
		shapes := make([]string, 0, 3)
		for _, shape := range op.Shapes() {
			shapes = append(shapes, shape.String())
		}
		out = append(out, pipeOperatorJSON{
			Name:              op.Name,
			Class:             op.Class.String(),
			Arg:               op.Arg.String(),
			ArgHint:           op.ArgHint(),
			Repeat:            op.Repeat.String(),
			Shapes:            shapes,
			Description:       op.Description,
			NeedsAddressField: op.NeedsAddressField,
			LocalOnly:         op.LocalOnly,
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func pipeUsage() {
	// All three sections are derived from the operator catalog
	// (internal/component/command/pipe_catalog.go). This page used to hold its
	// own list of ten, one of five hand-copied lists that had drifted apart.
	var global, data, stream []helpfmt.HelpEntry
	for _, op := range command.PipeOperatorCatalog() {
		var nb textbuf.Buffer
		nb.Str(op.Name)
		if hint := op.ArgHint(); hint != "" {
			nb.Str(" ").Str(hint)
		}
		name := nb.String()
		description := op.Description
		if op.LocalOnly {
			description = nb.Reset().Str(op.Description).
				Str(" (local process only; daemon-expanded remote chains refuse it)").String()
		}
		entry := helpfmt.HelpEntry{Name: name, Desc: description}
		switch op.Class {
		case command.ClassGlobal:
			global = append(global, entry)
		case command.ClassStream:
			stream = append(stream, entry)
		default:
			data = append(data, entry)
		}
	}

	p := helpfmt.Page{
		Command: "ze pipe",
		Summary: "Apply pipe operators to stdin",
		Usage:   []string{"<command> | ze pipe <operator> [args]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Global Operators (act on any answer)", Entries: global},
			{Title: "Row Operators (act where the answer has rows)", Entries: data},
			{Title: "Streaming Operators (act where the command keeps answering)", Entries: stream},
		},
		Examples: []string{
			"ze show bgp peer list | ze pipe json compact",
			"ze show bgp peer list | ze pipe yaml",
			"ze show bgp peer list | ze pipe first 5",
			"ze show bgp peer list | ze pipe display address state",
			"ze show bgp peer list | ze pipe match established | ze pipe count",
		},
	}
	p.WriteErr()
}
