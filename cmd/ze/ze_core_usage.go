// Design: docs/architecture/system-architecture.md -- ze usage

//go:build ze_core

package main

import (
	"github.com/ze-software/ze/cmd/ze/internal/helpfmt"
	cli "github.com/ze-software/ze/internal/component/cli/client"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
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
		Title: helpOptionsSectionTitle,
		Entries: []helpfmt.HelpEntry{
			{Name: "-d, --debug", Desc: "Enable debug logging (sets ze.log=debug for all subsystems)"},
			{Name: "-f <file>", Desc: "Use filesystem directly, bypass blob store"},
			{Name: "--plugin <name>", Desc: "Load plugin before starting (repeatable)"},
			{Name: "--web <port>", Desc: "Start web server on given port"},
			{Name: flagStartInsecureWeb, Desc: "Disable web auth (binds to localhost only)"},
			{Name: helpMCPPortOption, Desc: "Start MCP server on 127.0.0.1:<port>"},
			{Name: "--mcp-token <token>", Desc: "Bearer token for MCP authentication"},
			{Name: "--pprof <addr:port>", Desc: "Start pprof HTTP server (e.g. :6060)"},
			{Name: "--color", Desc: "Force colored output (even when not a TTY)"},
			{Name: "--no-color", Desc: "Disable colored output (also: NO_COLOR env var, TERM=dumb)"},
			{Name: "-V, --version", Desc: "Show version and exit"},
			{Name: flagExtendedVersion, Desc: "Show extended version (commit, go, os/arch)"},
		},
	})

	p := helpfmt.Page{
		Command:  "ze",
		Software: "ze Software",
		Usage: []string{
			"ze start [--plugin <name>]... <config>  Start with config file",
			"ze [--plugin <name>]... -               Start with config on stdin",
			"ze <verb> <command> [options]           Execute command (same grammar as ze cli)",
		},
		Sections: sections,
		Examples: []string{
			"ze start config.conf                 Start with config",
			"ze --plugin ze.hostname -            Start with hostname plugin, config on stdin",
			"ze show plugins                      List the plugins in this build",
			"ze help ai                           AI reference (commands, RPCs, MCP tools)",
			"ze help ai api                       Daemon API endpoints (ze-show:*, ...)",
			"ze cli                               Interactive CLI",
			"ze show bgp peer list                Show peer list",
			"ze show help                         List available show commands",
			"ze delete bgp peer 10.0.0.1        Remove a peer",
			"ze bgp decode <hex>                  Decode BGP message",
		},
	}
	p.WriteErr()
}
