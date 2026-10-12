// Design: docs/features/ai-first.md — skills command
// Overview: register.go — command registration

package skills

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/command"
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

// showSkills executes `show skills`: it lists every bundled skill.
func showSkills(validated command.ValidatedArgs) int {
	args := validated.Tokens()
	if wantsHelp(args) {
		usage()
		return 0
	}
	return runList(args)
}

// showSkill executes `show skill <name>`: it prints one bundled skill.
func showSkill(validated command.ValidatedArgs) int {
	args := validated.Tokens()
	if wantsHelp(args) {
		usage()
		return 0
	}
	return runGet(args)
}

// wantsHelp reports whether the operator asked for the usage page.
func wantsHelp(args []string) bool {
	for _, a := range args {
		if a == "-h" {
			return true
		}
		if a == "--help" {
			return true
		}
	}
	return false
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
		Command:   "ze show skills",
		ShortHelp: "List and retrieve version-matched Ze skills for agents",
		Usage:     []string{"ze show skills [--json]", "ze show skill <name> [--full] [--json]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Commands", Entries: []helpfmt.HelpEntry{
				{Name: "show skills", Desc: "List all bundled skills"},
				{Name: "show skill <name>", Desc: "Retrieve skill content by name"},
			}},
			{Title: "Options", Entries: []helpfmt.HelpEntry{
				{Name: flagJSON, Desc: "Output as structured JSON"},
				{Name: "--full", Desc: "Return expanded content (with examples and patterns)"},
			}},
		},
		Examples: []string{
			"ze show skills",
			"ze show skill ze",
			"ze show skill ze-diagnostics --full",
			"ze show skills --json",
		},
	}
	p.WriteErr()
}
