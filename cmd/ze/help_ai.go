// Design: docs/guide/mcp/overview.md -- AI help reference generator
// Related: help_command.go -- the `ze help command` usage page

//go:build ze_core

// The reference itself is answered by the help plugin (internal/plugins/help),
// which registers `help ai` as a local-data command. What stays here is the
// `ze help` page and the deprecated --ai spelling dispatchHelp still accepts.

package main

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/cmd/ze/internal/helpfmt"
	"github.com/ze-software/ze/internal/component/command"
)

// aiHelpRequested checks if the deprecated --ai flag was passed in the help args.
// The canonical form is the "ai" subcommand (ze help ai); --ai is a hidden alias.
func aiHelpRequested(args []string) bool {
	return slices.Contains(args, "--ai")
}

func helpUsage() {
	// Derive help subcommands from the local command registry.
	var subEntries []helpfmt.HelpEntry
	for _, lc := range command.ListLocal() {
		if !strings.HasPrefix(lc.Path, "help ") {
			continue
		}
		name := strings.TrimPrefix(lc.Path, "help ")
		subEntries = append(subEntries, helpfmt.HelpEntry{Name: name, Desc: lc.Meta.ShortHelp})
	}

	var sections []helpfmt.HelpSection
	if len(subEntries) > 0 {
		sections = append(sections, helpfmt.HelpSection{Title: "Subcommands", Entries: subEntries})
	}
	sections = append(sections, helpfmt.HelpSection{
		Title: "AI reference", Entries: []helpfmt.HelpEntry{
			{Name: "ai", Desc: "Summary with counts and quick start"},
			{Name: "ai --json", Desc: "Machine-readable JSON reference"},
			{Name: "ai cli", Desc: "CLI subcommands"},
			{Name: "ai api", Desc: "Daemon API commands with parameters"},
			{Name: "ai mcp", Desc: "MCP tools with parameters and examples"},
			{Name: "ai dispatch", Desc: "Dispatch keys for daemon commands"},
			{Name: "ai all", Desc: "Everything combined"},
		},
	})

	p := helpfmt.Page{
		Command:   "ze help",
		ShortHelp: "Show help and AI reference",
		Usage:     []string{"ze help ai [cli|api|mcp|dispatch|all] [--json]"},
		Sections:  sections,
	}
	p.WriteErr()
}
