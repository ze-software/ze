// Design: docs/architecture/api/commands.md — RIB CLI proxy handlers
// Overview: doc.go — bgp-cmd-rib plugin registration
//
// Package rib registers CLI proxy handlers that forward RIB commands to
// the bgp-rib plugin process. Each handler bridges the compile-time builtin
// RPC path (AllBuiltinRPCs → BuildCommandTree → ze show/run) to the runtime
// plugin command path (CommandRegistry → routeToProcess → bgp-rib SDK).
//
// Without these proxies, RIB commands are only reachable through the
// interactive CLI's plugin dispatch fallback, not through ze show/run.
package rib

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// Plugin command names — used in both RPCRegistration.PluginCommand and ForwardToPlugin
// to prevent divergence between the two.
const (
	cmdRibStatus     = "show bgp rib status"
	cmdRibShow       = "show bgp rib"
	cmdRibBest       = "show bgp rib best"
	cmdRibBestStatus = "show bgp rib best status"
	cmdRibClearIn    = "clear bgp rib in"
	cmdRibClearOut   = "clear bgp rib out"
	cmdRibInject     = "request bgp rib inject"
	cmdRibWithdraw   = "request bgp rib withdraw"
	cmdRibRPF        = "show bgp rib rpf"
	// cmdBgpPeerRib is the peer-scoped spelling of cmdRibShow. It forwards to
	// the same plugin command with a selector, so it answers the same rows.
	cmdBgpPeerRib = "show bgp peer rib"
)

// The keys of a route row, as the bgp-rib plugin writes them and as the column
// orders and address-field lists below declare them.
//
// `show bgp rib` and `show bgp peer rib` answer the SAME rows and so declare
// the same eleven columns twice. Naming each key once is what stops the two
// declarations drifting apart: the renderer pairs a declared name with a row
// key by exact spelling (commandRegistry.lookup in
// internal/component/command/column_order.go), so a name that only one of them
// misspells is a column ordered on one command and not the other, with nothing
// reporting it.
//
// The pipe filter names in registerPipeFilters are a different vocabulary and
// stay literal: a filter name is what an OPERATOR types, and `| peer` reaching
// the "peer" field is the bgp-rib pipeline's business, not this file's.
const (
	fieldPeer        = "peer"
	fieldDirection   = "direction"
	fieldFamily      = "family"
	fieldPrefix      = "prefix"
	fieldNextHop     = "next-hop"
	fieldPathID      = "path-id"
	fieldASPath      = "as-path"
	fieldOrigin      = "origin"
	fieldLocalPref   = "local-pref"
	fieldMED         = "med"
	fieldCommunities = "communities"

	// fieldCount is the row count `show bgp rib pool-stats` answers beside its
	// pool rows (pool_stats.go). It is NOT the `| count` pipe operator, which
	// registerPipeFilters spells for itself.
	fieldCount = "count"
)

func init() {
	registerPipeFilters()

	pluginserver.RegisterRPCs(
		// Read-only commands (exposed via "ze show")
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:status", Handler: forwardRibStatus, PluginCommand: cmdRibStatus},
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:routes", Handler: forwardRibRoutes, PluginCommand: cmdRibShow},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-rib", Handler: forwardRibRoutes, PluginCommand: cmdRibShow, RequiresSelector: true},
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:best", Handler: forwardRibBest, PluginCommand: cmdRibBest},
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:best-status", Handler: forwardRibBestStatus, PluginCommand: cmdRibBestStatus},
		// Write commands (exposed via "ze run" only)
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:clear-in", Handler: forwardRibClearIn, PluginCommand: cmdRibClearIn},
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:clear-out", Handler: forwardRibClearOut, PluginCommand: cmdRibClearOut},
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:inject", Handler: forwardRibInject, PluginCommand: cmdRibInject},
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:withdraw", Handler: forwardRibWithdraw, PluginCommand: cmdRibWithdraw},
		pluginserver.RPCRegistration{WireMethod: "ze-rib-api:rpf", Handler: forwardRibRPF, PluginCommand: cmdRibRPF},
	)
}

func registerPipeFilters() {
	command.RegisterPipeFilters([]string{cmdRibShow},
		command.PipeFilter{Name: "received", Description: "Select received routes", Leading: true},
		command.PipeFilter{Name: "advertised", Description: "Select advertised routes", Leading: true},
		command.PipeFilter{Name: "peer", Description: "Filter by peer", TakesArg: true},
		command.PipeFilter{Name: "family", Description: "Filter by AFI/SAFI", TakesArg: true},
		command.PipeFilter{Name: "prefix", Description: "Filter by prefix", TakesArg: true},
		command.PipeFilter{Name: "path", Description: "Filter by AS path", TakesArg: true},
		command.PipeFilter{Name: "community", Description: "Filter by standard community", TakesArg: true},
		command.PipeFilter{Name: "match", Description: "Cross-field structured match", TakesArg: true},
		command.PipeFilter{Name: "count", Description: "Count matching routes without serializing rows"},
		command.PipeFilter{Name: "first", Description: "Take first N routes", TakesArg: true},
		command.PipeFilter{Name: "last", Description: "Take last N routes", TakesArg: true},
		command.PipeFilter{Name: "histogram", Description: "Count routes by family and prefix length"},
		command.PipeFilter{Name: "graph", Description: "Render AS-path topology graph"},
	)
	command.RegisterPipeFilters([]string{cmdRibBest},
		command.PipeFilter{Name: "peer", Description: "Filter by peer", TakesArg: true},
		command.PipeFilter{Name: "family", Description: "Filter by AFI/SAFI", TakesArg: true},
		command.PipeFilter{Name: "prefix", Description: "Filter by prefix", TakesArg: true},
		command.PipeFilter{Name: "path", Description: "Filter by AS path", TakesArg: true},
		command.PipeFilter{Name: "community", Description: "Filter by standard community", TakesArg: true},
		command.PipeFilter{Name: "match", Description: "Cross-field structured match", TakesArg: true},
		command.PipeFilter{Name: "count", Description: "Count matching best paths without serializing rows"},
		command.PipeFilter{Name: "first", Description: "Take first N best paths", TakesArg: true},
		command.PipeFilter{Name: "last", Description: "Take last N best paths", TakesArg: true},
		command.PipeFilter{Name: "histogram", Description: "Count routes by family and prefix length"},
		command.PipeFilter{Name: "graph", Description: "Render AS-path topology graph"},
		command.PipeFilter{Name: "reason", Description: "Explain best-path selection"},
	)
	// Scalar-result commands: an EMPTY filter set, not an absent one. Filter
	// lookup is longest-prefix (command.commandMatchesPrefix), so without this
	// they would inherit cmdRibShow's route filters via the "show bgp rib"
	// prefix and offer `| peer`, `| histogram`, `| graph` on output that
	// has no routes to filter. Registering empty overrides that inheritance.
	command.RegisterPipeFilters([]string{cmdRibStatus, cmdRibBestStatus, cmdRibRPF})

	// `show bgp rib` answers FLAT ROWS: one envelope, one row per route, each
	// row carrying `peer` and `direction` as fields (owner ruling, 2026-08-23).
	// It answered an object keyed by direction and then by peer before, which
	// no row operator could act on: `| peer 10.0.0.1 | direction in` cannot be
	// expressed against two levels of envelope, and that is why the shape
	// changed.
	//
	// The order below is the one an operator reads a route in: whose it is and
	// which way it went, then which route, then what it carries.
	command.RegisterShape([]string{cmdRibShow}, command.ShapeTab)
	command.RegisterColumns([]string{cmdRibShow},
		command.ColumnOrder{
			fieldPeer, fieldDirection, fieldFamily, fieldPrefix,
			fieldNextHop, fieldPathID, fieldASPath, fieldOrigin, fieldLocalPref, fieldMED, fieldCommunities,
		},
	)

	// The peer address is what `| resolve` and `| origin` act on. Nothing else
	// in the row is declared an address, so neither operator decorates a field
	// by coincidence.
	command.RegisterAddressFields([]string{cmdRibShow}, fieldPeer, fieldNextHop)

	registerRibAnswerShapes()
}

// registerRibAnswerShapes declares what the rib commands OTHER than
// `show bgp rib` answer. That one declares beside its own pipe filters above,
// because the filters and the shape describe the same route rows.
//
// Each path here declares for ITSELF, and that is the whole point. A shape, a
// column order and an address-field list all resolve by the longest registered
// command path that is a prefix of the command (commandRegistry.lookup in
// internal/component/command/column_order.go), so the route declaration above
// reached the four paths beneath `show bgp rib` through that prefix. Each was
// published as supporting the row operators, was offered eleven route columns
// it never writes, and `| resolve` was accepted on a "peer" field none of them
// has. The empty filter set above already answered the same problem for the
// pipe-filter registry; these three registries needed the same answer.
func registerRibAnswerShapes() {
	// `show bgp peer rib` forwards to cmdRibShow with a peer selector
	// (forwardRibRoutes), so it answers the same route rows and declares the
	// same three things. It inherits none of them: it sits under
	// `show bgp peer`, and the BGP peer command plugin declares that path empty.
	command.RegisterShape([]string{cmdBgpPeerRib}, command.ShapeTab)
	command.RegisterColumns([]string{cmdBgpPeerRib},
		command.ColumnOrder{
			fieldPeer, fieldDirection, fieldFamily, fieldPrefix,
			fieldNextHop, fieldPathID, fieldASPath, fieldOrigin, fieldLocalPref, fieldMED, fieldCommunities,
		},
	)
	command.RegisterAddressFields([]string{cmdBgpPeerRib}, fieldPeer, fieldNextHop)

	// `show bgp rib best` answers one row for each best path, under
	// "best-path". The row is bestResult
	// (internal/component/bgp/plugins/rib/rib_pipeline_best.go) and it shares
	// only "family" and "prefix" with a route row, so the route order above
	// would have named nine columns it never writes.
	//
	// The order is the one an operator reads a best path in: which family and
	// which prefix, then who won it, then who tied with the winner, then what
	// the winning path carries.
	command.RegisterShape([]string{cmdRibBest}, command.ShapeTab)
	command.RegisterColumns([]string{cmdRibBest},
		command.ColumnOrder{fieldFamily, fieldPrefix, "best-peer", "multipath-peers", "attributes"},
	)
	// Two values of the row hold a bare IP address: the winning peer, and the
	// next hop inside "attributes" (enrichRouteMapFromEntry, rib_attr_format.go).
	// "prefix" and "multipath-peers" hold neither, and declaring either would
	// publish an operator that decorates nothing: resolveJSON and originJSON
	// decorate a map value that passes netip.ParseAddr
	// (internal/component/command/pipe_resolve.go, pipe_origin.go), a prefix
	// string does not parse, and an array element is walked past.
	command.RegisterAddressFields([]string{cmdRibBest}, "best-peer", fieldNextHop)

	// The three scalar commands answer ONE document, so every row operator is
	// refused by name before the command runs rather than after it.
	command.RegisterShape([]string{cmdRibStatus, cmdRibBestStatus, cmdRibRPF}, command.ShapeDoc)

	// `show bgp rib status` (RIBManager.status,
	// internal/component/bgp/plugins/rib/rib_commands.go) carries the totals,
	// then the per-peer counts, then the graceful-restart state that only a
	// restarting peer produces.
	//
	// Its two per-peer maps are also why the declaration matters more here than
	// anywhere else in this file. Each is keyed by peer address and each reads
	// as a row set, so the shape DERIVED from the answer was rows while no peer
	// was restarting and no rows once one was (rowsInKeyed refuses two). The
	// declared shape is the same answer in both.
	command.RegisterColumns([]string{cmdRibStatus},
		command.ColumnOrder{
			"running", "peers", "routes-in", "routes-out", "stale-routes",
			"route-counts", "gr-state",
		},
	)
	// RIBManager.bestPathStatus, same file.
	command.RegisterColumns([]string{cmdRibBestStatus},
		command.ColumnOrder{"running", "peers-with-rib", "total-routes"},
	)
	// RIBManager.rpfLookup, same file. The four keys after "found" are written
	// only when the lookup matched, and a declared order never hides or invents
	// a key, so one order serves both branches.
	command.RegisterColumns([]string{cmdRibRPF},
		command.ColumnOrder{
			"source", fieldFamily, "found",
			"matched-prefix", fieldNextHop, "admin-distance", "metric",
		},
	)

	// "source" and "next-hop" hold a bare address, so the rpf answer says so
	// even though its `doc` shape refuses both operators on its own: the
	// refusal an operator reads then names the real reason, which is that the
	// answer has no rows, rather than claiming no field holds an address.
	// "matched-prefix" is a prefix and fails netip.ParseAddr, so it is not one.
	command.RegisterAddressFields([]string{cmdRibRPF}, "source", fieldNextHop)
	// The two status answers hold peer addresses as the KEYS of a map and in no
	// field, so neither operator has anything to decorate.
	command.RegisterAddressFields([]string{cmdRibStatus, cmdRibBestStatus})
}

func forwardRibStatus(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdRibStatus, args, ctx.PeerSelector())
}

func forwardRibRoutes(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdRibShow, args, ctx.PeerSelector())
}

func forwardRibBest(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdRibBest, args, ctx.PeerSelector())
}

func forwardRibBestStatus(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdRibBestStatus, args, ctx.PeerSelector())
}

func forwardRibClearIn(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdRibClearIn, args, ctx.PeerSelector())
}

func forwardRibClearOut(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdRibClearOut, args, ctx.PeerSelector())
}

func forwardRibInject(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdRibInject, args, ctx.PeerSelector())
}

func forwardRibWithdraw(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdRibWithdraw, args, ctx.PeerSelector())
}

func forwardRibRPF(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdRibRPF, args, ctx.PeerSelector())
}
