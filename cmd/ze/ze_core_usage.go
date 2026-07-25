// Design: docs/architecture/system-architecture.md -- ze usage and plugin listing

//go:build ze_core

package main

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/ze-software/ze/cmd/ze/internal/helpfmt"
	cli "github.com/ze-software/ze/internal/component/cli/client"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/textbuf"
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
			"ze start [--plugin <name>]... <config>  Start with config file",
			"ze [--plugin <name>]... -               Start with config on stdin",
			"ze <verb> <command> [options]           Execute command (same grammar as ze cli)",
		},
		Sections: sections,
		Examples: []string{
			"ze start config.conf                 Start with config",
			"ze --plugin ze.hostname -            Start with hostname plugin, config on stdin",
			"ze --plugins                         List available plugins",
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

func printPlugins(jsonOutput bool) {
	plugins := plugin.InternalPluginInfo()

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(plugins)
		return
	}

	entries := make([]helpfmt.HelpEntry, 0, len(plugins))
	var tb textbuf.Buffer
	for _, info := range plugins {
		tb.Reset()
		desc := helpfmt.Summary(info.Description)
		tb.Str(desc)

		if len(info.Families) > 0 {
			tb.Str(" [").Str(textbuf.Join(info.Families, ", ")).Byte(']')
		}
		if len(info.RFCs) > 0 && !strings.Contains(desc, "RFC") {
			tb.Str(" (RFC ").Str(textbuf.Join(info.RFCs, ", ")).Byte(')')
		}

		entries = append(entries, helpfmt.HelpEntry{Name: info.Name, Desc: tb.String()})
	}

	p := helpfmt.Page{
		Command: "ze --plugins",
		Summary: "Internal plugins available in this build",
		Sections: []helpfmt.HelpSection{
			{Title: "Plugins", Entries: entries},
		},
	}
	p.WriteOut()
}
