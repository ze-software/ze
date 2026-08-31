// Design: docs/features/ai-first.md — skills command
// Overview: register.go — command registration

package skills

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// flagJSON is the CLI flag that selects structured output.
const flagJSON = "--json"

//go:embed data/*.md
var skillFS embed.FS

type skill struct {
	Name        string
	Description string
	File        string
	FullFile    string
}

var inventory = []skill{
	{Name: "ze", Description: "Ze network OS overview and agent entry points", File: "data/ze.md", FullFile: "data/ze-full.md"},
	{Name: "ze-diagnostics", Description: "Read Ze diagnostics, explanations, and fix plans", File: "data/ze-diagnostics.md"},
	{Name: "ze-config", Description: "Ze config syntax, validation, and common errors", File: "data/ze-config.md"},
	{Name: "ze-commands", Description: "CLI command reference, dispatch keys, and MCP tools", File: "data/ze-commands.md"},
	{Name: "ze-agent", Description: "Agent workflow for config validation and repair", File: "data/ze-agent.md"},
}

// Run executes the skills command.
func Run(args []string) int {
	if len(args) == 0 {
		usage()
		return 1
	}

	switch args[0] {
	case "help", "-h", "--help":
		usage()
		return 0
	case "list":
		return runList(args[1:])
	case "get":
		return runGet(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown skills subcommand: %s\n", args[0])
		usage()
		return 1
	}
}

func runList(args []string) int {
	jsonOutput := false
	for _, a := range args {
		if a == flagJSON {
			jsonOutput = true
		}
	}

	if jsonOutput {
		entries := make([]diagnostic.SkillEntry, len(inventory))
		for i, s := range inventory {
			entries[i] = diagnostic.SkillEntry{Name: s.Name, Description: s.Description}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(entries); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}

	b := textbuf.Get()
	defer b.Release()
	for _, s := range inventory {
		b.Reset().Str(s.Name).Str(" — ").Str(s.Description).Byte('\n')
		if err := b.StdOut(); err != nil {
			return 1
		}
	}
	return 0
}

func runGet(args []string) int {
	jsonOutput := false
	full := false
	var name string

	for _, a := range args {
		switch a {
		case flagJSON:
			jsonOutput = true
		case "--full":
			full = true
		default:
			name = a
		}
	}

	if name == "" {
		fmt.Fprintf(os.Stderr, "error: missing skill name\n")
		fmt.Fprintf(os.Stderr, "available: %s\n", skillNames())
		return 1
	}

	var found *skill
	for i := range inventory {
		if inventory[i].Name == name {
			found = &inventory[i]
			break
		}
	}
	if found == nil {
		fmt.Fprintf(os.Stderr, "error: unknown skill: %s\n", name)
		fmt.Fprintf(os.Stderr, "available: %s\n", skillNames())
		return 1
	}

	file := found.File
	if full && found.FullFile != "" {
		file = found.FullFile
	}

	content, err := skillFS.ReadFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: reading skill: %v\n", err)
		return 1
	}

	if jsonOutput {
		result := struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Content     string `json:"content"`
		}{
			Name:        found.Name,
			Description: found.Description,
			Content:     string(content),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}

	if _, err := os.Stdout.Write(content); err != nil {
		return 1
	}
	return 0
}

func skillNames() string {
	names := make([]string, len(inventory))
	for i, s := range inventory {
		names[i] = s.Name
	}
	return textbuf.Join(names, ", ")
}

func usage() {
	p := helpfmt.Page{
		Command: "ze skills",
		Summary: "List and retrieve version-matched Ze skills for agents",
		Usage:   []string{"ze skills list [--json]", "ze skills get <name> [--full] [--json]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Commands", Entries: []helpfmt.HelpEntry{
				{Name: "list", Desc: "List all bundled skills"},
				{Name: "get <name>", Desc: "Retrieve skill content by name"},
			}},
			{Title: "Options", Entries: []helpfmt.HelpEntry{
				{Name: flagJSON, Desc: "Output as structured JSON"},
				{Name: "--full", Desc: "Return expanded content (with examples and patterns)"},
			}},
		},
		Examples: []string{
			"ze skills list",
			"ze skills get ze",
			"ze skills get ze-diagnostics --full",
			"ze skills list --json",
		},
	}
	p.WriteErr()
}
