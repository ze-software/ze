// Design: docs/guide/mcp/overview.md -- AI help reference generator

//go:build ze_core

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	"codeberg.org/thomas-mangin/ze/internal/component/aihelp"
	cli "codeberg.org/thomas-mangin/ze/internal/component/cli/client"
	cmdregistry "codeberg.org/thomas-mangin/ze/internal/component/command/registry"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

// printAIHelp outputs a machine-friendly reference generated from code.
// All data comes from the plugin registry, YANG schemas, and RPC registrations
// so it is always in sync with the running binary.
//
// Sections are selected positionally:
//
//	ze help ai           Summary / table of contents
//	ze help ai cli       CLI subcommands (ze bgp, ze config, ...)
//	ze help ai api       Daemon API commands (YANG RPCs)
//	ze help ai mcp       MCP tools with parameters
//	ze help ai all       Everything
//
// The legacy flag form (ze help --ai --api) is accepted as a hidden alias, so
// section detection matches both "api" and "--api". --json stays a format flag.
func printAIHelp(args []string) {
	if slices.Contains(args, "--json") {
		printAIHelpJSON()
		return
	}

	showCLI := hasSection(args, "cli")
	showAPI := hasSection(args, "api")
	showMCP := hasSection(args, "mcp")
	showDispatch := hasSection(args, "dispatch")
	showAll := hasSection(args, "all")

	if showAll {
		showCLI = true
		showAPI = true
		showMCP = true
		showDispatch = true
	}

	summaryOnly := !showCLI && !showAPI && !showMCP && !showDispatch

	fmt.Println("# Ze AI Reference")
	fmt.Println("# Generated from code -- always matches this binary.")
	fmt.Println()

	if summaryOnly {
		printSummary()
		return
	}

	if showCLI {
		printCLICommands()
	}
	if showAPI {
		printAPICommands()
		printUpdateSyntax()
		printFamilies()
		printAIPlugins()
		printPeerSelectors()
		printFamilyAttributes()
		printRIBPipeline()
	}
	if showDispatch {
		printDispatchKeys()
	}
	if showMCP {
		printMCPTools()
	}

	// Recipes and errors are useful in any detailed view.
	if showCLI || showAPI || showMCP {
		printServices()
		printRecipes()
		printCommonErrors()
	}
	if showCLI || showMCP {
		printMinimalConfig()
	}
}

func printSummary() {
	fmt.Println("## Sections (use 'ze help ai <section>' for details)")
	fmt.Println()
	fmt.Println("  cli       CLI subcommands: ze bgp, ze config, ze show, ze signal, ...")
	fmt.Println("  api       Daemon API: all RPC commands, update syntax, families, plugins")
	fmt.Println("  mcp       MCP tools: ze_announce, ze_withdraw, ze_peers, ze_peer_control")
	fmt.Println("  dispatch  Dispatch keys for daemon commands")
	fmt.Println("  all       Everything")
	fmt.Println()

	regs := registry.All()
	var familyCount int
	seen := make(map[string]bool)
	for _, r := range regs {
		for _, f := range r.Families {
			if !seen[f] {
				seen[f] = true
				familyCount++
			}
		}
	}

	schemaReg := aihelp.SchemaRegistry()
	rpcCount := len(schemaReg.ListRPCs(""))
	builtinCount := len(pluginserver.AllBuiltinRPCs())

	fmt.Printf("  %d plugins, %d address families\n", len(regs), familyCount)
	fmt.Printf("  %d YANG RPCs, %d builtin RPCs\n", rpcCount, builtinCount)
	fmt.Printf("  MCP tools auto-generated from YANG command registry (ze_execute, ze_commands + per-group tools)\n")
	fmt.Println()
	fmt.Println("## Quick Start")
	fmt.Println()
	fmt.Println("  Daemon:  ze start --mcp 9718")
	fmt.Println("  Or:      ze --mcp 9718 config.conf")
	fmt.Println("  CLI:     ze cli")
	fmt.Println("  Show:    ze show <command>")
	fmt.Println("  Help:    ze help ai all")
}

func printCLICommands() {
	fmt.Println("## CLI Subcommands")
	fmt.Println()
	fmt.Println("  ze [global-flags] <command> [options]")
	fmt.Println()
	fmt.Println("  Modes: [offline] no daemon needed  [daemon] requires running daemon  [setup] one-time setup")
	fmt.Println()
	fmt.Println("  Global flags:")
	fmt.Println("    -d, --debug            Enable debug logging")
	fmt.Println("    -f <file>              Use filesystem directly, bypass blob store")
	fmt.Println("    --plugin <name>        Load plugin before starting (repeatable)")
	fmt.Println("    --mcp <port>           Start MCP server on 127.0.0.1:<port>")
	fmt.Println("    --web <port>           Start web server on 0.0.0.0:<port>")
	fmt.Println("    --web-only             Web UI only, no daemon (config editing only)")
	fmt.Println("    --insecure-web         Disable web auth (forces 127.0.0.1)")
	fmt.Println("    --pprof <addr:port>    Start pprof HTTP server")
	fmt.Println("    -V, --version          Show version")
	fmt.Println()

	// CLI tree. The subcommand list is static text that matches the dispatcher
	// in cmd/ze/main.go. It changes rarely and is verified by functional tests.
	cmds := aihelp.CLISubcommands()
	for _, c := range cmds {
		var tb textbuf.Buffer
		tb.Str("  ze ").PadRight(c.Name, 14).Str(" [").PadRight(c.Mode, 7).Str("] ").Str(c.Description)
		fmt.Println(tb.Slice())
		if c.Subs != "" {
			tb.Reset()
			tb.Str("    ").Str(c.Subs)
			fmt.Println(tb.Slice())
		}
	}
	fmt.Println()
}

func printAPICommands() {
	fmt.Println("## Daemon API Commands (YANG RPCs)")
	fmt.Println()
	fmt.Println("Format: wire-method (dispatch-key) description")
	fmt.Println()

	wireToPath := cli.WireToPath()
	schemaReg := aihelp.SchemaRegistry()

	rpcs := schemaReg.ListRPCs("")
	sort.Slice(rpcs, func(i, j int) bool {
		return rpcs[i].WireMethod < rpcs[j].WireMethod
	})

	for _, rpc := range rpcs {
		desc := rpc.Description
		if desc == "" {
			desc = "(no description)"
		}
		if idx := strings.Index(desc, ". "); idx > 0 && idx < 80 {
			desc = desc[:idx+1]
		}

		ro := ""
		if cliPath := wireToPath[rpc.WireMethod]; cliPath != "" && pluginserver.IsReadOnlyPath(cliPath) {
			ro = " [read-only]"
		}

		dispatch := wireToPath[rpc.WireMethod]
		if dispatch != "" {
			fmt.Printf("  %-44s (%-30s) %s%s\n", rpc.WireMethod, dispatch, desc, ro)
		} else {
			fmt.Printf("  %-44s %-32s %s%s\n", rpc.WireMethod, "", desc, ro)
		}
		for _, leaf := range rpc.Input {
			req := ""
			if leaf.Mandatory {
				req = " (REQUIRED)"
			}
			leafDesc := leaf.Description
			if leafDesc == "" {
				leafDesc = leaf.Type
			}
			fmt.Printf("    %-24s %s%s\n", leaf.Name, leafDesc, req)
		}
	}

	// Builtin RPCs without YANG metadata.
	shown := make(map[string]bool, len(rpcs))
	for _, rpc := range rpcs {
		shown[rpc.WireMethod] = true
	}

	builtins := pluginserver.AllBuiltinRPCs()
	sort.Slice(builtins, func(i, j int) bool {
		return builtins[i].WireMethod < builtins[j].WireMethod
	})

	for _, b := range builtins {
		if shown[b.WireMethod] {
			continue
		}
		help := b.WireMethod
		ro := ""
		dispatch := wireToPath[b.WireMethod]
		if dispatch != "" {
			if pluginserver.IsReadOnlyPath(dispatch) {
				ro = " [read-only]"
			}
			fmt.Printf("  %-44s (%-30s) %s%s\n", b.WireMethod, dispatch, help, ro)
		} else {
			fmt.Printf("  %-44s %-32s %s%s\n", b.WireMethod, "", help, ro)
		}
	}
	fmt.Println()
}

func printDispatchKeys() {
	fmt.Println("## Dispatch Keys (what you type)")
	fmt.Println()
	fmt.Println("These are the strings accepted by the daemon dispatcher.")
	fmt.Println("Use with: ze cli -c \"<dispatch-key>\"")
	fmt.Println()

	wireToPath := cli.WireToPath()
	builtins := pluginserver.AllBuiltinRPCs()

	type entry struct {
		dispatch   string
		wireMethod string
	}

	var entries []entry
	for _, b := range builtins {
		path := wireToPath[b.WireMethod]
		if path == "" {
			continue
		}
		entries = append(entries, entry{dispatch: path, wireMethod: b.WireMethod})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].dispatch < entries[j].dispatch
	})

	fmt.Printf("  %-40s %s\n", "DISPATCH KEY", "WIRE METHOD")
	for _, e := range entries {
		fmt.Printf("  %-40s %s\n", e.dispatch, e.wireMethod)
	}
	fmt.Println()
}

func printUpdateSyntax() {
	fmt.Println("## Update Text Syntax")
	fmt.Println()
	fmt.Println("  peer <selector> update text [attributes] nlri <family> <action> <prefix>...")
	fmt.Println()
	fmt.Println("  Selectors:  * (all), <ip-address>, <peer-name>")
	fmt.Println("  Actions:    add <prefix>, del <prefix>, eor")
	fmt.Println()
	fmt.Println("  Attributes (common):")
	fmt.Println("    origin <igp|egp|incomplete>")
	fmt.Println("    next-hop <ip-address>             (alias: nhop)")
	fmt.Println("    local-preference <N>")
	fmt.Println("    med <N>")
	fmt.Println("    as-path [<asn> ...]")
	fmt.Println("    community <value>                 (e.g. 65000:100, no-export)")
	fmt.Println("    large-community <value>           (e.g. 65000:100:200)")
	fmt.Println("    extended-community <value>")
	fmt.Println()
	fmt.Println("  Attributes (family-specific):")
	fmt.Println("    path-id <N>                       (ADD-PATH path identifier)")
	fmt.Println("    rd <value>                        (Route Distinguisher for VPN)")
	fmt.Println("    label <N>                         (MPLS label for labeled/VPN)")
	fmt.Println()
	fmt.Println("  Example:")
	fmt.Println("    peer * update text origin igp next-hop 1.1.1.1 local-preference 100 nlri ipv4/unicast add 10.0.0.0/24")
	fmt.Println()
}

func printFamilies() {
	fmt.Println("## Address Families")
	fmt.Println()

	families := make(map[string][]string)
	for _, reg := range registry.All() {
		for _, fam := range reg.Families {
			families[fam] = append(families[fam], reg.Name)
		}
	}

	// Builtin families (engine, not registered by plugins).
	for _, fam := range family.RegisteredFamilyNames() {
		if _, ok := families[fam]; !ok {
			families[fam] = []string{"builtin"}
		}
	}

	sorted := make([]string, 0, len(families))
	for f := range families {
		sorted = append(sorted, f)
	}
	sort.Strings(sorted)

	for _, fam := range sorted {
		plugins := families[fam]
		fmt.Printf("  %-24s (%s)\n", fam, textbuf.Join(plugins, ", "))
	}
	fmt.Println()
}

func printAIPlugins() {
	fmt.Println("## Plugins")
	fmt.Println()

	regs := registry.All()
	sort.Slice(regs, func(i, j int) bool {
		return regs[i].Name < regs[j].Name
	})

	for _, reg := range regs {
		fmt.Printf("  %-24s %s\n", reg.Name, reg.Description)
		if len(reg.RFCs) > 0 {
			fmt.Printf("    RFCs: %s\n", textbuf.Join(reg.RFCs, ", "))
		}
		if len(reg.Families) > 0 {
			fmt.Printf("    Families: %s\n", textbuf.Join(reg.Families, ", "))
		}
	}
	fmt.Println()
}

func printMCPTools() {
	fmt.Println("## MCP Tools (via --mcp <port>)")
	fmt.Println()
	fmt.Println("  Start: ze start --mcp <port>  or  ze --mcp <port> config.conf")
	fmt.Println("  Connect: POST http://127.0.0.1:<port>/ with JSON-RPC body")
	fmt.Println()
	fmt.Println("  Tools are auto-generated from the YANG command registry at tools/list time.")
	fmt.Println("  Each command group becomes a tool with an action enum. New YANG commands")
	fmt.Println("  appear as MCP tools automatically.")
	fmt.Println()
	fmt.Println("  Handcrafted tools:")
	fmt.Println("  ze_execute          Run any Ze command (escape hatch)")
	fmt.Println("    command           Full command string (REQUIRED)")
	fmt.Println()
	fmt.Println("  ze_commands         List all available daemon commands")
	fmt.Println()
	fmt.Println("  ze_reference        Full AI reference for this daemon (same as 'ze help ai --json')")
	fmt.Println()
	fmt.Println("  Run tools/list against a live daemon to see the full tool inventory.")
	fmt.Println()
	fmt.Println("  JSON-RPC Example:")
	fmt.Println(`    {"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"ze_execute","arguments":{"command":"summary"}}}`)
	fmt.Println()
}

func printPeerSelectors() {
	fmt.Println("## Peer Selectors")
	fmt.Println()
	fmt.Println("  Most commands accept a peer selector to target specific peers.")
	fmt.Println("  The reactor resolves selectors in priority order:")
	fmt.Println()
	fmt.Println("  *                All peers (default when omitted)")
	fmt.Println("  192.168.1.1      Exact IP address")
	fmt.Println("  my-peer          Peer name (from config, takes priority over IP)")
	fmt.Println("  as65001          All peers with remote AS 65001 (case-insensitive)")
	fmt.Println("  10.0.0.*         IP glob pattern (per-octet wildcard)")
	fmt.Println("  !192.168.1.1     Exclusion: all peers except this one")
	fmt.Println("  10.0.0.1,10.0.0.2  Comma-separated list (RIB commands only)")
	fmt.Println()
}

func printFamilyAttributes() {
	fmt.Println("## Family-Specific Attributes")
	fmt.Println()
	fmt.Println("  Some update text attributes only apply to specific address families:")
	fmt.Println()
	fmt.Println("  path-id <N>        ADD-PATH peers only (any family, requires ADD-PATH capability)")
	fmt.Println("  rd <value>         VPN families: ipv4/mpls-vpn, ipv6/mpls-vpn, l2vpn/evpn, l2vpn/vpls")
	fmt.Println("  label <N>          Labeled/VPN: ipv4/mpls-label, ipv6/mpls-label, */vpn")
	fmt.Println()
}

func printRIBPipeline() {
	fmt.Println("## RIB Show Pipeline")
	fmt.Println()
	fmt.Println("  show bgp rib [scope] [filters...] [terminal]")
	fmt.Println("  show bgp rib best [filters...] [terminal]")
	fmt.Println()
	fmt.Println("  Scopes (positional, first argument):")
	fmt.Println("    received         Adj-RIB-In only")
	fmt.Println("    sent             Adj-RIB-Out only")
	fmt.Println("    sent-received    Both (default)")
	fmt.Println()
	fmt.Println("  Filters (named, chainable):")
	fmt.Println("    family <afi/safi>     Address family (e.g. ipv4/unicast)")
	fmt.Println("    prefix <pattern>      Prefix string match (e.g. 192.168)")
	fmt.Println("    path <pattern>        AS-path: 64501 (anywhere), ^64501 (anchored), 64501,64502 (contiguous)")
	fmt.Println("    community <value>     Exact community match (e.g. 65000:100)")
	fmt.Println("    match <text>          Case-insensitive substring across all fields")
	fmt.Println()
	fmt.Println("  Terminals (last argument):")
	fmt.Println("    count                 Return {\"count\": N} instead of routes")
	fmt.Println("    json                  Full route details (default)")
	fmt.Println()
	fmt.Println("  Examples:")
	fmt.Println("    show bgp rib received family ipv4/unicast")
	fmt.Println("    show bgp rib sent prefix 10.0 count")
	fmt.Println("    show bgp rib received community 65000:100 path ^64501")
	fmt.Println("    show bgp rib best family ipv4/unicast json")
	fmt.Println()
}

// printServices generates the Services section from YANG conf modules.
// It walks all registered YANG modules looking for environment containers,
// extracts leaves with their types and defaults, and matches env vars.
func printServices() {
	fmt.Println("## Services (from YANG environment containers)")
	fmt.Println()
	fmt.Println("  Optional services started alongside the BGP daemon.")
	fmt.Println("  Enable via config block or CLI flag. Web UI requires ze init (blob storage).")
	fmt.Println()

	services := aihelp.Services()
	if len(services) == 0 {
		fmt.Println("  (no services found)")
		fmt.Println()
		return
	}

	// CLI flag mapping: service name -> flag syntax.
	cliFlags := map[string]string{
		"web": "--web <port>  --web-only  --insecure-web",
		"mcp": "--mcp <port>",
	}

	for _, svc := range services {
		desc := svc.Description
		if desc == "" {
			desc = svc.Name
		}
		var tb textbuf.Buffer
		tb.Str("  ").Str(svc.Name).Str(": ").Str(desc)
		fmt.Println(tb.Slice())

		if flag, ok := cliFlags[svc.Name]; ok {
			tb.Reset()
			tb.Str("    CLI flag:  ").Str(flag)
			fmt.Println(tb.Slice())
		}

		// Config syntax from leaves.
		tb.Reset()
		tb.Str("    Config:    environment { ").Str(svc.Name).Str(" {")
		for _, leaf := range svc.Leaves {
			if leaf.Default != "" {
				tb.Byte(' ').Str(leaf.Name).Byte(' ').Str(leaf.Default).Byte(';')
			}
		}
		tb.Str(" } }")
		fmt.Println(tb.Slice())

		// Leaf details.
		for _, leaf := range svc.Leaves {
			def := ""
			if leaf.Default != "" {
				var dtb textbuf.Buffer
				def = dtb.Str(" (default: ").Str(leaf.Default).Byte(')').String()
			}
			desc := leaf.Description
			if desc == "" {
				desc = leaf.Type
			}
			tb.Reset()
			tb.Str("    ").PadRight(leaf.Name, 20).Byte(' ').Str(desc).Str(def)
			fmt.Println(tb.Slice())
		}

		// Env vars.
		if len(svc.EnvVars) > 0 {
			tb.Reset()
			tb.Str("    Env vars:  ").Str(textbuf.Join(svc.EnvVars, ", "))
			fmt.Println(tb.Slice())
		}

		fmt.Println()
	}
}

func printRecipes() {
	fmt.Println("## Recipes")
	fmt.Println()
	fmt.Println("  Start daemon with MCP:")
	fmt.Println("    ze init && ze start --mcp 9718")
	fmt.Println()
	fmt.Println("  Start with config file:")
	fmt.Println("    ze config validate example.conf && ze --mcp 9718 example.conf")
	fmt.Println()
	fmt.Println("  Announce a route (CLI):")
	fmt.Println("    ze cli -c \"peer * update text origin igp next-hop 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24\"")
	fmt.Println()
	fmt.Println("  Announce a route (MCP):")
	fmt.Println("    {\"method\":\"tools/call\",\"params\":{\"name\":\"ze_announce\",\"arguments\":{\"family\":\"ipv4/unicast\",\"origin\":\"igp\",\"next-hop\":\"1.1.1.1\",\"prefixes\":[\"10.0.0.0/24\"]}}}")
	fmt.Println()
	fmt.Println("  Check peer state:")
	fmt.Println("    ze cli -c \"peer list\"")
	fmt.Println("    ze cli -c \"peer test-peer detail\"")
	fmt.Println()
	fmt.Println("  Show RIB:")
	fmt.Println("    ze cli -c \"show bgp rib received family ipv4/unicast\"")
	fmt.Println("    ze cli -c \"show bgp rib best\"")
	fmt.Println()
	fmt.Println("  Monitor live events:")
	fmt.Println("    ze cli -c \"bgp monitor\"")
	fmt.Println()
	fmt.Println("  Drain and teardown a peer:")
	fmt.Println("    ze cli -c \"request peer 10.0.0.1 pause\"")
	fmt.Println("    ze cli -c \"request peer * flush\"")
	fmt.Println("    ze cli -c \"request peer 10.0.0.1 teardown\"")
	fmt.Println()
	fmt.Println("  Test without a real peer:")
	fmt.Println("    ze-test peer --mode sink --port 1179 --asn 65001")
	fmt.Println()
}

func printCommonErrors() {
	fmt.Println("## Common Errors")
	fmt.Println()
	fmt.Println("  unknown family \"ipv4-unicast\"       Use slash separator: ipv4/unicast")
	fmt.Println("  peer not found \"10.0.0.1\"           Peer not configured; check: peer list")
	fmt.Println("  database already exists              Run: ze init --force (backs up old database)")
	fmt.Println("  connection refused (SSH)             Daemon not running; start with: ze start")
	fmt.Println("  no prefixes specified                REQUIRED field missing in ze_announce/ze_withdraw")
	fmt.Println("  unknown command \"...\"                Use: ze_commands (MCP) or ze cli -c \"help\"")
	fmt.Println("  web server disabled: requires blob  Run: ze init (creates database.zefs with TLS certs)")
	fmt.Println()
}

func printMinimalConfig() {
	fmt.Println("## Minimal Config")
	fmt.Println()
	fmt.Println("  bgp {")
	fmt.Println("      router-id 10.0.0.1")
	fmt.Println("      local {")
	fmt.Println("          as 65000")
	fmt.Println("      }")
	fmt.Println("      peer test-peer {")
	fmt.Println("          remote {")
	fmt.Println("              ip 10.0.0.2")
	fmt.Println("              as 65001")
	fmt.Println("          }")
	fmt.Println("          local {")
	fmt.Println("              ip 10.0.0.1")
	fmt.Println("          }")
	fmt.Println("          family {")
	fmt.Println("              ipv4/unicast")
	fmt.Println("          }")
	fmt.Println("      }")
	fmt.Println("  }")
	fmt.Println()
	fmt.Println("  Validate: ze config validate <file>")
	fmt.Println("  Start:    ze <file>  or  ze start (from database)")
	fmt.Println()
}

// printAIHelpJSON emits the machine-readable reference. The assembly lives in
// internal/component/aihelp so the MCP ze_reference tool returns identical data.
func printAIHelpJSON() {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(aihelp.Build()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}
}

// aiHelpRequested checks if the deprecated --ai flag was passed in the help args.
// The canonical form is the "ai" subcommand (ze help ai); --ai is a hidden alias.
func aiHelpRequested(args []string) bool {
	return slices.Contains(args, "--ai")
}

// hasSection reports whether an AI-help section was requested. It accepts both
// the canonical positional form ("api", from "ze help ai api") and the legacy
// flag form ("--api", from "ze help --ai --api").
func hasSection(args []string, name string) bool {
	for _, a := range args {
		if strings.TrimLeft(a, "-") == name {
			return true
		}
	}
	return false
}

func helpUsage() {
	// Derive help subcommands from the local command registry.
	var subEntries []helpfmt.HelpEntry
	for _, lc := range cmdregistry.ListLocal() {
		if !strings.HasPrefix(lc.Path, "help ") {
			continue
		}
		name := strings.TrimPrefix(lc.Path, "help ")
		subEntries = append(subEntries, helpfmt.HelpEntry{Name: name, Desc: lc.Meta.Description})
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
		Command:  "ze help",
		Summary:  "Show help and AI reference",
		Usage:    []string{"ze help ai [cli|api|mcp|dispatch|all] [--json]"},
		Sections: sections,
	}
	p.WriteErr()
}
