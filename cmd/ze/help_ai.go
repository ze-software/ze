// Design: docs/guide/mcp/overview.md -- AI help reference generator

package main

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/cli"
	ribschema "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/schema"
	bgpschema "codeberg.org/thomas-mangin/ze/internal/component/bgp/schema"
	"codeberg.org/thomas-mangin/ze/internal/component/command"
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	ipcschema "codeberg.org/thomas-mangin/ze/internal/core/ipc/schema"
)

// printAIHelp outputs a machine-friendly reference generated from code.
// All data comes from the plugin registry, YANG schemas, and RPC registrations
// so it is always in sync with the running binary.
//
// Sections are selected via flags:
//
//	--ai           Summary / table of contents
//	--ai --cli     CLI subcommands (ze bgp, ze config, ...)
//	--ai --api     Daemon API commands (YANG RPCs)
//	--ai --mcp     MCP tools with parameters
//	--ai --all     Everything
func printAIHelp(args []string) {
	showCLI := slices.Contains(args, "--cli")
	showAPI := slices.Contains(args, "--api")
	showMCP := slices.Contains(args, "--mcp")
	showDispatch := slices.Contains(args, "--dispatch")
	showAll := slices.Contains(args, "--all")

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
		printRecipes()
		printCommonErrors()
	}
	if showCLI || showMCP {
		printMinimalConfig()
	}
}

func printSummary() {
	fmt.Println("## Sections (use --ai --<section> for details)")
	fmt.Println()
	fmt.Println("  --cli    CLI subcommands: ze bgp, ze config, ze show, ze signal, ...")
	fmt.Println("  --api    Daemon API: all RPC commands, update syntax, families, plugins")
	fmt.Println("  --mcp    MCP tools: ze_announce, ze_withdraw, ze_peers, ze_peer_control")
	fmt.Println("  --all    Everything")
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

	schemaReg := buildAISchemaRegistry()
	rpcCount := len(schemaReg.ListRPCs(""))
	builtinCount := len(pluginserver.AllBuiltinRPCs())

	fmt.Printf("  %d plugins, %d address families\n", len(regs), familyCount)
	fmt.Printf("  %d YANG RPCs, %d builtin RPCs\n", rpcCount, builtinCount)
	fmt.Printf("  6 MCP tools (ze_announce, ze_withdraw, ze_peers, ze_peer_control, ze_execute, ze_commands)\n")
	fmt.Println()
	fmt.Println("## Quick Start")
	fmt.Println()
	fmt.Println("  Daemon:  ze start --mcp 9718")
	fmt.Println("  Or:      ze --mcp 9718 config.conf")
	fmt.Println("  CLI:     ze cli")
	fmt.Println("  Show:    ze show <command>")
	fmt.Println("  Help:    ze help --ai --all")
}

// cliCmd describes a CLI subcommand for the help output.
type cliCmd struct {
	cmd  string
	mode string // "offline", "daemon", or "setup"
	desc string
	subs string
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
	fmt.Println("    --insecure-web         Disable web auth (forces 127.0.0.1)")
	fmt.Println("    --pprof <addr:port>    Start pprof HTTP server")
	fmt.Println("    -V, --version          Show version")
	fmt.Println()

	// CLI tree. The subcommand list is static text that matches the dispatcher
	// in cmd/ze/main.go. It changes rarely and is verified by functional tests.
	cmds := cliSubcommands()
	for _, c := range cmds {
		fmt.Printf("  ze %-14s [%-7s] %s\n", c.cmd, c.mode, c.desc)
		if c.subs != "" {
			fmt.Printf("    %s\n", c.subs)
		}
	}
	fmt.Println()
}

// cliSubcommands returns the CLI subcommand tree.
// Verb commands (show, set, del, etc.) are generated from the YANG tree.
// Root commands (completion, exabgp, etc.) are listed statically.
func cliSubcommands() []cliCmd {
	var cmds []cliCmd

	// Dynamic: verb commands from YANG tree.
	yangTree := cli.YANGCommandTree()
	if yangTree != nil {
		for _, name := range sortedChildren(yangTree) {
			child := yangTree.Children[name]
			desc := child.Description
			if desc == "" {
				desc = name + " commands"
			}
			mode := "daemon"
			if command.IsReadOnlyVerb(name) {
				mode = "read-only"
			}
			cmds = append(cmds, cliCmd{name, mode, desc, "ze " + name + " help"})
		}
	}

	// Static: root commands that stay outside the verb tree.
	cmds = append(cmds,
		cliCmd{"bgp", "offline", "BGP protocol tools", "decode <hex>, encode <route>, plugin"},
		cliCmd{"cli", "daemon", "Interactive CLI for running daemons", "-c <cmd> for single command"},
		cliCmd{"completion", "offline", "Shell completion scripts", "bash, zsh, fish, nushell"},
		cliCmd{"config", "offline", "Configuration management", "edit, set, migrate, rollback, archive, import, rename"},
		cliCmd{"data", "offline", "ZeFS blob store management", "import, rm, ls, cat"},
		cliCmd{"exabgp", "offline", "ExaBGP bridge tools", "plugin, migrate"},
		cliCmd{"init", "setup", "Bootstrap database with SSH credentials", "--managed for fleet mode, --force to replace"},
		cliCmd{"interface", "offline", "Manage OS network interfaces", "create, delete, unit, addr, migrate"},
		cliCmd{"plugin", "offline", "Plugin system", "<plugin-name> for plugin CLI, test for debugging"},
		cliCmd{"signal", "daemon", "Send signals to daemon via SSH", "reload, stop, restart, quit"},
		cliCmd{"start", "setup", "Start daemon from database config", "--web <port>, --insecure-web, --mcp <port>"},
		cliCmd{"status", "daemon", "Check if daemon is running", "exit 0 = running, 1 = not"},
		cliCmd{"help", "offline", "Show help", "--ai [--cli|--api|--mcp|--dispatch|--all]"},
	)

	return cmds
}

// sortedChildren returns sorted child names of a command node.
func sortedChildren(node *command.Node) []string {
	names := make([]string, 0, len(node.Children))
	for name := range node.Children {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func printAPICommands() {
	fmt.Println("## Daemon API Commands (YANG RPCs)")
	fmt.Println()
	fmt.Println("Format: wire-method (dispatch-key) description")
	fmt.Println()

	wireToPath := cli.WireToPath()
	schemaReg := buildAISchemaRegistry()

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
	for _, fam := range []string{"ipv4/unicast", "ipv6/unicast", "ipv4/multicast", "ipv6/multicast"} {
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
		fmt.Printf("  %-24s (%s)\n", fam, strings.Join(plugins, ", "))
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
			fmt.Printf("    RFCs: %s\n", strings.Join(reg.RFCs, ", "))
		}
		if len(reg.Families) > 0 {
			fmt.Printf("    Families: %s\n", strings.Join(reg.Families, ", "))
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
	fmt.Println("  ze_announce         Announce BGP routes")
	fmt.Println("    peer              Peer selector (address, name, or *). Default: *")
	fmt.Println("    origin            igp | egp | incomplete")
	fmt.Println("    next-hop          Next-hop IP address")
	fmt.Println("    local-preference  LOCAL_PREF integer")
	fmt.Println("    as-path           List of ASNs [65000, 65001]")
	fmt.Println("    community         List of standard communities [\"65000:100\"]")
	fmt.Println("    family            Address family (REQUIRED, e.g. ipv4/unicast)")
	fmt.Println("    prefixes          List of prefixes (REQUIRED, e.g. [\"10.0.0.0/24\"])")
	fmt.Println()
	fmt.Println("  ze_withdraw         Withdraw BGP routes")
	fmt.Println("    peer              Peer selector. Default: *")
	fmt.Println("    family            Address family (REQUIRED)")
	fmt.Println("    prefixes          List of prefixes (REQUIRED)")
	fmt.Println()
	fmt.Println("  ze_peers            List BGP peers with state")
	fmt.Println("    peer              Optional peer selector for detail view")
	fmt.Println()
	fmt.Println("  ze_peer_control     Peer lifecycle management")
	fmt.Println("    peer              Peer selector (REQUIRED)")
	fmt.Println("    action            teardown | pause | resume | flush (REQUIRED)")
	fmt.Println()
	fmt.Println("  ze_execute          Run any Ze command (escape hatch)")
	fmt.Println("    command           Full command string (REQUIRED)")
	fmt.Println()
	fmt.Println("  ze_commands         List all available daemon commands")
	fmt.Println()
	fmt.Println("  JSON-RPC Example:")
	fmt.Println(`    {"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"ze_announce","arguments":{"family":"ipv4/unicast","origin":"igp","next-hop":"1.1.1.1","prefixes":["10.0.0.0/24"]}}}`)
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
	fmt.Println("  rd <value>         VPN families: ipv4/vpn, ipv6/vpn, l2vpn/evpn, l2vpn/vpls")
	fmt.Println("  label <N>          Labeled/VPN: ipv4/mpls-label, ipv6/mpls-label, */vpn")
	fmt.Println()
}

func printRIBPipeline() {
	fmt.Println("## RIB Show Pipeline")
	fmt.Println()
	fmt.Println("  rib show [scope] [filters...] [terminal]")
	fmt.Println("  rib best [filters...] [terminal]")
	fmt.Println()
	fmt.Println("  Scopes (positional, first argument):")
	fmt.Println("    received         Adj-RIB-In only")
	fmt.Println("    sent             Adj-RIB-Out only")
	fmt.Println("    sent-received    Both (default)")
	fmt.Println()
	fmt.Println("  Filters (named, chainable):")
	fmt.Println("    family <afi/safi>     Address family (e.g. ipv4/unicast)")
	fmt.Println("    cidr <prefix>         Prefix string match (e.g. 192.168)")
	fmt.Println("    path <pattern>        AS-path: 64501 (anywhere), ^64501 (anchored), 64501,64502 (contiguous)")
	fmt.Println("    community <value>     Exact community match (e.g. 65000:100)")
	fmt.Println("    match <text>          Case-insensitive substring across all fields")
	fmt.Println()
	fmt.Println("  Terminals (last argument):")
	fmt.Println("    count                 Return {\"count\": N} instead of routes")
	fmt.Println("    json                  Full route details (default)")
	fmt.Println()
	fmt.Println("  Examples:")
	fmt.Println("    rib show received family ipv4/unicast")
	fmt.Println("    rib show sent cidr 10.0 count")
	fmt.Println("    rib show received community 65000:100 path ^64501")
	fmt.Println("    rib best family ipv4/unicast json")
	fmt.Println()
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
	fmt.Println("    ze cli -c \"rib show received family ipv4/unicast\"")
	fmt.Println("    ze cli -c \"rib best\"")
	fmt.Println()
	fmt.Println("  Monitor live events:")
	fmt.Println("    ze cli -c \"bgp monitor\"")
	fmt.Println()
	fmt.Println("  Drain and teardown a peer:")
	fmt.Println("    ze cli -c \"peer 10.0.0.1 pause\"")
	fmt.Println("    ze cli -c \"peer * flush\"")
	fmt.Println("    ze cli -c \"peer 10.0.0.1 teardown\"")
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

// buildAISchemaRegistry builds a schema registry with YANG RPC metadata.
func buildAISchemaRegistry() *pluginserver.SchemaRegistry {
	schemaReg := pluginserver.NewSchemaRegistry()

	loader := yang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: load embedded YANG: %v\n", err)
		return schemaReg
	}
	if err := loader.LoadRegistered(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: load registered YANG: %v\n", err)
	}
	if err := loader.Resolve(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: resolve YANG: %v\n", err)
	}

	apiModules := []struct {
		name    string
		content string
	}{
		{"ze-bgp-api", bgpschema.ZeBGPAPIYANG},
		{"ze-system-api", ipcschema.ZeSystemAPIYANG},
		{"ze-plugin-api", ipcschema.ZePluginAPIYANG},
		{"ze-rib-api", ribschema.ZeRibAPIYANG},
	}
	for _, mod := range apiModules {
		if mod.content == "" {
			continue
		}
		rpcs := yang.ExtractRPCs(loader, mod.name)
		_ = schemaReg.RegisterRPCs(mod.name, rpcs)
	}

	return schemaReg
}

// findBuiltinRPC finds a builtin RPC by wire method.
func findBuiltinRPC(wireMethod string) *pluginserver.RPCRegistration {
	for _, b := range pluginserver.AllBuiltinRPCs() {
		if b.WireMethod == wireMethod {
			return &b
		}
	}
	return nil
}

// aiHelpRequested checks if --ai was passed in the help args.
func aiHelpRequested(args []string) bool {
	return slices.Contains(args, "--ai")
}

func helpUsage() {
	fmt.Fprintf(os.Stderr, `Usage: ze help [options]

Options:
  --ai           Summary with counts and quick start
  --ai --cli     CLI subcommands (ze bgp, ze config, ...)
  --ai --api     Daemon API commands with parameters (YANG RPCs)
  --ai --mcp     MCP tools with parameters and examples
  --ai --all     Everything combined
`)
}
