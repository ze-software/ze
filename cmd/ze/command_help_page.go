//go:build ze_core

// Design: docs/architecture/api/commands.md -- the operator's per-command help
// Related: help_command.go -- the same two projections, published as a catalog
//
// command_help_page.go builds what an operator reads for one command path. The
// invocation form is GENERATED from the command model, so the page states the
// arguments the command actually declares instead of a fixed "<command>
// [options]" that names none of them.

package main

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// commandHelpPage builds the help page for the command at path. A nil node is
// a path the tree does not hold, and it still produces a page.
//
// A node that runs a command states its generated form FIRST, because that is
// what the operator types. A node that also has subcommands keeps the
// navigation line after it, so both readings of the path are on the page.
//
// The node's two declared help texts go to the two fields that render them.
// The summary goes on the header line. The long explanation goes in the body
// block. Neither is derived from the other, and neither is shortened here.
func commandHelpPage(path []string, node *command.Node) helpfmt.Page {
	var tb textbuf.Buffer
	cmdPath := tb.Str("ze ").Join(path, " ").String()

	page := helpfmt.Page{Command: cmdPath}
	if node == nil {
		page.Usage = []string{navigationUsage(cmdPath)}
		return page
	}

	// Both texts are set whatever the node holds. A node with children states
	// its OWN summary and its OWN explanation first, and its children's
	// summaries after. Guarding these on a childless node is the defect this
	// page exists to remove: a parent that printed only its children left its
	// authored text unreachable to the operator who asked about that exact
	// path, which is what the retired writeHelp renderer did.
	page.Summary = node.Description
	page.LongHelp = node.LongHelp

	if tokens := command.Usage(path, node); len(tokens) > 0 {
		tb.Reset()
		page.Usage = append(page.Usage, tb.Str("ze ").Str(command.UsageLine(tokens)).String())
	}
	if len(node.Children) > 0 || len(page.Usage) == 0 {
		page.Usage = append(page.Usage, navigationUsage(cmdPath))
	}

	entries := make([]helpfmt.HelpEntry, 0, len(node.Children))
	for _, entry := range command.HelpEntries(node, nil) {
		entries = append(entries, helpfmt.HelpEntry{Name: entry.Name, Desc: entry.Desc})
	}
	page.Sections = []helpfmt.HelpSection{{Title: "Commands", Entries: entries}}

	return page
}

// navigationUsage is the line that says a path carries subcommands.
func navigationUsage(cmdPath string) string {
	var tb textbuf.Buffer
	return tb.Str(cmdPath).Str(" <command> [options]").String()
}
