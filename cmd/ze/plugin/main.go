// Design: docs/architecture/api/process-protocol.md — plugin CLI dispatch
//
// Package plugin provides the ze plugin subcommand.
package plugin

import (
	"fmt"
	"os"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/suggest"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// Run executes the plugin subcommand with the given arguments.
// Returns exit code.
func Run(args []string) int {
	if len(args) < 1 {
		usage()
		return 1
	}

	switch args[0] {
	case "test":
		// Test is a debugging tool, not a real plugin.
		return cmdPluginTest(args[1:])
	case "help", "-h", "--help": //nolint:goconst // consistent with main.go
		usage()
		return 0
	}

	// Look up in registry.
	reg := registry.Lookup(args[0])
	if reg == nil {
		fmt.Fprintf(os.Stderr, "unknown plugin subcommand: %s\n", args[0])
		if s := suggest.Command(args[0], append(registry.Names(), "test", "help")); s != "" {
			fmt.Fprintf(os.Stderr, "hint: did you mean '%s'?\n", s)
		}
		usage()
		return 1
	}
	return reg.CLIHandler(args[1:])
}

func usage() {
	// Build plugin entries dynamically from the registry.
	regs := registry.All()
	pluginEntries := make([]helpfmt.HelpEntry, 0, len(regs)+2)
	for _, reg := range regs {
		desc := reg.Description
		if len(reg.RFCs) > 0 {
			desc += " (RFC " + strings.Join(reg.RFCs, ", ") + ")"
		}
		pluginEntries = append(pluginEntries, helpfmt.HelpEntry{Name: reg.Name, Desc: desc})
	}
	pluginEntries = append(pluginEntries,
		helpfmt.HelpEntry{Name: "test", Desc: "Test plugin YANG schema and config delivery (debugging)"},
		helpfmt.HelpEntry{Name: "help", Desc: "Show this help"},
	)

	p := helpfmt.Page{
		Command: "ze plugin",
		Summary: "Plugin subcommands",
		Usage:   []string{"ze plugin <subcommand>"},
		Sections: []helpfmt.HelpSection{
			{Title: "Plugin Subcommands", Entries: pluginEntries},
		},
		Examples: []string{
			`ze plugin test --plugin ze.hostname --schema config.conf`,
			`ze plugin test --plugin ze.hostname --tree config.conf`,
			`ze plugin test --plugin ze.hostname --json config.conf`,
		},
		SeeAlso: []string{
			"Plugins run as API processes spawned by the router via plugin configuration.",
		},
	}
	p.Write()
}
