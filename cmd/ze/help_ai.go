// Design: docs/guide/mcp/overview.md -- AI help reference generator

//go:build ze_core

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/ze-software/ze/cmd/ze/internal/helpfmt"
	"github.com/ze-software/ze/internal/component/aihelp"
	cli "github.com/ze-software/ze/internal/component/cli/client"
	cmdregistry "github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
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
//
// Output routes through a helpfmt.RenderWriter: a write error (e.g. `ze help ai
// | head` closing the pipe) is captured and returned as a non-zero exit code
// instead of being silently swallowed.
func printAIHelp(args []string) int {
	return renderAIHelp(os.Stdout, args)
}

// renderAIHelp writes the reference to w and returns the exit code (non-zero on
// a write error). Split from printAIHelp so tests can drive a failing writer.
func renderAIHelp(w io.Writer, args []string) int {
	if slices.Contains(args, flagJSON) {
		return printAIHelpJSON(w)
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

	rw := helpfmt.NewRenderWriter(w)
	rw.Line("# Ze AI Reference")
	rw.Line("# Generated from code -- always matches this binary.")
	rw.Line("")

	if summaryOnly {
		printSummary(rw)
		return rw.ExitCode()
	}

	if showCLI {
		printCLICommands(rw)
	}
	if showAPI {
		printAPICommands(rw)
		printUpdateSyntax(rw)
		printFamilies(rw)
		printAIPlugins(rw)
		printPeerSelectors(rw)
		printFamilyAttributes(rw)
		printRIBPipeline(rw)
	}
	if showDispatch {
		printDispatchKeys(rw)
	}
	if showMCP {
		printMCPTools(rw)
	}

	// Recipes and errors are useful in any detailed view.
	if showCLI || showAPI || showMCP {
		printServices(rw)
		printRecipes(rw)
		printCommonErrors(rw)
	}
	if showCLI || showMCP {
		printMinimalConfig(rw)
	}
	return rw.ExitCode()
}

func printSummary(rw *helpfmt.RenderWriter) {
	rw.Line("## Sections (use 'ze help ai <section>' for details)")
	rw.Line("")
	rw.Line("  cli       CLI subcommands: ze bgp, ze config, ze show, ze signal, ...")
	rw.Line("  api       Daemon API: all RPC commands, update syntax, families, plugins")
	rw.Line("  mcp       MCP tools: ze_execute, ze_reference, ze_announce, ze_withdraw")
	rw.Line("  dispatch  Dispatch keys for daemon commands")
	rw.Line("  all       Everything")
	rw.Line("")

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

	var tb textbuf.Buffer
	rw.Line(tb.Reset().Str("  ").Int(int64(len(regs))).Str(" plugins, ").Int(int64(familyCount)).Str(" address families").String())
	rw.Line(tb.Reset().Str("  ").Int(int64(rpcCount)).Str(" YANG RPCs, ").Int(int64(builtinCount)).Str(" builtin RPCs").String())
	rw.Line("  MCP tools: ze_execute, ze_reference (handcrafted) + per-group tools auto-generated from the YANG command registry")
	rw.Line("")
	rw.Line("## Quick Start")
	rw.Line("")
	rw.Line("  Daemon:  ze start --mcp 9718 config.conf")
	rw.Line("  Or:      ze --mcp 9718 -")
	rw.Line("  CLI:     ze cli")
	rw.Line("  Show:    ze show <command>")
	rw.Line("  Help:    ze help ai all")
}

func printCLICommands(rw *helpfmt.RenderWriter) {
	rw.Line("## CLI Subcommands")
	rw.Line("")
	rw.Line("  ze [global-flags] <command> [options]")
	rw.Line("")
	rw.Line("  Modes: [offline] no daemon needed  [daemon] requires running daemon  [setup] one-time setup")
	rw.Line("")
	rw.Line("  Global flags:")
	rw.Line("    -d, --debug            Enable debug logging")
	rw.Line("    -f <file>              Use filesystem directly, bypass blob store")
	rw.Line("    --plugin <name>        Load plugin before starting (repeatable)")
	rw.Line("    --mcp <port>           Start MCP server on 127.0.0.1:<port>")
	rw.Line("    --web <port>           Start web server on 0.0.0.0:<port>")
	rw.Line("    --web-only             Web UI only, no daemon (config editing only)")
	rw.Line("    --insecure-web         Disable web auth (forces 127.0.0.1)")
	rw.Line("    --pprof <addr:port>    Start pprof HTTP server")
	rw.Line("    -V, --version          Show version")
	rw.Line("")

	// CLI tree. The subcommand list is static text that matches the dispatcher
	// in cmd/ze/main.go. It changes rarely and is verified by functional tests.
	cmds := aihelp.CLISubcommands()
	for _, c := range cmds {
		var tb textbuf.Buffer
		tb.Str("  ze ").PadRight(c.Name, 14).Str(" [").PadRight(c.Mode, 7).Str("] ").Str(c.Description)
		rw.Line(tb.Slice())
		if c.Subs != "" {
			tb.Reset()
			tb.Str("    ").Str(c.Subs)
			rw.Line(tb.Slice())
		}
	}
	rw.Line("")
}

func printAPICommands(rw *helpfmt.RenderWriter) {
	rw.Line("## Daemon API Commands (YANG RPCs)")
	rw.Line("")
	rw.Line("Format: wire-method (dispatch-key) description")
	rw.Line("")

	wireToPath := cli.WireToPath()
	schemaReg := aihelp.SchemaRegistry()

	rpcs := schemaReg.ListRPCs("")
	sort.Slice(rpcs, func(i, j int) bool {
		return rpcs[i].WireMethod < rpcs[j].WireMethod
	})

	var tb textbuf.Buffer
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
			rw.Line(tb.Reset().Str("  ").PadRight(rpc.WireMethod, 44).Str(" (").PadRight(dispatch, 30).Str(") ").Str(desc).Str(ro).String())
		} else {
			rw.Line(tb.Reset().Str("  ").PadRight(rpc.WireMethod, 44).Byte(' ').PadRight("", 32).Byte(' ').Str(desc).Str(ro).String())
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
			rw.Line(tb.Reset().Str("    ").PadRight(leaf.Name, 24).Byte(' ').Str(leafDesc).Str(req).String())
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

	for _, bi := range builtins {
		if shown[bi.WireMethod] {
			continue
		}
		help := bi.WireMethod
		ro := ""
		dispatch := wireToPath[bi.WireMethod]
		if dispatch != "" {
			if pluginserver.IsReadOnlyPath(dispatch) {
				ro = " [read-only]"
			}
			rw.Line(tb.Reset().Str("  ").PadRight(bi.WireMethod, 44).Str(" (").PadRight(dispatch, 30).Str(") ").Str(help).Str(ro).String())
		} else {
			rw.Line(tb.Reset().Str("  ").PadRight(bi.WireMethod, 44).Byte(' ').PadRight("", 32).Byte(' ').Str(help).Str(ro).String())
		}
	}
	rw.Line("")
}

func printDispatchKeys(rw *helpfmt.RenderWriter) {
	rw.Line("## Dispatch Keys (what you type)")
	rw.Line("")
	rw.Line("These are the strings accepted by the daemon dispatcher.")
	rw.Line("Use with: ze cli -c \"<dispatch-key>\"")
	rw.Line("")

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

	var tb textbuf.Buffer
	rw.Line(tb.Reset().Str("  ").PadRight("DISPATCH KEY", 40).Byte(' ').Str("WIRE METHOD").String())
	for _, e := range entries {
		rw.Line(tb.Reset().Str("  ").PadRight(e.dispatch, 40).Byte(' ').Str(e.wireMethod).String())
	}
	rw.Line("")
}

func printUpdateSyntax(rw *helpfmt.RenderWriter) {
	rw.Line("## Update Text Syntax")
	rw.Line("")
	rw.Line("  peer <selector> update text [attributes] nlri <family> <action> <prefix>...")
	rw.Line("")
	rw.Line("  Selectors:  * (all), <ip-address>, <peer-name>")
	rw.Line("  Actions:    add <prefix>, del <prefix>, eor")
	rw.Line("")
	rw.Line("  Attributes (common):")
	rw.Line("    origin <igp|egp|incomplete>")
	rw.Line("    next-hop <ip-address>             (alias: nhop)")
	rw.Line("    local-preference <N>")
	rw.Line("    med <N>")
	rw.Line("    as-path [<asn> ...]")
	rw.Line("    community <value>                 (e.g. 65000:100, no-export)")
	rw.Line("    large-community <value>           (e.g. 65000:100:200)")
	rw.Line("    extended-community <value>")
	rw.Line("")
	rw.Line("  Attributes (family-specific):")
	rw.Line("    path-id <N>                       (ADD-PATH path identifier)")
	rw.Line("    rd <value>                        (Route Distinguisher for VPN)")
	rw.Line("    label <N>                         (MPLS label for labeled/VPN)")
	rw.Line("")
	rw.Line("  Example:")
	rw.Line("    peer * update text origin igp next-hop 1.1.1.1 local-preference 100 nlri ipv4/unicast add 10.0.0.0/24")
	rw.Line("")
}

func printFamilies(rw *helpfmt.RenderWriter) {
	rw.Line("## Address Families")
	rw.Line("")

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

	var tb textbuf.Buffer
	for _, fam := range sorted {
		plugins := families[fam]
		rw.Line(tb.Reset().Str("  ").PadRight(fam, 24).Str(" (").Str(textbuf.Join(plugins, ", ")).Byte(')').String())
	}
	rw.Line("")
}

func printAIPlugins(rw *helpfmt.RenderWriter) {
	rw.Line("## Plugins")
	rw.Line("")

	regs := registry.All()
	sort.Slice(regs, func(i, j int) bool {
		return regs[i].Name < regs[j].Name
	})

	var tb textbuf.Buffer
	for _, reg := range regs {
		rw.Line(tb.Reset().Str("  ").PadRight(reg.Name, 24).Byte(' ').Str(reg.Description).String())
		if len(reg.RFCs) > 0 {
			rw.Line(tb.Reset().Str("    RFCs: ").Str(textbuf.Join(reg.RFCs, ", ")).String())
		}
		if len(reg.Families) > 0 {
			rw.Line(tb.Reset().Str("    Families: ").Str(textbuf.Join(reg.Families, ", ")).String())
		}
	}
	rw.Line("")
}

func printMCPTools(rw *helpfmt.RenderWriter) {
	rw.Line("## MCP Tools (via --mcp <port>)")
	rw.Line("")
	rw.Line("  Start: ze start --mcp <port> <config>  or  ze --mcp <port> -")
	rw.Line("  Connect: POST http://127.0.0.1:<port>/mcp with a JSON-RPC body")
	rw.Line("")
	rw.Line("  Protocol revision 2026-07-28 (the only one accepted). Stateless: every")
	rw.Line("  message is its own POST, with no initialize handshake and no session.")
	rw.Line("")
	rw.Line("  Required on every POST:")
	rw.Line("    MCP-Protocol-Version: 2026-07-28   header, must match the _meta version")
	rw.Line("    Mcp-Method: <method>               header, must match the body's method")
	rw.Line("    Mcp-Name: <params.name>            header, tools/call + prompts/get")
	rw.Line("    Mcp-Name: <params.uri>             header, resources/read")
	rw.Line(`    params._meta["io.modelcontextprotocol/protocolVersion"]    (required)`)
	rw.Line(`    params._meta["io.modelcontextprotocol/clientCapabilities"] (required, {} is valid)`)
	rw.Line("")
	rw.Line("  Tools are auto-generated from the YANG command registry at tools/list time.")
	rw.Line("  Each command group becomes a tool with an action enum. New YANG commands")
	rw.Line("  appear as MCP tools automatically.")
	rw.Line("")
	rw.Line("  Handcrafted tools:")
	rw.Line("  ze_execute          Run any Ze command (escape hatch)")
	rw.Line("    command           Full command string (REQUIRED)")
	rw.Line("")
	rw.Line("  ze_reference        Full AI reference for this daemon (same as 'ze help ai --json')")
	rw.Line("")
	rw.Line("  Call server/discover first to learn the supported versions and capabilities,")
	rw.Line("  then tools/list against a live daemon for the full tool inventory.")
	rw.Line("")
	rw.Line("  JSON-RPC Example (headers shown above are required and not repeated here):")
	rw.Line(`    {"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"ze_execute",`)
	rw.Line(`     "arguments":{"command":"summary"},`)
	rw.Line(`     "_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28",`)
	rw.Line(`              "io.modelcontextprotocol/clientCapabilities":{}}}}`)
	rw.Line("")
	rw.Line("  Every successful result carries resultType:\"complete\" and")
	rw.Line(`  _meta["io.modelcontextprotocol/serverInfo"].`)
	rw.Line("")
}

func printPeerSelectors(rw *helpfmt.RenderWriter) {
	rw.Line("## Peer Selectors")
	rw.Line("")
	rw.Line("  Most commands accept a peer selector to target specific peers.")
	rw.Line("  The reactor resolves selectors in priority order:")
	rw.Line("")
	rw.Line("  *                All peers (default when omitted)")
	rw.Line("  192.168.1.1      Exact IP address")
	rw.Line("  my-peer          Peer name (from config, takes priority over IP)")
	rw.Line("  as65001          All peers with remote AS 65001 (case-insensitive)")
	rw.Line("  10.0.0.*         IP glob pattern (per-octet wildcard)")
	rw.Line("  !192.168.1.1     Exclusion: all peers except this one")
	rw.Line("  10.0.0.1,10.0.0.2  Comma-separated list (RIB commands only)")
	rw.Line("")
}

func printFamilyAttributes(rw *helpfmt.RenderWriter) {
	rw.Line("## Family-Specific Attributes")
	rw.Line("")
	rw.Line("  Some update text attributes only apply to specific address families:")
	rw.Line("")
	rw.Line("  path-id <N>        ADD-PATH peers only (any family, requires ADD-PATH capability)")
	rw.Line("  rd <value>         VPN families: ipv4/mpls-vpn, ipv6/mpls-vpn, l2vpn/evpn, l2vpn/vpls")
	rw.Line("  label <N>          Labeled/VPN: ipv4/mpls-label, ipv6/mpls-label, */vpn")
	rw.Line("")
}

func printRIBPipeline(rw *helpfmt.RenderWriter) {
	rw.Line("## RIB Show Pipeline")
	rw.Line("")
	rw.Line("  show bgp rib [scope] [filters...] [terminal]")
	rw.Line("  show bgp rib best [filters...] [terminal]")
	rw.Line("")
	rw.Line("  Scopes (positional, first argument):")
	rw.Line("    received         Adj-RIB-In only")
	rw.Line("    sent             Adj-RIB-Out only")
	rw.Line("    sent-received    Both (default)")
	rw.Line("")
	rw.Line("  Filters (named, chainable):")
	rw.Line("    family <afi/safi>     Address family (e.g. ipv4/unicast)")
	rw.Line("    prefix <pattern>      Prefix string match (e.g. 192.168)")
	rw.Line("    path <pattern>        AS-path: 64501 (anywhere), ^64501 (anchored), 64501,64502 (contiguous)")
	rw.Line("    community <value>     Exact community match (e.g. 65000:100)")
	rw.Line("    match <text>          Case-insensitive substring across all fields")
	rw.Line("")
	rw.Line("  Terminals (last argument):")
	rw.Line("    count                 Return {\"count\": N} instead of routes")
	rw.Line("    json                  Full route details (default)")
	rw.Line("")
	rw.Line("  Examples:")
	rw.Line("    show bgp rib received family ipv4/unicast")
	rw.Line("    show bgp rib sent prefix 10.0 count")
	rw.Line("    show bgp rib received community 65000:100 path ^64501")
	rw.Line("    show bgp rib best family ipv4/unicast json")
	rw.Line("")
}

// printServices generates the Services section from YANG conf modules.
// It walks all registered YANG modules looking for environment containers,
// extracts leaves with their types and defaults, and matches env vars.
func printServices(rw *helpfmt.RenderWriter) {
	rw.Line("## Services (from YANG environment containers)")
	rw.Line("")
	rw.Line("  Optional services started alongside the BGP daemon.")
	rw.Line("  Enable via config block or CLI flag. Web UI requires ze init (blob storage).")
	rw.Line("")

	services := aihelp.Services()
	if len(services) == 0 {
		rw.Line("  (no services found)")
		rw.Line("")
		return
	}

	// CLI flag mapping: service name -> flag syntax.
	cliFlags := map[string]string{
		"web": "--web <port>  --web-only  --insecure-web",
		"mcp": helpMCPPortOption,
	}

	for _, svc := range services {
		desc := svc.Description
		if desc == "" {
			desc = svc.Name
		}
		var tb textbuf.Buffer
		tb.Str("  ").Str(svc.Name).Str(": ").Str(desc)
		rw.Line(tb.Slice())

		if flag, ok := cliFlags[svc.Name]; ok {
			tb.Reset()
			tb.Str("    CLI flag:  ").Str(flag)
			rw.Line(tb.Slice())
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
		rw.Line(tb.Slice())

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
			rw.Line(tb.Slice())
		}

		// Env vars.
		if len(svc.EnvVars) > 0 {
			tb.Reset()
			tb.Str("    Env vars:  ").Str(textbuf.Join(svc.EnvVars, ", "))
			rw.Line(tb.Slice())
		}

		rw.Line("")
	}
}

func printRecipes(rw *helpfmt.RenderWriter) {
	rw.Line("## Recipes")
	rw.Line("")
	rw.Line("  Start daemon with MCP:")
	rw.Line("    ze init && ze start --mcp 9718")
	rw.Line("")
	rw.Line("  Start with config file:")
	rw.Line("    ze config validate example.conf && ze --mcp 9718 example.conf")
	rw.Line("")
	rw.Line("  Announce a route (CLI):")
	rw.Line("    ze cli -c \"peer * update text origin igp next-hop 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24\"")
	rw.Line("")
	rw.Line("  Announce a route (MCP; POST to /mcp with the headers from the MCP Tools section):")
	rw.Line("    {\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"ze_announce\",")
	rw.Line("     \"arguments\":{\"family\":\"ipv4/unicast\",\"origin\":\"igp\",\"next-hop\":\"1.1.1.1\",\"prefixes\":[\"10.0.0.0/24\"]},")
	rw.Line("     \"_meta\":{\"io.modelcontextprotocol/protocolVersion\":\"2026-07-28\",")
	rw.Line("              \"io.modelcontextprotocol/clientCapabilities\":{}}}}")
	rw.Line("")
	rw.Line("  Check peer state:")
	rw.Line("    ze cli -c \"show bgp peer list\"")
	rw.Line("    ze cli -c \"show bgp peer test-peer detail\"")
	rw.Line("")
	rw.Line("  Show RIB:")
	rw.Line("    ze cli -c \"show bgp rib received family ipv4/unicast\"")
	rw.Line("    ze cli -c \"show bgp rib best\"")
	rw.Line("")
	rw.Line("  Monitor live events:")
	rw.Line("    ze cli -c \"bgp monitor\"")
	rw.Line("")
	rw.Line("  Drain and teardown a peer:")
	rw.Line("    ze cli -c \"request peer 10.0.0.1 pause\"")
	rw.Line("    ze cli -c \"request peer * flush\"")
	rw.Line("    ze cli -c \"request peer 10.0.0.1 teardown\"")
	rw.Line("")
	rw.Line("  Test without a real peer:")
	rw.Line("    ze-test peer --mode sink --port 1179 --asn 65001")
	rw.Line("")
}

func printCommonErrors(rw *helpfmt.RenderWriter) {
	rw.Line("## Common Errors")
	rw.Line("")
	rw.Line("  unknown family \"ipv4-unicast\"       Use slash separator: ipv4/unicast")
	rw.Line("  peer not found \"10.0.0.1\"           Peer not configured; check: peer list")
	rw.Line("  database already exists              Run: ze init --force (backs up old database)")
	rw.Line("  connection refused (SSH)             Daemon not running; start with: ze start")
	rw.Line("  no prefixes specified                REQUIRED field missing in ze_announce/ze_withdraw")
	rw.Line("  unknown command \"...\"                Use: ze_reference (MCP) or ze cli -c \"help\"")
	rw.Line("  web server disabled: requires blob  Run: ze init (creates database.zefs with TLS certs)")
	rw.Line("")
}

func printMinimalConfig(rw *helpfmt.RenderWriter) {
	rw.Line("## Minimal Config")
	rw.Line("")
	rw.Line("  bgp {")
	rw.Line("      router-id 10.0.0.1")
	rw.Line("      local {")
	rw.Line("          as 65000")
	rw.Line("      }")
	rw.Line("      peer test-peer {")
	rw.Line("          remote {")
	rw.Line("              ip 10.0.0.2")
	rw.Line("              as 65001")
	rw.Line("          }")
	rw.Line("          local {")
	rw.Line("              ip 10.0.0.1")
	rw.Line("          }")
	rw.Line("          family {")
	rw.Line("              ipv4/unicast")
	rw.Line("          }")
	rw.Line("      }")
	rw.Line("  }")
	rw.Line("")
	rw.Line("  Validate: ze config validate <file>")
	rw.Line("  Start:    ze <file>  or  ze start (from database)")
	rw.Line("")
}

// printAIHelpJSON emits the machine-readable reference. The assembly lives in
// internal/component/aihelp so the MCP ze_reference tool returns identical data.
// Writes route through a RenderWriter so a broken pipe yields a non-zero exit.
func printAIHelpJSON(w io.Writer) int {
	rw := helpfmt.NewRenderWriter(w)
	enc := json.NewEncoder(rw)
	enc.SetIndent("", "  ")
	if err := enc.Encode(aihelp.Build()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err) //nolint:errcheck // one-shot error to stderr
		return 1
	}
	return rw.ExitCode()
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
