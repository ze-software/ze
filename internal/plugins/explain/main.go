// Design: docs/features/ai-first.md — explain command
// Overview: register.go — command registration

package explain

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Run executes the explain command.
func Run(args []string) int {
	if len(args) == 0 {
		usage()
		return 1
	}

	jsonOutput := false
	var codes []string

	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOutput = true
		case "help", "-h", "--help":
			usage()
			return 0
		default:
			codes = append(codes, arg)
		}
	}

	if len(codes) == 0 {
		fmt.Fprintf(os.Stderr, "error: missing diagnostic code\n")
		usage()
		return 1
	}
	if len(codes) > 1 {
		fmt.Fprintf(os.Stderr, "error: expected one diagnostic code, got %d\n", len(codes))
		return 1
	}
	code := codes[0]

	meta := diagnostic.Lookup(code)
	if meta == nil {
		fmt.Fprintf(os.Stderr, "error: unknown diagnostic code: %s\n", code)
		all := diagnostic.AllCodes()
		if len(all) > 0 {
			fmt.Fprintf(os.Stderr, "valid codes: %s\n", textbuf.Join(all, ", "))
		}
		return 1
	}

	if jsonOutput {
		return outputJSON(meta)
	}
	return outputText(meta)
}

func outputJSON(meta *diagnostic.CodeMeta) int {
	result := diagnostic.ExplainResult{
		Code:         meta.Code,
		Title:        meta.Title,
		Description:  meta.Description,
		Examples:     meta.Examples,
		RelatedCodes: meta.RelatedCodes,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func outputText(meta *diagnostic.CodeMeta) int {
	b := textbuf.Get()
	defer b.Release()

	b.Reset().Str(meta.Code).Str(": ").Str(meta.Title).Byte('\n').Byte('\n')
	b.Str(meta.Description).Byte('\n')

	if len(meta.Examples) > 0 {
		b.Byte('\n').Str("Examples:\n")
		for _, ex := range meta.Examples {
			b.Str("  ").Str(ex).Byte('\n')
		}
	}
	if len(meta.RelatedCodes) > 0 {
		b.Byte('\n').Str("Related: ").Str(textbuf.Join(meta.RelatedCodes, ", ")).Byte('\n')
	}

	if _, err := os.Stdout.WriteString(b.Slice()); err != nil {
		return 1
	}
	return 0
}

func usage() {
	p := helpfmt.Page{
		Command: "ze explain",
		Summary: "Explain a diagnostic code emitted by ze config validate",
		Usage:   []string{"ze explain [--json] <diagnostic-code>"},
		Sections: []helpfmt.HelpSection{
			{Title: "Options", Entries: []helpfmt.HelpEntry{
				{Name: "--json", Desc: "Output structured JSON explanation"},
			}},
		},
		Examples: []string{
			"ze explain config-parse",
			"ze explain --json config-yang-type",
		},
	}
	p.WriteErr()
}
