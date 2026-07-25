// Design: docs/architecture/api/commands.md — command catalog
//
// help_command.go implements `ze help command [filter] [--json]`.
// It walks the full command tree (YANG verbs + offline local commands)
// and emits a flat, greppable catalog with descriptions.

//go:build ze_core

// JSON output is consumable by external tooling (e.g., wiki generators).

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/ze-software/ze/cmd/ze/internal/helpfmt"
	cli "github.com/ze-software/ze/internal/component/cli/client"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// commandArg describes a typed argument for a command.
type commandArg struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Values    []string `json:"values,omitempty"`
	Mandatory bool     `json:"mandatory,omitempty"`
}

// commandPipe describes a command-specific pipe filter.
type commandPipe struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	TakesArg    bool   `json:"takes-arg,omitempty"`
}

// commandEntry is a single command in the catalog.
type commandEntry struct {
	Path        string        `json:"path"`
	Description string        `json:"description,omitempty"`
	Mode        string        `json:"mode"`
	WireMethod  string        `json:"wire-method,omitempty"`
	Backend     []string      `json:"backend,omitempty"`
	TaskSupport string        `json:"task-support,omitempty"`
	Args        []commandArg  `json:"args,omitempty"`
	Pipes       []commandPipe `json:"pipes,omitempty"`
	GlobalPipes bool          `json:"global-pipes,omitempty"`
	Subcommands []string      `json:"subcommands,omitempty"`
}

// printHelpCommand implements `ze help command [filter...] [--json] [--verbose]`.
// Output routes through a helpfmt.RenderWriter so a broken pipe surfaces as a
// non-zero exit; returns the exit code.
func printHelpCommand(args []string) int {
	return renderHelpCommand(os.Stdout, args)
}

// renderHelpCommand writes the catalog to w and returns the exit code. Split
// from printHelpCommand so tests can drive a failing writer.
func renderHelpCommand(w io.Writer, args []string) int {
	jsonOutput := slices.Contains(args, "--json")
	verbose := slices.Contains(args, "--verbose") || slices.Contains(args, "-v")
	filter := extractCommandFilter(args)

	entries := collectCommands()

	if filter != "" {
		entries = filterCommands(entries, filter)
	}

	if jsonOutput {
		return printCommandJSON(w, entries)
	}

	rw := helpfmt.NewRenderWriter(w)
	if verbose {
		printCommandVerbose(rw, entries)
	} else {
		printCommandTable(rw, entries)
	}
	return rw.ExitCode()
}

// extractCommandFilter returns the first positional argument (not a flag).
func extractCommandFilter(args []string) string {
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			return a
		}
	}
	return ""
}

// collectCommands gathers every command from both the YANG dispatch
// registry and the offline local command registry into a flat sorted list.
func collectCommands() []commandEntry {
	var entries []commandEntry
	seen := make(map[string]bool)

	tree := cli.YANGCommandTree()
	wireToPaths := cli.WireToPaths()

	for wireMethod, cliPaths := range wireToPaths {
		for _, cliPath := range cliPaths {
			if seen[cliPath] {
				continue
			}
			mode := "daemon"
			if pluginserver.IsReadOnlyPath(cliPath) {
				mode = "read-only"
			}
			node := findNode(tree, cliPath)
			desc := ""
			if node != nil {
				desc = node.Description
			}
			e := commandEntry{
				Path:        cliPath,
				Description: desc,
				Mode:        mode,
				WireMethod:  wireMethod,
				GlobalPipes: true,
			}
			if node != nil {
				e.Args = extractArgs(node)
				e.Subcommands = extractSubcommands(node)
				e.Backend = node.Backend
				if node.TaskSupport != "" {
					e.TaskSupport = node.TaskSupport
				}
			}
			e.Pipes = extractPipes(cliPath)
			entries = append(entries, e)
			seen[cliPath] = true
		}
	}

	for _, lc := range registry.ListLocal() {
		if seen[lc.Path] {
			continue
		}
		mode := lc.Meta.Mode
		if mode == "" {
			mode = "offline"
		}
		entries = append(entries, commandEntry{
			Path:        lc.Path,
			Description: lc.Meta.Description,
			Mode:        mode,
		})
		seen[lc.Path] = true
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})

	return entries
}

// findNode looks up the node for a CLI path in the command tree.
func findNode(tree *command.Node, cliPath string) *command.Node {
	if tree == nil {
		return nil
	}
	return command.FindNode(tree, strings.Fields(cliPath))
}

// extractArgs converts a node's ArgDefs into the JSON-friendly commandArg slice.
func extractArgs(node *command.Node) []commandArg {
	if len(node.ArgDefs) == 0 {
		return nil
	}
	args := make([]commandArg, 0, len(node.ArgDefs))
	for _, ad := range node.ArgDefs {
		a := commandArg{
			Name:      ad.Name,
			Type:      argKindString(ad.Kind),
			Mandatory: ad.Mandatory,
		}
		if len(ad.EnumValues) > 0 {
			a.Values = ad.EnumValues
		}
		args = append(args, a)
	}
	return args
}

func argKindString(k command.ArgKind) string {
	switch k {
	case command.ArgEnum:
		return "enum"
	case command.ArgUint:
		return "uint"
	case command.ArgUnion:
		return "union"
	default:
		return "string"
	}
}

// extractSubcommands returns sorted child names for a node.
func extractSubcommands(node *command.Node) []string {
	if len(node.Children) == 0 {
		return nil
	}
	names := make([]string, 0, len(node.Children))
	for name := range node.Children {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// extractPipes returns command-specific pipe filters registered for a CLI path.
func extractPipes(cliPath string) []commandPipe {
	filters := command.PipeFiltersForCommand(cliPath)
	if len(filters) == 0 {
		return nil
	}
	pipes := make([]commandPipe, 0, len(filters))
	for _, f := range filters {
		pipes = append(pipes, commandPipe{
			Name:        f.Name,
			Description: f.Description,
			TakesArg:    f.TakesArg,
		})
	}
	return pipes
}

// filterCommands returns entries whose path or description contains
// the filter string (case-insensitive).
func filterCommands(entries []commandEntry, filter string) []commandEntry {
	lower := strings.ToLower(filter)
	var filtered []commandEntry
	for i := range entries {
		if strings.Contains(strings.ToLower(entries[i].Path), lower) ||
			strings.Contains(strings.ToLower(entries[i].Description), lower) {
			filtered = append(filtered, entries[i])
		}
	}
	return filtered
}

// printCommandJSON writes entries as a JSON array to w and returns the exit code.
func printCommandJSON(w io.Writer, entries []commandEntry) int {
	rw := helpfmt.NewRenderWriter(w)
	enc := json.NewEncoder(rw)
	enc.SetIndent("", "  ")
	if err := enc.Encode(entries); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err) //nolint:errcheck // one-shot error to stderr
		return 1
	}
	return rw.ExitCode()
}

// printCommandVerbose writes a detailed entry for each command through rw.
func printCommandVerbose(rw *helpfmt.RenderWriter, entries []commandEntry) {
	if len(entries) == 0 {
		fmt.Fprintf(os.Stderr, "no commands found\n") //nolint:errcheck // one-shot diagnostic to stderr
		return
	}

	var tb textbuf.Buffer
	tb.SetColor(slogutil.UseColor(os.Stdout))
	c := textbuf.C

	for i := range entries {
		e := &entries[i]
		if i > 0 {
			rw.Line("")
		}
		// Command path
		tb.Reset().Colored(c.BoldCyan).Str(e.Path).Colored(c.Reset)
		rw.Line(tb.Slice())

		// Description (full, multi-line)
		desc := e.Description
		if desc == "" {
			desc = "-"
		}
		for line := range strings.SplitSeq(desc, "\n") {
			tb.Reset().Str("  ").Str(line)
			rw.Line(tb.Slice())
		}

		// Mode, wire method, backend, task support
		tb.Reset().Str("  ").Colored(c.Dim).Str("mode: ").Str(e.Mode).Colored(c.Reset)
		if e.WireMethod != "" {
			tb.Str("  ").Colored(c.Dim).Str("wire: ").Str(e.WireMethod).Colored(c.Reset)
		}
		rw.Line(tb.Slice())

		if len(e.Backend) > 0 {
			tb.Reset().Str("  ").Colored(c.BrightYellow).Str("backend: ").Colored(c.Reset).Str(textbuf.Join(e.Backend, ", "))
			rw.Line(tb.Slice())
		}

		if e.TaskSupport != "" {
			tb.Reset().Str("  ").Colored(c.Dim).Str("task-support: ").Str(e.TaskSupport).Colored(c.Reset)
			rw.Line(tb.Slice())
		}

		// Arguments
		if len(e.Args) > 0 {
			tb.Reset().Str("  ").Colored(c.BrightYellow).Str("arguments:").Colored(c.Reset)
			rw.Line(tb.Slice())
			for _, a := range e.Args {
				tb.Reset().Str("    ").Str(a.Name).Str(" (").Str(a.Type).Str(")")
				if a.Mandatory {
					tb.Str(" REQUIRED")
				}
				rw.Line(tb.Slice())
				if len(a.Values) > 0 {
					tb.Reset().Str("      values: ").Str(textbuf.Join(a.Values, ", "))
					rw.Line(tb.Slice())
				}
			}
		}

		// Pipes
		if e.GlobalPipes || len(e.Pipes) > 0 {
			tb.Reset().Str("  ").Colored(c.BrightYellow).Str("pipes:").Colored(c.Reset)
			rw.Line(tb.Slice())
			if e.GlobalPipes {
				tb.Reset().Str("    ").Colored(c.Dim).Str("json, table, text, yaml, ndjson, match, count, resolve, origin, no-more").Colored(c.Reset)
				rw.Line(tb.Slice())
			}
			for _, p := range e.Pipes {
				tb.Reset().Str("    ").Str(p.Name)
				if p.TakesArg {
					tb.Str(" <value>")
				}
				tb.Str("  ").Colored(c.Dim).Str(p.Description).Colored(c.Reset)
				rw.Line(tb.Slice())
			}
		}

		// Subcommands
		if len(e.Subcommands) > 0 {
			tb.Reset().Str("  ").Colored(c.BrightYellow).Str("subcommands: ").Colored(c.Reset).Str(textbuf.Join(e.Subcommands, ", "))
			rw.Line(tb.Slice())
		}
	}
	rw.Line("")
	tb.Reset().Int(int64(len(entries))).Str(" commands")
	rw.Line(tb.Slice())
}

// printCommandTable writes entries as a human-readable table through rw.
func printCommandTable(rw *helpfmt.RenderWriter, entries []commandEntry) {
	if len(entries) == 0 {
		fmt.Fprintf(os.Stderr, "no commands found\n") //nolint:errcheck // one-shot diagnostic to stderr
		return
	}

	width := 0
	for i := range entries {
		if n := len(entries[i].Path); n > width {
			width = n
		}
	}
	if width < 16 {
		width = 16
	}

	var tb textbuf.Buffer
	tb.SetColor(slogutil.UseColor(os.Stdout))
	c := textbuf.C
	for i := range entries {
		e := &entries[i]
		desc := e.Description
		if desc == "" {
			desc = "-"
		}
		if i := strings.Index(desc, "\n"); i >= 0 {
			desc = desc[:i]
		}
		pad := strings.Repeat(" ", width-len(e.Path))
		tb.Reset().Str("  ").Colored(c.BoldCyan).Str(e.Path).Str(pad).Colored(c.Reset).Str("  ").Str(desc)
		rw.Line(tb.Slice())
	}
	rw.Line("")
	rw.Line(tb.Reset().Int(int64(len(entries))).Str(" commands").Slice())
}

// helpCommandUsage prints usage for `ze help command`.
func helpCommandUsage() {
	p := helpfmt.Page{
		Command: "ze help command",
		Summary: "List all available commands with descriptions",
		Usage:   []string{"ze help command [<filter>] [--json] [--verbose]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Options", Entries: []helpfmt.HelpEntry{
				{Name: "<filter>", Desc: "Show only commands matching this string (path or description)"},
				{Name: "--verbose, -v", Desc: "Show full description, arguments, pipes, and subcommands"},
				{Name: "--json", Desc: "Output as JSON array (for tooling, wiki generation)"},
			}},
		},
	}
	p.WriteErr()
}
