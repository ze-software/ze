// Design: docs/architecture/ospf/ospf-13-cli-diag-interop.md -- `show ospf ...` under the
// top-level show grammar.
// Related: register.go -- runOSPFEngine registers the matching CommandDecl set and the
// OnExecuteCommand render dispatch.
// Related: yang/ze-ospf-cmd.yang -- the owner command tree binding these wire methods.
//
// The OSPF introspection data lives in the engine plugin process, reachable via the
// commands the component registers ("show ospf neighbor", ...). These builtin RPCs
// are plugin proxies (the LDP/IS-IS model): each declares the plugin command it fronts
// via PluginCommand so the OSPF engine can register the same command name (instead of a
// builtin conflict), and the handler forwards straight to the plugin via
// ForwardToPlugin -- it must NOT re-Dispatch the command string (that would re-match
// this builtin and recurse). The ze-ospf:show-* methods are builtin RPCs under the ospf prefix,
// registered here in Go; ze-ospf-cmd.yang ships the command-tree nodes that bind them.

package ospf

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Plugin command names -- shared by RPCRegistration.PluginCommand and forwardToOSPF so
// the two cannot diverge. They match the CommandDecl names runOSPFEngine registers.
const (
	cmdShowProcess             = "show ospf"
	cmdShowIPv6                = "show ospf ipv6"
	cmdShowInstance            = "show ospf instance"
	cmdShowNeighbor            = "show ospf neighbor"
	cmdShowInterface           = "show ospf interface"
	cmdShowIPv6Interface       = "show ospf ipv6 interface"
	cmdShowDatabase            = "show ospf database"
	cmdShowDatabaseRI          = "show ospf database router-information"
	cmdShowTEDatabase          = "show ospf te-database"
	cmdShowRoute               = "show ospf route"
	cmdShowRouteFastReroute    = "show ospf route fast-reroute"
	cmdShowVirtualLinks        = "show ospf virtual-links"
	cmdShowBorderRouters       = "show ospf border-routers"
	cmdShowSPF                 = "show ospf spf"
	cmdShowLDPSync             = "show ospf ldp-sync"
	cmdShowGracefulRestart     = "show ospf graceful-restart"
	cmdShowIPv6GracefulRestart = "show ospf ipv6 graceful-restart"
	cmdShowSegmentRouting      = "show ospf segment-routing"
	cmdShowIPv6SegmentRouting  = "show ospf ipv6 segment-routing"
	cmdClearProcess            = "clear ospf process all"
	cmdClearNeighbor           = "clear ospf neighbor all"
	cmdClearCounters           = "clear ospf counters"
	cmdGRPrepare               = "request ospf graceful-restart"

	cmdShowDatabaseRouter       = "show ospf database router"
	cmdShowDatabaseNetwork      = "show ospf database network"
	cmdShowDatabaseSummary      = "show ospf database summary"
	cmdShowDatabaseASBRSummary  = "show ospf database asbr-summary"
	cmdShowDatabaseExternal     = "show ospf database external"
	cmdShowDatabaseNSSAExternal = "show ospf database nssa-external"
	cmdShowDatabaseOpaqueLink   = "show ospf database opaque-link"
	cmdShowDatabaseOpaqueArea   = "show ospf database opaque-area"
	cmdShowDatabaseOpaqueAS     = "show ospf database opaque-as"

	cmdShowDatabaseOpaqueAreaDetail = "show ospf database opaque-area detail"
	cmdShowDatabaseOpaqueASDetail   = "show ospf database opaque-as detail"
	cmdShowDatabaseOpaqueLinkDetail = "show ospf database opaque-link detail"
	cmdShowSPFDetail                = "show ospf spf detail"
	cmdShowNeighborDetail           = "show ospf neighbor detail"
	cmdShowInterfaceDetail          = "show ospf interface detail"

	cmdShowIPv6Database               = "show ospf ipv6 database"
	cmdShowIPv6DatabaseDetail         = "show ospf ipv6 database detail"
	cmdShowIPv6DatabaseRouterDetail   = "show ospf ipv6 database router detail"
	cmdShowIPv6DatabaseScopeLink      = "show ospf ipv6 database scope link"
	cmdShowIPv6DatabaseScopeArea      = "show ospf ipv6 database scope area"
	cmdShowIPv6DatabaseScopeAS        = "show ospf ipv6 database scope as"
	cmdShowIPv6DatabaseRI             = "show ospf ipv6 database router-information"
	cmdShowIPv6DatabaseExtended       = "show ospf ipv6 database extended"
	cmdShowIPv6DatabaseSegmentRouting = "show ospf ipv6 database segment-routing"
	cmdShowIPv6Instance               = "show ospf ipv6 instance"
	cmdShowIPv6Neighbor               = "show ospf ipv6 neighbor"
	cmdShowIPv6NeighborDetail         = "show ospf ipv6 neighbor detail"
	cmdShowIPv6InterfaceDetail        = "show ospf ipv6 interface detail"
	cmdShowIPv6SPF                    = "show ospf ipv6 spf"
	cmdShowIPv6SPFDetail              = "show ospf ipv6 spf detail"
	cmdShowIPv6Route                  = "show ospf ipv6 route"

	cmdDebugInjectOpaque  = "debug ip ospf inject opaque"
	cmdDebugInjectLSA     = "debug ipv6 ospf inject lsa"
	cmdDebugInjectEnable  = "debug ospf inject enable"
	cmdDebugInjectDisable = "debug ospf inject disable"
)

// keywordAddressFamily is the optional selector every OSPFv3 view takes
// (yang/ze-ospf-cmd.yang, grouping ospfv3-address-family). afViewForwarder
// hands it to the plugin as this keyword followed by one of the four afName*
// spellings, and OnExecuteCommand reads it in that one shape.
const keywordAddressFamily = "address-family"

// The names OSPF uses for one concept across its metric labels, its YANG leaves
// and its CLI keywords. One spelling each, so a metric and the leaf it reports
// on cannot drift apart.
const (
	labelArea        = "area"
	labelDirection   = "direction"
	labelFamily      = "family"
	labelInterface   = "interface"
	labelKind        = "kind"
	labelOpaqueType  = "opaque_type"
	labelReason      = "reason"
	labelScope       = "scope"
	labelState       = "state"
	labelTransitArea = "transit_area"
)

// dependencyConfigTree is the plugin every OSPF doctor check reads its input from.
const dependencyConfigTree = "config-tree"

// exampleDoctorJSON is the command every OSPF diagnostic code offers first.
const exampleDoctorJSON = "ze doctor --json"

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-ospf", Handler: forwardShowOSPFProcess, PluginCommand: cmdShowProcess},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-ipv6", Handler: forwardShowOSPFIPv6, PluginCommand: cmdShowIPv6},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-instance", Handler: forwardShowOSPFInstance, PluginCommand: cmdShowInstance},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-neighbor", Handler: forwardShowOSPFNeighbor, PluginCommand: cmdShowNeighbor},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-interface", Handler: forwardShowOSPFInterface, PluginCommand: cmdShowInterface},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-ipv6-interface", Handler: afViewForwarder(cmdShowIPv6Interface), PluginCommand: cmdShowIPv6Interface},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-database", Handler: forwardShowOSPFDatabase, PluginCommand: cmdShowDatabase},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-route", Handler: forwardShowOSPFRoute, PluginCommand: cmdShowRoute},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-route-fast-reroute", Handler: forwardShowOSPFRouteFastReroute, PluginCommand: cmdShowRouteFastReroute},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-virtual-links", Handler: forwardShowOSPFVirtualLinks, PluginCommand: cmdShowVirtualLinks},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-border-routers", Handler: forwardShowOSPFBorderRouters, PluginCommand: cmdShowBorderRouters},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-spf", Handler: forwardShowOSPFSPF, PluginCommand: cmdShowSPF},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-ldp-sync", Handler: forwardShowOSPFLDPSync, PluginCommand: cmdShowLDPSync},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-graceful-restart", Handler: forwardShowOSPFGracefulRestart, PluginCommand: cmdShowGracefulRestart},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-ipv6-graceful-restart", Handler: afViewForwarder(cmdShowIPv6GracefulRestart), PluginCommand: cmdShowIPv6GracefulRestart},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-segment-routing", Handler: forwardShowOSPFSegmentRouting, PluginCommand: cmdShowSegmentRouting},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-ipv6-segment-routing", Handler: afViewForwarder(cmdShowIPv6SegmentRouting), PluginCommand: cmdShowIPv6SegmentRouting},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-database-router", Handler: dbSubviewForwarder(cmdShowDatabaseRouter), PluginCommand: cmdShowDatabaseRouter},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-database-network", Handler: dbSubviewForwarder(cmdShowDatabaseNetwork), PluginCommand: cmdShowDatabaseNetwork},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-database-summary", Handler: dbSubviewForwarder(cmdShowDatabaseSummary), PluginCommand: cmdShowDatabaseSummary},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-database-asbr-summary", Handler: dbSubviewForwarder(cmdShowDatabaseASBRSummary), PluginCommand: cmdShowDatabaseASBRSummary},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-database-external", Handler: dbSubviewForwarder(cmdShowDatabaseExternal), PluginCommand: cmdShowDatabaseExternal},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-database-nssa-external", Handler: dbSubviewForwarder(cmdShowDatabaseNSSAExternal), PluginCommand: cmdShowDatabaseNSSAExternal},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-database-opaque-link", Handler: dbSubviewForwarder(cmdShowDatabaseOpaqueLink), PluginCommand: cmdShowDatabaseOpaqueLink},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-database-opaque-area", Handler: dbSubviewForwarder(cmdShowDatabaseOpaqueArea), PluginCommand: cmdShowDatabaseOpaqueArea},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-database-opaque-as", Handler: dbSubviewForwarder(cmdShowDatabaseOpaqueAS), PluginCommand: cmdShowDatabaseOpaqueAS},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-database-router-information", Handler: afViewForwarder(cmdShowDatabaseRI), PluginCommand: cmdShowDatabaseRI},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-te-database", Handler: dbSubviewForwarder(cmdShowTEDatabase), PluginCommand: cmdShowTEDatabase},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:clear-process", Handler: forwardClearOSPFProcess, PluginCommand: cmdClearProcess},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:clear-neighbor", Handler: forwardClearOSPFNeighbor, PluginCommand: cmdClearNeighbor},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:clear-counters", Handler: forwardClearOSPFCounters, PluginCommand: cmdClearCounters},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:graceful-restart-prepare", Handler: forwardOSPFGRPrepare, PluginCommand: cmdGRPrepare},

		// spec-ospf-ext-14 IPv4 deep-introspection views.
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-database-opaque-area-detail", Handler: dbSubviewForwarder(cmdShowDatabaseOpaqueAreaDetail), PluginCommand: cmdShowDatabaseOpaqueAreaDetail},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-database-opaque-as-detail", Handler: dbSubviewForwarder(cmdShowDatabaseOpaqueASDetail), PluginCommand: cmdShowDatabaseOpaqueASDetail},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-database-opaque-link-detail", Handler: dbSubviewForwarder(cmdShowDatabaseOpaqueLinkDetail), PluginCommand: cmdShowDatabaseOpaqueLinkDetail},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-spf-detail", Handler: dbSubviewForwarder(cmdShowSPFDetail), PluginCommand: cmdShowSPFDetail},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-neighbor-detail", Handler: dbSubviewForwarder(cmdShowNeighborDetail), PluginCommand: cmdShowNeighborDetail},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-interface-detail", Handler: dbSubviewForwarder(cmdShowInterfaceDetail), PluginCommand: cmdShowInterfaceDetail},

		// spec-ospf-ext-14 IPv6 (OSPFv3) deep-introspection views. Distinct ze-ospf:show-ospfv3-*
		// wire methods so the v4 and v6 surfaces cannot collide (AC-26, R-9).
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-ospfv3-database", Handler: afViewForwarder(cmdShowIPv6Database), PluginCommand: cmdShowIPv6Database},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-ospfv3-database-detail", Handler: afViewForwarder(cmdShowIPv6DatabaseDetail), PluginCommand: cmdShowIPv6DatabaseDetail},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-ospfv3-database-router-detail", Handler: afViewForwarder(cmdShowIPv6DatabaseRouterDetail), PluginCommand: cmdShowIPv6DatabaseRouterDetail},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-ospfv3-database-scope-link", Handler: afViewForwarder(cmdShowIPv6DatabaseScopeLink), PluginCommand: cmdShowIPv6DatabaseScopeLink},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-ospfv3-database-scope-area", Handler: afViewForwarder(cmdShowIPv6DatabaseScopeArea), PluginCommand: cmdShowIPv6DatabaseScopeArea},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-ospfv3-database-scope-as", Handler: afViewForwarder(cmdShowIPv6DatabaseScopeAS), PluginCommand: cmdShowIPv6DatabaseScopeAS},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-ospfv3-database-router-information", Handler: afViewForwarder(cmdShowIPv6DatabaseRI), PluginCommand: cmdShowIPv6DatabaseRI},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-ospfv3-database-extended", Handler: afViewForwarder(cmdShowIPv6DatabaseExtended), PluginCommand: cmdShowIPv6DatabaseExtended},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-ospfv3-database-segment-routing", Handler: afViewForwarder(cmdShowIPv6DatabaseSegmentRouting), PluginCommand: cmdShowIPv6DatabaseSegmentRouting},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-ospfv3-instance", Handler: dbSubviewForwarder(cmdShowIPv6Instance), PluginCommand: cmdShowIPv6Instance},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-ospfv3-neighbor", Handler: afViewForwarder(cmdShowIPv6Neighbor), PluginCommand: cmdShowIPv6Neighbor},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-ospfv3-neighbor-detail", Handler: afViewForwarder(cmdShowIPv6NeighborDetail), PluginCommand: cmdShowIPv6NeighborDetail},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-ospfv3-interface-detail", Handler: afViewForwarder(cmdShowIPv6InterfaceDetail), PluginCommand: cmdShowIPv6InterfaceDetail},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-ospfv3-spf", Handler: afViewForwarder(cmdShowIPv6SPF), PluginCommand: cmdShowIPv6SPF},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-ospfv3-spf-detail", Handler: afViewForwarder(cmdShowIPv6SPFDetail), PluginCommand: cmdShowIPv6SPFDetail},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:show-ospfv3-route", Handler: afViewForwarder(cmdShowIPv6Route), PluginCommand: cmdShowIPv6Route},

		// spec-ospf-ext-14 guarded LSA injection (both families). The inject proxies pass the
		// trailing keyword-value tokens through as args; the read-only authz `deny debug`
		// blocks them before dispatch (AC-16) and the engine debug-enablement is the 2nd gate.
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:debug-inject", Handler: forwardOSPFInjectV4, PluginCommand: cmdDebugInjectOpaque},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:debug-ospfv3-inject", Handler: forwardOSPFInjectV6, PluginCommand: cmdDebugInjectLSA},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:debug-inject-enable", Handler: dbSubviewForwarder(cmdDebugInjectEnable), PluginCommand: cmdDebugInjectEnable},
		pluginserver.RPCRegistration{WireMethod: "ze-ospf:debug-inject-disable", Handler: dbSubviewForwarder(cmdDebugInjectDisable), PluginCommand: cmdDebugInjectDisable},
	)
}

// forwardOSPFInjectV4 proxies `debug ip ospf inject opaque ...` to the OSPF engine, passing
// the trailing scope/id/type/hex tokens through as args (the inject grammar is variable).
func forwardOSPFInjectV4(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return forwardToOSPFArgs(ctx, cmdDebugInjectOpaque, args)
}

// forwardOSPFInjectV6 proxies `debug ipv6 ospf inject lsa ...` to the OSPF engine.
func forwardOSPFInjectV6(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return forwardToOSPFArgs(ctx, cmdDebugInjectLSA, args)
}

// forwardToOSPFArgs proxies a command AND its trailing arguments to the OSPF engine (unlike
// forwardToOSPF, which rejects extra args for the fixed-noun show/clear commands).
func forwardToOSPFArgs(ctx *pluginserver.CommandContext, command string, args []string) (*plugin.Response, error) {
	d := ctx.Dispatcher()
	if d == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "dispatcher unavailable"}, nil
	}
	return d.ForwardToPlugin(ctx, command, args, ctx.PeerSelector())
}

func forwardShowOSPFProcess(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return forwardToOSPF(ctx, cmdShowProcess, args)
}

func forwardShowOSPFIPv6(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return forwardToOSPF(ctx, cmdShowIPv6, args)
}

func forwardShowOSPFInstance(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return forwardToOSPF(ctx, cmdShowInstance, args)
}

func forwardShowOSPFNeighbor(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return forwardToOSPF(ctx, cmdShowNeighbor, args)
}

func forwardShowOSPFInterface(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return forwardToOSPF(ctx, cmdShowInterface, args)
}

func forwardShowOSPFDatabase(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return forwardToOSPF(ctx, cmdShowDatabase, args)
}

func forwardShowOSPFRoute(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return forwardToOSPF(ctx, cmdShowRoute, args)
}

func forwardShowOSPFRouteFastReroute(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return forwardToOSPF(ctx, cmdShowRouteFastReroute, args)
}

func forwardShowOSPFVirtualLinks(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return forwardToOSPF(ctx, cmdShowVirtualLinks, args)
}

func forwardShowOSPFBorderRouters(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return forwardToOSPF(ctx, cmdShowBorderRouters, args)
}

func forwardShowOSPFSPF(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return forwardToOSPF(ctx, cmdShowSPF, args)
}

func forwardShowOSPFLDPSync(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return forwardToOSPF(ctx, cmdShowLDPSync, args)
}

func forwardShowOSPFGracefulRestart(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return forwardToOSPF(ctx, cmdShowGracefulRestart, args)
}

func forwardShowOSPFSegmentRouting(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return forwardToOSPF(ctx, cmdShowSegmentRouting, args)
}

func forwardClearOSPFProcess(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return forwardToOSPF(ctx, cmdClearProcess, args)
}

func forwardClearOSPFNeighbor(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return forwardToOSPF(ctx, cmdClearNeighbor, args)
}

func forwardClearOSPFCounters(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return forwardToOSPF(ctx, cmdClearCounters, args)
}

// forwardOSPFGRPrepare proxies the operator `request ospf graceful-restart` action to the OSPF
// engine, which runs prepareRestart against its live state (RFC 3623 sec 2.1). Same forwarding
// contract as the clear commands: the noun is fixed in the grammar, so no arguments are taken.
func forwardOSPFGRPrepare(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return forwardToOSPF(ctx, cmdGRPrepare, args)
}

// dbSubviewForwarder builds a handler that proxies one `show ospf database <type>`
// subview to the engine. A closure avoids six near-identical named functions.
func dbSubviewForwarder(pluginCommand string) pluginserver.Handler {
	return func(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
		args := validated.Tokens()
		return forwardToOSPF(ctx, pluginCommand, args)
	}
}

// afViewForwarder builds the handler of one OSPFv3 view that takes the optional
// address-family keyword. The registry has already judged the value against the
// enumeration the model declares (yang/ze-ospf-cmd.yang, grouping
// ospfv3-address-family). Which families answer is the plugin's decision, because
// only the plugin knows which engines run, so the handler forwards and chooses
// nothing.
func afViewForwarder(pluginCommand string) pluginserver.Handler {
	return func(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
		args, refusal := addressFamilyArgs(pluginCommand, validated)
		if refusal != nil {
			return plugin.OperatorErrorResponse(refusal), nil
		}
		return forwardToOSPFArgs(ctx, pluginCommand, args)
	}
}

// addressFamilyArgs answers the tokens an OSPFv3 view hands the plugin: none, or
// keywordAddressFamily followed by the value the registry judged. The plugin then
// reads one shape however the operator typed the line: a bare family name the
// validator bound positionally is forwarded behind its keyword.
//
// A token no leaf took is refused rather than forwarded or dropped. Forwarded, the
// plugin would have to guess what it means; dropped, the view would answer a
// question the operator did not ask, every family where one was meant.
// `show ospf ipv6 neighbor ipv4-unicast detail` is that line: two spare tokens, so
// the validator binds neither to the leaf.
func addressFamilyArgs(pluginCommand string, validated command.ValidatedArgs) ([]string, *opererr.Error) {
	spare := validated.PositionalTokens()
	if len(spare) > 0 {
		// A lone spare token the validator bound to the leaf is the family itself.
		if _, bound := validated.Positional(keywordAddressFamily); !bound {
			// The fix is the bare command, which answers every running family: which
			// families run is the plugin's knowledge, so the daemon cannot spell a
			// narrowed line that would answer.
			var problem textbuf.Buffer
			problem.Str("unexpected words ").Quoted(textbuf.Join(spare, " ")).Str(" for ").Str(pluginCommand)
			return nil, opererr.New(problem.String(),
				"the command takes one optional keyword, address-family, followed by a family that show ospf ipv6 instance lists",
				pluginCommand)
		}
	}
	family, found := validated.Value(keywordAddressFamily)
	if !found {
		return nil, nil
	}
	return []string{keywordAddressFamily, family}, nil
}

// forwardToOSPF routes a fixed plugin command to the OSPF engine. The proxied commands
// accept no arguments (the noun is baked into the fixed command string by the grammar),
// so any extra args are rejected (the LDP/IS-IS proxy contract).
func forwardToOSPF(ctx *pluginserver.CommandContext, command string, args []string) (*plugin.Response, error) {
	if len(args) > 0 {
		var tb textbuf.Buffer
		return &plugin.Response{Status: plugin.StatusError, Error: tb.Str("unexpected argument; ").Str(command).Str(" takes none").String()}, nil
	}
	d := ctx.Dispatcher()
	if d == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "dispatcher unavailable"}, nil
	}
	return d.ForwardToPlugin(ctx, command, args, ctx.PeerSelector())
}
