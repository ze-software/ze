// Design: docs/architecture/system-architecture.md -- ze usage and plugin listing

//go:build ze_core

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	cli "codeberg.org/thomas-mangin/ze/internal/component/cli/client"
	"codeberg.org/thomas-mangin/ze/internal/component/command"
	"codeberg.org/thomas-mangin/ze/internal/component/command/registry"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

func zeUsage() {
	verbTree := cli.BuildCommandTree(false)
	cmdEntries := command.HelpEntries(verbTree, nil)
	verbEntries := make([]helpfmt.HelpEntry, len(cmdEntries))
	for i, e := range cmdEntries {
		verbEntries[i] = helpfmt.HelpEntry{Name: e.Name, Desc: e.Desc}
	}

	sections := []helpfmt.HelpSection{
		{Title: registry.SectionTitle(registry.SectionOperations)},
	}
	for _, se := range registry.ListRootBySection() {
		entries := make([]helpfmt.HelpEntry, len(se.Commands))
		for i, rc := range se.Commands {
			entries[i] = helpfmt.HelpEntry{Name: rc.Name, Desc: rc.Meta.Description}
		}
		title := registry.SectionTitle(se.Section)
		if title == "" {
			title = se.Section
		}
		if se.Section == registry.SectionOperations {
			sections[0].Entries = append(entries, sections[0].Entries...)
		} else {
			sections = append(sections, helpfmt.HelpSection{Title: title, Entries: entries})
		}
	}
	sections[0].Entries = append(sections[0].Entries, verbEntries...)

	sections = append(sections, helpfmt.HelpSection{
		Title: "Options",
		Entries: []helpfmt.HelpEntry{
			{Name: "-d, --debug", Desc: "Enable debug logging (sets ze.log=debug for all subsystems)"},
			{Name: "-f <file>", Desc: "Use filesystem directly, bypass blob store"},
			{Name: "--plugin <name>", Desc: "Load plugin before starting (repeatable)"},
			{Name: "--plugins", Desc: "List available internal plugins"},
			{Name: "--web <port>", Desc: "Start web server on given port"},
			{Name: "--insecure-web", Desc: "Disable web auth (binds to localhost only)"},
			{Name: "--mcp <port>", Desc: "Start MCP server on 127.0.0.1:<port>"},
			{Name: "--mcp-token <token>", Desc: "Bearer token for MCP authentication"},
			{Name: "--pprof <addr:port>", Desc: "Start pprof HTTP server (e.g. :6060)"},
			{Name: "--color", Desc: "Force colored output (even when not a TTY)"},
			{Name: "--no-color", Desc: "Disable colored output (also: NO_COLOR env var, TERM=dumb)"},
			{Name: "-V, --version", Desc: "Show version and exit"},
			{Name: "--extended-version", Desc: "Show extended version (commit, go, os/arch)"},
		},
	})

	p := helpfmt.Page{
		Command:  "ze",
		Software: "ze Software",
		Usage: []string{
			"ze [--plugin <name>]... <config>   Start with config file",
			"ze <verb> <command> [options]      Execute command (same grammar as ze cli)",
		},
		Sections: sections,
		Examples: []string{
			"ze config.conf                       Start with config",
			"ze --plugin ze.hostname config.conf  Start with hostname plugin",
			"ze --plugins                         List available plugins",
			"ze cli                               Interactive CLI",
			"ze show bgp peer list                Show peer list",
			"ze show help                         List available show commands",
			"ze delete bgp peer 10.0.0.1        Remove a peer",
			"ze bgp decode <hex>                  Decode BGP message",
		},
	}
	p.Write()
}

func printPlugins(jsonOutput bool) {
	plugins := plugin.InternalPluginInfo()

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(plugins)
		return
	}

	fmt.Fprintf(os.Stdout, "%-12s  %-35s  %-20s  %-15s  %s\n", //nolint:errcheck // CLI output
		"NAME", "DESCRIPTION", "RFC", "CAPABILITY", "FAMILY")
	fmt.Fprintf(os.Stdout, "%-12s  %-35s  %-20s  %-15s  %s\n", //nolint:errcheck // CLI output
		"----", "-----------", "---", "----------", "------")

	for _, info := range plugins {
		rfcs := strings.Join(info.RFCs, ", ")
		caps := ""
		if len(info.Capabilities) > 0 {
			capStrs := make([]string, len(info.Capabilities))
			for i, c := range info.Capabilities {
				capStrs[i] = strconv.Itoa(c)
			}
			caps = strings.Join(capStrs, ", ")
		}
		families := strings.Join(info.Families, ", ")

		fmt.Fprintf(os.Stdout, "%-12s  %-35s  %-20s  %-15s  %s\n", //nolint:errcheck // CLI output
			info.Name, info.Description, rfcs, caps, families)
	}
}
